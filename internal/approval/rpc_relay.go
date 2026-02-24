package approval

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// RPCRelay implements ApprovalRelay for Codex App Server's JSON-RPC
// approval protocol. It bridges Codex's CommandExecutionApproval and
// FileChangeApproval notifications into the provider-agnostic approval flow.
type RPCRelay struct {
	handler      ApprovalHandler
	providerName string
}

// NewRPCRelay creates a new RPCRelay for the given provider name (e.g. "codex").
func NewRPCRelay(providerName string) *RPCRelay {
	return &RPCRelay{
		providerName: providerName,
	}
}

// SupportsApproval reports that RPCRelay supports interactive approval.
func (r *RPCRelay) SupportsApproval() bool {
	return true
}

// SetApprovalHandler registers the handler that processes approval requests.
func (r *RPCRelay) SetApprovalHandler(handler ApprovalHandler) {
	r.handler = handler
}

// HandleRPCApproval processes a Codex JSON-RPC approval notification and
// returns the decision. The toolName should be a descriptive name like
// "command_execution" or "file_change". The toolInput is the raw JSON
// params from the RPC notification.
func (r *RPCRelay) HandleRPCApproval(ctx context.Context, executionID, toolName string, toolInput json.RawMessage) (ToolApprovalDecision, error) {
	if r.handler == nil {
		// No handler set: default to allow (backward-compatible auto-execute)
		return ToolApprovalDecision{
			Decision:  "allow",
			Reason:    "no approval handler configured",
			DecidedBy: "policy",
			DecidedAt: time.Now(),
		}, nil
	}

	req := ToolApprovalRequest{
		ExecutionID:  executionID,
		ApprovalID:   uuid.New().String(),
		ProviderName: r.providerName,
		ToolName:     toolName,
		ToolInput:    toolInput,
		RequestedAt:  time.Now(),
	}

	return r.handler(ctx, req)
}
