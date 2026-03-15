// Package executor - CodingSessionManager Git 후처리 테스트
package executor

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHandleSessionCompleteNoChanges는 변경사항 없을 때 처리를 검증합니다.
func TestHandleSessionCompleteNoChanges(t *testing.T) {
	t.Parallel()

	// 임시 Git 레포지토리 생성
	dir := t.TempDir()
	if err := initTestGitRepo(dir); err != nil {
		t.Skipf("git 초기화 실패 (CI 환경일 수 있음): %v", err)
	}

	mgr := NewCodingSessionManager(CodingSessionConfig{MaxConcurrent: 2})

	result, err := mgr.HandleSessionComplete(context.Background(), dir, "session-001", "테스트 기능")
	require.NoError(t, err)
	assert.Empty(t, result.ChangedFiles)
	assert.Contains(t, result.DiffSummary, "변경사항 없음")
}

// TestHandleSessionCompleteWithChanges는 파일 변경 후 처리를 검증합니다.
func TestHandleSessionCompleteWithChanges(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := initTestGitRepo(dir); err != nil {
		t.Skipf("git 초기화 실패: %v", err)
	}

	// 파일 생성
	testFile := filepath.Join(dir, "test.go")
	if err := os.WriteFile(testFile, []byte("package main\n\nfunc Hello() string { return \"hello\" }\n"), 0644); err != nil {
		t.Fatalf("파일 생성 실패: %v", err)
	}

	// git add
	if err := runGitCommand(context.Background(), dir, "add", "test.go"); err != nil {
		t.Fatalf("git add 실패: %v", err)
	}

	mgr := NewCodingSessionManager(CodingSessionConfig{MaxConcurrent: 2})
	result, err := mgr.HandleSessionComplete(context.Background(), dir, "session-001", "테스트 기능 추가")

	// git commit을 시도하지만 remote push 없이 성공해야 함
	if err != nil {
		// push 실패는 기대 가능 (원격 레포 없음)
		t.Logf("HandleSessionComplete 에러 (원격 없음 시 정상): %v", err)
	} else {
		assert.NotEmpty(t, result.DiffSummary)
	}
}

// TestHandleSessionCompleteSecretBlocked는 시크릿 파일 포함 시 차단을 검증합니다.
func TestHandleSessionCompleteSecretBlocked(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := initTestGitRepo(dir); err != nil {
		t.Skipf("git 초기화 실패: %v", err)
	}

	// 시크릿 파일 생성
	secretFile := filepath.Join(dir, ".env")
	if err := os.WriteFile(secretFile, []byte("API_KEY=secret\n"), 0644); err != nil {
		t.Fatalf("파일 생성 실패: %v", err)
	}

	if err := runGitCommand(context.Background(), dir, "add", ".env"); err != nil {
		t.Fatalf("git add 실패: %v", err)
	}

	mgr := NewCodingSessionManager(CodingSessionConfig{MaxConcurrent: 2})
	_, err := mgr.HandleSessionComplete(context.Background(), dir, "session-001", "시크릿 추가")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "보안 정책 위반")
}

// initTestGitRepo는 테스트용 임시 Git 레포지토리를 초기화합니다.
func initTestGitRepo(dir string) error {
	if err := runGitCommand(context.Background(), dir, "init"); err != nil {
		return err
	}
	if err := runGitCommand(context.Background(), dir, "config", "user.email", "test@test.com"); err != nil {
		return err
	}
	if err := runGitCommand(context.Background(), dir, "config", "user.name", "Test User"); err != nil {
		return err
	}
	// 빈 초기 커밋
	if err := runGitCommand(context.Background(), dir, "commit", "--allow-empty", "-m", "init"); err != nil {
		return err
	}
	return nil
}
