package provider

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/insajin/autopus-codex-rpc/protocol"
)

// TestCodexProtocol_JSONRPCRequest_MarshalUnmarshal은
// protocol.JSONRPCRequest의 직렬화/역직렬화 라운드트립을 검증합니다.
func TestCodexProtocol_JSONRPCRequest_MarshalUnmarshal(t *testing.T) {
	tests := []struct {
		name string
		req  protocol.JSONRPCRequest
	}{
		{
			name: "params가 없는 요청",
			req: protocol.JSONRPCRequest{
				JSONRPC: "2.0",
				Method:  "initialize",
				ID:      1,
			},
		},
		{
			name: "params가 있는 요청",
			req: protocol.JSONRPCRequest{
				JSONRPC: "2.0",
				Method:  "thread/start",
				ID:      42,
				Params:  json.RawMessage(`{"model":"gpt-5-codex","cwd":"/tmp"}`),
			},
		},
		{
			name: "빈 params 객체",
			req: protocol.JSONRPCRequest{
				JSONRPC: "2.0",
				Method:  "initialized",
				ID:      100,
				Params:  json.RawMessage(`{}`),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 직렬화
			data, err := json.Marshal(tt.req)
			if err != nil {
				t.Fatalf("직렬화 실패: %v", err)
			}

			// 역직렬화
			var got protocol.JSONRPCRequest
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("역직렬화 실패: %v", err)
			}

			// 검증
			if got.JSONRPC != tt.req.JSONRPC {
				t.Errorf("JSONRPC: got %q, want %q", got.JSONRPC, tt.req.JSONRPC)
			}
			if got.Method != tt.req.Method {
				t.Errorf("Method: got %q, want %q", got.Method, tt.req.Method)
			}
			if got.ID != tt.req.ID {
				t.Errorf("ID: got %d, want %d", got.ID, tt.req.ID)
			}
			if string(got.Params) != string(tt.req.Params) {
				t.Errorf("Params: got %s, want %s", string(got.Params), string(tt.req.Params))
			}
		})
	}
}

// TestCodexProtocol_JSONRPCResponse_MarshalUnmarshal은
// protocol.JSONRPCResponse의 Result/Error 변형별 직렬화/역직렬화를 검증합니다.
func TestCodexProtocol_JSONRPCResponse_MarshalUnmarshal(t *testing.T) {
	resultData := json.RawMessage(`{"threadId":"thread-123"}`)
	id1 := int64(1)
	id2 := int64(2)
	id3 := int64(3)

	tests := []struct {
		name      string
		resp      protocol.JSONRPCResponse
		hasResult bool
		hasError  bool
	}{
		{
			name: "Result가 있는 성공 응답",
			resp: protocol.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      &id1,
				Result:  &resultData,
			},
			hasResult: true,
			hasError:  false,
		},
		{
			name: "Error가 있는 실패 응답",
			resp: protocol.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      &id2,
				Error: &protocol.JSONRPCError{
					Code:    protocol.ErrCodeMethodNotFound,
					Message: "메서드를 찾을 수 없습니다",
				},
			},
			hasResult: false,
			hasError:  true,
		},
		{
			name: "Result와 Error 모두 없는 응답",
			resp: protocol.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      &id3,
			},
			hasResult: false,
			hasError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.resp)
			if err != nil {
				t.Fatalf("직렬화 실패: %v", err)
			}

			var got protocol.JSONRPCResponse
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("역직렬화 실패: %v", err)
			}

			if got.JSONRPC != tt.resp.JSONRPC {
				t.Errorf("JSONRPC: got %q, want %q", got.JSONRPC, tt.resp.JSONRPC)
			}
			if tt.resp.ID != nil && (got.ID == nil || *got.ID != *tt.resp.ID) {
				t.Errorf("ID: got %v, want %v", got.ID, tt.resp.ID)
			}

			// Result 검증
			if tt.hasResult {
				if got.Result == nil {
					t.Fatal("Result가 nil이지만 값이 예상됩니다")
				}
				if string(*got.Result) != string(*tt.resp.Result) {
					t.Errorf("Result: got %s, want %s", string(*got.Result), string(*tt.resp.Result))
				}
			} else {
				if got.Result != nil {
					t.Errorf("Result가 nil이 아니지만 nil이 예상됩니다: %s", string(*got.Result))
				}
			}

			// Error 검증
			if tt.hasError {
				if got.Error == nil {
					t.Fatal("Error가 nil이지만 값이 예상됩니다")
				}
				if got.Error.Code != tt.resp.Error.Code {
					t.Errorf("Error.Code: got %d, want %d", got.Error.Code, tt.resp.Error.Code)
				}
				if got.Error.Message != tt.resp.Error.Message {
					t.Errorf("Error.Message: got %q, want %q", got.Error.Message, tt.resp.Error.Message)
				}
			} else {
				if got.Error != nil {
					t.Errorf("Error가 nil이 아니지만 nil이 예상됩니다: %v", got.Error)
				}
			}
		})
	}
}

// TestCodexProtocol_JSONRPCNotification_MarshalUnmarshal은
// protocol.JSONRPCNotification의 직렬화/역직렬화를 검증합니다.
func TestCodexProtocol_JSONRPCNotification_MarshalUnmarshal(t *testing.T) {
	tests := []struct {
		name   string
		notif  protocol.JSONRPCNotification
		wantID bool // JSON에 id 필드가 없어야 합니다
	}{
		{
			name: "params가 있는 알림",
			notif: protocol.JSONRPCNotification{
				JSONRPC: "2.0",
				Method:  protocol.MethodAgentMessageDelta,
				Params:  json.RawMessage(`{"delta":"안녕하세요"}`),
			},
		},
		{
			name: "params가 없는 알림",
			notif: protocol.JSONRPCNotification{
				JSONRPC: "2.0",
				Method:  protocol.MethodInitialized,
			},
		},
		{
			name: "turn/completed 알림",
			notif: protocol.JSONRPCNotification{
				JSONRPC: "2.0",
				Method:  protocol.MethodTurnCompleted,
				Params:  json.RawMessage(`{"threadId":"thread-abc"}`),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.notif)
			if err != nil {
				t.Fatalf("직렬화 실패: %v", err)
			}

			// id 필드가 없는지 검증 (알림은 id가 없어야 합니다)
			var raw map[string]json.RawMessage
			if err := json.Unmarshal(data, &raw); err != nil {
				t.Fatalf("raw 파싱 실패: %v", err)
			}
			if _, hasID := raw["id"]; hasID {
				t.Error("알림 메시지에 id 필드가 존재합니다")
			}

			// 라운드트립 검증
			var got protocol.JSONRPCNotification
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("역직렬화 실패: %v", err)
			}

			if got.JSONRPC != tt.notif.JSONRPC {
				t.Errorf("JSONRPC: got %q, want %q", got.JSONRPC, tt.notif.JSONRPC)
			}
			if got.Method != tt.notif.Method {
				t.Errorf("Method: got %q, want %q", got.Method, tt.notif.Method)
			}
			if string(got.Params) != string(tt.notif.Params) {
				t.Errorf("Params: got %s, want %s", string(got.Params), string(tt.notif.Params))
			}
		})
	}
}

// TestCodexProtocol_DomainTypes_Roundtrip은
// 도메인 타입들의 직렬화/역직렬화 라운드트립을 검증합니다.
func TestCodexProtocol_DomainTypes_Roundtrip(t *testing.T) {
	t.Run("ThreadStartParams", func(t *testing.T) {
		original := protocol.ThreadStartParams{
			Model:          "gpt-5-codex",
			Cwd:            "/workspace/project",
			ApprovalPolicy: "auto-approve",
		}

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("직렬화 실패: %v", err)
		}

		var got protocol.ThreadStartParams
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("역직렬화 실패: %v", err)
		}

		if got.Model != original.Model {
			t.Errorf("Model: got %q, want %q", got.Model, original.Model)
		}
		if got.Cwd != original.Cwd {
			t.Errorf("Cwd: got %q, want %q", got.Cwd, original.Cwd)
		}
		if got.ApprovalPolicy != original.ApprovalPolicy {
			t.Errorf("ApprovalPolicy: got %q, want %q", got.ApprovalPolicy, original.ApprovalPolicy)
		}
	})

	t.Run("TurnStartParams", func(t *testing.T) {
		original := protocol.TurnStartParams{
			ThreadID: "thread-456",
			Input: []protocol.TurnInput{
				{Type: "text", Text: "파일 목록을 보여주세요"},
				{Type: "text", Text: "그리고 코드를 분석해주세요"},
			},
		}

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("직렬화 실패: %v", err)
		}

		var got protocol.TurnStartParams
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("역직렬화 실패: %v", err)
		}

		if got.ThreadID != original.ThreadID {
			t.Errorf("ThreadID: got %q, want %q", got.ThreadID, original.ThreadID)
		}
		if len(got.Input) != len(original.Input) {
			t.Fatalf("Input 길이: got %d, want %d", len(got.Input), len(original.Input))
		}
		for i, input := range got.Input {
			if input.Type != original.Input[i].Type {
				t.Errorf("Input[%d].Type: got %q, want %q", i, input.Type, original.Input[i].Type)
			}
			if input.Text != original.Input[i].Text {
				t.Errorf("Input[%d].Text: got %q, want %q", i, input.Text, original.Input[i].Text)
			}
		}
	})

	t.Run("AccountLoginParams_APIKey", func(t *testing.T) {
		original := protocol.AccountLoginParams{
			Method: "apiKey",
			APIKey: "sk-test-key-123",
		}

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("직렬화 실패: %v", err)
		}

		var got protocol.AccountLoginParams
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("역직렬화 실패: %v", err)
		}

		if got.Method != original.Method {
			t.Errorf("Method: got %q, want %q", got.Method, original.Method)
		}
		if got.APIKey != original.APIKey {
			t.Errorf("APIKey: got %q, want %q", got.APIKey, original.APIKey)
		}
	})

	t.Run("InitializeParams_and_Result", func(t *testing.T) {
		params := protocol.InitializeParams{ClientVersion: "1.0.0", ClientName: "test-client"}
		data, err := json.Marshal(params)
		if err != nil {
			t.Fatalf("직렬화 실패: %v", err)
		}
		var gotParams protocol.InitializeParams
		if err := json.Unmarshal(data, &gotParams); err != nil {
			t.Fatalf("역직렬화 실패: %v", err)
		}
		if gotParams.ClientVersion != params.ClientVersion {
			t.Errorf("ClientVersion: got %q, want %q", gotParams.ClientVersion, params.ClientVersion)
		}
		if gotParams.ClientName != params.ClientName {
			t.Errorf("ClientName: got %q, want %q", gotParams.ClientName, params.ClientName)
		}

		result := protocol.InitializeResult{ServerName: "test-server", ServerVersion: "2.0.0"}
		data, err = json.Marshal(result)
		if err != nil {
			t.Fatalf("직렬화 실패: %v", err)
		}
		var gotResult protocol.InitializeResult
		if err := json.Unmarshal(data, &gotResult); err != nil {
			t.Fatalf("역직렬화 실패: %v", err)
		}
		if gotResult.ServerVersion != result.ServerVersion {
			t.Errorf("ServerVersion: got %q, want %q", gotResult.ServerVersion, result.ServerVersion)
		}
	})

	t.Run("ItemCompletedParams_commandExecution", func(t *testing.T) {
		cmdData, _ := json.Marshal(protocol.CommandExecutionCompleted{
			Command:  "ls -la",
			ExitCode: 0,
			Output:   "total 42\n.",
		})
		original := protocol.ItemCompletedParams{
			ThreadID: "thread-001",
			ItemID:   "cmd-001",
			ItemType: "commandExecution",
			Data:     cmdData,
		}

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("직렬화 실패: %v", err)
		}

		var got protocol.ItemCompletedParams
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("역직렬화 실패: %v", err)
		}

		if got.ThreadID != original.ThreadID {
			t.Errorf("ThreadID: got %q, want %q", got.ThreadID, original.ThreadID)
		}
		if got.ItemID != original.ItemID {
			t.Errorf("ItemID: got %q, want %q", got.ItemID, original.ItemID)
		}
		if got.ItemType != original.ItemType {
			t.Errorf("ItemType: got %q, want %q", got.ItemType, original.ItemType)
		}
	})

	t.Run("ApprovalResponseParams", func(t *testing.T) {
		original := protocol.ApprovalResponseParams{
			ThreadID: "thread-789",
			ItemID:   "item-001",
			Decision: "accept",
		}

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("직렬화 실패: %v", err)
		}

		var got protocol.ApprovalResponseParams
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("역직렬화 실패: %v", err)
		}

		if got.ThreadID != original.ThreadID {
			t.Errorf("ThreadID: got %q, want %q", got.ThreadID, original.ThreadID)
		}
		if got.ItemID != original.ItemID {
			t.Errorf("ItemID: got %q, want %q", got.ItemID, original.ItemID)
		}
		if got.Decision != original.Decision {
			t.Errorf("Decision: got %q, want %q", got.Decision, original.Decision)
		}
	})

	t.Run("TurnCompletedParams", func(t *testing.T) {
		original := protocol.TurnCompletedParams{ThreadID: "thread-999"}
		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("직렬화 실패: %v", err)
		}

		var got protocol.TurnCompletedParams
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("역직렬화 실패: %v", err)
		}
		if got.ThreadID != original.ThreadID {
			t.Errorf("ThreadID: got %q, want %q", got.ThreadID, original.ThreadID)
		}
	})
}

// TestCodexProtocol_MapJSONRPCError은
// protocol.MapJSONRPCError의 에러 코드별 매핑을 검증합니다.
func TestCodexProtocol_MapJSONRPCError(t *testing.T) {
	tests := []struct {
		name        string
		rpcErr      *protocol.JSONRPCError
		wantNil     bool
		wantWrapped error
		wantContain string
	}{
		{
			name:    "nil 입력",
			rpcErr:  nil,
			wantNil: true,
		},
		{
			name: "컨텍스트 윈도우 초과",
			rpcErr: &protocol.JSONRPCError{
				Code:    protocol.ErrCodeContextWindowExceeded,
				Message: "context too long",
			},
			wantContain: "context too long",
		},
		{
			name: "사용량 제한 초과",
			rpcErr: &protocol.JSONRPCError{
				Code:    protocol.ErrCodeUsageLimitExceeded,
				Message: "usage limit reached",
			},
			wantContain: "usage limit reached",
		},
		{
			name: "알 수 없는 에러 코드 - JSONRPCError 그대로 반환",
			rpcErr: &protocol.JSONRPCError{
				Code:    protocol.ErrCodeInternalError,
				Message: "internal server error",
			},
			wantContain: "internal server error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := protocol.MapJSONRPCError(tt.rpcErr)

			if tt.wantNil {
				if err != nil {
					t.Errorf("nil을 기대했지만 에러를 받았습니다: %v", err)
				}
				return
			}

			if err == nil {
				t.Fatal("에러를 기대했지만 nil을 받았습니다")
			}

			if tt.wantWrapped != nil {
				if !errors.Is(err, tt.wantWrapped) {
					t.Errorf("errors.Is(%v, %v) = false, want true", err, tt.wantWrapped)
				}
			}

			if tt.wantContain != "" {
				errStr := err.Error()
				if !containsSubstring(errStr, tt.wantContain) {
					t.Errorf("에러 메시지에 %q가 포함되어야 합니다: %q", tt.wantContain, errStr)
				}
			}
		})
	}
}

// TestCodexProtocol_JSONRPCError_ErrorString은
// protocol.JSONRPCError.Error()의 출력 형식을 검증합니다.
func TestCodexProtocol_JSONRPCError_ErrorString(t *testing.T) {
	tests := []struct {
		name    string
		rpcErr  protocol.JSONRPCError
		wantStr string
	}{
		{
			name: "파싱 에러",
			rpcErr: protocol.JSONRPCError{
				Code:    protocol.ErrCodeParseError,
				Message: "invalid json",
			},
			wantStr: "JSON-RPC 에러 [-32700]: invalid json",
		},
		{
			name: "메서드 없음",
			rpcErr: protocol.JSONRPCError{
				Code:    protocol.ErrCodeMethodNotFound,
				Message: "method not found",
			},
			wantStr: "JSON-RPC 에러 [-32601]: method not found",
		},
		{
			name: "커스텀 에러 코드",
			rpcErr: protocol.JSONRPCError{
				Code:    protocol.ErrCodeContextWindowExceeded,
				Message: "context window exceeded",
			},
			wantStr: "JSON-RPC 에러 [-32001]: context window exceeded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.rpcErr.Error()
			if got != tt.wantStr {
				t.Errorf("Error(): got %q, want %q", got, tt.wantStr)
			}
		})
	}
}

// TestCodexProtocol_ErrorConstants는 에러 코드 상수 값을 검증합니다.
func TestCodexProtocol_ErrorConstants(t *testing.T) {
	if protocol.ErrCodeParseError != -32700 {
		t.Errorf("ErrCodeParseError: got %d, want -32700", protocol.ErrCodeParseError)
	}
	if protocol.ErrCodeInvalidRequest != -32600 {
		t.Errorf("ErrCodeInvalidRequest: got %d, want -32600", protocol.ErrCodeInvalidRequest)
	}
	if protocol.ErrCodeMethodNotFound != -32601 {
		t.Errorf("ErrCodeMethodNotFound: got %d, want -32601", protocol.ErrCodeMethodNotFound)
	}
	if protocol.ErrCodeInvalidParams != -32602 {
		t.Errorf("ErrCodeInvalidParams: got %d, want -32602", protocol.ErrCodeInvalidParams)
	}
	if protocol.ErrCodeInternalError != -32603 {
		t.Errorf("ErrCodeInternalError: got %d, want -32603", protocol.ErrCodeInternalError)
	}
	if protocol.ErrCodeContextWindowExceeded != -32001 {
		t.Errorf("ErrCodeContextWindowExceeded: got %d, want -32001", protocol.ErrCodeContextWindowExceeded)
	}
	if protocol.ErrCodeUsageLimitExceeded != -32002 {
		t.Errorf("ErrCodeUsageLimitExceeded: got %d, want -32002", protocol.ErrCodeUsageLimitExceeded)
	}
	if protocol.ErrCodeUnauthorized != -32003 {
		t.Errorf("ErrCodeUnauthorized: got %d, want -32003", protocol.ErrCodeUnauthorized)
	}
	if protocol.ErrCodeConnectionFailed != -32004 {
		t.Errorf("ErrCodeConnectionFailed: got %d, want -32004", protocol.ErrCodeConnectionFailed)
	}
}

// TestCodexProtocol_MethodConstants는 메서드 상수 값을 검증합니다.
func TestCodexProtocol_MethodConstants(t *testing.T) {
	methods := map[string]string{
		"MethodInitialize":                  protocol.MethodInitialize,
		"MethodInitialized":                 protocol.MethodInitialized,
		"MethodAccountLoginStart":           protocol.MethodAccountLoginStart,
		"MethodThreadStart":                 protocol.MethodThreadStart,
		"MethodTurnStart":                   protocol.MethodTurnStart,
		"MethodAgentMessageDelta":           protocol.MethodAgentMessageDelta,
		"MethodCommandExecutionOutputDelta": protocol.MethodCommandExecutionOutputDelta,
		"MethodItemCompleted":               protocol.MethodItemCompleted,
		"MethodCommandExecutionApproval":    protocol.MethodCommandExecutionApproval,
		"MethodFileChangeApproval":          protocol.MethodFileChangeApproval,
		"MethodTurnCompleted":               protocol.MethodTurnCompleted,
	}

	expectedValues := map[string]string{
		"MethodInitialize":                  "initialize",
		"MethodInitialized":                 "initialized",
		"MethodAccountLoginStart":           "account/login/start",
		"MethodThreadStart":                 "thread/start",
		"MethodTurnStart":                   "turn/start",
		"MethodAgentMessageDelta":           "item/agentMessage/delta",
		"MethodCommandExecutionOutputDelta": "item/commandExecution/outputDelta",
		"MethodItemCompleted":               "item/completed",
		"MethodCommandExecutionApproval":    "item/commandExecution/requestApproval",
		"MethodFileChangeApproval":          "item/fileChange/requestApproval",
		"MethodTurnCompleted":               "turn/completed",
	}

	for name, got := range methods {
		want := expectedValues[name]
		if got != want {
			t.Errorf("%s: got %q, want %q", name, got, want)
		}
	}
}

// containsSubstring은 문자열에 서브스트링이 포함되어 있는지 확인합니다.
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

// containsHelper는 실제 포함 여부를 검사합니다.
func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
