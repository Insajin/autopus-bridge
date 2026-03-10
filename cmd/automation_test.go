// automation_test.go는 automation 서브커맨드 핸들러 함수를 테스트합니다.
package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunAutomationList(t *testing.T) {
	// 테스트 자동화 목록
	automations := []Automation{
		{ID: "auto-001", Name: "Auto Deploy", TriggerType: "schedule", IsActive: true, AgentID: "agent-1"},
		{ID: "auto-002", Name: "Auto Report", TriggerType: "webhook", IsActive: false, AgentID: ""},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/projects/proj-1/automations" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(automations))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runAutomationList(client, &buf, "proj-1", false)
	if err != nil {
		t.Fatalf("runAutomationList 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Auto Deploy") {
		t.Errorf("출력에 'Auto Deploy'가 없습니다: %s", out)
	}
	if !strings.Contains(out, "Auto Report") {
		t.Errorf("출력에 'Auto Report'가 없습니다: %s", out)
	}
}

func TestRunAutomationListJSON(t *testing.T) {
	automations := []Automation{
		{ID: "auto-001", Name: "Auto Deploy", TriggerType: "schedule", IsActive: true},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(automations))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runAutomationList(client, &buf, "proj-1", true)
	if err != nil {
		t.Fatalf("runAutomationList JSON 오류: %v", err)
	}

	out := buf.String()
	var parsed []Automation
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, out)
	}
	if len(parsed) != 1 || parsed[0].ID != "auto-001" {
		t.Errorf("예상치 않은 결과: %v", parsed)
	}
}

func TestRunAutomationShow(t *testing.T) {
	automation := Automation{
		ID:          "auto-001",
		Name:        "Auto Deploy",
		Description: "배포 자동화",
		TriggerType: "schedule",
		IsActive:    true,
		AgentID:     "agent-1",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/automations/auto-001" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(automation))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runAutomationShow(client, &buf, "auto-001", false)
	if err != nil {
		t.Fatalf("runAutomationShow 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Auto Deploy") {
		t.Errorf("출력에 'Auto Deploy'가 없습니다: %s", out)
	}
	if !strings.Contains(out, "배포 자동화") {
		t.Errorf("출력에 '배포 자동화'가 없습니다: %s", out)
	}
}

func TestRunAutomationShowJSON(t *testing.T) {
	automation := Automation{
		ID:   "auto-001",
		Name: "Auto Deploy",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(automation))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runAutomationShow(client, &buf, "auto-001", true)
	if err != nil {
		t.Fatalf("runAutomationShow JSON 오류: %v", err)
	}

	var parsed Automation
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if parsed.ID != "auto-001" {
		t.Errorf("예상치 않은 ID: %s", parsed.ID)
	}
}

func TestRunAutomationCreate(t *testing.T) {
	var receivedBody map[string]interface{}
	newAutomation := Automation{
		ID:          "auto-new",
		Name:        "New Auto",
		TriggerType: "schedule",
		IsActive:    false,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/projects/proj-1/automations" || r.Method != http.MethodPost {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(newAutomation))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runAutomationCreate(client, &buf, "proj-1", "New Auto", "schedule", "", "", false)
	if err != nil {
		t.Fatalf("runAutomationCreate 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "New Auto") {
		t.Errorf("출력에 'New Auto'가 없습니다: %s", out)
	}

	// POST body에 name 필드 확인
	if receivedBody["name"] != "New Auto" {
		t.Errorf("POST body에 name 필드가 없습니다: %v", receivedBody)
	}
}

func TestRunAutomationDelete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/automations/auto-001" || r.Method != http.MethodDelete {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(map[string]string{"message": "deleted"}))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runAutomationDelete(client, &buf, "auto-001")
	if err != nil {
		t.Fatalf("runAutomationDelete 오류: %v", err)
	}
}

func TestRunAutomationToggle(t *testing.T) {
	toggled := Automation{
		ID:       "auto-001",
		Name:     "Auto Deploy",
		IsActive: true,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/automations/auto-001/toggle" || r.Method != http.MethodPost {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(toggled))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runAutomationToggle(client, &buf, "auto-001", false)
	if err != nil {
		t.Fatalf("runAutomationToggle 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Auto Deploy") {
		t.Errorf("출력에 'Auto Deploy'가 없습니다: %s", out)
	}
}

func TestRunAutomationAddAction(t *testing.T) {
	var receivedBody map[string]interface{}
	newAction := AutomationAction{
		ID:         "action-1",
		ActionType: "notify",
		OrderIndex: 1,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/automations/auto-001/actions" || r.Method != http.MethodPost {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(newAction))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runAutomationAddAction(client, &buf, "auto-001", "notify", "", 1, false)
	if err != nil {
		t.Fatalf("runAutomationAddAction 오류: %v", err)
	}

	// POST body에 action_type 필드 확인
	if receivedBody["action_type"] != "notify" {
		t.Errorf("POST body에 action_type 필드가 없습니다: %v", receivedBody)
	}
}

// TestRunAutomationShow_InvalidID는 유효하지 않은 ID에서 ValidateID 에러를 반환하는지 검증합니다.
func TestRunAutomationShow_InvalidID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runAutomationShow(client, &buf, "bad/id", false)
	if err == nil {
		t.Error("유효하지 않은 ID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

func TestRunAutomationDelete_InvalidID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runAutomationDelete(client, &buf, "bad id")
	if err == nil {
		t.Error("유효하지 않은 ID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

// TestRunAutomationListError는 API 에러 경로를 테스트합니다.
func TestRunAutomationListError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"success":false,"error":"not found"}`, http.StatusNotFound)
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runAutomationList(client, &buf, "proj-bad", false)
	if err == nil {
		t.Fatal("에러가 예상되었으나 nil 반환")
	}
}
