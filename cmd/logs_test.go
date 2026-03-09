// logs_test.go는 logs, metrics, health 명령어의 테스트를 정의합니다.
package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestRunMetrics_TableOutput은 metrics 명령의 테이블 출력을 검증합니다.
func TestRunMetrics_TableOutput(t *testing.T) {
	metrics := DashboardMetrics{
		ActiveAgents:    3,
		TotalMessages:   100,
		TotalTasks:      50,
		CompletedTasks:  45,
		AvgResponseTime: 1.23,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/dashboard/metrics") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(buildAPIResponse(metrics))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runMetrics(client, &buf, false)
	if err != nil {
		t.Fatalf("runMetrics 실패: %v", err)
	}

	out := buf.String()
	// 테이블 헤더와 값 확인
	if !strings.Contains(out, "ACTIVE_AGENTS") {
		t.Errorf("출력에 ACTIVE_AGENTS 헤더가 없습니다: %s", out)
	}
	if !strings.Contains(out, "3") {
		t.Errorf("출력에 active_agents 값(3)이 없습니다: %s", out)
	}
}

// TestRunMetrics_JSONOutput은 metrics 명령의 JSON 출력을 검증합니다.
func TestRunMetrics_JSONOutput(t *testing.T) {
	metrics := DashboardMetrics{
		ActiveAgents:   5,
		TotalMessages:  200,
		CompletedTasks: 80,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(buildAPIResponse(metrics))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runMetrics(client, &buf, true)
	if err != nil {
		t.Fatalf("runMetrics JSON 실패: %v", err)
	}

	var result DashboardMetrics
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &result); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if result.ActiveAgents != 5 {
		t.Errorf("ActiveAgents = %d, 기대값 5", result.ActiveAgents)
	}
}

// TestRunHealth_TableOutput은 health 명령의 테이블 출력을 검증합니다.
func TestRunHealth_TableOutput(t *testing.T) {
	health := OrgHealth{
		Status: "healthy",
		Score:  99.5,
		Components: map[string]string{
			"database": "ok",
			"api":      "ok",
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/org-health") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(buildAPIResponse(health))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runHealth(client, &buf, false)
	if err != nil {
		t.Fatalf("runHealth 실패: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "healthy") {
		t.Errorf("출력에 'healthy' 상태가 없습니다: %s", out)
	}
}

// TestRunHealth_JSONOutput은 health 명령의 JSON 출력을 검증합니다.
func TestRunHealth_JSONOutput(t *testing.T) {
	health := OrgHealth{
		Status: "degraded",
		Score:  60.0,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(buildAPIResponse(health))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runHealth(client, &buf, true)
	if err != nil {
		t.Fatalf("runHealth JSON 실패: %v", err)
	}

	var result OrgHealth
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &result); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if result.Status != "degraded" {
		t.Errorf("Status = %s, 기대값 degraded", result.Status)
	}
}

// TestRunMetrics_APIError는 API 오류 시 에러를 반환하는지 검증합니다.
func TestRunMetrics_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal server error"}`))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runMetrics(client, &buf, false)
	if err == nil {
		t.Fatal("서버 오류 시 에러를 반환해야 합니다")
	}
}

// TestRunHealth_APIError는 API 오류 시 에러를 반환하는지 검증합니다.
func TestRunHealth_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal server error"}`))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runHealth(client, &buf, false)
	if err == nil {
		t.Fatal("서버 오류 시 에러를 반환해야 합니다")
	}
}

// TestFormatSSEEvent_MetricUpdate는 metric_update 이벤트 포맷을 검증합니다.
func TestFormatSSEEvent_MetricUpdate(t *testing.T) {
	data, _ := json.Marshal(map[string]interface{}{"active_agents": 5})
	result := formatSSEEvent("metric_update", data, "", "")
	if !strings.Contains(result, "[METRIC]") {
		t.Errorf("metric_update 포맷에 [METRIC] 접두사가 없습니다: %s", result)
	}
}

// TestFormatSSEEvent_AgentTyping은 agent_typing 이벤트 포맷을 검증합니다.
func TestFormatSSEEvent_AgentTyping(t *testing.T) {
	payload := map[string]interface{}{
		"agent_id":   "agent-1",
		"text_delta": "hello world",
	}
	data, _ := json.Marshal(payload)
	result := formatSSEEvent("agent_typing", data, "", "")
	if !strings.Contains(result, "[AGENT]") {
		t.Errorf("agent_typing 포맷에 [AGENT] 접두사가 없습니다: %s", result)
	}
}

// TestFormatSSEEvent_AgentStatusChange는 agent_status_change 이벤트 포맷을 검증합니다.
func TestFormatSSEEvent_AgentStatusChange(t *testing.T) {
	payload := map[string]interface{}{
		"agent_id": "agent-123",
		"status":   "idle",
	}
	data, _ := json.Marshal(payload)
	result := formatSSEEvent("agent_status_change", data, "", "")
	if !strings.Contains(result, "[STATUS]") {
		t.Errorf("agent_status_change 포맷에 [STATUS] 접두사가 없습니다: %s", result)
	}
}

// TestFilterSSEEvent_AgentFilter는 에이전트 이름 필터를 검증합니다.
func TestFilterSSEEvent_AgentFilter(t *testing.T) {
	t.Run("일치하는_에이전트_허용", func(t *testing.T) {
		payload := map[string]interface{}{
			"agent_id":   "my-agent",
			"text_delta": "hello",
		}
		data, _ := json.Marshal(payload)
		if !shouldShowSSEEvent("agent_typing", data, "my-agent", "") {
			t.Error("일치하는 에이전트 이벤트는 허용해야 합니다")
		}
	})

	t.Run("불일치하는_에이전트_차단", func(t *testing.T) {
		payload := map[string]interface{}{
			"agent_id":   "other-agent",
			"text_delta": "hello",
		}
		data, _ := json.Marshal(payload)
		if shouldShowSSEEvent("agent_typing", data, "my-agent", "") {
			t.Error("불일치하는 에이전트 이벤트는 차단해야 합니다")
		}
	})

	t.Run("필터_없음_모두_허용", func(t *testing.T) {
		payload := map[string]interface{}{
			"agent_id":   "any-agent",
			"text_delta": "hello",
		}
		data, _ := json.Marshal(payload)
		if !shouldShowSSEEvent("agent_typing", data, "", "") {
			t.Error("필터 없으면 모든 이벤트를 허용해야 합니다")
		}
	})
}

// TestFilterSSEEvent_TypeFilter는 이벤트 타입 필터를 검증합니다.
func TestFilterSSEEvent_TypeFilter(t *testing.T) {
	t.Run("일치하는_타입_허용", func(t *testing.T) {
		data, _ := json.Marshal(map[string]interface{}{})
		if !shouldShowSSEEvent("metric_update", data, "", "metric_update") {
			t.Error("일치하는 타입 이벤트는 허용해야 합니다")
		}
	})

	t.Run("불일치하는_타입_차단", func(t *testing.T) {
		data, _ := json.Marshal(map[string]interface{}{})
		if shouldShowSSEEvent("agent_typing", data, "", "metric_update") {
			t.Error("불일치하는 타입 이벤트는 차단해야 합니다")
		}
	})
}

// TestSSELogsFromMockServer는 httptest SSE 서버에서 이벤트 수신을 검증합니다.
func TestSSELogsFromMockServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/stream") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "SSE not supported", http.StatusInternalServerError)
			return
		}
		// 이벤트 3개 전송 후 종료
		for i := 0; i < 3; i++ {
			payload := fmt.Sprintf(`{"type":"metric_update","data":{"active_agents":%d}}`, i+1)
			fmt.Fprintf(w, "data: %s\n\n", payload)
			flusher.Flush()
		}
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	// tail=3으로 3개 이벤트 수신 후 종료
	err := runLogs(client, &buf, "", "", 3)
	if err != nil {
		t.Fatalf("runLogs 실패: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "[METRIC]") {
		t.Errorf("출력에 [METRIC] 이벤트가 없습니다: %s", out)
	}
}

// TestFormatSSEEvent_UnknownType은 알 수 없는 이벤트 타입 포맷을 검증합니다.
func TestFormatSSEEvent_UnknownType(t *testing.T) {
	data, _ := json.Marshal(map[string]interface{}{"foo": "bar"})
	result := formatSSEEvent("custom_event", data, "", "")
	if !strings.Contains(result, "[custom_event]") {
		t.Errorf("알 수 없는 이벤트 타입에 [custom_event] 접두사가 없습니다: %s", result)
	}
}

// TestFormatSSEEvent_AgentTyping_InvalidJSON은 JSON 파싱 실패 시 폴백을 검증합니다.
func TestFormatSSEEvent_AgentTyping_InvalidJSON(t *testing.T) {
	result := formatSSEEvent("agent_typing", json.RawMessage(`invalid-json`), "", "")
	if !strings.Contains(result, "[AGENT]") {
		t.Errorf("JSON 파싱 실패 시 [AGENT] 접두사가 없습니다: %s", result)
	}
}

// TestFormatSSEEvent_AgentStatusChange_InvalidJSON은 JSON 파싱 실패 시 폴백을 검증합니다.
func TestFormatSSEEvent_AgentStatusChange_InvalidJSON(t *testing.T) {
	result := formatSSEEvent("agent_status_change", json.RawMessage(`invalid-json`), "", "")
	if !strings.Contains(result, "[STATUS]") {
		t.Errorf("JSON 파싱 실패 시 [STATUS] 접두사가 없습니다: %s", result)
	}
}

// TestSSELogs_ServerClosesImmediately는 서버가 즉시 연결을 끊을 때 정상 종료를 검증합니다.
func TestSSELogs_ServerClosesImmediately(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/stream") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		// 이벤트 없이 즉시 연결 종료 → events 채널 close → line 137-140 커버
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runLogs(client, &buf, "", "", 0)
	if err != nil {
		// SSE 재연결 실패로 에러 반환 가능 — 에러 채널 분기(line 154-158) 커버
		if !strings.Contains(err.Error(), "SSE") {
			t.Fatalf("예상과 다른 에러: %v", err)
		}
	}
}

// TestSSELogs_ServerError는 SSE 서버 오류 시 에러 반환을 검증합니다.
func TestSSELogs_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 모든 요청에 500 반환 → SSE 연결 실패 → 에러 채널로 전달
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runLogs(client, &buf, "", "", 0)
	if err == nil {
		t.Fatal("SSE 서버 오류 시 에러를 반환해야 합니다")
	}
	if !strings.Contains(err.Error(), "SSE") {
		t.Errorf("SSE 관련 에러 메시지를 기대했지만: %v", err)
	}
}

// TestSSELogs_WithTypeFilter는 타입 필터로 로그를 필터링하는지 검증합니다.
func TestSSELogs_WithTypeFilter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/stream") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		flusher, ok := w.(http.Flusher)
		if !ok {
			return
		}
		// metric_update 1개 + agent_typing 1개 전송
		fmt.Fprintf(w, "data: {\"type\":\"metric_update\",\"data\":{\"active_agents\":1}}\n\n")
		flusher.Flush()
		fmt.Fprintf(w, "data: {\"type\":\"agent_typing\",\"data\":{\"agent_id\":\"a1\",\"text_delta\":\"hi\"}}\n\n")
		flusher.Flush()
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	// metric_update만 tail=1로 수신
	err := runLogs(client, &buf, "", "metric_update", 1)
	if err != nil {
		t.Fatalf("runLogs with type filter 실패: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "[METRIC]") {
		t.Errorf("출력에 [METRIC] 이벤트가 없습니다: %s", out)
	}
	if strings.Contains(out, "[AGENT]") {
		t.Errorf("타입 필터로 [AGENT] 이벤트가 제외되어야 합니다: %s", out)
	}
}
