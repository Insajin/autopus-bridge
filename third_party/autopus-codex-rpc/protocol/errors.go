package protocol

import (
	"encoding/json"
	"fmt"
)

// JSONRPCError는 JSON-RPC 2.0 에러 객체이다.
// Go error 인터페이스를 구현하므로 직접 에러로 반환할 수 있다.
type JSONRPCError struct {
	// Code는 JSON-RPC 에러 코드이다.
	Code int `json:"code"`
	// Message는 에러 메시지이다.
	Message string `json:"message"`
	// Data는 추가 에러 데이터이다.
	Data json.RawMessage `json:"data,omitempty"`
}

// Error는 JSONRPCError를 Go error 인터페이스로 구현한다.
func (e *JSONRPCError) Error() string {
	return fmt.Sprintf("JSON-RPC 에러 [%d]: %s", e.Code, e.Message)
}

// --- JSON-RPC 2.0 표준 에러 코드 (정수 상수) ---

const (
	// ErrCodeParseError는 JSON 파싱 에러이다.
	ErrCodeParseError = -32700
	// ErrCodeInvalidRequest는 잘못된 요청이다.
	ErrCodeInvalidRequest = -32600
	// ErrCodeMethodNotFound는 메서드를 찾을 수 없다.
	ErrCodeMethodNotFound = -32601
	// ErrCodeInvalidParams는 잘못된 파라미터이다.
	ErrCodeInvalidParams = -32602
	// ErrCodeInternalError는 내부 에러이다.
	ErrCodeInternalError = -32603
	// ErrCodeContextWindowExceeded는 컨텍스트 윈도우 초과이다.
	ErrCodeContextWindowExceeded = -32001
	// ErrCodeUsageLimitExceeded는 사용량 제한 초과이다.
	ErrCodeUsageLimitExceeded = -32002
	// ErrCodeUnauthorized는 인증 실패이다.
	ErrCodeUnauthorized = -32003
	// ErrCodeConnectionFailed는 연결 실패이다.
	ErrCodeConnectionFailed = -32004
)

// --- Codex App Server 에러 코드 문자열 상수 ---

const (
	// CodexErrContextWindowExceeded는 컨텍스트 윈도우 초과 에러 코드이다.
	CodexErrContextWindowExceeded = "ContextWindowExceeded"
	// CodexErrUsageLimitExceeded는 사용량 제한 초과 에러 코드이다.
	CodexErrUsageLimitExceeded = "UsageLimitExceeded"
	// CodexErrUnauthorized는 인증 실패 에러 코드이다.
	CodexErrUnauthorized = "Unauthorized"
	// CodexErrHttpConnectionFailed는 HTTP 연결 실패 에러 코드이다.
	CodexErrHttpConnectionFailed = "HttpConnectionFailed"
	// CodexErrBadRequest는 잘못된 요청 에러 코드이다.
	CodexErrBadRequest = "BadRequest"
	// CodexErrSandboxError는 샌드박스 에러 코드이다.
	CodexErrSandboxError = "SandboxError"
)

// MapJSONRPCError는 JSON-RPC 에러 코드를 Go error로 변환한다.
// 알 수 없는 에러 코드는 JSONRPCError 자체를 반환한다.
func MapJSONRPCError(rpcErr *JSONRPCError) error {
	if rpcErr == nil {
		return nil
	}
	switch rpcErr.Code {
	case ErrCodeContextWindowExceeded:
		return fmt.Errorf("컨텍스트 윈도우 초과: %s", rpcErr.Message)
	case ErrCodeUsageLimitExceeded:
		return fmt.Errorf("사용량 제한 초과: %s", rpcErr.Message)
	case ErrCodeUnauthorized:
		return fmt.Errorf("인증 실패: %s", rpcErr.Message)
	case ErrCodeConnectionFailed:
		return fmt.Errorf("연결 실패: %s", rpcErr.Message)
	default:
		return rpcErr
	}
}
