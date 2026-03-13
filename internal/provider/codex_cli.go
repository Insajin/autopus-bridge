// Package provider는 AI 프로바이더 통합 레이어를 제공합니다.
package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// CodexCLILine는 codex CLI의 JSONL 출력 한 줄의 구조입니다.
// codex exec --json 명령은 여러 줄의 JSONL을 출력합니다.
// 주요 타입: thread.started, turn.started, item.completed, turn.completed
type CodexCLILine struct {
	Type string `json:"type"`
	// item.completed 타입의 아이템 정보
	Item *CodexCLIItem `json:"item,omitempty"`
	// turn.completed 타입의 사용량 정보
	Usage *CodexCLIUsage `json:"usage,omitempty"`
}

// CodexCLIItem은 item.completed 이벤트의 아이템 구조입니다.
type CodexCLIItem struct {
	ID   string `json:"id"`
	Type string `json:"type"` // "agent_message", "reasoning", "tool_call" 등
	Text string `json:"text,omitempty"`
}

// CodexCLIUsage는 turn.completed 이벤트의 토큰 사용량 구조입니다.
type CodexCLIUsage struct {
	InputTokens       int `json:"input_tokens,omitempty"`
	CachedInputTokens int `json:"cached_input_tokens,omitempty"`
	OutputTokens      int `json:"output_tokens,omitempty"`
}

// CodexCLIProvider는 codex CLI를 서브프로세스로 실행하는 프로바이더입니다.
// OpenAI Codex CLI를 통해 코드 생성 작업을 실행합니다.
type CodexCLIProvider struct {
	cliPath      string
	timeout      time.Duration
	config       ProviderConfig
	approvalMode string
	defaultArgs  []string
}

// CodexCLIProviderOption은 CodexCLIProvider 설정 옵션입니다.
type CodexCLIProviderOption func(*CodexCLIProvider)

// WithCodexCLIPath는 CLI 바이너리 경로를 설정합니다.
func WithCodexCLIPath(path string) CodexCLIProviderOption {
	return func(p *CodexCLIProvider) {
		p.cliPath = path
	}
}

// WithCodexCLITimeout은 CLI 실행 타임아웃을 설정합니다.
func WithCodexCLITimeout(timeout time.Duration) CodexCLIProviderOption {
	return func(p *CodexCLIProvider) {
		p.timeout = timeout
	}
}

// WithCodexCLIDefaultModel은 기본 모델을 설정합니다.
func WithCodexCLIDefaultModel(model string) CodexCLIProviderOption {
	return func(p *CodexCLIProvider) {
		p.config.DefaultModel = model
	}
}

// WithCodexCLIApprovalMode는 승인 모드를 설정합니다.
// 예: "auto-edit", "suggest", "full-auto"
func WithCodexCLIApprovalMode(mode string) CodexCLIProviderOption {
	return func(p *CodexCLIProvider) {
		p.approvalMode = mode
	}
}

// WithCodexCLIDefaultArgs는 기본 CLI 인자를 설정합니다.
func WithCodexCLIDefaultArgs(args []string) CodexCLIProviderOption {
	return func(p *CodexCLIProvider) {
		p.defaultArgs = args
	}
}

// NewCodexCLIProvider는 새로운 CodexCLIProvider를 생성합니다.
func NewCodexCLIProvider(opts ...CodexCLIProviderOption) (*CodexCLIProvider, error) {
	p := &CodexCLIProvider{
		cliPath: "codex",
		timeout: 5 * time.Minute,
		config: ProviderConfig{
			DefaultModel: "gpt-5.4",
		},
		approvalMode: "full-auto",
		defaultArgs:  nil,
	}

	// 옵션 적용
	for _, opt := range opts {
		opt(p)
	}

	// CLI 바이너리 존재 확인
	if err := p.checkCLI(); err != nil {
		return nil, err
	}

	return p, nil
}

// checkCLI는 codex CLI 바이너리가 존재하고 실행 가능한지 확인합니다.
func (p *CodexCLIProvider) checkCLI() error {
	path, err := exec.LookPath(p.cliPath)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrCLINotFound, p.cliPath)
	}
	p.cliPath = path
	return nil
}

// Name은 프로바이더 식별자를 반환합니다.
func (p *CodexCLIProvider) Name() string {
	return "codex"
}

// ValidateConfig는 프로바이더 설정의 유효성을 검사합니다.
func (p *CodexCLIProvider) ValidateConfig() error {
	return p.checkCLI()
}

// Supports는 주어진 모델명을 지원하는지 확인합니다.
// OpenRouter 형식(openai/o3-mini)과 레거시 형식 모두 지원합니다.
// CLI 모드에서는 gpt-, o4-, o3- 접두사를 가진 모든 모델을 지원합니다.
func (p *CodexCLIProvider) Supports(model string) bool {
	bare := StripProviderPrefix(model)
	return strings.HasPrefix(bare, "gpt-") || strings.HasPrefix(bare, "o4-") || strings.HasPrefix(bare, "o3-")
}

// Execute는 codex CLI를 통해 프롬프트를 실행하고 결과를 반환합니다.
// ResponseMode가 "tool_loop"인 경우 프롬프트 기반 도구 호출 시뮬레이션을 사용합니다.
func (p *CodexCLIProvider) Execute(ctx context.Context, req ExecuteRequest) (*ExecuteResponse, error) {
	// tool_loop 모드: 프롬프트에 도구 정의와 형식 지침을 포함시켜 시뮬레이션
	if req.ResponseMode == "tool_loop" {
		return p.executeToolLoopViaCLI(ctx, req)
	}

	startTime := time.Now()

	// 모델 결정 (OpenRouter 접두사 제거)
	model := StripProviderPrefix(req.Model)
	if model == "" {
		model = p.config.DefaultModel
	}

	// 지원 모델 확인
	if !p.Supports(model) {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedModel, model)
	}

	// 타임아웃 컨텍스트 생성
	execCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	// CLI 명령 구성
	// codex exec --json -m <model> --full-auto --skip-git-repo-check [-C <dir>] "prompt"
	args := []string{"exec", "--json", "-m", model}

	// 승인 모드 설정
	switch p.approvalMode {
	case "full-auto":
		args = append(args, "--full-auto")
	case "danger-full-access":
		args = append(args, "--dangerously-bypass-approvals-and-sandbox")
	default:
		// workspace-write가 기본 샌드박스 모드
		args = append(args, "-s", "workspace-write")
	}

	// git repo 체크 건너뛰기 (브릿지는 임시 작업 디렉토리에서 실행될 수 있음)
	args = append(args, "--skip-git-repo-check")

	// 작업 디렉토리 설정 (-C 플래그 사용)
	if req.WorkDir != "" {
		if _, err := os.Stat(req.WorkDir); err == nil {
			args = append(args, "-C", req.WorkDir)
		}
	}

	// 기본 추가 인자 적용
	if len(p.defaultArgs) > 0 {
		args = append(args, p.defaultArgs...)
	}

	// 프롬프트 추가
	args = append(args, req.Prompt)

	// 명령 실행
	cmd := exec.CommandContext(execCtx, p.cliPath, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	// 컨텍스트 취소 확인
	if execCtx.Err() == context.DeadlineExceeded {
		return nil, fmt.Errorf("%w: %v초 후 타임아웃", ErrCLITimeout, p.timeout.Seconds())
	}
	if execCtx.Err() == context.Canceled {
		return nil, fmt.Errorf("%w: %v", ErrContextCanceled, ctx.Err())
	}

	// 실행 에러 확인
	if err != nil {
		return nil, fmt.Errorf("%w: %v, stderr: %s", ErrCLIExecution, err, stderr.String())
	}

	// JSONL 출력 파싱
	// codex exec --json은 여러 줄의 JSONL을 출력합니다:
	// - item.completed (type=agent_message): 에이전트 응답 텍스트
	// - item.completed (type=reasoning): 추론 과정 (무시)
	// - turn.completed: 토큰 사용량 정보
	output := stdout.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	var agentMessages []string
	var usage *CodexCLIUsage
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var parsed CodexCLILine
		if err := json.Unmarshal([]byte(line), &parsed); err != nil {
			continue
		}

		switch parsed.Type {
		case "item.completed":
			if parsed.Item != nil && parsed.Item.Type == "agent_message" && parsed.Item.Text != "" {
				agentMessages = append(agentMessages, parsed.Item.Text)
			}
		case "turn.completed":
			if parsed.Usage != nil {
				usage = parsed.Usage
			}
		}
	}

	// 에이전트 메시지 결합
	resultText := strings.Join(agentMessages, "\n\n")

	if resultText == "" {
		// agent_message가 없으면 전체 출력을 결과로 사용
		resultText = output
	}

	// 토큰 사용량 설정
	tokenUsage := TokenUsage{}
	if usage != nil {
		tokenUsage.InputTokens = usage.InputTokens
		tokenUsage.OutputTokens = usage.OutputTokens
		tokenUsage.TotalTokens = usage.InputTokens + usage.OutputTokens
	}

	// 토큰 정보가 없으면 추정값 사용 (약 4자당 1토큰)
	if tokenUsage.TotalTokens == 0 {
		tokenUsage.InputTokens = len(req.Prompt) / 4
		tokenUsage.OutputTokens = len(resultText) / 4
		tokenUsage.TotalTokens = tokenUsage.InputTokens + tokenUsage.OutputTokens
	}

	return &ExecuteResponse{
		Output:     resultText,
		TokenUsage: tokenUsage,
		DurationMs: time.Since(startTime).Milliseconds(),
		Model:      model,
		StopReason: "end_turn",
	}, nil
}

// executeToolLoopViaCLI는 tool_loop 모드 요청을 CLI 프롬프트 기반 시뮬레이션으로 처리합니다.
// 도구 정의와 대화 이력을 프롬프트에 인코딩하여 모델이 구조화된 JSON 블록으로 응답하도록 유도합니다.
func (p *CodexCLIProvider) executeToolLoopViaCLI(ctx context.Context, req ExecuteRequest) (*ExecuteResponse, error) {
	// 도구 정의와 대화 이력을 단일 프롬프트로 합침
	toolLoopReq := buildToolLoopExecuteRequest(req)
	toolLoopReq.Model = req.Model // 모델 유지

	// 일반 실행으로 위임 (재귀 방지: ResponseMode 제거됨)
	resp, err := p.Execute(ctx, toolLoopReq)
	if err != nil {
		return nil, err
	}

	// 출력에서 도구 호출 파싱
	return wrapToolLoopResponse(resp, resp.Output), nil
}

// IsCodexCLIAvailable은 codex CLI가 사용 가능한지 확인합니다.
// 전역 함수로 레지스트리 초기화 시 사용됩니다.
func IsCodexCLIAvailable(cliPath string) bool {
	if cliPath == "" {
		cliPath = "codex"
	}
	_, err := exec.LookPath(cliPath)
	return err == nil
}
