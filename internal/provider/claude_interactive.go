// Package provider는 AI 프로바이더 통합 레이어를 제공합니다.
package provider

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/creack/pty"
	"github.com/rs/zerolog"

	"github.com/insajin/autopus-bridge/internal/approval"
	"github.com/insajin/autopus-bridge/internal/hook"
)

// InteractiveClaudeCLIProvider는 PTY 기반의 인터랙티브 Claude CLI 프로바이더입니다.
// Claude Code를 --print 없이 인터랙티브 모드로 실행하고,
// PreToolUse/PostToolUse 훅을 통해 도구 승인 흐름을 지원합니다.
//
// Provider와 approval.ApprovalRelay 인터페이스를 모두 구현합니다.
type InteractiveClaudeCLIProvider struct {
	// cliPath는 claude CLI 바이너리 경로입니다.
	cliPath string
	// timeout은 CLI 실행 타임아웃입니다.
	timeout time.Duration
	// config는 프로바이더 공통 설정입니다.
	config ProviderConfig
	// approvalPolicy는 도구 승인 정책입니다.
	approvalPolicy approval.ApprovalPolicy
	// hookServerPort는 훅 서버 포트입니다 (0 = 자동 할당).
	hookServerPort int
	// approvalTimeout은 승인 대기 타임아웃입니다.
	approvalTimeout time.Duration
	// approvalHandler는 외부에서 주입된 승인 핸들러입니다.
	approvalHandler approval.ApprovalHandler
	// logger는 구조화된 로거입니다.
	logger zerolog.Logger
}

// InteractiveClaudeCLIProviderOption은 InteractiveClaudeCLIProvider 설정 옵션입니다.
type InteractiveClaudeCLIProviderOption func(*InteractiveClaudeCLIProvider)

// WithInteractiveCLIPath는 CLI 바이너리 경로를 설정합니다.
func WithInteractiveCLIPath(path string) InteractiveClaudeCLIProviderOption {
	return func(p *InteractiveClaudeCLIProvider) {
		p.cliPath = path
	}
}

// WithInteractiveTimeout은 CLI 실행 타임아웃을 설정합니다.
func WithInteractiveTimeout(timeout time.Duration) InteractiveClaudeCLIProviderOption {
	return func(p *InteractiveClaudeCLIProvider) {
		p.timeout = timeout
	}
}

// WithInteractiveDefaultModel은 기본 모델을 설정합니다.
func WithInteractiveDefaultModel(model string) InteractiveClaudeCLIProviderOption {
	return func(p *InteractiveClaudeCLIProvider) {
		p.config.DefaultModel = model
	}
}

// WithInteractiveApprovalPolicy는 승인 정책을 설정합니다.
func WithInteractiveApprovalPolicy(policy approval.ApprovalPolicy) InteractiveClaudeCLIProviderOption {
	return func(p *InteractiveClaudeCLIProvider) {
		p.approvalPolicy = policy
	}
}

// WithInteractiveHookServerPort는 훅 서버 포트를 설정합니다.
func WithInteractiveHookServerPort(port int) InteractiveClaudeCLIProviderOption {
	return func(p *InteractiveClaudeCLIProvider) {
		p.hookServerPort = port
	}
}

// WithInteractiveApprovalTimeout은 승인 대기 타임아웃을 설정합니다.
func WithInteractiveApprovalTimeout(timeout time.Duration) InteractiveClaudeCLIProviderOption {
	return func(p *InteractiveClaudeCLIProvider) {
		p.approvalTimeout = timeout
	}
}

// WithInteractiveLogger는 로거를 설정합니다.
func WithInteractiveLogger(logger zerolog.Logger) InteractiveClaudeCLIProviderOption {
	return func(p *InteractiveClaudeCLIProvider) {
		p.logger = logger
	}
}

// NewInteractiveClaudeCLIProvider는 새로운 InteractiveClaudeCLIProvider를 생성합니다.
func NewInteractiveClaudeCLIProvider(opts ...InteractiveClaudeCLIProviderOption) (*InteractiveClaudeCLIProvider, error) {
	p := &InteractiveClaudeCLIProvider{
		cliPath:         "claude",
		timeout:         10 * time.Minute,
		approvalPolicy:  approval.ApprovalPolicyAutoApprove,
		approvalTimeout: 5 * time.Minute,
		logger:          zerolog.Nop(),
		config: ProviderConfig{
			DefaultModel: "claude-sonnet-4-20250514",
		},
	}

	for _, opt := range opts {
		opt(p)
	}

	p.logger = p.logger.With().Str("component", "claude-interactive").Logger()

	// CLI 바이너리 존재 확인
	if err := p.validateCLI(); err != nil {
		return nil, err
	}

	return p, nil
}

// Name은 프로바이더 식별자를 반환합니다.
func (p *InteractiveClaudeCLIProvider) Name() string {
	return "claude-interactive"
}

// ValidateConfig는 프로바이더 설정의 유효성을 검사합니다.
func (p *InteractiveClaudeCLIProvider) ValidateConfig() error {
	return p.validateCLI()
}

// Supports는 주어진 모델명을 지원하는지 확인합니다.
// claude- 접두사를 가진 모든 모델을 지원합니다.
func (p *InteractiveClaudeCLIProvider) Supports(model string) bool {
	return strings.HasPrefix(model, "claude-")
}

// SupportsApproval은 이 프로바이더가 인터랙티브 승인을 지원하는지 반환합니다.
// InteractiveClaudeCLIProvider는 항상 승인 흐름을 지원합니다.
func (p *InteractiveClaudeCLIProvider) SupportsApproval() bool {
	return true
}

// SetApprovalHandler는 승인 요청을 처리할 핸들러를 등록합니다.
// 이 핸들러는 PreToolUse 훅이 수신될 때 호출됩니다.
func (p *InteractiveClaudeCLIProvider) SetApprovalHandler(handler approval.ApprovalHandler) {
	p.approvalHandler = handler
}

// Execute는 Claude CLI를 인터랙티브 모드(PTY)로 실행하고 결과를 반환합니다.
//
// 실행 흐름:
//  1. 세션별 토큰 생성 (crypto/rand)
//  2. ApprovalManager, HookHandler, HookServer 생성 및 시작
//  3. 훅 설정이 포함된 세션 디렉토리 생성
//  4. PTY를 통해 Claude CLI를 인터랙티브 모드로 실행
//  5. 프롬프트를 stdin으로 전달하고 stdout 캡처
//  6. 프로세스 종료 대기 및 결과 반환
//  7. 정리: 훅 서버 중지, 세션 디렉토리 삭제
func (p *InteractiveClaudeCLIProvider) Execute(ctx context.Context, req ExecuteRequest) (*ExecuteResponse, error) {
	startTime := time.Now()

	// 모델 결정
	model := req.Model
	if model == "" {
		model = p.config.DefaultModel
	}
	if !p.Supports(model) {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedModel, model)
	}

	// 세션별 보안 토큰 생성
	sessionToken, err := generateSecureToken(32)
	if err != nil {
		return nil, fmt.Errorf("세션 토큰 생성 실패: %w", err)
	}

	// 승인 매니저 생성
	approvalMgr := hook.NewApprovalManager(p.approvalTimeout, p.logger)

	// OnApproval 콜백: 훅 핸들러에서 PreToolUse 수신 시 승인 흐름 브릿지
	onApproval := p.createApprovalBridge(ctx, approvalMgr)

	// 훅 핸들러 생성
	hookHandler := hook.NewHookHandler(
		approvalMgr,
		hook.WithOnApproval(onApproval),
		hook.WithHandlerLogger(p.logger),
	)

	// 훅 서버 생성 및 시작
	hookServer := hook.NewHookServer(
		hookHandler,
		sessionToken,
		hook.WithPort(p.hookServerPort),
		hook.WithLogger(p.logger),
	)

	actualPort, err := hookServer.Start(ctx)
	if err != nil {
		return nil, fmt.Errorf("훅 서버 시작 실패: %w", err)
	}
	defer func() {
		if stopErr := hookServer.Stop(); stopErr != nil {
			p.logger.Warn().Err(stopErr).Msg("훅 서버 중지 실패")
		}
	}()

	p.logger.Info().
		Int("port", actualPort).
		Msg("인터랙티브 모드 훅 서버 시작됨")

	// 세션 디렉토리 생성 (훅 설정 포함)
	baseDir := os.TempDir()
	sessionID := fmt.Sprintf("interactive-%d", time.Now().UnixNano())
	sessionDir, cleanup, err := hook.GenerateSessionDir(baseDir, sessionID, actualPort, sessionToken)
	if err != nil {
		return nil, fmt.Errorf("세션 디렉토리 생성 실패: %w", err)
	}
	defer cleanup()

	p.logger.Debug().
		Str("session_dir", sessionDir).
		Str("session_id", sessionID).
		Msg("세션 디렉토리 생성 완료")

	// 타임아웃 컨텍스트 생성
	execCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	// CLI 명령 구성 (인터랙티브 모드: --print 없음!)
	args := []string{
		"--model", model,
		"--verbose",
	}

	// 시스템 프롬프트 설정
	if req.SystemPrompt != "" {
		args = append(args, "--system-prompt", req.SystemPrompt)
	}

	// 도구 설정
	if len(req.Tools) > 0 {
		args = append(args, "--allowedTools", strings.Join(req.Tools, ","))
	}

	cmd := exec.CommandContext(execCtx, p.cliPath, args...)

	// 환경변수: CLAUDE_CONFIG_DIR을 세션 디렉토리로 설정
	cmd.Env = append(os.Environ(), "CLAUDE_CONFIG_DIR="+sessionDir)

	// 작업 디렉토리 설정
	if req.WorkDir != "" {
		if _, statErr := os.Stat(req.WorkDir); statErr == nil {
			cmd.Dir = req.WorkDir
		}
	}

	// PTY를 통해 프로세스 시작
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{
		Rows: 40,
		Cols: 120,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: PTY 시작 실패: %v", ErrCLIExecution, err)
	}
	defer func() {
		_ = ptmx.Close()
	}()

	// 프롬프트를 stdin으로 전달 (개행 추가)
	prompt := req.Prompt + "\n"
	if _, writeErr := ptmx.Write([]byte(prompt)); writeErr != nil {
		return nil, fmt.Errorf("%w: 프롬프트 전달 실패: %v", ErrCLIExecution, writeErr)
	}

	// stdout 캡처 (고루틴에서 PTY 출력 읽기)
	var outputBuf bytes.Buffer
	readDone := make(chan error, 1)
	go func() {
		_, copyErr := io.Copy(&outputBuf, ptmx)
		readDone <- copyErr
	}()

	// 프로세스 종료 대기
	waitErr := cmd.Wait()

	// PTY 읽기 완료 대기 (짧은 타임아웃)
	select {
	case <-readDone:
		// 읽기 완료
	case <-time.After(2 * time.Second):
		// PTY가 닫히면 읽기도 종료되므로 짧은 대기 후 계속 진행
		p.logger.Debug().Msg("PTY 읽기 타임아웃, 계속 진행")
	}

	// 승인 매니저 정리
	approvalMgr.CancelAll()

	// 컨텍스트 취소 확인
	if execCtx.Err() == context.DeadlineExceeded {
		return nil, fmt.Errorf("%w: %v초 후 타임아웃", ErrCLITimeout, p.timeout.Seconds())
	}
	if execCtx.Err() == context.Canceled {
		return nil, fmt.Errorf("%w: %v", ErrContextCanceled, ctx.Err())
	}

	// 실행 에러 확인 (PTY 모드에서는 exit code != 0 이 발생할 수 있음)
	output := outputBuf.String()
	if waitErr != nil && output == "" {
		return nil, fmt.Errorf("%w: %v", ErrCLIExecution, waitErr)
	}

	// ANSI 이스케이프 시퀀스 제거 (PTY 출력에 포함됨)
	cleanOutput := stripANSI(output)

	// 토큰 사용량 추정 (인터랙티브 모드에서는 정확한 토큰 정보가 없음)
	estimatedInputTokens := len(req.Prompt) / 4
	estimatedOutputTokens := len(cleanOutput) / 4

	return &ExecuteResponse{
		Output: cleanOutput,
		TokenUsage: TokenUsage{
			InputTokens:  estimatedInputTokens,
			OutputTokens: estimatedOutputTokens,
			TotalTokens:  estimatedInputTokens + estimatedOutputTokens,
		},
		DurationMs: time.Since(startTime).Milliseconds(),
		Model:      model,
		StopReason: "end_turn",
	}, nil
}

// createApprovalBridge는 훅 핸들러의 OnApproval 콜백과 approval.ApprovalHandler를
// 연결하는 브릿지 함수를 생성합니다.
//
// 흐름:
//  1. HookHandler가 PreToolUse를 수신하면 OnApproval 콜백을 고루틴에서 호출
//  2. 콜백이 hook.HookRequest를 approval.ToolApprovalRequest로 변환
//  3. approvalHandler를 호출하여 ToolApprovalDecision을 받음
//  4. ToolApprovalDecision을 hook.ApprovalDecision으로 변환
//  5. ApprovalManager.DeliverDecision으로 결과를 훅 핸들러에 전달
func (p *InteractiveClaudeCLIProvider) createApprovalBridge(
	ctx context.Context,
	approvalMgr *hook.ApprovalManager,
) hook.OnApprovalFunc {
	return func(_ context.Context, req hook.HookRequest) {
		// approvalHandler가 설정되지 않은 경우 자동 허용
		if p.approvalHandler == nil {
			p.logger.Debug().
				Str("approval_id", req.ApprovalID).
				Str("tool_name", req.ToolName).
				Msg("승인 핸들러 없음, 자동 허용")

			_ = approvalMgr.DeliverDecision(req.ApprovalID, hook.ApprovalDecision{
				Allow:  true,
				Reason: "no approval handler configured, auto-allow",
			})
			return
		}

		// hook.HookRequest → approval.ToolApprovalRequest 변환
		approvalReq := approval.ToolApprovalRequest{
			ApprovalID:   req.ApprovalID,
			ProviderName: "claude-interactive",
			ToolName:     req.ToolName,
			ToolInput:    req.ToolInput,
			SessionID:    req.SessionID,
			RequestedAt:  time.Now(),
		}

		p.logger.Debug().
			Str("approval_id", req.ApprovalID).
			Str("tool_name", req.ToolName).
			Msg("승인 핸들러로 승인 요청 전달")

		// 승인 핸들러 호출 (ApprovalRouter로 라우팅됨)
		decision, err := p.approvalHandler(ctx, approvalReq)

		// approval.ToolApprovalDecision → hook.ApprovalDecision 변환
		var hookDecision hook.ApprovalDecision
		if err != nil {
			p.logger.Error().
				Err(err).
				Str("approval_id", req.ApprovalID).
				Msg("승인 핸들러 에러, 거부 처리")
			hookDecision = hook.ApprovalDecision{
				Allow:  false,
				Reason: fmt.Sprintf("approval handler error: %v", err),
			}
		} else {
			hookDecision = hook.ApprovalDecision{
				Allow:  decision.Decision == "allow",
				Reason: decision.Reason,
			}
		}

		// 결과를 훅 핸들러에 전달
		if deliverErr := approvalMgr.DeliverDecision(req.ApprovalID, hookDecision); deliverErr != nil {
			p.logger.Warn().
				Err(deliverErr).
				Str("approval_id", req.ApprovalID).
				Msg("승인 결정 전달 실패 (이미 타임아웃되었을 수 있음)")
		}
	}
}

// validateCLI는 claude CLI 바이너리가 존재하고 실행 가능한지 확인합니다.
func (p *InteractiveClaudeCLIProvider) validateCLI() error {
	path, err := exec.LookPath(p.cliPath)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrCLINotFound, p.cliPath)
	}
	p.cliPath = path
	return nil
}

// generateSecureToken은 암호학적으로 안전한 랜덤 토큰을 생성합니다.
func generateSecureToken(length int) (string, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("crypto/rand 읽기 실패: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// stripANSI는 문자열에서 ANSI 이스케이프 시퀀스를 제거합니다.
// PTY 출력에는 터미널 제어 시퀀스가 포함되므로 정리가 필요합니다.
func stripANSI(s string) string {
	var buf bytes.Buffer
	inEscape := false
	for i := 0; i < len(s); i++ {
		if s[i] == 0x1b { // ESC
			inEscape = true
			continue
		}
		if inEscape {
			// CSI 시퀀스: ESC [ ... 최종 바이트(0x40-0x7E)
			if s[i] == '[' {
				// CSI 파라미터 및 중간 바이트 건너뛰기
				for i++; i < len(s); i++ {
					if s[i] >= 0x40 && s[i] <= 0x7E {
						break
					}
				}
				inEscape = false
				continue
			}
			// OSC 시퀀스: ESC ] ... ST(ESC \ 또는 BEL)
			if s[i] == ']' {
				for i++; i < len(s); i++ {
					if s[i] == 0x07 { // BEL
						break
					}
					if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '\\' {
						i++ // ST 건너뛰기
						break
					}
				}
				inEscape = false
				continue
			}
			// 기타 이스케이프 시퀀스 (단일 문자)
			inEscape = false
			continue
		}
		// 제어 문자 필터링 (개행/탭 제외)
		if s[i] < 0x20 && s[i] != '\n' && s[i] != '\t' && s[i] != '\r' {
			continue
		}
		buf.WriteByte(s[i])
	}
	return buf.String()
}

// compile-time 인터페이스 구현 확인
var (
	_ Provider              = (*InteractiveClaudeCLIProvider)(nil)
	_ approval.ApprovalRelay = (*InteractiveClaudeCLIProvider)(nil)
)

// parseInteractiveOutput은 인터랙티브 모드 출력에서 JSON 결과를 파싱합니다.
// Claude Code는 인터랙티브 모드에서도 verbose 출력에 JSON을 포함할 수 있습니다.
func parseInteractiveOutput(output string) *ClaudeCLIResponse {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// JSON 객체인지 빠르게 확인
		if !strings.HasPrefix(line, "{") {
			continue
		}
		var parsed ClaudeCLIResponse
		if err := json.Unmarshal([]byte(line), &parsed); err != nil {
			continue
		}
		if parsed.Type == "result" {
			return &parsed
		}
	}
	return nil
}
