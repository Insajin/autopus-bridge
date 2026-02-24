package hook

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// GenerateHookConfig creates a Claude Code settings.json-compatible
// hook configuration map. The returned structure can be merged into
// an existing settings.json or written as a standalone file.
//
// The generated hooks use a bash command that:
//  1. Reads stdin (tool info from Claude Code)
//  2. POSTs it to the local hook server with X-Session-Token auth
//  3. Parses the JSON response for the "decision" field
//  4. Exits 0 (allow) or 2 (deny)
func GenerateHookConfig(port int, token string) map[string]any {
	preToolScript := GenerateHookScript(port, token)

	return map[string]any{
		"hooks": map[string]any{
			"PreToolUse": []map[string]any{
				{
					"matcher": "*",
					"hooks": []map[string]any{
						{
							"type":    "command",
							"command": preToolScript,
						},
					},
				},
			},
		},
	}
}

// GenerateSessionDir creates a temporary session directory containing
// a .claude/settings.json file with hook configuration. This directory
// can be used as the working directory (or CLAUDE_CONFIG_DIR) when
// launching Claude Code so that hooks are automatically active.
//
// The returned cleanup function removes the entire session directory
// tree and should be called when the session ends.
func GenerateSessionDir(baseDir, sessionID string, port int, token string) (sessionDir string, cleanup func(), err error) {
	sessionsRoot := filepath.Join(baseDir, ".autopus-sessions")
	sessionDir = filepath.Join(sessionsRoot, sessionID)
	claudeDir := filepath.Join(sessionDir, ".claude")

	if err := os.MkdirAll(claudeDir, 0700); err != nil {
		return "", nil, fmt.Errorf("failed to create session directory: %w", err)
	}

	hookCfg := GenerateHookConfig(port, token)
	data, err := json.MarshalIndent(hookCfg, "", "  ")
	if err != nil {
		// Cleanup partially created directory on marshal failure.
		os.RemoveAll(sessionDir)
		return "", nil, fmt.Errorf("failed to marshal hook config: %w", err)
	}

	settingsPath := filepath.Join(claudeDir, "settings.json")
	if err := os.WriteFile(settingsPath, data, 0600); err != nil {
		os.RemoveAll(sessionDir)
		return "", nil, fmt.Errorf("failed to write settings.json: %w", err)
	}

	cleanup = func() {
		os.RemoveAll(sessionDir)
	}

	return sessionDir, cleanup, nil
}

// GenerateHookScript returns a bash one-liner that Claude Code hooks
// will execute. It reads tool info from stdin, POSTs it to the local
// hook server, inspects the "decision" field in the response, and
// exits with the appropriate code:
//   - exit 0: allow tool execution
//   - exit 2: deny (block) tool execution
func GenerateHookScript(port int, token string) string {
	// The script uses curl to POST stdin data to the hook server.
	// jq extracts the decision field; if jq is unavailable or
	// parsing fails the default behaviour is to allow (exit 0)
	// so that missing jq does not break the entire session.
	return fmt.Sprintf(
		`bash -c 'input=$(cat); result=$(echo "$input" | curl -s -X POST -H "Content-Type: application/json" -H "X-Session-Token: %s" -d @- "http://127.0.0.1:%d/hooks/pre-tool-use" 2>/dev/null); decision=$(echo "$result" | grep -o "\"decision\":\"[^\"]*\"" | head -1 | cut -d\" -f4); if [ "$decision" = "deny" ]; then exit 2; fi; exit 0'`,
		token, port,
	)
}
