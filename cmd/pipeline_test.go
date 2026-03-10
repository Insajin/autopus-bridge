// pipeline_test.go는 pipeline 서브커맨드 핸들러 함수를 테스트합니다.
package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunPipelineList(t *testing.T) {
	pipelines := []Pipeline{
		{ID: "pipe-1", Name: "deploy-prod", Status: "running", Stage: "build", CreatedAt: "2024-01-01T00:00:00Z"},
		{ID: "pipe-2", Name: "deploy-staging", Status: "success", Stage: "deploy", CreatedAt: "2024-01-02T00:00:00Z"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/deployment-pipelines" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(pipelines))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runPipelineList(client, &buf, false)
	if err != nil {
		t.Fatalf("runPipelineList 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "deploy-prod") {
		t.Errorf("출력에 'deploy-prod'가 없습니다: %s", out)
	}
	if !strings.Contains(out, "deploy-staging") {
		t.Errorf("출력에 'deploy-staging'이 없습니다: %s", out)
	}
}

func TestRunPipelineListJSON(t *testing.T) {
	pipelines := []Pipeline{
		{ID: "pipe-1", Name: "deploy-prod", Status: "running"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(pipelines))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runPipelineList(client, &buf, true)
	if err != nil {
		t.Fatalf("runPipelineList JSON 오류: %v", err)
	}

	var parsed []Pipeline
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if len(parsed) != 1 || parsed[0].ID != "pipe-1" {
		t.Errorf("예상치 않은 결과: %v", parsed)
	}
}

func TestRunPipelineListError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"success":false,"error":"server error"}`, http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runPipelineList(client, &buf, false)
	if err == nil {
		t.Fatal("에러가 예상되었으나 nil 반환")
	}
}

func TestRunPipelineShow(t *testing.T) {
	pipeline := Pipeline{ID: "pipe-1", Name: "deploy-prod", Status: "running", Stage: "build"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/deployment-pipelines/pipe-1" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(pipeline))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runPipelineShow(client, &buf, "pipe-1", false)
	if err != nil {
		t.Fatalf("runPipelineShow 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "deploy-prod") {
		t.Errorf("출력에 'deploy-prod'가 없습니다: %s", out)
	}
}

func TestRunPipelineShow_InvalidID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runPipelineShow(client, &buf, "bad/id", false)
	if err == nil {
		t.Error("유효하지 않은 ID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

func TestRunPipelineShowError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"success":false,"error":"not found"}`, http.StatusNotFound)
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runPipelineShow(client, &buf, "pipe-nonexistent", false)
	if err == nil {
		t.Fatal("에러가 예상되었으나 nil 반환")
	}
}

func TestRunPipelineEvents(t *testing.T) {
	events := []PipelineEvent{
		{ID: "evt-1", PipelineID: "pipe-1", EventType: "started", Message: "Pipeline started", CreatedAt: "2024-01-01T00:00:00Z"},
		{ID: "evt-2", PipelineID: "pipe-1", EventType: "stage_complete", Message: "Build complete", CreatedAt: "2024-01-01T00:01:00Z"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/deployment-pipelines/pipe-1/events" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(events))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runPipelineEvents(client, &buf, "pipe-1", false)
	if err != nil {
		t.Fatalf("runPipelineEvents 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "started") {
		t.Errorf("출력에 'started'가 없습니다: %s", out)
	}
	if !strings.Contains(out, "stage_complete") {
		t.Errorf("출력에 'stage_complete'이 없습니다: %s", out)
	}
}

func TestRunPipelineEvents_InvalidID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runPipelineEvents(client, &buf, "id with space", false)
	if err == nil {
		t.Error("유효하지 않은 ID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

func TestRunPipelineEventsJSON(t *testing.T) {
	events := []PipelineEvent{
		{ID: "evt-1", PipelineID: "pipe-1", EventType: "started"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(events))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runPipelineEvents(client, &buf, "pipe-1", true)
	if err != nil {
		t.Fatalf("runPipelineEvents JSON 오류: %v", err)
	}

	var parsed []PipelineEvent
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if len(parsed) != 1 || parsed[0].ID != "evt-1" {
		t.Errorf("예상치 않은 결과: %v", parsed)
	}
}

func TestRunPipelineRetry(t *testing.T) {
	pipeline := Pipeline{ID: "pipe-1", Name: "deploy-prod", Status: "running", Stage: "build"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/deployment-pipelines/pipe-1/retry" || r.Method != http.MethodPost {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(pipeline))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runPipelineRetry(client, &buf, "pipe-1", false)
	if err != nil {
		t.Fatalf("runPipelineRetry 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "deploy-prod") {
		t.Errorf("출력에 'deploy-prod'가 없습니다: %s", out)
	}
}

func TestRunPipelineRetry_InvalidID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runPipelineRetry(client, &buf, "bad/id", false)
	if err == nil {
		t.Error("유효하지 않은 ID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

func TestRunPipelineRetryError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"success":false,"error":"not found"}`, http.StatusNotFound)
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runPipelineRetry(client, &buf, "pipe-nonexistent", false)
	if err == nil {
		t.Fatal("에러가 예상되었으나 nil 반환")
	}
}

func TestRunPipelineCancel(t *testing.T) {
	pipeline := Pipeline{ID: "pipe-1", Name: "deploy-prod", Status: "cancelled", Stage: "build"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/deployment-pipelines/pipe-1/cancel" || r.Method != http.MethodPost {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(pipeline))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runPipelineCancel(client, &buf, "pipe-1", false)
	if err != nil {
		t.Fatalf("runPipelineCancel 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "cancelled") {
		t.Errorf("출력에 'cancelled'가 없습니다: %s", out)
	}
}

func TestRunPipelineCancel_InvalidID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runPipelineCancel(client, &buf, "../bad", false)
	if err == nil {
		t.Error("유효하지 않은 ID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

func TestRunPipelineCancelError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"success":false,"error":"not found"}`, http.StatusNotFound)
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runPipelineCancel(client, &buf, "pipe-nonexistent", false)
	if err == nil {
		t.Fatal("에러가 예상되었으나 nil 반환")
	}
}

func TestRunPipelineHistory(t *testing.T) {
	history := []DeploymentHistory{
		{ID: "hist-1", PipelineID: "pipe-1", Environment: "production", Status: "success", DeployedAt: "2024-01-01T00:00:00Z"},
		{ID: "hist-2", PipelineID: "pipe-2", Environment: "staging", Status: "failed", DeployedAt: "2024-01-02T00:00:00Z"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/deployment-history" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(history))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runPipelineHistory(client, &buf, false)
	if err != nil {
		t.Fatalf("runPipelineHistory 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "production") {
		t.Errorf("출력에 'production'이 없습니다: %s", out)
	}
	if !strings.Contains(out, "staging") {
		t.Errorf("출력에 'staging'이 없습니다: %s", out)
	}
}

func TestRunPipelineHistoryJSON(t *testing.T) {
	history := []DeploymentHistory{
		{ID: "hist-1", PipelineID: "pipe-1", Environment: "production", Status: "success"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(history))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runPipelineHistory(client, &buf, true)
	if err != nil {
		t.Fatalf("runPipelineHistory JSON 오류: %v", err)
	}

	var parsed []DeploymentHistory
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if len(parsed) != 1 || parsed[0].ID != "hist-1" {
		t.Errorf("예상치 않은 결과: %v", parsed)
	}
}

func TestRunPipelineHistoryError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"success":false,"error":"server error"}`, http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runPipelineHistory(client, &buf, false)
	if err == nil {
		t.Fatal("에러가 예상되었으나 nil 반환")
	}
}
