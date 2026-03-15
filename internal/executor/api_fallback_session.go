// Package executor - HTTP API 기반 폴백 코딩 세션
package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/google/uuid"
)

// apiMessage는 API 대화 히스토리의 단일 메시지입니다.
type apiMessage struct {
	// Role은 메시지 역할입니다 (system | user | assistant).
	Role string `json:"role"`
	// Content는 메시지 내용입니다.
	Content string `json:"content"`
}

// APIFallbackSession은 HTTP API 기반 코딩 세션입니다.
// CLI 도구가 없는 BYOK 유저를 위한 폴백 구현체입니다.
// @MX:NOTE: [AUTO] CLI 없는 환경(BYOK)에서 API 직접 호출 방식으로 코딩 세션 제공
// @MX:SPEC: SPEC-CODING-RELAY-001 TASK-012
type APIFallbackSession struct {
	// cfg는 세션 설정입니다.
	cfg CodingSessionConfig
	// sessionID는 세션 식별자입니다.
	sessionID string
	// history는 대화 히스토리입니다.
	history []apiMessage
	// opened는 세션 열림 여부입니다.
	opened bool
	// model은 사용할 AI 모델입니다.
	model string
	// httpClient는 HTTP 클라이언트입니다.
	httpClient *http.Client
	// apiKey는 Anthropic API 키입니다.
	apiKey string
}

// NewAPIFallbackSession은 새로운 APIFallbackSession을 생성합니다.
func NewAPIFallbackSession(cfg CodingSessionConfig) *APIFallbackSession {
	return &APIFallbackSession{
		cfg:        cfg,
		httpClient: &http.Client{},
	}
}

// Open은 API 세션을 초기화합니다.
func (s *APIFallbackSession) Open(_ context.Context, req CodingSessionOpenRequest) error {
	s.sessionID = uuid.New().String()
	s.history = nil
	s.model = req.Model
	s.apiKey = os.Getenv("ANTHROPIC_API_KEY")
	s.opened = true

	// 시스템 프롬프트가 있으면 히스토리에 추가
	if req.SystemPrompt != "" {
		s.history = append(s.history, apiMessage{
			Role:    "system",
			Content: req.SystemPrompt,
		})
	}

	return nil
}

// Send는 HTTP API를 통해 메시지를 전송하고 응답을 반환합니다.
func (s *APIFallbackSession) Send(ctx context.Context, message string) (*CodingSessionResponse, error) {
	if !s.opened {
		return nil, fmt.Errorf("세션이 열려있지 않습니다: Open()을 먼저 호출하세요")
	}

	// 사용자 메시지 히스토리 추가
	s.history = append(s.history, apiMessage{
		Role:    "user",
		Content: message,
	})

	// Anthropic Messages API 호출
	content, err := s.callAnthropicAPI(ctx)
	if err != nil {
		return nil, fmt.Errorf("API 호출 실패: %w", err)
	}

	// 어시스턴트 응답 히스토리 추가
	s.history = append(s.history, apiMessage{
		Role:    "assistant",
		Content: content,
	})

	return &CodingSessionResponse{
		Content:   content,
		SessionID: s.sessionID,
		TurnCount: len(s.history),
	}, nil
}

// SessionID는 세션 ID를 반환합니다.
func (s *APIFallbackSession) SessionID() string {
	return s.sessionID
}

// Close는 세션을 종료합니다.
func (s *APIFallbackSession) Close(_ context.Context) error {
	s.opened = false
	s.history = nil
	return nil
}

// callAnthropicAPI는 Anthropic Messages API를 호출합니다.
// @MX:WARN: [AUTO] API 키는 환경변수(ANTHROPIC_API_KEY)에서만 로드
// @MX:REASON: 하드코딩된 시크릿 방지 — 절대로 코드에 API 키를 포함하지 않음
func (s *APIFallbackSession) callAnthropicAPI(ctx context.Context) (string, error) {
	model := s.model
	if model == "" {
		model = "claude-sonnet-4-5"
	}

	// 시스템 프롬프트와 메시지 분리
	var systemPrompt string
	var messages []apiMessage
	for _, msg := range s.history {
		if msg.Role == "system" {
			systemPrompt = msg.Content
		} else {
			messages = append(messages, msg)
		}
	}

	// 요청 페이로드 구성
	payload := map[string]any{
		"model":      model,
		"max_tokens": 8192,
		"messages":   messages,
	}
	if systemPrompt != "" {
		payload["system"] = systemPrompt
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("요청 직렬화 실패: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("HTTP 요청 생성 실패: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")
	if s.apiKey != "" {
		req.Header.Set("x-api-key", s.apiKey)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP 요청 실패: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("응답 읽기 실패: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API 에러 (status=%d): %s", resp.StatusCode, string(respBody))
	}

	// 응답 파싱
	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("응답 파싱 실패: %w", err)
	}

	var text string
	for _, c := range result.Content {
		if c.Type == "text" {
			text += c.Text
		}
	}

	return text, nil
}
