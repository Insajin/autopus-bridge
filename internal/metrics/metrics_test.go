package metrics

import (
	"encoding/json"
	"sync"
	"testing"
	"time"
)

// TestNewMetrics verifies that a new Metrics instance is properly initialized.
func TestNewMetrics(t *testing.T) {
	m := NewMetrics()
	if m == nil {
		t.Fatal("NewMetrics() returned nil")
	}

	if m.ConnectionAttempts.Load() != 0 {
		t.Errorf("ConnectionAttempts = %d, want 0", m.ConnectionAttempts.Load())
	}
	if m.MessagesSent.Load() != 0 {
		t.Errorf("MessagesSent = %d, want 0", m.MessagesSent.Load())
	}
	if m.TasksReceived.Load() != 0 {
		t.Errorf("TasksReceived = %d, want 0", m.TasksReceived.Load())
	}
}

// TestMetrics_ConnectionCounters verifies connection metric increments.
func TestMetrics_ConnectionCounters(t *testing.T) {
	m := NewMetrics()

	m.ConnectionAttempts.Add(1)
	m.ConnectionAttempts.Add(1)
	m.ConnectionSuccesses.Add(1)
	m.ConnectionFailures.Add(1)
	m.Reconnections.Add(1)

	if m.ConnectionAttempts.Load() != 2 {
		t.Errorf("ConnectionAttempts = %d, want 2", m.ConnectionAttempts.Load())
	}
	if m.ConnectionSuccesses.Load() != 1 {
		t.Errorf("ConnectionSuccesses = %d, want 1", m.ConnectionSuccesses.Load())
	}
	if m.ConnectionFailures.Load() != 1 {
		t.Errorf("ConnectionFailures = %d, want 1", m.ConnectionFailures.Load())
	}
	if m.Reconnections.Load() != 1 {
		t.Errorf("Reconnections = %d, want 1", m.Reconnections.Load())
	}
}

// TestMetrics_MessageCounters verifies message metric increments.
func TestMetrics_MessageCounters(t *testing.T) {
	m := NewMetrics()

	m.MessagesSent.Add(5)
	m.MessagesReceived.Add(10)
	m.MessageErrors.Add(2)

	if m.MessagesSent.Load() != 5 {
		t.Errorf("MessagesSent = %d, want 5", m.MessagesSent.Load())
	}
	if m.MessagesReceived.Load() != 10 {
		t.Errorf("MessagesReceived = %d, want 10", m.MessagesReceived.Load())
	}
	if m.MessageErrors.Load() != 2 {
		t.Errorf("MessageErrors = %d, want 2", m.MessageErrors.Load())
	}
}

// TestMetrics_TaskCounters verifies task metric increments.
func TestMetrics_TaskCounters(t *testing.T) {
	m := NewMetrics()

	m.TasksReceived.Add(10)
	m.TasksCompleted.Add(7)
	m.TasksFailed.Add(2)
	m.TasksTimedOut.Add(1)

	if m.TasksReceived.Load() != 10 {
		t.Errorf("TasksReceived = %d, want 10", m.TasksReceived.Load())
	}
	if m.TasksCompleted.Load() != 7 {
		t.Errorf("TasksCompleted = %d, want 7", m.TasksCompleted.Load())
	}
	if m.TasksFailed.Load() != 2 {
		t.Errorf("TasksFailed = %d, want 2", m.TasksFailed.Load())
	}
	if m.TasksTimedOut.Load() != 1 {
		t.Errorf("TasksTimedOut = %d, want 1", m.TasksTimedOut.Load())
	}
}

// TestMetrics_RecordLatency verifies latency recording and averaging.
func TestMetrics_RecordLatency(t *testing.T) {
	m := NewMetrics()

	// No latency recorded yet.
	if m.AvgLatency() != 0 {
		t.Errorf("initial AvgLatency = %v, want 0", m.AvgLatency())
	}

	// Record a single latency.
	m.RecordLatency(100 * time.Millisecond)
	avg := m.AvgLatency()
	if avg < 90*time.Millisecond || avg > 110*time.Millisecond {
		t.Errorf("AvgLatency after 1 recording = %v, want ~100ms", avg)
	}

	// Record another latency to verify averaging works.
	m.RecordLatency(200 * time.Millisecond)
	avg = m.AvgLatency()
	// Running average: first=100ms, second=200ms -> avg should approach 150ms.
	if avg < 100*time.Millisecond || avg > 200*time.Millisecond {
		t.Errorf("AvgLatency after 2 recordings = %v, want ~150ms", avg)
	}
}

// TestMetrics_RecordHeartbeat verifies heartbeat recording.
func TestMetrics_RecordHeartbeat(t *testing.T) {
	m := NewMetrics()

	// No heartbeat recorded yet.
	if v := m.lastHeartbeat.Load(); v != nil {
		t.Error("initial lastHeartbeat should be nil")
	}

	// Record heartbeat.
	before := time.Now()
	m.RecordHeartbeat()
	after := time.Now()

	v := m.lastHeartbeat.Load()
	if v == nil {
		t.Fatal("lastHeartbeat is nil after RecordHeartbeat()")
	}

	hb, ok := v.(time.Time)
	if !ok {
		t.Fatal("lastHeartbeat is not time.Time")
	}

	if hb.Before(before) || hb.After(after) {
		t.Errorf("lastHeartbeat = %v, want between %v and %v", hb, before, after)
	}
}

// TestMetrics_Uptime verifies uptime calculation.
func TestMetrics_Uptime(t *testing.T) {
	m := NewMetrics()

	// Uptime should be very small right after creation.
	uptime := m.Uptime()
	if uptime < 0 {
		t.Errorf("Uptime = %v, want >= 0", uptime)
	}

	// Wait a short time and verify uptime increases.
	time.Sleep(10 * time.Millisecond)
	uptime2 := m.Uptime()
	if uptime2 <= uptime {
		t.Errorf("Uptime did not increase: %v <= %v", uptime2, uptime)
	}
}

// TestMetrics_Snapshot verifies that Snapshot captures all current values.
func TestMetrics_Snapshot(t *testing.T) {
	m := NewMetrics()

	// Set some values.
	m.ConnectionAttempts.Store(5)
	m.ConnectionSuccesses.Store(3)
	m.ConnectionFailures.Store(2)
	m.Reconnections.Store(1)
	m.MessagesSent.Store(100)
	m.MessagesReceived.Store(200)
	m.MessageErrors.Store(5)
	m.TasksReceived.Store(50)
	m.TasksCompleted.Store(45)
	m.TasksFailed.Store(3)
	m.TasksTimedOut.Store(2)
	m.RecordLatency(50 * time.Millisecond)
	m.RecordHeartbeat()

	snap := m.Snapshot()

	if snap.ConnectionAttempts != 5 {
		t.Errorf("snap.ConnectionAttempts = %d, want 5", snap.ConnectionAttempts)
	}
	if snap.ConnectionSuccesses != 3 {
		t.Errorf("snap.ConnectionSuccesses = %d, want 3", snap.ConnectionSuccesses)
	}
	if snap.ConnectionFailures != 2 {
		t.Errorf("snap.ConnectionFailures = %d, want 2", snap.ConnectionFailures)
	}
	if snap.Reconnections != 1 {
		t.Errorf("snap.Reconnections = %d, want 1", snap.Reconnections)
	}
	if snap.MessagesSent != 100 {
		t.Errorf("snap.MessagesSent = %d, want 100", snap.MessagesSent)
	}
	if snap.MessagesReceived != 200 {
		t.Errorf("snap.MessagesReceived = %d, want 200", snap.MessagesReceived)
	}
	if snap.MessageErrors != 5 {
		t.Errorf("snap.MessageErrors = %d, want 5", snap.MessageErrors)
	}
	if snap.TasksReceived != 50 {
		t.Errorf("snap.TasksReceived = %d, want 50", snap.TasksReceived)
	}
	if snap.TasksCompleted != 45 {
		t.Errorf("snap.TasksCompleted = %d, want 45", snap.TasksCompleted)
	}
	if snap.TasksFailed != 3 {
		t.Errorf("snap.TasksFailed = %d, want 3", snap.TasksFailed)
	}
	if snap.TasksTimedOut != 2 {
		t.Errorf("snap.TasksTimedOut = %d, want 2", snap.TasksTimedOut)
	}
	if snap.Timestamp.IsZero() {
		t.Error("snap.Timestamp is zero")
	}
	if snap.Uptime == "" {
		t.Error("snap.Uptime is empty")
	}
	if snap.LastHeartbeat == "" {
		t.Error("snap.LastHeartbeat is empty after RecordHeartbeat()")
	}
	if snap.AvgLatencyMs <= 0 {
		t.Errorf("snap.AvgLatencyMs = %f, want > 0", snap.AvgLatencyMs)
	}
}

// TestMetrics_SnapshotNoHeartbeat verifies snapshot when no heartbeat was recorded.
func TestMetrics_SnapshotNoHeartbeat(t *testing.T) {
	m := NewMetrics()
	snap := m.Snapshot()

	if snap.LastHeartbeat != "" {
		t.Errorf("snap.LastHeartbeat = %q, want empty", snap.LastHeartbeat)
	}
}

// TestMetrics_ToJSON verifies JSON serialization of metrics.
func TestMetrics_ToJSON(t *testing.T) {
	m := NewMetrics()

	m.ConnectionAttempts.Store(3)
	m.MessagesSent.Store(10)
	m.TasksCompleted.Store(5)
	m.RecordHeartbeat()

	data, err := m.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON() error: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("ToJSON() returned empty data")
	}

	// Verify it is valid JSON by unmarshaling.
	var snap MetricsSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	if snap.ConnectionAttempts != 3 {
		t.Errorf("JSON snap.ConnectionAttempts = %d, want 3", snap.ConnectionAttempts)
	}
	if snap.MessagesSent != 10 {
		t.Errorf("JSON snap.MessagesSent = %d, want 10", snap.MessagesSent)
	}
	if snap.TasksCompleted != 5 {
		t.Errorf("JSON snap.TasksCompleted = %d, want 5", snap.TasksCompleted)
	}
	if snap.LastHeartbeat == "" {
		t.Error("JSON snap.LastHeartbeat is empty")
	}
}

// TestMetrics_ToJSON_ValidStructure verifies that all expected JSON fields are present.
func TestMetrics_ToJSON_ValidStructure(t *testing.T) {
	m := NewMetrics()
	m.RecordLatency(10 * time.Millisecond)

	data, err := m.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON() error: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("failed to unmarshal JSON to map: %v", err)
	}

	expectedFields := []string{
		"timestamp", "uptime",
		"connection_attempts", "connection_successes", "connection_failures", "reconnections",
		"messages_sent", "messages_received", "message_errors",
		"tasks_received", "tasks_completed", "tasks_failed", "tasks_timed_out",
		"avg_latency_ms",
	}

	for _, field := range expectedFields {
		if _, exists := raw[field]; !exists {
			t.Errorf("JSON missing field: %s", field)
		}
	}
}

// TestMetrics_Reset verifies that Reset clears all counters.
func TestMetrics_Reset(t *testing.T) {
	m := NewMetrics()

	m.ConnectionAttempts.Store(10)
	m.MessagesSent.Store(50)
	m.TasksReceived.Store(20)
	m.RecordLatency(100 * time.Millisecond)

	m.Reset()

	if m.ConnectionAttempts.Load() != 0 {
		t.Errorf("after Reset, ConnectionAttempts = %d, want 0", m.ConnectionAttempts.Load())
	}
	if m.MessagesSent.Load() != 0 {
		t.Errorf("after Reset, MessagesSent = %d, want 0", m.MessagesSent.Load())
	}
	if m.TasksReceived.Load() != 0 {
		t.Errorf("after Reset, TasksReceived = %d, want 0", m.TasksReceived.Load())
	}
	if m.AvgLatency() != 0 {
		t.Errorf("after Reset, AvgLatency = %v, want 0", m.AvgLatency())
	}
}

// TestMetrics_ConcurrentAccess verifies thread safety of all metric operations.
func TestMetrics_ConcurrentAccess(t *testing.T) {
	m := NewMetrics()

	var wg sync.WaitGroup
	numGoroutines := 20
	opsPerGoroutine := 100

	// Concurrent writers for connection metrics.
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				m.ConnectionAttempts.Add(1)
				m.ConnectionSuccesses.Add(1)
				m.ConnectionFailures.Add(1)
				m.Reconnections.Add(1)
			}
		}()
	}

	// Concurrent writers for message metrics.
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				m.MessagesSent.Add(1)
				m.MessagesReceived.Add(1)
				m.MessageErrors.Add(1)
			}
		}()
	}

	// Concurrent writers for task metrics.
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				m.TasksReceived.Add(1)
				m.TasksCompleted.Add(1)
				m.TasksFailed.Add(1)
				m.TasksTimedOut.Add(1)
			}
		}()
	}

	// Concurrent latency recording.
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				m.RecordLatency(time.Duration(j) * time.Microsecond)
			}
		}()
	}

	// Concurrent heartbeat recording.
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				m.RecordHeartbeat()
			}
		}()
	}

	// Concurrent readers.
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				_ = m.Snapshot()
				_ = m.Uptime()
				_ = m.AvgLatency()
			}
		}()
	}

	wg.Wait()

	expected := int64(numGoroutines * opsPerGoroutine)
	if m.ConnectionAttempts.Load() != expected {
		t.Errorf("ConnectionAttempts = %d, want %d", m.ConnectionAttempts.Load(), expected)
	}
	if m.MessagesSent.Load() != expected {
		t.Errorf("MessagesSent = %d, want %d", m.MessagesSent.Load(), expected)
	}
	if m.TasksReceived.Load() != expected {
		t.Errorf("TasksReceived = %d, want %d", m.TasksReceived.Load(), expected)
	}
}

// TestMetrics_ConcurrentToJSON verifies thread safety of JSON serialization.
func TestMetrics_ConcurrentToJSON(t *testing.T) {
	m := NewMetrics()

	var wg sync.WaitGroup

	// Writers.
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				m.ConnectionAttempts.Add(1)
				m.MessagesSent.Add(1)
				m.RecordLatency(time.Millisecond)
				m.RecordHeartbeat()
			}
		}()
	}

	// Concurrent JSON serialization.
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				data, err := m.ToJSON()
				if err != nil {
					t.Errorf("ToJSON() error: %v", err)
					return
				}
				if len(data) == 0 {
					t.Error("ToJSON() returned empty data")
					return
				}
			}
		}()
	}

	wg.Wait()
}
