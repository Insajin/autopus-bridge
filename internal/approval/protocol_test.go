package approval

import (
	"encoding/json"
	"testing"
	"time"

	ws "github.com/insajin/autopus-agent-protocol"
)

// TestToolApprovalRequestPayload_JSON verifies that ToolApprovalRequestPayload
// can be marshaled to JSON and unmarshaled back with all fields preserved.
func TestToolApprovalRequestPayload_JSON(t *testing.T) {
	t.Parallel()

	now := time.Now().Truncate(time.Millisecond) // Truncate for JSON round-trip precision.
	toolInput := json.RawMessage(`{"command":"ls -la","cwd":"/home/user"}`)

	original := ws.ToolApprovalRequestPayload{
		ExecutionID:  "exec-proto-001",
		ApprovalID:   "approval-proto-001",
		ProviderName: "codex",
		ToolName:     "command_execution",
		ToolInput:    toolInput,
		SessionID:    "session-001",
		RequestedAt:  now,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal() error: %v", err)
	}

	var decoded ws.ToolApprovalRequestPayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal() error: %v", err)
	}

	if decoded.ExecutionID != original.ExecutionID {
		t.Errorf("ExecutionID = %q, want %q", decoded.ExecutionID, original.ExecutionID)
	}
	if decoded.ApprovalID != original.ApprovalID {
		t.Errorf("ApprovalID = %q, want %q", decoded.ApprovalID, original.ApprovalID)
	}
	if decoded.ProviderName != original.ProviderName {
		t.Errorf("ProviderName = %q, want %q", decoded.ProviderName, original.ProviderName)
	}
	if decoded.ToolName != original.ToolName {
		t.Errorf("ToolName = %q, want %q", decoded.ToolName, original.ToolName)
	}
	if decoded.SessionID != original.SessionID {
		t.Errorf("SessionID = %q, want %q", decoded.SessionID, original.SessionID)
	}
	if !decoded.RequestedAt.Equal(original.RequestedAt) {
		t.Errorf("RequestedAt = %v, want %v", decoded.RequestedAt, original.RequestedAt)
	}

	// Verify ToolInput round-trip.
	var originalInput, decodedInput map[string]string
	if err := json.Unmarshal(original.ToolInput, &originalInput); err != nil {
		t.Fatalf("original ToolInput unmarshal error: %v", err)
	}
	if err := json.Unmarshal(decoded.ToolInput, &decodedInput); err != nil {
		t.Fatalf("decoded ToolInput unmarshal error: %v", err)
	}
	if decodedInput["command"] != originalInput["command"] {
		t.Errorf("ToolInput[command] = %q, want %q", decodedInput["command"], originalInput["command"])
	}
	if decodedInput["cwd"] != originalInput["cwd"] {
		t.Errorf("ToolInput[cwd] = %q, want %q", decodedInput["cwd"], originalInput["cwd"])
	}

	// Verify JSON field names.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("raw unmarshal error: %v", err)
	}

	expectedFields := []string{
		"execution_id",
		"approval_id",
		"provider_name",
		"tool_name",
		"tool_input",
		"session_id",
		"requested_at",
	}
	for _, field := range expectedFields {
		if _, ok := raw[field]; !ok {
			t.Errorf("JSON output missing field %q", field)
		}
	}
}

// TestToolApprovalResponsePayload_JSON verifies that ToolApprovalResponsePayload
// can be marshaled to JSON and unmarshaled back with all fields preserved.
func TestToolApprovalResponsePayload_JSON(t *testing.T) {
	t.Parallel()

	now := time.Now().Truncate(time.Millisecond)

	original := ws.ToolApprovalResponsePayload{
		ExecutionID: "exec-proto-002",
		ApprovalID:  "approval-proto-002",
		Decision:    "deny",
		Reason:      "dangerous command detected",
		DecidedBy:   "agent",
		DecidedAt:   now,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal() error: %v", err)
	}

	var decoded ws.ToolApprovalResponsePayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal() error: %v", err)
	}

	if decoded.ExecutionID != original.ExecutionID {
		t.Errorf("ExecutionID = %q, want %q", decoded.ExecutionID, original.ExecutionID)
	}
	if decoded.ApprovalID != original.ApprovalID {
		t.Errorf("ApprovalID = %q, want %q", decoded.ApprovalID, original.ApprovalID)
	}
	if decoded.Decision != original.Decision {
		t.Errorf("Decision = %q, want %q", decoded.Decision, original.Decision)
	}
	if decoded.Reason != original.Reason {
		t.Errorf("Reason = %q, want %q", decoded.Reason, original.Reason)
	}
	if decoded.DecidedBy != original.DecidedBy {
		t.Errorf("DecidedBy = %q, want %q", decoded.DecidedBy, original.DecidedBy)
	}
	if !decoded.DecidedAt.Equal(original.DecidedAt) {
		t.Errorf("DecidedAt = %v, want %v", decoded.DecidedAt, original.DecidedAt)
	}

	// Verify JSON field names.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("raw unmarshal error: %v", err)
	}

	expectedFields := []string{
		"execution_id",
		"approval_id",
		"decision",
		"decided_by",
		"decided_at",
	}
	for _, field := range expectedFields {
		if _, ok := raw[field]; !ok {
			t.Errorf("JSON output missing field %q", field)
		}
	}
}

// TestToolApprovalResponsePayload_JSON_OmitsEmptyReason verifies that the Reason
// field is omitted from JSON when empty (omitempty tag).
func TestToolApprovalResponsePayload_JSON_OmitsEmptyReason(t *testing.T) {
	t.Parallel()

	original := ws.ToolApprovalResponsePayload{
		ExecutionID: "exec-proto-003",
		ApprovalID:  "approval-proto-003",
		Decision:    "allow",
		Reason:      "", // Empty reason.
		DecidedBy:   "policy",
		DecidedAt:   time.Now().Truncate(time.Millisecond),
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal() error: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("raw unmarshal error: %v", err)
	}

	// Reason has omitempty tag, so it should be absent when empty.
	if _, ok := raw["reason"]; ok {
		t.Error("JSON output includes 'reason' field when empty (should be omitted)")
	}
}

// TestTaskRequestPayload_ApprovalFields verifies that TaskRequestPayload correctly
// serializes the ApprovalPolicy and ExecutionMode fields added for
// SPEC-INTERACTIVE-CLI-001, and omits them when empty.
func TestTaskRequestPayload_ApprovalFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		approvalPolicy     string
		executionMode      string
		expectPolicyInJSON bool
		expectModeInJSON   bool
	}{
		{
			name:               "both fields populated",
			approvalPolicy:     "human-approve",
			executionMode:      "interactive",
			expectPolicyInJSON: true,
			expectModeInJSON:   true,
		},
		{
			name:               "both fields empty (omitempty)",
			approvalPolicy:     "",
			executionMode:      "",
			expectPolicyInJSON: false,
			expectModeInJSON:   false,
		},
		{
			name:               "only approval_policy set",
			approvalPolicy:     "agent-approve",
			executionMode:      "",
			expectPolicyInJSON: true,
			expectModeInJSON:   false,
		},
		{
			name:               "only execution_mode set",
			approvalPolicy:     "",
			executionMode:      "auto-execute",
			expectPolicyInJSON: false,
			expectModeInJSON:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			payload := ws.TaskRequestPayload{
				ExecutionID:    "exec-proto-004",
				Prompt:         "test prompt",
				Model:          "claude-sonnet-4-20250514",
				MaxTokens:      4096,
				Timeout:        300,
				ApprovalPolicy: tt.approvalPolicy,
				ExecutionMode:  tt.executionMode,
			}

			data, err := json.Marshal(payload)
			if err != nil {
				t.Fatalf("Marshal() error: %v", err)
			}

			var raw map[string]json.RawMessage
			if err := json.Unmarshal(data, &raw); err != nil {
				t.Fatalf("raw unmarshal error: %v", err)
			}

			_, hasPolicyField := raw["approval_policy"]
			if hasPolicyField != tt.expectPolicyInJSON {
				t.Errorf("approval_policy in JSON = %v, want %v", hasPolicyField, tt.expectPolicyInJSON)
			}

			_, hasModeField := raw["execution_mode"]
			if hasModeField != tt.expectModeInJSON {
				t.Errorf("execution_mode in JSON = %v, want %v", hasModeField, tt.expectModeInJSON)
			}

			// Round-trip: unmarshal and verify values.
			var decoded ws.TaskRequestPayload
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("Unmarshal() error: %v", err)
			}

			if decoded.ApprovalPolicy != tt.approvalPolicy {
				t.Errorf("ApprovalPolicy = %q, want %q", decoded.ApprovalPolicy, tt.approvalPolicy)
			}
			if decoded.ExecutionMode != tt.executionMode {
				t.Errorf("ExecutionMode = %q, want %q", decoded.ExecutionMode, tt.executionMode)
			}

			// Verify base fields are preserved.
			if decoded.ExecutionID != "exec-proto-004" {
				t.Errorf("ExecutionID = %q, want %q", decoded.ExecutionID, "exec-proto-004")
			}
			if decoded.Model != "claude-sonnet-4-20250514" {
				t.Errorf("Model = %q, want %q", decoded.Model, "claude-sonnet-4-20250514")
			}
		})
	}
}
