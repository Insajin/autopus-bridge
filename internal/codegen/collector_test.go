package codegen

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestNewCollector_Defaults(t *testing.T) {
	c := NewCollector()
	if c == nil {
		t.Fatal("NewCollector() returned nil")
	}
	if c.maxFiles != defaultMaxFiles {
		t.Errorf("maxFiles = %d, want %d", c.maxFiles, defaultMaxFiles)
	}
	if c.maxFileSize != defaultMaxFileSize {
		t.Errorf("maxFileSize = %d, want %d", c.maxFileSize, defaultMaxFileSize)
	}
	if c.maxTotalSize != defaultMaxTotalSize {
		t.Errorf("maxTotalSize = %d, want %d", c.maxTotalSize, defaultMaxTotalSize)
	}
}

func TestCollector_WithMaxFiles(t *testing.T) {
	c := NewCollector().WithMaxFiles(5)
	if c.maxFiles != 5 {
		t.Errorf("maxFiles = %d, want 5", c.maxFiles)
	}
}

func TestCollector_WithMaxFileSize(t *testing.T) {
	c := NewCollector().WithMaxFileSize(1024)
	if c.maxFileSize != 1024 {
		t.Errorf("maxFileSize = %d, want 1024", c.maxFileSize)
	}
}

func TestCollector_WithMaxTotalSize(t *testing.T) {
	c := NewCollector().WithMaxTotalSize(2048)
	if c.maxTotalSize != 2048 {
		t.Errorf("maxTotalSize = %d, want 2048", c.maxTotalSize)
	}
}

func TestCollector_Chaining(t *testing.T) {
	c := NewCollector().WithMaxFiles(3).WithMaxFileSize(100).WithMaxTotalSize(500)
	if c.maxFiles != 3 {
		t.Errorf("maxFiles = %d, want 3", c.maxFiles)
	}
	if c.maxFileSize != 100 {
		t.Errorf("maxFileSize = %d, want 100", c.maxFileSize)
	}
	if c.maxTotalSize != 500 {
		t.Errorf("maxTotalSize = %d, want 500", c.maxTotalSize)
	}
}

func TestCollect_BasicFiles(t *testing.T) {
	dir := t.TempDir()

	// Create test files
	writeTestFile(t, filepath.Join(dir, "index.ts"), "console.log('hello');")
	writeTestFile(t, filepath.Join(dir, "package.json"), `{"name":"test"}`)

	c := NewCollector()
	files, err := c.Collect(dir)
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("Collect() returned %d files, want 2", len(files))
	}

	// Verify files have content
	for _, f := range files {
		if f.Content == "" {
			t.Errorf("file %q has empty content", f.Path)
		}
		if f.SizeBytes == 0 {
			t.Errorf("file %q has zero size", f.Path)
		}
	}
}

func TestCollect_NestedDirectories(t *testing.T) {
	dir := t.TempDir()

	// Create nested structure
	srcDir := filepath.Join(dir, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(dir, "package.json"), `{"name":"test"}`)
	writeTestFile(t, filepath.Join(srcDir, "index.ts"), "export default {};")

	c := NewCollector()
	files, err := c.Collect(dir)
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("Collect() returned %d files, want 2", len(files))
	}

	// Check relative paths
	paths := make(map[string]bool)
	for _, f := range files {
		paths[f.Path] = true
	}
	if !paths["package.json"] {
		t.Error("missing package.json in collected files")
	}
	if !paths[filepath.Join("src", "index.ts")] {
		t.Error("missing src/index.ts in collected files")
	}
}

func TestCollect_SkipsNodeModules(t *testing.T) {
	dir := t.TempDir()

	// Create node_modules (should be skipped)
	nmDir := filepath.Join(dir, "node_modules", "some-pkg")
	if err := os.MkdirAll(nmDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(nmDir, "index.js"), "module.exports = {};")
	writeTestFile(t, filepath.Join(dir, "index.ts"), "console.log('main');")

	c := NewCollector()
	files, err := c.Collect(dir)
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("Collect() returned %d files, want 1 (node_modules should be skipped)", len(files))
	}
	if files[0].Path != "index.ts" {
		t.Errorf("path = %q, want %q", files[0].Path, "index.ts")
	}
}

func TestCollect_SkipsGitDir(t *testing.T) {
	dir := t.TempDir()

	gitDir := filepath.Join(dir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(gitDir, "HEAD"), "ref: refs/heads/main")
	writeTestFile(t, filepath.Join(dir, "main.ts"), "export {};")

	c := NewCollector()
	files, err := c.Collect(dir)
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("Collect() returned %d files, want 1 (.git should be skipped)", len(files))
	}
}

func TestCollect_SkipsDist(t *testing.T) {
	dir := t.TempDir()

	distDir := filepath.Join(dir, "dist")
	if err := os.MkdirAll(distDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(distDir, "bundle.js"), "compiled code")
	writeTestFile(t, filepath.Join(dir, "src.ts"), "source code")

	c := NewCollector()
	files, err := c.Collect(dir)
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("Collect() returned %d files, want 1 (dist should be skipped)", len(files))
	}
}

func TestCollect_BinaryFileDetection(t *testing.T) {
	dir := t.TempDir()

	// Create a binary file (contains null bytes)
	binaryContent := []byte{0x89, 0x50, 0x4E, 0x47, 0x00, 0x00, 0x01, 0x02}
	if err := os.WriteFile(filepath.Join(dir, "image.png"), binaryContent, 0644); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(dir, "readme.md"), "# Readme")

	c := NewCollector()
	files, err := c.Collect(dir)
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("Collect() returned %d files, want 1 (binary should be skipped)", len(files))
	}
	if files[0].Path != "readme.md" {
		t.Errorf("path = %q, want %q", files[0].Path, "readme.md")
	}
}

func TestCollect_FileCountLimit(t *testing.T) {
	dir := t.TempDir()

	// Create more files than the limit
	c := NewCollector().WithMaxFiles(2)
	for i := 0; i < 5; i++ {
		writeTestFile(t, filepath.Join(dir, fmt.Sprintf("file%d.txt", i)), "content")
	}

	files, err := c.Collect(dir)
	if err == nil {
		t.Fatal("Collect() expected error for file count limit exceeded, got nil")
	}
	// Some files should still be collected before the limit is hit
	if len(files) < 2 {
		t.Errorf("Collect() returned %d files, want at least 2", len(files))
	}
}

func TestCollect_SingleFileSizeLimit(t *testing.T) {
	dir := t.TempDir()

	// Create a file larger than max file size (should be silently skipped)
	c := NewCollector().WithMaxFileSize(10)
	writeTestFile(t, filepath.Join(dir, "large.txt"), "this content is way larger than 10 bytes")
	writeTestFile(t, filepath.Join(dir, "small.txt"), "tiny")

	files, err := c.Collect(dir)
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("Collect() returned %d files, want 1 (large file should be skipped)", len(files))
	}
	if files[0].Path != "small.txt" {
		t.Errorf("path = %q, want %q", files[0].Path, "small.txt")
	}
}

func TestCollect_TotalSizeLimit(t *testing.T) {
	dir := t.TempDir()

	c := NewCollector().WithMaxTotalSize(20)
	writeTestFile(t, filepath.Join(dir, "a.txt"), "1234567890") // 10 bytes
	writeTestFile(t, filepath.Join(dir, "b.txt"), "1234567890") // 10 bytes
	writeTestFile(t, filepath.Join(dir, "c.txt"), "1234567890") // 10 bytes - would exceed 20

	_, err := c.Collect(dir)
	if err == nil {
		t.Fatal("Collect() expected error for total size limit exceeded, got nil")
	}
}

func TestCollect_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()

	c := NewCollector()
	files, err := c.Collect(dir)
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("Collect() returned %d files, want 0 for empty dir", len(files))
	}
}

func TestCollect_NonExistentDirectory(t *testing.T) {
	c := NewCollector()
	_, err := c.Collect("/nonexistent/path/does/not/exist")
	if err == nil {
		t.Fatal("Collect() expected error for non-existent directory, got nil")
	}
}

func TestCollect_FileInsteadOfDirectory(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "notadir.txt")
	writeTestFile(t, filePath, "content")

	c := NewCollector()
	_, err := c.Collect(filePath)
	if err == nil {
		t.Fatal("Collect() expected error when path is a file, not a directory")
	}
}

func TestIsBinary_TextContent(t *testing.T) {
	text := []byte("Hello, this is plain text content.\nWith newlines.\n")
	if isBinary(text) {
		t.Error("isBinary() = true for text content, want false")
	}
}

func TestIsBinary_BinaryContent(t *testing.T) {
	binary := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00}
	if !isBinary(binary) {
		t.Error("isBinary() = false for binary content (contains null byte), want true")
	}
}

func TestIsBinary_EmptyContent(t *testing.T) {
	if isBinary([]byte{}) {
		t.Error("isBinary() = true for empty content, want false")
	}
}

func TestIsBinary_NullByteAfterCheckSize(t *testing.T) {
	// Content with null byte after the binaryCheckSize boundary
	content := make([]byte, binaryCheckSize+100)
	for i := range content {
		content[i] = 'a'
	}
	content[binaryCheckSize+50] = 0x00 // null byte after check boundary

	if isBinary(content) {
		t.Error("isBinary() = true, but null byte is after check boundary, want false")
	}
}

func TestIsBinary_NullByteWithinCheckSize(t *testing.T) {
	content := make([]byte, binaryCheckSize+100)
	for i := range content {
		content[i] = 'a'
	}
	content[100] = 0x00 // null byte within check boundary

	if !isBinary(content) {
		t.Error("isBinary() = false, but null byte is within check boundary, want true")
	}
}

// --- helpers ---

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file %q: %v", path, err)
	}
}
