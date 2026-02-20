package mcpserver

import (
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/rs/zerolog"
)

const (
	// ServerName은 MCP 서버 이름입니다.
	ServerName = "autopus-bridge"
	// ServerVersion은 MCP 서버 버전입니다.
	ServerVersion = "0.1.0"
)

// Server는 Autopus MCP 서버입니다.
// mark3labs/mcp-go를 사용하여 stdio 기반 MCP 프로토콜을 처리합니다.
type Server struct {
	mcpServer *server.MCPServer
	client    *BackendClient
	logger    zerolog.Logger
}

// NewServer는 새 MCP 서버를 생성합니다.
// BackendClient를 통해 Autopus 백엔드와 통신합니다.
func NewServer(client *BackendClient, logger zerolog.Logger) *Server {
	s := &Server{
		client: client,
		logger: logger.With().Str("component", "mcpserver").Logger(),
	}

	// MCP 서버 생성
	s.mcpServer = server.NewMCPServer(
		ServerName,
		ServerVersion,
		server.WithToolCapabilities(false),
		server.WithRecovery(),
	)

	// 도구 및 리소스 등록
	s.registerTools()
	s.registerResources()

	s.logger.Info().
		Str("name", ServerName).
		Str("version", ServerVersion).
		Msg("MCP 서버 초기화 완료")

	return s
}

// Start는 stdio 기반 MCP 서버를 시작합니다.
// 이 함수는 서버가 종료될 때까지 블로킹됩니다.
func (s *Server) Start() error {
	s.logger.Info().Msg("MCP 서버 시작 (stdio 트랜스포트)")
	return server.ServeStdio(s.mcpServer)
}

// registerTools는 모든 MCP 도구를 등록합니다.
func (s *Server) registerTools() {
	// 1. execute_task - Autopus 에이전트 태스크 실행
	executeTaskTool := mcp.NewTool("execute_task",
		mcp.WithDescription("Execute an Autopus agent task. Sends a prompt to a specified agent for processing."),
		mcp.WithString("agent_id",
			mcp.Required(),
			mcp.Description("ID of the agent to execute the task"),
		),
		mcp.WithString("prompt",
			mcp.Required(),
			mcp.Description("The prompt/instruction for the agent to process"),
		),
		mcp.WithString("workspace_id",
			mcp.Description("Target workspace ID (optional, uses default workspace if not specified)"),
		),
		mcp.WithString("model",
			mcp.Description("AI model to use (optional, uses agent's default model if not specified)"),
		),
	)
	s.mcpServer.AddTool(executeTaskTool, s.handleExecuteTask)

	// 2. list_agents - 사용 가능한 에이전트 목록
	listAgentsTool := mcp.NewTool("list_agents",
		mcp.WithDescription("List available Autopus agents. Returns agents accessible in the specified workspace."),
		mcp.WithString("workspace_id",
			mcp.Description("Workspace ID to filter agents (optional, lists all accessible agents if not specified)"),
		),
	)
	s.mcpServer.AddTool(listAgentsTool, s.handleListAgents)

	// 3. get_execution_status - 태스크 실행 상태 조회
	getStatusTool := mcp.NewTool("get_execution_status",
		mcp.WithDescription("Get the status of a task execution. Returns current state, result, or error information."),
		mcp.WithString("execution_id",
			mcp.Required(),
			mcp.Description("The execution ID returned from execute_task"),
		),
	)
	s.mcpServer.AddTool(getStatusTool, s.handleGetExecutionStatus)

	// 4. approve_execution - 실행 승인/거부
	approveExecutionTool := mcp.NewTool("approve_execution",
		mcp.WithDescription("Approve or reject a pending task execution that requires human review."),
		mcp.WithString("execution_id",
			mcp.Required(),
			mcp.Description("The execution ID to approve or reject"),
		),
		mcp.WithString("decision",
			mcp.Required(),
			mcp.Description("Decision: 'approve' or 'reject'"),
			mcp.Enum("approve", "reject"),
		),
		mcp.WithString("reason",
			mcp.Description("Reason for the decision (recommended for rejections)"),
		),
	)
	s.mcpServer.AddTool(approveExecutionTool, s.handleApproveExecution)

	// 5. manage_workspace - 워크스페이스 관리
	manageWorkspaceTool := mcp.NewTool("manage_workspace",
		mcp.WithDescription("Manage Autopus workspaces. Supports listing, creating, updating, and deleting workspaces."),
		mcp.WithString("action",
			mcp.Required(),
			mcp.Description("Action to perform"),
			mcp.Enum("list", "create", "update", "delete"),
		),
		mcp.WithString("workspace_id",
			mcp.Description("Workspace ID (required for update/delete)"),
		),
	)
	s.mcpServer.AddTool(manageWorkspaceTool, s.handleManageWorkspace)

	// 6. search_knowledge - 지식 베이스 검색
	searchKnowledgeTool := mcp.NewTool("search_knowledge",
		mcp.WithDescription("Search the Autopus knowledge base. Finds relevant documents and information."),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("Search query string"),
		),
		mcp.WithString("workspace_id",
			mcp.Description("Workspace ID to search within (optional)"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of results to return (default: 10, max: 50)"),
		),
	)
	s.mcpServer.AddTool(searchKnowledgeTool, s.handleSearchKnowledge)

	s.logger.Debug().Msg("MCP 도구 6개 등록 완료")
}

// registerResources는 모든 MCP 리소스를 등록합니다.
func (s *Server) registerResources() {
	// 1. autopus://status - 플랫폼 연결 상태
	statusResource := mcp.NewResource(
		"autopus://status",
		"Platform Status",
		mcp.WithResourceDescription("Autopus platform connection status and health information"),
		mcp.WithMIMEType("application/json"),
	)
	s.mcpServer.AddResource(statusResource, s.handleStatusResource)

	// 2. autopus://executions/{id} - 특정 실행 상세 (동적 리소스)
	executionTemplate := mcp.NewResourceTemplate(
		"autopus://executions/{id}",
		"Execution Details",
		mcp.WithTemplateDescription("Detailed information about a specific task execution"),
		mcp.WithTemplateMIMEType("application/json"),
	)
	s.mcpServer.AddResourceTemplate(executionTemplate, s.handleExecutionResource)

	// 3. autopus://workspaces - 워크스페이스 목록
	workspacesResource := mcp.NewResource(
		"autopus://workspaces",
		"Workspaces",
		mcp.WithResourceDescription("List of accessible Autopus workspaces"),
		mcp.WithMIMEType("application/json"),
	)
	s.mcpServer.AddResource(workspacesResource, s.handleWorkspacesResource)

	// 4. autopus://agents - 에이전트 카탈로그
	agentsResource := mcp.NewResource(
		"autopus://agents",
		"Agent Catalog",
		mcp.WithResourceDescription("Catalog of available Autopus agents"),
		mcp.WithMIMEType("application/json"),
	)
	s.mcpServer.AddResource(agentsResource, s.handleAgentsResource)

	s.logger.Debug().Msg("MCP 리소스 4개 등록 완료")
}
