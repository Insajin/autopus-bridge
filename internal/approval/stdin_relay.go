// Package approval provides a provider-agnostic approval relay system
// for interactive execution with tool-level approval routing.
package approval

import (
	"context"
	"regexp"

	"github.com/rs/zerolog"
)

// StdinApprovalPattern defines a pattern for detecting approval requests
// in CLI output and formatting responses to send back via stdin.
// Each pattern corresponds to a specific CLI tool's approval prompt format.
type StdinApprovalPattern struct {
	// Name is a human-readable identifier for this pattern (e.g. "gemini-cli", "opencode").
	Name string
	// Pattern is the compiled regex to detect an approval request in CLI output.
	Pattern *regexp.Regexp
	// ExtractFn extracts the tool name and tool input from a matched line.
	ExtractFn func(match string) (toolName string, toolInput string)
	// ResponseFn formats the approval decision into a string to send via stdin.
	ResponseFn func(decision string) string
}

// StdinRelay is a generic stdin/stdout based approval relay for CLI tools
// that use text-based interactive prompts for tool approval (e.g. Gemini CLI, OpenCode).
//
// Unlike HookRelay (HTTP-based, for Claude Code hooks) and RPCRelay (JSON-RPC, for Codex),
// StdinRelay intercepts approval prompts from stdout and sends responses via stdin.
//
// This is a PROTOTYPE implementation. Actual pattern matching for specific CLI tools
// will be added when those tools are integrated.
//
// SPEC-INTERACTIVE-CLI-001 Batch D, Step 25.
type StdinRelay struct {
	// patterns are the registered patterns for detecting approval requests.
	patterns []StdinApprovalPattern
	// handler is the callback that processes approval requests.
	handler ApprovalHandler
	// logger is the structured logger.
	logger zerolog.Logger
}

// NewStdinRelay creates a new StdinRelay with the given patterns and logger.
func NewStdinRelay(patterns []StdinApprovalPattern, logger zerolog.Logger) *StdinRelay {
	return &StdinRelay{
		patterns: patterns,
		logger: logger.With().
			Str("component", "stdin-relay").
			Logger(),
	}
}

// SupportsApproval reports that StdinRelay supports interactive approval.
// This satisfies the ApprovalRelay interface.
func (r *StdinRelay) SupportsApproval() bool {
	return true
}

// SetApprovalHandler registers the handler that processes approval requests.
// This satisfies the ApprovalRelay interface.
func (r *StdinRelay) SetApprovalHandler(handler ApprovalHandler) {
	r.handler = handler
}

// ScanOutput checks if a line of CLI output matches any registered approval pattern.
// If a match is found, it returns a ToolApprovalRequest and true.
// If no match is found, it returns nil and false.
func (r *StdinRelay) ScanOutput(line string) (*ToolApprovalRequest, bool) {
	for _, p := range r.patterns {
		if p.Pattern.MatchString(line) {
			toolName, toolInput := p.ExtractFn(line)

			req := &ToolApprovalRequest{
				ProviderName: p.Name,
				ToolName:     toolName,
			}

			// Set tool input as JSON if provided.
			if toolInput != "" {
				req.ToolInput = []byte(`{"input":"` + toolInput + `"}`)
			}

			r.logger.Info().
				Str("pattern", p.Name).
				Str("tool", toolName).
				Msg("approval request detected in CLI output")

			return req, true
		}
	}
	return nil, false
}

// FormatResponse formats a ToolApprovalDecision into a string to send via stdin
// to the CLI tool. It uses the ResponseFn of the first matching pattern.
// If no patterns are registered, it returns a simple "yes" or "no".
func (r *StdinRelay) FormatResponse(decision ToolApprovalDecision) string {
	simple := "no"
	if decision.Decision == "allow" {
		simple = "yes"
	}

	// If we have patterns with ResponseFn, use the first one.
	for _, p := range r.patterns {
		if p.ResponseFn != nil {
			return p.ResponseFn(decision.Decision)
		}
	}

	return simple
}

// HandleLine processes a single line of CLI output. If the line matches
// an approval pattern, it calls the registered handler and returns the
// formatted response. If no match or no handler, returns empty string.
func (r *StdinRelay) HandleLine(ctx context.Context, line string) string {
	req, matched := r.ScanOutput(line)
	if !matched {
		return ""
	}

	if r.handler == nil {
		r.logger.Warn().Msg("no approval handler registered, auto-allowing")
		return r.FormatResponse(ToolApprovalDecision{Decision: "allow"})
	}

	decision, err := r.handler(ctx, *req)
	if err != nil {
		r.logger.Error().Err(err).Msg("approval handler error, denying")
		return r.FormatResponse(ToolApprovalDecision{Decision: "deny", Reason: err.Error()})
	}

	return r.FormatResponse(decision)
}
