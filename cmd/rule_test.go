// rule_test.go는 rule 서브커맨드 핸들러 함수를 테스트합니다.
package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestRunRuleList는 규칙 목록 조회를 테스트합니다.
func TestRunRuleList(t *testing.T) {
	rules := []TriggerRule{
		{ID: "rule-1", Name: "Auto Deploy", EventType: "issue.closed", IsActive: true},
		{ID: "rule-2", Name: "Notify Team", EventType: "pr.merged", IsActive: false},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/rules" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(rules))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runRuleList(client, &buf, false)
	if err != nil {
		t.Fatalf("runRuleList 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Auto Deploy") {
		t.Errorf("출력에 'Auto Deploy'가 없습니다: %s", out)
	}
	if !strings.Contains(out, "issue.closed") {
		t.Errorf("출력에 'issue.closed'가 없습니다: %s", out)
	}
}

// TestRunRuleListJSON는 규칙 목록의 JSON 출력을 테스트합니다.
func TestRunRuleListJSON(t *testing.T) {
	rules := []TriggerRule{
		{ID: "rule-1", Name: "Auto Deploy", EventType: "issue.closed", IsActive: true},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(rules))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runRuleList(client, &buf, true)
	if err != nil {
		t.Fatalf("runRuleList JSON 오류: %v", err)
	}

	var parsed []TriggerRule
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if len(parsed) != 1 || parsed[0].ID != "rule-1" {
		t.Errorf("예상치 않은 결과: %v", parsed)
	}
}

// TestRunRuleShow는 규칙 상세 조회를 테스트합니다.
func TestRunRuleShow(t *testing.T) {
	rule := TriggerRule{ID: "rule-1", Name: "Auto Deploy", EventType: "issue.closed", IsActive: true, Description: "Auto deploys on close"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/rules/rule-1" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(rule))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runRuleShow(client, &buf, "rule-1", false)
	if err != nil {
		t.Fatalf("runRuleShow 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Auto Deploy") {
		t.Errorf("출력에 'Auto Deploy'가 없습니다: %s", out)
	}
	if !strings.Contains(out, "issue.closed") {
		t.Errorf("출력에 'issue.closed'가 없습니다: %s", out)
	}
}

// TestRunRuleShowJSON는 규칙 상세 조회의 JSON 출력을 테스트합니다.
func TestRunRuleShowJSON(t *testing.T) {
	rule := TriggerRule{ID: "rule-1", Name: "Auto Deploy", EventType: "issue.closed", IsActive: true}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(rule))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runRuleShow(client, &buf, "rule-1", true)
	if err != nil {
		t.Fatalf("runRuleShow JSON 오류: %v", err)
	}

	var parsed TriggerRule
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if parsed.ID != "rule-1" {
		t.Errorf("예상치 않은 ID: %s", parsed.ID)
	}
}

// TestRunRuleCreate는 규칙 생성을 테스트합니다.
func TestRunRuleCreate(t *testing.T) {
	newRule := TriggerRule{ID: "rule-new", Name: "New Rule", EventType: "pr.opened", IsActive: true}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/rules" || r.Method != http.MethodPost {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(newRule))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	params := RuleCreateParams{
		Name:      "New Rule",
		EventType: "pr.opened",
		Actions:   `[{"type":"notify"}]`,
	}
	err := runRuleCreate(client, &buf, params, false)
	if err != nil {
		t.Fatalf("runRuleCreate 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "New Rule") {
		t.Errorf("출력에 'New Rule'이 없습니다: %s", out)
	}
}

// TestRunRuleCreateJSON는 규칙 생성의 JSON 출력을 테스트합니다.
func TestRunRuleCreateJSON(t *testing.T) {
	newRule := TriggerRule{ID: "rule-new", Name: "New Rule", EventType: "pr.opened", IsActive: true}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(newRule))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	params := RuleCreateParams{
		Name:      "New Rule",
		EventType: "pr.opened",
		Actions:   `[{"type":"notify"}]`,
	}
	err := runRuleCreate(client, &buf, params, true)
	if err != nil {
		t.Fatalf("runRuleCreate JSON 오류: %v", err)
	}

	var parsed TriggerRule
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if parsed.ID != "rule-new" {
		t.Errorf("예상치 않은 ID: %s", parsed.ID)
	}
}

// TestRunRuleUpdate는 규칙 업데이트를 테스트합니다.
func TestRunRuleUpdate(t *testing.T) {
	updatedRule := TriggerRule{ID: "rule-1", Name: "Updated Rule", EventType: "issue.closed", IsActive: true}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/rules/rule-1" || r.Method != http.MethodPatch {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(updatedRule))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	params := RuleUpdateParams{
		Name: "Updated Rule",
	}
	err := runRuleUpdate(client, &buf, "rule-1", params, false)
	if err != nil {
		t.Fatalf("runRuleUpdate 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Updated Rule") {
		t.Errorf("출력에 'Updated Rule'이 없습니다: %s", out)
	}
}

// TestRunRuleDelete는 규칙 삭제를 테스트합니다.
func TestRunRuleDelete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/rules/rule-1" || r.Method != http.MethodDelete {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(map[string]string{"message": "deleted"}))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runRuleDelete(client, &buf, "rule-1")
	if err != nil {
		t.Fatalf("runRuleDelete 오류: %v", err)
	}
}

// TestRunRuleToggle는 규칙 활성화 토글을 테스트합니다.
func TestRunRuleToggle(t *testing.T) {
	toggled := TriggerRule{ID: "rule-1", Name: "Auto Deploy", EventType: "issue.closed", IsActive: false}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/rules/rule-1/toggle" || r.Method != http.MethodPatch {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(toggled))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runRuleToggle(client, &buf, "rule-1")
	if err != nil {
		t.Fatalf("runRuleToggle 오류: %v", err)
	}
}

// TestRunRuleLogs는 규칙 로그 조회를 테스트합니다.
func TestRunRuleLogs(t *testing.T) {
	logs := []RuleLog{
		{ID: "log-1", RuleID: "rule-1", Status: "success", FiredAt: "2026-03-10 09:00:00"},
		{ID: "log-2", RuleID: "rule-1", Status: "failed", FiredAt: "2026-03-10 10:00:00"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/rules/rule-1/logs" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(logs))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runRuleLogs(client, &buf, "rule-1", false)
	if err != nil {
		t.Fatalf("runRuleLogs 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "log-1") {
		t.Errorf("출력에 'log-1'이 없습니다: %s", out)
	}
	if !strings.Contains(out, "success") {
		t.Errorf("출력에 'success'가 없습니다: %s", out)
	}
}

// TestRunRuleLogsJSON는 규칙 로그의 JSON 출력을 테스트합니다.
func TestRunRuleLogsJSON(t *testing.T) {
	logs := []RuleLog{
		{ID: "log-1", RuleID: "rule-1", Status: "success", FiredAt: "2026-03-10 09:00:00"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(logs))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runRuleLogs(client, &buf, "rule-1", true)
	if err != nil {
		t.Fatalf("runRuleLogs JSON 오류: %v", err)
	}

	var parsed []RuleLog
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if len(parsed) != 1 || parsed[0].ID != "log-1" {
		t.Errorf("예상치 않은 결과: %v", parsed)
	}
}

// TestRunRuleShow_InvalidID는 유효하지 않은 ID에서 ValidateID 에러를 검증합니다.
func TestRunRuleShow_InvalidID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runRuleShow(client, &buf, "bad/id", false)
	if err == nil {
		t.Error("유효하지 않은 ID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

// TestRunRuleDelete_InvalidID는 유효하지 않은 ID에서 ValidateID 에러를 검증합니다.
func TestRunRuleDelete_InvalidID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runRuleDelete(client, &buf, "bad id")
	if err == nil {
		t.Error("유효하지 않은 ID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

// TestRunRuleToggle_InvalidID는 유효하지 않은 ID에서 ValidateID 에러를 검증합니다.
func TestRunRuleToggle_InvalidID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runRuleToggle(client, &buf, "../bad")
	if err == nil {
		t.Error("유효하지 않은 ID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

// TestRunRuleLogs_InvalidID는 유효하지 않은 ID에서 ValidateID 에러를 검증합니다.
func TestRunRuleLogs_InvalidID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runRuleLogs(client, &buf, "id with space", false)
	if err == nil {
		t.Error("유효하지 않은 ID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

// TestRunRuleUpdate_InvalidID는 유효하지 않은 ID에서 ValidateID 에러를 검증합니다.
func TestRunRuleUpdate_InvalidID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runRuleUpdate(client, &buf, "bad/id", RuleUpdateParams{Name: "x"}, false)
	if err == nil {
		t.Error("유효하지 않은 ID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

// TestRunRuleList_APIError는 API 에러 경로를 테스트합니다.
func TestRunRuleList_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"success":false,"error":"internal error"}`, http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runRuleList(client, &buf, false)
	if err == nil {
		t.Fatal("에러가 예상되었으나 nil 반환")
	}
}

// TestRunRuleDelete_APIError는 삭제 API 에러 경로를 테스트합니다.
func TestRunRuleDelete_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"success":false,"error":"not found"}`, http.StatusNotFound)
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runRuleDelete(client, &buf, "rule-nonexistent")
	if err == nil {
		t.Fatal("에러가 예상되었으나 nil 반환")
	}
}
