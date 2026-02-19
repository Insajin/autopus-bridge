// Package provider는 AI 프로바이더 통합 레이어를 제공합니다.
package provider

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// Gemini 지원 모델 목록
var geminiSupportedModels = []string{
	// Gemini 2.0
	"gemini-2.0-flash",
	"gemini-2.0-flash-exp",
	"gemini-2.0-flash-thinking-exp",
	// Gemini 1.5
	"gemini-1.5-pro",
	"gemini-1.5-pro-latest",
	"gemini-1.5-flash",
	"gemini-1.5-flash-latest",
}

// GeminiProvider는 Google Gemini API 프로바이더입니다.
type GeminiProvider struct {
	client *genai.Client
	config ProviderConfig
}

// GeminiProviderOption은 GeminiProvider 설정 옵션입니다.
type GeminiProviderOption func(*GeminiProvider)

// WithGeminiAPIKey는 API 키를 설정합니다.
func WithGeminiAPIKey(apiKey string) GeminiProviderOption {
	return func(p *GeminiProvider) {
		p.config.APIKey = apiKey
	}
}

// WithGeminiDefaultModel은 기본 모델을 설정합니다.
func WithGeminiDefaultModel(model string) GeminiProviderOption {
	return func(p *GeminiProvider) {
		p.config.DefaultModel = model
	}
}

// WithGeminiMaxRetries는 최대 재시도 횟수를 설정합니다.
func WithGeminiMaxRetries(retries int) GeminiProviderOption {
	return func(p *GeminiProvider) {
		p.config.MaxRetries = retries
	}
}

// NewGeminiProvider는 새로운 GeminiProvider를 생성합니다.
// API 키는 환경변수 GEMINI_API_KEY에서 가져옵니다 (REQ-N-01).
func NewGeminiProvider(ctx context.Context, opts ...GeminiProviderOption) (*GeminiProvider, error) {
	p := &GeminiProvider{
		config: ProviderConfig{
			DefaultModel: "gemini-2.0-flash",
			MaxRetries:   3,
			RetryDelayMs: 1000,
		},
	}

	// 옵션 적용
	for _, opt := range opts {
		opt(p)
	}

	// API 키가 설정되지 않은 경우 환경변수에서 가져옴
	if p.config.APIKey == "" {
		p.config.APIKey = os.Getenv("GEMINI_API_KEY")
	}

	// API 키 검증
	if p.config.APIKey == "" {
		return nil, fmt.Errorf("%w: GEMINI_API_KEY 환경변수를 설정하세요", ErrNoAPIKey)
	}

	// Gemini 클라이언트 생성
	client, err := genai.NewClient(ctx, option.WithAPIKey(p.config.APIKey))
	if err != nil {
		return nil, fmt.Errorf("gemini 클라이언트 생성 실패: %w", err)
	}
	p.client = client

	return p, nil
}

// Name은 프로바이더 식별자를 반환합니다.
func (p *GeminiProvider) Name() string {
	return "gemini"
}

// ValidateConfig는 프로바이더 설정의 유효성을 검사합니다.
func (p *GeminiProvider) ValidateConfig() error {
	if p.config.APIKey == "" {
		return fmt.Errorf("%w: GEMINI_API_KEY", ErrNoAPIKey)
	}
	return nil
}

// Supports는 주어진 모델명을 지원하는지 확인합니다.
func (p *GeminiProvider) Supports(model string) bool {
	// gemini- 접두사로 시작하는지 확인
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

// Execute는 프롬프트를 실행하고 결과를 반환합니다.
func (p *GeminiProvider) Execute(ctx context.Context, req ExecuteRequest) (*ExecuteResponse, error) {
	startTime := time.Now()

	// 모델 결정
	modelName := req.Model
	if modelName == "" {
		modelName = p.config.DefaultModel
	}

	// 지원 모델 확인
	if !p.Supports(modelName) {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedModel, modelName)
	}

	// 모델 인스턴스 생성
	model := p.client.GenerativeModel(modelName)

	// MaxTokens 설정
	if req.MaxTokens > 0 {
		model.SetMaxOutputTokens(int32(req.MaxTokens))
	}

	// 시스템 프롬프트 설정
	if req.SystemPrompt != "" {
		model.SystemInstruction = &genai.Content{
			Parts: []genai.Part{genai.Text(req.SystemPrompt)},
		}
	}

	// 재시도 로직과 함께 API 호출
	var response *genai.GenerateContentResponse
	var lastErr error

	for attempt := 0; attempt <= p.config.MaxRetries; attempt++ {
		if attempt > 0 {
			// 재시도 전 지연
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("%w: %v", ErrContextCanceled, ctx.Err())
			case <-time.After(time.Duration(p.config.RetryDelayMs*attempt) * time.Millisecond):
			}
		}

		response, lastErr = model.GenerateContent(ctx, genai.Text(req.Prompt))
		if lastErr == nil {
			break
		}

		// 레이트 리밋 에러 확인
		if isGeminiRateLimitError(lastErr) {
			lastErr = fmt.Errorf("%w: %v", ErrRateLimited, lastErr)
			continue
		}

		// 컨텍스트 취소 확인
		if errors.Is(lastErr, context.Canceled) || errors.Is(lastErr, context.DeadlineExceeded) {
			return nil, fmt.Errorf("%w: %v", ErrContextCanceled, lastErr)
		}

		// 재시도 불가능한 에러는 즉시 반환
		if !isGeminiRetryableError(lastErr) {
			break
		}
	}

	if lastErr != nil {
		return nil, fmt.Errorf("gemini API 호출 실패: %w", lastErr)
	}

	// 응답 텍스트 추출
	outputText := extractGeminiText(response)

	// 토큰 사용량
	tokenUsage := TokenUsage{}
	if response.UsageMetadata != nil {
		tokenUsage.InputTokens = int(response.UsageMetadata.PromptTokenCount)
		tokenUsage.OutputTokens = int(response.UsageMetadata.CandidatesTokenCount)
		tokenUsage.TotalTokens = int(response.UsageMetadata.TotalTokenCount)
		if response.UsageMetadata.CachedContentTokenCount > 0 {
			tokenUsage.CacheRead = int(response.UsageMetadata.CachedContentTokenCount)
		}
	}

	// 종료 사유 추출
	stopReason := ""
	if len(response.Candidates) > 0 && response.Candidates[0].FinishReason != genai.FinishReasonUnspecified {
		stopReason = response.Candidates[0].FinishReason.String()
	}

	// 실행 시간 계산
	durationMs := time.Since(startTime).Milliseconds()

	return &ExecuteResponse{
		Output:     outputText,
		TokenUsage: tokenUsage,
		DurationMs: durationMs,
		Model:      modelName,
		StopReason: stopReason,
	}, nil
}

// Close는 Gemini 클라이언트를 닫습니다.
func (p *GeminiProvider) Close() error {
	if p.client != nil {
		return p.client.Close()
	}
	return nil
}

// extractGeminiText는 Gemini 응답에서 텍스트를 추출합니다.
func extractGeminiText(response *genai.GenerateContentResponse) string {
	if response == nil || len(response.Candidates) == 0 {
		return ""
	}

	var parts []string
	for _, candidate := range response.Candidates {
		if candidate.Content == nil {
			continue
		}
		for _, part := range candidate.Content.Parts {
			if text, ok := part.(genai.Text); ok {
				parts = append(parts, string(text))
			}
		}
	}

	return strings.Join(parts, "")
}

// isGeminiRateLimitError는 레이트 리밋 에러인지 확인합니다.
func isGeminiRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "RESOURCE_EXHAUSTED") ||
		strings.Contains(errStr, "429") ||
		strings.Contains(errStr, "quota") ||
		strings.Contains(errStr, "rate limit")
}

// isGeminiRetryableError는 재시도 가능한 에러인지 확인합니다.
func isGeminiRetryableError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	// 서버 에러(5xx) 또는 타임아웃은 재시도 가능
	return strings.Contains(errStr, "500") ||
		strings.Contains(errStr, "502") ||
		strings.Contains(errStr, "503") ||
		strings.Contains(errStr, "504") ||
		strings.Contains(errStr, "INTERNAL") ||
		strings.Contains(errStr, "UNAVAILABLE") ||
		strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "connection")
}
