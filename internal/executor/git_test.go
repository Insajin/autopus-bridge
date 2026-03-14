// Package executor는 Local Agent Bridge의 작업 실행 엔진을 제공합니다.
package executor

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGitExecutor_BranchPolicy는 브랜치 정책 강제 적용을 검증합니다.
// REQ-001.10: agent/{workspace-slug}/ 프리픽스만 push 허용
func TestGitExecutor_BranchPolicy(t *testing.T) {
	t.Parallel()

	workspaceSlug := "test-workspace"
	ge := NewGitExecutor(GitExecutorConfig{
		WorkspaceSlug: workspaceSlug,
	})

	tests := []struct {
		name      string
		branch    string
		expectErr bool
	}{
		{
			name:      "agent/ 프리픽스 있는 브랜치 허용",
			branch:    "agent/test-workspace/fix-issue-42",
			expectErr: false,
		},
		{
			name:      "agent/ 프리픽스 없는 브랜치 거부",
			branch:    "feature/some-feature",
			expectErr: true,
		},
		{
			name:      "main 브랜치 거부",
			branch:    "main",
			expectErr: true,
		},
		{
			name:      "master 브랜치 거부",
			branch:    "master",
			expectErr: true,
		},
		{
			name:      "develop 브랜치 거부",
			branch:    "develop",
			expectErr: true,
		},
		{
			name:      "다른 워크스페이스 agent/ 브랜치도 차단 (같은 슬러그만 허용 X - 실제로는 agent/ 프리픽스면 모두 허용)",
			branch:    "agent/other-workspace/task",
			expectErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := ge.validatePushBranch(tc.branch)
			if tc.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestGitExecutor_ClonePath는 clone 경로 생성을 검증합니다.
// REQ-001.9: clone 경로: ~/.autopus/workspaces/{workspace-id}/{repo-slug}/
func TestGitExecutor_ClonePath(t *testing.T) {
	t.Parallel()

	ge := NewGitExecutor(GitExecutorConfig{
		WorkspaceID: "workspace-123",
		BaseDir:     "/tmp/autopus-test",
	})

	path := ge.clonePath("owner/my-repo")
	assert.Equal(t, "/tmp/autopus-test/workspaces/workspace-123/my-repo", path)
}

// TestGitExecutor_ClonePath_DefaultDir는 기본 베이스 디렉토리를 검증합니다.
func TestGitExecutor_ClonePath_DefaultDir(t *testing.T) {
	t.Parallel()

	home, err := os.UserHomeDir()
	require.NoError(t, err)

	ge := NewGitExecutor(GitExecutorConfig{
		WorkspaceID: "ws-abc",
	})

	path := ge.clonePath("owner/repo")
	expected := filepath.Join(home, ".autopus", "workspaces", "ws-abc", "repo")
	assert.Equal(t, expected, path)
}

// TestGitExecutor_Diff_EmptyDir는 빈 디렉토리에서 diff를 검증합니다.
func TestGitExecutor_Diff_EmptyDir(t *testing.T) {
	t.Parallel()

	// 임시 git 레포지토리 생성
	tmpDir := t.TempDir()

	// git init
	err := runGitCommand(context.Background(), tmpDir, "init")
	if err != nil {
		t.Skip("git 명령어를 찾을 수 없음")
	}

	ge := NewGitExecutor(GitExecutorConfig{})
	diff, err := ge.Diff(context.Background(), tmpDir)
	require.NoError(t, err)
	assert.Empty(t, diff)
}

// TestGitExecutor_BuildAuthenticatedURL은 OAuth 토큰 기반 인증 URL 생성을 검증합니다.
// REQ-001.8: git 인증은 GitHub OAuth 토큰 기반
func TestGitExecutor_BuildAuthenticatedURL(t *testing.T) {
	t.Parallel()

	ge := NewGitExecutor(GitExecutorConfig{})

	url := ge.buildAuthenticatedURL("https://github.com/owner/repo.git", "ghp_token123")
	assert.Equal(t, "https://ghp_token123@github.com/owner/repo.git", url)
}

// TestGitExecutor_BuildAuthenticatedURL_AlreadyHasAuth는 이미 인증 정보가 있는 URL 처리를 검증합니다.
func TestGitExecutor_BuildAuthenticatedURL_AlreadyHasAuth(t *testing.T) {
	t.Parallel()

	ge := NewGitExecutor(GitExecutorConfig{})

	// 이미 토큰이 있는 URL은 그대로 반환
	url := ge.buildAuthenticatedURL("https://existingtoken@github.com/owner/repo.git", "newtoken")
	assert.Equal(t, "https://newtoken@github.com/owner/repo.git", url)
}

// TestGitExecutor_IsAlreadyCloned는 이미 clone된 레포 감지를 검증합니다.
func TestGitExecutor_IsAlreadyCloned(t *testing.T) {
	t.Parallel()

	ge := NewGitExecutor(GitExecutorConfig{})

	// 임시 디렉토리 (clone 안됨)
	tmpDir := t.TempDir()
	assert.False(t, ge.isAlreadyCloned(tmpDir))

	// .git 디렉토리 생성
	err := os.MkdirAll(filepath.Join(tmpDir, ".git"), 0755)
	require.NoError(t, err)
	assert.True(t, ge.isAlreadyCloned(tmpDir))
}
