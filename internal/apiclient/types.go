// Package apiclient는 CLI 명령에서 사용하는 API 클라이언트 유틸리티를 제공합니다.
// BackendClient를 감싸는 편의 메서드와 출력 형식화 기능을 제공합니다.
package apiclient

import (
	"encoding/json"
	"strconv"
)

// APIError는 백엔드 오류 정보를 나타냅니다.
// 문자열/객체 두 형식 모두 역직렬화할 수 있도록 커스텀 파서를 제공합니다.
type APIError struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

// UnmarshalJSON은 "message" 문자열 또는 {code,message} 객체를 모두 허용합니다.
func (e *APIError) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}

	var msg string
	if err := json.Unmarshal(data, &msg); err == nil {
		e.Message = msg
		return nil
	}

	type alias APIError
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*e = APIError(decoded)
	return nil
}

// String은 사용자에게 보여줄 오류 메시지를 반환합니다.
func (e *APIError) String() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	return e.Code
}

// APIResponse는 백엔드 API의 표준 응답 형식과 일치합니다.
// Success=true이면 Data에 결과가 담기고, false이면 Error/Message에 오류 정보가 담깁니다.
type APIResponse struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data,omitempty"`
	Error   *APIError       `json:"error,omitempty"`
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

// FlexFloat64는 JSON에서 숫자 또는 문자열로 전달되는 float64 값을 역직렬화합니다.
// 백엔드가 "12.34" (string) 또는 12.34 (number) 모두 반환할 수 있을 때 사용합니다.
type FlexFloat64 float64

// UnmarshalJSON은 숫자 또는 문자열 형식의 float64를 모두 허용합니다.
func (f *FlexFloat64) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}

	// 숫자 리터럴 시도
	var num float64
	if err := json.Unmarshal(data, &num); err == nil {
		*f = FlexFloat64(num)
		return nil
	}

	// 문자열로 감싸진 숫자 시도
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	parsed, err := strconv.ParseFloat(str, 64)
	if err != nil {
		return err
	}
	*f = FlexFloat64(parsed)
	return nil
}

// Float64는 기본 float64 값을 반환합니다.
func (f FlexFloat64) Float64() float64 {
	return float64(f)
}

// MarshalJSON은 숫자로 직렬화합니다.
func (f FlexFloat64) MarshalJSON() ([]byte, error) {
	return json.Marshal(float64(f))
}

// KeyValue는 상세 출력에서 사용하는 키-값 쌍입니다.
type KeyValue struct {
	Key   string
	Value string
}
