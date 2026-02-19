package computeruse

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/insajin/autopus-agent-protocol"
)

// Handler handles computer use WebSocket messages.
// REQ-M2-01: Route computer_action messages to appropriate actions.
type Handler struct {
	sessionMgr *SessionManager
	security   *SecurityValidator
}

// NewHandler creates a new computer use Handler.
func NewHandler() *Handler {
	return &Handler{
		sessionMgr: NewSessionManager(),
		security:   NewSecurityValidator(),
	}
}

// SessionManager returns the handler's session manager for external lifecycle management.
func (h *Handler) SessionManager() *SessionManager {
	return h.sessionMgr
}

// GetActiveSessions returns all currently active computer use sessions.
// Used during WebSocket reconnection to restore session state (REQ-M3-04).
func (h *Handler) GetActiveSessions() []*Session {
	return h.sessionMgr.GetActiveSessions()
}

// HandleSessionStart processes computer_session_start messages.
// It creates a new browser session and launches the browser.
func (h *Handler) HandleSessionStart(ctx context.Context, payload ws.ComputerSessionPayload) error {
	log.Printf("[computer-use] starting session %s (execution=%s, viewport=%dx%d, headless=%v)",
		payload.SessionID, payload.ExecutionID, payload.ViewportW, payload.ViewportH, payload.Headless)

	// Create a new session.
	session, err := h.sessionMgr.CreateSession(
		payload.ExecutionID,
		payload.SessionID,
		payload.ViewportW,
		payload.ViewportH,
		payload.Headless,
		payload.URL,
	)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	// Launch the browser.
	if err := session.BrowserMgr.Launch(ctx); err != nil {
		// Clean up the session on launch failure.
		_ = h.sessionMgr.EndSession(payload.SessionID)
		return fmt.Errorf("failed to launch browser: %w", err)
	}

	// Navigate to initial URL if provided.
	if payload.URL != "" {
		if err := h.security.ValidateURL(payload.URL); err != nil {
			_ = h.sessionMgr.EndSession(payload.SessionID)
			return fmt.Errorf("initial URL blocked: %w", err)
		}
		if err := session.BrowserMgr.Navigate(ctx, payload.URL); err != nil {
			_ = h.sessionMgr.EndSession(payload.SessionID)
			return fmt.Errorf("failed to navigate to initial URL: %w", err)
		}
	}

	log.Printf("[computer-use] session %s started successfully", payload.SessionID)
	return nil
}

// HandleAction processes computer_action messages and returns the result.
// REQ-M2-01: Route computer_action messages to appropriate actions.
// REQ-M2-02: Check browser instance state before actions.
func (h *Handler) HandleAction(ctx context.Context, payload ws.ComputerActionPayload) (*ws.ComputerResultPayload, error) {
	start := time.Now()

	result := &ws.ComputerResultPayload{
		ExecutionID: payload.ExecutionID,
		SessionID:   payload.SessionID,
	}

	// Look up the session.
	session, exists := h.sessionMgr.GetSession(payload.SessionID)
	if !exists {
		result.Success = false
		result.Error = fmt.Sprintf("session %s not found", payload.SessionID)
		result.DurationMs = time.Since(start).Milliseconds()
		return result, nil
	}

	// Touch session to reset idle timer.
	h.sessionMgr.TouchSession(payload.SessionID)

	// Check browser state.
	if !session.BrowserMgr.IsActive() {
		result.Success = false
		result.Error = "browser is not active for this session"
		result.DurationMs = time.Since(start).Milliseconds()
		return result, nil
	}

	// Create action executor and execute the action.
	executor := NewActionExecutor(session.BrowserMgr, h.security)
	screenshot, err := executor.Execute(ctx, payload.Action, payload.Params)
	if err != nil {
		result.Success = false
		result.Error = err.Error()
		result.DurationMs = time.Since(start).Milliseconds()
		return result, nil
	}

	result.Success = true
	result.Screenshot = screenshot
	result.DurationMs = time.Since(start).Milliseconds()

	log.Printf("[computer-use] action %s on session %s completed in %dms",
		payload.Action, payload.SessionID, result.DurationMs)

	return result, nil
}

// HandleSessionEnd processes computer_session_end messages.
func (h *Handler) HandleSessionEnd(ctx context.Context, payload ws.ComputerSessionPayload) error {
	log.Printf("[computer-use] ending session %s", payload.SessionID)

	if err := h.sessionMgr.EndSession(payload.SessionID); err != nil {
		return fmt.Errorf("failed to end session: %w", err)
	}

	log.Printf("[computer-use] session %s ended", payload.SessionID)
	return nil
}
