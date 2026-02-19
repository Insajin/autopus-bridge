package websocket

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/insajin/autopus-agent-protocol"
)

// TestUnknownMessageType verifies that unknown message types are silently ignored
// without causing errors or crashes.
// NFR-03: Forward compatibility with new message types.
func TestUnknownMessageType(t *testing.T) {
	client := NewClient("ws://localhost:9999/ws", "test-token", "1.0.0")
	router := NewRouter(client)

	payload, _ := json.Marshal(map[string]string{
		"data": "some unknown data",
	})

	unknownTypes := []string{
		"future_feature",
		"agent_v3_command",
		"unknown_type",
		"experimental_action",
		"",
	}

	for _, msgType := range unknownTypes {
		msg := ws.AgentMessage{
			Type:      msgType,
			ID:        "msg-unknown-001",
			Timestamp: time.Now(),
			Payload:   payload,
		}

		err := router.HandleMessage(context.Background(), msg)
		if err != nil {
			t.Errorf("HandleMessage(type=%q) returned error: %v, want nil", msgType, err)
		}
	}
}

// TestMalformedPayload verifies that malformed payloads produce errors
// but do not crash the router.
// NFR-03: Robustness against malformed input data.
func TestMalformedPayload(t *testing.T) {
	client := NewClient("ws://localhost:9999/ws", "test-token", "1.0.0")

	router := NewRouter(client,
		WithErrorHandler(func(err error) {
			// Errors are expected for malformed payloads; just ensure no panic.
		}),
	)

	tests := []struct {
		name    string
		msgType string
		payload json.RawMessage
	}{
		{
			name:    "invalid JSON in task_request",
			msgType: ws.AgentMsgTaskReq,
			payload: json.RawMessage(`{invalid json`),
		},
		{
			name:    "empty payload in task_request",
			msgType: ws.AgentMsgTaskReq,
			payload: json.RawMessage(`{}`),
		},
		{
			name:    "null payload in build_request",
			msgType: ws.AgentMsgBuildReq,
			payload: json.RawMessage(`null`),
		},
		{
			name:    "array instead of object in test_request",
			msgType: ws.AgentMsgTestReq,
			payload: json.RawMessage(`[1,2,3]`),
		},
		{
			name:    "wrong types in qa_request",
			msgType: ws.AgentMsgQAReq,
			payload: json.RawMessage(`{"execution_id": 12345}`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := ws.AgentMessage{
				Type:      tt.msgType,
				ID:        "msg-malformed-001",
				Timestamp: time.Now(),
				Payload:   tt.payload,
			}

			// HandleMessage should not panic.
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("HandleMessage panicked: %v", r)
					}
				}()
				// Error may or may not be returned depending on handler behavior,
				// but the process must not crash.
				_ = router.HandleMessage(context.Background(), msg)
			}()
		})
	}
}

// TestMissingFields verifies that messages with missing optional fields are handled
// gracefully.
// NFR-03: Messages with missing optional fields must not crash.
func TestMissingFields(t *testing.T) {
	client := NewClient("ws://localhost:9999/ws", "test-token", "1.0.0")
	router := NewRouter(client)

	tests := []struct {
		name    string
		msgType string
		payload string
	}{
		{
			name:    "heartbeat with empty payload",
			msgType: ws.AgentMsgHeartbeat,
			payload: `{}`,
		},
		{
			name:    "heartbeat with only timestamp",
			msgType: ws.AgentMsgHeartbeat,
			payload: `{"timestamp":"2026-01-01T00:00:00Z"}`,
		},
		{
			name:    "task_request missing optional fields",
			msgType: ws.AgentMsgTaskReq,
			payload: `{"execution_id":"exec-001","prompt":"hello","model":"claude"}`,
		},
		{
			name:    "task_request missing timeout",
			msgType: ws.AgentMsgTaskReq,
			payload: `{"execution_id":"exec-002","prompt":"hello","model":"claude","max_tokens":100}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := ws.AgentMessage{
				Type:      tt.msgType,
				ID:        "msg-missing-001",
				Timestamp: time.Now(),
				Payload:   json.RawMessage(tt.payload),
			}

			// Should not panic.
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("HandleMessage panicked with missing fields: %v", r)
					}
				}()
				_ = router.HandleMessage(context.Background(), msg)
			}()
		})
	}
}

// TestLegacyMessageFormat verifies basic backward compatibility with older message formats.
// NFR-03: Legacy messages without signature field should be accepted.
func TestLegacyMessageFormat(t *testing.T) {
	// Legacy messages have no Signature field.
	signer := NewMessageSigner()
	// No secret set - simulates legacy connection.

	tests := []struct {
		name    string
		msgType string
	}{
		{
			name:    "legacy task_result without signature",
			msgType: ws.AgentMsgTaskResult,
		},
		{
			name:    "legacy task_request without signature",
			msgType: ws.AgentMsgTaskReq,
		},
		{
			name:    "legacy heartbeat without signature",
			msgType: ws.AgentMsgHeartbeat,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload, _ := json.Marshal(map[string]string{
				"execution_id": "exec-legacy-001",
			})
			msg := &ws.AgentMessage{
				Type:      tt.msgType,
				ID:        "msg-legacy-001",
				Timestamp: time.Now(),
				Payload:   payload,
				// No Signature field.
			}

			// Without a secret set, verification should pass (backward compatible).
			if !signer.Verify(msg) {
				t.Errorf("Verify() returned false for legacy message (type=%s)", tt.msgType)
			}
		})
	}
}

// TestLegacyMessageFormatWithSecret verifies that when a secret is set,
// legacy messages without signatures are properly rejected for critical types.
// NFR-03: Security upgrade path - reject unsigned critical messages when HMAC is active.
func TestLegacyMessageFormatWithSecret(t *testing.T) {
	signer := NewMessageSigner()
	signer.SetSecret([]byte("test-secret-32-bytes-long-value!"))

	tests := []struct {
		name       string
		msgType    string
		wantVerify bool
	}{
		{
			name:       "critical message without signature - rejected",
			msgType:    ws.AgentMsgTaskResult,
			wantVerify: false,
		},
		{
			name:       "critical message without signature - rejected (task_request)",
			msgType:    ws.AgentMsgTaskReq,
			wantVerify: false,
		},
		{
			name:       "non-critical message without signature - accepted",
			msgType:    ws.AgentMsgHeartbeat,
			wantVerify: true,
		},
		{
			name:       "non-critical message without signature - accepted (connect)",
			msgType:    ws.AgentMsgConnect,
			wantVerify: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload, _ := json.Marshal(map[string]string{
				"execution_id": "exec-legacy-001",
			})
			msg := &ws.AgentMessage{
				Type:      tt.msgType,
				ID:        "msg-legacy-001",
				Timestamp: time.Now(),
				Payload:   payload,
			}

			got := signer.Verify(msg)
			if got != tt.wantVerify {
				t.Errorf("Verify(type=%s) = %v, want %v", tt.msgType, got, tt.wantVerify)
			}
		})
	}
}

// TestMessageDeserializationWithExtraFields verifies that messages with unknown
// additional fields are deserialized without error.
// NFR-03: Forward compatibility with extended message schemas.
func TestMessageDeserializationWithExtraFields(t *testing.T) {
	// Simulate a message from a newer server version with extra fields.
	rawJSON := `{
		"type": "task_request",
		"id": "msg-extra-001",
		"timestamp": "2026-01-01T00:00:00Z",
		"payload": {"execution_id": "exec-001", "prompt": "hello", "model": "claude"},
		"signature": "",
		"future_field": "future_value",
		"metadata": {"version": "3.0"}
	}`

	var msg ws.AgentMessage
	err := json.Unmarshal([]byte(rawJSON), &msg)
	if err != nil {
		t.Fatalf("failed to unmarshal message with extra fields: %v", err)
	}

	if msg.Type != ws.AgentMsgTaskReq {
		t.Errorf("Type = %q, want %q", msg.Type, ws.AgentMsgTaskReq)
	}
	if msg.ID != "msg-extra-001" {
		t.Errorf("ID = %q, want %q", msg.ID, "msg-extra-001")
	}
}

// TestEmptyMessage verifies handling of completely empty messages.
// NFR-03: Edge case robustness.
func TestEmptyMessage(t *testing.T) {
	client := NewClient("ws://localhost:9999/ws", "test-token", "1.0.0")
	router := NewRouter(client)

	msg := ws.AgentMessage{}

	// Should not panic.
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("HandleMessage panicked on empty message: %v", r)
			}
		}()
		_ = router.HandleMessage(context.Background(), msg)
	}()
}

// TestConnectionStateStringCompleteness verifies that all connection states
// have valid string representations.
// NFR-03: All states must be representable for logging and monitoring.
func TestConnectionStateStringCompleteness(t *testing.T) {
	states := []struct {
		state ConnectionState
		want  string
	}{
		{StateDisconnected, "disconnected"},
		{StateConnecting, "connecting"},
		{StateAuthenticating, "authenticating"},
		{StateConnected, "connected"},
		{StateReconnecting, "reconnecting"},
		{StateClosed, "closed"},
	}

	for _, tt := range states {
		got := tt.state.String()
		if got != tt.want {
			t.Errorf("ConnectionState(%d).String() = %q, want %q", tt.state, got, tt.want)
		}
	}

	// Unknown state should return "unknown".
	unknown := ConnectionState(99)
	if got := unknown.String(); got != "unknown" {
		t.Errorf("ConnectionState(99).String() = %q, want %q", got, "unknown")
	}
}
