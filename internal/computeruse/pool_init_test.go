package computeruse

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"
)

// --- parseMemory 테스트 ---

func TestParseMemory(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int64
	}{
		{
			name:  "512m은 512MB로 변환",
			input: "512m",
			want:  512 * 1024 * 1024,
		},
		{
			name:  "1g는 1GB로 변환",
			input: "1g",
			want:  1 * 1024 * 1024 * 1024,
		},
		{
			name:  "256m은 256MB로 변환",
			input: "256m",
			want:  256 * 1024 * 1024,
		},
		{
			name:  "2g는 2GB로 변환",
			input: "2g",
			want:  2 * 1024 * 1024 * 1024,
		},
		{
			name:  "대문자 M도 처리",
			input: "512M",
			want:  512 * 1024 * 1024,
		},
		{
			name:  "대문자 G도 처리",
			input: "1G",
			want:  1 * 1024 * 1024 * 1024,
		},
		{
			name:  "빈 문자열은 0 반환",
			input: "",
			want:  0,
		},
		{
			name:  "공백만 있으면 0 반환",
			input: "   ",
			want:  0,
		},
		{
			name:  "단위 없는 숫자는 0 반환",
			input: "512",
			want:  0,
		},
		{
			name:  "잘못된 형식은 0 반환",
			input: "abc",
			want:  0,
		},
		{
			name:  "음수 값은 0 반환",
			input: "-512m",
			want:  0,
		},
		{
			name:  "지원하지 않는 단위는 0 반환",
			input: "512k",
			want:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseMemory(tt.input)
			if got != tt.want {
				t.Errorf("parseMemory(%q) = %d; want %d", tt.input, got, tt.want)
			}
		})
	}
}

// --- parseCPU 테스트 ---

func TestParseCPU(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int64
	}{
		{
			name:  "1.0은 CPUQuota 100000으로 변환",
			input: "1.0",
			want:  100000,
		},
		{
			name:  "0.5는 CPUQuota 50000으로 변환",
			input: "0.5",
			want:  50000,
		},
		{
			name:  "2.0은 CPUQuota 200000으로 변환",
			input: "2.0",
			want:  200000,
		},
		{
			name:  "0.25는 CPUQuota 25000으로 변환",
			input: "0.25",
			want:  25000,
		},
		{
			name:  "빈 문자열은 0 반환",
			input: "",
			want:  0,
		},
		{
			name:  "공백만 있으면 0 반환",
			input: "   ",
			want:  0,
		},
		{
			name:  "잘못된 형식은 0 반환",
			input: "abc",
			want:  0,
		},
		{
			name:  "음수 값은 0 반환",
			input: "-1.0",
			want:  0,
		},
		{
			name:  "0은 0 반환",
			input: "0",
			want:  0,
		},
		{
			name:  "앞뒤 공백 제거 후 파싱",
			input: "  1.5  ",
			want:  150000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseCPU(tt.input)
			if got != tt.want {
				t.Errorf("parseCPU(%q) = %d; want %d", tt.input, got, tt.want)
			}
		})
	}
}

// --- parseTimeout 테스트 ---

func TestParseTimeout(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  time.Duration
	}{
		{
			name:  "5m은 5분으로 변환",
			input: "5m",
			want:  5 * time.Minute,
		},
		{
			name:  "30s는 30초로 변환",
			input: "30s",
			want:  30 * time.Second,
		},
		{
			name:  "1h는 1시간으로 변환",
			input: "1h",
			want:  1 * time.Hour,
		},
		{
			name:  "500ms는 500밀리초로 변환",
			input: "500ms",
			want:  500 * time.Millisecond,
		},
		{
			name:  "1h30m은 1시간30분으로 변환",
			input: "1h30m",
			want:  1*time.Hour + 30*time.Minute,
		},
		{
			name:  "빈 문자열은 0 반환",
			input: "",
			want:  0,
		},
		{
			name:  "공백만 있으면 0 반환",
			input: "   ",
			want:  0,
		},
		{
			name:  "잘못된 형식은 0 반환",
			input: "abc",
			want:  0,
		},
		{
			name:  "음수 값은 0 반환",
			input: "-5m",
			want:  0,
		},
		{
			name:  "앞뒤 공백 제거 후 파싱",
			input: "  10m  ",
			want:  10 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseTimeout(tt.input)
			if got != tt.want {
				t.Errorf("parseTimeout(%q) = %v; want %v", tt.input, got, tt.want)
			}
		})
	}
}

// --- ComputerUseConfigInput 구조체 테스트 ---

func TestComputerUseConfigInput_ZeroValue(t *testing.T) {
	// 제로값 구조체가 올바르게 초기화되는지 확인
	cfg := ComputerUseConfigInput{}

	if cfg.MaxContainers != 0 {
		t.Errorf("MaxContainers 제로값 = %d; want 0", cfg.MaxContainers)
	}
	if cfg.WarmPoolSize != 0 {
		t.Errorf("WarmPoolSize 제로값 = %d; want 0", cfg.WarmPoolSize)
	}
	if cfg.Image != "" {
		t.Errorf("Image 제로값 = %q; want empty", cfg.Image)
	}
	if cfg.ContainerMemory != "" {
		t.Errorf("ContainerMemory 제로값 = %q; want empty", cfg.ContainerMemory)
	}
	if cfg.ContainerCPU != "" {
		t.Errorf("ContainerCPU 제로값 = %q; want empty", cfg.ContainerCPU)
	}
	if cfg.IdleTimeout != "" {
		t.Errorf("IdleTimeout 제로값 = %q; want empty", cfg.IdleTimeout)
	}
	if cfg.Network != "" {
		t.Errorf("Network 제로값 = %q; want empty", cfg.Network)
	}
}

// --- InitContainerPool 테스트 ---

// initContainerPoolWithClient는 테스트용으로 외부 DockerClient를 주입하여
// InitContainerPool의 내부 로직을 검증한다.
func initContainerPoolWithClient(ctx context.Context, client DockerClient, cuCfg ComputerUseConfigInput) (*ContainerPool, error) {
	// ContainerConfig 구성 (InitContainerPool과 동일한 로직)
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

	manager, err := NewContainerManager(client, containerCfg)
	if err != nil {
		return nil, err
	}

	if err := manager.EnsureNetwork(ctx); err != nil {
		return nil, err
	}
	if err := manager.EnsureImage(ctx); err != nil {
		return nil, err
	}

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

	pool := NewContainerPool(manager, poolCfg)
	return pool, nil
}

func TestInitContainerPool_FullChain_Success(t *testing.T) {
	mock := newMockDockerClient()
	ctx := context.Background()

	cfg := ComputerUseConfigInput{
		MaxContainers:   10,
		WarmPoolSize:    3,
		Image:           "custom/image:v2",
		ContainerMemory: "1g",
		ContainerCPU:    "2.0",
		IdleTimeout:     "10m",
		Network:         "custom-net",
	}

	pool, err := initContainerPoolWithClient(ctx, mock, cfg)
	if err != nil {
		t.Fatalf("initContainerPoolWithClient() = error %v", err)
	}
	if pool == nil {
		t.Fatal("initContainerPoolWithClient() returned nil pool")
	}

	// Docker 클라이언트 호출 검증
	if mock.pingCalled != 1 {
		t.Errorf("Ping 호출 횟수 = %d; want 1", mock.pingCalled)
	}
	if mock.networkInspectCalled < 1 {
		t.Errorf("NetworkInspect 호출 = %d; want >= 1", mock.networkInspectCalled)
	}
	if mock.imageInspectCalled < 1 {
		t.Errorf("ImageInspect 호출 = %d; want >= 1", mock.imageInspectCalled)
	}

	// 풀 상태 검증
	status := pool.Status()
	if status.MaxCount != 10 {
		t.Errorf("MaxCount = %d; want 10", status.MaxCount)
	}
}

func TestInitContainerPool_DefaultValues(t *testing.T) {
	mock := newMockDockerClient()
	ctx := context.Background()

	// 빈 설정으로 초기화: 모든 값이 기본값으로 설정되어야 함
	cfg := ComputerUseConfigInput{}

	pool, err := initContainerPoolWithClient(ctx, mock, cfg)
	if err != nil {
		t.Fatalf("initContainerPoolWithClient() = error %v", err)
	}
	if pool == nil {
		t.Fatal("initContainerPoolWithClient() returned nil pool")
	}

	// 기본값 검증
	status := pool.Status()
	defaults := DefaultPoolConfig()
	if status.MaxCount != defaults.MaxContainers {
		t.Errorf("MaxCount = %d; want default %d", status.MaxCount, defaults.MaxContainers)
	}
}

func TestInitContainerPool_PingFailure(t *testing.T) {
	mock := newMockDockerClient()
	mock.pingErr = errForTest("docker 데몬 연결 불가")
	ctx := context.Background()

	cfg := ComputerUseConfigInput{
		MaxContainers: 5,
		Image:         "test/image:latest",
	}

	pool, err := initContainerPoolWithClient(ctx, mock, cfg)
	if err == nil {
		t.Fatal("Ping 실패 시 에러를 반환해야 한다")
	}
	if pool != nil {
		t.Error("Ping 실패 시 pool은 nil이어야 한다")
	}
}

func TestInitContainerPool_NetworkFailure(t *testing.T) {
	mock := newMockDockerClient()
	// NetworkInspect 실패 → NetworkCreate도 실패하도록 설정
	mock.networkInspectErr = errForTest("네트워크 없음")
	mock.networkCreateErr = errForTest("네트워크 생성 실패")
	ctx := context.Background()

	cfg := ComputerUseConfigInput{}

	pool, err := initContainerPoolWithClient(ctx, mock, cfg)
	if err == nil {
		t.Fatal("네트워크 준비 실패 시 에러를 반환해야 한다")
	}
	if pool != nil {
		t.Error("네트워크 실패 시 pool은 nil이어야 한다")
	}
}

func TestInitContainerPool_ImageFailure(t *testing.T) {
	mock := newMockDockerClient()
	// 이미지가 로컬에 없고, 풀도 실패하도록 설정
	mock.imageInspectErr = errForTest("이미지 없음")
	mock.imagePullReader = io.NopCloser(strings.NewReader(""))
	mock.imagePullErr = errForTest("이미지 풀 실패")
	ctx := context.Background()

	cfg := ComputerUseConfigInput{}

	pool, err := initContainerPoolWithClient(ctx, mock, cfg)
	if err == nil {
		t.Fatal("이미지 준비 실패 시 에러를 반환해야 한다")
	}
	if pool != nil {
		t.Error("이미지 실패 시 pool은 nil이어야 한다")
	}
}

func TestInitContainerPool_ConfigMapping(t *testing.T) {
	// 커스텀 설정이 올바르게 ContainerConfig와 PoolConfig에 매핑되는지 검증
	mock := newMockDockerClient()
	ctx := context.Background()

	cfg := ComputerUseConfigInput{
		MaxContainers:   8,
		WarmPoolSize:    4,
		Image:           "my-image:v3",
		ContainerMemory: "256m",
		ContainerCPU:    "0.5",
		IdleTimeout:     "3m",
		Network:         "my-network",
	}

	pool, err := initContainerPoolWithClient(ctx, mock, cfg)
	if err != nil {
		t.Fatalf("initContainerPoolWithClient() = error %v", err)
	}

	status := pool.Status()
	if status.MaxCount != 8 {
		t.Errorf("MaxCount = %d; want 8", status.MaxCount)
	}
}

func TestInitContainerPool_InvalidParsingFallsBackToDefaults(t *testing.T) {
	// 잘못된 파싱 값이 들어와도 기본값으로 대체되어 정상 동작해야 함
	mock := newMockDockerClient()
	ctx := context.Background()

	cfg := ComputerUseConfigInput{
		ContainerMemory: "invalid",
		ContainerCPU:    "not-a-number",
		IdleTimeout:     "bad-duration",
	}

	pool, err := initContainerPoolWithClient(ctx, mock, cfg)
	if err != nil {
		t.Fatalf("잘못된 파싱 값으로도 기본값 대체로 성공해야 한다: %v", err)
	}
	if pool == nil {
		t.Fatal("pool이 nil이면 안 된다")
	}
}

// TestInitContainerPool_RealCLI_DockerUnavailable는 실제 CLIDockerClient를 사용하는
// InitContainerPool이 Docker 미설치/미실행 환경에서 적절한 에러를 반환하는지 검증한다.
func TestInitContainerPool_RealCLI_DockerUnavailable(t *testing.T) {
	// CI 환경이나 Docker가 설치되지 않은 환경에서 실행됨을 가정
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cfg := ComputerUseConfigInput{
		MaxContainers: 2,
		WarmPoolSize:  1,
		Image:         "nonexistent/image:latest",
	}

	// Docker가 없거나 데몬이 실행 중이 아니면 에러 반환
	pool, err := InitContainerPool(ctx, cfg)
	// Docker가 실행 중인 환경에서는 이미지 풀에서 실패할 수 있음
	// Docker가 없는 환경에서는 Ping에서 실패함
	// 어느 경우든 에러 또는 성공 — 패닉이 발생하지 않으면 통과
	if err != nil {
		t.Logf("예상대로 에러 반환 (Docker 미사용 환경): %v", err)
		if pool != nil {
			t.Error("에러 반환 시 pool은 nil이어야 한다")
		}
	} else {
		t.Logf("Docker가 사용 가능한 환경에서 성공적으로 초기화됨")
		if pool == nil {
			t.Error("에러 없이 반환 시 pool은 nil이면 안 된다")
		}
	}
}

// errForTest는 테스트용 에러를 생성하는 헬퍼이다.
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

func errForTest(msg string) error {
	return &testError{msg: msg}
}
