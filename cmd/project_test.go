// project_test.go는 project 서브커맨드 핸들러 함수의 단위 테스트를 제공합니다.
package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunProjectList(t *testing.T) {
	// 테스트 프로젝트 목록
	projects := []Project{
		{ID: "proj-1111-2222", Name: "Alpha Project", Slug: "alpha", Status: "active", Prefix: "ALP", IssueCounter: 5},
		{ID: "proj-3333-4444", Name: "Beta Project", Slug: "beta", Status: "archived", Prefix: "BET", IssueCounter: 12},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/projects" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(projects))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runProjectList(client, &buf, false)
	if err != nil {
		t.Fatalf("runProjectList 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Alpha Project") {
		t.Errorf("출력에 'Alpha Project'가 없습니다: %s", out)
	}
	if !strings.Contains(out, "Beta Project") {
		t.Errorf("출력에 'Beta Project'가 없습니다: %s", out)
	}
	// ID는 앞 8자만 출력
	if !strings.Contains(out, "proj-111") {
		t.Errorf("출력에 'proj-111'가 없습니다: %s", out)
	}
	// IssueCounter 출력 확인
	if !strings.Contains(out, "5") {
		t.Errorf("출력에 이슈 카운터 '5'가 없습니다: %s", out)
	}
}

func TestRunProjectListJSON(t *testing.T) {
	projects := []Project{
		{ID: "proj-1111-2222", Name: "Alpha Project", Slug: "alpha"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(projects))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runProjectList(client, &buf, true)
	if err != nil {
		t.Fatalf("runProjectList JSON 오류: %v", err)
	}

	out := buf.String()
	var parsed []Project
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, out)
	}
	if len(parsed) != 1 || parsed[0].ID != "proj-1111-2222" {
		t.Errorf("예상치 않은 결과: %v", parsed)
	}
}

func TestRunProjectShow(t *testing.T) {
	proj := Project{
		ID:          "proj-1111-2222",
		Name:        "Alpha Project",
		Slug:        "alpha",
		Status:      "active",
		Prefix:      "ALP",
		Description: "테스트 프로젝트",
		CreatedAt:   "2026-01-01T00:00:00Z",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/projects/proj-1111-2222" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(proj))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runProjectShow(client, &buf, "proj-1111-2222", false)
	if err != nil {
		t.Fatalf("runProjectShow 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Alpha Project") {
		t.Errorf("출력에 'Alpha Project'가 없습니다: %s", out)
	}
	if !strings.Contains(out, "테스트 프로젝트") {
		t.Errorf("출력에 설명이 없습니다: %s", out)
	}
}

func TestRunProjectCreate(t *testing.T) {
	proj := Project{
		ID:     "proj-new-1234",
		Name:   "New Project",
		Slug:   "new-project",
		Status: "active",
	}

	var capturedBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/projects" || r.Method != http.MethodPost {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		json.NewDecoder(r.Body).Decode(&capturedBody)
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(proj))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runProjectCreate(client, &buf, "New Project", "", false)
	if err != nil {
		t.Fatalf("runProjectCreate 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "New Project") {
		t.Errorf("출력에 'New Project'가 없습니다: %s", out)
	}
	// 요청 본문에 name이 포함되어야 합니다
	if capturedBody["name"] != "New Project" {
		t.Errorf("요청 본문 name = %v, want 'New Project'", capturedBody["name"])
	}
}

// TestRunProjectShowJSON는 runProjectShow의 JSON 출력 경로를 테스트합니다.
func TestRunProjectShowJSON(t *testing.T) {
	proj := Project{
		ID:     "proj-1111-2222",
		Name:   "Alpha Project",
		Slug:   "alpha",
		Status: "active",
		Prefix: "ALP",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(proj))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runProjectShow(client, &buf, "proj-1111-2222", true)
	if err != nil {
		t.Fatalf("runProjectShow JSON 오류: %v", err)
	}

	// 유효한 JSON 출력 확인
	var parsed Project
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if parsed.ID != "proj-1111-2222" {
		t.Errorf("예상치 않은 ID: %s", parsed.ID)
	}
}

// TestRunProjectCreateJSON는 runProjectCreate의 JSON 출력 경로를 테스트합니다.
func TestRunProjectCreateJSON(t *testing.T) {
	proj := Project{
		ID:     "proj-new-1234",
		Name:   "New Project",
		Slug:   "new-project",
		Status: "active",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(proj))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runProjectCreate(client, &buf, "New Project", "", true)
	if err != nil {
		t.Fatalf("runProjectCreate JSON 오류: %v", err)
	}

	// 유효한 JSON 출력 확인
	var parsed Project
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if parsed.ID != "proj-new-1234" {
		t.Errorf("예상치 않은 ID: %s", parsed.ID)
	}
}

// TestRunProjectShow_InvalidProjectID는 유효하지 않은 projectID에서 ValidateID 에러를 반환하는지 검증합니다.
func TestRunProjectShow_InvalidProjectID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runProjectShow(client, &buf, "bad/id", false)
	if err == nil {
		t.Error("유효하지 않은 projectID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

func TestRunProjectCreateWithPrefix(t *testing.T) {
	proj := Project{
		ID:     "proj-new-5678",
		Name:   "My App",
		Slug:   "my-app",
		Prefix: "APP",
		Status: "active",
	}

	var capturedBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedBody)
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(proj))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runProjectCreate(client, &buf, "My App", "APP", false)
	if err != nil {
		t.Fatalf("runProjectCreate with prefix 오류: %v", err)
	}

	// 요청 본문에 prefix가 포함되어야 합니다
	if capturedBody["prefix"] != "APP" {
		t.Errorf("요청 본문 prefix = %v, want 'APP'", capturedBody["prefix"])
	}
}
