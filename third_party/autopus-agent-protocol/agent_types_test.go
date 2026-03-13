package ws

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// TestComputerMessageTypeConstants verifies constant values for computer use message types.
func TestComputerMessageTypeConstants(t *testing.T) {
	tests := []struct {
		name     string
		got      string
		expected string
	}{
		{"AgentMsgComputerAction", AgentMsgComputerAction, "computer_action"},
		{"AgentMsgComputerResult", AgentMsgComputerResult, "computer_result"},
		{"AgentMsgComputerSessionStart", AgentMsgComputerSessionStart, "computer_session_start"},
		{"AgentMsgComputerSessionEnd", AgentMsgComputerSessionEnd, "computer_session_end"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expected {
				t.Errorf("got %q, want %q", tt.got, tt.expected)
			}
		})
	}
}

// TestBrowserMessageTypeConstants verifies constant values for agent-browser message types.
func TestBrowserMessageTypeConstants(t *testing.T) {
	tests := []struct {
		name     string
		got      string
		expected string
	}{
		{"AgentMsgBrowserSessionStart", AgentMsgBrowserSessionStart, "browser_session_start"},
		{"AgentMsgBrowserAction", AgentMsgBrowserAction, "browser_action"},
		{"AgentMsgBrowserSessionEnd", AgentMsgBrowserSessionEnd, "browser_session_end"},
		{"AgentMsgBrowserSessionReady", AgentMsgBrowserSessionReady, "browser_session_ready"},
		{"AgentMsgBrowserResult", AgentMsgBrowserResult, "browser_result"},
		{"AgentMsgBrowserNotAvailable", AgentMsgBrowserNotAvailable, "browser_not_available"},
		{"AgentMsgBrowserError", AgentMsgBrowserError, "browser_error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expected {
				t.Errorf("got %q, want %q", tt.got, tt.expected)
			}
		})
	}
}

// TestBrowserPayload_JSONRoundTrip verifies agent-browser payload JSON compatibility.
func TestBrowserPayload_JSONRoundTrip(t *testing.T) {
	t.Run("action payload", func(t *testing.T) {
		ref := 42
		payload := BrowserActionPayload{
			ExecutionID: "exec-browser-001",
			SessionID:   "sess-browser-001",
			Command:     "click",
			Ref:         &ref,
			Params: map[string]interface{}{
				"button": "left",
			},
		}

		data, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		var decoded BrowserActionPayload
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		if decoded.Command != "click" {
			t.Errorf("Command = %q, want click", decoded.Command)
		}
		if decoded.Ref == nil || *decoded.Ref != 42 {
			t.Fatalf("Ref = %v, want 42", decoded.Ref)
		}
		if decoded.Params["button"] != "left" {
			t.Errorf("Params[button] = %v, want left", decoded.Params["button"])
		}
	})

	t.Run("session and result payload", func(t *testing.T) {
		session := BrowserSessionPayload{
			ExecutionID: "exec-browser-002",
			SessionID:   "sess-browser-002",
			URL:         "https://example.com",
			Headless:    true,
			Status:      "ready",
		}
		result := BrowserResultPayload{
			ExecutionID: "exec-browser-002",
			SessionID:   "sess-browser-002",
			Success:     true,
			Snapshot:    "root: document",
			Screenshot:  "base64-data",
			Output:      "clicked button",
			DurationMs:  125,
		}

		sessionData, err := json.Marshal(session)
		if err != nil {
			t.Fatalf("session Marshal failed: %v", err)
		}
		resultData, err := json.Marshal(result)
		if err != nil {
			t.Fatalf("result Marshal failed: %v", err)
		}

		var decodedSession BrowserSessionPayload
		if err := json.Unmarshal(sessionData, &decodedSession); err != nil {
			t.Fatalf("session Unmarshal failed: %v", err)
		}
		var decodedResult BrowserResultPayload
		if err := json.Unmarshal(resultData, &decodedResult); err != nil {
			t.Fatalf("result Unmarshal failed: %v", err)
		}

		if decodedSession.Status != "ready" {
			t.Errorf("Status = %q, want ready", decodedSession.Status)
		}
		if decodedResult.Success != true {
			t.Errorf("Success = %v, want true", decodedResult.Success)
		}
		if decodedResult.DurationMs != 125 {
			t.Errorf("DurationMs = %d, want 125", decodedResult.DurationMs)
		}
	})
}

// TestMaxScreenshotSize verifies the maximum screenshot size constant is 2MB.
func TestMaxScreenshotSize(t *testing.T) {
	if MaxScreenshotSize != 2*1024*1024 {
		t.Errorf("MaxScreenshotSize = %d, want %d (2MB)", MaxScreenshotSize, 2*1024*1024)
	}
}

// TestComputerUseConstraintConstants verifies all computer use constraint constants.
func TestComputerUseConstraintConstants(t *testing.T) {
	if ComputerUseMaxIdleMin != 30 {
		t.Errorf("ComputerUseMaxIdleMin = %d, want 30", ComputerUseMaxIdleMin)
	}
	if ComputerUseMaxActiveHr != 2 {
		t.Errorf("ComputerUseMaxActiveHr = %d, want 2", ComputerUseMaxActiveHr)
	}
	if MaxConcurrentComputerSessions != 2 {
		t.Errorf("MaxConcurrentComputerSessions = %d, want 2", MaxConcurrentComputerSessions)
	}
}

// TestComputerActionPayload_Marshal tests JSON marshaling of ComputerActionPayload.
func TestComputerActionPayload_Marshal(t *testing.T) {
	t.Run("marshal with all fields", func(t *testing.T) {
		payload := ComputerActionPayload{
			ExecutionID: "exec-123",
			SessionID:   "sess-456",
			Action:      "click",
			Params: map[string]interface{}{
				"x": float64(100),
				"y": float64(200),
			},
		}

		data, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		var result map[string]interface{}
		if err := json.Unmarshal(data, &result); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		if result["execution_id"] != "exec-123" {
			t.Errorf("execution_id = %v, want exec-123", result["execution_id"])
		}
		if result["session_id"] != "sess-456" {
			t.Errorf("session_id = %v, want sess-456", result["session_id"])
		}
		if result["action"] != "click" {
			t.Errorf("action = %v, want click", result["action"])
		}

		params, ok := result["params"].(map[string]interface{})
		if !ok {
			t.Fatal("params is not a map")
		}
		if params["x"] != float64(100) {
			t.Errorf("params.x = %v, want 100", params["x"])
		}
		if params["y"] != float64(200) {
			t.Errorf("params.y = %v, want 200", params["y"])
		}
	})

	t.Run("marshal screenshot action", func(t *testing.T) {
		payload := ComputerActionPayload{
			ExecutionID: "exec-789",
			SessionID:   "sess-012",
			Action:      "screenshot",
			Params:      map[string]interface{}{},
		}

		data, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		s := string(data)
		if !strings.Contains(s, `"action":"screenshot"`) {
			t.Error("JSON should contain action:screenshot")
		}
		if !strings.Contains(s, `"execution_id":"exec-789"`) {
			t.Error("JSON should contain execution_id:exec-789")
		}
	})

	t.Run("marshal type action", func(t *testing.T) {
		payload := ComputerActionPayload{
			ExecutionID: "exec-001",
			SessionID:   "sess-001",
			Action:      "type",
			Params: map[string]interface{}{
				"text": "hello world",
			},
		}

		data, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		var result ComputerActionPayload
		if err := json.Unmarshal(data, &result); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		if result.Action != "type" {
			t.Errorf("Action = %q, want type", result.Action)
		}
		if result.Params["text"] != "hello world" {
			t.Errorf("Params[text] = %v, want hello world", result.Params["text"])
		}
	})

	t.Run("marshal navigate action", func(t *testing.T) {
		payload := ComputerActionPayload{
			ExecutionID: "exec-002",
			SessionID:   "sess-002",
			Action:      "navigate",
			Params: map[string]interface{}{
				"url": "https://example.com",
			},
		}

		data, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		var result ComputerActionPayload
		if err := json.Unmarshal(data, &result); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		if result.Action != "navigate" {
			t.Errorf("Action = %q, want navigate", result.Action)
		}
		if result.Params["url"] != "https://example.com" {
			t.Errorf("Params[url] = %v, want https://example.com", result.Params["url"])
		}
	})

	t.Run("marshal scroll action", func(t *testing.T) {
		payload := ComputerActionPayload{
			ExecutionID: "exec-003",
			SessionID:   "sess-003",
			Action:      "scroll",
			Params: map[string]interface{}{
				"direction": "down",
				"amount":    float64(300),
			},
		}

		data, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		var result ComputerActionPayload
		if err := json.Unmarshal(data, &result); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		if result.Action != "scroll" {
			t.Errorf("Action = %q, want scroll", result.Action)
		}
		if result.Params["direction"] != "down" {
			t.Errorf("Params[direction] = %v, want down", result.Params["direction"])
		}
		if result.Params["amount"] != float64(300) {
			t.Errorf("Params[amount] = %v, want 300", result.Params["amount"])
		}
	})
}

func TestAgentResponseRequestPayload_ToolLoopJSONRoundTrip(t *testing.T) {
	payload := AgentResponseRequestPayload{
		Type:         AgentMsgAgentResponseReq,
		ExecutionID:  "exec-agent-001",
		SystemPrompt: "You are the CEO agent.",
		Provider:     "openai",
		Model:        "gpt-5.4",
		MaxTokens:    2048,
		Timeout:      120,
		ResponseMode: "tool_loop",
		ToolLoopMessages: []ToolLoopMessage{
			{
				Role:    "user",
				Content: "Create a plan and call the right tool.",
			},
			{
				Role: "assistant",
				ToolCalls: []ToolLoopCall{
					{
						ID:    "call-1",
						Name:  "create_task",
						Input: json.RawMessage(`{"title":"Stabilize deploy"}`),
					},
				},
			},
			{
				Role: "tool",
				ToolResults: []ToolLoopResult{
					{
						ToolCallID: "call-1",
						ToolName:   "create_task",
						Content:    `{"task_id":"task-123"}`,
					},
				},
			},
		},
		ToolDefinitions: []ToolDefinition{
			{
				Name:        "create_task",
				Description: "Create a task in the workspace",
				InputSchema: json.RawMessage(`{"type":"object","properties":{"title":{"type":"string"}},"required":["title"]}`),
			},
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded AgentResponseRequestPayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.ResponseMode != "tool_loop" {
		t.Fatalf("ResponseMode = %q, want tool_loop", decoded.ResponseMode)
	}
	if len(decoded.ToolDefinitions) != 1 {
		t.Fatalf("ToolDefinitions len = %d, want 1", len(decoded.ToolDefinitions))
	}
	if string(decoded.ToolDefinitions[0].InputSchema) != `{"type":"object","properties":{"title":{"type":"string"}},"required":["title"]}` {
		t.Fatalf("unexpected tool schema: %s", string(decoded.ToolDefinitions[0].InputSchema))
	}
	if len(decoded.ToolLoopMessages) != 3 {
		t.Fatalf("ToolLoopMessages len = %d, want 3", len(decoded.ToolLoopMessages))
	}
	if decoded.ToolLoopMessages[1].ToolCalls[0].Name != "create_task" {
		t.Fatalf("ToolCalls[0].Name = %q, want create_task", decoded.ToolLoopMessages[1].ToolCalls[0].Name)
	}
	if decoded.ToolLoopMessages[2].ToolResults[0].ToolCallID != "call-1" {
		t.Fatalf("ToolResults[0].ToolCallID = %q, want call-1", decoded.ToolLoopMessages[2].ToolResults[0].ToolCallID)
	}
}

func TestAgentResponseCompletePayload_ToolCallsJSONRoundTrip(t *testing.T) {
	payload := AgentResponseCompletePayload{
		ExecutionID: "exec-agent-002",
		Output:      "Need to execute a tool before finalizing.",
		ExitCode:    0,
		Duration:    42,
		TokenUsage: &TokenUsage{
			InputTokens:  10,
			OutputTokens: 20,
			TotalTokens:  30,
		},
		Model:      "claude-sonnet-4-20250514",
		Provider:   "anthropic",
		StopReason: "tool_use",
		ToolCalls: []ToolLoopCall{
			{
				ID:    "call-2",
				Name:  "convene_meeting",
				Input: json.RawMessage(`{"topic":"Deploy review"}`),
			},
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded AgentResponseCompletePayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.Provider != "anthropic" {
		t.Fatalf("Provider = %q, want anthropic", decoded.Provider)
	}
	if decoded.StopReason != "tool_use" {
		t.Fatalf("StopReason = %q, want tool_use", decoded.StopReason)
	}
	if len(decoded.ToolCalls) != 1 {
		t.Fatalf("ToolCalls len = %d, want 1", len(decoded.ToolCalls))
	}
	if decoded.ToolCalls[0].Name != "convene_meeting" {
		t.Fatalf("ToolCalls[0].Name = %q, want convene_meeting", decoded.ToolCalls[0].Name)
	}
}

func TestIsCompatibleProtocolVersion(t *testing.T) {
	tests := []struct {
		version string
		want    bool
	}{
		{version: AgentProtocolVersion, want: true},
		{version: "1.1.7", want: true},
		{version: "1.2.0", want: false},
		{version: "2.0.0", want: false},
		{version: "", want: false},
		{version: "invalid", want: false},
	}

	for _, tt := range tests {
		if got := IsCompatibleProtocolVersion(tt.version); got != tt.want {
			t.Fatalf("IsCompatibleProtocolVersion(%q) = %v, want %v", tt.version, got, tt.want)
		}
	}
}

// TestComputerActionPayload_Unmarshal tests JSON unmarshaling of ComputerActionPayload.
func TestComputerActionPayload_Unmarshal(t *testing.T) {
	t.Run("unmarshal click action", func(t *testing.T) {
		jsonData := `{
			"execution_id": "exec-100",
			"session_id": "sess-200",
			"action": "click",
			"params": {"x": 50, "y": 75, "button": "left"}
		}`

		var payload ComputerActionPayload
		if err := json.Unmarshal([]byte(jsonData), &payload); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		if payload.ExecutionID != "exec-100" {
			t.Errorf("ExecutionID = %q, want exec-100", payload.ExecutionID)
		}
		if payload.SessionID != "sess-200" {
			t.Errorf("SessionID = %q, want sess-200", payload.SessionID)
		}
		if payload.Action != "click" {
			t.Errorf("Action = %q, want click", payload.Action)
		}
		if payload.Params["x"] != float64(50) {
			t.Errorf("Params[x] = %v, want 50", payload.Params["x"])
		}
		if payload.Params["y"] != float64(75) {
			t.Errorf("Params[y] = %v, want 75", payload.Params["y"])
		}
		if payload.Params["button"] != "left" {
			t.Errorf("Params[button] = %v, want left", payload.Params["button"])
		}
	})

	t.Run("unmarshal screenshot action with empty params", func(t *testing.T) {
		jsonData := `{
			"execution_id": "exec-101",
			"session_id": "sess-201",
			"action": "screenshot",
			"params": {}
		}`

		var payload ComputerActionPayload
		if err := json.Unmarshal([]byte(jsonData), &payload); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		if payload.Action != "screenshot" {
			t.Errorf("Action = %q, want screenshot", payload.Action)
		}
		if len(payload.Params) != 0 {
			t.Errorf("Params should be empty, got %v", payload.Params)
		}
	})

	t.Run("unmarshal with null params", func(t *testing.T) {
		jsonData := `{
			"execution_id": "exec-102",
			"session_id": "sess-202",
			"action": "screenshot",
			"params": null
		}`

		var payload ComputerActionPayload
		if err := json.Unmarshal([]byte(jsonData), &payload); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		if payload.Action != "screenshot" {
			t.Errorf("Action = %q, want screenshot", payload.Action)
		}
		if payload.Params != nil {
			t.Errorf("Params should be nil, got %v", payload.Params)
		}
	})

	t.Run("unmarshal type action with text", func(t *testing.T) {
		jsonData := `{
			"execution_id": "exec-103",
			"session_id": "sess-203",
			"action": "type",
			"params": {"text": "search query"}
		}`

		var payload ComputerActionPayload
		if err := json.Unmarshal([]byte(jsonData), &payload); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		if payload.Action != "type" {
			t.Errorf("Action = %q, want type", payload.Action)
		}
		if payload.Params["text"] != "search query" {
			t.Errorf("Params[text] = %v, want search query", payload.Params["text"])
		}
	})
}

// TestComputerResultPayload_Marshal tests JSON marshaling of ComputerResultPayload.
func TestComputerResultPayload_Marshal(t *testing.T) {
	t.Run("marshal success with screenshot", func(t *testing.T) {
		payload := ComputerResultPayload{
			ExecutionID: "exec-300",
			SessionID:   "sess-400",
			Success:     true,
			Screenshot:  "iVBORw0KGgoAAAANSUhEUg==",
			DurationMs:  150,
		}

		data, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		var result map[string]interface{}
		if err := json.Unmarshal(data, &result); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		if result["execution_id"] != "exec-300" {
			t.Errorf("execution_id = %v, want exec-300", result["execution_id"])
		}
		if result["session_id"] != "sess-400" {
			t.Errorf("session_id = %v, want sess-400", result["session_id"])
		}
		if result["success"] != true {
			t.Errorf("success = %v, want true", result["success"])
		}
		if result["screenshot"] != "iVBORw0KGgoAAAANSUhEUg==" {
			t.Errorf("screenshot = %v, want iVBORw0KGgoAAAANSUhEUg==", result["screenshot"])
		}
		if result["duration_ms"] != float64(150) {
			t.Errorf("duration_ms = %v, want 150", result["duration_ms"])
		}
	})

	t.Run("marshal success without screenshot - omitempty", func(t *testing.T) {
		payload := ComputerResultPayload{
			ExecutionID: "exec-301",
			SessionID:   "sess-401",
			Success:     true,
			DurationMs:  100,
		}

		data, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		if strings.Contains(string(data), "screenshot") {
			t.Error("empty screenshot should be omitted")
		}
	})

	t.Run("marshal failure with error - omitempty", func(t *testing.T) {
		payload := ComputerResultPayload{
			ExecutionID: "exec-302",
			SessionID:   "sess-402",
			Success:     false,
			Error:       "element not found",
			DurationMs:  50,
		}

		data, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		s := string(data)
		if !strings.Contains(s, `"error":"element not found"`) {
			t.Error("JSON should contain error message")
		}
		if strings.Contains(s, "screenshot") {
			t.Error("empty screenshot should be omitted")
		}
	})

	t.Run("marshal failure without error - omitempty", func(t *testing.T) {
		payload := ComputerResultPayload{
			ExecutionID: "exec-303",
			SessionID:   "sess-403",
			Success:     false,
			DurationMs:  25,
		}

		data, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		s := string(data)
		if strings.Contains(s, `"error"`) {
			t.Error("empty error should be omitted")
		}
		if strings.Contains(s, `"screenshot"`) {
			t.Error("empty screenshot should be omitted")
		}
	})
}

// TestComputerResultPayload_Unmarshal tests JSON unmarshaling of ComputerResultPayload.
func TestComputerResultPayload_Unmarshal(t *testing.T) {
	t.Run("unmarshal success with screenshot", func(t *testing.T) {
		jsonData := `{
			"execution_id": "exec-500",
			"session_id": "sess-600",
			"success": true,
			"screenshot": "base64data",
			"duration_ms": 200
		}`

		var payload ComputerResultPayload
		if err := json.Unmarshal([]byte(jsonData), &payload); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		if payload.ExecutionID != "exec-500" {
			t.Errorf("ExecutionID = %q, want exec-500", payload.ExecutionID)
		}
		if payload.SessionID != "sess-600" {
			t.Errorf("SessionID = %q, want sess-600", payload.SessionID)
		}
		if !payload.Success {
			t.Error("Success should be true")
		}
		if payload.Screenshot != "base64data" {
			t.Errorf("Screenshot = %q, want base64data", payload.Screenshot)
		}
		if payload.DurationMs != 200 {
			t.Errorf("DurationMs = %d, want 200", payload.DurationMs)
		}
		if payload.Error != "" {
			t.Errorf("Error should be empty, got %q", payload.Error)
		}
	})

	t.Run("unmarshal failure with error", func(t *testing.T) {
		jsonData := `{
			"execution_id": "exec-501",
			"session_id": "sess-601",
			"success": false,
			"error": "timeout waiting for page load",
			"duration_ms": 5000
		}`

		var payload ComputerResultPayload
		if err := json.Unmarshal([]byte(jsonData), &payload); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		if payload.Success {
			t.Error("Success should be false")
		}
		if payload.Error != "timeout waiting for page load" {
			t.Errorf("Error = %q, want timeout waiting for page load", payload.Error)
		}
		if payload.Screenshot != "" {
			t.Errorf("Screenshot should be empty, got %q", payload.Screenshot)
		}
	})

	t.Run("unmarshal without optional fields", func(t *testing.T) {
		jsonData := `{
			"execution_id": "exec-502",
			"session_id": "sess-602",
			"success": true,
			"duration_ms": 100
		}`

		var payload ComputerResultPayload
		if err := json.Unmarshal([]byte(jsonData), &payload); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		if !payload.Success {
			t.Error("Success should be true")
		}
		if payload.Screenshot != "" {
			t.Errorf("Screenshot should be empty, got %q", payload.Screenshot)
		}
		if payload.Error != "" {
			t.Errorf("Error should be empty, got %q", payload.Error)
		}
	})
}

// TestComputerSessionPayload_Marshal tests JSON marshaling of ComputerSessionPayload.
func TestComputerSessionPayload_Marshal(t *testing.T) {
	t.Run("marshal session start with all fields", func(t *testing.T) {
		payload := ComputerSessionPayload{
			ExecutionID: "exec-700",
			SessionID:   "sess-800",
			URL:         "https://example.com",
			ViewportW:   1280,
			ViewportH:   720,
			Headless:    true,
		}

		data, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		var result map[string]interface{}
		if err := json.Unmarshal(data, &result); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		if result["execution_id"] != "exec-700" {
			t.Errorf("execution_id = %v, want exec-700", result["execution_id"])
		}
		if result["session_id"] != "sess-800" {
			t.Errorf("session_id = %v, want sess-800", result["session_id"])
		}
		if result["url"] != "https://example.com" {
			t.Errorf("url = %v, want https://example.com", result["url"])
		}
		if result["viewport_w"] != float64(1280) {
			t.Errorf("viewport_w = %v, want 1280", result["viewport_w"])
		}
		if result["viewport_h"] != float64(720) {
			t.Errorf("viewport_h = %v, want 720", result["viewport_h"])
		}
		if result["headless"] != true {
			t.Errorf("headless = %v, want true", result["headless"])
		}
	})

	t.Run("marshal session without url - omitempty", func(t *testing.T) {
		payload := ComputerSessionPayload{
			ExecutionID: "exec-701",
			SessionID:   "sess-801",
			ViewportW:   1920,
			ViewportH:   1080,
			Headless:    false,
		}

		data, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		s := string(data)
		if strings.Contains(s, `"url"`) {
			t.Error("empty URL should be omitted")
		}
		if !strings.Contains(s, `"viewport_w":1920`) {
			t.Error("JSON should contain viewport_w:1920")
		}
		if !strings.Contains(s, `"viewport_h":1080`) {
			t.Error("JSON should contain viewport_h:1080")
		}
	})

	t.Run("marshal and unmarshal roundtrip", func(t *testing.T) {
		original := ComputerSessionPayload{
			ExecutionID: "exec-702",
			SessionID:   "sess-802",
			URL:         "https://test.example.com/app",
			ViewportW:   1024,
			ViewportH:   768,
			Headless:    false,
		}

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		var decoded ComputerSessionPayload
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		if !reflect.DeepEqual(original, decoded) {
			t.Errorf("roundtrip mismatch: got %+v, want %+v", decoded, original)
		}
	})
}

// TestComputerActionPayload_RoundTrip verifies marshal/unmarshal roundtrip for action payloads.
func TestComputerActionPayload_RoundTrip(t *testing.T) {
	original := ComputerActionPayload{
		ExecutionID: "exec-rt-1",
		SessionID:   "sess-rt-1",
		Action:      "click",
		Params: map[string]interface{}{
			"x":      float64(500),
			"y":      float64(300),
			"button": "left",
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded ComputerActionPayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if original.ExecutionID != decoded.ExecutionID {
		t.Errorf("ExecutionID = %q, want %q", decoded.ExecutionID, original.ExecutionID)
	}
	if original.SessionID != decoded.SessionID {
		t.Errorf("SessionID = %q, want %q", decoded.SessionID, original.SessionID)
	}
	if original.Action != decoded.Action {
		t.Errorf("Action = %q, want %q", decoded.Action, original.Action)
	}
	if original.Params["x"] != decoded.Params["x"] {
		t.Errorf("Params[x] = %v, want %v", decoded.Params["x"], original.Params["x"])
	}
	if original.Params["y"] != decoded.Params["y"] {
		t.Errorf("Params[y] = %v, want %v", decoded.Params["y"], original.Params["y"])
	}
	if original.Params["button"] != decoded.Params["button"] {
		t.Errorf("Params[button] = %v, want %v", decoded.Params["button"], original.Params["button"])
	}
}

// TestComputerResultPayload_RoundTrip verifies marshal/unmarshal roundtrip for result payloads.
func TestComputerResultPayload_RoundTrip(t *testing.T) {
	original := ComputerResultPayload{
		ExecutionID: "exec-rt-2",
		SessionID:   "sess-rt-2",
		Success:     true,
		Screenshot:  "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==",
		DurationMs:  350,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded ComputerResultPayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if !reflect.DeepEqual(original, decoded) {
		t.Errorf("roundtrip mismatch: got %+v, want %+v", decoded, original)
	}
}

// TestAgentMessageWithComputerPayload tests that computer payloads can be embedded in AgentMessage.
func TestAgentMessageWithComputerPayload(t *testing.T) {
	t.Run("agent message with computer action payload", func(t *testing.T) {
		actionPayload := ComputerActionPayload{
			ExecutionID: "exec-msg-1",
			SessionID:   "sess-msg-1",
			Action:      "click",
			Params:      map[string]interface{}{"x": float64(100), "y": float64(200)},
		}

		payloadBytes, err := json.Marshal(actionPayload)
		if err != nil {
			t.Fatalf("Marshal payload failed: %v", err)
		}

		msg := AgentMessage{
			Type:    AgentMsgComputerAction,
			ID:      "msg-001",
			Payload: json.RawMessage(payloadBytes),
		}

		data, err := json.Marshal(msg)
		if err != nil {
			t.Fatalf("Marshal message failed: %v", err)
		}

		var decoded AgentMessage
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal message failed: %v", err)
		}

		if decoded.Type != AgentMsgComputerAction {
			t.Errorf("Type = %q, want %q", decoded.Type, AgentMsgComputerAction)
		}

		var decodedPayload ComputerActionPayload
		if err := json.Unmarshal(decoded.Payload, &decodedPayload); err != nil {
			t.Fatalf("Unmarshal payload failed: %v", err)
		}

		if decodedPayload.Action != "click" {
			t.Errorf("Action = %q, want click", decodedPayload.Action)
		}
		if decodedPayload.ExecutionID != "exec-msg-1" {
			t.Errorf("ExecutionID = %q, want exec-msg-1", decodedPayload.ExecutionID)
		}
	})

	t.Run("agent message with computer result payload", func(t *testing.T) {
		resultPayload := ComputerResultPayload{
			ExecutionID: "exec-msg-2",
			SessionID:   "sess-msg-2",
			Success:     true,
			Screenshot:  "base64data",
			DurationMs:  100,
		}

		payloadBytes, err := json.Marshal(resultPayload)
		if err != nil {
			t.Fatalf("Marshal payload failed: %v", err)
		}

		msg := AgentMessage{
			Type:    AgentMsgComputerResult,
			ID:      "msg-002",
			Payload: json.RawMessage(payloadBytes),
		}

		data, err := json.Marshal(msg)
		if err != nil {
			t.Fatalf("Marshal message failed: %v", err)
		}

		var decoded AgentMessage
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal message failed: %v", err)
		}

		if decoded.Type != AgentMsgComputerResult {
			t.Errorf("Type = %q, want %q", decoded.Type, AgentMsgComputerResult)
		}

		var decodedPayload ComputerResultPayload
		if err := json.Unmarshal(decoded.Payload, &decodedPayload); err != nil {
			t.Fatalf("Unmarshal payload failed: %v", err)
		}

		if !decodedPayload.Success {
			t.Error("Success should be true")
		}
		if decodedPayload.Screenshot != "base64data" {
			t.Errorf("Screenshot = %q, want base64data", decodedPayload.Screenshot)
		}
	})

	t.Run("agent message with computer session payload", func(t *testing.T) {
		sessionPayload := ComputerSessionPayload{
			ExecutionID: "exec-msg-3",
			SessionID:   "sess-msg-3",
			URL:         "https://example.com",
			ViewportW:   1280,
			ViewportH:   720,
			Headless:    true,
		}

		payloadBytes, err := json.Marshal(sessionPayload)
		if err != nil {
			t.Fatalf("Marshal payload failed: %v", err)
		}

		msg := AgentMessage{
			Type:    AgentMsgComputerSessionStart,
			ID:      "msg-003",
			Payload: json.RawMessage(payloadBytes),
		}

		data, err := json.Marshal(msg)
		if err != nil {
			t.Fatalf("Marshal message failed: %v", err)
		}

		var decoded AgentMessage
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal message failed: %v", err)
		}

		if decoded.Type != AgentMsgComputerSessionStart {
			t.Errorf("Type = %q, want %q", decoded.Type, AgentMsgComputerSessionStart)
		}

		var decodedPayload ComputerSessionPayload
		if err := json.Unmarshal(decoded.Payload, &decodedPayload); err != nil {
			t.Fatalf("Unmarshal payload failed: %v", err)
		}

		if decodedPayload.URL != "https://example.com" {
			t.Errorf("URL = %q, want https://example.com", decodedPayload.URL)
		}
		if decodedPayload.ViewportW != 1280 {
			t.Errorf("ViewportW = %d, want 1280", decodedPayload.ViewportW)
		}
		if decodedPayload.ViewportH != 720 {
			t.Errorf("ViewportH = %d, want 720", decodedPayload.ViewportH)
		}
		if !decodedPayload.Headless {
			t.Error("Headless should be true")
		}
	})
}

// TestComputerPoolStatusMessageType은 풀 상태 메시지 타입 상수를 검증한다.
func TestComputerPoolStatusMessageType(t *testing.T) {
	if AgentMsgComputerPoolStatus != "computer_pool_status" {
		t.Errorf("AgentMsgComputerPoolStatus = %q, want %q", AgentMsgComputerPoolStatus, "computer_pool_status")
	}
}

// TestComputerResultPayload_ContainerID는 ContainerID 필드의 직렬화를 테스트한다.
func TestComputerResultPayload_ContainerID(t *testing.T) {
	t.Run("container_id 포함 시 직렬화", func(t *testing.T) {
		payload := ComputerResultPayload{
			ExecutionID: "exec-c1",
			SessionID:   "sess-c1",
			Success:     true,
			DurationMs:  100,
			ContainerID: "abc123def456",
		}

		data, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		s := string(data)
		if !strings.Contains(s, `"container_id":"abc123def456"`) {
			t.Errorf("JSON에 container_id가 포함되어야 함, got: %s", s)
		}

		// 역직렬화 검증
		var decoded ComputerResultPayload
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}
		if decoded.ContainerID != "abc123def456" {
			t.Errorf("ContainerID = %q, want %q", decoded.ContainerID, "abc123def456")
		}
	})

	t.Run("container_id 빈 문자열일 때 omitempty", func(t *testing.T) {
		payload := ComputerResultPayload{
			ExecutionID: "exec-c2",
			SessionID:   "sess-c2",
			Success:     true,
			DurationMs:  50,
		}

		data, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		if strings.Contains(string(data), "container_id") {
			t.Error("빈 container_id는 omitempty로 제외되어야 함")
		}
	})

	t.Run("하위 호환성: container_id 없는 JSON 역직렬화", func(t *testing.T) {
		// SPEC-COMPUTER-USE-001 형식의 기존 JSON (container_id 필드 없음)
		jsonData := `{
			"execution_id": "exec-old",
			"session_id": "sess-old",
			"success": true,
			"screenshot": "base64data",
			"duration_ms": 200
		}`

		var payload ComputerResultPayload
		if err := json.Unmarshal([]byte(jsonData), &payload); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		if payload.ExecutionID != "exec-old" {
			t.Errorf("ExecutionID = %q, want %q", payload.ExecutionID, "exec-old")
		}
		if payload.ContainerID != "" {
			t.Errorf("ContainerID는 빈 문자열이어야 함, got %q", payload.ContainerID)
		}
		if payload.Screenshot != "base64data" {
			t.Errorf("Screenshot = %q, want %q", payload.Screenshot, "base64data")
		}
	})
}

// TestComputerSessionPayload_ContainerID는 세션 페이로드의 ContainerID 직렬화를 테스트한다.
func TestComputerSessionPayload_ContainerID(t *testing.T) {
	t.Run("container_id 포함 시 직렬화", func(t *testing.T) {
		payload := ComputerSessionPayload{
			ExecutionID: "exec-sc1",
			SessionID:   "sess-sc1",
			ViewportW:   1280,
			ViewportH:   720,
			Headless:    true,
			ContainerID: "container-xyz789",
		}

		data, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		s := string(data)
		if !strings.Contains(s, `"container_id":"container-xyz789"`) {
			t.Errorf("JSON에 container_id가 포함되어야 함, got: %s", s)
		}

		var decoded ComputerSessionPayload
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}
		if decoded.ContainerID != "container-xyz789" {
			t.Errorf("ContainerID = %q, want %q", decoded.ContainerID, "container-xyz789")
		}
	})

	t.Run("container_id 빈 문자열일 때 omitempty", func(t *testing.T) {
		payload := ComputerSessionPayload{
			ExecutionID: "exec-sc2",
			SessionID:   "sess-sc2",
			ViewportW:   1920,
			ViewportH:   1080,
			Headless:    false,
		}

		data, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		if strings.Contains(string(data), "container_id") {
			t.Error("빈 container_id는 omitempty로 제외되어야 함")
		}
	})

	t.Run("하위 호환성: container_id 없는 JSON 역직렬화", func(t *testing.T) {
		// SPEC-COMPUTER-USE-001 형식의 기존 JSON
		jsonData := `{
			"execution_id": "exec-old-s",
			"session_id": "sess-old-s",
			"url": "https://example.com",
			"viewport_w": 1280,
			"viewport_h": 720,
			"headless": true
		}`

		var payload ComputerSessionPayload
		if err := json.Unmarshal([]byte(jsonData), &payload); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		if payload.ExecutionID != "exec-old-s" {
			t.Errorf("ExecutionID = %q, want %q", payload.ExecutionID, "exec-old-s")
		}
		if payload.ContainerID != "" {
			t.Errorf("ContainerID는 빈 문자열이어야 함, got %q", payload.ContainerID)
		}
		if payload.URL != "https://example.com" {
			t.Errorf("URL = %q, want %q", payload.URL, "https://example.com")
		}
	})
}

// TestComputerPoolStatusPayload_Marshal은 풀 상태 페이로드 직렬화를 테스트한다.
func TestComputerPoolStatusPayload_Marshal(t *testing.T) {
	t.Run("모든 필드 직렬화", func(t *testing.T) {
		payload := ComputerPoolStatusPayload{
			WarmCount:   2,
			ActiveCount: 3,
			MaxCount:    5,
		}

		data, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		var result map[string]interface{}
		if err := json.Unmarshal(data, &result); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		if result["warm_count"] != float64(2) {
			t.Errorf("warm_count = %v, want 2", result["warm_count"])
		}
		if result["active_count"] != float64(3) {
			t.Errorf("active_count = %v, want 3", result["active_count"])
		}
		if result["max_count"] != float64(5) {
			t.Errorf("max_count = %v, want 5", result["max_count"])
		}
	})

	t.Run("제로 값 직렬화", func(t *testing.T) {
		payload := ComputerPoolStatusPayload{
			WarmCount:   0,
			ActiveCount: 0,
			MaxCount:    10,
		}

		data, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		s := string(data)
		// 제로 값도 포함되어야 함 (omitempty 없음)
		if !strings.Contains(s, `"warm_count":0`) {
			t.Errorf("JSON에 warm_count:0이 포함되어야 함, got: %s", s)
		}
		if !strings.Contains(s, `"active_count":0`) {
			t.Errorf("JSON에 active_count:0이 포함되어야 함, got: %s", s)
		}
	})

	t.Run("역직렬화", func(t *testing.T) {
		jsonData := `{"warm_count":1,"active_count":4,"max_count":8}`

		var payload ComputerPoolStatusPayload
		if err := json.Unmarshal([]byte(jsonData), &payload); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		if payload.WarmCount != 1 {
			t.Errorf("WarmCount = %d, want 1", payload.WarmCount)
		}
		if payload.ActiveCount != 4 {
			t.Errorf("ActiveCount = %d, want 4", payload.ActiveCount)
		}
		if payload.MaxCount != 8 {
			t.Errorf("MaxCount = %d, want 8", payload.MaxCount)
		}
	})

	t.Run("라운드트립", func(t *testing.T) {
		original := ComputerPoolStatusPayload{
			WarmCount:   3,
			ActiveCount: 2,
			MaxCount:    10,
		}

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		var decoded ComputerPoolStatusPayload
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}

		if !reflect.DeepEqual(original, decoded) {
			t.Errorf("roundtrip mismatch: got %+v, want %+v", decoded, original)
		}
	})

	t.Run("AgentMessage 내 풀 상태 페이로드", func(t *testing.T) {
		poolPayload := ComputerPoolStatusPayload{
			WarmCount:   1,
			ActiveCount: 2,
			MaxCount:    5,
		}

		payloadBytes, err := json.Marshal(poolPayload)
		if err != nil {
			t.Fatalf("Marshal payload failed: %v", err)
		}

		msg := AgentMessage{
			Type:    AgentMsgComputerPoolStatus,
			ID:      "msg-pool-001",
			Payload: json.RawMessage(payloadBytes),
		}

		data, err := json.Marshal(msg)
		if err != nil {
			t.Fatalf("Marshal message failed: %v", err)
		}

		var decoded AgentMessage
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("Unmarshal message failed: %v", err)
		}

		if decoded.Type != AgentMsgComputerPoolStatus {
			t.Errorf("Type = %q, want %q", decoded.Type, AgentMsgComputerPoolStatus)
		}

		var decodedPayload ComputerPoolStatusPayload
		if err := json.Unmarshal(decoded.Payload, &decodedPayload); err != nil {
			t.Fatalf("Unmarshal payload failed: %v", err)
		}

		if decodedPayload.WarmCount != 1 {
			t.Errorf("WarmCount = %d, want 1", decodedPayload.WarmCount)
		}
		if decodedPayload.ActiveCount != 2 {
			t.Errorf("ActiveCount = %d, want 2", decodedPayload.ActiveCount)
		}
		if decodedPayload.MaxCount != 5 {
			t.Errorf("MaxCount = %d, want 5", decodedPayload.MaxCount)
		}
	})
}
