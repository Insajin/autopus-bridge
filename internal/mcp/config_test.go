package mcp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfigPath(t *testing.T) {
	path := DefaultConfigPath()
	if path == "" {
		t.Skip("UserHomeDir not available")
	}

	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".acos", "mcp-servers.json")
	if path != expected {
		t.Errorf("DefaultConfigPath() = %q, want %q", path, expected)
	}
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	cfg, err := LoadConfig("/nonexistent/path/mcp-servers.json")
	if err != nil {
		t.Fatalf("LoadConfig() returned error for missing file: %v", err)
	}
	if cfg == nil {
		t.Fatal("LoadConfig() returned nil config")
	}
	if cfg.Servers == nil {
		t.Fatal("LoadConfig() returned nil Servers map")
	}
	if len(cfg.Servers) != 0 {
		t.Errorf("LoadConfig() Servers length = %d, want 0", len(cfg.Servers))
	}
}

func TestLoadConfig_ValidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp-servers.json")

	content := `{
		"servers": {
			"test-server": {
				"name": "test-server",
				"command": "node",
				"args": ["server.js", "--port", "3000"],
				"env": {"NODE_ENV": "production"},
				"working_dir": "/tmp",
				"required_binary": "node"
			},
			"python-mcp": {
				"name": "python-mcp",
				"command": "python3",
				"args": ["-m", "mcp_server"]
			}
		}
	}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}

	if len(cfg.Servers) != 2 {
		t.Fatalf("Servers length = %d, want 2", len(cfg.Servers))
	}

	srv, ok := cfg.Servers["test-server"]
	if !ok {
		t.Fatal("test-server not found in config")
	}
	if srv.Command != "node" {
		t.Errorf("Command = %q, want %q", srv.Command, "node")
	}
	if len(srv.Args) != 3 {
		t.Errorf("Args length = %d, want 3", len(srv.Args))
	}
	if srv.Env["NODE_ENV"] != "production" {
		t.Errorf("Env[NODE_ENV] = %q, want %q", srv.Env["NODE_ENV"], "production")
	}
	if srv.WorkingDir != "/tmp" {
		t.Errorf("WorkingDir = %q, want %q", srv.WorkingDir, "/tmp")
	}
	if srv.RequiredBinary != "node" {
		t.Errorf("RequiredBinary = %q, want %q", srv.RequiredBinary, "node")
	}
}

func TestLoadConfig_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")

	if err := os.WriteFile(path, []byte("{invalid json}"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("LoadConfig() expected error for invalid JSON, got nil")
	}
}

func TestLoadConfig_NilServersField(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.json")

	if err := os.WriteFile(path, []byte(`{}`), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}
	if cfg.Servers == nil {
		t.Fatal("Servers should be initialized even if missing from JSON")
	}
}

func TestGetServerConfig(t *testing.T) {
	cfg := &LocalConfig{
		Servers: map[string]ServerConfig{
			"existing": {
				Name:    "existing",
				Command: "echo",
				Args:    []string{"hello"},
			},
		},
	}

	t.Run("existing server", func(t *testing.T) {
		srv, ok := cfg.GetServerConfig("existing")
		if !ok {
			t.Fatal("GetServerConfig() returned false for existing server")
		}
		if srv.Command != "echo" {
			t.Errorf("Command = %q, want %q", srv.Command, "echo")
		}
	})

	t.Run("non-existing server", func(t *testing.T) {
		_, ok := cfg.GetServerConfig("nonexistent")
		if ok {
			t.Fatal("GetServerConfig() returned true for non-existing server")
		}
	})
}
