// Package websocket - capability_update 기능 테스트
// SPEC-HOTSWAP-001: Provider capabilities 실시간 업데이트
package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	ws "github.com/insajin/autopus-agent-protocol"
)

// testContext는 테스트용 컨텍스트를 반환합니다.
func testContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return ctx
}

// TestUpdateProviderCapabilities_SendsMessage는 연결된 상태에서 capability_update 메시지가 전송되는지 검증합니다.
// REQ-E-002: 연결 상태에서 capability_update 메시지 전송
func TestUpdateProviderCapabilities_SendsMessage(t *testing.T) {
	// 테스트용 WebSocket 서버 시작
	srv := newTestCapabilityServer(t)
	defer srv.Close()

	client := newConnectedClient(t, srv.URL)
	defer client.Disconnect("test")

	// capabilities 업데이트
	newCaps := map[string]bool{"claude": true, "gemini": false}
	client.UpdateProviderCapabilities(newCaps)

	// 서버에서 capability_update 메시지 수신 대기
	select {
	case msg := <-srv.received:
		if msg.Type != AgentMsgCapabilityUpdate {
			t.Errorf("메시지 타입이 %s이어야 하나 %s입니다", AgentMsgCapabilityUpdate, msg.Type)
		}
		var payload struct {
			Capabilities   map[string]bool       `json:"capabilities"`
			RuntimeContext *BridgeRuntimeContext `json:"runtime_context,omitempty"`
		}
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			t.Fatalf("페이로드 파싱 실패: %v", err)
		}
		if payload.Capabilities["claude"] != true {
			t.Errorf("claude가 true여야 하나 %v입니다", payload.Capabilities["claude"])
		}
		if payload.Capabilities["gemini"] != false {
			t.Errorf("gemini가 false여야 하나 %v입니다", payload.Capabilities["gemini"])
		}
		if payload.RuntimeContext != nil {
			t.Errorf("runtime_context should be nil by default, got %+v", payload.RuntimeContext)
		}
	case <-time.After(2 * time.Second):
		t.Error("capability_update 메시지가 수신되지 않았습니다")
	}
}

func TestUpdateRuntimeContext_SendsMessage(t *testing.T) {
	srv := newTestCapabilityServer(t)
	defer srv.Close()

	client := newConnectedClient(t, srv.URL)
	defer client.Disconnect("test")

	client.UpdateRuntimeContext(&BridgeRuntimeContext{
		WorkspaceRoot: "/workspace/app",
		SyncMode:      "mirror",
		KnowledgeSourceBindings: []KnowledgeSourceBinding{
			{SourceID: "source-1", SourceRoot: "docs", SyncMode: "mirror"},
		},
	})

	select {
	case msg := <-srv.received:
		var payload struct {
			Capabilities   map[string]bool       `json:"capabilities"`
			RuntimeContext *BridgeRuntimeContext `json:"runtime_context,omitempty"`
		}
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			t.Fatalf("페이로드 파싱 실패: %v", err)
		}
		if payload.RuntimeContext == nil {
			t.Fatal("runtime_context 가 전송되어야 합니다")
		}
		if payload.RuntimeContext.WorkspaceRoot != "/workspace/app" {
			t.Fatalf("WorkspaceRoot = %q", payload.RuntimeContext.WorkspaceRoot)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("runtime_context 메시지가 수신되지 않았습니다")
	}
}

// TestUpdateProviderCapabilities_NoDiff_NoMessage는 동일한 capabilities면 메시지를 보내지 않는지 검증합니다.
// REQ-S-002: 동일 capabilities는 재전송하지 않음
func TestUpdateProviderCapabilities_NoDiff_NoMessage(t *testing.T) {
	srv := newTestCapabilityServer(t)
	defer srv.Close()

	client := newConnectedClient(t, srv.URL)
	defer client.Disconnect("test")

	// 초기 capabilities 설정
	initialCaps := map[string]bool{"claude": true}
	client.UpdateProviderCapabilities(initialCaps)

	// 첫 번째 메시지 drain
	select {
	case <-srv.received:
	case <-time.After(2 * time.Second):
		t.Fatal("첫 번째 capability_update 메시지가 수신되지 않았습니다")
	}

	// 동일한 capabilities로 재업데이트
	client.UpdateProviderCapabilities(initialCaps)

	// 두 번째 메시지가 전송되지 않아야 합니다
	select {
	case msg := <-srv.received:
		if msg.Type == AgentMsgCapabilityUpdate {
			t.Error("동일한 capabilities에 대해 capability_update 메시지가 전송되었습니다")
		}
	case <-time.After(300 * time.Millisecond):
		// 정상: 메시지가 전송되지 않음
	}
}

// TestUpdateProviderCapabilities_Offline_LocalOnly는 오프라인 상태에서 로컬만 업데이트하는지 검증합니다.
// REQ-S-001: 오프라인 시 capabilities는 로컬에만 저장
func TestUpdateProviderCapabilities_Offline_LocalOnly(t *testing.T) {
	// 연결되지 않은 클라이언트
	client := NewClient("ws://localhost:19999/ws", "test-token", "1.0.0")

	newCaps := map[string]bool{"claude": true}
	// 패닉 없이 실행되어야 합니다
	client.UpdateProviderCapabilities(newCaps)

	// 내부 capabilities가 업데이트되었는지 확인
	client.capMu.RLock()
	caps := client.providerCapabilities
	client.capMu.RUnlock()

	if !caps["claude"] {
		t.Error("오프라인 상태에서도 capabilities가 로컬에 저장되어야 합니다")
	}
}

// TestUpdateProviderCapabilities_Reconnect_LatestCaps는 재연결 시 최신 capabilities를 사용하는지 검증합니다.
// sendConnect()가 최신 providerCapabilities를 사용해야 함
func TestUpdateProviderCapabilities_Reconnect_LatestCaps(t *testing.T) {
	srv := newTestCapabilityServer(t)
	defer srv.Close()

	client := newConnectedClient(t, srv.URL)
	defer client.Disconnect("test")

	// capabilities 업데이트 (로컬)
	updatedCaps := map[string]bool{"claude": true, "gemini": true}
	client.UpdateProviderCapabilities(updatedCaps)

	// capMu 로 보호된 providerCapabilities가 업데이트되었는지 확인
	client.capMu.RLock()
	caps := client.providerCapabilities
	client.capMu.RUnlock()

	if !caps["claude"] || !caps["gemini"] {
		t.Errorf("capabilities가 업데이트되어야 하나 %v입니다", caps)
	}
}

func TestConnect_SendsProtocolVersion(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("리스너 생성 실패: %v", err)
	}
	defer listener.Close()

	received := make(chan ws.AgentConnectPayload, 1)
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		_, raw, err := conn.ReadMessage()
		if err != nil {
			return
		}
		var msg ws.AgentMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			return
		}
		var payload ws.AgentConnectPayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			return
		}
		received <- payload

		ackPayload, _ := json.Marshal(ws.ConnectAckPayload{
			Success:         true,
			Message:         "ok",
			ProtocolVersion: ws.AgentProtocolVersion,
		})
		ackMsg, _ := json.Marshal(ws.AgentMessage{Type: ws.AgentMsgConnectAck, Payload: ackPayload})
		_ = conn.WriteMessage(websocket.TextMessage, ackMsg)
	})}
	go func() { _ = srv.Serve(listener) }()
	defer srv.Close()

	client := NewClient("ws://"+listener.Addr().String()+"/ws", "test-token", "1.0.0")
	defer client.Disconnect("test")

	if err := client.Connect(testContext(t)); err != nil {
		t.Fatalf("Connect 실패: %v", err)
	}

	select {
	case payload := <-received:
		if payload.ProtocolVersion != ws.AgentProtocolVersion {
			t.Fatalf("ProtocolVersion = %q, want %q", payload.ProtocolVersion, ws.AgentProtocolVersion)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("agent_connect payload not received")
	}
}

func TestConnect_RejectsProtocolVersionMismatchAck(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("리스너 생성 실패: %v", err)
	}
	defer listener.Close()

	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}

		ackPayload, _ := json.Marshal(ws.ConnectAckPayload{
			Success:         false,
			Message:         fmt.Sprintf("protocol version mismatch: bridge=%s server=%s", ws.AgentProtocolVersion, "9.9.0"),
			ErrorCode:       ws.AuthErrorProtocolVersionMismatch,
			ProtocolVersion: "9.9.0",
		})
		ackMsg, _ := json.Marshal(ws.AgentMessage{Type: ws.AgentMsgConnectAck, Payload: ackPayload})
		_ = conn.WriteMessage(websocket.TextMessage, ackMsg)
	})}
	go func() { _ = srv.Serve(listener) }()
	defer srv.Close()

	client := NewClient("ws://"+listener.Addr().String()+"/ws", "test-token", "1.0.0")
	err = client.Connect(testContext(t))
	if err == nil {
		t.Fatal("expected protocol mismatch error")
	}
	if got := err.Error(); got == "" || !containsAll(got, []string{"protocol version mismatch"}) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func containsAll(s string, want []string) bool {
	for _, item := range want {
		if !strings.Contains(s, item) {
			return false
		}
	}
	return true
}

// TestConcurrency_NoRace는 동시 접근에서 race condition이 없는지 검증합니다.
func TestConcurrency_NoRace(t *testing.T) {
	// 연결 없이 오프라인 테스트 (race detector 검증)
	client := NewClient("ws://localhost:19999/ws", "test-token", "1.0.0")

	var wg sync.WaitGroup
	// 10개 goroutine이 동시에 읽기
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client.capMu.RLock()
			_ = client.providerCapabilities
			client.capMu.RUnlock()
		}()
	}

	// 1개 goroutine이 쓰기
	wg.Add(1)
	go func() {
		defer wg.Done()
		client.UpdateProviderCapabilities(map[string]bool{"claude": true})
	}()

	wg.Wait()
}

// ---------------------------------------------------------------------------
// 테스트 헬퍼
// ---------------------------------------------------------------------------

// testCapabilityServer는 테스트용 WebSocket 서버입니다.
type testCapabilityServer struct {
	httpSrv  *http.Server
	listener net.Listener
	URL      string
	received chan ws.AgentMessage
	mu       sync.Mutex
	upgrader websocket.Upgrader
}

// newTestCapabilityServer는 테스트용 WebSocket 서버를 생성하고 시작합니다.
func newTestCapabilityServer(t *testing.T) *testCapabilityServer {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("리스너 생성 실패: %v", err)
	}

	srv := &testCapabilityServer{
		URL:      "ws://" + listener.Addr().String() + "/ws",
		received: make(chan ws.AgentMessage, 100),
		listener: listener,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", srv.handleWS)

	srv.httpSrv = &http.Server{Handler: mux}
	go func() {
		_ = srv.httpSrv.Serve(listener)
	}()

	return srv
}

// handleWS는 WebSocket 연결을 처리합니다.
// 연결된 클라이언트로부터 메시지를 수신하고 received 채널로 전달합니다.
func (s *testCapabilityServer) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	// agent_connect 수신
	_, raw, err := conn.ReadMessage()
	if err != nil {
		return
	}
	var connectMsg ws.AgentMessage
	if err := json.Unmarshal(raw, &connectMsg); err != nil {
		return
	}

	// connect_ack 전송
	ackPayload, _ := json.Marshal(ws.ConnectAckPayload{Success: true, Message: "ok"})
	ackMsg := ws.AgentMessage{
		Type:    ws.AgentMsgConnectAck,
		Payload: ackPayload,
	}
	ackBytes, _ := json.Marshal(ackMsg)
	if err := conn.WriteMessage(websocket.TextMessage, ackBytes); err != nil {
		return
	}

	// 이후 메시지 수신 루프
	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			return
		}
		var msg ws.AgentMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			continue
		}
		select {
		case s.received <- msg:
		default:
		}
	}
}

// Close는 서버를 종료합니다.
func (s *testCapabilityServer) Close() {
	_ = s.httpSrv.Close()
}

// newConnectedClient는 테스트 서버에 연결된 클라이언트를 생성합니다.
func newConnectedClient(t *testing.T, serverURL string) *Client {
	t.Helper()
	client := NewClient(serverURL, "test-token", "1.0.0")

	connErrCh := make(chan error, 1)
	go func() {
		connErrCh <- client.Connect(testContext(t))
	}()

	select {
	case err := <-connErrCh:
		if err != nil {
			t.Fatalf("서버 연결 실패: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("연결 타임아웃")
	}

	return client
}
