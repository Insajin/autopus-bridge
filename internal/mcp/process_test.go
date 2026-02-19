package mcp

import (
	"context"
	"os/exec"
	"testing"
	"time"
)

func TestProcessInfo_IsRunning_NilCmd(t *testing.T) {
	p := &ProcessInfo{}
	if p.IsRunning() {
		t.Error("IsRunning() = true for nil cmd, want false")
	}
}

func TestProcessInfo_IsRunning_NilProcess(t *testing.T) {
	p := &ProcessInfo{cmd: &exec.Cmd{}}
	if p.IsRunning() {
		t.Error("IsRunning() = true for nil Process, want false")
	}
}

func TestProcessInfo_Stop_NilCmd(t *testing.T) {
	p := &ProcessInfo{}
	if err := p.Stop(); err != nil {
		t.Errorf("Stop() error = %v for nil cmd, want nil", err)
	}
}

func TestProcessInfo_Stop_RealProcess(t *testing.T) {
	// startProcess를 사용하여 실제 환경과 동일한 조건으로 테스트
	cfg := ServerConfig{
		Name:    "test-sleep",
		Command: "sleep",
		Args:    []string{"60"},
	}

	p, err := startProcess(context.Background(), cfg)
	if err != nil {
		t.Fatalf("startProcess() error: %v", err)
	}

	if !p.IsRunning() {
		t.Fatal("IsRunning() = false for running process, want true")
	}

	if err := p.Stop(); err != nil {
		t.Errorf("Stop() error = %v, want nil", err)
	}

	// SIGTERM/SIGKILL 후 프로세스 정리 대기
	time.Sleep(500 * time.Millisecond)

	if p.IsRunning() {
		t.Error("IsRunning() = true after Stop(), want false")
	}
}

func TestProcessInfo_Cleanup(t *testing.T) {
	cancelled := false
	p := &ProcessInfo{
		cancel: func() { cancelled = true },
	}
	p.cleanup()
	if !cancelled {
		t.Error("cleanup() did not call cancel function")
	}
}

func TestProcessInfo_Cleanup_NilCancel(t *testing.T) {
	p := &ProcessInfo{}
	// cleanup with nil cancel should not panic
	p.cleanup()
}

func TestStartProcess_CommandNotFound(t *testing.T) {
	cfg := ServerConfig{
		Name:    "bad-server",
		Command: "nonexistent-binary-xyz-12345",
	}
	_, err := startProcess(context.Background(), cfg)
	if err == nil {
		t.Fatal("startProcess() expected error for missing command, got nil")
	}
}

func TestStartProcess_RequiredBinaryNotFound(t *testing.T) {
	cfg := ServerConfig{
		Name:           "bad-server",
		Command:        "echo",
		RequiredBinary: "nonexistent-required-binary-xyz",
	}
	_, err := startProcess(context.Background(), cfg)
	if err == nil {
		t.Fatal("startProcess() expected error for missing required binary, got nil")
	}
}

func TestStartProcess_Success(t *testing.T) {
	cfg := ServerConfig{
		Name:    "echo-test",
		Command: "sleep",
		Args:    []string{"5"},
		Env:     map[string]string{"TEST_VAR": "hello"},
	}

	proc, err := startProcess(context.Background(), cfg)
	if err != nil {
		t.Fatalf("startProcess() error: %v", err)
	}

	if proc.Name != "echo-test" {
		t.Errorf("Name = %q, want %q", proc.Name, "echo-test")
	}
	if proc.PID <= 0 {
		t.Errorf("PID = %d, want > 0", proc.PID)
	}
	if proc.Command != "sleep" {
		t.Errorf("Command = %q, want %q", proc.Command, "sleep")
	}
	if proc.StartedAt.IsZero() {
		t.Error("StartedAt is zero")
	}

	// 정리
	if err := proc.Stop(); err != nil {
		t.Errorf("cleanup Stop() error: %v", err)
	}
}

func TestLogWriter(t *testing.T) {
	tests := []struct {
		name  string
		level string
		input string
	}{
		{"info level", "info", "test info message"},
		{"error level", "error", "test error message"},
		{"default level", "debug", "test debug message"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &logWriter{name: "test-server", level: tt.level}
			n, err := w.Write([]byte(tt.input))
			if err != nil {
				t.Errorf("Write() error = %v", err)
			}
			if n != len(tt.input) {
				t.Errorf("Write() returned %d, want %d", n, len(tt.input))
			}
		})
	}
}

func TestStopGracePeriod(t *testing.T) {
	if StopGracePeriod != 5*time.Second {
		t.Errorf("StopGracePeriod = %v, want 5s", StopGracePeriod)
	}
}
