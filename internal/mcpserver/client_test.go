package mcpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/insajin/autopus-bridge/internal/auth"
	"github.com/rs/zerolog"
)

// mockTokenRefresher는 테스트용 TokenRefresher를 생성합니다.
// 실제 auth.TokenRefresher를 유효한 토큰으로 초기화합니다.
func mockTokenRefresher() *auth.TokenRefresher {
	creds := &auth.Credentials{
		AccessToken:  "test-access-token-valid",
		RefreshToken: "test-refresh-token",
		ExpiresAt:    time.Now().Add(1 * time.Hour),
		ServerURL:    "wss://test.autopus.co/ws/agent",
		UserEmail:    "test@autopus.co",
		WorkspaceID:  "ws-test-001",
	}
	return auth.NewTokenRefresher(creds)
}

// newTestClient는 테스트용 BackendClient를 생성합니다.
func newTestClient(serverURL string) *BackendClient {
	logger := zerolog.Nop()
	return NewBackendClient(serverURL, mockTokenRefresher(), 5*time.Second, logger)
}

// TestDo_SuccessfulRequest는 정상 요청을 테스트합니다.
func TestDo_SuccessfulRequest(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Authorization 헤더 확인
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			t.Error("Authorization 헤더가 없습니다")
		}
		if authHeader != "Bearer test-access-token-valid" {
			t.Errorf("예상하지 못한 Authorization 헤더: %s", authHeader)
		}

		// Content-Type 확인
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("예상하지 못한 Content-Type: %s", r.Header.Get("Content-Type"))
		}

		resp := apiResponse{
			Success: true,
			Data:    json.RawMessage(`{"result":"ok"}`),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	client := newTestClient(server.URL)
	resp, err := client.Do(context.Background(), http.MethodGet, "/test", nil)
	if err != nil {
		t.Fatalf("예상하지 못한 오류: %v", err)
	}
	if !resp.Success {
		t.Error("응답이 성공이어야 합니다")
	}
}

// TestDo_WithRequestBody는 요청 본문이 있는 경우를 테스트합니다.
func TestDo_WithRequestBody(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("예상 메서드 POST, 실제: %s", r.Method)
		}

		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("요청 본문 파싱 실패: %v", err)
		}

		if body["name"] != "test-agent" {
			t.Errorf("예상 이름 test-agent, 실제: %s", body["name"])
		}

		resp := apiResponse{
			Success: true,
			Data:    json.RawMessage(`{"id":"agent-001"}`),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	client := newTestClient(server.URL)
	body := map[string]string{"name": "test-agent"}
	resp, err := client.Do(context.Background(), http.MethodPost, "/agents", body)
	if err != nil {
		t.Fatalf("예상하지 못한 오류: %v", err)
	}
	if !resp.Success {
		t.Error("응답이 성공이어야 합니다")
	}
}

// TestDo_ServerError는 서버 오류 응답을 테스트합니다.
func TestDo_ServerError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	client := newTestClient(server.URL)
	_, err := client.Do(context.Background(), http.MethodGet, "/test", nil)
	if err == nil {
		t.Fatal("서버 오류 시 에러가 반환되어야 합니다")
	}
}

// TestDo_ClientError는 4xx 응답을 테스트합니다.
func TestDo_ClientError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		resp := apiResponse{
			Success: false,
			Error:   "Resource not found",
		}
		json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	client := newTestClient(server.URL)
	_, err := client.Do(context.Background(), http.MethodGet, "/nonexistent", nil)
	if err == nil {
		t.Fatal("4xx 응답 시 에러가 반환되어야 합니다")
	}
}

// TestDo_BackendUnreachable는 백엔드 연결 실패를 테스트합니다.
func TestDo_BackendUnreachable(t *testing.T) {
	client := newTestClient("http://localhost:1") // 연결 불가능한 주소
	_, err := client.Do(context.Background(), http.MethodGet, "/test", nil)
	if err == nil {
		t.Fatal("연결 실패 시 에러가 반환되어야 합니다")
	}
}

// TestExecuteTask는 태스크 실행 API를 테스트합니다.
func TestExecuteTask(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/executions" {
			t.Errorf("예상 경로 /api/v1/executions, 실제: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("예상 메서드 POST, 실제: %s", r.Method)
		}

		var req ExecuteTaskRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("요청 파싱 실패: %v", err)
		}

		if req.AgentID != "agent-001" {
			t.Errorf("예상 agent_id agent-001, 실제: %s", req.AgentID)
		}

		resp := apiResponse{
			Success: true,
			Data:    json.RawMessage(`{"execution_id":"exec-001","status":"running"}`),
		}
		json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	client := newTestClient(server.URL)
	result, err := client.ExecuteTask(context.Background(), &ExecuteTaskRequest{
		AgentID: "agent-001",
		Prompt:  "Hello, agent!",
	})
	if err != nil {
		t.Fatalf("예상하지 못한 오류: %v", err)
	}
	if result.ExecutionID != "exec-001" {
		t.Errorf("예상 execution_id exec-001, 실제: %s", result.ExecutionID)
	}
	if result.Status != "running" {
		t.Errorf("예상 status running, 실제: %s", result.Status)
	}
}

// TestListAgents는 에이전트 목록 조회를 테스트합니다.
func TestListAgents(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/agents" {
			t.Errorf("예상 경로 /api/v1/agents, 실제: %s", r.URL.Path)
		}

		resp := apiResponse{
			Success: true,
			Data:    json.RawMessage(`{"agents":[{"id":"agent-001","name":"Test Agent"}],"total":1}`),
		}
		json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	client := newTestClient(server.URL)
	result, err := client.ListAgents(context.Background(), "")
	if err != nil {
		t.Fatalf("예상하지 못한 오류: %v", err)
	}
	if result.Total != 1 {
		t.Errorf("예상 total 1, 실제: %d", result.Total)
	}
	if len(result.Agents) != 1 {
		t.Fatalf("예상 에이전트 수 1, 실제: %d", len(result.Agents))
	}
	if result.Agents[0].ID != "agent-001" {
		t.Errorf("예상 agent ID agent-001, 실제: %s", result.Agents[0].ID)
	}
}

// TestListAgents_WithWorkspaceID는 workspace_id 파라미터를 테스트합니다.
func TestListAgents_WithWorkspaceID(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wsID := r.URL.Query().Get("workspace_id")
		if wsID != "ws-001" {
			t.Errorf("예상 workspace_id ws-001, 실제: %s", wsID)
		}

		resp := apiResponse{
			Success: true,
			Data:    json.RawMessage(`{"agents":[],"total":0}`),
		}
		json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	client := newTestClient(server.URL)
	_, err := client.ListAgents(context.Background(), "ws-001")
	if err != nil {
		t.Fatalf("예상하지 못한 오류: %v", err)
	}
}

// TestGetExecutionStatus는 실행 상태 조회를 테스트합니다.
func TestGetExecutionStatus(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/executions/exec-001" {
			t.Errorf("예상 경로 /api/v1/executions/exec-001, 실제: %s", r.URL.Path)
		}

		resp := apiResponse{
			Success: true,
			Data:    json.RawMessage(`{"execution_id":"exec-001","status":"completed"}`),
		}
		json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	client := newTestClient(server.URL)
	result, err := client.GetExecutionStatus(context.Background(), "exec-001")
	if err != nil {
		t.Fatalf("예상하지 못한 오류: %v", err)
	}
	if result.Status != "completed" {
		t.Errorf("예상 status completed, 실제: %s", result.Status)
	}
}

// TestApproveExecution은 실행 승인을 테스트합니다.
func TestApproveExecution(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/executions/exec-001/approve" {
			t.Errorf("예상 경로: /api/v1/executions/exec-001/approve, 실제: %s", r.URL.Path)
		}

		var req ApproveExecutionRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.Decision != "approve" {
			t.Errorf("예상 decision approve, 실제: %s", req.Decision)
		}

		resp := apiResponse{
			Success: true,
			Data:    json.RawMessage(`{"execution_id":"exec-001","status":"approved"}`),
		}
		json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	client := newTestClient(server.URL)
	result, err := client.ApproveExecution(context.Background(), &ApproveExecutionRequest{
		ExecutionID: "exec-001",
		Decision:    "approve",
	})
	if err != nil {
		t.Fatalf("예상하지 못한 오류: %v", err)
	}
	if result.Status != "approved" {
		t.Errorf("예상 status approved, 실제: %s", result.Status)
	}
}

// TestManageWorkspace_List는 워크스페이스 목록 조회를 테스트합니다.
func TestManageWorkspace_List(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces" {
			t.Errorf("예상 경로 /api/v1/workspaces, 실제: %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("예상 메서드 GET, 실제: %s", r.Method)
		}

		resp := apiResponse{
			Success: true,
			Data:    json.RawMessage(`{"workspaces":[{"id":"ws-001","name":"Test WS"}]}`),
		}
		json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	client := newTestClient(server.URL)
	result, err := client.ManageWorkspace(context.Background(), &ManageWorkspaceRequest{
		Action: "list",
	})
	if err != nil {
		t.Fatalf("예상하지 못한 오류: %v", err)
	}
	if len(result.Workspaces) != 1 {
		t.Errorf("예상 워크스페이스 수 1, 실제: %d", len(result.Workspaces))
	}
}

// TestManageWorkspace_InvalidAction은 유효하지 않은 액션을 테스트합니다.
func TestManageWorkspace_InvalidAction(t *testing.T) {
	client := newTestClient("http://localhost:1")
	_, err := client.ManageWorkspace(context.Background(), &ManageWorkspaceRequest{
		Action: "invalid",
	})
	if err == nil {
		t.Fatal("유효하지 않은 액션 시 에러가 반환되어야 합니다")
	}
}

// TestSearchKnowledge는 지식 검색을 테스트합니다.
func TestSearchKnowledge(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/knowledge/search" {
			t.Errorf("예상 경로 /api/v1/knowledge/search, 실제: %s", r.URL.Path)
		}

		var req SearchKnowledgeRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.Query != "test query" {
			t.Errorf("예상 query 'test query', 실제: '%s'", req.Query)
		}

		resp := apiResponse{
			Success: true,
			Data:    json.RawMessage(`{"results":[{"id":"doc-001","title":"Test Doc","content":"Test content","score":0.95}],"total":1,"query":"test query"}`),
		}
		json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	client := newTestClient(server.URL)
	result, err := client.SearchKnowledge(context.Background(), &SearchKnowledgeRequest{
		Query: "test query",
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("예상하지 못한 오류: %v", err)
	}
	if result.Total != 1 {
		t.Errorf("예상 total 1, 실제: %d", result.Total)
	}
	if len(result.Results) != 1 {
		t.Fatalf("예상 결과 수 1, 실제: %d", len(result.Results))
	}
	if result.Results[0].Score != 0.95 {
		t.Errorf("예상 score 0.95, 실제: %f", result.Results[0].Score)
	}
}

// TestManageWorkspace_Create는 워크스페이스 생성을 테스트합니다.
func TestManageWorkspace_Create(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces" {
			t.Errorf("예상 경로 /api/v1/workspaces, 실제: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("예상 메서드 POST, 실제: %s", r.Method)
		}

		// body가 존재하는지 확인
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("요청 본문 파싱 실패: %v", err)
		}

		resp := apiResponse{
			Success: true,
			Data:    json.RawMessage(`{"workspace":{"id":"ws-new-001","name":"New Workspace"},"message":"created"}`),
		}
		json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	client := newTestClient(server.URL)
	result, err := client.ManageWorkspace(context.Background(), &ManageWorkspaceRequest{
		Action: "create",
		Config: map[string]interface{}{"name": "New Workspace"},
	})
	if err != nil {
		t.Fatalf("예상하지 못한 오류: %v", err)
	}
	if result.Workspace == nil {
		t.Fatal("workspace가 nil입니다")
	}
	if result.Workspace.ID != "ws-new-001" {
		t.Errorf("예상 workspace ID ws-new-001, 실제: %s", result.Workspace.ID)
	}
}

// TestManageWorkspace_Update는 워크스페이스 수정을 테스트합니다.
func TestManageWorkspace_Update(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-001" {
			t.Errorf("예상 경로 /api/v1/workspaces/ws-001, 실제: %s", r.URL.Path)
		}
		if r.Method != http.MethodPut {
			t.Errorf("예상 메서드 PUT, 실제: %s", r.Method)
		}

		resp := apiResponse{
			Success: true,
			Data:    json.RawMessage(`{"workspace":{"id":"ws-001","name":"Updated WS"},"message":"updated"}`),
		}
		json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	client := newTestClient(server.URL)
	result, err := client.ManageWorkspace(context.Background(), &ManageWorkspaceRequest{
		Action:      "update",
		WorkspaceID: "ws-001",
		Config:      map[string]interface{}{"name": "Updated WS"},
	})
	if err != nil {
		t.Fatalf("예상하지 못한 오류: %v", err)
	}
	if result.Workspace == nil {
		t.Fatal("workspace가 nil입니다")
	}
	if result.Workspace.Name != "Updated WS" {
		t.Errorf("예상 workspace name 'Updated WS', 실제: %s", result.Workspace.Name)
	}
}

// TestManageWorkspace_Delete는 워크스페이스 삭제를 테스트합니다.
func TestManageWorkspace_Delete(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-001" {
			t.Errorf("예상 경로 /api/v1/workspaces/ws-001, 실제: %s", r.URL.Path)
		}
		if r.Method != http.MethodDelete {
			t.Errorf("예상 메서드 DELETE, 실제: %s", r.Method)
		}

		resp := apiResponse{
			Success: true,
			Data:    json.RawMessage(`{"message":"deleted"}`),
		}
		json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	client := newTestClient(server.URL)
	result, err := client.ManageWorkspace(context.Background(), &ManageWorkspaceRequest{
		Action:      "delete",
		WorkspaceID: "ws-001",
	})
	if err != nil {
		t.Fatalf("예상하지 못한 오류: %v", err)
	}
	if result.Message != "deleted" {
		t.Errorf("예상 message 'deleted', 실제: %s", result.Message)
	}
}

// TestDo_ClientError_MessageFallback는 4xx 응답에서 error 필드가 비어있고 message만 있는 경우를 테스트합니다.
func TestDo_ClientError_MessageFallback(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		resp := apiResponse{
			Success: false,
			Message: "Bad request details",
		}
		json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	client := newTestClient(server.URL)
	_, err := client.Do(context.Background(), http.MethodPost, "/test", nil)
	if err == nil {
		t.Fatal("4xx 응답 시 에러가 반환되어야 합니다")
	}
	if !contains(err.Error(), "Bad request details") {
		t.Errorf("에러 메시지에 'Bad request details'가 포함되어야 합니다, 실제: %s", err.Error())
	}
}

// TestDo_ClientError_EmptyErrorAndMessage는 4xx 응답에서 error와 message 모두 비어있는 경우를 테스트합니다.
func TestDo_ClientError_EmptyErrorAndMessage(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		resp := apiResponse{
			Success: false,
		}
		json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	client := newTestClient(server.URL)
	_, err := client.Do(context.Background(), http.MethodGet, "/forbidden", nil)
	if err == nil {
		t.Fatal("4xx 응답 시 에러가 반환되어야 합니다")
	}
	if !contains(err.Error(), "HTTP 403") {
		t.Errorf("에러 메시지에 'HTTP 403'이 포함되어야 합니다, 실제: %s", err.Error())
	}
}

// TestDo_InvalidResponseJSON은 비정상 JSON 응답을 테스트합니다.
func TestDo_InvalidResponseJSON(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not json content"))
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	client := newTestClient(server.URL)
	_, err := client.Do(context.Background(), http.MethodGet, "/broken", nil)
	if err == nil {
		t.Fatal("비정상 JSON 응답 시 에러가 반환되어야 합니다")
	}
	if !contains(err.Error(), "응답 파싱 실패") {
		t.Errorf("에러 메시지에 '응답 파싱 실패'가 포함되어야 합니다, 실제: %s", err.Error())
	}
}

// TestExecuteTask_InvalidResponseData는 data 필드가 잘못된 경우를 테스트합니다.
func TestExecuteTask_InvalidResponseData(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := apiResponse{
			Success: true,
			Data:    json.RawMessage(`"invalid-not-an-object"`),
		}
		json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	client := newTestClient(server.URL)
	_, err := client.ExecuteTask(context.Background(), &ExecuteTaskRequest{
		AgentID: "agent-001",
		Prompt:  "test",
	})
	if err == nil {
		t.Fatal("잘못된 data 형식 시 에러가 반환되어야 합니다")
	}
}

// TestGetExecutionStatus_InvalidResponseData는 data 필드가 잘못된 경우를 테스트합니다.
func TestGetExecutionStatus_InvalidResponseData(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := apiResponse{
			Success: true,
			Data:    json.RawMessage(`"invalid"`),
		}
		json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	client := newTestClient(server.URL)
	_, err := client.GetExecutionStatus(context.Background(), "exec-001")
	if err == nil {
		t.Fatal("잘못된 data 형식 시 에러가 반환되어야 합니다")
	}
}

// TestApproveExecution_InvalidResponseData는 data 필드가 잘못된 경우를 테스트합니다.
func TestApproveExecution_InvalidResponseData(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := apiResponse{
			Success: true,
			Data:    json.RawMessage(`"invalid"`),
		}
		json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	client := newTestClient(server.URL)
	_, err := client.ApproveExecution(context.Background(), &ApproveExecutionRequest{
		ExecutionID: "exec-001",
		Decision:    "approve",
	})
	if err == nil {
		t.Fatal("잘못된 data 형식 시 에러가 반환되어야 합니다")
	}
}

// TestSearchKnowledge_InvalidResponseData는 data 필드가 잘못된 경우를 테스트합니다.
func TestSearchKnowledge_InvalidResponseData(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := apiResponse{
			Success: true,
			Data:    json.RawMessage(`"invalid"`),
		}
		json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	client := newTestClient(server.URL)
	_, err := client.SearchKnowledge(context.Background(), &SearchKnowledgeRequest{
		Query: "test",
	})
	if err == nil {
		t.Fatal("잘못된 data 형식 시 에러가 반환되어야 합니다")
	}
}

// TestManageWorkspace_InvalidResponseData는 data 필드가 잘못된 경우를 테스트합니다.
func TestManageWorkspace_InvalidResponseData(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := apiResponse{
			Success: true,
			Data:    json.RawMessage(`"invalid"`),
		}
		json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	client := newTestClient(server.URL)
	_, err := client.ManageWorkspace(context.Background(), &ManageWorkspaceRequest{
		Action: "list",
	})
	if err == nil {
		t.Fatal("잘못된 data 형식 시 에러가 반환되어야 합니다")
	}
}

// TestListAgents_InvalidResponseData는 data 필드가 잘못된 경우를 테스트합니다.
func TestListAgents_InvalidResponseData(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := apiResponse{
			Success: true,
			Data:    json.RawMessage(`"invalid"`),
		}
		json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	client := newTestClient(server.URL)
	_, err := client.ListAgents(context.Background(), "")
	if err == nil {
		t.Fatal("잘못된 data 형식 시 에러가 반환되어야 합니다")
	}
}

// contains는 문자열 포함 여부를 검사하는 헬퍼입니다.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestExtractIDFromURI는 URI에서 ID 추출을 테스트합니다.
func TestExtractIDFromURI(t *testing.T) {
	tests := []struct {
		name         string
		uri          string
		resourceType string
		expected     string
	}{
		{
			name:         "정상 execution URI",
			uri:          "autopus://executions/exec-001",
			resourceType: "executions",
			expected:     "exec-001",
		},
		{
			name:         "UUID 형식 ID",
			uri:          "autopus://executions/550e8400-e29b-41d4-a716-446655440000",
			resourceType: "executions",
			expected:     "550e8400-e29b-41d4-a716-446655440000",
		},
		{
			name:         "잘못된 접두사",
			uri:          "other://executions/exec-001",
			resourceType: "executions",
			expected:     "",
		},
		{
			name:         "잘못된 리소스 타입",
			uri:          "autopus://agents/agent-001",
			resourceType: "executions",
			expected:     "",
		},
		{
			name:         "추가 경로 세그먼트",
			uri:          "autopus://executions/exec-001/details",
			resourceType: "executions",
			expected:     "exec-001",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractIDFromURI(tt.uri, tt.resourceType)
			if result != tt.expected {
				t.Errorf("extractIDFromURI(%q, %q) = %q, expected %q", tt.uri, tt.resourceType, result, tt.expected)
			}
		})
	}
}
