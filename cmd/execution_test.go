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

func makeExecutionTestClient(serverURL, workspaceID string) *apiclient.Client {
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

func TestRunExecutionList(t *testing.T) {
	executions := []ExecutionSummary{
		{ID: "exec-1", Status: "completed", AgentID: "ag-1"},
		{ID: "exec-2", Status: "running", AgentID: "ag-2"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/executions") {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		payload, _ := json.Marshal(executions)
		resp := map[string]interface{}{
			"success": true,
			"data":    json.RawMessage(payload),
		}
		b, _ := json.Marshal(resp)
		w.Write(b)
	}))
	defer srv.Close()

	client := makeExecutionTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	opts := executionListOpts{AgentName: "", StatusFilter: "", Limit: 10}
	err := runExecutionList(client, &buf, opts, false)
	if err != nil {
		t.Fatalf("runExecutionList 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "exec-1") {
		t.Errorf("출력에 'exec-1'이 없습니다: %s", out)
	}
}

func TestRunExecutionListWithAgentFilter(t *testing.T) {
	// agent filter가 주어지면 /agents/:agentId/executions 호출
	agents := []DashboardAgent{
		{ID: "ag-1", Name: "CTO"},
	}
	executions := []ExecutionSummary{
		{ID: "exec-1", Status: "completed", AgentID: "ag-1"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		var data interface{}
		if strings.Contains(r.URL.Path, "/dashboard/agents") {
			data = agents
		} else {
			data = executions
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

	client := makeExecutionTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	opts := executionListOpts{AgentName: "CTO", StatusFilter: "", Limit: 10}
	err := runExecutionList(client, &buf, opts, false)
	if err != nil {
		t.Fatalf("runExecutionList with agent 오류: %v", err)
	}
}

func TestRunExecutionShow(t *testing.T) {
	detail := ExecutionDetail{
		ID:     "exec-1",
		Status: "completed",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/exec-1") {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		payload, _ := json.Marshal(detail)
		resp := map[string]interface{}{
			"success": true,
			"data":    json.RawMessage(payload),
		}
		b, _ := json.Marshal(resp)
		w.Write(b)
	}))
	defer srv.Close()

	client := makeExecutionTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runExecutionShow(client, &buf, "exec-1", false)
	if err != nil {
		t.Fatalf("runExecutionShow 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "exec-1") {
		t.Errorf("출력에 'exec-1'이 없습니다: %s", out)
	}
}
