package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/insajin/autopus-bridge/internal/auth"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/rs/zerolog"
)

// --- 테스트 헬퍼 ---

// testToken은 테스트에서 사용하는 고정 JWT 토큰입니다.
const testToken = "test-jwt-token-abc123"

// newTestTokenRefresher는 항상 고정 토큰을 반환하는 TokenRefresher를 생성합니다.
// 실제 Supabase 갱신 없이 auth.TokenRefresher를 사용할 수 있도록
// 유효한 Credentials를 주입합니다.
func newTestTokenRefresher() *auth.TokenRefresher {
	creds := &auth.Credentials{
		AccessToken:  testToken,
		RefreshToken: "test-refresh-token",
		ExpiresAt:    time.Now().Add(1 * time.Hour), // 1시간 후 만료
	}
	return auth.NewTokenRefresher(creds)
}

// newTestServer는 테스트용 MCP Server를 생성합니다.
// mock HTTP 서버 URL과 테스트 TokenRefresher를 사용합니다.
func newTestServer(mockURL string, cacheTTL ...time.Duration) *Server {
	logger := zerolog.Nop()
	tokenRefresher := newTestTokenRefresher()
	client := NewBackendClient(mockURL, tokenRefresher, 5*time.Second, logger)
	return NewServer(client, logger, cacheTTL...)
}

// makeCallToolRequest는 테스트용 CallToolRequest를 생성하는 헬퍼입니다.
func makeCallToolRequest(name string, args map[string]interface{}) mcp.CallToolRequest {
	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      name,
			Arguments: args,
		},
	}
}

// makeReadResourceRequest는 테스트용 ReadResourceRequest를 생성하는 헬퍼입니다.
func makeReadResourceRequest(uri string) mcp.ReadResourceRequest {
	return mcp.ReadResourceRequest{
		Params: mcp.ReadResourceParams{
			URI: uri,
		},
	}
}

// extractTextFromToolResult는 CallToolResult에서 텍스트 콘텐츠를 추출합니다.
func extractTextFromToolResult(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	if result == nil {
		t.Fatal("CallToolResult가 nil입니다")
	}
	if len(result.Content) == 0 {
		t.Fatal("CallToolResult.Content가 비어 있습니다")
	}
	tc, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("Content[0]이 TextContent가 아닙니다: %T", result.Content[0])
	}
	return tc.Text
}

// extractTextFromResourceResult는 ResourceContents에서 텍스트를 추출합니다.
func extractTextFromResourceResult(t *testing.T, contents []mcp.ResourceContents) string {
	t.Helper()
	if len(contents) == 0 {
		t.Fatal("ResourceContents가 비어 있습니다")
	}
	tc, ok := contents[0].(mcp.TextResourceContents)
	if !ok {
		t.Fatalf("ResourceContents[0]이 TextResourceContents가 아닙니다: %T", contents[0])
	}
	return tc.Text
}

// newMockBackend는 API 라우트별로 응답을 반환하는 mock HTTP 서버를 생성합니다.
// handler는 각 요청의 (method, path)를 기반으로 응답을 결정합니다.
func newMockBackend(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	return httptest.NewServer(handler)
}

// standardMockHandler는 모든 API 엔드포인트에 성공 응답을 반환하는 표준 mock 핸들러입니다.
// authHeader를 검증하고, 각 경로에 따라 적절한 apiResponse를 반환합니다.
func standardMockHandler(t *testing.T) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		// JWT 인증 헤더 검증
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			writeAPIError(w, http.StatusUnauthorized, "missing authorization header")
			return
		}
		if authHeader != "Bearer "+testToken {
			writeAPIError(w, http.StatusUnauthorized, "invalid token")
			return
		}

		path := r.URL.Path
		method := r.Method

		switch {
		// POST /api/v1/executions - execute_task
		case method == http.MethodPost && path == "/api/v1/executions":
			var reqBody ExecuteTaskRequest
			if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
				writeAPIError(w, http.StatusBadRequest, "invalid request body")
				return
			}
			writeAPISuccess(w, ExecuteTaskResponse{
				ExecutionID: "exec-001",
				Status:      "running",
				Message:     fmt.Sprintf("Task submitted to agent %s", reqBody.AgentID),
			})

		// GET /api/v1/agents - list_agents
		case method == http.MethodGet && strings.HasPrefix(path, "/api/v1/agents"):
			writeAPISuccess(w, ListAgentsResponse{
				Agents: []AgentInfo{
					{ID: "agent-1", Name: "Code Reviewer", Description: "Automated code review"},
					{ID: "agent-2", Name: "Test Generator", Description: "Generate test cases"},
				},
				Total: 2,
			})

		// GET /api/v1/executions/{id} - get_execution_status
		case method == http.MethodGet && strings.HasPrefix(path, "/api/v1/executions/"):
			executionID := strings.TrimPrefix(path, "/api/v1/executions/")
			// approve 경로인 경우 무시
			if strings.Contains(executionID, "/") {
				break
			}
			writeAPISuccess(w, ExecutionStatus{
				ExecutionID: executionID,
				Status:      "completed",
				Result:      json.RawMessage(`{"output":"task done"}`),
				CreatedAt:   "2025-01-01T00:00:00Z",
				UpdatedAt:   "2025-01-01T00:01:00Z",
			})

		// POST /api/v1/executions/{id}/approve - approve_execution
		case method == http.MethodPost && strings.Contains(path, "/approve"):
			var reqBody ApproveExecutionRequest
			if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
				writeAPIError(w, http.StatusBadRequest, "invalid request body")
				return
			}
			writeAPISuccess(w, ApproveExecutionResponse{
				ExecutionID: reqBody.ExecutionID,
				Status:      "approved",
				Message:     "Execution approved",
			})

		// GET /api/v1/workspaces - list workspaces
		case method == http.MethodGet && path == "/api/v1/workspaces":
			writeAPISuccess(w, ManageWorkspaceResponse{
				Workspaces: []WorkspaceInfo{
					{ID: "ws-1", Name: "Default", Slug: "default"},
					{ID: "ws-2", Name: "Production", Slug: "prod"},
				},
			})

		// POST /api/v1/workspaces - create workspace
		case method == http.MethodPost && path == "/api/v1/workspaces":
			writeAPISuccess(w, ManageWorkspaceResponse{
				Workspace: &WorkspaceInfo{ID: "ws-new", Name: "New Workspace"},
				Message:   "Workspace created",
			})

		// GET /api/v1/workspaces/{id}
		case method == http.MethodGet && strings.HasPrefix(path, "/api/v1/workspaces/"):
			wsID := strings.TrimPrefix(path, "/api/v1/workspaces/")
			writeAPISuccess(w, ManageWorkspaceResponse{
				Workspace: &WorkspaceInfo{ID: wsID, Name: "Workspace " + wsID},
			})

		// PUT /api/v1/workspaces/{id}
		case method == http.MethodPut && strings.HasPrefix(path, "/api/v1/workspaces/"):
			wsID := strings.TrimPrefix(path, "/api/v1/workspaces/")
			writeAPISuccess(w, ManageWorkspaceResponse{
				Workspace: &WorkspaceInfo{ID: wsID, Name: "Updated"},
				Message:   "Workspace updated",
			})

		// DELETE /api/v1/workspaces/{id}
		case method == http.MethodDelete && strings.HasPrefix(path, "/api/v1/workspaces/"):
			writeAPISuccess(w, ManageWorkspaceResponse{
				Message: "Workspace deleted",
			})

		// POST /api/v1/knowledge/search - search_knowledge
		case method == http.MethodPost && path == "/api/v1/knowledge/search":
			var reqBody SearchKnowledgeRequest
			if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
				writeAPIError(w, http.StatusBadRequest, "invalid request body")
				return
			}
			writeAPISuccess(w, SearchKnowledgeResponse{
				Results: []KnowledgeResult{
					{ID: "k-1", Title: "Setup Guide", Content: "How to set up...", Score: 0.95},
				},
				Total: 1,
				Query: reqBody.Query,
			})

		default:
			writeAPIError(w, http.StatusNotFound, "endpoint not found")
		}
	}
}

// writeAPISuccess는 성공 apiResponse를 JSON으로 작성합니다.
func writeAPISuccess(w http.ResponseWriter, data interface{}) {
	dataBytes, _ := json.Marshal(data)
	resp := apiResponse{
		Success: true,
		Data:    json.RawMessage(dataBytes),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// writeAPIError는 에러 apiResponse를 JSON으로 작성합니다.
func writeAPIError(w http.ResponseWriter, statusCode int, errMsg string) {
	resp := apiResponse{
		Success: false,
		Error:   errMsg,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(resp)
}

// =====================================
// Scenario 4: BackendClient를 통한 도구 호출 + 인증
// =====================================

func TestIntegration_ExecuteTask(t *testing.T) {
	t.Parallel()

	mock := newMockBackend(t, standardMockHandler(t))
	defer mock.Close()

	srv := newTestServer(mock.URL)
	ctx := context.Background()

	tests := []struct {
		name       string
		args       map[string]interface{}
		wantErr    bool
		wantErrMsg string
		checkResp  func(t *testing.T, text string)
	}{
		{
			name: "기본 태스크 실행 성공",
			args: map[string]interface{}{
				"agent_id": "agent-1",
				"prompt":   "Review this code",
			},
			checkResp: func(t *testing.T, text string) {
				var resp ExecuteTaskResponse
				if err := json.Unmarshal([]byte(text), &resp); err != nil {
					t.Fatalf("응답 JSON 파싱 실패: %v", err)
				}
				if resp.ExecutionID != "exec-001" {
					t.Errorf("ExecutionID = %q, want %q", resp.ExecutionID, "exec-001")
				}
				if resp.Status != "running" {
					t.Errorf("Status = %q, want %q", resp.Status, "running")
				}
			},
		},
		{
			name: "모든 선택 파라미터 포함",
			args: map[string]interface{}{
				"agent_id":     "agent-1",
				"prompt":       "Generate tests",
				"workspace_id": "ws-1",
				"tools":        "search,calculator,browser",
				"model":        "gpt-4",
			},
			checkResp: func(t *testing.T, text string) {
				var resp ExecuteTaskResponse
				if err := json.Unmarshal([]byte(text), &resp); err != nil {
					t.Fatalf("응답 JSON 파싱 실패: %v", err)
				}
				if resp.ExecutionID == "" {
					t.Error("ExecutionID가 비어 있습니다")
				}
			},
		},
		{
			name: "agent_id 누락 시 에러",
			args: map[string]interface{}{
				"prompt": "Review this code",
			},
			wantErr:    true,
			wantErrMsg: "agent_id",
		},
		{
			name: "prompt 누락 시 에러",
			args: map[string]interface{}{
				"agent_id": "agent-1",
			},
			wantErr:    true,
			wantErrMsg: "prompt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := makeCallToolRequest("execute_task", tt.args)
			result, err := srv.handleExecuteTask(ctx, req)
			if err != nil {
				t.Fatalf("handleExecuteTask 에러: %v", err)
			}

			text := extractTextFromToolResult(t, result)
			if tt.wantErr {
				if !result.IsError {
					t.Fatal("IsError가 false이지만, 에러를 기대했습니다")
				}
				if !strings.Contains(text, tt.wantErrMsg) {
					t.Errorf("에러 메시지에 %q 포함 기대, 실제: %q", tt.wantErrMsg, text)
				}
				return
			}

			if result.IsError {
				t.Fatalf("예상하지 않은 에러: %s", text)
			}
			if tt.checkResp != nil {
				tt.checkResp(t, text)
			}
		})
	}
}

func TestIntegration_ListAgents(t *testing.T) {
	t.Parallel()

	mock := newMockBackend(t, standardMockHandler(t))
	defer mock.Close()

	srv := newTestServer(mock.URL)
	ctx := context.Background()

	req := makeCallToolRequest("list_agents", map[string]interface{}{
		"workspace_id": "ws-1",
		"filter":       "code",
	})

	result, err := srv.handleListAgents(ctx, req)
	if err != nil {
		t.Fatalf("handleListAgents 에러: %v", err)
	}

	text := extractTextFromToolResult(t, result)
	if result.IsError {
		t.Fatalf("예상하지 않은 에러: %s", text)
	}

	var resp ListAgentsResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("응답 JSON 파싱 실패: %v", err)
	}
	if resp.Total != 2 {
		t.Errorf("Total = %d, want %d", resp.Total, 2)
	}
	if len(resp.Agents) != 2 {
		t.Errorf("Agents 개수 = %d, want %d", len(resp.Agents), 2)
	}
}

func TestIntegration_GetExecutionStatus(t *testing.T) {
	t.Parallel()

	mock := newMockBackend(t, standardMockHandler(t))
	defer mock.Close()

	srv := newTestServer(mock.URL)
	ctx := context.Background()

	tests := []struct {
		name       string
		args       map[string]interface{}
		wantErr    bool
		wantErrMsg string
		checkResp  func(t *testing.T, text string)
	}{
		{
			name: "실행 상태 조회 성공",
			args: map[string]interface{}{
				"execution_id": "exec-001",
			},
			checkResp: func(t *testing.T, text string) {
				var resp ExecutionStatus
				if err := json.Unmarshal([]byte(text), &resp); err != nil {
					t.Fatalf("응답 JSON 파싱 실패: %v", err)
				}
				if resp.ExecutionID != "exec-001" {
					t.Errorf("ExecutionID = %q, want %q", resp.ExecutionID, "exec-001")
				}
				if resp.Status != "completed" {
					t.Errorf("Status = %q, want %q", resp.Status, "completed")
				}
			},
		},
		{
			name:       "execution_id 누락 시 에러",
			args:       map[string]interface{}{},
			wantErr:    true,
			wantErrMsg: "execution_id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := makeCallToolRequest("get_execution_status", tt.args)
			result, err := srv.handleGetExecutionStatus(ctx, req)
			if err != nil {
				t.Fatalf("handleGetExecutionStatus 에러: %v", err)
			}

			text := extractTextFromToolResult(t, result)
			if tt.wantErr {
				if !result.IsError {
					t.Fatal("IsError가 false이지만, 에러를 기대했습니다")
				}
				if !strings.Contains(text, tt.wantErrMsg) {
					t.Errorf("에러 메시지에 %q 포함 기대, 실제: %q", tt.wantErrMsg, text)
				}
				return
			}

			if result.IsError {
				t.Fatalf("예상하지 않은 에러: %s", text)
			}
			if tt.checkResp != nil {
				tt.checkResp(t, text)
			}
		})
	}
}

func TestIntegration_ApproveExecution(t *testing.T) {
	t.Parallel()

	mock := newMockBackend(t, standardMockHandler(t))
	defer mock.Close()

	srv := newTestServer(mock.URL)
	ctx := context.Background()

	tests := []struct {
		name       string
		args       map[string]interface{}
		wantErr    bool
		wantErrMsg string
		checkResp  func(t *testing.T, text string)
	}{
		{
			name: "실행 승인 성공",
			args: map[string]interface{}{
				"execution_id": "exec-001",
				"decision":     "approve",
				"reason":       "Looks good",
			},
			checkResp: func(t *testing.T, text string) {
				var resp ApproveExecutionResponse
				if err := json.Unmarshal([]byte(text), &resp); err != nil {
					t.Fatalf("응답 JSON 파싱 실패: %v", err)
				}
				if resp.Status != "approved" {
					t.Errorf("Status = %q, want %q", resp.Status, "approved")
				}
			},
		},
		{
			name: "실행 거부 성공",
			args: map[string]interface{}{
				"execution_id": "exec-002",
				"decision":     "reject",
				"reason":       "Not ready",
			},
			checkResp: func(t *testing.T, text string) {
				var resp ApproveExecutionResponse
				if err := json.Unmarshal([]byte(text), &resp); err != nil {
					t.Fatalf("응답 JSON 파싱 실패: %v", err)
				}
				// mock은 항상 "approved"를 반환하지만, 요청 파싱 자체가 성공적인지 확인
				if resp.ExecutionID == "" {
					t.Error("ExecutionID가 비어 있습니다")
				}
			},
		},
		{
			name: "잘못된 decision 값",
			args: map[string]interface{}{
				"execution_id": "exec-001",
				"decision":     "maybe",
			},
			wantErr:    true,
			wantErrMsg: "approve",
		},
		{
			name: "execution_id 누락",
			args: map[string]interface{}{
				"decision": "approve",
			},
			wantErr:    true,
			wantErrMsg: "execution_id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := makeCallToolRequest("approve_execution", tt.args)
			result, err := srv.handleApproveExecution(ctx, req)
			if err != nil {
				t.Fatalf("handleApproveExecution 에러: %v", err)
			}

			text := extractTextFromToolResult(t, result)
			if tt.wantErr {
				if !result.IsError {
					t.Fatal("IsError가 false이지만, 에러를 기대했습니다")
				}
				if !strings.Contains(text, tt.wantErrMsg) {
					t.Errorf("에러 메시지에 %q 포함 기대, 실제: %q", tt.wantErrMsg, text)
				}
				return
			}

			if result.IsError {
				t.Fatalf("예상하지 않은 에러: %s", text)
			}
			if tt.checkResp != nil {
				tt.checkResp(t, text)
			}
		})
	}
}

func TestIntegration_ManageWorkspace(t *testing.T) {
	t.Parallel()

	mock := newMockBackend(t, standardMockHandler(t))
	defer mock.Close()

	srv := newTestServer(mock.URL)
	ctx := context.Background()

	tests := []struct {
		name       string
		args       map[string]interface{}
		wantErr    bool
		wantErrMsg string
		checkResp  func(t *testing.T, text string)
	}{
		{
			name: "워크스페이스 목록 조회",
			args: map[string]interface{}{
				"action": "list",
			},
			checkResp: func(t *testing.T, text string) {
				var resp ManageWorkspaceResponse
				if err := json.Unmarshal([]byte(text), &resp); err != nil {
					t.Fatalf("응답 JSON 파싱 실패: %v", err)
				}
				if len(resp.Workspaces) != 2 {
					t.Errorf("Workspaces 개수 = %d, want %d", len(resp.Workspaces), 2)
				}
			},
		},
		{
			name: "워크스페이스 생성",
			args: map[string]interface{}{
				"action": "create",
				"config": `{"name":"New Workspace"}`,
			},
			checkResp: func(t *testing.T, text string) {
				var resp ManageWorkspaceResponse
				if err := json.Unmarshal([]byte(text), &resp); err != nil {
					t.Fatalf("응답 JSON 파싱 실패: %v", err)
				}
				if resp.Workspace == nil {
					t.Fatal("Workspace가 nil입니다")
				}
			},
		},
		{
			name: "워크스페이스 조회 (get)",
			args: map[string]interface{}{
				"action":       "get",
				"workspace_id": "ws-1",
			},
			checkResp: func(t *testing.T, text string) {
				var resp ManageWorkspaceResponse
				if err := json.Unmarshal([]byte(text), &resp); err != nil {
					t.Fatalf("응답 JSON 파싱 실패: %v", err)
				}
				if resp.Workspace == nil {
					t.Fatal("Workspace가 nil입니다")
				}
				if resp.Workspace.ID != "ws-1" {
					t.Errorf("Workspace.ID = %q, want %q", resp.Workspace.ID, "ws-1")
				}
			},
		},
		{
			name: "get에 workspace_id 누락 시 에러",
			args: map[string]interface{}{
				"action": "get",
			},
			wantErr:    true,
			wantErrMsg: "workspace_id is required",
		},
		{
			name: "잘못된 action",
			args: map[string]interface{}{
				"action": "invalid_action",
			},
			wantErr:    true,
			wantErrMsg: "action must be one of",
		},
		{
			name: "잘못된 config JSON",
			args: map[string]interface{}{
				"action": "create",
				"config": "{invalid json}",
			},
			wantErr:    true,
			wantErrMsg: "invalid config JSON",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := makeCallToolRequest("manage_workspace", tt.args)
			result, err := srv.handleManageWorkspace(ctx, req)
			if err != nil {
				t.Fatalf("handleManageWorkspace 에러: %v", err)
			}

			text := extractTextFromToolResult(t, result)
			if tt.wantErr {
				if !result.IsError {
					t.Fatal("IsError가 false이지만, 에러를 기대했습니다")
				}
				if !strings.Contains(text, tt.wantErrMsg) {
					t.Errorf("에러 메시지에 %q 포함 기대, 실제: %q", tt.wantErrMsg, text)
				}
				return
			}

			if result.IsError {
				t.Fatalf("예상하지 않은 에러: %s", text)
			}
			if tt.checkResp != nil {
				tt.checkResp(t, text)
			}
		})
	}
}

func TestIntegration_SearchKnowledge(t *testing.T) {
	t.Parallel()

	mock := newMockBackend(t, standardMockHandler(t))
	defer mock.Close()

	srv := newTestServer(mock.URL)
	ctx := context.Background()

	tests := []struct {
		name       string
		args       map[string]interface{}
		wantErr    bool
		wantErrMsg string
		checkResp  func(t *testing.T, text string)
	}{
		{
			name: "지식 검색 성공",
			args: map[string]interface{}{
				"query": "setup guide",
			},
			checkResp: func(t *testing.T, text string) {
				var resp SearchKnowledgeResponse
				if err := json.Unmarshal([]byte(text), &resp); err != nil {
					t.Fatalf("응답 JSON 파싱 실패: %v", err)
				}
				if resp.Total != 1 {
					t.Errorf("Total = %d, want %d", resp.Total, 1)
				}
				if resp.Query != "setup guide" {
					t.Errorf("Query = %q, want %q", resp.Query, "setup guide")
				}
				if len(resp.Results) == 0 {
					t.Fatal("Results가 비어 있습니다")
				}
				if resp.Results[0].Score != 0.95 {
					t.Errorf("Score = %f, want %f", resp.Results[0].Score, 0.95)
				}
			},
		},
		{
			name: "선택 파라미터 포함 검색",
			args: map[string]interface{}{
				"query":        "API docs",
				"workspace_id": "ws-1",
				"limit":        float64(5), // JSON 숫자는 float64로 변환됨
				"filters":      `{"source":"docs"}`,
			},
			checkResp: func(t *testing.T, text string) {
				var resp SearchKnowledgeResponse
				if err := json.Unmarshal([]byte(text), &resp); err != nil {
					t.Fatalf("응답 JSON 파싱 실패: %v", err)
				}
				if resp.Query != "API docs" {
					t.Errorf("Query = %q, want %q", resp.Query, "API docs")
				}
			},
		},
		{
			name:       "query 누락 시 에러",
			args:       map[string]interface{}{},
			wantErr:    true,
			wantErrMsg: "query",
		},
		{
			name: "잘못된 filters JSON",
			args: map[string]interface{}{
				"query":   "test",
				"filters": "{bad json}",
			},
			wantErr:    true,
			wantErrMsg: "invalid filters JSON",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := makeCallToolRequest("search_knowledge", tt.args)
			result, err := srv.handleSearchKnowledge(ctx, req)
			if err != nil {
				t.Fatalf("handleSearchKnowledge 에러: %v", err)
			}

			text := extractTextFromToolResult(t, result)
			if tt.wantErr {
				if !result.IsError {
					t.Fatal("IsError가 false이지만, 에러를 기대했습니다")
				}
				if !strings.Contains(text, tt.wantErrMsg) {
					t.Errorf("에러 메시지에 %q 포함 기대, 실제: %q", tt.wantErrMsg, text)
				}
				return
			}

			if result.IsError {
				t.Fatalf("예상하지 않은 에러: %s", text)
			}
			if tt.checkResp != nil {
				tt.checkResp(t, text)
			}
		})
	}
}

// TestIntegration_AuthHeaderVerification은 JWT 인증 헤더가 올바르게 전송되는지 검증합니다.
func TestIntegration_AuthHeaderVerification(t *testing.T) {
	t.Parallel()

	var capturedAuthHeader string
	var capturedContentType string

	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuthHeader = r.Header.Get("Authorization")
		capturedContentType = r.Header.Get("Content-Type")

		agentsData, _ := json.Marshal(ListAgentsResponse{
			Agents: []AgentInfo{{ID: "a1", Name: "Agent1"}},
			Total:  1,
		})
		writeAPISuccess(w, json.RawMessage(agentsData))
	}))
	defer mock.Close()

	srv := newTestServer(mock.URL)
	ctx := context.Background()

	req := makeCallToolRequest("list_agents", map[string]interface{}{})
	result, err := srv.handleListAgents(ctx, req)
	if err != nil {
		t.Fatalf("handleListAgents 에러: %v", err)
	}

	// 인증 헤더 검증
	expectedAuth := "Bearer " + testToken
	if capturedAuthHeader != expectedAuth {
		t.Errorf("Authorization 헤더 = %q, want %q", capturedAuthHeader, expectedAuth)
	}

	// Content-Type 헤더 검증
	if capturedContentType != "application/json" {
		t.Errorf("Content-Type 헤더 = %q, want %q", capturedContentType, "application/json")
	}

	// 응답이 에러가 아닌지 확인
	if result.IsError {
		text := extractTextFromToolResult(t, result)
		t.Fatalf("예상하지 않은 에러: %s", text)
	}
}

// TestIntegration_RequestParametersPassed는 요청 파라미터가 백엔드로 올바르게 전달되는지 검증합니다.
func TestIntegration_RequestParametersPassed(t *testing.T) {
	t.Parallel()

	var capturedBody ExecuteTaskRequest
	var capturedPath string

	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path

		if r.Method == http.MethodPost {
			json.NewDecoder(r.Body).Decode(&capturedBody)
		}

		execData, _ := json.Marshal(ExecuteTaskResponse{
			ExecutionID: "exec-test",
			Status:      "running",
		})
		writeAPISuccess(w, json.RawMessage(execData))
	}))
	defer mock.Close()

	srv := newTestServer(mock.URL)
	ctx := context.Background()

	req := makeCallToolRequest("execute_task", map[string]interface{}{
		"agent_id":     "agent-custom",
		"prompt":       "Do something specific",
		"workspace_id": "ws-99",
		"tools":        "tool1,tool2,tool3",
		"model":        "claude-3",
	})

	result, err := srv.handleExecuteTask(ctx, req)
	if err != nil {
		t.Fatalf("handleExecuteTask 에러: %v", err)
	}
	if result.IsError {
		text := extractTextFromToolResult(t, result)
		t.Fatalf("예상하지 않은 에러: %s", text)
	}

	// 요청 경로 검증
	if capturedPath != "/api/v1/executions" {
		t.Errorf("요청 경로 = %q, want %q", capturedPath, "/api/v1/executions")
	}

	// 요청 본문 파라미터 검증
	if capturedBody.AgentID != "agent-custom" {
		t.Errorf("AgentID = %q, want %q", capturedBody.AgentID, "agent-custom")
	}
	if capturedBody.Prompt != "Do something specific" {
		t.Errorf("Prompt = %q, want %q", capturedBody.Prompt, "Do something specific")
	}
	if capturedBody.WorkspaceID != "ws-99" {
		t.Errorf("WorkspaceID = %q, want %q", capturedBody.WorkspaceID, "ws-99")
	}
	if len(capturedBody.Tools) != 3 {
		t.Errorf("Tools 개수 = %d, want %d", len(capturedBody.Tools), 3)
	}
	if capturedBody.Model != "claude-3" {
		t.Errorf("Model = %q, want %q", capturedBody.Model, "claude-3")
	}
}

// =====================================
// Scenario 5: 리소스 핸들러가 올바른 데이터 반환
// =====================================

func TestIntegration_StatusResource(t *testing.T) {
	t.Parallel()

	mock := newMockBackend(t, standardMockHandler(t))
	defer mock.Close()

	srv := newTestServer(mock.URL)
	ctx := context.Background()

	req := makeReadResourceRequest("autopus://status")
	contents, err := srv.handleStatusResource(ctx, req)
	if err != nil {
		t.Fatalf("handleStatusResource 에러: %v", err)
	}

	text := extractTextFromResourceResult(t, contents)

	var status PlatformStatus
	if err := json.Unmarshal([]byte(text), &status); err != nil {
		t.Fatalf("상태 JSON 파싱 실패: %v", err)
	}

	if !status.Connected {
		t.Error("Connected = false, want true")
	}
	if status.ServerName != ServerName {
		t.Errorf("ServerName = %q, want %q", status.ServerName, ServerName)
	}
	if status.Version != ServerVersion {
		t.Errorf("Version = %q, want %q", status.Version, ServerVersion)
	}
	if status.BackendURL != mock.URL {
		t.Errorf("BackendURL = %q, want %q", status.BackendURL, mock.URL)
	}
	if status.Message != "Connected to Autopus backend" {
		t.Errorf("Message = %q, want %q", status.Message, "Connected to Autopus backend")
	}
}

func TestIntegration_WorkspacesResource(t *testing.T) {
	t.Parallel()

	mock := newMockBackend(t, standardMockHandler(t))
	defer mock.Close()

	srv := newTestServer(mock.URL)
	ctx := context.Background()

	req := makeReadResourceRequest("autopus://workspaces")
	contents, err := srv.handleWorkspacesResource(ctx, req)
	if err != nil {
		t.Fatalf("handleWorkspacesResource 에러: %v", err)
	}

	text := extractTextFromResourceResult(t, contents)

	var resp ManageWorkspaceResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("워크스페이스 JSON 파싱 실패: %v", err)
	}

	if len(resp.Workspaces) != 2 {
		t.Errorf("Workspaces 개수 = %d, want %d", len(resp.Workspaces), 2)
	}
	if resp.Workspaces[0].ID != "ws-1" {
		t.Errorf("첫 번째 워크스페이스 ID = %q, want %q", resp.Workspaces[0].ID, "ws-1")
	}
	if resp.Workspaces[1].Slug != "prod" {
		t.Errorf("두 번째 워크스페이스 Slug = %q, want %q", resp.Workspaces[1].Slug, "prod")
	}
}

func TestIntegration_AgentsResource(t *testing.T) {
	t.Parallel()

	mock := newMockBackend(t, standardMockHandler(t))
	defer mock.Close()

	srv := newTestServer(mock.URL)
	ctx := context.Background()

	req := makeReadResourceRequest("autopus://agents")
	contents, err := srv.handleAgentsResource(ctx, req)
	if err != nil {
		t.Fatalf("handleAgentsResource 에러: %v", err)
	}

	text := extractTextFromResourceResult(t, contents)

	var resp ListAgentsResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("에이전트 JSON 파싱 실패: %v", err)
	}

	if resp.Total != 2 {
		t.Errorf("Total = %d, want %d", resp.Total, 2)
	}
	if len(resp.Agents) != 2 {
		t.Errorf("Agents 개수 = %d, want %d", len(resp.Agents), 2)
	}
	if resp.Agents[0].Name != "Code Reviewer" {
		t.Errorf("첫 번째 에이전트 Name = %q, want %q", resp.Agents[0].Name, "Code Reviewer")
	}
}

func TestIntegration_ExecutionResource(t *testing.T) {
	t.Parallel()

	mock := newMockBackend(t, standardMockHandler(t))
	defer mock.Close()

	srv := newTestServer(mock.URL)
	ctx := context.Background()

	req := makeReadResourceRequest("autopus://executions/exec-abc")
	contents, err := srv.handleExecutionResource(ctx, req)
	if err != nil {
		t.Fatalf("handleExecutionResource 에러: %v", err)
	}

	text := extractTextFromResourceResult(t, contents)

	var status ExecutionStatus
	if err := json.Unmarshal([]byte(text), &status); err != nil {
		t.Fatalf("실행 상태 JSON 파싱 실패: %v", err)
	}

	if status.ExecutionID != "exec-abc" {
		t.Errorf("ExecutionID = %q, want %q", status.ExecutionID, "exec-abc")
	}
	if status.Status != "completed" {
		t.Errorf("Status = %q, want %q", status.Status, "completed")
	}
}

// =====================================
// Scenario 6: 캐시를 통한 Graceful Degradation
// =====================================

func TestIntegration_GracefulDegradation_StatusCache(t *testing.T) {
	t.Parallel()

	mock := newMockBackend(t, standardMockHandler(t))
	srv := newTestServer(mock.URL, 10*time.Minute) // 긴 TTL로 캐시 만료 방지
	ctx := context.Background()

	// Step 1: 성공적인 리소스 호출 (데이터가 캐시됨)
	req := makeReadResourceRequest("autopus://status")
	contents, err := srv.handleStatusResource(ctx, req)
	if err != nil {
		t.Fatalf("첫 번째 호출 에러: %v", err)
	}

	text := extractTextFromResourceResult(t, contents)
	var status PlatformStatus
	if err := json.Unmarshal([]byte(text), &status); err != nil {
		t.Fatalf("상태 JSON 파싱 실패: %v", err)
	}
	if !status.Connected {
		t.Fatal("첫 번째 호출에서 Connected = false, want true")
	}

	// Step 2: 백엔드 서버 종료
	mock.Close()

	// Step 3: 같은 리소스 다시 요청 - 캐시된 데이터가 반환되어야 함
	contents, err = srv.handleStatusResource(ctx, req)
	if err != nil {
		t.Fatalf("두 번째 호출(캐시 폴백) 에러: %v", err)
	}

	text = extractTextFromResourceResult(t, contents)
	var cachedStatus PlatformStatus
	if err := json.Unmarshal([]byte(text), &cachedStatus); err != nil {
		t.Fatalf("캐시된 상태 JSON 파싱 실패: %v", err)
	}

	// 캐시 메타데이터 검증
	if cachedStatus.Connected {
		t.Error("캐시 폴백에서 Connected = true, want false")
	}
	if !cachedStatus.Cached {
		t.Error("캐시 폴백에서 Cached = false, want true")
	}
	if cachedStatus.CachedAt == "" {
		t.Error("CachedAt이 비어 있습니다")
	}
	if !strings.Contains(cachedStatus.Message, "Backend unreachable") {
		t.Errorf("Message에 'Backend unreachable' 포함 기대, 실제: %q", cachedStatus.Message)
	}
	if !strings.Contains(cachedStatus.Message, "cached data") {
		t.Errorf("Message에 'cached data' 포함 기대, 실제: %q", cachedStatus.Message)
	}
}

func TestIntegration_GracefulDegradation_WorkspacesCache(t *testing.T) {
	t.Parallel()

	mock := newMockBackend(t, standardMockHandler(t))
	srv := newTestServer(mock.URL, 10*time.Minute)
	ctx := context.Background()

	// Step 1: 성공적인 호출로 캐시 채우기
	req := makeReadResourceRequest("autopus://workspaces")
	contents, err := srv.handleWorkspacesResource(ctx, req)
	if err != nil {
		t.Fatalf("첫 번째 호출 에러: %v", err)
	}

	text := extractTextFromResourceResult(t, contents)
	var firstResp ManageWorkspaceResponse
	if err := json.Unmarshal([]byte(text), &firstResp); err != nil {
		t.Fatalf("워크스페이스 JSON 파싱 실패: %v", err)
	}
	if len(firstResp.Workspaces) != 2 {
		t.Fatalf("첫 번째 호출 Workspaces 개수 = %d, want %d", len(firstResp.Workspaces), 2)
	}

	// Step 2: 백엔드 종료
	mock.Close()

	// Step 3: 캐시 폴백 확인
	contents, err = srv.handleWorkspacesResource(ctx, req)
	if err != nil {
		t.Fatalf("캐시 폴백 호출 에러: %v", err)
	}

	text = extractTextFromResourceResult(t, contents)
	var cachedResp CachedResponse
	if err := json.Unmarshal([]byte(text), &cachedResp); err != nil {
		t.Fatalf("캐시 응답 JSON 파싱 실패: %v", err)
	}

	if !cachedResp.Cached {
		t.Error("캐시 폴백에서 Cached = false, want true")
	}
	if cachedResp.CachedAt == "" {
		t.Error("CachedAt이 비어 있습니다")
	}
	if cachedResp.Data == nil {
		t.Error("캐시된 Data가 nil입니다")
	}
}

func TestIntegration_GracefulDegradation_AgentsCache(t *testing.T) {
	t.Parallel()

	mock := newMockBackend(t, standardMockHandler(t))
	srv := newTestServer(mock.URL, 10*time.Minute)
	ctx := context.Background()

	// Step 1: 성공적인 호출
	req := makeReadResourceRequest("autopus://agents")
	_, err := srv.handleAgentsResource(ctx, req)
	if err != nil {
		t.Fatalf("첫 번째 호출 에러: %v", err)
	}

	// Step 2: 백엔드 종료
	mock.Close()

	// Step 3: 캐시 폴백 확인
	contents, err := srv.handleAgentsResource(ctx, req)
	if err != nil {
		t.Fatalf("캐시 폴백 호출 에러: %v", err)
	}

	text := extractTextFromResourceResult(t, contents)
	var cachedResp CachedResponse
	if err := json.Unmarshal([]byte(text), &cachedResp); err != nil {
		t.Fatalf("캐시 응답 JSON 파싱 실패: %v", err)
	}

	if !cachedResp.Cached {
		t.Error("캐시 폴백에서 Cached = false, want true")
	}
}

func TestIntegration_GracefulDegradation_NoCacheAvailable(t *testing.T) {
	t.Parallel()

	// 이미 종료된 서버 URL 사용 (캐시 없이 바로 실패)
	mock := newMockBackend(t, standardMockHandler(t))
	mockURL := mock.URL
	mock.Close() // 즉시 종료

	srv := newTestServer(mockURL)
	ctx := context.Background()

	// 캐시 없이 요청 - 에러 응답이지만 panic 없이 반환
	req := makeReadResourceRequest("autopus://status")
	contents, err := srv.handleStatusResource(ctx, req)
	if err != nil {
		t.Fatalf("에러 반환 기대하지만 함수 에러: %v", err)
	}

	text := extractTextFromResourceResult(t, contents)
	var status PlatformStatus
	if err := json.Unmarshal([]byte(text), &status); err != nil {
		t.Fatalf("상태 JSON 파싱 실패: %v", err)
	}

	if status.Connected {
		t.Error("Connected = true, want false (서버 미연결)")
	}
	if !strings.Contains(status.Message, "Backend unreachable") {
		t.Errorf("Message에 'Backend unreachable' 포함 기대, 실제: %q", status.Message)
	}
}

func TestIntegration_GracefulDegradation_ServerContinuesRunning(t *testing.T) {
	t.Parallel()

	mock := newMockBackend(t, standardMockHandler(t))
	srv := newTestServer(mock.URL, 10*time.Minute)
	ctx := context.Background()

	// Step 1: 정상 호출로 캐시 채우기
	statusReq := makeReadResourceRequest("autopus://status")
	_, err := srv.handleStatusResource(ctx, statusReq)
	if err != nil {
		t.Fatalf("정상 호출 에러: %v", err)
	}

	// Step 2: 서버 종료
	mock.Close()

	// Step 3: 여러 리소스를 연속 호출하여 서버가 panic 없이 동작하는지 확인
	for i := 0; i < 5; i++ {
		contents, err := srv.handleStatusResource(ctx, statusReq)
		if err != nil {
			t.Fatalf("반복 호출 #%d 에러: %v", i+1, err)
		}
		if len(contents) == 0 {
			t.Fatalf("반복 호출 #%d: 빈 응답", i+1)
		}
	}

	// Step 4: 도구 호출도 에러를 반환하지만 panic하지 않아야 함
	toolReq := makeCallToolRequest("list_agents", map[string]interface{}{})
	result, err := srv.handleListAgents(ctx, toolReq)
	if err != nil {
		t.Fatalf("도구 호출 에러: %v", err)
	}
	// 도구 호출은 에러를 반환해야 하지만, 함수 자체는 nil 에러
	if !result.IsError {
		t.Log("서버 종료 후 도구 호출이 의외로 성공했습니다 (가능한 경우)")
	}
}

// =====================================
// Scenario 13: 인증 실패 처리
// =====================================

func TestIntegration_AuthFailure_401(t *testing.T) {
	t.Parallel()

	// 401 Unauthorized만 반환하는 mock 서버
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeAPIError(w, http.StatusUnauthorized, "Invalid or expired token")
	}))
	defer mock.Close()

	srv := newTestServer(mock.URL)
	ctx := context.Background()

	tests := []struct {
		name    string
		handler func(ctx context.Context) (*mcp.CallToolResult, error)
	}{
		{
			name: "execute_task 인증 실패",
			handler: func(ctx context.Context) (*mcp.CallToolResult, error) {
				req := makeCallToolRequest("execute_task", map[string]interface{}{
					"agent_id": "agent-1",
					"prompt":   "test",
				})
				return srv.handleExecuteTask(ctx, req)
			},
		},
		{
			name: "list_agents 인증 실패",
			handler: func(ctx context.Context) (*mcp.CallToolResult, error) {
				req := makeCallToolRequest("list_agents", map[string]interface{}{})
				return srv.handleListAgents(ctx, req)
			},
		},
		{
			name: "get_execution_status 인증 실패",
			handler: func(ctx context.Context) (*mcp.CallToolResult, error) {
				req := makeCallToolRequest("get_execution_status", map[string]interface{}{
					"execution_id": "exec-001",
				})
				return srv.handleGetExecutionStatus(ctx, req)
			},
		},
		{
			name: "approve_execution 인증 실패",
			handler: func(ctx context.Context) (*mcp.CallToolResult, error) {
				req := makeCallToolRequest("approve_execution", map[string]interface{}{
					"execution_id": "exec-001",
					"decision":     "approve",
				})
				return srv.handleApproveExecution(ctx, req)
			},
		},
		{
			name: "manage_workspace 인증 실패",
			handler: func(ctx context.Context) (*mcp.CallToolResult, error) {
				req := makeCallToolRequest("manage_workspace", map[string]interface{}{
					"action": "list",
				})
				return srv.handleManageWorkspace(ctx, req)
			},
		},
		{
			name: "search_knowledge 인증 실패",
			handler: func(ctx context.Context) (*mcp.CallToolResult, error) {
				req := makeCallToolRequest("search_knowledge", map[string]interface{}{
					"query": "test",
				})
				return srv.handleSearchKnowledge(ctx, req)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.handler(ctx)
			if err != nil {
				t.Fatalf("함수 에러 (nil 기대): %v", err)
			}

			// 에러 결과여야 함
			if !result.IsError {
				t.Fatal("IsError = false, want true (인증 실패 시 에러)")
			}

			text := extractTextFromToolResult(t, result)
			if !strings.Contains(strings.ToLower(text), "invalid") &&
				!strings.Contains(strings.ToLower(text), "expired") &&
				!strings.Contains(strings.ToLower(text), "오류") &&
				!strings.Contains(strings.ToLower(text), "failed") {
				t.Errorf("에러 메시지에 인증 관련 키워드 기대, 실제: %q", text)
			}
		})
	}
}

func TestIntegration_AuthFailure_ServerContinuesRunning(t *testing.T) {
	t.Parallel()

	var callCount atomic.Int32
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := callCount.Add(1)
		if count <= 2 {
			// 처음 2번은 401
			writeAPIError(w, http.StatusUnauthorized, "Invalid token")
			return
		}
		// 그 후에는 정상 응답
		writeAPISuccess(w, ListAgentsResponse{
			Agents: []AgentInfo{{ID: "a1", Name: "Agent1"}},
			Total:  1,
		})
	}))
	defer mock.Close()

	srv := newTestServer(mock.URL)
	ctx := context.Background()

	// 첫 번째 호출: 401 에러
	req := makeCallToolRequest("list_agents", map[string]interface{}{})
	result, err := srv.handleListAgents(ctx, req)
	if err != nil {
		t.Fatalf("첫 번째 호출 함수 에러: %v", err)
	}
	if !result.IsError {
		t.Error("첫 번째 호출: IsError = false, want true")
	}

	// 두 번째 호출: 여전히 401 에러
	result, err = srv.handleListAgents(ctx, req)
	if err != nil {
		t.Fatalf("두 번째 호출 함수 에러: %v", err)
	}
	if !result.IsError {
		t.Error("두 번째 호출: IsError = false, want true")
	}

	// 세 번째 호출: 정상 응답 (서버가 살아있음을 확인)
	result, err = srv.handleListAgents(ctx, req)
	if err != nil {
		t.Fatalf("세 번째 호출 함수 에러: %v", err)
	}
	if result.IsError {
		text := extractTextFromToolResult(t, result)
		t.Fatalf("세 번째 호출 예상하지 않은 에러: %s", text)
	}
}

// =====================================
// Scenario 16: 동시 도구 호출
// =====================================

func TestIntegration_ConcurrentToolCalls(t *testing.T) {
	t.Parallel()

	mock := newMockBackend(t, standardMockHandler(t))
	defer mock.Close()

	srv := newTestServer(mock.URL)
	ctx := context.Background()

	const numGoroutines = 10

	// 다양한 도구 호출을 정의
	type toolCall struct {
		name    string
		handler func(ctx context.Context) (*mcp.CallToolResult, error)
	}

	calls := []toolCall{
		{
			name: "execute_task",
			handler: func(ctx context.Context) (*mcp.CallToolResult, error) {
				req := makeCallToolRequest("execute_task", map[string]interface{}{
					"agent_id": "agent-1",
					"prompt":   "concurrent test",
				})
				return srv.handleExecuteTask(ctx, req)
			},
		},
		{
			name: "list_agents",
			handler: func(ctx context.Context) (*mcp.CallToolResult, error) {
				req := makeCallToolRequest("list_agents", map[string]interface{}{})
				return srv.handleListAgents(ctx, req)
			},
		},
		{
			name: "get_execution_status",
			handler: func(ctx context.Context) (*mcp.CallToolResult, error) {
				req := makeCallToolRequest("get_execution_status", map[string]interface{}{
					"execution_id": "exec-001",
				})
				return srv.handleGetExecutionStatus(ctx, req)
			},
		},
		{
			name: "approve_execution",
			handler: func(ctx context.Context) (*mcp.CallToolResult, error) {
				req := makeCallToolRequest("approve_execution", map[string]interface{}{
					"execution_id": "exec-001",
					"decision":     "approve",
				})
				return srv.handleApproveExecution(ctx, req)
			},
		},
		{
			name: "manage_workspace",
			handler: func(ctx context.Context) (*mcp.CallToolResult, error) {
				req := makeCallToolRequest("manage_workspace", map[string]interface{}{
					"action": "list",
				})
				return srv.handleManageWorkspace(ctx, req)
			},
		},
		{
			name: "search_knowledge",
			handler: func(ctx context.Context) (*mcp.CallToolResult, error) {
				req := makeCallToolRequest("search_knowledge", map[string]interface{}{
					"query": "concurrent search",
				})
				return srv.handleSearchKnowledge(ctx, req)
			},
		},
	}

	var wg sync.WaitGroup
	errors := make(chan string, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			// 각 goroutine에서 다른 도구를 호출
			call := calls[index%len(calls)]
			result, err := call.handler(ctx)
			if err != nil {
				errors <- fmt.Sprintf("goroutine %d (%s): 함수 에러: %v", index, call.name, err)
				return
			}
			if result == nil {
				errors <- fmt.Sprintf("goroutine %d (%s): result가 nil", index, call.name)
				return
			}
			if result.IsError {
				text := extractTextFromToolResult(t, result)
				errors <- fmt.Sprintf("goroutine %d (%s): 예상하지 않은 에러: %s", index, call.name, text)
				return
			}
			if len(result.Content) == 0 {
				errors <- fmt.Sprintf("goroutine %d (%s): Content가 비어 있음", index, call.name)
				return
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// 에러 수집 및 보고
	var errs []string
	for errMsg := range errors {
		errs = append(errs, errMsg)
	}

	if len(errs) > 0 {
		for _, e := range errs {
			t.Error(e)
		}
	}
}

func TestIntegration_ConcurrentResourceAndToolCalls(t *testing.T) {
	t.Parallel()

	mock := newMockBackend(t, standardMockHandler(t))
	defer mock.Close()

	srv := newTestServer(mock.URL)
	ctx := context.Background()

	const numGoroutines = 10
	var wg sync.WaitGroup
	errors := make(chan string, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			switch index % 4 {
			case 0:
				// 도구 호출
				req := makeCallToolRequest("list_agents", map[string]interface{}{})
				result, err := srv.handleListAgents(ctx, req)
				if err != nil {
					errors <- fmt.Sprintf("goroutine %d: list_agents 에러: %v", index, err)
					return
				}
				if result.IsError {
					errors <- fmt.Sprintf("goroutine %d: list_agents 실패", index)
				}

			case 1:
				// status 리소스
				req := makeReadResourceRequest("autopus://status")
				contents, err := srv.handleStatusResource(ctx, req)
				if err != nil {
					errors <- fmt.Sprintf("goroutine %d: status 에러: %v", index, err)
					return
				}
				if len(contents) == 0 {
					errors <- fmt.Sprintf("goroutine %d: status 빈 응답", index)
				}

			case 2:
				// workspaces 리소스
				req := makeReadResourceRequest("autopus://workspaces")
				contents, err := srv.handleWorkspacesResource(ctx, req)
				if err != nil {
					errors <- fmt.Sprintf("goroutine %d: workspaces 에러: %v", index, err)
					return
				}
				if len(contents) == 0 {
					errors <- fmt.Sprintf("goroutine %d: workspaces 빈 응답", index)
				}

			case 3:
				// agents 리소스
				req := makeReadResourceRequest("autopus://agents")
				contents, err := srv.handleAgentsResource(ctx, req)
				if err != nil {
					errors <- fmt.Sprintf("goroutine %d: agents 에러: %v", index, err)
					return
				}
				if len(contents) == 0 {
					errors <- fmt.Sprintf("goroutine %d: agents 빈 응답", index)
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	var errs []string
	for errMsg := range errors {
		errs = append(errs, errMsg)
	}

	if len(errs) > 0 {
		for _, e := range errs {
			t.Error(e)
		}
	}
}

// TestIntegration_ConcurrentCacheAccess는 캐시의 동시 읽기/쓰기를 검증합니다.
// -race 플래그로 실행하여 데이터 레이스를 감지합니다.
func TestIntegration_ConcurrentCacheAccess(t *testing.T) {
	t.Parallel()

	mock := newMockBackend(t, standardMockHandler(t))
	srv := newTestServer(mock.URL, 10*time.Minute)
	ctx := context.Background()

	const numGoroutines = 10
	var wg sync.WaitGroup

	// 일부 goroutine은 리소스를 읽고 (캐시 Set), 일부는 캐시를 읽음
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			// 리소스 호출로 캐시 갱신
			req := makeReadResourceRequest("autopus://agents")
			_, _ = srv.handleAgentsResource(ctx, req)

			// 캐시 직접 읽기
			srv.cache.Get(cacheKeyAgents)
			srv.cache.GetStale(cacheKeyAgents)
		}(i)
	}

	wg.Wait()

	// 서버 종료 후에도 캐시가 정상 동작하는지 확인
	mock.Close()

	req := makeReadResourceRequest("autopus://agents")
	contents, err := srv.handleAgentsResource(ctx, req)
	if err != nil {
		t.Fatalf("캐시 폴백 에러: %v", err)
	}
	if len(contents) == 0 {
		t.Fatal("캐시 폴백 빈 응답")
	}
}
