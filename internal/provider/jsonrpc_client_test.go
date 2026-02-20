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

	"github.com/insajin/autopus-codex-rpc/client"
	"github.com/insajin/autopus-codex-rpc/protocol"
)

// newTestJSONRPCClient는 테스트용 JSON-RPC 클라이언트를 생성합니다.
func newTestJSONRPCClient(stdin io.WriteCloser, stdout io.Reader) *client.Client {
	return client.NewJSONRPCClient(stdin, stdout, client.NopLogger())
}

// TestJSONRPCClient_CallResponse는 기본 요청/응답 흐름을 검증합니다.
func TestJSONRPCClient_CallResponse(t *testing.T) {
	// mock stdin/stdout 파이프 생성
	clientStdinR, clientStdinW := io.Pipe()
	serverStdoutR, serverStdoutW := io.Pipe()

	c := newTestJSONRPCClient(clientStdinW, serverStdoutR)
	defer func() {
		serverStdoutW.Close()
		c.Close()
	}()

	// mock 서버: 클라이언트가 보낸 요청을 읽고 응답 전송
	go func() {
		buf := make([]byte, 4096)
		n, err := clientStdinR.Read(buf)
		if err != nil {
			return
		}

		// 요청 파싱
		var req protocol.JSONRPCRequest
		if err := json.Unmarshal(buf[:n], &req); err != nil {
			return
		}

		// 응답 전송 (ID는 *int64)
		result := json.RawMessage(`{"threadId":"thread-test-001"}`)
		resp := protocol.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      &req.ID,
			Result:  &result,
		}
		data, _ := json.Marshal(resp)
		data = append(data, '\n')
		_, _ = serverStdoutW.Write(data)
	}()

	// Call 실행
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := c.Call(ctx, "thread/start", protocol.ThreadStartParams{
		Model: "gpt-5-codex",
		Cwd:   "/tmp",
	})

	if err != nil {
		t.Fatalf("Call 실패: %v", err)
	}
	if result == nil {
		t.Fatal("result가 nil입니다")
	}

	// 결과 파싱 검증
	var threadResult protocol.ThreadStartResult
	if err := json.Unmarshal(*result, &threadResult); err != nil {
		t.Fatalf("result 파싱 실패: %v", err)
	}
	if threadResult.ThreadID != "thread-test-001" {
		t.Errorf("ThreadID: got %q, want %q", threadResult.ThreadID, "thread-test-001")
	}
}

// TestJSONRPCClient_Notification은 알림 디스패치를 검증합니다.
func TestJSONRPCClient_Notification(t *testing.T) {
	_, clientStdinW := io.Pipe()
	serverStdoutR, serverStdoutW := io.Pipe()

	c := newTestJSONRPCClient(clientStdinW, serverStdoutR)
	defer func() {
		serverStdoutW.Close()
		c.Close()
	}()

	// 알림 수신 채널
	received := make(chan string, 1)
	c.OnNotification(protocol.MethodAgentMessageDelta, func(method string, params json.RawMessage) {
		var delta protocol.AgentMessageDelta
		if err := json.Unmarshal(params, &delta); err == nil {
			received <- delta.Delta
		}
	})

	// 서버에서 알림 전송
	go func() {
		notif := protocol.JSONRPCNotification{
			JSONRPC: "2.0",
			Method:  protocol.MethodAgentMessageDelta,
			Params:  json.RawMessage(`{"delta":"안녕하세요, 테스트입니다."}`),
		}
		data, _ := json.Marshal(notif)
		data = append(data, '\n')
		_, _ = serverStdoutW.Write(data)
	}()

	// 알림 수신 확인
	select {
	case text := <-received:
		if text != "안녕하세요, 테스트입니다." {
			t.Errorf("알림 텍스트: got %q, want %q", text, "안녕하세요, 테스트입니다.")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("알림 수신 타임아웃")
	}
}

// TestJSONRPCClient_ConcurrentCalls는 동시 호출 안전성을 검증합니다.
func TestJSONRPCClient_ConcurrentCalls(t *testing.T) {
	clientStdinR, clientStdinW := io.Pipe()
	serverStdoutR, serverStdoutW := io.Pipe()

	c := newTestJSONRPCClient(clientStdinW, serverStdoutR)
	defer func() {
		serverStdoutW.Close()
		c.Close()
	}()

	const numCalls = 5

	// mock 서버: 요청을 읽고 각각에 응답
	go func() {
		remaining := make([]byte, 0, 65536)

		for i := 0; i < numCalls; i++ {
			for {
				if idx := findNewline(remaining); idx >= 0 {
					line := remaining[:idx]
					remaining = remaining[idx+1:]

					var req protocol.JSONRPCRequest
					if err := json.Unmarshal(line, &req); err != nil {
						continue
					}

					// 요청 ID를 포함한 결과로 응답
					resultStr := fmt.Sprintf(`{"value":%d}`, req.ID)
					result := json.RawMessage(resultStr)
					resp := protocol.JSONRPCResponse{
						JSONRPC: "2.0",
						ID:      &req.ID,
						Result:  &result,
					}
					data, _ := json.Marshal(resp)
					data = append(data, '\n')
					_, _ = serverStdoutW.Write(data)
					break
				}

				buf := make([]byte, 4096)
				n, err := clientStdinR.Read(buf)
				if err != nil {
					return
				}
				remaining = append(remaining, buf[:n]...)
			}
		}
	}()

	// 동시에 여러 Call 실행
	var wg sync.WaitGroup
	errs := make(chan error, numCalls)

	for i := 0; i < numCalls; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			result, err := c.Call(ctx, "test/method", map[string]string{"key": "value"})
			if err != nil {
				errs <- fmt.Errorf("Call 실패: %w", err)
				return
			}
			if result == nil {
				errs <- fmt.Errorf("result가 nil입니다")
				return
			}
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("동시 호출 에러: %v", err)
	}
}

// TestJSONRPCClient_Timeout은 컨텍스트 타임아웃 처리를 검증합니다.
func TestJSONRPCClient_Timeout(t *testing.T) {
	clientStdinR, clientStdinW := io.Pipe()
	serverStdoutR, serverStdoutW := io.Pipe()

	c := newTestJSONRPCClient(clientStdinW, serverStdoutR)
	defer func() {
		serverStdoutW.Close()
		c.Close()
	}()

	// stdin 파이프 드레인 (쓰기가 블록되지 않도록)
	go func() {
		buf := make([]byte, 4096)
		for {
			_, err := clientStdinR.Read(buf)
			if err != nil {
				return
			}
			// 응답을 보내지 않음 - 타임아웃 유발
		}
	}()

	// 매우 짧은 타임아웃 설정 (응답을 보내지 않음)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := c.Call(ctx, "slow/method", nil)

	if err == nil {
		t.Fatal("타임아웃 에러를 기대했지만 nil을 받았습니다")
	}

	// 에러 메시지에 타임아웃 관련 내용이 포함되어야 함
	errStr := err.Error()
	if !strings.Contains(errStr, "context deadline exceeded") &&
		!strings.Contains(errStr, "타임아웃") {
		t.Errorf("타임아웃 에러를 기대했지만 다른 에러를 받았습니다: %v", err)
	}
}

// TestJSONRPCClient_ConnectionClosed는 Close() 후 Call이 에러를 반환하는지 검증합니다.
func TestJSONRPCClient_ConnectionClosed(t *testing.T) {
	_, clientStdinW := io.Pipe()
	serverStdoutR, serverStdoutW := io.Pipe()

	c := newTestJSONRPCClient(clientStdinW, serverStdoutR)

	// 서버 stdout을 즉시 닫음 (readLoop에 EOF 시그널)
	serverStdoutW.Close()

	// Close()를 호출하여 클라이언트를 명시적으로 종료
	_ = c.Close()

	// 종료된 후 Call 시도 - 에러를 기대함 (컨텍스트 타임아웃으로 종료)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	_, err := c.Call(ctx, "test/method", nil)
	if err == nil {
		t.Fatal("연결 종료 에러를 기대했지만 nil을 받았습니다")
	}

	// 에러 메시지에 종료 관련 내용이 포함되어야 함
	t.Logf("연결 종료 에러 (예상됨): %v", err)
}

// TestJSONRPCClient_ErrorResponse는 에러 응답 처리를 검증합니다.
func TestJSONRPCClient_ErrorResponse(t *testing.T) {
	clientStdinR, clientStdinW := io.Pipe()
	serverStdoutR, serverStdoutW := io.Pipe()

	c := newTestJSONRPCClient(clientStdinW, serverStdoutR)
	defer func() {
		serverStdoutW.Close()
		c.Close()
	}()

	// mock 서버: 에러 응답 전송
	go func() {
		buf := make([]byte, 4096)
		n, err := clientStdinR.Read(buf)
		if err != nil {
			return
		}

		var req protocol.JSONRPCRequest
		if err := json.Unmarshal(buf[:n], &req); err != nil {
			return
		}

		// 에러 응답 전송
		resp := protocol.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      &req.ID,
			Error: &protocol.JSONRPCError{
				Code:    protocol.ErrCodeMethodNotFound,
				Message: "method not found: unknown/method",
			},
		}
		data, _ := json.Marshal(resp)
		data = append(data, '\n')
		_, _ = serverStdoutW.Write(data)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := c.Call(ctx, "unknown/method", nil)
	if err == nil {
		t.Fatal("에러를 기대했지만 nil을 받았습니다")
	}

	// 에러 메시지가 포함되어야 함
	errStr := err.Error()
	if !strings.Contains(errStr, "method not found") && !strings.Contains(errStr, "not found") {
		t.Errorf("예상된 에러 메시지가 포함되어 있지 않습니다: %v", err)
	}
}

// TestJSONRPCClient_NotifySend는 Notify 메서드 전송을 검증합니다.
func TestJSONRPCClient_NotifySend(t *testing.T) {
	clientStdinR, clientStdinW := io.Pipe()
	serverStdoutR, serverStdoutW := io.Pipe()

	c := newTestJSONRPCClient(clientStdinW, serverStdoutR)
	defer func() {
		serverStdoutW.Close()
		c.Close()
	}()

	// 전송된 데이터를 읽을 채널
	readCh := make(chan []byte, 1)
	go func() {
		buf := make([]byte, 4096)
		n, err := clientStdinR.Read(buf)
		if err != nil {
			return
		}
		readCh <- append([]byte(nil), buf[:n]...)
	}()

	// 알림 전송
	err := c.Notify(protocol.MethodInitialized, nil)
	if err != nil {
		t.Fatalf("Notify 실패: %v", err)
	}

	// 전송된 데이터 확인
	select {
	case data := <-readCh:
		lineEnd := findNewline(data)
		if lineEnd < 0 {
			lineEnd = len(data)
		}

		var notif protocol.JSONRPCNotification
		if err := json.Unmarshal(data[:lineEnd], &notif); err != nil {
			t.Fatalf("전송 데이터 파싱 실패: %v", err)
		}

		if notif.JSONRPC != "2.0" {
			t.Errorf("JSONRPC: got %q, want %q", notif.JSONRPC, "2.0")
		}
		if notif.Method != protocol.MethodInitialized {
			t.Errorf("Method: got %q, want %q", notif.Method, protocol.MethodInitialized)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("전송 데이터 읽기 타임아웃")
	}
}

// findNewline은 바이트 슬라이스에서 줄바꿈 문자의 인덱스를 찾습니다.
func findNewline(b []byte) int {
	for i, c := range b {
		if c == '\n' {
			return i
		}
	}
	return -1
}

// TestJSONRPCClient_MultipleNotifications는 여러 알림 핸들러를 검증합니다.
func TestJSONRPCClient_MultipleNotifications(t *testing.T) {
	_, clientStdinW := io.Pipe()
	serverStdoutR, serverStdoutW := io.Pipe()

	c := newTestJSONRPCClient(clientStdinW, serverStdoutR)
	defer func() {
		serverStdoutW.Close()
		c.Close()
	}()

	// 두 가지 알림 핸들러 등록
	messageCh := make(chan string, 1)
	turnCh := make(chan string, 1)

	c.OnNotification(protocol.MethodAgentMessageDelta, func(method string, params json.RawMessage) {
		var delta protocol.AgentMessageDelta
		if err := json.Unmarshal(params, &delta); err == nil {
			messageCh <- delta.Delta
		}
	})

	c.OnNotification(protocol.MethodTurnCompleted, func(method string, params json.RawMessage) {
		var completed protocol.TurnCompletedParams
		if err := json.Unmarshal(params, &completed); err == nil {
			turnCh <- completed.ThreadID
		}
	})

	// 서버에서 두 가지 알림 전송
	go func() {
		// 메시지 델타 알림
		notif1 := protocol.JSONRPCNotification{
			JSONRPC: "2.0",
			Method:  protocol.MethodAgentMessageDelta,
			Params:  json.RawMessage(`{"delta":"hello"}`),
		}
		data, _ := json.Marshal(notif1)
		data = append(data, '\n')
		_, _ = serverStdoutW.Write(data)

		// Turn 완료 알림
		notif2 := protocol.JSONRPCNotification{
			JSONRPC: "2.0",
			Method:  protocol.MethodTurnCompleted,
			Params:  json.RawMessage(`{"threadId":"t-123"}`),
		}
		data, _ = json.Marshal(notif2)
		data = append(data, '\n')
		_, _ = serverStdoutW.Write(data)
	}()

	// 두 알림 모두 수신 확인
	timeout := time.After(5 * time.Second)

	select {
	case text := <-messageCh:
		if text != "hello" {
			t.Errorf("메시지 텍스트: got %q, want %q", text, "hello")
		}
	case <-timeout:
		t.Fatal("메시지 알림 수신 타임아웃")
	}

	select {
	case threadID := <-turnCh:
		if threadID != "t-123" {
			t.Errorf("ThreadID: got %q, want %q", threadID, "t-123")
		}
	case <-timeout:
		t.Fatal("Turn 완료 알림 수신 타임아웃")
	}
}
