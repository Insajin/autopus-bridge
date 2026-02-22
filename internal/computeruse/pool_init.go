package computeruse

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ComputerUseConfigInput은 외부 설정(config.ComputerUseConfig)으로부터 매핑되는 입력 구조체이다.
// cmd/connect.go 등 호출측에서 config 값을 이 구조체에 채워 InitContainerPool에 전달한다.
type ComputerUseConfigInput struct {
	MaxContainers   int    // 최대 동시 컨테이너 수
	WarmPoolSize    int    // 웜 풀 크기
	Image           string // Docker 이미지 이름
	ContainerMemory string // 메모리 제한 (예: "512m", "1g")
	ContainerCPU    string // CPU 할당량 (예: "1.0", "0.5")
	IdleTimeout     string // 유휴 타임아웃 (예: "5m", "30s")
	Network         string // Docker 네트워크 이름
}

// InitContainerPool은 설정으로부터 컨테이너 풀을 초기화한다.
// Docker 데몬 연결 확인, 네트워크/이미지 준비, 풀 생성을 모두 수행한다.
// Docker가 사용 불가능하면 에러를 반환한다.
func InitContainerPool(ctx context.Context, cuCfg ComputerUseConfigInput) (*ContainerPool, error) {
	// 1단계: CLIDockerClient 생성
	client := NewCLIDockerClient("")

	// 2단계: ContainerConfig 구성
	defaults := DefaultContainerConfig()
	containerCfg := ContainerConfig{
		Image:        defaults.Image,
		Network:      defaults.Network,
		MemoryLimit:  defaults.MemoryLimit,
		CPUQuota:     defaults.CPUQuota,
		PIDLimit:     defaults.PIDLimit,
		TmpfsSize:    defaults.TmpfsSize,
		StartTimeout: defaults.StartTimeout,
	}

	if cuCfg.Image != "" {
		containerCfg.Image = cuCfg.Image
	}
	if cuCfg.Network != "" {
		containerCfg.Network = cuCfg.Network
	}
	if cuCfg.ContainerMemory != "" {
		if mem := parseMemory(cuCfg.ContainerMemory); mem > 0 {
			containerCfg.MemoryLimit = mem
		}
	}
	if cuCfg.ContainerCPU != "" {
		if cpu := parseCPU(cuCfg.ContainerCPU); cpu > 0 {
			containerCfg.CPUQuota = cpu
		}
	}

	// 3단계: ContainerManager 생성 (내부에서 Docker 데몬 Ping 수행)
	manager, err := NewContainerManager(client, containerCfg)
	if err != nil {
		return nil, fmt.Errorf("컨테이너 매니저 생성 실패: %w", err)
	}

	// 4단계: 네트워크 준비
	if err := manager.EnsureNetwork(ctx); err != nil {
		return nil, fmt.Errorf("Docker 네트워크 준비 실패: %w", err)
	}

	// 5단계: 이미지 준비
	if err := manager.EnsureImage(ctx); err != nil {
		return nil, fmt.Errorf("Docker 이미지 준비 실패: %w", err)
	}

	// 6단계: PoolConfig 구성
	poolDefaults := DefaultPoolConfig()
	poolCfg := PoolConfig{
		MaxContainers: poolDefaults.MaxContainers,
		WarmPoolSize:  poolDefaults.WarmPoolSize,
		IdleTimeout:   poolDefaults.IdleTimeout,
	}

	if cuCfg.MaxContainers > 0 {
		poolCfg.MaxContainers = cuCfg.MaxContainers
	}
	if cuCfg.WarmPoolSize > 0 {
		poolCfg.WarmPoolSize = cuCfg.WarmPoolSize
	}
	if cuCfg.IdleTimeout != "" {
		if timeout := parseTimeout(cuCfg.IdleTimeout); timeout > 0 {
			poolCfg.IdleTimeout = timeout
		}
	}

	// 7단계: ContainerPool 생성
	pool := NewContainerPool(manager, poolCfg)
	return pool, nil
}

// parseMemory는 메모리 문자열을 바이트 단위 정수로 변환한다.
// 지원 형식: "512m" (메가바이트), "1g" (기가바이트), "256m" 등.
// 파싱 실패 시 0을 반환한다.
func parseMemory(s string) int64 {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return 0
	}

	// 마지막 문자로 단위 판별
	unit := s[len(s)-1]
	numStr := s[:len(s)-1]

	val, err := strconv.ParseFloat(numStr, 64)
	if err != nil || val <= 0 {
		return 0
	}

	switch unit {
	case 'm':
		return int64(val * 1024 * 1024)
	case 'g':
		return int64(val * 1024 * 1024 * 1024)
	default:
		return 0
	}
}

// parseCPU는 CPU 문자열을 Docker CPUQuota 값으로 변환한다.
// "1.0"은 100000 (1 CPU), "0.5"는 50000 (0.5 CPU)에 해당한다.
// 파싱 실패 시 0을 반환한다.
func parseCPU(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}

	val, err := strconv.ParseFloat(s, 64)
	if err != nil || val <= 0 {
		return 0
	}

	return int64(val * 100000)
}

// parseTimeout은 타임아웃 문자열을 time.Duration으로 변환한다.
// Go 표준 time.ParseDuration 형식을 지원한다 (예: "5m", "30s", "1h").
// 파싱 실패 시 0을 반환한다.
func parseTimeout(s string) time.Duration {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}

	d, err := time.ParseDuration(s)
	if err != nil || d <= 0 {
		return 0
	}
	return d
}
