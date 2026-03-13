package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"testing"
	"time"

	"github.com/insajin/autopus-agent-protocol"
)

type stubTaskExecutor struct {
	result ws.TaskResultPayload
	err    error
}

func (s *stubTaskExecutor) Execute(ctx context.Context, task ws.TaskRequestPayload) (ws.TaskResultPayload, error) {
	return s.result, s.err
}

func (s *stubTaskExecutor) ExecuteAgentResponse(ctx context.Context, req ws.AgentResponseRequestPayload) (ws.AgentResponseCompletePayload, error) {
	return ws.AgentResponseCompletePayload{}, nil
}

type stubTaskMessageSender struct {
	progress []ws.TaskProgressPayload
	results  []ws.TaskResultPayload
	errors   []ws.TaskErrorPayload
}

func (s *stubTaskMessageSender) SendTaskProgress(payload ws.TaskProgressPayload) error {
	s.progress = append(s.progress, payload)
	return nil
}

func (s *stubTaskMessageSender) SendTaskResult(payload ws.TaskResultPayload) error {
	s.results = append(s.results, payload)
	return nil
}

func (s *stubTaskMessageSender) SendTaskError(payload ws.TaskErrorPayload) error {
	s.errors = append(s.errors, payload)
	return nil
}

// ---------------------------------------------------------------------------
// Tests: Computer Use Handler Routing (SPEC-COMPUTER-USE-001)
// ---------------------------------------------------------------------------

// TestHandleComputerSessionStart_InvalidPayload verifies that invalid JSON
// in a computer_session_start message does not crash the router.
func TestHandleComputerSessionStart_InvalidPayload(t *testing.T) {
	client := NewClient("ws://localhost:9999/ws", "test-token", "1.0.0")
	router := NewRouter(client, WithErrorHandler(func(err error) {
		// Error expected for invalid payloads; ensure no panic.
	}))

	tests := []struct {
		name    string
		payload json.RawMessage
	}{
		{
			name:    "malformed JSON",
			payload: json.RawMessage(`{invalid json`),
		},
		{
			name:    "array instead of object",
			payload: json.RawMessage(`[1,2,3]`),
		},
		{
			name:    "number instead of object",
			payload: json.RawMessage(`12345`),
		},
		{
			name:    "string instead of object",
			payload: json.RawMessage(`"not an object"`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := ws.AgentMessage{
				Type:      ws.AgentMsgComputerSessionStart,
				ID:        "msg-cs-invalid-001",
				Timestamp: time.Now(),
				Payload:   tt.payload,
			}

			// Must not panic regardless of client connection state.
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("handleComputerSessionStart panicked on %s: %v", tt.name, r)
					}
				}()
				// Error is expected (either parse error or send failure).
				_ = router.HandleMessage(context.Background(), msg)
			}()
		})
	}
}

// TestHandleComputerAction_InvalidPayload verifies that invalid JSON
// in a computer_action message does not crash the router.
func TestHandleComputerAction_InvalidPayload(t *testing.T) {
	client := NewClient("ws://localhost:9999/ws", "test-token", "1.0.0")
	router := NewRouter(client, WithErrorHandler(func(err error) {}))

	tests := []struct {
		name    string
		payload json.RawMessage
	}{
		{
			name:    "malformed JSON",
			payload: json.RawMessage(`{broken`),
		},
		{
			name:    "null payload",
			payload: json.RawMessage(`null`),
		},
		{
			name:    "empty string",
			payload: json.RawMessage(`""`),
		},
		{
			name:    "nested invalid",
			payload: json.RawMessage(`{"action": {"nested": "wrong type"}}`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := ws.AgentMessage{
				Type:      ws.AgentMsgComputerAction,
				ID:        "msg-ca-invalid-001",
				Timestamp: time.Now(),
				Payload:   tt.payload,
			}

			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("handleComputerAction panicked on %s: %v", tt.name, r)
					}
				}()
				_ = router.HandleMessage(context.Background(), msg)
			}()
		})
	}
}

// TestHandleComputerSessionEnd_InvalidPayload verifies that invalid JSON
// in a computer_session_end message does not crash the router.
func TestHandleComputerSessionEnd_InvalidPayload(t *testing.T) {
	client := NewClient("ws://localhost:9999/ws", "test-token", "1.0.0")
	router := NewRouter(client, WithErrorHandler(func(err error) {}))

	tests := []struct {
		name    string
		payload json.RawMessage
	}{
		{
			name:    "malformed JSON",
			payload: json.RawMessage(`not json at all`),
		},
		{
			name:    "boolean instead of object",
			payload: json.RawMessage(`true`),
		},
		{
			name:    "empty array",
			payload: json.RawMessage(`[]`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := ws.AgentMessage{
				Type:      ws.AgentMsgComputerSessionEnd,
				ID:        "msg-ce-invalid-001",
				Timestamp: time.Now(),
				Payload:   tt.payload,
			}

			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("handleComputerSessionEnd panicked on %s: %v", tt.name, r)
					}
				}()
				_ = router.HandleMessage(context.Background(), msg)
			}()
		})
	}
}

// TestHandleComputerAction_ValidPayload verifies that a well-formed
// computer_action payload is parsed without panic. The actual action
// execution may fail (no handler, no connection) but parsing should succeed.
func TestHandleComputerAction_ValidPayload(t *testing.T) {
	client := NewClient("ws://localhost:9999/ws", "test-token", "1.0.0")
	router := NewRouter(client, WithErrorHandler(func(err error) {}))

	payload, err := json.Marshal(ws.ComputerActionPayload{
		ExecutionID: "exec-cu-001",
		SessionID:   "session-001",
		Action:      "screenshot",
		Params:      map[string]interface{}{"format": "png"},
	})
	if err != nil {
		t.Fatalf("failed to marshal ComputerActionPayload: %v", err)
	}

	msg := ws.AgentMessage{
		Type:      ws.AgentMsgComputerAction,
		ID:        "msg-ca-valid-001",
		Timestamp: time.Now(),
		Payload:   payload,
	}

	// Must not panic. Error is expected because computerUseHandler is nil
	// and client is not connected, but the JSON parsing itself should work.
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("handleComputerAction panicked on valid payload: %v", r)
			}
		}()
		_ = router.HandleMessage(context.Background(), msg)
	}()
}

// TestHandleComputerSessionStart_ValidPayload verifies parsing of a well-formed
// computer_session_start payload without panic.
func TestHandleComputerSessionStart_ValidPayload(t *testing.T) {
	client := NewClient("ws://localhost:9999/ws", "test-token", "1.0.0")
	router := NewRouter(client, WithErrorHandler(func(err error) {}))

	payload, err := json.Marshal(ws.ComputerSessionPayload{
		ExecutionID: "exec-cu-002",
		SessionID:   "session-002",
		URL:         "https://example.com",
		ViewportW:   1280,
		ViewportH:   720,
		Headless:    true,
	})
	if err != nil {
		t.Fatalf("failed to marshal ComputerSessionPayload: %v", err)
	}

	msg := ws.AgentMessage{
		Type:      ws.AgentMsgComputerSessionStart,
		ID:        "msg-cs-valid-001",
		Timestamp: time.Now(),
		Payload:   payload,
	}

	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("handleComputerSessionStart panicked on valid payload: %v", r)
			}
		}()
		_ = router.HandleMessage(context.Background(), msg)
	}()
}

// TestHandleComputerSessionEnd_ValidPayload verifies parsing of a well-formed
// computer_session_end payload without panic.
func TestHandleComputerSessionEnd_ValidPayload(t *testing.T) {
	client := NewClient("ws://localhost:9999/ws", "test-token", "1.0.0")
	router := NewRouter(client, WithErrorHandler(func(err error) {}))

	payload, err := json.Marshal(ws.ComputerSessionPayload{
		ExecutionID: "exec-cu-003",
		SessionID:   "session-003",
	})
	if err != nil {
		t.Fatalf("failed to marshal ComputerSessionPayload: %v", err)
	}

	msg := ws.AgentMessage{
		Type:      ws.AgentMsgComputerSessionEnd,
		ID:        "msg-ce-valid-001",
		Timestamp: time.Now(),
		Payload:   payload,
	}

	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("handleComputerSessionEnd panicked on valid payload: %v", r)
			}
		}()
		err := router.HandleMessage(context.Background(), msg)
		// With no computerUseHandler set, session end with valid payload
		// should return nil (the handler checks nil and returns nil).
		if err != nil {
			t.Errorf("expected nil error for session end with nil handler, got: %v", err)
		}
	}()
}

// TestHandleComputerSessionEnd_NilHandler_ReturnsNil verifies that
// handleComputerSessionEnd with nil computerUseHandler returns nil (graceful no-op).
func TestHandleComputerSessionEnd_NilHandler_ReturnsNil(t *testing.T) {
	client := NewClient("ws://localhost:9999/ws", "test-token", "1.0.0")
	// Create router WITHOUT ComputerUseHandler option.
	router := NewRouter(client)

	payload, _ := json.Marshal(ws.ComputerSessionPayload{
		ExecutionID: "exec-cu-end",
		SessionID:   "session-end-001",
	})

	msg := ws.AgentMessage{
		Type:      ws.AgentMsgComputerSessionEnd,
		ID:        "msg-ce-nil-001",
		Timestamp: time.Now(),
		Payload:   payload,
	}

	err := router.HandleMessage(context.Background(), msg)
	if err != nil {
		t.Errorf("HandleMessage returned error %v, want nil for nil computerUseHandler", err)
	}
}

// TestComputerUseHandlersRegistered verifies that all three computer use
// message types are registered as handlers in the router.
func TestComputerUseHandlersRegistered(t *testing.T) {
	client := NewClient("ws://localhost:9999/ws", "test-token", "1.0.0")
	router := NewRouter(client)

	expectedTypes := []string{
		ws.AgentMsgComputerSessionStart,
		ws.AgentMsgComputerAction,
		ws.AgentMsgComputerSessionEnd,
	}

	router.handlersMu.RLock()
	defer router.handlersMu.RUnlock()

	for _, msgType := range expectedTypes {
		if _, exists := router.handlers[msgType]; !exists {
			t.Errorf("handler for %q not registered", msgType)
		}
	}
}

// TestWithComputerUseHandler_Option verifies the RouterOption correctly sets
// the computerUseHandler field.
func TestWithComputerUseHandler_Option(t *testing.T) {
	client := NewClient("ws://localhost:9999/ws", "test-token", "1.0.0")

	// Without option: handler should be nil.
	router1 := NewRouter(client)
	if router1.computerUseHandler != nil {
		t.Error("computerUseHandler should be nil when option is not provided")
	}

	// With nil option: handler should remain nil.
	router2 := NewRouter(client, WithComputerUseHandler(nil))
	if router2.computerUseHandler != nil {
		t.Error("computerUseHandler should be nil when passed nil")
	}
}

func TestHandleTaskRequest_UsesCustomTaskSender(t *testing.T) {
	client := NewClient("ws://localhost:9999/ws", "test-token", "1.0.0")
	taskSender := &stubTaskMessageSender{}
	router := NewRouter(
		client,
		WithTaskExecutor(&stubTaskExecutor{
			result: ws.TaskResultPayload{
				ExecutionID: "exec-1",
				Output:      "done",
			},
		}),
		WithTaskMessageSender(taskSender),
	)

	payload, err := json.Marshal(ws.TaskRequestPayload{
		ExecutionID: "exec-1",
		Prompt:      "hello",
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	msg := ws.AgentMessage{
		Type:      ws.AgentMsgTaskReq,
		ID:        "msg-task-1",
		Timestamp: time.Now(),
		Payload:   payload,
	}

	if err := router.HandleMessage(context.Background(), msg); err != nil {
		t.Fatalf("HandleMessage() error = %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for len(taskSender.results) == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}

	if len(taskSender.progress) == 0 {
		t.Fatal("expected progress message")
	}
	if len(taskSender.results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(taskSender.results))
	}
	if got := taskSender.results[0].ExecutionID; got != "exec-1" {
		t.Fatalf("result.ExecutionID = %q, want %q", got, "exec-1")
	}
}

// ---------------------------------------------------------------------------
// Tests: isRetryableError 에러 분류기
// ---------------------------------------------------------------------------

// mockRetryableError는 retryable 인터페이스를 구현하는 테스트용 에러입니다.
type mockRetryableError struct {
	msg       string
	retryable bool
}

func (e *mockRetryableError) Error() string     { return e.msg }
func (e *mockRetryableError) IsRetryable() bool { return e.retryable }

// mockNetError는 net.Error 인터페이스를 구현하는 테스트용 에러입니다.
type mockNetError struct {
	msg       string
	timeout   bool
	temporary bool
}

func (e *mockNetError) Error() string   { return e.msg }
func (e *mockNetError) Timeout() bool   { return e.timeout }
func (e *mockNetError) Temporary() bool { return e.temporary }

func TestIsRetryableError_Nil(t *testing.T) {
	if isRetryableError(nil) {
		t.Error("nil 에러는 재시도 불가여야 합니다")
	}
}

func TestIsRetryableError_RetryableInterface(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "retryable 인터페이스 - 재시도 가능",
			err:      &mockRetryableError{msg: "timeout", retryable: true},
			expected: true,
		},
		{
			name:     "retryable 인터페이스 - 재시도 불가",
			err:      &mockRetryableError{msg: "invalid", retryable: false},
			expected: false,
		},
		{
			name:     "래핑된 retryable 에러 - 재시도 가능",
			err:      fmt.Errorf("wrapped: %w", &mockRetryableError{msg: "rate limited", retryable: true}),
			expected: true,
		},
		{
			name:     "래핑된 retryable 에러 - 재시도 불가",
			err:      fmt.Errorf("wrapped: %w", &mockRetryableError{msg: "auth failed", retryable: false}),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isRetryableError(tt.err)
			if got != tt.expected {
				t.Errorf("isRetryableError(%v) = %v, want %v", tt.err, got, tt.expected)
			}
		})
	}
}

func TestIsRetryableError_ContextErrors(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "context.DeadlineExceeded - 재시도 가능",
			err:      context.DeadlineExceeded,
			expected: true,
		},
		{
			name:     "래핑된 context.DeadlineExceeded",
			err:      fmt.Errorf("task failed: %w", context.DeadlineExceeded),
			expected: true,
		},
		{
			name:     "context.Canceled - 재시도 불가",
			err:      context.Canceled,
			expected: false,
		},
		{
			name:     "래핑된 context.Canceled",
			err:      fmt.Errorf("task failed: %w", context.Canceled),
			expected: false,
		},
		{
			name:     "os.ErrDeadlineExceeded - 재시도 가능",
			err:      os.ErrDeadlineExceeded,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isRetryableError(tt.err)
			if got != tt.expected {
				t.Errorf("isRetryableError(%v) = %v, want %v", tt.err, got, tt.expected)
			}
		})
	}
}

func TestIsRetryableError_NetworkErrors(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "net.Error 타임아웃",
			err:      &mockNetError{msg: "i/o timeout", timeout: true},
			expected: true,
		},
		{
			name:     "net.Error 비타임아웃 (일반 네트워크 장애)",
			err:      &mockNetError{msg: "connection reset", timeout: false},
			expected: true,
		},
		{
			name:     "래핑된 net.Error",
			err:      fmt.Errorf("send failed: %w", &mockNetError{msg: "timeout", timeout: true}),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isRetryableError(tt.err)
			if got != tt.expected {
				t.Errorf("isRetryableError(%v) = %v, want %v", tt.err, got, tt.expected)
			}
		})
	}
}

func TestIsRetryableError_IOErrors(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "io.EOF - 재시도 가능",
			err:      io.EOF,
			expected: true,
		},
		{
			name:     "io.ErrUnexpectedEOF - 재시도 가능",
			err:      io.ErrUnexpectedEOF,
			expected: true,
		},
		{
			name:     "래핑된 io.EOF",
			err:      fmt.Errorf("read body: %w", io.EOF),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isRetryableError(tt.err)
			if got != tt.expected {
				t.Errorf("isRetryableError(%v) = %v, want %v", tt.err, got, tt.expected)
			}
		})
	}
}

func TestIsRetryableError_StringPatterns_Retryable(t *testing.T) {
	retryableMessages := []struct {
		name string
		msg  string
	}{
		{"서버 에러 500", "HTTP 500 Internal Server Error"},
		{"Bad Gateway 502", "upstream returned 502"},
		{"Service Unavailable 503", "503 service unavailable"},
		{"Gateway Timeout 504", "504 gateway timeout"},
		{"레이트 리밋 429", "429 too many requests"},
		{"타임아웃 문자열", "request timeout exceeded"},
		{"timed out", "operation timed out"},
		{"connection refused", "dial tcp: connection refused"},
		{"connection reset", "read: connection reset by peer"},
		{"broken pipe", "write: broken pipe"},
		{"temporary failure", "temporary DNS failure"},
		{"서버 unavailable", "service unavailable, try again later"},
		{"server_error", "server_error: internal failure"},
		{"rate limit", "rate limit exceeded"},
		{"rate_limit", "error: rate_limit"},
		{"레이트 리밋 한국어", "API 레이트 리밋 초과"},
		{"too many requests", "too many requests, slow down"},
		{"internal error", "internal server error occurred"},
		{"overloaded", "server overloaded, please retry"},
		{"eof in message", "unexpected eof while reading"},
	}

	for _, tt := range retryableMessages {
		t.Run(tt.name, func(t *testing.T) {
			err := errors.New(tt.msg)
			if !isRetryableError(err) {
				t.Errorf("isRetryableError(%q) = false, want true (재시도 가능해야 합니다)", tt.msg)
			}
		})
	}
}

func TestIsRetryableError_StringPatterns_NonRetryable(t *testing.T) {
	nonRetryableMessages := []struct {
		name string
		msg  string
	}{
		{"인증 실패 401", "HTTP 401 Unauthorized"},
		{"권한 없음 403", "403 Forbidden: access denied"},
		{"리소스 없음 404", "404 Not Found"},
		{"잘못된 요청 400", "400 Bad Request: missing field"},
		{"유효성 검사", "validation error: email is required"},
		{"invalid 입력", "invalid request payload"},
		{"API 키 미설정", "API 키가 설정되지 않았습니다"},
		{"API key 영문", "no api key configured"},
		{"프로바이더 미등록", "프로바이더를 찾을 수 없습니다"},
		{"provider not found", "provider not found for model"},
		{"unauthorized", "unauthorized: token expired"},
		{"forbidden", "forbidden: insufficient permissions"},
		{"not found", "resource not found"},
		{"bad request", "bad request: malformed JSON"},
		{"sandbox 위반", "sandbox policy violation"},
		{"permission denied", "permission denied: read-only"},
	}

	for _, tt := range nonRetryableMessages {
		t.Run(tt.name, func(t *testing.T) {
			err := errors.New(tt.msg)
			if isRetryableError(err) {
				t.Errorf("isRetryableError(%q) = true, want false (재시도 불가여야 합니다)", tt.msg)
			}
		})
	}
}

func TestIsRetryableError_DefaultFalse(t *testing.T) {
	// 어떤 패턴에도 매칭되지 않는 일반 에러는 재시도 불가
	unknownErrors := []string{
		"something went wrong",
		"unexpected result",
		"data processing failed",
		"calculation error",
	}

	for _, msg := range unknownErrors {
		t.Run(msg, func(t *testing.T) {
			err := errors.New(msg)
			if isRetryableError(err) {
				t.Errorf("isRetryableError(%q) = true, want false (알 수 없는 에러는 재시도 불가여야 합니다)", msg)
			}
		})
	}
}

func TestIsRetryableError_NonRetryablePriority(t *testing.T) {
	// 재시도 불가 패턴이 재시도 가능 패턴보다 우선해야 합니다.
	// 예: "401 unauthorized timeout" - 401이 포함되므로 재시도 불가
	mixedErrors := []struct {
		name     string
		msg      string
		expected bool
	}{
		{
			name:     "401과 timeout 혼합 - 재시도 불가 우선",
			msg:      "401 unauthorized request timeout",
			expected: false,
		},
		{
			name:     "400과 server error 혼합 - 재시도 불가 우선",
			msg:      "400 bad request from server",
			expected: false,
		},
		{
			name:     "validation과 503 혼합 - 재시도 불가 우선",
			msg:      "validation failed: 503 service",
			expected: false,
		},
	}

	for _, tt := range mixedErrors {
		t.Run(tt.name, func(t *testing.T) {
			err := errors.New(tt.msg)
			got := isRetryableError(err)
			if got != tt.expected {
				t.Errorf("isRetryableError(%q) = %v, want %v", tt.msg, got, tt.expected)
			}
		})
	}
}

func TestIsRetryableError_RealNetError(t *testing.T) {
	// 실제 net 패키지의 에러 타입 사용
	_, err := net.DialTimeout("tcp", "192.0.2.1:12345", 1*time.Millisecond)
	if err == nil {
		t.Skip("예상대로 연결 실패하지 않음, 테스트 스킵")
	}

	// 실제 네트워크 에러는 재시도 가능해야 합니다
	if !isRetryableError(err) {
		t.Errorf("실제 net.DialTimeout 에러(%v)는 재시도 가능해야 합니다", err)
	}
}
