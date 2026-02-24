package hook

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rs/zerolog"
)

// MCPServerConfig represents a single MCP server entry in Claude Code's settings.
type MCPServerConfig struct {
	// Command is the executable command for the MCP server.
	Command string `json:"command"`
	// Args is the list of arguments for the command.
	Args []string `json:"args"`
	// Env is a map of environment variables to pass to the MCP process.
	Env map[string]string `json:"env,omitempty"`
}

// mcpSettings represents the structure of .claude/settings.json
// relevant to MCP server configuration.
type mcpSettings struct {
	MCPServers map[string]MCPServerConfig `json:"mcpServers,omitempty"`
	// Preserve any other top-level fields.
	Extra map[string]json.RawMessage `json:"-"`
}

// MCPInjector handles injecting the Autopus MCP server configuration
// into a Claude Code session directory's .claude/settings.json.
// This enables Claude Code to access Autopus platform capabilities
// (executeTask, listAgents, etc.) as MCP tools during interactive sessions.
//
// SPEC-INTERACTIVE-CLI-001 Batch D, Step 23.
type MCPInjector struct {
	// bridgePort is the port where Bridge's MCP endpoint is running.
	bridgePort int
	// tools is the list of available MCP tool names.
	tools []string
	// logger is the structured logger.
	logger zerolog.Logger
}

// NewMCPInjector creates a new MCPInjector with the given bridge port and tool list.
func NewMCPInjector(bridgePort int, tools []string, logger zerolog.Logger) *MCPInjector {
	return &MCPInjector{
		bridgePort: bridgePort,
		tools:      tools,
		logger: logger.With().
			Str("component", "mcp-injector").
			Logger(),
	}
}

// BuildMCPServerEntry creates the MCP server configuration entry for Autopus.
func (m *MCPInjector) BuildMCPServerEntry(sessionToken string) MCPServerConfig {
	mcpURL := fmt.Sprintf("http://localhost:%d/mcp", m.bridgePort)

	cfg := MCPServerConfig{
		Command: "npx",
		Args:    []string{"-y", "@anthropic/mcp-remote", mcpURL},
	}

	if sessionToken != "" {
		cfg.Env = map[string]string{
			"AUTOPUS_SESSION_TOKEN": sessionToken,
		}
	}

	return cfg
}

// InjectMCPConfig reads the existing settings.json in the session directory,
// adds or updates the "autopus" MCP server entry, and writes it back.
// If the settings file does not exist, it creates a new one.
func (m *MCPInjector) InjectMCPConfig(sessionDir string, sessionToken string) error {
	settingsDir := filepath.Join(sessionDir, ".claude")
	settingsPath := filepath.Join(settingsDir, "settings.json")

	// Ensure .claude directory exists.
	if err := os.MkdirAll(settingsDir, 0755); err != nil {
		return fmt.Errorf("failed to create .claude directory: %w", err)
	}

	// Read existing settings or start with empty object.
	existing := make(map[string]json.RawMessage)
	data, err := os.ReadFile(settingsPath)
	if err == nil && len(data) > 0 {
		if jsonErr := json.Unmarshal(data, &existing); jsonErr != nil {
			m.logger.Warn().
				Str("path", settingsPath).
				Err(jsonErr).
				Msg("existing settings.json is invalid JSON, will be overwritten")
			existing = make(map[string]json.RawMessage)
		}
	}
	// If err != nil (file doesn't exist), we proceed with empty map.

	// Parse or create the mcpServers section.
	var mcpServers map[string]MCPServerConfig
	if raw, ok := existing["mcpServers"]; ok {
		if err := json.Unmarshal(raw, &mcpServers); err != nil {
			m.logger.Warn().
				Err(err).
				Msg("existing mcpServers is invalid, resetting")
			mcpServers = make(map[string]MCPServerConfig)
		}
	} else {
		mcpServers = make(map[string]MCPServerConfig)
	}

	// Set the autopus MCP server entry.
	mcpServers["autopus"] = m.BuildMCPServerEntry(sessionToken)

	// Marshal mcpServers back into existing.
	mcpRaw, err := json.Marshal(mcpServers)
	if err != nil {
		return fmt.Errorf("failed to marshal mcpServers: %w", err)
	}
	existing["mcpServers"] = mcpRaw

	// Write settings back.
	output, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	if err := os.WriteFile(settingsPath, output, 0644); err != nil {
		return fmt.Errorf("failed to write settings.json: %w", err)
	}

	m.logger.Info().
		Str("path", settingsPath).
		Int("bridge_port", m.bridgePort).
		Int("tool_count", len(m.tools)).
		Msg("MCP config injected into settings.json")

	return nil
}
