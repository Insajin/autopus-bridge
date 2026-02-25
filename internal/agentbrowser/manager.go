package agentbrowser

import (
	"context"
	"encoding/base64"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

const (
	// maxRestartAttempts는 크래시 시 최대 자동 재시작 횟수이다 (REQ-M1-05).
	maxRestartAttempts = 3
	// standbyTimeout은 유휴 대기 전환 시간이다 (REQ-M1-07).
	standbyTimeout = 10 * time.Minute
)

// Manager는 agent-browser CLI 데몬의 라이프사이클을 관리한다.
type Manager struct {
	logger   zerolog.Logger
	executor *CommandExecutor

	// state는 현재 데몬 상태이다.
	state ManagerState
	// mu는 상태 접근을 보호하는 뮤텍스이다.
	mu sync.Mutex

	// restartCount는 연속 자동 재시작 횟수이다.
	restartCount int
	// lastActiveAt은 마지막 명령 실행 시각이다.
	lastActiveAt time.Time
	// onStatusChange는 상태 변경 콜백이다.
	onStatusChange StatusCallback

	// headless는 브라우저 헤드리스 모드 여부이다.
	headless bool
	// initialURL은 세션 시작 시 이동할 초기 URL이다.
	initialURL string

	// cicdConfig는 CI/CD 환경 설정이다 (REQ-M5-04).
	cicdConfig CICDConfig

	// cancel은 데몬 goroutine 취소 함수이다.
	cancel context.CancelFunc
}

// ManagerOption은 Manager 설정을 위한 함수형 옵션이다.
type ManagerOption func(*Manager)

// WithStatusCallback은 상태 변경 콜백을 설정한다.
func WithStatusCallback(cb StatusCallback) ManagerOption {
	return func(m *Manager) {
		m.onStatusChange = cb
	}
}

// WithHeadless는 헤드리스 모드를 설정한다.
func WithHeadless(headless bool) ManagerOption {
	return func(m *Manager) {
		m.headless = headless
	}
}

// WithInitialURL은 초기 URL을 설정한다.
func WithInitialURL(url string) ManagerOption {
	return func(m *Manager) {
		m.initialURL = url
	}
}

// WithCICDConfig는 CI/CD 환경 설정을 적용한다 (REQ-M5-04).
// CI/CD 모드가 활성화되면 headless가 자동으로 강제 설정된다.
func WithCICDConfig(config CICDConfig) ManagerOption {
	return func(m *Manager) {
		m.cicdConfig = config
		// CI/CD 모드에서는 headless를 강제 활성화한다.
		if config.Headless {
			m.headless = true
		}
	}
}

// NewManager는 새로운 Manager를 생성한다.
func NewManager(logger zerolog.Logger, executor *CommandExecutor, opts ...ManagerOption) *Manager {
	m := &Manager{
		logger:       logger,
		executor:     executor,
		state:        StateStopped,
		lastActiveAt: time.Now(),
	}
	for _, opt := range opts {
		opt(m)
	}

	// CI/CD 설정을 executor에 전파한다.
	if m.cicdConfig.IsEnabled() {
		m.executor.SetCICDConfig(m.cicdConfig)
	}

	return m
}

// CICDConfig는 현재 CI/CD 설정을 반환한다.
func (m *Manager) CICDConfig() CICDConfig {
	return m.cicdConfig
}

// Start는 agent-browser 데몬을 시작한다.
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.state == StateReady || m.state == StateBusy {
		return fmt.Errorf("agent-browser가 이미 실행 중입니다 (state=%s)", m.state)
	}

	m.setState(StateStarting)
	m.restartCount = 0

	// 초기 URL이 있으면 open 명령으로 브라우저 시작
	if m.initialURL != "" {
		_, err := m.executor.Execute(ctx, "open", m.initialURL)
		if err != nil {
			m.setState(StateError)
			return fmt.Errorf("agent-browser 시작 실패: %w", err)
		}
	}

	m.setState(StateReady)
	m.lastActiveAt = time.Now()

	m.logger.Info().
		Bool("headless", m.headless).
		Str("initial_url", m.initialURL).
		Bool("cicd_mode", m.cicdConfig.IsEnabled()).
		Msg("agent-browser 데몬 시작 완료")

	return nil
}

// Stop은 agent-browser 데몬을 정상적으로 종료한다.
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.state == StateStopped {
		return nil
	}

	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}

	m.setState(StateStopped)
	m.logger.Info().Msg("agent-browser 데몬 중지")

	return nil
}

// Execute는 agent-browser 명령을 실행하고 결과를 반환한다.
func (m *Manager) Execute(ctx context.Context, command string, ref *int, params map[string]interface{}) (*ExecutionResult, error) {
	m.mu.Lock()
	if m.state != StateReady && m.state != StateStandby {
		state := m.state
		m.mu.Unlock()
		return nil, fmt.Errorf("agent-browser가 준비되지 않았습니다 (state=%s)", state)
	}
	m.setState(StateBusy)
	m.lastActiveAt = time.Now()
	m.mu.Unlock()

	// 명령 실행
	payload := BrowserActionPayload{
		Command: command,
		Ref:     ref,
		Params:  params,
	}
	cmdResult, err := m.executor.ExecuteFromPayload(ctx, payload)

	m.mu.Lock()
	defer m.mu.Unlock()

	// 실행 후 상태 복원
	if m.state == StateBusy {
		m.setState(StateReady)
	}

	if err != nil {
		m.restartCount++
		if m.restartCount >= maxRestartAttempts {
			m.setState(StateError)
			return nil, fmt.Errorf("agent-browser 명령 실행 실패 (최대 재시작 횟수 초과): %w", err)
		}
		return nil, fmt.Errorf("agent-browser 명령 실행 실패: %w", err)
	}

	// 성공 시 재시작 카운터 초기화
	m.restartCount = 0

	// CommandResult를 ExecutionResult로 변환
	execResult := &ExecutionResult{
		Output:     cmdResult.Output,
		Snapshot:   cmdResult.Snapshot,
		Error:      cmdResult.Error,
		DurationMs: cmdResult.DurationMs,
	}

	// 스크린샷 바이너리를 base64로 인코딩
	if len(cmdResult.Screenshot) > 0 {
		execResult.Screenshot = base64.StdEncoding.EncodeToString(cmdResult.Screenshot)
	}

	return execResult, nil
}

// State는 현재 매니저 상태를 반환한다.
func (m *Manager) State() ManagerState {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state
}

// IsReady는 매니저가 명령을 받을 준비가 되었는지 반환한다.
func (m *Manager) IsReady() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state == StateReady || m.state == StateStandby
}

// LastActiveAt은 마지막 명령 실행 시각을 반환한다.
func (m *Manager) LastActiveAt() time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastActiveAt
}

// RestartCount는 연속 자동 재시작 횟수를 반환한다.
func (m *Manager) RestartCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.restartCount
}

// EnterStandby는 유휴 대기 모드로 전환한다 (REQ-M1-07).
// 브라우저를 중지하되 데몬은 유지한다.
func (m *Manager) EnterStandby() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.state == StateReady {
		m.setState(StateStandby)
		m.logger.Info().Msg("agent-browser 유휴 대기 모드 전환")
	}
}

// CheckStandby는 유휴 시간이 standbyTimeout을 초과했는지 확인하고,
// 초과 시 대기 모드로 전환한다.
func (m *Manager) CheckStandby() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.state != StateReady {
		return false
	}

	if time.Since(m.lastActiveAt) > standbyTimeout {
		m.setState(StateStandby)
		m.logger.Info().
			Dur("idle_duration", time.Since(m.lastActiveAt)).
			Msg("유휴 시간 초과로 대기 모드 전환")
		return true
	}

	return false
}

// setState는 내부 상태를 변경하고 콜백을 호출한다.
// 호출자가 mu를 이미 잠근 상태여야 한다.
func (m *Manager) setState(state ManagerState) {
	oldState := m.state
	m.state = state
	if m.onStatusChange != nil && oldState != state {
		// 콜백은 락 밖에서 호출하면 데드락 위험이 있으므로, goroutine으로 실행
		cb := m.onStatusChange
		go cb(state, fmt.Sprintf("상태 변경: %s -> %s", oldState, state))
	}
}
