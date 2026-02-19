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

// CodexCLIResponse는 codex CLI의 JSON 출력 구조입니다.
// codex --quiet --output-format json 명령의 출력을 파싱합니다.
type CodexCLIResponse struct {
	// Type은 응답 타입입니다 (예: "result").
	Type string `json:"type"`
	// Result는 실행 결과입니다.
	Result string `json:"result"`
	// Subtype은 결과 하위 타입입니다.
	Subtype string `json:"subtype,omitempty"`
	// CostUSD는 실행 비용(USD)입니다.
	CostUSD float64 `json:"cost_usd,omitempty"`
	// DurationMS는 실행 시간(밀리초)입니다.
	DurationMS int64 `json:"duration_ms,omitempty"`
	// DurationAPIMS는 API 호출 시간(밀리초)입니다.
	DurationAPIMS int64 `json:"duration_api_ms,omitempty"`
	// NumTurns는 대화 턴 수입니다.
	NumTurns int `json:"num_turns,omitempty"`
	// SessionID는 세션 ID입니다.
	SessionID string `json:"session_id,omitempty"`
	// TotalInputTokens는 총 입력 토큰 수입니다.
	TotalInputTokens int `json:"total_input_tokens,omitempty"`
	// TotalOutputTokens는 총 출력 토큰 수입니다.
	TotalOutputTokens int `json:"total_output_tokens,omitempty"`
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
			DefaultModel: "gpt-5-codex",
		},
		approvalMode: "auto-edit",
		defaultArgs:  []string{"--quiet"},
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
// CLI 모드에서는 gpt- 또는 o4- 접두사를 가진 모든 모델을 지원합니다.
func (p *CodexCLIProvider) Supports(model string) bool {
	return strings.HasPrefix(model, "gpt-") || strings.HasPrefix(model, "o4-")
}

// Execute는 codex CLI를 통해 프롬프트를 실행하고 결과를 반환합니다.
func (p *CodexCLIProvider) Execute(ctx context.Context, req ExecuteRequest) (*ExecuteResponse, error) {
	startTime := time.Now()

	// 모델 결정
	model := req.Model
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
	// codex --quiet --output-format json --model <model> --approval-mode <mode> "prompt"
	args := make([]string, 0, len(p.defaultArgs)+8)
	args = append(args, p.defaultArgs...)
	args = append(args,
		"--output-format", "json",
		"--model", model,
		"--approval-mode", p.approvalMode,
	)

	// 프롬프트 추가
	args = append(args, req.Prompt)

	// 명령 실행
	cmd := exec.CommandContext(execCtx, p.cliPath, args...)

	// 작업 디렉토리 설정 (존재하는 경우에만)
	// 서버가 Docker 컨테이너 내부 경로를 보내는 경우 호스트에 없을 수 있음
	if req.WorkDir != "" {
		if _, err := os.Stat(req.WorkDir); err == nil {
			cmd.Dir = req.WorkDir
		}
	}

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

	// JSON 출력 파싱
	// codex CLI는 여러 줄의 JSON을 출력할 수 있으므로 마지막 result 타입을 찾음
	output := stdout.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	var response *CodexCLIResponse
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var parsed CodexCLIResponse
		if err := json.Unmarshal([]byte(line), &parsed); err != nil {
			continue
		}

		// result 타입의 응답을 찾음
		if parsed.Type == "result" {
			response = &parsed
		}
	}

	if response == nil {
		// JSON 파싱 실패 시 전체 출력을 결과로 사용
		// 토큰 사용량은 추정값 사용 (약 4자당 1토큰)
		estimatedInputTokens := len(req.Prompt) / 4
		estimatedOutputTokens := len(output) / 4

		return &ExecuteResponse{
			Output: output,
			TokenUsage: TokenUsage{
				InputTokens:  estimatedInputTokens,
				OutputTokens: estimatedOutputTokens,
				TotalTokens:  estimatedInputTokens + estimatedOutputTokens,
			},
			DurationMs: time.Since(startTime).Milliseconds(),
			Model:      model,
			StopReason: "end_turn",
		}, nil
	}

	// 토큰 사용량 설정
	tokenUsage := TokenUsage{
		InputTokens:  response.TotalInputTokens,
		OutputTokens: response.TotalOutputTokens,
		TotalTokens:  response.TotalInputTokens + response.TotalOutputTokens,
	}

	// 토큰 정보가 없으면 추정값 사용
	if tokenUsage.TotalTokens == 0 {
		tokenUsage.InputTokens = len(req.Prompt) / 4
		tokenUsage.OutputTokens = len(response.Result) / 4
		tokenUsage.TotalTokens = tokenUsage.InputTokens + tokenUsage.OutputTokens
	}

	// 실행 시간 설정 (CLI 응답 값 또는 실제 측정값)
	durationMs := response.DurationMS
	if durationMs == 0 {
		durationMs = time.Since(startTime).Milliseconds()
	}

	return &ExecuteResponse{
		Output:     response.Result,
		TokenUsage: tokenUsage,
		DurationMs: durationMs,
		Model:      model,
		StopReason: "end_turn",
	}, nil
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
