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

// newTestLogger는 테스트용 로거를 생성합니다.
func newTestLogger() zerolog.Logger {
	return zerolog.Nop()
}

// TestJSONRPCClient_CallResponse는 기본 요청/응답 흐름을 검증합니다.
func TestJSONRPCClient_CallResponse(t *testing.T) {
	// mock stdin/stdout 파이프 생성
	clientStdinR, clientStdinW := io.Pipe()
	serverStdoutR, serverStdoutW := io.Pipe()

	logger := newTestLogger()
	client := NewJSONRPCClient(clientStdinW, serverStdoutR, logger)
	defer func() {
		serverStdoutW.Close()
		client.Close()
	}()

	// mock 서버: 클라이언트가 보낸 요청을 읽고 응답 전송
	go func() {
		// 클라이언트가 보낸 요청 읽기
		buf := make([]byte, 4096)
		n, err := clientStdinR.Read(buf)
		if err != nil {
			return
		}

		// 요청 파싱
		var req JSONRPCRequest
		if err := json.Unmarshal(buf[:n], &req); err != nil {
			return
		}

		// 응답 전송
		result := json.RawMessage(`{"threadId":"thread-test-001"}`)
		resp := JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  &result,
		}
		data, _ := json.Marshal(resp)
		data = append(data, '\n')
		_, _ = serverStdoutW.Write(data)
	}()

	// Call 실행
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := client.Call(ctx, "thread/start", ThreadStartParams{
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
	var threadResult ThreadStartResult
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

	logger := newTestLogger()
	client := NewJSONRPCClient(clientStdinW, serverStdoutR, logger)
	defer func() {
		serverStdoutW.Close()
		client.Close()
	}()

	// 알림 수신 채널
	received := make(chan string, 1)
	client.OnNotification(MethodAgentMessageDelta, func(method string, params json.RawMessage) {
		var delta AgentMessageDelta
		if err := json.Unmarshal(params, &delta); err == nil {
			received <- delta.Text
		}
	})

	// 서버에서 알림 전송
	go func() {
		notif := JSONRPCNotification{
			JSONRPC: "2.0",
			Method:  MethodAgentMessageDelta,
			Params:  json.RawMessage(`{"text":"안녕하세요, 테스트입니다."}`),
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

	logger := newTestLogger()
	client := NewJSONRPCClient(clientStdinW, serverStdoutR, logger)
	defer func() {
		serverStdoutW.Close()
		client.Close()
	}()

	const numCalls = 5

	// mock 서버: 요청을 읽고 각각에 응답
	go func() {
		remaining := make([]byte, 0, 65536)

		for i := 0; i < numCalls; i++ {
			// 요청 한 줄 읽기
			for {
				// remaining에 완전한 줄이 있는지 확인
				if idx := findNewline(remaining); idx >= 0 {
					line := remaining[:idx]
					remaining = remaining[idx+1:]

					var req JSONRPCRequest
					if err := json.Unmarshal(line, &req); err != nil {
						continue
					}

					// 요청 ID를 포함한 결과로 응답
					resultStr := fmt.Sprintf(`{"value":%d}`, req.ID)
					result := json.RawMessage(resultStr)
					resp := JSONRPCResponse{
						JSONRPC: "2.0",
						ID:      req.ID,
						Result:  &result,
					}
					data, _ := json.Marshal(resp)
					data = append(data, '\n')
					_, _ = serverStdoutW.Write(data)
					break
				}

				// 더 읽기
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

			result, err := client.Call(ctx, "test/method", map[string]string{"key": "value"})
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

	logger := newTestLogger()
	client := NewJSONRPCClient(clientStdinW, serverStdoutR, logger)
	defer func() {
		serverStdoutW.Close()
		client.Close()
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

	_, err := client.Call(ctx, "slow/method", nil)

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

// TestJSONRPCClient_ConnectionClosed는 EOF 처리를 검증합니다.
func TestJSONRPCClient_ConnectionClosed(t *testing.T) {
	_, clientStdinW := io.Pipe()
	serverStdoutR, serverStdoutW := io.Pipe()

	logger := newTestLogger()
	client := NewJSONRPCClient(clientStdinW, serverStdoutR, logger)

	// 서버 stdout을 즉시 닫음 (EOF)
	serverStdoutW.Close()

	// readLoop이 done 채널을 닫을 때까지 대기
	select {
	case <-client.done:
		// 정상: readLoop이 종료됨
	case <-time.After(5 * time.Second):
		t.Fatal("readLoop 종료 대기 타임아웃")
	}

	// 종료된 후 Call 시도
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	_, err := client.Call(ctx, "test/method", nil)
	if err == nil {
		t.Fatal("연결 종료 에러를 기대했지만 nil을 받았습니다")
	}

	// ErrConnectionClosed 확인
	if err != ErrConnectionClosed {
		errStr := err.Error()
		if !strings.Contains(errStr, "종료") && !strings.Contains(errStr, "closed") {
			t.Errorf("연결 종료 에러를 기대했지만 다른 에러를 받았습니다: %v", err)
		}
	}

	client.Close()
}

// TestJSONRPCClient_ErrorResponse는 에러 응답 처리를 검증합니다.
func TestJSONRPCClient_ErrorResponse(t *testing.T) {
	clientStdinR, clientStdinW := io.Pipe()
	serverStdoutR, serverStdoutW := io.Pipe()

	logger := newTestLogger()
	client := NewJSONRPCClient(clientStdinW, serverStdoutR, logger)
	defer func() {
		serverStdoutW.Close()
		client.Close()
	}()

	// mock 서버: 에러 응답 전송
	go func() {
		buf := make([]byte, 4096)
		n, err := clientStdinR.Read(buf)
		if err != nil {
			return
		}

		var req JSONRPCRequest
		if err := json.Unmarshal(buf[:n], &req); err != nil {
			return
		}

		// 에러 응답 전송
		resp := JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &JSONRPCError{
				Code:    ErrCodeMethodNotFound,
				Message: "method not found: unknown/method",
			},
		}
		data, _ := json.Marshal(resp)
		data = append(data, '\n')
		_, _ = serverStdoutW.Write(data)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := client.Call(ctx, "unknown/method", nil)
	if err == nil {
		t.Fatal("에러를 기대했지만 nil을 받았습니다")
	}

	// JSONRPCError가 반환되어야 함 (MapJSONRPCError의 default 케이스)
	var rpcErr *JSONRPCError
	if ok := isJSONRPCError(err, &rpcErr); ok {
		if rpcErr.Code != ErrCodeMethodNotFound {
			t.Errorf("에러 코드: got %d, want %d", rpcErr.Code, ErrCodeMethodNotFound)
		}
	}
}

// TestJSONRPCClient_NotifySend는 Notify 메서드 전송을 검증합니다.
func TestJSONRPCClient_NotifySend(t *testing.T) {
	clientStdinR, clientStdinW := io.Pipe()
	serverStdoutR, serverStdoutW := io.Pipe()

	logger := newTestLogger()
	client := NewJSONRPCClient(clientStdinW, serverStdoutR, logger)
	defer func() {
		serverStdoutW.Close()
		client.Close()
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
	err := client.Notify(MethodInitialized, nil)
	if err != nil {
		t.Fatalf("Notify 실패: %v", err)
	}

	// 전송된 데이터 확인
	select {
	case data := <-readCh:
		// 줄바꿈 문자 위치 찾기
		lineEnd := findNewline(data)
		if lineEnd < 0 {
			lineEnd = len(data)
		}

		var notif JSONRPCNotification
		if err := json.Unmarshal(data[:lineEnd], &notif); err != nil {
			t.Fatalf("전송 데이터 파싱 실패: %v", err)
		}

		if notif.JSONRPC != "2.0" {
			t.Errorf("JSONRPC: got %q, want %q", notif.JSONRPC, "2.0")
		}
		if notif.Method != MethodInitialized {
			t.Errorf("Method: got %q, want %q", notif.Method, MethodInitialized)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("전송 데이터 읽기 타임아웃")
	}
}

// TestJSONRPCClient_MultipleNotifications는 여러 알림 핸들러를 검증합니다.
func TestJSONRPCClient_MultipleNotifications(t *testing.T) {
	_, clientStdinW := io.Pipe()
	serverStdoutR, serverStdoutW := io.Pipe()

	logger := newTestLogger()
	client := NewJSONRPCClient(clientStdinW, serverStdoutR, logger)
	defer func() {
		serverStdoutW.Close()
		client.Close()
	}()

	// 두 가지 알림 핸들러 등록
	messageCh := make(chan string, 1)
	turnCh := make(chan string, 1)

	client.OnNotification(MethodAgentMessageDelta, func(method string, params json.RawMessage) {
		var delta AgentMessageDelta
		if err := json.Unmarshal(params, &delta); err == nil {
			messageCh <- delta.Text
		}
	})

	client.OnNotification(MethodTurnCompleted, func(method string, params json.RawMessage) {
		var completed TurnCompletedParams
		if err := json.Unmarshal(params, &completed); err == nil {
			turnCh <- completed.ThreadID
		}
	})

	// 서버에서 두 가지 알림 전송
	go func() {
		// 메시지 델타 알림
		notif1 := JSONRPCNotification{
			JSONRPC: "2.0",
			Method:  MethodAgentMessageDelta,
			Params:  json.RawMessage(`{"text":"hello"}`),
		}
		data, _ := json.Marshal(notif1)
		data = append(data, '\n')
		_, _ = serverStdoutW.Write(data)

		// Turn 완료 알림
		notif2 := JSONRPCNotification{
			JSONRPC: "2.0",
			Method:  MethodTurnCompleted,
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

// findNewline은 바이트 슬라이스에서 줄바꿈 문자의 인덱스를 찾습니다.
func findNewline(b []byte) int {
	for i, c := range b {
		if c == '\n' {
			return i
		}
	}
	return -1
}

// isJSONRPCError는 에러가 JSONRPCError인지 확인합니다.
func isJSONRPCError(err error, target **JSONRPCError) bool {
	if rpcErr, ok := err.(*JSONRPCError); ok {
		*target = rpcErr
		return true
	}
	return false
}
