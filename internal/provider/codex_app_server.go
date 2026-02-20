// Package provider는 AI 프로바이더 통합 레이어를 제공합니다.
package provider

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
)

// ===== AppServerProcess: Codex App Server 프로세스 관리자 =====

// AppServerProcess는 Codex App Server 프로세스를 관리합니다.
// exec.Cmd를 사용하여 하위 프로세스를 시작하고, stdin/stdout 파이프를 통해
// JSON-RPC 2.0 프로토콜로 통신합니다.
type AppServerProcess struct {
	cmd          *exec.Cmd
	client       *JSONRPCClient
	cliPath      string
	restartCount int
	maxRestarts  int
	mu           sync.Mutex
	running      atomic.Bool
	logger       zerolog.Logger
}

// NewAppServerProcess는 새로운 AppServerProcess를 생성합니다.
// cliPath는 Codex CLI 바이너리의 경로입니다.
func NewAppServerProcess(cliPath string, logger zerolog.Logger) *AppServerProcess {
	return &AppServerProcess{
		cliPath:     cliPath,
		maxRestarts: 3,
		logger:      logger.With().Str("component", "app-server-process").Logger(),
	}
}

// Start는 Codex App Server 프로세스를 시작합니다.
// stdin/stdout/stderr 파이프를 설정하고 JSON-RPC 클라이언트를 생성합니다.
// 초기화 핸드셰이크(initialize + initialized)를 수행합니다.
func (p *AppServerProcess) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 프로세스 커맨드 생성
	p.cmd = exec.Command(p.cliPath, "app-server")

	// stdin 파이프 설정 (클라이언트 -> 서버)
	stdinPipe, err := p.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin 파이프 생성 실패: %w", err)
	}

	// stdout 파이프 설정 (서버 -> 클라이언트)
	stdoutPipe, err := p.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout 파이프 생성 실패: %w", err)
	}

	// stderr 파이프 설정 (서버 에러/로그)
	stderrPipe, err := p.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr 파이프 생성 실패: %w", err)
	}

	// 프로세스 시작
	if err := p.cmd.Start(); err != nil {
		return fmt.Errorf("프로세스 시작 실패: %w", err)
	}

	p.running.Store(true)

	// JSON-RPC 클라이언트 생성
	p.client = NewJSONRPCClient(stdinPipe, stdoutPipe, p.logger)

	// stderr 로거 고루틴 시작
	go p.logStderr(stderrPipe)

	// 프로세스 모니터 고루틴 시작
	go p.monitor()

	// 초기화 핸드셰이크 수행
	if err := p.initialize(ctx); err != nil {
		// 핸드셰이크 실패 시 프로세스 정리
		p.running.Store(false)
		if p.client != nil {
			_ = p.client.Close()
		}
		if p.cmd.Process != nil {
			_ = p.cmd.Process.Kill()
		}
		return err
	}

	p.logger.Info().Msg("App Server 프로세스 시작 완료")
	return nil
}

// Stop은 Codex App Server 프로세스를 중지합니다.
// 먼저 SIGTERM을 보내고, 5초 내 종료되지 않으면 SIGKILL을 보냅니다.
func (p *AppServerProcess) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.running.Load() {
		return nil
	}

	p.running.Store(false)

	// JSON-RPC 클라이언트 종료
	if p.client != nil {
		_ = p.client.Close()
	}

	if p.cmd == nil || p.cmd.Process == nil {
		return nil
	}

	// SIGTERM 전송
	if err := p.cmd.Process.Signal(os.Interrupt); err != nil {
		// SIGTERM 실패 시 즉시 SIGKILL
		_ = p.cmd.Process.Kill()
		return nil
	}

	// 5초 대기 후 SIGKILL
	done := make(chan error, 1)
	go func() {
		_, err := p.cmd.Process.Wait()
		done <- err
	}()

	select {
	case <-done:
		p.logger.Info().Msg("App Server 프로세스 정상 종료")
	case <-time.After(5 * time.Second):
		p.logger.Warn().Msg("App Server 프로세스 SIGKILL 전송")
		_ = p.cmd.Process.Kill()
	}

	return nil
}

// Restart는 프로세스를 재시작합니다.
// 최대 재시작 횟수를 초과하면 ErrMaxRestartsExceeded를 반환합니다.
func (p *AppServerProcess) Restart(ctx context.Context) error {
	p.restartCount++
	if p.restartCount >= p.maxRestarts {
		return ErrMaxRestartsExceeded
	}

	p.logger.Info().
		Int("restartCount", p.restartCount).
		Int("maxRestarts", p.maxRestarts).
		Msg("App Server 프로세스 재시작")

	if err := p.Stop(); err != nil {
		p.logger.Warn().Err(err).Msg("프로세스 중지 실패 (재시작 계속 시도)")
	}

	return p.Start(ctx)
}

// Client는 JSON-RPC 클라이언트를 반환합니다.
// 프로세스가 실행 중이 아니면 nil을 반환합니다.
func (p *AppServerProcess) Client() *JSONRPCClient {
	if !p.running.Load() {
		return nil
	}
	return p.client
}

// IsRunning은 프로세스가 실행 중인지 여부를 반환합니다.
func (p *AppServerProcess) IsRunning() bool {
	return p.running.Load()
}

// initialize는 서버와 초기화 핸드셰이크를 수행합니다.
// "initialize" 요청을 보내고 응답을 받은 후 "initialized" 알림을 전송합니다.
func (p *AppServerProcess) initialize(ctx context.Context) error {
	// 10초 타임아웃 컨텍스트
	initCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// initialize 요청 전송
	result, err := p.client.Call(initCtx, MethodInitialize, InitializeParams{
		ClientVersion: "autopus-bridge/1.0",
	})
	if err != nil {
		if initCtx.Err() != nil {
			return ErrHandshakeTimeout
		}
		return fmt.Errorf("초기화 요청 실패: %w", err)
	}

	// 초기화 결과 파싱
	if result != nil {
		var initResult InitializeResult
		if err := json.Unmarshal(*result, &initResult); err != nil {
			p.logger.Warn().Err(err).Msg("초기화 결과 파싱 실패 (무시)")
		} else {
			p.logger.Info().
				Str("serverVersion", initResult.ServerVersion).
				Msg("서버 초기화 완료")
		}
	}

	// initialized 알림 전송
	if err := p.client.Notify(MethodInitialized, nil); err != nil {
		return fmt.Errorf("initialized 알림 전송 실패: %w", err)
	}

	return nil
}

// logStderr는 프로세스의 stderr 출력을 로깅하는 고루틴입니다.
func (p *AppServerProcess) logStderr(stderr io.Reader) {
	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		p.logger.Warn().
			Str("source", "stderr").
			Str("line", scanner.Text()).
			Msg("App Server stderr")
	}
}

// monitor는 프로세스 종료를 감시하는 고루틴입니다.
// 프로세스가 예기치 않게 종료되면 자동 재시작을 시도합니다.
func (p *AppServerProcess) monitor() {
	if p.cmd == nil {
		return
	}

	// cmd.Wait()은 프로세스가 종료될 때까지 블록
	err := p.cmd.Wait()

	// running이 여전히 true이면 예기치 않은 종료
	if p.running.Load() {
		p.logger.Error().
			Err(err).
			Msg("App Server 프로세스 예기치 않게 종료, 재시작 시도")

		p.running.Store(false)

		// 백그라운드 컨텍스트로 재시작 시도
		if restartErr := p.Restart(context.Background()); restartErr != nil {
			p.logger.Error().
				Err(restartErr).
				Msg("App Server 프로세스 재시작 실패")
		}
	}
}

// ===== CodexAppServerProvider: Provider 인터페이스 구현 =====

// CodexAppServerProvider는 Codex App Server를 사용하는 프로바이더입니다.
// JSON-RPC 2.0 프로토콜을 통해 App Server와 통신하며,
// 실시간 스트리밍을 지원합니다.
type CodexAppServerProvider struct {
	process        *AppServerProcess
	config         ProviderConfig
	approvalPolicy string
	authMethod     string // "apiKey" 또는 "chatgptAuthTokens"
	authKey        string // 실제 키 값
	logger         zerolog.Logger
}

// CodexAppServerOption은 CodexAppServerProvider 설정 옵션입니다.
type CodexAppServerOption func(*CodexAppServerProvider)

// WithAppServerLogger는 로거를 설정합니다.
func WithAppServerLogger(logger zerolog.Logger) CodexAppServerOption {
	return func(p *CodexAppServerProvider) {
		p.logger = logger
	}
}

// WithAppServerApprovalPolicy는 승인 정책을 설정합니다.
// 예: "auto-approve", "deny-all"
func WithAppServerApprovalPolicy(policy string) CodexAppServerOption {
	return func(p *CodexAppServerProvider) {
		p.approvalPolicy = policy
	}
}

// WithAppServerAuth는 인증 방식과 키를 설정합니다.
// method는 "apiKey" 또는 "chatgptAuthTokens"입니다.
func WithAppServerAuth(method, key string) CodexAppServerOption {
	return func(p *CodexAppServerProvider) {
		p.authMethod = method
		p.authKey = key
	}
}

// NewCodexAppServerProvider는 새로운 CodexAppServerProvider를 생성합니다.
// 프로세스를 시작하고 인증을 수행한 후 프로바이더를 반환합니다.
func NewCodexAppServerProvider(cliPath string, opts ...CodexAppServerOption) (*CodexAppServerProvider, error) {
	p := &CodexAppServerProvider{
		approvalPolicy: "auto-approve",
		authMethod:     "apiKey",
		logger:         zerolog.New(os.Stderr).With().Timestamp().Logger(),
	}

	// 옵션 적용
	for _, opt := range opts {
		opt(p)
	}

	// AppServerProcess 생성 및 시작
	p.process = NewAppServerProcess(cliPath, p.logger)
	if err := p.process.Start(context.Background()); err != nil {
		return nil, fmt.Errorf("App Server 프로세스 시작 실패: %w", err)
	}

	// 인증 수행
	if err := p.authenticate(context.Background()); err != nil {
		_ = p.process.Stop()
		return nil, fmt.Errorf("인증 실패: %w", err)
	}

	return p, nil
}

// Name은 프로바이더 식별자를 반환합니다.
func (p *CodexAppServerProvider) Name() string {
	return "codex"
}

// Supports는 주어진 모델명을 지원하는지 확인합니다.
// codexSupportedModels 슬라이스와 비교합니다.
func (p *CodexAppServerProvider) Supports(model string) bool {
	for _, supported := range codexSupportedModels {
		if model == supported {
			return true
		}
	}
	return false
}

// ValidateConfig는 프로바이더 설정의 유효성을 검사합니다.
// 프로세스 실행 상태와 인증 키를 확인합니다.
func (p *CodexAppServerProvider) ValidateConfig() error {
	if !p.process.IsRunning() {
		return ErrProcessNotRunning
	}
	if p.authKey == "" {
		return ErrNoAPIKey
	}
	return nil
}

// Execute는 프롬프트를 실행하고 결과를 반환합니다.
// 스트리밍 콜백 없이 executeInternal을 호출합니다.
func (p *CodexAppServerProvider) Execute(ctx context.Context, req ExecuteRequest) (*ExecuteResponse, error) {
	return p.executeInternal(ctx, req, nil)
}

// ExecuteStreaming은 실시간 스트리밍을 지원하는 실행입니다.
// onDelta 콜백을 통해 텍스트 청크가 준비될 때마다 알림을 받습니다.
func (p *CodexAppServerProvider) ExecuteStreaming(ctx context.Context, req ExecuteRequest, onDelta StreamCallback) (*ExecuteResponse, error) {
	return p.executeInternal(ctx, req, onDelta)
}

// authenticate는 App Server에 인증을 수행합니다.
func (p *CodexAppServerProvider) authenticate(ctx context.Context) error {
	client := p.process.Client()
	if client == nil {
		return ErrProcessNotRunning
	}

	// 30초 타임아웃 컨텍스트
	authCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// 인증 파라미터 구성
	params := AccountLoginParams{
		Method: p.authMethod,
	}
	if p.authMethod == "apiKey" {
		params.APIKey = p.authKey
	} else if p.authMethod == "chatgptAuthTokens" {
		params.ChatGPTAuthTokens = p.authKey
	}

	// 인증 요청 전송
	_, err := client.Call(authCtx, MethodAccountLoginStart, params)
	if err != nil {
		return fmt.Errorf("인증 요청 실패: %w", err)
	}

	p.logger.Info().
		Str("method", p.authMethod).
		Msg("App Server 인증 완료")

	return nil
}

// executeInternal은 실제 실행 로직을 구현합니다.
// 1. thread/start로 스레드 생성
// 2. 알림 핸들러 등록 (메시지 델타, 명령 실행, MCP 도구 호출, 승인 요청, Turn 완료)
// 3. turn/start로 턴 시작
// 4. Turn 완료 대기
// 5. 결과 조립 및 반환
func (p *CodexAppServerProvider) executeInternal(ctx context.Context, req ExecuteRequest, onDelta StreamCallback) (*ExecuteResponse, error) {
	// 1. 클라이언트 확인
	client := p.process.Client()
	if client == nil || !p.process.IsRunning() {
		return nil, ErrProcessNotRunning
	}

	// 2. 승인 정책 결정
	approvalPolicy := p.approvalPolicy
	if approvalPolicy == "" {
		approvalPolicy = "auto-approve"
	}

	// 3. thread/start 호출
	model := req.Model
	if model == "" {
		model = "gpt-5-codex"
	}

	cwd := req.WorkDir
	if cwd == "" {
		cwd = "."
	}

	threadResult, err := client.Call(ctx, MethodThreadStart, ThreadStartParams{
		Model:          model,
		Cwd:            cwd,
		ApprovalPolicy: approvalPolicy,
	})
	if err != nil {
		return nil, fmt.Errorf("thread/start 실패: %w", err)
	}

	// thread/start 결과 파싱
	var thread ThreadStartResult
	if threadResult != nil {
		if err := json.Unmarshal(*threadResult, &thread); err != nil {
			return nil, fmt.Errorf("thread/start 결과 파싱 실패: %w", err)
		}
	}

	// 4. 알림 핸들러 등록
	turnDone := make(chan struct{})
	var toolCalls []ToolCall
	var mu sync.Mutex // outputBuilder와 toolCalls 보호용 뮤텍스
	var outputBuilder strings.Builder

	// StreamAccumulator 생성 (스트리밍 모드일 때)
	var accumulator *StreamAccumulator
	if onDelta != nil {
		accumulator = NewStreamAccumulator()
	}

	// 에이전트 메시지 델타 핸들러
	client.OnNotification(MethodAgentMessageDelta, func(method string, params json.RawMessage) {
		var delta AgentMessageDelta
		if err := json.Unmarshal(params, &delta); err != nil {
			p.logger.Warn().Err(err).Msg("에이전트 메시지 델타 파싱 실패")
			return
		}

		mu.Lock()
		outputBuilder.WriteString(delta.Text)
		mu.Unlock()

		// 스트리밍 콜백 처리 (StreamAccumulator는 자체 뮤텍스 보유)
		if onDelta != nil && accumulator != nil {
			accumulator.Add(delta.Text)
			if accumulator.ShouldFlush() {
				flushed := accumulator.Flush()
				onDelta(flushed, accumulator.GetAccumulated())
			}
		}
	})

	// 명령 실행 완료 핸들러
	client.OnNotification(MethodCommandExecution, func(method string, params json.RawMessage) {
		var item CommandExecutionItem
		if err := json.Unmarshal(params, &item); err != nil {
			p.logger.Warn().Err(err).Msg("명령 실행 아이템 파싱 실패")
			return
		}

		// ToolCall 형태로 변환
		inputData, _ := json.Marshal(map[string]interface{}{
			"command": item.Command,
			"output":  item.Output,
		})

		mu.Lock()
		toolCalls = append(toolCalls, ToolCall{
			ID:    item.ID,
			Name:  "command_execution",
			Input: inputData,
		})
		mu.Unlock()
	})

	// MCP 도구 호출 핸들러
	client.OnNotification(MethodMCPToolCall, func(method string, params json.RawMessage) {
		var item MCPToolCallItem
		if err := json.Unmarshal(params, &item); err != nil {
			p.logger.Warn().Err(err).Msg("MCP 도구 호출 아이템 파싱 실패")
			return
		}

		mu.Lock()
		toolCalls = append(toolCalls, ToolCall{
			ID:    item.ID,
			Name:  item.ToolName,
			Input: item.Input,
		})
		mu.Unlock()
	})

	// 승인 요청 핸들러
	client.OnNotification(MethodRequestApproval, func(method string, params json.RawMessage) {
		var approvalParams RequestApprovalParams
		if err := json.Unmarshal(params, &approvalParams); err != nil {
			p.logger.Warn().Err(err).Msg("승인 요청 파싱 실패")
			return
		}

		// 승인 정책에 따라 결정
		decision := "accept"
		if approvalPolicy == "deny-all" {
			decision = "decline"
		}

		response := ApprovalResponse{
			ExecutionID: approvalParams.ExecutionID,
			Decision:    decision,
		}

		if err := client.Notify("requestApproval/response", response); err != nil {
			p.logger.Warn().Err(err).Msg("승인 응답 전송 실패")
		}
	})

	// Turn 완료 핸들러
	client.OnNotification(MethodTurnCompleted, func(method string, params json.RawMessage) {
		close(turnDone)
	})

	// 5. turn/start 호출
	_, err = client.Call(ctx, MethodTurnStart, TurnStartParams{
		ThreadID: thread.ThreadID,
		Input: []TurnInput{
			{
				Type: "text",
				Text: req.Prompt,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("turn/start 실패: %w", err)
	}

	// 6. Turn 완료 또는 컨텍스트 취소 대기
	select {
	case <-turnDone:
		// 정상 완료
	case <-ctx.Done():
		return nil, fmt.Errorf("실행 타임아웃: %w", ctx.Err())
	}

	// 7. 남은 스트리밍 데이터 플러시
	if onDelta != nil && accumulator != nil && accumulator.HasPending() {
		remaining := accumulator.FlushAll()
		onDelta(remaining, accumulator.GetAccumulated())
	}

	// 8. 결과 조립 및 반환
	mu.Lock()
	output := outputBuilder.String()
	resultToolCalls := make([]ToolCall, len(toolCalls))
	copy(resultToolCalls, toolCalls)
	mu.Unlock()

	return &ExecuteResponse{
		Output:    output,
		ToolCalls: resultToolCalls,
		Model:     model,
	}, nil
}

// Close는 프로바이더를 종료합니다. 프로세스를 중지합니다.
func (p *CodexAppServerProvider) Close() error {
	return p.process.Stop()
}
