package codegen

import (
	"encoding/json"
	"log/slog"
	"testing"
	"time"
)

func TestNewGenerator_Defaults(t *testing.T) {
	g := NewGenerator("/usr/bin/claude", "/tmp/work", 0, nil)
	if g == nil {
		t.Fatal("NewGenerator() returned nil")
	}
	if g.timeout != 5*time.Minute {
		t.Errorf("timeout = %v, want 5m", g.timeout)
	}
	if g.claudePath != "/usr/bin/claude" {
		t.Errorf("claudePath = %q, want %q", g.claudePath, "/usr/bin/claude")
	}
	if g.workDir != "/tmp/work" {
		t.Errorf("workDir = %q, want %q", g.workDir, "/tmp/work")
	}
	if g.logger == nil {
		t.Error("logger is nil, want default logger")
	}
}

func TestNewGenerator_CustomTimeout(t *testing.T) {
	customTimeout := 10 * time.Minute
	g := NewGenerator("claude", "/tmp", customTimeout, nil)
	if g.timeout != customTimeout {
		t.Errorf("timeout = %v, want %v", g.timeout, customTimeout)
	}
}

func TestNewGenerator_CustomLogger(t *testing.T) {
	logger := slog.Default()
	g := NewGenerator("claude", "/tmp", 0, logger)
	if g.logger != logger {
		t.Error("logger does not match provided logger")
	}
}

func TestGenerate_EmptyServiceName(t *testing.T) {
	g := NewGenerator("claude", "/tmp", 0, nil)
	req := GenerateRequest{
		ServiceName: "",
		Description: "test",
		OutputDir:   "/tmp/out",
	}
	_, err := g.Generate(t.Context(), req, nil)
	if err == nil {
		t.Fatal("Generate() expected error for empty ServiceName, got nil")
	}
}

func TestGenerate_EmptyDescription(t *testing.T) {
	g := NewGenerator("claude", "/tmp", 0, nil)
	req := GenerateRequest{
		ServiceName: "test-svc",
		Description: "",
		OutputDir:   "/tmp/out",
	}
	_, err := g.Generate(t.Context(), req, nil)
	if err == nil {
		t.Fatal("Generate() expected error for empty Description, got nil")
	}
}

func TestGenerate_EmptyOutputDir(t *testing.T) {
	g := NewGenerator("claude", "/tmp", 0, nil)
	req := GenerateRequest{
		ServiceName: "test-svc",
		Description: "a test service",
		OutputDir:   "",
	}
	_, err := g.Generate(t.Context(), req, nil)
	if err == nil {
		t.Fatal("Generate() expected error for empty OutputDir, got nil")
	}
}

func TestBuildPrompt_Basic(t *testing.T) {
	g := NewGenerator("claude", "/tmp", 0, nil)
	req := GenerateRequest{
		ServiceName: "weather-api",
		Description: "A weather API service",
	}

	prompt := g.buildPrompt(req)

	// Check that service name and description are present
	if !containsStr(prompt, "weather-api") {
		t.Error("prompt missing ServiceName")
	}
	if !containsStr(prompt, "A weather API service") {
		t.Error("prompt missing Description")
	}
	// Without TemplateID, "Template:" should not appear
	if containsStr(prompt, "- Template:") {
		t.Error("prompt should not contain Template line when TemplateID is empty")
	}
	// Without RequiredAPIs, "Required Tools" section should not appear
	if containsStr(prompt, "Required Tools") {
		t.Error("prompt should not contain Required Tools section when RequiredAPIs is empty")
	}
	// With no AuthType, should mention "none"
	if !containsStr(prompt, "none") {
		t.Error("prompt should contain 'none' auth type when AuthType is empty")
	}
}

func TestBuildPrompt_WithTemplateID(t *testing.T) {
	g := NewGenerator("claude", "/tmp", 0, nil)
	req := GenerateRequest{
		ServiceName: "svc",
		Description: "desc",
		TemplateID:  "basic-api",
	}

	prompt := g.buildPrompt(req)

	if !containsStr(prompt, "- Template: basic-api") {
		t.Error("prompt missing TemplateID")
	}
}

func TestBuildPrompt_WithRequiredAPIs(t *testing.T) {
	g := NewGenerator("claude", "/tmp", 0, nil)
	req := GenerateRequest{
		ServiceName:  "svc",
		Description:  "desc",
		RequiredAPIs: []string{"getWeather", "getForecast", "getAlerts"},
	}

	prompt := g.buildPrompt(req)

	if !containsStr(prompt, "Required Tools") {
		t.Error("prompt missing Required Tools section")
	}
	if !containsStr(prompt, "1. getWeather") {
		t.Error("prompt missing first API")
	}
	if !containsStr(prompt, "2. getForecast") {
		t.Error("prompt missing second API")
	}
	if !containsStr(prompt, "3. getAlerts") {
		t.Error("prompt missing third API")
	}
}

func TestBuildPrompt_WithAuthType(t *testing.T) {
	g := NewGenerator("claude", "/tmp", 0, nil)

	tests := []struct {
		name     string
		authType string
		expect   string
	}{
		{"api_key", "api_key", "Auth Type: api_key"},
		{"oauth2", "oauth2", "Auth Type: oauth2"},
		{"none_explicit", "none", "none (no authentication required)"},
		{"empty", "", "none (no authentication required)"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := GenerateRequest{
				ServiceName: "svc",
				Description: "desc",
				AuthType:    tc.authType,
			}
			prompt := g.buildPrompt(req)
			if !containsStr(prompt, tc.expect) {
				t.Errorf("prompt does not contain %q", tc.expect)
			}
		})
	}
}

func TestParseTokenUsage_ValidJSON(t *testing.T) {
	g := NewGenerator("claude", "/tmp", 0, nil)

	resp := claudeCLIResponse{
		Type:              "result",
		TotalInputTokens:  1000,
		TotalOutputTokens: 500,
	}
	data, _ := json.Marshal(resp)

	tokens := g.parseTokenUsage(data)
	if tokens != 1500 {
		t.Errorf("parseTokenUsage() = %d, want 1500", tokens)
	}
}

func TestParseTokenUsage_MultiLineOutput(t *testing.T) {
	g := NewGenerator("claude", "/tmp", 0, nil)

	// Simulate multi-line output where first line is not "result" type
	line1 := `{"type":"progress","result":"","total_input_tokens":100,"total_output_tokens":50}`
	line2 := `{"type":"result","result":"done","total_input_tokens":2000,"total_output_tokens":800}`

	output := []byte(line1 + "\n" + line2 + "\n")

	tokens := g.parseTokenUsage(output)
	if tokens != 2800 {
		t.Errorf("parseTokenUsage() = %d, want 2800", tokens)
	}
}

func TestParseTokenUsage_NoResultType(t *testing.T) {
	g := NewGenerator("claude", "/tmp", 0, nil)

	// Output with no "result" type line
	line := `{"type":"progress","result":"","total_input_tokens":100,"total_output_tokens":50}`
	output := []byte(line)

	tokens := g.parseTokenUsage(output)
	if tokens != 0 {
		t.Errorf("parseTokenUsage() = %d, want 0 when no result type", tokens)
	}
}

func TestParseTokenUsage_InvalidJSON(t *testing.T) {
	g := NewGenerator("claude", "/tmp", 0, nil)

	output := []byte("not valid json at all")
	tokens := g.parseTokenUsage(output)
	if tokens != 0 {
		t.Errorf("parseTokenUsage() = %d, want 0 for invalid JSON", tokens)
	}
}

func TestParseTokenUsage_EmptyOutput(t *testing.T) {
	g := NewGenerator("claude", "/tmp", 0, nil)

	tokens := g.parseTokenUsage([]byte(""))
	if tokens != 0 {
		t.Errorf("parseTokenUsage() = %d, want 0 for empty output", tokens)
	}
}

func TestParseTokenUsage_EmptyLinesAndResult(t *testing.T) {
	g := NewGenerator("claude", "/tmp", 0, nil)

	// Output with empty lines between JSON lines
	output := []byte("\n\n" + `{"type":"result","total_input_tokens":300,"total_output_tokens":200}` + "\n\n")
	tokens := g.parseTokenUsage(output)
	if tokens != 500 {
		t.Errorf("parseTokenUsage() = %d, want 500", tokens)
	}
}

func TestParseTokenUsage_ResultNotLastLine(t *testing.T) {
	g := NewGenerator("claude", "/tmp", 0, nil)

	// Result in first line, followed by non-result
	line1 := `{"type":"result","total_input_tokens":400,"total_output_tokens":100}`
	line2 := `{"type":"progress","total_input_tokens":0,"total_output_tokens":0}`

	output := []byte(line1 + "\n" + line2)

	// parseTokenUsage scans from the end, so it should skip "progress" and find "result"
	tokens := g.parseTokenUsage(output)
	if tokens != 500 {
		t.Errorf("parseTokenUsage() = %d, want 500", tokens)
	}
}

// containsStr is a helper that checks if s contains substr.
func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
