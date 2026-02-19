package websocket

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	gorillaWs "github.com/gorilla/websocket"
	"github.com/insajin/autopus-agent-protocol"
	"github.com/insajin/autopus-bridge/internal/computeruse"
)

// REQ-M3-04: Tests for session state restoration on WebSocket reconnection.

// TestRouter_OnReconnected_NoHandler verifies OnReconnected is a no-op when
// no Computer Use handler is configured.
func TestRouter_OnReconnected_NoHandler(t *testing.T) {
	client := NewClient("ws://localhost:9999", "tok", "1.0")
	router := NewRouter(client)

	err := router.OnReconnected(context.Background())
	if err != nil {
		t.Errorf("OnReconnected() without handler = error %v; want nil", err)
	}
}

// TestRouter_OnReconnected_NoActiveSessions verifies OnReconnected is a no-op
// when the handler has no active sessions.
func TestRouter_OnReconnected_NoActiveSessions(t *testing.T) {
	client := NewClient("ws://localhost:9999", "tok", "1.0")
	handler := computeruse.NewHandler()
	router := NewRouter(client, WithComputerUseHandler(handler))

	err := router.OnReconnected(context.Background())
	if err != nil {
		t.Errorf("OnReconnected() with no sessions = error %v; want nil", err)
	}
}

// TestRouter_OnReconnected_ContextCancelled verifies OnReconnected returns
// early when the context is cancelled.
func TestRouter_OnReconnected_ContextCancelled(t *testing.T) {
	client := NewClient("ws://localhost:9999", "tok", "1.0")
	handler := computeruse.NewHandler()
	router := NewRouter(client, WithComputerUseHandler(handler))

	// Create a session so OnReconnected has work to do.
	sm := handler.SessionManager()
	_, _ = sm.CreateSession("exec-1", "sess-1", 1280, 720, true, "http://example.com")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	err := router.OnReconnected(ctx)
	if err != context.Canceled {
		t.Errorf("OnReconnected() with cancelled ctx = %v; want context.Canceled", err)
	}
}

// TestRouter_OnReconnected_SendsSessionAndPendingResults verifies that
// OnReconnected sends session restoration messages and pending results
// over a real WebSocket connection.
func TestRouter_OnReconnected_SendsSessionAndPendingResults(t *testing.T) {
	// Collect all messages sent by the client.
	received := make(chan ws.AgentMessage, 20)
	serverDone := make(chan struct{})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := gorillaWs.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Logf("upgrade error: %v", err)
			return
		}
		defer func() { _ = conn.Close() }()

		// Read agent_connect and send ack.
		_, data, err := conn.ReadMessage()
		if err != nil {
			t.Logf("read connect error: %v", err)
			return
		}
		var connectMsg ws.AgentMessage
		_ = json.Unmarshal(data, &connectMsg)

		ackPayload, _ := json.Marshal(ws.ConnectAckPayload{
			Success: true,
			Message: "ok",
		})
		ackMsg := ws.AgentMessage{
			Type:      ws.AgentMsgConnectAck,
			ID:        "ack-1",
			Timestamp: time.Now(),
			Payload:   ackPayload,
		}
		ackData, _ := json.Marshal(ackMsg)
		_ = conn.WriteMessage(gorillaWs.TextMessage, ackData)

		// Read subsequent messages (session restore + pending results).
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				close(serverDone)
				return
			}
			var msg ws.AgentMessage
			if err := json.Unmarshal(data, &msg); err != nil {
				continue
			}
			received <- msg
		}
	}))
	defer server.Close()

	wsURL := "ws" + server.URL[4:] // http -> ws
	client := NewClient(wsURL, "test-token", "1.0")

	// Connect the client.
	ctx := context.Background()
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect() = error %v", err)
	}
	defer func() { _ = client.Disconnect("test done") }()

	// Set up Computer Use handler with an active session and pending results.
	cuHandler := computeruse.NewHandler()
	sm := cuHandler.SessionManager()
	session, err := sm.CreateSession("exec-42", "sess-reconnect", 1920, 1080, false, "http://example.com/app")
	if err != nil {
		t.Fatalf("CreateSession() = error %v", err)
	}

	// Queue pending results on the session.
	pendingResult := ws.ComputerResultPayload{
		ExecutionID: "exec-42",
		SessionID:   "sess-reconnect",
		Success:     true,
		Screenshot:  "base64data",
		DurationMs:  150,
	}
	session.QueueResult(pendingResult)

	router := NewRouter(client, WithComputerUseHandler(cuHandler))

	// Execute OnReconnected.
	if err := router.OnReconnected(ctx); err != nil {
		t.Fatalf("OnReconnected() = error %v", err)
	}

	// Wait for messages to arrive at the server.
	// We expect:
	// 1. computer_session_start (session restore)
	// 2. computer_result (pending result resend)
	var sessionMsg, resultMsg ws.AgentMessage
	timeout := time.After(5 * time.Second)

	for i := 0; i < 2; i++ {
		select {
		case msg := <-received:
			switch msg.Type {
			case ws.AgentMsgComputerSessionStart:
				sessionMsg = msg
			case ws.AgentMsgComputerResult:
				resultMsg = msg
			default:
				t.Logf("unexpected message type: %s", msg.Type)
			}
		case <-timeout:
			t.Fatal("timed out waiting for messages from OnReconnected")
		}
	}

	// Verify session restore message.
	if sessionMsg.Type != ws.AgentMsgComputerSessionStart {
		t.Errorf("session restore msg type = %q; want %q", sessionMsg.Type, ws.AgentMsgComputerSessionStart)
	}
	var sessionPayload ws.ComputerSessionPayload
	if err := json.Unmarshal(sessionMsg.Payload, &sessionPayload); err != nil {
		t.Fatalf("unmarshal session payload: %v", err)
	}
	if sessionPayload.SessionID != "sess-reconnect" {
		t.Errorf("session payload SessionID = %q; want %q", sessionPayload.SessionID, "sess-reconnect")
	}
	if sessionPayload.ExecutionID != "exec-42" {
		t.Errorf("session payload ExecutionID = %q; want %q", sessionPayload.ExecutionID, "exec-42")
	}
	if sessionPayload.ViewportW != 1920 || sessionPayload.ViewportH != 1080 {
		t.Errorf("session payload viewport = %dx%d; want 1920x1080", sessionPayload.ViewportW, sessionPayload.ViewportH)
	}
	if sessionPayload.URL != "http://example.com/app" {
		t.Errorf("session payload URL = %q; want %q", sessionPayload.URL, "http://example.com/app")
	}

	// Verify pending result message.
	if resultMsg.Type != ws.AgentMsgComputerResult {
		t.Errorf("result msg type = %q; want %q", resultMsg.Type, ws.AgentMsgComputerResult)
	}
	var resultPayload ws.ComputerResultPayload
	if err := json.Unmarshal(resultMsg.Payload, &resultPayload); err != nil {
		t.Fatalf("unmarshal result payload: %v", err)
	}
	if resultPayload.SessionID != "sess-reconnect" {
		t.Errorf("result payload SessionID = %q; want %q", resultPayload.SessionID, "sess-reconnect")
	}
	if resultPayload.ExecutionID != "exec-42" {
		t.Errorf("result payload ExecutionID = %q; want %q", resultPayload.ExecutionID, "exec-42")
	}
	if !resultPayload.Success {
		t.Error("result payload Success = false; want true")
	}
	if resultPayload.Screenshot != "base64data" {
		t.Errorf("result payload Screenshot = %q; want %q", resultPayload.Screenshot, "base64data")
	}

	// Verify pending results were drained from the session.
	if session.PendingResultCount() != 0 {
		t.Errorf("session pending count after OnReconnected = %d; want 0", session.PendingResultCount())
	}
}

// TestRouter_ImplementsReconnectionHandler verifies that *Router satisfies
// the ReconnectionHandler interface.
func TestRouter_ImplementsReconnectionHandler(t *testing.T) {
	client := NewClient("ws://localhost:9999", "tok", "1.0")
	router := NewRouter(client)

	// Compile-time check: Router must implement ReconnectionHandler.
	var _ ReconnectionHandler = router
}
