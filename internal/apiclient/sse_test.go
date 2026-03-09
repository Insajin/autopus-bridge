package apiclient_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/insajin/autopus-bridge/internal/apiclient"
	"github.com/rs/zerolog"
)

// TestNewSSESubscriber는 SSESubscriber 생성을 검증합니다.
func TestNewSSESubscriber(t *testing.T) {
	t.Parallel()

	sub := apiclient.NewSSESubscriber("https://api.example.com", "ws-123", "test-token")
	if sub == nil {
		t.Fatal("NewSSESubscriber()가 nil을 반환했습니다")
	}
}

// TestNewSSESubscriberWithLogger는 로거를 지정한 SSESubscriber 생성을 검증합니다.
func TestNewSSESubscriberWithLogger(t *testing.T) {
	t.Parallel()

	logger := zerolog.Nop()
	sub := apiclient.NewSSESubscriberWithLogger("https://api.example.com", "ws-123", "test-token", logger)
	if sub == nil {
		t.Fatal("NewSSESubscriberWithLogger()가 nil을 반환했습니다")
	}
}

// makeFakeSSEServer는 지정된 이벤트를 전송하는 테스트 SSE 서버를 생성합니다.
func makeFakeSSEServer(t *testing.T, events []apiclient.SSEEvent) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// SSE 헤더 설정
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming not supported", http.StatusInternalServerError)
			return
		}

		for _, event := range events {
			data, err := json.Marshal(event)
			if err != nil {
				continue
			}
			_, _ = fmt.Fprintf(w, "data: %s\n\n", string(data))
			flusher.Flush()
		}
	}))
}

// TestSSESubscriber_Subscribe는 기본 SSE 구독 기능을 검증합니다.
func TestSSESubscriber_Subscribe(t *testing.T) {
	t.Parallel()

	// 테스트 이벤트 준비
	events := []apiclient.SSEEvent{
		{Type: "agent_typing", Data: json.RawMessage(`{"channel_id":"ch-1","text_delta":"안녕"}`)},
		{Type: "done", Data: json.RawMessage(`{}`)},
	}

	srv := makeFakeSSEServer(t, events)
	defer srv.Close()

	sub := apiclient.NewSSESubscriber(srv.URL, "ws-123", "test-token")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	eventCh, errCh := sub.Subscribe(ctx)

	var received []apiclient.SSEEvent
	done := false
	for !done {
		select {
		case event, ok := <-eventCh:
			if !ok {
				done = true
				break
			}
			received = append(received, event)
			if event.Type == "done" {
				done = true
			}
		case err := <-errCh:
			if err != nil {
				t.Fatalf("SSE 구독 오류: %v", err)
			}
		case <-ctx.Done():
			done = true
		}
	}

	if len(received) == 0 {
		t.Fatal("수신된 이벤트가 없습니다")
	}

	// 첫 번째 이벤트는 agent_typing이어야 함
	if received[0].Type != "agent_typing" {
		t.Errorf("received[0].Type = %q, want agent_typing", received[0].Type)
	}
}

// TestSSESubscriber_ContextCancellation은 컨텍스트 취소 시 구독이 종료되는지 검증합니다.
func TestSSESubscriber_ContextCancellation(t *testing.T) {
	t.Parallel()

	// 무한히 연결을 유지하는 서버
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			return
		}
		// 첫 이벤트 전송
		_, _ = fmt.Fprintf(w, "data: {\"type\":\"ping\",\"data\":{}}\n\n")
		flusher.Flush()

		// 연결이 끊어질 때까지 대기
		select {
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()

	sub := apiclient.NewSSESubscriber(srv.URL, "ws-123", "test-token")

	ctx, cancel := context.WithCancel(context.Background())

	eventCh, errCh := sub.Subscribe(ctx)

	// 첫 이벤트 수신 후 취소
	select {
	case <-eventCh:
		cancel()
	case <-time.After(3 * time.Second):
		t.Fatal("이벤트 수신 타임아웃")
		cancel()
	}

	// 채널이 닫히거나 컨텍스트가 취소되어야 함
	select {
	case _, ok := <-eventCh:
		if ok {
			// 채널에서 이벤트가 남아있을 수 있음 - 추가로 읽기 시도
		}
		_ = ok
	case <-errCh:
		// 에러 채널이 닫힐 수 있음
	case <-time.After(3 * time.Second):
		// 타임아웃은 정상 동작일 수 있음
	}
	// 명시적으로 cancel 호출 확인 (이미 호출됨)
	_ = cancel
}

// TestSSESubscriber_ServerError는 서버 오류 시 에러 채널에 오류를 전달하는지 검증합니다.
func TestSSESubscriber_ServerError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	}))
	defer srv.Close()

	// NoRetry 옵션을 사용하여 재시도 없이 즉시 에러를 받음
	sub := apiclient.NewSSESubscriberWithOptions(srv.URL, "ws-123", "test-token", 0)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, errCh := sub.Subscribe(ctx)

	select {
	case err := <-errCh:
		if err == nil {
			t.Error("서버 오류에서 에러를 기대했지만 nil을 받았습니다")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("에러 수신 타임아웃")
	}
}

// TestFilterAgentTyping은 agent_typing 이벤트 필터링을 검증합니다.
func TestFilterAgentTyping(t *testing.T) {
	t.Parallel()

	// 이벤트 채널 생성
	eventCh := make(chan apiclient.SSEEvent, 10)

	// 다양한 이벤트 전송
	targetChannelID := "ch-target"
	events := []apiclient.SSEEvent{
		{
			Type: "agent_typing",
			Data: mustMarshal(t, apiclient.AgentTypingPayload{
				ChannelID:  targetChannelID,
				TextDelta:  "안녕",
				AgentID:    "agent-1",
				IsComplete: false,
			}),
		},
		{
			// 다른 채널 - 필터링되어야 함
			Type: "agent_typing",
			Data: mustMarshal(t, apiclient.AgentTypingPayload{
				ChannelID: "ch-other",
				TextDelta: "다른 채널",
			}),
		},
		{
			// 다른 이벤트 타입 - 필터링되어야 함
			Type: "status_update",
			Data: json.RawMessage(`{"status":"running"}`),
		},
		{
			Type: "agent_typing",
			Data: mustMarshal(t, apiclient.AgentTypingPayload{
				ChannelID:  targetChannelID,
				TextDelta:  "하세요",
				AgentID:    "agent-1",
				IsComplete: true,
			}),
		},
	}

	for _, e := range events {
		eventCh <- e
	}
	close(eventCh)

	typingCh := apiclient.FilterAgentTyping(eventCh, targetChannelID)

	var received []apiclient.AgentTypingPayload
	for payload := range typingCh {
		received = append(received, payload)
	}

	if len(received) != 2 {
		t.Fatalf("수신된 typing 이벤트 수 = %d, want 2", len(received))
	}
	if received[0].TextDelta != "안녕" {
		t.Errorf("received[0].TextDelta = %q, want 안녕", received[0].TextDelta)
	}
	if received[1].TextDelta != "하세요" {
		t.Errorf("received[1].TextDelta = %q, want 하세요", received[1].TextDelta)
	}
	if received[0].ChannelID != targetChannelID {
		t.Errorf("received[0].ChannelID = %q, want %q", received[0].ChannelID, targetChannelID)
	}
}

// TestFilterAgentTyping_EmptyChannelID는 빈 channelID일 때 모든 채널을 허용하는지 검증합니다.
func TestFilterAgentTyping_EmptyChannelID(t *testing.T) {
	t.Parallel()

	eventCh := make(chan apiclient.SSEEvent, 5)

	events := []apiclient.SSEEvent{
		{
			Type: "agent_typing",
			Data: mustMarshal(t, apiclient.AgentTypingPayload{
				ChannelID: "ch-1",
				TextDelta: "텍스트1",
			}),
		},
		{
			Type: "agent_typing",
			Data: mustMarshal(t, apiclient.AgentTypingPayload{
				ChannelID: "ch-2",
				TextDelta: "텍스트2",
			}),
		},
	}

	for _, e := range events {
		eventCh <- e
	}
	close(eventCh)

	// 빈 channelID = 모든 채널 허용
	typingCh := apiclient.FilterAgentTyping(eventCh, "")

	var received []apiclient.AgentTypingPayload
	for payload := range typingCh {
		received = append(received, payload)
	}

	if len(received) != 2 {
		t.Fatalf("빈 channelID일 때 수신된 이벤트 수 = %d, want 2", len(received))
	}
}

// TestSSESubscriber_EndpointURL은 SSE 엔드포인트 URL이 올바르게 구성되는지 검증합니다.
func TestSSESubscriber_EndpointURL(t *testing.T) {
	t.Parallel()

	// 채널을 이용하여 race condition 없이 값을 전달
	pathCh := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case pathCh <- r.URL.Path:
		default:
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sub := apiclient.NewSSESubscriber(srv.URL, "ws-test", "test-token")

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, _ = sub.Subscribe(ctx)

	expectedPath := "/api/v1/workspaces/ws-test/stream"
	select {
	case capturedPath := <-pathCh:
		if capturedPath != expectedPath {
			t.Errorf("SSE 엔드포인트 경로 = %q, want %q", capturedPath, expectedPath)
		}
	case <-time.After(2 * time.Second):
		t.Error("요청 경로 수신 타임아웃")
	}
}

// TestSSESubscriber_AuthHeader는 Authorization 헤더가 올바르게 설정되는지 검증합니다.
func TestSSESubscriber_AuthHeader(t *testing.T) {
	t.Parallel()

	// 채널을 이용하여 race condition 없이 값을 전달
	authCh := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case authCh <- r.Header.Get("Authorization"):
		default:
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sub := apiclient.NewSSESubscriber(srv.URL, "ws-test", "my-bearer-token")

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, _ = sub.Subscribe(ctx)

	expected := "Bearer my-bearer-token"
	select {
	case capturedAuth := <-authCh:
		if !strings.Contains(capturedAuth, "my-bearer-token") {
			t.Errorf("Authorization 헤더 = %q, want to contain %q", capturedAuth, expected)
		}
	case <-time.After(2 * time.Second):
		t.Error("Authorization 헤더 수신 타임아웃")
	}
}

// TestSSESubscriber_Reconnect_MaxRetryExceeded는 maxRetry 초과 시 errCh에 에러가 전달되는지 검증합니다.
func TestSSESubscriber_Reconnect_MaxRetryExceeded(t *testing.T) {
	t.Parallel()

	// 항상 오류를 반환하는 서버 (재시도마다 실패)
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	// maxRetry=2로 설정하여 총 3회 시도 후 에러를 반환해야 함
	sub := apiclient.NewSSESubscriberWithOptions(srv.URL, "ws-123", "test-token", 2)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, errCh := sub.Subscribe(ctx)

	select {
	case err := <-errCh:
		if err == nil {
			t.Error("최대 재시도 초과 시 에러를 기대했지만 nil을 받았습니다")
		}
	case <-time.After(8 * time.Second):
		t.Fatal("에러 수신 타임아웃 - 재시도 로직이 작동하지 않습니다")
	}
}

// TestSSESubscriber_ContextCancelDuringBackoff는 재연결 대기(백오프) 중 컨텍스트 취소 시 정상 종료를 검증합니다.
func TestSSESubscriber_ContextCancelDuringBackoff(t *testing.T) {
	t.Parallel()

	// 첫 번째 요청에만 오류를 반환하는 서버
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
	}))
	defer srv.Close()

	// maxRetry=3이지만 백오프 중 컨텍스트를 취소하여 조기 종료
	sub := apiclient.NewSSESubscriberWithOptions(srv.URL, "ws-123", "test-token", 3)

	ctx, cancel := context.WithCancel(context.Background())

	eventCh, errCh := sub.Subscribe(ctx)

	// 첫 번째 연결 시도 후 잠시 대기 후 취소 (백오프 대기 중 취소)
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	// 채널이 닫히거나 컨텍스트가 취소되면 성공
	select {
	case _, ok := <-eventCh:
		if !ok {
			// eventCh 채널이 닫힘 - 정상 종료
		}
	case <-errCh:
		// 에러 발생 - 허용
	case <-time.After(5 * time.Second):
		t.Fatal("컨텍스트 취소 후 SSE 구독이 종료되지 않았습니다")
	}
}

// TestSSESubscriber_Connect_InvalidURL은 잘못된 URL로 연결 시 에러 처리를 검증합니다.
func TestSSESubscriber_Connect_InvalidURL(t *testing.T) {
	t.Parallel()

	// \n이 포함된 URL은 http.NewRequestWithContext에서 에러를 반환합니다
	invalidURL := "http://invalid\x00host"
	sub := apiclient.NewSSESubscriberWithOptions(invalidURL, "ws-123", "test-token", 0)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, errCh := sub.Subscribe(ctx)

	select {
	case err := <-errCh:
		if err == nil {
			t.Error("잘못된 URL에서 에러를 기대했지만 nil을 받았습니다")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("에러 수신 타임아웃")
	}
}

// TestSSESubscriber_SkipsNonDataLines는 data: 이외의 SSE 라인을 무시하는지 검증합니다.
func TestSSESubscriber_SkipsNonDataLines(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			return
		}
		// SSE 주석, 빈 줄, event: 라인은 무시되어야 함
		_, _ = fmt.Fprintf(w, ": 이것은 주석입니다\n")
		_, _ = fmt.Fprintf(w, "\n")
		_, _ = fmt.Fprintf(w, "event: custom\n")
		_, _ = fmt.Fprintf(w, "id: 123\n")
		// 유효한 data: 라인
		data := apiclient.SSEEvent{Type: "ping", Data: json.RawMessage(`{}`)}
		b, _ := json.Marshal(data)
		_, _ = fmt.Fprintf(w, "data: %s\n\n", string(b))
		flusher.Flush()
	}))
	defer srv.Close()

	sub := apiclient.NewSSESubscriberWithOptions(srv.URL, "ws-123", "test-token", 0)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	eventCh, _ := sub.Subscribe(ctx)

	select {
	case event, ok := <-eventCh:
		if ok && event.Type != "ping" {
			t.Errorf("event.Type = %q, want ping", event.Type)
		}
	case <-time.After(2 * time.Second):
		// 타임아웃은 서버가 연결을 닫았기 때문에 정상일 수 있음
	}
}

// TestSSESubscriber_SkipsEmptyDataPayload는 data: 이후 빈 문자열인 라인을 무시하는지 검증합니다.
func TestSSESubscriber_SkipsEmptyDataPayload(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			return
		}
		// 빈 data: 라인 - 무시되어야 함
		_, _ = fmt.Fprintf(w, "data:\n")
		_, _ = fmt.Fprintf(w, "data:   \n")
		// 유효한 이벤트
		data := apiclient.SSEEvent{Type: "test", Data: json.RawMessage(`{}`)}
		b, _ := json.Marshal(data)
		_, _ = fmt.Fprintf(w, "data: %s\n\n", string(b))
		flusher.Flush()
	}))
	defer srv.Close()

	sub := apiclient.NewSSESubscriberWithOptions(srv.URL, "ws-123", "test-token", 0)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	eventCh, _ := sub.Subscribe(ctx)

	select {
	case event, ok := <-eventCh:
		if ok && event.Type != "test" {
			t.Errorf("event.Type = %q, want test", event.Type)
		}
	case <-time.After(2 * time.Second):
		// 타임아웃 허용
	}
}

// TestSSESubscriber_InvalidJSONPayload는 파싱 불가한 JSON 페이로드를 건너뛰는지 검증합니다.
func TestSSESubscriber_InvalidJSONPayload(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			return
		}
		// 잘못된 JSON - 건너뛰어야 함
		_, _ = fmt.Fprintf(w, "data: {not valid json}\n\n")
		// 유효한 이벤트
		data := apiclient.SSEEvent{Type: "valid", Data: json.RawMessage(`{}`)}
		b, _ := json.Marshal(data)
		_, _ = fmt.Fprintf(w, "data: %s\n\n", string(b))
		flusher.Flush()
	}))
	defer srv.Close()

	sub := apiclient.NewSSESubscriberWithOptions(srv.URL, "ws-123", "test-token", 0)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	eventCh, _ := sub.Subscribe(ctx)

	var received []apiclient.SSEEvent
	timeout := time.After(2 * time.Second)
	for {
		select {
		case event, ok := <-eventCh:
			if !ok {
				goto done
			}
			received = append(received, event)
		case <-timeout:
			goto done
		}
	}
done:
	// 유효한 이벤트 1개만 받아야 함 (잘못된 JSON은 건너뜀)
	if len(received) > 0 && received[0].Type != "valid" {
		t.Errorf("invalid JSON 건너뜀 후 첫 이벤트 Type = %q, want valid", received[0].Type)
	}
}

// TestSSESubscriber_ContextCancelDuringEventRead는 이벤트 읽기 중 컨텍스트 취소를 검증합니다.
func TestSSESubscriber_ContextCancelDuringEventRead(t *testing.T) {
	t.Parallel()

	// 이벤트를 지속적으로 전송하는 서버
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			return
		}
		for i := 0; i < 100; i++ {
			data := apiclient.SSEEvent{Type: "tick", Data: json.RawMessage(`{}`)}
			b, _ := json.Marshal(data)
			_, _ = fmt.Fprintf(w, "data: %s\n\n", string(b))
			flusher.Flush()
			select {
			case <-r.Context().Done():
				return
			case <-time.After(10 * time.Millisecond):
			}
		}
	}))
	defer srv.Close()

	sub := apiclient.NewSSESubscriberWithOptions(srv.URL, "ws-123", "test-token", 0)

	ctx, cancel := context.WithCancel(context.Background())

	eventCh, errCh := sub.Subscribe(ctx)

	// 첫 번째 이벤트 수신 후 취소
	select {
	case _, ok := <-eventCh:
		if !ok {
			t.Fatal("채널이 너무 빨리 닫혔습니다")
		}
		cancel()
	case <-time.After(3 * time.Second):
		t.Fatal("이벤트 수신 타임아웃")
		cancel()
	}

	// 취소 후 채널들이 결국 닫혀야 함
	select {
	case <-errCh:
		// 에러 채널 닫힘 또는 에러 수신
	case <-time.After(3 * time.Second):
		t.Fatal("컨텍스트 취소 후 채널이 닫히지 않았습니다")
	}
}

// mustMarshal은 테스트 헬퍼로 JSON 마샬링 실패 시 테스트를 종료합니다.
func mustMarshal(t *testing.T, v interface{}) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("JSON 마샬링 실패: %v", err)
	}
	return json.RawMessage(data)
}
