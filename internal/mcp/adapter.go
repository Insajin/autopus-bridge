// Package mcp는 Local Agent Bridge의 MCP 서버 프로세스 관리를 제공합니다.
// adapter.go는 Manager를 websocket.MCPServerStarter 인터페이스에 맞추는 어댑터입니다.
// SPEC-SKILL-V2-001 Block D: Dynamic MCP Provisioning
package mcp

import "context"

// StarterAdapter는 Manager를 MCPServerStarter 인터페이스에 맞추는 어댑터입니다.
// websocket 패키지에서 정의한 인터페이스를 직접 import하지 않고,
// 동일한 시그니처로 구현하여 인터페이스 분리 원칙을 따릅니다.
type StarterAdapter struct {
	manager *Manager
}

// NewStarterAdapter는 새로운 StarterAdapter를 생성합니다.
func NewStarterAdapter(manager *Manager) *StarterAdapter {
	return &StarterAdapter{manager: manager}
}

// StartServer는 MCP 서버를 시작합니다.
// 서버 설정을 ServerConfig로 변환하여 Manager.Start를 호출합니다.
func (a *StarterAdapter) StartServer(ctx context.Context, name, command string, args []string, env map[string]string, workingDir string) (pid int, err error) {
	cfg := &ServerConfig{
		Name:       name,
		Command:    command,
		Args:       args,
		Env:        env,
		WorkingDir: workingDir,
	}

	proc, err := a.manager.Start(ctx, name, cfg)
	if err != nil {
		return 0, err
	}

	return proc.PID, nil
}

// StopServer는 MCP 서버를 중지합니다.
// force가 true이면 ForceStop, false이면 Stop을 호출합니다.
func (a *StarterAdapter) StopServer(name string, force bool) error {
	if force {
		return a.manager.ForceStop(name)
	}
	return a.manager.Stop(name)
}
