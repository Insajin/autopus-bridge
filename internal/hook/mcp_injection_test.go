package hook

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
)

func testLogger() zerolog.Logger {
	return zerolog.New(zerolog.NewTestWriter(nil)).Level(zerolog.Disabled)
}

func TestBuildMCPServerEntry(t *testing.T) {
	t.Parallel()

	tools := []string{"executeTask", "listAgents", "getExecutionStatus"}
	injector := NewMCPInjector(9876, tools, testLogger())

	entry := injector.BuildMCPServerEntry("test-token-abc")

	if entry.Command != "npx" {
		t.Errorf("expected command 'npx', got %q", entry.Command)
	}
	if len(entry.Args) != 3 {
		t.Fatalf("expected 3 args, got %d", len(entry.Args))
	}
	if entry.Args[0] != "-y" {
		t.Errorf("expected first arg '-y', got %q", entry.Args[0])
	}
	if entry.Args[1] != "@anthropic/mcp-remote" {
		t.Errorf("expected second arg '@anthropic/mcp-remote', got %q", entry.Args[1])
	}
	expectedURL := "http://localhost:9876/mcp"
	if entry.Args[2] != expectedURL {
		t.Errorf("expected third arg %q, got %q", expectedURL, entry.Args[2])
	}
	if entry.Env["AUTOPUS_SESSION_TOKEN"] != "test-token-abc" {
		t.Errorf("expected session token 'test-token-abc', got %q", entry.Env["AUTOPUS_SESSION_TOKEN"])
	}
}

func TestBuildMCPServerEntry_NoToken(t *testing.T) {
	t.Parallel()

	injector := NewMCPInjector(8080, nil, testLogger())
	entry := injector.BuildMCPServerEntry("")

	if entry.Env != nil {
		t.Errorf("expected nil env when token is empty, got %v", entry.Env)
	}
}

func TestInjectMCPConfig_NewFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	injector := NewMCPInjector(9876, []string{"executeTask"}, testLogger())

	err := injector.InjectMCPConfig(tmpDir, "session-token-123")
	if err != nil {
		t.Fatalf("InjectMCPConfig error: %v", err)
	}

	// Verify the file was created.
	settingsPath := filepath.Join(tmpDir, ".claude", "settings.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("failed to read settings.json: %v", err)
	}

	var settings map[string]json.RawMessage
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("invalid JSON in settings.json: %v", err)
	}

	var mcpServers map[string]MCPServerConfig
	if err := json.Unmarshal(settings["mcpServers"], &mcpServers); err != nil {
		t.Fatalf("invalid mcpServers JSON: %v", err)
	}

	autopus, ok := mcpServers["autopus"]
	if !ok {
		t.Fatal("expected 'autopus' key in mcpServers")
	}
	if autopus.Command != "npx" {
		t.Errorf("expected command 'npx', got %q", autopus.Command)
	}
	if autopus.Env["AUTOPUS_SESSION_TOKEN"] != "session-token-123" {
		t.Errorf("expected session token, got %q", autopus.Env["AUTOPUS_SESSION_TOKEN"])
	}
}

func TestInjectMCPConfig_ExistingSettings(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	settingsDir := filepath.Join(tmpDir, ".claude")
	if err := os.MkdirAll(settingsDir, 0755); err != nil {
		t.Fatalf("failed to create .claude dir: %v", err)
	}

	// Create existing settings with another MCP server and a custom field.
	existing := map[string]interface{}{
		"customField": "customValue",
		"mcpServers": map[string]interface{}{
			"other-server": map[string]interface{}{
				"command": "other-cmd",
				"args":    []string{"--flag"},
			},
		},
	}
	existingJSON, _ := json.MarshalIndent(existing, "", "  ")
	settingsPath := filepath.Join(settingsDir, "settings.json")
	if err := os.WriteFile(settingsPath, existingJSON, 0644); err != nil {
		t.Fatalf("failed to write existing settings: %v", err)
	}

	injector := NewMCPInjector(5555, nil, testLogger())
	err := injector.InjectMCPConfig(tmpDir, "tok")
	if err != nil {
		t.Fatalf("InjectMCPConfig error: %v", err)
	}

	// Read back and verify both servers exist.
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("failed to read settings.json: %v", err)
	}

	var settings map[string]json.RawMessage
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// Custom field should be preserved.
	if settings["customField"] == nil {
		t.Error("expected customField to be preserved")
	}

	var mcpServers map[string]MCPServerConfig
	if err := json.Unmarshal(settings["mcpServers"], &mcpServers); err != nil {
		t.Fatalf("invalid mcpServers JSON: %v", err)
	}

	// Existing server should be preserved.
	if _, ok := mcpServers["other-server"]; !ok {
		t.Error("expected 'other-server' to be preserved in mcpServers")
	}

	// Autopus server should be added.
	autopus, ok := mcpServers["autopus"]
	if !ok {
		t.Fatal("expected 'autopus' key in mcpServers")
	}
	expectedURL := "http://localhost:5555/mcp"
	if len(autopus.Args) < 3 || autopus.Args[2] != expectedURL {
		t.Errorf("expected MCP URL %q in args, got %v", expectedURL, autopus.Args)
	}
}
