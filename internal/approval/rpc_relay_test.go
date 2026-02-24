package approval

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

// Compile-time interface compliance check: RPCRelay must satisfy ApprovalRelay.
var _ ApprovalRelay = (*RPCRelay)(nil)

// TestRPCRelay_InterfaceCompliance verifies that RPCRelay satisfies the ApprovalRelay interface
// at compile time. The var _ declaration above enforces this; this test documents the intent.
func TestRPCRelay_InterfaceCompliance(t *testing.T) {
	t.Parallel()

	relay := NewRPCRelay("codex")

	// Verify the interface methods are callable.
	_ = relay.SupportsApproval()
	relay.SetApprovalHandler(nil)
}

// TestRPCRelay_SupportsApproval verifies that SupportsApproval() always returns true.
func TestRPCRelay_SupportsApproval(t *testing.T) {
	t.Parallel()

	relay := NewRPCRelay("codex")
	if !relay.SupportsApproval() {
		t.Fatal("SupportsApproval() = false, want true")
	}
}

// TestRPCRelay_HandleRPCApproval_NoHandler verifies that when no handler is set,
// HandleRPCApproval returns an allow decision for backward compatibility.
func TestRPCRelay_HandleRPCApproval_NoHandler(t *testing.T) {
	t.Parallel()

	relay := NewRPCRelay("codex")
	toolInput := json.RawMessage(`{"command":"ls -la"}`)

	decision, err := relay.HandleRPCApproval(context.Background(), "exec-001", "command_execution", toolInput)
	if err != nil {
		t.Fatalf("HandleRPCApproval() error: %v", err)
	}
	if decision.Decision != "allow" {
		t.Errorf("Decision = %q, want %q", decision.Decision, "allow")
	}
	if decision.DecidedBy != "policy" {
		t.Errorf("DecidedBy = %q, want %q", decision.DecidedBy, "policy")
	}
	if decision.Reason == "" {
		t.Error("Reason is empty, want non-empty explanation")
	}
	if decision.DecidedAt.IsZero() {
		t.Error("DecidedAt is zero time")
	}
}

// TestRPCRelay_HandleRPCApproval_WithHandler_Allow verifies that when a handler
// returns an allow decision, the decision is propagated correctly.
func TestRPCRelay_HandleRPCApproval_WithHandler_Allow(t *testing.T) {
	t.Parallel()

	relay := NewRPCRelay("codex")

	expectedReason := "command is safe"
	relay.SetApprovalHandler(func(_ context.Context, req ToolApprovalRequest) (ToolApprovalDecision, error) {
		// Verify the request fields are propagated.
		if req.ExecutionID != "exec-002" {
			t.Errorf("handler received ExecutionID = %q, want %q", req.ExecutionID, "exec-002")
		}
		if req.ProviderName != "codex" {
			t.Errorf("handler received ProviderName = %q, want %q", req.ProviderName, "codex")
		}
		if req.ToolName != "command_execution" {
			t.Errorf("handler received ToolName = %q, want %q", req.ToolName, "command_execution")
		}

		return ToolApprovalDecision{
			Decision:  "allow",
			Reason:    expectedReason,
			DecidedBy: "agent",
			DecidedAt: time.Now(),
		}, nil
	})

	toolInput := json.RawMessage(`{"command":"echo hello"}`)
	decision, err := relay.HandleRPCApproval(context.Background(), "exec-002", "command_execution", toolInput)
	if err != nil {
		t.Fatalf("HandleRPCApproval() error: %v", err)
	}
	if decision.Decision != "allow" {
		t.Errorf("Decision = %q, want %q", decision.Decision, "allow")
	}
	if decision.Reason != expectedReason {
		t.Errorf("Reason = %q, want %q", decision.Reason, expectedReason)
	}
	if decision.DecidedBy != "agent" {
		t.Errorf("DecidedBy = %q, want %q", decision.DecidedBy, "agent")
	}
}

// TestRPCRelay_HandleRPCApproval_WithHandler_Deny verifies that when a handler
// returns a deny decision, the decision is propagated correctly.
func TestRPCRelay_HandleRPCApproval_WithHandler_Deny(t *testing.T) {
	t.Parallel()

	relay := NewRPCRelay("codex")

	expectedReason := "rm -rf is dangerous"
	relay.SetApprovalHandler(func(_ context.Context, req ToolApprovalRequest) (ToolApprovalDecision, error) {
		return ToolApprovalDecision{
			Decision:  "deny",
			Reason:    expectedReason,
			DecidedBy: "human",
			DecidedAt: time.Now(),
		}, nil
	})

	toolInput := json.RawMessage(`{"command":"rm -rf /"}`)
	decision, err := relay.HandleRPCApproval(context.Background(), "exec-003", "command_execution", toolInput)
	if err != nil {
		t.Fatalf("HandleRPCApproval() error: %v", err)
	}
	if decision.Decision != "deny" {
		t.Errorf("Decision = %q, want %q", decision.Decision, "deny")
	}
	if decision.Reason != expectedReason {
		t.Errorf("Reason = %q, want %q", decision.Reason, expectedReason)
	}
	if decision.DecidedBy != "human" {
		t.Errorf("DecidedBy = %q, want %q", decision.DecidedBy, "human")
	}
}

// TestRPCRelay_HandleRPCApproval_GeneratesUniqueIDs verifies that two
// consecutive calls to HandleRPCApproval produce different approval IDs.
func TestRPCRelay_HandleRPCApproval_GeneratesUniqueIDs(t *testing.T) {
	t.Parallel()

	relay := NewRPCRelay("codex")

	var capturedIDs []string
	relay.SetApprovalHandler(func(_ context.Context, req ToolApprovalRequest) (ToolApprovalDecision, error) {
		capturedIDs = append(capturedIDs, req.ApprovalID)
		return ToolApprovalDecision{
			Decision:  "allow",
			Reason:    "test",
			DecidedBy: "policy",
			DecidedAt: time.Now(),
		}, nil
	})

	toolInput := json.RawMessage(`{}`)

	_, err := relay.HandleRPCApproval(context.Background(), "exec-004", "tool_a", toolInput)
	if err != nil {
		t.Fatalf("first HandleRPCApproval() error: %v", err)
	}

	_, err = relay.HandleRPCApproval(context.Background(), "exec-004", "tool_b", toolInput)
	if err != nil {
		t.Fatalf("second HandleRPCApproval() error: %v", err)
	}

	if len(capturedIDs) != 2 {
		t.Fatalf("expected 2 captured IDs, got %d", len(capturedIDs))
	}
	if capturedIDs[0] == capturedIDs[1] {
		t.Errorf("two calls produced the same ApprovalID: %q", capturedIDs[0])
	}
	if capturedIDs[0] == "" || capturedIDs[1] == "" {
		t.Error("ApprovalID should not be empty")
	}
}

// TestRPCRelay_SetApprovalHandler verifies that SetApprovalHandler properly
// registers a handler and replaces the previous one.
func TestRPCRelay_SetApprovalHandler(t *testing.T) {
	t.Parallel()

	relay := NewRPCRelay("codex")
	toolInput := json.RawMessage(`{}`)

	// Initially no handler: should return allow by default.
	decision, err := relay.HandleRPCApproval(context.Background(), "exec-005", "tool", toolInput)
	if err != nil {
		t.Fatalf("HandleRPCApproval() with no handler error: %v", err)
	}
	if decision.Decision != "allow" {
		t.Errorf("no handler: Decision = %q, want %q", decision.Decision, "allow")
	}

	// Set handler that denies.
	relay.SetApprovalHandler(func(_ context.Context, _ ToolApprovalRequest) (ToolApprovalDecision, error) {
		return ToolApprovalDecision{
			Decision:  "deny",
			Reason:    "handler-1 denies",
			DecidedBy: "agent",
			DecidedAt: time.Now(),
		}, nil
	})

	decision, err = relay.HandleRPCApproval(context.Background(), "exec-006", "tool", toolInput)
	if err != nil {
		t.Fatalf("HandleRPCApproval() with deny handler error: %v", err)
	}
	if decision.Decision != "deny" {
		t.Errorf("deny handler: Decision = %q, want %q", decision.Decision, "deny")
	}

	// Replace handler with one that allows.
	relay.SetApprovalHandler(func(_ context.Context, _ ToolApprovalRequest) (ToolApprovalDecision, error) {
		return ToolApprovalDecision{
			Decision:  "allow",
			Reason:    "handler-2 allows",
			DecidedBy: "agent",
			DecidedAt: time.Now(),
		}, nil
	})

	decision, err = relay.HandleRPCApproval(context.Background(), "exec-007", "tool", toolInput)
	if err != nil {
		t.Fatalf("HandleRPCApproval() with allow handler error: %v", err)
	}
	if decision.Decision != "allow" {
		t.Errorf("allow handler: Decision = %q, want %q", decision.Decision, "allow")
	}
}

// TestRPCRelay_HandleRPCApproval_RequestFields verifies that all request fields
// are correctly populated in the ToolApprovalRequest passed to the handler.
func TestRPCRelay_HandleRPCApproval_RequestFields(t *testing.T) {
	t.Parallel()

	relay := NewRPCRelay("gemini")
	toolInput := json.RawMessage(`{"file":"/tmp/test.txt","content":"hello"}`)

	var captured ToolApprovalRequest
	relay.SetApprovalHandler(func(_ context.Context, req ToolApprovalRequest) (ToolApprovalDecision, error) {
		captured = req
		return ToolApprovalDecision{
			Decision:  "allow",
			Reason:    "ok",
			DecidedBy: "policy",
			DecidedAt: time.Now(),
		}, nil
	})

	_, err := relay.HandleRPCApproval(context.Background(), "exec-008", "file_change", toolInput)
	if err != nil {
		t.Fatalf("HandleRPCApproval() error: %v", err)
	}

	if captured.ExecutionID != "exec-008" {
		t.Errorf("ExecutionID = %q, want %q", captured.ExecutionID, "exec-008")
	}
	if captured.ProviderName != "gemini" {
		t.Errorf("ProviderName = %q, want %q", captured.ProviderName, "gemini")
	}
	if captured.ToolName != "file_change" {
		t.Errorf("ToolName = %q, want %q", captured.ToolName, "file_change")
	}
	if captured.ApprovalID == "" {
		t.Error("ApprovalID is empty")
	}
	if captured.RequestedAt.IsZero() {
		t.Error("RequestedAt is zero time")
	}

	// Verify tool input is preserved.
	var parsedInput map[string]string
	if err := json.Unmarshal(captured.ToolInput, &parsedInput); err != nil {
		t.Fatalf("ToolInput unmarshal error: %v", err)
	}
	if parsedInput["file"] != "/tmp/test.txt" {
		t.Errorf("ToolInput[file] = %q, want %q", parsedInput["file"], "/tmp/test.txt")
	}
}

// TestRPCRelay_HandleRPCApproval_HandlerError verifies that errors from the
// handler are propagated to the caller.
func TestRPCRelay_HandleRPCApproval_HandlerError(t *testing.T) {
	t.Parallel()

	relay := NewRPCRelay("codex")

	relay.SetApprovalHandler(func(_ context.Context, _ ToolApprovalRequest) (ToolApprovalDecision, error) {
		return ToolApprovalDecision{}, context.DeadlineExceeded
	})

	toolInput := json.RawMessage(`{}`)
	_, err := relay.HandleRPCApproval(context.Background(), "exec-009", "tool", toolInput)
	if err == nil {
		t.Fatal("expected error from handler, got nil")
	}
}
