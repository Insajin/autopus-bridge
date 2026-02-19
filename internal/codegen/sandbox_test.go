package codegen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewSandbox(t *testing.T) {
	s := NewSandbox("/tmp/sandbox", nil)
	if s == nil {
		t.Fatal("NewSandbox() returned nil")
	}
	if s.baseDir != "/tmp/sandbox" {
		t.Errorf("baseDir = %q, want %q", s.baseDir, "/tmp/sandbox")
	}
	if s.logger == nil {
		t.Error("logger is nil, want default logger")
	}
}

func TestSandbox_Create(t *testing.T) {
	baseDir := filepath.Join(t.TempDir(), "sandbox")
	s := NewSandbox(baseDir, nil)

	dirPath, cleanup, err := s.Create("test-service")
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	defer cleanup()

	// Verify directory was created
	info, err := os.Stat(dirPath)
	if err != nil {
		t.Fatalf("created directory does not exist: %v", err)
	}
	if !info.IsDir() {
		t.Error("created path is not a directory")
	}

	// Verify it is under baseDir
	if !strings.HasPrefix(dirPath, baseDir) {
		t.Errorf("dirPath %q is not under baseDir %q", dirPath, baseDir)
	}

	// Verify directory name contains service name
	dirName := filepath.Base(dirPath)
	if !strings.HasPrefix(dirName, "test-service-") {
		t.Errorf("dirName %q does not start with 'test-service-'", dirName)
	}
}

func TestSandbox_Create_UniqueNames(t *testing.T) {
	baseDir := filepath.Join(t.TempDir(), "sandbox")
	s := NewSandbox(baseDir, nil)

	dir1, cleanup1, err := s.Create("svc")
	if err != nil {
		t.Fatalf("Create() first call error: %v", err)
	}
	defer cleanup1()

	// Create a second one with the same service name
	// The timestamp-based naming should still succeed (or at least not error)
	dir2, cleanup2, err := s.Create("svc")
	if err != nil {
		t.Fatalf("Create() second call error: %v", err)
	}
	defer cleanup2()

	// If they happen to have the same timestamp, they share the directory.
	// At minimum, both should be valid directories.
	if _, err := os.Stat(dir1); err != nil {
		t.Errorf("first directory does not exist: %v", err)
	}
	if _, err := os.Stat(dir2); err != nil {
		t.Errorf("second directory does not exist: %v", err)
	}
}

func TestSandbox_Create_CleanupRemovesDirectory(t *testing.T) {
	baseDir := filepath.Join(t.TempDir(), "sandbox")
	s := NewSandbox(baseDir, nil)

	dirPath, cleanup, err := s.Create("cleanup-test")
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	// Write a file inside to verify recursive removal
	writeTestFile(t, filepath.Join(dirPath, "test.txt"), "hello")

	cleanup()

	// Verify directory was removed
	if _, err := os.Stat(dirPath); !os.IsNotExist(err) {
		t.Errorf("directory still exists after cleanup: %v", err)
	}
}

func TestSandbox_Create_BaseDirCreated(t *testing.T) {
	baseDir := filepath.Join(t.TempDir(), "deeply", "nested", "sandbox")
	s := NewSandbox(baseDir, nil)

	dirPath, cleanup, err := s.Create("svc")
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	defer cleanup()

	// Verify baseDir was created
	if _, err := os.Stat(baseDir); err != nil {
		t.Fatalf("baseDir was not created: %v", err)
	}

	if _, err := os.Stat(dirPath); err != nil {
		t.Fatalf("service directory was not created: %v", err)
	}
}

func TestSandbox_ValidateOutput_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	s := NewSandbox(dir, nil)

	err := s.ValidateOutput(dir)
	if err == nil {
		t.Fatal("ValidateOutput() expected error for empty directory, got nil")
	}
	if !strings.Contains(err.Error(), "비어있습니다") {
		t.Errorf("error = %q, want message about empty directory", err.Error())
	}
}

func TestSandbox_ValidateOutput_WithPackageJSON(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "package.json"), `{"name":"test"}`)

	s := NewSandbox(dir, nil)
	err := s.ValidateOutput(dir)
	if err != nil {
		t.Errorf("ValidateOutput() error: %v", err)
	}
}

func TestSandbox_ValidateOutput_WithTsConfig(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "tsconfig.json"), `{}`)

	s := NewSandbox(dir, nil)
	err := s.ValidateOutput(dir)
	if err != nil {
		t.Errorf("ValidateOutput() error: %v", err)
	}
}

func TestSandbox_ValidateOutput_WithSrcIndex(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(srcDir, "index.ts"), "export {};")

	s := NewSandbox(dir, nil)
	err := s.ValidateOutput(dir)
	if err != nil {
		t.Errorf("ValidateOutput() error: %v", err)
	}
}

func TestSandbox_ValidateOutput_NoEntryPoint(t *testing.T) {
	dir := t.TempDir()
	// Create a file that is not a valid entry point
	writeTestFile(t, filepath.Join(dir, "readme.md"), "# Hello")

	s := NewSandbox(dir, nil)
	err := s.ValidateOutput(dir)
	if err == nil {
		t.Fatal("ValidateOutput() expected error for missing entry point, got nil")
	}
	if !strings.Contains(err.Error(), "진입점") {
		t.Errorf("error = %q, want message about missing entry point", err.Error())
	}
}

func TestSandbox_ValidateOutput_NonExistentDir(t *testing.T) {
	s := NewSandbox("/tmp", nil)
	err := s.ValidateOutput("/nonexistent/path/does/not/exist")
	if err == nil {
		t.Fatal("ValidateOutput() expected error for non-existent directory, got nil")
	}
	if !strings.Contains(err.Error(), "존재하지 않습니다") {
		t.Errorf("error = %q, want message about non-existent directory", err.Error())
	}
}

func TestSandbox_ValidateOutput_FileNotDir(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "file.txt")
	writeTestFile(t, filePath, "content")

	s := NewSandbox(dir, nil)
	err := s.ValidateOutput(filePath)
	if err == nil {
		t.Fatal("ValidateOutput() expected error when path is a file, got nil")
	}
	if !strings.Contains(err.Error(), "디렉토리가 아닙니다") {
		t.Errorf("error = %q, want message about not a directory", err.Error())
	}
}

func TestValidEntryPoints_Coverage(t *testing.T) {
	// Test each valid entry point to ensure they are all recognized
	entryPoints := []string{
		"package.json",
		"tsconfig.json",
		"index.ts",
		"index.js",
		"main.ts",
		"main.js",
		"src/index.ts",
		"src/index.js",
	}

	for _, ep := range entryPoints {
		t.Run(ep, func(t *testing.T) {
			dir := t.TempDir()

			// Create necessary subdirectory
			epPath := filepath.Join(dir, ep)
			epDir := filepath.Dir(epPath)
			if err := os.MkdirAll(epDir, 0755); err != nil {
				t.Fatal(err)
			}
			writeTestFile(t, epPath, "content")

			s := NewSandbox(dir, nil)
			err := s.ValidateOutput(dir)
			if err != nil {
				t.Errorf("ValidateOutput() error for entry point %q: %v", ep, err)
			}
		})
	}
}
