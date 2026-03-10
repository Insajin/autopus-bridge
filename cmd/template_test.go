// template_test.go는 template 서브커맨드 핸들러 함수를 테스트합니다.
package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunTemplateList(t *testing.T) {
	templates := []AgentTemplate{
		{ID: "tmpl-1", Name: "백엔드 에이전트", Domain: "engineering", Category: "development"},
		{ID: "tmpl-2", Name: "프론트엔드 에이전트", Domain: "engineering", Category: "development"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/agent-templates/" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(templates))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runTemplateList(client, &buf, "", "", false)
	if err != nil {
		t.Fatalf("runTemplateList 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "백엔드 에이전트") {
		t.Errorf("출력에 '백엔드 에이전트'가 없습니다: %s", out)
	}
	if !strings.Contains(out, "프론트엔드 에이전트") {
		t.Errorf("출력에 '프론트엔드 에이전트'가 없습니다: %s", out)
	}
}

func TestRunTemplateListWithFilter(t *testing.T) {
	templates := []AgentTemplate{
		{ID: "tmpl-1", Name: "백엔드 에이전트", Domain: "engineering", Category: "development"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/agent-templates/" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		// 쿼리 파라미터 검증
		q := r.URL.Query()
		if q.Get("domain") != "engineering" {
			http.Error(w, "domain 파라미터 없음", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(templates))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runTemplateList(client, &buf, "engineering", "", false)
	if err != nil {
		t.Fatalf("runTemplateList 필터 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "백엔드 에이전트") {
		t.Errorf("출력에 '백엔드 에이전트'가 없습니다: %s", out)
	}
}

func TestRunTemplateListJSON(t *testing.T) {
	templates := []AgentTemplate{
		{ID: "tmpl-1", Name: "백엔드 에이전트", Domain: "engineering", Category: "development"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(templates))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runTemplateList(client, &buf, "", "", true)
	if err != nil {
		t.Fatalf("runTemplateList JSON 오류: %v", err)
	}

	var parsed []AgentTemplate
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if len(parsed) != 1 || parsed[0].ID != "tmpl-1" {
		t.Errorf("예상치 않은 결과: %v", parsed)
	}
}

func TestRunTemplateShow(t *testing.T) {
	tmpl := AgentTemplate{ID: "tmpl-1", Name: "백엔드 에이전트", Domain: "engineering", Category: "development", Description: "백엔드 개발 에이전트"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/agent-templates/tmpl-1" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(tmpl))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runTemplateShow(client, &buf, "tmpl-1", false)
	if err != nil {
		t.Fatalf("runTemplateShow 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "백엔드 에이전트") {
		t.Errorf("출력에 '백엔드 에이전트'가 없습니다: %s", out)
	}
	if !strings.Contains(out, "백엔드 개발 에이전트") {
		t.Errorf("출력에 '백엔드 개발 에이전트'가 없습니다: %s", out)
	}
}

func TestRunTemplateShowJSON(t *testing.T) {
	tmpl := AgentTemplate{ID: "tmpl-1", Name: "백엔드 에이전트", Domain: "engineering"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(tmpl))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runTemplateShow(client, &buf, "tmpl-1", true)
	if err != nil {
		t.Fatalf("runTemplateShow JSON 오류: %v", err)
	}

	var parsed AgentTemplate
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if parsed.ID != "tmpl-1" {
		t.Errorf("예상치 않은 ID: %s", parsed.ID)
	}
}

func TestRunTemplateShow_InvalidID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runTemplateShow(client, &buf, "bad/id", false)
	if err == nil {
		t.Error("유효하지 않은 ID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

func TestRunTemplateDomains(t *testing.T) {
	domains := []TemplateDomain{
		{Name: "engineering", Count: 5},
		{Name: "marketing", Count: 3},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/agent-templates/domains" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(domains))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runTemplateDomains(client, &buf)
	if err != nil {
		t.Fatalf("runTemplateDomains 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "engineering") {
		t.Errorf("출력에 'engineering'이 없습니다: %s", out)
	}
	if !strings.Contains(out, "marketing") {
		t.Errorf("출력에 'marketing'이 없습니다: %s", out)
	}
}

func TestRunTemplateDomains_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"success":false,"error":"서버 오류"}`, http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runTemplateDomains(client, &buf)
	if err == nil {
		t.Fatal("에러가 예상되었으나 nil 반환")
	}
}

func TestRunTemplateCategories(t *testing.T) {
	categories := []TemplateCategory{
		{Name: "development", Count: 8},
		{Name: "ops", Count: 4},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/agent-templates/categories" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(categories))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runTemplateCategories(client, &buf)
	if err != nil {
		t.Fatalf("runTemplateCategories 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "development") {
		t.Errorf("출력에 'development'가 없습니다: %s", out)
	}
	if !strings.Contains(out, "ops") {
		t.Errorf("출력에 'ops'가 없습니다: %s", out)
	}
}

func TestRunTemplateCategories_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"success":false,"error":"서버 오류"}`, http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runTemplateCategories(client, &buf)
	if err == nil {
		t.Fatal("에러가 예상되었으나 nil 반환")
	}
}

func TestRunTemplateDeploy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/agent-templates/tmpl-1/deploy" || r.Method != http.MethodPost {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(map[string]interface{}{
			"id":   "agent-new",
			"name": "백엔드 에이전트 인스턴스",
		}))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runTemplateDeploy(client, &buf, "tmpl-1")
	if err != nil {
		t.Fatalf("runTemplateDeploy 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "agent-new") {
		t.Errorf("출력에 'agent-new'가 없습니다: %s", out)
	}
}

func TestRunTemplateDeploy_InvalidID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runTemplateDeploy(client, &buf, "bad/id")
	if err == nil {
		t.Error("유효하지 않은 ID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

func TestRunTemplateDeploy_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"success":false,"error":"배포 실패"}`, http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runTemplateDeploy(client, &buf, "tmpl-1")
	if err == nil {
		t.Fatal("에러가 예상되었으나 nil 반환")
	}
}
