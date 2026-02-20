package websocket

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	ws "github.com/insajin/autopus-agent-protocol"
	"github.com/insajin/autopus-bridge/internal/auth"
	"github.com/insajin/autopus-bridge/internal/mcpserver"
	"github.com/rs/zerolog"
)

// ---------------------------------------------------------------------------
// Integration Tests: MCP Serve Lifecycle (SPEC-AI-003 M6)
// ---------------------------------------------------------------------------
//
// 이 파일은 WebSocket MCP serve 라이프사이클의 통합 테스트를 포함합니다.
// 모의 WebSocket 서버를 통해 실제 메시지 라운드트립을 검증합니다.
//
// 시나리오 8:  전체 라이프사이클 (start -> ready -> stop -> result)
// 시나리오 8b: 이미 실행 중인 서버에 대한 중복 시작
// 시나리오 8c: 실행 중이 아닌 서버에 대한 중지
// 시나리오 15: 기존 핸들러 회귀 검증

// mcpServeTestUpgrader는 테스트용 WebSocket 업그레이더입니다.
var mcpServeTestUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// mcpServeTestRouter는 인증이 설정된 Router를 생성하는 헬퍼입니다.
func mcpServeTestRouter(client *Client) *Router {
	creds := &auth.Credentials{
		AccessToken:  "test-token",
		RefreshToken: "test-refresh",
		ExpiresAt:    time.Now().Add(1 * time.Hour),
		ServerURL:    "https://api.autopus.co",
	}
	tr := auth.NewTokenRefresher(creds)
	logger := zerolog.Nop()

	return NewRouter(client,
		WithErrorHandler(func(err error) {}),
		WithMCPServeAuth(tr, logger),
	)
}

// setupMCPServeTestServer는 모의 WebSocket 서버를 생성합니다.
// 연결 핸드셰이크(connect_ack)를 자동으로 처리하고,
// 이후 서버에서 보내는 메시지와 클라이언트 응답을 채널로 전달합니다.
//
// serverSend: 테스트에서 클라이언트로 보낼 메시지
// clientResponses: 클라이언트가 서버로 보낸 응답 메시지
func setupMCPServeTestServer(t *testing.T) (
	server *httptest.Server,
	serverSend chan ws.AgentMessage,
	clientResponses chan ws.AgentMessage,
) {
	t.Helper()

	serverSend = make(chan ws.AgentMessage, 10)
	clientResponses = make(chan ws.AgentMessage, 10)

	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := mcpServeTestUpgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Logf("[mock-server] upgrade error: %v", err)
			return
		}
		defer conn.Close()

		// 1. agent_connect 메시지 수신
		_, data, err := conn.ReadMessage()
		if err != nil {
			t.Logf("[mock-server] read connect error: %v", err)
			return
		}

		var connectMsg ws.AgentMessage
		if err := json.Unmarshal(data, &connectMsg); err != nil {
			t.Logf("[mock-server] unmarshal connect error: %v", err)
			return
		}
		if connectMsg.Type != ws.AgentMsgConnect {
			t.Logf("[mock-server] expected agent_connect, got %s", connectMsg.Type)
			return
		}

		// 2. connect_ack 전송
		ackPayload, _ := json.Marshal(ws.ConnectAckPayload{
			Success: true,
			Message: "authenticated",
		})
		ackMsg := ws.AgentMessage{
			Type:      ws.AgentMsgConnectAck,
			ID:        "ack-001",
			Timestamp: time.Now(),
			Payload:   ackPayload,
		}
		ackData, _ := json.Marshal(ackMsg)
		if err := conn.WriteMessage(websocket.TextMessage, ackData); err != nil {
			t.Logf("[mock-server] write ack error: %v", err)
			return
		}

		// 3. 서버에서 클라이언트로 메시지 전송 goroutine
		go func() {
			for msg := range serverSend {
				msgData, err := json.Marshal(msg)
				if err != nil {
					t.Logf("[mock-server] marshal error: %v", err)
					continue
				}
				if err := conn.WriteMessage(websocket.TextMessage, msgData); err != nil {
					t.Logf("[mock-server] write error: %v", err)
					return
				}
			}
		}()

		// 4. 클라이언트 응답 수신 루프
		for {
			_, msgData, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var msg ws.AgentMessage
			if err := json.Unmarshal(msgData, &msg); err != nil {
				continue
			}
			// heartbeat 메시지는 무시 (테스트 관련 메시지만 전달)
			if msg.Type == ws.AgentMsgHeartbeat {
				continue
			}
			clientResponses <- msg
		}
	}))

	return server, serverSend, clientResponses
}

// connectMCPServeTestClient는 모의 서버에 연결된 Client + Router를 생성합니다.
func connectMCPServeTestClient(t *testing.T, serverURL string) (*Client, *Router) {
	t.Helper()

	wsURL := "ws" + strings.TrimPrefix(serverURL, "http") + "/ws"
	client := NewClient(wsURL, "test-token", "1.0.0")
	router := mcpServeTestRouter(client)
	client.SetMessageHandler(router)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	t.Cleanup(func() {
		// MCP 서버 goroutine 정리
		router.mcpServeMu.Lock()
		if router.mcpServeCancel != nil {
			router.mcpServeCancel()
		}
		router.mcpServeServer = nil
		router.mcpServeCancel = nil
		router.mcpServeMu.Unlock()

		client.Disconnect("test complete")
	})

	return client, router
}

// waitForResponse는 타임아웃 내에 응답 메시지를 수신합니다.
func waitForResponse(t *testing.T, ch <-chan ws.AgentMessage, timeout time.Duration) ws.AgentMessage {
	t.Helper()

	select {
	case msg := <-ch:
		return msg
	case <-time.After(timeout):
		t.Fatalf("timeout waiting for response (waited %v)", timeout)
		return ws.AgentMessage{} // unreachable
	}
}

// ---------------------------------------------------------------------------
// Scenario 8: 전체 MCP serve 라이프사이클
// ---------------------------------------------------------------------------

// TestIntegration_MCPServeLifecycle는 전체 MCP serve 라이프사이클을 검증합니다.
//
// 1. mcp_serve_start 전송 -> mcp_serve_ready 응답 확인 (mcp_serve_result가 아님)
// 2. 응답 페이로드의 status가 "started"인지 확인
// 3. mcp_serve_stop 전송 -> mcp_serve_result 응답, status="stopped" 확인
// 4. MCPServeStatus() == "stopped" 확인
func TestIntegration_MCPServeLifecycle(t *testing.T) {
	server, serverSend, clientResponses := setupMCPServeTestServer(t)
	defer server.Close()

	_, router := connectMCPServeTestClient(t, server.URL)

	// --- Step 1: mcp_serve_start 전송 ---
	startPayload, _ := json.Marshal(ws.MCPServeStartPayload{
		BackendURL: "https://api.test.autopus.co",
	})
	serverSend <- ws.AgentMessage{
		Type:      ws.AgentMsgMCPServeStart,
		ID:        "msg-lifecycle-start-001",
		Timestamp: time.Now(),
		Payload:   startPayload,
	}

	// --- Step 2: mcp_serve_ready 응답 확인 ---
	resp := waitForResponse(t, clientResponses, 5*time.Second)

	if resp.Type != ws.AgentMsgMCPServeReady {
		t.Errorf("mcp_serve_start 응답 타입: got %q, want %q", resp.Type, ws.AgentMsgMCPServeReady)
	}
	if resp.ID != "msg-lifecycle-start-001" {
		t.Errorf("응답 메시지 ID: got %q, want %q", resp.ID, "msg-lifecycle-start-001")
	}

	// 응답 페이로드 검증
	var readyPayload ws.MCPServeResultPayload
	if err := json.Unmarshal(resp.Payload, &readyPayload); err != nil {
		t.Fatalf("mcp_serve_ready 페이로드 파싱 실패: %v", err)
	}
	if readyPayload.Status != "started" {
		t.Errorf("mcp_serve_ready status: got %q, want %q", readyPayload.Status, "started")
	}

	// --- Step 3: mcp_serve_stop 전송 ---
	// NOTE: 테스트 환경에서 server.ServeStdio()는 즉시 반환될 수 있으므로
	// mcpServeServer가 goroutine에 의해 nil로 설정될 수 있습니다.
	// stop은 어느 경우든 mcp_serve_result 타입으로 "stopped"를 반환합니다.
	stopPayload, _ := json.Marshal(ws.MCPServeStopPayload{
		Reason: "lifecycle test stop",
	})
	serverSend <- ws.AgentMessage{
		Type:      ws.AgentMsgMCPServeStop,
		ID:        "msg-lifecycle-stop-001",
		Timestamp: time.Now(),
		Payload:   stopPayload,
	}

	// --- Step 4: mcp_serve_result 응답 확인 ---
	stopResp := waitForResponse(t, clientResponses, 5*time.Second)

	if stopResp.Type != ws.AgentMsgMCPServeResult {
		t.Errorf("mcp_serve_stop 응답 타입: got %q, want %q", stopResp.Type, ws.AgentMsgMCPServeResult)
	}
	if stopResp.ID != "msg-lifecycle-stop-001" {
		t.Errorf("응답 메시지 ID: got %q, want %q", stopResp.ID, "msg-lifecycle-stop-001")
	}

	// 응답 페이로드 검증
	var stopResultPayload ws.MCPServeResultPayload
	if err := json.Unmarshal(stopResp.Payload, &stopResultPayload); err != nil {
		t.Fatalf("mcp_serve_result 페이로드 파싱 실패: %v", err)
	}
	if stopResultPayload.Status != "stopped" {
		t.Errorf("mcp_serve_result status: got %q, want %q", stopResultPayload.Status, "stopped")
	}

	// --- Step 5: MCPServeStatus() == "stopped" 확인 ---
	status := router.MCPServeStatus()
	if status != "stopped" {
		t.Errorf("MCPServeStatus after stop: got %q, want %q", status, "stopped")
	}
}

// ---------------------------------------------------------------------------
// Scenario 8b: 이미 실행 중인 서버 중복 시작
// ---------------------------------------------------------------------------

// TestIntegration_MCPServeStartAlreadyRunning은 이미 실행 중인 서버에
// 중복 시작 요청을 보냈을 때 에러 응답(mcp_serve_result)을 반환하는지 검증합니다.
//
// 테스트 환경에서 server.ServeStdio()가 즉시 반환되어 mcpServeServer가
// goroutine에 의해 nil로 설정되는 레이스를 방지하기 위해,
// mcpServeServer를 직접 설정하여 "실행 중" 상태를 시뮬레이션합니다.
func TestIntegration_MCPServeStartAlreadyRunning(t *testing.T) {
	server, serverSend, clientResponses := setupMCPServeTestServer(t)
	defer server.Close()

	_, router := connectMCPServeTestClient(t, server.URL)

	// --- Step 1: "실행 중" 상태 시뮬레이션 ---
	// 테스트 환경에서는 실제 MCP 서버의 ServeStdio()가 즉시 반환되므로
	// 직접 mcpServeServer를 설정하여 결정론적 테스트를 수행합니다.
	router.mcpServeMu.Lock()
	router.mcpServeServer = &mcpserver.Server{} // non-nil = "running"
	router.mcpServeMu.Unlock()

	// MCPServeStatus가 "running"인지 확인
	if status := router.MCPServeStatus(); status != "running" {
		t.Fatalf("MCPServeStatus 사전 조건: got %q, want %q", status, "running")
	}

	// --- Step 2: 중복 시작 요청 전송 ---
	startPayload, _ := json.Marshal(ws.MCPServeStartPayload{
		BackendURL: "https://api.test.autopus.co",
	})
	serverSend <- ws.AgentMessage{
		Type:      ws.AgentMsgMCPServeStart,
		ID:        "msg-dup-start-001",
		Timestamp: time.Now(),
		Payload:   startPayload,
	}

	// --- Step 3: mcp_serve_result (에러) 응답 확인 ---
	dupResp := waitForResponse(t, clientResponses, 5*time.Second)

	// 핵심 검증: 이미 실행 중일 때는 mcp_serve_ready가 아닌 mcp_serve_result 타입이어야 함
	if dupResp.Type == ws.AgentMsgMCPServeReady {
		t.Errorf("이미 실행 중인 서버에 대해 mcp_serve_ready가 아닌 mcp_serve_result 타입이어야 합니다")
	}
	if dupResp.Type != ws.AgentMsgMCPServeResult {
		t.Errorf("중복 시작 응답 타입: got %q, want %q", dupResp.Type, ws.AgentMsgMCPServeResult)
	}

	var dupPayload ws.MCPServeResultPayload
	if err := json.Unmarshal(dupResp.Payload, &dupPayload); err != nil {
		t.Fatalf("중복 시작 페이로드 파싱 실패: %v", err)
	}
	if dupPayload.Status != "error" {
		t.Errorf("중복 시작 status: got %q, want %q", dupPayload.Status, "error")
	}
	if dupPayload.Error == "" {
		t.Error("중복 시작 에러 메시지가 비어 있음")
	}

	// --- Step 4: 상태 확인 (여전히 running) ---
	if status := router.MCPServeStatus(); status != "running" {
		t.Errorf("중복 시작 후 MCPServeStatus: got %q, want %q", status, "running")
	}

	// --- Step 5: 정상 종료 ---
	stopPayload, _ := json.Marshal(ws.MCPServeStopPayload{
		Reason: "cleanup after duplicate start test",
	})
	serverSend <- ws.AgentMessage{
		Type:      ws.AgentMsgMCPServeStop,
		ID:        "msg-dup-stop-001",
		Timestamp: time.Now(),
		Payload:   stopPayload,
	}

	stopResp := waitForResponse(t, clientResponses, 5*time.Second)
	if stopResp.Type != ws.AgentMsgMCPServeResult {
		t.Errorf("종료 응답 타입: got %q, want %q", stopResp.Type, ws.AgentMsgMCPServeResult)
	}

	var stopResultPayload ws.MCPServeResultPayload
	if err := json.Unmarshal(stopResp.Payload, &stopResultPayload); err != nil {
		t.Fatalf("종료 페이로드 파싱 실패: %v", err)
	}
	if stopResultPayload.Status != "stopped" {
		t.Errorf("종료 status: got %q, want %q", stopResultPayload.Status, "stopped")
	}

	// 종료 후 상태 확인
	if status := router.MCPServeStatus(); status != "stopped" {
		t.Errorf("종료 후 MCPServeStatus: got %q, want %q", status, "stopped")
	}
}

// ---------------------------------------------------------------------------
// Scenario 8c: 실행 중이 아닌 서버 중지
// ---------------------------------------------------------------------------

// TestIntegration_MCPServeStopWithoutStart는 MCP 서버가 실행되지 않은 상태에서
// mcp_serve_stop을 보냈을 때 "stopped" 상태의 mcp_serve_result를 반환하는지 검증합니다.
func TestIntegration_MCPServeStopWithoutStart(t *testing.T) {
	server, serverSend, clientResponses := setupMCPServeTestServer(t)
	defer server.Close()

	_, router := connectMCPServeTestClient(t, server.URL)

	// MCPServeStatus가 "stopped"인지 사전 확인
	if status := router.MCPServeStatus(); status != "stopped" {
		t.Fatalf("초기 MCPServeStatus: got %q, want %q", status, "stopped")
	}

	// mcp_serve_stop 전송 (시작 없이)
	stopPayload, _ := json.Marshal(ws.MCPServeStopPayload{
		Reason: "stop without start test",
	})
	serverSend <- ws.AgentMessage{
		Type:      ws.AgentMsgMCPServeStop,
		ID:        "msg-nostart-stop-001",
		Timestamp: time.Now(),
		Payload:   stopPayload,
	}

	// mcp_serve_result 응답 확인 (graceful)
	resp := waitForResponse(t, clientResponses, 5*time.Second)
	if resp.Type != ws.AgentMsgMCPServeResult {
		t.Errorf("시작 없이 중지 응답 타입: got %q, want %q", resp.Type, ws.AgentMsgMCPServeResult)
	}

	var resultPayload ws.MCPServeResultPayload
	if err := json.Unmarshal(resp.Payload, &resultPayload); err != nil {
		t.Fatalf("응답 페이로드 파싱 실패: %v", err)
	}

	// status가 "stopped"이어야 함 (에러가 아닌 graceful 응답)
	if resultPayload.Status != "stopped" {
		t.Errorf("시작 없이 중지 status: got %q, want %q", resultPayload.Status, "stopped")
	}

	// 에러 필드는 비어 있어야 함 (정상적인 "이미 중지" 응답)
	if resultPayload.Error != "" {
		t.Errorf("시작 없이 중지 error 필드가 비어 있지 않음: %q", resultPayload.Error)
	}

	// MCPServeStatus가 여전히 "stopped"인지 확인
	if status := router.MCPServeStatus(); status != "stopped" {
		t.Errorf("시작 없이 중지 후 MCPServeStatus: got %q, want %q", status, "stopped")
	}
}

// ---------------------------------------------------------------------------
// Scenario 15: 기존 핸들러 회귀 검증 (확장)
// ---------------------------------------------------------------------------

// TestIntegration_MCPServeHandlerRegistryRegression은 MCP serve 핸들러 추가로 인해
// 기존 핸들러가 영향받지 않았는지 확인하는 회귀 테스트입니다.
// handler_mcpserve_test.go의 TestExistingHandlersStillRegistered를 보완합니다.
func TestIntegration_MCPServeHandlerRegistryRegression(t *testing.T) {
	client := NewClient("ws://localhost:9999/ws", "test-token", "1.0.0")
	router := NewRouter(client)

	// 기존 핸들러 (SPEC-AI-003 M6 이전부터 존재)
	legacyHandlers := []string{
		ws.AgentMsgHeartbeat,
		ws.AgentMsgTaskReq,
		ws.AgentMsgBuildReq,
		ws.AgentMsgTestReq,
		ws.AgentMsgQAReq,
		ws.AgentMsgCLIRequest,
		ws.AgentMsgMCPStart,
		ws.AgentMsgMCPStop,
		ws.AgentMsgComputerSessionStart,
		ws.AgentMsgComputerAction,
		ws.AgentMsgComputerSessionEnd,
		ws.AgentMsgMCPCodegenRequest,
		ws.AgentMsgMCPDeploy,
	}

	// MCP serve 핸들러 (SPEC-AI-003 M3)
	mcpServeHandlers := []string{
		ws.AgentMsgMCPServeStart,
		ws.AgentMsgMCPServeStop,
	}

	router.handlersMu.RLock()
	defer router.handlersMu.RUnlock()

	// 기존 핸들러 검증
	for _, msgType := range legacyHandlers {
		if _, exists := router.handlers[msgType]; !exists {
			t.Errorf("기존 핸들러 %q 가 등록되어 있지 않음 (회귀)", msgType)
		}
	}

	// MCP serve 핸들러 검증
	for _, msgType := range mcpServeHandlers {
		if _, exists := router.handlers[msgType]; !exists {
			t.Errorf("MCP serve 핸들러 %q 가 등록되어 있지 않음", msgType)
		}
	}

	// 전체 핸들러 수 검증 (핸들러가 예상치 않게 제거되지 않았는지)
	expectedCount := len(legacyHandlers) + len(mcpServeHandlers)
	actualCount := len(router.handlers)
	if actualCount < expectedCount {
		t.Errorf("등록된 핸들러 수: got %d, want at least %d", actualCount, expectedCount)
	}
}

// ---------------------------------------------------------------------------
// 응답 메시지 타입 구분 검증
// ---------------------------------------------------------------------------

// TestIntegration_MCPServeResponseTypeDistinction은 시작 성공/에러/중지 각 시나리오별
// 응답 메시지 타입이 올바르게 구분되는지 검증합니다.
//
// - mcp_serve_start 성공 -> mcp_serve_ready (NOT mcp_serve_result)
// - mcp_serve_start 에러 (이미 실행 중) -> mcp_serve_result (NOT mcp_serve_ready)
// - mcp_serve_stop -> mcp_serve_result (항상)
func TestIntegration_MCPServeResponseTypeDistinction(t *testing.T) {
	t.Run("start success returns mcp_serve_ready", func(t *testing.T) {
		server, serverSend, clientResponses := setupMCPServeTestServer(t)
		defer server.Close()

		_, _ = connectMCPServeTestClient(t, server.URL)

		startPayload, _ := json.Marshal(ws.MCPServeStartPayload{
			BackendURL: "https://api.test.autopus.co",
		})
		serverSend <- ws.AgentMessage{
			Type:      ws.AgentMsgMCPServeStart,
			ID:        "msg-type-start-ok",
			Timestamp: time.Now(),
			Payload:   startPayload,
		}

		resp := waitForResponse(t, clientResponses, 5*time.Second)

		// 핵심 검증: 성공 시 mcp_serve_ready 타입이어야 함
		if resp.Type == ws.AgentMsgMCPServeResult {
			t.Errorf("mcp_serve_start 성공 시 mcp_serve_result가 아닌 mcp_serve_ready 타입이어야 합니다")
		}
		if resp.Type != ws.AgentMsgMCPServeReady {
			t.Errorf("응답 타입: got %q, want %q", resp.Type, ws.AgentMsgMCPServeReady)
		}
	})

	t.Run("duplicate start returns mcp_serve_result with error", func(t *testing.T) {
		server, serverSend, clientResponses := setupMCPServeTestServer(t)
		defer server.Close()

		_, router := connectMCPServeTestClient(t, server.URL)

		// "이미 실행 중" 상태를 결정론적으로 시뮬레이션
		router.mcpServeMu.Lock()
		router.mcpServeServer = &mcpserver.Server{}
		router.mcpServeMu.Unlock()

		startPayload, _ := json.Marshal(ws.MCPServeStartPayload{
			BackendURL: "https://api.test.autopus.co",
		})
		serverSend <- ws.AgentMessage{
			Type:      ws.AgentMsgMCPServeStart,
			ID:        "msg-type-start-dup",
			Timestamp: time.Now(),
			Payload:   startPayload,
		}

		resp := waitForResponse(t, clientResponses, 5*time.Second)

		// 핵심 검증: 에러 시 mcp_serve_result 타입이어야 함
		if resp.Type == ws.AgentMsgMCPServeReady {
			t.Errorf("이미 실행 중일 때 mcp_serve_ready가 아닌 mcp_serve_result 타입이어야 합니다")
		}
		if resp.Type != ws.AgentMsgMCPServeResult {
			t.Errorf("응답 타입: got %q, want %q", resp.Type, ws.AgentMsgMCPServeResult)
		}
	})

	t.Run("stop always returns mcp_serve_result", func(t *testing.T) {
		server, serverSend, clientResponses := setupMCPServeTestServer(t)
		defer server.Close()

		_, _ = connectMCPServeTestClient(t, server.URL)

		stopPayload, _ := json.Marshal(ws.MCPServeStopPayload{
			Reason: "type check stop",
		})
		serverSend <- ws.AgentMessage{
			Type:      ws.AgentMsgMCPServeStop,
			ID:        "msg-type-stop",
			Timestamp: time.Now(),
			Payload:   stopPayload,
		}

		resp := waitForResponse(t, clientResponses, 5*time.Second)

		// 핵심 검증: stop 시 항상 mcp_serve_result 타입이어야 함
		if resp.Type != ws.AgentMsgMCPServeResult {
			t.Errorf("응답 타입: got %q, want %q", resp.Type, ws.AgentMsgMCPServeResult)
		}
	})
}
