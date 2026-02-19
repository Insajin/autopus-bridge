package computeruse

import (
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
	if session.BrowserMgr == nil {
		t.Error("session.BrowserMgr is nil; want non-nil")
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

func TestSessionManager_cleanupExpiredSessions(t *testing.T) {
	sm := NewSessionManager()
	// Set very short idle timeout for testing.
	sm.maxIdle = 1 * time.Millisecond

	_, _ = sm.CreateSession("exec-1", "sess-1", 1280, 720, true, "")

	// Wait for idle timeout to expire.
	time.Sleep(10 * time.Millisecond)

	sm.cleanupExpiredSessions()

	if sm.ActiveCount() != 0 {
		t.Errorf("ActiveCount after cleanup = %d; want 0", sm.ActiveCount())
	}
}
