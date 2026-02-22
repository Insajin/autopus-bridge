package computeruse

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// --- Mock DockerClient ---

// mockDockerClient는 테스트용 DockerClient 구현체이다.
type mockDockerClient struct {
	// 호출 기록
	pingCalled           int
	createCalled         int
	startCalled          int
	stopCalled           int
	removeCalled         int
	inspectCalled        int
	networkCreateCalled  int
	networkInspectCalled int
	imageInspectCalled   int
	imagePullCalled      int

	// 반환값 설정
	pingErr           error
	createID          string
	createErr         error
	startErr          error
	stopErr           error
	removeErr         error
	inspectResult     *ContainerInspectResult
	inspectErr        error
	networkCreateErr  error
	networkInspectErr error
	imageInspectErr   error
	imagePullReader   io.ReadCloser
	imagePullErr      error

	// 마지막 호출 인자 기록
	lastCreateConfig *ContainerCreateConfig
	lastStopTimeout  *time.Duration
	lastRemoveForce  bool
}

func newMockDockerClient() *mockDockerClient {
	return &mockDockerClient{
		createID: "abc123def456",
		inspectResult: &ContainerInspectResult{
			ID:       "abc123def456",
			Status:   "running",
			HostPort: "49152",
		},
		imagePullReader: io.NopCloser(strings.NewReader("")),
	}
}

func (m *mockDockerClient) Ping(ctx context.Context) error {
	m.pingCalled++
	return m.pingErr
}

func (m *mockDockerClient) ContainerCreate(ctx context.Context, config *ContainerCreateConfig) (string, error) {
	m.createCalled++
	m.lastCreateConfig = config
	return m.createID, m.createErr
}

func (m *mockDockerClient) ContainerStart(ctx context.Context, containerID string) error {
	m.startCalled++
	return m.startErr
}

func (m *mockDockerClient) ContainerStop(ctx context.Context, containerID string, timeout *time.Duration) error {
	m.stopCalled++
	m.lastStopTimeout = timeout
	return m.stopErr
}

func (m *mockDockerClient) ContainerRemove(ctx context.Context, containerID string, force bool) error {
	m.removeCalled++
	m.lastRemoveForce = force
	return m.removeErr
}

func (m *mockDockerClient) ContainerInspect(ctx context.Context, containerID string) (*ContainerInspectResult, error) {
	m.inspectCalled++
	return m.inspectResult, m.inspectErr
}

func (m *mockDockerClient) NetworkCreate(ctx context.Context, name string) error {
	m.networkCreateCalled++
	return m.networkCreateErr
}

func (m *mockDockerClient) NetworkInspect(ctx context.Context, name string) error {
	m.networkInspectCalled++
	return m.networkInspectErr
}

func (m *mockDockerClient) ImageInspect(ctx context.Context, imageRef string) error {
	m.imageInspectCalled++
	return m.imageInspectErr
}

func (m *mockDockerClient) ImagePull(ctx context.Context, imageRef string) (io.ReadCloser, error) {
	m.imagePullCalled++
	return m.imagePullReader, m.imagePullErr
}

func (m *mockDockerClient) Close() error {
	return nil
}

// --- DefaultContainerConfig 테스트 ---

func TestDefaultContainerConfig(t *testing.T) {
	cfg := DefaultContainerConfig()

	if cfg.Image != "autopus/chromium-sandbox:latest" {
		t.Errorf("Image = %q; want %q", cfg.Image, "autopus/chromium-sandbox:latest")
	}
	if cfg.Network != "autopus-sandbox-net" {
		t.Errorf("Network = %q; want %q", cfg.Network, "autopus-sandbox-net")
	}
	if cfg.MemoryLimit != 512*1024*1024 {
		t.Errorf("MemoryLimit = %d; want %d", cfg.MemoryLimit, 512*1024*1024)
	}
	if cfg.CPUQuota != 100000 {
		t.Errorf("CPUQuota = %d; want %d", cfg.CPUQuota, 100000)
	}
	if cfg.PIDLimit != 100 {
		t.Errorf("PIDLimit = %d; want %d", cfg.PIDLimit, 100)
	}
	if cfg.TmpfsSize != "64m" {
		t.Errorf("TmpfsSize = %q; want %q", cfg.TmpfsSize, "64m")
	}
}

// --- NewContainerManager 테스트 ---

func TestNewContainerManager(t *testing.T) {
	mock := newMockDockerClient()
	cfg := DefaultContainerConfig()

	cm, err := NewContainerManager(mock, cfg)
	if err != nil {
		t.Fatalf("NewContainerManager() = error %v; want nil", err)
	}
	if cm == nil {
		t.Fatal("NewContainerManager() returned nil")
	}
	if mock.pingCalled != 1 {
		t.Errorf("Ping 호출 횟수 = %d; want 1", mock.pingCalled)
	}
}

func TestNewContainerManager_NilClient(t *testing.T) {
	cfg := DefaultContainerConfig()

	_, err := NewContainerManager(nil, cfg)
	if err == nil {
		t.Error("NewContainerManager(nil) = nil error; want error")
	}
}

func TestNewContainerManager_PingError(t *testing.T) {
	mock := newMockDockerClient()
	mock.pingErr = fmt.Errorf("connection refused")
	cfg := DefaultContainerConfig()

	_, err := NewContainerManager(mock, cfg)
	if err == nil {
		t.Error("NewContainerManager() with ping error = nil; want error")
	}
	if !strings.Contains(err.Error(), "connection refused") {
		t.Errorf("error = %q; want containing 'connection refused'", err.Error())
	}
}

// --- ContainerManager.Create 테스트 ---

func TestContainerManager_Create(t *testing.T) {
	mock := newMockDockerClient()
	cfg := DefaultContainerConfig()
	cm, _ := NewContainerManager(mock, cfg)

	ctx := context.Background()
	info, err := cm.Create(ctx)
	if err != nil {
		t.Fatalf("Create() = error %v; want nil", err)
	}

	if info.ID != "abc123def456" {
		t.Errorf("info.ID = %q; want %q", info.ID, "abc123def456")
	}
	if info.HostPort != "49152" {
		t.Errorf("info.HostPort = %q; want %q", info.HostPort, "49152")
	}
	if info.Status != "running" {
		t.Errorf("info.Status = %q; want %q", info.Status, "running")
	}
	if info.CreatedAt.IsZero() {
		t.Error("info.CreatedAt is zero; want non-zero")
	}
	if mock.createCalled != 1 {
		t.Errorf("ContainerCreate 호출 횟수 = %d; want 1", mock.createCalled)
	}
	if mock.startCalled != 1 {
		t.Errorf("ContainerStart 호출 횟수 = %d; want 1", mock.startCalled)
	}
}

func TestContainerManager_Create_AppliesConfig(t *testing.T) {
	mock := newMockDockerClient()
	cfg := DefaultContainerConfig()
	cm, _ := NewContainerManager(mock, cfg)

	ctx := context.Background()
	_, _ = cm.Create(ctx)

	// 생성 설정 검증
	cc := mock.lastCreateConfig
	if cc == nil {
		t.Fatal("lastCreateConfig is nil")
	}
	if cc.Image != cfg.Image {
		t.Errorf("Image = %q; want %q", cc.Image, cfg.Image)
	}
	if cc.MemoryLimit != cfg.MemoryLimit {
		t.Errorf("MemoryLimit = %d; want %d", cc.MemoryLimit, cfg.MemoryLimit)
	}
	if cc.CPUQuota != cfg.CPUQuota {
		t.Errorf("CPUQuota = %d; want %d", cc.CPUQuota, cfg.CPUQuota)
	}
	if cc.PIDLimit != cfg.PIDLimit {
		t.Errorf("PIDLimit = %d; want %d", cc.PIDLimit, cfg.PIDLimit)
	}
	if !cc.ReadOnly {
		t.Error("ReadOnly = false; want true")
	}
	if cc.User != "sandbox" {
		t.Errorf("User = %q; want %q", cc.User, "sandbox")
	}
}

func TestContainerManager_Create_CreateError(t *testing.T) {
	mock := newMockDockerClient()
	mock.createErr = fmt.Errorf("image not found")
	cfg := DefaultContainerConfig()
	cm, _ := NewContainerManager(mock, cfg)

	ctx := context.Background()
	_, err := cm.Create(ctx)
	if err == nil {
		t.Error("Create() with create error = nil; want error")
	}
	if !strings.Contains(err.Error(), "image not found") {
		t.Errorf("error = %q; want containing 'image not found'", err.Error())
	}
}

func TestContainerManager_Create_StartError_Cleanup(t *testing.T) {
	mock := newMockDockerClient()
	mock.startErr = fmt.Errorf("port conflict")
	cfg := DefaultContainerConfig()
	cm, _ := NewContainerManager(mock, cfg)

	ctx := context.Background()
	_, err := cm.Create(ctx)
	if err == nil {
		t.Error("Create() with start error = nil; want error")
	}
	// 시작 실패 시 생성된 컨테이너 정리 확인
	if mock.removeCalled != 1 {
		t.Errorf("ContainerRemove 호출 횟수 = %d; want 1 (cleanup)", mock.removeCalled)
	}
}

func TestContainerManager_Create_InspectError_Cleanup(t *testing.T) {
	mock := newMockDockerClient()
	mock.inspectErr = fmt.Errorf("inspect failed")
	cfg := DefaultContainerConfig()
	cm, _ := NewContainerManager(mock, cfg)

	ctx := context.Background()
	_, err := cm.Create(ctx)
	if err == nil {
		t.Error("Create() with inspect error = nil; want error")
	}
	// inspect 실패 시 정지+삭제 호출 확인
	if mock.stopCalled != 1 {
		t.Errorf("ContainerStop 호출 횟수 = %d; want 1 (cleanup)", mock.stopCalled)
	}
	if mock.removeCalled != 1 {
		t.Errorf("ContainerRemove 호출 횟수 = %d; want 1 (cleanup)", mock.removeCalled)
	}
}

// --- ContainerManager.Remove 테스트 ---

func TestContainerManager_Remove(t *testing.T) {
	mock := newMockDockerClient()
	cfg := DefaultContainerConfig()
	cm, _ := NewContainerManager(mock, cfg)

	ctx := context.Background()
	err := cm.Remove(ctx, "abc123def456")
	if err != nil {
		t.Fatalf("Remove() = error %v; want nil", err)
	}
	if mock.stopCalled != 1 {
		t.Errorf("ContainerStop 호출 횟수 = %d; want 1", mock.stopCalled)
	}
	if mock.removeCalled != 1 {
		t.Errorf("ContainerRemove 호출 횟수 = %d; want 1", mock.removeCalled)
	}
}

func TestContainerManager_Remove_EmptyID(t *testing.T) {
	mock := newMockDockerClient()
	cfg := DefaultContainerConfig()
	cm, _ := NewContainerManager(mock, cfg)

	ctx := context.Background()
	err := cm.Remove(ctx, "")
	if err == nil {
		t.Error("Remove('') = nil; want error")
	}
}

func TestContainerManager_Remove_StopError_StillRemoves(t *testing.T) {
	mock := newMockDockerClient()
	mock.stopErr = fmt.Errorf("already stopped")
	cfg := DefaultContainerConfig()
	cm, _ := NewContainerManager(mock, cfg)

	ctx := context.Background()
	err := cm.Remove(ctx, "abc123def456")
	// 정지 실패해도 삭제는 시도해야 한다
	if err != nil {
		t.Fatalf("Remove() = error %v; want nil (stop error ignored)", err)
	}
	if mock.removeCalled != 1 {
		t.Errorf("ContainerRemove 호출 횟수 = %d; want 1 (despite stop error)", mock.removeCalled)
	}
}

func TestContainerManager_Remove_RemoveError(t *testing.T) {
	mock := newMockDockerClient()
	mock.removeErr = fmt.Errorf("permission denied")
	cfg := DefaultContainerConfig()
	cm, _ := NewContainerManager(mock, cfg)

	ctx := context.Background()
	err := cm.Remove(ctx, "abc123def456")
	if err == nil {
		t.Error("Remove() with remove error = nil; want error")
	}
	if !strings.Contains(err.Error(), "permission denied") {
		t.Errorf("error = %q; want containing 'permission denied'", err.Error())
	}
}

// --- ContainerManager.EnsureNetwork 테스트 ---

func TestContainerManager_EnsureNetwork_AlreadyExists(t *testing.T) {
	mock := newMockDockerClient()
	cfg := DefaultContainerConfig()
	cm, _ := NewContainerManager(mock, cfg)

	ctx := context.Background()
	err := cm.EnsureNetwork(ctx)
	if err != nil {
		t.Fatalf("EnsureNetwork() = error %v; want nil", err)
	}
	// 네트워크가 이미 존재하므로 생성 호출 없음
	if mock.networkCreateCalled != 0 {
		t.Errorf("NetworkCreate 호출 횟수 = %d; want 0 (already exists)", mock.networkCreateCalled)
	}
}

func TestContainerManager_EnsureNetwork_Creates(t *testing.T) {
	mock := newMockDockerClient()
	mock.networkInspectErr = fmt.Errorf("network not found")
	cfg := DefaultContainerConfig()
	cm, _ := NewContainerManager(mock, cfg)

	ctx := context.Background()
	err := cm.EnsureNetwork(ctx)
	if err != nil {
		t.Fatalf("EnsureNetwork() = error %v; want nil", err)
	}
	if mock.networkCreateCalled != 1 {
		t.Errorf("NetworkCreate 호출 횟수 = %d; want 1", mock.networkCreateCalled)
	}
}

func TestContainerManager_EnsureNetwork_CreateError(t *testing.T) {
	mock := newMockDockerClient()
	mock.networkInspectErr = fmt.Errorf("not found")
	mock.networkCreateErr = fmt.Errorf("permission denied")
	cfg := DefaultContainerConfig()
	cm, _ := NewContainerManager(mock, cfg)

	ctx := context.Background()
	err := cm.EnsureNetwork(ctx)
	if err == nil {
		t.Error("EnsureNetwork() with create error = nil; want error")
	}
}

// --- ContainerManager.EnsureImage 테스트 ---

func TestContainerManager_EnsureImage_AlreadyExists(t *testing.T) {
	mock := newMockDockerClient()
	cfg := DefaultContainerConfig()
	cm, _ := NewContainerManager(mock, cfg)

	ctx := context.Background()
	err := cm.EnsureImage(ctx)
	if err != nil {
		t.Fatalf("EnsureImage() = error %v; want nil", err)
	}
	if mock.imagePullCalled != 0 {
		t.Errorf("ImagePull 호출 횟수 = %d; want 0 (already exists)", mock.imagePullCalled)
	}
}

func TestContainerManager_EnsureImage_Pulls(t *testing.T) {
	mock := newMockDockerClient()
	mock.imageInspectErr = fmt.Errorf("image not found")
	cfg := DefaultContainerConfig()
	cm, _ := NewContainerManager(mock, cfg)

	ctx := context.Background()
	err := cm.EnsureImage(ctx)
	if err != nil {
		t.Fatalf("EnsureImage() = error %v; want nil", err)
	}
	if mock.imagePullCalled != 1 {
		t.Errorf("ImagePull 호출 횟수 = %d; want 1", mock.imagePullCalled)
	}
}

func TestContainerManager_EnsureImage_PullError(t *testing.T) {
	mock := newMockDockerClient()
	mock.imageInspectErr = fmt.Errorf("not found")
	mock.imagePullErr = fmt.Errorf("unauthorized")
	cfg := DefaultContainerConfig()
	cm, _ := NewContainerManager(mock, cfg)

	ctx := context.Background()
	err := cm.EnsureImage(ctx)
	if err == nil {
		t.Error("EnsureImage() with pull error = nil; want error")
	}
}

// --- ContainerManager.HealthCheck 테스트 ---

func TestContainerManager_HealthCheck_NotRunning(t *testing.T) {
	mock := newMockDockerClient()
	mock.inspectResult = &ContainerInspectResult{
		ID:       "abc123",
		Status:   "exited",
		HostPort: "49152",
	}
	cfg := DefaultContainerConfig()
	cm, _ := NewContainerManager(mock, cfg)

	ctx := context.Background()
	err := cm.HealthCheck(ctx, "abc123")
	if err == nil {
		t.Error("HealthCheck() for exited container = nil; want error")
	}
	if !strings.Contains(err.Error(), "실행 중이 아닙니다") {
		t.Errorf("error = %q; want containing '실행 중이 아닙니다'", err.Error())
	}
}

func TestContainerManager_HealthCheck_InspectError(t *testing.T) {
	mock := newMockDockerClient()
	mock.inspectErr = fmt.Errorf("container not found")
	cfg := DefaultContainerConfig()
	cm, _ := NewContainerManager(mock, cfg)

	ctx := context.Background()
	err := cm.HealthCheck(ctx, "nonexistent")
	if err == nil {
		t.Error("HealthCheck() with inspect error = nil; want error")
	}
}

func TestContainerManager_HealthCheck_Running_CDPEndpointOK(t *testing.T) {
	// CDP 엔드포인트를 시뮬레이션하는 HTTP 테스트 서버
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/json/version" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"Browser":"Chrome/120.0"}`))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// 테스트 서버의 포트 추출
	serverPort := strings.TrimPrefix(server.URL, "http://127.0.0.1:")

	mock := newMockDockerClient()
	mock.inspectResult = &ContainerInspectResult{
		ID:       "running-container",
		Status:   "running",
		HostPort: serverPort,
	}
	cfg := DefaultContainerConfig()
	cm, _ := NewContainerManager(mock, cfg)

	ctx := context.Background()
	err := cm.HealthCheck(ctx, "running-container")
	if err != nil {
		t.Fatalf("HealthCheck() for running container with CDP = error %v; want nil", err)
	}
}

func TestContainerManager_HealthCheck_Running_CDPEndpointNon200(t *testing.T) {
	// 503 응답을 반환하는 테스트 서버
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	serverPort := strings.TrimPrefix(server.URL, "http://127.0.0.1:")

	mock := newMockDockerClient()
	mock.inspectResult = &ContainerInspectResult{
		ID:       "unhealthy-container",
		Status:   "running",
		HostPort: serverPort,
	}
	cfg := DefaultContainerConfig()
	cm, _ := NewContainerManager(mock, cfg)

	ctx := context.Background()
	err := cm.HealthCheck(ctx, "unhealthy-container")
	if err == nil {
		t.Error("HealthCheck() with non-200 CDP response = nil; want error")
	}
	if !strings.Contains(err.Error(), "비정상 응답") {
		t.Errorf("error = %q; want containing '비정상 응답'", err.Error())
	}
}

func TestContainerManager_EnsureImage_ReadError(t *testing.T) {
	mock := newMockDockerClient()
	mock.imageInspectErr = fmt.Errorf("not found")
	// io.Copy가 실패하도록 에러를 반환하는 Reader 사용
	mock.imagePullReader = io.NopCloser(&errorReader{err: fmt.Errorf("stream interrupted")})
	cfg := DefaultContainerConfig()
	cm, _ := NewContainerManager(mock, cfg)

	ctx := context.Background()
	err := cm.EnsureImage(ctx)
	if err == nil {
		t.Error("EnsureImage() with read error = nil; want error")
	}
	if !strings.Contains(err.Error(), "스트림 읽기 실패") {
		t.Errorf("error = %q; want containing '스트림 읽기 실패'", err.Error())
	}
}

// errorReader는 Read 호출 시 항상 에러를 반환하는 io.Reader이다.
type errorReader struct {
	err error
}

func (r *errorReader) Read(p []byte) (int, error) {
	return 0, r.err
}

func TestContainerManager_HealthCheck_Running_CDPEndpointConnectionRefused(t *testing.T) {
	mock := newMockDockerClient()
	mock.inspectResult = &ContainerInspectResult{
		ID:       "unreachable-container",
		Status:   "running",
		HostPort: "1", // 연결 불가능한 포트
	}
	cfg := DefaultContainerConfig()
	cm, _ := NewContainerManager(mock, cfg)

	ctx := context.Background()
	err := cm.HealthCheck(ctx, "unreachable-container")
	if err == nil {
		t.Error("HealthCheck() with unreachable CDP = nil; want error")
	}
	if !strings.Contains(err.Error(), "CDP 엔드포인트 접근 실패") {
		t.Errorf("error = %q; want containing 'CDP 엔드포인트 접근 실패'", err.Error())
	}
}
