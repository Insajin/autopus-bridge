// Package executor - CodexSession 테스트
package executor

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCodexSessionInterfaceCompliance는 CodexSession이 CodingSession 인터페이스를 구현하는지 검증합니다.
func TestCodexSessionInterfaceCompliance(t *testing.T) {
	t.Parallel()

	var _ CodingSession = (*CodexSession)(nil)
}

// TestNewCodexSession는 생성자를 검증합니다.
func TestNewCodexSession(t *testing.T) {
	t.Parallel()

	session := NewCodexSession()
	require.NotNil(t, session)
}

// TestCodexSessionSendWithoutOpen는 Open 없이 Send 시 에러를 반환하는지 검증합니다.
func TestCodexSessionSendWithoutOpen(t *testing.T) {
	t.Parallel()

	session := NewCodexSession()
	_, err := session.Send(context.Background(), "hello")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "세션이 열려 있지 않습니다")
}

// TestCodexSessionCloseWithoutOpen는 Open 없이 Close가 에러 없이 동작하는지 검증합니다.
func TestCodexSessionCloseWithoutOpen(t *testing.T) {
	t.Parallel()

	session := NewCodexSession()
	err := session.Close(context.Background())
	require.NoError(t, err)
}

// TestCodexSessionSessionIDEmpty는 Open 전 SessionID가 빈 문자열인지 검증합니다.
func TestCodexSessionSessionIDEmpty(t *testing.T) {
	t.Parallel()

	session := NewCodexSession()
	assert.Empty(t, session.SessionID())
}

// TestCodexSessionWithMockClient는 모의 RPC 클라이언트를 사용한 Send를 검증합니다.
func TestCodexSessionWithMockClient(t *testing.T) {
	t.Parallel()

	// mockCodexRPC는 CodexRPCClient 인터페이스를 구현합니다.
	mock := &mockCodexRPC{
		threadID:  "thread-abc",
		response:  "작업 완료되었습니다",
	}
	session := NewCodexSessionWithClient(mock)

	err := session.Open(context.Background(), CodingSessionOpenRequest{
		WorkDir: "/tmp",
		Model:   "codex-mini",
	})
	require.NoError(t, err)
	assert.Equal(t, "thread-abc", session.SessionID())

	resp, err := session.Send(context.Background(), "테스트 작업")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "작업 완료되었습니다", resp.Content)
	assert.Equal(t, "thread-abc", resp.SessionID)

	err = session.Close(context.Background())
	require.NoError(t, err)
}

// TestCodexSessionResumeSession는 ResumeSession이 있으면 thread/resume를 사용하는지 검증합니다.
func TestCodexSessionResumeSession(t *testing.T) {
	t.Parallel()

	mock := &mockCodexRPC{
		threadID: "existing-thread",
		response: "재개 응답",
	}
	session := NewCodexSessionWithClient(mock)

	err := session.Open(context.Background(), CodingSessionOpenRequest{
		ResumeSession: "existing-thread",
	})
	require.NoError(t, err)
	assert.True(t, mock.resumeCalled, "thread/resume가 호출되어야 합니다")
	assert.Equal(t, "existing-thread", session.SessionID())
}

// TestCodexSessionSendError는 RPC 클라이언트 에러가 전파되는지 검증합니다.
func TestCodexSessionSendError(t *testing.T) {
	t.Parallel()

	mock := &mockCodexRPC{
		threadID:  "thread-xyz",
		turnError: fmt.Errorf("RPC 오류"),
	}
	session := NewCodexSessionWithClient(mock)

	err := session.Open(context.Background(), CodingSessionOpenRequest{})
	require.NoError(t, err)

	_, err = session.Send(context.Background(), "메시지")
	require.Error(t, err)
}

// mockCodexRPC는 테스트용 CodexRPCClient 모의 구현입니다.
type mockCodexRPC struct {
	threadID     string
	response     string
	turnError    error
	resumeCalled bool
	closeCalled  bool
}

func (m *mockCodexRPC) ThreadStart(ctx context.Context, params codexThreadStartParams) (string, error) {
	return m.threadID, nil
}

func (m *mockCodexRPC) ThreadResume(ctx context.Context, threadID string) error {
	m.resumeCalled = true
	return nil
}

func (m *mockCodexRPC) TurnRun(ctx context.Context, threadID, message string) (string, error) {
	if m.turnError != nil {
		return "", m.turnError
	}
	return m.response, nil
}

func (m *mockCodexRPC) Close() error {
	m.closeCalled = true
	return nil
}
