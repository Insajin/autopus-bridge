package computeruse

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/insajin/autopus-agent-protocol"
)

// HandlerOption은 Handler 설정을 위한 함수형 옵션이다.
type HandlerOption func(*Handler)

// WithContainerPool은 Handler에 컨테이너 풀을 설정한다.
func WithContainerPool(pool *ContainerPool) HandlerOption {
	return func(h *Handler) {
		h.pool = pool
	}
}

// Handler handles computer use WebSocket messages.
// REQ-M2-01: Route computer_action messages to appropriate actions.
type Handler struct {
	sessionMgr *SessionManager
	security   *SecurityValidator
	pool       *ContainerPool // 컨테이너 풀 (nil이면 로컬 모드)
}

// NewHandler creates a new computer use Handler.
func NewHandler(opts ...HandlerOption) *Handler {
	h := &Handler{
		sessionMgr: NewSessionManager(),
		security:   NewSecurityValidator(),
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
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

	// 브라우저 실행
	if err := session.Backend.Launch(ctx); err != nil {
		// 실행 실패 시 세션 정리
		_ = h.sessionMgr.EndSession(payload.SessionID)
		return fmt.Errorf("failed to launch browser: %w", err)
	}

	// 초기 URL이 지정되면 이동
	if payload.URL != "" {
		if err := h.security.ValidateURL(payload.URL); err != nil {
			_ = h.sessionMgr.EndSession(payload.SessionID)
			return fmt.Errorf("initial URL blocked: %w", err)
		}
		if err := session.Backend.Navigate(ctx, payload.URL); err != nil {
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

	// 브라우저 상태 확인
	if !session.Backend.IsActive() {
		result.Success = false
		result.Error = "browser is not active for this session"
		result.DurationMs = time.Since(start).Milliseconds()
		return result, nil
	}

	// 액션 실행기 생성 및 실행
	executor := NewActionExecutor(session.Backend, h.security)
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

	// SPEC-COMPUTER-USE-002: 컨테이너 모드일 때 컨테이너 ID 포함
	if session.ContainerID != "" {
		result.ContainerID = session.ContainerID
	}

	log.Printf("[computer-use] action %s on session %s completed in %dms",
		payload.Action, payload.SessionID, result.DurationMs)

	return result, nil
}

// PoolStatus는 컨테이너 풀의 상태를 반환한다.
// 풀이 설정되지 않았으면 nil을 반환한다.
func (h *Handler) PoolStatus() *PoolStatus {
	if h.pool == nil {
		return nil
	}
	status := h.pool.Status()
	return &status
}

// GetPoolStatusPayload는 WebSocket으로 전송할 풀 상태 페이로드를 반환한다.
// 풀이 설정되지 않았으면 nil을 반환한다.
func (h *Handler) GetPoolStatusPayload() *ws.ComputerPoolStatusPayload {
	if h.pool == nil {
		return nil
	}
	status := h.pool.Status()
	return &ws.ComputerPoolStatusPayload{
		WarmCount:   status.WarmCount,
		ActiveCount: status.ActiveCount,
		MaxCount:    status.MaxCount,
	}
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
