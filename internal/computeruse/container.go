package computeruse

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

// ContainerConfig는 컨테이너 생성에 필요한 설정을 정의한다.
type ContainerConfig struct {
	Image       string        // Docker 이미지 이름 (기본: "autopus/chromium-sandbox:latest")
	Network     string        // Docker 네트워크 이름 (기본: "autopus-sandbox-net")
	MemoryLimit int64         // 메모리 제한 (바이트, 기본: 512MB)
	CPUQuota    int64         // CPU 할당량 (기본: 100000 = 1.0 CPU)
	PIDLimit    int64         // PID 제한 (기본: 100)
	TmpfsSize   string        // tmpfs 크기 (기본: "64m")
	StartTimeout time.Duration // 컨테이너 시작 타임아웃 (기본: 30초)
}

// DefaultContainerConfig는 기본 컨테이너 설정을 반환한다.
func DefaultContainerConfig() ContainerConfig {
	return ContainerConfig{
		Image:        "autopus/chromium-sandbox:latest",
		Network:      "autopus-sandbox-net",
		MemoryLimit:  512 * 1024 * 1024, // 512MB
		CPUQuota:     100000,             // 1.0 CPU
		PIDLimit:     100,
		TmpfsSize:    "64m",
		StartTimeout: 30 * time.Second,
	}
}

// ContainerInfo는 생성된 컨테이너의 정보를 담는다.
type ContainerInfo struct {
	ID        string    // Docker 컨테이너 ID
	HostPort  string    // CDP 연결을 위한 호스트 포트 (예: "49152")
	Status    string    // 컨테이너 상태 (예: "running", "stopped")
	CreatedAt time.Time // 생성 시각
}

// DockerClient는 Docker SDK 호출을 추상화하는 인터페이스이다.
// 테스트에서 mock으로 대체할 수 있도록 필요한 메서드만 정의한다.
type DockerClient interface {
	// ContainerCreate는 새 컨테이너를 생성하고 ID를 반환한다.
	ContainerCreate(ctx context.Context, config *ContainerCreateConfig) (string, error)

	// ContainerStart는 컨테이너를 시작한다.
	ContainerStart(ctx context.Context, containerID string) error

	// ContainerStop은 컨테이너를 정지한다.
	ContainerStop(ctx context.Context, containerID string, timeout *time.Duration) error

	// ContainerRemove는 컨테이너를 삭제한다.
	ContainerRemove(ctx context.Context, containerID string, force bool) error

	// ContainerInspect는 컨테이너 상태 정보를 조회한다.
	ContainerInspect(ctx context.Context, containerID string) (*ContainerInspectResult, error)

	// NetworkCreate는 Docker 네트워크를 생성한다.
	NetworkCreate(ctx context.Context, name string) error

	// NetworkInspect는 네트워크 존재 여부를 확인한다.
	NetworkInspect(ctx context.Context, name string) error

	// ImageInspect는 이미지 존재 여부를 확인한다.
	ImageInspect(ctx context.Context, imageRef string) error

	// ImagePull은 이미지를 풀한다.
	ImagePull(ctx context.Context, imageRef string) (io.ReadCloser, error)

	// Ping은 Docker 데몬 연결을 확인한다.
	Ping(ctx context.Context) error

	// Close는 Docker 클라이언트를 정리한다.
	Close() error
}

// ContainerCreateConfig는 컨테이너 생성에 필요한 설정을 전달한다.
type ContainerCreateConfig struct {
	Image       string
	HostPort    string // 호스트에 바인딩할 포트 (빈 문자열이면 랜덤 할당)
	Network     string
	MemoryLimit int64
	CPUQuota    int64
	PIDLimit    int64
	TmpfsSize   string
	ReadOnly    bool   // 읽기 전용 루트 파일시스템
	User        string // 실행 사용자
}

// ContainerInspectResult는 컨테이너 조회 결과를 담는다.
type ContainerInspectResult struct {
	ID       string
	Status   string // "running", "exited" 등
	HostPort string // 매핑된 호스트 포트
}

// ContainerManager는 Docker 컨테이너의 생명주기를 관리한다.
type ContainerManager struct {
	client  DockerClient
	config  ContainerConfig
	mu      sync.Mutex
}

// NewContainerManager는 DockerClient와 설정으로 ContainerManager를 생성한다.
func NewContainerManager(client DockerClient, cfg ContainerConfig) (*ContainerManager, error) {
	if client == nil {
		return nil, fmt.Errorf("docker client는 nil일 수 없습니다")
	}

	// Docker 데몬 연결 확인
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx); err != nil {
		return nil, fmt.Errorf("docker 데몬 연결 실패: %w", err)
	}

	return &ContainerManager{
		client: client,
		config: cfg,
	}, nil
}

// Create는 새로운 샌드박스 컨테이너를 생성하고 시작한다.
// 리소스 제한, 네트워크 격리, 읽기 전용 파일시스템을 적용한다.
func (cm *ContainerManager) Create(ctx context.Context) (*ContainerInfo, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	createCfg := &ContainerCreateConfig{
		Image:       cm.config.Image,
		Network:     cm.config.Network,
		MemoryLimit: cm.config.MemoryLimit,
		CPUQuota:    cm.config.CPUQuota,
		PIDLimit:    cm.config.PIDLimit,
		TmpfsSize:   cm.config.TmpfsSize,
		ReadOnly:    true,
		User:        "sandbox",
	}

	// 컨테이너 생성
	containerID, err := cm.client.ContainerCreate(ctx, createCfg)
	if err != nil {
		return nil, fmt.Errorf("컨테이너 생성 실패: %w", err)
	}

	// 컨테이너 시작
	if err := cm.client.ContainerStart(ctx, containerID); err != nil {
		// 시작 실패 시 생성된 컨테이너 정리
		_ = cm.client.ContainerRemove(ctx, containerID, true)
		return nil, fmt.Errorf("컨테이너 시작 실패: %w", err)
	}

	// 컨테이너 정보 조회 (매핑된 포트 확인)
	inspect, err := cm.client.ContainerInspect(ctx, containerID)
	if err != nil {
		_ = cm.client.ContainerStop(ctx, containerID, nil)
		_ = cm.client.ContainerRemove(ctx, containerID, true)
		return nil, fmt.Errorf("컨테이너 정보 조회 실패: %w", err)
	}

	info := &ContainerInfo{
		ID:        containerID,
		HostPort:  inspect.HostPort,
		Status:    inspect.Status,
		CreatedAt: time.Now(),
	}

	log.Printf("[computer-use] 컨테이너 생성 완료: id=%s, port=%s", containerID[:12], info.HostPort)
	return info, nil
}

// Remove는 컨테이너를 정지하고 삭제한다.
func (cm *ContainerManager) Remove(ctx context.Context, containerID string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if containerID == "" {
		return fmt.Errorf("컨테이너 ID가 비어있습니다")
	}

	// 정지 (타임아웃 10초)
	timeout := 10 * time.Second
	if err := cm.client.ContainerStop(ctx, containerID, &timeout); err != nil {
		log.Printf("[computer-use] 컨테이너 정지 실패 (강제 삭제 시도): id=%s, err=%v", containerID[:min(12, len(containerID))], err)
	}

	// 삭제
	if err := cm.client.ContainerRemove(ctx, containerID, true); err != nil {
		return fmt.Errorf("컨테이너 삭제 실패: %w", err)
	}

	log.Printf("[computer-use] 컨테이너 삭제 완료: id=%s", containerID[:min(12, len(containerID))])
	return nil
}

// HealthCheck는 컨테이너의 CDP 엔드포인트 접근 가능 여부를 확인한다.
func (cm *ContainerManager) HealthCheck(ctx context.Context, containerID string) error {
	// 컨테이너 상태 확인
	inspect, err := cm.client.ContainerInspect(ctx, containerID)
	if err != nil {
		return fmt.Errorf("컨테이너 상태 조회 실패: %w", err)
	}

	if inspect.Status != "running" {
		return fmt.Errorf("컨테이너가 실행 중이 아닙니다: status=%s", inspect.Status)
	}

	// CDP 엔드포인트 접근 확인
	cdpURL := fmt.Sprintf("http://127.0.0.1:%s/json/version", inspect.HostPort)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cdpURL, nil)
	if err != nil {
		return fmt.Errorf("CDP 확인 요청 생성 실패: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("CDP 엔드포인트 접근 실패: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("CDP 엔드포인트 비정상 응답: status=%d", resp.StatusCode)
	}

	return nil
}

// EnsureNetwork는 Docker 네트워크가 없으면 생성한다.
func (cm *ContainerManager) EnsureNetwork(ctx context.Context) error {
	// 네트워크 존재 여부 확인
	if err := cm.client.NetworkInspect(ctx, cm.config.Network); err == nil {
		return nil // 이미 존재
	}

	// 네트워크 생성
	if err := cm.client.NetworkCreate(ctx, cm.config.Network); err != nil {
		return fmt.Errorf("docker 네트워크 생성 실패: %w", err)
	}

	log.Printf("[computer-use] docker 네트워크 생성 완료: %s", cm.config.Network)
	return nil
}

// EnsureImage는 Docker 이미지가 로컬에 없으면 풀한다.
func (cm *ContainerManager) EnsureImage(ctx context.Context) error {
	// 이미지 존재 여부 확인
	if err := cm.client.ImageInspect(ctx, cm.config.Image); err == nil {
		return nil // 이미 존재
	}

	// 이미지 풀
	log.Printf("[computer-use] 이미지 풀 시작: %s", cm.config.Image)
	reader, err := cm.client.ImagePull(ctx, cm.config.Image)
	if err != nil {
		return fmt.Errorf("이미지 풀 실패: %w", err)
	}
	defer reader.Close()

	// 풀 완료까지 읽기 (진행 상태 소비)
	if _, err := io.Copy(io.Discard, reader); err != nil {
		return fmt.Errorf("이미지 풀 스트림 읽기 실패: %w", err)
	}

	log.Printf("[computer-use] 이미지 풀 완료: %s", cm.config.Image)
	return nil
}

// min은 두 정수 중 작은 값을 반환한다.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
