package agentbrowser

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

func TestNewHandler(t *testing.T) {
	h := NewHandler()
	if h == nil {
		t.Fatal("NewHandler() returned nil")
	}
	if h.sessionMgr == nil {
		t.Error("sessionMgr is nil; want non-nil")
	}
	if h.installChecker == nil {
		t.Error("installChecker is nil; want non-nil")
	}
}

func TestNewHandler_WithLogger(t *testing.T) {
	logger := zerolog.Nop()
	h := NewHandler(WithLogger(logger))
	if h == nil {
		t.Fatal("NewHandler(WithLogger) returned nil")
	}
}

func TestHandler_SessionManager(t *testing.T) {
	h := NewHandler()
	sm := h.SessionManager()
	if sm == nil {
		t.Fatal("SessionManager() returned nil")
	}
	// 동일 인스턴스 반환 확인
	if h.SessionManager() != sm {
		t.Error("SessionManager() returned different instance on second call")
	}
}

// --- HandleSessionStart 테스트 ---

func TestHandler_HandleSessionStart_NotInstalled(t *testing.T) {
	h := NewHandler()

	// agent-browser 미설치 상태 모킹
	h.installChecker.lookPathFn = func(file string) (string, error) {
		return "", &exec.Error{Name: file, Err: exec.ErrNotFound}
	}

	payload := BrowserSessionPayload{
		ExecutionID: "exec-1",
		SessionID:   "sess-1",
		URL:         "https://example.com",
		Headless:    true,
	}

	result, err := h.HandleSessionStart(context.Background(), payload)
	if err == nil {
		t.Fatal("HandleSessionStart() returned nil error; want error for not installed")
	}
	if !strings.Contains(err.Error(), "설치되어 있지 않습니다") {
		t.Errorf("error = %q; want containing '설치되어 있지 않습니다'", err.Error())
	}

	// not_available 응답 확인
	if result == nil {
		t.Fatal("result is nil; want not_available response")
	}
	if result.Status != "not_available" {
		t.Errorf("result.Status = %q; want %q", result.Status, "not_available")
	}
	if result.ExecutionID != "exec-1" {
		t.Errorf("result.ExecutionID = %q; want %q", result.ExecutionID, "exec-1")
	}
	if result.SessionID != "sess-1" {
		t.Errorf("result.SessionID = %q; want %q", result.SessionID, "sess-1")
	}
}

func TestHandler_HandleSessionStart_Success(t *testing.T) {
	h := NewHandler()

	// 모두 설치된 상태 모킹
	h.installChecker.lookPathFn = func(file string) (string, error) {
		return "/usr/local/bin/" + file, nil
	}
	h.installChecker.commandFn = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "echo", "v1.0.0")
	}

	// URL을 비워서 Manager.Start가 open 명령을 실행하지 않도록 한다.
	// (agent-browser 바이너리가 설치되지 않은 환경에서 테스트 가능)
	payload := BrowserSessionPayload{
		ExecutionID: "exec-1",
		SessionID:   "sess-1",
		Headless:    true,
	}

	result, err := h.HandleSessionStart(context.Background(), payload)
	if err != nil {
		t.Fatalf("HandleSessionStart() returned error: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil; want ready response")
	}
	if result.Status != "ready" {
		t.Errorf("result.Status = %q; want %q", result.Status, "ready")
	}
	if result.ExecutionID != "exec-1" {
		t.Errorf("result.ExecutionID = %q; want %q", result.ExecutionID, "exec-1")
	}
	if result.SessionID != "sess-1" {
		t.Errorf("result.SessionID = %q; want %q", result.SessionID, "sess-1")
	}

	// 세션이 생성되었는지 확인
	if h.SessionManager().ActiveCount() != 1 {
		t.Errorf("ActiveCount() = %d; want 1", h.SessionManager().ActiveCount())
	}

	// 정리
	_ = h.HandleSessionEnd(context.Background(), payload)
}

func TestHandler_HandleSessionStart_DuplicateSession(t *testing.T) {
	h := NewHandler()

	// 설치 상태 모킹
	h.installChecker.lookPathFn = func(file string) (string, error) {
		return "/usr/local/bin/" + file, nil
	}
	h.installChecker.commandFn = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "echo", "v1.0.0")
	}

	// URL을 비워서 Manager.Start가 open 명령을 실행하지 않도록 한다.
	payload := BrowserSessionPayload{
		ExecutionID: "exec-1",
		SessionID:   "sess-dup",
		Headless:    true,
	}

	// 첫 번째 세션 시작
	_, err := h.HandleSessionStart(context.Background(), payload)
	if err != nil {
		t.Fatalf("first HandleSessionStart() returned error: %v", err)
	}

	// 동일 세션 ID로 두 번째 시작 시도
	_, err = h.HandleSessionStart(context.Background(), payload)
	if err == nil {
		t.Fatal("second HandleSessionStart() returned nil error; want error for duplicate")
	}
	if !strings.Contains(err.Error(), "세션 생성 실패") {
		t.Errorf("error = %q; want containing '세션 생성 실패'", err.Error())
	}

	// 정리
	_ = h.HandleSessionEnd(context.Background(), payload)
}

func TestHandler_HandleSessionStart_ManagerStartFailure(t *testing.T) {
	h := NewHandler()

	// 설치 상태 모킹 (install check는 성공하지만 manager start는 실패)
	h.installChecker.lookPathFn = func(file string) (string, error) {
		return "/usr/local/bin/" + file, nil
	}
	h.installChecker.commandFn = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "echo", "v1.0.0")
	}

	// 취소된 컨텍스트를 사용하여 manager.Start가 실패하도록 하려면
	// initialURL이 있어야 하고 executor가 실패해야 한다.
	// HandleSessionStart 내부에서 executor를 새로 생성하므로
	// manager.Start가 open 명령을 실행할 때 실패시키기 어렵다.
	// 대신, Manager.Start는 executor.Execute를 호출하고 이는 실제 agent-browser를 실행한다.
	// agent-browser가 설치되지 않은 환경에서는 실제로 실패한다.
	// 이 테스트는 install check가 성공했지만 실제 실행이 실패하는 케이스를 검증한다.

	payload := BrowserSessionPayload{
		ExecutionID: "exec-1",
		SessionID:   "sess-fail",
		URL:         "https://example.com",
		Headless:    true,
	}

	// agent-browser 바이너리가 실제로 존재하지 않으면 manager.Start가 실패
	// (CI 환경에서 agent-browser가 설치되지 않은 경우)
	result, err := h.HandleSessionStart(context.Background(), payload)
	if err != nil {
		// Manager 시작 실패 시 세션이 정리되었는지 확인
		_, exists := h.SessionManager().GetSession("sess-fail")
		if exists {
			t.Error("session still exists after failed manager start; want cleanup")
		}
		// 에러 메시지 확인
		if !strings.Contains(err.Error(), "시작 실패") {
			t.Errorf("error = %q; want containing '시작 실패'", err.Error())
		}
	} else {
		// agent-browser가 설치된 환경에서는 성공할 수 있다
		if result.Status != "ready" {
			t.Errorf("result.Status = %q; want %q", result.Status, "ready")
		}
		// 정리
		_ = h.HandleSessionEnd(context.Background(), payload)
	}
}

// --- HandleAction 테스트 ---

func TestHandler_HandleAction_SessionNotFound(t *testing.T) {
	h := NewHandler()

	payload := BrowserActionPayload{
		ExecutionID: "exec-1",
		SessionID:   "nonexistent",
		Command:     "click",
	}

	result, err := h.HandleAction(context.Background(), payload)
	if err != nil {
		t.Fatalf("HandleAction() returned error: %v; want nil (errors in result)", err)
	}
	if result == nil {
		t.Fatal("result is nil; want non-nil")
	}
	if result.Success {
		t.Error("result.Success = true; want false for non-existent session")
	}
	if !strings.Contains(result.Error, "찾을 수 없습니다") {
		t.Errorf("result.Error = %q; want containing '찾을 수 없습니다'", result.Error)
	}
	if result.DurationMs < 0 {
		t.Errorf("result.DurationMs = %d; want >= 0", result.DurationMs)
	}
}

func TestHandler_HandleAction_ResultFields(t *testing.T) {
	h := NewHandler()

	payload := BrowserActionPayload{
		ExecutionID: "exec-42",
		SessionID:   "sess-missing",
		Command:     "get",
	}

	result, _ := h.HandleAction(context.Background(), payload)
	if result.ExecutionID != "exec-42" {
		t.Errorf("result.ExecutionID = %q; want %q", result.ExecutionID, "exec-42")
	}
	if result.SessionID != "sess-missing" {
		t.Errorf("result.SessionID = %q; want %q", result.SessionID, "sess-missing")
	}
}

func TestHandler_HandleAction_ManagerNotReady(t *testing.T) {
	h := NewHandler()

	// 설치 상태 모킹
	h.installChecker.lookPathFn = func(file string) (string, error) {
		return "/usr/local/bin/" + file, nil
	}
	h.installChecker.commandFn = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "echo", "v1.0.0")
	}

	// 세션 생성 (Manager가 stopped 상태)
	executor := NewCommandExecutor(noopLogger())
	manager := NewManager(noopLogger(), executor)
	// Manager를 시작하지 않고 세션에 추가
	_, _ = h.sessionMgr.CreateSession("exec-1", "sess-1", manager, true, "")

	payload := BrowserActionPayload{
		ExecutionID: "exec-1",
		SessionID:   "sess-1",
		Command:     "click",
	}

	result, err := h.HandleAction(context.Background(), payload)
	if err != nil {
		t.Fatalf("HandleAction() returned error: %v", err)
	}
	if result.Success {
		t.Error("result.Success = true; want false for not-ready manager")
	}
	if !strings.Contains(result.Error, "준비되지 않았습니다") {
		t.Errorf("result.Error = %q; want containing '준비되지 않았습니다'", result.Error)
	}
}

func TestHandler_HandleAction_Success(t *testing.T) {
	h := NewHandler()

	// 설치 상태 모킹
	h.installChecker.lookPathFn = func(file string) (string, error) {
		return "/usr/local/bin/" + file, nil
	}
	h.installChecker.commandFn = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "echo", "v1.0.0")
	}

	// 세션 생성 + Manager를 ready 상태로 설정
	executor := NewCommandExecutor(noopLogger())
	executor.commandFn = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "echo", "action result")
	}
	manager := NewManager(noopLogger(), executor)
	_ = manager.Start(context.Background())

	_, _ = h.sessionMgr.CreateSession("exec-1", "sess-1", manager, true, "")

	payload := BrowserActionPayload{
		ExecutionID: "exec-1",
		SessionID:   "sess-1",
		Command:     "get",
		Params:      map[string]interface{}{"type": "url"},
	}

	result, err := h.HandleAction(context.Background(), payload)
	if err != nil {
		t.Fatalf("HandleAction() returned error: %v", err)
	}
	if !result.Success {
		t.Errorf("result.Success = false; want true (error: %s)", result.Error)
	}
	if result.DurationMs < 0 {
		t.Errorf("result.DurationMs = %d; want >= 0", result.DurationMs)
	}

	// 정리
	_ = manager.Stop()
}

func TestHandler_HandleAction_ExecutionError(t *testing.T) {
	h := NewHandler()

	// 세션 생성 + Manager를 ready 상태로 설정
	executor := NewCommandExecutor(noopLogger())
	executor.commandFn = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		// 실패하는 명령
		return exec.CommandContext(ctx, "false")
	}
	manager := NewManager(noopLogger(), executor)

	// initialURL 없이 시작 (open 명령 실행하지 않음)
	_ = manager.Start(context.Background())

	_, _ = h.sessionMgr.CreateSession("exec-1", "sess-err", manager, true, "")

	payload := BrowserActionPayload{
		ExecutionID: "exec-1",
		SessionID:   "sess-err",
		Command:     "click",
	}

	result, err := h.HandleAction(context.Background(), payload)
	if err != nil {
		t.Fatalf("HandleAction() returned error: %v; want nil (errors in result)", err)
	}
	if result.Success {
		t.Error("result.Success = true; want false for execution error")
	}

	// 정리
	_ = manager.Stop()
}

func TestHandler_HandleAction_TouchesSession(t *testing.T) {
	h := NewHandler()

	executor := NewCommandExecutor(noopLogger())
	manager := NewManager(noopLogger(), executor)
	_, _ = h.sessionMgr.CreateSession("exec-1", "sess-touch", manager, true, "")

	session, _ := h.sessionMgr.GetSession("sess-touch")
	originalTime := session.LastActiveAt

	payload := BrowserActionPayload{
		ExecutionID: "exec-1",
		SessionID:   "sess-touch",
		Command:     "screenshot",
	}

	// HandleAction은 Manager가 not-ready여도 TouchSession을 호출한다
	_, _ = h.HandleAction(context.Background(), payload)

	session, _ = h.sessionMgr.GetSession("sess-touch")
	if session.LastActiveAt.Before(originalTime) {
		t.Error("HandleAction did not touch session; LastActiveAt went backwards")
	}
}

// --- HandleSessionEnd 테스트 ---

func TestHandler_HandleSessionEnd_Success(t *testing.T) {
	h := NewHandler()

	// 세션 생성
	executor := NewCommandExecutor(noopLogger())
	manager := NewManager(noopLogger(), executor)
	_, _ = h.sessionMgr.CreateSession("exec-1", "sess-end", manager, true, "")

	payload := BrowserSessionPayload{
		ExecutionID: "exec-1",
		SessionID:   "sess-end",
	}

	err := h.HandleSessionEnd(context.Background(), payload)
	if err != nil {
		t.Fatalf("HandleSessionEnd() returned error: %v", err)
	}

	_, exists := h.sessionMgr.GetSession("sess-end")
	if exists {
		t.Error("session still exists after HandleSessionEnd(); want removed")
	}
}

func TestHandler_HandleSessionEnd_NotFound(t *testing.T) {
	h := NewHandler()

	payload := BrowserSessionPayload{
		ExecutionID: "exec-1",
		SessionID:   "nonexistent",
	}

	err := h.HandleSessionEnd(context.Background(), payload)
	if err == nil {
		t.Fatal("HandleSessionEnd() returned nil error; want error for non-existent session")
	}
	if !strings.Contains(err.Error(), "세션 종료 실패") {
		t.Errorf("error = %q; want containing '세션 종료 실패'", err.Error())
	}
}

func TestHandler_HandleSessionEnd_MultipleSessions(t *testing.T) {
	h := NewHandler()

	executor := NewCommandExecutor(noopLogger())
	manager1 := NewManager(noopLogger(), executor)
	manager2 := NewManager(noopLogger(), executor)
	_, _ = h.sessionMgr.CreateSession("exec-1", "sess-1", manager1, true, "")
	_, _ = h.sessionMgr.CreateSession("exec-2", "sess-2", manager2, true, "")

	// sess-1만 종료
	payload := BrowserSessionPayload{
		ExecutionID: "exec-1",
		SessionID:   "sess-1",
	}
	err := h.HandleSessionEnd(context.Background(), payload)
	if err != nil {
		t.Fatalf("HandleSessionEnd() returned error: %v", err)
	}

	// sess-1 제거, sess-2 유지 확인
	if h.SessionManager().ActiveCount() != 1 {
		t.Errorf("ActiveCount() = %d; want 1", h.SessionManager().ActiveCount())
	}
	_, exists := h.sessionMgr.GetSession("sess-2")
	if !exists {
		t.Error("sess-2 was removed; want it to remain")
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

	executor := NewCommandExecutor(noopLogger())
	manager1 := NewManager(noopLogger(), executor)
	manager2 := NewManager(noopLogger(), executor)
	_, _ = h.sessionMgr.CreateSession("exec-1", "sess-1", manager1, true, "")
	_, _ = h.sessionMgr.CreateSession("exec-2", "sess-2", manager2, true, "")

	sessions := h.GetActiveSessions()
	if len(sessions) != 2 {
		t.Errorf("GetActiveSessions() = %d; want 2", len(sessions))
	}

	ids := make(map[string]bool)
	for _, s := range sessions {
		ids[s.ID] = true
	}
	if !ids["sess-1"] || !ids["sess-2"] {
		t.Errorf("GetActiveSessions() missing expected sessions: %v", ids)
	}
}

// --- CI/CD 모드 Handler 테스트 ---

func TestNewHandler_WithCICDMode(t *testing.T) {
	config := CICDConfig{
		Headless:   true,
		JSONOutput: true,
		NoColor:    true,
	}
	h := NewHandler(WithCICDMode(config))

	if !h.cicdConfig.Headless {
		t.Error("handler.cicdConfig.Headless = false; want true")
	}
	if !h.cicdConfig.JSONOutput {
		t.Error("handler.cicdConfig.JSONOutput = false; want true")
	}
	if !h.cicdConfig.NoColor {
		t.Error("handler.cicdConfig.NoColor = false; want true")
	}
}

func TestHandler_HandleSessionStart_CICDMode(t *testing.T) {
	// CI/CD 환경 변수 모킹
	withMockEnv(t, map[string]string{})

	config := CICDConfig{
		Headless:   true,
		JSONOutput: true,
		NoColor:    true,
	}
	h := NewHandler(WithCICDMode(config))

	// 설치 상태 모킹
	h.installChecker.lookPathFn = func(file string) (string, error) {
		return "/usr/local/bin/" + file, nil
	}
	h.installChecker.commandFn = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "echo", "v1.0.0")
	}

	// headless=false로 요청하더라도 CI/CD 모드에서는 true로 강제된다.
	payload := BrowserSessionPayload{
		ExecutionID: "exec-1",
		SessionID:   "sess-cicd",
		Headless:    false,
	}

	result, err := h.HandleSessionStart(context.Background(), payload)
	if err != nil {
		t.Fatalf("HandleSessionStart() returned error: %v", err)
	}
	if result.Status != "ready" {
		t.Errorf("result.Status = %q; want %q", result.Status, "ready")
	}

	// 세션이 headless=true로 생성되었는지 확인
	session, exists := h.SessionManager().GetSession("sess-cicd")
	if !exists {
		t.Fatal("session not found after HandleSessionStart")
	}
	if !session.Headless {
		t.Error("session.Headless = false; want true (forced by CI/CD)")
	}
	if !session.CICDMode {
		t.Error("session.CICDMode = false; want true")
	}

	// 정리
	_ = h.HandleSessionEnd(context.Background(), payload)
}

func TestHandler_HandleSessionStart_NoCICDMode(t *testing.T) {
	// CI/CD 설정 없는 Handler
	h := NewHandler()

	// 설치 상태 모킹
	h.installChecker.lookPathFn = func(file string) (string, error) {
		return "/usr/local/bin/" + file, nil
	}
	h.installChecker.commandFn = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "echo", "v1.0.0")
	}

	payload := BrowserSessionPayload{
		ExecutionID: "exec-1",
		SessionID:   "sess-normal",
		Headless:    false,
	}

	result, err := h.HandleSessionStart(context.Background(), payload)
	if err != nil {
		t.Fatalf("HandleSessionStart() returned error: %v", err)
	}
	if result.Status != "ready" {
		t.Errorf("result.Status = %q; want %q", result.Status, "ready")
	}

	session, _ := h.SessionManager().GetSession("sess-normal")
	if session.Headless {
		t.Error("session.Headless = true; want false (no CI/CD mode)")
	}
	if session.CICDMode {
		t.Error("session.CICDMode = true; want false (no CI/CD mode)")
	}

	// 정리
	_ = h.HandleSessionEnd(context.Background(), payload)
}

func TestHandler_GetActiveSessions_AfterEndSession(t *testing.T) {
	h := NewHandler()

	executor := NewCommandExecutor(noopLogger())
	manager1 := NewManager(noopLogger(), executor)
	manager2 := NewManager(noopLogger(), executor)
	_, _ = h.sessionMgr.CreateSession("exec-1", "sess-1", manager1, true, "")
	_, _ = h.sessionMgr.CreateSession("exec-2", "sess-2", manager2, true, "")

	_ = h.HandleSessionEnd(context.Background(), BrowserSessionPayload{SessionID: "sess-1"})

	sessions := h.GetActiveSessions()
	if len(sessions) != 1 {
		t.Errorf("GetActiveSessions() after end = %d; want 1", len(sessions))
	}
	if sessions[0].ID != "sess-2" {
		t.Errorf("remaining session ID = %q; want %q", sessions[0].ID, "sess-2")
	}
}
