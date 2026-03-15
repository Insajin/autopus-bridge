// Package ws provides shared WebSocket communication types for Local Agent
// and server communication. This package is used by both backend and
// local-agent-bridge modules.
package ws

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Agent WebSocket message type constants.
const (
	AgentMsgConnect    = "agent_connect"
	AgentMsgConnectAck = "agent_connect_ack" // Server -> Agent auth acknowledgement
	AgentMsgDisconnect = "agent_disconnect"
	AgentMsgHeartbeat  = "agent_heartbeat"
	AgentMsgTaskReq    = "task_request"
	AgentMsgTaskProg   = "task_progress"
	AgentMsgTaskResult = "task_result"
	AgentMsgTaskError  = "task_error"

	// Build operation message types (FR-P3-01).
	AgentMsgBuildReq    = "build_request"
	AgentMsgBuildResult = "build_result"

	// Test operation message types (FR-P3-02).
	AgentMsgTestReq    = "test_request"
	AgentMsgTestResult = "test_result"

	// QA operation message types (FR-P3-03).
	AgentMsgQAReq    = "qa_request"
	AgentMsgQAResult = "qa_result"

	// Computer Use message types (SPEC-COMPUTER-USE-001)
	AgentMsgComputerAction       = "computer_action"
	AgentMsgComputerResult       = "computer_result"
	AgentMsgComputerSessionStart = "computer_session_start"
	AgentMsgComputerSessionEnd   = "computer_session_end"

	// Computer Use Container Pool message types (SPEC-COMPUTER-USE-002)
	AgentMsgComputerPoolStatus = "computer_pool_status"

	// Agent Browser message types (SPEC-BROWSER-AGENT-001)
	AgentMsgBrowserSessionStart = "browser_session_start"
	AgentMsgBrowserAction       = "browser_action"
	AgentMsgBrowserSessionEnd   = "browser_session_end"
	AgentMsgBrowserSessionReady = "browser_session_ready"
	AgentMsgBrowserResult       = "browser_result"
	AgentMsgBrowserNotAvailable = "browser_not_available"
	AgentMsgBrowserError        = "browser_error"

	// Worker Orchestrator message types (SPEC-AUTONOMOUS-WORKER-001)
	AgentMsgWorkerPhaseChanged = "worker_phase_changed"

	// Skill Discovery V2 message types (SPEC-SKILL-V2-001)
	AgentMsgProjectContext = "project_context" // Bridge -> Server: 프로젝트 기술 스택 컨텍스트
	AgentMsgCLIRequest     = "cli_request"     // Server -> Bridge: CLI 명령어 실행 요청
	AgentMsgCLIResult      = "cli_result"      // Bridge -> Server: CLI 실행 결과

	// MCP Provisioning (SPEC-SKILL-V2-001 Phase 3)
	AgentMsgMCPStart = "mcp_start" // Server -> Bridge: MCP server start request
	AgentMsgMCPReady = "mcp_ready" // Bridge -> Server: MCP server ready
	AgentMsgMCPStop  = "mcp_stop"  // Server -> Bridge: MCP server stop request
	AgentMsgMCPError = "mcp_error" // Bridge -> Server: MCP server error report

	// Self-Expansion message types (SPEC-SELF-EXPAND-001)
	AgentMsgMCPCodegenRequest  = "mcp_codegen_request"  // Server -> Bridge
	AgentMsgMCPCodegenProgress = "mcp_codegen_progress" // Bridge -> Server
	AgentMsgMCPCodegenResult   = "mcp_codegen_result"   // Bridge -> Server
	AgentMsgMCPDeploy          = "mcp_deploy"           // Server -> Bridge
	AgentMsgMCPDeployResult    = "mcp_deploy_result"    // Bridge -> Server
	AgentMsgMCPHealthReport    = "mcp_health_report"    // Bridge -> Server

	// MCP Server (serve) lifecycle management (SPEC-AI-003 M3)
	AgentMsgMCPServeStart  = "mcp_serve_start"  // Server -> Bridge: MCP server 제공 시작 요청
	AgentMsgMCPServeStop   = "mcp_serve_stop"   // Server -> Bridge: MCP server 제공 중지 요청
	AgentMsgMCPServeReady  = "mcp_serve_ready"  // Bridge -> Server: MCP server 시작 완료 (REQ-INTEGRATION-001)
	AgentMsgMCPServeResult = "mcp_serve_result" // Bridge -> Server: MCP server 제공 결과 (stop/error)

	// Tool Approval message types (SPEC-INTERACTIVE-CLI-001)
	AgentMsgToolApprovalReq  = "tool_approval_request"  // Bridge -> Server: 도구 승인 요청
	AgentMsgToolApprovalResp = "tool_approval_response" // Server -> Bridge: 도구 승인 응답

	// Agent Response Protocol message types (SPEC-BRIDGE-GATEWAY-001)
	// BridgeAgentExecutor가 사용하는 새로운 프로토콜. task_request 대비 스트리밍 전용 메시지 지원.
	AgentMsgAgentResponseReq      = "agent_response_request"  // Server -> Bridge: AI 에이전트 응답 요청
	AgentMsgAgentResponseStream   = "agent_response_stream"   // Bridge -> Server: 스트리밍 텍스트 청크
	AgentMsgAgentResponseComplete = "agent_response_complete" // Bridge -> Server: 최종 응답 완료
	AgentMsgAgentResponseError    = "agent_response_error"    // Bridge -> Server: 응답 에러

	// CodeOps message types (SPEC-CODEOPS-001)
	// REQ-003.7: CodeOpsRequest WebSocket 메시지 타입
	// REQ-003.8: CodeOpsResult WebSocket 메시지 타입
	AgentMsgCodeOpsRequest = "codeops_request" // Server -> Bridge: 코드 수정 요청
	AgentMsgCodeOpsResult  = "codeops_result"  // Bridge -> Server: 코드 수정 결과

	// CodingRelay message types (SPEC-CODING-RELAY-001)
	AgentMsgCodingRelayRequest  = "coding_relay_request"  // Server -> Bridge: 코딩 릴레이 시작 요청
	AgentMsgCodingRelayEvaluate = "coding_relay_evaluate" // Bridge -> Server: 이터레이션 결과 (Worker 평가 대기)
	AgentMsgCodingRelayFeedback = "coding_relay_feedback" // Server -> Bridge: Worker AI 피드백 또는 APPROVED
	AgentMsgCodingRelayComplete = "coding_relay_complete" // Bridge -> Server: 릴레이 완료 (성공/실패)
	AgentMsgCodingRelayError    = "coding_relay_error"    // Bridge -> Server: 릴레이 오류
	AgentMsgCodingRelayProgress = "coding_relay_progress" // Bridge -> Server: 이터레이션 진행 상황 업데이트
)

const (
	// AgentProtocolVersion is the shared backend<->bridge wire contract version.
	// Only minor/patch compatible peers should communicate on the same connection.
	AgentProtocolVersion = "1.1.0"
)

// AgentMessage is the envelope for all Local Agent WebSocket messages.
type AgentMessage struct {
	Type      string          `json:"type"`
	ID        string          `json:"id"`
	Timestamp time.Time       `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
	Signature string          `json:"signature,omitempty"` // HMAC-SHA256 서명 (SEC-P2-02)
}

// AgentConnectPayload is sent when a Local Agent connects.
type AgentConnectPayload struct {
	Version              string          `json:"version"`                         // Agent version
	ProtocolVersion      string          `json:"protocol_version,omitempty"`      // Shared wire contract version
	Capabilities         []string        `json:"capabilities"`                    // Supported CLI list
	ProviderCapabilities map[string]bool `json:"provider_capabilities,omitempty"` // 지원하는 프로바이더 목록 (SPEC-BRIDGE-GATEWAY-001)
	WorkspaceID          string          `json:"workspace_id,omitempty"`          // Selected workspace scope for this bridge session
	LastExecID           string          `json:"last_exec_id"`                    // Last processed execution ID (for reconnect)
	Token                string          `json:"token"`                           // JWT token for message-based auth (FR-P2-02)
}

// ConnectAckPayload is sent from server to agent after successful authentication.
type ConnectAckPayload struct {
	Success         bool   `json:"success"`
	Message         string `json:"message,omitempty"`
	ErrorCode       string `json:"error_code,omitempty"`       // "token_expired", "token_invalid", "protocol_version_mismatch"
	HMACSecret      string `json:"hmac_secret,omitempty"`      // HMAC 공유 시크릿 (SEC-P2-02)
	ProtocolVersion string `json:"protocol_version,omitempty"` // Shared wire contract version acknowledged by backend
}

// AgentDisconnectPayload is sent when a Local Agent disconnects.
type AgentDisconnectPayload struct {
	Reason string `json:"reason,omitempty"`
}

// AgentHeartbeatPayload is exchanged for keepalive.
type AgentHeartbeatPayload struct {
	Timestamp      time.Time `json:"timestamp"`
	MCPServeStatus string    `json:"mcp_serve_status,omitempty"` // "running" | "stopped" | "" (SPEC-AI-003 M3)
}

// TaskRequestPayload is sent from server to Local Agent to request execution.
type TaskRequestPayload struct {
	ExecutionID    string   `json:"execution_id"`
	Prompt         string   `json:"prompt"`
	SystemPrompt   string   `json:"system_prompt,omitempty"` // agent_response_request에서 전달되는 시스템 프롬프트
	Provider       string   `json:"provider,omitempty"`
	Model          string   `json:"model"`
	MaxTokens      int      `json:"max_tokens"`
	Tools          []string `json:"tools,omitempty"`
	Timeout        int      `json:"timeout_seconds"`
	WorkDir        string   `json:"work_dir,omitempty"`
	ApprovalPolicy string   `json:"approval_policy,omitempty"` // SPEC-INTERACTIVE-CLI-001: "auto-execute", "auto-approve", "agent-approve", "human-approve"
	ExecutionMode  string   `json:"execution_mode,omitempty"`  // SPEC-INTERACTIVE-CLI-001: "auto-execute", "interactive"
}

// TaskProgressPayload is sent from Local Agent to server for streaming updates.
type TaskProgressPayload struct {
	ExecutionID     string `json:"execution_id"`
	Progress        int    `json:"progress"` // 0-100
	Message         string `json:"message"`
	Type            string `json:"type"`                       // "text", "tool_use", etc.
	TextDelta       string `json:"text_delta,omitempty"`       // Incremental text chunk (streaming)
	AccumulatedText string `json:"accumulated_text,omitempty"` // Full text accumulated so far (streaming)
}

// TaskResultPayload is sent from Local Agent when execution completes.
type TaskResultPayload struct {
	ExecutionID string      `json:"execution_id"`
	Output      string      `json:"output"`
	ExitCode    int         `json:"exit_code"`
	Duration    int64       `json:"duration_ms"`
	TokenUsage  *TokenUsage `json:"token_usage,omitempty"`
	Error       string      `json:"error,omitempty"`
}

// TokenUsage tracks token consumption.
type TokenUsage struct {
	InputTokens   int `json:"input_tokens"`
	OutputTokens  int `json:"output_tokens"`
	TotalTokens   int `json:"total_tokens"`
	CacheRead     int `json:"cache_read,omitempty"`
	CacheCreation int `json:"cache_creation,omitempty"`
}

// TaskErrorPayload is sent when execution fails.
type TaskErrorPayload struct {
	ExecutionID string `json:"execution_id"`
	Code        string `json:"code"`
	Message     string `json:"message"`
	Retryable   bool   `json:"retryable"`
}

// BuildRequestPayload is sent from server to Local Agent to request a build (FR-P3-01).
type BuildRequestPayload struct {
	ExecutionID string   `json:"execution_id"`
	WorkDir     string   `json:"work_dir"`
	Command     string   `json:"command"`
	Env         []string `json:"env,omitempty"`
	Timeout     int      `json:"timeout_seconds"`
}

// BuildResultPayload is sent from Local Agent when a build completes (FR-P3-01).
type BuildResultPayload struct {
	ExecutionID string   `json:"execution_id"`
	Success     bool     `json:"success"`
	Output      string   `json:"output"`
	ExitCode    int      `json:"exit_code"`
	DurationMs  int64    `json:"duration_ms"`
	Artifacts   []string `json:"artifacts,omitempty"`
}

// TestRequestPayload is sent from server to Local Agent to request test execution (FR-P3-02).
type TestRequestPayload struct {
	ExecutionID string `json:"execution_id"`
	WorkDir     string `json:"work_dir"`
	Command     string `json:"command"`
	Pattern     string `json:"pattern,omitempty"`
	Timeout     int    `json:"timeout_seconds"`
}

// TestResultPayload is sent from Local Agent when test execution completes (FR-P3-02).
type TestResultPayload struct {
	ExecutionID string      `json:"execution_id"`
	Success     bool        `json:"success"`
	Output      string      `json:"output"`
	ExitCode    int         `json:"exit_code"`
	DurationMs  int64       `json:"duration_ms"`
	Summary     TestSummary `json:"summary"`
}

// TestSummary contains aggregated test result counts.
type TestSummary struct {
	Total   int `json:"total"`
	Passed  int `json:"passed"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped"`
}

// QARequestPayload is sent from server to Local Agent to request a QA pipeline (FR-P3-03).
type QARequestPayload struct {
	ExecutionID   string           `json:"execution_id"`
	WorkDir       string           `json:"work_dir"`
	BuildCommand  string           `json:"build_command,omitempty"`
	ServiceConfig *ServiceConfig   `json:"service_config,omitempty"`
	TestCommand   string           `json:"test_command,omitempty"`
	BrowserQA     *BrowserQAConfig `json:"browser_qa,omitempty"`
	Timeout       int              `json:"timeout_seconds"`
}

// ServiceConfig describes a service to start before running QA tests.
type ServiceConfig struct {
	Command      string `json:"command"`
	HealthCheck  string `json:"health_check"`
	ReadyTimeout int    `json:"ready_timeout"`
}

// BrowserQAConfig describes browser-based QA test parameters.
type BrowserQAConfig struct {
	Script     string `json:"script"`
	Browser    string `json:"browser"`
	Headless   bool   `json:"headless"`
	Screenshot bool   `json:"screenshot"`
	Video      bool   `json:"video"`
}

// QAResultPayload is sent from Local Agent when a QA pipeline completes (FR-P3-03).
type QAResultPayload struct {
	ExecutionID string          `json:"execution_id"`
	Success     bool            `json:"success"`
	DurationMs  int64           `json:"duration_ms"`
	Stages      []QAStageResult `json:"stages"`
	Screenshots []string        `json:"screenshots,omitempty"`
	Videos      []string        `json:"videos,omitempty"`
}

// QAStageResult contains the result of a single QA pipeline stage.
type QAStageResult struct {
	Name       string `json:"name"`
	Success    bool   `json:"success"`
	Output     string `json:"output"`
	DurationMs int64  `json:"duration_ms"`
	Error      string `json:"error,omitempty"`
}

// ComputerActionPayload represents a computer use action request from server to bridge.
type ComputerActionPayload struct {
	ExecutionID string                 `json:"execution_id"`
	SessionID   string                 `json:"session_id"`
	Action      string                 `json:"action"` // screenshot, click, type, scroll, navigate
	Params      map[string]interface{} `json:"params"`
}

// ComputerResultPayload represents a computer use action result from bridge to server.
type ComputerResultPayload struct {
	ExecutionID string `json:"execution_id"`
	SessionID   string `json:"session_id"`
	Success     bool   `json:"success"`
	Screenshot  string `json:"screenshot,omitempty"` // base64 PNG or JPEG
	Error       string `json:"error,omitempty"`
	DurationMs  int64  `json:"duration_ms"`
	ContainerID string `json:"container_id,omitempty"` // SPEC-COMPUTER-USE-002: 컨테이너 ID
}

// ComputerSessionPayload represents a computer use session start/end message.
type ComputerSessionPayload struct {
	ExecutionID string `json:"execution_id"`
	SessionID   string `json:"session_id"`
	URL         string `json:"url,omitempty"`
	ViewportW   int    `json:"viewport_w"`
	ViewportH   int    `json:"viewport_h"`
	Headless    bool   `json:"headless"`
	ContainerID string `json:"container_id,omitempty"` // SPEC-COMPUTER-USE-002: 컨테이너 ID
}

// ComputerPoolStatusPayload는 컨테이너 풀 상태를 보고한다.
// SPEC-COMPUTER-USE-002 REQ-C6-04
type ComputerPoolStatusPayload struct {
	WarmCount   int `json:"warm_count"`
	ActiveCount int `json:"active_count"`
	MaxCount    int `json:"max_count"`
}

// BrowserActionPayload represents an agent-browser action request from server to bridge.
type BrowserActionPayload struct {
	ExecutionID string                 `json:"execution_id"`
	SessionID   string                 `json:"session_id"`
	Command     string                 `json:"command"`
	Ref         *int                   `json:"ref,omitempty"`
	Params      map[string]interface{} `json:"params,omitempty"`
}

// BrowserResultPayload represents an agent-browser action result from bridge to server.
type BrowserResultPayload struct {
	ExecutionID string `json:"execution_id"`
	SessionID   string `json:"session_id"`
	Success     bool   `json:"success"`
	Snapshot    string `json:"snapshot,omitempty"`
	Screenshot  string `json:"screenshot,omitempty"`
	Output      string `json:"output,omitempty"`
	Error       string `json:"error,omitempty"`
	DurationMs  int64  `json:"duration_ms"`
}

// BrowserSessionPayload represents an agent-browser session lifecycle message.
type BrowserSessionPayload struct {
	ExecutionID string `json:"execution_id"`
	SessionID   string `json:"session_id"`
	URL         string `json:"url,omitempty"`
	Headless    bool   `json:"headless"`
	Status      string `json:"status"` // starting, ready, busy, error, stopped, not_available
}

// WorkerPhaseChangedPayload represents a worker phase transition event.
// SPEC-AUTONOMOUS-WORKER-001 REQ-M5-04
type WorkerPhaseChangedPayload struct {
	ExecutionID string `json:"execution_id"`
	Phase       string `json:"phase"`
	PhaseIndex  int    `json:"phase_index"`
	TotalPhases int    `json:"total_phases"`
	Iteration   int    `json:"iteration"`
	Status      string `json:"status"` // started, completed, failed
	Message     string `json:"message"`
}

// ToolApprovalRequestPayload is sent from Bridge to Server when a CLI tool
// requests approval for a tool execution (SPEC-INTERACTIVE-CLI-001).
type ToolApprovalRequestPayload struct {
	ExecutionID  string          `json:"execution_id"`
	ApprovalID   string          `json:"approval_id"`
	ProviderName string          `json:"provider_name"` // "claude", "codex", "gemini"
	ToolName     string          `json:"tool_name"`
	ToolInput    json.RawMessage `json:"tool_input"`
	SessionID    string          `json:"session_id"`
	RequestedAt  time.Time       `json:"requested_at"`
}

// ToolApprovalResponsePayload is sent from Server to Bridge with the
// approval decision for a pending tool execution (SPEC-INTERACTIVE-CLI-001).
type ToolApprovalResponsePayload struct {
	ExecutionID string    `json:"execution_id"`
	ApprovalID  string    `json:"approval_id"`
	Decision    string    `json:"decision"` // "allow" or "deny"
	Reason      string    `json:"reason,omitempty"`
	DecidedBy   string    `json:"decided_by"` // "policy", "agent", "human"
	DecidedAt   time.Time `json:"decided_at"`
}

// --- Agent Response Protocol payloads (SPEC-BRIDGE-GATEWAY-001) ---

// ToolDefinition is a provider-agnostic business tool schema.
type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema,omitempty"`
}

// ToolLoopCall is a model-requested business tool invocation.
type ToolLoopCall struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

// ToolLoopResult is the backend result for a tool invocation.
type ToolLoopResult struct {
	ToolCallID string `json:"tool_call_id"`
	ToolName   string `json:"tool_name,omitempty"`
	Content    string `json:"content"`
	IsError    bool   `json:"is_error,omitempty"`
}

// ToolLoopMessage is a provider-agnostic message item for native tool loops.
type ToolLoopMessage struct {
	Role        string           `json:"role"`
	Content     string           `json:"content,omitempty"`
	ToolCalls   []ToolLoopCall   `json:"tool_calls,omitempty"`
	ToolResults []ToolLoopResult `json:"tool_results,omitempty"`
}

// AgentResponseRequestPayload is sent from server to bridge to request an AI agent response.
// BridgeAgentExecutor가 사용하며, 플랫 JSON 형식(envelope 미사용)으로 수신된다.
type AgentResponseRequestPayload struct {
	Type             string            `json:"type"` // "agent_response_request" (플랫 형식에서 포함됨)
	ExecutionID      string            `json:"execution_id"`
	Prompt           string            `json:"prompt"`
	SystemPrompt     string            `json:"system_prompt,omitempty"`
	Provider         string            `json:"provider,omitempty"`
	Model            string            `json:"model"`
	MaxTokens        int               `json:"max_tokens"`
	Tools            []string          `json:"tools,omitempty"`
	Timeout          int               `json:"timeout"` // 초 단위 타임아웃
	WorkDir          string            `json:"work_dir,omitempty"`
	ApprovalPolicy   string            `json:"approval_policy,omitempty"`
	ExecutionMode    string            `json:"execution_mode,omitempty"`
	ResponseMode     string            `json:"response_mode,omitempty"`
	ToolLoopMessages []ToolLoopMessage `json:"tool_loop_messages,omitempty"`
	ToolDefinitions  []ToolDefinition  `json:"tool_definitions,omitempty"`
}

// AgentResponseStreamPayload is sent from bridge to server for streaming text chunks.
type AgentResponseStreamPayload struct {
	Type            string `json:"type,omitempty"`
	ExecutionID     string `json:"execution_id"`
	TextDelta       string `json:"text_delta,omitempty"`
	AccumulatedText string `json:"accumulated_text,omitempty"`
}

// AgentResponseCompletePayload is sent from bridge to server when agent response is complete.
type AgentResponseCompletePayload struct {
	Type        string         `json:"type,omitempty"`
	ExecutionID string         `json:"execution_id"`
	Output      string         `json:"output"`
	ExitCode    int            `json:"exit_code"`
	Duration    int64          `json:"duration_ms"`
	TokenUsage  *TokenUsage    `json:"token_usage,omitempty"`
	Model       string         `json:"model,omitempty"`
	Provider    string         `json:"provider,omitempty"`
	StopReason  string         `json:"stop_reason,omitempty"`
	ToolCalls   []ToolLoopCall `json:"tool_calls,omitempty"`
}

// AgentResponseErrorPayload is sent from bridge to server when agent response fails.
type AgentResponseErrorPayload struct {
	Type        string `json:"type,omitempty"`
	ExecutionID string `json:"execution_id"`
	Code        string `json:"code"`
	Message     string `json:"message"`
	Retryable   bool   `json:"retryable"`
}

// Authentication error codes for ConnectAckPayload.ErrorCode.
const (
	AuthErrorTokenExpired            = "token_expired"
	AuthErrorTokenInvalid            = "token_invalid"
	AuthErrorProtocolVersionMismatch = "protocol_version_mismatch"
	AuthErrorWorkspaceRequired       = "workspace_required"
)

// IsCompatibleProtocolVersion reports whether the given version is wire-compatible
// with the current shared contract. Missing versions are treated as legacy callers
// and should be handled by the caller according to rollout policy.
func IsCompatibleProtocolVersion(version string) bool {
	if version == "" {
		return false
	}
	wantMajor, wantMinor, err := parseProtocolVersion(AgentProtocolVersion)
	if err != nil {
		return false
	}
	gotMajor, gotMinor, err := parseProtocolVersion(version)
	if err != nil {
		return false
	}
	return wantMajor == gotMajor && wantMinor == gotMinor
}

func parseProtocolVersion(version string) (int, int, error) {
	parts := strings.Split(strings.TrimSpace(version), ".")
	if len(parts) < 2 {
		return 0, 0, fmt.Errorf("invalid protocol version %q", version)
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid major version %q", version)
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid minor version %q", version)
	}
	return major, minor, nil
}

// Computer Use constraints
const (
	MaxScreenshotSize             = 2 * 1024 * 1024 // 2MB
	ComputerUseMaxIdleMin         = 30              // 30 minutes idle timeout
	ComputerUseMaxActiveHr        = 2               // 2 hours max active session
	MaxConcurrentComputerSessions = 2               // per workspace
)

// CodingRelayRequestPayload는 Server가 Bridge에 코딩 릴레이 세션을 시작하도록 요청한다.
type CodingRelayRequestPayload struct {
	RequestID        string  `json:"request_id"`
	TaskDescription  string  `json:"task_description"`
	RepoConnectionID string  `json:"repo_connection_id"`
	SessionID        string  `json:"session_id,omitempty"`
	MaxIterations    int     `json:"max_iterations,omitempty"`
	WorkspaceID      string  `json:"workspace_id"`
	WorkerID         string  `json:"worker_id"`
	MaxBudgetUSD     float64 `json:"max_budget_usd,omitempty"`
}

// CodingRelayEvaluatePayload는 Bridge가 이터레이션 결과를 Server에 전달하고 Worker 평가를 요청한다.
type CodingRelayEvaluatePayload struct {
	RequestID    string   `json:"request_id"`
	Iteration    int      `json:"iteration"`
	Content      string   `json:"content"`
	DiffSummary  string   `json:"diff_summary"`
	TestOutput   string   `json:"test_output"`
	FilesChanged []string `json:"files_changed"`
	SessionID    string   `json:"session_id"`
}

// CodingRelayFeedbackPayload는 Server가 Worker AI 평가 결과(피드백 또는 승인)를 Bridge에 전달한다.
type CodingRelayFeedbackPayload struct {
	RequestID string `json:"request_id"`
	Feedback  string `json:"feedback"`
	Approved  bool   `json:"approved"`
}

// CodingRelayCompletePayload는 Bridge가 릴레이 세션 완료 결과를 Server에 전달한다.
type CodingRelayCompletePayload struct {
	RequestID    string   `json:"request_id"`
	Success      bool     `json:"success"`
	DiffSummary  string   `json:"diff_summary"`
	FilesChanged []string `json:"files_changed"`
	TestOutput   string   `json:"test_output"`
	SessionID    string   `json:"session_id"`
	PushedBranch string   `json:"pushed_branch,omitempty"`
	Iterations   int      `json:"iterations"`
	CostUSD      float64  `json:"cost_usd"`
}

// CodingRelayErrorPayload는 Bridge가 릴레이 세션 오류를 Server에 보고한다.
type CodingRelayErrorPayload struct {
	RequestID string `json:"request_id"`
	Error     string `json:"error"`
	Code      string `json:"code"`
	SessionID string `json:"session_id,omitempty"`
}

// CodingRelayProgressPayload는 Bridge가 이터레이션 진행 상황을 Server에 업데이트한다.
type CodingRelayProgressPayload struct {
	RequestID string `json:"request_id"`
	Iteration int    `json:"iteration"`
	Status    string `json:"status"`
	Message   string `json:"message"`
}
