package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

// handleExecuteTask는 execute_task 도구 핸들러입니다.
// Autopus 에이전트에 태스크를 전달하여 실행합니다.
func (s *Server) handleExecuteTask(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	agentID, err := request.RequireString("agent_id")
	if err != nil {
		return mcp.NewToolResultError("required parameter 'agent_id' is missing or invalid"), nil
	}

	prompt, err := request.RequireString("prompt")
	if err != nil {
		return mcp.NewToolResultError("required parameter 'prompt' is missing or invalid"), nil
	}

	workspaceID := request.GetString("workspace_id", "")
	model := request.GetString("model", "")

	s.logger.Info().
		Str("agent_id", agentID).
		Str("workspace_id", workspaceID).
		Msg("태스크 실행 요청")

	resp, err := s.client.ExecuteTask(ctx, &ExecuteTaskRequest{
		AgentID:     agentID,
		Prompt:      prompt,
		WorkspaceID: workspaceID,
		Model:       model,
	})
	if err != nil {
		s.logger.Error().Err(err).Msg("태스크 실행 실패")
		return mcp.NewToolResultError(fmt.Sprintf("Failed to execute task: %s", err.Error())), nil
	}

	result, err := json.Marshal(resp)
	if err != nil {
		return mcp.NewToolResultError("Failed to serialize response"), nil
	}

	return mcp.NewToolResultText(string(result)), nil
}

// handleListAgents는 list_agents 도구 핸들러입니다.
// 사용 가능한 Autopus 에이전트 목록을 반환합니다.
func (s *Server) handleListAgents(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	workspaceID := request.GetString("workspace_id", "")

	s.logger.Info().
		Str("workspace_id", workspaceID).
		Msg("에이전트 목록 조회")

	resp, err := s.client.ListAgents(ctx, workspaceID)
	if err != nil {
		s.logger.Error().Err(err).Msg("에이전트 목록 조회 실패")
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list agents: %s", err.Error())), nil
	}

	result, err := json.Marshal(resp)
	if err != nil {
		return mcp.NewToolResultError("Failed to serialize response"), nil
	}

	return mcp.NewToolResultText(string(result)), nil
}

// handleGetExecutionStatus는 get_execution_status 도구 핸들러입니다.
// 태스크 실행 상태를 조회합니다.
func (s *Server) handleGetExecutionStatus(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	executionID, err := request.RequireString("execution_id")
	if err != nil {
		return mcp.NewToolResultError("required parameter 'execution_id' is missing or invalid"), nil
	}

	s.logger.Info().
		Str("execution_id", executionID).
		Msg("실행 상태 조회")

	resp, err := s.client.GetExecutionStatus(ctx, executionID)
	if err != nil {
		s.logger.Error().Err(err).Msg("실행 상태 조회 실패")
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get execution status: %s", err.Error())), nil
	}

	result, err := json.Marshal(resp)
	if err != nil {
		return mcp.NewToolResultError("Failed to serialize response"), nil
	}

	return mcp.NewToolResultText(string(result)), nil
}

// handleApproveExecution은 approve_execution 도구 핸들러입니다.
// 대기 중인 태스크 실행을 승인하거나 거부합니다.
func (s *Server) handleApproveExecution(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	executionID, err := request.RequireString("execution_id")
	if err != nil {
		return mcp.NewToolResultError("required parameter 'execution_id' is missing or invalid"), nil
	}

	decision, err := request.RequireString("decision")
	if err != nil {
		return mcp.NewToolResultError("required parameter 'decision' is missing or invalid"), nil
	}

	if decision != "approve" && decision != "reject" {
		return mcp.NewToolResultError("decision must be 'approve' or 'reject'"), nil
	}

	reason := request.GetString("reason", "")

	s.logger.Info().
		Str("execution_id", executionID).
		Str("decision", decision).
		Msg("실행 승인/거부 요청")

	resp, err := s.client.ApproveExecution(ctx, &ApproveExecutionRequest{
		ExecutionID: executionID,
		Decision:    decision,
		Reason:      reason,
	})
	if err != nil {
		s.logger.Error().Err(err).Msg("승인/거부 실패")
		return mcp.NewToolResultError(fmt.Sprintf("Failed to approve/reject execution: %s", err.Error())), nil
	}

	result, err := json.Marshal(resp)
	if err != nil {
		return mcp.NewToolResultError("Failed to serialize response"), nil
	}

	return mcp.NewToolResultText(string(result)), nil
}

// handleManageWorkspace는 manage_workspace 도구 핸들러입니다.
// 워크스페이스를 관리합니다 (목록, 생성, 수정, 삭제).
func (s *Server) handleManageWorkspace(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	action, err := request.RequireString("action")
	if err != nil {
		return mcp.NewToolResultError("required parameter 'action' is missing or invalid"), nil
	}

	validActions := map[string]bool{"list": true, "create": true, "update": true, "delete": true}
	if !validActions[action] {
		return mcp.NewToolResultError("action must be one of: list, create, update, delete"), nil
	}

	workspaceID := request.GetString("workspace_id", "")

	// update와 delete에는 workspace_id 필수
	if (action == "update" || action == "delete") && workspaceID == "" {
		return mcp.NewToolResultError(fmt.Sprintf("workspace_id is required for '%s' action", action)), nil
	}

	s.logger.Info().
		Str("action", action).
		Str("workspace_id", workspaceID).
		Msg("워크스페이스 관리 요청")

	resp, err := s.client.ManageWorkspace(ctx, &ManageWorkspaceRequest{
		Action:      action,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		s.logger.Error().Err(err).Msg("워크스페이스 관리 실패")
		return mcp.NewToolResultError(fmt.Sprintf("Failed to manage workspace: %s", err.Error())), nil
	}

	result, err := json.Marshal(resp)
	if err != nil {
		return mcp.NewToolResultError("Failed to serialize response"), nil
	}

	return mcp.NewToolResultText(string(result)), nil
}

// handleSearchKnowledge는 search_knowledge 도구 핸들러입니다.
// 지식 베이스를 검색합니다.
func (s *Server) handleSearchKnowledge(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query, err := request.RequireString("query")
	if err != nil {
		return mcp.NewToolResultError("required parameter 'query' is missing or invalid"), nil
	}

	workspaceID := request.GetString("workspace_id", "")

	limit := request.GetInt("limit", 10)
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	s.logger.Info().
		Str("query", query).
		Str("workspace_id", workspaceID).
		Int("limit", limit).
		Msg("지식 검색 요청")

	resp, err := s.client.SearchKnowledge(ctx, &SearchKnowledgeRequest{
		Query:       query,
		WorkspaceID: workspaceID,
		Limit:       limit,
	})
	if err != nil {
		s.logger.Error().Err(err).Msg("지식 검색 실패")
		return mcp.NewToolResultError(fmt.Sprintf("Failed to search knowledge: %s", err.Error())), nil
	}

	result, err := json.Marshal(resp)
	if err != nil {
		return mcp.NewToolResultError("Failed to serialize response"), nil
	}

	return mcp.NewToolResultText(string(result)), nil
}
