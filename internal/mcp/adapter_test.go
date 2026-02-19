package mcp

import (
	"context"
	"testing"
)

func TestNewStarterAdapter(t *testing.T) {
	cfg := newTestConfig(nil)
	m := NewManager(cfg)
	adapter := NewStarterAdapter(m)
	if adapter == nil {
		t.Fatal("NewStarterAdapter() returned nil")
	}
	if adapter.manager != m {
		t.Error("adapter.manager does not match input manager")
	}
}

func TestStarterAdapter_StartServer(t *testing.T) {
	cfg := newTestConfig(nil)
	m := NewManager(cfg)
	adapter := NewStarterAdapter(m)

	pid, err := adapter.StartServer(
		context.Background(),
		"test-srv",
		"sleep",
		[]string{"10"},
		map[string]string{"KEY": "val"},
		"",
	)
	if err != nil {
		t.Fatalf("StartServer() error: %v", err)
	}
	defer func() { _ = m.ForceStop("test-srv") }()

	if pid <= 0 {
		t.Errorf("StartServer() pid = %d, want > 0", pid)
	}

	// 프로세스가 Manager에 등록되었는지 확인
	proc, ok := m.GetProcessInfo("test-srv")
	if !ok {
		t.Fatal("Server not found in manager after StartServer()")
	}
	if proc.PID != pid {
		t.Errorf("Manager PID = %d, StartServer returned %d", proc.PID, pid)
	}
}

func TestStarterAdapter_StartServer_Error(t *testing.T) {
	cfg := newTestConfig(nil)
	m := NewManager(cfg)
	adapter := NewStarterAdapter(m)

	_, err := adapter.StartServer(
		context.Background(),
		"bad",
		"nonexistent-cmd-xyz",
		nil, nil, "",
	)
	if err == nil {
		t.Fatal("StartServer() expected error for bad command, got nil")
	}
}

func TestStarterAdapter_StopServer_Graceful(t *testing.T) {
	cfg := newTestConfig(nil)
	m := NewManager(cfg)
	adapter := NewStarterAdapter(m)

	_, err := adapter.StartServer(context.Background(), "srv", "sleep", []string{"10"}, nil, "")
	if err != nil {
		t.Fatalf("StartServer() error: %v", err)
	}

	if err := adapter.StopServer("srv", false); err != nil {
		t.Errorf("StopServer(force=false) error: %v", err)
	}

	// 서버가 제거되었는지 확인
	_, ok := m.GetProcessInfo("srv")
	if ok {
		t.Error("Server still in manager after StopServer()")
	}
}

func TestStarterAdapter_StopServer_Force(t *testing.T) {
	cfg := newTestConfig(nil)
	m := NewManager(cfg)
	adapter := NewStarterAdapter(m)

	_, err := adapter.StartServer(context.Background(), "srv", "sleep", []string{"10"}, nil, "")
	if err != nil {
		t.Fatalf("StartServer() error: %v", err)
	}

	if err := adapter.StopServer("srv", true); err != nil {
		t.Errorf("StopServer(force=true) error: %v", err)
	}

	_, ok := m.GetProcessInfo("srv")
	if ok {
		t.Error("Server still in manager after ForceStop()")
	}
}

func TestStarterAdapter_StopServer_NotRunning(t *testing.T) {
	cfg := newTestConfig(nil)
	m := NewManager(cfg)
	adapter := NewStarterAdapter(m)

	// Graceful stop on non-existing returns error
	err := adapter.StopServer("nonexistent", false)
	if err == nil {
		t.Error("StopServer(force=false) expected error for non-running server, got nil")
	}

	// Force stop on non-existing returns nil
	err = adapter.StopServer("nonexistent", true)
	if err != nil {
		t.Errorf("StopServer(force=true) error for non-running: %v", err)
	}
}
