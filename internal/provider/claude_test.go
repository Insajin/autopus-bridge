package provider

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestClaudeProvider_Capabilities_SupportsComputerUse verifies that
// ClaudeProvider.Capabilities() returns SupportsComputerUse: true.
func TestClaudeProvider_Capabilities_SupportsComputerUse(t *testing.T) {
	// ClaudeProvider requires an API key to construct, so we test
	// the Capabilities method directly on a zero-value struct.
	// This tests the method implementation without needing a real API key.
	p := &ClaudeProvider{}
	caps := p.Capabilities()

	if !caps.SupportsComputerUse {
		t.Error("ClaudeProvider.Capabilities().SupportsComputerUse should be true")
	}
}

// TestContainsTool tests the containsTool helper function.
func TestContainsTool(t *testing.T) {
	tests := []struct {
		name  string
		tools []string
		tool  string
		want  bool
	}{
		{
			name:  "tool found in list",
			tools: []string{"read", "write", "computer_use"},
			tool:  "computer_use",
			want:  true,
		},
		{
			name:  "tool not in list",
			tools: []string{"read", "write"},
			tool:  "computer_use",
			want:  false,
		},
		{
			name:  "empty tools list",
			tools: []string{},
			tool:  "computer_use",
			want:  false,
		},
		{
			name:  "nil tools list",
			tools: nil,
			tool:  "computer_use",
			want:  false,
		},
		{
			name:  "single matching tool",
			tools: []string{"computer_use"},
			tool:  "computer_use",
			want:  true,
		},
		{
			name:  "partial match does not count",
			tools: []string{"computer_use_v2"},
			tool:  "computer_use",
			want:  false,
		},
		{
			name:  "case sensitive match",
			tools: []string{"Computer_Use"},
			tool:  "computer_use",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containsTool(tt.tools, tt.tool)
			if got != tt.want {
				t.Errorf("containsTool(%v, %q) = %v, want %v", tt.tools, tt.tool, got, tt.want)
			}
		})
	}
}

// TestRedactScreenshotData tests the RedactScreenshotData utility function.
func TestRedactScreenshotData(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no screenshot data - passthrough",
			input:    `{"type":"text","content":"hello world"}`,
			expected: `{"type":"text","content":"hello world"}`,
		},
		{
			name:     "image type detected (no space)",
			input:    `{"type":"image","data":"iVBORw0KGgo="}`,
			expected: "[SCREENSHOT_REDACTED]",
		},
		{
			name:     "image type detected (with space)",
			input:    `{"type": "image","data": "iVBORw0KGgo="}`,
			expected: "[SCREENSHOT_REDACTED]",
		},
		{
			name:     "media_type image detected (no space)",
			input:    `{"media_type":"image/png","data":"base64data"}`,
			expected: "[SCREENSHOT_REDACTED]",
		},
		{
			name:     "media_type image detected (with space)",
			input:    `{"media_type": "image/png","data": "base64data"}`,
			expected: "[SCREENSHOT_REDACTED]",
		},
		{
			name:     "media_type image/jpeg",
			input:    `{"media_type":"image/jpeg","data":"base64jpeg"}`,
			expected: "[SCREENSHOT_REDACTED]",
		},
		{
			name:     "short string without image markers",
			input:    "short text",
			expected: "short text",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RedactScreenshotData(tt.input)
			if got != tt.expected {
				t.Errorf("RedactScreenshotData() = %q, want %q", got, tt.expected)
			}
		})
	}

	// Test long data field redaction separately (>1024 bytes with "data" field)
	t.Run("long base64 data field redaction", func(t *testing.T) {
		// Create a string longer than 1024 bytes with a "data" field
		longData := `{"data":"` + strings.Repeat("A", 1100) + `"}`
		got := RedactScreenshotData(longData)

		if !strings.HasPrefix(got, "[SCREENSHOT_REDACTED: ") {
			t.Errorf("expected redacted prefix for long data field, got: %q", got[:50])
		}
		if !strings.HasSuffix(got, "...]") {
			t.Errorf("expected redacted suffix for long data field, got: %q", got[len(got)-10:])
		}
	})

	t.Run("long data field with space redaction", func(t *testing.T) {
		longData := `{"data": "` + strings.Repeat("B", 1100) + `"}`
		got := RedactScreenshotData(longData)

		if !strings.HasPrefix(got, "[SCREENSHOT_REDACTED: ") {
			t.Errorf("expected redacted prefix for long data field with space, got: %q", got[:50])
		}
	})

	t.Run("long string without data field - passthrough", func(t *testing.T) {
		longText := `{"content":"` + strings.Repeat("C", 1100) + `"}`
		got := RedactScreenshotData(longText)

		if got != longText {
			t.Error("long string without 'data' field should not be redacted")
		}
	})
}

// TestProviderCapabilities_SupportsComputerUse tests the ProviderCapabilities struct.
func TestProviderCapabilities_SupportsComputerUse(t *testing.T) {
	t.Run("default is false", func(t *testing.T) {
		caps := ProviderCapabilities{}
		if caps.SupportsComputerUse {
			t.Error("default SupportsComputerUse should be false")
		}
	})

	t.Run("can be set to true", func(t *testing.T) {
		caps := ProviderCapabilities{SupportsComputerUse: true}
		if !caps.SupportsComputerUse {
			t.Error("SupportsComputerUse should be true when set")
		}
	})
}

// TestToolCallJSON tests that ToolCall JSON marshaling works correctly.
func TestToolCallJSON(t *testing.T) {
	tc := ToolCall{
		ID:    "tool-123",
		Name:  "computer",
		Input: json.RawMessage(`{"action":"screenshot"}`),
	}

	data, err := json.Marshal(tc)
	if err != nil {
		t.Fatalf("failed to marshal ToolCall: %v", err)
	}

	var decoded ToolCall
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("failed to unmarshal ToolCall: %v", err)
	}

	if decoded.ID != tc.ID {
		t.Errorf("ID mismatch: got %s, want %s", decoded.ID, tc.ID)
	}
	if decoded.Name != tc.Name {
		t.Errorf("Name mismatch: got %s, want %s", decoded.Name, tc.Name)
	}
	if string(decoded.Input) != string(tc.Input) {
		t.Errorf("Input mismatch: got %s, want %s", decoded.Input, tc.Input)
	}
}
