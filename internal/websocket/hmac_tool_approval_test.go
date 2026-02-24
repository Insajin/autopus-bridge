package websocket

import (
	"encoding/json"
	"testing"
	"time"

	ws "github.com/insajin/autopus-agent-protocol"
)

// TestCriticalMessageTypes_ToolApproval verifies that both tool approval message
// types are registered in the criticalMessageTypes map for HMAC signing (SEC-P2-02).
func TestCriticalMessageTypes_ToolApproval(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		msgType string
	}{
		{
			name:    "tool_approval_request is critical",
			msgType: ws.AgentMsgToolApprovalReq,
		},
		{
			name:    "tool_approval_response is critical",
			msgType: ws.AgentMsgToolApprovalResp,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if !criticalMessageTypes[tt.msgType] {
				t.Errorf("criticalMessageTypes[%q] = false, want true", tt.msgType)
			}

			// Also verify via MessageSigner.IsCriticalMessage for consistency.
			signer := NewMessageSigner()
			if !signer.IsCriticalMessage(tt.msgType) {
				t.Errorf("IsCriticalMessage(%q) = false, want true", tt.msgType)
			}
		})
	}
}

// TestMessageSigner_SignToolApproval verifies that signing a tool_approval_request
// message produces a non-empty HMAC signature when a secret is set.
func TestMessageSigner_SignToolApproval(t *testing.T) {
	t.Parallel()

	signer := NewMessageSigner()
	secret := []byte("test-secret-32-bytes-long-value!")
	signer.SetSecret(secret)

	payload, err := json.Marshal(map[string]string{
		"execution_id":  "exec-ta-001",
		"approval_id":   "approval-001",
		"provider_name": "codex",
		"tool_name":     "command_execution",
	})
	if err != nil {
		t.Fatalf("payload marshal error: %v", err)
	}

	msg := &ws.AgentMessage{
		Type:      ws.AgentMsgToolApprovalReq,
		ID:        "msg-ta-001",
		Timestamp: time.Now(),
		Payload:   payload,
	}

	if err := signer.Sign(msg); err != nil {
		t.Fatalf("Sign() error: %v", err)
	}

	if msg.Signature == "" {
		t.Fatal("Sign() produced empty signature for tool_approval_request")
	}
}

// TestMessageSigner_VerifyToolApproval verifies that a signed tool approval
// message can be verified in a round-trip (sign then verify).
func TestMessageSigner_VerifyToolApproval(t *testing.T) {
	t.Parallel()

	signer := NewMessageSigner()
	secret := []byte("test-secret-32-bytes-long-value!")
	signer.SetSecret(secret)

	tests := []struct {
		name    string
		msgType string
	}{
		{
			name:    "tool_approval_request round-trip",
			msgType: ws.AgentMsgToolApprovalReq,
		},
		{
			name:    "tool_approval_response round-trip",
			msgType: ws.AgentMsgToolApprovalResp,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			payload, err := json.Marshal(map[string]string{
				"execution_id": "exec-ta-002",
				"approval_id":  "approval-002",
				"decision":     "allow",
			})
			if err != nil {
				t.Fatalf("payload marshal error: %v", err)
			}

			msg := &ws.AgentMessage{
				Type:      tt.msgType,
				ID:        "msg-ta-002",
				Timestamp: time.Now(),
				Payload:   payload,
			}

			// Sign the message.
			if err := signer.Sign(msg); err != nil {
				t.Fatalf("Sign() error: %v", err)
			}

			if msg.Signature == "" {
				t.Fatalf("Sign() produced empty signature for %s", tt.msgType)
			}

			// Verify should succeed with same signer.
			if !signer.Verify(msg) {
				t.Fatalf("Verify() returned false for validly signed %s message", tt.msgType)
			}

			// Tamper with payload and verify should fail.
			tamperedPayload, _ := json.Marshal(map[string]string{
				"execution_id": "exec-ta-002",
				"approval_id":  "approval-002",
				"decision":     "deny",
			})
			msg.Payload = tamperedPayload

			if signer.Verify(msg) {
				t.Fatalf("Verify() returned true for tampered %s message", tt.msgType)
			}
		})
	}
}

// TestMessageSigner_ToolApproval_MissingSignature verifies that a tool approval
// message without a signature is rejected when a secret is set.
func TestMessageSigner_ToolApproval_MissingSignature(t *testing.T) {
	t.Parallel()

	signer := NewMessageSigner()
	secret := []byte("test-secret-32-bytes-long-value!")
	signer.SetSecret(secret)

	payload, _ := json.Marshal(map[string]string{
		"execution_id": "exec-ta-003",
		"approval_id":  "approval-003",
	})

	tests := []struct {
		name    string
		msgType string
	}{
		{
			name:    "tool_approval_request without signature",
			msgType: ws.AgentMsgToolApprovalReq,
		},
		{
			name:    "tool_approval_response without signature",
			msgType: ws.AgentMsgToolApprovalResp,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			msg := &ws.AgentMessage{
				Type:      tt.msgType,
				ID:        "msg-ta-003",
				Timestamp: time.Now(),
				Payload:   payload,
				// Signature intentionally omitted.
			}

			if signer.Verify(msg) {
				t.Fatalf("Verify() returned true for unsigned %s message", tt.msgType)
			}
		})
	}
}
