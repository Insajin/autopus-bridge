package provider

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestClaudeCLIProvider_ExecuteStreaming_UsesVerboseFlag(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script based test is not supported on Windows")
	}

	tmpDir := t.TempDir()
	argsLogPath := filepath.Join(tmpDir, "args.log")
	cliPath := filepath.Join(tmpDir, "fake-claude")

	script := "#!/bin/sh\n" +
		"set -eu\n" +
		"printf '%s\\n' \"$@\" > \"" + argsLogPath + "\"\n" +
		"has_verbose=0\n" +
		"has_stream=0\n" +
		"prev=''\n" +
		"for arg in \"$@\"; do\n" +
		"  if [ \"$arg\" = \"--verbose\" ]; then has_verbose=1; fi\n" +
		"  if [ \"$prev\" = \"--output-format\" ] && [ \"$arg\" = \"stream-json\" ]; then has_stream=1; fi\n" +
		"  prev=\"$arg\"\n" +
		"done\n" +
		"if [ \"$has_stream\" -eq 1 ] && [ \"$has_verbose\" -ne 1 ]; then\n" +
		"  echo 'Error: When using --print, --output-format=stream-json requires --verbose' >&2\n" +
		"  exit 1\n" +
		"fi\n" +
		"echo '{\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"Hel\"}}'\n" +
		"echo '{\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"lo\"}}'\n" +
		"echo '{\"type\":\"result\",\"subtype\":\"success\",\"result\":\"Hello\",\"duration_ms\":1,\"total_input_tokens\":1,\"total_output_tokens\":1}'\n"

	if err := os.WriteFile(cliPath, []byte(script), 0o755); err != nil {
		t.Fatalf("failed to create fake claude CLI: %v", err)
	}

	p, err := NewClaudeCLIProvider(
		WithCLIPath(cliPath),
		WithCLITimeout(3*time.Second),
	)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	var deltas []string
	resp, err := p.ExecuteStreaming(
		context.Background(),
		ExecuteRequest{
			Prompt: "test",
			Model:  "claude-sonnet-4-20250514",
		},
		func(textDelta, _ string) {
			deltas = append(deltas, textDelta)
		},
	)
	if err != nil {
		t.Fatalf("ExecuteStreaming failed: %v", err)
	}

	if resp.Output != "Hello" {
		t.Fatalf("unexpected output: got %q, want %q", resp.Output, "Hello")
	}

	if len(deltas) == 0 {
		t.Fatal("expected at least one streaming delta callback")
	}

	argsBytes, err := os.ReadFile(argsLogPath)
	if err != nil {
		t.Fatalf("failed to read args log: %v", err)
	}

	if !strings.Contains(string(argsBytes), "--verbose") {
		t.Fatalf("expected --verbose in CLI args, got: %s", string(argsBytes))
	}
}
