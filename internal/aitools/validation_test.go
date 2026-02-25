package aitools

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// testExecutor는 사전 설정된 응답을 반환하는 테스트용 CommandExecutor입니다.
type testExecutor struct {
	output []byte
	err    error
	delay  time.Duration
}

// Execute는 설정된 지연 시간 후 사전 정의된 응답을 반환합니다.
// 컨텍스트 취소 시 즉시 에러를 반환합니다.
func (e *testExecutor) Execute(ctx context.Context, name string, args ...string) ([]byte, error) {
	if e.delay > 0 {
		select {
		case <-time.After(e.delay):
			// 지연 완료
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return e.output, e.err
}

// ---------------------------------------------------------------------------
// UT-013: ValidateClaudeCLI 성공 시나리오
// ---------------------------------------------------------------------------

func TestValidateClaudeCLI_Success(t *testing.T) {
	// Arrange: Claude CLI가 정상 응답을 반환하는 경우
	executor := &testExecutor{
		output: []byte(`{"result": "pong"}`),
		err:    nil,
	}
	v := &Validator{Executor: executor}

	// Act
	ctx := context.Background()
	result := v.ValidateClaudeCLI(ctx)

	// Assert
	if result.ProviderName != "Claude Code" {
		t.Errorf("ProviderName = %q, want %q", result.ProviderName, "Claude Code")
	}
	if result.Status != ValidationSuccess {
		t.Errorf("Status = %q, want %q", result.Status, ValidationSuccess)
	}
	if result.Error != nil {
		t.Errorf("Error가 nil이어야 하지만 %v를 받았습니다", result.Error)
	}
	if result.ResponseTime <= 0 {
		t.Errorf("ResponseTime이 양수여야 하지만 %v를 받았습니다", result.ResponseTime)
	}
}

// ---------------------------------------------------------------------------
// UT-014: ValidateClaudeCLI 타임아웃 시나리오
// ---------------------------------------------------------------------------

func TestValidateClaudeCLI_Timeout(t *testing.T) {
	// Arrange: CLI 응답이 타임아웃보다 오래 걸리는 경우
	executor := &testExecutor{
		output: nil,
		err:    nil,
		delay:  5 * time.Second, // 컨텍스트 타임아웃보다 긴 지연
	}
	v := &Validator{Executor: executor}

	// 50ms 타임아웃 컨텍스트 사용 (테스트 속도를 위해 짧게 설정)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Act
	result := v.ValidateClaudeCLI(ctx)

	// Assert
	if result.Status != ValidationTimeout {
		t.Errorf("Status = %q, want %q", result.Status, ValidationTimeout)
	}
	if result.Error == nil {
		t.Error("타임아웃 시 Error가 nil이면 안 됩니다")
	}
}

// ---------------------------------------------------------------------------
// UT-015: ValidateClaudeCLI 인증 실패 시나리오
// ---------------------------------------------------------------------------

func TestValidateClaudeCLI_AuthFailure(t *testing.T) {
	// Arrange: CLI가 인증 관련 에러를 반환하는 경우
	executor := &testExecutor{
		output: []byte("Error: Invalid API key"),
		err:    errors.New("exit status 1"),
	}
	v := &Validator{Executor: executor}

	ctx := context.Background()

	// Act
	result := v.ValidateClaudeCLI(ctx)

	// Assert
	if result.Status != ValidationAuthFailure {
		t.Errorf("Status = %q, want %q", result.Status, ValidationAuthFailure)
	}
	if result.Error == nil {
		t.Error("인증 실패 시 Error가 nil이면 안 됩니다")
	}
}

// ---------------------------------------------------------------------------
// ValidateClaudeCLI 레이트 리밋 시나리오
// ---------------------------------------------------------------------------

func TestValidateClaudeCLI_RateLimit(t *testing.T) {
	// Arrange: CLI가 레이트 리밋 에러를 반환하는 경우
	executor := &testExecutor{
		output: []byte("Error: rate limit exceeded"),
		err:    errors.New("exit status 1"),
	}
	v := &Validator{Executor: executor}

	ctx := context.Background()

	// Act
	result := v.ValidateClaudeCLI(ctx)

	// Assert
	if result.Status != ValidationRateLimit {
		t.Errorf("Status = %q, want %q", result.Status, ValidationRateLimit)
	}
}

// ---------------------------------------------------------------------------
// ValidateClaudeCLI 일반 에러 시나리오
// ---------------------------------------------------------------------------

func TestValidateClaudeCLI_GenericError(t *testing.T) {
	// Arrange: CLI가 인식할 수 없는 에러를 반환하는 경우
	executor := &testExecutor{
		output: []byte("something went wrong"),
		err:    errors.New("exit status 1"),
	}
	v := &Validator{Executor: executor}

	ctx := context.Background()

	// Act
	result := v.ValidateClaudeCLI(ctx)

	// Assert
	if result.Status != ValidationError {
		t.Errorf("Status = %q, want %q", result.Status, ValidationError)
	}
	if result.Error == nil {
		t.Error("일반 에러 시 Error가 nil이면 안 됩니다")
	}
}

// ---------------------------------------------------------------------------
// ValidateCodexCLI 테스트
// ---------------------------------------------------------------------------

func TestValidateCodexCLI_Success(t *testing.T) {
	executor := &testExecutor{
		output: []byte("pong"),
		err:    nil,
	}
	v := &Validator{Executor: executor}

	ctx := context.Background()
	result := v.ValidateCodexCLI(ctx)

	if result.ProviderName != "Codex CLI" {
		t.Errorf("ProviderName = %q, want %q", result.ProviderName, "Codex CLI")
	}
	if result.Status != ValidationSuccess {
		t.Errorf("Status = %q, want %q", result.Status, ValidationSuccess)
	}
}

func TestValidateCodexCLI_Timeout(t *testing.T) {
	executor := &testExecutor{
		delay: 5 * time.Second,
	}
	v := &Validator{Executor: executor}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	result := v.ValidateCodexCLI(ctx)

	if result.Status != ValidationTimeout {
		t.Errorf("Status = %q, want %q", result.Status, ValidationTimeout)
	}
}

func TestValidateCodexCLI_AuthFailure(t *testing.T) {
	executor := &testExecutor{
		output: []byte("authentication failed"),
		err:    errors.New("exit status 1"),
	}
	v := &Validator{Executor: executor}

	ctx := context.Background()
	result := v.ValidateCodexCLI(ctx)

	if result.Status != ValidationAuthFailure {
		t.Errorf("Status = %q, want %q", result.Status, ValidationAuthFailure)
	}
}

// ---------------------------------------------------------------------------
// ValidateGeminiCLI 테스트
// ---------------------------------------------------------------------------

func TestValidateGeminiCLI_Success(t *testing.T) {
	executor := &testExecutor{
		output: []byte("pong response"),
		err:    nil,
	}
	v := &Validator{Executor: executor}

	ctx := context.Background()
	result := v.ValidateGeminiCLI(ctx)

	if result.ProviderName != "Gemini CLI" {
		t.Errorf("ProviderName = %q, want %q", result.ProviderName, "Gemini CLI")
	}
	if result.Status != ValidationSuccess {
		t.Errorf("Status = %q, want %q", result.Status, ValidationSuccess)
	}
}

func TestValidateGeminiCLI_Timeout(t *testing.T) {
	executor := &testExecutor{
		delay: 5 * time.Second,
	}
	v := &Validator{Executor: executor}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	result := v.ValidateGeminiCLI(ctx)

	if result.Status != ValidationTimeout {
		t.Errorf("Status = %q, want %q", result.Status, ValidationTimeout)
	}
}

func TestValidateGeminiCLI_AuthFailure(t *testing.T) {
	executor := &testExecutor{
		output: []byte("API key not valid"),
		err:    errors.New("exit status 1"),
	}
	v := &Validator{Executor: executor}

	ctx := context.Background()
	result := v.ValidateGeminiCLI(ctx)

	if result.Status != ValidationAuthFailure {
		t.Errorf("Status = %q, want %q", result.Status, ValidationAuthFailure)
	}
}

// ---------------------------------------------------------------------------
// ValidateAll 테스트
// ---------------------------------------------------------------------------

func TestValidateAll_AllProviders(t *testing.T) {
	executor := &testExecutor{
		output: []byte(`{"result": "ok"}`),
		err:    nil,
	}
	v := &Validator{Executor: executor}

	ctx := context.Background()
	results := v.ValidateAll(ctx, []string{"claude", "codex", "gemini"})

	if len(results) != 3 {
		t.Fatalf("결과 개수 = %d, want 3", len(results))
	}

	// 순서가 요청 순서와 동일한지 확인
	expectedNames := []string{"Claude Code", "Codex CLI", "Gemini CLI"}
	for i, expected := range expectedNames {
		if results[i].ProviderName != expected {
			t.Errorf("results[%d].ProviderName = %q, want %q", i, results[i].ProviderName, expected)
		}
		if results[i].Status != ValidationSuccess {
			t.Errorf("results[%d].Status = %q, want %q", i, results[i].Status, ValidationSuccess)
		}
	}
}

func TestValidateAll_SingleProvider(t *testing.T) {
	executor := &testExecutor{
		output: []byte("ok"),
		err:    nil,
	}
	v := &Validator{Executor: executor}

	ctx := context.Background()
	results := v.ValidateAll(ctx, []string{"claude"})

	if len(results) != 1 {
		t.Fatalf("결과 개수 = %d, want 1", len(results))
	}
	if results[0].ProviderName != "Claude Code" {
		t.Errorf("ProviderName = %q, want %q", results[0].ProviderName, "Claude Code")
	}
}

func TestValidateAll_UnknownProvider(t *testing.T) {
	executor := &testExecutor{
		output: []byte("ok"),
		err:    nil,
	}
	v := &Validator{Executor: executor}

	ctx := context.Background()
	results := v.ValidateAll(ctx, []string{"unknown"})

	if len(results) != 1 {
		t.Fatalf("결과 개수 = %d, want 1", len(results))
	}
	if results[0].Status != ValidationError {
		t.Errorf("Status = %q, want %q", results[0].Status, ValidationError)
	}
	if !strings.Contains(results[0].Message, "unknown") {
		t.Errorf("Message에 프로바이더 이름이 포함되어야 합니다: %q", results[0].Message)
	}
}

func TestValidateAll_EmptyProviders(t *testing.T) {
	executor := &testExecutor{}
	v := &Validator{Executor: executor}

	ctx := context.Background()
	results := v.ValidateAll(ctx, []string{})

	if len(results) != 0 {
		t.Errorf("빈 프로바이더 목록에서 결과 개수 = %d, want 0", len(results))
	}
}

func TestValidateAll_NilProviders(t *testing.T) {
	executor := &testExecutor{}
	v := &Validator{Executor: executor}

	ctx := context.Background()
	results := v.ValidateAll(ctx, nil)

	if results == nil {
		t.Error("nil 프로바이더 목록에서 결과가 nil이면 안 됩니다")
	}
	if len(results) != 0 {
		t.Errorf("nil 프로바이더 목록에서 결과 개수 = %d, want 0", len(results))
	}
}

// ---------------------------------------------------------------------------
// defaultExecutor 테스트
// ---------------------------------------------------------------------------

func TestDefaultExecutor_Implements_CommandExecutor(t *testing.T) {
	// defaultExecutor가 CommandExecutor 인터페이스를 구현하는지 컴파일 타임 확인
	var _ CommandExecutor = &defaultExecutor{}
}

// ---------------------------------------------------------------------------
// NewValidator 편의 함수 테스트
// ---------------------------------------------------------------------------

func TestNewValidator_UsesDefaultExecutor(t *testing.T) {
	v := NewValidator()
	if v == nil {
		t.Fatal("NewValidator()가 nil을 반환하면 안 됩니다")
	}
	if v.Executor == nil {
		t.Fatal("NewValidator().Executor가 nil이면 안 됩니다")
	}

	// defaultExecutor 타입인지 확인
	if _, ok := v.Executor.(*defaultExecutor); !ok {
		t.Error("NewValidator().Executor가 *defaultExecutor 타입이어야 합니다")
	}
}

// ---------------------------------------------------------------------------
// 컨텍스트 취소 시나리오
// ---------------------------------------------------------------------------

func TestValidateClaudeCLI_ContextCancelled(t *testing.T) {
	executor := &testExecutor{
		delay: 5 * time.Second,
	}
	v := &Validator{Executor: executor}

	// 이미 취소된 컨텍스트
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result := v.ValidateClaudeCLI(ctx)

	if result.Status != ValidationTimeout {
		t.Errorf("Status = %q, want %q", result.Status, ValidationTimeout)
	}
}

// ---------------------------------------------------------------------------
// 인증 에러 패턴 인식 테스트 (다양한 에러 메시지)
// ---------------------------------------------------------------------------

func TestClassifyError_AuthPatterns(t *testing.T) {
	authMessages := []string{
		"Error: Invalid API key",
		"invalid api key provided",
		"authentication failed",
		"unauthorized access",
		"API key not valid",
		"invalid_api_key",
		"PERMISSION_DENIED",
		"expired token",
	}

	for _, msg := range authMessages {
		t.Run(msg, func(t *testing.T) {
			status := classifyError([]byte(msg), errors.New("exit status 1"))
			if status != ValidationAuthFailure {
				t.Errorf("classifyError(%q) = %q, want %q", msg, status, ValidationAuthFailure)
			}
		})
	}
}

func TestClassifyError_RateLimitPatterns(t *testing.T) {
	rateLimitMessages := []string{
		"rate limit exceeded",
		"rate_limit_error",
		"too many requests",
		"429 Too Many Requests",
	}

	for _, msg := range rateLimitMessages {
		t.Run(msg, func(t *testing.T) {
			status := classifyError([]byte(msg), errors.New("exit status 1"))
			if status != ValidationRateLimit {
				t.Errorf("classifyError(%q) = %q, want %q", msg, status, ValidationRateLimit)
			}
		})
	}
}

func TestClassifyError_GenericError(t *testing.T) {
	status := classifyError([]byte("some unknown error"), errors.New("exit status 1"))
	if status != ValidationError {
		t.Errorf("classifyError = %q, want %q", status, ValidationError)
	}
}

// ---------------------------------------------------------------------------
// ResponseTime이 항상 기록되는지 확인
// ---------------------------------------------------------------------------

func TestValidateClaudeCLI_ResponseTimeAlwaysRecorded(t *testing.T) {
	tests := []struct {
		name     string
		executor *testExecutor
	}{
		{
			name: "성공 시",
			executor: &testExecutor{
				output: []byte(`{"result": "ok"}`),
			},
		},
		{
			name: "에러 시",
			executor: &testExecutor{
				output: []byte("error"),
				err:    errors.New("failed"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := &Validator{Executor: tt.executor}
			result := v.ValidateClaudeCLI(context.Background())
			if result.ResponseTime <= 0 {
				t.Errorf("ResponseTime이 양수여야 하지만 %v를 받았습니다", result.ResponseTime)
			}
		})
	}
}
