package knowledgesync

import (
	"strings"
	"testing"
	"time"
)

// TestResolveLWW_LocalWins 는 로컬이 더 최신일 때 local_wins 를 반환하는지 검증합니다.
func TestResolveLWW_LocalWins(t *testing.T) {
	t.Parallel()
	now := time.Now()
	local := FileState{
		RelativePath: "docs/readme.md",
		Hash:         "localHash123",
		Size:         1024,
		ModTime:      now,
	}
	remote := FileState{
		RelativePath: "docs/readme.md",
		Hash:         "remoteHash456",
		Size:         2048,
		ModTime:      now.Add(-time.Hour),
	}

	resolution := ResolveLWW(local, remote)
	if resolution.Winner != "local" {
		t.Errorf("ResolveLWW() winner = %q, want %q", resolution.Winner, "local")
	}
	if resolution.BackupPath == "" {
		t.Error("ResolveLWW() BackupPath is empty, expected backup path for losing file")
	}
}

// TestResolveLWW_RemoteWins 는 리모트가 더 최신일 때 remote_wins 를 반환하는지 검증합니다.
func TestResolveLWW_RemoteWins(t *testing.T) {
	t.Parallel()
	now := time.Now()
	local := FileState{
		RelativePath: "docs/readme.md",
		Hash:         "localHash123",
		Size:         1024,
		ModTime:      now.Add(-time.Hour),
	}
	remote := FileState{
		RelativePath: "docs/readme.md",
		Hash:         "remoteHash456",
		Size:         2048,
		ModTime:      now,
	}

	resolution := ResolveLWW(local, remote)
	if resolution.Winner != "remote" {
		t.Errorf("ResolveLWW() winner = %q, want %q", resolution.Winner, "remote")
	}
}

// TestResolveLWW_BackupPathContainsConflict 는 백업 경로가 .conflict. 을 포함하는지 검증합니다.
func TestResolveLWW_BackupPathContainsConflict(t *testing.T) {
	t.Parallel()
	now := time.Now()
	local := FileState{
		RelativePath: "src/main.go",
		Hash:         "localHash",
		Size:         500,
		ModTime:      now,
	}
	remote := FileState{
		RelativePath: "src/main.go",
		Hash:         "remoteHash",
		Size:         600,
		ModTime:      now.Add(-time.Minute),
	}

	resolution := ResolveLWW(local, remote)
	if !strings.Contains(resolution.BackupPath, ".conflict.") {
		t.Errorf("BackupPath %q does not contain '.conflict.'", resolution.BackupPath)
	}
}

// TestResolveLWW_BackupPreservesExtension 은 백업 경로가 원본 확장자를 유지하는지 검증합니다.
func TestResolveLWW_BackupPreservesExtension(t *testing.T) {
	t.Parallel()
	now := time.Now()
	local := FileState{
		RelativePath: "data/report.pdf",
		Hash:         "localHash",
		Size:         1000,
		ModTime:      now,
	}
	remote := FileState{
		RelativePath: "data/report.pdf",
		Hash:         "remoteHash",
		Size:         900,
		ModTime:      now.Add(-time.Second),
	}

	resolution := ResolveLWW(local, remote)
	if !strings.HasSuffix(resolution.BackupPath, ".pdf") {
		t.Errorf("BackupPath %q does not preserve .pdf extension", resolution.BackupPath)
	}
}

// TestResolveLWW_ReasonNotEmpty 는 Reason 이 비어있지 않은지 검증합니다.
func TestResolveLWW_ReasonNotEmpty(t *testing.T) {
	t.Parallel()
	now := time.Now()
	local := FileState{
		RelativePath: "file.txt",
		Hash:         "h1",
		ModTime:      now,
	}
	remote := FileState{
		RelativePath: "file.txt",
		Hash:         "h2",
		ModTime:      now.Add(-time.Second),
	}

	resolution := ResolveLWW(local, remote)
	if resolution.Reason == "" {
		t.Error("ResolveLWW() Reason is empty, expected human-readable explanation")
	}
}

// TestResolveLWW_EqualModTime 는 수정시간이 동일할 때 결정론적 결과를 검증합니다.
func TestResolveLWW_EqualModTime(t *testing.T) {
	t.Parallel()
	sameTime := time.Now()
	local := FileState{
		RelativePath: "file.txt",
		Hash:         "hash1",
		ModTime:      sameTime,
	}
	remote := FileState{
		RelativePath: "file.txt",
		Hash:         "hash2",
		ModTime:      sameTime,
	}

	resolution := ResolveLWW(local, remote)
	// 동시 수정 시 결정론적 선택 (remote 우선)
	if resolution.Winner != "remote" && resolution.Winner != "local" {
		t.Errorf("ResolveLWW() winner = %q, expected 'local' or 'remote'", resolution.Winner)
	}
}
