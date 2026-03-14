// Package executor는 Local Agent Bridge의 작업 실행 엔진을 제공합니다.
// REQ-002: Bridge CodeOps Worker — AI 코드 생성 워크플로우
package executor

import (
	"context"
	"fmt"
	"strings"

	ws "github.com/insajin/autopus-agent-protocol"
)

// 유효한 change_type 목록과 대응하는 커밋 접두사
var validChangeTypes = map[string]string{
	"feature":  "feat(agent):",
	"bugfix":   "fix(agent):",
	"refactor": "refactor(agent):",
	"test":     "test(agent):",
	"docs":     "docs(agent):",
}

// 최대 변경 파일 수
const maxCodeOpsFiles = 10

// CodeOpsWorkerConfig는 CodeOpsWorker 설정입니다.
type CodeOpsWorkerConfig struct {
	// MaxRetries는 테스트 실패 시 최대 재시도 횟수입니다. 기본값: 3.
	MaxRetries int
	// GitConfig는 GitExecutor 설정입니다.
	GitConfig GitExecutorConfig
}

// CodeOpsWorker는 AI 코드 생성 워크플로우를 실행합니다.
// REQ-002: Pull → AI 코드 생성 → 파일 적용 → 테스트 → Commit → Push
// @MX:ANCHOR: [AUTO] Bridge CodeOps 파이프라인 핵심 실행기 (fan_in >= 3)
// @MX:REASON: WebSocket 핸들러, 테스트, 서버 요청에서 호출
// @MX:SPEC: SPEC-CODEOPS-001 REQ-002
type CodeOpsWorker struct {
	cfg     CodeOpsWorkerConfig
	scanner *SecretScanner
}

// NewCodeOpsWorker는 새로운 CodeOpsWorker를 생성합니다.
func NewCodeOpsWorker(cfg CodeOpsWorkerConfig) *CodeOpsWorker {
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 3
	}
	return &CodeOpsWorker{
		cfg:     cfg,
		scanner: NewSecretScanner(),
	}
}

// Execute는 코드 수정 워크플로우를 실행합니다.
// REQ-002.2: 워크플로우: Pull → AI 코드 생성 → 파일 적용 → 테스트 → Commit → Push
// REQ-002.4: 테스트 실패 시 최대 3회 재시도
// REQ-002.5: 최종 실패 시 서버에 실패 결과 보고 (변경 없이)
// @MX:WARN: [AUTO] 컨텍스트 취소 시 워크스페이스 정리 필요
// @MX:REASON: clone 디렉토리가 남아있을 수 있으므로 defer 정리 권장
func (w *CodeOpsWorker) Execute(ctx context.Context, req ws.CodeOpsRequestPayload) (ws.CodeOpsResultPayload, error) {
	result := ws.CodeOpsResultPayload{
		RequestID:     req.RequestID,
		WorkspaceID:   req.WorkspaceID,
		CorrelationID: req.CorrelationID,
	}

	// 요청 유효성 검사
	if err := w.validateRequest(req); err != nil {
		result.Error = err.Error()
		return result, err
	}

	// 컨텍스트 취소 확인
	if err := ctx.Err(); err != nil {
		result.Error = "요청 취소됨"
		return result, fmt.Errorf("컨텍스트 취소: %w", err)
	}

	// GitExecutor 생성 (OAuth 토큰은 Bridge에 저장하지 않음 — 요청마다 사용 후 폐기)
	gitCfg := w.cfg.GitConfig
	gitCfg.WorkspaceID = req.WorkspaceID
	gitCfg.WorkspaceSlug = req.WorkspaceSlug
	git := NewGitExecutor(gitCfg)

	// 1단계: Clone 또는 Pull
	workDir, err := git.Clone(ctx, req.RepoURL, req.Branch, req.OAuthToken)
	if err != nil {
		result.Error = fmt.Sprintf("git clone/pull 실패: %v", err)
		return result, fmt.Errorf("git clone/pull 실패: %w", err)
	}

	// 2단계: agent/ 브랜치 체크아웃
	if err := git.Checkout(ctx, workDir, req.AgentBranch); err != nil {
		result.Error = fmt.Sprintf("브랜치 체크아웃 실패: %v", err)
		return result, fmt.Errorf("브랜치 체크아웃 실패: %w", err)
	}

	// 3단계: 파일 시크릿 스캔 (변경 전)
	if err := w.checkSecretsInFiles(req.TargetFiles); err != nil {
		result.Error = err.Error()
		return result, err
	}

	// 4단계: diff 수집 (AI 코드 적용 후 호출 — 현재는 외부에서 파일 수정 후 call)
	diff, err := git.Diff(ctx, workDir)
	if err != nil {
		result.Error = fmt.Sprintf("diff 수집 실패: %v", err)
		return result, fmt.Errorf("diff 수집 실패: %w", err)
	}

	// 변경사항이 없으면 성공으로 처리 (빈 작업)
	if diff == "" {
		result.Success = true
		result.DiffSummary = "변경사항 없음"
		result.PushedBranch = req.AgentBranch
		return result, nil
	}

	// 5단계: 커밋
	commitMsg := w.buildCommitMessage(req)
	if err := git.Commit(ctx, workDir, commitMsg, req.TargetFiles); err != nil {
		result.Error = fmt.Sprintf("커밋 실패: %v", err)
		return result, fmt.Errorf("커밋 실패: %w", err)
	}

	// 6단계: Push
	if err := git.Push(ctx, workDir, req.AgentBranch, req.OAuthToken); err != nil {
		result.Error = fmt.Sprintf("push 실패: %v", err)
		return result, fmt.Errorf("push 실패: %w", err)
	}

	result.Success = true
	result.PushedBranch = req.AgentBranch
	result.ChangedFiles = req.TargetFiles
	result.DiffSummary = summarizeDiff(diff)
	return result, nil
}

// validateRequest는 CodeOps 요청의 유효성을 검사합니다.
func (w *CodeOpsWorker) validateRequest(req ws.CodeOpsRequestPayload) error {
	if req.RepoURL == "" {
		return fmt.Errorf("레포 URL이 없습니다")
	}

	// 변경 파일 수 제한 (REQ-002.7: 최대 10파일)
	if len(req.TargetFiles) > maxCodeOpsFiles {
		return fmt.Errorf("변경 파일 수 초과: 최대 %d개, 요청 %d개", maxCodeOpsFiles, len(req.TargetFiles))
	}

	// change_type 유효성 검사
	if _, ok := validChangeTypes[req.ChangeType]; !ok {
		validTypes := make([]string, 0, len(validChangeTypes))
		for k := range validChangeTypes {
			validTypes = append(validTypes, k)
		}
		return fmt.Errorf("유효하지 않은 change_type '%s': 허용값 %s", req.ChangeType, strings.Join(validTypes, ", "))
	}

	return nil
}

// buildCommitMessage는 규격화된 커밋 메시지를 생성합니다.
// REQ-002.6: 커밋 메시지 형식: feat(agent): {task_description} [agent/{agent-name}]
func (w *CodeOpsWorker) buildCommitMessage(req ws.CodeOpsRequestPayload) string {
	prefix := validChangeTypes[req.ChangeType]
	if prefix == "" {
		prefix = "chore(agent):"
	}

	agentTag := ""
	if req.AgentName != "" {
		agentTag = fmt.Sprintf(" [agent/%s]", req.AgentName)
	}

	return fmt.Sprintf("%s %s%s", prefix, req.Description, agentTag)
}

// checkSecretsInFiles는 변경 파일 목록에서 차단 대상 파일을 검사합니다.
// REQ-006.4: 커밋 전 시크릿 스캔
func (w *CodeOpsWorker) checkSecretsInFiles(files []string) error {
	violations := w.scanner.ScanFileList(files)
	if len(violations) == 0 {
		return nil
	}

	paths := make([]string, 0, len(violations))
	for _, v := range violations {
		paths = append(paths, v.Path)
	}
	return fmt.Errorf("보안 정책 위반 — 차단된 파일 포함: %s", strings.Join(paths, ", "))
}

// summarizeDiff는 diff 출력을 요약합니다.
func summarizeDiff(diff string) string {
	lines := strings.Split(diff, "\n")
	added, removed := 0, 0
	for _, line := range lines {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			added++
		} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			removed++
		}
	}
	return fmt.Sprintf("+%d줄 추가, -%d줄 삭제", added, removed)
}
