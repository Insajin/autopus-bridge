// Package apiclient는 CLI 명령에서 사용하는 API 클라이언트 유틸리티를 제공합니다.
// BackendClient를 감싸는 편의 메서드와 출력 형식화 기능을 제공합니다.
package apiclient

import "encoding/json"

// APIResponse는 백엔드 API의 표준 응답 형식과 일치합니다.
// Success=true이면 Data에 결과가 담기고, false이면 Error/Message에 오류 정보가 담깁니다.
type APIResponse struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data,omitempty"`
	Error   string          `json:"error,omitempty"`
	Message string          `json:"message,omitempty"`
	Meta    *PageMeta       `json:"meta,omitempty"`
}

// PageMeta는 페이지네이션 응답의 메타 정보입니다.
type PageMeta struct {
	Page       int `json:"page"`
	PageSize   int `json:"page_size"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

// AgentTypingPayload는 Unified SSE의 agent_typing 이벤트 페이로드입니다.
// 채널에서 에이전트가 응답을 생성하는 동안 발생합니다.
type AgentTypingPayload struct {
	ChannelID       string `json:"channel_id"`
	AgentID         string `json:"agent_id"`
	ThreadID        string `json:"thread_id,omitempty"`
	TextDelta       string `json:"text_delta"`
	AccumulatedText string `json:"accumulated_text"`
	IsComplete      bool   `json:"is_complete"`
	Timestamp       string `json:"timestamp"`
}

// SSEEvent는 파싱된 SSE 이벤트를 나타냅니다.
// Type은 이벤트 종류이고, Data는 이벤트별 페이로드입니다.
type SSEEvent struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// KeyValue는 상세 출력에서 사용하는 키-값 쌍입니다.
type KeyValue struct {
	Key   string
	Value string
}
