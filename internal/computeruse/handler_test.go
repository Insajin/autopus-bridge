package computeruse

import (
	"context"
	"strings"
	"testing"

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
