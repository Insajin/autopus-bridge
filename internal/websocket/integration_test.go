//go:build integration

// Package websocket provides integration tests for WebSocket communication.
// NFR-05: Integration tests for message round-trips, HMAC verification,
// and task tracking with reconnect scenarios.
package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/insajin/autopus-agent-protocol"
)

// upgrader is a shared WebSocket upgrader for mock servers.
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// TestIntegration_FullMessageRoundTrip verifies a complete message round-trip
// between client and mock server.
func TestIntegration_FullMessageRoundTrip(t *testing.T) {
	// Create a mock WebSocket server.
	serverReceived := make(chan ws.AgentMessage, 10)
	serverDone := make(chan struct{})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Logf("upgrade error: %v", err)
			return
		}
		defer conn.Close()

		// Read the agent_connect message.
		_, data, err := conn.ReadMessage()
		if err != nil {
			t.Logf("read connect error: %v", err)
			return
		}

		var connectMsg ws.AgentMessage
		if err := json.Unmarshal(data, &connectMsg); err != nil {
			t.Logf("unmarshal connect error: %v", err)
			return
		}

		if connectMsg.Type != ws.AgentMsgConnect {
			t.Logf("expected agent_connect, got %s", connectMsg.Type)
			return
		}

		// Send connect_ack.
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
			t.Logf("write ack error: %v", err)
			return
		}

		// Read subsequent messages and forward to channel.
		for {
			_, msgData, err := conn.ReadMessage()
			if err != nil {
				close(serverDone)
				return
			}
			var msg ws.AgentMessage
			if err := json.Unmarshal(msgData, &msg); err != nil {
				continue
			}
			serverReceived <- msg
		}
	}))
	defer server.Close()

	// Convert HTTP URL to WebSocket URL.
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"

	// Create client and connect.
	client := NewClient(wsURL, "test-token", "1.0.0")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer client.Disconnect("test complete")

	// Verify connected state.
	if client.State() != StateConnected {
		t.Errorf("state = %v, want Connected", client.State())
	}

	// Send a task result message.
	resultPayload, _ := json.Marshal(ws.TaskResultPayload{
		ExecutionID: "exec-roundtrip-001",
		Output:      "round trip test output",
		ExitCode:    0,
		Duration:    100,
	})

	if err := client.SendTaskResult(ws.TaskResultPayload{
		ExecutionID: "exec-roundtrip-001",
		Output:      "round trip test output",
		ExitCode:    0,
		Duration:    100,
	}); err != nil {
		t.Fatalf("SendTaskResult failed: %v", err)
	}
	_ = resultPayload

	// Wait for server to receive the message.
	select {
	case msg := <-serverReceived:
		if msg.Type != ws.AgentMsgTaskResult {
			t.Errorf("server received type = %q, want %q", msg.Type, ws.AgentMsgTaskResult)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for server to receive message")
	}
}

// TestIntegration_HMACRoundTrip verifies that HMAC signing and verification
// work correctly in a full round-trip scenario.
func TestIntegration_HMACRoundTrip(t *testing.T) {
	secret := []byte("integration-test-hmac-secret-32b")

	// Create signer pair (client-side and server-side use same secret).
	clientSigner := NewMessageSigner()
	clientSigner.SetSecret(secret)

	serverSigner := NewMessageSigner()
	serverSigner.SetSecret(secret)

	// Simulate client signing a message.
	payload, _ := json.Marshal(ws.TaskResultPayload{
		ExecutionID: "exec-hmac-001",
		Output:      "HMAC round-trip test",
		ExitCode:    0,
	})

	msg := &ws.AgentMessage{
		Type:      ws.AgentMsgTaskResult,
		ID:        "msg-hmac-001",
		Timestamp: time.Now(),
		Payload:   payload,
	}

	// Client signs.
	if err := clientSigner.Sign(msg); err != nil {
		t.Fatalf("client Sign failed: %v", err)
	}
	if msg.Signature == "" {
		t.Fatal("signature is empty after signing")
	}

	// Serialize and deserialize (simulates network transmission).
	wire, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var received ws.AgentMessage
	if err := json.Unmarshal(wire, &received); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	// Server verifies.
	if !serverSigner.Verify(&received) {
		t.Fatal("server verification failed for valid signed message")
	}

	// Tamper with the message and verify rejection.
	tamperedPayload, _ := json.Marshal(ws.TaskResultPayload{
		ExecutionID: "exec-hmac-001",
		Output:      "TAMPERED output",
		ExitCode:    0,
	})
	received.Payload = tamperedPayload

	if serverSigner.Verify(&received) {
		t.Fatal("server verification passed for tampered message")
	}
}

// TestIntegration_TaskTrackingWithReconnect verifies that the task tracker
// correctly maintains state across a simulated reconnect scenario.
func TestIntegration_TaskTrackingWithReconnect(t *testing.T) {
	tracker := NewTaskTracker()

	// Simulate pre-reconnect state: several active tasks.
	taskIDs := []string{"exec-001", "exec-002", "exec-003", "exec-004", "exec-005"}
	taskTypes := []string{"task", "build", "test", "qa", "task"}

	for i, id := range taskIDs {
		tracker.Track(id, taskTypes[i])
	}

	if count := tracker.GetActiveTaskCount(); count != 5 {
		t.Fatalf("pre-reconnect active count = %d, want 5", count)
	}

	// Simulate reconnect: some tasks completed on server side.
	completedOnServer := []string{"exec-001", "exec-003"}
	for _, id := range completedOnServer {
		tracker.Complete(id)
	}

	// Verify remaining active tasks.
	if count := tracker.GetActiveTaskCount(); count != 3 {
		t.Errorf("post-reconnect active count = %d, want 3", count)
	}

	// Verify specific tasks.
	if tracker.IsActive("exec-001") {
		t.Error("exec-001 should be inactive after completion")
	}
	if !tracker.IsActive("exec-002") {
		t.Error("exec-002 should still be active")
	}
	if tracker.IsActive("exec-003") {
		t.Error("exec-003 should be inactive after completion")
	}
	if !tracker.IsActive("exec-004") {
		t.Error("exec-004 should still be active")
	}
	if !tracker.IsActive("exec-005") {
		t.Error("exec-005 should still be active")
	}

	// Verify GetActiveTasks returns correct set.
	activeTasks := tracker.GetActiveTasks()
	activeSet := make(map[string]bool)
	for _, id := range activeTasks {
		activeSet[id] = true
	}

	expectedActive := []string{"exec-002", "exec-004", "exec-005"}
	for _, id := range expectedActive {
		if !activeSet[id] {
			t.Errorf("%s should be in active tasks list", id)
		}
	}
}

// TestIntegration_ConcurrentTaskTrackingWithReconnect simulates concurrent
// task operations during a reconnect scenario.
func TestIntegration_ConcurrentTaskTrackingWithReconnect(t *testing.T) {
	tracker := NewTaskTracker()

	var wg sync.WaitGroup

	// Phase 1: Rapidly add tasks.
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			tracker.Track(fmt.Sprintf("reconnect-exec-%03d", id), "task")
		}(i)
	}
	wg.Wait()

	if count := tracker.GetActiveTaskCount(); count != 100 {
		t.Fatalf("after tracking, active count = %d, want 100", count)
	}

	// Phase 2: Simulate reconnect - concurrent completions and new trackings.
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			if id < 50 {
				// Complete existing tasks.
				tracker.Complete(fmt.Sprintf("reconnect-exec-%03d", id))
			} else {
				// Add new tasks (simulating tasks received after reconnect).
				tracker.Track(fmt.Sprintf("reconnect-new-%03d", id), "task")
			}
		}(i)
	}
	wg.Wait()

	// Phase 3: Concurrent reads during stabilization.
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = tracker.GetActiveTasks()
			_ = tracker.GetActiveTaskCount()
		}()
	}
	wg.Wait()

	// We should have: 100 original - 50 completed + 50 new = 100 active tasks.
	if count := tracker.GetActiveTaskCount(); count != 100 {
		t.Errorf("after reconnect simulation, active count = %d, want 100", count)
	}
}

// TestIntegration_ServerSendTaskRequest verifies that a mock server can send
// a task_request and the client router handles it.
func TestIntegration_ServerSendTaskRequest(t *testing.T) {
	taskHandled := make(chan string, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// Read agent_connect.
		_, _, err = conn.ReadMessage()
		if err != nil {
			return
		}

		// Send connect_ack.
		ackPayload, _ := json.Marshal(ws.ConnectAckPayload{
			Success: true,
		})
		ackMsg := ws.AgentMessage{
			Type:      ws.AgentMsgConnectAck,
			ID:        "ack-001",
			Timestamp: time.Now(),
			Payload:   ackPayload,
		}
		ackData, _ := json.Marshal(ackMsg)
		_ = conn.WriteMessage(websocket.TextMessage, ackData)

		// Send a task_request to the client.
		taskPayload, _ := json.Marshal(ws.TaskRequestPayload{
			ExecutionID: "server-task-001",
			Prompt:      "integration test prompt",
			Model:       "claude-sonnet",
			MaxTokens:   100,
			Timeout:     60,
		})
		taskMsg := ws.AgentMessage{
			Type:      ws.AgentMsgTaskReq,
			ID:        "task-msg-001",
			Timestamp: time.Now(),
			Payload:   taskPayload,
		}
		taskData, _ := json.Marshal(taskMsg)
		_ = conn.WriteMessage(websocket.TextMessage, taskData)

		// Keep connection open to receive response.
		for {
			_, msgData, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var msg ws.AgentMessage
			if err := json.Unmarshal(msgData, &msg); err != nil {
				continue
			}
			// We expect a task_error since there is no real executor.
			if msg.Type == ws.AgentMsgTaskError {
				var errPayload ws.TaskErrorPayload
				if err := json.Unmarshal(msg.Payload, &errPayload); err == nil {
					taskHandled <- errPayload.ExecutionID
				}
			}
		}
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	client := NewClient(wsURL, "test-token", "1.0.0")

	// Set up router with no executor (will produce error response).
	router := NewRouter(client)
	client.SetMessageHandler(router)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer client.Disconnect("test complete")

	// Wait for the task to be handled.
	select {
	case execID := <-taskHandled:
		if execID != "server-task-001" {
			t.Errorf("handled execution_id = %q, want %q", execID, "server-task-001")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for task to be handled")
	}
}
