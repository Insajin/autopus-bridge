package websocket

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/insajin/autopus-agent-protocol"
)

// ---------------------------------------------------------------------------
// Tests: Computer Use Handler Routing (SPEC-COMPUTER-USE-001)
// ---------------------------------------------------------------------------

// TestHandleComputerSessionStart_InvalidPayload verifies that invalid JSON
// in a computer_session_start message does not crash the router.
func TestHandleComputerSessionStart_InvalidPayload(t *testing.T) {
	client := NewClient("ws://localhost:9999/ws", "test-token", "1.0.0")
	router := NewRouter(client, WithErrorHandler(func(err error) {
		// Error expected for invalid payloads; ensure no panic.
	}))

	tests := []struct {
		name    string
		payload json.RawMessage
	}{
		{
			name:    "malformed JSON",
			payload: json.RawMessage(`{invalid json`),
		},
		{
			name:    "array instead of object",
			payload: json.RawMessage(`[1,2,3]`),
		},
		{
			name:    "number instead of object",
			payload: json.RawMessage(`12345`),
		},
		{
			name:    "string instead of object",
			payload: json.RawMessage(`"not an object"`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := ws.AgentMessage{
				Type:      ws.AgentMsgComputerSessionStart,
				ID:        "msg-cs-invalid-001",
				Timestamp: time.Now(),
				Payload:   tt.payload,
			}

			// Must not panic regardless of client connection state.
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("handleComputerSessionStart panicked on %s: %v", tt.name, r)
					}
				}()
				// Error is expected (either parse error or send failure).
				_ = router.HandleMessage(context.Background(), msg)
			}()
		})
	}
}

// TestHandleComputerAction_InvalidPayload verifies that invalid JSON
// in a computer_action message does not crash the router.
func TestHandleComputerAction_InvalidPayload(t *testing.T) {
	client := NewClient("ws://localhost:9999/ws", "test-token", "1.0.0")
	router := NewRouter(client, WithErrorHandler(func(err error) {}))

	tests := []struct {
		name    string
		payload json.RawMessage
	}{
		{
			name:    "malformed JSON",
			payload: json.RawMessage(`{broken`),
		},
		{
			name:    "null payload",
			payload: json.RawMessage(`null`),
		},
		{
			name:    "empty string",
			payload: json.RawMessage(`""`),
		},
		{
			name:    "nested invalid",
			payload: json.RawMessage(`{"action": {"nested": "wrong type"}}`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := ws.AgentMessage{
				Type:      ws.AgentMsgComputerAction,
				ID:        "msg-ca-invalid-001",
				Timestamp: time.Now(),
				Payload:   tt.payload,
			}

			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("handleComputerAction panicked on %s: %v", tt.name, r)
					}
				}()
				_ = router.HandleMessage(context.Background(), msg)
			}()
		})
	}
}

// TestHandleComputerSessionEnd_InvalidPayload verifies that invalid JSON
// in a computer_session_end message does not crash the router.
func TestHandleComputerSessionEnd_InvalidPayload(t *testing.T) {
	client := NewClient("ws://localhost:9999/ws", "test-token", "1.0.0")
	router := NewRouter(client, WithErrorHandler(func(err error) {}))

	tests := []struct {
		name    string
		payload json.RawMessage
	}{
		{
			name:    "malformed JSON",
			payload: json.RawMessage(`not json at all`),
		},
		{
			name:    "boolean instead of object",
			payload: json.RawMessage(`true`),
		},
		{
			name:    "empty array",
			payload: json.RawMessage(`[]`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := ws.AgentMessage{
				Type:      ws.AgentMsgComputerSessionEnd,
				ID:        "msg-ce-invalid-001",
				Timestamp: time.Now(),
				Payload:   tt.payload,
			}

			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("handleComputerSessionEnd panicked on %s: %v", tt.name, r)
					}
				}()
				_ = router.HandleMessage(context.Background(), msg)
			}()
		})
	}
}

// TestHandleComputerAction_ValidPayload verifies that a well-formed
// computer_action payload is parsed without panic. The actual action
// execution may fail (no handler, no connection) but parsing should succeed.
func TestHandleComputerAction_ValidPayload(t *testing.T) {
	client := NewClient("ws://localhost:9999/ws", "test-token", "1.0.0")
	router := NewRouter(client, WithErrorHandler(func(err error) {}))

	payload, err := json.Marshal(ws.ComputerActionPayload{
		ExecutionID: "exec-cu-001",
		SessionID:   "session-001",
		Action:      "screenshot",
		Params:      map[string]interface{}{"format": "png"},
	})
	if err != nil {
		t.Fatalf("failed to marshal ComputerActionPayload: %v", err)
	}

	msg := ws.AgentMessage{
		Type:      ws.AgentMsgComputerAction,
		ID:        "msg-ca-valid-001",
		Timestamp: time.Now(),
		Payload:   payload,
	}

	// Must not panic. Error is expected because computerUseHandler is nil
	// and client is not connected, but the JSON parsing itself should work.
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("handleComputerAction panicked on valid payload: %v", r)
			}
		}()
		_ = router.HandleMessage(context.Background(), msg)
	}()
}

// TestHandleComputerSessionStart_ValidPayload verifies parsing of a well-formed
// computer_session_start payload without panic.
func TestHandleComputerSessionStart_ValidPayload(t *testing.T) {
	client := NewClient("ws://localhost:9999/ws", "test-token", "1.0.0")
	router := NewRouter(client, WithErrorHandler(func(err error) {}))

	payload, err := json.Marshal(ws.ComputerSessionPayload{
		ExecutionID: "exec-cu-002",
		SessionID:   "session-002",
		URL:         "https://example.com",
		ViewportW:   1280,
		ViewportH:   720,
		Headless:    true,
	})
	if err != nil {
		t.Fatalf("failed to marshal ComputerSessionPayload: %v", err)
	}

	msg := ws.AgentMessage{
		Type:      ws.AgentMsgComputerSessionStart,
		ID:        "msg-cs-valid-001",
		Timestamp: time.Now(),
		Payload:   payload,
	}

	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("handleComputerSessionStart panicked on valid payload: %v", r)
			}
		}()
		_ = router.HandleMessage(context.Background(), msg)
	}()
}

// TestHandleComputerSessionEnd_ValidPayload verifies parsing of a well-formed
// computer_session_end payload without panic.
func TestHandleComputerSessionEnd_ValidPayload(t *testing.T) {
	client := NewClient("ws://localhost:9999/ws", "test-token", "1.0.0")
	router := NewRouter(client, WithErrorHandler(func(err error) {}))

	payload, err := json.Marshal(ws.ComputerSessionPayload{
		ExecutionID: "exec-cu-003",
		SessionID:   "session-003",
	})
	if err != nil {
		t.Fatalf("failed to marshal ComputerSessionPayload: %v", err)
	}

	msg := ws.AgentMessage{
		Type:      ws.AgentMsgComputerSessionEnd,
		ID:        "msg-ce-valid-001",
		Timestamp: time.Now(),
		Payload:   payload,
	}

	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("handleComputerSessionEnd panicked on valid payload: %v", r)
			}
		}()
		err := router.HandleMessage(context.Background(), msg)
		// With no computerUseHandler set, session end with valid payload
		// should return nil (the handler checks nil and returns nil).
		if err != nil {
			t.Errorf("expected nil error for session end with nil handler, got: %v", err)
		}
	}()
}

// TestHandleComputerSessionEnd_NilHandler_ReturnsNil verifies that
// handleComputerSessionEnd with nil computerUseHandler returns nil (graceful no-op).
func TestHandleComputerSessionEnd_NilHandler_ReturnsNil(t *testing.T) {
	client := NewClient("ws://localhost:9999/ws", "test-token", "1.0.0")
	// Create router WITHOUT ComputerUseHandler option.
	router := NewRouter(client)

	payload, _ := json.Marshal(ws.ComputerSessionPayload{
		ExecutionID: "exec-cu-end",
		SessionID:   "session-end-001",
	})

	msg := ws.AgentMessage{
		Type:      ws.AgentMsgComputerSessionEnd,
		ID:        "msg-ce-nil-001",
		Timestamp: time.Now(),
		Payload:   payload,
	}

	err := router.HandleMessage(context.Background(), msg)
	if err != nil {
		t.Errorf("HandleMessage returned error %v, want nil for nil computerUseHandler", err)
	}
}

// TestComputerUseHandlersRegistered verifies that all three computer use
// message types are registered as handlers in the router.
func TestComputerUseHandlersRegistered(t *testing.T) {
	client := NewClient("ws://localhost:9999/ws", "test-token", "1.0.0")
	router := NewRouter(client)

	expectedTypes := []string{
		ws.AgentMsgComputerSessionStart,
		ws.AgentMsgComputerAction,
		ws.AgentMsgComputerSessionEnd,
	}

	router.handlersMu.RLock()
	defer router.handlersMu.RUnlock()

	for _, msgType := range expectedTypes {
		if _, exists := router.handlers[msgType]; !exists {
			t.Errorf("handler for %q not registered", msgType)
		}
	}
}

// TestWithComputerUseHandler_Option verifies the RouterOption correctly sets
// the computerUseHandler field.
func TestWithComputerUseHandler_Option(t *testing.T) {
	client := NewClient("ws://localhost:9999/ws", "test-token", "1.0.0")

	// Without option: handler should be nil.
	router1 := NewRouter(client)
	if router1.computerUseHandler != nil {
		t.Error("computerUseHandler should be nil when option is not provided")
	}

	// With nil option: handler should remain nil.
	router2 := NewRouter(client, WithComputerUseHandler(nil))
	if router2.computerUseHandler != nil {
		t.Error("computerUseHandler should be nil when passed nil")
	}
}
