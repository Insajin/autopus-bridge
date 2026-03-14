// Package executor는 Local Agent Bridge의 작업 실행 엔진을 제공합니다.
// REQ-001: Bridge GitExecutor — 로컬 Git 워크스페이스 관리
package executor

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// git clone 관련 타임아웃 상수
const (
	// GitCloneTimeout은 git clone 작업의 타임아웃입니다 (5분).
	GitCloneTimeout = 5 * time.Minute
	// GitPushTimeout은 git push 작업의 타임아웃입니다 (2분).
	GitPushTimeout = 2 * time.Minute
)

// GitExecutorConfig는 GitExecutor 설정입니다.
type GitExecutorConfig struct {
	// WorkspaceID는 워크스페이스 ID입니다 (clone 경로 구성용).
	WorkspaceID string
	// WorkspaceSlug는 워크스페이스 슬러그입니다 (브랜치 정책용).
	WorkspaceSlug string
	// BaseDir는 clone 기본 디렉토리입니다. 비어있으면 ~/.autopus를 사용합니다.
	BaseDir string
	// GitUserName은 커밋 시 사용할 git 사용자 이름입니다.
	GitUserName string
	// GitUserEmail은 커밋 시 사용할 git 사용자 이메일입니다.
	GitUserEmail string
}

// GitExecutor는 로컬 Git 워크스페이스를 관리합니다.
// REQ-001: Bridge GitExecutor — clone/pull/checkout/commit/push/diff
// @MX:ANCHOR: [AUTO] CodeOps 파이프라인의 Git 작업 진입점 (fan_in >= 3)
// @MX:REASON: CodeOpsWorker, WebSocket 핸들러, 테스트에서 직접 호출됨
// @MX:SPEC: SPEC-CODEOPS-001 REQ-001
type GitExecutor struct {
	cfg GitExecutorConfig
}

// NewGitExecutor는 새로운 GitExecutor를 생성합니다.
func NewGitExecutor(cfg GitExecutorConfig) *GitExecutor {
	return &GitExecutor{cfg: cfg}
}

// Clone은 레포지토리를 로컬에 clone합니다.
// REQ-001.2: Clone(ctx, repoURL, branch, targetDir) — 레포 최초 clone
// REQ-001.8: git 인증은 GitHub OAuth 토큰 기반
func (g *GitExecutor) Clone(ctx context.Context, repoURL, branch, oauthToken string) (string, error) {
	// OAuth 토큰으로 인증 URL 구성
	authURL := g.buildAuthenticatedURL(repoURL, oauthToken)

	// repoURL에서 레포 슬러그 추출하여 clone 경로 결정
	repoSlug := extractRepoSlug(repoURL)
	targetDir := g.clonePath(repoSlug)

	// 이미 clone된 경우 pull로 업데이트
	if g.isAlreadyCloned(targetDir) {
		if err := g.Pull(ctx, targetDir, branch, oauthToken); err != nil {
			return "", fmt.Errorf("기존 clone 업데이트 실패: %w", err)
		}
		return targetDir, nil
	}

	// 부모 디렉토리 생성
	if err := os.MkdirAll(filepath.Dir(targetDir), 0750); err != nil {
		return "", fmt.Errorf("clone 디렉토리 생성 실패: %w", err)
	}

	// clone 타임아웃 설정
	cloneCtx, cancel := context.WithTimeout(ctx, GitCloneTimeout)
	defer cancel()

	// git clone 실행 (--single-branch로 지정 브랜치만 clone)
	args := []string{"clone", "--branch", branch, "--single-branch", authURL, targetDir}
	if err := runGitCommand(cloneCtx, "", args...); err != nil {
		return "", fmt.Errorf("git clone 실패 (repo=%s, branch=%s): %w", repoURL, branch, err)
	}

	// git 사용자 설정 (커밋 메타데이터용)
	if err := g.configureGitUser(ctx, targetDir); err != nil {
		// 치명적이지 않음 — 경고만 기록
		_ = err
	}

	return targetDir, nil
}

// Pull은 기존 clone을 최신 상태로 업데이트합니다.
// REQ-001.3: Pull(ctx, workDir, branch) — 기존 clone 업데이트
func (g *GitExecutor) Pull(ctx context.Context, workDir, branch, oauthToken string) error {
	// 원격 URL 갱신 (토큰 만료 대비)
	authURL, err := g.getRemoteURL(ctx, workDir, oauthToken)
	if err == nil && authURL != "" {
		_ = runGitCommand(ctx, workDir, "remote", "set-url", "origin", authURL)
	}

	if err := runGitCommand(ctx, workDir, "fetch", "origin"); err != nil {
		return fmt.Errorf("git fetch 실패: %w", err)
	}

	if err := runGitCommand(ctx, workDir, "checkout", branch); err != nil {
		return fmt.Errorf("git checkout 실패 (branch=%s): %w", branch, err)
	}

	if err := runGitCommand(ctx, workDir, "pull", "origin", branch); err != nil {
		return fmt.Errorf("git pull 실패 (branch=%s): %w", branch, err)
	}

	return nil
}

// Checkout은 새 agent/ 브랜치를 생성하고 체크아웃합니다.
// REQ-001.4: Checkout(ctx, workDir, branchName) — 새 agent/ 브랜치 생성
func (g *GitExecutor) Checkout(ctx context.Context, workDir, branchName string) error {
	// 브랜치명 검증 (agent/ 프리픽스 필요)
	if err := g.validatePushBranch(branchName); err != nil {
		return err
	}

	// 원격에 브랜치가 이미 있는지 확인
	if remoteExists(ctx, workDir, branchName) {
		// 원격 브랜치 체크아웃
		if err := runGitCommand(ctx, workDir, "checkout", "-B", branchName, "origin/"+branchName); err != nil {
			return fmt.Errorf("원격 브랜치 체크아웃 실패: %w", err)
		}
		return nil
	}

	// 새 브랜치 생성
	if err := runGitCommand(ctx, workDir, "checkout", "-b", branchName); err != nil {
		return fmt.Errorf("새 브랜치 생성 실패 (branch=%s): %w", branchName, err)
	}

	return nil
}

// Commit은 변경 파일을 스테이징하고 커밋합니다.
// REQ-001.5: Commit(ctx, workDir, message, files) — 변경 파일 스테이징 + 커밋
// REQ-002.6: 커밋 메시지 형식: feat(agent): {task_description} [agent/{agent-name}]
func (g *GitExecutor) Commit(ctx context.Context, workDir, message string, files []string) error {
	if len(files) == 0 {
		return fmt.Errorf("커밋할 파일이 없습니다")
	}

	// 파일 스테이징 (지정된 파일만)
	addArgs := append([]string{"add", "--"}, files...)
	if err := runGitCommand(ctx, workDir, addArgs...); err != nil {
		return fmt.Errorf("git add 실패: %w", err)
	}

	// 커밋
	if err := runGitCommand(ctx, workDir, "commit", "-m", message); err != nil {
		return fmt.Errorf("git commit 실패: %w", err)
	}

	return nil
}

// Push는 원격에 push합니다.
// REQ-001.6: Push(ctx, workDir, branch) — 원격 push (agent/ 프리픽스만 허용)
// REQ-001.10: 브랜치 정책 강제 적용
func (g *GitExecutor) Push(ctx context.Context, workDir, branch, oauthToken string) error {
	// 브랜치 정책 검증 (agent/ 프리픽스만 허용)
	if err := g.validatePushBranch(branch); err != nil {
		return err
	}

	// 원격 URL에 토큰 적용
	authURL, err := g.getRemoteURL(ctx, workDir, oauthToken)
	if err == nil && authURL != "" {
		if setErr := runGitCommand(ctx, workDir, "remote", "set-url", "origin", authURL); setErr != nil {
			return fmt.Errorf("원격 URL 설정 실패: %w", setErr)
		}
	}

	// push 타임아웃 설정
	pushCtx, cancel := context.WithTimeout(ctx, GitPushTimeout)
	defer cancel()

	if err := runGitCommand(pushCtx, workDir, "push", "-u", "origin", branch); err != nil {
		return fmt.Errorf("git push 실패 (branch=%s): %w", branch, err)
	}

	return nil
}

// Diff는 현재 변경사항을 반환합니다.
// REQ-001.7: Diff(ctx, workDir) — 현재 변경사항 diff 반환
func (g *GitExecutor) Diff(ctx context.Context, workDir string) (string, error) {
	output, err := runGitCommandOutput(ctx, workDir, "diff", "HEAD")
	if err != nil {
		// 초기 커밋이 없는 경우 빈 diff 반환
		return "", nil
	}
	return output, nil
}

// validatePushBranch는 push 대상 브랜치명이 정책을 준수하는지 검증합니다.
// REQ-001.10: agent/{workspace-slug}/ 프리픽스만 push 허용, main/master push 차단
func (g *GitExecutor) validatePushBranch(branch string) error {
	// main/master 직접 push 절대 차단
	switch branch {
	case "main", "master", "develop", "release", "staging", "production", "prod":
		return fmt.Errorf("보호된 브랜치 '%s'에 push할 수 없습니다: 직접 push가 금지되어 있습니다", branch)
	}

	// agent/ 프리픽스 필수
	if !strings.HasPrefix(branch, "agent/") {
		return fmt.Errorf("브랜치 '%s'은 'agent/' 프리픽스가 없습니다: 에이전트 push는 agent/로 시작하는 브랜치만 허용됩니다", branch)
	}

	return nil
}

// clonePath는 레포 슬러그에 대한 로컬 clone 경로를 반환합니다.
// REQ-001.9: clone 경로: ~/.autopus/workspaces/{workspace-id}/{repo-slug}/
func (g *GitExecutor) clonePath(repoSlug string) string {
	// owner/repo 형태에서 repo 이름만 추출
	slug := repoSlug
	if parts := strings.Split(repoSlug, "/"); len(parts) >= 2 {
		slug = parts[len(parts)-1]
	}
	// .git 접미사 제거
	slug = strings.TrimSuffix(slug, ".git")

	baseDir := g.cfg.BaseDir
	if baseDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = os.TempDir()
		}
		baseDir = filepath.Join(home, ".autopus")
	}

	return filepath.Join(baseDir, "workspaces", g.cfg.WorkspaceID, slug)
}

// buildAuthenticatedURL은 OAuth 토큰을 포함한 인증 URL을 구성합니다.
// REQ-001.8: git 인증은 GitHub OAuth 토큰 기반
// REQ-006.8: 토큰은 서버에서 전달되며 Bridge에 저장되지 않음
func (g *GitExecutor) buildAuthenticatedURL(repoURL, oauthToken string) string {
	if oauthToken == "" {
		return repoURL
	}

	parsed, err := url.Parse(repoURL)
	if err != nil {
		return repoURL
	}

	// 기존 인증 정보 교체
	parsed.User = url.User(oauthToken)
	return parsed.String()
}

// isAlreadyCloned는 디렉토리가 이미 git 레포지토리인지 확인합니다.
func (g *GitExecutor) isAlreadyCloned(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil && info.IsDir()
}

// getRemoteURL은 현재 원격 URL을 가져와 토큰을 적용합니다.
func (g *GitExecutor) getRemoteURL(ctx context.Context, workDir, oauthToken string) (string, error) {
	rawURL, err := runGitCommandOutput(ctx, workDir, "remote", "get-url", "origin")
	if err != nil {
		return "", err
	}
	rawURL = strings.TrimSpace(rawURL)
	if oauthToken == "" {
		return rawURL, nil
	}
	return g.buildAuthenticatedURL(rawURL, oauthToken), nil
}

// configureGitUser는 git 사용자 설정을 구성합니다.
func (g *GitExecutor) configureGitUser(ctx context.Context, workDir string) error {
	name := g.cfg.GitUserName
	if name == "" {
		name = "Autopus Agent"
	}
	email := g.cfg.GitUserEmail
	if email == "" {
		email = "agent@autopus.ai"
	}

	if err := runGitCommand(ctx, workDir, "config", "user.name", name); err != nil {
		return err
	}
	if err := runGitCommand(ctx, workDir, "config", "user.email", email); err != nil {
		return err
	}
	return nil
}

// runGitCommand는 git 명령어를 실행합니다.
// 절대로 shell을 사용하지 않고 exec.Command를 직접 사용합니다.
func runGitCommand(ctx context.Context, workDir string, args ...string) error {
	_, err := runGitCommandOutput(ctx, workDir, args...)
	return err
}

// runGitCommandOutput은 git 명령어를 실행하고 출력을 반환합니다.
func runGitCommandOutput(ctx context.Context, workDir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...) //nolint:gosec // git 명령어는 사용자 입력 없음
	cmd.Dir = workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s 실패: %w (stderr: %s)", strings.Join(args, " "), err, stderr.String())
	}

	return stdout.String(), nil
}

// remoteExists는 원격 브랜치가 존재하는지 확인합니다.
func remoteExists(ctx context.Context, workDir, branch string) bool {
	_, err := runGitCommandOutput(ctx, workDir, "ls-remote", "--exit-code", "origin", branch)
	return err == nil
}

// extractRepoSlug는 레포 URL에서 슬러그를 추출합니다.
// 예: "https://github.com/owner/repo.git" → "owner/repo"
func extractRepoSlug(repoURL string) string {
	parsed, err := url.Parse(repoURL)
	if err != nil {
		return repoURL
	}
	path := strings.TrimPrefix(parsed.Path, "/")
	path = strings.TrimSuffix(path, ".git")
	return path
}
