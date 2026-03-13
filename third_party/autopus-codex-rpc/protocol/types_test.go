package protocol_test

import (
	"encoding/json"
	"testing"

	"github.com/insajin/autopus-codex-rpc/protocol"
)

// TestJSONRPCRequest_Serialization은 JSONRPCRequest의 JSON 직렬화를 검증한다.
func TestJSONRPCRequest_Serialization(t *testing.T) {
	t.Run("기본 요청 직렬화", func(t *testing.T) {
		params, _ := json.Marshal(map[string]string{"key": "value"})
		req := protocol.JSONRPCRequest{
			JSONRPC: "2.0",
			Method:  "test/method",
			ID:      1,
			Params:  json.RawMessage(params),
		}

		data, err := json.Marshal(req)
		if err != nil {
			t.Fatalf("직렬화 실패: %v", err)
		}

		var decoded protocol.JSONRPCRequest
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("역직렬화 실패: %v", err)
		}

		if decoded.JSONRPC != "2.0" {
			t.Errorf("JSONRPC 필드 불일치: got %q, want %q", decoded.JSONRPC, "2.0")
		}
		if decoded.Method != "test/method" {
			t.Errorf("Method 필드 불일치: got %q, want %q", decoded.Method, "test/method")
		}
		if decoded.ID != 1 {
			t.Errorf("ID 필드 불일치: got %d, want %d", decoded.ID, 1)
		}
	})

	t.Run("Params 없는 요청 직렬화", func(t *testing.T) {
		req := protocol.JSONRPCRequest{
			JSONRPC: "2.0",
			Method:  "initialize",
			ID:      42,
		}

		data, err := json.Marshal(req)
		if err != nil {
			t.Fatalf("직렬화 실패: %v", err)
		}

		// params 필드가 없어야 함
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(data, &raw); err != nil {
			t.Fatalf("raw 역직렬화 실패: %v", err)
		}
		if _, ok := raw["params"]; ok {
			t.Error("params 필드가 없어야 하지만 존재함")
		}
	})

	t.Run("Params는 json.RawMessage 타입이어야 함", func(t *testing.T) {
		params := json.RawMessage(`{"model":"o4-mini","cwd":"/tmp"}`)
		req := protocol.JSONRPCRequest{
			JSONRPC: "2.0",
			Method:  "thread/start",
			ID:      1,
			Params:  params,
		}

		data, err := json.Marshal(req)
		if err != nil {
			t.Fatalf("직렬화 실패: %v", err)
		}

		var decoded protocol.JSONRPCRequest
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("역직렬화 실패: %v", err)
		}

		// Params가 원본과 동일해야 함
		var original, got map[string]string
		json.Unmarshal(params, &original)
		json.Unmarshal(decoded.Params, &got)

		if original["model"] != got["model"] {
			t.Errorf("Params.model 불일치: got %q, want %q", got["model"], original["model"])
		}
	})
}

// TestJSONRPCResponse_Serialization은 JSONRPCResponse의 JSON 직렬화를 검증한다.
func TestJSONRPCResponse_Serialization(t *testing.T) {
	t.Run("성공 응답 직렬화", func(t *testing.T) {
		result := json.RawMessage(`{"threadId":"abc123"}`)
		id := int64(5)
		resp := protocol.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      &id,
			Result:  &result,
		}

		data, err := json.Marshal(resp)
		if err != nil {
			t.Fatalf("직렬화 실패: %v", err)
		}

		var decoded protocol.JSONRPCResponse
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("역직렬화 실패: %v", err)
		}

		if decoded.ID == nil || *decoded.ID != 5 {
			t.Errorf("ID 불일치: got %v, want 5", decoded.ID)
		}
		if decoded.Result == nil {
			t.Error("Result가 nil이면 안 됨")
		}
		if decoded.Error != nil {
			t.Error("Error가 nil이어야 함")
		}
	})

	t.Run("에러 응답 직렬화", func(t *testing.T) {
		id := int64(3)
		resp := protocol.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      &id,
			Error: &protocol.JSONRPCError{
				Code:    -32600,
				Message: "Invalid Request",
			},
		}

		data, err := json.Marshal(resp)
		if err != nil {
			t.Fatalf("직렬화 실패: %v", err)
		}

		var decoded protocol.JSONRPCResponse
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("역직렬화 실패: %v", err)
		}

		if decoded.Error == nil {
			t.Fatal("Error가 nil이면 안 됨")
		}
		if decoded.Error.Code != -32600 {
			t.Errorf("Error.Code 불일치: got %d, want %d", decoded.Error.Code, -32600)
		}
	})

	t.Run("알림 응답(ID nil) 직렬화", func(t *testing.T) {
		result := json.RawMessage(`{}`)
		resp := protocol.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      nil,
			Result:  &result,
		}

		data, err := json.Marshal(resp)
		if err != nil {
			t.Fatalf("직렬화 실패: %v", err)
		}

		// id 필드가 null 또는 없어야 함
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(data, &raw); err != nil {
			t.Fatalf("raw 역직렬화 실패: %v", err)
		}

		if idField, ok := raw["id"]; ok && string(idField) != "null" {
			t.Errorf("ID가 null이어야 하지만: %s", string(idField))
		}
	})
}

// TestJSONRPCNotification_Serialization은 JSONRPCNotification의 JSON 직렬화를 검증한다.
func TestJSONRPCNotification_Serialization(t *testing.T) {
	t.Run("알림 직렬화", func(t *testing.T) {
		params := json.RawMessage(`{"threadId":"t1","turnId":"turn1"}`)
		notif := protocol.JSONRPCNotification{
			JSONRPC: "2.0",
			Method:  "turn/completed",
			Params:  params,
		}

		data, err := json.Marshal(notif)
		if err != nil {
			t.Fatalf("직렬화 실패: %v", err)
		}

		var decoded protocol.JSONRPCNotification
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("역직렬화 실패: %v", err)
		}

		if decoded.Method != "turn/completed" {
			t.Errorf("Method 불일치: got %q, want %q", decoded.Method, "turn/completed")
		}
	})
}

// TestCodexDomainTypes는 Codex 도메인 타입의 직렬화를 검증한다.
func TestCodexDomainTypes(t *testing.T) {
	t.Run("ThreadStartParams 직렬화", func(t *testing.T) {
		params := protocol.ThreadStartParams{
			Model:          "o4-mini",
			Cwd:            "/workspace",
			ApprovalPolicy: "auto-approve",
			Sandbox:        true,
		}

		data, err := json.Marshal(params)
		if err != nil {
			t.Fatalf("직렬화 실패: %v", err)
		}

		var decoded protocol.ThreadStartParams
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("역직렬화 실패: %v", err)
		}

		if decoded.Model != "o4-mini" {
			t.Errorf("Model 불일치: got %q, want %q", decoded.Model, "o4-mini")
		}
		if !decoded.Sandbox {
			t.Error("Sandbox가 true여야 함")
		}
	})

	t.Run("ThreadStartParams Sandbox omitempty", func(t *testing.T) {
		params := protocol.ThreadStartParams{
			Model: "o4-mini",
			Cwd:   "/workspace",
		}

		data, err := json.Marshal(params)
		if err != nil {
			t.Fatalf("직렬화 실패: %v", err)
		}

		var raw map[string]json.RawMessage
		if err := json.Unmarshal(data, &raw); err != nil {
			t.Fatalf("역직렬화 실패: %v", err)
		}
		// Sandbox=false면 omitempty로 인해 필드 없어야 함
		if _, ok := raw["sandbox"]; ok {
			t.Error("sandbox 필드가 없어야 하지만 존재함 (omitempty)")
		}
	})

	t.Run("InitializeParams clientInfo 포함", func(t *testing.T) {
		params := protocol.InitializeParams{
			ClientInfo: protocol.ClientInfo{
				Name:    "autopus-bridge",
				Version: "1.0.0",
			},
		}

		data, err := json.Marshal(params)
		if err != nil {
			t.Fatalf("직렬화 실패: %v", err)
		}

		// clientInfo 필드가 올바르게 직렬화되는지 확인
		var raw map[string]interface{}
		if err := json.Unmarshal(data, &raw); err != nil {
			t.Fatalf("raw 역직렬화 실패: %v", err)
		}
		if _, ok := raw["clientInfo"]; !ok {
			t.Error("clientInfo 필드가 있어야 하지만 존재하지 않음")
		}

		var decoded protocol.InitializeParams
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("역직렬화 실패: %v", err)
		}

		if decoded.ClientInfo.Name != "autopus-bridge" {
			t.Errorf("ClientInfo.Name 불일치: got %q, want %q", decoded.ClientInfo.Name, "autopus-bridge")
		}
	})

	t.Run("AccountLoginParams 직렬화", func(t *testing.T) {
		params := protocol.AccountLoginParams{
			Type:   "apiKey",
			APIKey: "sk-test-123",
		}

		data, err := json.Marshal(params)
		if err != nil {
			t.Fatalf("직렬화 실패: %v", err)
		}

		var decoded protocol.AccountLoginParams
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("역직렬화 실패: %v", err)
		}

		if decoded.Type != "apiKey" {
			t.Errorf("Type 불일치: got %q, want %q", decoded.Type, "apiKey")
		}
		if decoded.APIKey != "sk-test-123" {
			t.Errorf("APIKey 불일치: got %q, want %q", decoded.APIKey, "sk-test-123")
		}
	})
}
