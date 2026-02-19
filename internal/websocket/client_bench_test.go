package websocket

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/insajin/autopus-agent-protocol"
)

// BenchmarkMessageSerialization benchmarks WebSocket message JSON serialization.
// Target: consistent ns/op for message encoding.
func BenchmarkMessageSerialization(b *testing.B) {
	payload, _ := json.Marshal(map[string]interface{}{
		"execution_id": "exec-bench-001",
		"output":       "benchmark test output with moderate length content for realistic testing",
		"exit_code":    0,
		"duration_ms":  1234,
	})

	msg := ws.AgentMessage{
		Type:      ws.AgentMsgTaskResult,
		ID:        "msg-bench-001",
		Timestamp: time.Now(),
		Payload:   payload,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := json.Marshal(msg)
		if err != nil {
			b.Fatalf("Marshal error: %v", err)
		}
	}
}

// BenchmarkMessageDeserialization benchmarks WebSocket message JSON deserialization.
// Target: consistent ns/op for message decoding.
func BenchmarkMessageDeserialization(b *testing.B) {
	payload, _ := json.Marshal(map[string]interface{}{
		"execution_id": "exec-bench-001",
		"output":       "benchmark test output with moderate length content for realistic testing",
		"exit_code":    0,
		"duration_ms":  1234,
	})

	msg := ws.AgentMessage{
		Type:      ws.AgentMsgTaskResult,
		ID:        "msg-bench-001",
		Timestamp: time.Now(),
		Payload:   payload,
	}

	data, _ := json.Marshal(msg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var decoded ws.AgentMessage
		err := json.Unmarshal(data, &decoded)
		if err != nil {
			b.Fatalf("Unmarshal error: %v", err)
		}
	}
}

// BenchmarkHMACSigning benchmarks HMAC-SHA256 signing performance.
// Target: signing overhead should be minimal compared to network latency.
func BenchmarkHMACSigning(b *testing.B) {
	signer := NewMessageSigner()
	signer.SetSecret([]byte("benchmark-secret-32-bytes-long!!"))

	payload, _ := json.Marshal(map[string]string{
		"execution_id": "exec-bench-001",
		"output":       "benchmark test output for HMAC signing performance measurement",
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		msg := &ws.AgentMessage{
			Type:      ws.AgentMsgTaskResult,
			ID:        "msg-bench-001",
			Timestamp: time.Now(),
			Payload:   payload,
		}
		err := signer.Sign(msg)
		if err != nil {
			b.Fatalf("Sign error: %v", err)
		}
	}
}

// BenchmarkHMACVerification benchmarks HMAC-SHA256 verification performance.
// Target: verification overhead should be minimal compared to network latency.
func BenchmarkHMACVerification(b *testing.B) {
	signer := NewMessageSigner()
	signer.SetSecret([]byte("benchmark-secret-32-bytes-long!!"))

	payload, _ := json.Marshal(map[string]string{
		"execution_id": "exec-bench-001",
		"output":       "benchmark test output for HMAC verification performance measurement",
	})

	msg := &ws.AgentMessage{
		Type:      ws.AgentMsgTaskResult,
		ID:        "msg-bench-001",
		Timestamp: time.Now(),
		Payload:   payload,
	}
	_ = signer.Sign(msg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if !signer.Verify(msg) {
			b.Fatal("Verify returned false for valid message")
		}
	}
}

// BenchmarkHMACSignAndVerify benchmarks the full sign+verify cycle.
// This represents the total HMAC overhead per critical message.
func BenchmarkHMACSignAndVerify(b *testing.B) {
	signer := NewMessageSigner()
	signer.SetSecret([]byte("benchmark-secret-32-bytes-long!!"))

	payload, _ := json.Marshal(map[string]string{
		"execution_id": "exec-bench-001",
		"output":       "benchmark test output for full HMAC round-trip performance",
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		msg := &ws.AgentMessage{
			Type:      ws.AgentMsgTaskResult,
			ID:        "msg-bench-001",
			Timestamp: time.Now(),
			Payload:   payload,
		}
		_ = signer.Sign(msg)
		if !signer.Verify(msg) {
			b.Fatal("Verify returned false")
		}
	}
}

// BenchmarkMessageRouting benchmarks message type routing via the Router.
// Target: routing overhead should be negligible (<1us).
func BenchmarkMessageRouting(b *testing.B) {
	client := NewClient("ws://localhost:8080/ws", "test-token", "1.0.0")
	router := NewRouter(client)

	payload, _ := json.Marshal(ws.AgentHeartbeatPayload{
		Timestamp: time.Now(),
	})

	msg := ws.AgentMessage{
		Type:      ws.AgentMsgHeartbeat,
		ID:        "msg-bench-001",
		Timestamp: time.Now(),
		Payload:   payload,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = router.HandleMessage(context.TODO(), msg)
	}
}

// BenchmarkBuildSigningPayload benchmarks the signing payload construction.
func BenchmarkBuildSigningPayload(b *testing.B) {
	payload, _ := json.Marshal(map[string]string{
		"execution_id": "exec-bench-001",
		"output":       "payload construction benchmark data",
	})
	ts := time.Now()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = buildSigningPayload(ws.AgentMsgTaskResult, "msg-bench-001", ts, payload)
	}
}

// BenchmarkLargePayloadSerialization benchmarks serialization with a large payload.
// This tests performance with realistic large AI model responses.
func BenchmarkLargePayloadSerialization(b *testing.B) {
	// Create a ~10KB payload to simulate a real response.
	largeOutput := make([]byte, 10*1024)
	for i := range largeOutput {
		largeOutput[i] = byte('a' + (i % 26))
	}

	payload, _ := json.Marshal(map[string]interface{}{
		"execution_id": "exec-bench-large",
		"output":       string(largeOutput),
		"exit_code":    0,
		"duration_ms":  5678,
	})

	msg := ws.AgentMessage{
		Type:      ws.AgentMsgTaskResult,
		ID:        "msg-bench-large",
		Timestamp: time.Now(),
		Payload:   payload,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := json.Marshal(msg)
		if err != nil {
			b.Fatalf("Marshal error: %v", err)
		}
	}
}
