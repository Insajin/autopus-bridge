// Package hook provides a localhost HTTP hook server for Claude Code's
// PreToolUse / PostToolUse hook system. It intercepts tool calls, relays
// approval requests through a callback (typically to a WebSocket client),
// and blocks until a decision is delivered or a timeout expires.
package hook

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
)

// ApprovalDecision represents the outcome of an approval request.
type ApprovalDecision struct {
	// Allow indicates whether the tool execution is permitted.
	Allow bool `json:"allow"`
	// Reason provides a human-readable explanation for the decision.
	Reason string `json:"reason,omitempty"`
}

// ApprovalManager coordinates pending approval requests for the hook flow.
// Each request creates a buffered channel keyed by approval ID; the caller
// blocks until a decision is delivered or the configured timeout elapses.
type ApprovalManager struct {
	pending sync.Map // map[string]chan ApprovalDecision
	timeout time.Duration
	count   atomic.Int64
	logger  zerolog.Logger
}

// ErrApprovalTimeout is returned when no decision arrives within the
// configured timeout window.
var ErrApprovalTimeout = errors.New("approval request timed out")

// ErrApprovalNotFound is returned when DeliverDecision is called with
// an approval ID that has no pending request.
var ErrApprovalNotFound = errors.New("approval ID not found")

// NewApprovalManager creates an ApprovalManager with the given timeout
// and logger. If timeout is zero or negative, a default of 5 minutes
// is used.
func NewApprovalManager(timeout time.Duration, logger zerolog.Logger) *ApprovalManager {
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	return &ApprovalManager{
		timeout: timeout,
		logger:  logger.With().Str("component", "hook-approval").Logger(),
	}
}

// RequestApproval creates a pending approval channel for approvalID,
// then blocks until a decision is delivered via DeliverDecision or the
// context/timeout expires. On timeout the request is automatically
// denied and cleaned up.
func (m *ApprovalManager) RequestApproval(ctx context.Context, approvalID string) (ApprovalDecision, error) {
	ch := make(chan ApprovalDecision, 1)
	m.pending.Store(approvalID, ch)
	m.count.Add(1)

	m.logger.Debug().
		Str("approval_id", approvalID).
		Dur("timeout", m.timeout).
		Msg("approval request registered, waiting for decision")

	defer func() {
		m.pending.Delete(approvalID)
		m.count.Add(-1)
	}()

	timer := time.NewTimer(m.timeout)
	defer timer.Stop()

	select {
	case decision := <-ch:
		m.logger.Info().
			Str("approval_id", approvalID).
			Bool("allow", decision.Allow).
			Str("reason", decision.Reason).
			Msg("approval decision received")
		return decision, nil
	case <-timer.C:
		m.logger.Warn().
			Str("approval_id", approvalID).
			Msg("approval request timed out, denying by default")
		return ApprovalDecision{Allow: false, Reason: "approval timed out"}, ErrApprovalTimeout
	case <-ctx.Done():
		m.logger.Warn().
			Str("approval_id", approvalID).
			Msg("approval request cancelled via context")
		return ApprovalDecision{Allow: false, Reason: "request cancelled"}, ctx.Err()
	}
}

// DeliverDecision sends a decision to the pending approval channel
// identified by approvalID. Returns ErrApprovalNotFound if no pending
// request exists for the given ID.
func (m *ApprovalManager) DeliverDecision(approvalID string, decision ApprovalDecision) error {
	val, ok := m.pending.Load(approvalID)
	if !ok {
		m.logger.Warn().
			Str("approval_id", approvalID).
			Msg("no pending approval found for delivery")
		return ErrApprovalNotFound
	}

	ch := val.(chan ApprovalDecision)
	select {
	case ch <- decision:
		m.logger.Debug().
			Str("approval_id", approvalID).
			Bool("allow", decision.Allow).
			Msg("decision delivered to pending approval")
	default:
		// Channel already has a decision buffered; drop duplicate.
		m.logger.Warn().
			Str("approval_id", approvalID).
			Msg("decision already buffered, ignoring duplicate")
	}
	return nil
}

// CancelAll sends a deny decision to every pending approval and clears
// the internal map. This should be called during graceful shutdown.
func (m *ApprovalManager) CancelAll() {
	deny := ApprovalDecision{Allow: false, Reason: "session cancelled"}
	cancelled := 0

	m.pending.Range(func(key, value any) bool {
		ch := value.(chan ApprovalDecision)
		select {
		case ch <- deny:
		default:
		}
		m.pending.Delete(key)
		cancelled++
		return true
	})

	m.count.Store(0)
	m.logger.Info().
		Int("cancelled", cancelled).
		Msg("all pending approvals cancelled")
}

// PendingCount returns the number of currently pending approval requests.
func (m *ApprovalManager) PendingCount() int {
	return int(m.count.Load())
}
