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

// GeminiCLIResponse는 gemini CLI의 JSON 출력 구조입니다.
// gemini --output-format json 명령의 출력을 파싱합니다.
type GeminiCLIResponse struct {
	// Type은 응답 타입입니다 (예: "result").
	Type string `json:"type"`
	// Result는 실행 결과입니다.
	Result string `json:"result"`
	// DurationMS는 실행 시간(밀리초)입니다.
	DurationMS int64 `json:"duration_ms,omitempty"`
	// TotalInputTokens는 총 입력 토큰 수입니다.
	TotalInputTokens int `json:"total_input_tokens,omitempty"`
	// TotalOutputTokens는 총 출력 토큰 수입니다.
	TotalOutputTokens int `json:"total_output_tokens,omitempty"`
}

// GeminiCLIProvider는 gemini CLI를 서브프로세스로 실행하는 프로바이더입니다.
// Google 구독 사용자가 API 키 없이 CLI를 통해 Gemini를 사용할 수 있습니다.
// gemini 바이너리가 없을 경우 npx @google/gemini-cli를 폴백으로 사용합니다.
type GeminiCLIProvider struct {
	cliPath         string
	fallbackCommand string
	useFallback     bool
	timeout         time.Duration
	config          ProviderConfig
}

// GeminiCLIProviderOption은 GeminiCLIProvider 설정 옵션입니다.
type GeminiCLIProviderOption func(*GeminiCLIProvider)

// WithGeminiCLIPath는 CLI 바이너리 경로를 설정합니다.
func WithGeminiCLIPath(path string) GeminiCLIProviderOption {
	return func(p *GeminiCLIProvider) {
		p.cliPath = path
	}
}

// WithGeminiCLITimeout은 CLI 실행 타임아웃을 설정합니다.
func WithGeminiCLITimeout(timeout time.Duration) GeminiCLIProviderOption {
	return func(p *GeminiCLIProvider) {
		p.timeout = timeout
	}
}

// WithGeminiCLIDefaultModel은 기본 모델을 설정합니다.
func WithGeminiCLIDefaultModel(model string) GeminiCLIProviderOption {
	return func(p *GeminiCLIProvider) {
		p.config.DefaultModel = model
	}
}

// WithGeminiCLIFallbackCommand는 폴백 명령어를 설정합니다.
// gemini 바이너리를 찾을 수 없을 때 사용할 npx 명령어입니다.
func WithGeminiCLIFallbackCommand(cmd string) GeminiCLIProviderOption {
	return func(p *GeminiCLIProvider) {
		p.fallbackCommand = cmd
	}
}

// NewGeminiCLIProvider는 새로운 GeminiCLIProvider를 생성합니다.
// gemini 바이너리를 먼저 확인하고, 없으면 npx @google/gemini-cli를 폴백으로 사용합니다.
func NewGeminiCLIProvider(opts ...GeminiCLIProviderOption) (*GeminiCLIProvider, error) {
	p := &GeminiCLIProvider{
		cliPath:         "gemini",
		fallbackCommand: "npx @google/gemini-cli",
		timeout:         5 * time.Minute,
		config: ProviderConfig{
			DefaultModel: "gemini-2.0-flash",
		},
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

// checkCLI는 gemini CLI 바이너리가 존재하고 실행 가능한지 확인합니다.
// gemini 바이너리를 먼저 찾고, 없으면 npx를 폴백으로 사용합니다.
func (p *GeminiCLIProvider) checkCLI() error {
	// 1. gemini 바이너리 직접 확인
	path, err := exec.LookPath(p.cliPath)
	if err == nil {
		p.cliPath = path
		p.useFallback = false
		return nil
	}

	// 2. npx 폴백 확인
	npxPath, npxErr := exec.LookPath("npx")
	if npxErr == nil {
		p.cliPath = npxPath
		p.useFallback = true
		return nil
	}

	return fmt.Errorf("%w: gemini 또는 npx를 찾을 수 없습니다", ErrCLINotFound)
}

// Name은 프로바이더 식별자를 반환합니다.
func (p *GeminiCLIProvider) Name() string {
	return "gemini"
}

// ValidateConfig는 프로바이더 설정의 유효성을 검사합니다.
func (p *GeminiCLIProvider) ValidateConfig() error {
	return p.checkCLI()
}

// Supports는 주어진 모델명을 지원하는지 확인합니다.
// CLI 모드에서는 gemini- 접두사를 가진 모든 모델을 지원하며,
// gemini.go에 정의된 geminiSupportedModels 목록도 참조합니다.
func (p *GeminiCLIProvider) Supports(model string) bool {
	if !strings.HasPrefix(model, "gemini-") {
		return false
	}

	// 지원 모델 목록에서 확인
	for _, supported := range geminiSupportedModels {
		if model == supported {
			return true
		}
	}

	// gemini-2.0-*, gemini-1.5-* 패턴 매칭
	if strings.HasPrefix(model, "gemini-2.0-") ||
		strings.HasPrefix(model, "gemini-1.5-") {
		return true
	}

	return false
}

// Execute는 gemini CLI를 통해 프롬프트를 실행하고 결과를 반환합니다.
func (p *GeminiCLIProvider) Execute(ctx context.Context, req ExecuteRequest) (*ExecuteResponse, error) {
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
	var args []string
	if p.useFallback {
		// npx @google/gemini-cli --output-format json --model <model> "prompt"
		args = []string{
			"@google/gemini-cli",
			"--output-format", "json",
			"--model", model,
			req.Prompt,
		}
	} else {
		// gemini --output-format json --model <model> "prompt"
		args = []string{
			"--output-format", "json",
			"--model", model,
			req.Prompt,
		}
	}

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
	// gemini CLI는 여러 줄의 JSON을 출력할 수 있으므로 마지막 result 타입을 찾음
	output := stdout.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	var response *GeminiCLIResponse
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var parsed GeminiCLIResponse
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

// IsGeminiCLIAvailable은 gemini CLI가 사용 가능한지 확인합니다.
// gemini 바이너리 또는 npx 폴백 중 하나라도 사용 가능하면 true를 반환합니다.
// 전역 함수로 레지스트리 초기화 시 사용됩니다.
func IsGeminiCLIAvailable(cliPath, fallbackCmd string) bool {
	// gemini 바이너리 확인
	if cliPath == "" {
		cliPath = "gemini"
	}
	_, err := exec.LookPath(cliPath)
	if err == nil {
		return true
	}

	// npx 폴백 확인
	if fallbackCmd == "" {
		fallbackCmd = "npx @google/gemini-cli"
	}
	// 폴백 명령어에서 실행 파일명 추출 (첫 번째 토큰)
	parts := strings.Fields(fallbackCmd)
	if len(parts) > 0 {
		_, npxErr := exec.LookPath(parts[0])
		return npxErr == nil
	}

	return false
}
