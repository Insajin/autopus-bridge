// Package provider는 AI 프로바이더 통합 레이어를 제공합니다.
// REQ-E-02: 작업 요청 시 해당 AI 프로바이더를 통해 작업 실행
// REQ-S-05: 설정된 AI 프로바이더가 없으면 연결 시도 거부
package provider

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
)

// 프로바이더 관련 에러 정의
var (
	// ErrNoAPIKey는 API 키가 설정되지 않았을 때 반환됩니다.
	ErrNoAPIKey = errors.New("API 키가 설정되지 않았습니다")

	// ErrUnsupportedModel은 지원하지 않는 모델을 요청했을 때 반환됩니다.
	ErrUnsupportedModel = errors.New("지원하지 않는 모델입니다")

	// ErrProviderNotFound는 프로바이더를 찾을 수 없을 때 반환됩니다.
	ErrProviderNotFound = errors.New("프로바이더를 찾을 수 없습니다")

	// ErrRateLimited는 API 레이트 리밋에 걸렸을 때 반환됩니다.
	ErrRateLimited = errors.New("API 레이트 리밋 초과")

	// ErrContextCanceled는 컨텍스트가 취소되었을 때 반환됩니다.
	ErrContextCanceled = errors.New("요청이 취소되었습니다")
)

// Provider는 AI 프로바이더 인터페이스입니다.
// 각 AI 서비스(Claude, Gemini 등)는 이 인터페이스를 구현합니다.
type Provider interface {
	// Name은 프로바이더 식별자를 반환합니다 (예: "claude", "gemini").
	Name() string

	// Execute는 프롬프트를 실행하고 결과를 반환합니다.
	Execute(ctx context.Context, req ExecuteRequest) (*ExecuteResponse, error)

	// ValidateConfig는 프로바이더 설정의 유효성을 검사합니다.
	// API 키 존재 여부 등을 확인합니다.
	ValidateConfig() error

	// Supports는 주어진 모델명을 지원하는지 확인합니다.
	Supports(model string) bool
}

// ExecuteRequest는 AI 프로바이더 실행 요청입니다.
type ExecuteRequest struct {
	// Prompt는 AI에게 전달할 프롬프트입니다.
	Prompt string

	// Model은 사용할 모델명입니다.
	// 예: "claude-sonnet-4-20250514", "gemini-2.0-flash"
	Model string

	// MaxTokens는 생성할 최대 토큰 수입니다.
	MaxTokens int

	// Tools는 사용 가능한 도구 목록입니다.
	Tools []string

	// WorkDir는 작업 디렉토리입니다.
	WorkDir string

	// SystemPrompt는 시스템 프롬프트입니다 (선택적).
	SystemPrompt string
}

// ExecuteResponse는 AI 프로바이더 실행 결과입니다.
type ExecuteResponse struct {
	// Output은 AI의 응답 텍스트입니다.
	Output string

	// TokenUsage는 토큰 사용량입니다.
	TokenUsage TokenUsage

	// DurationMs는 실행 시간(밀리초)입니다.
	DurationMs int64

	// Model은 실제 사용된 모델명입니다.
	Model string

	// StopReason은 응답 종료 사유입니다.
	// 예: "end_turn", "max_tokens", "stop_sequence", "tool_use"
	StopReason string

	// ToolCalls는 모델이 요청한 도구 호출 목록입니다.
	// StopReason이 "tool_use"일 때 채워집니다.
	ToolCalls []ToolCall
}

// ToolCall은 모델이 요청한 단일 도구 호출입니다.
type ToolCall struct {
	// ID는 도구 호출의 고유 식별자입니다.
	ID string `json:"id"`

	// Name은 호출할 도구의 이름입니다 (예: "computer").
	Name string `json:"name"`

	// Input은 도구에 전달할 파라미터입니다 (JSON).
	Input json.RawMessage `json:"input"`
}

// ProviderCapabilities는 프로바이더가 지원하는 기능을 나타냅니다.
type ProviderCapabilities struct {
	// SupportsComputerUse는 computer_use 도구 지원 여부입니다.
	SupportsComputerUse bool
}

// RedactScreenshotData는 로그용으로 base64 스크린샷 데이터를 제거합니다.
// tool_result에 포함된 base64 이미지 데이터를 "[SCREENSHOT_REDACTED]"로 대체합니다.
func RedactScreenshotData(input string) string {
	// base64 인코딩된 이미지 데이터 패턴 감지 (data:image/... 또는 긴 base64 문자열)
	if strings.Contains(input, "\"type\":\"image\"") ||
		strings.Contains(input, "\"type\": \"image\"") ||
		strings.Contains(input, "\"media_type\":\"image/") ||
		strings.Contains(input, "\"media_type\": \"image/") {
		return "[SCREENSHOT_REDACTED]"
	}

	// 매우 긴 base64 문자열 감지 (1KB 이상의 연속 base64 문자)
	if len(input) > 1024 {
		// base64 데이터가 포함된 "data" 필드 확인
		if strings.Contains(input, "\"data\":\"") || strings.Contains(input, "\"data\": \"") {
			return "[SCREENSHOT_REDACTED: " + input[:100] + "...]"
		}
	}

	return input
}

// TokenUsage는 토큰 사용량을 추적합니다.
type TokenUsage struct {
	// InputTokens는 입력 토큰 수입니다.
	InputTokens int

	// OutputTokens는 출력 토큰 수입니다.
	OutputTokens int

	// TotalTokens는 총 토큰 수입니다.
	TotalTokens int

	// CacheRead는 캐시에서 읽은 토큰 수입니다.
	CacheRead int

	// CacheCreation은 캐시 생성에 사용된 토큰 수입니다.
	CacheCreation int
}

// ToWSTokenUsage는 TokenUsage를 ws.TokenUsage로 변환합니다.
// WebSocket 메시지 전송 시 사용됩니다.
func (t *TokenUsage) ToWSTokenUsage() interface{} {
	// ws.TokenUsage 구조체와 동일한 필드를 가진 익명 구조체 반환
	return struct {
		InputTokens   int `json:"input_tokens"`
		OutputTokens  int `json:"output_tokens"`
		TotalTokens   int `json:"total_tokens"`
		CacheRead     int `json:"cache_read,omitempty"`
		CacheCreation int `json:"cache_creation,omitempty"`
	}{
		InputTokens:   t.InputTokens,
		OutputTokens:  t.OutputTokens,
		TotalTokens:   t.TotalTokens,
		CacheRead:     t.CacheRead,
		CacheCreation: t.CacheCreation,
	}
}

// ProviderConfig는 프로바이더 공통 설정입니다.
type ProviderConfig struct {
	// APIKey는 API 인증 키입니다.
	// REQ-N-01: 환경변수에서만 가져옴
	APIKey string

	// DefaultModel은 기본 사용 모델입니다.
	DefaultModel string

	// MaxRetries는 최대 재시도 횟수입니다.
	MaxRetries int

	// RetryDelayMs는 재시도 간 지연 시간(밀리초)입니다.
	RetryDelayMs int
}
