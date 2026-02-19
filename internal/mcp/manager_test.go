package mcp

import (
	"context"
	"testing"
	"time"
)

func newTestConfig(servers map[string]ServerConfig) *LocalConfig {
	if servers == nil {
		servers = make(map[string]ServerConfig)
	}
	return &LocalConfig{Servers: servers}
}

func TestNewManager(t *testing.T) {
	cfg := newTestConfig(nil)
	m := NewManager(cfg)
	if m == nil {
		t.Fatal("NewManager() returned nil")
	}
	if m.processes == nil {
		t.Fatal("processes map is nil")
	}
}

func TestManager_StartWithOverride(t *testing.T) {
	cfg := newTestConfig(nil)
	m := NewManager(cfg)

	override := &ServerConfig{
		Command: "sleep",
		Args:    []string{"10"},
	}

	proc, err := m.Start(context.Background(), "test-server", override)
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer m.ForceStop("test-server") //nolint:errcheck

	if proc.Name != "test-server" {
		t.Errorf("Name = %q, want %q", proc.Name, "test-server")
	}
	if proc.PID <= 0 {
		t.Errorf("PID = %d, want > 0", proc.PID)
	}
}

func TestManager_StartWithLocalConfig(t *testing.T) {
	cfg := newTestConfig(map[string]ServerConfig{
		"local-srv": {
			Command: "sleep",
			Args:    []string{"10"},
		},
	})
	m := NewManager(cfg)

	proc, err := m.Start(context.Background(), "local-srv", nil)
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer m.ForceStop("local-srv") //nolint:errcheck

	if proc.Name != "local-srv" {
		t.Errorf("Name = %q, want %q", proc.Name, "local-srv")
	}
}

func TestManager_StartNotFound(t *testing.T) {
	cfg := newTestConfig(nil)
	m := NewManager(cfg)

	_, err := m.Start(context.Background(), "nonexistent", nil)
	if err == nil {
		t.Fatal("Start() expected error for missing config, got nil")
	}
}

func TestManager_StartIdempotent(t *testing.T) {
	cfg := newTestConfig(nil)
	m := NewManager(cfg)

	override := &ServerConfig{
		Command: "sleep",
		Args:    []string{"10"},
	}

	proc1, err := m.Start(context.Background(), "srv", override)
	if err != nil {
		t.Fatalf("Start() first call error: %v", err)
	}
	defer m.ForceStop("srv") //nolint:errcheck

	proc2, err := m.Start(context.Background(), "srv", override)
	if err != nil {
		t.Fatalf("Start() second call error: %v", err)
	}

	if proc1.PID != proc2.PID {
		t.Errorf("Start() returned different PID on second call: %d vs %d", proc1.PID, proc2.PID)
	}
}

func TestManager_Stop(t *testing.T) {
	cfg := newTestConfig(nil)
	m := NewManager(cfg)

	override := &ServerConfig{Command: "sleep", Args: []string{"10"}}
	_, err := m.Start(context.Background(), "srv", override)
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	if err := m.Stop("srv"); err != nil {
		t.Errorf("Stop() error: %v", err)
	}

	// 두 번째 Stop은 에러 반환
	if err := m.Stop("srv"); err == nil {
		t.Error("Stop() expected error for already stopped server, got nil")
	}
}

func TestManager_StopNotRunning(t *testing.T) {
	cfg := newTestConfig(nil)
	m := NewManager(cfg)

	err := m.Stop("nonexistent")
	if err == nil {
		t.Fatal("Stop() expected error for non-running server, got nil")
	}
}

func TestManager_ForceStop(t *testing.T) {
	cfg := newTestConfig(nil)
	m := NewManager(cfg)

	override := &ServerConfig{Command: "sleep", Args: []string{"10"}}
	_, err := m.Start(context.Background(), "srv", override)
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	if err := m.ForceStop("srv"); err != nil {
		t.Errorf("ForceStop() error: %v", err)
	}

	// ForceStop on non-existing should return nil (no error)
	if err := m.ForceStop("srv"); err != nil {
		t.Errorf("ForceStop() on non-existing error: %v", err)
	}
}

func TestManager_StopAll(t *testing.T) {
	cfg := newTestConfig(nil)
	m := NewManager(cfg)

	for _, name := range []string{"srv1", "srv2", "srv3"} {
		override := &ServerConfig{Command: "sleep", Args: []string{"10"}}
		if _, err := m.Start(context.Background(), name, override); err != nil {
			t.Fatalf("Start(%s) error: %v", name, err)
		}
	}

	running := m.ListRunning()
	if len(running) != 3 {
		t.Fatalf("ListRunning() = %d, want 3", len(running))
	}

	m.StopAll()

	running = m.ListRunning()
	if len(running) != 0 {
		t.Errorf("ListRunning() after StopAll = %d, want 0", len(running))
	}
}

func TestManager_HealthCheck(t *testing.T) {
	cfg := newTestConfig(nil)
	m := NewManager(cfg)

	override := &ServerConfig{Command: "sleep", Args: []string{"10"}}
	_, err := m.Start(context.Background(), "srv", override)
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer m.ForceStop("srv") //nolint:errcheck

	healthy, err := m.HealthCheck("srv")
	if err != nil {
		t.Fatalf("HealthCheck() error: %v", err)
	}
	if !healthy {
		t.Error("HealthCheck() = false for running server, want true")
	}

	// 등록되지 않은 서버
	_, err = m.HealthCheck("nonexistent")
	if err == nil {
		t.Error("HealthCheck() expected error for non-registered server, got nil")
	}
}

func TestManager_ListRunning(t *testing.T) {
	cfg := newTestConfig(nil)
	m := NewManager(cfg)

	// 빈 상태
	running := m.ListRunning()
	if len(running) != 0 {
		t.Errorf("ListRunning() = %d for empty, want 0", len(running))
	}

	override := &ServerConfig{Command: "sleep", Args: []string{"10"}}
	if _, err := m.Start(context.Background(), "srv1", override); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer m.ForceStop("srv1") //nolint:errcheck

	running = m.ListRunning()
	if len(running) != 1 {
		t.Fatalf("ListRunning() = %d, want 1", len(running))
	}
	if running[0].Name != "srv1" {
		t.Errorf("Name = %q, want %q", running[0].Name, "srv1")
	}
}

func TestManager_IsAvailable(t *testing.T) {
	cfg := newTestConfig(map[string]ServerConfig{
		"configured": {Command: "echo"},
	})
	m := NewManager(cfg)

	if !m.IsAvailable("configured") {
		t.Error("IsAvailable() = false for configured server, want true")
	}
	if m.IsAvailable("not-configured") {
		t.Error("IsAvailable() = true for non-configured server, want false")
	}
}

func TestManager_GetProcessInfo(t *testing.T) {
	cfg := newTestConfig(nil)
	m := NewManager(cfg)

	override := &ServerConfig{Command: "sleep", Args: []string{"10"}}
	if _, err := m.Start(context.Background(), "srv", override); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer m.ForceStop("srv") //nolint:errcheck

	proc, ok := m.GetProcessInfo("srv")
	if !ok {
		t.Fatal("GetProcessInfo() returned false for running server")
	}
	if proc.Name != "srv" {
		t.Errorf("Name = %q, want %q", proc.Name, "srv")
	}

	_, ok = m.GetProcessInfo("nonexistent")
	if ok {
		t.Error("GetProcessInfo() returned true for non-existing server")
	}
}

func TestManager_StartupTimeout(t *testing.T) {
	cfg := newTestConfig(nil)
	m := NewManager(cfg)

	timeout := m.StartupTimeout()
	if timeout != 30*time.Second {
		t.Errorf("StartupTimeout() = %v, want 30s", timeout)
	}
}
