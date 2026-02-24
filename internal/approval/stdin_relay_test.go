package approval

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func stdinTestLogger() zerolog.Logger {
	return zerolog.New(zerolog.NewTestWriter(nil)).Level(zerolog.Disabled)
}

func TestStdinRelay_SupportsApproval(t *testing.T) {
	t.Parallel()

	relay := NewStdinRelay(nil, stdinTestLogger())
	if !relay.SupportsApproval() {
		t.Error("expected SupportsApproval to return true")
	}
}

func TestStdinRelay_ScanOutput_NoMatch(t *testing.T) {
	t.Parallel()

	patterns := []StdinApprovalPattern{
		{
			Name:    "test-cli",
			Pattern: regexp.MustCompile(`\[APPROVE\?\] (.+)`),
			ExtractFn: func(match string) (string, string) {
				return "test_tool", ""
			},
		},
	}

	relay := NewStdinRelay(patterns, stdinTestLogger())

	req, matched := relay.ScanOutput("this is normal output")
	if matched {
		t.Error("expected no match for normal output")
	}
	if req != nil {
		t.Error("expected nil request for no match")
	}
}

func TestStdinRelay_ScanOutput_Match(t *testing.T) {
	t.Parallel()

	patterns := []StdinApprovalPattern{
		{
			Name:    "test-cli",
			Pattern: regexp.MustCompile(`\[APPROVE\?\]\s+(\w+)`),
			ExtractFn: func(match string) (string, string) {
				// Extract tool name from the match.
				re := regexp.MustCompile(`\[APPROVE\?\]\s+(\w+)`)
				sub := re.FindStringSubmatch(match)
				if len(sub) >= 2 {
					return sub[1], "some input"
				}
				return "unknown", ""
			},
			ResponseFn: func(decision string) string {
				if decision == "allow" {
					return "y"
				}
				return "n"
			},
		},
	}

	relay := NewStdinRelay(patterns, stdinTestLogger())

	req, matched := relay.ScanOutput("[APPROVE?] RunCommand ls -la")
	if !matched {
		t.Fatal("expected match for approval prompt")
	}
	if req == nil {
		t.Fatal("expected non-nil request")
	}
	if req.ProviderName != "test-cli" {
		t.Errorf("expected provider 'test-cli', got %q", req.ProviderName)
	}
	if req.ToolName != "RunCommand" {
		t.Errorf("expected tool 'RunCommand', got %q", req.ToolName)
	}
}

func TestStdinRelay_FormatResponse(t *testing.T) {
	t.Parallel()

	patterns := []StdinApprovalPattern{
		{
			Name:    "test-cli",
			Pattern: regexp.MustCompile(`prompt`),
			ExtractFn: func(match string) (string, string) {
				return "tool", ""
			},
			ResponseFn: func(decision string) string {
				if decision == "allow" {
					return "YES"
				}
				return "NO"
			},
		},
	}

	relay := NewStdinRelay(patterns, stdinTestLogger())

	allowResp := relay.FormatResponse(ToolApprovalDecision{Decision: "allow"})
	if allowResp != "YES" {
		t.Errorf("expected 'YES', got %q", allowResp)
	}

	denyResp := relay.FormatResponse(ToolApprovalDecision{Decision: "deny"})
	if denyResp != "NO" {
		t.Errorf("expected 'NO', got %q", denyResp)
	}
}

func TestStdinRelay_FormatResponse_NoPattern(t *testing.T) {
	t.Parallel()

	// No patterns: should fall back to "yes"/"no".
	relay := NewStdinRelay(nil, stdinTestLogger())

	allowResp := relay.FormatResponse(ToolApprovalDecision{Decision: "allow"})
	if allowResp != "yes" {
		t.Errorf("expected 'yes', got %q", allowResp)
	}

	denyResp := relay.FormatResponse(ToolApprovalDecision{Decision: "deny"})
	if denyResp != "no" {
		t.Errorf("expected 'no', got %q", denyResp)
	}
}

func TestStdinRelay_HandleLine_Match(t *testing.T) {
	t.Parallel()

	patterns := []StdinApprovalPattern{
		{
			Name:    "test",
			Pattern: regexp.MustCompile(`\[APPROVE\]`),
			ExtractFn: func(match string) (string, string) {
				return "tool", ""
			},
			ResponseFn: func(decision string) string {
				return strings.ToUpper(decision)
			},
		},
	}

	relay := NewStdinRelay(patterns, stdinTestLogger())
	relay.SetApprovalHandler(func(ctx context.Context, req ToolApprovalRequest) (ToolApprovalDecision, error) {
		return ToolApprovalDecision{
			Decision:  "allow",
			Reason:    "test allow",
			DecidedBy: "test",
			DecidedAt: time.Now(),
		}, nil
	})

	resp := relay.HandleLine(context.Background(), "[APPROVE] run something")
	if resp != "ALLOW" {
		t.Errorf("expected 'ALLOW', got %q", resp)
	}
}

func TestStdinRelay_HandleLine_NoMatch(t *testing.T) {
	t.Parallel()

	relay := NewStdinRelay(nil, stdinTestLogger())
	resp := relay.HandleLine(context.Background(), "normal output line")
	if resp != "" {
		t.Errorf("expected empty string for no match, got %q", resp)
	}
}

func TestStdinRelay_HandleLine_NoHandler(t *testing.T) {
	t.Parallel()

	patterns := []StdinApprovalPattern{
		{
			Name:    "test",
			Pattern: regexp.MustCompile(`\[APPROVE\]`),
			ExtractFn: func(match string) (string, string) {
				return "tool", ""
			},
		},
	}

	relay := NewStdinRelay(patterns, stdinTestLogger())
	// No handler set, should auto-allow.
	resp := relay.HandleLine(context.Background(), "[APPROVE] something")
	if resp != "yes" {
		t.Errorf("expected 'yes' (auto-allow), got %q", resp)
	}
}
