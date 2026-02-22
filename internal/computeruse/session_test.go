package computeruse

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestSessionManager_CreateSession(t *testing.T) {
	sm := NewSessionManager()

	session, err := sm.CreateSession("exec-1", "sess-1", 1280, 720, true, "http://localhost:3000")
	if err != nil {
		t.Fatalf("CreateSession() = error %v; want nil", err)
	}

	if session.ID != "sess-1" {
		t.Errorf("session.ID = %q; want %q", session.ID, "sess-1")
	}
	if session.ExecutionID != "exec-1" {
		t.Errorf("session.ExecutionID = %q; want %q", session.ExecutionID, "exec-1")
	}
	if session.ViewportW != 1280 || session.ViewportH != 720 {
		t.Errorf("viewport = %dx%d; want 1280x720", session.ViewportW, session.ViewportH)
	}
	if session.Backend == nil {
		t.Error("session.Backend is nil; want non-nil")
	}
}

func TestSessionManager_CreateSession_DefaultViewport(t *testing.T) {
	sm := NewSessionManager()

	session, err := sm.CreateSession("exec-1", "sess-1", 0, 0, true, "")
	if err != nil {
		t.Fatalf("CreateSession() = error %v; want nil", err)
	}

	if session.ViewportW != 1280 || session.ViewportH != 720 {
		t.Errorf("viewport = %dx%d; want default 1280x720", session.ViewportW, session.ViewportH)
	}
}

func TestSessionManager_CreateSession_DuplicateID(t *testing.T) {
	sm := NewSessionManager()

	_, err := sm.CreateSession("exec-1", "sess-1", 1280, 720, true, "")
	if err != nil {
		t.Fatalf("first CreateSession() = error %v; want nil", err)
	}

	_, err = sm.CreateSession("exec-2", "sess-1", 1280, 720, true, "")
	if err == nil {
		t.Error("duplicate CreateSession() = nil; want error")
	}
}

func TestSessionManager_CreateSession_MaxConcurrent(t *testing.T) {
	sm := NewSessionManager()

	_, err := sm.CreateSession("exec-1", "sess-1", 1280, 720, true, "")
	if err != nil {
		t.Fatalf("CreateSession(1) = error %v; want nil", err)
	}

	_, err = sm.CreateSession("exec-2", "sess-2", 1280, 720, true, "")
	if err != nil {
		t.Fatalf("CreateSession(2) = error %v; want nil", err)
	}

	// Third session should fail (max is 2).
	_, err = sm.CreateSession("exec-3", "sess-3", 1280, 720, true, "")
	if err == nil {
		t.Error("CreateSession(3) = nil; want error (max concurrent reached)")
	}
}

func TestSessionManager_GetSession(t *testing.T) {
	sm := NewSessionManager()

	_, _ = sm.CreateSession("exec-1", "sess-1", 1280, 720, true, "")

	session, exists := sm.GetSession("sess-1")
	if !exists || session == nil {
		t.Fatal("GetSession(sess-1) not found; want found")
	}

	_, exists = sm.GetSession("nonexistent")
	if exists {
		t.Error("GetSession(nonexistent) found; want not found")
	}
}

func TestSessionManager_EndSession(t *testing.T) {
	sm := NewSessionManager()

	_, _ = sm.CreateSession("exec-1", "sess-1", 1280, 720, true, "")

	err := sm.EndSession("sess-1")
	if err != nil {
		t.Fatalf("EndSession() = error %v; want nil", err)
	}

	_, exists := sm.GetSession("sess-1")
	if exists {
		t.Error("GetSession after EndSession found; want not found")
	}

	// Ending a nonexistent session should return error.
	err = sm.EndSession("nonexistent")
	if err == nil {
		t.Error("EndSession(nonexistent) = nil; want error")
	}
}

func TestSessionManager_TouchSession(t *testing.T) {
	sm := NewSessionManager()

	_, _ = sm.CreateSession("exec-1", "sess-1", 1280, 720, true, "")

	session, _ := sm.GetSession("sess-1")
	originalTime := session.LastActiveAt

	// Small delay to ensure time difference.
	time.Sleep(10 * time.Millisecond)

	sm.TouchSession("sess-1")

	session, _ = sm.GetSession("sess-1")
	if !session.LastActiveAt.After(originalTime) {
		t.Error("TouchSession did not update LastActiveAt")
	}
}

func TestSessionManager_ActiveCount(t *testing.T) {
	sm := NewSessionManager()

	if sm.ActiveCount() != 0 {
		t.Errorf("ActiveCount() = %d; want 0", sm.ActiveCount())
	}

	_, _ = sm.CreateSession("exec-1", "sess-1", 1280, 720, true, "")
	if sm.ActiveCount() != 1 {
		t.Errorf("ActiveCount() = %d; want 1", sm.ActiveCount())
	}

	_, _ = sm.CreateSession("exec-2", "sess-2", 1280, 720, true, "")
	if sm.ActiveCount() != 2 {
		t.Errorf("ActiveCount() = %d; want 2", sm.ActiveCount())
	}

	_ = sm.EndSession("sess-1")
	if sm.ActiveCount() != 1 {
		t.Errorf("ActiveCount() = %d; want 1", sm.ActiveCount())
	}
}

// REQ-M3-04: Pending results queue tests.

func TestSession_QueueResult(t *testing.T) {
	sm := NewSessionManager()
	session, _ := sm.CreateSession("exec-1", "sess-1", 1280, 720, true, "")

	if session.PendingResultCount() != 0 {
		t.Errorf("PendingResultCount() = %d; want 0", session.PendingResultCount())
	}

	session.QueueResult("result-1")
	session.QueueResult("result-2")

	if session.PendingResultCount() != 2 {
		t.Errorf("PendingResultCount() = %d; want 2", session.PendingResultCount())
	}
}

func TestSession_DrainPendingResults(t *testing.T) {
	sm := NewSessionManager()
	session, _ := sm.CreateSession("exec-1", "sess-1", 1280, 720, true, "")

	session.QueueResult("result-1")
	session.QueueResult("result-2")
	session.QueueResult("result-3")

	results := session.DrainPendingResults()

	if len(results) != 3 {
		t.Fatalf("DrainPendingResults() returned %d results; want 3", len(results))
	}

	// Verify payloads.
	for i, r := range results {
		expected := fmt.Sprintf("result-%d", i+1)
		if r.Payload != expected {
			t.Errorf("results[%d].Payload = %v; want %q", i, r.Payload, expected)
		}
		if r.CreatedAt.IsZero() {
			t.Errorf("results[%d].CreatedAt is zero; want non-zero", i)
		}
	}

	// Queue should be empty after drain.
	if session.PendingResultCount() != 0 {
		t.Errorf("PendingResultCount() after drain = %d; want 0", session.PendingResultCount())
	}
}

func TestSession_DrainPendingResults_Empty(t *testing.T) {
	sm := NewSessionManager()
	session, _ := sm.CreateSession("exec-1", "sess-1", 1280, 720, true, "")

	results := session.DrainPendingResults()

	if results != nil {
		t.Errorf("DrainPendingResults() on empty = %v; want nil", results)
	}
}

func TestSession_QueueResult_AfterDrain(t *testing.T) {
	sm := NewSessionManager()
	session, _ := sm.CreateSession("exec-1", "sess-1", 1280, 720, true, "")

	// First cycle: queue and drain.
	session.QueueResult("first")
	_ = session.DrainPendingResults()

	// Second cycle: queue new results after drain.
	session.QueueResult("second")
	results := session.DrainPendingResults()

	if len(results) != 1 {
		t.Fatalf("DrainPendingResults() = %d results; want 1", len(results))
	}
	if results[0].Payload != "second" {
		t.Errorf("results[0].Payload = %v; want %q", results[0].Payload, "second")
	}
}

func TestSessionManager_GetActiveSessions(t *testing.T) {
	sm := NewSessionManager()

	// No sessions initially.
	sessions := sm.GetActiveSessions()
	if len(sessions) != 0 {
		t.Errorf("GetActiveSessions() = %d; want 0", len(sessions))
	}

	// Create two sessions.
	_, _ = sm.CreateSession("exec-1", "sess-1", 1280, 720, true, "")
	_, _ = sm.CreateSession("exec-2", "sess-2", 1280, 720, true, "")

	sessions = sm.GetActiveSessions()
	if len(sessions) != 2 {
		t.Errorf("GetActiveSessions() = %d; want 2", len(sessions))
	}

	// Verify all returned sessions have valid IDs.
	ids := make(map[string]bool)
	for _, s := range sessions {
		ids[s.ID] = true
	}
	if !ids["sess-1"] || !ids["sess-2"] {
		t.Errorf("GetActiveSessions() missing expected sessions: got %v", ids)
	}

	// End one session.
	_ = sm.EndSession("sess-1")
	sessions = sm.GetActiveSessions()
	if len(sessions) != 1 {
		t.Errorf("GetActiveSessions() after end = %d; want 1", len(sessions))
	}
	if sessions[0].ID != "sess-2" {
		t.Errorf("remaining session ID = %q; want %q", sessions[0].ID, "sess-2")
	}
}

func TestSessionManager_cleanupExpiredSessions_IdleTimeout(t *testing.T) {
	sm := NewSessionManager()
	// 매우 짧은 유휴 타임아웃 설정
	sm.maxIdle = 1 * time.Millisecond

	_, _ = sm.CreateSession("exec-1", "sess-1", 1280, 720, true, "")

	// 유휴 타임아웃 만료 대기
	time.Sleep(10 * time.Millisecond)

	sm.cleanupExpiredSessions()

	if sm.ActiveCount() != 0 {
		t.Errorf("ActiveCount after idle cleanup = %d; want 0", sm.ActiveCount())
	}
}

func TestSessionManager_cleanupExpiredSessions_MaxActiveTimeout(t *testing.T) {
	sm := NewSessionManager()
	// 유휴 타임아웃은 길게, 최대 활성 시간은 짧게 설정
	sm.maxIdle = 1 * time.Hour
	sm.maxActive = 1 * time.Millisecond

	_, _ = sm.CreateSession("exec-1", "sess-active-expired", 1280, 720, true, "")

	// 최대 활성 시간 만료 대기
	time.Sleep(10 * time.Millisecond)

	sm.cleanupExpiredSessions()

	if sm.ActiveCount() != 0 {
		t.Errorf("ActiveCount after max active cleanup = %d; want 0", sm.ActiveCount())
	}

	_, exists := sm.GetSession("sess-active-expired")
	if exists {
		t.Error("session still exists after max active timeout; want removed")
	}
}

func TestSessionManager_cleanupExpiredSessions_MixedExpiry(t *testing.T) {
	sm := NewSessionManager()
	sm.maxIdle = 1 * time.Millisecond
	sm.maxActive = 1 * time.Hour

	// 세션 생성
	_, _ = sm.CreateSession("exec-1", "sess-expired", 1280, 720, true, "")

	// 유휴 타임아웃 만료 대기
	time.Sleep(10 * time.Millisecond)

	// 두 번째 세션은 방금 생성 (아직 만료되지 않음)
	// maxPerWorkspace 제한으로 인해 먼저 정리 후 생성
	sm.cleanupExpiredSessions()

	// 첫 번째 세션만 정리됨
	if sm.ActiveCount() != 0 {
		t.Errorf("ActiveCount = %d; want 0 (expired session removed)", sm.ActiveCount())
	}

	// 새 세션 생성 가능 확인
	_, err := sm.CreateSession("exec-2", "sess-fresh", 1280, 720, true, "")
	if err != nil {
		t.Fatalf("CreateSession after cleanup = error %v; want nil", err)
	}

	// Touch로 유휴 타이머 갱신
	sm.TouchSession("sess-fresh")

	sm.cleanupExpiredSessions()

	// 새 세션은 아직 유효
	if sm.ActiveCount() != 1 {
		t.Errorf("ActiveCount = %d; want 1 (fresh session should remain)", sm.ActiveCount())
	}
}

func TestSessionManager_cleanupExpiredSessions_WithMockBackend(t *testing.T) {
	sm := NewSessionManager()
	sm.maxIdle = 1 * time.Millisecond

	session, _ := sm.CreateSession("exec-1", "sess-mock-cleanup", 1280, 720, true, "")

	// Backend를 mock으로 교체하여 Close 호출 확인
	mock := newMockBrowserBackend()
	mock.active = true
	session.Backend = mock

	time.Sleep(10 * time.Millisecond)
	sm.cleanupExpiredSessions()

	if sm.ActiveCount() != 0 {
		t.Errorf("ActiveCount = %d; want 0", sm.ActiveCount())
	}
	if mock.closeCalled != 1 {
		t.Errorf("Backend.Close 호출 횟수 = %d; want 1", mock.closeCalled)
	}
}

// --- StartCleanupLoop 테스트 ---

func TestSessionManager_StartCleanupLoop_CancelStops(t *testing.T) {
	sm := NewSessionManager()

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		sm.StartCleanupLoop(ctx)
		close(done)
	}()

	// 즉시 취소
	cancel()

	select {
	case <-done:
		// 정상 종료
	case <-time.After(5 * time.Second):
		t.Error("StartCleanupLoop이 컨텍스트 취소 후 종료되지 않음")
	}
}

func TestSessionManager_StartCleanupLoop_CleansUpOnCancel(t *testing.T) {
	sm := NewSessionManager()

	// mock backend로 세션 생성
	session, _ := sm.CreateSession("exec-1", "sess-cleanup-loop", 1280, 720, true, "")
	mock := newMockBrowserBackend()
	mock.active = true
	session.Backend = mock

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		sm.StartCleanupLoop(ctx)
		close(done)
	}()

	// 취소하여 closeAllSessions 실행 트리거
	cancel()

	select {
	case <-done:
		// 정상 종료
	case <-time.After(5 * time.Second):
		t.Fatal("StartCleanupLoop이 종료되지 않음")
	}

	// closeAllSessions에 의해 세션이 정리되어야 한다
	if sm.ActiveCount() != 0 {
		t.Errorf("ActiveCount after cleanup loop cancel = %d; want 0", sm.ActiveCount())
	}
	if mock.closeCalled != 1 {
		t.Errorf("Backend.Close 호출 횟수 = %d; want 1", mock.closeCalled)
	}
}

// --- closeAllSessions 테스트 ---

func TestSessionManager_closeAllSessions(t *testing.T) {
	sm := NewSessionManager()

	// mock backend로 세션 2개 생성
	session1, _ := sm.CreateSession("exec-1", "sess-close-1", 1280, 720, true, "")
	mock1 := newMockBrowserBackend()
	mock1.active = true
	session1.Backend = mock1

	session2, _ := sm.CreateSession("exec-2", "sess-close-2", 1280, 720, true, "")
	mock2 := newMockBrowserBackend()
	mock2.active = true
	session2.Backend = mock2

	if sm.ActiveCount() != 2 {
		t.Fatalf("ActiveCount = %d; want 2", sm.ActiveCount())
	}

	sm.closeAllSessions()

	if sm.ActiveCount() != 0 {
		t.Errorf("ActiveCount after closeAllSessions = %d; want 0", sm.ActiveCount())
	}
	if mock1.closeCalled != 1 {
		t.Errorf("mock1.Close 호출 횟수 = %d; want 1", mock1.closeCalled)
	}
	if mock2.closeCalled != 1 {
		t.Errorf("mock2.Close 호출 횟수 = %d; want 1", mock2.closeCalled)
	}
}

func TestSessionManager_closeAllSessions_Empty(t *testing.T) {
	sm := NewSessionManager()

	// 세션 없는 상태에서 호출해도 패닉 없어야 한다
	sm.closeAllSessions()

	if sm.ActiveCount() != 0 {
		t.Errorf("ActiveCount = %d; want 0", sm.ActiveCount())
	}
}

func TestSessionManager_cleanupExpiredSessions_WithCloseError(t *testing.T) {
	sm := NewSessionManager()
	sm.maxIdle = 1 * time.Millisecond

	session, _ := sm.CreateSession("exec-1", "sess-cleanup-err", 1280, 720, true, "")
	mock := newMockBrowserBackend()
	mock.active = true
	mock.closeErr = fmt.Errorf("browser cleanup failed")
	session.Backend = mock

	time.Sleep(10 * time.Millisecond)
	sm.cleanupExpiredSessions()

	// Close 에러가 있어도 세션은 제거되어야 한다
	if sm.ActiveCount() != 0 {
		t.Errorf("ActiveCount = %d; want 0", sm.ActiveCount())
	}
	if mock.closeCalled != 1 {
		t.Errorf("Backend.Close 호출 횟수 = %d; want 1", mock.closeCalled)
	}
}

func TestSessionManager_EndSession_WithCloseError(t *testing.T) {
	sm := NewSessionManager()

	session, _ := sm.CreateSession("exec-1", "sess-close-err", 1280, 720, true, "")
	mock := newMockBrowserBackend()
	mock.active = true
	mock.closeErr = fmt.Errorf("browser close failed")
	session.Backend = mock

	// Close 에러가 있어도 EndSession은 성공해야 한다 (세션은 제거됨)
	err := sm.EndSession("sess-close-err")
	if err != nil {
		t.Errorf("EndSession() with close error = %v; want nil", err)
	}

	// 세션이 제거되었는지 확인
	_, exists := sm.GetSession("sess-close-err")
	if exists {
		t.Error("session still exists after EndSession with close error; want removed")
	}
}

func TestSessionManager_closeAllSessions_WithCloseError(t *testing.T) {
	sm := NewSessionManager()

	session, _ := sm.CreateSession("exec-1", "sess-close-err", 1280, 720, true, "")
	mock := newMockBrowserBackend()
	mock.active = true
	mock.closeErr = fmt.Errorf("close failed")
	session.Backend = mock

	// Close 에러가 있어도 세션은 정리되어야 한다
	sm.closeAllSessions()

	if sm.ActiveCount() != 0 {
		t.Errorf("ActiveCount = %d; want 0 (session removed despite close error)", sm.ActiveCount())
	}
}
