package cmd

import (
	"testing"
	"time"
)

func TestBuildReconnectArgs(t *testing.T) {
	origCfgFile := cfgFile
	origVerbose := verbose
	defer func() {
		cfgFile = origCfgFile
		verbose = origVerbose
	}()

	cfgFile = "/tmp/autopus/config.yaml"
	verbose = true

	got := buildReconnectArgs(&runningConnectProcess{
		PID:       1234,
		ServerURL: "wss://api.autopus.co/ws/agent",
	})

	want := []string{
		"--config", "/tmp/autopus/config.yaml",
		"--verbose",
		"connect",
		"--server", "wss://api.autopus.co/ws/agent",
	}

	if len(got) != len(want) {
		t.Fatalf("len(args) = %d, want %d (%v)", len(got), len(want), got)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("args[%d] = %q, want %q (full=%v)", i, got[i], want[i], got)
		}
	}
}

func TestWaitForProcessExit(t *testing.T) {
	origRunning := updateProcessRunningFn
	origSleep := updateSleep
	defer func() {
		updateProcessRunningFn = origRunning
		updateSleep = origSleep
	}()

	callCount := 0
	updateProcessRunningFn = func(pid int) bool {
		callCount++
		return callCount < 3
	}
	updateSleep = func(_ time.Duration) {}

	if !waitForProcessExit(42, updateStopPollInterval*4) {
		t.Fatal("waitForProcessExit() = false, want true")
	}
}
