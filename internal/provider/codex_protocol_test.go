package provider

import (
	"encoding/json"
	"errors"
	"testing"
)

// TestCodexProtocol_JSONRPCRequest_MarshalUnmarshal은
// JSONRPCRequest의 직렬화/역직렬화 라운드트립을 검증합니다.
func TestCodexProtocol_JSONRPCRequest_MarshalUnmarshal(t *testing.T) {
	tests := []struct {
		name string
		req  JSONRPCRequest
	}{
		{
			name: "params가 없는 요청",
			req: JSONRPCRequest{
				JSONRPC: "2.0",
				Method:  "initialize",
				ID:      1,
			},
		},
		{
			name: "params가 있는 요청",
			req: JSONRPCRequest{
				JSONRPC: "2.0",
				Method:  "thread/start",
				ID:      42,
				Params:  json.RawMessage(`{"model":"gpt-5-codex","cwd":"/tmp"}`),
			},
		},
		{
			name: "빈 params 객체",
			req: JSONRPCRequest{
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
			var got JSONRPCRequest
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
// JSONRPCResponse의 Result/Error 변형별 직렬화/역직렬화를 검증합니다.
func TestCodexProtocol_JSONRPCResponse_MarshalUnmarshal(t *testing.T) {
	resultData := json.RawMessage(`{"threadId":"thread-123"}`)

	tests := []struct {
		name      string
		resp      JSONRPCResponse
		hasResult bool
		hasError  bool
	}{
		{
			name: "Result가 있는 성공 응답",
			resp: JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      1,
				Result:  &resultData,
			},
			hasResult: true,
			hasError:  false,
		},
		{
			name: "Error가 있는 실패 응답",
			resp: JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      2,
				Error: &JSONRPCError{
					Code:    ErrCodeMethodNotFound,
					Message: "메서드를 찾을 수 없습니다",
				},
			},
			hasResult: false,
			hasError:  true,
		},
		{
			name: "Result와 Error 모두 없는 응답",
			resp: JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      3,
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

			var got JSONRPCResponse
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("역직렬화 실패: %v", err)
			}

			if got.JSONRPC != tt.resp.JSONRPC {
				t.Errorf("JSONRPC: got %q, want %q", got.JSONRPC, tt.resp.JSONRPC)
			}
			if got.ID != tt.resp.ID {
				t.Errorf("ID: got %d, want %d", got.ID, tt.resp.ID)
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
// JSONRPCNotification의 직렬화/역직렬화를 검증합니다.
func TestCodexProtocol_JSONRPCNotification_MarshalUnmarshal(t *testing.T) {
	tests := []struct {
		name   string
		notif  JSONRPCNotification
		wantID bool // JSON에 id 필드가 없어야 합니다
	}{
		{
			name: "params가 있는 알림",
			notif: JSONRPCNotification{
				JSONRPC: "2.0",
				Method:  MethodAgentMessageDelta,
				Params:  json.RawMessage(`{"text":"안녕하세요"}`),
			},
		},
		{
			name: "params가 없는 알림",
			notif: JSONRPCNotification{
				JSONRPC: "2.0",
				Method:  MethodInitialized,
			},
		},
		{
			name: "turn/completed 알림",
			notif: JSONRPCNotification{
				JSONRPC: "2.0",
				Method:  MethodTurnCompleted,
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
			var got JSONRPCNotification
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
		original := ThreadStartParams{
			Model:          "gpt-5-codex",
			Cwd:            "/workspace/project",
			ApprovalPolicy: "auto-approve",
		}

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("직렬화 실패: %v", err)
		}

		var got ThreadStartParams
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
		original := TurnStartParams{
			ThreadID: "thread-456",
			Input: []TurnInput{
				{Type: "text", Text: "파일 목록을 보여주세요"},
				{Type: "text", Text: "그리고 코드를 분석해주세요"},
			},
		}

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("직렬화 실패: %v", err)
		}

		var got TurnStartParams
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
		original := AccountLoginParams{
			Method: "apiKey",
			APIKey: "sk-test-key-123",
		}

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("직렬화 실패: %v", err)
		}

		var got AccountLoginParams
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
		params := InitializeParams{ClientVersion: "1.0.0"}
		data, err := json.Marshal(params)
		if err != nil {
			t.Fatalf("직렬화 실패: %v", err)
		}
		var gotParams InitializeParams
		if err := json.Unmarshal(data, &gotParams); err != nil {
			t.Fatalf("역직렬화 실패: %v", err)
		}
		if gotParams.ClientVersion != params.ClientVersion {
			t.Errorf("ClientVersion: got %q, want %q", gotParams.ClientVersion, params.ClientVersion)
		}

		result := InitializeResult{ServerVersion: "2.0.0"}
		data, err = json.Marshal(result)
		if err != nil {
			t.Fatalf("직렬화 실패: %v", err)
		}
		var gotResult InitializeResult
		if err := json.Unmarshal(data, &gotResult); err != nil {
			t.Fatalf("역직렬화 실패: %v", err)
		}
		if gotResult.ServerVersion != result.ServerVersion {
			t.Errorf("ServerVersion: got %q, want %q", gotResult.ServerVersion, result.ServerVersion)
		}
	})

	t.Run("CommandExecutionItem", func(t *testing.T) {
		original := CommandExecutionItem{
			ID:       "cmd-001",
			Command:  "ls -la",
			Output:   "total 42\ndrwxr-xr-x 5 user group 160 Jan 1 00:00 .",
			ExitCode: 0,
		}

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("직렬화 실패: %v", err)
		}

		var got CommandExecutionItem
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("역직렬화 실패: %v", err)
		}

		if got.ID != original.ID {
			t.Errorf("ID: got %q, want %q", got.ID, original.ID)
		}
		if got.Command != original.Command {
			t.Errorf("Command: got %q, want %q", got.Command, original.Command)
		}
		if got.Output != original.Output {
			t.Errorf("Output: got %q, want %q", got.Output, original.Output)
		}
		if got.ExitCode != original.ExitCode {
			t.Errorf("ExitCode: got %d, want %d", got.ExitCode, original.ExitCode)
		}
	})

	t.Run("MCPToolCallItem", func(t *testing.T) {
		original := MCPToolCallItem{
			ID:       "mcp-001",
			ToolName: "read_file",
			Input:    json.RawMessage(`{"path":"/tmp/test.txt"}`),
			Output:   "파일 내용입니다",
		}

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("직렬화 실패: %v", err)
		}

		var got MCPToolCallItem
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("역직렬화 실패: %v", err)
		}

		if got.ID != original.ID {
			t.Errorf("ID: got %q, want %q", got.ID, original.ID)
		}
		if got.ToolName != original.ToolName {
			t.Errorf("ToolName: got %q, want %q", got.ToolName, original.ToolName)
		}
		if string(got.Input) != string(original.Input) {
			t.Errorf("Input: got %s, want %s", string(got.Input), string(original.Input))
		}
		if got.Output != original.Output {
			t.Errorf("Output: got %q, want %q", got.Output, original.Output)
		}
	})

	t.Run("ApprovalResponse", func(t *testing.T) {
		original := ApprovalResponse{
			ExecutionID: "exec-789",
			Decision:    "accept",
		}

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("직렬화 실패: %v", err)
		}

		var got ApprovalResponse
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("역직렬화 실패: %v", err)
		}

		if got.ExecutionID != original.ExecutionID {
			t.Errorf("ExecutionID: got %q, want %q", got.ExecutionID, original.ExecutionID)
		}
		if got.Decision != original.Decision {
			t.Errorf("Decision: got %q, want %q", got.Decision, original.Decision)
		}
	})

	t.Run("TurnCompletedParams", func(t *testing.T) {
		original := TurnCompletedParams{ThreadID: "thread-999"}
		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("직렬화 실패: %v", err)
		}

		var got TurnCompletedParams
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("역직렬화 실패: %v", err)
		}
		if got.ThreadID != original.ThreadID {
			t.Errorf("ThreadID: got %q, want %q", got.ThreadID, original.ThreadID)
		}
	})
}

// TestCodexProtocol_MapJSONRPCError은
// MapJSONRPCError의 에러 코드별 매핑을 검증합니다.
func TestCodexProtocol_MapJSONRPCError(t *testing.T) {
	tests := []struct {
		name        string
		rpcErr      *JSONRPCError
		wantNil     bool
		wantWrapped error // errors.Is로 검증할 에러
		wantContain string
	}{
		{
			name:    "nil 입력",
			rpcErr:  nil,
			wantNil: true,
		},
		{
			name: "컨텍스트 윈도우 초과",
			rpcErr: &JSONRPCError{
				Code:    ErrCodeContextWindowExceeded,
				Message: "context too long",
			},
			wantContain: "컨텍스트 윈도우 초과",
		},
		{
			name: "사용량 제한 초과",
			rpcErr: &JSONRPCError{
				Code:    ErrCodeUsageLimitExceeded,
				Message: "usage limit reached",
			},
			wantContain: "사용량 제한 초과",
		},
		{
			name: "인증 실패 - ErrNoAPIKey 래핑",
			rpcErr: &JSONRPCError{
				Code:    ErrCodeUnauthorized,
				Message: "invalid api key",
			},
			wantWrapped: ErrNoAPIKey,
		},
		{
			name: "연결 실패 - ErrConnectionClosed 래핑",
			rpcErr: &JSONRPCError{
				Code:    ErrCodeConnectionFailed,
				Message: "connection refused",
			},
			wantWrapped: ErrConnectionClosed,
		},
		{
			name: "알 수 없는 에러 코드 - JSONRPCError 그대로 반환",
			rpcErr: &JSONRPCError{
				Code:    ErrCodeInternalError,
				Message: "internal server error",
			},
			wantContain: "internal server error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := MapJSONRPCError(tt.rpcErr)

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
// JSONRPCError.Error()의 출력 형식을 검증합니다.
func TestCodexProtocol_JSONRPCError_ErrorString(t *testing.T) {
	tests := []struct {
		name     string
		rpcErr   JSONRPCError
		wantStr  string
	}{
		{
			name: "파싱 에러",
			rpcErr: JSONRPCError{
				Code:    ErrCodeParseError,
				Message: "invalid json",
			},
			wantStr: "JSON-RPC 에러 [-32700]: invalid json",
		},
		{
			name: "메서드 없음",
			rpcErr: JSONRPCError{
				Code:    ErrCodeMethodNotFound,
				Message: "method not found",
			},
			wantStr: "JSON-RPC 에러 [-32601]: method not found",
		},
		{
			name: "커스텀 에러 코드",
			rpcErr: JSONRPCError{
				Code:    ErrCodeContextWindowExceeded,
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
	// JSON-RPC 표준 에러 코드 검증
	if ErrCodeParseError != -32700 {
		t.Errorf("ErrCodeParseError: got %d, want -32700", ErrCodeParseError)
	}
	if ErrCodeInvalidRequest != -32600 {
		t.Errorf("ErrCodeInvalidRequest: got %d, want -32600", ErrCodeInvalidRequest)
	}
	if ErrCodeMethodNotFound != -32601 {
		t.Errorf("ErrCodeMethodNotFound: got %d, want -32601", ErrCodeMethodNotFound)
	}
	if ErrCodeInvalidParams != -32602 {
		t.Errorf("ErrCodeInvalidParams: got %d, want -32602", ErrCodeInvalidParams)
	}
	if ErrCodeInternalError != -32603 {
		t.Errorf("ErrCodeInternalError: got %d, want -32603", ErrCodeInternalError)
	}

	// 커스텀 에러 코드 검증
	if ErrCodeContextWindowExceeded != -32001 {
		t.Errorf("ErrCodeContextWindowExceeded: got %d, want -32001", ErrCodeContextWindowExceeded)
	}
	if ErrCodeUsageLimitExceeded != -32002 {
		t.Errorf("ErrCodeUsageLimitExceeded: got %d, want -32002", ErrCodeUsageLimitExceeded)
	}
	if ErrCodeUnauthorized != -32003 {
		t.Errorf("ErrCodeUnauthorized: got %d, want -32003", ErrCodeUnauthorized)
	}
	if ErrCodeConnectionFailed != -32004 {
		t.Errorf("ErrCodeConnectionFailed: got %d, want -32004", ErrCodeConnectionFailed)
	}
}

// TestCodexProtocol_MethodConstants는 메서드 상수 값을 검증합니다.
func TestCodexProtocol_MethodConstants(t *testing.T) {
	methods := map[string]string{
		"MethodInitialize":                   MethodInitialize,
		"MethodInitialized":                  MethodInitialized,
		"MethodAccountLoginStart":            MethodAccountLoginStart,
		"MethodThreadStart":                  MethodThreadStart,
		"MethodTurnStart":                    MethodTurnStart,
		"MethodAgentMessageDelta":            MethodAgentMessageDelta,
		"MethodCommandExecutionOutputDelta":  MethodCommandExecutionOutputDelta,
		"MethodCommandExecution":             MethodCommandExecution,
		"MethodMCPToolCall":                  MethodMCPToolCall,
		"MethodRequestApproval":              MethodRequestApproval,
		"MethodTurnCompleted":                MethodTurnCompleted,
	}

	expectedValues := map[string]string{
		"MethodInitialize":                   "initialize",
		"MethodInitialized":                  "initialized",
		"MethodAccountLoginStart":            "account/login/start",
		"MethodThreadStart":                  "thread/start",
		"MethodTurnStart":                    "turn/start",
		"MethodAgentMessageDelta":            "item/agentMessage/delta",
		"MethodCommandExecutionOutputDelta":  "item/commandExecution/outputDelta",
		"MethodCommandExecution":             "item/commandExecution",
		"MethodMCPToolCall":                  "item/mcpToolCall",
		"MethodRequestApproval":              "requestApproval",
		"MethodTurnCompleted":                "turn/completed",
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
