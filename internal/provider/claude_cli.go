// Package provider는 AI 프로바이더 통합 레이어를 제공합니다.
package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

// CLI 관련 에러 정의
var (
	// ErrCLINotFound는 claude CLI 바이너리를 찾을 수 없을 때 반환됩니다.
	ErrCLINotFound = errors.New("claude CLI 바이너리를 찾을 수 없습니다")

	// ErrCLITimeout은 CLI 실행이 타임아웃되었을 때 반환됩니다.
	ErrCLITimeout = errors.New("claude CLI 실행 타임아웃")

	// ErrCLIExecution은 CLI 실행 중 에러가 발생했을 때 반환됩니다.
	ErrCLIExecution = errors.New("claude CLI 실행 실패")
)

// ClaudeCLIResponse는 claude CLI의 JSON 출력 구조입니다.
// claude --print --output-format json 명령의 출력을 파싱합니다.
type ClaudeCLIResponse struct {
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

// ClaudeCLIProvider는 claude CLI를 서브프로세스로 실행하는 프로바이더입니다.
// Claude 구독 사용자가 API 키 없이 CLI를 통해 Claude를 사용할 수 있습니다.
type ClaudeCLIProvider struct {
	cliPath string
	timeout time.Duration
	config  ProviderConfig
}

// ClaudeCLIProviderOption은 ClaudeCLIProvider 설정 옵션입니다.
type ClaudeCLIProviderOption func(*ClaudeCLIProvider)

// WithCLIPath는 CLI 바이너리 경로를 설정합니다.
func WithCLIPath(path string) ClaudeCLIProviderOption {
	return func(p *ClaudeCLIProvider) {
		p.cliPath = path
	}
}

// WithCLITimeout은 CLI 실행 타임아웃을 설정합니다.
func WithCLITimeout(timeout time.Duration) ClaudeCLIProviderOption {
	return func(p *ClaudeCLIProvider) {
		p.timeout = timeout
	}
}

// WithCLIDefaultModel은 기본 모델을 설정합니다.
func WithCLIDefaultModel(model string) ClaudeCLIProviderOption {
	return func(p *ClaudeCLIProvider) {
		p.config.DefaultModel = model
	}
}

// NewClaudeCLIProvider는 새로운 ClaudeCLIProvider를 생성합니다.
func NewClaudeCLIProvider(opts ...ClaudeCLIProviderOption) (*ClaudeCLIProvider, error) {
	p := &ClaudeCLIProvider{
		cliPath: "claude",
		timeout: 5 * time.Minute,
		config: ProviderConfig{
			DefaultModel: "claude-sonnet-4-20250514",
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

// checkCLI는 claude CLI 바이너리가 존재하고 실행 가능한지 확인합니다.
func (p *ClaudeCLIProvider) checkCLI() error {
	path, err := exec.LookPath(p.cliPath)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrCLINotFound, p.cliPath)
	}
	p.cliPath = path
	return nil
}

// Name은 프로바이더 식별자를 반환합니다.
func (p *ClaudeCLIProvider) Name() string {
	return "claude"
}

// ValidateConfig는 프로바이더 설정의 유효성을 검사합니다.
func (p *ClaudeCLIProvider) ValidateConfig() error {
	return p.checkCLI()
}

// Supports는 주어진 모델명을 지원하는지 확인합니다.
// CLI 모드에서는 claude- 접두사를 가진 모든 모델을 지원합니다.
func (p *ClaudeCLIProvider) Supports(model string) bool {
	return strings.HasPrefix(model, "claude-")
}

// Execute는 claude CLI를 통해 프롬프트를 실행하고 결과를 반환합니다.
func (p *ClaudeCLIProvider) Execute(ctx context.Context, req ExecuteRequest) (*ExecuteResponse, error) {
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
	// claude --print --output-format json --model <model> "prompt"
	args := []string{
		"--print",
		"--output-format", "json",
		"--model", model,
	}

	// 시스템 프롬프트 설정
	if req.SystemPrompt != "" {
		args = append(args, "--system-prompt", req.SystemPrompt)
	}

	// 도구 설정 (computer_use 등)
	if len(req.Tools) > 0 {
		args = append(args, "--allowedTools", strings.Join(req.Tools, ","))
	}

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
	// claude CLI는 여러 줄의 JSON을 출력할 수 있으므로 마지막 result 타입을 찾음
	output := stdout.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	var response *ClaudeCLIResponse
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var parsed ClaudeCLIResponse
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

// StreamCallback은 스트리밍 중 텍스트 델타가 발생할 때 호출되는 콜백입니다.
// textDelta는 새로 추가된 텍스트, accumulatedText는 지금까지 누적된 전체 텍스트입니다.
type StreamCallback func(textDelta string, accumulatedText string)

// ExecuteStreaming은 claude CLI를 stream-json 모드로 실행하여 실시간 스트리밍을 제공합니다.
// onDelta 콜백은 블록 스트리밍 로직에 따라 텍스트 청크가 준비될 때 호출됩니다.
func (p *ClaudeCLIProvider) ExecuteStreaming(ctx context.Context, req ExecuteRequest, onDelta StreamCallback) (*ExecuteResponse, error) {
	startTime := time.Now()

	model := req.Model
	if model == "" {
		model = p.config.DefaultModel
	}

	if !p.Supports(model) {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedModel, model)
	}

	execCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	// stream-json 모드로 CLI 명령 구성
	// Claude Code CLI 2.1.x에서는 --print + stream-json 조합에 --verbose가 필요합니다.
	args := []string{
		"--print",
		"--verbose",
		"--output-format", "stream-json",
		"--model", model,
	}

	if req.SystemPrompt != "" {
		args = append(args, "--system-prompt", req.SystemPrompt)
	}

	if len(req.Tools) > 0 {
		args = append(args, "--allowedTools", strings.Join(req.Tools, ","))
	}

	args = append(args, req.Prompt)

	cmd := exec.CommandContext(execCtx, p.cliPath, args...)

	if req.WorkDir != "" {
		if _, err := os.Stat(req.WorkDir); err == nil {
			cmd.Dir = req.WorkDir
		}
	}

	// StdoutPipe로 실시간 출력 읽기
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("%w: stdout pipe 생성 실패: %v", ErrCLIExecution, err)
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("%w: 프로세스 시작 실패: %v", ErrCLIExecution, err)
	}

	// StreamAccumulator로 블록 스트리밍
	accumulator := NewStreamAccumulator()
	var resultLine *StreamLine

	// 타임아웃 기반 플러시를 위한 goroutine
	flushDone := make(chan struct{})
	go func() {
		defer close(flushDone)
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-execCtx.Done():
				return
			case <-flushDone:
				return
			case <-ticker.C:
				if accumulator.ShouldFlush() {
					delta := accumulator.Flush()
					if delta != "" && onDelta != nil {
						onDelta(delta, accumulator.GetAccumulated())
					}
				}
			}
		}
	}()

	// NDJSON 라인 스캔
	scanner := bufio.NewScanner(stdoutPipe)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024) // 최대 1MB 라인
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		parsed, parseErr := ParseStreamLine(line)
		if parseErr != nil {
			continue
		}

		if parsed.IsTextDelta() {
			accumulator.Add(parsed.Delta.Text)
			// ShouldFlush 체크는 ticker goroutine에서도 하지만, 즉시 체크도 수행
			if accumulator.ShouldFlush() {
				delta := accumulator.Flush()
				if delta != "" && onDelta != nil {
					onDelta(delta, accumulator.GetAccumulated())
				}
			}
		}

		if parsed.IsResult() {
			resultLine = parsed
		}
	}

	// 플러시 goroutine 종료 신호 (채널이 이미 닫혔을 수 있으므로 recover)
	func() {
		defer func() { _ = recover() }()
		flushDone <- struct{}{}
	}()

	// 남은 버퍼 플러시
	if accumulator.HasPending() {
		delta := accumulator.FlushAll()
		if delta != "" && onDelta != nil {
			onDelta(delta, accumulator.GetAccumulated())
		}
	}

	// 프로세스 종료 대기
	waitErr := cmd.Wait()

	if execCtx.Err() == context.DeadlineExceeded {
		return nil, fmt.Errorf("%w: %v초 후 타임아웃", ErrCLITimeout, p.timeout.Seconds())
	}
	if execCtx.Err() == context.Canceled {
		return nil, fmt.Errorf("%w: %v", ErrContextCanceled, ctx.Err())
	}

	if waitErr != nil && resultLine == nil {
		return nil, fmt.Errorf("%w: %v, stderr: %s", ErrCLIExecution, waitErr, stderr.String())
	}

	// 결과 구성
	accumulatedText := accumulator.GetAccumulated()

	if resultLine != nil {
		tokenUsage := TokenUsage{
			InputTokens:  resultLine.TotalInputTokens,
			OutputTokens: resultLine.TotalOutputTokens,
			TotalTokens:  resultLine.TotalInputTokens + resultLine.TotalOutputTokens,
		}
		if tokenUsage.TotalTokens == 0 {
			tokenUsage.InputTokens = len(req.Prompt) / 4
			tokenUsage.OutputTokens = len(resultLine.Result) / 4
			tokenUsage.TotalTokens = tokenUsage.InputTokens + tokenUsage.OutputTokens
		}

		durationMs := resultLine.DurationMS
		if durationMs == 0 {
			durationMs = time.Since(startTime).Milliseconds()
		}

		output := resultLine.Result
		if output == "" {
			output = accumulatedText
		}

		return &ExecuteResponse{
			Output:     output,
			TokenUsage: tokenUsage,
			DurationMs: durationMs,
			Model:      model,
			StopReason: "end_turn",
		}, nil
	}

	// result 이벤트가 없는 경우 누적 텍스트 사용
	if accumulatedText != "" {
		log.Printf("[ClaudeCLI] stream-json: result 이벤트 없음, 누적 텍스트 사용 (%d chars)", len(accumulatedText))
		estimatedInputTokens := len(req.Prompt) / 4
		estimatedOutputTokens := len(accumulatedText) / 4
		return &ExecuteResponse{
			Output: accumulatedText,
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

	return nil, fmt.Errorf("%w: stream-json 출력에서 결과를 추출할 수 없습니다, stderr: %s", ErrCLIExecution, stderr.String())
}

// IsCLIAvailable은 claude CLI가 사용 가능한지 확인합니다.
// 전역 함수로 레지스트리 초기화 시 사용됩니다.
func IsCLIAvailable(cliPath string) bool {
	if cliPath == "" {
		cliPath = "claude"
	}
	_, err := exec.LookPath(cliPath)
	return err == nil
}
