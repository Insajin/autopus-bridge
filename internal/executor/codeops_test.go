// Package executor는 Local Agent Bridge의 작업 실행 엔진을 제공합니다.
package executor

import (
	"context"
	"testing"

	ws "github.com/insajin/autopus-agent-protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCodeOpsWorker_ValidateRequest는 요청 유효성 검사를 검증합니다.
// REQ-002.7: 변경 범위 제한 — 단일 태스크당 최대 10파일, 1000줄 변경
func TestCodeOpsWorker_ValidateRequest(t *testing.T) {
	t.Parallel()

	worker := NewCodeOpsWorker(CodeOpsWorkerConfig{})

	tests := []struct {
		name      string
		req       ws.CodeOpsRequestPayload
		expectErr bool
		errMsg    string
	}{
		{
			name: "유효한 요청",
			req: ws.CodeOpsRequestPayload{
				RepoURL:       "https://github.com/owner/repo.git",
				Branch:        "main",
				AgentBranch:   "agent/workspace/fix-issue",
				Description:   "버그 수정",
				TargetFiles:   []string{"main.go", "service.go"},
				ChangeType:    "bugfix",
				OAuthToken:    "token",
				WorkspaceSlug: "workspace",
			},
			expectErr: false,
		},
		{
			name: "10파일 초과 거부",
			req: ws.CodeOpsRequestPayload{
				RepoURL:     "https://github.com/owner/repo.git",
				TargetFiles: []string{"f1", "f2", "f3", "f4", "f5", "f6", "f7", "f8", "f9", "f10", "f11"},
				ChangeType:  "feature",
			},
			expectErr: true,
			errMsg:    "10",
		},
		{
			name: "change_type 유효성 검사",
			req: ws.CodeOpsRequestPayload{
				RepoURL:     "https://github.com/owner/repo.git",
				TargetFiles: []string{"main.go"},
				ChangeType:  "invalid_type",
			},
			expectErr: true,
		},
		{
			name: "레포 URL 없음",
			req: ws.CodeOpsRequestPayload{
				TargetFiles: []string{"main.go"},
				ChangeType:  "feature",
			},
			expectErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := worker.validateRequest(tc.req)
			if tc.expectErr {
				require.Error(t, err)
				if tc.errMsg != "" {
					assert.Contains(t, err.Error(), tc.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestCodeOpsWorker_BuildCommitMessage는 커밋 메시지 형식을 검증합니다.
// REQ-002.6: 커밋 메시지 형식: feat(agent): {task_description} [agent/{agent-name}]
func TestCodeOpsWorker_BuildCommitMessage(t *testing.T) {
	t.Parallel()

	worker := NewCodeOpsWorker(CodeOpsWorkerConfig{})

	tests := []struct {
		name        string
		req         ws.CodeOpsRequestPayload
		wantPrefix  string
		wantContain string
	}{
		{
			name: "bugfix 타입",
			req: ws.CodeOpsRequestPayload{
				Description: "로그인 버그 수정",
				ChangeType:  "bugfix",
				AgentName:   "worker-dev-01",
			},
			wantPrefix:  "fix(agent):",
			wantContain: "[agent/worker-dev-01]",
		},
		{
			name: "feature 타입",
			req: ws.CodeOpsRequestPayload{
				Description: "새 기능 추가",
				ChangeType:  "feature",
				AgentName:   "worker-dev-02",
			},
			wantPrefix:  "feat(agent):",
			wantContain: "[agent/worker-dev-02]",
		},
		{
			name: "refactor 타입",
			req: ws.CodeOpsRequestPayload{
				Description: "코드 정리",
				ChangeType:  "refactor",
				AgentName:   "worker-dev-01",
			},
			wantPrefix:  "refactor(agent):",
			wantContain: "[agent/worker-dev-01]",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			msg := worker.buildCommitMessage(tc.req)
			assert.True(t, len(msg) > 0)
			assert.Contains(t, msg, tc.wantContain)
			assert.Contains(t, msg, tc.wantPrefix)
		})
	}
}

// TestCodeOpsWorker_SecretScanBeforeCommit는 커밋 전 시크릿 스캔을 검증합니다.
// REQ-006.4: 커밋 전 시크릿 스캔
func TestCodeOpsWorker_SecretScanBeforeCommit(t *testing.T) {
	t.Parallel()

	worker := NewCodeOpsWorker(CodeOpsWorkerConfig{})

	// .env 파일이 포함된 변경 목록
	files := []string{"main.go", ".env", "service.go"}
	err := worker.checkSecretsInFiles(files)
	require.Error(t, err)
	assert.Contains(t, err.Error(), ".env")
}

// TestCodeOpsWorker_SecretScanCleanFiles는 시크릿 없는 파일 스캔을 검증합니다.
func TestCodeOpsWorker_SecretScanCleanFiles(t *testing.T) {
	t.Parallel()

	worker := NewCodeOpsWorker(CodeOpsWorkerConfig{})

	files := []string{"main.go", "service.go", "handler.go"}
	err := worker.checkSecretsInFiles(files)
	assert.NoError(t, err)
}

// TestCodeOpsWorker_Execute_Cancelled는 컨텍스트 취소 처리를 검증합니다.
func TestCodeOpsWorker_Execute_Cancelled(t *testing.T) {
	t.Parallel()

	worker := NewCodeOpsWorker(CodeOpsWorkerConfig{
		MaxRetries: 1,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 즉시 취소

	req := ws.CodeOpsRequestPayload{
		RepoURL:       "https://github.com/owner/repo.git",
		Branch:        "main",
		AgentBranch:   "agent/ws/task",
		Description:   "test",
		TargetFiles:   []string{"main.go"},
		ChangeType:    "feature",
		OAuthToken:    "token",
		WorkspaceSlug: "ws",
		WorkspaceID:   "ws-id",
	}

	_, err := worker.Execute(ctx, req)
	require.Error(t, err)
}
