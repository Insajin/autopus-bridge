// Package executor는 Local Agent Bridge의 작업 실행 엔진을 제공합니다.
// REQ-E-02: 작업 요청 시 해당 AI 프로바이더를 통해 작업 실행
// REQ-E-03: 실행 중 task_progress 이벤트 전송
// REQ-E-04: 완료 시 task_result 이벤트 전송
// REQ-E-05: 실패 시 task_error 이벤트 전송
package executor

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/insajin/autopus-agent-protocol"
	"github.com/insajin/autopus-bridge/internal/approval"
	"github.com/insajin/autopus-bridge/internal/provider"
	"github.com/insajin/autopus-bridge/internal/websocket"
	"github.com/rs/zerolog"
)

// 에러 코드 상수
const (
	// ErrorCodeProviderNotFound는 모델의 프로바이더가 등록되지 않았을 때 사용됩니다.
	// 백엔드의 BridgeErrCodeProviderNotFound("provider_not_found")와 동일한 값 사용.
	ErrorCodeProviderNotFound = "provider_not_found"
	// ErrorCodeProviderError는 프로바이더 API 에러 시 사용됩니다.
	ErrorCodeProviderError = "PROVIDER_ERROR"
	// ErrorCodeTimeout은 작업 타임아웃 시 사용됩니다.
	ErrorCodeTimeout = "TIMEOUT"
	// ErrorCodeCancelled는 컨텍스트 취소 시 사용됩니다.
	ErrorCodeCancelled = "CANCELLED"
	// ErrorCodeInternalError는 예상치 못한 에러 시 사용됩니다.
	ErrorCodeInternalError = "INTERNAL_ERROR"
	// ErrorCodeSandboxViolationTask는 샌드박스 정책 위반 시 사용됩니다.
	// SEC-P2-03: 작업 디렉토리 샌드박싱
	ErrorCodeSandboxViolationTask = "SANDBOX_VIOLATION"
)

// 실행 관련 상수
const (
	// ProgressReportInterval은 진행 상황 보고 간격입니다 (5초).
	ProgressReportInterval = 5 * time.Second
	// DefaultTimeout은 기본 작업 타임아웃입니다 (10분).
	DefaultTimeout = 10 * time.Minute
)

// TaskSender는 WebSocket을 통해 작업 관련 메시지를 전송하는 인터페이스입니다.
// websocket.Client와의 결합도를 낮추기 위해 사용됩니다.
type TaskSender interface {
	// SendTaskProgress는 작업 진행 상황을 전송합니다.
	SendTaskProgress(payload ws.TaskProgressPayload) error
	// SendTaskResult는 작업 결과를 전송합니다.
	SendTaskResult(payload ws.TaskResultPayload) error
	// SendTaskError는 작업 오류를 전송합니다.
	SendTaskError(payload ws.TaskErrorPayload) error
	// SetLastExecID는 마지막 실행 ID를 설정합니다.
	SetLastExecID(execID string)
}

// runningTask는 현재 실행 중인 작업 정보입니다.
type runningTask struct {
	// executionID는 실행 ID입니다.
	executionID string
	// startTime은 작업 시작 시간입니다.
	startTime time.Time
	// cancel은 작업 취소 함수입니다.
	cancel context.CancelFunc
}

// TaskExecutor는 작업 실행 엔진입니다.
// REQ-E-02: AI 프로바이더를 통한 작업 실행 조율
// REQ-S-01: 연결 상태에서 요청 수락
// REQ-S-03: 실행 중 새 요청 큐잉
type TaskExecutor struct {
	// registry는 AI 프로바이더 레지스트리입니다.
	registry *provider.Registry
	// sender는 WebSocket 메시지 전송자입니다.
	sender TaskSender
	// queue는 작업 대기 큐입니다.
	queue *TaskQueue
	// sandbox는 작업 디렉토리 샌드박스입니다.
	// SEC-P2-03: 작업 디렉토리 샌드박싱
	sandbox *Sandbox
	// logger는 로거입니다.
	logger zerolog.Logger
	// currentTask는 현재 실행 중인 작업입니다.
	currentTask atomic.Value // *runningTask

	// done은 실행기 종료를 알리는 채널입니다.
	done chan struct{}
	// wg는 고루틴 종료 대기를 위한 WaitGroup입니다.
	wg sync.WaitGroup
	// running은 실행기 상태입니다.
	running atomic.Bool
}

// TaskExecutorOption은 TaskExecutor 설정 옵션입니다.
type TaskExecutorOption func(*TaskExecutor)

// WithLogger는 로거를 설정합니다.
func WithLogger(logger zerolog.Logger) TaskExecutorOption {
	return func(e *TaskExecutor) {
		e.logger = logger
	}
}

// WithQueue는 작업 큐를 설정합니다.
func WithQueue(queue *TaskQueue) TaskExecutorOption {
	return func(e *TaskExecutor) {
		e.queue = queue
	}
}

// WithSandbox는 샌드박스를 설정합니다.
// SEC-P2-03: 작업 디렉토리 샌드박싱
func WithSandbox(sandbox *Sandbox) TaskExecutorOption {
	return func(e *TaskExecutor) {
		e.sandbox = sandbox
	}
}

// NewTaskExecutor는 새로운 작업 실행기를 생성합니다.
func NewTaskExecutor(registry *provider.Registry, sender TaskSender, opts ...TaskExecutorOption) *TaskExecutor {
	e := &TaskExecutor{
		registry: registry,
		sender:   sender,
		queue:    NewTaskQueue(),
		logger:   zerolog.Nop(),
		done:     make(chan struct{}),
	}

	for _, opt := range opts {
		opt(e)
	}

	e.currentTask.Store((*runningTask)(nil))

	return e
}

// Start는 작업 실행 루프를 시작합니다.
func (e *TaskExecutor) Start(ctx context.Context) {
	if e.running.Swap(true) {
		// 이미 실행 중
		return
	}

	e.wg.Add(1)
	go e.runLoop(ctx)

	e.logger.Info().Msg("작업 실행기 시작됨")
}

// Stop은 작업 실행기를 중지합니다.
func (e *TaskExecutor) Stop() {
	if !e.running.Load() {
		return
	}

	e.logger.Info().Msg("작업 실행기 중지 중...")

	// 현재 실행 중인 작업 취소
	if task := e.getCurrentTask(); task != nil {
		task.cancel()
	}

	// 종료 신호 전송
	close(e.done)

	// 큐 대기 해제
	e.queue.Wakeup()

	// 고루틴 종료 대기
	e.wg.Wait()

	e.running.Store(false)
	e.logger.Info().Msg("작업 실행기 중지됨")
}

// runLoop는 작업 처리 루프입니다.
func (e *TaskExecutor) runLoop(ctx context.Context) {
	defer e.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case <-e.done:
			return
		default:
		}

		// 큐에서 작업 가져오기 (블로킹)
		task, err := e.queue.GetBlocking(e.done)
		if err != nil {
			// 종료 신호로 인한 중단
			select {
			case <-e.done:
				return
			default:
				e.logger.Error().Err(err).Msg("큐에서 작업 가져오기 실패")
				continue
			}
		}

		// 작업 실행
		e.executeTask(ctx, task)
	}
}

// Submit은 작업을 실행 큐에 제출합니다.
// REQ-S-03: 실행 중 새 요청 큐잉
func (e *TaskExecutor) Submit(task ws.TaskRequestPayload) error {
	if !e.running.Load() {
		return errors.New("실행기가 중지된 상태입니다")
	}

	err := e.queue.Add(task)
	if err != nil {
		e.logger.Warn().
			Str("execution_id", task.ExecutionID).
			Err(err).
			Msg("작업 큐잉 실패")
		return err
	}

	e.logger.Info().
		Str("execution_id", task.ExecutionID).
		Str("model", task.Model).
		Int("queue_size", e.queue.Size()).
		Msg("작업 큐에 추가됨")

	return nil
}

// Execute는 작업을 즉시 실행합니다.
// websocket.TaskExecutor 인터페이스 구현을 위해 제공됩니다.
func (e *TaskExecutor) Execute(ctx context.Context, task ws.TaskRequestPayload) (ws.TaskResultPayload, error) {
	// 타임아웃 설정 (REQ-N-03)
	timeout := time.Duration(task.Timeout) * time.Second
	if timeout <= 0 {
		timeout = DefaultTimeout
	}

	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 현재 작업 등록
	rt := &runningTask{
		executionID: task.ExecutionID,
		startTime:   time.Now(),
		cancel:      cancel,
	}
	e.currentTask.Store(rt)
	defer e.currentTask.Store((*runningTask)(nil))

	e.logger.Info().
		Str("execution_id", task.ExecutionID).
		Str("model", task.Model).
		Dur("timeout", timeout).
		Msg("작업 실행 시작")

	// SEC-P2-03: 샌드박스 검증 - WorkDir가 허용된 경로인지 확인
	if e.sandbox != nil {
		if err := e.sandbox.ValidateWorkDir(task.WorkDir); err != nil {
			e.logger.Warn().
				Str("execution_id", task.ExecutionID).
				Str("work_dir", task.WorkDir).
				Err(err).
				Msg("샌드박스 정책 위반")
			return ws.TaskResultPayload{}, &TaskError{
				Code:      ErrorCodeSandboxViolationTask,
				Message:   fmt.Sprintf("작업 디렉토리 접근 거부: %v", err),
				Retryable: false,
			}
		}
	}

	// 프로바이더 조회 (REQ-E-02): 명시적 provider 필드 우선, 없으면 모델명 기반 해석
	// 서버가 빈 model/provider를 전송하면 등록된 기본 프로바이더로 폴백
	if task.Provider == "" && task.Model == "" {
		e.logger.Warn().
			Str("execution_id", task.ExecutionID).
			Msg("서버에서 빈 provider/model 수신, 기본 프로바이더로 폴백")
	}
	resolution, err := e.registry.ResolveForTask(task.Provider, task.Model)
	if err != nil {
		e.logger.Error().
			Str("execution_id", task.ExecutionID).
			Str("provider", task.Provider).
			Str("model", task.Model).
			Err(err).
			Msg("프로바이더 조회 실패")
		return ws.TaskResultPayload{}, &TaskError{
			Code:    ErrorCodeProviderNotFound,
			Message: fmt.Sprintf("모델 '%s'에 대한 프로바이더를 찾을 수 없습니다: %v", task.Model, err),
		}
	}

	prov := resolution.Provider
	execModel := resolution.Model

	// Resolution source별 로깅
	switch resolution.Source {
	case provider.ResolutionSourceOverride:
		e.logger.Warn().
			Str("execution_id", task.ExecutionID).
			Str("original_provider", task.Provider).
			Str("original_model", task.Model).
			Str("resolved_provider", prov.Name()).
			Str("resolved_model", execModel).
			Msg("서버 값 비어있거나 해석 실패, 오버라이드 폴백 적용")
	case provider.ResolutionSourceFallback:
		e.logger.Warn().
			Str("execution_id", task.ExecutionID).
			Str("resolved_provider", prov.Name()).
			Str("resolved_model", execModel).
			Msg("오버라이드 미설정, 기본 프로바이더 폴백")
	}

	// SPEC-INTERACTIVE-CLI-001: ApprovalRelay 지원 프로바이더에 승인 핸들러 설정
	if task.ApprovalPolicy != "" && task.ApprovalPolicy != string(approval.ApprovalPolicyAutoExecute) {
		if relay, ok := prov.(approval.ApprovalRelay); ok && relay.SupportsApproval() {
			policy := approval.ApprovalPolicy(task.ApprovalPolicy)
			timeout := 5 * time.Minute
			if task.Timeout > 0 {
				timeout = time.Duration(task.Timeout) * time.Second
			}
			router := approval.NewApprovalRouter(policy, timeout)
			relay.SetApprovalHandler(router.HandleApproval)

			e.logger.Info().
				Str("execution_id", task.ExecutionID).
				Str("approval_policy", task.ApprovalPolicy).
				Str("provider", prov.Name()).
				Msg("ApprovalRelay 활성화됨")
		}
	}

	// 진행 상황 보고 고루틴 시작
	progressDone := make(chan struct{})
	go e.reportProgress(execCtx, task.ExecutionID, progressDone)
	req := provider.ExecuteRequest{
		Prompt:       task.Prompt,
		SystemPrompt: task.SystemPrompt,
		Model:        execModel,
		MaxTokens:    task.MaxTokens,
		Tools:        task.Tools,
		WorkDir:      task.WorkDir,
	}

	// 스트리밍 지원 프로바이더인 경우 스트리밍 실행, 아니면 기존 방식
	var resp *provider.ExecuteResponse
	streamCallback := func(textDelta, accumulatedText string) {
		_ = e.sender.SendTaskProgress(ws.TaskProgressPayload{
			ExecutionID:     task.ExecutionID,
			Progress:        50,
			Message:         "스트리밍 중...",
			Type:            "text",
			TextDelta:       textDelta,
			AccumulatedText: accumulatedText,
		})
	}
	if cliProv, ok := prov.(*provider.ClaudeCLIProvider); ok {
		resp, err = cliProv.ExecuteStreaming(execCtx, req, streamCallback)
	} else if appSrvProv, ok := prov.(*provider.CodexAppServerProvider); ok {
		resp, err = appSrvProv.ExecuteStreaming(execCtx, req, streamCallback)
	} else {
		resp, err = prov.Execute(execCtx, req)
	}

	// 진행 상황 보고 중지
	close(progressDone)

	if err != nil {
		return ws.TaskResultPayload{}, e.classifyError(execCtx, err, task.ExecutionID)
	}

	// 결과 생성 (REQ-E-04)
	result := ws.TaskResultPayload{
		ExecutionID: task.ExecutionID,
		Output:      resp.Output,
		ExitCode:    0,
		Duration:    resp.DurationMs,
		TokenUsage: &ws.TokenUsage{
			InputTokens:   resp.TokenUsage.InputTokens,
			OutputTokens:  resp.TokenUsage.OutputTokens,
			TotalTokens:   resp.TokenUsage.TotalTokens,
			CacheRead:     resp.TokenUsage.CacheRead,
			CacheCreation: resp.TokenUsage.CacheCreation,
		},
	}

	e.logger.Info().
		Str("execution_id", task.ExecutionID).
		Int64("duration_ms", resp.DurationMs).
		Int("input_tokens", resp.TokenUsage.InputTokens).
		Int("output_tokens", resp.TokenUsage.OutputTokens).
		Msg("작업 실행 완료")

	return result, nil
}

// ExecuteAgentResponse executes the richer agent_response_request path.
// It preserves native tool-loop metadata instead of coercing it into task_request.
func (e *TaskExecutor) ExecuteAgentResponse(ctx context.Context, req ws.AgentResponseRequestPayload) (ws.AgentResponseCompletePayload, error) {
	timeout := time.Duration(req.Timeout) * time.Second
	if timeout <= 0 {
		timeout = DefaultTimeout
	}

	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	rt := &runningTask{
		executionID: req.ExecutionID,
		startTime:   time.Now(),
		cancel:      cancel,
	}
	e.currentTask.Store(rt)
	defer e.currentTask.Store((*runningTask)(nil))

	if e.sandbox != nil {
		if err := e.sandbox.ValidateWorkDir(req.WorkDir); err != nil {
			return ws.AgentResponseCompletePayload{}, &TaskError{
				Code:      ErrorCodeSandboxViolationTask,
				Message:   fmt.Sprintf("작업 디렉토리 접근 거부: %v", err),
				Retryable: false,
			}
		}
	}

	resolution, err := e.registry.ResolveForTask(req.Provider, req.Model)
	if err != nil {
		return ws.AgentResponseCompletePayload{}, &TaskError{
			Code:    ErrorCodeProviderNotFound,
			Message: fmt.Sprintf("모델 '%s'에 대한 프로바이더를 찾을 수 없습니다: %v", req.Model, err),
		}
	}

	prov := resolution.Provider
	execModel := resolution.Model

	if req.ApprovalPolicy != "" && req.ApprovalPolicy != string(approval.ApprovalPolicyAutoExecute) {
		if relay, ok := prov.(approval.ApprovalRelay); ok && relay.SupportsApproval() {
			policy := approval.ApprovalPolicy(req.ApprovalPolicy)
			router := approval.NewApprovalRouter(policy, timeout)
			relay.SetApprovalHandler(router.HandleApproval)
		}
	}

	providerReq := provider.ExecuteRequest{
		Prompt:           req.Prompt,
		SystemPrompt:     req.SystemPrompt,
		Model:            execModel,
		MaxTokens:        req.MaxTokens,
		Tools:            req.Tools,
		WorkDir:          req.WorkDir,
		ResponseMode:     req.ResponseMode,
		ToolLoopMessages: req.ToolLoopMessages,
		ToolDefinitions:  req.ToolDefinitions,
	}

	resp, err := prov.Execute(execCtx, providerReq)
	if err != nil {
		return ws.AgentResponseCompletePayload{}, e.classifyError(execCtx, err, req.ExecutionID)
	}

	providerName := resp.Provider
	if providerName == "" {
		providerName = prov.Name()
	}

	return ws.AgentResponseCompletePayload{
		ExecutionID: req.ExecutionID,
		Output:      resp.Output,
		ExitCode:    0,
		Duration:    resp.DurationMs,
		TokenUsage: &ws.TokenUsage{
			InputTokens:   resp.TokenUsage.InputTokens,
			OutputTokens:  resp.TokenUsage.OutputTokens,
			TotalTokens:   resp.TokenUsage.TotalTokens,
			CacheRead:     resp.TokenUsage.CacheRead,
			CacheCreation: resp.TokenUsage.CacheCreation,
		},
		Model:      resp.Model,
		Provider:   providerName,
		StopReason: resp.StopReason,
		ToolCalls:  convertToolCalls(resp.ToolCalls),
	}, nil
}

// executeTask는 큐에서 가져온 작업을 실행하고 결과를 전송합니다.
func (e *TaskExecutor) executeTask(ctx context.Context, task ws.TaskRequestPayload) {
	// 시작 진행 상황 전송
	_ = e.sender.SendTaskProgress(ws.TaskProgressPayload{
		ExecutionID: task.ExecutionID,
		Progress:    0,
		Message:     "작업 시작",
		Type:        "text",
	})

	// 작업 실행
	result, err := e.Execute(ctx, task)

	if err != nil {
		// 에러 상세 로깅
		e.logger.Error().
			Str("execution_id", task.ExecutionID).
			Str("model", task.Model).
			Str("provider", task.Provider).
			Err(err).
			Msg("작업 실행 실패")
		// 에러 전송 (REQ-E-05)
		taskErr := e.toTaskError(err, task.ExecutionID)
		if sendErr := e.sender.SendTaskError(taskErr); sendErr != nil {
			e.logger.Error().
				Str("execution_id", task.ExecutionID).
				Err(sendErr).
				Msg("에러 전송 실패")
		}
		return
	}

	// 완료 진행 상황 전송
	_ = e.sender.SendTaskProgress(ws.TaskProgressPayload{
		ExecutionID: task.ExecutionID,
		Progress:    100,
		Message:     "작업 완료",
		Type:        "text",
	})

	// 결과 전송 (REQ-E-04)
	if sendErr := e.sender.SendTaskResult(result); sendErr != nil {
		e.logger.Error().
			Str("execution_id", task.ExecutionID).
			Err(sendErr).
			Msg("결과 전송 실패")
	}
}

func convertToolCalls(calls []provider.ToolCall) []ws.ToolLoopCall {
	if len(calls) == 0 {
		return nil
	}
	result := make([]ws.ToolLoopCall, 0, len(calls))
	for _, call := range calls {
		result = append(result, ws.ToolLoopCall{
			ID:    call.ID,
			Name:  call.Name,
			Input: call.Input,
		})
	}
	return result
}

// reportProgress는 주기적으로 진행 상황을 보고합니다.
// REQ-E-03: 실행 중 task_progress 이벤트 전송 (5초 간격)
func (e *TaskExecutor) reportProgress(ctx context.Context, executionID string, done <-chan struct{}) {
	ticker := time.NewTicker(ProgressReportInterval)
	defer ticker.Stop()

	progress := 10 // 시작 진행률

	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		case <-ticker.C:
			// 진행률 증가 (최대 90%까지)
			if progress < 90 {
				progress += 5
			}

			payload := ws.TaskProgressPayload{
				ExecutionID: executionID,
				Progress:    progress,
				Message:     "작업 실행 중...",
				Type:        "text",
			}

			if err := e.sender.SendTaskProgress(payload); err != nil {
				e.logger.Warn().
					Str("execution_id", executionID).
					Err(err).
					Msg("진행 상황 전송 실패")
			}
		}
	}
}

// classifyError는 에러를 분류하여 TaskError로 변환합니다.
// REQ-E-05: 적절한 에러 코드로 task_error 전송
func (e *TaskExecutor) classifyError(ctx context.Context, err error, executionID string) *TaskError {
	// 컨텍스트 관련 에러 확인
	if ctx.Err() != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return &TaskError{
				Code:      ErrorCodeTimeout,
				Message:   "작업 실행 시간이 초과되었습니다",
				Retryable: true,
			}
		}
		if errors.Is(ctx.Err(), context.Canceled) {
			return &TaskError{
				Code:      ErrorCodeCancelled,
				Message:   "작업이 취소되었습니다",
				Retryable: false,
			}
		}
	}

	// 프로바이더 에러 확인
	if errors.Is(err, provider.ErrProviderNotFound) {
		return &TaskError{
			Code:      ErrorCodeProviderNotFound,
			Message:   err.Error(),
			Retryable: false,
		}
	}

	if errors.Is(err, provider.ErrRateLimited) {
		return &TaskError{
			Code:      ErrorCodeProviderError,
			Message:   "API 레이트 리밋 초과",
			Retryable: true,
		}
	}

	if errors.Is(err, provider.ErrNoAPIKey) {
		return &TaskError{
			Code:      ErrorCodeProviderNotFound,
			Message:   "API 키가 설정되지 않았습니다",
			Retryable: false,
		}
	}

	// 기타 프로바이더 에러
	return &TaskError{
		Code:      ErrorCodeInternalError,
		Message:   fmt.Sprintf("작업 실행 중 오류 발생: %v", err),
		Retryable: false,
	}
}

// toTaskError는 에러를 TaskErrorPayload로 변환합니다.
func (e *TaskExecutor) toTaskError(err error, executionID string) ws.TaskErrorPayload {
	var taskErr *TaskError
	if errors.As(err, &taskErr) {
		return ws.TaskErrorPayload{
			ExecutionID: executionID,
			Code:        taskErr.Code,
			Message:     taskErr.Message,
			Retryable:   taskErr.Retryable,
		}
	}

	return ws.TaskErrorPayload{
		ExecutionID: executionID,
		Code:        ErrorCodeInternalError,
		Message:     err.Error(),
		Retryable:   false,
	}
}

// getCurrentTask는 현재 실행 중인 작업을 반환합니다.
func (e *TaskExecutor) getCurrentTask() *runningTask {
	if v := e.currentTask.Load(); v != nil {
		return v.(*runningTask)
	}
	return nil
}

// IsExecuting은 현재 작업이 실행 중인지 확인합니다.
func (e *TaskExecutor) IsExecuting() bool {
	return e.getCurrentTask() != nil
}

// CurrentExecutionID는 현재 실행 중인 작업의 ID를 반환합니다.
func (e *TaskExecutor) CurrentExecutionID() string {
	if task := e.getCurrentTask(); task != nil {
		return task.executionID
	}
	return ""
}

// QueueSize는 대기 중인 작업 수를 반환합니다.
func (e *TaskExecutor) QueueSize() int {
	return e.queue.Size()
}

// CancelCurrent는 현재 실행 중인 작업을 취소합니다.
func (e *TaskExecutor) CancelCurrent() bool {
	if task := e.getCurrentTask(); task != nil {
		task.cancel()
		return true
	}
	return false
}

// TaskError는 작업 실행 에러입니다.
type TaskError struct {
	// Code는 에러 코드입니다.
	Code string
	// Message는 에러 메시지입니다.
	Message string
	// Retryable은 재시도 가능 여부입니다.
	Retryable bool
}

// Error는 에러 메시지를 반환합니다.
func (e *TaskError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap은 원본 에러를 반환합니다.
func (e *TaskError) Unwrap() error {
	return nil
}

// ErrorCode는 에러 코드를 반환합니다.
// websocket.codeError 인터페이스를 만족합니다.
func (e *TaskError) ErrorCode() string {
	return e.Code
}

// IsRetryable은 에러의 재시도 가능 여부를 반환합니다.
// websocket.retryable 인터페이스를 만족합니다.
func (e *TaskError) IsRetryable() bool {
	return e.Retryable
}

// Ensure TaskExecutor implements websocket.TaskExecutor interface.
var _ websocket.TaskExecutor = (*TaskExecutor)(nil)
