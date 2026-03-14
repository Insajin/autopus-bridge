package protocol_test

import (
	"encoding/json"
	"testing"

	"github.com/insajin/autopus-codex-rpc/protocol"
)

// TestCapabilities_OptOutNotificationMethods는 Capabilities 구조체의
// OptOutNotificationMethods 필드 직렬화를 검증한다.
func TestCapabilities_OptOutNotificationMethods(t *testing.T) {
	t.Run("OptOutNotificationMethods 있는 경우 직렬화", func(t *testing.T) {
		caps := protocol.Capabilities{
			ExperimentalApi:           true,
			OptOutNotificationMethods: []string{"turn/plan/updated", "turn/diff/updated"},
		}

		data, err := json.Marshal(caps)
		if err != nil {
			t.Fatalf("직렬화 실패: %v", err)
		}

		var decoded protocol.Capabilities
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("역직렬화 실패: %v", err)
		}

		if !decoded.ExperimentalApi {
			t.Error("ExperimentalApi가 true여야 함")
		}
		if len(decoded.OptOutNotificationMethods) != 2 {
			t.Errorf("OptOutNotificationMethods 길이 불일치: got %d, want 2", len(decoded.OptOutNotificationMethods))
		}
		if decoded.OptOutNotificationMethods[0] != "turn/plan/updated" {
			t.Errorf("첫 번째 메서드 불일치: got %q, want %q", decoded.OptOutNotificationMethods[0], "turn/plan/updated")
		}
	})

	t.Run("OptOutNotificationMethods 없을 때 omitempty", func(t *testing.T) {
		caps := protocol.Capabilities{
			ExperimentalApi: true,
		}

		data, err := json.Marshal(caps)
		if err != nil {
			t.Fatalf("직렬화 실패: %v", err)
		}

		var raw map[string]json.RawMessage
		if err := json.Unmarshal(data, &raw); err != nil {
			t.Fatalf("raw 역직렬화 실패: %v", err)
		}
		// OptOutNotificationMethods가 nil이면 omitempty로 필드 없어야 함
		if _, ok := raw["optOutNotificationMethods"]; ok {
			t.Error("optOutNotificationMethods 필드가 없어야 하지만 존재함 (omitempty)")
		}
	})

	t.Run("하위 호환성: ExperimentalApi만 있는 기존 JSON 역직렬화", func(t *testing.T) {
		raw := `{"experimentalApi":true}`
		var caps protocol.Capabilities
		if err := json.Unmarshal([]byte(raw), &caps); err != nil {
			t.Fatalf("역직렬화 실패: %v", err)
		}
		if !caps.ExperimentalApi {
			t.Error("ExperimentalApi가 true여야 함")
		}
		if caps.OptOutNotificationMethods != nil {
			t.Errorf("OptOutNotificationMethods가 nil이어야 함, got: %v", caps.OptOutNotificationMethods)
		}
	})
}

// TestCollabToolCallItem는 CollabToolCallItem 타입의 직렬화를 검증한다.
func TestCollabToolCallItem(t *testing.T) {
	t.Run("기본 CollabToolCallItem 직렬화", func(t *testing.T) {
		args := json.RawMessage(`{"path":"/tmp/test.go"}`)
		item := protocol.CollabToolCallItem{
			Type:      "tool_call",
			ID:        "item_abc",
			Name:      "read_file",
			Arguments: args,
			CallID:    "call_123",
		}

		data, err := json.Marshal(item)
		if err != nil {
			t.Fatalf("직렬화 실패: %v", err)
		}

		var decoded protocol.CollabToolCallItem
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("역직렬화 실패: %v", err)
		}

		if decoded.Type != "tool_call" {
			t.Errorf("Type 불일치: got %q, want %q", decoded.Type, "tool_call")
		}
		if decoded.ID != "item_abc" {
			t.Errorf("ID 불일치: got %q, want %q", decoded.ID, "item_abc")
		}
		if decoded.Name != "read_file" {
			t.Errorf("Name 불일치: got %q, want %q", decoded.Name, "read_file")
		}
		if decoded.CallID != "call_123" {
			t.Errorf("CallID 불일치: got %q, want %q", decoded.CallID, "call_123")
		}
	})

	t.Run("선택 필드 없을 때 omitempty", func(t *testing.T) {
		item := protocol.CollabToolCallItem{
			Type: "tool_call",
			Name: "list_files",
		}

		data, err := json.Marshal(item)
		if err != nil {
			t.Fatalf("직렬화 실패: %v", err)
		}

		var raw map[string]json.RawMessage
		if err := json.Unmarshal(data, &raw); err != nil {
			t.Fatalf("raw 역직렬화 실패: %v", err)
		}

		// ID, Arguments, CallID는 omitempty로 없어야 함
		if _, ok := raw["id"]; ok {
			t.Error("id 필드가 없어야 하지만 존재함 (omitempty)")
		}
		if _, ok := raw["arguments"]; ok {
			t.Error("arguments 필드가 없어야 하지만 존재함 (omitempty)")
		}
		if _, ok := raw["callId"]; ok {
			t.Error("callId 필드가 없어야 하지만 존재함 (omitempty)")
		}
	})

	t.Run("Arguments는 임의의 JSON이어야 함", func(t *testing.T) {
		nestedArgs := json.RawMessage(`{"files":["a.go","b.go"],"recursive":true}`)
		item := protocol.CollabToolCallItem{
			Type:      "tool_call",
			Name:      "list_files",
			Arguments: nestedArgs,
		}

		data, err := json.Marshal(item)
		if err != nil {
			t.Fatalf("직렬화 실패: %v", err)
		}

		var decoded protocol.CollabToolCallItem
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("역직렬화 실패: %v", err)
		}

		// Arguments가 원래 JSON 구조를 유지해야 함
		var origArgs, gotArgs map[string]interface{}
		json.Unmarshal(nestedArgs, &origArgs)
		json.Unmarshal(decoded.Arguments, &gotArgs)

		if gotArgs["recursive"] != origArgs["recursive"] {
			t.Errorf("Arguments.recursive 불일치: got %v, want %v", gotArgs["recursive"], origArgs["recursive"])
		}
	})
}

// TestTurnStartParams_Extension은 TurnStartParams 확장 필드 직렬화를 검증한다.
func TestTurnStartParams_Extension(t *testing.T) {
	t.Run("확장 필드 직렬화", func(t *testing.T) {
		schema := json.RawMessage(`{"type":"object","properties":{}}`)
		devInstructions := "항상 한국어로 답하라"
		params := protocol.TurnStartParams{
			ThreadID:         "thread_123",
			Input:            []protocol.TurnInput{{Type: "text", Text: "hello"}},
			Effort:           "high",
			Summary:          "auto",
			CollaborationMode: "sequential",
			OutputSchema:     schema,
			Settings: &protocol.TurnSettings{
				DeveloperInstructions: &devInstructions,
			},
		}

		data, err := json.Marshal(params)
		if err != nil {
			t.Fatalf("직렬화 실패: %v", err)
		}

		var decoded protocol.TurnStartParams
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("역직렬화 실패: %v", err)
		}

		if decoded.Effort != "high" {
			t.Errorf("Effort 불일치: got %q, want %q", decoded.Effort, "high")
		}
		if decoded.Summary != "auto" {
			t.Errorf("Summary 불일치: got %q, want %q", decoded.Summary, "auto")
		}
		if decoded.CollaborationMode != "sequential" {
			t.Errorf("CollaborationMode 불일치: got %q, want %q", decoded.CollaborationMode, "sequential")
		}
		if decoded.Settings == nil {
			t.Fatal("Settings가 nil이면 안 됨")
		}
		if decoded.Settings.DeveloperInstructions == nil {
			t.Fatal("Settings.DeveloperInstructions가 nil이면 안 됨")
		}
		if *decoded.Settings.DeveloperInstructions != devInstructions {
			t.Errorf("Settings.DeveloperInstructions 불일치: got %q, want %q",
				*decoded.Settings.DeveloperInstructions, devInstructions)
		}
	})

	t.Run("하위 호환성: 기존 필드만 있는 경우", func(t *testing.T) {
		raw := `{"threadId":"t1","input":[{"type":"text","text":"hi"}]}`
		var params protocol.TurnStartParams
		if err := json.Unmarshal([]byte(raw), &params); err != nil {
			t.Fatalf("역직렬화 실패: %v", err)
		}
		if params.ThreadID != "t1" {
			t.Errorf("ThreadID 불일치: got %q, want %q", params.ThreadID, "t1")
		}
		// 확장 필드는 기본값(빈 값)이어야 함
		if params.Effort != "" {
			t.Errorf("Effort가 빈 값이어야 함, got %q", params.Effort)
		}
		if params.Settings != nil {
			t.Errorf("Settings가 nil이어야 함, got %v", params.Settings)
		}
	})

	t.Run("확장 필드 없을 때 omitempty", func(t *testing.T) {
		params := protocol.TurnStartParams{
			ThreadID: "thread_456",
			Input:    []protocol.TurnInput{{Type: "text", Text: "test"}},
		}

		data, err := json.Marshal(params)
		if err != nil {
			t.Fatalf("직렬화 실패: %v", err)
		}

		var raw map[string]json.RawMessage
		if err := json.Unmarshal(data, &raw); err != nil {
			t.Fatalf("역직렬화 실패: %v", err)
		}

		for _, field := range []string{"effort", "summary", "collaborationMode", "outputSchema", "settings"} {
			if _, ok := raw[field]; ok {
				t.Errorf("%s 필드가 없어야 하지만 존재함 (omitempty)", field)
			}
		}
	})
}

// TestTurnSteerParams는 TurnSteerParams 직렬화를 검증한다.
func TestTurnSteerParams(t *testing.T) {
	t.Run("TurnSteerParams 직렬화", func(t *testing.T) {
		input := json.RawMessage(`[{"type":"text","text":"방향 전환"}]`)
		params := protocol.TurnSteerParams{
			ThreadID: "thread_789",
			Input:    input,
		}

		data, err := json.Marshal(params)
		if err != nil {
			t.Fatalf("직렬화 실패: %v", err)
		}

		var decoded protocol.TurnSteerParams
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("역직렬화 실패: %v", err)
		}

		if decoded.ThreadID != "thread_789" {
			t.Errorf("ThreadID 불일치: got %q, want %q", decoded.ThreadID, "thread_789")
		}
		if string(decoded.Input) != string(input) {
			t.Errorf("Input 불일치: got %q, want %q", string(decoded.Input), string(input))
		}
	})

	t.Run("TurnSteerParams JSON 키 검증", func(t *testing.T) {
		params := protocol.TurnSteerParams{
			ThreadID: "thread_abc",
			Input:    json.RawMessage(`"text input"`),
		}

		data, err := json.Marshal(params)
		if err != nil {
			t.Fatalf("직렬화 실패: %v", err)
		}

		var raw map[string]json.RawMessage
		if err := json.Unmarshal(data, &raw); err != nil {
			t.Fatalf("raw 역직렬화 실패: %v", err)
		}

		if _, ok := raw["threadId"]; !ok {
			t.Error("threadId 필드가 있어야 함")
		}
		if _, ok := raw["input"]; !ok {
			t.Error("input 필드가 있어야 함")
		}
	})
}

// TestThreadManagementTypes는 Thread 관리 타입들의 직렬화를 검증한다.
func TestThreadManagementTypes(t *testing.T) {
	t.Run("ThreadForkParams 직렬화", func(t *testing.T) {
		params := protocol.ThreadForkParams{
			ThreadID: "thread_original",
		}

		data, err := json.Marshal(params)
		if err != nil {
			t.Fatalf("직렬화 실패: %v", err)
		}

		var decoded protocol.ThreadForkParams
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("역직렬화 실패: %v", err)
		}
		if decoded.ThreadID != "thread_original" {
			t.Errorf("ThreadID 불일치: got %q, want %q", decoded.ThreadID, "thread_original")
		}
	})

	t.Run("ThreadForkResult 직렬화", func(t *testing.T) {
		result := protocol.ThreadForkResult{
			ThreadID: "thread_forked",
		}

		data, err := json.Marshal(result)
		if err != nil {
			t.Fatalf("직렬화 실패: %v", err)
		}

		var decoded protocol.ThreadForkResult
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("역직렬화 실패: %v", err)
		}
		if decoded.ThreadID != "thread_forked" {
			t.Errorf("ThreadID 불일치: got %q, want %q", decoded.ThreadID, "thread_forked")
		}
	})

	t.Run("ThreadReadParams 직렬화", func(t *testing.T) {
		params := protocol.ThreadReadParams{
			ThreadID: "thread_abc",
		}

		data, err := json.Marshal(params)
		if err != nil {
			t.Fatalf("직렬화 실패: %v", err)
		}

		var decoded protocol.ThreadReadParams
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("역직렬화 실패: %v", err)
		}
		if decoded.ThreadID != "thread_abc" {
			t.Errorf("ThreadID 불일치: got %q, want %q", decoded.ThreadID, "thread_abc")
		}
	})

	t.Run("ThreadReadResult 직렬화", func(t *testing.T) {
		result := protocol.ThreadReadResult{
			ThreadID: "thread_abc",
			Status:   "running",
			Items:    json.RawMessage(`[{"type":"agentMessage"}]`),
		}

		data, err := json.Marshal(result)
		if err != nil {
			t.Fatalf("직렬화 실패: %v", err)
		}

		var decoded protocol.ThreadReadResult
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("역직렬화 실패: %v", err)
		}
		if decoded.ThreadID != "thread_abc" {
			t.Errorf("ThreadID 불일치: got %q, want %q", decoded.ThreadID, "thread_abc")
		}
		if decoded.Status != "running" {
			t.Errorf("Status 불일치: got %q, want %q", decoded.Status, "running")
		}
	})

	t.Run("ThreadListParams omitempty 검증", func(t *testing.T) {
		params := protocol.ThreadListParams{}

		data, err := json.Marshal(params)
		if err != nil {
			t.Fatalf("직렬화 실패: %v", err)
		}

		var raw map[string]json.RawMessage
		if err := json.Unmarshal(data, &raw); err != nil {
			t.Fatalf("역직렬화 실패: %v", err)
		}

		// 빈 필드는 omitempty로 없어야 함
		if _, ok := raw["limit"]; ok {
			t.Error("limit 필드가 없어야 하지만 존재함 (omitempty)")
		}
	})

	t.Run("ThreadListResult 직렬화", func(t *testing.T) {
		result := protocol.ThreadListResult{
			Threads: json.RawMessage(`[{"threadId":"t1"},{"threadId":"t2"}]`),
		}

		data, err := json.Marshal(result)
		if err != nil {
			t.Fatalf("직렬화 실패: %v", err)
		}

		var decoded protocol.ThreadListResult
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("역직렬화 실패: %v", err)
		}
		if decoded.Threads == nil {
			t.Error("Threads가 nil이면 안 됨")
		}
	})

	t.Run("ThreadRollbackParams 직렬화", func(t *testing.T) {
		params := protocol.ThreadRollbackParams{
			ThreadID: "thread_to_rollback",
			TurnID:   "turn_checkpoint",
		}

		data, err := json.Marshal(params)
		if err != nil {
			t.Fatalf("직렬화 실패: %v", err)
		}

		var decoded protocol.ThreadRollbackParams
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("역직렬화 실패: %v", err)
		}
		if decoded.ThreadID != "thread_to_rollback" {
			t.Errorf("ThreadID 불일치: got %q, want %q", decoded.ThreadID, "thread_to_rollback")
		}
		if decoded.TurnID != "turn_checkpoint" {
			t.Errorf("TurnID 불일치: got %q, want %q", decoded.TurnID, "turn_checkpoint")
		}
	})

	t.Run("ThreadRollbackResult omitempty 검증", func(t *testing.T) {
		result := protocol.ThreadRollbackResult{}

		data, err := json.Marshal(result)
		if err != nil {
			t.Fatalf("직렬화 실패: %v", err)
		}

		if string(data) != "{}" {
			// omitempty 구조체는 빈 JSON 객체여야 함
			var raw map[string]json.RawMessage
			json.Unmarshal(data, &raw)
			if len(raw) > 0 {
				t.Errorf("빈 ThreadRollbackResult가 빈 객체여야 함, got: %s", string(data))
			}
		}
	})
}

// TestModelListResult는 ModelListResult 직렬화를 검증한다.
func TestModelListResult(t *testing.T) {
	t.Run("ModelListResult 직렬화", func(t *testing.T) {
		result := protocol.ModelListResult{
			Models: []protocol.ModelInfo{
				{
					ID:          "o4-mini",
					Name:        "GPT-4o mini",
					Description: "빠른 소형 모델",
				},
				{
					ID:   "o3",
					Name: "GPT-o3",
				},
			},
		}

		data, err := json.Marshal(result)
		if err != nil {
			t.Fatalf("직렬화 실패: %v", err)
		}

		var decoded protocol.ModelListResult
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("역직렬화 실패: %v", err)
		}

		if len(decoded.Models) != 2 {
			t.Errorf("Models 길이 불일치: got %d, want 2", len(decoded.Models))
		}
		if decoded.Models[0].ID != "o4-mini" {
			t.Errorf("첫 번째 모델 ID 불일치: got %q, want %q", decoded.Models[0].ID, "o4-mini")
		}
	})

	t.Run("ModelInfo omitempty 검증", func(t *testing.T) {
		info := protocol.ModelInfo{
			ID:   "gpt-5",
			Name: "GPT-5",
		}

		data, err := json.Marshal(info)
		if err != nil {
			t.Fatalf("직렬화 실패: %v", err)
		}

		var raw map[string]json.RawMessage
		if err := json.Unmarshal(data, &raw); err != nil {
			t.Fatalf("역직렬화 실패: %v", err)
		}

		if _, ok := raw["description"]; ok {
			t.Error("description 필드가 없어야 하지만 존재함 (omitempty)")
		}
	})
}

// TestReviewTypes는 Review 관련 타입 직렬화를 검증한다.
func TestReviewTypes(t *testing.T) {
	t.Run("ReviewStartParams 직렬화", func(t *testing.T) {
		params := protocol.ReviewStartParams{
			ThreadID: "thread_review",
			Prompt:   "코드를 리뷰해줘",
		}

		data, err := json.Marshal(params)
		if err != nil {
			t.Fatalf("직렬화 실패: %v", err)
		}

		var decoded protocol.ReviewStartParams
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("역직렬화 실패: %v", err)
		}

		if decoded.ThreadID != "thread_review" {
			t.Errorf("ThreadID 불일치: got %q, want %q", decoded.ThreadID, "thread_review")
		}
		if decoded.Prompt != "코드를 리뷰해줘" {
			t.Errorf("Prompt 불일치: got %q, want %q", decoded.Prompt, "코드를 리뷰해줘")
		}
	})

	t.Run("ReviewStartResult 직렬화", func(t *testing.T) {
		result := protocol.ReviewStartResult{
			ReviewID: "review_001",
		}

		data, err := json.Marshal(result)
		if err != nil {
			t.Fatalf("직렬화 실패: %v", err)
		}

		var decoded protocol.ReviewStartResult
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("역직렬화 실패: %v", err)
		}

		if decoded.ReviewID != "review_001" {
			t.Errorf("ReviewID 불일치: got %q, want %q", decoded.ReviewID, "review_001")
		}
	})
}

// TestConfigTypes는 Config 관련 타입 직렬화를 검증한다.
func TestConfigTypes(t *testing.T) {
	t.Run("ConfigReadResult 직렬화", func(t *testing.T) {
		result := protocol.ConfigReadResult{
			Config: json.RawMessage(`{"model":"o4-mini","sandbox":"read-only"}`),
		}

		data, err := json.Marshal(result)
		if err != nil {
			t.Fatalf("직렬화 실패: %v", err)
		}

		var decoded protocol.ConfigReadResult
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("역직렬화 실패: %v", err)
		}

		if decoded.Config == nil {
			t.Error("Config가 nil이면 안 됨")
		}
	})

	t.Run("ConfigWriteParams 직렬화", func(t *testing.T) {
		params := protocol.ConfigWriteParams{
			Key:   "model",
			Value: json.RawMessage(`"o3-mini"`),
		}

		data, err := json.Marshal(params)
		if err != nil {
			t.Fatalf("직렬화 실패: %v", err)
		}

		var decoded protocol.ConfigWriteParams
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("역직렬화 실패: %v", err)
		}

		if decoded.Key != "model" {
			t.Errorf("Key 불일치: got %q, want %q", decoded.Key, "model")
		}
	})

	t.Run("ConfigBatchWriteParams 직렬화", func(t *testing.T) {
		params := protocol.ConfigBatchWriteParams{
			Updates: map[string]json.RawMessage{
				"model":   json.RawMessage(`"o4-mini"`),
				"sandbox": json.RawMessage(`"read-only"`),
			},
		}

		data, err := json.Marshal(params)
		if err != nil {
			t.Fatalf("직렬화 실패: %v", err)
		}

		var decoded protocol.ConfigBatchWriteParams
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("역직렬화 실패: %v", err)
		}

		if len(decoded.Updates) != 2 {
			t.Errorf("Updates 길이 불일치: got %d, want 2", len(decoded.Updates))
		}
	})
}

// TestUtilityListTypes는 유틸리티 목록 타입들의 직렬화를 검증한다.
func TestUtilityListTypes(t *testing.T) {
	t.Run("SkillsListResult 직렬화", func(t *testing.T) {
		result := protocol.SkillsListResult{
			Skills: []protocol.SkillInfo{
				{ID: "skill_1", Name: "Python Expert", Enabled: true},
				{ID: "skill_2", Name: "Go Expert", Enabled: false},
			},
		}

		data, err := json.Marshal(result)
		if err != nil {
			t.Fatalf("직렬화 실패: %v", err)
		}

		var decoded protocol.SkillsListResult
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("역직렬화 실패: %v", err)
		}

		if len(decoded.Skills) != 2 {
			t.Errorf("Skills 길이 불일치: got %d, want 2", len(decoded.Skills))
		}
		if !decoded.Skills[0].Enabled {
			t.Error("첫 번째 Skill이 Enabled여야 함")
		}
	})

	t.Run("McpServerStatusListResult 직렬화", func(t *testing.T) {
		result := protocol.McpServerStatusListResult{
			Servers: []protocol.McpServerStatus{
				{Name: "filesystem", Status: "connected"},
				{Name: "git", Status: "disconnected"},
			},
		}

		data, err := json.Marshal(result)
		if err != nil {
			t.Fatalf("직렬화 실패: %v", err)
		}

		var decoded protocol.McpServerStatusListResult
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("역직렬화 실패: %v", err)
		}

		if len(decoded.Servers) != 2 {
			t.Errorf("Servers 길이 불일치: got %d, want 2", len(decoded.Servers))
		}
		if decoded.Servers[0].Status != "connected" {
			t.Errorf("첫 번째 서버 Status 불일치: got %q, want %q", decoded.Servers[0].Status, "connected")
		}
	})

	t.Run("ExperimentalFeatureListResult 직렬화", func(t *testing.T) {
		result := protocol.ExperimentalFeatureListResult{
			Features: []protocol.ExperimentalFeature{
				{ID: "dynamic_tools", Name: "Dynamic Tools", Enabled: true},
			},
		}

		data, err := json.Marshal(result)
		if err != nil {
			t.Fatalf("직렬화 실패: %v", err)
		}

		var decoded protocol.ExperimentalFeatureListResult
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("역직렬화 실패: %v", err)
		}

		if len(decoded.Features) != 1 {
			t.Errorf("Features 길이 불일치: got %d, want 1", len(decoded.Features))
		}
		if !decoded.Features[0].Enabled {
			t.Error("Feature가 Enabled여야 함")
		}
	})
}
