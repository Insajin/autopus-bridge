// Package metrics provides operational metrics tracking for the Local Agent Bridge.
// NFR-04: Structured observability metrics for connection, message, and task monitoring.
package metrics

import (
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"
)

// Metrics tracks operational metrics for the bridge.
// All fields are thread-safe for concurrent access.
type Metrics struct {
	// Connection metrics
	ConnectionAttempts  atomic.Int64
	ConnectionSuccesses atomic.Int64
	ConnectionFailures  atomic.Int64
	Reconnections       atomic.Int64

	// Message metrics
	MessagesSent     atomic.Int64
	MessagesReceived atomic.Int64
	MessageErrors    atomic.Int64

	// Task metrics
	TasksReceived  atomic.Int64
	TasksCompleted atomic.Int64
	TasksFailed    atomic.Int64
	TasksTimedOut  atomic.Int64

	// Timing metrics
	startTime     time.Time
	lastHeartbeat atomic.Value // time.Time
	avgLatencyNs  atomic.Int64
	latencyCount  atomic.Int64

	mu sync.RWMutex
}

// MetricsSnapshot is a point-in-time copy of all metrics.
type MetricsSnapshot struct {
	Timestamp           time.Time `json:"timestamp"`
	Uptime              string    `json:"uptime"`
	ConnectionAttempts  int64     `json:"connection_attempts"`
	ConnectionSuccesses int64     `json:"connection_successes"`
	ConnectionFailures  int64     `json:"connection_failures"`
	Reconnections       int64     `json:"reconnections"`
	MessagesSent        int64     `json:"messages_sent"`
	MessagesReceived    int64     `json:"messages_received"`
	MessageErrors       int64     `json:"message_errors"`
	TasksReceived       int64     `json:"tasks_received"`
	TasksCompleted      int64     `json:"tasks_completed"`
	TasksFailed         int64     `json:"tasks_failed"`
	TasksTimedOut       int64     `json:"tasks_timed_out"`
	AvgLatencyMs        float64   `json:"avg_latency_ms"`
	LastHeartbeat       string    `json:"last_heartbeat,omitempty"`
}

// NewMetrics creates a new Metrics instance with the start time set to now.
func NewMetrics() *Metrics {
	m := &Metrics{
		startTime: time.Now(),
	}
	return m
}

// RecordLatency records a single latency measurement and updates the running average.
func (m *Metrics) RecordLatency(d time.Duration) {
	ns := d.Nanoseconds()
	count := m.latencyCount.Add(1)

	// Running average: newAvg = oldAvg + (newValue - oldAvg) / count
	// Use a CAS loop for atomic update of the average.
	for {
		oldAvg := m.avgLatencyNs.Load()
		newAvg := oldAvg + (ns-oldAvg)/count
		if m.avgLatencyNs.CompareAndSwap(oldAvg, newAvg) {
			break
		}
		// Reload count in case it changed.
		count = m.latencyCount.Load()
		if count == 0 {
			count = 1
		}
	}
}

// RecordHeartbeat records the time of the last heartbeat.
func (m *Metrics) RecordHeartbeat() {
	m.lastHeartbeat.Store(time.Now())
}

// Uptime returns the duration since the metrics instance was created.
func (m *Metrics) Uptime() time.Duration {
	return time.Since(m.startTime)
}

// AvgLatency returns the average recorded latency.
// Returns 0 if no latency has been recorded.
func (m *Metrics) AvgLatency() time.Duration {
	ns := m.avgLatencyNs.Load()
	return time.Duration(ns)
}

// Snapshot returns a point-in-time copy of all metrics.
func (m *Metrics) Snapshot() MetricsSnapshot {
	snap := MetricsSnapshot{
		Timestamp:           time.Now(),
		Uptime:              m.Uptime().Round(time.Millisecond).String(),
		ConnectionAttempts:  m.ConnectionAttempts.Load(),
		ConnectionSuccesses: m.ConnectionSuccesses.Load(),
		ConnectionFailures:  m.ConnectionFailures.Load(),
		Reconnections:       m.Reconnections.Load(),
		MessagesSent:        m.MessagesSent.Load(),
		MessagesReceived:    m.MessagesReceived.Load(),
		MessageErrors:       m.MessageErrors.Load(),
		TasksReceived:       m.TasksReceived.Load(),
		TasksCompleted:      m.TasksCompleted.Load(),
		TasksFailed:         m.TasksFailed.Load(),
		TasksTimedOut:       m.TasksTimedOut.Load(),
		AvgLatencyMs:        float64(m.avgLatencyNs.Load()) / float64(time.Millisecond),
	}

	if v := m.lastHeartbeat.Load(); v != nil {
		if t, ok := v.(time.Time); ok && !t.IsZero() {
			snap.LastHeartbeat = t.Format(time.RFC3339)
		}
	}

	return snap
}

// ToJSON returns a JSON-encoded representation of the current metrics snapshot.
func (m *Metrics) ToJSON() ([]byte, error) {
	snap := m.Snapshot()
	return json.Marshal(snap)
}

// Reset resets all metric counters to zero while preserving the start time.
func (m *Metrics) Reset() {
	m.ConnectionAttempts.Store(0)
	m.ConnectionSuccesses.Store(0)
	m.ConnectionFailures.Store(0)
	m.Reconnections.Store(0)
	m.MessagesSent.Store(0)
	m.MessagesReceived.Store(0)
	m.MessageErrors.Store(0)
	m.TasksReceived.Store(0)
	m.TasksCompleted.Store(0)
	m.TasksFailed.Store(0)
	m.TasksTimedOut.Store(0)
	m.avgLatencyNs.Store(0)
	m.latencyCount.Store(0)

	m.mu.Lock()
	m.startTime = time.Now()
	m.mu.Unlock()
}
