package computeruse

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

// --- mockBrowserBackend: 테스트용 BrowserBackend 구현체 ---

// mockBrowserBackend은 BrowserBackend 인터페이스를 구현하는 테스트 전용 mock이다.
// 호출 횟수 추적과 반환값 설정이 가능하다.
type mockBrowserBackend struct {
	mu sync.Mutex

	// 상태
	active bool

	// 호출 횟수
	launchCalled     int
	closeCalled      int
	isActiveCalled   int
	screenshotCalled int
	navigateCalled   int
	clickCalled      int
	typeCalled       int
	scrollCalled     int

	// 마지막 호출 인자
	lastNavigateURL     string
	lastClickX          float64
	lastClickY          float64
	lastTypeText        string
	lastScrollDirection string
	lastScrollAmount    int

	// 반환값 설정
	launchErr     error
	closeErr      error
	screenshotData []byte
	screenshotErr error
	navigateErr   error
	clickErr      error
	typeErr       error
	scrollErr     error
}

func newMockBrowserBackend() *mockBrowserBackend {
	return &mockBrowserBackend{
		// 기본 스크린샷: 유효한 base64 인코딩 가능한 작은 PNG 헤더 바이트
		screenshotData: []byte("fake-png-screenshot-data"),
	}
}

func (m *mockBrowserBackend) Launch(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.launchCalled++
	if m.launchErr != nil {
		return m.launchErr
	}
	m.active = true
	return nil
}

func (m *mockBrowserBackend) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closeCalled++
	m.active = false
	return m.closeErr
}

func (m *mockBrowserBackend) IsActive() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.isActiveCalled++
	return m.active
}

func (m *mockBrowserBackend) Screenshot(ctx context.Context) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.screenshotCalled++
	if m.screenshotErr != nil {
		return nil, m.screenshotErr
	}
	return m.screenshotData, nil
}

func (m *mockBrowserBackend) Navigate(ctx context.Context, url string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.navigateCalled++
	m.lastNavigateURL = url
	return m.navigateErr
}

func (m *mockBrowserBackend) Click(ctx context.Context, x, y float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clickCalled++
	m.lastClickX = x
	m.lastClickY = y
	return m.clickErr
}

func (m *mockBrowserBackend) Type(ctx context.Context, text string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.typeCalled++
	m.lastTypeText = text
	return m.typeErr
}

func (m *mockBrowserBackend) Scroll(ctx context.Context, direction string, amount int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.scrollCalled++
	m.lastScrollDirection = direction
	m.lastScrollAmount = amount
	return m.scrollErr
}

// --- mockBrowserBackend 컴파일 타임 인터페이스 확인 ---
var _ BrowserBackend = (*mockBrowserBackend)(nil)

// --- ContainerBrowserBackend 생성자 테스트 ---

func TestNewContainerBrowserBackend(t *testing.T) {
	container := &ContainerInfo{
		ID:        "test-container-123456",
		HostPort:  "49200",
		Status:    "running",
		CreatedAt: time.Now(),
	}

	cb := NewContainerBrowserBackend(container, 1920, 1080)
	if cb == nil {
		t.Fatal("NewContainerBrowserBackend() returned nil")
	}
	if cb.container != container {
		t.Error("container 필드가 올바르게 설정되지 않음")
	}
	if cb.viewportW != 1920 {
		t.Errorf("viewportW = %d; want 1920", cb.viewportW)
	}
	if cb.viewportH != 1080 {
		t.Errorf("viewportH = %d; want 1080", cb.viewportH)
	}
	if cb.active {
		t.Error("active = true; want false (초기 상태)")
	}
}

func TestNewContainerBrowserBackend_ZeroViewport(t *testing.T) {
	container := &ContainerInfo{
		ID:       "test-container-000",
		HostPort: "49300",
	}

	cb := NewContainerBrowserBackend(container, 0, 0)
	if cb == nil {
		t.Fatal("NewContainerBrowserBackend(0, 0) returned nil")
	}
	if cb.viewportW != 0 || cb.viewportH != 0 {
		t.Errorf("viewport = %dx%d; want 0x0", cb.viewportW, cb.viewportH)
	}
}

// --- ContainerBrowserBackend 인터페이스 구현 확인 ---

func TestContainerBrowserBackend_ImplementsBrowserBackend(t *testing.T) {
	container := &ContainerInfo{ID: "test", HostPort: "49200"}
	var backend BrowserBackend = NewContainerBrowserBackend(container, 1280, 720)
	if backend == nil {
		t.Fatal("ContainerBrowserBackend does not implement BrowserBackend")
	}
}

// --- IsActive 테스트 ---

func TestContainerBrowserBackend_IsActive_BeforeLaunch(t *testing.T) {
	container := &ContainerInfo{ID: "test-container", HostPort: "49200"}
	cb := NewContainerBrowserBackend(container, 1280, 720)

	if cb.IsActive() {
		t.Error("IsActive() = true before Launch; want false")
	}
}

func TestContainerBrowserBackend_IsActive_WhenActive(t *testing.T) {
	container := &ContainerInfo{ID: "test-container", HostPort: "49200"}
	cb := NewContainerBrowserBackend(container, 1280, 720)

	// active 플래그를 직접 설정하여 테스트
	cb.mu.Lock()
	cb.active = true
	cb.mu.Unlock()

	if !cb.IsActive() {
		t.Error("IsActive() = false; want true when active flag set")
	}
}

// --- Close 테스트 ---

func TestContainerBrowserBackend_Close_WhenNotActive(t *testing.T) {
	container := &ContainerInfo{ID: "test-container", HostPort: "49200"}
	cb := NewContainerBrowserBackend(container, 1280, 720)

	// 활성화되지 않은 상태에서 Close는 no-op이어야 한다
	err := cb.Close()
	if err != nil {
		t.Errorf("Close() when not active = error %v; want nil", err)
	}
}

func TestContainerBrowserBackend_Close_WhenActive(t *testing.T) {
	container := &ContainerInfo{ID: "test-container", HostPort: "49200"}
	cb := NewContainerBrowserBackend(container, 1280, 720)

	// active 플래그를 직접 설정
	cb.mu.Lock()
	cb.active = true
	cb.mu.Unlock()

	err := cb.Close()
	if err != nil {
		t.Errorf("Close() = error %v; want nil", err)
	}
	if cb.IsActive() {
		t.Error("IsActive() = true after Close; want false")
	}
}

func TestContainerBrowserBackend_Close_Idempotent(t *testing.T) {
	container := &ContainerInfo{ID: "test-container", HostPort: "49200"}
	cb := NewContainerBrowserBackend(container, 1280, 720)

	// 여러 번 Close 호출해도 에러 없어야 한다
	for i := 0; i < 3; i++ {
		if err := cb.Close(); err != nil {
			t.Errorf("Close() call %d = error %v; want nil", i+1, err)
		}
	}
}

func TestContainerBrowserBackend_Close_CleansUpCancelFuncs(t *testing.T) {
	container := &ContainerInfo{ID: "test-container", HostPort: "49200"}
	cb := NewContainerBrowserBackend(container, 1280, 720)

	// cancel 함수 설정 후 Close 호출 시 패닉 없이 정상 동작해야 한다
	ctx, cancel := context.WithCancel(context.Background())
	cb.mu.Lock()
	cb.active = true
	cb.allocCancel = cancel
	cb.taskCancel = func() {} // no-op
	cb.allocCtx = ctx
	cb.mu.Unlock()

	err := cb.Close()
	if err != nil {
		t.Errorf("Close() with cancel funcs = error %v; want nil", err)
	}
}

// --- Launch 에러 테스트 ---

func TestContainerBrowserBackend_Launch_AlreadyLaunched(t *testing.T) {
	container := &ContainerInfo{ID: "test-container", HostPort: "49200"}
	cb := NewContainerBrowserBackend(container, 1280, 720)

	// active를 직접 설정하여 이미 실행 중인 상태 시뮬레이션
	cb.mu.Lock()
	cb.active = true
	cb.mu.Unlock()

	ctx := context.Background()
	err := cb.Launch(ctx)
	if err == nil {
		t.Error("Launch() on already-active backend = nil; want error")
	}
	if !strings.Contains(err.Error(), "already launched") {
		t.Errorf("Launch() error = %q; want containing 'already launched'", err.Error())
	}
}

// --- Screenshot/Navigate/Click/Type/Scroll: "not active" 에러 테스트 ---

func TestContainerBrowserBackend_Screenshot_NotActive(t *testing.T) {
	container := &ContainerInfo{ID: "test-container", HostPort: "49200"}
	cb := NewContainerBrowserBackend(container, 1280, 720)
	ctx := context.Background()

	data, err := cb.Screenshot(ctx)
	if err == nil {
		t.Error("Screenshot() when not active = nil error; want error")
	}
	if data != nil {
		t.Errorf("Screenshot() when not active returned data (len=%d); want nil", len(data))
	}
	if !strings.Contains(err.Error(), "browser is not active") {
		t.Errorf("Screenshot() error = %q; want containing 'browser is not active'", err.Error())
	}
}

func TestContainerBrowserBackend_Navigate_NotActive(t *testing.T) {
	container := &ContainerInfo{ID: "test-container", HostPort: "49200"}
	cb := NewContainerBrowserBackend(container, 1280, 720)
	ctx := context.Background()

	err := cb.Navigate(ctx, "http://example.com")
	if err == nil {
		t.Error("Navigate() when not active = nil error; want error")
	}
	if !strings.Contains(err.Error(), "browser is not active") {
		t.Errorf("Navigate() error = %q; want containing 'browser is not active'", err.Error())
	}
}

func TestContainerBrowserBackend_Click_NotActive(t *testing.T) {
	container := &ContainerInfo{ID: "test-container", HostPort: "49200"}
	cb := NewContainerBrowserBackend(container, 1280, 720)
	ctx := context.Background()

	err := cb.Click(ctx, 100, 200)
	if err == nil {
		t.Error("Click() when not active = nil error; want error")
	}
	if !strings.Contains(err.Error(), "browser is not active") {
		t.Errorf("Click() error = %q; want containing 'browser is not active'", err.Error())
	}
}

func TestContainerBrowserBackend_Type_NotActive(t *testing.T) {
	container := &ContainerInfo{ID: "test-container", HostPort: "49200"}
	cb := NewContainerBrowserBackend(container, 1280, 720)
	ctx := context.Background()

	err := cb.Type(ctx, "hello")
	if err == nil {
		t.Error("Type() when not active = nil error; want error")
	}
	if !strings.Contains(err.Error(), "browser is not active") {
		t.Errorf("Type() error = %q; want containing 'browser is not active'", err.Error())
	}
}

func TestContainerBrowserBackend_Scroll_NotActive(t *testing.T) {
	container := &ContainerInfo{ID: "test-container", HostPort: "49200"}
	cb := NewContainerBrowserBackend(container, 1280, 720)
	ctx := context.Background()

	err := cb.Scroll(ctx, "down", 300)
	if err == nil {
		t.Error("Scroll() when not active = nil error; want error")
	}
	if !strings.Contains(err.Error(), "browser is not active") {
		t.Errorf("Scroll() error = %q; want containing 'browser is not active'", err.Error())
	}
}

// --- ActionExecutor + mockBrowserBackend 통합 테스트 ---

func TestActionExecutor_Execute_Screenshot(t *testing.T) {
	mock := newMockBrowserBackend()
	mock.active = true
	security := NewSecurityValidator()
	executor := NewActionExecutor(mock, security)

	ctx := context.Background()
	screenshot, err := executor.Execute(ctx, "screenshot", nil)
	if err != nil {
		t.Fatalf("Execute(screenshot) = error %v; want nil", err)
	}
	if screenshot == "" {
		t.Error("Execute(screenshot) returned empty string; want base64 screenshot")
	}

	// base64 디코딩 가능 확인
	decoded, decErr := base64.StdEncoding.DecodeString(screenshot)
	if decErr != nil {
		t.Fatalf("screenshot is not valid base64: %v", decErr)
	}
	if len(decoded) == 0 {
		t.Error("decoded screenshot is empty")
	}

	if mock.screenshotCalled != 1 {
		t.Errorf("Screenshot 호출 횟수 = %d; want 1", mock.screenshotCalled)
	}
}

func TestActionExecutor_Execute_Click(t *testing.T) {
	mock := newMockBrowserBackend()
	mock.active = true
	security := NewSecurityValidator()
	executor := NewActionExecutor(mock, security)

	ctx := context.Background()
	params := map[string]interface{}{"x": 150.0, "y": 250.0}
	screenshot, err := executor.Execute(ctx, "click", params)
	if err != nil {
		t.Fatalf("Execute(click) = error %v; want nil", err)
	}
	if screenshot == "" {
		t.Error("Execute(click) returned empty screenshot")
	}
	if mock.clickCalled != 1 {
		t.Errorf("Click 호출 횟수 = %d; want 1", mock.clickCalled)
	}
	if mock.lastClickX != 150.0 || mock.lastClickY != 250.0 {
		t.Errorf("Click 좌표 = (%.0f, %.0f); want (150, 250)", mock.lastClickX, mock.lastClickY)
	}
	// 스크린샷은 액션 후 항상 캡처됨
	if mock.screenshotCalled != 1 {
		t.Errorf("Screenshot 호출 횟수 = %d; want 1", mock.screenshotCalled)
	}
}

func TestActionExecutor_Execute_Click_Error(t *testing.T) {
	mock := newMockBrowserBackend()
	mock.active = true
	mock.clickErr = fmt.Errorf("click failed")
	security := NewSecurityValidator()
	executor := NewActionExecutor(mock, security)

	ctx := context.Background()
	params := map[string]interface{}{"x": 10.0, "y": 20.0}
	_, err := executor.Execute(ctx, "click", params)
	if err == nil {
		t.Error("Execute(click) with error = nil; want error")
	}
}

func TestActionExecutor_Execute_Click_InvalidParams(t *testing.T) {
	mock := newMockBrowserBackend()
	mock.active = true
	security := NewSecurityValidator()
	executor := NewActionExecutor(mock, security)

	ctx := context.Background()
	// x 파라미터 누락
	params := map[string]interface{}{"y": 20.0}
	_, err := executor.Execute(ctx, "click", params)
	if err == nil {
		t.Error("Execute(click) with missing x = nil; want error")
	}
	if !strings.Contains(err.Error(), "invalid click params") {
		t.Errorf("error = %q; want containing 'invalid click params'", err.Error())
	}
}

func TestActionExecutor_Execute_Type(t *testing.T) {
	mock := newMockBrowserBackend()
	mock.active = true
	security := NewSecurityValidator()
	executor := NewActionExecutor(mock, security)

	ctx := context.Background()
	params := map[string]interface{}{"text": "hello world"}
	screenshot, err := executor.Execute(ctx, "type", params)
	if err != nil {
		t.Fatalf("Execute(type) = error %v; want nil", err)
	}
	if screenshot == "" {
		t.Error("Execute(type) returned empty screenshot")
	}
	if mock.typeCalled != 1 {
		t.Errorf("Type 호출 횟수 = %d; want 1", mock.typeCalled)
	}
	if mock.lastTypeText != "hello world" {
		t.Errorf("Type 텍스트 = %q; want %q", mock.lastTypeText, "hello world")
	}
}

func TestActionExecutor_Execute_Type_Error(t *testing.T) {
	mock := newMockBrowserBackend()
	mock.active = true
	mock.typeErr = fmt.Errorf("type failed")
	security := NewSecurityValidator()
	executor := NewActionExecutor(mock, security)

	ctx := context.Background()
	params := map[string]interface{}{"text": "test"}
	_, err := executor.Execute(ctx, "type", params)
	if err == nil {
		t.Error("Execute(type) with error = nil; want error")
	}
}

func TestActionExecutor_Execute_Type_InvalidParams(t *testing.T) {
	mock := newMockBrowserBackend()
	mock.active = true
	security := NewSecurityValidator()
	executor := NewActionExecutor(mock, security)

	ctx := context.Background()
	params := map[string]interface{}{} // text 누락
	_, err := executor.Execute(ctx, "type", params)
	if err == nil {
		t.Error("Execute(type) with missing text = nil; want error")
	}
	if !strings.Contains(err.Error(), "invalid type params") {
		t.Errorf("error = %q; want containing 'invalid type params'", err.Error())
	}
}

func TestActionExecutor_Execute_Scroll(t *testing.T) {
	mock := newMockBrowserBackend()
	mock.active = true
	security := NewSecurityValidator()
	executor := NewActionExecutor(mock, security)

	ctx := context.Background()
	params := map[string]interface{}{"direction": "down", "amount": 500.0}
	screenshot, err := executor.Execute(ctx, "scroll", params)
	if err != nil {
		t.Fatalf("Execute(scroll) = error %v; want nil", err)
	}
	if screenshot == "" {
		t.Error("Execute(scroll) returned empty screenshot")
	}
	if mock.scrollCalled != 1 {
		t.Errorf("Scroll 호출 횟수 = %d; want 1", mock.scrollCalled)
	}
	if mock.lastScrollDirection != "down" || mock.lastScrollAmount != 500 {
		t.Errorf("Scroll 인자 = (%q, %d); want (\"down\", 500)", mock.lastScrollDirection, mock.lastScrollAmount)
	}
}

func TestActionExecutor_Execute_Scroll_Error(t *testing.T) {
	mock := newMockBrowserBackend()
	mock.active = true
	mock.scrollErr = fmt.Errorf("scroll failed")
	security := NewSecurityValidator()
	executor := NewActionExecutor(mock, security)

	ctx := context.Background()
	params := map[string]interface{}{"direction": "down", "amount": 300.0}
	_, err := executor.Execute(ctx, "scroll", params)
	if err == nil {
		t.Error("Execute(scroll) with error = nil; want error")
	}
}

func TestActionExecutor_Execute_Scroll_InvalidParams(t *testing.T) {
	mock := newMockBrowserBackend()
	mock.active = true
	security := NewSecurityValidator()
	executor := NewActionExecutor(mock, security)

	ctx := context.Background()
	params := map[string]interface{}{} // direction 누락
	_, err := executor.Execute(ctx, "scroll", params)
	if err == nil {
		t.Error("Execute(scroll) with missing direction = nil; want error")
	}
	if !strings.Contains(err.Error(), "invalid scroll params") {
		t.Errorf("error = %q; want containing 'invalid scroll params'", err.Error())
	}
}

func TestActionExecutor_Execute_Navigate(t *testing.T) {
	mock := newMockBrowserBackend()
	mock.active = true
	security := NewSecurityValidator()
	executor := NewActionExecutor(mock, security)

	ctx := context.Background()
	params := map[string]interface{}{"url": "http://example.com"}
	screenshot, err := executor.Execute(ctx, "navigate", params)
	if err != nil {
		t.Fatalf("Execute(navigate) = error %v; want nil", err)
	}
	if screenshot == "" {
		t.Error("Execute(navigate) returned empty screenshot")
	}
	if mock.navigateCalled != 1 {
		t.Errorf("Navigate 호출 횟수 = %d; want 1", mock.navigateCalled)
	}
	if mock.lastNavigateURL != "http://example.com" {
		t.Errorf("Navigate URL = %q; want %q", mock.lastNavigateURL, "http://example.com")
	}
}

func TestActionExecutor_Execute_Navigate_Error(t *testing.T) {
	mock := newMockBrowserBackend()
	mock.active = true
	mock.navigateErr = fmt.Errorf("navigate failed")
	security := NewSecurityValidator()
	executor := NewActionExecutor(mock, security)

	ctx := context.Background()
	params := map[string]interface{}{"url": "http://example.com"}
	_, err := executor.Execute(ctx, "navigate", params)
	if err == nil {
		t.Error("Execute(navigate) with error = nil; want error")
	}
}

func TestActionExecutor_Execute_Navigate_InvalidParams(t *testing.T) {
	mock := newMockBrowserBackend()
	mock.active = true
	security := NewSecurityValidator()
	executor := NewActionExecutor(mock, security)

	ctx := context.Background()
	params := map[string]interface{}{} // url 누락
	_, err := executor.Execute(ctx, "navigate", params)
	if err == nil {
		t.Error("Execute(navigate) with missing url = nil; want error")
	}
	if !strings.Contains(err.Error(), "invalid navigate params") {
		t.Errorf("error = %q; want containing 'invalid navigate params'", err.Error())
	}
}

func TestActionExecutor_Execute_Navigate_BlockedURL(t *testing.T) {
	mock := newMockBrowserBackend()
	mock.active = true
	security := NewSecurityValidator()
	executor := NewActionExecutor(mock, security)

	ctx := context.Background()
	params := map[string]interface{}{"url": "file:///etc/passwd"}
	_, err := executor.Execute(ctx, "navigate", params)
	if err == nil {
		t.Error("Execute(navigate) with blocked URL = nil; want error")
	}
	if !strings.Contains(err.Error(), "URL blocked") {
		t.Errorf("error = %q; want containing 'URL blocked'", err.Error())
	}
	// Navigate는 호출되지 않아야 한다
	if mock.navigateCalled != 0 {
		t.Errorf("Navigate 호출 횟수 = %d; want 0 (URL blocked before navigate)", mock.navigateCalled)
	}
}

func TestActionExecutor_Execute_UnknownAction(t *testing.T) {
	mock := newMockBrowserBackend()
	mock.active = true
	security := NewSecurityValidator()
	executor := NewActionExecutor(mock, security)

	ctx := context.Background()
	_, err := executor.Execute(ctx, "unknown_action", nil)
	if err == nil {
		t.Error("Execute(unknown_action) = nil; want error")
	}
	if !strings.Contains(err.Error(), "unknown action") {
		t.Errorf("error = %q; want containing 'unknown action'", err.Error())
	}
}

func TestActionExecutor_Execute_BrowserNotActive(t *testing.T) {
	mock := newMockBrowserBackend()
	mock.active = false // 비활성 상태
	security := NewSecurityValidator()
	executor := NewActionExecutor(mock, security)

	ctx := context.Background()
	_, err := executor.Execute(ctx, "screenshot", nil)
	if err == nil {
		t.Error("Execute() when not active = nil; want error")
	}
	if !strings.Contains(err.Error(), "browser is not active") {
		t.Errorf("error = %q; want containing 'browser is not active'", err.Error())
	}
}

func TestActionExecutor_Execute_ScreenshotError(t *testing.T) {
	mock := newMockBrowserBackend()
	mock.active = true
	mock.screenshotErr = fmt.Errorf("screenshot capture failed")
	security := NewSecurityValidator()
	executor := NewActionExecutor(mock, security)

	ctx := context.Background()
	_, err := executor.Execute(ctx, "screenshot", nil)
	if err == nil {
		t.Error("Execute(screenshot) with screenshot error = nil; want error")
	}
	if !strings.Contains(err.Error(), "failed to capture screenshot") {
		t.Errorf("error = %q; want containing 'failed to capture screenshot'", err.Error())
	}
}

func TestActionExecutor_Execute_EncodeScreenshotError(t *testing.T) {
	// encodeScreenshot 실패 경로 테스트: 2MB 초과하지만 유효하지 않은 PNG 데이터 반환
	invalidLargeData := make([]byte, MaxScreenshotBytes+100)
	for i := range invalidLargeData {
		invalidLargeData[i] = byte(i % 256)
	}

	mock := newMockBrowserBackend()
	mock.active = true
	mock.screenshotData = invalidLargeData
	security := NewSecurityValidator()
	executor := NewActionExecutor(mock, security)

	ctx := context.Background()
	_, err := executor.Execute(ctx, "screenshot", nil)
	if err == nil {
		t.Error("Execute(screenshot) with invalid large PNG = nil; want encode error")
	}
	if !strings.Contains(err.Error(), "failed to encode screenshot") {
		t.Errorf("error = %q; want containing 'failed to encode screenshot'", err.Error())
	}
}

func TestActionExecutor_Execute_Scroll_UpDirection(t *testing.T) {
	// "up" 방향 스크롤 테스트 (커버리지: Scroll 내부 deltaY 음수 변환 경로)
	mock := newMockBrowserBackend()
	mock.active = true
	security := NewSecurityValidator()
	executor := NewActionExecutor(mock, security)

	ctx := context.Background()
	params := map[string]interface{}{"direction": "up", "amount": 200.0}
	screenshot, err := executor.Execute(ctx, "scroll", params)
	if err != nil {
		t.Fatalf("Execute(scroll up) = error %v; want nil", err)
	}
	if screenshot == "" {
		t.Error("Execute(scroll up) returned empty screenshot")
	}
	if mock.lastScrollDirection != "up" || mock.lastScrollAmount != 200 {
		t.Errorf("Scroll args = (%q, %d); want (\"up\", 200)", mock.lastScrollDirection, mock.lastScrollAmount)
	}
}
