// planning_test.go는 planning 서브커맨드 핸들러 함수를 테스트합니다.
package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunPlanningGoals(t *testing.T) {
	goals := []StrategicGoal{
		{ID: "g-1", Title: "성장 목표", OwnerAgentID: "agent-1", Status: "active"},
		{ID: "g-2", Title: "수익 목표", OwnerAgentID: "agent-2", Status: "pending"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/planning/goals" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(goals))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runPlanningGoals(client, &buf, false)
	if err != nil {
		t.Fatalf("runPlanningGoals 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "성장 목표") {
		t.Errorf("출력에 '성장 목표'가 없습니다: %s", out)
	}
	if !strings.Contains(out, "수익 목표") {
		t.Errorf("출력에 '수익 목표'가 없습니다: %s", out)
	}
}

func TestRunPlanningGoalsJSON(t *testing.T) {
	goals := []StrategicGoal{
		{ID: "g-1", Title: "성장 목표", OwnerAgentID: "agent-1", Status: "active"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(goals))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runPlanningGoals(client, &buf, true)
	if err != nil {
		t.Fatalf("runPlanningGoals JSON 오류: %v", err)
	}

	var parsed []StrategicGoal
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if len(parsed) != 1 || parsed[0].ID != "g-1" {
		t.Errorf("예상치 않은 결과: %v", parsed)
	}
}

func TestRunPlanningGoalCreate(t *testing.T) {
	newGoal := StrategicGoal{ID: "g-new", Title: "새 목표", Status: "active"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/goals" || r.Method != http.MethodPost {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(newGoal))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runPlanningGoalCreate(client, &buf, "새 목표", "목표 설명", "agent-1", "성공 기준", false)
	if err != nil {
		t.Fatalf("runPlanningGoalCreate 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "새 목표") {
		t.Errorf("출력에 '새 목표'가 없습니다: %s", out)
	}
}

func TestRunPlanningInitiatives(t *testing.T) {
	initiatives := []Initiative{
		{ID: "i-1", Title: "이니셔티브 1", GoalID: "g-1", Status: "active"},
		{ID: "i-2", Title: "이니셔티브 2", GoalID: "g-2", Status: "pending"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/planning/initiatives" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(initiatives))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runPlanningInitiatives(client, &buf, false)
	if err != nil {
		t.Fatalf("runPlanningInitiatives 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "이니셔티브 1") {
		t.Errorf("출력에 '이니셔티브 1'이 없습니다: %s", out)
	}
	if !strings.Contains(out, "이니셔티브 2") {
		t.Errorf("출력에 '이니셔티브 2'가 없습니다: %s", out)
	}
}

func TestRunPlanningInitiativeCreate(t *testing.T) {
	newInitiative := Initiative{ID: "i-new", Title: "새 이니셔티브", GoalID: "g-1", Status: "active"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/planning/initiatives" || r.Method != http.MethodPost {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(newInitiative))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runPlanningInitiativeCreate(client, &buf, "새 이니셔티브", "설명", "g-1", false)
	if err != nil {
		t.Fatalf("runPlanningInitiativeCreate 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "새 이니셔티브") {
		t.Errorf("출력에 '새 이니셔티브'가 없습니다: %s", out)
	}
}

func TestRunPlanningAlignment(t *testing.T) {
	alignment := AlignmentStatus{TotalGoals: 10, AlignedGoals: 8, AlignmentScore: 0.8}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/planning/alignment" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(alignment))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runPlanningAlignment(client, &buf, false)
	if err != nil {
		t.Fatalf("runPlanningAlignment 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "10") {
		t.Errorf("출력에 총 목표 수 '10'이 없습니다: %s", out)
	}
}

func TestRunPlanningAlignmentJSON(t *testing.T) {
	alignment := AlignmentStatus{TotalGoals: 10, AlignedGoals: 8, AlignmentScore: 0.8}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(alignment))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runPlanningAlignment(client, &buf, true)
	if err != nil {
		t.Fatalf("runPlanningAlignment JSON 오류: %v", err)
	}

	var parsed AlignmentStatus
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if parsed.TotalGoals != 10 {
		t.Errorf("예상치 않은 total_goals: %d", parsed.TotalGoals)
	}
}

func TestRunPlanningGoalsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"success":false,"error":"server error"}`, http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runPlanningGoals(client, &buf, false)
	if err == nil {
		t.Fatal("에러가 예상되었으나 nil 반환")
	}
}

func TestRunPlanningInitiativesJSON(t *testing.T) {
	initiatives := []Initiative{
		{ID: "i-1", Title: "이니셔티브 1", GoalID: "g-1", Status: "active"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(initiatives))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runPlanningInitiatives(client, &buf, true)
	if err != nil {
		t.Fatalf("runPlanningInitiatives JSON 오류: %v", err)
	}

	var parsed []Initiative
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if len(parsed) != 1 || parsed[0].ID != "i-1" {
		t.Errorf("예상치 않은 결과: %v", parsed)
	}
}
