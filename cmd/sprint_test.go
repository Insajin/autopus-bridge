// sprint_test.go는 sprint 서브커맨드 핸들러 함수를 테스트합니다.
package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunSprintList(t *testing.T) {
	// 테스트 스프린트 목록
	sprints := []Sprint{
		{ID: "sp-1", Name: "Sprint 1", Status: "active", Goal: "MVP 완성", StartDate: "2026-03-01", EndDate: "2026-03-14", IssueCount: 12},
		{ID: "sp-2", Name: "Sprint 2", Status: "planned", Goal: "베타 배포", StartDate: "2026-03-15", EndDate: "2026-03-28", IssueCount: 8},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/projects/proj-1/sprints" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(sprints))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runSprintList(client, &buf, "proj-1", false)
	if err != nil {
		t.Fatalf("runSprintList 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Sprint 1") {
		t.Errorf("출력에 'Sprint 1'이 없습니다: %s", out)
	}
	if !strings.Contains(out, "Sprint 2") {
		t.Errorf("출력에 'Sprint 2'가 없습니다: %s", out)
	}
	if !strings.Contains(out, "active") {
		t.Errorf("출력에 'active'가 없습니다: %s", out)
	}
}

func TestRunSprintListJSON(t *testing.T) {
	sprints := []Sprint{
		{ID: "sp-1", Name: "Sprint 1", Status: "active"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(sprints))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runSprintList(client, &buf, "proj-1", true)
	if err != nil {
		t.Fatalf("runSprintList JSON 오류: %v", err)
	}

	var parsed []Sprint
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if len(parsed) != 1 || parsed[0].ID != "sp-1" {
		t.Errorf("예상치 않은 결과: %v", parsed)
	}
}

func TestRunSprintShow(t *testing.T) {
	sp := Sprint{ID: "sp-1", Name: "Sprint 1", Status: "active", Goal: "MVP 완성", IssueCount: 12}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/sprints/sp-1" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(sp))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runSprintShow(client, &buf, "sp-1", false)
	if err != nil {
		t.Fatalf("runSprintShow 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Sprint 1") {
		t.Errorf("출력에 'Sprint 1'이 없습니다: %s", out)
	}
	if !strings.Contains(out, "MVP 완성") {
		t.Errorf("출력에 'MVP 완성'이 없습니다: %s", out)
	}
}

func TestRunSprintShowJSON(t *testing.T) {
	sp := Sprint{ID: "sp-1", Name: "Sprint 1", Status: "active"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(sp))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runSprintShow(client, &buf, "sp-1", true)
	if err != nil {
		t.Fatalf("runSprintShow JSON 오류: %v", err)
	}

	var parsed Sprint
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if parsed.ID != "sp-1" {
		t.Errorf("예상치 않은 ID: %s", parsed.ID)
	}
}

func TestRunSprintCreate(t *testing.T) {
	newSprint := Sprint{ID: "sp-new", Name: "New Sprint", Status: "planned"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/projects/proj-1/sprints" || r.Method != http.MethodPost {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(newSprint))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runSprintCreate(client, &buf, "proj-1", "New Sprint", "", "", "", false)
	if err != nil {
		t.Fatalf("runSprintCreate 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "New Sprint") {
		t.Errorf("출력에 'New Sprint'가 없습니다: %s", out)
	}
}

func TestRunSprintDelete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/sprints/sp-1" || r.Method != http.MethodDelete {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(map[string]string{"message": "deleted"}))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runSprintDelete(client, &buf, "sp-1")
	if err != nil {
		t.Fatalf("runSprintDelete 오류: %v", err)
	}
}

func TestRunSprintStart(t *testing.T) {
	sp := Sprint{ID: "sp-1", Name: "Sprint 1", Status: "active"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/sprints/sp-1/start" || r.Method != http.MethodPost {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(sp))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runSprintStart(client, &buf, "sp-1")
	if err != nil {
		t.Fatalf("runSprintStart 오류: %v", err)
	}
}

func TestRunSprintComplete(t *testing.T) {
	sp := Sprint{ID: "sp-1", Name: "Sprint 1", Status: "completed"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/sprints/sp-1/complete" || r.Method != http.MethodPost {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(sp))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runSprintComplete(client, &buf, "sp-1")
	if err != nil {
		t.Fatalf("runSprintComplete 오류: %v", err)
	}
}

func TestRunSprintAddIssue(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/sprints/sp-1/issues/iss-1" || r.Method != http.MethodPost {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(map[string]string{"message": "added"}))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runSprintAddIssue(client, &buf, "sp-1", "iss-1")
	if err != nil {
		t.Fatalf("runSprintAddIssue 오류: %v", err)
	}
}

func TestRunSprintRemoveIssue(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/sprints/sp-1/issues/iss-1" || r.Method != http.MethodDelete {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(map[string]string{"message": "removed"}))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runSprintRemoveIssue(client, &buf, "sp-1", "iss-1")
	if err != nil {
		t.Fatalf("runSprintRemoveIssue 오류: %v", err)
	}
}

func TestRunSprintIssues(t *testing.T) {
	issues := []SprintIssue{
		{ID: "iss-1", Title: "Fix bug", Status: "open", Priority: 1},
		{ID: "iss-2", Title: "Add feature", Status: "in_progress", Priority: 2},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/sprints/sp-1/issues" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(issues))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runSprintIssues(client, &buf, "sp-1", false)
	if err != nil {
		t.Fatalf("runSprintIssues 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Fix bug") {
		t.Errorf("출력에 'Fix bug'가 없습니다: %s", out)
	}
	if !strings.Contains(out, "Add feature") {
		t.Errorf("출력에 'Add feature'가 없습니다: %s", out)
	}
}

// TestRunSprintShow_InvalidID는 유효하지 않은 sprintID에서 ValidateID 에러를 반환하는지 검증합니다.
func TestRunSprintShow_InvalidID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runSprintShow(client, &buf, "bad/id", false)
	if err == nil {
		t.Error("유효하지 않은 sprintID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

// TestRunSprintDelete_InvalidID는 유효하지 않은 sprintID에서 ValidateID 에러를 반환하는지 검증합니다.
func TestRunSprintDelete_InvalidID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runSprintDelete(client, &buf, "bad id")
	if err == nil {
		t.Error("유효하지 않은 sprintID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

// TestRunSprintListError는 runSprintList의 API 에러 경로를 테스트합니다.
func TestRunSprintListError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"success":false,"error":"not found"}`, http.StatusNotFound)
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runSprintList(client, &buf, "proj-nonexistent", false)
	if err == nil {
		t.Fatal("에러가 예상되었으나 nil 반환")
	}
	if !strings.Contains(err.Error(), "스프린트 목록 조회 실패") {
		t.Errorf("에러 메시지에 '스프린트 목록 조회 실패'가 없습니다: %v", err)
	}
}

// TestRunSprintIssuesJSON는 runSprintIssues의 JSON 출력 경로를 테스트합니다.
func TestRunSprintIssuesJSON(t *testing.T) {
	issues := []SprintIssue{
		{ID: "iss-1", Title: "Fix bug", Status: "open"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(issues))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runSprintIssues(client, &buf, "sp-1", true)
	if err != nil {
		t.Fatalf("runSprintIssues JSON 오류: %v", err)
	}

	var parsed []SprintIssue
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if len(parsed) != 1 || parsed[0].ID != "iss-1" {
		t.Errorf("예상치 않은 결과: %v", parsed)
	}
}
