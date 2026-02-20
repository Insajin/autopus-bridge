package mcpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/rs/zerolog"
)

// TestNewServer는 MCP 서버 초기화를 테스트합니다.
func TestNewServer(t *testing.T) {
	logger := zerolog.Nop()
	client := newTestClient("http://localhost:8080")
	srv := NewServer(client, logger)

	if srv == nil {
		t.Fatal("서버가 nil입니다")
	}
	if srv.mcpServer == nil {
		t.Fatal("mcpServer가 nil입니다")
	}
	if srv.client == nil {
		t.Fatal("client가 nil입니다")
	}
}

// TestToolHandler_ExecuteTask_MissingParams는 필수 파라미터 누락을 테스트합니다.
func TestToolHandler_ExecuteTask_MissingParams(t *testing.T) {
	logger := zerolog.Nop()
	client := newTestClient("http://localhost:1")
	srv := NewServer(client, logger)

	// agent_id 누락
	req := mcp.CallToolRequest{}
	req.Params.Name = "execute_task"
	req.Params.Arguments = map[string]interface{}{
		"prompt": "test prompt",
	}

	result, err := srv.handleExecuteTask(context.Background(), req)
	if err != nil {
		t.Fatalf("핸들러가 에러를 반환하면 안됩니다: %v", err)
	}
	if result == nil {
		t.Fatal("결과가 nil입니다")
	}
	// 에러 응답인지 확인 (IsError 필드)
	if !result.IsError {
		t.Error("필수 파라미터 누락 시 에러 응답이어야 합니다")
	}
}

// TestToolHandler_ExecuteTask_Success는 정상 태스크 실행을 테스트합니다.
func TestToolHandler_ExecuteTask_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := apiResponse{
			Success: true,
			Data:    json.RawMessage(`{"execution_id":"exec-001","status":"running"}`),
		}
		json.NewEncoder(w).Encode(resp)
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	logger := zerolog.Nop()
	client := newTestClient(server.URL)
	srv := NewServer(client, logger)

	req := mcp.CallToolRequest{}
	req.Params.Name = "execute_task"
	req.Params.Arguments = map[string]interface{}{
		"agent_id": "agent-001",
		"prompt":   "Hello agent",
	}

	result, err := srv.handleExecuteTask(context.Background(), req)
	if err != nil {
		t.Fatalf("핸들러 오류: %v", err)
	}
	if result.IsError {
		t.Errorf("성공 응답이어야 합니다")
	}
}

// TestToolHandler_ListAgents는 에이전트 목록 조회를 테스트합니다.
func TestToolHandler_ListAgents(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := apiResponse{
			Success: true,
			Data:    json.RawMessage(`{"agents":[{"id":"agent-001","name":"Test"}],"total":1}`),
		}
		json.NewEncoder(w).Encode(resp)
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	logger := zerolog.Nop()
	client := newTestClient(server.URL)
	srv := NewServer(client, logger)

	req := mcp.CallToolRequest{}
	req.Params.Name = "list_agents"
	req.Params.Arguments = map[string]interface{}{}

	result, err := srv.handleListAgents(context.Background(), req)
	if err != nil {
		t.Fatalf("핸들러 오류: %v", err)
	}
	if result.IsError {
		t.Errorf("성공 응답이어야 합니다")
	}
}

// TestToolHandler_GetExecutionStatus_MissingID는 execution_id 누락을 테스트합니다.
func TestToolHandler_GetExecutionStatus_MissingID(t *testing.T) {
	logger := zerolog.Nop()
	client := newTestClient("http://localhost:1")
	srv := NewServer(client, logger)

	req := mcp.CallToolRequest{}
	req.Params.Name = "get_execution_status"
	req.Params.Arguments = map[string]interface{}{}

	result, err := srv.handleGetExecutionStatus(context.Background(), req)
	if err != nil {
		t.Fatalf("핸들러가 에러를 반환하면 안됩니다: %v", err)
	}
	if !result.IsError {
		t.Error("필수 파라미터 누락 시 에러 응답이어야 합니다")
	}
}

// TestToolHandler_ApproveExecution_InvalidDecision은 유효하지 않은 decision을 테스트합니다.
func TestToolHandler_ApproveExecution_InvalidDecision(t *testing.T) {
	logger := zerolog.Nop()
	client := newTestClient("http://localhost:1")
	srv := NewServer(client, logger)

	req := mcp.CallToolRequest{}
	req.Params.Name = "approve_execution"
	req.Params.Arguments = map[string]interface{}{
		"execution_id": "exec-001",
		"decision":     "maybe",
	}

	result, err := srv.handleApproveExecution(context.Background(), req)
	if err != nil {
		t.Fatalf("핸들러가 에러를 반환하면 안됩니다: %v", err)
	}
	if !result.IsError {
		t.Error("유효하지 않은 decision 시 에러 응답이어야 합니다")
	}
}

// TestToolHandler_ManageWorkspace_RequiresWorkspaceID는 update/delete에 workspace_id 필수를 테스트합니다.
func TestToolHandler_ManageWorkspace_RequiresWorkspaceID(t *testing.T) {
	logger := zerolog.Nop()
	client := newTestClient("http://localhost:1")
	srv := NewServer(client, logger)

	for _, action := range []string{"update", "delete"} {
		t.Run(action, func(t *testing.T) {
			req := mcp.CallToolRequest{}
			req.Params.Name = "manage_workspace"
			req.Params.Arguments = map[string]interface{}{
				"action": action,
			}

			result, err := srv.handleManageWorkspace(context.Background(), req)
			if err != nil {
				t.Fatalf("핸들러가 에러를 반환하면 안됩니다: %v", err)
			}
			if !result.IsError {
				t.Errorf("%s 액션에 workspace_id 누락 시 에러 응답이어야 합니다", action)
			}
		})
	}
}

// TestToolHandler_ManageWorkspace_InvalidAction은 유효하지 않은 액션을 테스트합니다.
func TestToolHandler_ManageWorkspace_InvalidAction(t *testing.T) {
	logger := zerolog.Nop()
	client := newTestClient("http://localhost:1")
	srv := NewServer(client, logger)

	req := mcp.CallToolRequest{}
	req.Params.Name = "manage_workspace"
	req.Params.Arguments = map[string]interface{}{
		"action": "explode",
	}

	result, err := srv.handleManageWorkspace(context.Background(), req)
	if err != nil {
		t.Fatalf("핸들러가 에러를 반환하면 안됩니다: %v", err)
	}
	if !result.IsError {
		t.Error("유효하지 않은 액션 시 에러 응답이어야 합니다")
	}
}

// TestToolHandler_SearchKnowledge_MissingQuery는 query 누락을 테스트합니다.
func TestToolHandler_SearchKnowledge_MissingQuery(t *testing.T) {
	logger := zerolog.Nop()
	client := newTestClient("http://localhost:1")
	srv := NewServer(client, logger)

	req := mcp.CallToolRequest{}
	req.Params.Name = "search_knowledge"
	req.Params.Arguments = map[string]interface{}{}

	result, err := srv.handleSearchKnowledge(context.Background(), req)
	if err != nil {
		t.Fatalf("핸들러가 에러를 반환하면 안됩니다: %v", err)
	}
	if !result.IsError {
		t.Error("필수 파라미터 누락 시 에러 응답이어야 합니다")
	}
}

// TestToolHandler_SearchKnowledge_LimitClamping은 limit 범위 제한을 테스트합니다.
func TestToolHandler_SearchKnowledge_LimitClamping(t *testing.T) {
	var receivedLimit int
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req SearchKnowledgeRequest
		json.NewDecoder(r.Body).Decode(&req)
		receivedLimit = req.Limit

		resp := apiResponse{
			Success: true,
			Data:    json.RawMessage(`{"results":[],"total":0,"query":"test"}`),
		}
		json.NewEncoder(w).Encode(resp)
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	logger := zerolog.Nop()
	client := newTestClient(server.URL)
	srv := NewServer(client, logger)

	// limit > 50인 경우 50으로 클램핑
	req := mcp.CallToolRequest{}
	req.Params.Name = "search_knowledge"
	req.Params.Arguments = map[string]interface{}{
		"query": "test",
		"limit": float64(100),
	}

	_, err := srv.handleSearchKnowledge(context.Background(), req)
	if err != nil {
		t.Fatalf("핸들러 오류: %v", err)
	}
	if receivedLimit != 50 {
		t.Errorf("limit가 50으로 클램핑되어야 합니다, 실제: %d", receivedLimit)
	}
}

// TestResourceHandler_Status_BackendUnreachable는 백엔드 미연결 시 graceful degradation을 테스트합니다.
func TestResourceHandler_Status_BackendUnreachable(t *testing.T) {
	logger := zerolog.Nop()
	client := NewBackendClient("http://localhost:1", mockTokenRefresher(), 1*time.Second, logger)
	srv := NewServer(client, logger)

	req := mcp.ReadResourceRequest{}
	req.Params.URI = "autopus://status"

	contents, err := srv.handleStatusResource(context.Background(), req)
	if err != nil {
		t.Fatalf("리소스 핸들러가 에러를 반환하면 안됩니다: %v", err)
	}
	if len(contents) != 1 {
		t.Fatalf("콘텐츠 수 1이어야 합니다, 실제: %d", len(contents))
	}

	// 응답을 파싱하여 connected=false 확인
	textContent, ok := contents[0].(mcp.TextResourceContents)
	if !ok {
		t.Fatal("TextResourceContents여야 합니다")
	}

	var status PlatformStatus
	if err := json.Unmarshal([]byte(textContent.Text), &status); err != nil {
		t.Fatalf("상태 파싱 실패: %v", err)
	}
	if status.Connected {
		t.Error("백엔드 미연결 시 connected=false이어야 합니다")
	}
	if status.Message == "" {
		t.Error("에러 메시지가 있어야 합니다")
	}
}

// TestResourceHandler_Status_Connected는 백엔드 연결 성공을 테스트합니다.
func TestResourceHandler_Status_Connected(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := apiResponse{
			Success: true,
			Data:    json.RawMessage(`{"agents":[],"total":0}`),
		}
		json.NewEncoder(w).Encode(resp)
	})
	mockServer := httptest.NewServer(handler)
	defer mockServer.Close()

	logger := zerolog.Nop()
	client := newTestClient(mockServer.URL)
	srv := NewServer(client, logger)

	req := mcp.ReadResourceRequest{}
	req.Params.URI = "autopus://status"

	contents, err := srv.handleStatusResource(context.Background(), req)
	if err != nil {
		t.Fatalf("리소스 핸들러 오류: %v", err)
	}

	textContent, ok := contents[0].(mcp.TextResourceContents)
	if !ok {
		t.Fatal("TextResourceContents여야 합니다")
	}

	var status PlatformStatus
	json.Unmarshal([]byte(textContent.Text), &status)
	if !status.Connected {
		t.Error("연결 성공 시 connected=true이어야 합니다")
	}
}

// TestResourceHandler_Execution_InvalidURI는 유효하지 않은 execution URI를 테스트합니다.
func TestResourceHandler_Execution_InvalidURI(t *testing.T) {
	logger := zerolog.Nop()
	client := newTestClient("http://localhost:1")
	srv := NewServer(client, logger)

	req := mcp.ReadResourceRequest{}
	req.Params.URI = "autopus://invalid/path"

	_, err := srv.handleExecutionResource(context.Background(), req)
	if err == nil {
		t.Error("유효하지 않은 URI 시 에러가 반환되어야 합니다")
	}
}

// TestResourceHandler_Workspaces_GracefulDegradation은 워크스페이스 리소스의 graceful degradation을 테스트합니다.
func TestResourceHandler_Workspaces_GracefulDegradation(t *testing.T) {
	logger := zerolog.Nop()
	client := NewBackendClient("http://localhost:1", mockTokenRefresher(), 1*time.Second, logger)
	srv := NewServer(client, logger)

	req := mcp.ReadResourceRequest{}
	req.Params.URI = "autopus://workspaces"

	contents, err := srv.handleWorkspacesResource(context.Background(), req)
	if err != nil {
		t.Fatalf("graceful degradation 시 에러를 반환하면 안됩니다: %v", err)
	}
	if len(contents) != 1 {
		t.Fatalf("콘텐츠가 반환되어야 합니다, 실제: %d", len(contents))
	}

	textContent, ok := contents[0].(mcp.TextResourceContents)
	if !ok {
		t.Fatal("TextResourceContents여야 합니다")
	}

	var resp map[string]interface{}
	json.Unmarshal([]byte(textContent.Text), &resp)
	if _, hasError := resp["error"]; !hasError {
		t.Error("에러 정보가 포함되어야 합니다")
	}
}

// TestResourceHandler_Agents_GracefulDegradation은 에이전트 리소스의 graceful degradation을 테스트합니다.
func TestResourceHandler_Agents_GracefulDegradation(t *testing.T) {
	logger := zerolog.Nop()
	client := NewBackendClient("http://localhost:1", mockTokenRefresher(), 1*time.Second, logger)
	srv := NewServer(client, logger)

	req := mcp.ReadResourceRequest{}
	req.Params.URI = "autopus://agents"

	contents, err := srv.handleAgentsResource(context.Background(), req)
	if err != nil {
		t.Fatalf("graceful degradation 시 에러를 반환하면 안됩니다: %v", err)
	}
	if len(contents) != 1 {
		t.Fatalf("콘텐츠가 반환되어야 합니다, 실제: %d", len(contents))
	}
}

// --- 리소스 핸들러 성공 경로 테스트 ---

// TestResourceHandler_Execution_Success는 실행 상세 리소스의 성공 경로를 테스트합니다.
func TestResourceHandler_Execution_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/executions/exec-test-001" {
			t.Errorf("예상 경로 /api/v1/executions/exec-test-001, 실제: %s", r.URL.Path)
		}

		resp := apiResponse{
			Success: true,
			Data:    json.RawMessage(`{"execution_id":"exec-test-001","status":"completed","result":{"output":"done"}}`),
		}
		json.NewEncoder(w).Encode(resp)
	})
	mockServer := httptest.NewServer(handler)
	defer mockServer.Close()

	logger := zerolog.Nop()
	client := newTestClient(mockServer.URL)
	srv := NewServer(client, logger)

	req := mcp.ReadResourceRequest{}
	req.Params.URI = "autopus://executions/exec-test-001"

	contents, err := srv.handleExecutionResource(context.Background(), req)
	if err != nil {
		t.Fatalf("리소스 핸들러 오류: %v", err)
	}
	if len(contents) != 1 {
		t.Fatalf("콘텐츠 수 1이어야 합니다, 실제: %d", len(contents))
	}

	textContent, ok := contents[0].(mcp.TextResourceContents)
	if !ok {
		t.Fatal("TextResourceContents여야 합니다")
	}

	var status ExecutionStatus
	if err := json.Unmarshal([]byte(textContent.Text), &status); err != nil {
		t.Fatalf("실행 상태 파싱 실패: %v", err)
	}
	if status.ExecutionID != "exec-test-001" {
		t.Errorf("예상 execution_id exec-test-001, 실제: %s", status.ExecutionID)
	}
	if status.Status != "completed" {
		t.Errorf("예상 status completed, 실제: %s", status.Status)
	}
}

// TestResourceHandler_Execution_BackendError는 실행 상세 리소스의 백엔드 에러 시 graceful degradation을 테스트합니다.
func TestResourceHandler_Execution_BackendError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		resp := apiResponse{
			Success: false,
			Error:   "Execution not found",
		}
		json.NewEncoder(w).Encode(resp)
	})
	mockServer := httptest.NewServer(handler)
	defer mockServer.Close()

	logger := zerolog.Nop()
	client := newTestClient(mockServer.URL)
	srv := NewServer(client, logger)

	req := mcp.ReadResourceRequest{}
	req.Params.URI = "autopus://executions/exec-nonexistent"

	contents, err := srv.handleExecutionResource(context.Background(), req)
	if err != nil {
		t.Fatalf("graceful degradation 시 에러를 반환하면 안됩니다: %v", err)
	}
	if len(contents) != 1 {
		t.Fatalf("콘텐츠가 반환되어야 합니다, 실제: %d", len(contents))
	}

	textContent, ok := contents[0].(mcp.TextResourceContents)
	if !ok {
		t.Fatal("TextResourceContents여야 합니다")
	}

	var resp map[string]string
	json.Unmarshal([]byte(textContent.Text), &resp)
	if _, hasError := resp["error"]; !hasError {
		t.Error("에러 정보가 포함되어야 합니다")
	}
	if resp["execution_id"] != "exec-nonexistent" {
		t.Errorf("execution_id가 포함되어야 합니다, 실제: %s", resp["execution_id"])
	}
}

// TestResourceHandler_Workspaces_Success는 워크스페이스 목록 리소스의 성공 경로를 테스트합니다.
func TestResourceHandler_Workspaces_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := apiResponse{
			Success: true,
			Data:    json.RawMessage(`{"workspaces":[{"id":"ws-001","name":"Test WS"},{"id":"ws-002","name":"Prod WS"}]}`),
		}
		json.NewEncoder(w).Encode(resp)
	})
	mockServer := httptest.NewServer(handler)
	defer mockServer.Close()

	logger := zerolog.Nop()
	client := newTestClient(mockServer.URL)
	srv := NewServer(client, logger)

	req := mcp.ReadResourceRequest{}
	req.Params.URI = "autopus://workspaces"

	contents, err := srv.handleWorkspacesResource(context.Background(), req)
	if err != nil {
		t.Fatalf("리소스 핸들러 오류: %v", err)
	}
	if len(contents) != 1 {
		t.Fatalf("콘텐츠 수 1이어야 합니다, 실제: %d", len(contents))
	}

	textContent, ok := contents[0].(mcp.TextResourceContents)
	if !ok {
		t.Fatal("TextResourceContents여야 합니다")
	}

	var resp ManageWorkspaceResponse
	if err := json.Unmarshal([]byte(textContent.Text), &resp); err != nil {
		t.Fatalf("워크스페이스 응답 파싱 실패: %v", err)
	}
	if len(resp.Workspaces) != 2 {
		t.Errorf("예상 워크스페이스 수 2, 실제: %d", len(resp.Workspaces))
	}
}

// TestResourceHandler_Agents_Success는 에이전트 카탈로그 리소스의 성공 경로를 테스트합니다.
func TestResourceHandler_Agents_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := apiResponse{
			Success: true,
			Data:    json.RawMessage(`{"agents":[{"id":"agent-001","name":"Agent A"},{"id":"agent-002","name":"Agent B"}],"total":2}`),
		}
		json.NewEncoder(w).Encode(resp)
	})
	mockServer := httptest.NewServer(handler)
	defer mockServer.Close()

	logger := zerolog.Nop()
	client := newTestClient(mockServer.URL)
	srv := NewServer(client, logger)

	req := mcp.ReadResourceRequest{}
	req.Params.URI = "autopus://agents"

	contents, err := srv.handleAgentsResource(context.Background(), req)
	if err != nil {
		t.Fatalf("리소스 핸들러 오류: %v", err)
	}
	if len(contents) != 1 {
		t.Fatalf("콘텐츠 수 1이어야 합니다, 실제: %d", len(contents))
	}

	textContent, ok := contents[0].(mcp.TextResourceContents)
	if !ok {
		t.Fatal("TextResourceContents여야 합니다")
	}

	var resp ListAgentsResponse
	if err := json.Unmarshal([]byte(textContent.Text), &resp); err != nil {
		t.Fatalf("에이전트 응답 파싱 실패: %v", err)
	}
	if resp.Total != 2 {
		t.Errorf("예상 total 2, 실제: %d", resp.Total)
	}
	if len(resp.Agents) != 2 {
		t.Errorf("예상 에이전트 수 2, 실제: %d", len(resp.Agents))
	}
}

// --- 도구 핸들러 성공 경로 테스트 ---

// TestToolHandler_GetExecutionStatus_Success는 실행 상태 조회 성공을 테스트합니다.
func TestToolHandler_GetExecutionStatus_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := apiResponse{
			Success: true,
			Data:    json.RawMessage(`{"execution_id":"exec-001","status":"completed","result":{"output":"success"}}`),
		}
		json.NewEncoder(w).Encode(resp)
	})
	mockServer := httptest.NewServer(handler)
	defer mockServer.Close()

	logger := zerolog.Nop()
	client := newTestClient(mockServer.URL)
	srv := NewServer(client, logger)

	req := mcp.CallToolRequest{}
	req.Params.Name = "get_execution_status"
	req.Params.Arguments = map[string]interface{}{
		"execution_id": "exec-001",
	}

	result, err := srv.handleGetExecutionStatus(context.Background(), req)
	if err != nil {
		t.Fatalf("핸들러 오류: %v", err)
	}
	if result.IsError {
		t.Error("성공 응답이어야 합니다")
	}
}

// TestToolHandler_GetExecutionStatus_BackendError는 백엔드 오류 시 에러 응답을 테스트합니다.
func TestToolHandler_GetExecutionStatus_BackendError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	})
	mockServer := httptest.NewServer(handler)
	defer mockServer.Close()

	logger := zerolog.Nop()
	client := newTestClient(mockServer.URL)
	srv := NewServer(client, logger)

	req := mcp.CallToolRequest{}
	req.Params.Name = "get_execution_status"
	req.Params.Arguments = map[string]interface{}{
		"execution_id": "exec-001",
	}

	result, err := srv.handleGetExecutionStatus(context.Background(), req)
	if err != nil {
		t.Fatalf("핸들러가 에러를 반환하면 안됩니다: %v", err)
	}
	if !result.IsError {
		t.Error("백엔드 오류 시 에러 응답이어야 합니다")
	}
}

// TestToolHandler_ApproveExecution_Success는 실행 승인 성공을 테스트합니다.
func TestToolHandler_ApproveExecution_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := apiResponse{
			Success: true,
			Data:    json.RawMessage(`{"execution_id":"exec-001","status":"approved","message":"Approved successfully"}`),
		}
		json.NewEncoder(w).Encode(resp)
	})
	mockServer := httptest.NewServer(handler)
	defer mockServer.Close()

	logger := zerolog.Nop()
	client := newTestClient(mockServer.URL)
	srv := NewServer(client, logger)

	req := mcp.CallToolRequest{}
	req.Params.Name = "approve_execution"
	req.Params.Arguments = map[string]interface{}{
		"execution_id": "exec-001",
		"decision":     "approve",
		"reason":       "Looks good",
	}

	result, err := srv.handleApproveExecution(context.Background(), req)
	if err != nil {
		t.Fatalf("핸들러 오류: %v", err)
	}
	if result.IsError {
		t.Error("성공 응답이어야 합니다")
	}
}

// TestToolHandler_ApproveExecution_Reject는 실행 거부 성공을 테스트합니다.
func TestToolHandler_ApproveExecution_Reject(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body ApproveExecutionRequest
		json.NewDecoder(r.Body).Decode(&body)

		if body.Decision != "reject" {
			t.Errorf("예상 decision reject, 실제: %s", body.Decision)
		}

		resp := apiResponse{
			Success: true,
			Data:    json.RawMessage(`{"execution_id":"exec-001","status":"rejected"}`),
		}
		json.NewEncoder(w).Encode(resp)
	})
	mockServer := httptest.NewServer(handler)
	defer mockServer.Close()

	logger := zerolog.Nop()
	client := newTestClient(mockServer.URL)
	srv := NewServer(client, logger)

	req := mcp.CallToolRequest{}
	req.Params.Name = "approve_execution"
	req.Params.Arguments = map[string]interface{}{
		"execution_id": "exec-001",
		"decision":     "reject",
		"reason":       "Not ready",
	}

	result, err := srv.handleApproveExecution(context.Background(), req)
	if err != nil {
		t.Fatalf("핸들러 오류: %v", err)
	}
	if result.IsError {
		t.Error("성공 응답이어야 합니다")
	}
}

// TestToolHandler_ApproveExecution_MissingExecutionID는 execution_id 누락을 테스트합니다.
func TestToolHandler_ApproveExecution_MissingExecutionID(t *testing.T) {
	logger := zerolog.Nop()
	client := newTestClient("http://localhost:1")
	srv := NewServer(client, logger)

	req := mcp.CallToolRequest{}
	req.Params.Name = "approve_execution"
	req.Params.Arguments = map[string]interface{}{
		"decision": "approve",
	}

	result, err := srv.handleApproveExecution(context.Background(), req)
	if err != nil {
		t.Fatalf("핸들러가 에러를 반환하면 안됩니다: %v", err)
	}
	if !result.IsError {
		t.Error("execution_id 누락 시 에러 응답이어야 합니다")
	}
}

// TestToolHandler_ApproveExecution_MissingDecision은 decision 누락을 테스트합니다.
func TestToolHandler_ApproveExecution_MissingDecision(t *testing.T) {
	logger := zerolog.Nop()
	client := newTestClient("http://localhost:1")
	srv := NewServer(client, logger)

	req := mcp.CallToolRequest{}
	req.Params.Name = "approve_execution"
	req.Params.Arguments = map[string]interface{}{
		"execution_id": "exec-001",
	}

	result, err := srv.handleApproveExecution(context.Background(), req)
	if err != nil {
		t.Fatalf("핸들러가 에러를 반환하면 안됩니다: %v", err)
	}
	if !result.IsError {
		t.Error("decision 누락 시 에러 응답이어야 합니다")
	}
}

// TestToolHandler_ApproveExecution_BackendError는 승인 시 백엔드 오류를 테스트합니다.
func TestToolHandler_ApproveExecution_BackendError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	})
	mockServer := httptest.NewServer(handler)
	defer mockServer.Close()

	logger := zerolog.Nop()
	client := newTestClient(mockServer.URL)
	srv := NewServer(client, logger)

	req := mcp.CallToolRequest{}
	req.Params.Name = "approve_execution"
	req.Params.Arguments = map[string]interface{}{
		"execution_id": "exec-001",
		"decision":     "approve",
	}

	result, err := srv.handleApproveExecution(context.Background(), req)
	if err != nil {
		t.Fatalf("핸들러가 에러를 반환하면 안됩니다: %v", err)
	}
	if !result.IsError {
		t.Error("백엔드 오류 시 에러 응답이어야 합니다")
	}
}

// TestToolHandler_ManageWorkspace_ListSuccess는 워크스페이스 목록 조회 성공을 테스트합니다.
func TestToolHandler_ManageWorkspace_ListSuccess(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := apiResponse{
			Success: true,
			Data:    json.RawMessage(`{"workspaces":[{"id":"ws-001","name":"Test"}]}`),
		}
		json.NewEncoder(w).Encode(resp)
	})
	mockServer := httptest.NewServer(handler)
	defer mockServer.Close()

	logger := zerolog.Nop()
	client := newTestClient(mockServer.URL)
	srv := NewServer(client, logger)

	req := mcp.CallToolRequest{}
	req.Params.Name = "manage_workspace"
	req.Params.Arguments = map[string]interface{}{
		"action": "list",
	}

	result, err := srv.handleManageWorkspace(context.Background(), req)
	if err != nil {
		t.Fatalf("핸들러 오류: %v", err)
	}
	if result.IsError {
		t.Error("성공 응답이어야 합니다")
	}
}

// TestToolHandler_ManageWorkspace_CreateSuccess는 워크스페이스 생성 성공을 테스트합니다.
func TestToolHandler_ManageWorkspace_CreateSuccess(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := apiResponse{
			Success: true,
			Data:    json.RawMessage(`{"workspace":{"id":"ws-new","name":"New"},"message":"created"}`),
		}
		json.NewEncoder(w).Encode(resp)
	})
	mockServer := httptest.NewServer(handler)
	defer mockServer.Close()

	logger := zerolog.Nop()
	client := newTestClient(mockServer.URL)
	srv := NewServer(client, logger)

	req := mcp.CallToolRequest{}
	req.Params.Name = "manage_workspace"
	req.Params.Arguments = map[string]interface{}{
		"action": "create",
	}

	result, err := srv.handleManageWorkspace(context.Background(), req)
	if err != nil {
		t.Fatalf("핸들러 오류: %v", err)
	}
	if result.IsError {
		t.Error("성공 응답이어야 합니다")
	}
}

// TestToolHandler_ManageWorkspace_DeleteSuccess는 워크스페이스 삭제 성공을 테스트합니다.
func TestToolHandler_ManageWorkspace_DeleteSuccess(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := apiResponse{
			Success: true,
			Data:    json.RawMessage(`{"message":"deleted"}`),
		}
		json.NewEncoder(w).Encode(resp)
	})
	mockServer := httptest.NewServer(handler)
	defer mockServer.Close()

	logger := zerolog.Nop()
	client := newTestClient(mockServer.URL)
	srv := NewServer(client, logger)

	req := mcp.CallToolRequest{}
	req.Params.Name = "manage_workspace"
	req.Params.Arguments = map[string]interface{}{
		"action":       "delete",
		"workspace_id": "ws-001",
	}

	result, err := srv.handleManageWorkspace(context.Background(), req)
	if err != nil {
		t.Fatalf("핸들러 오류: %v", err)
	}
	if result.IsError {
		t.Error("성공 응답이어야 합니다")
	}
}

// TestToolHandler_ManageWorkspace_MissingAction은 action 누락을 테스트합니다.
func TestToolHandler_ManageWorkspace_MissingAction(t *testing.T) {
	logger := zerolog.Nop()
	client := newTestClient("http://localhost:1")
	srv := NewServer(client, logger)

	req := mcp.CallToolRequest{}
	req.Params.Name = "manage_workspace"
	req.Params.Arguments = map[string]interface{}{}

	result, err := srv.handleManageWorkspace(context.Background(), req)
	if err != nil {
		t.Fatalf("핸들러가 에러를 반환하면 안됩니다: %v", err)
	}
	if !result.IsError {
		t.Error("action 누락 시 에러 응답이어야 합니다")
	}
}

// TestToolHandler_ManageWorkspace_BackendError는 워크스페이스 관리 시 백엔드 오류를 테스트합니다.
func TestToolHandler_ManageWorkspace_BackendError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	})
	mockServer := httptest.NewServer(handler)
	defer mockServer.Close()

	logger := zerolog.Nop()
	client := newTestClient(mockServer.URL)
	srv := NewServer(client, logger)

	req := mcp.CallToolRequest{}
	req.Params.Name = "manage_workspace"
	req.Params.Arguments = map[string]interface{}{
		"action": "list",
	}

	result, err := srv.handleManageWorkspace(context.Background(), req)
	if err != nil {
		t.Fatalf("핸들러가 에러를 반환하면 안됩니다: %v", err)
	}
	if !result.IsError {
		t.Error("백엔드 오류 시 에러 응답이어야 합니다")
	}
}

// TestToolHandler_ExecuteTask_MissingPrompt는 prompt 누락을 테스트합니다.
func TestToolHandler_ExecuteTask_MissingPrompt(t *testing.T) {
	logger := zerolog.Nop()
	client := newTestClient("http://localhost:1")
	srv := NewServer(client, logger)

	req := mcp.CallToolRequest{}
	req.Params.Name = "execute_task"
	req.Params.Arguments = map[string]interface{}{
		"agent_id": "agent-001",
	}

	result, err := srv.handleExecuteTask(context.Background(), req)
	if err != nil {
		t.Fatalf("핸들러가 에러를 반환하면 안됩니다: %v", err)
	}
	if !result.IsError {
		t.Error("prompt 누락 시 에러 응답이어야 합니다")
	}
}

// TestToolHandler_ExecuteTask_BackendError는 태스크 실행 시 백엔드 오류를 테스트합니다.
func TestToolHandler_ExecuteTask_BackendError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	})
	mockServer := httptest.NewServer(handler)
	defer mockServer.Close()

	logger := zerolog.Nop()
	client := newTestClient(mockServer.URL)
	srv := NewServer(client, logger)

	req := mcp.CallToolRequest{}
	req.Params.Name = "execute_task"
	req.Params.Arguments = map[string]interface{}{
		"agent_id": "agent-001",
		"prompt":   "test",
	}

	result, err := srv.handleExecuteTask(context.Background(), req)
	if err != nil {
		t.Fatalf("핸들러가 에러를 반환하면 안됩니다: %v", err)
	}
	if !result.IsError {
		t.Error("백엔드 오류 시 에러 응답이어야 합니다")
	}
}

// TestToolHandler_ListAgents_BackendError는 에이전트 목록 조회 시 백엔드 오류를 테스트합니다.
func TestToolHandler_ListAgents_BackendError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	})
	mockServer := httptest.NewServer(handler)
	defer mockServer.Close()

	logger := zerolog.Nop()
	client := newTestClient(mockServer.URL)
	srv := NewServer(client, logger)

	req := mcp.CallToolRequest{}
	req.Params.Name = "list_agents"
	req.Params.Arguments = map[string]interface{}{}

	result, err := srv.handleListAgents(context.Background(), req)
	if err != nil {
		t.Fatalf("핸들러가 에러를 반환하면 안됩니다: %v", err)
	}
	if !result.IsError {
		t.Error("백엔드 오류 시 에러 응답이어야 합니다")
	}
}

// TestToolHandler_SearchKnowledge_Success는 지식 검색 성공을 테스트합니다.
func TestToolHandler_SearchKnowledge_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := apiResponse{
			Success: true,
			Data:    json.RawMessage(`{"results":[{"id":"doc-001","title":"Doc","content":"content","score":0.9}],"total":1,"query":"test"}`),
		}
		json.NewEncoder(w).Encode(resp)
	})
	mockServer := httptest.NewServer(handler)
	defer mockServer.Close()

	logger := zerolog.Nop()
	client := newTestClient(mockServer.URL)
	srv := NewServer(client, logger)

	req := mcp.CallToolRequest{}
	req.Params.Name = "search_knowledge"
	req.Params.Arguments = map[string]interface{}{
		"query": "test knowledge",
	}

	result, err := srv.handleSearchKnowledge(context.Background(), req)
	if err != nil {
		t.Fatalf("핸들러 오류: %v", err)
	}
	if result.IsError {
		t.Error("성공 응답이어야 합니다")
	}
}

// TestToolHandler_SearchKnowledge_BackendError는 지식 검색 시 백엔드 오류를 테스트합니다.
func TestToolHandler_SearchKnowledge_BackendError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	})
	mockServer := httptest.NewServer(handler)
	defer mockServer.Close()

	logger := zerolog.Nop()
	client := newTestClient(mockServer.URL)
	srv := NewServer(client, logger)

	req := mcp.CallToolRequest{}
	req.Params.Name = "search_knowledge"
	req.Params.Arguments = map[string]interface{}{
		"query": "test",
	}

	result, err := srv.handleSearchKnowledge(context.Background(), req)
	if err != nil {
		t.Fatalf("핸들러가 에러를 반환하면 안됩니다: %v", err)
	}
	if !result.IsError {
		t.Error("백엔드 오류 시 에러 응답이어야 합니다")
	}
}

// TestToolHandler_SearchKnowledge_NegativeLimit은 limit가 0이하일 때 기본값 10으로 설정되는지 테스트합니다.
func TestToolHandler_SearchKnowledge_NegativeLimit(t *testing.T) {
	var receivedLimit int
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req SearchKnowledgeRequest
		json.NewDecoder(r.Body).Decode(&req)
		receivedLimit = req.Limit

		resp := apiResponse{
			Success: true,
			Data:    json.RawMessage(`{"results":[],"total":0,"query":"test"}`),
		}
		json.NewEncoder(w).Encode(resp)
	})
	mockServer := httptest.NewServer(handler)
	defer mockServer.Close()

	logger := zerolog.Nop()
	client := newTestClient(mockServer.URL)
	srv := NewServer(client, logger)

	req := mcp.CallToolRequest{}
	req.Params.Name = "search_knowledge"
	req.Params.Arguments = map[string]interface{}{
		"query": "test",
		"limit": float64(-5),
	}

	_, err := srv.handleSearchKnowledge(context.Background(), req)
	if err != nil {
		t.Fatalf("핸들러 오류: %v", err)
	}
	if receivedLimit != 10 {
		t.Errorf("음수 limit는 10으로 설정되어야 합니다, 실제: %d", receivedLimit)
	}
}

// TestToolHandler_ExecuteTask_WithOptionalParams는 선택적 파라미터 전달을 테스트합니다.
func TestToolHandler_ExecuteTask_WithOptionalParams(t *testing.T) {
	var receivedReq ExecuteTaskRequest
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedReq)

		resp := apiResponse{
			Success: true,
			Data:    json.RawMessage(`{"execution_id":"exec-002","status":"running"}`),
		}
		json.NewEncoder(w).Encode(resp)
	})
	mockServer := httptest.NewServer(handler)
	defer mockServer.Close()

	logger := zerolog.Nop()
	client := newTestClient(mockServer.URL)
	srv := NewServer(client, logger)

	req := mcp.CallToolRequest{}
	req.Params.Name = "execute_task"
	req.Params.Arguments = map[string]interface{}{
		"agent_id":     "agent-001",
		"prompt":       "Hello",
		"workspace_id": "ws-test",
		"model":        "gpt-4",
	}

	result, err := srv.handleExecuteTask(context.Background(), req)
	if err != nil {
		t.Fatalf("핸들러 오류: %v", err)
	}
	if result.IsError {
		t.Error("성공 응답이어야 합니다")
	}
	if receivedReq.WorkspaceID != "ws-test" {
		t.Errorf("예상 workspace_id ws-test, 실제: %s", receivedReq.WorkspaceID)
	}
	if receivedReq.Model != "gpt-4" {
		t.Errorf("예상 model gpt-4, 실제: %s", receivedReq.Model)
	}
}
