package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/insajin/autopus-bridge/internal/auth"
	"github.com/insajin/autopus-bridge/internal/mcpserver"
	"github.com/rs/zerolog"

	"github.com/insajin/autopus-bridge/internal/apiclient"
)

// makeAgentTestClient는 테스트용 apiclient.Client를 생성합니다.
func makeAgentTestClient(serverURL, workspaceID string) *apiclient.Client {
	creds := &auth.Credentials{
		AccessToken: "test-token",
		ServerURL:   serverURL,
		WorkspaceID: workspaceID,
		ExpiresAt:   time.Now().Add(1 * time.Hour), // 만료되지 않은 토큰
	}
	tr := auth.NewTokenRefresher(creds)
	backend := mcpserver.NewBackendClient(serverURL, tr, 0, zerolog.Nop())
	return apiclient.New(backend, creds, tr)
}

func TestRunAgentList(t *testing.T) {
	agents := []DashboardAgent{
		{ID: "ag-1", Name: "CTO", Status: "active"},
		{ID: "ag-2", Name: "CMO", Status: "idle"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/dashboard/agents") {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		payload, _ := json.Marshal(agents)
		resp := map[string]interface{}{
			"success": true,
			"data":    json.RawMessage(payload),
		}
		b, _ := json.Marshal(resp)
		w.Write(b)
	}))
	defer srv.Close()

	client := makeAgentTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runAgentList(client, &buf, false)
	if err != nil {
		t.Fatalf("runAgentList 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "CTO") {
		t.Errorf("출력에 'CTO'가 없습니다: %s", out)
	}
	if !strings.Contains(out, "CMO") {
		t.Errorf("출력에 'CMO'가 없습니다: %s", out)
	}
}

func TestRunAgentShow(t *testing.T) {
	// 에이전트 목록 (resolveAgentRef에서 이름 조회용)
	dashboardAgents := []DashboardAgent{
		{ID: "ag-1", Name: "CTO"},
	}
	agentDetail := AgentDetail{
		ID:   "ag-1",
		Name: "CTO",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var data interface{}
		if strings.Contains(r.URL.Path, "/dashboard/agents") {
			data = dashboardAgents
		} else if strings.Contains(r.URL.Path, "/agents/ag-1") {
			data = agentDetail
		} else {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		payload, _ := json.Marshal(data)
		resp := map[string]interface{}{
			"success": true,
			"data":    json.RawMessage(payload),
		}
		b, _ := json.Marshal(resp)
		w.Write(b)
	}))
	defer srv.Close()

	client := makeAgentTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	// 이름으로 조회
	err := runAgentShow(client, &buf, "CTO", false)
	if err != nil {
		t.Fatalf("runAgentShow 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "CTO") {
		t.Errorf("출력에 'CTO'가 없습니다: %s", out)
	}
}

func TestFindDashboardAgentByName(t *testing.T) {
	agents := []DashboardAgent{
		{ID: "ag-1", Name: "CTO"},
		{ID: "ag-2", Name: "CMO"},
		{ID: "ag-3", Name: "Growth PM"},
	}

	// 대소문자 무관 정확 매칭
	result, err := findDashboardAgentByName(agents, "cto")
	if err != nil {
		t.Fatalf("findDashboardAgentByName 오류: %v", err)
	}
	if result.ID != "ag-1" {
		t.Errorf("ID = %q, want %q", result.ID, "ag-1")
	}

	// 부분 매칭
	result, err = findDashboardAgentByName(agents, "growth")
	if err != nil {
		t.Fatalf("findDashboardAgentByName 오류: %v", err)
	}
	if result.ID != "ag-3" {
		t.Errorf("ID = %q, want %q", result.ID, "ag-3")
	}

	// 찾을 수 없는 경우
	_, err = findDashboardAgentByName(agents, "nonexistent")
	if err == nil {
		t.Fatal("에러가 발생해야 합니다")
	}
}

func TestRunAgentActivity(t *testing.T) {
	dashboardAgents := []DashboardAgent{
		{ID: "ag-1", Name: "CTO"},
	}
	activity := []map[string]interface{}{
		{"id": "act-1", "type": "task_completed", "created_at": "2026-01-01T00:00:00Z"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var data interface{}
		if strings.Contains(r.URL.Path, "/activity") {
			data = activity
		} else if strings.Contains(r.URL.Path, "/dashboard/agents") {
			data = dashboardAgents
		} else {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		payload, _ := json.Marshal(data)
		resp := map[string]interface{}{
			"success": true,
			"data":    json.RawMessage(payload),
		}
		b, _ := json.Marshal(resp)
		w.Write(b)
	}))
	defer srv.Close()

	client := makeAgentTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	// 이름으로 조회
	err := runAgentActivity(client, &buf, "CTO", false)
	if err != nil {
		t.Fatalf("runAgentActivity 오류: %v", err)
	}
}
