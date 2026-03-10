// decision_test.go는 decision 서브커맨드 핸들러 함수를 테스트합니다.
package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestRunDecisionList는 decision list 테이블 출력을 테스트합니다.
func TestRunDecisionList(t *testing.T) {
	decisions := []Decision{
		{ID: "dec-uuid-1234-5678", Topic: "Deploy strategy", Status: "resolved", Level: "team", InitiatedBy: "agent-1"},
		{ID: "dec-uuid-abcd-efgh", Topic: "Architecture change", Status: "pending", Level: "org", InitiatedBy: "agent-2"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/decisions" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(decisions))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runDecisionList(client, &buf, "", "", 0, 0, false)
	if err != nil {
		t.Fatalf("runDecisionList 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Deploy strategy") {
		t.Errorf("출력에 'Deploy strategy'가 없습니다: %s", out)
	}
	if !strings.Contains(out, "Architecture change") {
		t.Errorf("출력에 'Architecture change'가 없습니다: %s", out)
	}
}

// TestRunDecisionListJSON은 decision list JSON 출력을 테스트합니다.
func TestRunDecisionListJSON(t *testing.T) {
	decisions := []Decision{
		{ID: "dec-uuid-1234-5678", Topic: "Deploy strategy", Status: "resolved"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(decisions))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runDecisionList(client, &buf, "", "", 0, 0, true)
	if err != nil {
		t.Fatalf("runDecisionList JSON 오류: %v", err)
	}

	var parsed []Decision
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if len(parsed) != 1 || parsed[0].ID != "dec-uuid-1234-5678" {
		t.Errorf("예상치 않은 결과: %v", parsed)
	}
}

// TestRunDecisionListWithQueryParams는 쿼리 파라미터가 포함된 list 요청을 테스트합니다.
func TestRunDecisionListWithQueryParams(t *testing.T) {
	decisions := []Decision{
		{ID: "dec-uuid-1234-5678", Topic: "Deploy strategy", Status: "pending", Level: "team"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/decisions" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		// 쿼리 파라미터 확인
		if r.URL.Query().Get("status") != "pending" {
			http.Error(w, "status param missing", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(decisions))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runDecisionList(client, &buf, "pending", "", 0, 0, false)
	if err != nil {
		t.Fatalf("runDecisionList with params 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Deploy strategy") {
		t.Errorf("출력에 'Deploy strategy'가 없습니다: %s", out)
	}
}

// TestRunDecisionShow는 decision show 상세 출력을 테스트합니다.
func TestRunDecisionShow(t *testing.T) {
	dec := Decision{
		ID:          "dec-uuid-1234-5678",
		Topic:       "Deploy strategy",
		Context:     "We need to decide on deployment approach",
		Status:      "resolved",
		Level:       "team",
		InitiatedBy: "agent-1",
		Outcome:     "Blue-green deployment approved",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/decisions/dec-uuid-1234-5678" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(dec))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runDecisionShow(client, &buf, "dec-uuid-1234-5678", false)
	if err != nil {
		t.Fatalf("runDecisionShow 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Deploy strategy") {
		t.Errorf("출력에 'Deploy strategy'가 없습니다: %s", out)
	}
	if !strings.Contains(out, "Blue-green deployment approved") {
		t.Errorf("출력에 'Blue-green deployment approved'가 없습니다: %s", out)
	}
}

// TestRunDecisionShowJSON는 decision show JSON 출력을 테스트합니다.
func TestRunDecisionShowJSON(t *testing.T) {
	dec := Decision{ID: "dec-uuid-1234-5678", Topic: "Deploy strategy", Status: "resolved"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(dec))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runDecisionShow(client, &buf, "dec-uuid-1234-5678", true)
	if err != nil {
		t.Fatalf("runDecisionShow JSON 오류: %v", err)
	}

	var parsed Decision
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if parsed.ID != "dec-uuid-1234-5678" {
		t.Errorf("예상치 않은 ID: %s", parsed.ID)
	}
}

// TestRunDecisionShow_InvalidID는 유효하지 않은 ID에서 ValidateID 에러를 반환하는지 검증합니다.
func TestRunDecisionShow_InvalidID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runDecisionShow(client, &buf, "bad/id", false)
	if err == nil {
		t.Error("유효하지 않은 ID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

// TestRunDecisionCreate는 decision create를 테스트합니다.
func TestRunDecisionCreate(t *testing.T) {
	newDec := Decision{ID: "dec-uuid-new1-5678", Topic: "New decision", Status: "pending"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/decisions" || r.Method != http.MethodPost {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(newDec))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runDecisionCreate(client, &buf, "New decision", "Some context", "agent-1", []string{"agent-2", "agent-3"}, false)
	if err != nil {
		t.Fatalf("runDecisionCreate 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "New decision") {
		t.Errorf("출력에 'New decision'이 없습니다: %s", out)
	}
}

// TestRunDecisionResolve는 decision resolve를 테스트합니다.
func TestRunDecisionResolve(t *testing.T) {
	resolved := Decision{ID: "dec-uuid-1234-5678", Topic: "Deploy strategy", Status: "resolved", Outcome: "Approved"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/decisions/dec-uuid-1234-5678/resolve" || r.Method != http.MethodPatch {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(resolved))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runDecisionResolve(client, &buf, "dec-uuid-1234-5678", "Approved", "Cost-effective", "agent-1")
	if err != nil {
		t.Fatalf("runDecisionResolve 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Approved") {
		t.Errorf("출력에 'Approved'가 없습니다: %s", out)
	}
}

// TestRunDecisionResolve_InvalidID는 유효하지 않은 ID에서 에러를 반환하는지 검증합니다.
func TestRunDecisionResolve_InvalidID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runDecisionResolve(client, &buf, "bad id", "outcome", "rationale", "agent-1")
	if err == nil {
		t.Error("유효하지 않은 ID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

// TestRunDecisionHumanResolve는 decision human-resolve를 테스트합니다.
func TestRunDecisionHumanResolve(t *testing.T) {
	resolved := Decision{ID: "dec-uuid-1234-5678", Topic: "Deploy strategy", Status: "approved"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/decisions/dec-uuid-1234-5678/human-resolve" || r.Method != http.MethodPatch {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(resolved))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runDecisionHumanResolve(client, &buf, "dec-uuid-1234-5678", true, "Looks good", "human-1")
	if err != nil {
		t.Fatalf("runDecisionHumanResolve 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "approved") {
		t.Errorf("출력에 'approved'가 없습니다: %s", out)
	}
}

// TestRunDecisionHumanResolve_InvalidID는 유효하지 않은 ID에서 에러를 반환하는지 검증합니다.
func TestRunDecisionHumanResolve_InvalidID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runDecisionHumanResolve(client, &buf, "bad/id", true, "feedback", "human-1")
	if err == nil {
		t.Error("유효하지 않은 ID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

// TestRunDecisionEscalate는 decision escalate를 테스트합니다.
func TestRunDecisionEscalate(t *testing.T) {
	escalated := Decision{ID: "dec-uuid-1234-5678", Topic: "Deploy strategy", Status: "escalated"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/decisions/dec-uuid-1234-5678/escalate" || r.Method != http.MethodPost {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(escalated))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runDecisionEscalate(client, &buf, "dec-uuid-1234-5678", "Too complex", "agent-1")
	if err != nil {
		t.Fatalf("runDecisionEscalate 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "escalated") {
		t.Errorf("출력에 'escalated'가 없습니다: %s", out)
	}
}

// TestRunDecisionEscalate_InvalidID는 유효하지 않은 ID에서 에러를 반환하는지 검증합니다.
func TestRunDecisionEscalate_InvalidID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runDecisionEscalate(client, &buf, "bad id", "reason", "agent-1")
	if err == nil {
		t.Error("유효하지 않은 ID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

// TestRunDecisionAuditLog는 decision audit-log를 테스트합니다.
func TestRunDecisionAuditLog(t *testing.T) {
	logs := []DecisionAuditLog{
		{ID: "log-1", Action: "created", Actor: "agent-1"},
		{ID: "log-2", Action: "resolved", Actor: "agent-2"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/decisions/dec-uuid-1234-5678/audit-log" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(logs))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runDecisionAuditLog(client, &buf, "dec-uuid-1234-5678", false)
	if err != nil {
		t.Fatalf("runDecisionAuditLog 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "created") {
		t.Errorf("출력에 'created'가 없습니다: %s", out)
	}
	if !strings.Contains(out, "resolved") {
		t.Errorf("출력에 'resolved'가 없습니다: %s", out)
	}
}

// TestRunDecisionAuditLog_InvalidID는 유효하지 않은 ID에서 에러를 반환하는지 검증합니다.
func TestRunDecisionAuditLog_InvalidID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runDecisionAuditLog(client, &buf, "bad/id", false)
	if err == nil {
		t.Error("유효하지 않은 ID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

// TestRunDecisionConsensus는 decision consensus GET을 테스트합니다.
func TestRunDecisionConsensus(t *testing.T) {
	status := ConsensusStatus{DecisionID: "dec-uuid-1234-5678", Status: "in_progress", Votes: 2, Required: 3}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/decisions/dec-uuid-1234-5678/consensus" || r.Method != http.MethodGet {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(status))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runDecisionConsensus(client, &buf, "dec-uuid-1234-5678", false)
	if err != nil {
		t.Fatalf("runDecisionConsensus 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "in_progress") {
		t.Errorf("출력에 'in_progress'가 없습니다: %s", out)
	}
}

// TestRunDecisionConsensus_InvalidID는 유효하지 않은 ID에서 에러를 반환하는지 검증합니다.
func TestRunDecisionConsensus_InvalidID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runDecisionConsensus(client, &buf, "bad id", false)
	if err == nil {
		t.Error("유효하지 않은 ID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

// TestRunDecisionConsensusStart는 decision consensus-start를 테스트합니다.
func TestRunDecisionConsensusStart(t *testing.T) {
	status := ConsensusStatus{DecisionID: "dec-uuid-1234-5678", Status: "started", Votes: 0, Required: 3}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/decisions/dec-uuid-1234-5678/consensus" || r.Method != http.MethodPost {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(status))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runDecisionConsensusStart(client, &buf, "dec-uuid-1234-5678", false)
	if err != nil {
		t.Fatalf("runDecisionConsensusStart 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "started") {
		t.Errorf("출력에 'started'가 없습니다: %s", out)
	}
}

// TestRunDecisionConsensusStart_InvalidID는 유효하지 않은 ID에서 에러를 반환하는지 검증합니다.
func TestRunDecisionConsensusStart_InvalidID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runDecisionConsensusStart(client, &buf, "bad/id", false)
	if err == nil {
		t.Error("유효하지 않은 ID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

// TestRunDecisionVote는 decision vote를 테스트합니다.
func TestRunDecisionVote(t *testing.T) {
	status := ConsensusStatus{DecisionID: "dec-uuid-1234-5678", Status: "in_progress", Votes: 3, Required: 3}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/decisions/dec-uuid-1234-5678/consensus/vote" || r.Method != http.MethodPost {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(status))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runDecisionVote(client, &buf, "dec-uuid-1234-5678", false)
	if err != nil {
		t.Fatalf("runDecisionVote 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "in_progress") {
		t.Errorf("출력에 'in_progress'가 없습니다: %s", out)
	}
}

// TestRunDecisionVote_InvalidID는 유효하지 않은 ID에서 에러를 반환하는지 검증합니다.
func TestRunDecisionVote_InvalidID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runDecisionVote(client, &buf, "bad id", false)
	if err == nil {
		t.Error("유효하지 않은 ID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

// TestRunDecisionConfidence는 decision confidence를 테스트합니다.
func TestRunDecisionConfidence(t *testing.T) {
	score := ConfidenceScore{DecisionID: "dec-uuid-1234-5678", Score: 0.87, Factors: "votes,history"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/decisions/dec-uuid-1234-5678/confidence" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(score))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runDecisionConfidence(client, &buf, "dec-uuid-1234-5678", false)
	if err != nil {
		t.Fatalf("runDecisionConfidence 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "0.87") {
		t.Errorf("출력에 '0.87'이 없습니다: %s", out)
	}
}

// TestRunDecisionConfidence_InvalidID는 유효하지 않은 ID에서 에러를 반환하는지 검증합니다.
func TestRunDecisionConfidence_InvalidID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runDecisionConfidence(client, &buf, "bad/id", false)
	if err == nil {
		t.Error("유효하지 않은 ID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

// TestRunDecisionListAPIError는 API 에러 경로를 테스트합니다.
func TestRunDecisionListAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"success":false,"error":"internal server error"}`, http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runDecisionList(client, &buf, "", "", 0, 0, false)
	if err == nil {
		t.Fatal("에러가 예상되었으나 nil 반환")
	}
}
