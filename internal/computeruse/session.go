package computeruse

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// PendingResult represents an action result that has not yet been
// successfully sent over the WebSocket connection. Results are queued
// before sending so they can be resent after a reconnection (REQ-M3-04).
type PendingResult struct {
	Payload   interface{}
	CreatedAt time.Time
}

// Session represents an active computer use browser session.
type Session struct {
	ID           string
	ExecutionID  string
	Backend      BrowserBackend   // 브라우저 백엔드 (로컬 또는 컨테이너)
	ContainerID  string           // 컨테이너 모드일 때 Docker 컨테이너 ID (빈 문자열이면 로컬)
	CreatedAt    time.Time
	LastActiveAt time.Time
	URL          string
	ViewportW    int
	ViewportH    int
	Headless     bool

	// pendingResults stores action results waiting for successful delivery (REQ-M3-04).
	pendingResults []PendingResult
	mu             sync.Mutex
}

// SessionManager manages computer use browser sessions.
// REQ-M2-09: 30-minute idle timeout.
type SessionManager struct {
	sessions        map[string]*Session
	mu              sync.RWMutex
	maxIdle         time.Duration // 30 minutes
	maxActive       time.Duration // 2 hours
	maxPerWorkspace int           // 2
}

// NewSessionManager creates a new SessionManager with default timeouts.
func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions:        make(map[string]*Session),
		maxIdle:         30 * time.Minute,
		maxActive:       2 * time.Hour,
		maxPerWorkspace: 2,
	}
}

// CreateSession creates a new browser session with the given parameters.
// Returns an error if the maximum number of concurrent sessions is reached.
func (sm *SessionManager) CreateSession(executionID, sessionID string, viewportW, viewportH int, headless bool, initialURL string) (*Session, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Check concurrent session limit.
	if len(sm.sessions) >= sm.maxPerWorkspace {
		return nil, fmt.Errorf("maximum concurrent sessions reached (%d)", sm.maxPerWorkspace)
	}

	// Check if session already exists.
	if _, exists := sm.sessions[sessionID]; exists {
		return nil, fmt.Errorf("session %s already exists", sessionID)
	}

	// Apply default viewport if not specified.
	if viewportW <= 0 {
		viewportW = 1280
	}
	if viewportH <= 0 {
		viewportH = 720
	}

	now := time.Now()
	session := &Session{
		ID:           sessionID,
		ExecutionID:  executionID,
		Backend:      NewBrowserManager(viewportW, viewportH, headless),
		CreatedAt:    now,
		LastActiveAt: now,
		URL:          initialURL,
		ViewportW:    viewportW,
		ViewportH:    viewportH,
		Headless:     headless,
	}

	sm.sessions[sessionID] = session

	return session, nil
}

// GetSession returns the session with the given ID, if it exists.
func (sm *SessionManager) GetSession(sessionID string) (*Session, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, exists := sm.sessions[sessionID]
	return session, exists
}

// EndSession terminates the session with the given ID and closes its browser.
func (sm *SessionManager) EndSession(sessionID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session %s not found", sessionID)
	}

	// 브라우저 종료
	if session.Backend != nil {
		if err := session.Backend.Close(); err != nil {
			log.Printf("[computer-use] failed to close browser for session %s: %v", sessionID, err)
		}
	}

	delete(sm.sessions, sessionID)
	return nil
}

// TouchSession resets the idle timer for the given session.
func (sm *SessionManager) TouchSession(sessionID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if session, exists := sm.sessions[sessionID]; exists {
		session.LastActiveAt = time.Now()
	}
}

// QueueResult stores an action result that needs to be sent over the
// WebSocket connection. The result stays queued until DrainPendingResults
// is called after a successful send (REQ-M3-04).
func (s *Session) QueueResult(payload interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pendingResults = append(s.pendingResults, PendingResult{
		Payload:   payload,
		CreatedAt: time.Now(),
	})
}

// DrainPendingResults returns all pending results and clears the queue.
// Called after results have been successfully sent (REQ-M3-04).
func (s *Session) DrainPendingResults() []PendingResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	results := s.pendingResults
	s.pendingResults = nil
	return results
}

// PendingResultCount returns the number of pending results in the queue.
func (s *Session) PendingResultCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.pendingResults)
}

// GetActiveSessions returns all currently active sessions.
// Used during reconnection to restore session state (REQ-M3-04).
func (sm *SessionManager) GetActiveSessions() []*Session {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	sessions := make([]*Session, 0, len(sm.sessions))
	for _, s := range sm.sessions {
		sessions = append(sessions, s)
	}
	return sessions
}

// ActiveCount returns the number of active sessions.
func (sm *SessionManager) ActiveCount() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.sessions)
}

// StartCleanupLoop starts a background goroutine that periodically removes
// idle and expired sessions. It stops when the context is cancelled.
func (sm *SessionManager) StartCleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Clean up all remaining sessions on shutdown.
			sm.closeAllSessions()
			return
		case <-ticker.C:
			sm.cleanupExpiredSessions()
		}
	}
}

// cleanupExpiredSessions removes sessions that have exceeded idle or active timeouts.
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
			log.Printf("[computer-use] closing session %s: %s", id, reason)

			if session.Backend != nil {
				if err := session.Backend.Close(); err != nil {
					log.Printf("[computer-use] failed to close browser for session %s: %v", id, err)
				}
			}
			delete(sm.sessions, id)
		}
	}
}

// closeAllSessions closes all active sessions. Called during shutdown.
func (sm *SessionManager) closeAllSessions() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for id, session := range sm.sessions {
		log.Printf("[computer-use] shutting down session %s", id)
		if session.Backend != nil {
			if err := session.Backend.Close(); err != nil {
				log.Printf("[computer-use] failed to close browser for session %s: %v", id, err)
			}
		}
		delete(sm.sessions, id)
	}
}
