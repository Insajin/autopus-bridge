package computeruse

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// 컴파일 타임 인터페이스 구현 확인
var _ DockerClient = (*CLIDockerClient)(nil)

// CLIDockerClient는 Docker CLI를 통해 DockerClient 인터페이스를 구현한다.
// Docker SDK 의존성 없이 exec.CommandContext로 docker 명령어를 실행한다.
type CLIDockerClient struct {
	dockerPath string // docker 바이너리 경로 (기본: "docker")
}

// NewCLIDockerClient는 지정된 docker 바이너리 경로로 새 CLIDockerClient를 생성한다.
// dockerPath가 빈 문자열이면 기본값 "docker"를 사용한다.
func NewCLIDockerClient(dockerPath string) *CLIDockerClient {
	if dockerPath == "" {
		dockerPath = "docker"
	}
	return &CLIDockerClient{
		dockerPath: dockerPath,
	}
}

// runCmd는 Docker CLI 명령어를 실행하고 stdout 결과를 반환한다.
func (c *CLIDockerClient) runCmd(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, c.dockerPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

// Ping은 Docker 데몬 연결을 확인한다.
// `docker info --format '{{.ServerVersion}}'`을 실행하여 서버 버전을 확인한다.
func (c *CLIDockerClient) Ping(ctx context.Context) error {
	output, err := c.runCmd(ctx, "info", "--format", "{{.ServerVersion}}")
	if err != nil {
		return fmt.Errorf("docker 데몬 ping 실패: %w", err)
	}
	if output == "" {
		return fmt.Errorf("docker 데몬으로부터 빈 응답을 받았습니다")
	}
	log.Printf("[computer-use] docker 데몬 연결 확인: version=%s", output)
	return nil
}

// buildCreateArgs는 ContainerCreateConfig로부터 docker create 명령어 인자를 생성한다.
func buildCreateArgs(config *ContainerCreateConfig) []string {
	args := []string{"create"}

	// 메모리 제한
	if config.MemoryLimit > 0 {
		args = append(args, "--memory", strconv.FormatInt(config.MemoryLimit, 10))
	}

	// CPU 할당량 (CPUQuota/100000.0 -> CPUs 형식)
	if config.CPUQuota > 0 {
		cpus := float64(config.CPUQuota) / 100000.0
		args = append(args, "--cpus", strconv.FormatFloat(cpus, 'f', 1, 64))
	}

	// PID 제한
	if config.PIDLimit > 0 {
		args = append(args, "--pids-limit", strconv.FormatInt(config.PIDLimit, 10))
	}

	// 읽기 전용 루트 파일시스템
	if config.ReadOnly {
		args = append(args, "--read-only")
	}

	// 실행 사용자
	if config.User != "" {
		args = append(args, "--user", config.User)
	}

	// tmpfs 마운트 (/tmp, /dev/shm)
	if config.TmpfsSize != "" {
		args = append(args, "--tmpfs", fmt.Sprintf("/tmp:size=%s", config.TmpfsSize))
		args = append(args, "--tmpfs", fmt.Sprintf("/dev/shm:size=%s", config.TmpfsSize))
	}

	// Docker 네트워크
	if config.Network != "" {
		args = append(args, "--network", config.Network)
	}

	// CDP 포트 매핑 (랜덤 호스트 포트 -> 컨테이너 9222)
	args = append(args, "-p", "0:9222")

	// Docker 이미지 (마지막 인자)
	args = append(args, config.Image)

	return args
}

// ContainerCreate는 새 컨테이너를 생성하고 컨테이너 ID를 반환한다.
func (c *CLIDockerClient) ContainerCreate(ctx context.Context, config *ContainerCreateConfig) (string, error) {
	args := buildCreateArgs(config)

	output, err := c.runCmd(ctx, args...)
	if err != nil {
		return "", fmt.Errorf("컨테이너 생성 실패: %w", err)
	}

	containerID := strings.TrimSpace(output)
	if containerID == "" {
		return "", fmt.Errorf("컨테이너 생성 후 빈 ID를 반환받았습니다")
	}

	log.Printf("[computer-use] CLI 컨테이너 생성: id=%s", containerID[:min(12, len(containerID))])
	return containerID, nil
}

// ContainerStart는 컨테이너를 시작한다.
func (c *CLIDockerClient) ContainerStart(ctx context.Context, containerID string) error {
	_, err := c.runCmd(ctx, "start", containerID)
	if err != nil {
		return fmt.Errorf("컨테이너 시작 실패 (id=%s): %w", containerID[:min(12, len(containerID))], err)
	}
	return nil
}

// ContainerStop은 컨테이너를 정지한다.
// timeout이 nil이면 기본값 10초를 사용한다.
func (c *CLIDockerClient) ContainerStop(ctx context.Context, containerID string, timeout *time.Duration) error {
	seconds := 10
	if timeout != nil {
		seconds = int(timeout.Seconds())
	}

	_, err := c.runCmd(ctx, "stop", "--time", strconv.Itoa(seconds), containerID)
	if err != nil {
		return fmt.Errorf("컨테이너 정지 실패 (id=%s): %w", containerID[:min(12, len(containerID))], err)
	}
	return nil
}

// ContainerRemove는 컨테이너를 삭제한다.
// force가 true이면 -f 옵션으로 강제 삭제한다.
func (c *CLIDockerClient) ContainerRemove(ctx context.Context, containerID string, force bool) error {
	args := []string{"rm"}
	if force {
		args = append(args, "-f")
	}
	args = append(args, containerID)

	_, err := c.runCmd(ctx, args...)
	if err != nil {
		return fmt.Errorf("컨테이너 삭제 실패 (id=%s): %w", containerID[:min(12, len(containerID))], err)
	}
	return nil
}

// parseInspectOutput은 docker inspect의 포맷된 출력을 파싱한다.
// 출력 형식: "컨테이너ID|상태|호스트포트"
func parseInspectOutput(output string) (*ContainerInspectResult, error) {
	parts := strings.SplitN(output, "|", 3)
	if len(parts) != 3 {
		return nil, fmt.Errorf("예상하지 못한 inspect 출력 형식: %q", output)
	}

	return &ContainerInspectResult{
		ID:       parts[0],
		Status:   parts[1],
		HostPort: parts[2],
	}, nil
}

// ContainerInspect는 컨테이너 상태 정보를 조회한다.
func (c *CLIDockerClient) ContainerInspect(ctx context.Context, containerID string) (*ContainerInspectResult, error) {
	format := `{{.Id}}|{{.State.Status}}|{{(index (index .NetworkSettings.Ports "9222/tcp") 0).HostPort}}`
	output, err := c.runCmd(ctx, "inspect", "--format", format, containerID)
	if err != nil {
		return nil, fmt.Errorf("컨테이너 조회 실패 (id=%s): %w", containerID[:min(12, len(containerID))], err)
	}

	result, err := parseInspectOutput(output)
	if err != nil {
		return nil, fmt.Errorf("컨테이너 조회 결과 파싱 실패: %w", err)
	}

	return result, nil
}

// NetworkCreate는 Docker 네트워크를 생성한다.
func (c *CLIDockerClient) NetworkCreate(ctx context.Context, name string) error {
	_, err := c.runCmd(ctx, "network", "create", name)
	if err != nil {
		return fmt.Errorf("네트워크 생성 실패 (name=%s): %w", name, err)
	}
	log.Printf("[computer-use] Docker 네트워크 생성: %s", name)
	return nil
}

// NetworkInspect는 네트워크 존재 여부를 확인한다.
// 네트워크가 존재하면 nil, 존재하지 않으면 에러를 반환한다.
func (c *CLIDockerClient) NetworkInspect(ctx context.Context, name string) error {
	_, err := c.runCmd(ctx, "network", "inspect", name)
	if err != nil {
		return fmt.Errorf("네트워크 조회 실패 (name=%s): %w", name, err)
	}
	return nil
}

// ImageInspect는 이미지 존재 여부를 확인한다.
// 이미지가 존재하면 nil, 존재하지 않으면 에러를 반환한다.
func (c *CLIDockerClient) ImageInspect(ctx context.Context, imageRef string) error {
	_, err := c.runCmd(ctx, "image", "inspect", imageRef)
	if err != nil {
		return fmt.Errorf("이미지 조회 실패 (ref=%s): %w", imageRef, err)
	}
	return nil
}

// ImagePull은 이미지를 풀하고 프로세스의 stdout을 ReadCloser로 반환한다.
// 호출자는 반환된 ReadCloser를 닫아야 한다.
func (c *CLIDockerClient) ImagePull(ctx context.Context, imageRef string) (io.ReadCloser, error) {
	cmd := exec.CommandContext(ctx, c.dockerPath, "pull", imageRef)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("이미지 풀 stdout 파이프 생성 실패: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("이미지 풀 시작 실패 (ref=%s): %w", imageRef, err)
	}

	log.Printf("[computer-use] 이미지 풀 시작: %s", imageRef)

	// pullReadCloser는 stdout을 읽고 프로세스 완료를 대기하는 ReadCloser이다.
	return &pullReadCloser{
		ReadCloser: stdout,
		cmd:        cmd,
	}, nil
}

// pullReadCloser는 docker pull 프로세스의 stdout을 래핑한다.
// Close 시 프로세스 완료를 대기한다.
type pullReadCloser struct {
	io.ReadCloser
	cmd *exec.Cmd
}

// Close는 stdout 파이프를 닫고 프로세스 완료를 대기한다.
func (p *pullReadCloser) Close() error {
	// stdout 파이프 먼저 닫기
	if err := p.ReadCloser.Close(); err != nil {
		// 파이프 닫기 실패해도 프로세스 대기는 시도
		_ = p.cmd.Wait()
		return fmt.Errorf("이미지 풀 stdout 닫기 실패: %w", err)
	}
	// 프로세스 완료 대기
	if err := p.cmd.Wait(); err != nil {
		return fmt.Errorf("이미지 풀 프로세스 완료 대기 실패: %w", err)
	}
	return nil
}

// Close는 CLI 기반 클라이언트의 정리 작업을 수행한다.
// CLI 클라이언트는 상태를 유지하지 않으므로 no-op이다.
func (c *CLIDockerClient) Close() error {
	return nil
}
