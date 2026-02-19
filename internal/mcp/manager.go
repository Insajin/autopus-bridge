package mcp

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// Manager는 MCP 서버 프로세스의 라이프사이클을 관리합니다.
// SPEC-SKILL-V2-001 Block D: Dynamic MCP Provisioning
type Manager struct {
	config    *LocalConfig
	processes map[string]*ProcessInfo
	mu        sync.RWMutex
}

// NewManager는 새로운 MCP Manager를 생성합니다.
func NewManager(config *LocalConfig) *Manager {
	return &Manager{
		config:    config,
		processes: make(map[string]*ProcessInfo),
	}
}

// Start는 이름으로 MCP 서버를 시작합니다.
// 서버 설정은 로컬 config 또는 서버에서 전달받은 설정을 사용합니다.
func (m *Manager) Start(ctx context.Context, name string, overrideCfg *ServerConfig) (*ProcessInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 이미 실행 중인지 확인
	if existing, ok := m.processes[name]; ok && existing.IsRunning() {
		return existing, nil
	}

	// 설정 결정: override > 로컬 config
	var cfg ServerConfig
	if overrideCfg != nil {
		cfg = *overrideCfg
		cfg.Name = name
	} else if localCfg, ok := m.config.GetServerConfig(name); ok {
		cfg = *localCfg
		cfg.Name = name
	} else {
		return nil, fmt.Errorf("MCP 서버 %q 설정을 찾을 수 없음", name)
	}

	proc, err := startProcess(ctx, cfg)
	if err != nil {
		return nil, err
	}

	m.processes[name] = proc
	log.Info().
		Str("name", name).
		Int("pid", proc.PID).
		Msg("[mcp] 서버 시작 완료")

	return proc, nil
}

// Stop은 이름으로 MCP 서버를 중지합니다.
func (m *Manager) Stop(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	proc, ok := m.processes[name]
	if !ok {
		return fmt.Errorf("MCP 서버 %q가 실행 중이 아님", name)
	}

	if err := proc.Stop(); err != nil {
		return fmt.Errorf("MCP 서버 %q 중지 실패: %w", name, err)
	}

	delete(m.processes, name)
	return nil
}

// ForceStop은 MCP 서버를 강제 종료합니다.
func (m *Manager) ForceStop(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	proc, ok := m.processes[name]
	if !ok {
		return nil
	}

	if proc.cmd != nil && proc.cmd.Process != nil {
		_ = proc.cmd.Process.Kill()
	}
	proc.cleanup()

	delete(m.processes, name)
	return nil
}

// StopAll은 모든 실행 중인 MCP 서버를 중지합니다.
func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, proc := range m.processes {
		log.Info().Str("name", name).Msg("[mcp] 모든 서버 중지 중")
		if err := proc.Stop(); err != nil {
			log.Error().Err(err).Str("name", name).Msg("[mcp] 서버 중지 실패")
		}
	}

	m.processes = make(map[string]*ProcessInfo)
}

// HealthCheck는 특정 MCP 서버의 상태를 확인합니다.
func (m *Manager) HealthCheck(name string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	proc, ok := m.processes[name]
	if !ok {
		return false, fmt.Errorf("MCP 서버 %q가 등록되지 않음", name)
	}

	return proc.IsRunning(), nil
}

// ListRunning은 현재 실행 중인 MCP 서버 목록을 반환합니다.
func (m *Manager) ListRunning() []ProcessInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []ProcessInfo
	for _, proc := range m.processes {
		if proc.IsRunning() {
			result = append(result, ProcessInfo{
				Name:      proc.Name,
				PID:       proc.PID,
				Port:      proc.Port,
				Command:   proc.Command,
				StartedAt: proc.StartedAt,
			})
		}
	}
	return result
}

// IsAvailable은 로컬 설정에 해당 MCP 서버가 있는지 확인합니다.
func (m *Manager) IsAvailable(name string) bool {
	if _, ok := m.config.GetServerConfig(name); ok {
		return true
	}
	return false
}

// GetProcessInfo는 실행 중인 MCP 서버의 프로세스 정보를 반환합니다.
func (m *Manager) GetProcessInfo(name string) (*ProcessInfo, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	proc, ok := m.processes[name]
	if !ok {
		return nil, false
	}
	return proc, true
}

// StartupTimeout은 MCP 서버 시작에 사용되는 최대 대기 시간을 반환합니다.
func (m *Manager) StartupTimeout() time.Duration {
	return 30 * time.Second
}
