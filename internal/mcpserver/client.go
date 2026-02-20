// Package mcpserver는 Autopus MCP 서버를 구현합니다.
// Claude Code가 Autopus 플랫폼 기능에 MCP 도구/리소스로 접근할 수 있도록 합니다.
package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/insajin/autopus-bridge/internal/auth"
	"github.com/rs/zerolog"
)

// BackendClient는 브릿지의 인증 시스템을 재사용하는 Autopus 백엔드 API 클라이언트입니다.
// TokenRefresher를 통해 JWT 토큰을 자동으로 관리합니다.
type BackendClient struct {
	baseURL      string
	httpClient   *http.Client
	logger       zerolog.Logger
	tokenRefresh *auth.TokenRefresher
}

// NewBackendClient는 새 BackendClient를 생성합니다.
// baseURL은 백엔드 API의 기본 URL입니다 (예: https://api.autopus.co).
// tokenRefresher는 브릿지의 인증 시스템에서 재사용하는 TokenRefresher입니다.
func NewBackendClient(baseURL string, tokenRefresher *auth.TokenRefresher, timeout time.Duration, logger zerolog.Logger) *BackendClient {
	return &BackendClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		logger:       logger.With().Str("component", "mcpserver.client").Logger(),
		tokenRefresh: tokenRefresher,
	}
}

// apiResponse는 백엔드 API의 표준 응답 형식입니다.
type apiResponse struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data,omitempty"`
	Error   string          `json:"error,omitempty"`
	Message string          `json:"message,omitempty"`
}

// Do는 인증된 HTTP 요청을 실행합니다.
// TokenRefresher에서 현재 유효한 JWT 토큰을 가져와 Authorization 헤더에 추가합니다.
func (c *BackendClient) Do(ctx context.Context, method, path string, body interface{}) (*apiResponse, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("요청 본문 직렬화 실패: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	url := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("HTTP 요청 생성 실패: %w", err)
	}

	// TokenRefresher에서 유효한 토큰 가져오기
	token, err := c.tokenRefresh.GetToken()
	if err != nil {
		return nil, fmt.Errorf("인증 토큰 획득 실패: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	c.logger.Debug().
		Str("method", method).
		Str("path", path).
		Msg("API 요청 전송")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("백엔드 통신 실패 (서버에 연결할 수 없습니다): %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("응답 읽기 실패: %w", err)
	}

	if resp.StatusCode >= 500 {
		return nil, fmt.Errorf("백엔드 서버 오류 (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	var apiResp apiResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("응답 파싱 실패 (HTTP %d): %w", resp.StatusCode, err)
	}

	if resp.StatusCode >= 400 {
		errMsg := apiResp.Error
		if errMsg == "" {
			errMsg = apiResp.Message
		}
		if errMsg == "" {
			errMsg = fmt.Sprintf("HTTP %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("API 오류: %s", errMsg)
	}

	return &apiResp, nil
}

// ExecuteTaskRequest는 태스크 실행 요청 파라미터입니다.
type ExecuteTaskRequest struct {
	AgentID     string   `json:"agent_id"`
	Prompt      string   `json:"prompt"`
	WorkspaceID string   `json:"workspace_id,omitempty"`
	Tools       []string `json:"tools,omitempty"`
	Model       string   `json:"model,omitempty"`
}

// ExecuteTaskResponse는 태스크 실행 응답입니다.
type ExecuteTaskResponse struct {
	ExecutionID string `json:"execution_id"`
	Status      string `json:"status"`
	Message     string `json:"message,omitempty"`
}

// ExecuteTask는 Autopus 에이전트 태스크를 실행합니다.
func (c *BackendClient) ExecuteTask(ctx context.Context, req *ExecuteTaskRequest) (*ExecuteTaskResponse, error) {
	resp, err := c.Do(ctx, http.MethodPost, "/api/v1/executions", req)
	if err != nil {
		return nil, err
	}

	var result ExecuteTaskResponse
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("태스크 실행 응답 파싱 실패: %w", err)
	}
	return &result, nil
}

// AgentInfo는 에이전트 정보입니다.
type AgentInfo struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Tools       []string `json:"tools,omitempty"`
	Model       string   `json:"model,omitempty"`
}

// ListAgentsResponse는 에이전트 목록 응답입니다.
type ListAgentsResponse struct {
	Agents []AgentInfo `json:"agents"`
	Total  int         `json:"total"`
}

// ListAgents는 사용 가능한 에이전트 목록을 조회합니다.
// opts는 선택적 파라미터입니다: 첫 번째 값은 filter 문자열로 사용됩니다.
func (c *BackendClient) ListAgents(ctx context.Context, workspaceID string, opts ...string) (*ListAgentsResponse, error) {
	path := "/api/v1/agents"
	var queryParams []string
	if workspaceID != "" {
		queryParams = append(queryParams, "workspace_id="+workspaceID)
	}
	if len(opts) > 0 && opts[0] != "" {
		queryParams = append(queryParams, "filter="+opts[0])
	}
	if len(queryParams) > 0 {
		path += "?" + strings.Join(queryParams, "&")
	}

	resp, err := c.Do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var result ListAgentsResponse
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("에이전트 목록 응답 파싱 실패: %w", err)
	}
	return &result, nil
}

// ExecutionStatus는 실행 상태 정보입니다.
type ExecutionStatus struct {
	ExecutionID string          `json:"execution_id"`
	Status      string          `json:"status"`
	Result      json.RawMessage `json:"result,omitempty"`
	Error       string          `json:"error,omitempty"`
	CreatedAt   string          `json:"created_at,omitempty"`
	UpdatedAt   string          `json:"updated_at,omitempty"`
}

// GetExecutionStatus는 태스크 실행 상태를 조회합니다.
func (c *BackendClient) GetExecutionStatus(ctx context.Context, executionID string) (*ExecutionStatus, error) {
	resp, err := c.Do(ctx, http.MethodGet, "/api/v1/executions/"+executionID, nil)
	if err != nil {
		return nil, err
	}

	var result ExecutionStatus
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("실행 상태 응답 파싱 실패: %w", err)
	}
	return &result, nil
}

// ApproveExecutionRequest는 실행 승인/거부 요청입니다.
type ApproveExecutionRequest struct {
	ExecutionID string `json:"execution_id"`
	Decision    string `json:"decision"` // "approve" 또는 "reject"
	Reason      string `json:"reason,omitempty"`
}

// ApproveExecutionResponse는 실행 승인/거부 응답입니다.
type ApproveExecutionResponse struct {
	ExecutionID string `json:"execution_id"`
	Status      string `json:"status"`
	Message     string `json:"message,omitempty"`
}

// ApproveExecution은 실행을 승인하거나 거부합니다.
func (c *BackendClient) ApproveExecution(ctx context.Context, req *ApproveExecutionRequest) (*ApproveExecutionResponse, error) {
	resp, err := c.Do(ctx, http.MethodPost, "/api/v1/executions/"+req.ExecutionID+"/approve", req)
	if err != nil {
		return nil, err
	}

	var result ApproveExecutionResponse
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("승인 응답 파싱 실패: %w", err)
	}
	return &result, nil
}

// ManageWorkspaceRequest는 워크스페이스 관리 요청입니다.
type ManageWorkspaceRequest struct {
	Action      string                 `json:"action"` // "list", "create", "update", "delete"
	WorkspaceID string                 `json:"workspace_id,omitempty"`
	Config      map[string]interface{} `json:"config,omitempty"`
}

// WorkspaceInfo는 워크스페이스 정보입니다.
type WorkspaceInfo struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Slug        string                 `json:"slug,omitempty"`
	Description string                 `json:"description,omitempty"`
	Config      map[string]interface{} `json:"config,omitempty"`
}

// ManageWorkspaceResponse는 워크스페이스 관리 응답입니다.
type ManageWorkspaceResponse struct {
	Workspaces []WorkspaceInfo `json:"workspaces,omitempty"`
	Workspace  *WorkspaceInfo  `json:"workspace,omitempty"`
	Message    string          `json:"message,omitempty"`
}

// ManageWorkspace는 워크스페이스를 관리합니다.
func (c *BackendClient) ManageWorkspace(ctx context.Context, req *ManageWorkspaceRequest) (*ManageWorkspaceResponse, error) {
	var method string
	var path string

	switch req.Action {
	case "get":
		method = http.MethodGet
		path = "/api/v1/workspaces/" + req.WorkspaceID
	case "list":
		method = http.MethodGet
		path = "/api/v1/workspaces"
	case "create":
		method = http.MethodPost
		path = "/api/v1/workspaces"
	case "update":
		method = http.MethodPut
		path = "/api/v1/workspaces/" + req.WorkspaceID
	case "delete":
		method = http.MethodDelete
		path = "/api/v1/workspaces/" + req.WorkspaceID
	default:
		return nil, fmt.Errorf("알 수 없는 워크스페이스 액션: %s", req.Action)
	}

	var body interface{}
	if method != http.MethodGet && method != http.MethodDelete {
		body = req
	}

	resp, err := c.Do(ctx, method, path, body)
	if err != nil {
		return nil, err
	}

	var result ManageWorkspaceResponse
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("워크스페이스 응답 파싱 실패: %w", err)
	}
	return &result, nil
}

// SearchKnowledgeRequest는 지식 검색 요청입니다.
type SearchKnowledgeRequest struct {
	Query       string                 `json:"query"`
	WorkspaceID string                 `json:"workspace_id,omitempty"`
	Limit       int                    `json:"limit,omitempty"`
	Filters     map[string]interface{} `json:"filters,omitempty"`
}

// KnowledgeResult는 검색 결과 항목입니다.
type KnowledgeResult struct {
	ID       string  `json:"id"`
	Title    string  `json:"title"`
	Content  string  `json:"content"`
	Score    float64 `json:"score,omitempty"`
	Source   string  `json:"source,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// SearchKnowledgeResponse는 지식 검색 응답입니다.
type SearchKnowledgeResponse struct {
	Results []KnowledgeResult `json:"results"`
	Total   int               `json:"total"`
	Query   string            `json:"query"`
}

// SearchKnowledge는 지식 베이스를 검색합니다.
func (c *BackendClient) SearchKnowledge(ctx context.Context, req *SearchKnowledgeRequest) (*SearchKnowledgeResponse, error) {
	resp, err := c.Do(ctx, http.MethodPost, "/api/v1/knowledge/search", req)
	if err != nil {
		return nil, err
	}

	var result SearchKnowledgeResponse
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("지식 검색 응답 파싱 실패: %w", err)
	}
	return &result, nil
}
