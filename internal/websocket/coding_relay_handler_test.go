// Package websocket - CodingRelay 핸들러 테스트
package websocket

import (
	"context"
	"encoding/json"
	"testing"

	ws "github.com/insajin/autopus-agent-protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockCodingRelayRunner는 테스트용 CodingRelayRunner 구현체입니다.
type mockCodingRelayRunner struct {
	called bool
}

func (m *mockCodingRelayRunner) RunRelay(_ context.Context, _ ws.CodingRelayRequestPayload, _ func(string, []byte) error, _ <-chan ws.CodingRelayFeedbackPayload) {
	m.called = true
}

// newTestCodingRelayRunner는 테스트용 CodingRelayRunner를 생성합니다.
func newTestCodingRelayRunner() *mockCodingRelayRunner {
	return &mockCodingRelayRunner{}
}

// TestCodingRelayHandlerRegistered는 coding_relay_request 핸들러가 등록되는지 검증합니다.
func TestCodingRelayHandlerRegistered(t *testing.T) {
	t.Parallel()

	client := NewClient("ws://localhost:9999/ws", "test-token", "1.0.0")
	router := NewRouter(client)

	router.handlersMu.RLock()
	_, requestRegistered := router.handlers[ws.AgentMsgCodingRelayRequest]
	_, feedbackRegistered := router.handlers[ws.AgentMsgCodingRelayFeedback]
	router.handlersMu.RUnlock()

	assert.True(t, requestRegistered, "coding_relay_request 핸들러가 등록되지 않음")
	assert.True(t, feedbackRegistered, "coding_relay_feedback 핸들러가 등록되지 않음")
}

// TestCodingRelayRequestHandlerNoRunner는 CodingRelayRunner 없이 요청 처리 시 패닉이 발생하지 않는지 검증합니다.
func TestCodingRelayRequestHandlerNoRunner(t *testing.T) {
	t.Parallel()

	client := NewClient("ws://localhost:9999/ws", "test-token", "1.0.0")
	router := NewRouter(client, WithErrorHandler(func(err error) {}))
	// CodingSessionManager를 설정하지 않음

	payload, err := json.Marshal(ws.CodingRelayRequestPayload{
		RequestID:       "req-001",
		TaskDescription: "테스트 태스크",
		WorkspaceID:     "ws-001",
		WorkerID:        "worker-001",
	})
	require.NoError(t, err)

	msg := ws.AgentMessage{
		Type:    ws.AgentMsgCodingRelayRequest,
		Payload: payload,
	}

	// Manager 없으면 패닉 없이 처리 완료
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("handleCodingRelayRequest panicked: %v", r)
			}
		}()
		_ = router.HandleMessage(context.Background(), msg)
	}()
}

// TestCodingRelayFeedbackHandlerNoSession은 세션이 없을 때 피드백 처리를 검증합니다.
func TestCodingRelayFeedbackHandlerNoSession(t *testing.T) {
	t.Parallel()

	client := NewClient("ws://localhost:9999/ws", "test-token", "1.0.0")
	router := NewRouter(client, WithErrorHandler(func(err error) {}))

	payload, err := json.Marshal(ws.CodingRelayFeedbackPayload{
		RequestID: "req-001",
		Feedback:  "좋은 작업입니다",
		Approved:  false,
	})
	require.NoError(t, err)

	msg := ws.AgentMessage{
		Type:    ws.AgentMsgCodingRelayFeedback,
		Payload: payload,
	}

	// 세션이 없어도 패닉이 발생하지 않아야 함
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("handleCodingRelayFeedback panicked: %v", r)
			}
		}()
		_ = router.HandleMessage(context.Background(), msg)
	}()
}

// TestWithCodingRelayRunner는 WithCodingRelayRunner 옵션이 올바르게 설정되는지 검증합니다.
func TestWithCodingRelayRunner(t *testing.T) {
	t.Parallel()

	client := NewClient("ws://localhost:9999/ws", "test-token", "1.0.0")
	runner := newTestCodingRelayRunner()
	router := NewRouter(client, WithCodingRelayRunner(runner))

	assert.NotNil(t, router.codingRelayRunner)
	assert.Equal(t, runner, router.codingRelayRunner)
}

// TestCodingRelayRequestParseInvalidPayload는 잘못된 페이로드 파싱 에러를 검증합니다.
func TestCodingRelayRequestParseInvalidPayload(t *testing.T) {
	t.Parallel()

	client := NewClient("ws://localhost:9999/ws", "test-token", "1.0.0")
	router := NewRouter(client, WithErrorHandler(func(err error) {}))

	msg := ws.AgentMessage{
		Type:    ws.AgentMsgCodingRelayRequest,
		Payload: []byte(`invalid json`),
	}

	err := router.HandleMessage(context.Background(), msg)
	// 파싱 에러는 핸들러 에러로 반환
	require.Error(t, err)
}
