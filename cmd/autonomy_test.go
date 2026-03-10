// autonomy_test.go는 autonomy 서브커맨드 핸들러 함수를 테스트합니다.
package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunAutonomyPhase(t *testing.T) {
	phase := AutonomyPhase{Phase: "semi-autonomous", Description: "반자율 단계"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/autonomy/phase" || r.Method != http.MethodGet {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(phase))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runAutonomyPhase(client, &buf, false)
	if err != nil {
		t.Fatalf("runAutonomyPhase 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "semi-autonomous") {
		t.Errorf("출력에 'semi-autonomous'가 없습니다: %s", out)
	}
}

func TestRunAutonomyPhaseJSON(t *testing.T) {
	phase := AutonomyPhase{Phase: "semi-autonomous", Description: "반자율 단계"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(phase))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runAutonomyPhase(client, &buf, true)
	if err != nil {
		t.Fatalf("runAutonomyPhase JSON 오류: %v", err)
	}

	var parsed AutonomyPhase
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if parsed.Phase != "semi-autonomous" {
		t.Errorf("예상치 않은 phase: %s", parsed.Phase)
	}
}

func TestRunAutonomyPhaseUpdate(t *testing.T) {
	updatedPhase := AutonomyPhase{Phase: "autonomous", Description: "자율 단계"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/autonomy/phase" || r.Method != http.MethodPatch {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(updatedPhase))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runAutonomyPhaseUpdate(client, &buf, "autonomous", false)
	if err != nil {
		t.Fatalf("runAutonomyPhaseUpdate 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "autonomous") {
		t.Errorf("출력에 'autonomous'가 없습니다: %s", out)
	}
}

func TestRunAutonomyHistory(t *testing.T) {
	history := []AutonomyHistory{
		{ID: "h-1", Phase: "manual", ChangedAt: "2024-01-01", ChangedBy: "user-1"},
		{ID: "h-2", Phase: "semi-autonomous", ChangedAt: "2024-02-01", ChangedBy: "user-2"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/autonomy/phase/history" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(history))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runAutonomyHistory(client, &buf, false)
	if err != nil {
		t.Fatalf("runAutonomyHistory 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "manual") {
		t.Errorf("출력에 'manual'이 없습니다: %s", out)
	}
	if !strings.Contains(out, "semi-autonomous") {
		t.Errorf("출력에 'semi-autonomous'가 없습니다: %s", out)
	}
}

func TestRunAutonomyHistoryJSON(t *testing.T) {
	history := []AutonomyHistory{
		{ID: "h-1", Phase: "manual", ChangedAt: "2024-01-01", ChangedBy: "user-1"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(history))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runAutonomyHistory(client, &buf, true)
	if err != nil {
		t.Fatalf("runAutonomyHistory JSON 오류: %v", err)
	}

	var parsed []AutonomyHistory
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if len(parsed) != 1 || parsed[0].ID != "h-1" {
		t.Errorf("예상치 않은 결과: %v", parsed)
	}
}

func TestRunAutonomyReadiness(t *testing.T) {
	readiness := AutonomyReadiness{CurrentPhase: "semi-autonomous", NextPhase: "autonomous", ReadyScore: 0.75}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/autonomy/transition/readiness" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(readiness))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runAutonomyReadiness(client, &buf, false)
	if err != nil {
		t.Fatalf("runAutonomyReadiness 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "semi-autonomous") {
		t.Errorf("출력에 'semi-autonomous'가 없습니다: %s", out)
	}
}

func TestRunAutonomyTransitionHistory(t *testing.T) {
	history := []AutonomyHistory{
		{ID: "th-1", Phase: "autonomous", ChangedAt: "2024-03-01", ChangedBy: "agent-1"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/autonomy/transition/history" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(history))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runAutonomyTransitionHistory(client, &buf, false)
	if err != nil {
		t.Fatalf("runAutonomyTransitionHistory 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "autonomous") {
		t.Errorf("출력에 'autonomous'가 없습니다: %s", out)
	}
}

func TestRunAutonomyTrends(t *testing.T) {
	trends := []AutonomyTrend{
		{Period: "2024-01", Score: 0.5},
		{Period: "2024-02", Score: 0.7},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/autonomy/transition/trends" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(trends))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runAutonomyTrends(client, &buf, false)
	if err != nil {
		t.Fatalf("runAutonomyTrends 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "2024-01") {
		t.Errorf("출력에 '2024-01'이 없습니다: %s", out)
	}
	if !strings.Contains(out, "2024-02") {
		t.Errorf("출력에 '2024-02'가 없습니다: %s", out)
	}
}

func TestRunAutonomyTrendsJSON(t *testing.T) {
	trends := []AutonomyTrend{
		{Period: "2024-01", Score: 0.5},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(trends))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runAutonomyTrends(client, &buf, true)
	if err != nil {
		t.Fatalf("runAutonomyTrends JSON 오류: %v", err)
	}

	var parsed []AutonomyTrend
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if len(parsed) != 1 || parsed[0].Period != "2024-01" {
		t.Errorf("예상치 않은 결과: %v", parsed)
	}
}

func TestRunAutonomyRecommendation(t *testing.T) {
	rec := AutonomyRecommendation{RecommendedPhase: "autonomous", Confidence: 0.9, Rationale: "준비 완료"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/autonomy/transition/recommendation" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(rec))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runAutonomyRecommendation(client, &buf, false)
	if err != nil {
		t.Fatalf("runAutonomyRecommendation 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "autonomous") {
		t.Errorf("출력에 'autonomous'가 없습니다: %s", out)
	}
}

func TestRunAutonomyRecommendationJSON(t *testing.T) {
	rec := AutonomyRecommendation{RecommendedPhase: "autonomous", Confidence: 0.9, Rationale: "준비 완료"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(rec))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runAutonomyRecommendation(client, &buf, true)
	if err != nil {
		t.Fatalf("runAutonomyRecommendation JSON 오류: %v", err)
	}

	var parsed AutonomyRecommendation
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if parsed.RecommendedPhase != "autonomous" {
		t.Errorf("예상치 않은 recommended_phase: %s", parsed.RecommendedPhase)
	}
}

func TestRunAutonomyPhaseError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"success":false,"error":"server error"}`, http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runAutonomyPhase(client, &buf, false)
	if err == nil {
		t.Fatal("에러가 예상되었으나 nil 반환")
	}
}

func TestRunAutonomyPhaseUpdateJSON(t *testing.T) {
	updatedPhase := AutonomyPhase{Phase: "autonomous", Description: "자율 단계"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(updatedPhase))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runAutonomyPhaseUpdate(client, &buf, "autonomous", true)
	if err != nil {
		t.Fatalf("runAutonomyPhaseUpdate JSON 오류: %v", err)
	}

	var parsed AutonomyPhase
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if parsed.Phase != "autonomous" {
		t.Errorf("예상치 않은 phase: %s", parsed.Phase)
	}
}
