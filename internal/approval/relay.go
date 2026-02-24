// Package approval provides a provider-agnostic approval relay system
// for interactive execution with tool-level approval routing.
package approval

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// ApprovalRelay is an interface that providers implement to support
// interactive tool approval. Providers that support approval relay
// can intercept tool calls and route them through an approval flow.
type ApprovalRelay interface {
	// SupportsApproval reports whether the provider supports interactive approval.
	SupportsApproval() bool
	// SetApprovalHandler registers the handler that processes approval requests.
	SetApprovalHandler(handler ApprovalHandler)
}

// ApprovalHandler is a function that processes a tool approval request
// and returns a decision. It is called by the provider when a tool call
// requires approval.
type ApprovalHandler func(ctx context.Context, req ToolApprovalRequest) (ToolApprovalDecision, error)

// ToolApprovalRequest represents a request for tool execution approval.
// It is provider-agnostic and contains all information needed to evaluate
// whether a tool call should be allowed or denied.
type ToolApprovalRequest struct {
	// ExecutionID is the unique identifier for the execution session.
	ExecutionID string `json:"execution_id"`
	// ApprovalID is the unique identifier for this approval request.
	ApprovalID string `json:"approval_id"`
	// ProviderName is the name of the AI provider (e.g. "claude", "codex", "gemini").
	ProviderName string `json:"provider_name"`
	// ToolName is the name of the tool being invoked.
	ToolName string `json:"tool_name"`
	// ToolInput is the raw JSON input to the tool.
	ToolInput json.RawMessage `json:"tool_input"`
	// SessionID is the identifier for the current session.
	SessionID string `json:"session_id"`
	// RequestedAt is the time when the approval was requested.
	RequestedAt time.Time `json:"requested_at"`
}

// ToolApprovalDecision represents the decision made on an approval request.
type ToolApprovalDecision struct {
	// Decision is the approval decision ("allow" or "deny").
	Decision string `json:"decision"`
	// Reason is the human-readable reason for the decision.
	Reason string `json:"reason"`
	// DecidedBy indicates who made the decision ("policy", "agent", "human").
	DecidedBy string `json:"decided_by"`
	// DecidedAt is the time when the decision was made.
	DecidedAt time.Time `json:"decided_at"`
}

// ApprovalPolicy is the policy that determines how tool approvals are handled.
type ApprovalPolicy string

const (
	// ApprovalPolicyAutoExecute means no interactive mode; backward compatible.
	// Tool calls are executed immediately without any approval flow.
	ApprovalPolicyAutoExecute ApprovalPolicy = "auto-execute"
	// ApprovalPolicyAutoApprove means all tool calls are approved automatically
	// with audit logging. The caller is responsible for logging.
	ApprovalPolicyAutoApprove ApprovalPolicy = "auto-approve"
	// ApprovalPolicyAgentApprove means an AI agent evaluates the risk of
	// each tool call and decides whether to approve or deny.
	ApprovalPolicyAgentApprove ApprovalPolicy = "agent-approve"
	// ApprovalPolicyHumanApprove means the user must explicitly approve
	// or deny each tool call.
	ApprovalPolicyHumanApprove ApprovalPolicy = "human-approve"
)

// ApprovalRouter is a common router for all providers that routes
// tool approval requests based on the configured policy.
type ApprovalRouter struct {
	policy           ApprovalPolicy
	pendingApprovals sync.Map // map[string]chan ToolApprovalDecision
	timeout          time.Duration
}

// NewApprovalRouter creates a new ApprovalRouter with the given policy and timeout.
func NewApprovalRouter(policy ApprovalPolicy, timeout time.Duration) *ApprovalRouter {
	return &ApprovalRouter{
		policy:  policy,
		timeout: timeout,
	}
}

// HandleApproval processes an approval request based on the configured policy.
// For auto-execute and auto-approve policies, it returns allow immediately.
// For agent-approve and human-approve policies, it creates a pending approval
// and waits for a decision to be delivered via DeliverDecision.
func (r *ApprovalRouter) HandleApproval(ctx context.Context, req ToolApprovalRequest) (ToolApprovalDecision, error) {
	now := time.Now()

	// Auto-execute: return allow immediately (no interactive mode)
	if r.policy == ApprovalPolicyAutoExecute {
		return ToolApprovalDecision{
			Decision:  "allow",
			Reason:    "auto-execute policy: no approval required",
			DecidedBy: "policy",
			DecidedAt: now,
		}, nil
	}

	// Auto-approve: return allow immediately (caller logs audit)
	if r.policy == ApprovalPolicyAutoApprove {
		return ToolApprovalDecision{
			Decision:  "allow",
			Reason:    "auto-approve policy: approved with audit",
			DecidedBy: "policy",
			DecidedAt: now,
		}, nil
	}

	// Agent-approve or human-approve: wait for decision
	ch := make(chan ToolApprovalDecision, 1)
	r.pendingApprovals.Store(req.ApprovalID, ch)

	defer func() {
		r.pendingApprovals.Delete(req.ApprovalID)
	}()

	select {
	case decision := <-ch:
		return decision, nil
	case <-time.After(r.timeout):
		return ToolApprovalDecision{
			Decision:  "deny",
			Reason:    "approval timed out",
			DecidedBy: "policy",
			DecidedAt: time.Now(),
		}, nil
	case <-ctx.Done():
		return ToolApprovalDecision{
			Decision:  "deny",
			Reason:    fmt.Sprintf("context cancelled: %v", ctx.Err()),
			DecidedBy: "policy",
			DecidedAt: time.Now(),
		}, ctx.Err()
	}
}

// DeliverDecision delivers a decision to a pending approval request.
// Returns an error if the approval ID is not found.
func (r *ApprovalRouter) DeliverDecision(approvalID string, decision ToolApprovalDecision) error {
	val, ok := r.pendingApprovals.LoadAndDelete(approvalID)
	if !ok {
		return fmt.Errorf("approval not found: %s", approvalID)
	}

	ch, ok := val.(chan ToolApprovalDecision)
	if !ok {
		return fmt.Errorf("invalid pending approval channel for: %s", approvalID)
	}

	// Non-blocking send; if the channel already has a value or the
	// handler has timed out, we skip.
	select {
	case ch <- decision:
	default:
	}

	return nil
}

// CancelAll cancels all pending approvals by sending a deny decision
// to all waiting channels.
func (r *ApprovalRouter) CancelAll() {
	now := time.Now()
	r.pendingApprovals.Range(func(key, value any) bool {
		ch, ok := value.(chan ToolApprovalDecision)
		if ok {
			select {
			case ch <- ToolApprovalDecision{
				Decision:  "deny",
				Reason:    "all pending approvals cancelled",
				DecidedBy: "policy",
				DecidedAt: now,
			}:
			default:
			}
		}
		r.pendingApprovals.Delete(key)
		return true
	})
}
