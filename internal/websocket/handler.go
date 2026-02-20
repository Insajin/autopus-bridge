// Package websocket는 Local Agent Bridge의 WebSocket 통신을 담당합니다.
// 메시지 라우팅 및 처리를 담당합니다.
package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/insajin/autopus-agent-protocol"
	"github.com/insajin/autopus-bridge/internal/auth"
	"github.com/insajin/autopus-bridge/internal/codegen"
	"github.com/insajin/autopus-bridge/internal/computeruse"
	"github.com/insajin/autopus-bridge/internal/mcp"
	"github.com/insajin/autopus-bridge/internal/mcpserver"
	"github.com/rs/zerolog"
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

	// codegenExecutor는 MCP 서버 코드 생성을 담당합니다 (SPEC-SELF-EXPAND-001).
	codegenExecutor CodegenExecutor
	// mcpDeployer는 MCP 서버 배포를 담당합니다 (SPEC-SELF-EXPAND-001).
	mcpDeployer MCPDeployExecutor
	// codegenSandboxBaseDir는 코드 생성 샌드박스 기본 디렉토리입니다.
	codegenSandboxBaseDir string

	// onError는 에러 발생 시 호출되는 콜백입니다.
	onError func(err error)

	// MCP 서버(serve) 라이프사이클 관리 (SPEC-AI-003 M3)
	// mcpServeTokenRefresher는 MCP 서버 BackendClient 생성에 필요한 토큰 갱신기입니다.
	mcpServeTokenRefresher *auth.TokenRefresher
	// mcpServeLogger는 MCP 서버 로거입니다.
	mcpServeLogger zerolog.Logger
	// mcpServeServer는 현재 실행 중인 MCP 서버입니다.
	mcpServeServer *mcpserver.Server
	// mcpServeCancel은 MCP 서버 goroutine을 취소하는 함수입니다.
	mcpServeCancel context.CancelFunc
	// mcpServeMu는 MCP 서버 상태 접근을 보호하는 뮤텍스입니다.
	mcpServeMu sync.Mutex
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

// WithMCPServeAuth는 MCP 서버(serve) 시작에 필요한 인증 정보를 설정합니다 (SPEC-AI-003 M3).
func WithMCPServeAuth(tokenRefresher *auth.TokenRefresher, logger zerolog.Logger) RouterOption {
	return func(r *Router) {
		r.mcpServeTokenRefresher = tokenRefresher
		r.mcpServeLogger = logger
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

	// SPEC-AI-003 M3 T-26: 하트비트에 MCP 서버 상태 포함
	client.SetHeartbeatEnricher(r.MCPServeStatus)

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

	// MCP Codegen/Deploy 핸들러 (SPEC-SELF-EXPAND-001)
	r.RegisterHandler(ws.AgentMsgMCPCodegenRequest, r.handleMCPCodegenRequest)
	r.RegisterHandler(ws.AgentMsgMCPDeploy, r.handleMCPDeploy)

	// MCP 서버(serve) 라이프사이클 핸들러 (SPEC-AI-003 M3)
	r.RegisterHandler(ws.AgentMsgMCPServeStart, r.handleMCPServeStart)
	r.RegisterHandler(ws.AgentMsgMCPServeStop, r.handleMCPServeStop)
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
		// 알 수 없는 메시지 타입은 무시
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
		// 실행 실패 시 에러 응답
		errPayload := ws.TaskErrorPayload{
			ExecutionID: task.ExecutionID,
			Code:        "EXECUTION_ERROR",
			Message:     err.Error(),
			Retryable:   isRetryableError(err),
		}
		_ = r.client.SendTaskError(errPayload)
		return
	}

	// 결과 전송
	_ = r.client.SendTaskResult(result)
}

// isRetryableError는 재시도 가능한 에러인지 확인합니다.
func isRetryableError(err error) bool {
	// TODO: 에러 타입에 따라 재시도 가능 여부 결정
	// 현재는 모든 에러를 재시도 불가로 처리
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

// OnReconnected restores Computer Use session state after a WebSocket
// reconnection. For each active session it notifies the server that the
// session is still alive and resends any pending action results that were
// not delivered before the connection dropped (REQ-M3-04).
//
// Implements the ReconnectionHandler interface.
func (r *Router) OnReconnected(ctx context.Context) error {
	if r.computerUseHandler == nil {
		return nil
	}

	sessions := r.computerUseHandler.GetActiveSessions()
	if len(sessions) == 0 {
		return nil
	}

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

// handleMCPServeStart는 서버로부터 수신한 MCP 서버(serve) 시작 요청을 처리합니다 (SPEC-AI-003 M3 T-24).
// BackendClient를 생성하고 MCP 서버를 stdio 트랜스포트로 시작합니다.
func (r *Router) handleMCPServeStart(ctx context.Context, msg ws.AgentMessage) error {
	var req ws.MCPServeStartPayload
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		return r.sendMCPServeResult(msg.ID, "error", "", fmt.Sprintf("mcp_serve_start 페이로드 파싱 실패: %v", err))
	}

	// 인증 인프라 확인
	if r.mcpServeTokenRefresher == nil {
		log.Printf("[mcp-serve] TokenRefresher가 설정되지 않음, 요청 무시")
		return r.sendMCPServeResult(msg.ID, "error", "", "MCP 서버 인증 정보가 설정되지 않음")
	}

	r.mcpServeMu.Lock()
	defer r.mcpServeMu.Unlock()

	// 이미 실행 중인 서버가 있는지 확인
	if r.mcpServeServer != nil {
		log.Printf("[mcp-serve] MCP 서버가 이미 실행 중입니다")
		return r.sendMCPServeResult(msg.ID, "error", "", "MCP 서버가 이미 실행 중입니다")
	}

	// BackendClient 생성
	backendURL := req.BackendURL
	if backendURL == "" {
		backendURL = "https://api.autopus.co"
	}
	backendClient := mcpserver.NewBackendClient(
		backendURL,
		r.mcpServeTokenRefresher,
		30*time.Second,
		r.mcpServeLogger,
	)

	// MCP 서버 생성
	srv := mcpserver.NewServer(backendClient, r.mcpServeLogger)
	r.mcpServeServer = srv

	// MCP 서버를 goroutine에서 시작 (stdio 블로킹)
	serveCtx, cancel := context.WithCancel(ctx)
	r.mcpServeCancel = cancel

	go func() {
		log.Printf("[mcp-serve] MCP 서버 시작")
		if err := srv.Start(); err != nil {
			log.Printf("[mcp-serve] MCP 서버 종료: %v", err)
		}

		// 서버 종료 시 상태 정리
		r.mcpServeMu.Lock()
		r.mcpServeServer = nil
		r.mcpServeCancel = nil
		r.mcpServeMu.Unlock()

		// 컨텍스트 취소 (이미 취소된 경우 no-op)
		cancel()
		_ = serveCtx // serveCtx는 향후 graceful shutdown에 활용 가능
	}()

	log.Printf("[mcp-serve] MCP 서버 시작 완료 (backend_url=%s)", backendURL)
	return r.sendMCPServeReady(msg.ID, "started", "MCP 서버가 시작되었습니다")
}

// handleMCPServeStop은 서버로부터 수신한 MCP 서버(serve) 중지 요청을 처리합니다 (SPEC-AI-003 M3 T-25).
func (r *Router) handleMCPServeStop(ctx context.Context, msg ws.AgentMessage) error {
	var req ws.MCPServeStopPayload
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		return r.sendMCPServeResult(msg.ID, "error", "", fmt.Sprintf("mcp_serve_stop 페이로드 파싱 실패: %v", err))
	}

	r.mcpServeMu.Lock()
	defer r.mcpServeMu.Unlock()

	// 실행 중인 서버가 없는 경우
	if r.mcpServeServer == nil {
		log.Printf("[mcp-serve] 실행 중인 MCP 서버가 없습니다")
		return r.sendMCPServeResult(msg.ID, "stopped", "실행 중인 MCP 서버가 없습니다", "")
	}

	// 컨텍스트 취소를 통한 graceful shutdown
	if r.mcpServeCancel != nil {
		r.mcpServeCancel()
	}

	// 리소스 정리
	r.mcpServeServer = nil
	r.mcpServeCancel = nil

	reason := req.Reason
	if reason == "" {
		reason = "서버 중지 요청"
	}
	log.Printf("[mcp-serve] MCP 서버 중지 완료 (reason=%s)", reason)
	return r.sendMCPServeResult(msg.ID, "stopped", "MCP 서버가 중지되었습니다", "")
}

// sendMCPServeReady는 MCP 서버(serve) 시작 완료 메시지를 서버로 전송합니다 (SPEC-AI-003 REQ-INTEGRATION-001).
// mcp_serve_start 성공 시에만 사용하며, 에러 응답은 sendMCPServeResult를 사용합니다.
func (r *Router) sendMCPServeReady(msgID, status, message string) error {
	resultPayload := ws.MCPServeResultPayload{
		Status:  status,
		Message: message,
	}

	payload, err := json.Marshal(resultPayload)
	if err != nil {
		return fmt.Errorf("mcp_serve_ready 직렬화 실패: %w", err)
	}

	respMsg := ws.AgentMessage{
		Type:      ws.AgentMsgMCPServeReady,
		ID:        msgID,
		Timestamp: time.Now(),
		Payload:   payload,
	}

	return r.client.Send(respMsg)
}

// sendMCPServeResult는 MCP 서버(serve) 결과 메시지를 서버로 전송합니다 (SPEC-AI-003 M3).
// mcp_serve_stop 응답 및 에러 응답에 사용합니다.
func (r *Router) sendMCPServeResult(msgID, status, message, errMsg string) error {
	resultPayload := ws.MCPServeResultPayload{
		Status:  status,
		Message: message,
		Error:   errMsg,
	}

	payload, err := json.Marshal(resultPayload)
	if err != nil {
		return fmt.Errorf("mcp_serve_result 직렬화 실패: %w", err)
	}

	respMsg := ws.AgentMessage{
		Type:      ws.AgentMsgMCPServeResult,
		ID:        msgID,
		Timestamp: time.Now(),
		Payload:   payload,
	}

	return r.client.Send(respMsg)
}

// MCPServeStatus는 현재 MCP 서버(serve) 상태를 반환합니다 (SPEC-AI-003 M3 T-26).
// 하트비트 enricher 콜백으로 사용됩니다.
func (r *Router) MCPServeStatus() string {
	r.mcpServeMu.Lock()
	defer r.mcpServeMu.Unlock()

	if r.mcpServeServer != nil {
		return "running"
	}
	return "stopped"
}
