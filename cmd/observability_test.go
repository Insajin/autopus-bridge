// observability_test.go는 observability 서브커맨드 핸들러 함수를 테스트합니다.
package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunObservabilityAgents(t *testing.T) {
	agents := []ObservabilityAgent{
		{ID: "agt-1", Name: "monitor-agent", Status: "active", TaskCount: 10, SuccessRate: 0.95},
		{ID: "agt-2", Name: "collect-agent", Status: "idle", TaskCount: 5, SuccessRate: 0.80},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/observability/agents" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(agents))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runObservabilityAgents(client, &buf, false)
	if err != nil {
		t.Fatalf("runObservabilityAgents 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "monitor-agent") {
		t.Errorf("출력에 'monitor-agent'가 없습니다: %s", out)
	}
	if !strings.Contains(out, "collect-agent") {
		t.Errorf("출력에 'collect-agent'가 없습니다: %s", out)
	}
}

func TestRunObservabilityAgentsJSON(t *testing.T) {
	agents := []ObservabilityAgent{
		{ID: "agt-1", Name: "monitor-agent", Status: "active"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(agents))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runObservabilityAgents(client, &buf, true)
	if err != nil {
		t.Fatalf("runObservabilityAgents JSON 오류: %v", err)
	}

	var parsed []ObservabilityAgent
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if len(parsed) != 1 || parsed[0].ID != "agt-1" {
		t.Errorf("예상치 않은 결과: %v", parsed)
	}
}

func TestRunObservabilityAgentsWrappedResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(map[string]interface{}{
			"agents": []map[string]interface{}{
				{
					"agent_id":         "550e8400-e29b-41d4-a716-446655440000",
					"agent_type":       "ceo",
					"total_executions": 7,
					"success_rate":     0.91,
				},
			},
		}))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runObservabilityAgents(client, &buf, false)
	if err != nil {
		t.Fatalf("runObservabilityAgents wrapped 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "ceo") {
		t.Errorf("출력에 'ceo'가 없습니다: %s", out)
	}
}

func TestRunObservabilityAgentsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"success":false,"error":"server error"}`, http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runObservabilityAgents(client, &buf, false)
	if err == nil {
		t.Fatal("에러가 예상되었으나 nil 반환")
	}
}

func TestRunObservabilityAgent(t *testing.T) {
	agent := ObservabilityAgent{ID: "agt-1", Name: "monitor-agent", Status: "active", TaskCount: 10, SuccessRate: 0.95}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/observability/agents/agt-1" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(agent))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runObservabilityAgent(client, &buf, "agt-1", false)
	if err != nil {
		t.Fatalf("runObservabilityAgent 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "monitor-agent") {
		t.Errorf("출력에 'monitor-agent'가 없습니다: %s", out)
	}
}

func TestRunObservabilityAgent_InvalidID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runObservabilityAgent(client, &buf, "bad/id", false)
	if err == nil {
		t.Error("유효하지 않은 ID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

func TestRunObservabilityAgentError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"success":false,"error":"not found"}`, http.StatusNotFound)
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runObservabilityAgent(client, &buf, "agt-nonexistent", false)
	if err == nil {
		t.Fatal("에러가 예상되었으나 nil 반환")
	}
}

func TestRunObservabilityExecutions(t *testing.T) {
	execs := []ObservabilityExecution{
		{ID: "exec-1", AgentID: "agt-1", AgentName: "monitor-agent", Status: "success", Duration: "1.2s", CreatedAt: "2024-01-01T00:00:00Z"},
		{ID: "exec-2", AgentID: "agt-2", AgentName: "collect-agent", Status: "failed", Duration: "0.5s", CreatedAt: "2024-01-02T00:00:00Z"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/observability/executions" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(execs))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runObservabilityExecutions(client, &buf, false)
	if err != nil {
		t.Fatalf("runObservabilityExecutions 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "monitor-agent") {
		t.Errorf("출력에 'monitor-agent'가 없습니다: %s", out)
	}
	if !strings.Contains(out, "collect-agent") {
		t.Errorf("출력에 'collect-agent'가 없습니다: %s", out)
	}
}

func TestRunObservabilityExecutionsJSON(t *testing.T) {
	execs := []ObservabilityExecution{
		{ID: "exec-1", AgentName: "monitor-agent", Status: "success"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(execs))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runObservabilityExecutions(client, &buf, true)
	if err != nil {
		t.Fatalf("runObservabilityExecutions JSON 오류: %v", err)
	}

	var parsed []ObservabilityExecution
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if len(parsed) != 1 || parsed[0].ID != "exec-1" {
		t.Errorf("예상치 않은 결과: %v", parsed)
	}
}

func TestRunObservabilityCost(t *testing.T) {
	cost := ObservabilityCost{TotalCost: 12.34, ByAgent: "agent-a:5.00,agent-b:7.34", Period: "2024-01"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/observability/cost" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(cost))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runObservabilityCost(client, &buf, false)
	if err != nil {
		t.Fatalf("runObservabilityCost 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "12.34") && !strings.Contains(out, "TotalCost") && !strings.Contains(out, "total_cost") {
		t.Errorf("출력에 비용 정보가 없습니다: %s", out)
	}
}

func TestRunObservabilityCostJSON(t *testing.T) {
	cost := ObservabilityCost{TotalCost: 12.34, Period: "2024-01"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(cost))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runObservabilityCost(client, &buf, true)
	if err != nil {
		t.Fatalf("runObservabilityCost JSON 오류: %v", err)
	}

	var parsed ObservabilityCost
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if parsed.TotalCost != 12.34 {
		t.Errorf("예상치 않은 비용: %v", parsed.TotalCost)
	}
}

func TestRunObservabilityHealth(t *testing.T) {
	health := ObservabilityHealth{Status: "healthy", Score: 0.98, Issues: 0, CheckedAt: "2024-01-01T00:00:00Z"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/observability/health" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(health))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runObservabilityHealth(client, &buf, false)
	if err != nil {
		t.Fatalf("runObservabilityHealth 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "healthy") {
		t.Errorf("출력에 'healthy'가 없습니다: %s", out)
	}
}

func TestRunObservabilityHealthJSON(t *testing.T) {
	health := ObservabilityHealth{Status: "healthy", Score: 0.98}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(health))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runObservabilityHealth(client, &buf, true)
	if err != nil {
		t.Fatalf("runObservabilityHealth JSON 오류: %v", err)
	}

	var parsed ObservabilityHealth
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if parsed.Status != "healthy" {
		t.Errorf("예상치 않은 상태: %s", parsed.Status)
	}
}

func TestRunObservabilityTrends(t *testing.T) {
	trends := []ObservabilityTrend{
		{Period: "2024-01", Metric: "latency", Value: 150.5},
		{Period: "2024-02", Metric: "latency", Value: 140.2},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/observability/trends" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(trends))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runObservabilityTrends(client, &buf, false)
	if err != nil {
		t.Fatalf("runObservabilityTrends 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "latency") {
		t.Errorf("출력에 'latency'가 없습니다: %s", out)
	}
	if !strings.Contains(out, "2024-01") {
		t.Errorf("출력에 '2024-01'이 없습니다: %s", out)
	}
}

func TestRunObservabilityTrendsJSON(t *testing.T) {
	trends := []ObservabilityTrend{
		{Period: "2024-01", Metric: "latency", Value: 150.5},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(trends))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runObservabilityTrends(client, &buf, true)
	if err != nil {
		t.Fatalf("runObservabilityTrends JSON 오류: %v", err)
	}

	var parsed []ObservabilityTrend
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if len(parsed) != 1 || parsed[0].Period != "2024-01" {
		t.Errorf("예상치 않은 결과: %v", parsed)
	}
}

func TestRunObservabilityExecutionsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"success":false,"error":"server error"}`, http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runObservabilityExecutions(client, &buf, false)
	if err == nil {
		t.Fatal("에러가 예상되었으나 nil 반환")
	}
}

func TestRunObservabilityCostError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"success":false,"error":"server error"}`, http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runObservabilityCost(client, &buf, false)
	if err == nil {
		t.Fatal("에러가 예상되었으나 nil 반환")
	}
}

func TestRunObservabilityHealthError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"success":false,"error":"server error"}`, http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runObservabilityHealth(client, &buf, false)
	if err == nil {
		t.Fatal("에러가 예상되었으나 nil 반환")
	}
}

func TestRunObservabilityTrendsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"success":false,"error":"server error"}`, http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runObservabilityTrends(client, &buf, false)
	if err == nil {
		t.Fatal("에러가 예상되었으나 nil 반환")
	}
}
