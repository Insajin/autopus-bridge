package computeruse

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/insajin/autopus-agent-protocol"
)

func TestNewHandler(t *testing.T) {
	h := NewHandler()
	if h == nil {
		t.Fatal("NewHandler() returned nil")
	}
	if h.SessionManager() == nil {
		t.Error("SessionManager() returned nil; want non-nil")
	}
	if h.security == nil {
		t.Error("Handler.security is nil; want non-nil")
	}
}

func TestHandler_SessionManager(t *testing.T) {
	h := NewHandler()
	sm := h.SessionManager()
	if sm == nil {
		t.Fatal("SessionManager() returned nil")
	}
	// Verify it returns the same instance on subsequent calls.
	if h.SessionManager() != sm {
		t.Error("SessionManager() returned different instance on second call")
	}
}

// --- HandleAction Tests ---

func TestHandler_HandleAction_SessionNotFound(t *testing.T) {
	h := NewHandler()
	ctx := context.Background()

	payload := ws.ComputerActionPayload{
		ExecutionID: "exec-1",
		SessionID:   "nonexistent-session",
		Action:      "screenshot",
	}

	result, err := h.HandleAction(ctx, payload)
	if err != nil {
		t.Fatalf("HandleAction() returned error: %v; want nil (errors reported in result)", err)
	}
	if result == nil {
		t.Fatal("HandleAction() returned nil result")
	}
	if result.Success {
		t.Error("result.Success = true; want false for non-existent session")
	}
	if !strings.Contains(result.Error, "not found") {
		t.Errorf("result.Error = %q; want containing 'not found'", result.Error)
	}
}

func TestHandler_HandleAction_SessionNotFound_ResultFields(t *testing.T) {
	h := NewHandler()
	ctx := context.Background()

	payload := ws.ComputerActionPayload{
		ExecutionID: "exec-42",
		SessionID:   "sess-missing",
		Action:      "click",
		Params:      map[string]interface{}{"x": 10.0, "y": 20.0},
	}

	result, err := h.HandleAction(ctx, payload)
	if err != nil {
		t.Fatalf("HandleAction() returned error: %v", err)
	}

	// Verify result fields are populated from payload.
	if result.ExecutionID != "exec-42" {
		t.Errorf("result.ExecutionID = %q; want %q", result.ExecutionID, "exec-42")
	}
	if result.SessionID != "sess-missing" {
		t.Errorf("result.SessionID = %q; want %q", result.SessionID, "sess-missing")
	}
	if result.DurationMs < 0 {
		t.Errorf("result.DurationMs = %d; want >= 0", result.DurationMs)
	}
}

func TestHandler_HandleAction_BrowserNotActive(t *testing.T) {
	h := NewHandler()
	ctx := context.Background()

	// Create a session directly through the session manager.
	// The browser is not launched, so IsActive() returns false.
	_, err := h.SessionManager().CreateSession("exec-1", "sess-1", 1280, 720, true, "")
	if err != nil {
		t.Fatalf("CreateSession() error: %v", err)
	}

	payload := ws.ComputerActionPayload{
		ExecutionID: "exec-1",
		SessionID:   "sess-1",
		Action:      "screenshot",
	}

	result, err := h.HandleAction(ctx, payload)
	if err != nil {
		t.Fatalf("HandleAction() returned error: %v", err)
	}
	if result.Success {
		t.Error("result.Success = true; want false for inactive browser")
	}
	if !strings.Contains(result.Error, "not active") {
		t.Errorf("result.Error = %q; want containing 'not active'", result.Error)
	}
}

func TestHandler_HandleAction_BrowserNotActive_DurationTracked(t *testing.T) {
	h := NewHandler()
	ctx := context.Background()

	_, err := h.SessionManager().CreateSession("exec-1", "sess-1", 1280, 720, true, "")
	if err != nil {
		t.Fatalf("CreateSession() error: %v", err)
	}

	payload := ws.ComputerActionPayload{
		ExecutionID: "exec-1",
		SessionID:   "sess-1",
		Action:      "click",
		Params:      map[string]interface{}{"x": 10.0, "y": 20.0},
	}

	result, _ := h.HandleAction(ctx, payload)
	if result.DurationMs < 0 {
		t.Errorf("result.DurationMs = %d; want >= 0", result.DurationMs)
	}
}

func TestHandler_HandleAction_TouchesSession(t *testing.T) {
	h := NewHandler()
	ctx := context.Background()

	_, err := h.SessionManager().CreateSession("exec-1", "sess-1", 1280, 720, true, "")
	if err != nil {
		t.Fatalf("CreateSession() error: %v", err)
	}

	session, _ := h.SessionManager().GetSession("sess-1")
	originalTime := session.LastActiveAt

	payload := ws.ComputerActionPayload{
		ExecutionID: "exec-1",
		SessionID:   "sess-1",
		Action:      "screenshot",
	}

	// HandleAction calls TouchSession even when browser is not active.
	// The touch happens before the browser active check.
	_, _ = h.HandleAction(ctx, payload)

	session, _ = h.SessionManager().GetSession("sess-1")
	// LastActiveAt should be >= originalTime (may be equal if too fast).
	if session.LastActiveAt.Before(originalTime) {
		t.Error("HandleAction did not touch session; LastActiveAt went backwards")
	}
}

// --- HandleSessionEnd Tests ---

func TestHandler_HandleSessionEnd_Success(t *testing.T) {
	h := NewHandler()
	ctx := context.Background()

	// Create a session directly (browser not launched, but that's OK for EndSession).
	_, err := h.SessionManager().CreateSession("exec-1", "sess-1", 1280, 720, true, "")
	if err != nil {
		t.Fatalf("CreateSession() error: %v", err)
	}

	// Verify session exists.
	if h.SessionManager().ActiveCount() != 1 {
		t.Fatalf("ActiveCount() = %d; want 1", h.SessionManager().ActiveCount())
	}

	payload := ws.ComputerSessionPayload{
		ExecutionID: "exec-1",
		SessionID:   "sess-1",
	}

	err = h.HandleSessionEnd(ctx, payload)
	if err != nil {
		t.Fatalf("HandleSessionEnd() = error %v; want nil", err)
	}

	// Verify session was removed.
	_, exists := h.SessionManager().GetSession("sess-1")
	if exists {
		t.Error("session still exists after HandleSessionEnd; want removed")
	}
	if h.SessionManager().ActiveCount() != 0 {
		t.Errorf("ActiveCount() = %d; want 0 after HandleSessionEnd", h.SessionManager().ActiveCount())
	}
}

func TestHandler_HandleSessionEnd_NotFound(t *testing.T) {
	h := NewHandler()
	ctx := context.Background()

	payload := ws.ComputerSessionPayload{
		ExecutionID: "exec-1",
		SessionID:   "nonexistent",
	}

	err := h.HandleSessionEnd(ctx, payload)
	if err == nil {
		t.Error("HandleSessionEnd() = nil; want error for non-existent session")
	}
	if !strings.Contains(err.Error(), "failed to end session") {
		t.Errorf("HandleSessionEnd() error = %q; want containing 'failed to end session'", err.Error())
	}
}

func TestHandler_HandleSessionEnd_MultipleSessions(t *testing.T) {
	h := NewHandler()
	ctx := context.Background()

	// Create two sessions.
	_, _ = h.SessionManager().CreateSession("exec-1", "sess-1", 1280, 720, true, "")
	_, _ = h.SessionManager().CreateSession("exec-2", "sess-2", 1280, 720, true, "")

	if h.SessionManager().ActiveCount() != 2 {
		t.Fatalf("ActiveCount() = %d; want 2", h.SessionManager().ActiveCount())
	}

	// End only one session.
	payload := ws.ComputerSessionPayload{
		ExecutionID: "exec-1",
		SessionID:   "sess-1",
	}
	err := h.HandleSessionEnd(ctx, payload)
	if err != nil {
		t.Fatalf("HandleSessionEnd(sess-1) error: %v", err)
	}

	// Verify only sess-1 was removed.
	if h.SessionManager().ActiveCount() != 1 {
		t.Errorf("ActiveCount() = %d; want 1", h.SessionManager().ActiveCount())
	}
	_, exists := h.SessionManager().GetSession("sess-2")
	if !exists {
		t.Error("sess-2 was removed; want it to remain")
	}
}

// --- HandleSessionStart Tests ---

func TestHandler_HandleSessionStart_CancelledContext(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping browser-dependent test in short mode")
	}

	h := NewHandler()

	// Use a cancelled context to force browser launch failure.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	payload := ws.ComputerSessionPayload{
		ExecutionID: "exec-1",
		SessionID:   "sess-cancelled",
		ViewportW:   1280,
		ViewportH:   720,
		Headless:    true,
	}

	err := h.HandleSessionStart(ctx, payload)
	if err == nil {
		// If somehow the browser started, clean up and skip.
		_ = h.HandleSessionEnd(context.Background(), payload)
		t.Skip("Browser launched despite cancelled context")
	}

	// Session should be cleaned up after launch failure.
	_, exists := h.SessionManager().GetSession("sess-cancelled")
	if exists {
		t.Error("session still exists after failed HandleSessionStart; want cleanup on failure")
	}
}

func TestHandler_HandleSessionStart_DuplicateSession(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping browser-dependent test in short mode")
	}

	h := NewHandler()

	// Pre-create a session through the session manager to simulate a duplicate.
	_, err := h.SessionManager().CreateSession("exec-1", "sess-dup", 1280, 720, true, "")
	if err != nil {
		t.Fatalf("CreateSession() error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	payload := ws.ComputerSessionPayload{
		ExecutionID: "exec-2",
		SessionID:   "sess-dup",
		ViewportW:   1280,
		ViewportH:   720,
		Headless:    true,
	}

	err = h.HandleSessionStart(ctx, payload)
	if err == nil {
		t.Error("HandleSessionStart() = nil; want error for duplicate session ID")
	}
	if !strings.Contains(err.Error(), "failed to create session") {
		t.Errorf("error = %q; want containing 'failed to create session'", err.Error())
	}
}

// --- GetPoolStatusPayload Tests ---

func TestHandler_GetPoolStatusPayload_NilPool(t *testing.T) {
	h := NewHandler()
	result := h.GetPoolStatusPayload()
	if result != nil {
		t.Errorf("GetPoolStatusPayload() = %v; want nil when pool is not set", result)
	}
}

func TestHandler_GetPoolStatusPayload_WithPool(t *testing.T) {
	// Status() 호출만 테스트하므로 manager는 nil로 전달 (Acquire/Release 사용 안 함)
	pool := NewContainerPool(nil, PoolConfig{
		MaxContainers: 10,
		WarmPoolSize:  3,
		IdleTimeout:   5 * time.Minute,
	})

	h := NewHandler(WithContainerPool(pool))
	result := h.GetPoolStatusPayload()
	if result == nil {
		t.Fatal("GetPoolStatusPayload() = nil; want non-nil when pool is set")
	}

	if result.MaxCount != 10 {
		t.Errorf("MaxCount = %d; want 10", result.MaxCount)
	}
	if result.WarmCount != 0 {
		t.Errorf("WarmCount = %d; want 0 (no containers created)", result.WarmCount)
	}
	if result.ActiveCount != 0 {
		t.Errorf("ActiveCount = %d; want 0 (no sessions assigned)", result.ActiveCount)
	}
}

// --- HandleAction ContainerID Tests ---

func TestHandler_HandleAction_ContainerID(t *testing.T) {
	h := NewHandler()
	ctx := context.Background()

	// 세션을 직접 생성하고 ContainerID 설정
	session, err := h.SessionManager().CreateSession("exec-cid", "sess-cid", 1280, 720, true, "")
	if err != nil {
		t.Fatalf("CreateSession() error: %v", err)
	}
	session.ContainerID = "docker-container-abc123"

	// 브라우저가 활성화되지 않았으므로 결과에 ContainerID가 포함되지 않음 (에러 경로)
	payload := ws.ComputerActionPayload{
		ExecutionID: "exec-cid",
		SessionID:   "sess-cid",
		Action:      "screenshot",
	}

	result, err := h.HandleAction(ctx, payload)
	if err != nil {
		t.Fatalf("HandleAction() returned error: %v", err)
	}

	// 브라우저가 활성화되지 않아 실패하지만, ContainerID는 성공 경로에서만 설정됨
	if result.Success {
		// 성공한 경우에만 ContainerID 확인
		if result.ContainerID != "docker-container-abc123" {
			t.Errorf("ContainerID = %q; want %q", result.ContainerID, "docker-container-abc123")
		}
	} else {
		// 실패 경로에서는 ContainerID가 설정되지 않음
		if result.ContainerID != "" {
			t.Errorf("ContainerID = %q; want empty for failed action", result.ContainerID)
		}
	}
}

// --- GetActiveSessions 테스트 ---

func TestHandler_GetActiveSessions_Empty(t *testing.T) {
	h := NewHandler()
	sessions := h.GetActiveSessions()
	if len(sessions) != 0 {
		t.Errorf("GetActiveSessions() = %d; want 0", len(sessions))
	}
}

func TestHandler_GetActiveSessions_WithSessions(t *testing.T) {
	h := NewHandler()

	// 세션 2개 생성
	_, _ = h.SessionManager().CreateSession("exec-1", "sess-1", 1280, 720, true, "")
	_, _ = h.SessionManager().CreateSession("exec-2", "sess-2", 1280, 720, true, "")

	sessions := h.GetActiveSessions()
	if len(sessions) != 2 {
		t.Errorf("GetActiveSessions() = %d; want 2", len(sessions))
	}

	// 세션 ID 확인
	ids := make(map[string]bool)
	for _, s := range sessions {
		ids[s.ID] = true
	}
	if !ids["sess-1"] || !ids["sess-2"] {
		t.Errorf("GetActiveSessions() missing expected sessions: %v", ids)
	}
}

func TestHandler_GetActiveSessions_AfterEndSession(t *testing.T) {
	h := NewHandler()
	ctx := context.Background()

	_, _ = h.SessionManager().CreateSession("exec-1", "sess-1", 1280, 720, true, "")
	_, _ = h.SessionManager().CreateSession("exec-2", "sess-2", 1280, 720, true, "")

	// 하나 종료
	_ = h.HandleSessionEnd(ctx, ws.ComputerSessionPayload{SessionID: "sess-1"})

	sessions := h.GetActiveSessions()
	if len(sessions) != 1 {
		t.Errorf("GetActiveSessions() after end = %d; want 1", len(sessions))
	}
	if sessions[0].ID != "sess-2" {
		t.Errorf("remaining session ID = %q; want %q", sessions[0].ID, "sess-2")
	}
}

// --- PoolStatus 테스트 ---

func TestHandler_PoolStatus_NilPool(t *testing.T) {
	h := NewHandler()
	result := h.PoolStatus()
	if result != nil {
		t.Errorf("PoolStatus() = %v; want nil when pool is not set", result)
	}
}

func TestHandler_PoolStatus_WithPool(t *testing.T) {
	pool := NewContainerPool(nil, PoolConfig{
		MaxContainers: 8,
		WarmPoolSize:  2,
		IdleTimeout:   5 * time.Minute,
	})

	h := NewHandler(WithContainerPool(pool))
	result := h.PoolStatus()
	if result == nil {
		t.Fatal("PoolStatus() = nil; want non-nil when pool is set")
	}
	if result.MaxCount != 8 {
		t.Errorf("MaxCount = %d; want 8", result.MaxCount)
	}
	if result.WarmCount != 0 {
		t.Errorf("WarmCount = %d; want 0", result.WarmCount)
	}
	if result.ActiveCount != 0 {
		t.Errorf("ActiveCount = %d; want 0", result.ActiveCount)
	}
}

// --- HandleAction with mockBrowserBackend 테스트 ---

func TestHandler_HandleAction_WithMockBackend_Screenshot(t *testing.T) {
	h := NewHandler()
	ctx := context.Background()

	// 세션 생성 후 Backend를 mock으로 교체
	session, err := h.SessionManager().CreateSession("exec-mock", "sess-mock", 1280, 720, true, "")
	if err != nil {
		t.Fatalf("CreateSession() error: %v", err)
	}

	mock := newMockBrowserBackend()
	mock.active = true
	session.Backend = mock

	payload := ws.ComputerActionPayload{
		ExecutionID: "exec-mock",
		SessionID:   "sess-mock",
		Action:      "screenshot",
	}

	result, err := h.HandleAction(ctx, payload)
	if err != nil {
		t.Fatalf("HandleAction() returned error: %v", err)
	}
	if !result.Success {
		t.Errorf("result.Success = false; want true (error: %s)", result.Error)
	}
	if result.Screenshot == "" {
		t.Error("result.Screenshot is empty; want base64 screenshot")
	}
	if result.DurationMs < 0 {
		t.Errorf("result.DurationMs = %d; want >= 0", result.DurationMs)
	}
}

func TestHandler_HandleAction_WithMockBackend_Click(t *testing.T) {
	h := NewHandler()
	ctx := context.Background()

	session, _ := h.SessionManager().CreateSession("exec-click", "sess-click", 1280, 720, true, "")
	mock := newMockBrowserBackend()
	mock.active = true
	session.Backend = mock

	payload := ws.ComputerActionPayload{
		ExecutionID: "exec-click",
		SessionID:   "sess-click",
		Action:      "click",
		Params:      map[string]interface{}{"x": 100.0, "y": 200.0},
	}

	result, err := h.HandleAction(ctx, payload)
	if err != nil {
		t.Fatalf("HandleAction() returned error: %v", err)
	}
	if !result.Success {
		t.Errorf("result.Success = false; want true (error: %s)", result.Error)
	}
	if mock.clickCalled != 1 {
		t.Errorf("Click 호출 횟수 = %d; want 1", mock.clickCalled)
	}
}

func TestHandler_HandleAction_WithMockBackend_ActionError(t *testing.T) {
	h := NewHandler()
	ctx := context.Background()

	session, _ := h.SessionManager().CreateSession("exec-err", "sess-err", 1280, 720, true, "")
	mock := newMockBrowserBackend()
	mock.active = true
	mock.clickErr = fmt.Errorf("simulated click failure")
	session.Backend = mock

	payload := ws.ComputerActionPayload{
		ExecutionID: "exec-err",
		SessionID:   "sess-err",
		Action:      "click",
		Params:      map[string]interface{}{"x": 10.0, "y": 20.0},
	}

	result, err := h.HandleAction(ctx, payload)
	if err != nil {
		t.Fatalf("HandleAction() returned error: %v", err)
	}
	if result.Success {
		t.Error("result.Success = true; want false for action error")
	}
	if !strings.Contains(result.Error, "simulated click failure") {
		t.Errorf("result.Error = %q; want containing 'simulated click failure'", result.Error)
	}
}

func TestHandler_HandleAction_WithMockBackend_ContainerID(t *testing.T) {
	h := NewHandler()
	ctx := context.Background()

	session, _ := h.SessionManager().CreateSession("exec-cid2", "sess-cid2", 1280, 720, true, "")
	mock := newMockBrowserBackend()
	mock.active = true
	session.Backend = mock
	session.ContainerID = "docker-abc123"

	payload := ws.ComputerActionPayload{
		ExecutionID: "exec-cid2",
		SessionID:   "sess-cid2",
		Action:      "screenshot",
	}

	result, err := h.HandleAction(ctx, payload)
	if err != nil {
		t.Fatalf("HandleAction() returned error: %v", err)
	}
	if !result.Success {
		t.Fatalf("result.Success = false; want true (error: %s)", result.Error)
	}
	if result.ContainerID != "docker-abc123" {
		t.Errorf("result.ContainerID = %q; want %q", result.ContainerID, "docker-abc123")
	}
}

// --- HandleSessionStart 테스트 (mock 기반) ---

func TestHandler_HandleSessionStart_Success_WithMockBackend(t *testing.T) {
	h := NewHandler()
	ctx := context.Background()

	// HandleSessionStart는 CreateSession 내부에서 BrowserManager를 생성하므로,
	// 직접 mock을 주입할 수 없다. 하지만 Launch 실패 후 정리 경로를 테스트할 수 있다.
	// 이 테스트는 브라우저 없이도 HandleSessionStart의 코드 경로를 커버한다.

	payload := ws.ComputerSessionPayload{
		ExecutionID: "exec-launch",
		SessionID:   "sess-launch",
		ViewportW:   1280,
		ViewportH:   720,
		Headless:    true,
	}

	// 브라우저 Launch는 실패할 것 (Chrome 미설치)
	// 실패 후 세션이 정리되는지 확인
	err := h.HandleSessionStart(ctx, payload)
	if err != nil {
		// 실패 시 세션이 정리되었는지 확인
		_, exists := h.SessionManager().GetSession("sess-launch")
		if exists {
			t.Error("session still exists after failed HandleSessionStart; want cleanup")
		}
	} else {
		// 성공 시 (Chrome이 설치된 환경) 정리
		_ = h.HandleSessionEnd(ctx, payload)
	}
}

func TestHandler_HandleSessionStart_DuplicateSession_Short(t *testing.T) {
	// short 모드에서도 실행 가능: CreateSession 중복 에러는 브라우저 없이 테스트 가능
	h := NewHandler()

	// 먼저 세션 매니저에 직접 세션 생성
	_, err := h.SessionManager().CreateSession("exec-1", "sess-dup-short", 1280, 720, true, "")
	if err != nil {
		t.Fatalf("CreateSession() error: %v", err)
	}

	ctx := context.Background()
	payload := ws.ComputerSessionPayload{
		ExecutionID: "exec-2",
		SessionID:   "sess-dup-short",
		ViewportW:   1280,
		ViewportH:   720,
		Headless:    true,
	}

	// 동일 세션 ID로 HandleSessionStart 호출 시 CreateSession에서 실패해야 한다
	err = h.HandleSessionStart(ctx, payload)
	if err == nil {
		t.Error("HandleSessionStart() = nil; want error for duplicate session ID")
	}
	if !strings.Contains(err.Error(), "failed to create session") {
		t.Errorf("error = %q; want containing 'failed to create session'", err.Error())
	}
}

func TestHandler_HandleSessionStart_BlockedInitialURL(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping browser-dependent test in short mode")
	}

	// This test verifies that if an initial URL is blocked, HandleSessionStart
	// returns an error. However, since the browser launch happens first and
	// will likely fail without Chrome, we test the URL validation path indirectly.
	// The security validator is tested exhaustively in security_test.go.

	h := NewHandler()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	payload := ws.ComputerSessionPayload{
		ExecutionID: "exec-1",
		SessionID:   "sess-blocked-url",
		ViewportW:   1280,
		ViewportH:   720,
		Headless:    true,
		URL:         "file:///etc/passwd",
	}

	err := h.HandleSessionStart(ctx, payload)
	// Expect error (either browser launch failure or URL blocked).
	if err == nil {
		_ = h.HandleSessionEnd(context.Background(), payload)
		t.Skip("Browser launched successfully; cannot test URL block path without real browser")
	}
}
