// Package provider는 AI 프로바이더 통합 레이어를 제공합니다.
package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Codex 지원 모델 목록
var codexSupportedModels = []string{
	// GPT-5 Codex
	"gpt-5-codex",
	// O4 Mini
	"o4-mini",
}

// openAIChatRequest는 OpenAI Chat Completions API 요청 구조입니다.
type openAIChatRequest struct {
	Model     string              `json:"model"`
	Messages  []openAIChatMessage `json:"messages"`
	MaxTokens int                 `json:"max_tokens,omitempty"`
}

// openAIChatMessage는 OpenAI 메시지 구조입니다.
type openAIChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// openAIChatResponse는 OpenAI Chat Completions API 응답 구조입니다.
type openAIChatResponse struct {
	ID      string               `json:"id"`
	Object  string               `json:"object"`
	Created int64                `json:"created"`
	Model   string               `json:"model"`
	Choices []openAIChatChoice   `json:"choices"`
	Usage   openAIUsage          `json:"usage"`
	Error   *openAIErrorResponse `json:"error,omitempty"`
}

// openAIChatChoice는 OpenAI 응답 선택지입니다.
type openAIChatChoice struct {
	Index        int               `json:"index"`
	Message      openAIChatMessage `json:"message"`
	FinishReason string            `json:"finish_reason"`
}

// openAIUsage는 OpenAI 토큰 사용량입니다.
type openAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// openAIErrorResponse는 OpenAI API 에러 응답 구조입니다.
type openAIErrorResponse struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

// CodexProvider는 OpenAI Codex API 프로바이더입니다.
type CodexProvider struct {
	httpClient *http.Client
	config     ProviderConfig
	baseURL    string
}

// CodexProviderOption은 CodexProvider 설정 옵션입니다.
type CodexProviderOption func(*CodexProvider)

// WithCodexAPIKey는 API 키를 설정합니다.
func WithCodexAPIKey(apiKey string) CodexProviderOption {
	return func(p *CodexProvider) {
		p.config.APIKey = apiKey
	}
}

// WithCodexDefaultModel은 기본 모델을 설정합니다.
func WithCodexDefaultModel(model string) CodexProviderOption {
	return func(p *CodexProvider) {
		p.config.DefaultModel = model
	}
}

// WithCodexMaxRetries는 최대 재시도 횟수를 설정합니다.
func WithCodexMaxRetries(retries int) CodexProviderOption {
	return func(p *CodexProvider) {
		p.config.MaxRetries = retries
	}
}

// NewCodexProvider는 새로운 CodexProvider를 생성합니다.
// API 키는 환경변수 OPENAI_API_KEY에서 가져옵니다 (REQ-N-01).
func NewCodexProvider(opts ...CodexProviderOption) (*CodexProvider, error) {
	p := &CodexProvider{
		httpClient: &http.Client{
			Timeout: 5 * time.Minute,
		},
		config: ProviderConfig{
			DefaultModel: "gpt-5-codex",
			MaxRetries:   3,
			RetryDelayMs: 1000,
		},
		baseURL: "https://api.openai.com/v1",
	}

	// 옵션 적용
	for _, opt := range opts {
		opt(p)
	}

	// API 키가 설정되지 않은 경우 환경변수에서 가져옴
	if p.config.APIKey == "" {
		p.config.APIKey = os.Getenv("OPENAI_API_KEY")
	}

	// API 키 검증
	if p.config.APIKey == "" {
		return nil, fmt.Errorf("%w: OPENAI_API_KEY 환경변수를 설정하세요", ErrNoAPIKey)
	}

	return p, nil
}

// Name은 프로바이더 식별자를 반환합니다.
func (p *CodexProvider) Name() string {
	return "codex"
}

// ValidateConfig는 프로바이더 설정의 유효성을 검사합니다.
func (p *CodexProvider) ValidateConfig() error {
	if p.config.APIKey == "" {
		return fmt.Errorf("%w: OPENAI_API_KEY", ErrNoAPIKey)
	}
	return nil
}

// Supports는 주어진 모델명을 지원하는지 확인합니다.
func (p *CodexProvider) Supports(model string) bool {
	// gpt- 또는 o4- 접두사로 시작하는지 확인
	if !strings.HasPrefix(model, "gpt-") && !strings.HasPrefix(model, "o4-") {
		return false
	}

	// 지원 모델 목록에서 확인
	for _, supported := range codexSupportedModels {
		if model == supported {
			return true
		}
	}

	// gpt-5-*, o4-* 패턴 매칭
	if strings.HasPrefix(model, "gpt-5-") ||
		strings.HasPrefix(model, "o4-") {
		return true
	}

	return false
}

// Execute는 프롬프트를 실행하고 결과를 반환합니다.
func (p *CodexProvider) Execute(ctx context.Context, req ExecuteRequest) (*ExecuteResponse, error) {
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

	// MaxTokens 기본값 설정
	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4096
	}

	// 메시지 빌드
	messages := []openAIChatMessage{}

	// 시스템 프롬프트 설정
	if req.SystemPrompt != "" {
		messages = append(messages, openAIChatMessage{
			Role:    "system",
			Content: req.SystemPrompt,
		})
	}

	// 사용자 프롬프트 추가
	messages = append(messages, openAIChatMessage{
		Role:    "user",
		Content: req.Prompt,
	})

	// 요청 파라미터 구성
	chatReq := openAIChatRequest{
		Model:     model,
		Messages:  messages,
		MaxTokens: maxTokens,
	}

	// 재시도 로직과 함께 API 호출
	var chatResp *openAIChatResponse
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

		chatResp, lastErr = p.doRequest(ctx, chatReq)
		if lastErr == nil {
			break
		}

		// 레이트 리밋 에러 확인
		if isCodexRateLimitError(lastErr) {
			lastErr = fmt.Errorf("%w: %v", ErrRateLimited, lastErr)
			continue
		}

		// 컨텍스트 취소 확인
		if errors.Is(lastErr, context.Canceled) || errors.Is(lastErr, context.DeadlineExceeded) {
			return nil, fmt.Errorf("%w: %v", ErrContextCanceled, lastErr)
		}

		// 재시도 불가능한 에러는 즉시 반환
		if !isCodexRetryableError(lastErr) {
			break
		}
	}

	if lastErr != nil {
		return nil, fmt.Errorf("codex API 호출 실패: %w", lastErr)
	}

	// 응답 텍스트 추출
	var outputText string
	if len(chatResp.Choices) > 0 {
		outputText = chatResp.Choices[0].Message.Content
	}

	// 토큰 사용량
	tokenUsage := TokenUsage{
		InputTokens:  chatResp.Usage.PromptTokens,
		OutputTokens: chatResp.Usage.CompletionTokens,
		TotalTokens:  chatResp.Usage.TotalTokens,
	}

	// 종료 사유 매핑
	stopReason := mapCodexFinishReason(chatResp)

	// 실행 시간 계산
	durationMs := time.Since(startTime).Milliseconds()

	return &ExecuteResponse{
		Output:     outputText,
		TokenUsage: tokenUsage,
		DurationMs: durationMs,
		Model:      chatResp.Model,
		StopReason: stopReason,
	}, nil
}

// doRequest는 OpenAI Chat Completions API에 HTTP 요청을 전송합니다.
func (p *CodexProvider) doRequest(ctx context.Context, chatReq openAIChatRequest) (*openAIChatResponse, error) {
	// 요청 본문 직렬화
	body, err := json.Marshal(chatReq)
	if err != nil {
		return nil, fmt.Errorf("요청 직렬화 실패: %w", err)
	}

	// HTTP 요청 생성
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("HTTP 요청 생성 실패: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.config.APIKey)

	// HTTP 요청 전송
	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("HTTP 요청 실패: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// 응답 본문 읽기
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("응답 읽기 실패: %w", err)
	}

	// HTTP 상태 코드 확인
	if resp.StatusCode != http.StatusOK {
		// 에러 응답 파싱 시도
		var errResp openAIChatResponse
		if json.Unmarshal(respBody, &errResp) == nil && errResp.Error != nil {
			return nil, fmt.Errorf("OpenAI API 에러 (HTTP %d): [%s] %s",
				resp.StatusCode, errResp.Error.Type, errResp.Error.Message)
		}
		return nil, fmt.Errorf("OpenAI API 에러 (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	// 응답 파싱
	var chatResp openAIChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, fmt.Errorf("응답 파싱 실패: %w", err)
	}

	return &chatResp, nil
}

// mapCodexFinishReason은 OpenAI의 finish_reason을 내부 stop reason으로 매핑합니다.
func mapCodexFinishReason(resp *openAIChatResponse) string {
	if resp == nil || len(resp.Choices) == 0 {
		return ""
	}

	switch resp.Choices[0].FinishReason {
	case "stop":
		return "end_turn"
	case "length":
		return "max_tokens"
	case "content_filter":
		return "content_filter"
	case "tool_calls", "function_call":
		return "tool_use"
	default:
		return resp.Choices[0].FinishReason
	}
}

// isCodexRateLimitError는 레이트 리밋 에러인지 확인합니다.
func isCodexRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "rate_limit") ||
		strings.Contains(errStr, "429") ||
		strings.Contains(errStr, "too many requests") ||
		strings.Contains(errStr, "Rate limit")
}

// isCodexRetryableError는 재시도 가능한 에러인지 확인합니다.
func isCodexRetryableError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	// 서버 에러(5xx) 또는 타임아웃은 재시도 가능
	return strings.Contains(errStr, "500") ||
		strings.Contains(errStr, "502") ||
		strings.Contains(errStr, "503") ||
		strings.Contains(errStr, "504") ||
		strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "connection") ||
		strings.Contains(errStr, "server_error")
}
