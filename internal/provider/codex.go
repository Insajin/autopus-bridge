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

	ws "github.com/insajin/autopus-agent-protocol"
)

// openAIChatRequest는 OpenAI Chat Completions API 요청 구조입니다.
type openAIChatRequest struct {
	Model      string              `json:"model"`
	Messages   []openAIChatMessage `json:"messages"`
	MaxTokens  int                 `json:"max_tokens,omitempty"`
	Tools      []openAIChatTool    `json:"tools,omitempty"`
	ToolChoice string              `json:"tool_choice,omitempty"`
}

// openAIChatMessage는 OpenAI 메시지 구조입니다.
type openAIChatMessage struct {
	Role       string               `json:"role"`
	Content    any                  `json:"content,omitempty"`
	ToolCalls  []openAIChatToolCall `json:"tool_calls,omitempty"`
	ToolCallID string               `json:"tool_call_id,omitempty"`
	Name       string               `json:"name,omitempty"`
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

type openAIChatTool struct {
	Type     string                `json:"type"`
	Function openAIChatFunctionDef `json:"function"`
}

type openAIChatFunctionDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters"`
}

type openAIChatToolCall struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Function openAIChatFunctionCall `json:"function"`
}

type openAIChatFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
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
			DefaultModel: "gpt-5.4",
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
// OpenRouter 형식(openai/o3-mini)과 레거시 형식 모두 지원합니다.
// gpt-*, o4-*, o3- 접두사를 가진 모든 모델을 지원하여 새 버전 자동 반영.
func (p *CodexProvider) Supports(model string) bool {
	bare := StripProviderPrefix(model)
	return strings.HasPrefix(bare, "gpt-") || strings.HasPrefix(bare, "o4-") || strings.HasPrefix(bare, "o3-")
}

// Execute는 프롬프트를 실행하고 결과를 반환합니다.
func (p *CodexProvider) Execute(ctx context.Context, req ExecuteRequest) (*ExecuteResponse, error) {
	if req.ResponseMode == "tool_loop" {
		return p.executeToolLoop(ctx, req)
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
		outputText = messageContentString(chatResp.Choices[0].Message.Content)
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
		Provider:   p.Name(),
		StopReason: stopReason,
	}, nil
}

func (p *CodexProvider) executeToolLoop(ctx context.Context, req ExecuteRequest) (*ExecuteResponse, error) {
	startTime := time.Now()

	model := StripProviderPrefix(req.Model)
	if model == "" {
		model = p.config.DefaultModel
	}
	if !p.Supports(model) {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedModel, model)
	}

	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4096
	}

	chatReq := openAIChatRequest{
		Model:      model,
		Messages:   buildOpenAIToolLoopMessages(req),
		MaxTokens:  maxTokens,
		Tools:      buildOpenAITools(req.ToolDefinitions),
		ToolChoice: "auto",
	}

	chatResp, err := p.doRequest(ctx, chatReq)
	if err != nil {
		return nil, err
	}

	outputText := ""
	toolCalls := make([]ToolCall, 0)
	stopReason := ""
	if len(chatResp.Choices) > 0 {
		outputText = messageContentString(chatResp.Choices[0].Message.Content)
		stopReason = mapCodexFinishReason(chatResp)
		for _, call := range chatResp.Choices[0].Message.ToolCalls {
			toolCalls = append(toolCalls, ToolCall{
				ID:    call.ID,
				Name:  call.Function.Name,
				Input: json.RawMessage(call.Function.Arguments),
			})
		}
	}

	tokenUsage := TokenUsage{
		InputTokens:  chatResp.Usage.PromptTokens,
		OutputTokens: chatResp.Usage.CompletionTokens,
		TotalTokens:  chatResp.Usage.TotalTokens,
	}

	return &ExecuteResponse{
		Output:     outputText,
		TokenUsage: tokenUsage,
		DurationMs: time.Since(startTime).Milliseconds(),
		Model:      chatResp.Model,
		Provider:   p.Name(),
		StopReason: stopReason,
		ToolCalls:  toolCalls,
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

func buildOpenAIToolLoopMessages(req ExecuteRequest) []openAIChatMessage {
	messages := make([]openAIChatMessage, 0, len(req.ToolLoopMessages)+1)
	if req.SystemPrompt != "" {
		messages = append(messages, openAIChatMessage{
			Role:    "system",
			Content: req.SystemPrompt,
		})
	}

	for _, msg := range req.ToolLoopMessages {
		switch msg.Role {
		case "tool":
			for _, result := range msg.ToolResults {
				messages = append(messages, openAIChatMessage{
					Role:       "tool",
					ToolCallID: result.ToolCallID,
					Name:       result.ToolName,
					Content:    result.Content,
				})
			}
		default:
			toolCalls := make([]openAIChatToolCall, 0, len(msg.ToolCalls))
			for _, call := range msg.ToolCalls {
				toolCalls = append(toolCalls, openAIChatToolCall{
					ID:   call.ID,
					Type: "function",
					Function: openAIChatFunctionCall{
						Name:      call.Name,
						Arguments: string(call.Input),
					},
				})
			}
			messages = append(messages, openAIChatMessage{
				Role:      msg.Role,
				Content:   msg.Content,
				ToolCalls: toolCalls,
			})
		}
	}

	return messages
}

func buildOpenAITools(defs []ws.ToolDefinition) []openAIChatTool {
	if len(defs) == 0 {
		return nil
	}
	tools := make([]openAIChatTool, 0, len(defs))
	for _, def := range defs {
		tools = append(tools, openAIChatTool{
			Type: "function",
			Function: openAIChatFunctionDef{
				Name:        def.Name,
				Description: def.Description,
				Parameters:  def.InputSchema,
			},
		})
	}
	return tools
}

func messageContentString(content any) string {
	switch v := content.(type) {
	case string:
		return v
	case nil:
		return ""
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return ""
		}
		return string(data)
	}
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
