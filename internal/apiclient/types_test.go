// Package apiclient는 CLI 명령에서 사용하는 API 클라이언트 유틸리티를 제공합니다.
package apiclient_test

import (
	"encoding/json"
	"testing"

	"github.com/insajin/autopus-bridge/internal/apiclient"
)

// TestAPIResponse_JSONRoundtrip은 APIResponse의 JSON 마샬링/언마샬링을 검증합니다.
func TestAPIResponse_JSONRoundtrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr bool
		check   func(t *testing.T, resp apiclient.APIResponse)
	}{
		{
			name:  "성공 응답 (단건 data)",
			input: `{"success":true,"data":{"id":"123","name":"test"}}`,
			check: func(t *testing.T, resp apiclient.APIResponse) {
				t.Helper()
				if !resp.Success {
					t.Errorf("Success = %v, want true", resp.Success)
				}
				if resp.Error != nil {
					t.Errorf("Error = %v, want nil", resp.Error)
				}
				if len(resp.Data) == 0 {
					t.Error("Data가 비어있습니다")
				}
			},
		},
		{
			name:  "에러 응답",
			input: `{"success":false,"error":"NOT_FOUND","message":"리소스를 찾을 수 없습니다"}`,
			check: func(t *testing.T, resp apiclient.APIResponse) {
				t.Helper()
				if resp.Success {
					t.Errorf("Success = %v, want false", resp.Success)
				}
				if resp.Error == nil || resp.Error.Message != "NOT_FOUND" {
					t.Errorf("Error = %+v, want message NOT_FOUND", resp.Error)
				}
				if resp.Message != "리소스를 찾을 수 없습니다" {
					t.Errorf("Message = %q, want 리소스를 찾을 수 없습니다", resp.Message)
				}
			},
		},
		{
			name:  "구조화된 에러 응답",
			input: `{"success":false,"error":{"code":"BAD_REQUEST","message":"잘못된 요청입니다"}}`,
			check: func(t *testing.T, resp apiclient.APIResponse) {
				t.Helper()
				if resp.Success {
					t.Errorf("Success = %v, want false", resp.Success)
				}
				if resp.Error == nil {
					t.Fatal("Error가 nil입니다")
				}
				if resp.Error.Code != "BAD_REQUEST" {
					t.Errorf("Error.Code = %q, want BAD_REQUEST", resp.Error.Code)
				}
				if resp.Error.Message != "잘못된 요청입니다" {
					t.Errorf("Error.Message = %q, want 잘못된 요청입니다", resp.Error.Message)
				}
			},
		},
		{
			name:  "페이지네이션 응답 (meta 포함)",
			input: `{"success":true,"data":[1,2,3],"meta":{"page":1,"page_size":10,"total":100,"total_pages":10}}`,
			check: func(t *testing.T, resp apiclient.APIResponse) {
				t.Helper()
				if !resp.Success {
					t.Errorf("Success = %v, want true", resp.Success)
				}
				if resp.Meta == nil {
					t.Fatal("Meta가 nil입니다")
				}
				if resp.Meta.Page != 1 {
					t.Errorf("Meta.Page = %d, want 1", resp.Meta.Page)
				}
				if resp.Meta.PageSize != 10 {
					t.Errorf("Meta.PageSize = %d, want 10", resp.Meta.PageSize)
				}
				if resp.Meta.Total != 100 {
					t.Errorf("Meta.Total = %d, want 100", resp.Meta.Total)
				}
				if resp.Meta.TotalPages != 10 {
					t.Errorf("Meta.TotalPages = %d, want 10", resp.Meta.TotalPages)
				}
			},
		},
		{
			name:  "meta 없는 응답",
			input: `{"success":true,"data":[]}`,
			check: func(t *testing.T, resp apiclient.APIResponse) {
				t.Helper()
				if resp.Meta != nil {
					t.Errorf("Meta = %v, want nil", resp.Meta)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var resp apiclient.APIResponse
			if err := json.Unmarshal([]byte(tt.input), &resp); err != nil {
				if !tt.wantErr {
					t.Fatalf("언마샬링 실패: %v", err)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("에러를 기대했지만 성공했습니다")
			}
			tt.check(t, resp)

			// 마샬링 후 재언마샬링 검증
			data, err := json.Marshal(resp)
			if err != nil {
				t.Fatalf("마샬링 실패: %v", err)
			}
			var resp2 apiclient.APIResponse
			if err := json.Unmarshal(data, &resp2); err != nil {
				t.Fatalf("재언마샬링 실패: %v", err)
			}
			tt.check(t, resp2)
		})
	}
}

// TestPageMeta_JSONRoundtrip은 PageMeta의 JSON 마샬링/언마샬링을 검증합니다.
func TestPageMeta_JSONRoundtrip(t *testing.T) {
	t.Parallel()

	meta := apiclient.PageMeta{
		Page:       2,
		PageSize:   20,
		Total:      200,
		TotalPages: 10,
	}

	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("PageMeta 마샬링 실패: %v", err)
	}

	var got apiclient.PageMeta
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("PageMeta 언마샬링 실패: %v", err)
	}

	if got.Page != meta.Page {
		t.Errorf("Page = %d, want %d", got.Page, meta.Page)
	}
	if got.PageSize != meta.PageSize {
		t.Errorf("PageSize = %d, want %d", got.PageSize, meta.PageSize)
	}
	if got.Total != meta.Total {
		t.Errorf("Total = %d, want %d", got.Total, meta.Total)
	}
	if got.TotalPages != meta.TotalPages {
		t.Errorf("TotalPages = %d, want %d", got.TotalPages, meta.TotalPages)
	}
}

// TestAgentTypingPayload_JSONRoundtrip은 AgentTypingPayload의 JSON 마샬링/언마샬링을 검증합니다.
func TestAgentTypingPayload_JSONRoundtrip(t *testing.T) {
	t.Parallel()

	payload := apiclient.AgentTypingPayload{
		ChannelID:       "ch-001",
		AgentID:         "agent-001",
		ThreadID:        "thread-001",
		TextDelta:       "안녕",
		AccumulatedText: "안녕하세요",
		IsComplete:      false,
		Timestamp:       "2026-03-09T00:00:00Z",
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("AgentTypingPayload 마샬링 실패: %v", err)
	}

	var got apiclient.AgentTypingPayload
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("AgentTypingPayload 언마샬링 실패: %v", err)
	}

	if got.ChannelID != payload.ChannelID {
		t.Errorf("ChannelID = %q, want %q", got.ChannelID, payload.ChannelID)
	}
	if got.AgentID != payload.AgentID {
		t.Errorf("AgentID = %q, want %q", got.AgentID, payload.AgentID)
	}
	if got.ThreadID != payload.ThreadID {
		t.Errorf("ThreadID = %q, want %q", got.ThreadID, payload.ThreadID)
	}
	if got.TextDelta != payload.TextDelta {
		t.Errorf("TextDelta = %q, want %q", got.TextDelta, payload.TextDelta)
	}
	if got.AccumulatedText != payload.AccumulatedText {
		t.Errorf("AccumulatedText = %q, want %q", got.AccumulatedText, payload.AccumulatedText)
	}
	if got.IsComplete != payload.IsComplete {
		t.Errorf("IsComplete = %v, want %v", got.IsComplete, payload.IsComplete)
	}
}

// TestAgentTypingPayload_ThreadIDOmitempty는 thread_id 필드가 비어있을 때 omitempty를 검증합니다.
func TestAgentTypingPayload_ThreadIDOmitempty(t *testing.T) {
	t.Parallel()

	payload := apiclient.AgentTypingPayload{
		ChannelID: "ch-001",
		AgentID:   "agent-001",
		// ThreadID 생략
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("마샬링 실패: %v", err)
	}

	// thread_id가 JSON에 포함되지 않아야 함
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("언마샬링 실패: %v", err)
	}
	if _, exists := m["thread_id"]; exists {
		t.Error("thread_id가 omitempty인데 JSON에 포함되었습니다")
	}
}

// TestSSEEvent_JSONRoundtrip은 SSEEvent의 JSON 마샬링/언마샬링을 검증합니다.
func TestSSEEvent_JSONRoundtrip(t *testing.T) {
	t.Parallel()

	rawData := json.RawMessage(`{"channel_id":"ch-001","text_delta":"hello"}`)
	event := apiclient.SSEEvent{
		Type: "agent_typing",
		Data: rawData,
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("SSEEvent 마샬링 실패: %v", err)
	}

	var got apiclient.SSEEvent
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("SSEEvent 언마샬링 실패: %v", err)
	}

	if got.Type != event.Type {
		t.Errorf("Type = %q, want %q", got.Type, event.Type)
	}
	if string(got.Data) != string(event.Data) {
		t.Errorf("Data = %s, want %s", string(got.Data), string(event.Data))
	}
}

// TestFlexFloat64는 FlexFloat64의 숫자/문자열 양방향 역직렬화를 검증합니다.
func TestFlexFloat64(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    float64
		wantErr bool
	}{
		{name: "숫자 리터럴", input: `12.34`, want: 12.34},
		{name: "정수 리터럴", input: `100`, want: 100},
		{name: "문자열 숫자", input: `"45.67"`, want: 45.67},
		{name: "문자열 정수", input: `"200"`, want: 200},
		{name: "영", input: `0`, want: 0},
		{name: "문자열 영", input: `"0"`, want: 0},
		{name: "null", input: `null`, want: 0},
		{name: "잘못된 문자열", input: `"abc"`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var f apiclient.FlexFloat64
			err := json.Unmarshal([]byte(tt.input), &f)
			if tt.wantErr {
				if err == nil {
					t.Fatal("에러를 기대했지만 성공했습니다")
				}
				return
			}
			if err != nil {
				t.Fatalf("언마샬링 실패: %v", err)
			}
			if f.Float64() != tt.want {
				t.Errorf("got %v, want %v", f.Float64(), tt.want)
			}

			// 마샬링 검증: 항상 숫자로 직렬화
			data, err := json.Marshal(f)
			if err != nil {
				t.Fatalf("마샬링 실패: %v", err)
			}
			var f2 apiclient.FlexFloat64
			if err := json.Unmarshal(data, &f2); err != nil {
				t.Fatalf("재언마샬링 실패: %v", err)
			}
			if f2.Float64() != tt.want {
				t.Errorf("재언마샬링 got %v, want %v", f2.Float64(), tt.want)
			}
		})
	}
}

// TestKeyValue는 KeyValue 구조체의 기본 동작을 검증합니다.
func TestKeyValue(t *testing.T) {
	t.Parallel()

	kv := apiclient.KeyValue{Key: "Name", Value: "Autopus"}
	if kv.Key != "Name" {
		t.Errorf("Key = %q, want Name", kv.Key)
	}
	if kv.Value != "Autopus" {
		t.Errorf("Value = %q, want Autopus", kv.Value)
	}
}
