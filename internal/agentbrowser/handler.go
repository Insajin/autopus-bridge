package agentbrowser

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"
)

// HandlerOption은 Handler 설정을 위한 함수형 옵션이다.
type HandlerOption func(*Handler)

// WithLogger는 Handler에 로거를 설정한다.
func WithLogger(logger zerolog.Logger) HandlerOption {
	return func(h *Handler) {
		h.logger = logger
	}
}

// WithCICDMode는 CI/CD 환경 설정을 Handler에 적용한다 (REQ-M5-04).
// Handler가 생성하는 모든 Manager 및 Session에 CI/CD 설정을 전파한다.
func WithCICDMode(config CICDConfig) HandlerOption {
	return func(h *Handler) {
		h.cicdConfig = config
	}
}

// Handler는 agent-browser WebSocket 메시지를 처리하는 메인 핸들러이다.
// Manager, SessionManager, HealthChecker, InstallChecker를 조합한다.
type Handler struct {
	sessionMgr     *SessionManager
	installChecker *InstallChecker
	logger         zerolog.Logger
	// cicdConfig는 CI/CD 환경 설정이다 (REQ-M5-04).
	cicdConfig CICDConfig
}

// NewHandler는 새로운 Handler를 생성한다.
func NewHandler(opts ...HandlerOption) *Handler {
	logger := zerolog.Nop()
	h := &Handler{
		logger: logger,
	}
	for _, opt := range opts {
		opt(h)
	}

	h.sessionMgr = NewSessionManager(h.logger)
	h.installChecker = NewInstallChecker(h.logger)

	// CI/CD 설정을 SessionManager에 전파한다 (REQ-M5-04).
	if h.cicdConfig.IsEnabled() {
		h.sessionMgr.SetCICDConfig(h.cicdConfig)
		h.logger.Info().
			Bool("headless", h.cicdConfig.Headless).
			Bool("json_output", h.cicdConfig.JSONOutput).
			Bool("no_color", h.cicdConfig.NoColor).
			Dur("timeout", h.cicdConfig.Timeout).
			Msg("CI/CD 모드 활성화")
	}

	return h
}

// SessionManager는 핸들러의 세션 매니저를 반환한다.
func (h *Handler) SessionManager() *SessionManager {
	return h.sessionMgr
}

// HandleSessionStart는 browser_session_start 메시지를 처리한다.
// 설치 확인 후 세션을 생성하고 데몬을 시작한다.
func (h *Handler) HandleSessionStart(ctx context.Context, payload BrowserSessionPayload) (*BrowserSessionPayload, error) {
	h.logger.Info().
		Str("session_id", payload.SessionID).
		Str("execution_id", payload.ExecutionID).
		Bool("headless", payload.Headless).
		Str("url", payload.URL).
		Msg("agent-browser 세션 시작 요청")

	// 설치 확인
	checkResult, err := h.installChecker.Check(ctx)
	if err != nil {
		return nil, fmt.Errorf("설치 상태 확인 실패: %w", err)
	}

	if !checkResult.Installed {
		// browser_not_available 응답
		notAvailable := &BrowserSessionPayload{
			ExecutionID: payload.ExecutionID,
			SessionID:   payload.SessionID,
			Status:      "not_available",
		}
		return notAvailable, fmt.Errorf("agent-browser가 설치되어 있지 않습니다: %s", checkResult.InstallGuide)
	}

	// 명령 실행기 및 매니저 생성
	executor := NewCommandExecutor(h.logger)
	managerOpts := []ManagerOption{
		WithHeadless(payload.Headless),
		WithInitialURL(payload.URL),
	}

	// CI/CD 모드가 활성화된 경우 Manager에 설정 전파 (REQ-M5-04)
	if h.cicdConfig.IsEnabled() {
		managerOpts = append(managerOpts, WithCICDConfig(h.cicdConfig))
	}

	manager := NewManager(h.logger, executor, managerOpts...)

	// 세션 생성
	_, err = h.sessionMgr.CreateSession(
		payload.ExecutionID,
		payload.SessionID,
		manager,
		payload.Headless,
		payload.URL,
	)
	if err != nil {
		return nil, fmt.Errorf("세션 생성 실패: %w", err)
	}

	// 매니저 시작
	if err := manager.Start(ctx); err != nil {
		// 시작 실패 시 세션 정리
		_ = h.sessionMgr.EndSession(payload.SessionID)
		return nil, fmt.Errorf("agent-browser 시작 실패: %w", err)
	}

	// browser_session_ready 응답
	ready := &BrowserSessionPayload{
		ExecutionID: payload.ExecutionID,
		SessionID:   payload.SessionID,
		URL:         payload.URL,
		Headless:    payload.Headless,
		Status:      "ready",
	}

	h.logger.Info().
		Str("session_id", payload.SessionID).
		Msg("agent-browser 세션 시작 완료")

	return ready, nil
}

// HandleAction은 browser_action 메시지를 처리하고 결과를 반환한다.
func (h *Handler) HandleAction(ctx context.Context, payload BrowserActionPayload) (*BrowserResultPayload, error) {
	start := time.Now()

	result := &BrowserResultPayload{
		ExecutionID: payload.ExecutionID,
		SessionID:   payload.SessionID,
	}

	// 세션 조회
	session, exists := h.sessionMgr.GetSession(payload.SessionID)
	if !exists {
		result.Success = false
		result.Error = fmt.Sprintf("세션 %s를 찾을 수 없습니다", payload.SessionID)
		result.DurationMs = time.Since(start).Milliseconds()
		return result, nil
	}

	// 유휴 타이머 갱신
	h.sessionMgr.TouchSession(payload.SessionID)

	// 매니저 상태 확인
	if session.Manager == nil || !session.Manager.IsReady() {
		result.Success = false
		result.Error = "agent-browser가 준비되지 않았습니다"
		result.DurationMs = time.Since(start).Milliseconds()
		return result, nil
	}

	// 명령 실행
	execResult, err := session.Manager.Execute(ctx, payload.Command, payload.Ref, payload.Params)
	if err != nil {
		result.Success = false
		result.Error = err.Error()
		result.DurationMs = time.Since(start).Milliseconds()
		return result, nil
	}

	result.Success = true
	result.Output = execResult.Output
	result.Snapshot = execResult.Snapshot
	result.Screenshot = execResult.Screenshot
	result.DurationMs = time.Since(start).Milliseconds()

	// 실행 결과에 에러가 있는 경우 (명령은 성공했지만 결과에 에러가 포함된 경우)
	if execResult.Error != "" {
		result.Success = false
		result.Error = execResult.Error
	}

	h.logger.Debug().
		Str("session_id", payload.SessionID).
		Str("command", payload.Command).
		Int64("duration_ms", result.DurationMs).
		Bool("success", result.Success).
		Msg("agent-browser 액션 실행 완료")

	return result, nil
}

// HandleSessionEnd는 browser_session_end 메시지를 처리한다.
func (h *Handler) HandleSessionEnd(ctx context.Context, payload BrowserSessionPayload) error {
	h.logger.Info().
		Str("session_id", payload.SessionID).
		Msg("agent-browser 세션 종료 요청")

	if err := h.sessionMgr.EndSession(payload.SessionID); err != nil {
		return fmt.Errorf("세션 종료 실패: %w", err)
	}

	h.logger.Info().
		Str("session_id", payload.SessionID).
		Msg("agent-browser 세션 종료 완료")

	return nil
}

// GetActiveSessions는 현재 활성 세션 목록을 반환한다.
// WebSocket 재연결 시 세션 상태 복원에 사용된다.
func (h *Handler) GetActiveSessions() []*Session {
	return h.sessionMgr.GetActiveSessions()
}
