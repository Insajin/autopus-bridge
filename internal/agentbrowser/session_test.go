package agentbrowser

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestNewSessionManager(t *testing.T) {
	logger := noopLogger()
	sm := NewSessionManager(logger)

	if sm == nil {
		t.Fatal("NewSessionManager() returned nil")
	}
	if sm.sessions == nil {
		t.Error("sessions map is nil; want initialized")
	}
	if sm.maxConcurrent != 3 {
		t.Errorf("maxConcurrent = %d; want 3", sm.maxConcurrent)
	}
	if sm.ActiveCount() != 0 {
		t.Errorf("ActiveCount() = %d; want 0", sm.ActiveCount())
	}
}

func TestSessionManager_CreateSession_Success(t *testing.T) {
	logger := noopLogger()
	sm := NewSessionManager(logger)
	executor := NewCommandExecutor(logger)
	manager := NewManager(logger, executor)

	session, err := sm.CreateSession("exec-1", "sess-1", manager, true, "https://example.com")
	if err != nil {
		t.Fatalf("CreateSession() returned error: %v", err)
	}
	if session == nil {
		t.Fatal("session is nil; want non-nil")
	}
	if session.ID != "sess-1" {
		t.Errorf("session.ID = %q; want %q", session.ID, "sess-1")
	}
	if session.ExecutionID != "exec-1" {
		t.Errorf("session.ExecutionID = %q; want %q", session.ExecutionID, "exec-1")
	}
	if !session.Headless {
		t.Error("session.Headless = false; want true")
	}
	if session.URL != "https://example.com" {
		t.Errorf("session.URL = %q; want %q", session.URL, "https://example.com")
	}
	if session.Manager != manager {
		t.Error("session.Manager does not match the provided manager")
	}
	if sm.ActiveCount() != 1 {
		t.Errorf("ActiveCount() = %d; want 1", sm.ActiveCount())
	}
}

func TestSessionManager_CreateSession_DuplicateID(t *testing.T) {
	logger := noopLogger()
	sm := NewSessionManager(logger)
	executor := NewCommandExecutor(logger)
	manager := NewManager(logger, executor)

	_, _ = sm.CreateSession("exec-1", "sess-1", manager, true, "")

	_, err := sm.CreateSession("exec-2", "sess-1", manager, true, "")
	if err == nil {
		t.Fatal("CreateSession() returned nil error; want error for duplicate ID")
	}
	if !strings.Contains(err.Error(), "이미 존재") {
		t.Errorf("error = %q; want containing '이미 존재'", err.Error())
	}
}

func TestSessionManager_CreateSession_MaxConcurrent(t *testing.T) {
	logger := noopLogger()
	sm := NewSessionManager(logger)
	executor := NewCommandExecutor(logger)

	// maxConcurrent=3 이므로 3개까지 생성 가능
	for i := 0; i < 3; i++ {
		manager := NewManager(logger, executor)
		_, err := sm.CreateSession("exec", "sess-"+string(rune('a'+i)), manager, true, "")
		if err != nil {
			t.Fatalf("CreateSession(%d) returned error: %v", i, err)
		}
	}

	// 4번째 세션 생성 시 오류
	manager := NewManager(logger, executor)
	_, err := sm.CreateSession("exec", "sess-d", manager, true, "")
	if err == nil {
		t.Fatal("CreateSession() returned nil error; want error for max concurrent")
	}
	if !strings.Contains(err.Error(), "최대 동시 세션") {
		t.Errorf("error = %q; want containing '최대 동시 세션'", err.Error())
	}
}

func TestSessionManager_GetSession(t *testing.T) {
	logger := noopLogger()
	sm := NewSessionManager(logger)
	executor := NewCommandExecutor(logger)
	manager := NewManager(logger, executor)

	_, _ = sm.CreateSession("exec-1", "sess-1", manager, true, "")

	session, exists := sm.GetSession("sess-1")
	if !exists {
		t.Fatal("GetSession() returned false; want true")
	}
	if session.ID != "sess-1" {
		t.Errorf("session.ID = %q; want %q", session.ID, "sess-1")
	}
}

func TestSessionManager_GetSession_NotFound(t *testing.T) {
	logger := noopLogger()
	sm := NewSessionManager(logger)

	_, exists := sm.GetSession("nonexistent")
	if exists {
		t.Error("GetSession() returned true; want false for non-existent session")
	}
}

func TestSessionManager_EndSession_Success(t *testing.T) {
	logger := noopLogger()
	sm := NewSessionManager(logger)
	executor := NewCommandExecutor(logger)
	manager := NewManager(logger, executor)

	_, _ = sm.CreateSession("exec-1", "sess-1", manager, true, "")

	err := sm.EndSession("sess-1")
	if err != nil {
		t.Fatalf("EndSession() returned error: %v", err)
	}

	_, exists := sm.GetSession("sess-1")
	if exists {
		t.Error("session still exists after EndSession(); want removed")
	}
	if sm.ActiveCount() != 0 {
		t.Errorf("ActiveCount() = %d; want 0", sm.ActiveCount())
	}
}

func TestSessionManager_EndSession_NotFound(t *testing.T) {
	logger := noopLogger()
	sm := NewSessionManager(logger)

	err := sm.EndSession("nonexistent")
	if err == nil {
		t.Fatal("EndSession() returned nil error; want error for non-existent session")
	}
	if !strings.Contains(err.Error(), "찾을 수 없습니다") {
		t.Errorf("error = %q; want containing '찾을 수 없습니다'", err.Error())
	}
}

func TestSessionManager_TouchSession(t *testing.T) {
	logger := noopLogger()
	sm := NewSessionManager(logger)
	executor := NewCommandExecutor(logger)
	manager := NewManager(logger, executor)

	_, _ = sm.CreateSession("exec-1", "sess-1", manager, true, "")

	session, _ := sm.GetSession("sess-1")
	originalTime := session.LastActiveAt

	// 약간의 지연 후 Touch
	time.Sleep(1 * time.Millisecond)
	sm.TouchSession("sess-1")

	session, _ = sm.GetSession("sess-1")
	if !session.LastActiveAt.After(originalTime) {
		t.Error("TouchSession() did not update LastActiveAt")
	}
}

func TestSessionManager_TouchSession_NonExistent(t *testing.T) {
	logger := noopLogger()
	sm := NewSessionManager(logger)

	// 존재하지 않는 세션에 대한 Touch는 패닉하지 않아야 한다
	sm.TouchSession("nonexistent")
}

func TestSessionManager_GetActiveSessions(t *testing.T) {
	logger := noopLogger()
	sm := NewSessionManager(logger)
	executor := NewCommandExecutor(logger)

	// 빈 목록
	sessions := sm.GetActiveSessions()
	if len(sessions) != 0 {
		t.Errorf("GetActiveSessions() = %d; want 0", len(sessions))
	}

	// 2개 생성
	manager1 := NewManager(logger, executor)
	manager2 := NewManager(logger, executor)
	_, _ = sm.CreateSession("exec-1", "sess-1", manager1, true, "")
	_, _ = sm.CreateSession("exec-2", "sess-2", manager2, true, "")

	sessions = sm.GetActiveSessions()
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

func TestSessionManager_ActiveCount(t *testing.T) {
	logger := noopLogger()
	sm := NewSessionManager(logger)
	executor := NewCommandExecutor(logger)

	if sm.ActiveCount() != 0 {
		t.Errorf("ActiveCount() = %d; want 0", sm.ActiveCount())
	}

	manager := NewManager(logger, executor)
	_, _ = sm.CreateSession("exec-1", "sess-1", manager, true, "")

	if sm.ActiveCount() != 1 {
		t.Errorf("ActiveCount() = %d; want 1", sm.ActiveCount())
	}

	_ = sm.EndSession("sess-1")

	if sm.ActiveCount() != 0 {
		t.Errorf("ActiveCount() = %d; want 0 after EndSession", sm.ActiveCount())
	}
}

// --- PendingResult 테스트 ---

func TestSession_QueueResult(t *testing.T) {
	session := &Session{
		ID:          "sess-1",
		ExecutionID: "exec-1",
	}

	session.QueueResult("result-1")
	session.QueueResult("result-2")

	count := session.PendingResultCount()
	if count != 2 {
		t.Errorf("PendingResultCount() = %d; want 2", count)
	}
}

func TestSession_DrainPendingResults(t *testing.T) {
	session := &Session{
		ID:          "sess-1",
		ExecutionID: "exec-1",
	}

	session.QueueResult("result-1")
	session.QueueResult("result-2")

	results := session.DrainPendingResults()
	if len(results) != 2 {
		t.Errorf("DrainPendingResults() returned %d; want 2", len(results))
	}

	// Drain 후 큐가 비어야 한다
	if session.PendingResultCount() != 0 {
		t.Errorf("PendingResultCount() after drain = %d; want 0", session.PendingResultCount())
	}
}

func TestSession_DrainPendingResults_Empty(t *testing.T) {
	session := &Session{
		ID:          "sess-1",
		ExecutionID: "exec-1",
	}

	results := session.DrainPendingResults()
	if results != nil {
		t.Errorf("DrainPendingResults() on empty session = %v; want nil", results)
	}
}

func TestSession_PendingResultCount_Concurrent(t *testing.T) {
	session := &Session{
		ID:          "sess-1",
		ExecutionID: "exec-1",
	}

	// 동시에 여러 goroutine에서 큐잉
	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func(idx int) {
			session.QueueResult(idx)
			done <- struct{}{}
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	if session.PendingResultCount() != 10 {
		t.Errorf("PendingResultCount() = %d; want 10", session.PendingResultCount())
	}
}

func TestSession_PendingResult_Timestamp(t *testing.T) {
	session := &Session{
		ID:          "sess-1",
		ExecutionID: "exec-1",
	}

	before := time.Now()
	session.QueueResult("result")
	after := time.Now()

	results := session.DrainPendingResults()
	if len(results) != 1 {
		t.Fatalf("DrainPendingResults() returned %d; want 1", len(results))
	}

	if results[0].CreatedAt.Before(before) || results[0].CreatedAt.After(after) {
		t.Errorf("CreatedAt = %v; want between %v and %v", results[0].CreatedAt, before, after)
	}

	payload, ok := results[0].Payload.(string)
	if !ok {
		t.Fatalf("Payload type = %T; want string", results[0].Payload)
	}
	if payload != "result" {
		t.Errorf("Payload = %q; want %q", payload, "result")
	}
}

// --- 정리 루프 테스트 ---

func TestSessionManager_CleanupExpiredSessions_Idle(t *testing.T) {
	logger := noopLogger()
	sm := NewSessionManager(logger)
	sm.maxIdle = 1 * time.Millisecond // 매우 짧은 유휴 타임아웃

	executor := NewCommandExecutor(logger)
	manager := NewManager(logger, executor)

	_, _ = sm.CreateSession("exec-1", "sess-1", manager, true, "")

	// 유휴 타임아웃 경과
	time.Sleep(10 * time.Millisecond)

	sm.cleanupExpiredSessions()

	if sm.ActiveCount() != 0 {
		t.Errorf("ActiveCount() = %d; want 0 after idle timeout cleanup", sm.ActiveCount())
	}
}

func TestSessionManager_CleanupExpiredSessions_Active(t *testing.T) {
	logger := noopLogger()
	sm := NewSessionManager(logger)
	sm.maxActive = 1 * time.Millisecond // 매우 짧은 활성 타임아웃

	executor := NewCommandExecutor(logger)
	manager := NewManager(logger, executor)

	_, _ = sm.CreateSession("exec-1", "sess-1", manager, true, "")

	// 활성 타임아웃 경과
	time.Sleep(10 * time.Millisecond)

	sm.cleanupExpiredSessions()

	if sm.ActiveCount() != 0 {
		t.Errorf("ActiveCount() = %d; want 0 after active timeout cleanup", sm.ActiveCount())
	}
}

func TestSessionManager_CleanupExpiredSessions_NotExpired(t *testing.T) {
	logger := noopLogger()
	sm := NewSessionManager(logger)

	executor := NewCommandExecutor(logger)
	manager := NewManager(logger, executor)

	_, _ = sm.CreateSession("exec-1", "sess-1", manager, true, "")

	sm.cleanupExpiredSessions()

	// 기본 타임아웃이 충분히 길므로 세션이 유지되어야 한다
	if sm.ActiveCount() != 1 {
		t.Errorf("ActiveCount() = %d; want 1 (not expired)", sm.ActiveCount())
	}
}

func TestSessionManager_StartCleanupLoop_Cancel(t *testing.T) {
	logger := noopLogger()
	sm := NewSessionManager(logger)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		sm.StartCleanupLoop(ctx)
		close(done)
	}()

	cancel()

	select {
	case <-done:
		// 정상 종료
	case <-time.After(5 * time.Second):
		t.Fatal("StartCleanupLoop() did not stop after context cancellation")
	}
}

func TestSessionManager_CloseAllSessions(t *testing.T) {
	logger := noopLogger()
	sm := NewSessionManager(logger)
	executor := NewCommandExecutor(logger)

	// 3개의 세션 생성
	for i := 0; i < 3; i++ {
		manager := NewManager(logger, executor)
		_, _ = sm.CreateSession("exec", "sess-"+string(rune('a'+i)), manager, true, "")
	}

	if sm.ActiveCount() != 3 {
		t.Fatalf("ActiveCount() = %d; want 3 before closeAll", sm.ActiveCount())
	}

	sm.closeAllSessions()

	if sm.ActiveCount() != 0 {
		t.Errorf("ActiveCount() = %d; want 0 after closeAllSessions()", sm.ActiveCount())
	}
}

// --- CI/CD 모드에서 세션 생성 테스트 ---

func TestSessionManager_CreateSession_CICDForcesHeadless(t *testing.T) {
	// CI/CD 환경 변수 모킹
	withMockEnv(t, map[string]string{})

	logger := noopLogger()
	sm := NewSessionManager(logger)
	sm.SetCICDConfig(CICDConfig{Headless: true, JSONOutput: true, NoColor: true})

	executor := NewCommandExecutor(logger)
	manager := NewManager(logger, executor)

	// headless=false로 요청하더라도 CI/CD 모드에서는 true로 강제된다.
	session, err := sm.CreateSession("exec-1", "sess-cicd", manager, false, "https://example.com")
	if err != nil {
		t.Fatalf("CreateSession() returned error: %v", err)
	}
	if !session.Headless {
		t.Error("session.Headless = false; want true (forced by CI/CD mode)")
	}
	if !session.CICDMode {
		t.Error("session.CICDMode = false; want true")
	}
}

func TestSessionManager_CreateSession_NotCICD(t *testing.T) {
	// CI/CD 환경 변수 없음
	withMockEnv(t, map[string]string{})

	logger := noopLogger()
	sm := NewSessionManager(logger)
	// CI/CD 설정 없음 (기본값)

	executor := NewCommandExecutor(logger)
	manager := NewManager(logger, executor)

	session, err := sm.CreateSession("exec-1", "sess-normal", manager, false, "")
	if err != nil {
		t.Fatalf("CreateSession() returned error: %v", err)
	}
	if session.Headless {
		t.Error("session.Headless = true; want false when not in CI/CD")
	}
	if session.CICDMode {
		t.Error("session.CICDMode = true; want false when not in CI/CD")
	}
}

func TestSessionManager_CreateSession_CICDProvider(t *testing.T) {
	withMockEnv(t, map[string]string{"GITHUB_ACTIONS": "true"})

	logger := noopLogger()
	sm := NewSessionManager(logger)
	sm.SetCICDConfig(CICDConfig{Headless: true})

	executor := NewCommandExecutor(logger)
	manager := NewManager(logger, executor)

	session, err := sm.CreateSession("exec-1", "sess-gh", manager, true, "")
	if err != nil {
		t.Fatalf("CreateSession() returned error: %v", err)
	}
	if session.CICDProvider != "github-actions" {
		t.Errorf("session.CICDProvider = %q; want %q", session.CICDProvider, "github-actions")
	}
}

func TestSessionManager_CreateSession_CICDPreservesHeadlessTrue(t *testing.T) {
	withMockEnv(t, map[string]string{})

	logger := noopLogger()
	sm := NewSessionManager(logger)
	sm.SetCICDConfig(CICDConfig{Headless: true})

	executor := NewCommandExecutor(logger)
	manager := NewManager(logger, executor)

	// headless=true로 요청한 경우에도 CI/CD에서 true가 유지된다.
	session, err := sm.CreateSession("exec-1", "sess-hl", manager, true, "")
	if err != nil {
		t.Fatalf("CreateSession() returned error: %v", err)
	}
	if !session.Headless {
		t.Error("session.Headless = false; want true")
	}
}

// --- EndSession이 Manager.Stop()을 호출하는지 테스트 ---

func TestSessionManager_EndSession_StopsManager(t *testing.T) {
	logger := noopLogger()
	sm := NewSessionManager(logger)
	executor := NewCommandExecutor(logger)
	manager := NewManager(logger, executor)

	// Manager를 시작 상태로 만들기
	_ = manager.Start(context.Background())
	if manager.State() != StateReady {
		t.Fatalf("manager state = %q; want %q", manager.State(), StateReady)
	}

	_, _ = sm.CreateSession("exec-1", "sess-1", manager, true, "")

	err := sm.EndSession("sess-1")
	if err != nil {
		t.Fatalf("EndSession() returned error: %v", err)
	}

	// Manager가 중지되었는지 확인
	if manager.State() != StateStopped {
		t.Errorf("manager state after EndSession = %q; want %q", manager.State(), StateStopped)
	}
}
