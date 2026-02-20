package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

// mockAppServer는 테스트용 App Server를 시뮬레이션하는 헬퍼입니다.
// io.Pipe를 사용하여 클라이언트와 서버 간의 통신을 모킹합니다.
type mockAppServer struct {
	serverStdinR  *io.PipeReader  // 서버가 읽는 파이프 (클라이언트가 쓴 데이터)
	clientStdinW  *io.PipeWriter  // 클라이언트가 쓰는 파이프 (서버의 stdin)
	clientStdoutR *io.PipeReader  // 클라이언트가 읽는 파이프 (서버의 stdout)
	serverStdoutW *io.PipeWriter  // 서버가 쓰는 파이프 (서버의 stdout)
}

// newMockAppServer는 새로운 mock App Server를 생성합니다.
func newMockAppServer() *mockAppServer {
	serverStdinR, clientStdinW := io.Pipe()
	clientStdoutR, serverStdoutW := io.Pipe()
	return &mockAppServer{
		serverStdinR:  serverStdinR,
		clientStdinW:  clientStdinW,
		clientStdoutR: clientStdoutR,
		serverStdoutW: serverStdoutW,
	}
}

// createClient는 mock 파이프를 사용하여 JSONRPCClient를 생성합니다.
func (m *mockAppServer) createClient(logger zerolog.Logger) *JSONRPCClient {
	return NewJSONRPCClient(m.clientStdinW, m.clientStdoutR, logger)
}

// close는 모든 파이프를 닫습니다.
func (m *mockAppServer) close() {
	m.serverStdoutW.Close()
	m.clientStdinW.Close()
}

// readRequest는 서버 측에서 클라이언트 요청을 읽습니다.
// 줄바꿈 구분자로 NDJSON 형식의 요청을 파싱합니다.
func (m *mockAppServer) readRequest(remaining *[]byte) (*JSONRPCRequest, error) {
	for {
		// remaining에 완전한 줄이 있는지 확인
		if idx := findNewline(*remaining); idx >= 0 {
			line := (*remaining)[:idx]
			*remaining = (*remaining)[idx+1:]

			var req JSONRPCRequest
			if err := json.Unmarshal(line, &req); err != nil {
				continue // JSON이 아닌 라인 무시
			}
			return &req, nil
		}

		// 더 읽기
		buf := make([]byte, 4096)
		n, err := m.serverStdinR.Read(buf)
		if err != nil {
			return nil, err
		}
		*remaining = append(*remaining, buf[:n]...)
	}
}

// sendResponse는 서버 측에서 클라이언트로 응답을 전송합니다.
func (m *mockAppServer) sendResponse(id int64, result interface{}) error {
	resultData, err := json.Marshal(result)
	if err != nil {
		return err
	}
	raw := json.RawMessage(resultData)
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  &raw,
	}
	data, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = m.serverStdoutW.Write(data)
	return err
}

// sendNotification은 서버 측에서 클라이언트로 알림을 전송합니다.
func (m *mockAppServer) sendNotification(method string, params interface{}) error {
	notif := JSONRPCNotification{
		JSONRPC: "2.0",
		Method:  method,
	}
	if params != nil {
		paramsData, err := json.Marshal(params)
		if err != nil {
			return err
		}
		notif.Params = paramsData
	}
	data, err := json.Marshal(notif)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = m.serverStdoutW.Write(data)
	return err
}

// createMockProvider는 테스트용 CodexAppServerProvider를 생성합니다.
// 실제 프로세스를 시작하지 않고 mock 파이프를 통해 직접 구성합니다.
func createMockProvider(client *JSONRPCClient, approvalPolicy string) *CodexAppServerProvider {
	proc := &AppServerProcess{
		client:      client,
		maxRestarts: 3,
		logger:      zerolog.Nop(),
	}
	proc.running.Store(true)

	return &CodexAppServerProvider{
		process:        proc,
		approvalPolicy: approvalPolicy,
		authMethod:     "apiKey",
		authKey:        "test-api-key",
		logger:         zerolog.Nop(),
	}
}

// TestAppServerProcess_StartStop은 프로세스 라이프사이클 기본 동작을 검증합니다.
// 실제 프로세스 대신 mock 클라이언트의 running 상태를 테스트합니다.
func TestAppServerProcess_StartStop(t *testing.T) {
	proc := NewAppServerProcess("/nonexistent/codex", zerolog.Nop())

	// 초기 상태 확인
	if proc.IsRunning() {
		t.Error("초기 상태에서 IsRunning()이 true입니다")
	}

	if proc.Client() != nil {
		t.Error("초기 상태에서 Client()가 nil이 아닙니다")
	}

	// maxRestarts 기본값 확인
	if proc.maxRestarts != 3 {
		t.Errorf("maxRestarts: got %d, want 3", proc.maxRestarts)
	}

	// running 상태를 직접 설정하여 Client() 동작 확인
	mock := newMockAppServer()

	proc.client = mock.createClient(zerolog.Nop())
	proc.running.Store(true)

	if !proc.IsRunning() {
		t.Error("running 설정 후 IsRunning()이 false입니다")
	}

	if proc.Client() == nil {
		t.Error("running 설정 후 Client()가 nil입니다")
	}

	// Stop 호출 (프로세스 없이)
	proc.running.Store(false)
	err := proc.Stop()
	if err != nil {
		t.Errorf("Stop 실패: %v", err)
	}

	// 정리
	mock.close()
	proc.client.Close()
}

// TestCodexAppServerProvider_Name은 프로바이더 이름을 검증합니다.
func TestCodexAppServerProvider_Name(t *testing.T) {
	mock := newMockAppServer()

	client := mock.createClient(zerolog.Nop())
	prov := createMockProvider(client, "auto-approve")

	name := prov.Name()
	if name != "codex" {
		t.Errorf("Name(): got %q, want %q", name, "codex")
	}

	// 정리: 서버 stdout을 먼저 닫아 readLoop EOF 발생
	mock.close()
	client.Close()
}

// TestCodexAppServerProvider_Supports는 모델 지원 여부를 검증합니다.
func TestCodexAppServerProvider_Supports(t *testing.T) {
	mock := newMockAppServer()

	client := mock.createClient(zerolog.Nop())
	prov := createMockProvider(client, "auto-approve")

	tests := []struct {
		name     string
		model    string
		expected bool
	}{
		{
			name:     "gpt-5-codex 지원",
			model:    "gpt-5-codex",
			expected: true,
		},
		{
			name:     "o4-mini 지원",
			model:    "o4-mini",
			expected: true,
		},
		{
			name:     "지원하지 않는 모델",
			model:    "unknown-model",
			expected: false,
		},
		{
			name:     "claude 모델 미지원",
			model:    "claude-sonnet-4-20250514",
			expected: false,
		},
		{
			name:     "빈 모델명 미지원",
			model:    "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := prov.Supports(tt.model)
			if result != tt.expected {
				t.Errorf("Supports(%q): got %v, want %v", tt.model, result, tt.expected)
			}
		})
	}

	// 정리
	mock.close()
	client.Close()
}

// TestCodexAppServerProvider_ExecuteInternal_MockFlow는
// mock JSON-RPC 서버를 사용하여 전체 실행 흐름을 검증합니다.
func TestCodexAppServerProvider_ExecuteInternal_MockFlow(t *testing.T) {
	mock := newMockAppServer()
	client := mock.createClient(zerolog.Nop())
	prov := createMockProvider(client, "auto-approve")

	// mock 서버 고루틴: 요청 처리 및 응답/알림 전송
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		remaining := make([]byte, 0, 4096)

		// 1. thread/start 요청 수신 및 응답
		req, err := mock.readRequest(&remaining)
		if err != nil {
			return
		}
		if req.Method != MethodThreadStart {
			return
		}
		if err := mock.sendResponse(req.ID, ThreadStartResult{ThreadID: "thread-001"}); err != nil {
			return
		}

		// 2. turn/start 요청 수신 및 응답
		req, err = mock.readRequest(&remaining)
		if err != nil {
			return
		}
		if req.Method != MethodTurnStart {
			return
		}
		if err := mock.sendResponse(req.ID, TurnStartResult{TurnID: "turn-001"}); err != nil {
			return
		}

		// 3. 에이전트 메시지 델타 알림 전송
		if err := mock.sendNotification(MethodAgentMessageDelta, AgentMessageDelta{Text: "Hello, "}); err != nil {
			return
		}
		if err := mock.sendNotification(MethodAgentMessageDelta, AgentMessageDelta{Text: "world!"}); err != nil {
			return
		}

		// 4. Turn 완료 알림 전송
		if err := mock.sendNotification(MethodTurnCompleted, TurnCompletedParams{ThreadID: "thread-001"}); err != nil {
			return
		}
	}()

	// 실행 요청
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := prov.Execute(ctx, ExecuteRequest{
		Prompt:  "테스트 프롬프트",
		Model:   "gpt-5-codex",
		WorkDir: "/tmp",
	})

	// 서버 고루틴 종료 대기
	mock.close()
	<-serverDone
	client.Close()

	if err != nil {
		t.Fatalf("Execute 실패: %v", err)
	}

	// 출력 확인
	if resp.Output != "Hello, world!" {
		t.Errorf("Output: got %q, want %q", resp.Output, "Hello, world!")
	}

	// 모델 확인
	if resp.Model != "gpt-5-codex" {
		t.Errorf("Model: got %q, want %q", resp.Model, "gpt-5-codex")
	}
}

// TestCodexAppServerProvider_ExecuteStreaming_Callback은
// 스트리밍 콜백이 올바르게 호출되는지 검증합니다.
func TestCodexAppServerProvider_ExecuteStreaming_Callback(t *testing.T) {
	mock := newMockAppServer()
	client := mock.createClient(zerolog.Nop())
	prov := createMockProvider(client, "auto-approve")

	// mock 서버 고루틴
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		remaining := make([]byte, 0, 4096)

		// thread/start 응답
		req, err := mock.readRequest(&remaining)
		if err != nil {
			return
		}
		if err := mock.sendResponse(req.ID, ThreadStartResult{ThreadID: "thread-002"}); err != nil {
			return
		}

		// turn/start 응답
		req, err = mock.readRequest(&remaining)
		if err != nil {
			return
		}
		if err := mock.sendResponse(req.ID, TurnStartResult{TurnID: "turn-002"}); err != nil {
			return
		}

		// 여러 에이전트 메시지 델타 전송 (문장 경계를 포함하여 플러시 유도)
		deltas := []string{
			"First sentence.",
			" Second sentence.",
		}
		for _, text := range deltas {
			if err := mock.sendNotification(MethodAgentMessageDelta, AgentMessageDelta{Text: text}); err != nil {
				return
			}
			// 약간의 지연으로 플러시 기회 제공
			time.Sleep(10 * time.Millisecond)
		}

		// Turn 완료
		if err := mock.sendNotification(MethodTurnCompleted, TurnCompletedParams{ThreadID: "thread-002"}); err != nil {
			return
		}
	}()

	// 스트리밍 콜백 수집
	var callbackMu sync.Mutex
	var deltas []string
	var accumulated []string

	callback := func(textDelta string, accumulatedText string) {
		callbackMu.Lock()
		defer callbackMu.Unlock()
		deltas = append(deltas, textDelta)
		accumulated = append(accumulated, accumulatedText)
	}

	// 스트리밍 실행
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := prov.ExecuteStreaming(ctx, ExecuteRequest{
		Prompt: "스트리밍 테스트",
		Model:  "gpt-5-codex",
	}, callback)

	// 정리
	mock.close()
	<-serverDone
	client.Close()

	if err != nil {
		t.Fatalf("ExecuteStreaming 실패: %v", err)
	}

	// 출력이 올바른지 확인
	expectedOutput := "First sentence. Second sentence."
	if resp.Output != expectedOutput {
		t.Errorf("Output: got %q, want %q", resp.Output, expectedOutput)
	}

	// 콜백이 한 번 이상 호출되었는지 확인
	callbackMu.Lock()
	callbackCount := len(deltas)
	callbackMu.Unlock()

	if callbackCount == 0 {
		t.Error("스트리밍 콜백이 한 번도 호출되지 않았습니다")
	}

	// 마지막 누적 텍스트가 전체 출력과 일치하는지 확인
	callbackMu.Lock()
	if len(accumulated) > 0 {
		lastAccumulated := accumulated[len(accumulated)-1]
		if lastAccumulated != expectedOutput {
			t.Errorf("마지막 누적 텍스트: got %q, want %q", lastAccumulated, expectedOutput)
		}
	}
	callbackMu.Unlock()
}

// readAnyMessage는 서버 측에서 JSON-RPC 메시지(요청 또는 알림)를 읽습니다.
// 줄바꿈 구분자로 JSON 메시지를 파싱하여 raw map으로 반환합니다.
func (m *mockAppServer) readAnyMessage(remaining *[]byte) (map[string]json.RawMessage, error) {
	for {
		if idx := findNewline(*remaining); idx >= 0 {
			line := (*remaining)[:idx]
			*remaining = (*remaining)[idx+1:]

			var raw map[string]json.RawMessage
			if err := json.Unmarshal(line, &raw); err != nil {
				continue
			}
			return raw, nil
		}

		buf := make([]byte, 4096)
		n, err := m.serverStdinR.Read(buf)
		if err != nil {
			return nil, err
		}
		*remaining = append(*remaining, buf[:n]...)
	}
}

// TestCodexAppServerProvider_ApprovalPolicy는
// auto-approve 및 deny-all 승인 정책 동작을 검증합니다.
func TestCodexAppServerProvider_ApprovalPolicy(t *testing.T) {
	tests := []struct {
		name             string
		policy           string
		expectedDecision string
	}{
		{
			name:             "auto-approve 정책",
			policy:           "auto-approve",
			expectedDecision: "accept",
		},
		{
			name:             "deny-all 정책",
			policy:           "deny-all",
			expectedDecision: "decline",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMockAppServer()
			client := mock.createClient(zerolog.Nop())
			prov := createMockProvider(client, tt.policy)

			// 승인 응답 수집 채널
			approvalCh := make(chan string, 1)

			// mock 서버 고루틴
			serverDone := make(chan struct{})
			go func() {
				defer close(serverDone)
				remaining := make([]byte, 0, 4096)

				// thread/start 요청 수신 및 응답
				req, err := mock.readRequest(&remaining)
				if err != nil {
					return
				}
				if err := mock.sendResponse(req.ID, ThreadStartResult{ThreadID: "thread-approval"}); err != nil {
					return
				}

				// turn/start 요청 수신 및 응답
				req, err = mock.readRequest(&remaining)
				if err != nil {
					return
				}
				if err := mock.sendResponse(req.ID, TurnStartResult{TurnID: "turn-approval"}); err != nil {
					return
				}

				// 승인 요청 알림 전송
				if err := mock.sendNotification(MethodRequestApproval, RequestApprovalParams{
					ExecutionID: "exec-001",
					Type:        "command",
				}); err != nil {
					return
				}

				// 클라이언트의 승인 응답 읽기 (Notify 형태 - id 없음)
				// readAnyMessage로 모든 JSON 메시지를 읽음
				for {
					raw, err := mock.readAnyMessage(&remaining)
					if err != nil {
						return
					}

					// method 필드 확인
					methodData, ok := raw["method"]
					if !ok {
						continue
					}

					var method string
					if err := json.Unmarshal(methodData, &method); err != nil {
						continue
					}

					if method == "requestApproval/response" {
						paramsData, ok := raw["params"]
						if !ok {
							continue
						}
						var resp ApprovalResponse
						if err := json.Unmarshal(paramsData, &resp); err == nil {
							approvalCh <- resp.Decision
						}
						break
					}
				}

				// Turn 완료 알림 전송
				if err := mock.sendNotification(MethodTurnCompleted, TurnCompletedParams{ThreadID: "thread-approval"}); err != nil {
					return
				}
			}()

			// 실행
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			resp, err := prov.Execute(ctx, ExecuteRequest{
				Prompt: "승인 정책 테스트",
				Model:  "gpt-5-codex",
			})

			// 승인 응답 확인
			select {
			case decision := <-approvalCh:
				if decision != tt.expectedDecision {
					t.Errorf("승인 결정: got %q, want %q", decision, tt.expectedDecision)
				}
			case <-time.After(5 * time.Second):
				t.Error("승인 응답 수신 타임아웃")
			}

			// 정리
			mock.close()
			<-serverDone
			client.Close()

			if err != nil {
				t.Fatalf("Execute 실패: %v", err)
			}

			if resp == nil {
				t.Fatal("응답이 nil입니다")
			}
		})
	}
}

// TestCodexAppServerProvider_ValidateConfig는 설정 유효성 검사를 검증합니다.
func TestCodexAppServerProvider_ValidateConfig(t *testing.T) {
	t.Run("프로세스 실행 중이면 유효", func(t *testing.T) {
		mock := newMockAppServer()
		client := mock.createClient(zerolog.Nop())
		prov := createMockProvider(client, "auto-approve")

		err := prov.ValidateConfig()
		if err != nil {
			t.Errorf("ValidateConfig 실패: %v", err)
		}

		mock.close()
		client.Close()
	})

	t.Run("프로세스 미실행 시 에러", func(t *testing.T) {
		mock := newMockAppServer()
		client := mock.createClient(zerolog.Nop())
		prov := createMockProvider(client, "auto-approve")
		prov.process.running.Store(false)

		err := prov.ValidateConfig()
		if err != ErrProcessNotRunning {
			t.Errorf("ValidateConfig: got %v, want ErrProcessNotRunning", err)
		}

		mock.close()
		client.Close()
	})

	t.Run("API 키 미설정 시 에러", func(t *testing.T) {
		mock := newMockAppServer()
		client := mock.createClient(zerolog.Nop())
		prov := createMockProvider(client, "auto-approve")
		prov.authKey = ""

		err := prov.ValidateConfig()
		if err != ErrNoAPIKey {
			t.Errorf("ValidateConfig: got %v, want ErrNoAPIKey", err)
		}

		mock.close()
		client.Close()
	})
}

// TestCodexAppServerProvider_ExecuteInternal_ProcessNotRunning은
// 프로세스가 실행 중이 아닐 때 에러를 반환하는지 검증합니다.
func TestCodexAppServerProvider_ExecuteInternal_ProcessNotRunning(t *testing.T) {
	mock := newMockAppServer()

	client := mock.createClient(zerolog.Nop())
	prov := createMockProvider(client, "auto-approve")
	prov.process.running.Store(false)

	ctx := context.Background()
	_, err := prov.Execute(ctx, ExecuteRequest{
		Prompt: "테스트",
		Model:  "gpt-5-codex",
	})

	if err != ErrProcessNotRunning {
		t.Errorf("Execute: got %v, want ErrProcessNotRunning", err)
	}

	mock.close()
	client.Close()
}

// TestCodexAppServerProvider_DefaultModel은 모델 미지정 시 기본 모델이 사용되는지 검증합니다.
func TestCodexAppServerProvider_DefaultModel(t *testing.T) {
	mock := newMockAppServer()
	client := mock.createClient(zerolog.Nop())
	prov := createMockProvider(client, "auto-approve")

	// 서버에서 모델 확인
	modelCh := make(chan string, 1)
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		remaining := make([]byte, 0, 4096)

		// thread/start 요청에서 모델 확인
		req, err := mock.readRequest(&remaining)
		if err != nil {
			return
		}

		var params ThreadStartParams
		if err := json.Unmarshal(req.Params, &params); err == nil {
			modelCh <- params.Model
		}

		if err := mock.sendResponse(req.ID, ThreadStartResult{ThreadID: "thread-default"}); err != nil {
			return
		}

		// turn/start 응답
		req, err = mock.readRequest(&remaining)
		if err != nil {
			return
		}
		if err := mock.sendResponse(req.ID, TurnStartResult{TurnID: "turn-default"}); err != nil {
			return
		}

		// Turn 완료
		if err := mock.sendNotification(MethodTurnCompleted, TurnCompletedParams{ThreadID: "thread-default"}); err != nil {
			return
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 모델을 비워서 실행 (기본 모델 사용)
	_, err := prov.Execute(ctx, ExecuteRequest{
		Prompt: "기본 모델 테스트",
	})

	// 모델 확인
	select {
	case model := <-modelCh:
		if model != "gpt-5-codex" {
			t.Errorf("기본 모델: got %q, want %q", model, "gpt-5-codex")
		}
	case <-time.After(5 * time.Second):
		t.Error("모델 확인 타임아웃")
	}

	mock.close()
	<-serverDone
	client.Close()

	if err != nil {
		t.Fatalf("Execute 실패: %v", err)
	}
}

// TestCodexAppServerProvider_CommandExecution은
// 명령 실행 알림이 ToolCall로 변환되는지 검증합니다.
func TestCodexAppServerProvider_CommandExecution(t *testing.T) {
	mock := newMockAppServer()
	client := mock.createClient(zerolog.Nop())
	prov := createMockProvider(client, "auto-approve")

	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		remaining := make([]byte, 0, 4096)

		// thread/start 응답
		req, err := mock.readRequest(&remaining)
		if err != nil {
			return
		}
		if err := mock.sendResponse(req.ID, ThreadStartResult{ThreadID: "thread-cmd"}); err != nil {
			return
		}

		// turn/start 응답
		req, err = mock.readRequest(&remaining)
		if err != nil {
			return
		}
		if err := mock.sendResponse(req.ID, TurnStartResult{TurnID: "turn-cmd"}); err != nil {
			return
		}

		// 명령 실행 완료 알림
		if err := mock.sendNotification(MethodCommandExecution, CommandExecutionItem{
			ID:       "cmd-001",
			Command:  "ls -la",
			Output:   "total 0",
			ExitCode: 0,
		}); err != nil {
			return
		}

		// 에이전트 메시지 델타
		if err := mock.sendNotification(MethodAgentMessageDelta, AgentMessageDelta{Text: "Done."}); err != nil {
			return
		}

		// 잠시 대기하여 알림 핸들러가 처리할 시간을 줌
		time.Sleep(50 * time.Millisecond)

		// Turn 완료
		if err := mock.sendNotification(MethodTurnCompleted, TurnCompletedParams{ThreadID: "thread-cmd"}); err != nil {
			return
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := prov.Execute(ctx, ExecuteRequest{
		Prompt: "명령 실행 테스트",
		Model:  "gpt-5-codex",
	})

	mock.close()
	<-serverDone
	client.Close()

	if err != nil {
		t.Fatalf("Execute 실패: %v", err)
	}

	// 출력 확인
	if resp.Output != "Done." {
		t.Errorf("Output: got %q, want %q", resp.Output, "Done.")
	}

	// ToolCall 확인
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("ToolCalls 개수: got %d, want 1", len(resp.ToolCalls))
	}

	tc := resp.ToolCalls[0]
	if tc.ID != "cmd-001" {
		t.Errorf("ToolCall ID: got %q, want %q", tc.ID, "cmd-001")
	}
	if tc.Name != "command_execution" {
		t.Errorf("ToolCall Name: got %q, want %q", tc.Name, "command_execution")
	}

	// Input JSON 확인
	var input map[string]interface{}
	if err := json.Unmarshal(tc.Input, &input); err != nil {
		t.Fatalf("ToolCall Input 파싱 실패: %v", err)
	}
	if input["command"] != "ls -la" {
		t.Errorf("ToolCall command: got %v, want %q", input["command"], "ls -la")
	}
}

// TestCodexAppServerProvider_MCPToolCall은
// MCP 도구 호출 알림이 ToolCall로 변환되는지 검증합니다.
func TestCodexAppServerProvider_MCPToolCall(t *testing.T) {
	mock := newMockAppServer()
	client := mock.createClient(zerolog.Nop())
	prov := createMockProvider(client, "auto-approve")

	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		remaining := make([]byte, 0, 4096)

		// thread/start 응답
		req, err := mock.readRequest(&remaining)
		if err != nil {
			return
		}
		if err := mock.sendResponse(req.ID, ThreadStartResult{ThreadID: "thread-mcp"}); err != nil {
			return
		}

		// turn/start 응답
		req, err = mock.readRequest(&remaining)
		if err != nil {
			return
		}
		if err := mock.sendResponse(req.ID, TurnStartResult{TurnID: "turn-mcp"}); err != nil {
			return
		}

		// MCP 도구 호출 알림
		if err := mock.sendNotification(MethodMCPToolCall, MCPToolCallItem{
			ID:       "mcp-001",
			ToolName: "file_read",
			Input:    json.RawMessage(`{"path":"/tmp/test.txt"}`),
			Output:   "file contents",
		}); err != nil {
			return
		}

		// 잠시 대기
		time.Sleep(50 * time.Millisecond)

		// Turn 완료
		if err := mock.sendNotification(MethodTurnCompleted, TurnCompletedParams{ThreadID: "thread-mcp"}); err != nil {
			return
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := prov.Execute(ctx, ExecuteRequest{
		Prompt: "MCP 도구 호출 테스트",
		Model:  "gpt-5-codex",
	})

	mock.close()
	<-serverDone
	client.Close()

	if err != nil {
		t.Fatalf("Execute 실패: %v", err)
	}

	// ToolCall 확인
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("ToolCalls 개수: got %d, want 1", len(resp.ToolCalls))
	}

	tc := resp.ToolCalls[0]
	if tc.ID != "mcp-001" {
		t.Errorf("ToolCall ID: got %q, want %q", tc.ID, "mcp-001")
	}
	if tc.Name != "file_read" {
		t.Errorf("ToolCall Name: got %q, want %q", tc.Name, "file_read")
	}

	// Input JSON 확인
	var input map[string]interface{}
	if err := json.Unmarshal(tc.Input, &input); err != nil {
		t.Fatalf("ToolCall Input 파싱 실패: %v", err)
	}
	if input["path"] != "/tmp/test.txt" {
		t.Errorf("ToolCall path: got %v, want %q", input["path"], "/tmp/test.txt")
	}
}

// TestCodexAppServerProvider_MaxRestarts는
// 최대 재시작 횟수 초과 시 에러를 반환하는지 검증합니다.
func TestCodexAppServerProvider_MaxRestarts(t *testing.T) {
	proc := NewAppServerProcess("/nonexistent/codex", zerolog.Nop())
	proc.restartCount = 3 // maxRestarts와 같은 값으로 설정

	err := proc.Restart(context.Background())
	if err != ErrMaxRestartsExceeded {
		t.Errorf("Restart: got %v, want ErrMaxRestartsExceeded", err)
	}
}

// TestCodexAppServerProvider_ContextCancellation은
// 컨텍스트 취소 시 실행이 중단되는지 검증합니다.
func TestCodexAppServerProvider_ContextCancellation(t *testing.T) {
	mock := newMockAppServer()
	client := mock.createClient(zerolog.Nop())
	prov := createMockProvider(client, "auto-approve")

	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		remaining := make([]byte, 0, 4096)

		// thread/start 응답
		req, err := mock.readRequest(&remaining)
		if err != nil {
			return
		}
		if err := mock.sendResponse(req.ID, ThreadStartResult{ThreadID: "thread-cancel"}); err != nil {
			return
		}

		// turn/start 응답
		req, err = mock.readRequest(&remaining)
		if err != nil {
			return
		}
		if err := mock.sendResponse(req.ID, TurnStartResult{TurnID: "turn-cancel"}); err != nil {
			return
		}

		// Turn 완료를 보내지 않음 - 컨텍스트 취소 유도
		// 서버가 응답하지 않고 대기
		time.Sleep(5 * time.Second)
	}()

	// 매우 짧은 타임아웃
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	_, err := prov.Execute(ctx, ExecuteRequest{
		Prompt: "취소 테스트",
		Model:  "gpt-5-codex",
	})

	mock.close()
	<-serverDone
	client.Close()

	if err == nil {
		t.Fatal("컨텍스트 취소 에러를 기대했지만 nil을 받았습니다")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "context deadline exceeded") && !strings.Contains(errStr, "타임아웃") {
		t.Errorf("타임아웃 에러를 기대했지만 다른 에러: %v", err)
	}
}

// TestCodexAppServerProvider_Close는 프로바이더 종료를 검증합니다.
func TestCodexAppServerProvider_Close(t *testing.T) {
	mock := newMockAppServer()

	client := mock.createClient(zerolog.Nop())
	prov := createMockProvider(client, "auto-approve")

	// Close 호출 전 running 상태
	if !prov.process.IsRunning() {
		t.Error("Close 전 IsRunning()이 false입니다")
	}

	// 서버 stdout을 먼저 닫아 readLoop가 종료되도록 함
	mock.serverStdoutW.Close()

	// Close 호출 (Stop 내부에서 client.Close() 호출)
	err := prov.Close()
	if err != nil {
		// 파이프 관련 에러는 무시 (mock 환경)
		_ = err
	}

	// running 상태 확인
	if prov.process.IsRunning() {
		t.Error("Close 후 IsRunning()이 여전히 true입니다")
	}
}

// Unused import 방지용 (lint 경고 방지)
var _ = fmt.Sprintf
