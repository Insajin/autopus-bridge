package agentbrowser

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// PendingResult는 아직 WebSocket으로 전송되지 않은 액션 결과이다.
// 재연결 후 미전송 결과를 재전송할 수 있다 (REQ-M3-04).
type PendingResult struct {
	Payload   interface{}
	CreatedAt time.Time
}

// Session은 활성 agent-browser 세션을 나타낸다.
type Session struct {
	ID          string
	ExecutionID string
	Manager     *Manager
	CreatedAt   time.Time
	LastActiveAt time.Time
	URL         string
	Headless    bool

	// CICDMode는 CI/CD 환경에서 생성된 세션인지 여부이다 (REQ-M5-04).
	CICDMode bool
	// CICDProvider는 감지된 CI/CD 제공자 이름이다 (예: "github-actions").
	CICDProvider string

	// pendingResults는 전송 대기 중인 액션 결과 큐이다 (REQ-M3-04).
	pendingResults []PendingResult
	mu             sync.Mutex
}

// QueueResult는 전송 대기 큐에 결과를 추가한다.
func (s *Session) QueueResult(payload interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pendingResults = append(s.pendingResults, PendingResult{
		Payload:   payload,
		CreatedAt: time.Now(),
	})
}

// DrainPendingResults는 대기 중인 모든 결과를 반환하고 큐를 비운다.
func (s *Session) DrainPendingResults() []PendingResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	results := s.pendingResults
	s.pendingResults = nil
	return results
}

// PendingResultCount는 대기 중인 결과의 수를 반환한다.
func (s *Session) PendingResultCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.pendingResults)
}

// SessionManager는 agent-browser 세션을 관리한다.
type SessionManager struct {
	sessions map[string]*Session
	mu       sync.RWMutex
	logger   zerolog.Logger

	// maxIdle은 유휴 대기 전환 시간이다 (10분).
	maxIdle time.Duration
	// maxActive는 최대 활성 세션 시간이다 (2시간).
	maxActive time.Duration
	// maxConcurrent는 최대 동시 세션 수이다 (3).
	maxConcurrent int

	// cicdConfig는 CI/CD 환경 설정이다 (REQ-M5-04).
	cicdConfig CICDConfig
}

// NewSessionManager는 새로운 SessionManager를 생성한다.
func NewSessionManager(logger zerolog.Logger) *SessionManager {
	return &SessionManager{
		sessions:      make(map[string]*Session),
		logger:        logger,
		maxIdle:       10 * time.Minute,
		maxActive:     2 * time.Hour,
		maxConcurrent: 3,
	}
}

// SetCICDConfig는 SessionManager에 CI/CD 환경 설정을 적용한다 (REQ-M5-04).
func (sm *SessionManager) SetCICDConfig(config CICDConfig) {
	sm.cicdConfig = config
}

// CreateSession은 새로운 agent-browser 세션을 생성한다.
// 최대 동시 세션 수에 도달하면 오류를 반환한다.
// CI/CD 모드가 활성화된 경우 headless가 요청값과 무관하게 강제 설정된다.
func (sm *SessionManager) CreateSession(executionID, sessionID string, manager *Manager, headless bool, initialURL string) (*Session, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// 동시 세션 수 제한 확인
	if len(sm.sessions) >= sm.maxConcurrent {
		return nil, fmt.Errorf("최대 동시 세션 수에 도달했습니다 (%d)", sm.maxConcurrent)
	}

	// 중복 세션 확인
	if _, exists := sm.sessions[sessionID]; exists {
		return nil, fmt.Errorf("세션 %s가 이미 존재합니다", sessionID)
	}

	// CI/CD 모드에서는 headless를 강제 활성화한다 (REQ-M5-04).
	effectiveHeadless := headless
	if sm.cicdConfig.Headless {
		effectiveHeadless = true
	}

	now := time.Now()
	session := &Session{
		ID:           sessionID,
		ExecutionID:  executionID,
		Manager:      manager,
		CreatedAt:    now,
		LastActiveAt: now,
		URL:          initialURL,
		Headless:     effectiveHeadless,
		CICDMode:     sm.cicdConfig.IsEnabled(),
		CICDProvider: DetectedCICDProvider(),
	}

	sm.sessions[sessionID] = session

	sm.logger.Info().
		Str("session_id", sessionID).
		Str("execution_id", executionID).
		Bool("headless", effectiveHeadless).
		Bool("cicd_mode", session.CICDMode).
		Str("cicd_provider", session.CICDProvider).
		Msg("agent-browser 세션 생성")

	return session, nil
}

// GetSession은 주어진 ID의 세션을 반환한다.
func (sm *SessionManager) GetSession(sessionID string) (*Session, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, exists := sm.sessions[sessionID]
	return session, exists
}

// EndSession은 주어진 ID의 세션을 종료하고 매니저를 중지한다.
func (sm *SessionManager) EndSession(sessionID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return fmt.Errorf("세션 %s를 찾을 수 없습니다", sessionID)
	}

	// 매니저 중지
	if session.Manager != nil {
		if err := session.Manager.Stop(); err != nil {
			sm.logger.Warn().
				Err(err).
				Str("session_id", sessionID).
				Msg("agent-browser 매니저 중지 실패")
		}
	}

	delete(sm.sessions, sessionID)

	sm.logger.Info().
		Str("session_id", sessionID).
		Msg("agent-browser 세션 종료")

	return nil
}

// TouchSession은 세션의 유휴 타이머를 갱신한다.
func (sm *SessionManager) TouchSession(sessionID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if session, exists := sm.sessions[sessionID]; exists {
		session.LastActiveAt = time.Now()
	}
}

// GetActiveSessions는 현재 활성 세션 목록을 반환한다.
func (sm *SessionManager) GetActiveSessions() []*Session {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	sessions := make([]*Session, 0, len(sm.sessions))
	for _, s := range sm.sessions {
		sessions = append(sessions, s)
	}
	return sessions
}

// ActiveCount는 활성 세션 수를 반환한다.
func (sm *SessionManager) ActiveCount() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.sessions)
}

// StartCleanupLoop은 주기적으로 유휴/만료된 세션을 제거하는 백그라운드 goroutine을 시작한다.
// 컨텍스트가 취소되면 중지된다.
func (sm *SessionManager) StartCleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			sm.closeAllSessions()
			return
		case <-ticker.C:
			sm.cleanupExpiredSessions()
		}
	}
}

// cleanupExpiredSessions는 유휴/활성 타임아웃을 초과한 세션을 제거한다.
func (sm *SessionManager) cleanupExpiredSessions() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	now := time.Now()
	for id, session := range sm.sessions {
		idleExpired := now.Sub(session.LastActiveAt) > sm.maxIdle
		activeExpired := now.Sub(session.CreatedAt) > sm.maxActive

		if idleExpired || activeExpired {
			reason := "idle timeout"
			if activeExpired {
				reason = "max active time exceeded"
			}
			sm.logger.Info().
				Str("session_id", id).
				Str("reason", reason).
				Msg("만료된 agent-browser 세션 정리")

			if session.Manager != nil {
				if err := session.Manager.Stop(); err != nil {
					sm.logger.Warn().
						Err(err).
						Str("session_id", id).
						Msg("매니저 중지 실패")
				}
			}
			delete(sm.sessions, id)
		}
	}
}

// closeAllSessions는 모든 활성 세션을 종료한다. 셧다운 시 호출된다.
func (sm *SessionManager) closeAllSessions() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for id, session := range sm.sessions {
		sm.logger.Info().
			Str("session_id", id).
			Msg("agent-browser 세션 셧다운")
		if session.Manager != nil {
			if err := session.Manager.Stop(); err != nil {
				sm.logger.Warn().
					Err(err).
					Str("session_id", id).
					Msg("매니저 중지 실패")
			}
		}
		delete(sm.sessions, id)
	}
}
