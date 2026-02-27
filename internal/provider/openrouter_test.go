package provider

import "testing"

func TestIsOpenRouterFormat(t *testing.T) {
	tests := []struct {
		model    string
		expected bool
	}{
		{"openai/o3-mini", true},
		{"anthropic/claude-sonnet-4-6", true},
		{"google/gemini-2.0-flash", true},
		{"o3-mini", false},
		{"claude-sonnet-4-6", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			if got := IsOpenRouterFormat(tt.model); got != tt.expected {
				t.Errorf("IsOpenRouterFormat(%q) = %v, want %v", tt.model, got, tt.expected)
			}
		})
	}
}

func TestParseOpenRouterID(t *testing.T) {
	tests := []struct {
		input      string
		wantPrefix string
		wantModel  string
	}{
		{"openai/o3-mini", "openai", "o3-mini"},
		{"anthropic/claude-sonnet-4-6", "anthropic", "claude-sonnet-4-6"},
		{"google/gemini-2.0-flash", "google", "gemini-2.0-flash"},
		{"o3-mini", "", "o3-mini"},
		{"", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			prefix, model := ParseOpenRouterID(tt.input)
			if prefix != tt.wantPrefix || model != tt.wantModel {
				t.Errorf("ParseOpenRouterID(%q) = (%q, %q), want (%q, %q)",
					tt.input, prefix, model, tt.wantPrefix, tt.wantModel)
			}
		})
	}
}

func TestResolveProviderName(t *testing.T) {
	tests := []struct {
		prefix   string
		expected string
	}{
		{"openai", "codex"},
		{"anthropic", "claude"},
		{"google", "gemini"},
		{"unknown", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.prefix, func(t *testing.T) {
			if got := ResolveProviderName(tt.prefix); got != tt.expected {
				t.Errorf("ResolveProviderName(%q) = %q, want %q", tt.prefix, got, tt.expected)
			}
		})
	}
}

func TestStripProviderPrefix(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"openai/o3-mini", "o3-mini"},
		{"anthropic/claude-sonnet-4-6", "claude-sonnet-4-6"},
		{"google/gemini-2.0-flash", "gemini-2.0-flash"},
		{"o3-mini", "o3-mini"},
		{"gpt-5-codex", "gpt-5-codex"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := StripProviderPrefix(tt.input); got != tt.expected {
				t.Errorf("StripProviderPrefix(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
