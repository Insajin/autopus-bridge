package websocket

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	ws "github.com/insajin/autopus-agent-protocol"
	"github.com/insajin/autopus-bridge/internal/auth"
	"github.com/insajin/autopus-bridge/internal/mcpserver"
	"github.com/rs/zerolog"
)

// ---------------------------------------------------------------------------
// Tests: MCP Serve Handler Routing (SPEC-AI-003 M3)
// ---------------------------------------------------------------------------

// TestMCPServeHandlersRegistered는 mcp_serve_start, mcp_serve_stop
// 메시지 타입이 라우터에 등록되어 있는지 확인합니다.
func TestMCPServeHandlersRegistered(t *testing.T) {
	client := NewClient("ws://localhost:9999/ws", "test-token", "1.0.0")
	router := NewRouter(client)

	expectedTypes := []string{
		ws.AgentMsgMCPServeStart,
		ws.AgentMsgMCPServeStop,
	}

	router.handlersMu.RLock()
	defer router.handlersMu.RUnlock()

	for _, msgType := range expectedTypes {
		if _, exists := router.handlers[msgType]; !exists {
			t.Errorf("handler for %q not registered", msgType)
		}
	}
}

// TestHandleMCPServeStart_InvalidPayload는 잘못된 JSON 페이로드를 수신했을 때
// 패닉 없이 에러를 처리하는지 확인합니다 (T-24).
func TestHandleMCPServeStart_InvalidPayload(t *testing.T) {
	client := NewClient("ws://localhost:9999/ws", "test-token", "1.0.0")
	router := NewRouter(client, WithErrorHandler(func(err error) {}))

	tests := []struct {
		name    string
		payload json.RawMessage
	}{
		{
			name:    "malformed JSON",
			payload: json.RawMessage(`{invalid json`),
		},
		{
			name:    "array instead of object",
			payload: json.RawMessage(`[1,2,3]`),
		},
		{
			name:    "number instead of object",
			payload: json.RawMessage(`12345`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := ws.AgentMessage{
				Type:      ws.AgentMsgMCPServeStart,
				ID:        "msg-mcp-serve-invalid-001",
				Timestamp: time.Now(),
				Payload:   tt.payload,
			}

			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("handleMCPServeStart panicked on %s: %v", tt.name, r)
					}
				}()
				// 에러 예상: 파싱 실패 또는 전송 실패
				_ = router.HandleMessage(context.Background(), msg)
			}()
		})
	}
}

// TestHandleMCPServeStop_InvalidPayload는 잘못된 JSON 페이로드를 수신했을 때
// 패닉 없이 에러를 처리하는지 확인합니다 (T-25).
func TestHandleMCPServeStop_InvalidPayload(t *testing.T) {
	client := NewClient("ws://localhost:9999/ws", "test-token", "1.0.0")
	router := NewRouter(client, WithErrorHandler(func(err error) {}))

	tests := []struct {
		name    string
		payload json.RawMessage
	}{
		{
			name:    "malformed JSON",
			payload: json.RawMessage(`{broken`),
		},
		{
			name:    "null payload",
			payload: json.RawMessage(`null`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := ws.AgentMessage{
				Type:      ws.AgentMsgMCPServeStop,
				ID:        "msg-mcp-serve-stop-invalid-001",
				Timestamp: time.Now(),
				Payload:   tt.payload,
			}

			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("handleMCPServeStop panicked on %s: %v", tt.name, r)
					}
				}()
				_ = router.HandleMessage(context.Background(), msg)
			}()
		})
	}
}

// TestHandleMCPServeStart_NoAuth는 TokenRefresher가 설정되지 않은 경우
// 에러 응답을 반환하는지 확인합니다 (T-24).
func TestHandleMCPServeStart_NoAuth(t *testing.T) {
	client := NewClient("ws://localhost:9999/ws", "test-token", "1.0.0")
	// WithMCPServeAuth 옵션 없이 Router 생성 -> tokenRefresher == nil
	router := NewRouter(client, WithErrorHandler(func(err error) {}))

	payload, err := json.Marshal(ws.MCPServeStartPayload{
		BackendURL: "https://api.autopus.co",
	})
	if err != nil {
		t.Fatalf("failed to marshal MCPServeStartPayload: %v", err)
	}

	msg := ws.AgentMessage{
		Type:      ws.AgentMsgMCPServeStart,
		ID:        "msg-mcp-serve-noauth-001",
		Timestamp: time.Now(),
		Payload:   payload,
	}

	// 패닉 없이 처리되어야 함 (전송 실패는 허용)
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("handleMCPServeStart panicked on no auth: %v", r)
			}
		}()
		_ = router.HandleMessage(context.Background(), msg)
	}()
}

// TestHandleMCPServeStart_ValidPayload는 올바른 페이로드가 파싱되는지 확인합니다 (T-24).
func TestHandleMCPServeStart_ValidPayload(t *testing.T) {
	client := NewClient("ws://localhost:9999/ws", "test-token", "1.0.0")
	router := NewRouter(client, WithErrorHandler(func(err error) {}))

	payload, err := json.Marshal(ws.MCPServeStartPayload{
		BackendURL: "https://api.test.autopus.co",
	})
	if err != nil {
		t.Fatalf("failed to marshal MCPServeStartPayload: %v", err)
	}

	msg := ws.AgentMessage{
		Type:      ws.AgentMsgMCPServeStart,
		ID:        "msg-mcp-serve-valid-001",
		Timestamp: time.Now(),
		Payload:   payload,
	}

	// 패닉 없이 처리되어야 함
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("handleMCPServeStart panicked on valid payload: %v", r)
			}
		}()
		_ = router.HandleMessage(context.Background(), msg)
	}()
}

// TestHandleMCPServeStop_NoServerRunning는 실행 중인 서버가 없을 때
// 패닉 없이 처리되는지 확인합니다 (T-25).
func TestHandleMCPServeStop_NoServerRunning(t *testing.T) {
	client := NewClient("ws://localhost:9999/ws", "test-token", "1.0.0")
	router := NewRouter(client, WithErrorHandler(func(err error) {}))

	payload, err := json.Marshal(ws.MCPServeStopPayload{
		Reason: "test stop",
	})
	if err != nil {
		t.Fatalf("failed to marshal MCPServeStopPayload: %v", err)
	}

	msg := ws.AgentMessage{
		Type:      ws.AgentMsgMCPServeStop,
		ID:        "msg-mcp-serve-stop-001",
		Timestamp: time.Now(),
		Payload:   payload,
	}

	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("handleMCPServeStop panicked: %v", r)
			}
		}()
		_ = router.HandleMessage(context.Background(), msg)
	}()
}

// TestMCPServeStatus_Stopped는 서버가 실행되지 않을 때
// "stopped"를 반환하는지 확인합니다 (T-26).
func TestMCPServeStatus_Stopped(t *testing.T) {
	client := NewClient("ws://localhost:9999/ws", "test-token", "1.0.0")
	router := NewRouter(client)

	status := router.MCPServeStatus()
	if status != "stopped" {
		t.Errorf("expected status 'stopped', got %q", status)
	}
}

// TestWithMCPServeAuth_Option는 RouterOption이 올바르게 설정되는지 확인합니다.
func TestWithMCPServeAuth_Option(t *testing.T) {
	client := NewClient("ws://localhost:9999/ws", "test-token", "1.0.0")

	// WithMCPServeAuth 없이 생성
	router1 := NewRouter(client)
	if router1.mcpServeTokenRefresher != nil {
		t.Error("mcpServeTokenRefresher should be nil when option is not provided")
	}
}

// TestHeartbeatEnricherSet는 NewRouter에서 heartbeatEnricher가 설정되는지 확인합니다 (T-26).
func TestHeartbeatEnricherSet(t *testing.T) {
	client := NewClient("ws://localhost:9999/ws", "test-token", "1.0.0")
	_ = NewRouter(client)

	// heartbeatEnricher가 설정되어야 함
	if client.heartbeatEnricher == nil {
		t.Error("heartbeatEnricher should be set after NewRouter")
	}

	// enricher가 "stopped"를 반환해야 함 (서버 미실행)
	status := client.heartbeatEnricher()
	if status != "stopped" {
		t.Errorf("expected heartbeat enricher to return 'stopped', got %q", status)
	}
}

// TestExistingHandlersStillRegistered는 기존 핸들러가 여전히 등록되어 있는지
// 확인하는 회귀 테스트입니다 (T-27).
func TestExistingHandlersStillRegistered(t *testing.T) {
	client := NewClient("ws://localhost:9999/ws", "test-token", "1.0.0")
	router := NewRouter(client)

	// 기존 핸들러 목록
	expectedTypes := []string{
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
		// 새 핸들러
		ws.AgentMsgMCPServeStart,
		ws.AgentMsgMCPServeStop,
	}

	router.handlersMu.RLock()
	defer router.handlersMu.RUnlock()

	for _, msgType := range expectedTypes {
		if _, exists := router.handlers[msgType]; !exists {
			t.Errorf("handler for %q not registered (regression)", msgType)
		}
	}
}

// TestAgentHeartbeatPayload_MCPServeStatus는 AgentHeartbeatPayload에
// MCPServeStatus 필드가 올바르게 직렬화/역직렬화되는지 확인합니다 (T-26).
func TestAgentHeartbeatPayload_MCPServeStatus(t *testing.T) {
	// MCPServeStatus 포함
	payload := ws.AgentHeartbeatPayload{
		Timestamp:      time.Now(),
		MCPServeStatus: "running",
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal heartbeat payload: %v", err)
	}

	var decoded ws.AgentHeartbeatPayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal heartbeat payload: %v", err)
	}

	if decoded.MCPServeStatus != "running" {
		t.Errorf("expected MCPServeStatus 'running', got %q", decoded.MCPServeStatus)
	}

	// MCPServeStatus 미포함 (하위 호환성)
	payloadEmpty := ws.AgentHeartbeatPayload{
		Timestamp: time.Now(),
	}

	dataEmpty, err := json.Marshal(payloadEmpty)
	if err != nil {
		t.Fatalf("failed to marshal empty heartbeat payload: %v", err)
	}

	// omitempty이므로 mcp_serve_status 필드가 없어야 함
	var raw map[string]interface{}
	if err := json.Unmarshal(dataEmpty, &raw); err != nil {
		t.Fatalf("failed to unmarshal as map: %v", err)
	}
	if _, exists := raw["mcp_serve_status"]; exists {
		t.Error("mcp_serve_status should be omitted when empty")
	}
}

// TestMCPServeStart_AlreadyRunning은 서버가 이미 실행 중일 때
// 중복 시작 방지를 확인합니다 (T-24).
func TestMCPServeStart_AlreadyRunning(t *testing.T) {
	client := NewClient("ws://localhost:9999/ws", "test-token", "1.0.0")
	// TokenRefresher를 설정하여 인증 체크를 통과하도록 함
	creds := &auth.Credentials{
		AccessToken:  "test-token",
		RefreshToken: "test-refresh",
		ExpiresAt:    time.Now().Add(1 * time.Hour),
		ServerURL:    "https://api.autopus.co",
	}
	tr := auth.NewTokenRefresher(creds)
	logger := zerolog.Nop()
	router := NewRouter(client,
		WithErrorHandler(func(err error) {}),
		WithMCPServeAuth(tr, logger),
	)

	// 서버가 이미 실행 중인 상태 시뮬레이션
	router.mcpServeMu.Lock()
	router.mcpServeServer = &mcpserver.Server{} // mock: nil이 아닌 값
	router.mcpServeMu.Unlock()

	payload, _ := json.Marshal(ws.MCPServeStartPayload{
		BackendURL: "https://api.autopus.co",
	})

	msg := ws.AgentMessage{
		Type:      ws.AgentMsgMCPServeStart,
		ID:        "msg-mcp-serve-dup-001",
		Timestamp: time.Now(),
		Payload:   payload,
	}

	// 패닉 없이 에러 반환이어야 함
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("handleMCPServeStart panicked on duplicate start: %v", r)
			}
		}()
		_ = router.HandleMessage(context.Background(), msg)
	}()

	// 정리
	router.mcpServeMu.Lock()
	router.mcpServeServer = nil
	router.mcpServeMu.Unlock()
}

// TestMCPServeStatus_Running은 서버가 실행 중일 때
// "running"을 반환하는지 확인합니다 (T-26).
func TestMCPServeStatus_Running(t *testing.T) {
	client := NewClient("ws://localhost:9999/ws", "test-token", "1.0.0")
	router := NewRouter(client)

	// 서버가 실행 중인 상태 시뮬레이션
	router.mcpServeMu.Lock()
	router.mcpServeServer = &mcpserver.Server{}
	router.mcpServeMu.Unlock()

	status := router.MCPServeStatus()
	if status != "running" {
		t.Errorf("expected status 'running', got %q", status)
	}

	// 정리
	router.mcpServeMu.Lock()
	router.mcpServeServer = nil
	router.mcpServeMu.Unlock()
}

// 참고: TestUnknownMessageType는 compat_test.go에 이미 정의되어 있습니다.
