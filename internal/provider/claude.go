// Package provider는 AI 프로바이더 통합 레이어를 제공합니다.
package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/packages/param"
	ws "github.com/insajin/autopus-agent-protocol"
)

// Claude 지원 모델 목록
var claudeSupportedModels = []string{
	// Claude 3.5 Sonnet
	"claude-3-5-sonnet-20240620",
	"claude-3-5-sonnet-20241022",
	"claude-3-5-sonnet-latest",
	// Claude Sonnet 4
	"claude-sonnet-4-20250514",
	"claude-sonnet-4-latest",
	// Claude Opus 4
	"claude-opus-4-20250514",
	"claude-opus-4-latest",
}

// ClaudeProvider는 Anthropic Claude API 프로바이더입니다.
type ClaudeProvider struct {
	client anthropic.Client
	config ProviderConfig
}

// ClaudeProviderOption은 ClaudeProvider 설정 옵션입니다.
type ClaudeProviderOption func(*ClaudeProvider)

// WithClaudeAPIKey는 API 키를 설정합니다.
func WithClaudeAPIKey(apiKey string) ClaudeProviderOption {
	return func(p *ClaudeProvider) {
		p.config.APIKey = apiKey
	}
}

// WithClaudeDefaultModel은 기본 모델을 설정합니다.
func WithClaudeDefaultModel(model string) ClaudeProviderOption {
	return func(p *ClaudeProvider) {
		p.config.DefaultModel = model
	}
}

// WithClaudeMaxRetries는 최대 재시도 횟수를 설정합니다.
func WithClaudeMaxRetries(retries int) ClaudeProviderOption {
	return func(p *ClaudeProvider) {
		p.config.MaxRetries = retries
	}
}

// NewClaudeProvider는 새로운 ClaudeProvider를 생성합니다.
// API 키는 환경변수 ANTHROPIC_API_KEY에서 가져옵니다 (REQ-N-01).
func NewClaudeProvider(opts ...ClaudeProviderOption) (*ClaudeProvider, error) {
	p := &ClaudeProvider{
		config: ProviderConfig{
			DefaultModel: "claude-sonnet-4-20250514",
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
		p.config.APIKey = os.Getenv("ANTHROPIC_API_KEY")
	}

	// API 키 검증
	if p.config.APIKey == "" {
		return nil, fmt.Errorf("%w: ANTHROPIC_API_KEY 환경변수를 설정하세요", ErrNoAPIKey)
	}

	// Anthropic 클라이언트 생성
	p.client = anthropic.NewClient(
		option.WithAPIKey(p.config.APIKey),
	)

	return p, nil
}

// Name은 프로바이더 식별자를 반환합니다.
func (p *ClaudeProvider) Name() string {
	return "claude"
}

// ValidateConfig는 프로바이더 설정의 유효성을 검사합니다.
func (p *ClaudeProvider) ValidateConfig() error {
	if p.config.APIKey == "" {
		return fmt.Errorf("%w: ANTHROPIC_API_KEY", ErrNoAPIKey)
	}
	return nil
}

// Supports는 주어진 모델명을 지원하는지 확인합니다.
func (p *ClaudeProvider) Supports(model string) bool {
	// claude- 접두사로 시작하는지 확인
	if !strings.HasPrefix(model, "claude-") {
		return false
	}

	// 지원 모델 목록에서 확인
	for _, supported := range claudeSupportedModels {
		if model == supported {
			return true
		}
	}

	// claude-3-5-*, claude-sonnet-4-*, claude-opus-4-* 패턴 매칭
	if strings.HasPrefix(model, "claude-3-5-sonnet-") ||
		strings.HasPrefix(model, "claude-sonnet-4-") ||
		strings.HasPrefix(model, "claude-opus-4-") {
		return true
	}

	return false
}

// Capabilities는 프로바이더가 지원하는 기능을 반환합니다.
func (p *ClaudeProvider) Capabilities() ProviderCapabilities {
	return ProviderCapabilities{
		SupportsComputerUse: true,
	}
}

// Execute는 프롬프트를 실행하고 결과를 반환합니다.
func (p *ClaudeProvider) Execute(ctx context.Context, req ExecuteRequest) (*ExecuteResponse, error) {
	if req.ResponseMode == "tool_loop" {
		return p.executeToolLoop(ctx, req)
	}

	// computer_use 도구가 요청된 경우 Beta API 사용
	if containsTool(req.Tools, "computer_use") {
		return p.executeWithBeta(ctx, req)
	}

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
	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(req.Prompt)),
	}

	// 요청 파라미터 구성
	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: int64(maxTokens),
		Messages:  messages,
	}

	// 시스템 프롬프트 설정
	if req.SystemPrompt != "" {
		params.System = []anthropic.TextBlockParam{
			{Text: req.SystemPrompt},
		}
	}

	// 재시도 로직과 함께 API 호출
	var response *anthropic.Message
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

		response, lastErr = p.client.Messages.New(ctx, params)
		if lastErr == nil {
			break
		}

		// 레이트 리밋 에러 확인
		if isClaudeRateLimitError(lastErr) {
			lastErr = fmt.Errorf("%w: %v", ErrRateLimited, lastErr)
			continue
		}

		// 컨텍스트 취소 확인
		if errors.Is(lastErr, context.Canceled) || errors.Is(lastErr, context.DeadlineExceeded) {
			return nil, fmt.Errorf("%w: %v", ErrContextCanceled, lastErr)
		}

		// 재시도 불가능한 에러는 즉시 반환
		if !isClaudeRetryableError(lastErr) {
			break
		}
	}

	if lastErr != nil {
		return nil, fmt.Errorf("claude API 호출 실패: %w", lastErr)
	}

	// 응답 콘텐츠 추출 (텍스트 + 도구 호출)
	var outputText string
	var toolCalls []ToolCall
	for _, block := range response.Content {
		switch block.Type {
		case "text":
			outputText += block.Text
		case "tool_use":
			toolCalls = append(toolCalls, ToolCall{
				ID:    block.ID,
				Name:  block.Name,
				Input: json.RawMessage(block.Input),
			})
		}
	}

	// 토큰 사용량
	tokenUsage := TokenUsage{
		InputTokens:  int(response.Usage.InputTokens),
		OutputTokens: int(response.Usage.OutputTokens),
		TotalTokens:  int(response.Usage.InputTokens + response.Usage.OutputTokens),
	}

	// 캐시 토큰 정보가 있으면 추가
	if response.Usage.CacheReadInputTokens > 0 {
		tokenUsage.CacheRead = int(response.Usage.CacheReadInputTokens)
	}
	if response.Usage.CacheCreationInputTokens > 0 {
		tokenUsage.CacheCreation = int(response.Usage.CacheCreationInputTokens)
	}

	// 실행 시간 계산
	durationMs := time.Since(startTime).Milliseconds()

	return &ExecuteResponse{
		Output:     outputText,
		TokenUsage: tokenUsage,
		DurationMs: durationMs,
		Model:      string(response.Model),
		Provider:   p.Name(),
		StopReason: string(response.StopReason),
		ToolCalls:  toolCalls,
	}, nil
}

func (p *ClaudeProvider) executeToolLoop(ctx context.Context, req ExecuteRequest) (*ExecuteResponse, error) {
	startTime := time.Now()

	model := req.Model
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

	messages, err := buildClaudeToolLoopMessages(req.ToolLoopMessages)
	if err != nil {
		return nil, err
	}
	tools, err := buildClaudeTools(req.ToolDefinitions)
	if err != nil {
		return nil, err
	}

	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: int64(maxTokens),
		Messages:  messages,
		Tools:     tools,
	}
	if req.SystemPrompt != "" {
		params.System = []anthropic.TextBlockParam{{Text: req.SystemPrompt}}
	}

	response, err := p.client.Messages.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("claude API tool-loop 호출 실패: %w", err)
	}

	var outputText string
	var toolCalls []ToolCall
	for _, block := range response.Content {
		switch block.Type {
		case "text":
			outputText += block.Text
		case "tool_use":
			toolCalls = append(toolCalls, ToolCall{
				ID:    block.ID,
				Name:  block.Name,
				Input: json.RawMessage(block.Input),
			})
		}
	}

	tokenUsage := TokenUsage{
		InputTokens:  int(response.Usage.InputTokens),
		OutputTokens: int(response.Usage.OutputTokens),
		TotalTokens:  int(response.Usage.InputTokens + response.Usage.OutputTokens),
	}
	if response.Usage.CacheReadInputTokens > 0 {
		tokenUsage.CacheRead = int(response.Usage.CacheReadInputTokens)
	}
	if response.Usage.CacheCreationInputTokens > 0 {
		tokenUsage.CacheCreation = int(response.Usage.CacheCreationInputTokens)
	}

	return &ExecuteResponse{
		Output:     outputText,
		TokenUsage: tokenUsage,
		DurationMs: time.Since(startTime).Milliseconds(),
		Model:      string(response.Model),
		Provider:   p.Name(),
		StopReason: string(response.StopReason),
		ToolCalls:  toolCalls,
	}, nil
}

// executeWithBeta는 Beta API를 사용하여 computer_use 도구를 포함한 요청을 실행합니다.
func (p *ClaudeProvider) executeWithBeta(ctx context.Context, req ExecuteRequest) (*ExecuteResponse, error) {
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

	// Beta 메시지 빌드
	messages := []anthropic.BetaMessageParam{
		anthropic.NewBetaUserMessage(anthropic.NewBetaTextBlock(req.Prompt)),
	}

	// Beta 도구 구성
	var tools []anthropic.BetaToolUnionParam
	for _, toolName := range req.Tools {
		switch toolName {
		case "computer_use":
			// computer_use 도구: 기본 뷰포트 1280x720
			tools = append(tools, anthropic.BetaToolUnionParamOfComputerUseTool20250124(
				720,  // displayHeightPx
				1280, // displayWidthPx
			))
		}
	}

	// Beta 요청 파라미터 구성
	betaParams := anthropic.BetaMessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: int64(maxTokens),
		Messages:  messages,
		Tools:     tools,
		Betas: []anthropic.AnthropicBeta{
			anthropic.AnthropicBetaComputerUse2025_01_24,
		},
	}

	// 시스템 프롬프트 설정
	if req.SystemPrompt != "" {
		betaParams.System = []anthropic.BetaTextBlockParam{
			{Text: req.SystemPrompt},
		}
	}

	// 재시도 로직과 함께 Beta API 호출
	var response *anthropic.BetaMessage
	var lastErr error

	for attempt := 0; attempt <= p.config.MaxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("%w: %v", ErrContextCanceled, ctx.Err())
			case <-time.After(time.Duration(p.config.RetryDelayMs*attempt) * time.Millisecond):
			}
		}

		response, lastErr = p.client.Beta.Messages.New(ctx, betaParams)
		if lastErr == nil {
			break
		}

		if isClaudeRateLimitError(lastErr) {
			lastErr = fmt.Errorf("%w: %v", ErrRateLimited, lastErr)
			continue
		}

		if errors.Is(lastErr, context.Canceled) || errors.Is(lastErr, context.DeadlineExceeded) {
			return nil, fmt.Errorf("%w: %v", ErrContextCanceled, lastErr)
		}

		if !isClaudeRetryableError(lastErr) {
			break
		}
	}

	if lastErr != nil {
		return nil, fmt.Errorf("claude Beta API 호출 실패: %w", lastErr)
	}

	// Beta 응답 콘텐츠 추출 (텍스트 + 도구 호출)
	var outputText string
	var toolCalls []ToolCall
	for _, block := range response.Content {
		switch block.Type {
		case "text":
			outputText += block.Text
		case "tool_use":
			toolCalls = append(toolCalls, ToolCall{
				ID:    block.ID,
				Name:  block.Name,
				Input: json.RawMessage(block.Input),
			})
		}
	}

	// 토큰 사용량
	tokenUsage := TokenUsage{
		InputTokens:  int(response.Usage.InputTokens),
		OutputTokens: int(response.Usage.OutputTokens),
		TotalTokens:  int(response.Usage.InputTokens + response.Usage.OutputTokens),
	}

	if response.Usage.CacheReadInputTokens > 0 {
		tokenUsage.CacheRead = int(response.Usage.CacheReadInputTokens)
	}
	if response.Usage.CacheCreationInputTokens > 0 {
		tokenUsage.CacheCreation = int(response.Usage.CacheCreationInputTokens)
	}

	durationMs := time.Since(startTime).Milliseconds()

	return &ExecuteResponse{
		Output:     outputText,
		TokenUsage: tokenUsage,
		DurationMs: durationMs,
		Model:      string(response.Model),
		Provider:   p.Name(),
		StopReason: string(response.StopReason),
		ToolCalls:  toolCalls,
	}, nil
}

func buildClaudeToolLoopMessages(messages []ws.ToolLoopMessage) ([]anthropic.MessageParam, error) {
	result := make([]anthropic.MessageParam, 0, len(messages))
	for _, msg := range messages {
		blocks := make([]anthropic.ContentBlockParamUnion, 0, 1+len(msg.ToolCalls)+len(msg.ToolResults))
		if msg.Content != "" {
			blocks = append(blocks, anthropic.NewTextBlock(msg.Content))
		}
		for _, call := range msg.ToolCalls {
			var input any
			if len(call.Input) > 0 {
				if err := json.Unmarshal(call.Input, &input); err != nil {
					return nil, fmt.Errorf("tool %q input decode failed: %w", call.Name, err)
				}
			}
			blocks = append(blocks, anthropic.NewToolUseBlock(call.ID, input, call.Name))
		}
		for _, toolResult := range msg.ToolResults {
			blocks = append(blocks, anthropic.NewToolResultBlock(toolResult.ToolCallID, toolResult.Content, toolResult.IsError))
		}
		if len(blocks) == 0 {
			continue
		}
		if msg.Role == "assistant" {
			result = append(result, anthropic.NewAssistantMessage(blocks...))
		} else {
			result = append(result, anthropic.NewUserMessage(blocks...))
		}
	}
	return result, nil
}

func buildClaudeTools(defs []ws.ToolDefinition) ([]anthropic.ToolUnionParam, error) {
	if len(defs) == 0 {
		return nil, nil
	}
	tools := make([]anthropic.ToolUnionParam, 0, len(defs))
	for _, def := range defs {
		schema, err := buildClaudeToolInputSchema(def.InputSchema)
		if err != nil {
			return nil, fmt.Errorf("tool %q schema build failed: %w", def.Name, err)
		}
		tool := anthropic.ToolUnionParamOfTool(schema, def.Name)
		if def.Description != "" && tool.OfTool != nil {
			tool.OfTool.Description = param.NewOpt(def.Description)
		}
		tools = append(tools, tool)
	}
	return tools, nil
}

func buildClaudeToolInputSchema(raw json.RawMessage) (anthropic.ToolInputSchemaParam, error) {
	if len(raw) == 0 {
		return anthropic.ToolInputSchemaParam{}, nil
	}
	var schemaMap map[string]interface{}
	if err := json.Unmarshal(raw, &schemaMap); err != nil {
		return anthropic.ToolInputSchemaParam{}, fmt.Errorf("failed to unmarshal schema: %w", err)
	}

	result := anthropic.ToolInputSchemaParam{}
	if props, ok := schemaMap["properties"]; ok {
		result.Properties = props
	}
	if requiredRaw, ok := schemaMap["required"].([]interface{}); ok {
		required := make([]string, 0, len(requiredRaw))
		for _, entry := range requiredRaw {
			if s, ok := entry.(string); ok {
				required = append(required, s)
			}
		}
		result.Required = required
	}
	return result, nil
}

// containsTool은 도구 목록에서 특정 도구가 있는지 확인합니다.
func containsTool(tools []string, name string) bool {
	for _, t := range tools {
		if t == name {
			return true
		}
	}
	return false
}

// isClaudeRateLimitError는 레이트 리밋 에러인지 확인합니다.
func isClaudeRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "rate_limit") ||
		strings.Contains(errStr, "429") ||
		strings.Contains(errStr, "too many requests")
}

// isClaudeRetryableError는 재시도 가능한 에러인지 확인합니다.
func isClaudeRetryableError(err error) bool {
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
		strings.Contains(errStr, "connection")
}
