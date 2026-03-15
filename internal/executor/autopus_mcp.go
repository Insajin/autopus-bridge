// Package executor - Autopus 인프로세스 MCP 서버
package executor

import (
	"context"
	"encoding/json"
	"sync"

	claudecode "github.com/bpowers/go-claudecode"
	"github.com/bpowers/go-claudecode/chat"
	"github.com/rs/zerolog/log"
)

// AutopusMCPServer는 Claude Code SDK에서 호출 가능한 인프로세스 MCP 서버입니다.
// context, report, ask 3개의 도구를 제공합니다.
// @MX:NOTE: [AUTO] claudecode.MCPSDKConfig로 ClaudeCodeSession에 주입하는 인프로세스 MCP 서버
type AutopusMCPServer struct {
	// tools는 등록된 MCP 도구 목록입니다.
	tools []chat.Tool
	// contextStore는 컨텍스트 키-값 저장소입니다.
	contextStore map[string]string
	// mu는 contextStore 접근을 보호하는 뮤텍스입니다.
	mu sync.RWMutex
}

// NewAutopusMCPServer는 새로운 AutopusMCPServer를 생성합니다.
func NewAutopusMCPServer() *AutopusMCPServer {
	srv := &AutopusMCPServer{
		contextStore: make(map[string]string),
	}
	srv.tools = []chat.Tool{
		&mcpContextTool{srv: srv},
		&mcpReportTool{srv: srv},
		&mcpAskTool{srv: srv},
	}
	return srv
}

// Tools는 등록된 MCP 도구 목록을 반환합니다.
func (s *AutopusMCPServer) Tools() []chat.Tool {
	return s.tools
}

// SDKConfig는 claudecode.MCPSDKConfig를 반환합니다.
// ClaudeCodeSession.Open 시 claudecode.WithSDKMCPServer로 전달합니다.
func (s *AutopusMCPServer) SDKConfig() *claudecode.MCPSDKConfig {
	return &claudecode.MCPSDKConfig{
		Name:  "autopus",
		Tools: s.tools,
	}
}

// SetContext는 컨텍스트 저장소에 키-값을 설정합니다.
func (s *AutopusMCPServer) SetContext(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.contextStore[key] = value
}

// GetContext는 컨텍스트 저장소에서 키로 값을 조회합니다.
func (s *AutopusMCPServer) GetContext(key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.contextStore[key]
	return v, ok
}

// ===== context 도구 =====

// mcpContextTool은 작업 컨텍스트를 조회하는 MCP 도구입니다.
type mcpContextTool struct {
	srv *AutopusMCPServer
}

func (t *mcpContextTool) Name() string { return "context" }

func (t *mcpContextTool) Description() string {
	return "현재 작업 컨텍스트를 조회합니다. key로 특정 컨텍스트 값을 가져옵니다."
}

func (t *mcpContextTool) MCPJsonSchema() string {
	return `{
		"name": "context",
		"description": "현재 작업 컨텍스트를 조회합니다. key로 특정 컨텍스트 값을 가져옵니다.",
		"inputSchema": {
			"type": "object",
			"properties": {
				"key": {
					"type": "string",
					"description": "조회할 컨텍스트 키 (예: current_task, workspace_id)"
				}
			},
			"required": ["key"]
		}
	}`
}

// Call은 지정된 key의 컨텍스트 값을 반환합니다.
func (t *mcpContextTool) Call(_ context.Context, input string) string {
	var req struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal([]byte(input), &req); err != nil {
		return marshalError("입력 파싱 실패: " + err.Error())
	}

	value, _ := t.srv.GetContext(req.Key)
	result := map[string]interface{}{
		"key":   req.Key,
		"value": value,
	}
	return marshalResult(result)
}

// ===== report 도구 =====

// mcpReportTool은 에이전트 작업 결과를 보고하는 MCP 도구입니다.
type mcpReportTool struct {
	srv *AutopusMCPServer
}

func (t *mcpReportTool) Name() string { return "report" }

func (t *mcpReportTool) Description() string {
	return "에이전트 작업 결과를 Autopus 서버에 보고합니다."
}

func (t *mcpReportTool) MCPJsonSchema() string {
	return `{
		"name": "report",
		"description": "에이전트 작업 결과를 Autopus 서버에 보고합니다.",
		"inputSchema": {
			"type": "object",
			"properties": {
				"status": {
					"type": "string",
					"description": "작업 상태 (completed, failed, in_progress)",
					"enum": ["completed", "failed", "in_progress"]
				},
				"summary": {
					"type": "string",
					"description": "작업 결과 요약"
				},
				"details": {
					"type": "string",
					"description": "상세 결과 내용 (선택)"
				}
			},
			"required": ["status", "summary"]
		}
	}`
}

// Call은 작업 결과를 로깅하고 확인 응답을 반환합니다.
func (t *mcpReportTool) Call(_ context.Context, input string) string {
	var req struct {
		Status  string `json:"status"`
		Summary string `json:"summary"`
		Details string `json:"details"`
	}
	if err := json.Unmarshal([]byte(input), &req); err != nil {
		return marshalError("입력 파싱 실패: " + err.Error())
	}

	log.Info().
		Str("tool", "report").
		Str("status", req.Status).
		Str("summary", req.Summary).
		Msg("[autopus-mcp] 작업 결과 보고")

	return marshalResult(map[string]interface{}{
		"status":  "ok",
		"message": "보고 수신 완료",
	})
}

// ===== ask 도구 =====

// mcpAskTool은 에이전트가 사용자에게 질문할 수 있는 MCP 도구입니다.
type mcpAskTool struct {
	srv *AutopusMCPServer
}

func (t *mcpAskTool) Name() string { return "ask" }

func (t *mcpAskTool) Description() string {
	return "에이전트가 작업 진행 중 사용자에게 질문하거나 선택을 요청합니다."
}

func (t *mcpAskTool) MCPJsonSchema() string {
	return `{
		"name": "ask",
		"description": "에이전트가 작업 진행 중 사용자에게 질문하거나 선택을 요청합니다.",
		"inputSchema": {
			"type": "object",
			"properties": {
				"question": {
					"type": "string",
					"description": "사용자에게 물어볼 질문"
				},
				"options": {
					"type": "array",
					"items": {"type": "string"},
					"description": "선택지 목록 (선택)"
				}
			},
			"required": ["question"]
		}
	}`
}

// Call은 질문을 로깅하고 자율 모드 기본 응답을 반환합니다.
// 에이전트가 자율적으로 진행할 수 있도록 "proceed" 기본값을 반환합니다.
func (t *mcpAskTool) Call(_ context.Context, input string) string {
	var req struct {
		Question string   `json:"question"`
		Options  []string `json:"options"`
	}
	if err := json.Unmarshal([]byte(input), &req); err != nil {
		return marshalError("입력 파싱 실패: " + err.Error())
	}

	log.Info().
		Str("tool", "ask").
		Str("question", req.Question).
		Strs("options", req.Options).
		Msg("[autopus-mcp] 에이전트 질문")

	// 자율 모드에서는 첫 번째 옵션 또는 "proceed"를 반환합니다.
	answer := "proceed"
	if len(req.Options) > 0 {
		answer = req.Options[0]
	}

	return marshalResult(map[string]interface{}{
		"answer":   answer,
		"question": req.Question,
	})
}

// marshalResult는 성공 결과를 JSON 문자열로 직렬화합니다.
func marshalResult(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		return `{"error":"직렬화 실패"}`
	}
	return string(data)
}

// marshalError는 에러 결과를 JSON 문자열로 직렬화합니다.
func marshalError(msg string) string {
	data, _ := json.Marshal(map[string]string{"error": msg})
	return string(data)
}
