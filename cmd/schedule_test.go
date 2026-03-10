// schedule_test.go는 schedule 서브커맨드 핸들러 함수를 테스트합니다.
package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunScheduleList(t *testing.T) {
	// 테스트 스케줄 목록
	schedules := []Schedule{
		{ID: "sched-001", Name: "Daily Report", CronExpression: "0 9 * * *", Timezone: "Asia/Seoul", IsActive: true},
		{ID: "sched-002", Name: "Weekly Sync", CronExpression: "0 10 * * 1", Timezone: "UTC", IsActive: false},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/schedules" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(schedules))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runScheduleList(client, &buf, false)
	if err != nil {
		t.Fatalf("runScheduleList 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Daily Report") {
		t.Errorf("출력에 'Daily Report'가 없습니다: %s", out)
	}
	if !strings.Contains(out, "Weekly Sync") {
		t.Errorf("출력에 'Weekly Sync'가 없습니다: %s", out)
	}
}

func TestRunScheduleListJSON(t *testing.T) {
	schedules := []Schedule{
		{ID: "sched-001", Name: "Daily Report", CronExpression: "0 9 * * *", IsActive: true},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(schedules))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runScheduleList(client, &buf, true)
	if err != nil {
		t.Fatalf("runScheduleList JSON 오류: %v", err)
	}

	out := buf.String()
	var parsed []Schedule
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, out)
	}
	if len(parsed) != 1 || parsed[0].ID != "sched-001" {
		t.Errorf("예상치 않은 결과: %v", parsed)
	}
}

func TestRunScheduleShow(t *testing.T) {
	schedule := Schedule{
		ID:             "sched-001",
		Name:           "Daily Report",
		Description:    "일별 리포트 생성",
		CronExpression: "0 9 * * *",
		Timezone:       "Asia/Seoul",
		IsActive:       true,
		TargetAgentID:  "agent-1",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/schedules/sched-001" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(schedule))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runScheduleShow(client, &buf, "sched-001", false)
	if err != nil {
		t.Fatalf("runScheduleShow 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Daily Report") {
		t.Errorf("출력에 'Daily Report'가 없습니다: %s", out)
	}
	if !strings.Contains(out, "일별 리포트 생성") {
		t.Errorf("출력에 '일별 리포트 생성'이 없습니다: %s", out)
	}
}

func TestRunScheduleShowJSON(t *testing.T) {
	schedule := Schedule{
		ID:   "sched-001",
		Name: "Daily Report",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(schedule))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runScheduleShow(client, &buf, "sched-001", true)
	if err != nil {
		t.Fatalf("runScheduleShow JSON 오류: %v", err)
	}

	var parsed Schedule
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if parsed.ID != "sched-001" {
		t.Errorf("예상치 않은 ID: %s", parsed.ID)
	}
}

func TestRunScheduleCreate(t *testing.T) {
	var receivedBody map[string]interface{}
	newSchedule := Schedule{
		ID:             "sched-new",
		Name:           "New Schedule",
		CronExpression: "0 8 * * *",
		IsActive:       false,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/schedules" || r.Method != http.MethodPost {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(newSchedule))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runScheduleCreate(client, &buf, "New Schedule", "0 8 * * *", "Asia/Seoul", "", "", false)
	if err != nil {
		t.Fatalf("runScheduleCreate 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "New Schedule") {
		t.Errorf("출력에 'New Schedule'가 없습니다: %s", out)
	}

	// POST body에 name 필드 확인
	if receivedBody["name"] != "New Schedule" {
		t.Errorf("POST body에 name 필드가 없습니다: %v", receivedBody)
	}
}

func TestRunScheduleDelete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/schedules/sched-001" || r.Method != http.MethodDelete {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(map[string]string{"message": "deleted"}))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runScheduleDelete(client, &buf, "sched-001")
	if err != nil {
		t.Fatalf("runScheduleDelete 오류: %v", err)
	}
}

func TestRunScheduleToggle(t *testing.T) {
	toggled := Schedule{
		ID:       "sched-001",
		Name:     "Daily Report",
		IsActive: true,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/schedules/sched-001/toggle" || r.Method != http.MethodPatch {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(toggled))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runScheduleToggle(client, &buf, "sched-001", false)
	if err != nil {
		t.Fatalf("runScheduleToggle 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Daily Report") {
		t.Errorf("출력에 'Daily Report'가 없습니다: %s", out)
	}
}

func TestRunScheduleLogs(t *testing.T) {
	logs := []ScheduleLog{
		{ID: "log-001", ScheduleID: "sched-001", Status: "completed", StartedAt: "2026-03-10 09:00:00", FinishedAt: "2026-03-10 09:00:05"},
		{ID: "log-002", ScheduleID: "sched-001", Status: "failed", StartedAt: "2026-03-09 09:00:00", FinishedAt: "2026-03-09 09:00:03"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/schedules/sched-001/logs" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(logs))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runScheduleLogs(client, &buf, "sched-001", 0, 0, false)
	if err != nil {
		t.Fatalf("runScheduleLogs 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "completed") {
		t.Errorf("출력에 'completed'가 없습니다: %s", out)
	}
	if !strings.Contains(out, "failed") {
		t.Errorf("출력에 'failed'가 없습니다: %s", out)
	}
}

func TestRunScheduleLogsWithParams(t *testing.T) {
	logs := []ScheduleLog{
		{ID: "log-001", ScheduleID: "sched-001", Status: "completed"},
	}

	var receivedURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedURL = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(logs))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runScheduleLogs(client, &buf, "sched-001", 10, 20, false)
	if err != nil {
		t.Fatalf("runScheduleLogs 오류: %v", err)
	}

	// limit, offset 쿼리 파라미터 확인
	if !strings.Contains(receivedURL, "limit=10") {
		t.Errorf("URL에 limit=10이 없습니다: %s", receivedURL)
	}
	if !strings.Contains(receivedURL, "offset=20") {
		t.Errorf("URL에 offset=20이 없습니다: %s", receivedURL)
	}
}

// TestRunScheduleShow_InvalidID는 유효하지 않은 ID에서 ValidateID 에러를 반환하는지 검증합니다.
func TestRunScheduleShow_InvalidID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runScheduleShow(client, &buf, "bad/id", false)
	if err == nil {
		t.Error("유효하지 않은 ID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

func TestRunScheduleDelete_InvalidID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runScheduleDelete(client, &buf, "bad id")
	if err == nil {
		t.Error("유효하지 않은 ID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

// TestRunScheduleListError는 API 에러 경로를 테스트합니다.
func TestRunScheduleListError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"success":false,"error":"server error"}`, http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runScheduleList(client, &buf, false)
	if err == nil {
		t.Fatal("에러가 예상되었으나 nil 반환")
	}
}
