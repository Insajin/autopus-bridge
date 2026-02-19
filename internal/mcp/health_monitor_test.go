package mcp

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestNewHealthMonitor_DefaultInterval(t *testing.T) {
	cfg := newTestConfig(nil)
	m := NewManager(cfg)
	hm := NewHealthMonitor(m, 0)

	if hm == nil {
		t.Fatal("NewHealthMonitor() returned nil")
	}
	if hm.interval != 60*time.Second {
		t.Errorf("interval = %v, want 60s", hm.interval)
	}
	if hm.stats == nil {
		t.Error("stats map is nil")
	}
}

func TestNewHealthMonitor_NegativeInterval(t *testing.T) {
	cfg := newTestConfig(nil)
	m := NewManager(cfg)
	hm := NewHealthMonitor(m, -5*time.Second)

	if hm.interval != 60*time.Second {
		t.Errorf("interval = %v, want 60s for negative input", hm.interval)
	}
}

func TestNewHealthMonitor_CustomInterval(t *testing.T) {
	cfg := newTestConfig(nil)
	m := NewManager(cfg)
	hm := NewHealthMonitor(m, 30*time.Second)

	if hm.interval != 30*time.Second {
		t.Errorf("interval = %v, want 30s", hm.interval)
	}
}

func TestRecordCall_BasicUpdate(t *testing.T) {
	cfg := newTestConfig(nil)
	m := NewManager(cfg)
	hm := NewHealthMonitor(m, time.Minute)

	hm.RecordCall("server-a", 100, nil)

	hm.statsMu.RLock()
	stats := hm.stats["server-a"]
	hm.statsMu.RUnlock()

	if stats == nil {
		t.Fatal("stats for server-a is nil")
	}
	if stats.TotalCalls != 1 {
		t.Errorf("TotalCalls = %d, want 1", stats.TotalCalls)
	}
	if stats.TotalRespMs != 100 {
		t.Errorf("TotalRespMs = %d, want 100", stats.TotalRespMs)
	}
	if stats.ErrorCount != 0 {
		t.Errorf("ErrorCount = %d, want 0", stats.ErrorCount)
	}
}

func TestRecordCall_WithError(t *testing.T) {
	cfg := newTestConfig(nil)
	m := NewManager(cfg)
	hm := NewHealthMonitor(m, time.Minute)

	testErr := errors.New("connection timeout")
	hm.RecordCall("server-a", 500, testErr)

	hm.statsMu.RLock()
	stats := hm.stats["server-a"]
	hm.statsMu.RUnlock()

	if stats.ErrorCount != 1 {
		t.Errorf("ErrorCount = %d, want 1", stats.ErrorCount)
	}
	if stats.LastError != "connection timeout" {
		t.Errorf("LastError = %q, want %q", stats.LastError, "connection timeout")
	}
	if stats.LastErrorTime.IsZero() {
		t.Error("LastErrorTime is zero, want non-zero")
	}
}

func TestRecordCall_MultipleCalls(t *testing.T) {
	cfg := newTestConfig(nil)
	m := NewManager(cfg)
	hm := NewHealthMonitor(m, time.Minute)

	hm.RecordCall("server-a", 100, nil)
	hm.RecordCall("server-a", 200, nil)
	hm.RecordCall("server-a", 300, errors.New("err"))

	hm.statsMu.RLock()
	stats := hm.stats["server-a"]
	hm.statsMu.RUnlock()

	if stats.TotalCalls != 3 {
		t.Errorf("TotalCalls = %d, want 3", stats.TotalCalls)
	}
	if stats.TotalRespMs != 600 {
		t.Errorf("TotalRespMs = %d, want 600", stats.TotalRespMs)
	}
	if stats.ErrorCount != 1 {
		t.Errorf("ErrorCount = %d, want 1", stats.ErrorCount)
	}
}

func TestRecordCall_MultipleServers(t *testing.T) {
	cfg := newTestConfig(nil)
	m := NewManager(cfg)
	hm := NewHealthMonitor(m, time.Minute)

	hm.RecordCall("server-a", 100, nil)
	hm.RecordCall("server-b", 200, nil)

	hm.statsMu.RLock()
	defer hm.statsMu.RUnlock()

	if len(hm.stats) != 2 {
		t.Errorf("stats has %d entries, want 2", len(hm.stats))
	}
	if hm.stats["server-a"].TotalCalls != 1 {
		t.Errorf("server-a TotalCalls = %d, want 1", hm.stats["server-a"].TotalCalls)
	}
	if hm.stats["server-b"].TotalCalls != 1 {
		t.Errorf("server-b TotalCalls = %d, want 1", hm.stats["server-b"].TotalCalls)
	}
}

func TestRecordCall_ConcurrentSafety(t *testing.T) {
	cfg := newTestConfig(nil)
	m := NewManager(cfg)
	hm := NewHealthMonitor(m, time.Minute)

	var wg sync.WaitGroup
	const goroutines = 50
	const callsPerGoroutine = 100

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < callsPerGoroutine; j++ {
				hm.RecordCall("concurrent-server", 10, nil)
			}
		}()
	}
	wg.Wait()

	hm.statsMu.RLock()
	stats := hm.stats["concurrent-server"]
	hm.statsMu.RUnlock()

	expected := goroutines * callsPerGoroutine
	if stats.TotalCalls != expected {
		t.Errorf("TotalCalls = %d, want %d", stats.TotalCalls, expected)
	}
}

func TestCalcAvgResponseMs_ZeroCalls(t *testing.T) {
	stats := &ServerStats{TotalCalls: 0, TotalRespMs: 0}
	avg := calcAvgResponseMs(stats)
	if avg != 0 {
		t.Errorf("calcAvgResponseMs() = %d, want 0 for zero calls", avg)
	}
}

func TestCalcAvgResponseMs_Normal(t *testing.T) {
	stats := &ServerStats{TotalCalls: 4, TotalRespMs: 400}
	avg := calcAvgResponseMs(stats)
	if avg != 100 {
		t.Errorf("calcAvgResponseMs() = %d, want 100", avg)
	}
}

func TestCalcAvgResponseMs_IntegerDivision(t *testing.T) {
	stats := &ServerStats{TotalCalls: 3, TotalRespMs: 100}
	avg := calcAvgResponseMs(stats)
	// 100 / 3 = 33 (integer division)
	if avg != 33 {
		t.Errorf("calcAvgResponseMs() = %d, want 33", avg)
	}
}

func TestCalcErrorRate_ZeroCalls(t *testing.T) {
	stats := &ServerStats{TotalCalls: 0, ErrorCount: 0}
	rate := calcErrorRate(stats)
	if rate != 0 {
		t.Errorf("calcErrorRate() = %f, want 0 for zero calls", rate)
	}
}

func TestCalcErrorRate_NoErrors(t *testing.T) {
	stats := &ServerStats{TotalCalls: 10, ErrorCount: 0}
	rate := calcErrorRate(stats)
	if rate != 0 {
		t.Errorf("calcErrorRate() = %f, want 0 for no errors", rate)
	}
}

func TestCalcErrorRate_AllErrors(t *testing.T) {
	stats := &ServerStats{TotalCalls: 5, ErrorCount: 5}
	rate := calcErrorRate(stats)
	if rate != 1.0 {
		t.Errorf("calcErrorRate() = %f, want 1.0 for all errors", rate)
	}
}

func TestCalcErrorRate_HalfErrors(t *testing.T) {
	stats := &ServerStats{TotalCalls: 10, ErrorCount: 5}
	rate := calcErrorRate(stats)
	if rate != 0.5 {
		t.Errorf("calcErrorRate() = %f, want 0.5", rate)
	}
}

func TestCollectHealth_NoRunningServers(t *testing.T) {
	cfg := newTestConfig(nil)
	m := NewManager(cfg)
	hm := NewHealthMonitor(m, time.Minute)

	report := hm.CollectHealth()
	if len(report.Servers) != 0 {
		t.Errorf("Servers = %d, want 0 for no running servers and no stats", len(report.Servers))
	}
	if report.ReportedAt.IsZero() {
		t.Error("ReportedAt is zero")
	}
}

func TestCollectHealth_StoppedServerWithStats(t *testing.T) {
	cfg := newTestConfig(nil)
	m := NewManager(cfg)
	hm := NewHealthMonitor(m, time.Minute)

	// Record stats for a server that is not running
	hm.RecordCall("stopped-svc", 100, nil)
	hm.RecordCall("stopped-svc", 200, errors.New("oops"))

	report := hm.CollectHealth()
	if len(report.Servers) != 1 {
		t.Fatalf("Servers = %d, want 1", len(report.Servers))
	}

	srv := report.Servers[0]
	if srv.Name != "stopped-svc" {
		t.Errorf("Name = %q, want %q", srv.Name, "stopped-svc")
	}
	if srv.Status != "stopped" {
		t.Errorf("Status = %q, want %q", srv.Status, "stopped")
	}
	if srv.TotalCalls != 2 {
		t.Errorf("TotalCalls = %d, want 2", srv.TotalCalls)
	}
	if srv.ErrorCount != 1 {
		t.Errorf("ErrorCount = %d, want 1", srv.ErrorCount)
	}
	if srv.UptimeSeconds != 0 {
		t.Errorf("UptimeSeconds = %d, want 0 for stopped server", srv.UptimeSeconds)
	}
	if srv.LastError == nil {
		t.Error("LastError is nil, want non-nil")
	} else if *srv.LastError != "oops" {
		t.Errorf("LastError = %q, want %q", *srv.LastError, "oops")
	}
}

func TestCollectHealth_RunningServer(t *testing.T) {
	cfg := newTestConfig(nil)
	m := NewManager(cfg)

	override := &ServerConfig{Command: "sleep", Args: []string{"30"}}
	_, err := m.Start(context.Background(), "running-svc", override)
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer m.ForceStop("running-svc") //nolint:errcheck

	hm := NewHealthMonitor(m, time.Minute)
	hm.RecordCall("running-svc", 50, nil)

	report := hm.CollectHealth()

	// Find the running server in the report
	var found bool
	for _, srv := range report.Servers {
		if srv.Name == "running-svc" {
			found = true
			if srv.Status != "running" {
				t.Errorf("Status = %q, want %q", srv.Status, "running")
			}
			if srv.TotalCalls != 1 {
				t.Errorf("TotalCalls = %d, want 1", srv.TotalCalls)
			}
			if srv.AvgResponseMs != 50 {
				t.Errorf("AvgResponseMs = %d, want 50", srv.AvgResponseMs)
			}
			if srv.ErrorRate != 0 {
				t.Errorf("ErrorRate = %f, want 0", srv.ErrorRate)
			}
			if srv.LastError != nil {
				t.Errorf("LastError = %v, want nil", srv.LastError)
			}
			break
		}
	}
	if !found {
		t.Error("running-svc not found in health report")
	}
}

func TestCollectHealth_HighErrorRate(t *testing.T) {
	cfg := newTestConfig(nil)
	m := NewManager(cfg)

	override := &ServerConfig{Command: "sleep", Args: []string{"30"}}
	_, err := m.Start(context.Background(), "error-svc", override)
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer m.ForceStop("error-svc") //nolint:errcheck

	hm := NewHealthMonitor(m, time.Minute)

	// Record 10+ calls with >50% error rate to trigger "error" status
	for i := 0; i < 10; i++ {
		hm.RecordCall("error-svc", 100, errors.New("fail"))
	}
	hm.RecordCall("error-svc", 50, nil) // 1 success

	report := hm.CollectHealth()

	for _, srv := range report.Servers {
		if srv.Name == "error-svc" {
			if srv.Status != "error" {
				t.Errorf("Status = %q, want %q for high error rate", srv.Status, "error")
			}
			return
		}
	}
	t.Error("error-svc not found in health report")
}

func TestCollectHealth_LowErrorRateNoStatusChange(t *testing.T) {
	cfg := newTestConfig(nil)
	m := NewManager(cfg)

	override := &ServerConfig{Command: "sleep", Args: []string{"30"}}
	_, err := m.Start(context.Background(), "ok-svc", override)
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer m.ForceStop("ok-svc") //nolint:errcheck

	hm := NewHealthMonitor(m, time.Minute)

	// Record 10+ calls with low error rate (under 50%)
	for i := 0; i < 8; i++ {
		hm.RecordCall("ok-svc", 50, nil)
	}
	hm.RecordCall("ok-svc", 100, errors.New("occasional"))
	hm.RecordCall("ok-svc", 50, nil)

	report := hm.CollectHealth()

	for _, srv := range report.Servers {
		if srv.Name == "ok-svc" {
			if srv.Status != "running" {
				t.Errorf("Status = %q, want %q for low error rate", srv.Status, "running")
			}
			return
		}
	}
	t.Error("ok-svc not found in health report")
}

func TestStartAndStop(t *testing.T) {
	cfg := newTestConfig(nil)
	m := NewManager(cfg)
	hm := NewHealthMonitor(m, 50*time.Millisecond)

	var reportCount int
	var mu sync.Mutex
	reportFn := func(report HealthReport) {
		mu.Lock()
		reportCount++
		mu.Unlock()
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hm.Start(ctx, reportFn)

	// Wait enough time for a few ticks
	time.Sleep(200 * time.Millisecond)

	hm.Stop()

	mu.Lock()
	count := reportCount
	mu.Unlock()

	if count == 0 {
		t.Error("reportFn was never called, want at least 1 call")
	}
}

func TestStop_Idempotent(t *testing.T) {
	cfg := newTestConfig(nil)
	m := NewManager(cfg)
	hm := NewHealthMonitor(m, time.Minute)

	ctx := context.Background()
	hm.Start(ctx, nil)

	// Calling Stop multiple times should not panic
	hm.Stop()
	hm.Stop()
}

func TestStop_WithoutStart(t *testing.T) {
	cfg := newTestConfig(nil)
	m := NewManager(cfg)
	hm := NewHealthMonitor(m, time.Minute)

	// Stop without Start should not panic
	hm.Stop()
}

func TestStart_ContextCancellation(t *testing.T) {
	cfg := newTestConfig(nil)
	m := NewManager(cfg)
	hm := NewHealthMonitor(m, 50*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())

	var reportCount int
	var mu sync.Mutex
	hm.Start(ctx, func(report HealthReport) {
		mu.Lock()
		reportCount++
		mu.Unlock()
	})

	// Let it run briefly
	time.Sleep(100 * time.Millisecond)

	// Cancel context to stop the monitor
	cancel()

	// Wait for goroutine to exit
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	countBefore := reportCount
	mu.Unlock()

	// After cancellation, no more reports should come
	time.Sleep(150 * time.Millisecond)

	mu.Lock()
	countAfter := reportCount
	mu.Unlock()

	if countAfter != countBefore {
		t.Errorf("reports continued after context cancellation: before=%d, after=%d", countBefore, countAfter)
	}
}

func TestStart_NilReportFn(t *testing.T) {
	cfg := newTestConfig(nil)
	m := NewManager(cfg)
	hm := NewHealthMonitor(m, 50*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start with nil reportFn should not panic
	hm.Start(ctx, nil)

	time.Sleep(150 * time.Millisecond)

	hm.Stop()
}
