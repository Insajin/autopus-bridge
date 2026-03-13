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

	"github.com/insajin/autopus-bridge/internal/approval"
	ws "github.com/insajin/autopus-agent-protocol"
	"github.com/insajin/autopus-codex-rpc/client"
	"github.com/insajin/autopus-codex-rpc/protocol"
	"github.com/rs/zerolog"
)

// ===== zerologAdapter: zerolog -> client.Logger 어댑터 =====

// zerologAdapter는 zerolog.Logger를 client.Logger 인터페이스로 래핑합니다.
type zerologAdapter struct {
	logger zerolog.Logger
}

func (a zerologAdapter) Debug(msg string, keysAndValues ...interface{}) {
	event := a.logger.Debug()
	for i := 0; i+1 < len(keysAndValues); i += 2 {
		key, ok := keysAndValues[i].(string)
		if !ok {
			continue
		}
		event = event.Interface(key, keysAndValues[i+1])
	}
	event.Msg(msg)
}

func (a zerologAdapter) Warn(msg string, keysAndValues ...interface{}) {
	event := a.logger.Warn()
	for i := 0; i+1 < len(keysAndValues); i += 2 {
		key, ok := keysAndValues[i].(string)
		if !ok {
			continue
		}
		event = event.Interface(key, keysAndValues[i+1])
	}
	event.Msg(msg)
}

func (a zerologAdapter) Error(msg string, keysAndValues ...interface{}) {
	event := a.logger.Error()
	for i := 0; i+1 < len(keysAndValues); i += 2 {
		key, ok := keysAndValues[i].(string)
		if !ok {
			continue
		}
		event = event.Interface(key, keysAndValues[i+1])
	}
	event.Msg(msg)
}

// ===== AppServerProcess: Codex App Server 프로세스 관리자 =====

// 프로세스 관련 에러 변수 (Bridge 전용)
var (
	// ErrConnectionClosed는 JSON-RPC 연결이 종료되었을 때 반환됩니다.
	ErrConnectionClosed = fmt.Errorf("JSON-RPC 연결이 종료되었습니다")
	// ErrMaxRestartsExceeded는 최대 재시작 횟수를 초과했을 때 반환됩니다.
	ErrMaxRestartsExceeded = fmt.Errorf("최대 재시작 횟수를 초과했습니다")
	// ErrHandshakeTimeout은 초기화 핸드셰이크 타임아웃입니다.
	ErrHandshakeTimeout = fmt.Errorf("초기화 핸드셰이크 타임아웃")
	// ErrProcessNotRunning은 App Server 프로세스가 실행 중이 아닐 때 반환됩니다.
	ErrProcessNotRunning = fmt.Errorf("App Server 프로세스가 실행 중이 아닙니다")
)

// AppServerProcess는 Codex App Server 프로세스를 관리합니다.
// exec.Cmd를 사용하여 하위 프로세스를 시작하고, stdin/stdout 파이프를 통해
// JSON-RPC 2.0 프로토콜로 통신합니다.
type AppServerProcess struct {
	cmd          *exec.Cmd
	client       *client.Client
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

	// 공유 JSON-RPC 클라이언트 생성 (zerologAdapter 사용)
	p.client = client.NewJSONRPCClient(stdinPipe, stdoutPipe, zerologAdapter{p.logger})

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
func (p *AppServerProcess) Client() *client.Client {
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

	// initialize 요청 전송 (experimentalApi 능력 포함)
	result, err := p.client.Call(initCtx, protocol.MethodInitialize, protocol.InitializeParams{
		ClientInfo: protocol.ClientInfo{
			Name:    "autopus-bridge",
			Version: "1.0.0",
		},
		Capabilities: protocol.Capabilities{
			ExperimentalApi: true,
		},
	})
	if err != nil {
		if initCtx.Err() != nil {
			return ErrHandshakeTimeout
		}
		return fmt.Errorf("초기화 요청 실패: %w", err)
	}

	// 초기화 결과 파싱 (공유 프로토콜 타입 사용)
	if result != nil {
		var initResult protocol.InitializeResult
		if err := json.Unmarshal(*result, &initResult); err != nil {
			p.logger.Warn().Err(err).Msg("초기화 결과 파싱 실패 (무시)")
		} else {
			p.logger.Info().
				Str("serverName", initResult.ServerName).
				Str("serverVersion", initResult.ServerVersion).
				Msg("서버 초기화 완료")
		}
	}

	// initialized 알림 전송
	if err := p.client.Notify(protocol.MethodInitialized, nil); err != nil {
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
	authMethod     string // "apiKey", "chatgpt", "chatgptAuthTokens"
	authKey        string // API 키 또는 ChatGPT access token
	authAccountID  string // chatgptAuthTokens 전용 account ID
	rpcRelay       *approval.RPCRelay
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

// WithAppServerDefaultModel은 App Server의 기본 모델을 설정합니다.
func WithAppServerDefaultModel(model string) CodexAppServerOption {
	return func(p *CodexAppServerProvider) {
		p.config.DefaultModel = model
	}
}

// WithAppServerAuth는 인증 방식을 설정합니다.
// method:
//   - "apiKey"            : key=API Key
//   - "chatgpt"           : 저장된 로컬 인증 사용
//   - "chatgptAuthTokens" : key=access token, accountID=account id
func WithAppServerAuth(method, key, accountID string) CodexAppServerOption {
	return func(p *CodexAppServerProvider) {
		p.authMethod = method
		p.authKey = key
		p.authAccountID = accountID
	}
}

// NewCodexAppServerProvider는 새로운 CodexAppServerProvider를 생성합니다.
// 프로세스를 시작하고 인증을 수행한 후 프로바이더를 반환합니다.
func NewCodexAppServerProvider(cliPath string, opts ...CodexAppServerOption) (*CodexAppServerProvider, error) {
	p := &CodexAppServerProvider{
		approvalPolicy: "never",
		authMethod:     "apiKey",
		rpcRelay:       approval.NewRPCRelay("codex"),
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
// OpenRouter 형식(openai/o3-mini)과 레거시 형식 모두 지원합니다.
// Supports는 주어진 모델명을 지원하는지 확인합니다.
// gpt-*, o4-*, o3- 접두사를 가진 모든 모델을 지원하여 새 버전 자동 반영.
func (p *CodexAppServerProvider) Supports(model string) bool {
	bare := StripProviderPrefix(model)
	return strings.HasPrefix(bare, "gpt-") || strings.HasPrefix(bare, "o4-") || strings.HasPrefix(bare, "o3-")
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
	rpcClient := p.process.Client()
	if rpcClient == nil {
		return ErrProcessNotRunning
	}

	// 30초 타임아웃 컨텍스트
	authCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// 인증 파라미터 구성 (공유 프로토콜 타입 사용)
	params := protocol.AccountLoginParams{
		Type: p.authMethod,
	}
	switch p.authMethod {
	case "apiKey":
		params.APIKey = p.authKey
	case "chatgptAuthTokens":
		params.AccessToken = p.authKey
		params.ChatGPTAccountID = p.authAccountID
	case "chatgpt":
		// 저장된 로컬 인증을 사용하므로 추가 필드 없음
	default:
		return fmt.Errorf("지원하지 않는 인증 방식: %s", p.authMethod)
	}

	// 인증 요청 전송
	_, err := rpcClient.Call(authCtx, protocol.MethodAccountLoginStart, params)
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
// 2. 알림 핸들러 등록 (메시지 델타, 아이템 완료, 승인 요청, Turn 완료)
// 3. turn/start로 턴 시작
// 4. Turn 완료 대기
// 5. 결과 조립 및 반환
func (p *CodexAppServerProvider) executeInternal(ctx context.Context, req ExecuteRequest, onDelta StreamCallback) (*ExecuteResponse, error) {
	// 1. 클라이언트 확인
	rpcClient := p.process.Client()
	if rpcClient == nil || !p.process.IsRunning() {
		return nil, ErrProcessNotRunning
	}

	// 2. 승인 정책 결정 (Codex App Server 호환 매핑)
	approvalPolicy := p.approvalPolicy
	if approvalPolicy == "" {
		approvalPolicy = "auto-approve"
	}
	// Codex App Server는 auto-approve/deny-all을 인식하지 못하므로 변환
	// auto-approve → never (모든 도구 호출 자동 승인)
	// deny-all → reject (모든 도구 호출 거부)
	switch approvalPolicy {
	case "auto-approve", "auto-execute", "full-auto":
		approvalPolicy = "never"
	case "deny-all":
		approvalPolicy = "reject"
	}

	// 3. thread/start 호출
	model := req.Model
	if model == "" {
		model = p.config.DefaultModel
		if model == "" {
			model = "gpt-5.4"
		}
	}

	cwd := req.WorkDir
	if cwd == "" {
		cwd = "."
	}

	// tool_loop 모드일 때 dynamicTools로 네이티브 도구를 등록한다.
	// capabilities.experimentalApi = true 필요 (initialize 시 이미 설정됨).
	var dynamicTools []protocol.DynamicToolDefinition
	if req.ResponseMode == "tool_loop" && len(req.ToolDefinitions) > 0 {
		dynamicTools = make([]protocol.DynamicToolDefinition, 0, len(req.ToolDefinitions))
		for _, td := range req.ToolDefinitions {
			dt := protocol.DynamicToolDefinition{
				Name:        td.Name,
				Description: td.Description,
			}
			if len(td.InputSchema) > 0 {
				dt.InputSchema = td.InputSchema
			}
			dynamicTools = append(dynamicTools, dt)
		}
		p.logger.Info().Int("tool_count", len(dynamicTools)).Msg("tool_loop: dynamicTools 네이티브 등록")
	}

	threadResult, err := rpcClient.Call(ctx, protocol.MethodThreadStart, protocol.ThreadStartParams{
		Model:          model,
		Cwd:            cwd,
		ApprovalPolicy: approvalPolicy,
		DynamicTools:   dynamicTools,
	})
	if err != nil {
		return nil, fmt.Errorf("thread/start 실패: %w", err)
	}

	// thread/start 결과 파싱
	// Codex App Server는 {"thread":{"id":"..."},...} 형태로 응답하므로
	// 중첩된 thread.id를 추출한다.
	var threadID string
	if threadResult != nil {
		// 먼저 중첩 구조 시도: {"thread":{"id":"..."}}
		var nested struct {
			Thread struct {
				ID string `json:"id"`
			} `json:"thread"`
		}
		if err := json.Unmarshal(*threadResult, &nested); err == nil && nested.Thread.ID != "" {
			threadID = nested.Thread.ID
		} else {
			// 폴백: 플랫 구조 {"threadId":"..."}
			var flat protocol.ThreadStartResult
			if err := json.Unmarshal(*threadResult, &flat); err == nil {
				threadID = flat.ThreadID
			}
		}
		p.logger.Debug().Str("thread_id", threadID).Msg("thread/start 파싱 완료")
	} else {
		p.logger.Warn().Msg("thread/start 결과가 nil")
	}
	// thread 변수를 기존 코드와 호환되게 유지
	thread := protocol.ThreadStartResult{ThreadID: threadID}

	// 4. 알림 핸들러 등록
	turnDone := make(chan struct{})
	// sync.Once로 turnDone 채널을 한 번만 닫는다.
	// 구버전(turn/completed)과 신버전(codex/event/task_complete) 이벤트가 모두 도달할 수 있으므로
	// 중복 close로 인한 패닉을 방지한다.
	var turnDoneOnce sync.Once
	closeTurnDone := func() {
		turnDoneOnce.Do(func() {
			close(turnDone)
		})
	}

	var toolCalls []ToolCall
	var mu sync.Mutex // outputBuilder와 toolCalls 보호용 뮤텍스
	var outputBuilder strings.Builder

	// StreamAccumulator 생성 (스트리밍 모드일 때)
	var accumulator *StreamAccumulator
	if onDelta != nil {
		accumulator = NewStreamAccumulator()
	}

	// appendDelta는 텍스트 증분을 outputBuilder에 추가하고 스트리밍 콜백을 처리한다.
	// 구버전/신버전 이벤트 핸들러가 공통으로 사용한다.
	appendDelta := func(text string) {
		if text == "" {
			return
		}
		mu.Lock()
		outputBuilder.WriteString(text)
		mu.Unlock()

		// 스트리밍 콜백 처리 (StreamAccumulator는 자체 뮤텍스 보유)
		if onDelta != nil && accumulator != nil {
			accumulator.Add(text)
			if accumulator.ShouldFlush() {
				flushed := accumulator.Flush()
				onDelta(flushed, accumulator.GetAccumulated())
			}
		}
	}

	// 에이전트 메시지 델타 핸들러 — 구버전: item/agentMessage/delta
	// (공유 프로토콜: AgentMessageDelta.Delta)
	rpcClient.OnNotification(protocol.MethodAgentMessageDelta, func(method string, params json.RawMessage) {
		var delta protocol.AgentMessageDelta
		if err := json.Unmarshal(params, &delta); err != nil {
			p.logger.Warn().Err(err).Msg("에이전트 메시지 델타 파싱 실패 (구버전)")
			return
		}
		appendDelta(delta.Delta)
	})

	// 에이전트 메시지 델타 핸들러 — 신버전: codex/event/agent_message_delta
	// delta 필드를 우선 사용하고, 없으면 content 필드를 시도한다.
	rpcClient.OnNotification(protocol.MethodCodexAgentMessageDelta, func(method string, params json.RawMessage) {
		var raw struct {
			Delta   string `json:"delta"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal(params, &raw); err != nil {
			p.logger.Warn().Err(err).Msg("에이전트 메시지 델타 파싱 실패 (codex/event/agent_message_delta)")
			return
		}
		text := raw.Delta
		if text == "" {
			text = raw.Content
		}
		appendDelta(text)
	})

	// 에이전트 메시지 콘텐츠 델타 핸들러 — 신버전: codex/event/agent_message_content_delta
	// delta 필드를 우선 사용하고, 없으면 content 필드를 시도한다.
	rpcClient.OnNotification(protocol.MethodCodexAgentMessageContentDelta, func(method string, params json.RawMessage) {
		var raw struct {
			Delta   string `json:"delta"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal(params, &raw); err != nil {
			p.logger.Warn().Err(err).Msg("에이전트 메시지 콘텐츠 델타 파싱 실패 (codex/event/agent_message_content_delta)")
			return
		}
		text := raw.Delta
		if text == "" {
			text = raw.Content
		}
		appendDelta(text)
	})

	// 완성된 에이전트 메시지 핸들러 — 신버전: codex/event/agent_message
	// 모든 델타 이후 전체 텍스트를 포함하여 도달한다. 델타 누적과 중복을 피하기 위해
	// 현재 outputBuilder 내용이 비어있을 때만 반영한다.
	rpcClient.OnNotification(protocol.MethodCodexAgentMessage, func(method string, params json.RawMessage) {
		var raw struct {
			Text    string `json:"text"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal(params, &raw); err != nil {
			p.logger.Warn().Err(err).Msg("완성된 에이전트 메시지 파싱 실패 (codex/event/agent_message)")
			return
		}
		text := raw.Text
		if text == "" {
			text = raw.Content
		}
		if text == "" {
			return
		}
		// 델타 누적이 없었던 경우에만 전체 메시지를 사용한다.
		mu.Lock()
		if outputBuilder.Len() == 0 {
			outputBuilder.WriteString(text)
		}
		mu.Unlock()
	})

	// 아이템 완료 핸들러 (commandExecution, mcpToolCall, dynamicToolCall 등)
	// 구버전: item/completed — handleItemCompleted 로직을 함수로 분리하여 재사용한다.
	handleItemCompleted := func(method string, params json.RawMessage) {
		var item protocol.ItemCompletedParams
		if err := json.Unmarshal(params, &item); err != nil {
			p.logger.Warn().Err(err).Msg("아이템 완료 파싱 실패")
			return
		}

		switch item.ItemType {
		case "commandExecution":
			var cmdData protocol.CommandExecutionCompleted
			if err := json.Unmarshal(item.Data, &cmdData); err != nil {
				p.logger.Warn().Err(err).Msg("명령 실행 데이터 파싱 실패")
				return
			}

			inputData, _ := json.Marshal(map[string]interface{}{
				"command": cmdData.Command,
				"output":  cmdData.Output,
			})

			mu.Lock()
			toolCalls = append(toolCalls, ToolCall{
				ID:    item.ItemID,
				Name:  "command_execution",
				Input: inputData,
			})
			mu.Unlock()

		case "mcpToolCall":
			var mcpData protocol.MCPToolCallCompleted
			if err := json.Unmarshal(item.Data, &mcpData); err != nil {
				p.logger.Warn().Err(err).Msg("MCP 도구 호출 데이터 파싱 실패")
				return
			}

			inputData, _ := json.Marshal(map[string]string{"input": mcpData.Input})
			mu.Lock()
			toolCalls = append(toolCalls, ToolCall{
				ID:    item.ItemID,
				Name:  mcpData.ToolName,
				Input: inputData,
			})
			mu.Unlock()

		case "dynamicToolCall", "collabToolCall":
			// 동적/협업 도구 호출 완료 데이터 — 동일한 {tool, arguments} JSON 구조
			var tcData struct {
				Tool      string          `json:"tool"`
				Arguments json.RawMessage `json:"arguments"`
			}
			if err := json.Unmarshal(item.Data, &tcData); err != nil {
				p.logger.Warn().Err(err).Str("item_type", item.ItemType).Msg("도구 호출 완료 데이터 파싱 실패")
				return
			}
			mu.Lock()
			toolCalls = append(toolCalls, ToolCall{
				ID:    item.ItemID,
				Name:  tcData.Tool,
				Input: tcData.Arguments,
			})
			mu.Unlock()

		default:
			// 알 수 없는 아이템 타입 — 향후 프로토콜 확장을 위해 경고만 로깅하고 패닉하지 않는다.
			p.logger.Warn().
				Str("item_type", item.ItemType).
				Str("item_id", item.ItemID).
				Msg("알 수 없는 아이템 타입 수신 — 무시")
		}
	}

	// 구버전: item/completed 핸들러 등록
	rpcClient.OnNotification(protocol.MethodItemCompleted, handleItemCompleted)
	// 신버전: codex/event/item_completed 핸들러 등록 (v0.114.0+ 하위 호환)
	rpcClient.OnNotification(protocol.MethodCodexItemCompleted, handleItemCompleted)

	// item/tool/call 요청 핸들러 등록 (서버 -> 클라이언트 요청, id 있음)
	// 서버가 동적 도구 실행을 요청할 때 호출된다.
	// dynamicTools 네이티브 등록을 지원하므로 이 핸들러로 실제 도구 호출이 전달된다.
	rpcClient.OnRequest(protocol.MethodToolCall, func(id int64, method string, params json.RawMessage) interface{} {
		var dtcReq protocol.DynamicToolCallRequest
		if err := json.Unmarshal(params, &dtcReq); err != nil {
			p.logger.Warn().Err(err).Msg("item/tool/call 요청 파싱 실패")
			return protocol.DynamicToolCallResponse{
				Success:      false,
				ContentItems: []protocol.ContentItem{{Type: "inputText", Text: "요청 파싱 실패"}},
			}
		}

		// 도구 호출 ID 결정: CallID > ItemID 우선순위
		callID := dtcReq.CallID
		if callID == "" {
			callID = dtcReq.ItemID
		}

		mu.Lock()
		toolCalls = append(toolCalls, ToolCall{
			ID:    callID,
			Name:  dtcReq.Tool,
			Input: dtcReq.Arguments,
		})
		mu.Unlock()

		p.logger.Debug().
			Str("call_id", callID).
			Str("tool", dtcReq.Tool).
			Msg("동적 도구 호출 캡처 완료")

		return protocol.DynamicToolCallResponse{
			Success: true,
			ContentItems: []protocol.ContentItem{
				{Type: "inputText", Text: "Tool call captured by bridge"},
			},
		}
	})

	// 명령 실행 승인 요청 핸들러
	rpcClient.OnNotification(protocol.MethodCommandExecutionApproval, func(method string, params json.RawMessage) {
		var approvalReq protocol.ApprovalRequest
		if err := json.Unmarshal(params, &approvalReq); err != nil {
			p.logger.Warn().Err(err).Msg("승인 요청 파싱 실패")
			return
		}

		decision := "accept"
		if p.rpcRelay != nil && p.rpcRelay.SupportsApproval() {
			toolInput, _ := json.Marshal(approvalReq)
			d, err := p.rpcRelay.HandleRPCApproval(context.Background(), thread.ThreadID, "command_execution", toolInput)
			if err != nil || d.Decision == "deny" {
				decision = "decline"
			}
		} else if approvalPolicy == "reject" {
			decision = "decline"
		}

		response := protocol.ApprovalResponseParams{
			ThreadID: approvalReq.ThreadID,
			ItemID:   approvalReq.ItemID,
			Decision: decision,
		}

		if err := rpcClient.Notify("item/commandExecution/approvalResponse", response); err != nil {
			p.logger.Warn().Err(err).Msg("승인 응답 전송 실패")
		}
	})

	// 파일 변경 승인 요청 핸들러
	rpcClient.OnNotification(protocol.MethodFileChangeApproval, func(method string, params json.RawMessage) {
		var approvalReq protocol.ApprovalRequest
		if err := json.Unmarshal(params, &approvalReq); err != nil {
			p.logger.Warn().Err(err).Msg("파일 변경 승인 요청 파싱 실패")
			return
		}

		decision := "accept"
		if p.rpcRelay != nil && p.rpcRelay.SupportsApproval() {
			toolInput, _ := json.Marshal(approvalReq)
			d, err := p.rpcRelay.HandleRPCApproval(context.Background(), thread.ThreadID, "file_change", toolInput)
			if err != nil || d.Decision == "deny" {
				decision = "decline"
			}
		} else if approvalPolicy == "reject" {
			decision = "decline"
		}

		response := protocol.ApprovalResponseParams{
			ThreadID: approvalReq.ThreadID,
			ItemID:   approvalReq.ItemID,
			Decision: decision,
		}

		if err := rpcClient.Notify("item/fileChange/approvalResponse", response); err != nil {
			p.logger.Warn().Err(err).Msg("파일 변경 승인 응답 전송 실패")
		}
	})

	// Turn 완료 핸들러 — 구버전: turn/completed
	// sync.Once를 통해 turnDone 채널을 안전하게 한 번만 닫는다.
	turnStarted := make(chan struct{}) // turn/start 호출 후 닫힘
	rpcClient.OnNotification(protocol.MethodTurnCompleted, func(method string, params json.RawMessage) {
		// turn/start가 호출되기 전에 도착한 turn/completed는 무시한다
		select {
		case <-turnStarted:
			p.logger.Debug().Str("method", method).Msg("turn/completed 수신 → turnDone 닫기")
			closeTurnDone()
		default:
			p.logger.Warn().Str("method", method).Msg("turn/start 전에 도착한 turn/completed — 무시")
		}
	})

	// Turn 완료 핸들러 — 신버전: codex/event/task_complete (v0.114.0+)
	// 구버전과 신버전 이벤트가 동시에 도달할 수 있으므로 sync.Once로 보호한다.
	rpcClient.OnNotification(protocol.MethodCodexTaskComplete, func(method string, params json.RawMessage) {
		select {
		case <-turnStarted:
			p.logger.Debug().Str("method", method).Msg("task_complete 수신 → turnDone 닫기")
			closeTurnDone()
		default:
			p.logger.Warn().Str("method", method).Msg("turn/start 전에 도착한 task_complete — 무시")
		}
	})

	// 5. turn/start 호출
	// tool_loop 모드일 때 대화 이력을 프롬프트에 포함한다.
	turnPrompt := req.Prompt
	if req.ResponseMode == "tool_loop" && len(req.ToolLoopMessages) > 0 {
		turnPrompt = buildAppServerTurnPrompt(req)
	}

	_, err = rpcClient.Call(ctx, protocol.MethodTurnStart, protocol.TurnStartParams{
		ThreadID: thread.ThreadID,
		Input: []protocol.TurnInput{
			{
				Type: "text",
				Text: turnPrompt,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("turn/start 실패: %w", err)
	}
	close(turnStarted) // turn/start 완료 → 이제부터 turn/completed 수신 허용

	// 6. Turn 완료 또는 컨텍스트 취소 대기
	// resolveExecuteTimeout으로 계산한 타임아웃을 적용하여 행(hang) 방지
	// developerInstructions(~20KB)를 처리하는 1차 턴은 최대 77초 소요 확인됨
	turnTimeoutDur := resolveExecuteTimeout(req.Timeout)
	turnTimeout := time.After(turnTimeoutDur)
	select {
	case <-turnDone:
		// 정상 완료
	case <-ctx.Done():
		return nil, fmt.Errorf("실행 타임아웃: %w", ctx.Err())
	case <-turnTimeout:
		p.logger.Warn().
			Str("response_mode", req.ResponseMode).
			Dur("timeout", turnTimeoutDur).
			Msg("턴 타임아웃 초과")
		return nil, fmt.Errorf("턴 타임아웃: %v 초과", turnTimeoutDur)
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

	// 디버그: tool_loop 실행 결과 로깅
	if req.ResponseMode == "tool_loop" {
		outputPreview := output
		if len(outputPreview) > 500 {
			outputPreview = outputPreview[:500] + "...(truncated)"
		}
		p.logger.Info().
			Int("output_len", len(output)).
			Int("native_tool_calls", len(resultToolCalls)).
			Str("output_preview", outputPreview).
			Msg("tool_loop: 실행 결과")
	}

	// tool_loop 모드일 때 텍스트 출력에서 도구 호출 마커를 파싱한다.
	if req.ResponseMode == "tool_loop" && len(resultToolCalls) == 0 && output != "" {
		parsedCalls, remainingText := parseCLIToolCalls(output)
		if len(parsedCalls) > 0 {
			resultToolCalls = parsedCalls
			output = remainingText
			p.logger.Debug().Int("parsed_tool_calls", len(parsedCalls)).Msg("tool_loop: 텍스트에서 도구 호출 파싱 완료")
		}
	}

	resp := &ExecuteResponse{
		Output:    output,
		ToolCalls: resultToolCalls,
		Model:     model,
	}
	if len(resultToolCalls) > 0 {
		resp.StopReason = "tool_use"
	} else if req.ResponseMode == "tool_loop" {
		resp.StopReason = "end_turn"
	}
	return resp, nil
}

// buildAppServerToolInstructions는 Codex App Server의 developerInstructions에 주입할
// 도구 정의 텍스트를 생성한다. Codex v0.114.0은 ThreadStartParams.DynamicTools를
// 아직 지원하지 않으므로 프롬프트 주입 방식을 사용한다.
func buildAppServerToolInstructions(tools []ws.ToolDefinition) string {
	var sb strings.Builder

	sb.WriteString("=== AVAILABLE TOOLS ===\n")
	sb.WriteString("You have access to the following tools. When a task requires tool usage, you MUST call the appropriate tool.\n\n")

	for _, tool := range tools {
		sb.WriteString(fmt.Sprintf("Tool: %s\n", tool.Name))
		if tool.Description != "" {
			sb.WriteString(fmt.Sprintf("Description: %s\n", tool.Description))
		}
		if len(tool.InputSchema) > 0 && string(tool.InputSchema) != "null" {
			var schemaObj interface{}
			if err := json.Unmarshal(tool.InputSchema, &schemaObj); err == nil {
				if pretty, err := json.MarshalIndent(schemaObj, "  ", "  "); err == nil {
					sb.WriteString("Parameters:\n  ")
					sb.Write(pretty)
					sb.WriteString("\n")
				}
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString("=== TOOL CALL FORMAT ===\n")
	sb.WriteString("CRITICAL: When you need to call a tool, output EXACTLY this format:\n\n")
	sb.WriteString("<<<TOOL_CALL>>>\n")
	sb.WriteString(`{"name": "tool_name", "arguments": {"param1": "value1"}}`)
	sb.WriteString("\n<<<END_TOOL_CALL>>>\n\n")
	sb.WriteString("Rules:\n")
	sb.WriteString("- The JSON MUST be on a single line between the markers\n")
	sb.WriteString("- 'arguments' must match the tool's parameter schema exactly\n")
	sb.WriteString("- You can call multiple tools by outputting multiple <<<TOOL_CALL>>> blocks\n")
	sb.WriteString("- If you do NOT need to call a tool, respond with normal text only\n")
	sb.WriteString("- Always prefer calling tools over describing what you would do\n")

	return sb.String()
}

// buildAppServerTurnPrompt는 tool_loop 모드에서 대화 이력을 포함한 턴 프롬프트를 생성한다.
// 프롬프트 폭발을 방지하기 위해 최근 4개 메시지만 유지한다 (assistant+tool 쌍 2턴분).
func buildAppServerTurnPrompt(req ExecuteRequest) string {
	var sb strings.Builder

	// 대화 이력 윈도잉: 최근 4개 메시지만 포함 (이전 턴이 많으면 잘라냄)
	messages := req.ToolLoopMessages
	const maxHistoryMessages = 4
	if len(messages) > maxHistoryMessages {
		sb.WriteString("[Previous conversation history truncated for brevity]\n\n")
		messages = messages[len(messages)-maxHistoryMessages:]
	}

	for _, msg := range messages {
		switch msg.Role {
		case "user":
			sb.WriteString("[User]: ")
			sb.WriteString(msg.Content)
			sb.WriteString("\n\n")
		case "assistant":
			sb.WriteString("[Assistant]: ")
			if msg.Content != "" {
				sb.WriteString(msg.Content)
			}
			for _, tc := range msg.ToolCalls {
				sb.WriteString("\n<<<TOOL_CALL>>>\n")
				payload := struct {
					Name      string          `json:"name"`
					Arguments json.RawMessage `json:"arguments"`
				}{Name: tc.Name, Arguments: tc.Input}
				if data, err := json.Marshal(payload); err == nil {
					sb.Write(data)
				}
				sb.WriteString("\n<<<END_TOOL_CALL>>>")
			}
			sb.WriteString("\n\n")
		case "tool":
			sb.WriteString("[Tool Results]:\n")
			for _, tr := range msg.ToolResults {
				sb.WriteString(fmt.Sprintf("  %s: %s\n", tr.ToolName, tr.Content))
				if tr.IsError {
					sb.WriteString("  (ERROR)\n")
				}
			}
			sb.WriteString("\n")
		}
	}

	// 현재 사용자 프롬프트 추가
	if req.Prompt != "" {
		sb.WriteString("[User]: ")
		sb.WriteString(req.Prompt)
		sb.WriteString("\n\n")
	}

	return sb.String()
}

// resolveExecuteTimeout은 ExecuteRequest.Timeout 값을 실제 타임아웃 Duration으로 변환한다.
// - 0 또는 음수: 기본값 120초
// - 30초 미만: 30초로 클램핑
// - 600초 초과: 600초로 클램핑
// @MX:ANCHOR: [AUTO] 타임아웃 클램핑 규칙 변경 시 테스트(TestCodexAppServerProvider_TimeoutResolution) 반드시 갱신
// @MX:REASON: [AUTO] 기본값/최소/최대 클램핑 삼중 경계 — 단순 상수 변경도 테스트 실패로 이어짐
func resolveExecuteTimeout(timeoutSecs int) time.Duration {
	const (
		defaultTimeout = 120 * time.Second
		minTimeout     = 30 * time.Second
		maxTimeout     = 600 * time.Second
	)

	if timeoutSecs <= 0 {
		return defaultTimeout
	}

	d := time.Duration(timeoutSecs) * time.Second
	if d < minTimeout {
		return minTimeout
	}
	if d > maxTimeout {
		return maxTimeout
	}
	return d
}

// Steer는 진행 중인 Turn의 방향을 전환합니다.
// turn/steer JSON-RPC 요청을 전송하여 에이전트의 다음 행동을 유도합니다.
func (p *CodexAppServerProvider) Steer(ctx context.Context, threadID string, input json.RawMessage) error {
	rpcClient := p.process.Client()
	if rpcClient == nil || !p.process.IsRunning() {
		return ErrProcessNotRunning
	}

	_, err := rpcClient.Call(ctx, protocol.MethodTurnSteer, protocol.TurnSteerParams{
		ThreadID: threadID,
		Input:    input,
	})
	if err != nil {
		return fmt.Errorf("turn/steer 실패: %w", err)
	}
	return nil
}

// Close는 프로바이더를 종료합니다. 프로세스를 중지합니다.
func (p *CodexAppServerProvider) Close() error {
	return p.process.Stop()
}

// SupportsApproval은 이 프로바이더가 인터랙티브 승인을 지원하는지 반환합니다.
func (p *CodexAppServerProvider) SupportsApproval() bool {
	return p.rpcRelay.SupportsApproval()
}

// SetApprovalHandler는 승인 요청을 처리할 핸들러를 등록합니다.
func (p *CodexAppServerProvider) SetApprovalHandler(handler approval.ApprovalHandler) {
	p.rpcRelay.SetApprovalHandler(handler)
}

// compile-time 인터페이스 구현 확인
var _ approval.ApprovalRelay = (*CodexAppServerProvider)(nil)
