// Package executor - Codex App Server 기반 코딩 세션
package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"sync"

	"github.com/insajin/autopus-codex-rpc/client"
	"github.com/insajin/autopus-codex-rpc/protocol"
	"github.com/rs/zerolog/log"
)

// codexThreadStartParams는 thread/start 요청에 필요한 내부 파라미터입니다.
// 테스트 모의 객체가 일치하도록 별도 타입으로 정의합니다.
type codexThreadStartParams struct {
	// WorkDir은 작업 디렉토리입니다.
	WorkDir string
	// Model은 사용할 모델입니다.
	Model string
	// SystemPrompt는 개발자 지침(시스템 프롬프트)입니다.
	SystemPrompt string
}

// CodexRPCClient는 Codex App Server JSON-RPC 클라이언트 인터페이스입니다.
// 테스트에서 모의 객체로 교체할 수 있도록 추상화합니다.
// @MX:ANCHOR: [AUTO] CodexSession 테스트 격리를 위한 RPC 추상화 인터페이스 (fan_in >= 3)
// @MX:REASON: CodexSession, mockCodexRPC(테스트), rpcClientAdapter에서 구현
type CodexRPCClient interface {
	// ThreadStart는 새 Thread를 시작하고 ThreadID를 반환합니다.
	ThreadStart(ctx context.Context, params codexThreadStartParams) (string, error)
	// ThreadResume는 기존 Thread를 재개합니다.
	ThreadResume(ctx context.Context, threadID string) error
	// TurnRun은 Turn을 실행하고 응답 텍스트를 반환합니다.
	TurnRun(ctx context.Context, threadID, message string) (string, error)
	// Close는 RPC 클라이언트를 종료합니다.
	Close() error
}

// rpcClientAdapter는 autopus-codex-rpc client.Client를 CodexRPCClient로 래핑합니다.
// @MX:NOTE: [AUTO] turn/completed 알림을 OnNotification 핸들러로 비동기 수신 후 채널로 동기화
type rpcClientAdapter struct {
	// c는 JSON-RPC 2.0 클라이언트입니다.
	c *client.Client
}

// newRPCClientAdapter는 codex CLI를 실행하고 RPC 어댑터를 생성합니다.
func newRPCClientAdapter(ctx context.Context) (*rpcClientAdapter, error) {
	cliPath, err := exec.LookPath("codex")
	if err != nil {
		return nil, fmt.Errorf("codex CLI를 찾을 수 없습니다: %w", err)
	}

	cmd := exec.CommandContext(ctx, cliPath, "app-server")

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin 파이프 생성 실패: %w", err)
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout 파이프 생성 실패: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("codex 프로세스 시작 실패: %w", err)
	}

	rpcClient := client.NewJSONRPCClient(stdinPipe, stdoutPipe, client.NopLogger())

	// 초기화 핸드셰이크 수행
	if _, err := rpcClient.Call(ctx, protocol.MethodInitialize, protocol.InitializeParams{
		ClientInfo:   protocol.ClientInfo{Name: "autopus-bridge", Version: "1.0.0"},
		Capabilities: protocol.Capabilities{ExperimentalApi: true},
	}); err != nil {
		_ = rpcClient.Close()
		_ = cmd.Process.Kill()
		return nil, fmt.Errorf("초기화 핸드셰이크 실패: %w", err)
	}

	if err := rpcClient.Notify(protocol.MethodInitialized, nil); err != nil {
		_ = rpcClient.Close()
		_ = cmd.Process.Kill()
		return nil, fmt.Errorf("initialized 알림 전송 실패: %w", err)
	}

	return &rpcClientAdapter{c: rpcClient}, nil
}

// ThreadStart는 새 Thread를 시작합니다.
func (a *rpcClientAdapter) ThreadStart(ctx context.Context, params codexThreadStartParams) (string, error) {
	startParams := protocol.ThreadStartParams{
		Cwd:            params.WorkDir,
		Model:          params.Model,
		ApprovalPolicy: "never",
		Sandbox:        "danger-full-access",
	}
	if params.SystemPrompt != "" {
		startParams.DeveloperInstructions = params.SystemPrompt
	}

	result, err := a.c.Call(ctx, protocol.MethodThreadStart, startParams)
	if err != nil {
		return "", fmt.Errorf("thread/start 실패: %w", err)
	}
	if result == nil {
		return "", fmt.Errorf("thread/start 응답이 nil입니다")
	}

	var threadResult protocol.ThreadStartResult
	if err := json.Unmarshal(*result, &threadResult); err != nil {
		return "", fmt.Errorf("thread/start 응답 파싱 실패: %w", err)
	}

	return threadResult.ThreadID, nil
}

// ThreadResume는 기존 Thread를 재개합니다.
func (a *rpcClientAdapter) ThreadResume(ctx context.Context, threadID string) error {
	params := protocol.ThreadResumeParams{ThreadID: threadID}
	if _, err := a.c.Call(ctx, protocol.MethodThreadResume, params); err != nil {
		return fmt.Errorf("thread/resume 실패: %w", err)
	}
	return nil
}

// TurnRun은 turn/start를 호출하고 turn/completed 알림을 대기하여 응답 텍스트를 반환합니다.
// @MX:NOTE: [AUTO] 채널 기반 동기 대기: OnNotification 등록 → turn/start 호출 → 채널 수신
func (a *rpcClientAdapter) TurnRun(ctx context.Context, threadID, message string) (string, error) {
	doneCh := make(chan string, 1)
	var contentBuilder strings.Builder
	var mu sync.Mutex

	// agentMessage/delta: 텍스트 증분 수집 (구 버전)
	a.c.OnNotification(protocol.MethodAgentMessageDelta, func(method string, params json.RawMessage) {
		var delta struct {
			Delta string `json:"delta"`
		}
		if err := json.Unmarshal(params, &delta); err == nil && delta.Delta != "" {
			mu.Lock()
			contentBuilder.WriteString(delta.Delta)
			mu.Unlock()
		}
	})

	// codex/event/agent_message_content_delta: 텍스트 증분 수집 (v0.114.0+)
	a.c.OnNotification(protocol.MethodCodexAgentMessageContentDelta, func(method string, params json.RawMessage) {
		var delta struct {
			Delta   string `json:"delta"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal(params, &delta); err == nil {
			mu.Lock()
			if delta.Delta != "" {
				contentBuilder.WriteString(delta.Delta)
			} else if delta.Content != "" {
				contentBuilder.WriteString(delta.Content)
			}
			mu.Unlock()
		}
	})

	// turn/completed: 완료 신호 (구 버전)
	a.c.OnNotification(protocol.MethodTurnCompleted, func(method string, params json.RawMessage) {
		mu.Lock()
		content := contentBuilder.String()
		mu.Unlock()
		select {
		case doneCh <- content:
		default:
		}
	})

	// codex/event/task_complete: 완료 신호 (v0.114.0+)
	a.c.OnNotification(protocol.MethodCodexTaskComplete, func(method string, params json.RawMessage) {
		mu.Lock()
		content := contentBuilder.String()
		mu.Unlock()
		select {
		case doneCh <- content:
		default:
		}
	})

	// turn/start 요청 전송
	turnParams := protocol.TurnStartParams{
		ThreadID: threadID,
		Input:    []protocol.TurnInput{{Type: "text", Text: message}},
	}
	if _, err := a.c.Call(ctx, protocol.MethodTurnStart, turnParams); err != nil {
		return "", fmt.Errorf("turn/start 실패: %w", err)
	}

	// 완료 대기
	select {
	case content := <-doneCh:
		return content, nil
	case <-ctx.Done():
		return "", fmt.Errorf("turn 완료 대기 타임아웃: %w", ctx.Err())
	}
}

// Close는 RPC 클라이언트를 종료합니다.
func (a *rpcClientAdapter) Close() error {
	return a.c.Close()
}

// CodexSession은 Codex App Server와의 코딩 세션입니다.
// CodingSession 인터페이스를 구현하며, autopus-codex-rpc JSON-RPC 프로토콜을 사용합니다.
// @MX:ANCHOR: [AUTO] Codex CLI 기반 CodingSession 구현체 (fan_in >= 3)
// @MX:REASON: CodingSessionManager.CreateSession, coding_session_test.go, codex_session_test.go에서 사용
// @MX:SPEC: SPEC-CODING-RELAY-001 TASK-011
type CodexSession struct {
	// rpc는 Codex App Server JSON-RPC 클라이언트입니다.
	rpc CodexRPCClient
	// threadID는 현재 Codex Thread ID (SessionID로 사용)입니다.
	threadID string
	// opened는 Open이 호출되었는지 여부입니다.
	opened bool
}

// NewCodexSession은 새로운 CodexSession을 생성합니다.
// 실제 codex CLI 프로세스는 Open 호출 시에 시작합니다.
func NewCodexSession() *CodexSession {
	return &CodexSession{}
}

// NewCodexSessionWithClient는 커스텀 RPC 클라이언트로 CodexSession을 생성합니다.
// 테스트에서 모의 객체를 주입할 때 사용합니다.
func NewCodexSessionWithClient(rpc CodexRPCClient) *CodexSession {
	return &CodexSession{rpc: rpc}
}

// Open은 Codex App Server 프로세스를 시작하고 Thread를 생성합니다.
// ResumeSession이 있으면 기존 Thread를 재개합니다.
func (s *CodexSession) Open(ctx context.Context, req CodingSessionOpenRequest) error {
	// RPC 클라이언트가 없으면 실제 프로세스 시작
	if s.rpc == nil {
		adapter, err := newRPCClientAdapter(ctx)
		if err != nil {
			return fmt.Errorf("Codex RPC 클라이언트 시작 실패: %w", err)
		}
		s.rpc = adapter
	}

	// ResumeSession이 있으면 Thread 재개
	if req.ResumeSession != "" {
		if err := s.rpc.ThreadResume(ctx, req.ResumeSession); err != nil {
			return fmt.Errorf("Thread 재개 실패: %w", err)
		}
		s.threadID = req.ResumeSession
		s.opened = true
		return nil
	}

	// 새 Thread 시작
	threadID, err := s.rpc.ThreadStart(ctx, codexThreadStartParams{
		WorkDir:      req.WorkDir,
		Model:        req.Model,
		SystemPrompt: req.SystemPrompt,
	})
	if err != nil {
		return fmt.Errorf("Thread 시작 실패: %w", err)
	}

	s.threadID = threadID
	s.opened = true
	log.Debug().Str("threadID", threadID).Msg("[codex-session] Thread 시작 완료")
	return nil
}

// Send는 Codex에 메시지를 전송하고 응답을 반환합니다.
// turn/start → turn/completed 흐름으로 동기 응답을 반환합니다.
func (s *CodexSession) Send(ctx context.Context, message string) (*CodingSessionResponse, error) {
	if !s.opened {
		return nil, fmt.Errorf("세션이 열려 있지 않습니다")
	}

	content, err := s.rpc.TurnRun(ctx, s.threadID, message)
	if err != nil {
		return nil, fmt.Errorf("Turn 실행 실패: %w", err)
	}

	return &CodingSessionResponse{
		Content:   content,
		SessionID: s.threadID,
	}, nil
}

// SessionID는 현재 Codex Thread ID를 반환합니다.
func (s *CodexSession) SessionID() string {
	return s.threadID
}

// Close는 RPC 클라이언트를 종료합니다.
func (s *CodexSession) Close(_ context.Context) error {
	if s.rpc == nil {
		return nil
	}
	return s.rpc.Close()
}
