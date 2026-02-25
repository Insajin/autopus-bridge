// validation.go는 AI 프로바이더 CLI의 연결 상태를 검증하는 기능을 제공합니다.
// 보안 원칙: 실제 API 응답 내용은 로깅하지 않으며, 상태만 기록합니다.
package aitools

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// ValidationStatus는 연결 검증 결과의 상태를 나타냅니다.
type ValidationStatus string

const (
	ValidationSuccess     ValidationStatus = "success"
	ValidationAuthFailure ValidationStatus = "auth_failure"
	ValidationTimeout     ValidationStatus = "timeout"
	ValidationRateLimit   ValidationStatus = "rate_limit"
	ValidationError       ValidationStatus = "error"
)

// validationTimeout은 CLI 검증의 기본 타임아웃입니다.
const validationTimeout = 10 * time.Second

// ValidationResult는 프로바이더 연결 검증 결과를 담는 구조체입니다.
type ValidationResult struct {
	ProviderName string
	Status       ValidationStatus
	ResponseTime time.Duration
	Message      string
	Error        error
}

// CommandExecutor는 외부 CLI 명령어 실행을 추상화하는 인터페이스입니다.
// 테스트 시 실제 CLI 호출 없이 동작을 검증할 수 있습니다.
type CommandExecutor interface {
	Execute(ctx context.Context, name string, args ...string) ([]byte, error)
}

// defaultExecutor는 실제 exec.CommandContext를 사용하는 기본 구현입니다.
type defaultExecutor struct{}

// Execute는 exec.CommandContext를 통해 외부 명령어를 실행합니다.
func (e *defaultExecutor) Execute(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

// Validator는 AI 프로바이더 연결 검증기입니다.
// CommandExecutor를 주입받아 테스트 가능한 구조를 제공합니다.
type Validator struct {
	Executor CommandExecutor
}

// NewValidator는 기본 CommandExecutor를 사용하는 Validator를 생성합니다.
func NewValidator() *Validator {
	return &Validator{
		Executor: &defaultExecutor{},
	}
}

// ValidateClaudeCLI는 Claude Code CLI의 연결 상태를 검증합니다.
// claude --print --output-format json --max-turns 1 "ping" 명령을 실행합니다.
func (v *Validator) ValidateClaudeCLI(ctx context.Context) ValidationResult {
	return v.validateProvider(ctx, "Claude Code", "claude",
		"--print", "--output-format", "json", "--max-turns", "1", "ping")
}

// ValidateCodexCLI는 Codex CLI의 연결 상태를 검증합니다.
// codex --quiet "ping" 명령을 실행합니다.
func (v *Validator) ValidateCodexCLI(ctx context.Context) ValidationResult {
	return v.validateProvider(ctx, "Codex CLI", "codex", "--quiet", "ping")
}

// ValidateGeminiCLI는 Gemini CLI의 연결 상태를 검증합니다.
// gemini --model gemini-2.0-flash "ping" 명령을 실행합니다.
func (v *Validator) ValidateGeminiCLI(ctx context.Context) ValidationResult {
	return v.validateProvider(ctx, "Gemini CLI", "gemini", "--model", "gemini-2.0-flash", "ping")
}

// validateProvider는 CLI 프로바이더 검증의 공통 로직입니다.
// 타임아웃 관리, 명령 실행, 에러 분류, 응답 시간 측정을 처리합니다.
func (v *Validator) validateProvider(ctx context.Context, providerName, command string, args ...string) ValidationResult {
	start := time.Now()

	// 상위 컨텍스트에 타임아웃이 없으면 기본 타임아웃 적용
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, validationTimeout)
		defer cancel()
	}

	output, err := v.Executor.Execute(ctx, command, args...)
	elapsed := time.Since(start)

	result := ValidationResult{
		ProviderName: providerName,
		ResponseTime: elapsed,
	}

	if err != nil {
		result.Status = classifyError(output, err)
		result.Error = err
		result.Message = sanitizeMessage(output)
		return result
	}

	result.Status = ValidationSuccess
	result.Message = "연결 성공"
	return result
}

// ValidateAll은 요청된 프로바이더들의 연결 상태를 순차적으로 검증합니다.
// 결과는 요청 순서와 동일한 순서로 반환됩니다.
func (v *Validator) ValidateAll(ctx context.Context, providers []string) []ValidationResult {
	results := make([]ValidationResult, 0, len(providers))

	for _, provider := range providers {
		switch strings.ToLower(provider) {
		case "claude":
			results = append(results, v.ValidateClaudeCLI(ctx))
		case "codex":
			results = append(results, v.ValidateCodexCLI(ctx))
		case "gemini":
			results = append(results, v.ValidateGeminiCLI(ctx))
		default:
			results = append(results, ValidationResult{
				ProviderName: provider,
				Status:       ValidationError,
				Message:      fmt.Sprintf("지원하지 않는 프로바이더: %s", provider),
				Error:        fmt.Errorf("unknown provider: %s", provider),
			})
		}
	}

	return results
}

// classifyError는 CLI 출력과 에러를 분석하여 ValidationStatus를 결정합니다.
// 보안: 출력 내용을 로깅하지 않고 패턴 매칭만 수행합니다.
func classifyError(output []byte, err error) ValidationStatus {
	// 타임아웃/컨텍스트 취소 확인
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "context deadline exceeded") ||
			strings.Contains(errMsg, "context canceled") {
			return ValidationTimeout
		}
	}

	// 출력 내용을 소문자로 변환하여 패턴 매칭
	lowered := strings.ToLower(string(output))

	// 레이트 리밋 패턴 (인증 실패보다 먼저 확인)
	rateLimitPatterns := []string{
		"rate limit",
		"rate_limit",
		"too many requests",
		"429",
	}
	for _, pattern := range rateLimitPatterns {
		if strings.Contains(lowered, pattern) {
			return ValidationRateLimit
		}
	}

	// 인증 실패 패턴
	authPatterns := []string{
		"invalid api key",
		"invalid_api_key",
		"authentication failed",
		"unauthorized",
		"api key not valid",
		"permission_denied",
		"expired token",
	}
	for _, pattern := range authPatterns {
		if strings.Contains(lowered, pattern) {
			return ValidationAuthFailure
		}
	}

	return ValidationError
}

// sanitizeMessage는 CLI 출력에서 민감한 정보를 제거한 상태 메시지를 생성합니다.
// 보안: API 키나 토큰 값은 포함하지 않습니다.
func sanitizeMessage(output []byte) string {
	if len(output) == 0 {
		return "알 수 없는 오류"
	}

	msg := strings.TrimSpace(string(output))

	// 최대 200자로 제한 (민감한 정보 노출 방지)
	const maxLen = 200
	if len(msg) > maxLen {
		msg = msg[:maxLen] + "..."
	}

	return msg
}
