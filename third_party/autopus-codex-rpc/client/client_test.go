package client_test

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

// mockPipe는 테스트용 io.ReadWriter 구현체이다.
type mockPipe struct {
	mu     sync.Mutex
	buf    strings.Builder
	ch     chan string
	closed bool
}

func newMockPipe() *mockPipe {
	return &mockPipe{ch: make(chan string, 100)}
}

// Write는 수신된 데이터를 ch에 전송한다 (클라이언트가 보내는 요청 캡처용).
func (p *mockPipe) Write(data []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return 0, io.ErrClosedPipe
	}
	p.ch <- string(data)
	return len(data), nil
}

// Close는 파이프를 닫는다.
func (p *mockPipe) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closed = true
	close(p.ch)
	return nil
}

// serverSide는 서버 측 응답을 시뮬레이션하는 io.Reader이다.
type serverSide struct {
	mu     sync.Mutex
	buf    []byte
	cond   *sync.Cond
	closed bool
}

func newServerSide() *serverSide {
	s := &serverSide{}
	s.cond = sync.NewCond(&s.mu)
	return s
}

// Send는 서버에서 클라이언트로 메시지를 전송한다.
func (s *serverSide) Send(data []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.buf = append(s.buf, data...)
	s.buf = append(s.buf, '\n')
	s.cond.Signal()
}

// Read는 버퍼에서 데이터를 읽는다.
func (s *serverSide) Read(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for len(s.buf) == 0 && !s.closed {
		s.cond.Wait()
	}

	if len(s.buf) == 0 && s.closed {
		return 0, io.EOF
	}

	n := copy(p, s.buf)
	s.buf = s.buf[n:]
	return n, nil
}

// Close는 서버 측을 닫는다.
func (s *serverSide) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	s.cond.Broadcast()
}

// TestNewJSONRPCClient는 클라이언트 생성을 검증한다.
func TestNewJSONRPCClient(t *testing.T) {
	stdin := newMockPipe()
	stdout := newServerSide()

	c := client.NewJSONRPCClient(stdin, stdout, client.NopLogger())
	if c == nil {
		t.Fatal("클라이언트가 nil이면 안 됨")
	}
	// stdout을 먼저 닫아 readLoop이 EOF를 받아 종료되도록 함
	stdout.Close()
	_ = c.Close()
}

// TestCall_Success는 성공적인 RPC 호출을 검증한다.
func TestCall_Success(t *testing.T) {
	stdin := newMockPipe()
	stdout := newServerSide()

	c := client.NewJSONRPCClient(stdin, stdout, client.NopLogger())
	defer c.Close()

	// 서버 시뮬레이션: 요청을 받아 응답 전송
	go func() {
		// 클라이언트 요청 읽기
		reqStr, ok := <-stdin.ch
		if !ok {
			return
		}

		var req protocol.JSONRPCRequest
		if err := json.Unmarshal([]byte(strings.TrimSpace(reqStr)), &req); err != nil {
			return
		}

		// 응답 전송
		result := json.RawMessage(`{"threadId":"test-thread-001"}`)
		resp := protocol.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      &req.ID,
			Result:  &result,
		}
		respData, _ := json.Marshal(resp)
		stdout.Send(respData)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	params := protocol.ThreadStartParams{
		Model: "o4-mini",
		Cwd:   "/workspace",
	}
	result, err := c.Call(ctx, protocol.MethodThreadStart, params)
	if err != nil {
		t.Fatalf("Call 실패: %v", err)
	}
	if result == nil {
		t.Fatal("결과가 nil이면 안 됨")
	}

	var threadResult protocol.ThreadStartResult
	if err := json.Unmarshal(*result, &threadResult); err != nil {
		t.Fatalf("결과 역직렬화 실패: %v", err)
	}
	if threadResult.ThreadID != "test-thread-001" {
		t.Errorf("ThreadID 불일치: got %q, want %q", threadResult.ThreadID, "test-thread-001")
	}
}

// TestCall_Error는 서버 에러 응답 처리를 검증한다.
func TestCall_Error(t *testing.T) {
	stdin := newMockPipe()
	stdout := newServerSide()

	c := client.NewJSONRPCClient(stdin, stdout, client.NopLogger())
	defer c.Close()

	go func() {
		reqStr, ok := <-stdin.ch
		if !ok {
			return
		}

		var req protocol.JSONRPCRequest
		if err := json.Unmarshal([]byte(strings.TrimSpace(reqStr)), &req); err != nil {
			return
		}

		// 에러 응답 전송
		resp := protocol.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      &req.ID,
			Error: &protocol.JSONRPCError{
				Code:    protocol.ErrCodeUnauthorized,
				Message: "Unauthorized: API key invalid",
			},
		}
		respData, _ := json.Marshal(resp)
		stdout.Send(respData)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := c.Call(ctx, protocol.MethodInitialize, nil)
	if err == nil {
		t.Fatal("에러가 반환되어야 함")
	}
}

// TestCall_ContextTimeout은 컨텍스트 타임아웃 처리를 검증한다.
func TestCall_ContextTimeout(t *testing.T) {
	stdin := newMockPipe()
	stdout := newServerSide()

	c := client.NewJSONRPCClient(stdin, stdout, client.NopLogger())
	defer c.Close()
	defer stdout.Close()

	// 서버가 응답하지 않는 상황
	go func() {
		// 요청을 소비하지만 응답하지 않음
		<-stdin.ch
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := c.Call(ctx, "slow/method", nil)
	if err == nil {
		t.Fatal("타임아웃 에러가 반환되어야 함")
	}
}

// TestNotify는 알림 전송을 검증한다.
func TestNotify(t *testing.T) {
	stdin := newMockPipe()
	stdout := newServerSide()

	c := client.NewJSONRPCClient(stdin, stdout, client.NopLogger())
	defer c.Close()
	defer stdout.Close()

	err := c.Notify(protocol.MethodInitialized, nil)
	if err != nil {
		t.Fatalf("Notify 실패: %v", err)
	}

	// 전송된 데이터 확인
	select {
	case sent := <-stdin.ch:
		var notif protocol.JSONRPCNotification
		if err := json.Unmarshal([]byte(strings.TrimSpace(sent)), &notif); err != nil {
			t.Fatalf("전송된 알림 파싱 실패: %v", err)
		}
		if notif.Method != protocol.MethodInitialized {
			t.Errorf("Method 불일치: got %q, want %q", notif.Method, protocol.MethodInitialized)
		}
		if notif.JSONRPC != "2.0" {
			t.Errorf("JSONRPC 불일치: got %q, want %q", notif.JSONRPC, "2.0")
		}
	case <-time.After(time.Second):
		t.Fatal("전송 타임아웃")
	}
}

// TestOnNotification은 수신 알림 핸들러를 검증한다.
func TestOnNotification(t *testing.T) {
	stdin := newMockPipe()
	stdout := newServerSide()

	c := client.NewJSONRPCClient(stdin, stdout, client.NopLogger())
	defer c.Close()

	received := make(chan string, 1)
	c.OnNotification(protocol.MethodTurnCompleted, func(method string, params json.RawMessage) {
		received <- method
	})

	// 서버에서 알림 전송
	notif := protocol.JSONRPCNotification{
		JSONRPC: "2.0",
		Method:  protocol.MethodTurnCompleted,
		Params:  json.RawMessage(`{"threadId":"t1"}`),
	}
	notifData, _ := json.Marshal(notif)
	stdout.Send(notifData)

	select {
	case method := <-received:
		if method != protocol.MethodTurnCompleted {
			t.Errorf("수신된 메서드 불일치: got %q, want %q", method, protocol.MethodTurnCompleted)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("알림 수신 타임아웃")
	}

	stdout.Close()
}

// TestConcurrentCalls는 동시 요청 처리를 검증한다 (go test -race로 검증).
func TestConcurrentCalls(t *testing.T) {
	stdin := newMockPipe()
	stdout := newServerSide()

	c := client.NewJSONRPCClient(stdin, stdout, client.NopLogger())
	defer c.Close()

	// 서버 시뮬레이션: 요청 ID에 맞는 응답 전송
	go func() {
		for {
			reqStr, ok := <-stdin.ch
			if !ok {
				return
			}

			var req protocol.JSONRPCRequest
			if err := json.Unmarshal([]byte(strings.TrimSpace(reqStr)), &req); err != nil {
				return
			}

			result := json.RawMessage(fmt.Sprintf(`{"id":%d}`, req.ID))
			resp := protocol.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      &req.ID,
				Result:  &result,
			}
			respData, _ := json.Marshal(resp)
			stdout.Send(respData)
		}
	}()

	const numGoroutines = 10
	var wg sync.WaitGroup
	errs := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			result, err := c.Call(ctx, "test/method", map[string]int{"idx": idx})
			if err != nil {
				errs <- fmt.Errorf("goroutine %d: %v", idx, err)
				return
			}
			if result == nil {
				errs <- fmt.Errorf("goroutine %d: 결과가 nil", idx)
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Error(err)
	}
	stdout.Close()
}

// TestNotify_WithParams는 params를 가진 알림 전송을 검증한다.
func TestNotify_WithParams(t *testing.T) {
	stdin := newMockPipe()
	stdout := newServerSide()

	c := client.NewJSONRPCClient(stdin, stdout, client.NopLogger())
	defer stdout.Close()
	defer c.Close()

	params := map[string]string{"key": "value"}
	err := c.Notify(protocol.MethodInitialized, params)
	if err != nil {
		t.Fatalf("Notify 실패: %v", err)
	}

	select {
	case sent := <-stdin.ch:
		var notif protocol.JSONRPCNotification
		if err := json.Unmarshal([]byte(strings.TrimSpace(sent)), &notif); err != nil {
			t.Fatalf("파싱 실패: %v", err)
		}
		if notif.Params == nil {
			t.Error("Params가 nil이면 안 됨")
		}
	case <-time.After(time.Second):
		t.Fatal("타임아웃")
	}
}

// TestHandleResponse_NilID는 ID가 없는 응답 처리를 검증한다.
func TestHandleResponse_NilID(t *testing.T) {
	stdin := newMockPipe()
	stdout := newServerSide()

	c := client.NewJSONRPCClient(stdin, stdout, client.NopLogger())
	defer c.Close()

	// ID가 null인 응답 전송 (알림처럼 보이지만 result가 있음)
	stdout.Send([]byte(`{"jsonrpc":"2.0","id":null,"result":{}}`))

	// readLoop이 계속 동작해야 함
	time.Sleep(100 * time.Millisecond)
	stdout.Close()
}

// TestNotify_AfterClose는 종료 후 Notify 호출을 검증한다.
func TestNotify_AfterClose(t *testing.T) {
	stdin := newMockPipe()
	stdout := newServerSide()
	stdout.Close()

	c := client.NewJSONRPCClient(stdin, stdout, client.NopLogger())
	_ = c.Close()

	// 종료 후 Notify는 에러를 반환해야 함
	err := c.Notify(protocol.MethodInitialized, nil)
	if err == nil {
		t.Error("종료 후 Notify는 에러를 반환해야 함")
	}
}

// TestCall_AfterClose는 종료 후 Call 호출을 검증한다.
func TestCall_AfterClose(t *testing.T) {
	stdin := newMockPipe()
	stdout := newServerSide()
	stdout.Close()

	c := client.NewJSONRPCClient(stdin, stdout, client.NopLogger())
	_ = c.Close()

	ctx := context.Background()
	_, err := c.Call(ctx, "test/method", nil)
	if err == nil {
		t.Error("종료 후 Call은 에러를 반환해야 함")
	}
}

// TestReadLoop_NonJSONLines는 readLoop이 비-JSON 라인을 무시하는지 검증한다.
func TestReadLoop_NonJSONLines(t *testing.T) {
	stdin := newMockPipe()
	stdout := newServerSide()

	c := client.NewJSONRPCClient(stdin, stdout, client.NopLogger())
	defer c.Close()

	// 비-JSON 라인과 빈 라인 전송
	stdout.Send([]byte("not-json-line"))
	stdout.Send([]byte("another-plain-text"))

	// 그 후 실제 응답 전송
	go func() {
		reqStr, ok := <-stdin.ch
		if !ok {
			return
		}
		var req protocol.JSONRPCRequest
		if err := json.Unmarshal([]byte(strings.TrimSpace(reqStr)), &req); err != nil {
			return
		}
		result := json.RawMessage(`{"ok":true}`)
		resp := protocol.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      &req.ID,
			Result:  &result,
		}
		respData, _ := json.Marshal(resp)
		stdout.Send(respData)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// 비-JSON 라인이 있어도 정상 응답 수신 가능해야 함
	result, err := c.Call(ctx, "test/method", nil)
	if err != nil {
		t.Fatalf("Call 실패: %v", err)
	}
	if result == nil {
		t.Error("결과가 nil이면 안 됨")
	}
	stdout.Close()
}

// TestReadLoop_InvalidJSON은 readLoop이 잘못된 JSON을 처리하는지 검증한다.
func TestReadLoop_InvalidJSON(t *testing.T) {
	stdin := newMockPipe()
	stdout := newServerSide()

	c := client.NewJSONRPCClient(stdin, stdout, client.NopLogger())
	defer c.Close()

	// 잘못된 JSON 전송 ('{' 로 시작하지만 유효하지 않음)
	stdout.Send([]byte("{invalid-json"))

	// 그 후 정상 알림 전송
	received := make(chan bool, 1)
	c.OnNotification(protocol.MethodTurnCompleted, func(method string, params json.RawMessage) {
		received <- true
	})

	notif := protocol.JSONRPCNotification{
		JSONRPC: "2.0",
		Method:  protocol.MethodTurnCompleted,
	}
	notifData, _ := json.Marshal(notif)
	stdout.Send(notifData)

	select {
	case <-received:
		// 잘못된 JSON 후에도 정상 처리 가능
	case <-time.After(2 * time.Second):
		t.Fatal("알림 수신 타임아웃")
	}
	stdout.Close()
}

// TestHandleResponse_UnknownID는 알 수 없는 ID의 응답 처리를 검증한다.
func TestHandleResponse_UnknownID(t *testing.T) {
	stdin := newMockPipe()
	stdout := newServerSide()

	c := client.NewJSONRPCClient(stdin, stdout, client.NopLogger())
	defer c.Close()

	// 알 수 없는 ID의 응답 전송 (대기 중인 요청 없음)
	unknownID := int64(99999)
	result := json.RawMessage(`{}`)
	resp := protocol.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      &unknownID,
		Result:  &result,
	}
	respData, _ := json.Marshal(resp)
	stdout.Send(respData)

	// 에러 없이 무시되어야 함 (readLoop이 계속 동작해야 함)
	time.Sleep(100 * time.Millisecond)
	stdout.Close()
}

// TestHandleNotification_UnregisteredMethod는 등록되지 않은 알림 메서드 처리를 검증한다.
func TestHandleNotification_UnregisteredMethod(t *testing.T) {
	stdin := newMockPipe()
	stdout := newServerSide()

	c := client.NewJSONRPCClient(stdin, stdout, client.NopLogger())
	defer c.Close()

	// 핸들러 없는 알림 전송
	notif := protocol.JSONRPCNotification{
		JSONRPC: "2.0",
		Method:  "unknown/method",
		Params:  json.RawMessage(`{}`),
	}
	notifData, _ := json.Marshal(notif)
	stdout.Send(notifData)

	// 에러 없이 무시되어야 함
	time.Sleep(100 * time.Millisecond)
	stdout.Close()
}

// TestClose_PendingRequestsHandled는 Close 시 대기 중인 요청 처리를 검증한다.
func TestClose_PendingRequestsHandled(t *testing.T) {
	stdin := newMockPipe()
	stdout := newServerSide()

	c := client.NewJSONRPCClient(stdin, stdout, client.NopLogger())

	// 응답이 없는 요청 시작
	errCh := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err := c.Call(ctx, "slow/method", nil)
		errCh <- err
	}()

	// 요청이 전송될 때까지 대기
	time.Sleep(100 * time.Millisecond)

	// 클라이언트 종료
	stdout.Close()
	_ = c.Close()

	// 대기 중인 요청이 에러와 함께 반환되어야 함
	select {
	case err := <-errCh:
		if err == nil {
			t.Error("종료 시 에러가 반환되어야 함")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("종료 타임아웃")
	}
}

// unmarshalable은 JSON 직렬화가 불가능한 타입이다 (채널은 JSON으로 변환 불가).
type unmarshalable struct {
	Ch chan int
}

// TestDone_ChannelClosedAfterEOF는 Done() 채널이 readLoop 종료 후 닫히는지 검증한다.
func TestDone_ChannelClosedAfterEOF(t *testing.T) {
	stdin := newMockPipe()
	stdout := newServerSide()

	c := client.NewJSONRPCClient(stdin, stdout, client.NopLogger())

	// Done() 채널이 열려 있는지 확인
	select {
	case <-c.Done():
		t.Fatal("readLoop 종료 전에 Done 채널이 닫히면 안 됨")
	default:
		// 정상: 아직 열려 있음
	}

	// stdout을 닫아 readLoop에 EOF 전달
	stdout.Close()

	// Done() 채널이 닫힐 때까지 대기
	select {
	case <-c.Done():
		// 정상: readLoop 종료 후 Done 채널이 닫혔음
	case <-time.After(2 * time.Second):
		t.Fatal("readLoop 종료 후 Done 채널이 닫혀야 함")
	}

	_ = c.Close()
}

// TestNotify_MarshalError는 직렬화 불가 params로 Notify 호출 시 에러 반환을 검증한다.
func TestNotify_MarshalError(t *testing.T) {
	stdin := newMockPipe()
	stdout := newServerSide()
	defer stdout.Close()

	c := client.NewJSONRPCClient(stdin, stdout, client.NopLogger())
	defer c.Close()

	// 직렬화 불가 타입으로 Notify 호출
	err := c.Notify("test/method", unmarshalable{Ch: make(chan int)})
	if err == nil {
		t.Fatal("직렬화 불가 params로 Notify는 에러를 반환해야 함")
	}
}

// TestCall_MarshalError는 직렬화 불가 params로 Call 호출 시 에러 반환을 검증한다.
func TestCall_MarshalError(t *testing.T) {
	stdin := newMockPipe()
	stdout := newServerSide()
	defer stdout.Close()

	c := client.NewJSONRPCClient(stdin, stdout, client.NopLogger())
	defer c.Close()

	ctx := context.Background()
	// 직렬화 불가 타입으로 Call 호출
	_, err := c.Call(ctx, "test/method", unmarshalable{Ch: make(chan int)})
	if err == nil {
		t.Fatal("직렬화 불가 params로 Call은 에러를 반환해야 함")
	}
}
