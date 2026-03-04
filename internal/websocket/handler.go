// Package websocket는 Local Agent Bridge의 WebSocket 통신을 담당합니다.
// 메시지 라우팅 및 처리를 담당합니다.
package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/insajin/autopus-agent-protocol"
	"github.com/insajin/autopus-bridge/internal/agentbrowser"
	"github.com/insajin/autopus-bridge/internal/codegen"
	"github.com/insajin/autopus-bridge/internal/computeruse"
	"github.com/insajin/autopus-bridge/internal/mcp"
)

// MessageHandler는 WebSocket 메시지를 처리하는 인터페이스입니다.
type MessageHandler interface {
	// HandleMessage는 수신된 메시지를 처리합니다.
	HandleMessage(ctx context.Context, msg ws.AgentMessage) error
}

// TaskExecutor는 작업 실행을 담당하는 인터페이스입니다.
type TaskExecutor interface {
	// Execute는 작업을 실행합니다.
	Execute(ctx context.Context, task ws.TaskRequestPayload) (ws.TaskResultPayload, error)
}

// BuildExecutor는 빌드 실행을 담당하는 인터페이스입니다 (FR-P3-01).
type BuildExecutor interface {
	Execute(ctx context.Context, req ws.BuildRequestPayload) *ws.BuildResultPayload
}

// TestExecutor는 테스트 실행을 담당하는 인터페이스입니다 (FR-P3-02).
type TestExecutor interface {
	Execute(ctx context.Context, req ws.TestRequestPayload) *ws.TestResultPayload
}

// QAExecutor는 QA 파이프라인 실행을 담당하는 인터페이스입니다 (FR-P3-03).
type QAExecutor interface {
	Execute(ctx context.Context, req ws.QARequestPayload) *ws.QAResultPayload
}

// CLIExecutor는 CLI 명령어 실행을 담당하는 인터페이스입니다 (SPEC-SKILL-V2-001 Block C).
type CLIExecutor interface {
	Execute(ctx context.Context, req *ws.CLIRequestPayload) *ws.CLIResultPayload
}

// ProjectAnalyzer는 프로젝트 기술 스택 분석을 담당하는 인터페이스입니다 (SPEC-SKILL-V2-001 Block A).
type ProjectAnalyzer interface {
	Analyze(rootDir string) (*ws.ProjectContextPayload, error)
}

// MCPServerStarter는 MCP 서버 시작/중지를 담당하는 인터페이스입니다 (SPEC-SKILL-V2-001 Block D).
type MCPServerStarter interface {
	StartServer(ctx context.Context, name, command string, args []string, env map[string]string, workingDir string) (pid int, err error)
	StopServer(name string, force bool) error
}

// CodegenExecutor는 MCP 서버 코드 생성을 담당하는 인터페이스입니다 (SPEC-SELF-EXPAND-001).
type CodegenExecutor interface {
	Generate(ctx context.Context, req codegen.GenerateRequest, progressFn codegen.ProgressFn) (*codegen.GenerateResult, error)
}

// MCPDeployExecutor는 MCP 서버 배포를 담당하는 인터페이스입니다 (SPEC-SELF-EXPAND-001).
type MCPDeployExecutor interface {
	Deploy(ctx context.Context, serviceName string, files []mcp.DeployFile, envVars map[string]string) (string, error)
}

// HandlerFunc는 특정 메시지 타입에 대한 핸들러 함수입니다.
type HandlerFunc func(ctx context.Context, msg ws.AgentMessage) error

// Router는 메시지 타입에 따라 적절한 핸들러로 라우팅합니다.
type Router struct {
	// handlers는 메시지 타입별 핸들러 맵입니다.
	handlers map[string]HandlerFunc
	// handlersMu는 handlers 맵 접근을 보호하는 뮤텍스입니다.
	handlersMu sync.RWMutex

	// client는 WebSocket 클라이언트입니다 (응답 전송용).
	client *Client
	// executor는 작업 실행기입니다.
	executor TaskExecutor
	// buildExecutor는 빌드 실행기입니다 (FR-P3-01).
	buildExecutor BuildExecutor
	// testExecutor는 테스트 실행기입니다 (FR-P3-02).
	testExecutor TestExecutor
	// qaExecutor는 QA 파이프라인 실행기입니다 (FR-P3-03).
	qaExecutor QAExecutor
	// cliExecutor는 CLI 명령어 실행기입니다 (SPEC-SKILL-V2-001 Block C).
	cliExecutor CLIExecutor
	// projectAnalyzer는 프로젝트 기술 스택 분석기입니다 (SPEC-SKILL-V2-001 Block A).
	projectAnalyzer ProjectAnalyzer
	// mcpStarter는 MCP 서버 시작/중지를 담당합니다 (SPEC-SKILL-V2-001 Block D).
	mcpStarter MCPServerStarter

	// computerUseHandler는 Computer Use 핸들러입니다 (SPEC-COMPUTER-USE-001).
	computerUseHandler *computeruse.Handler

	// agentBrowserHandler는 Agent Browser 핸들러입니다 (SPEC-BROWSER-AGENT-001).
	agentBrowserHandler *agentbrowser.Handler

	// codegenExecutor는 MCP 서버 코드 생성을 담당합니다 (SPEC-SELF-EXPAND-001).
	codegenExecutor CodegenExecutor
	// mcpDeployer는 MCP 서버 배포를 담당합니다 (SPEC-SELF-EXPAND-001).
	mcpDeployer MCPDeployExecutor
	// codegenSandboxBaseDir는 코드 생성 샌드박스 기본 디렉토리입니다.
	codegenSandboxBaseDir string

	// onError는 에러 발생 시 호출되는 콜백입니다.
	onError func(err error)
}

// RouterOption은 Router 설정 옵션입니다.
type RouterOption func(*Router)

// WithTaskExecutor는 작업 실행기를 설정합니다.
func WithTaskExecutor(executor TaskExecutor) RouterOption {
	return func(r *Router) {
		r.executor = executor
	}
}

// WithBuildExecutor는 빌드 실행기를 설정합니다 (FR-P3-01).
func WithBuildExecutor(executor BuildExecutor) RouterOption {
	return func(r *Router) {
		r.buildExecutor = executor
	}
}

// WithTestExecutor는 테스트 실행기를 설정합니다 (FR-P3-02).
func WithTestExecutor(executor TestExecutor) RouterOption {
	return func(r *Router) {
		r.testExecutor = executor
	}
}

// WithQAExecutor는 QA 파이프라인 실행기를 설정합니다 (FR-P3-03).
func WithQAExecutor(executor QAExecutor) RouterOption {
	return func(r *Router) {
		r.qaExecutor = executor
	}
}

// WithCLIExecutor는 CLI 명령어 실행기를 설정합니다 (SPEC-SKILL-V2-001 Block C).
func WithCLIExecutor(executor CLIExecutor) RouterOption {
	return func(r *Router) {
		r.cliExecutor = executor
	}
}

// WithProjectAnalyzer는 프로젝트 기술 스택 분석기를 설정합니다 (SPEC-SKILL-V2-001 Block A).
func WithProjectAnalyzer(analyzer ProjectAnalyzer) RouterOption {
	return func(r *Router) {
		r.projectAnalyzer = analyzer
	}
}

// WithMCPStarter는 MCP 서버 시작/중지를 설정합니다 (SPEC-SKILL-V2-001 Block D).
func WithMCPStarter(starter MCPServerStarter) RouterOption {
	return func(r *Router) {
		r.mcpStarter = starter
	}
}

// WithComputerUseHandler는 Computer Use 핸들러를 설정합니다 (SPEC-COMPUTER-USE-001).
func WithComputerUseHandler(handler *computeruse.Handler) RouterOption {
	return func(r *Router) {
		r.computerUseHandler = handler
	}
}

// WithAgentBrowserHandler는 Agent Browser 핸들러를 설정합니다 (SPEC-BROWSER-AGENT-001).
func WithAgentBrowserHandler(handler *agentbrowser.Handler) RouterOption {
	return func(r *Router) {
		r.agentBrowserHandler = handler
	}
}

// WithErrorHandler는 에러 핸들러를 설정합니다.
func WithErrorHandler(handler func(err error)) RouterOption {
	return func(r *Router) {
		r.onError = handler
	}
}

// WithCodegenExecutor는 MCP 서버 코드 생성기를 설정합니다 (SPEC-SELF-EXPAND-001).
func WithCodegenExecutor(executor CodegenExecutor) RouterOption {
	return func(r *Router) {
		r.codegenExecutor = executor
	}
}

// WithMCPDeployer는 MCP 서버 배포기를 설정합니다 (SPEC-SELF-EXPAND-001).
func WithMCPDeployer(deployer MCPDeployExecutor) RouterOption {
	return func(r *Router) {
		r.mcpDeployer = deployer
	}
}

// WithCodegenSandboxBaseDir는 코드 생성 샌드박스 기본 디렉토리를 설정합니다.
func WithCodegenSandboxBaseDir(dir string) RouterOption {
	return func(r *Router) {
		r.codegenSandboxBaseDir = dir
	}
}

// NewRouter는 새로운 메시지 라우터를 생성합니다.
func NewRouter(client *Client, opts ...RouterOption) *Router {
	r := &Router{
		handlers: make(map[string]HandlerFunc),
		client:   client,
	}

	for _, opt := range opts {
		opt(r)
	}

	// 기본 핸들러 등록
	r.registerDefaultHandlers()

	return r
}

// registerDefaultHandlers는 기본 메시지 핸들러를 등록합니다.
func (r *Router) registerDefaultHandlers() {
	// 하트비트 응답 핸들러
	r.RegisterHandler(ws.AgentMsgHeartbeat, r.handleHeartbeat)

	// 작업 요청 핸들러
	r.RegisterHandler(ws.AgentMsgTaskReq, r.handleTaskRequest)

	// 빌드 요청 핸들러 (FR-P3-01)
	r.RegisterHandler(ws.AgentMsgBuildReq, r.handleBuildRequest)

	// 테스트 요청 핸들러 (FR-P3-02)
	r.RegisterHandler(ws.AgentMsgTestReq, r.handleTestRequest)

	// QA 요청 핸들러 (FR-P3-03)
	r.RegisterHandler(ws.AgentMsgQAReq, r.handleQARequest)

	// CLI 요청 핸들러 (SPEC-SKILL-V2-001 Block C)
	r.RegisterHandler(ws.AgentMsgCLIRequest, r.handleCLIRequest)

	// MCP 서버 관리 핸들러 (SPEC-SKILL-V2-001 Block D)
	r.RegisterHandler(ws.AgentMsgMCPStart, r.handleMCPStart)
	r.RegisterHandler(ws.AgentMsgMCPStop, r.handleMCPStop)

	// Computer Use 핸들러 (SPEC-COMPUTER-USE-001)
	r.RegisterHandler(ws.AgentMsgComputerSessionStart, r.handleComputerSessionStart)
	r.RegisterHandler(ws.AgentMsgComputerAction, r.handleComputerAction)
	r.RegisterHandler(ws.AgentMsgComputerSessionEnd, r.handleComputerSessionEnd)

	// Agent Browser 핸들러 (SPEC-BROWSER-AGENT-001)
	r.RegisterHandler(agentMsgBrowserSessionStart, r.handleBrowserSessionStart)
	r.RegisterHandler(agentMsgBrowserAction, r.handleBrowserAction)
	r.RegisterHandler(agentMsgBrowserSessionEnd, r.handleBrowserSessionEnd)

	// MCP Codegen/Deploy 핸들러 (SPEC-SELF-EXPAND-001)
	r.RegisterHandler(ws.AgentMsgMCPCodegenRequest, r.handleMCPCodegenRequest)
	r.RegisterHandler(ws.AgentMsgMCPDeploy, r.handleMCPDeploy)

	// Agent Response Protocol 핸들러 (SPEC-BRIDGE-GATEWAY-001)
	r.RegisterHandler(ws.AgentMsgAgentResponseReq, r.handleAgentResponseRequest)
}

// RegisterHandler는 메시지 타입에 대한 핸들러를 등록합니다.
func (r *Router) RegisterHandler(msgType string, handler HandlerFunc) {
	r.handlersMu.Lock()
	defer r.handlersMu.Unlock()
	r.handlers[msgType] = handler
}

// HandleMessage는 수신된 메시지를 적절한 핸들러로 라우팅합니다.
// MessageHandler 인터페이스 구현.
func (r *Router) HandleMessage(ctx context.Context, msg ws.AgentMessage) error {
	r.handlersMu.RLock()
	handler, exists := r.handlers[msg.Type]
	r.handlersMu.RUnlock()

	if !exists {
		log.Printf("[handler] 미등록 메시지 타입: type=%s", msg.Type)
		return nil
	}


	if err := handler(ctx, msg); err != nil {
		if r.onError != nil {
			r.onError(fmt.Errorf("메시지 처리 실패 (type=%s): %w", msg.Type, err))
		}
		return err
	}

	return nil
}

// handleHeartbeat는 하트비트 메시지를 처리합니다.
func (r *Router) handleHeartbeat(ctx context.Context, msg ws.AgentMessage) error {
	// 하트비트는 client.readLoop에서 직접 처리되므로
	// 여기서는 추가 처리가 필요 없음
	return nil
}

// handleTaskRequest는 작업 요청 메시지를 처리합니다.
func (r *Router) handleTaskRequest(ctx context.Context, msg ws.AgentMessage) error {
	var task ws.TaskRequestPayload
	if err := json.Unmarshal(msg.Payload, &task); err != nil {
		// 페이로드 파싱 실패 시 에러 응답
		errPayload := ws.TaskErrorPayload{
			ExecutionID: "",
			Code:        "INVALID_PAYLOAD",
			Message:     fmt.Sprintf("작업 요청 페이로드 파싱 실패: %v", err),
			Retryable:   false,
		}
		return r.client.SendTaskError(errPayload)
	}

	// FR-P2-04: 태스크 추적 시작
	r.client.TaskTracker().Track(task.ExecutionID, "task")

	// 작업 실행기가 없으면 에러 응답
	if r.executor == nil {
		r.client.TaskTracker().Complete(task.ExecutionID)
		errPayload := ws.TaskErrorPayload{
			ExecutionID: task.ExecutionID,
			Code:        "NO_EXECUTOR",
			Message:     "작업 실행기가 설정되지 않았습니다",
			Retryable:   false,
		}
		return r.client.SendTaskError(errPayload)
	}

	// 비동기로 작업 실행
	go r.executeTask(ctx, task)

	return nil
}

// executeTask는 작업을 실행하고 결과를 전송합니다.
func (r *Router) executeTask(ctx context.Context, task ws.TaskRequestPayload) {
	// FR-P2-04: 작업 완료 시 추적 목록에서 제거
	defer r.client.TaskTracker().Complete(task.ExecutionID)

	// 진행 상황 전송: 시작
	_ = r.client.SendTaskProgress(ws.TaskProgressPayload{
		ExecutionID: task.ExecutionID,
		Progress:    0,
		Message:     "작업 시작",
		Type:        "text",
	})

	// 작업 실행
	result, err := r.executor.Execute(ctx, task)
	if err != nil {
		log.Printf("[task-request] 실행 실패: execution_id=%s provider=%s model=%s err=%v", task.ExecutionID, task.Provider, task.Model, err)
		// 실행 실패 시 에러 응답: TaskError의 구체적 에러 코드를 전파
		code := "EXECUTION_ERROR"
		type codeError interface{ ErrorCode() string }
		if ce, ok := err.(codeError); ok {
			code = ce.ErrorCode()
		}
		errPayload := ws.TaskErrorPayload{
			ExecutionID: task.ExecutionID,
			Code:        code,
			Message:     err.Error(),
			Retryable:   isRetryableError(err),
		}
		_ = r.client.SendTaskError(errPayload)
		return
	}

	// 결과 전송
	_ = r.client.SendTaskResult(result)
}

// handleAgentResponseRequest는 에이전트 응답 요청 메시지를 처리합니다 (SPEC-BRIDGE-GATEWAY-001).
// BridgeAgentExecutor가 보내는 새 프로토콜로, task_request와 동일한 실행 흐름이나
// agent_response_stream/complete/error 메시지 타입으로 응답한다.
func (r *Router) handleAgentResponseRequest(ctx context.Context, msg ws.AgentMessage) error {
	// 디버그: 원시 메시지 크기만 로깅 (system_prompt가 매우 클 수 있음)
	log.Printf("[agent-response] 수신: payload_size=%d bytes", len(msg.Payload))

	var req ws.AgentResponseRequestPayload
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		log.Printf("[agent-response] 페이로드 파싱 실패: %v", err)
		errPayload := ws.AgentResponseErrorPayload{
			ExecutionID: "",
			Code:        "INVALID_PAYLOAD",
			Message:     fmt.Sprintf("agent_response_request 페이로드 파싱 실패: %v", err),
			Retryable:   false,
		}
		return r.client.SendAgentResponseError(errPayload)
	}

	log.Printf("[agent-response] 파싱 완료: execution_id=%s provider=%s model=%s", req.ExecutionID, req.Provider, req.Model)

	// 태스크 추적 시작
	r.client.TaskTracker().Track(req.ExecutionID, "agent_response")

	// 작업 실행기가 없으면 에러 응답
	if r.executor == nil {
		r.client.TaskTracker().Complete(req.ExecutionID)
		errPayload := ws.AgentResponseErrorPayload{
			ExecutionID: req.ExecutionID,
			Code:        "NO_EXECUTOR",
			Message:     "작업 실행기가 설정되지 않았습니다",
			Retryable:   false,
		}
		return r.client.SendAgentResponseError(errPayload)
	}

	// AgentResponseRequestPayload -> TaskRequestPayload로 변환하여 기존 실행기 재사용
	task := ws.TaskRequestPayload{
		ExecutionID:    req.ExecutionID,
		Prompt:         req.Prompt,
		SystemPrompt:   req.SystemPrompt,
		Provider:       req.Provider,
		Model:          req.Model,
		MaxTokens:      req.MaxTokens,
		Tools:          req.Tools,
		Timeout:        req.Timeout, // agent_response_request의 "timeout"을 TaskRequestPayload의 "timeout_seconds"에 매핑
		WorkDir:        req.WorkDir,
		ApprovalPolicy: req.ApprovalPolicy,
		ExecutionMode:  req.ExecutionMode,
	}

	// 비동기로 작업 실행 (응답은 agent_response_* 메시지 타입 사용)
	log.Printf("[agent-response] 비동기 실행 시작: execution_id=%s", req.ExecutionID)
	go r.executeAgentResponse(ctx, task)

	return nil
}

// executeAgentResponse는 에이전트 응답 요청을 실행하고 결과를 전송합니다 (SPEC-BRIDGE-GATEWAY-001).
// executeTask와 동일한 실행 흐름이나 agent_response_* 메시지 타입으로 응답한다.
func (r *Router) executeAgentResponse(ctx context.Context, task ws.TaskRequestPayload) {
	defer r.client.TaskTracker().Complete(task.ExecutionID)
	log.Printf("[agent-response] 실행 시작: execution_id=%s model=%s", task.ExecutionID, task.Model)

	// 작업 실행
	result, err := r.executor.Execute(ctx, task)
	if err != nil {
		log.Printf("[agent-response] 실행 에러: execution_id=%s err=%v", task.ExecutionID, err)
		code := "EXECUTION_ERROR"
		type codeError interface{ ErrorCode() string }
		if ce, ok := err.(codeError); ok {
			code = ce.ErrorCode()
		}
		errPayload := ws.AgentResponseErrorPayload{
			ExecutionID: task.ExecutionID,
			Code:        code,
			Message:     err.Error(),
			Retryable:   isRetryableError(err),
		}
		_ = r.client.SendAgentResponseError(errPayload)
		return
	}

	// 완료 응답 전송
	log.Printf("[agent-response] 실행 완료: execution_id=%s duration=%dms", task.ExecutionID, result.Duration)
	completePayload := ws.AgentResponseCompletePayload{
		ExecutionID: task.ExecutionID,
		Output:      result.Output,
		ExitCode:    result.ExitCode,
		Duration:    result.Duration,
		TokenUsage:  result.TokenUsage,
	}
	_ = r.client.SendAgentResponseComplete(completePayload)
}

// retryable은 재시도 가능 여부를 노출하는 에러 인터페이스입니다.
// executor.TaskError 등 Retryable 정보를 포함하는 에러 타입과 호환됩니다.
type retryable interface {
	IsRetryable() bool
}

// isRetryableError는 에러 타입에 따라 재시도 가능 여부를 결정합니다.
//
// 재시도 가능:
//   - 타임아웃 (context.DeadlineExceeded, net.Error.Timeout)
//   - 일시적 네트워크 장애 (connection reset/refused, io.EOF)
//   - 서버 에러 (500, 502, 503, 504)
//   - 레이트 리밋 (429)
//
// 재시도 불가:
//   - 컨텍스트 취소 (context.Canceled) - 의도적 취소
//   - 인증 실패 (401, 403)
//   - 잘못된 요청 (400)
//   - 리소스 없음 (404)
//   - 유효성 검사 에러
//   - API 키 미설정
//   - 프로바이더 미등록
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// 1. retryable 인터페이스 구현 여부 확인 (executor.TaskError 등)
	var r retryable
	if errors.As(err, &r) {
		return r.IsRetryable()
	}

	// 2. 컨텍스트 에러 확인
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, os.ErrDeadlineExceeded) {
		return true // 타임아웃은 재시도 가능
	}
	if errors.Is(err, context.Canceled) {
		return false // 의도적 취소는 재시도 불가
	}

	// 3. 네트워크 에러 확인
	var netErr net.Error
	if errors.As(err, &netErr) {
		// 타임아웃은 재시도 가능
		if netErr.Timeout() {
			return true
		}
		// 그 외 네트워크 에러도 일시적일 가능성 높음
		return true
	}

	// 4. 연결 끊김 에러 확인 (io.EOF, io.ErrUnexpectedEOF)
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}

	// 5. 에러 메시지 기반 분류 (래핑된 에러 또는 문자열 기반 에러)
	errStr := strings.ToLower(err.Error())

	// 5a. 재시도 불가 패턴 우선 확인 (보수적 접근)
	nonRetryablePatterns := []string{
		"401", "403", "404", "400",
		"unauthorized", "forbidden", "not found", "bad request",
		"invalid", "validation",
		"api 키", "api key", "no api key",
		"프로바이더를 찾을 수 없", "provider not found",
		"sandbox", "permission denied",
	}
	for _, pattern := range nonRetryablePatterns {
		if strings.Contains(errStr, pattern) {
			return false
		}
	}

	// 5b. 재시도 가능 패턴 확인
	retryablePatterns := []string{
		"500", "502", "503", "504", "429",
		"timeout", "timed out",
		"connection refused", "connection reset",
		"broken pipe", "no such host",
		"temporary", "unavailable",
		"server error", "server_error",
		"internal", "overloaded",
		"rate limit", "rate_limit", "레이트 리밋",
		"too many requests",
		"eof",
	}
	for _, pattern := range retryablePatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}

	// 6. 기본값: 재시도 불가 (보수적)
	return false
}

// handleBuildRequest는 빌드 요청 메시지를 처리합니다 (FR-P3-01).
func (r *Router) handleBuildRequest(ctx context.Context, msg ws.AgentMessage) error {
	var req ws.BuildRequestPayload
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		errPayload := ws.TaskErrorPayload{
			ExecutionID: "",
			Code:        "INVALID_PAYLOAD",
			Message:     fmt.Sprintf("빌드 요청 페이로드 파싱 실패: %v", err),
			Retryable:   false,
		}
		return r.client.SendTaskError(errPayload)
	}

	// FR-P2-04: 빌드 태스크 추적 시작
	r.client.TaskTracker().Track(req.ExecutionID, "build")

	if r.buildExecutor == nil {
		r.client.TaskTracker().Complete(req.ExecutionID)
		errPayload := ws.TaskErrorPayload{
			ExecutionID: req.ExecutionID,
			Code:        "NO_EXECUTOR",
			Message:     "빌드 실행기가 설정되지 않았습니다",
			Retryable:   false,
		}
		return r.client.SendTaskError(errPayload)
	}

	// 비동기로 빌드 실행
	go func() {
		defer r.client.TaskTracker().Complete(req.ExecutionID) // FR-P2-04
		result := r.buildExecutor.Execute(ctx, req)
		_ = r.client.SendBuildResult(*result)
	}()

	return nil
}

// handleTestRequest는 테스트 요청 메시지를 처리합니다 (FR-P3-02).
func (r *Router) handleTestRequest(ctx context.Context, msg ws.AgentMessage) error {
	var req ws.TestRequestPayload
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		errPayload := ws.TaskErrorPayload{
			ExecutionID: "",
			Code:        "INVALID_PAYLOAD",
			Message:     fmt.Sprintf("테스트 요청 페이로드 파싱 실패: %v", err),
			Retryable:   false,
		}
		return r.client.SendTaskError(errPayload)
	}

	// FR-P2-04: 테스트 태스크 추적 시작
	r.client.TaskTracker().Track(req.ExecutionID, "test")

	if r.testExecutor == nil {
		r.client.TaskTracker().Complete(req.ExecutionID)
		errPayload := ws.TaskErrorPayload{
			ExecutionID: req.ExecutionID,
			Code:        "NO_EXECUTOR",
			Message:     "테스트 실행기가 설정되지 않았습니다",
			Retryable:   false,
		}
		return r.client.SendTaskError(errPayload)
	}

	// 비동기로 테스트 실행
	go func() {
		defer r.client.TaskTracker().Complete(req.ExecutionID) // FR-P2-04
		result := r.testExecutor.Execute(ctx, req)
		_ = r.client.SendTestResult(*result)
	}()

	return nil
}

// handleQARequest는 QA 요청 메시지를 처리합니다 (FR-P3-03).
func (r *Router) handleQARequest(ctx context.Context, msg ws.AgentMessage) error {
	var req ws.QARequestPayload
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		errPayload := ws.TaskErrorPayload{
			ExecutionID: "",
			Code:        "INVALID_PAYLOAD",
			Message:     fmt.Sprintf("QA 요청 페이로드 파싱 실패: %v", err),
			Retryable:   false,
		}
		return r.client.SendTaskError(errPayload)
	}

	// FR-P2-04: QA 태스크 추적 시작
	r.client.TaskTracker().Track(req.ExecutionID, "qa")

	if r.qaExecutor == nil {
		r.client.TaskTracker().Complete(req.ExecutionID)
		errPayload := ws.TaskErrorPayload{
			ExecutionID: req.ExecutionID,
			Code:        "NO_EXECUTOR",
			Message:     "QA 실행기가 설정되지 않았습니다",
			Retryable:   false,
		}
		return r.client.SendTaskError(errPayload)
	}

	// 비동기로 QA 파이프라인 실행
	go func() {
		defer r.client.TaskTracker().Complete(req.ExecutionID) // FR-P2-04
		result := r.qaExecutor.Execute(ctx, req)
		_ = r.client.SendQAResult(*result)
	}()

	return nil
}

// handleComputerSessionStart는 Computer Use 세션 시작 메시지를 처리합니다 (SPEC-COMPUTER-USE-001).
func (r *Router) handleComputerSessionStart(ctx context.Context, msg ws.AgentMessage) error {
	var payload ws.ComputerSessionPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		errPayload := ws.TaskErrorPayload{
			ExecutionID: "",
			Code:        "INVALID_PAYLOAD",
			Message:     fmt.Sprintf("computer session start 페이로드 파싱 실패: %v", err),
			Retryable:   false,
		}
		return r.client.SendTaskError(errPayload)
	}

	if r.computerUseHandler == nil {
		errPayload := ws.TaskErrorPayload{
			ExecutionID: payload.ExecutionID,
			Code:        "NO_HANDLER",
			Message:     "Computer Use 핸들러가 설정되지 않았습니다",
			Retryable:   false,
		}
		return r.client.SendTaskError(errPayload)
	}

	if err := r.computerUseHandler.HandleSessionStart(ctx, payload); err != nil {
		result := ws.ComputerResultPayload{
			ExecutionID: payload.ExecutionID,
			SessionID:   payload.SessionID,
			Success:     false,
			Error:       err.Error(),
			DurationMs:  0,
		}
		return r.client.SendComputerResult(result)
	}

	return nil
}

// handleComputerAction은 Computer Use 액션 메시지를 처리합니다 (SPEC-COMPUTER-USE-001).
func (r *Router) handleComputerAction(ctx context.Context, msg ws.AgentMessage) error {
	var payload ws.ComputerActionPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		errPayload := ws.TaskErrorPayload{
			ExecutionID: "",
			Code:        "INVALID_PAYLOAD",
			Message:     fmt.Sprintf("computer action 페이로드 파싱 실패: %v", err),
			Retryable:   false,
		}
		return r.client.SendTaskError(errPayload)
	}

	if r.computerUseHandler == nil {
		result := ws.ComputerResultPayload{
			ExecutionID: payload.ExecutionID,
			SessionID:   payload.SessionID,
			Success:     false,
			Error:       "Computer Use 핸들러가 설정되지 않았습니다",
			DurationMs:  0,
		}
		return r.client.SendComputerResult(result)
	}

	// 비동기로 액션 실행 (REQ-M3-04: 결과를 세션에 큐잉 후 전송)
	go func() {
		start := time.Now()
		result, err := r.computerUseHandler.HandleAction(ctx, payload)

		var resultPayload ws.ComputerResultPayload
		if err != nil {
			resultPayload = ws.ComputerResultPayload{
				ExecutionID: payload.ExecutionID,
				SessionID:   payload.SessionID,
				Success:     false,
				Error:       err.Error(),
				DurationMs:  time.Since(start).Milliseconds(),
			}
		} else {
			resultPayload = *result
		}

		// REQ-M3-04: 전송 전에 세션 pending 큐에 저장
		session, exists := r.computerUseHandler.SessionManager().GetSession(payload.SessionID)
		if exists && session != nil {
			session.QueueResult(resultPayload)
		}

		// 전송 시도 - 성공 시 pending 큐에서 제거
		if sendErr := r.client.SendComputerResult(resultPayload); sendErr == nil {
			if exists && session != nil {
				session.DrainPendingResults()
			}
		} else {
			log.Printf("[computer-use] failed to send result for session %s, queued for reconnection: %v",
				payload.SessionID, sendErr)
		}
	}()

	return nil
}

// handleComputerSessionEnd는 Computer Use 세션 종료 메시지를 처리합니다 (SPEC-COMPUTER-USE-001).
func (r *Router) handleComputerSessionEnd(ctx context.Context, msg ws.AgentMessage) error {
	var payload ws.ComputerSessionPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		errPayload := ws.TaskErrorPayload{
			ExecutionID: "",
			Code:        "INVALID_PAYLOAD",
			Message:     fmt.Sprintf("computer session end 페이로드 파싱 실패: %v", err),
			Retryable:   false,
		}
		return r.client.SendTaskError(errPayload)
	}

	if r.computerUseHandler == nil {
		return nil
	}

	if err := r.computerUseHandler.HandleSessionEnd(ctx, payload); err != nil {
		if r.onError != nil {
			r.onError(fmt.Errorf("computer session end 실패 (session=%s): %w", payload.SessionID, err))
		}
	}

	return nil
}

// OnReconnected restores Computer Use and Agent Browser session state after a
// WebSocket reconnection. For each active session it notifies the server that
// the session is still alive and resends any pending action results that were
// not delivered before the connection dropped (REQ-M3-04).
//
// Implements the ReconnectionHandler interface.
func (r *Router) OnReconnected(ctx context.Context) error {
	// Computer Use 세션 복원
	if r.computerUseHandler != nil {
		sessions := r.computerUseHandler.GetActiveSessions()
		if len(sessions) > 0 {
			log.Printf("[computer-use] reconnected: restoring %d active session(s)", len(sessions))

			for _, session := range sessions {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
				}

				// 서버에 활성 세션 알림
				sessionPayload := ws.ComputerSessionPayload{
					ExecutionID: session.ExecutionID,
					SessionID:   session.ID,
					URL:         session.URL,
					ViewportW:   session.ViewportW,
					ViewportH:   session.ViewportH,
					Headless:    session.Headless,
				}
				if err := r.client.sendMessage(ws.AgentMsgComputerSessionStart, sessionPayload); err != nil {
					log.Printf("[computer-use] failed to restore session %s: %v", session.ID, err)
					continue
				}

				log.Printf("[computer-use] session %s restored", session.ID)

				// 미전송 결과 재전송
				pending := session.DrainPendingResults()
				for _, p := range pending {
					if result, ok := p.Payload.(ws.ComputerResultPayload); ok {
						if err := r.client.SendComputerResult(result); err != nil {
							log.Printf("[computer-use] failed to resend pending result for session %s, re-queuing: %v",
								session.ID, err)
							// 재전송 실패 시 다시 큐에 넣기
							session.QueueResult(result)
						} else {
							log.Printf("[computer-use] resent pending result for session %s (age=%v)",
								session.ID, time.Since(p.CreatedAt))
						}
					}
				}
			}
		}
	}

	// Agent Browser 세션 복원 (SPEC-BROWSER-AGENT-001)
	if r.agentBrowserHandler != nil {
		sessions := r.agentBrowserHandler.GetActiveSessions()
		if len(sessions) > 0 {
			log.Printf("[agent-browser] reconnected: restoring %d active session(s)", len(sessions))

			for _, session := range sessions {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
				}

				// 서버에 활성 세션 알림
				sessionPayload := agentbrowser.BrowserSessionPayload{
					ExecutionID: session.ExecutionID,
					SessionID:   session.ID,
					URL:         session.URL,
					Headless:    session.Headless,
				}
				if err := r.client.sendMessage(agentMsgBrowserSessionStart, sessionPayload); err != nil {
					log.Printf("[agent-browser] failed to restore session %s: %v", session.ID, err)
					continue
				}

				log.Printf("[agent-browser] session %s restored", session.ID)

				// 미전송 결과 재전송
				pending := session.DrainPendingResults()
				for _, p := range pending {
					if result, ok := p.Payload.(agentbrowser.BrowserResultPayload); ok {
						if err := r.client.sendMessage(agentMsgBrowserResult, result); err != nil {
							log.Printf("[agent-browser] failed to resend pending result for session %s, re-queuing: %v",
								session.ID, err)
							session.QueueResult(result)
						} else {
							log.Printf("[agent-browser] resent pending result for session %s (age=%v)",
								session.ID, time.Since(p.CreatedAt))
						}
					}
				}
			}
		}
	}

	return nil
}

// Agent Browser 메시지 타입 상수 (SPEC-BROWSER-AGENT-001).
// autopus-agent-protocol 패키지에 정의될 때까지 로컬 상수를 유지한다.
const (
	agentMsgBrowserSessionStart = "browser_session_start"
	agentMsgBrowserAction       = "browser_action"
	agentMsgBrowserSessionEnd   = "browser_session_end"
	agentMsgBrowserSessionReady = "browser_session_ready"
	agentMsgBrowserResult       = "browser_result"
	agentMsgBrowserNotAvailable = "browser_not_available"
	agentMsgBrowserError        = "browser_error"
)

// handleBrowserSessionStart는 Agent Browser 세션 시작 메시지를 처리합니다 (SPEC-BROWSER-AGENT-001).
func (r *Router) handleBrowserSessionStart(ctx context.Context, msg ws.AgentMessage) error {
	var payload agentbrowser.BrowserSessionPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		errPayload := ws.TaskErrorPayload{
			ExecutionID: "",
			Code:        "INVALID_PAYLOAD",
			Message:     fmt.Sprintf("browser session start 페이로드 파싱 실패: %v", err),
			Retryable:   false,
		}
		return r.client.SendTaskError(errPayload)
	}

	if r.agentBrowserHandler == nil {
		errPayload := ws.TaskErrorPayload{
			ExecutionID: payload.ExecutionID,
			Code:        "NO_HANDLER",
			Message:     "Agent Browser 핸들러가 설정되지 않았습니다",
			Retryable:   false,
		}
		return r.client.SendTaskError(errPayload)
	}

	result, err := r.agentBrowserHandler.HandleSessionStart(ctx, payload)
	if err != nil {
		// not_available 또는 에러 응답
		msgType := agentMsgBrowserError
		if result != nil && result.Status == "not_available" {
			msgType = agentMsgBrowserNotAvailable
		}
		if result != nil {
			_ = r.client.sendMessage(msgType, result)
		}
		if r.onError != nil {
			r.onError(fmt.Errorf("browser session start 실패 (session=%s): %w", payload.SessionID, err))
		}
		return nil
	}

	// browser_session_ready 응답
	return r.client.sendMessage(agentMsgBrowserSessionReady, result)
}

// handleBrowserAction은 Agent Browser 액션 메시지를 처리합니다 (SPEC-BROWSER-AGENT-001).
func (r *Router) handleBrowserAction(ctx context.Context, msg ws.AgentMessage) error {
	var payload agentbrowser.BrowserActionPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		errPayload := ws.TaskErrorPayload{
			ExecutionID: "",
			Code:        "INVALID_PAYLOAD",
			Message:     fmt.Sprintf("browser action 페이로드 파싱 실패: %v", err),
			Retryable:   false,
		}
		return r.client.SendTaskError(errPayload)
	}

	if r.agentBrowserHandler == nil {
		resultPayload := agentbrowser.BrowserResultPayload{
			ExecutionID: payload.ExecutionID,
			SessionID:   payload.SessionID,
			Success:     false,
			Error:       "Agent Browser 핸들러가 설정되지 않았습니다",
			DurationMs:  0,
		}
		return r.client.sendMessage(agentMsgBrowserResult, resultPayload)
	}

	// 비동기로 액션 실행 (REQ-M3-04: 결과를 세션에 큐잉 후 전송)
	go func() {
		result, err := r.agentBrowserHandler.HandleAction(ctx, payload)

		var resultPayload agentbrowser.BrowserResultPayload
		if err != nil {
			resultPayload = agentbrowser.BrowserResultPayload{
				ExecutionID: payload.ExecutionID,
				SessionID:   payload.SessionID,
				Success:     false,
				Error:       err.Error(),
			}
		} else if result != nil {
			resultPayload = *result
		}

		// REQ-M3-04: 전송 전에 세션 pending 큐에 저장
		session, exists := r.agentBrowserHandler.SessionManager().GetSession(payload.SessionID)
		if exists && session != nil {
			session.QueueResult(resultPayload)
		}

		// 전송 시도 - 성공 시 pending 큐에서 제거
		if sendErr := r.client.sendMessage(agentMsgBrowserResult, resultPayload); sendErr == nil {
			if exists && session != nil {
				session.DrainPendingResults()
			}
		} else {
			log.Printf("[agent-browser] failed to send result for session %s, queued for reconnection: %v",
				payload.SessionID, sendErr)
		}
	}()

	return nil
}

// handleBrowserSessionEnd는 Agent Browser 세션 종료 메시지를 처리합니다 (SPEC-BROWSER-AGENT-001).
func (r *Router) handleBrowserSessionEnd(ctx context.Context, msg ws.AgentMessage) error {
	var payload agentbrowser.BrowserSessionPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		errPayload := ws.TaskErrorPayload{
			ExecutionID: "",
			Code:        "INVALID_PAYLOAD",
			Message:     fmt.Sprintf("browser session end 페이로드 파싱 실패: %v", err),
			Retryable:   false,
		}
		return r.client.SendTaskError(errPayload)
	}

	if r.agentBrowserHandler == nil {
		return nil
	}

	if err := r.agentBrowserHandler.HandleSessionEnd(ctx, payload); err != nil {
		if r.onError != nil {
			r.onError(fmt.Errorf("browser session end 실패 (session=%s): %w", payload.SessionID, err))
		}
	}

	return nil
}

// handleMCPCodegenRequest는 서버로부터 수신한 MCP 코드 생성 요청을 처리합니다 (SPEC-SELF-EXPAND-001).
func (r *Router) handleMCPCodegenRequest(ctx context.Context, msg ws.AgentMessage) error {
	var req ws.MCPCodegenRequestPayload
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		return r.sendMCPError(msg.ID, req.ServiceName, fmt.Sprintf("mcp_codegen_request 페이로드 파싱 실패: %v", err), true)
	}

	if r.codegenExecutor == nil {
		log.Printf("[self-expand] codegen executor가 설정되지 않음, 요청 무시: %s", req.ServiceName)
		return r.sendMCPError(msg.ID, req.ServiceName, "코드 생성기가 설정되지 않음", true)
	}

	// 비동기로 코드 생성 실행
	go func() {
		// 샌드박스 디렉토리 생성
		sandboxBase := r.codegenSandboxBaseDir
		if sandboxBase == "" {
			homeDir, _ := os.UserHomeDir()
			sandboxBase = filepath.Join(homeDir, ".acos", "codegen-sandbox")
		}
		sandbox := codegen.NewSandbox(sandboxBase, nil)
		outputDir, cleanup, err := sandbox.Create(req.ServiceName)
		if err != nil {
			log.Printf("[self-expand] 샌드박스 생성 실패 (service=%s): %v", req.ServiceName, err)
			r.sendCodegenError(msg.ID, err.Error())
			return
		}
		defer cleanup()

		// 진행 상황 보고 콜백
		progressFn := func(phase string, progress int, message string) {
			_ = r.client.SendMCPCodegenProgress(msg.ID, ws.MCPCodegenProgressPayload{
				Phase:    phase,
				Progress: progress,
				Message:  message,
			})
		}

		// 코드 생성 실행
		genReq := codegen.GenerateRequest{
			ServiceName:  req.ServiceName,
			TemplateID:   req.TemplateID,
			Description:  req.Description,
			RequiredAPIs: req.RequiredAPIs,
			AuthType:     req.AuthType,
			OutputDir:    outputDir,
		}

		result, err := r.codegenExecutor.Generate(ctx, genReq, progressFn)
		if err != nil {
			log.Printf("[self-expand] 코드 생성 실패 (service=%s): %v", req.ServiceName, err)
			r.sendCodegenError(msg.ID, err.Error())
			return
		}

		// 에러가 있는 결과
		if result.Error != "" {
			r.sendCodegenError(msg.ID, result.Error)
			return
		}

		// 성공 결과 전송
		files := make([]ws.MCPGeneratedFile, 0, len(result.Files))
		var totalSize int64
		for _, f := range result.Files {
			files = append(files, ws.MCPGeneratedFile{
				Path:      f.Path,
				Content:   f.Content,
				SizeBytes: f.SizeBytes,
			})
			totalSize += f.SizeBytes
		}

		_ = r.client.SendMCPCodegenResult(msg.ID, ws.MCPCodegenResultPayload{
			Status:           "success",
			Files:            files,
			TotalFiles:       len(files),
			TotalSizeBytes:   totalSize,
			GenerationDurMs:  result.DurationMs,
			ClaudeTokensUsed: result.TokensUsed,
		})

		log.Printf("[self-expand] 코드 생성 완료 (service=%s, files=%d)", req.ServiceName, len(files))
	}()

	return nil
}

// sendCodegenError는 코드 생성 에러 결과를 서버로 전송합니다.
func (r *Router) sendCodegenError(msgID, errMsg string) {
	_ = r.client.SendMCPCodegenResult(msgID, ws.MCPCodegenResultPayload{
		Status: "error",
		Error:  errMsg,
	})
}

// handleMCPDeploy는 서버로부터 수신한 MCP 배포 요청을 처리합니다 (SPEC-SELF-EXPAND-001).
func (r *Router) handleMCPDeploy(ctx context.Context, msg ws.AgentMessage) error {
	var req ws.MCPDeployPayload
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		return r.sendMCPError(msg.ID, "", fmt.Sprintf("mcp_deploy 페이로드 파싱 실패: %v", err), true)
	}

	if r.mcpDeployer == nil {
		log.Printf("[self-expand] MCP deployer가 설정되지 않음, 요청 무시: %s", req.ServiceName)
		_ = r.client.SendMCPDeployResult(msg.ID, ws.MCPDeployResultPayload{
			ServiceName: req.ServiceName,
			Success:     false,
			Error:       "MCP 배포기가 설정되지 않음",
		})
		return nil
	}

	// 비동기로 배포 실행
	go func() {
		// ws 파일을 mcp.DeployFile로 변환
		files := make([]mcp.DeployFile, 0, len(req.Files))
		for _, f := range req.Files {
			files = append(files, mcp.DeployFile{
				Path:    f.Path,
				Content: f.Content,
			})
		}

		deployPath, err := r.mcpDeployer.Deploy(ctx, req.ServiceName, files, req.EnvVars)
		if err != nil {
			log.Printf("[self-expand] MCP 배포 실패 (service=%s): %v", req.ServiceName, err)
			_ = r.client.SendMCPDeployResult(msg.ID, ws.MCPDeployResultPayload{
				ServiceName: req.ServiceName,
				Success:     false,
				Error:       err.Error(),
			})
			return
		}

		_ = r.client.SendMCPDeployResult(msg.ID, ws.MCPDeployResultPayload{
			ServiceName: req.ServiceName,
			Success:     true,
			DeployPath:  deployPath,
		})

		log.Printf("[self-expand] MCP 배포 완료 (service=%s, path=%s)", req.ServiceName, deployPath)
	}()

	return nil
}

// ProgressReporter는 작업 진행 상황을 보고하는 헬퍼입니다.
type ProgressReporter struct {
	client      *Client
	executionID string
}

// NewProgressReporter는 새로운 ProgressReporter를 생성합니다.
func NewProgressReporter(client *Client, executionID string) *ProgressReporter {
	return &ProgressReporter{
		client:      client,
		executionID: executionID,
	}
}

// Report는 진행 상황을 보고합니다.
func (p *ProgressReporter) Report(progress int, message, msgType string) error {
	return p.client.SendTaskProgress(ws.TaskProgressPayload{
		ExecutionID: p.executionID,
		Progress:    progress,
		Message:     message,
		Type:        msgType,
	})
}

// ReportText는 텍스트 타입 진행 상황을 보고합니다.
func (p *ProgressReporter) ReportText(progress int, message string) error {
	return p.Report(progress, message, "text")
}

// ReportToolUse는 도구 사용 타입 진행 상황을 보고합니다.
func (p *ProgressReporter) ReportToolUse(progress int, message string) error {
	return p.Report(progress, message, "tool_use")
}

// TaskHandler는 작업 요청을 처리하는 함수 타입입니다.
type TaskHandler func(ctx context.Context, task ws.TaskRequestPayload, reporter *ProgressReporter) (ws.TaskResultPayload, error)

// SimpleExecutor는 함수 기반의 간단한 작업 실행기입니다.
type SimpleExecutor struct {
	client  *Client
	handler TaskHandler
}

// NewSimpleExecutor는 새로운 SimpleExecutor를 생성합니다.
func NewSimpleExecutor(client *Client, handler TaskHandler) *SimpleExecutor {
	return &SimpleExecutor{
		client:  client,
		handler: handler,
	}
}

// Execute는 작업을 실행합니다.
// TaskExecutor 인터페이스 구현.
func (e *SimpleExecutor) Execute(ctx context.Context, task ws.TaskRequestPayload) (ws.TaskResultPayload, error) {
	reporter := NewProgressReporter(e.client, task.ExecutionID)
	return e.handler(ctx, task, reporter)
}

// handleCLIRequest는 서버로부터 수신한 CLI 명령어 실행 요청을 처리합니다 (SPEC-SKILL-V2-001 Block C).
func (r *Router) handleCLIRequest(ctx context.Context, msg ws.AgentMessage) error {
	var req ws.CLIRequestPayload
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		errPayload := ws.TaskErrorPayload{
			ExecutionID: "",
			Code:        "INVALID_PAYLOAD",
			Message:     fmt.Sprintf("cli_request 페이로드 파싱 실패: %v", err),
			Retryable:   false,
		}
		return r.client.SendTaskError(errPayload)
	}

	if r.cliExecutor == nil {
		log.Printf("[skill-v2] CLI executor가 설정되지 않음, 요청 무시")
		return nil
	}

	// 비동기로 CLI 실행 (블로킹 방지)
	go func() {
		result := r.cliExecutor.Execute(ctx, &req)

		// 결과를 cli_result 메시지로 전송
		resultPayload, err := json.Marshal(result)
		if err != nil {
			log.Printf("[skill-v2] CLI 결과 직렬화 실패: %v", err)
			return
		}

		respMsg := ws.AgentMessage{
			Type:      ws.AgentMsgCLIResult,
			ID:        msg.ID, // 원본 요청 ID를 그대로 사용하여 매칭
			Timestamp: time.Now(),
			Payload:   resultPayload,
		}

		if err := r.client.Send(respMsg); err != nil {
			log.Printf("[skill-v2] CLI 결과 전송 실패: %v", err)
		}
	}()

	return nil
}

// handleMCPStart는 서버로부터 수신한 MCP 서버 시작 요청을 처리합니다 (SPEC-SKILL-V2-001 Block D).
func (r *Router) handleMCPStart(ctx context.Context, msg ws.AgentMessage) error {
	var req ws.MCPStartPayload
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		return r.sendMCPError(msg.ID, "", fmt.Sprintf("mcp_start 페이로드 파싱 실패: %v", err), true)
	}

	if r.mcpStarter == nil {
		log.Printf("[mcp] MCP starter가 설정되지 않음, 요청 무시: %s", req.ServerName)
		return r.sendMCPError(msg.ID, req.ServerName, "MCP 관리자가 설정되지 않음", true)
	}

	// 비동기로 MCP 서버 시작 (블로킹 방지)
	go func() {
		pid, err := r.mcpStarter.StartServer(ctx, req.ServerName, req.Command, req.Args, req.Env, req.WorkingDir)
		if err != nil {
			log.Printf("[mcp] MCP 서버 시작 실패 (name=%s): %v", req.ServerName, err)
			if sendErr := r.sendMCPError(msg.ID, req.ServerName, err.Error(), true); sendErr != nil {
				log.Printf("[mcp] MCP 에러 메시지 전송 실패: %v", sendErr)
			}
			return
		}

		// mcp_ready 응답 전송
		readyPayload := ws.MCPReadyPayload{
			ServerName:     req.ServerName,
			Transport:      "stdio",
			ConnectionInfo: fmt.Sprintf("pid:%d", pid),
			PID:            pid,
		}

		payload, err := json.Marshal(readyPayload)
		if err != nil {
			log.Printf("[mcp] mcp_ready 직렬화 실패: %v", err)
			return
		}

		respMsg := ws.AgentMessage{
			Type:      ws.AgentMsgMCPReady,
			ID:        msg.ID,
			Timestamp: time.Now(),
			Payload:   payload,
		}

		if err := r.client.Send(respMsg); err != nil {
			log.Printf("[mcp] mcp_ready 전송 실패: %v", err)
		} else {
			log.Printf("[mcp] MCP 서버 시작 완료 (name=%s, pid=%d)", req.ServerName, pid)
		}
	}()

	return nil
}

// handleMCPStop은 서버로부터 수신한 MCP 서버 중지 요청을 처리합니다 (SPEC-SKILL-V2-001 Block D).
func (r *Router) handleMCPStop(ctx context.Context, msg ws.AgentMessage) error {
	var req ws.MCPStopPayload
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		return r.sendMCPError(msg.ID, "", fmt.Sprintf("mcp_stop 페이로드 파싱 실패: %v", err), true)
	}

	if r.mcpStarter == nil {
		log.Printf("[mcp] MCP starter가 설정되지 않음, 중지 요청 무시: %s", req.ServerName)
		return nil
	}

	if err := r.mcpStarter.StopServer(req.ServerName, req.Force); err != nil {
		log.Printf("[mcp] MCP 서버 중지 실패 (name=%s): %v", req.ServerName, err)
		return r.sendMCPError(msg.ID, req.ServerName, err.Error(), false)
	}

	log.Printf("[mcp] MCP 서버 중지 완료 (name=%s, force=%v)", req.ServerName, req.Force)
	return nil
}

// sendMCPError는 MCP 에러 메시지를 서버로 전송합니다 (SPEC-SKILL-V2-001 Block D).
func (r *Router) sendMCPError(msgID, serverName, errMsg string, isFatal bool) error {
	errPayload := ws.MCPErrorPayload{
		ServerName: serverName,
		Error:      errMsg,
		IsFatal:    isFatal,
	}

	payload, err := json.Marshal(errPayload)
	if err != nil {
		return fmt.Errorf("mcp_error 직렬화 실패: %w", err)
	}

	respMsg := ws.AgentMessage{
		Type:      ws.AgentMsgMCPError,
		ID:        msgID,
		Timestamp: time.Now(),
		Payload:   payload,
	}

	return r.client.Send(respMsg)
}

// SendProjectContext는 프로젝트 기술 스택을 분석하여 서버로 전송합니다 (SPEC-SKILL-V2-001 Block A).
// 연결 성공 후 호출되어야 합니다.
func (r *Router) SendProjectContext(rootDir string) error {
	if r.projectAnalyzer == nil {
		return nil
	}

	ctx, err := r.projectAnalyzer.Analyze(rootDir)
	if err != nil {
		log.Printf("[skill-v2] 프로젝트 분석 실패 (dir=%s): %v", rootDir, err)
		return err
	}

	payload, err := json.Marshal(ctx)
	if err != nil {
		return fmt.Errorf("project_context 직렬화 실패: %w", err)
	}

	msg := ws.AgentMessage{
		Type:      ws.AgentMsgProjectContext,
		ID:        fmt.Sprintf("project-ctx-%d", time.Now().UnixMilli()),
		Timestamp: time.Now(),
		Payload:   payload,
	}

	if err := r.client.Send(msg); err != nil {
		return fmt.Errorf("project_context 전송 실패: %w", err)
	}

	log.Printf("[skill-v2] 프로젝트 컨텍스트 전송 완료 (root=%s, langs=%v, frameworks=%v)",
		ctx.ProjectRoot, ctx.TechStack.Languages, ctx.TechStack.Frameworks)
	return nil
}
