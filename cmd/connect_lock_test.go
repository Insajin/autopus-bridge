package cmd

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestAcquireConnectLockRejectsRunningProcessWithoutReplace(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	lockPath := filepath.Join(home, ".config", "autopus", connectLockFileName)
	if err := os.MkdirAll(filepath.Dir(lockPath), 0700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(lockPath, []byte("4321\n"), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	origRunning := connectProcessRunningFn
	origStop := connectStopProcessFn
	t.Cleanup(func() {
		connectProcessRunningFn = origRunning
		connectStopProcessFn = origStop
	})

	connectProcessRunningFn = func(pid int) bool {
		return pid == 4321
	}
	connectStopProcessFn = func(pid int) error {
		t.Fatalf("connectStopProcessFn(%d) should not be called", pid)
		return nil
	}

	_, err := acquireConnectLock("", false)
	if err == nil {
		t.Fatal("acquireConnectLock(false) error = nil, want running process error")
	}
	if !strings.Contains(err.Error(), "PID: 4321") {
		t.Fatalf("error = %q, want PID context", err)
	}
	if !strings.Contains(err.Error(), "--replace") {
		t.Fatalf("error = %q, want replace hint", err)
	}
}

func TestAcquireConnectLockReplacesRunningProcess(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	lockPath := filepath.Join(home, ".config", "autopus", connectLockFileName)
	if err := os.MkdirAll(filepath.Dir(lockPath), 0700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(lockPath, []byte("4321\n"), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	origRunning := connectProcessRunningFn
	origStop := connectStopProcessFn
	t.Cleanup(func() {
		connectProcessRunningFn = origRunning
		connectStopProcessFn = origStop
	})

	running := true
	stoppedPID := 0
	connectProcessRunningFn = func(pid int) bool {
		return pid == 4321 && running
	}
	connectStopProcessFn = func(pid int) error {
		stoppedPID = pid
		running = false
		return nil
	}

	lock, err := acquireConnectLock("", true)
	if err != nil {
		t.Fatalf("acquireConnectLock(true) error = %v", err)
	}

	if stoppedPID != 4321 {
		t.Fatalf("stopped PID = %d, want 4321", stoppedPID)
	}

	data, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if got, want := strings.TrimSpace(string(data)), strconv.Itoa(os.Getpid()); got != want {
		t.Fatalf("lock content = %q, want %q", got, want)
	}

	lock.Release()
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Fatalf("lock file still exists after Release(): %v", err)
	}
}
