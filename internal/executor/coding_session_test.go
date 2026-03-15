// Package executor - CodingSession 인터페이스 테스트
package executor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockCodingSession은 테스트용 CodingSession 구현체입니다.
type mockCodingSession struct {
	sessionID string
	openErr   error
	sendResp  *CodingSessionResponse
	sendErr   error
	closeErr  error
}

func (m *mockCodingSession) Open(_ context.Context, _ CodingSessionOpenRequest) error {
	return m.openErr
}

func (m *mockCodingSession) Send(_ context.Context, _ string) (*CodingSessionResponse, error) {
	return m.sendResp, m.sendErr
}

func (m *mockCodingSession) SessionID() string {
	return m.sessionID
}

func (m *mockCodingSession) Close(_ context.Context) error {
	return m.closeErr
}

// TestCodingSessionInterfaceCompliance는 mockCodingSession이 CodingSession 인터페이스를 구현하는지 검증합니다.
func TestCodingSessionInterfaceCompliance(t *testing.T) {
	t.Parallel()

	// 인터페이스 타입 어서션 — 컴파일 시점 검증
	var _ CodingSession = (*mockCodingSession)(nil)
}

// TestCodingSessionOpenRequest는 CodingSessionOpenRequest 구조체를 검증합니다.
func TestCodingSessionOpenRequest(t *testing.T) {
	t.Parallel()

	req := CodingSessionOpenRequest{
		WorkDir:       "/tmp/project",
		Model:         "claude-sonnet-4-5",
		MaxBudgetUSD:  5.0,
		Tools:         []string{"bash", "edit"},
		AllowedTools:  []string{"bash"},
		ResumeSession: "session-123",
		SystemPrompt:  "You are a helpful coding assistant.",
	}

	assert.Equal(t, "/tmp/project", req.WorkDir)
	assert.Equal(t, "claude-sonnet-4-5", req.Model)
	assert.Equal(t, 5.0, req.MaxBudgetUSD)
	assert.Len(t, req.Tools, 2)
	assert.Len(t, req.AllowedTools, 1)
	assert.Equal(t, "session-123", req.ResumeSession)
}

// TestCodingSessionResponse는 CodingSessionResponse 구조체를 검증합니다.
func TestCodingSessionResponse(t *testing.T) {
	t.Parallel()

	resp := CodingSessionResponse{
		Content:      "작업 완료",
		DiffSummary:  "+10줄 추가, -2줄 삭제",
		TestOutput:   "PASS: 5 tests",
		FilesChanged: []string{"main.go", "handler.go"},
		SessionID:    "sess-abc",
		CostUSD:      0.05,
		TurnCount:    3,
	}

	assert.Equal(t, "작업 완료", resp.Content)
	assert.Equal(t, "+10줄 추가, -2줄 삭제", resp.DiffSummary)
	assert.Len(t, resp.FilesChanged, 2)
	assert.Equal(t, 0.05, resp.CostUSD)
	assert.Equal(t, 3, resp.TurnCount)
}

// TestCodingSessionConfig는 CodingSessionConfig 구조체를 검증합니다.
func TestCodingSessionConfig(t *testing.T) {
	t.Parallel()

	cfg := CodingSessionConfig{
		MaxConcurrent:      2,
		RelayMaxIterations: 10,
		SessionTimeoutMin:  30,
		MaxBudgetUSD:       20.0,
		PreferredProvider:  "claude",
	}

	assert.Equal(t, 2, cfg.MaxConcurrent)
	assert.Equal(t, 10, cfg.RelayMaxIterations)
	assert.Equal(t, 30, cfg.SessionTimeoutMin)
	assert.Equal(t, CodingProviderClaude, CodingProvider(cfg.PreferredProvider))
}

// TestCodingProviderConstants는 CodingProvider 상수를 검증합니다.
func TestCodingProviderConstants(t *testing.T) {
	t.Parallel()

	assert.Equal(t, CodingProvider("claude"), CodingProviderClaude)
	assert.Equal(t, CodingProvider("codex"), CodingProviderCodex)
	assert.Equal(t, CodingProvider("gemini"), CodingProviderGemini)
	assert.Equal(t, CodingProvider("opencode"), CodingProviderOpenCode)
	assert.Equal(t, CodingProvider("api"), CodingProviderAPI)
}

// TestMockCodingSessionOpen은 mockCodingSession의 Open 메서드를 검증합니다.
func TestMockCodingSessionOpen(t *testing.T) {
	t.Parallel()

	session := &mockCodingSession{sessionID: "test-session"}
	err := session.Open(context.Background(), CodingSessionOpenRequest{
		WorkDir: "/tmp/project",
	})
	require.NoError(t, err)
}

// TestMockCodingSessionSend는 mockCodingSession의 Send 메서드를 검증합니다.
func TestMockCodingSessionSend(t *testing.T) {
	t.Parallel()

	expected := &CodingSessionResponse{
		Content:   "코드 작성 완료",
		SessionID: "test-session",
	}
	session := &mockCodingSession{
		sessionID: "test-session",
		sendResp:  expected,
	}

	resp, err := session.Send(context.Background(), "테스트 메시지")
	require.NoError(t, err)
	assert.Equal(t, expected.Content, resp.Content)
	assert.Equal(t, "test-session", session.SessionID())
}
