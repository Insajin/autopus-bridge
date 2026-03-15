// Package executor - APIFallbackSession 테스트
package executor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAPIFallbackSessionInterfaceCompliance는 APIFallbackSession이 CodingSession 인터페이스를 구현하는지 검증합니다.
func TestAPIFallbackSessionInterfaceCompliance(t *testing.T) {
	t.Parallel()

	var _ CodingSession = (*APIFallbackSession)(nil)
}

// TestAPIFallbackSessionNew는 NewAPIFallbackSession 생성자를 검증합니다.
func TestAPIFallbackSessionNew(t *testing.T) {
	t.Parallel()

	cfg := CodingSessionConfig{MaxBudgetUSD: 5.0}
	session := NewAPIFallbackSession(cfg)
	require.NotNil(t, session)
	assert.Empty(t, session.SessionID())
}

// TestAPIFallbackSessionSendNotOpened는 Open 없이 Send 호출 시 에러를 검증합니다.
func TestAPIFallbackSessionSendNotOpened(t *testing.T) {
	t.Parallel()

	session := NewAPIFallbackSession(CodingSessionConfig{})
	_, err := session.Send(context.Background(), "테스트 메시지")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "세션이 열려있지 않습니다")
}

// TestAPIFallbackSessionCloseNotOpened는 Open 없이 Close 호출 시 에러가 없는지 검증합니다.
func TestAPIFallbackSessionCloseNotOpened(t *testing.T) {
	t.Parallel()

	session := NewAPIFallbackSession(CodingSessionConfig{})
	err := session.Close(context.Background())
	assert.NoError(t, err)
}

// TestAPIFallbackSessionOpenInitializesHistory는 Open 후 히스토리 초기화를 검증합니다.
func TestAPIFallbackSessionOpenInitializesHistory(t *testing.T) {
	t.Parallel()

	session := NewAPIFallbackSession(CodingSessionConfig{})
	// 시스템 프롬프트 없이 Open
	err := session.Open(context.Background(), CodingSessionOpenRequest{
		WorkDir: "/tmp",
	})
	require.NoError(t, err)
	assert.True(t, session.opened)
	assert.NotEmpty(t, session.sessionID)
	assert.Empty(t, session.history) // 시스템 프롬프트 없으면 히스토리 비어있음
}

// TestAPIFallbackSessionOpenWithSystemPrompt는 시스템 프롬프트 포함 Open을 검증합니다.
func TestAPIFallbackSessionOpenWithSystemPrompt(t *testing.T) {
	t.Parallel()

	session := NewAPIFallbackSession(CodingSessionConfig{})
	err := session.Open(context.Background(), CodingSessionOpenRequest{
		WorkDir:      "/tmp",
		SystemPrompt: "당신은 코딩 어시스턴트입니다.",
	})
	require.NoError(t, err)
	// 시스템 프롬프트가 있으면 히스토리에 추가됨
	assert.Len(t, session.history, 1)
	assert.Equal(t, "system", session.history[0].Role)
}
