package mcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewDeployer(t *testing.T) {
	cfg := newTestConfig(nil)
	m := NewManager(cfg)
	d := NewDeployer("/tmp/mcp-servers", m)

	if d == nil {
		t.Fatal("NewDeployer() returned nil")
	}
	if d.baseDir != "/tmp/mcp-servers" {
		t.Errorf("baseDir = %q, want %q", d.baseDir, "/tmp/mcp-servers")
	}
	if d.manager != m {
		t.Error("manager does not match provided manager")
	}
}

func TestDeployer_Deploy_EmptyServiceName(t *testing.T) {
	cfg := newTestConfig(nil)
	m := NewManager(cfg)
	d := NewDeployer(t.TempDir(), m)

	_, err := d.Deploy(t.Context(), "", nil, nil)
	if err == nil {
		t.Fatal("Deploy() expected error for empty service name, got nil")
	}
}

func TestDeployer_Deploy_CreatesDirectoryAndFiles(t *testing.T) {
	baseDir := t.TempDir()
	cfg := newTestConfig(nil)
	m := NewManager(cfg)
	d := NewDeployer(baseDir, m)

	files := []DeployFile{
		{Path: "package.json", Content: `{"name":"test-svc"}`},
		{Path: "src/index.ts", Content: "console.log('hello');"},
	}

	// Deploy will call manager.Start which will try to run "npm start" and fail.
	// But files should still be written to disk.
	serviceDir, _ := d.Deploy(t.Context(), "test-svc", files, nil)

	// Verify service directory was created
	expectedDir := filepath.Join(baseDir, "test-svc")
	if serviceDir != expectedDir {
		t.Errorf("serviceDir = %q, want %q", serviceDir, expectedDir)
	}

	// Verify files were written
	pkgContent, err := os.ReadFile(filepath.Join(expectedDir, "package.json"))
	if err != nil {
		t.Fatalf("package.json not written: %v", err)
	}
	if string(pkgContent) != `{"name":"test-svc"}` {
		t.Errorf("package.json content = %q, want %q", string(pkgContent), `{"name":"test-svc"}`)
	}

	srcContent, err := os.ReadFile(filepath.Join(expectedDir, "src", "index.ts"))
	if err != nil {
		t.Fatalf("src/index.ts not written: %v", err)
	}
	if string(srcContent) != "console.log('hello');" {
		t.Errorf("src/index.ts content = %q", string(srcContent))
	}
}

func TestDeployer_Deploy_WritesEnvFile(t *testing.T) {
	baseDir := t.TempDir()
	cfg := newTestConfig(nil)
	m := NewManager(cfg)
	d := NewDeployer(baseDir, m)

	files := []DeployFile{
		{Path: "index.ts", Content: "export {};"},
	}
	envVars := map[string]string{
		"API_KEY":    "secret123",
		"DEBUG_MODE": "true",
	}

	// Deploy may fail at manager.Start, but env file should still be written
	d.Deploy(t.Context(), "env-svc", files, envVars) //nolint:errcheck

	envPath := filepath.Join(baseDir, "env-svc", ".env")
	content, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf(".env file not written: %v", err)
	}

	envContent := string(content)
	if !strings.Contains(envContent, "API_KEY=secret123") {
		t.Error(".env missing API_KEY=secret123")
	}
	if !strings.Contains(envContent, "DEBUG_MODE=true") {
		t.Error(".env missing DEBUG_MODE=true")
	}
	// File should end with newline
	if !strings.HasSuffix(envContent, "\n") {
		t.Error(".env file does not end with newline")
	}
}

func TestDeployer_Deploy_NoEnvFileWhenEmpty(t *testing.T) {
	baseDir := t.TempDir()
	cfg := newTestConfig(nil)
	m := NewManager(cfg)
	d := NewDeployer(baseDir, m)

	files := []DeployFile{
		{Path: "index.ts", Content: "export {};"},
	}

	d.Deploy(t.Context(), "no-env-svc", files, nil) //nolint:errcheck

	envPath := filepath.Join(baseDir, "no-env-svc", ".env")
	if _, err := os.Stat(envPath); !os.IsNotExist(err) {
		t.Error(".env file should not exist when envVars is nil")
	}
}

func TestDeployer_Undeploy_EmptyServiceName(t *testing.T) {
	cfg := newTestConfig(nil)
	m := NewManager(cfg)
	d := NewDeployer(t.TempDir(), m)

	err := d.Undeploy("")
	if err == nil {
		t.Fatal("Undeploy() expected error for empty service name, got nil")
	}
}

func TestDeployer_Undeploy_RemovesDirectory(t *testing.T) {
	baseDir := t.TempDir()
	cfg := newTestConfig(nil)
	m := NewManager(cfg)
	d := NewDeployer(baseDir, m)

	// Manually create a service directory
	serviceDir := filepath.Join(baseDir, "to-remove")
	if err := os.MkdirAll(serviceDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(serviceDir, "index.ts"), []byte("code"), 0644); err != nil {
		t.Fatal(err)
	}

	// Undeploy will try to stop the server (which is not running, so it warns) then remove dir
	err := d.Undeploy("to-remove")
	if err != nil {
		t.Fatalf("Undeploy() error: %v", err)
	}

	if _, err := os.Stat(serviceDir); !os.IsNotExist(err) {
		t.Error("service directory still exists after Undeploy")
	}
}

func TestDeployer_ListDeployed_Empty(t *testing.T) {
	baseDir := t.TempDir()
	cfg := newTestConfig(nil)
	m := NewManager(cfg)
	d := NewDeployer(baseDir, m)

	services, err := d.ListDeployed()
	if err != nil {
		t.Fatalf("ListDeployed() error: %v", err)
	}
	if len(services) != 0 {
		t.Errorf("ListDeployed() returned %d services, want 0", len(services))
	}
}

func TestDeployer_ListDeployed_WithServices(t *testing.T) {
	baseDir := t.TempDir()
	cfg := newTestConfig(nil)
	m := NewManager(cfg)
	d := NewDeployer(baseDir, m)

	// Create service directories
	for _, name := range []string{"svc-a", "svc-b", "svc-c"} {
		if err := os.MkdirAll(filepath.Join(baseDir, name), 0755); err != nil {
			t.Fatal(err)
		}
	}
	// Create a file (should not be listed as a service)
	if err := os.WriteFile(filepath.Join(baseDir, "config.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	services, err := d.ListDeployed()
	if err != nil {
		t.Fatalf("ListDeployed() error: %v", err)
	}
	if len(services) != 3 {
		t.Fatalf("ListDeployed() returned %d services, want 3", len(services))
	}

	serviceSet := make(map[string]bool)
	for _, s := range services {
		serviceSet[s] = true
	}
	for _, name := range []string{"svc-a", "svc-b", "svc-c"} {
		if !serviceSet[name] {
			t.Errorf("ListDeployed() missing service %q", name)
		}
	}
}

func TestDeployer_ListDeployed_NonExistentBaseDir(t *testing.T) {
	cfg := newTestConfig(nil)
	m := NewManager(cfg)
	d := NewDeployer("/nonexistent/base/dir", m)

	services, err := d.ListDeployed()
	if err != nil {
		t.Fatalf("ListDeployed() error: %v", err)
	}
	if services != nil {
		t.Errorf("ListDeployed() = %v, want nil for non-existent dir", services)
	}
}

func TestWriteEnvFile(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	envVars := map[string]string{
		"KEY1": "value1",
		"KEY2": "value2",
	}

	err := writeEnvFile(envPath, envVars)
	if err != nil {
		t.Fatalf("writeEnvFile() error: %v", err)
	}

	content, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("failed to read env file: %v", err)
	}

	envContent := string(content)
	if !strings.Contains(envContent, "KEY1=value1") {
		t.Error("env file missing KEY1=value1")
	}
	if !strings.Contains(envContent, "KEY2=value2") {
		t.Error("env file missing KEY2=value2")
	}
	if !strings.HasSuffix(envContent, "\n") {
		t.Error("env file does not end with newline")
	}
}

func TestWriteEnvFile_SingleVar(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	err := writeEnvFile(envPath, map[string]string{"ONLY_KEY": "only_value"})
	if err != nil {
		t.Fatalf("writeEnvFile() error: %v", err)
	}

	content, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("failed to read env file: %v", err)
	}
	if string(content) != "ONLY_KEY=only_value\n" {
		t.Errorf("content = %q, want %q", string(content), "ONLY_KEY=only_value\n")
	}
}

func TestDeployer_BuildServerConfig_WithPackageJSON(t *testing.T) {
	baseDir := t.TempDir()
	serviceDir := filepath.Join(baseDir, "svc")
	if err := os.MkdirAll(serviceDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Create package.json so it picks "npm start"
	if err := os.WriteFile(filepath.Join(serviceDir, "package.json"), []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := newTestConfig(nil)
	m := NewManager(cfg)
	d := NewDeployer(baseDir, m)

	sc := d.buildServerConfig("svc", serviceDir, nil)
	if sc.Command != "npm" {
		t.Errorf("Command = %q, want %q", sc.Command, "npm")
	}
	if len(sc.Args) != 1 || sc.Args[0] != "start" {
		t.Errorf("Args = %v, want [start]", sc.Args)
	}
	if sc.WorkingDir != serviceDir {
		t.Errorf("WorkingDir = %q, want %q", sc.WorkingDir, serviceDir)
	}
}

func TestDeployer_BuildServerConfig_WithoutPackageJSON(t *testing.T) {
	baseDir := t.TempDir()
	serviceDir := filepath.Join(baseDir, "svc")
	if err := os.MkdirAll(serviceDir, 0755); err != nil {
		t.Fatal(err)
	}

	cfg := newTestConfig(nil)
	m := NewManager(cfg)
	d := NewDeployer(baseDir, m)

	sc := d.buildServerConfig("svc", serviceDir, nil)
	if sc.Command != "npx" {
		t.Errorf("Command = %q, want %q", sc.Command, "npx")
	}
	if len(sc.Args) != 2 || sc.Args[0] != "tsx" || sc.Args[1] != "src/index.ts" {
		t.Errorf("Args = %v, want [tsx src/index.ts]", sc.Args)
	}
}

func TestDeployer_BuildServerConfig_EnvVars(t *testing.T) {
	baseDir := t.TempDir()
	serviceDir := filepath.Join(baseDir, "svc")
	if err := os.MkdirAll(serviceDir, 0755); err != nil {
		t.Fatal(err)
	}

	cfg := newTestConfig(nil)
	m := NewManager(cfg)
	d := NewDeployer(baseDir, m)

	envVars := map[string]string{"API_KEY": "test"}
	sc := d.buildServerConfig("svc", serviceDir, envVars)
	if sc.Env == nil {
		t.Fatal("Env is nil")
	}
	if sc.Env["API_KEY"] != "test" {
		t.Errorf("Env[API_KEY] = %q, want %q", sc.Env["API_KEY"], "test")
	}
	if sc.Name != "svc" {
		t.Errorf("Name = %q, want %q", sc.Name, "svc")
	}
}
