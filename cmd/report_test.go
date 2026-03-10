// report_test.go는 report 서브커맨드 핸들러 함수를 테스트합니다.
package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestRunReportList는 에이전트 ID 없이 워크스페이스 기준 리포트 목록 조회를 테스트합니다.
func TestRunReportList(t *testing.T) {
	reports := []ScheduledReport{
		{ID: "rep-1", AgentID: "agent-1", ReportType: "daily", CronExpression: "0 9 * * *", IsActive: true},
		{ID: "rep-2", AgentID: "agent-2", ReportType: "weekly", CronExpression: "0 9 * * 1", IsActive: false},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/scheduled-reports" {
			http.Error(w, "not found: "+r.URL.Path, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(reports))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runReportList(client, &buf, "", false)
	if err != nil {
		t.Fatalf("runReportList 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "daily") {
		t.Errorf("출력에 'daily'가 없습니다: %s", out)
	}
	if !strings.Contains(out, "weekly") {
		t.Errorf("출력에 'weekly'가 없습니다: %s", out)
	}
}

// TestRunReportListWithAgent는 에이전트 ID로 리포트 목록 조회를 테스트합니다.
func TestRunReportListWithAgent(t *testing.T) {
	reports := []ScheduledReport{
		{ID: "rep-1", AgentID: "agent-1", ReportType: "daily", CronExpression: "0 9 * * *", IsActive: true},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/agents/agent-1/scheduled-reports" {
			http.Error(w, "not found: "+r.URL.Path, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(reports))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runReportList(client, &buf, "agent-1", false)
	if err != nil {
		t.Fatalf("runReportList with agent 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "daily") {
		t.Errorf("출력에 'daily'가 없습니다: %s", out)
	}
}

// TestRunReportListJSON는 리포트 목록 JSON 출력을 테스트합니다.
func TestRunReportListJSON(t *testing.T) {
	reports := []ScheduledReport{
		{ID: "rep-1", AgentID: "agent-1", ReportType: "daily", IsActive: true},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(reports))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runReportList(client, &buf, "", true)
	if err != nil {
		t.Fatalf("runReportList JSON 오류: %v", err)
	}

	var parsed []ScheduledReport
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if len(parsed) != 1 || parsed[0].ID != "rep-1" {
		t.Errorf("예상치 않은 결과: %v", parsed)
	}
}

// TestRunReportShow는 리포트 상세 조회를 테스트합니다.
func TestRunReportShow(t *testing.T) {
	report := ScheduledReport{
		ID:             "rep-1",
		AgentID:        "agent-1",
		ChannelID:      "ch-1",
		ReportType:     "daily",
		CronExpression: "0 9 * * *",
		IsActive:       true,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/scheduled-reports/rep-1" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(report))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runReportShow(client, &buf, "rep-1", false)
	if err != nil {
		t.Fatalf("runReportShow 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "daily") {
		t.Errorf("출력에 'daily'가 없습니다: %s", out)
	}
	if !strings.Contains(out, "agent-1") {
		t.Errorf("출력에 'agent-1'이 없습니다: %s", out)
	}
}

// TestRunReportShowJSON는 리포트 상세 JSON 출력을 테스트합니다.
func TestRunReportShowJSON(t *testing.T) {
	report := ScheduledReport{ID: "rep-1", ReportType: "daily", IsActive: true}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(report))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runReportShow(client, &buf, "rep-1", true)
	if err != nil {
		t.Fatalf("runReportShow JSON 오류: %v", err)
	}

	var parsed ScheduledReport
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if parsed.ID != "rep-1" {
		t.Errorf("예상치 않은 ID: %s", parsed.ID)
	}
}

// TestRunReportShow_InvalidID는 유효하지 않은 ID에서 에러를 반환하는지 테스트합니다.
func TestRunReportShow_InvalidID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runReportShow(client, &buf, "bad/id", false)
	if err == nil {
		t.Error("유효하지 않은 ID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

// TestRunReportCreate는 리포트 생성을 테스트합니다.
func TestRunReportCreate(t *testing.T) {
	newReport := ScheduledReport{
		ID:             "rep-new",
		AgentID:        "agent-1",
		ChannelID:      "ch-1",
		ReportType:     "daily",
		CronExpression: "0 9 * * *",
		IsActive:       true,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/agents/agent-1/scheduled-reports" || r.Method != http.MethodPost {
			http.Error(w, "not found: "+r.URL.Path, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(newReport))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runReportCreate(client, &buf, "agent-1", "ch-1", "daily", "0 9 * * *", "", false)
	if err != nil {
		t.Fatalf("runReportCreate 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "daily") {
		t.Errorf("출력에 'daily'가 없습니다: %s", out)
	}
}

// TestRunReportCreateJSON는 리포트 생성 JSON 출력을 테스트합니다.
func TestRunReportCreateJSON(t *testing.T) {
	newReport := ScheduledReport{ID: "rep-new", ReportType: "daily"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(newReport))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runReportCreate(client, &buf, "agent-1", "ch-1", "daily", "0 9 * * *", "", true)
	if err != nil {
		t.Fatalf("runReportCreate JSON 오류: %v", err)
	}

	var parsed ScheduledReport
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if parsed.ID != "rep-new" {
		t.Errorf("예상치 않은 ID: %s", parsed.ID)
	}
}

// TestRunReportUpdate는 리포트 업데이트를 테스트합니다.
func TestRunReportUpdate(t *testing.T) {
	updatedReport := ScheduledReport{
		ID:             "rep-1",
		ReportType:     "weekly",
		CronExpression: "0 9 * * 1",
		IsActive:       false,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/scheduled-reports/rep-1" || r.Method != http.MethodPut {
			http.Error(w, "not found: "+r.URL.Path+" method: "+r.Method, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(updatedReport))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runReportUpdate(client, &buf, "rep-1", "", "weekly", "0 9 * * 1", false, false)
	if err != nil {
		t.Fatalf("runReportUpdate 오류: %v", err)
	}
}

// TestRunReportUpdate_InvalidID는 유효하지 않은 ID에서 에러를 반환하는지 테스트합니다.
func TestRunReportUpdate_InvalidID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runReportUpdate(client, &buf, "bad id", "", "", "", false, false)
	if err == nil {
		t.Error("유효하지 않은 ID에서 에러가 발생해야 합니다")
	}
}

// TestRunReportDelete는 리포트 삭제를 테스트합니다.
func TestRunReportDelete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/scheduled-reports/rep-1" || r.Method != http.MethodDelete {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(map[string]string{"message": "deleted"}))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runReportDelete(client, &buf, "rep-1")
	if err != nil {
		t.Fatalf("runReportDelete 오류: %v", err)
	}
}

// TestRunReportDelete_InvalidID는 유효하지 않은 ID에서 에러를 반환하는지 테스트합니다.
func TestRunReportDelete_InvalidID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runReportDelete(client, &buf, "bad/id")
	if err == nil {
		t.Error("유효하지 않은 ID에서 에러가 발생해야 합니다")
	}
}

// TestRunReportTrigger는 리포트 트리거를 테스트합니다.
func TestRunReportTrigger(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/scheduled-reports/rep-1/trigger" || r.Method != http.MethodPost {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(map[string]string{"message": "triggered"}))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runReportTrigger(client, &buf, "rep-1")
	if err != nil {
		t.Fatalf("runReportTrigger 오류: %v", err)
	}
}

// TestRunReportTrigger_InvalidID는 유효하지 않은 ID에서 에러를 반환하는지 테스트합니다.
func TestRunReportTrigger_InvalidID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runReportTrigger(client, &buf, "../bad")
	if err == nil {
		t.Error("유효하지 않은 ID에서 에러가 발생해야 합니다")
	}
}

// TestRunReportToggle는 리포트 활성화 토글을 테스트합니다.
func TestRunReportToggle(t *testing.T) {
	report := ScheduledReport{ID: "rep-1", IsActive: true}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/scheduled-reports/rep-1/toggle" || r.Method != http.MethodPatch {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(report))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runReportToggle(client, &buf, "rep-1")
	if err != nil {
		t.Fatalf("runReportToggle 오류: %v", err)
	}
}

// TestRunReportToggle_InvalidID는 유효하지 않은 ID에서 에러를 반환하는지 테스트합니다.
func TestRunReportToggle_InvalidID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runReportToggle(client, &buf, "bad id")
	if err == nil {
		t.Error("유효하지 않은 ID에서 에러가 발생해야 합니다")
	}
}

// TestRunReportDeleteError는 API 에러 경로를 테스트합니다.
func TestRunReportDeleteError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"success":false,"error":"not found"}`, http.StatusNotFound)
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runReportDelete(client, &buf, "rep-nonexistent")
	if err == nil {
		t.Fatal("에러가 예상되었으나 nil 반환")
	}
}

// TestRunReportListError는 리포트 목록 API 에러 경로를 테스트합니다.
func TestRunReportListError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"success":false,"error":"internal error"}`, http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runReportList(client, &buf, "", false)
	if err == nil {
		t.Fatal("에러가 예상되었으나 nil 반환")
	}
}
