// Package executor - 코딩 세션 관리자
package executor

import (
	"context"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"sync"
)

// GitResult는 Git 후처리 결과입니다.
type GitResult struct {
	// DiffSummary는 변경사항 요약입니다.
	DiffSummary string
	// ChangedFiles는 변경된 파일 목록입니다.
	ChangedFiles []string
	// PushedBranch는 push된 브랜치 이름입니다.
	PushedBranch string
}

// 기본 동시 실행 세션 수
const defaultMaxConcurrent = 2

// CodingSessionManager는 코딩 세션의 동시성과 라이프사이클을 관리합니다.
// @MX:ANCHOR: [AUTO] 코딩 릴레이 세션 팩토리 및 세마포어 관리 (fan_in >= 3)
// @MX:REASON: WebSocket 핸들러, 릴레이 서비스, 테스트에서 직접 사용
// @MX:SPEC: SPEC-CODING-RELAY-001 TASK-004
type CodingSessionManager struct {
	// semaphore는 동시 실행 세션 수를 제한하는 버퍼 채널입니다.
	semaphore chan struct{}
	// maxConcurrent는 최대 동시 실행 세션 수입니다.
	maxConcurrent int
	// sessions는 활성 세션 맵입니다.
	sessions map[string]CodingSession
	// mu는 sessions 맵 접근을 보호하는 뮤텍스입니다.
	mu sync.RWMutex
	// cfg는 세션 관리자 설정입니다.
	cfg CodingSessionConfig
	// providerOnce는 프로바이더 탐지를 한 번만 수행하도록 보장합니다.
	providerOnce sync.Once
	// cachedProviders는 캐싱된 프로바이더 탐지 결과입니다.
	cachedProviders []CodingProvider
}

// NewCodingSessionManager는 새로운 CodingSessionManager를 생성합니다.
func NewCodingSessionManager(cfg CodingSessionConfig) *CodingSessionManager {
	maxConcurrent := cfg.MaxConcurrent
	if maxConcurrent <= 0 {
		maxConcurrent = defaultMaxConcurrent
	}

	return &CodingSessionManager{
		semaphore:     make(chan struct{}, maxConcurrent),
		maxConcurrent: maxConcurrent,
		sessions:      make(map[string]CodingSession),
		cfg:           cfg,
	}
}

// DetectProviders는 시스템에 설치된 코딩 에이전트 CLI 도구를 탐지하고 우선순위 순으로 반환합니다.
// websocket.CodingSessionManagerIface를 구현합니다.
func (m *CodingSessionManager) DetectProviders() []string {
	detected := m.detectProviders()
	result := make([]string, len(detected))
	for i, p := range detected {
		result[i] = string(p)
	}
	return result
}

// detectProviders는 내부 구현으로 CodingProvider 타입을 반환합니다.
// CLI 가용성은 프로세스 수명 동안 변하지 않으므로 결과를 캐싱합니다.
func (m *CodingSessionManager) detectProviders() []CodingProvider {
	m.providerOnce.Do(func() {
		// 탐지할 CLI 도구와 우선순위 순서
		type providerEntry struct {
			binary   string
			provider CodingProvider
			priority int
		}

		candidates := []providerEntry{
			{"claude", CodingProviderClaude, 1},
			{"codex", CodingProviderCodex, 2},
			{"gemini", CodingProviderGemini, 3},
			{"opencode", CodingProviderOpenCode, 4},
		}

		var found []providerEntry
		for _, c := range candidates {
			if _, err := exec.LookPath(c.binary); err == nil {
				found = append(found, c)
			}
		}

		// 우선순위 순으로 정렬
		sort.Slice(found, func(i, j int) bool {
			return found[i].priority < found[j].priority
		})

		m.cachedProviders = make([]CodingProvider, len(found))
		for i, f := range found {
			m.cachedProviders[i] = f.provider
		}
	})
	return m.cachedProviders
}

// CreateSession은 지정된 프로바이더 타입의 CodingSession을 생성합니다.
func (m *CodingSessionManager) CreateSession(provider CodingProvider) (CodingSession, error) {
	switch provider {
	case CodingProviderClaude:
		return NewClaudeCodeSession(), nil
	case CodingProviderAPI:
		return NewAPIFallbackSession(m.cfg), nil
	case CodingProviderCodex:
		return NewCodexSession(), nil
	case CodingProviderGemini:
		return nil, fmt.Errorf("Gemini 프로바이더는 아직 구현되지 않았습니다")
	case CodingProviderOpenCode:
		return nil, fmt.Errorf("OpenCode 프로바이더는 아직 구현되지 않았습니다")
	default:
		return nil, fmt.Errorf("알 수 없는 프로바이더: %s", provider)
	}
}

// AcquireSlot은 세마포어 슬롯을 획득합니다. 컨텍스트가 취소되면 에러를 반환합니다.
func (m *CodingSessionManager) AcquireSlot(ctx context.Context) error {
	select {
	case m.semaphore <- struct{}{}:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("세션 슬롯 획득 타임아웃: %w", ctx.Err())
	}
}

// ReleaseSlot은 세마포어 슬롯을 해제합니다.
func (m *CodingSessionManager) ReleaseSlot() {
	select {
	case <-m.semaphore:
	default:
		// 슬롯이 없으면 아무것도 하지 않음 (방어적 처리)
	}
}

// RegisterSession은 세션을 등록합니다.
func (m *CodingSessionManager) RegisterSession(id string, session CodingSession) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[id] = session
}

// GetSession은 세션 ID로 세션을 조회합니다.
func (m *CodingSessionManager) GetSession(id string) (CodingSession, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	session, ok := m.sessions[id]
	return session, ok
}

// CloseSession은 세션을 종료하고 맵에서 제거합니다.
func (m *CodingSessionManager) CloseSession(ctx context.Context, id string) error {
	m.mu.Lock()
	session, ok := m.sessions[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("세션을 찾을 수 없습니다: %s", id)
	}
	delete(m.sessions, id)
	m.mu.Unlock()

	return session.Close(ctx)
}

// HandleSessionComplete는 세션 완료 후 Git 후처리를 수행합니다.
// 흐름: git diff → 시크릿 스캔 → commit → push
// @MX:NOTE: [AUTO] 세션 완료 후 변경사항을 agent/ 브랜치에 커밋하고 push
// @MX:SPEC: SPEC-CODING-RELAY-001 TASK-013
func (m *CodingSessionManager) HandleSessionComplete(ctx context.Context, workDir, sessionID, description string) (*GitResult, error) {
	git := NewGitExecutor(m.cfg.GitConfig)

	// 1단계: git diff HEAD — 변경사항 수집
	diff, err := git.Diff(ctx, workDir)
	if err != nil {
		return nil, fmt.Errorf("diff 수집 실패: %w", err)
	}

	// 변경사항 없으면 빈 결과 반환
	if diff == "" {
		return &GitResult{
			DiffSummary: "변경사항 없음",
		}, nil
	}

	// 2단계: 변경된 파일 목록 수집
	changedFiles, err := m.getChangedFiles(ctx, workDir)
	if err != nil {
		return nil, fmt.Errorf("변경 파일 목록 수집 실패: %w", err)
	}

	// 3단계: 시크릿 스캔 (차단 대상 파일 검사)
	scanner := NewSecretScanner()
	violations := scanner.ScanFileList(changedFiles)
	if len(violations) > 0 {
		blocked := make([]string, len(violations))
		for i, v := range violations {
			blocked[i] = v.Path
		}
		return nil, fmt.Errorf("보안 정책 위반 — 차단된 파일 포함: %s", strings.Join(blocked, ", "))
	}

	// 4단계: 커밋
	commitMsg := fmt.Sprintf("feat(agent): %s [agent/%s]", description, sessionID)
	if err := git.Commit(ctx, workDir, commitMsg, changedFiles); err != nil {
		return nil, fmt.Errorf("커밋 실패: %w", err)
	}

	// 5단계: Push (브랜치명은 현재 체크아웃된 브랜치 사용)
	branch, err := m.getCurrentBranch(ctx, workDir)
	if err != nil {
		return nil, fmt.Errorf("현재 브랜치 조회 실패: %w", err)
	}

	if err := git.Push(ctx, workDir, branch, ""); err != nil {
		return nil, fmt.Errorf("push 실패: %w", err)
	}

	return &GitResult{
		DiffSummary:  summarizeDiff(diff),
		ChangedFiles: changedFiles,
		PushedBranch: branch,
	}, nil
}

// getChangedFiles는 staged 변경 파일 목록을 반환합니다.
func (m *CodingSessionManager) getChangedFiles(ctx context.Context, workDir string) ([]string, error) {
	output, err := runGitCommandOutput(ctx, workDir, "diff", "--name-only", "HEAD")
	if err != nil {
		// 초기 커밋이 없을 경우 staged 파일 목록 반환
		output, err = runGitCommandOutput(ctx, workDir, "diff", "--name-only", "--cached")
		if err != nil {
			return nil, err
		}
	}

	var files []string
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

// getCurrentBranch는 현재 체크아웃된 브랜치 이름을 반환합니다.
func (m *CodingSessionManager) getCurrentBranch(ctx context.Context, workDir string) (string, error) {
	output, err := runGitCommandOutput(ctx, workDir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output), nil
}
