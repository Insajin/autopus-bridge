// Package executor - Autopus MCP 서버 테스트
package executor

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAutopusMCPServerNew는 NewAutopusMCPServer 생성자를 검증합니다.
func TestAutopusMCPServerNew(t *testing.T) {
	t.Parallel()

	srv := NewAutopusMCPServer()
	require.NotNil(t, srv)
}

// TestAutopusMCPServerTools는 3개의 도구가 등록되었는지 검증합니다.
func TestAutopusMCPServerTools(t *testing.T) {
	t.Parallel()

	srv := NewAutopusMCPServer()
	tools := srv.Tools()
	require.Len(t, tools, 3, "context, report, ask 3개 도구가 있어야 합니다")

	names := make([]string, len(tools))
	for i, tool := range tools {
		names[i] = tool.Name()
	}
	assert.Contains(t, names, "context")
	assert.Contains(t, names, "report")
	assert.Contains(t, names, "ask")
}

// TestAutopusMCPContextTool는 context 도구의 JSON 스키마를 검증합니다.
func TestAutopusMCPContextTool(t *testing.T) {
	t.Parallel()

	srv := NewAutopusMCPServer()
	var contextTool interface {
		Name() string
		MCPJsonSchema() string
		Description() string
	}
	for _, tool := range srv.Tools() {
		if tool.Name() == "context" {
			contextTool = tool
			break
		}
	}
	require.NotNil(t, contextTool, "context 도구가 있어야 합니다")

	schema := contextTool.MCPJsonSchema()
	var parsed map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(schema), &parsed))
	assert.Equal(t, "context", parsed["name"])
	assert.NotEmpty(t, parsed["description"])
}

// TestAutopusMCPReportTool는 report 도구의 Call을 검증합니다.
func TestAutopusMCPReportTool(t *testing.T) {
	t.Parallel()

	srv := NewAutopusMCPServer()
	var reportTool interface {
		Name() string
		Call(context.Context, string) string
	}
	for _, tool := range srv.Tools() {
		if tool.Name() == "report" {
			reportTool = tool
			break
		}
	}
	require.NotNil(t, reportTool)

	// 유효한 JSON 입력으로 호출
	input := `{"status": "completed", "summary": "테스트 완료"}`
	result := reportTool.Call(context.Background(), input)
	assert.NotEmpty(t, result)

	var parsed map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(result), &parsed))
	assert.Equal(t, "ok", parsed["status"])
}

// TestAutopusMCPAskTool는 ask 도구의 Call을 검증합니다.
func TestAutopusMCPAskTool(t *testing.T) {
	t.Parallel()

	srv := NewAutopusMCPServer()
	var askTool interface {
		Name() string
		Call(context.Context, string) string
	}
	for _, tool := range srv.Tools() {
		if tool.Name() == "ask" {
			askTool = tool
			break
		}
	}
	require.NotNil(t, askTool)

	// ask 도구는 질문을 로깅하고 결과를 반환해야 함
	input := `{"question": "작업을 계속할까요?", "options": ["yes", "no"]}`
	result := askTool.Call(context.Background(), input)
	assert.NotEmpty(t, result)

	var parsed map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(result), &parsed))
	assert.NotNil(t, parsed["answer"])
}

// TestAutopusMCPContextToolCall는 context 도구의 Call을 검증합니다.
func TestAutopusMCPContextToolCall(t *testing.T) {
	t.Parallel()

	srv := NewAutopusMCPServer()
	var contextTool interface {
		Call(context.Context, string) string
	}
	for _, tool := range srv.Tools() {
		if tool.Name() == "context" {
			contextTool = tool
			break
		}
	}
	require.NotNil(t, contextTool)

	// context 도구: 작업 컨텍스트 조회
	input := `{"key": "current_task"}`
	result := contextTool.Call(context.Background(), input)
	assert.NotEmpty(t, result)

	var parsed map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(result), &parsed))
	// 빈 컨텍스트에서도 응답은 있어야 함
	assert.Contains(t, parsed, "value")
}

// TestAutopusMCPSDKConfig는 SDK MCP 설정 생성을 검증합니다.
func TestAutopusMCPSDKConfig(t *testing.T) {
	t.Parallel()

	srv := NewAutopusMCPServer()
	config := srv.SDKConfig()
	require.NotNil(t, config)
	assert.Equal(t, "autopus", config.Name)
	assert.Len(t, config.Tools, 3)
}
