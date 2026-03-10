// approval_test.go는 approval 서브커맨드 핸들러 함수를 테스트합니다.
package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestRunApprovalList는 approval list 테이블 출력을 테스트합니다.
func TestRunApprovalList(t *testing.T) {
	approvals := []Approval{
		{ID: "apr-uuid-1234-5678", Title: "Deploy v2.0", Status: "pending", RequestedBy: "agent-1"},
		{ID: "apr-uuid-abcd-efgh", Title: "Schema migration", Status: "approved", RequestedBy: "agent-2"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/projects/proj-1/approvals" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(approvals))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runApprovalList(client, &buf, "proj-1", false)
	if err != nil {
		t.Fatalf("runApprovalList 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Deploy v2.0") {
		t.Errorf("출력에 'Deploy v2.0'이 없습니다: %s", out)
	}
	if !strings.Contains(out, "Schema migration") {
		t.Errorf("출력에 'Schema migration'이 없습니다: %s", out)
	}
}

// TestRunApprovalListJSON은 approval list JSON 출력을 테스트합니다.
func TestRunApprovalListJSON(t *testing.T) {
	approvals := []Approval{
		{ID: "apr-uuid-1234-5678", Title: "Deploy v2.0", Status: "pending"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(approvals))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runApprovalList(client, &buf, "proj-1", true)
	if err != nil {
		t.Fatalf("runApprovalList JSON 오류: %v", err)
	}

	var parsed []Approval
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if len(parsed) != 1 || parsed[0].ID != "apr-uuid-1234-5678" {
		t.Errorf("예상치 않은 결과: %v", parsed)
	}
}

// TestRunApprovalShow는 approval show 상세 출력을 테스트합니다.
func TestRunApprovalShow(t *testing.T) {
	apr := Approval{
		ID:          "apr-uuid-1234-5678",
		Title:       "Deploy v2.0",
		Status:      "pending",
		RequestedBy: "agent-1",
		ApprovedBy:  "",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/approvals/apr-uuid-1234-5678" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(apr))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runApprovalShow(client, &buf, "apr-uuid-1234-5678", false)
	if err != nil {
		t.Fatalf("runApprovalShow 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Deploy v2.0") {
		t.Errorf("출력에 'Deploy v2.0'이 없습니다: %s", out)
	}
	if !strings.Contains(out, "agent-1") {
		t.Errorf("출력에 'agent-1'이 없습니다: %s", out)
	}
}

// TestRunApprovalShowJSON은 approval show JSON 출력을 테스트합니다.
func TestRunApprovalShowJSON(t *testing.T) {
	apr := Approval{ID: "apr-uuid-1234-5678", Title: "Deploy v2.0", Status: "pending"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(apr))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runApprovalShow(client, &buf, "apr-uuid-1234-5678", true)
	if err != nil {
		t.Fatalf("runApprovalShow JSON 오류: %v", err)
	}

	var parsed Approval
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if parsed.ID != "apr-uuid-1234-5678" {
		t.Errorf("예상치 않은 ID: %s", parsed.ID)
	}
}

// TestRunApprovalShow_InvalidID는 유효하지 않은 ID에서 ValidateID 에러를 반환하는지 검증합니다.
func TestRunApprovalShow_InvalidID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runApprovalShow(client, &buf, "bad/id", false)
	if err == nil {
		t.Error("유효하지 않은 ID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

// TestRunApprovalApprove는 approval approve를 테스트합니다.
func TestRunApprovalApprove(t *testing.T) {
	approved := Approval{ID: "apr-uuid-1234-5678", Title: "Deploy v2.0", Status: "approved"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/approvals/apr-uuid-1234-5678/approve" || r.Method != http.MethodPost {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(approved))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runApprovalApprove(client, &buf, "apr-uuid-1234-5678")
	if err != nil {
		t.Fatalf("runApprovalApprove 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "approved") {
		t.Errorf("출력에 'approved'가 없습니다: %s", out)
	}
}

// TestRunApprovalApprove_InvalidID는 유효하지 않은 ID에서 에러를 반환하는지 검증합니다.
func TestRunApprovalApprove_InvalidID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runApprovalApprove(client, &buf, "bad id")
	if err == nil {
		t.Error("유효하지 않은 ID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

// TestRunApprovalReject는 approval reject를 테스트합니다.
func TestRunApprovalReject(t *testing.T) {
	rejected := Approval{ID: "apr-uuid-1234-5678", Title: "Deploy v2.0", Status: "rejected"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/approvals/apr-uuid-1234-5678/reject" || r.Method != http.MethodPost {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(rejected))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runApprovalReject(client, &buf, "apr-uuid-1234-5678")
	if err != nil {
		t.Fatalf("runApprovalReject 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "rejected") {
		t.Errorf("출력에 'rejected'가 없습니다: %s", out)
	}
}

// TestRunApprovalReject_InvalidID는 유효하지 않은 ID에서 에러를 반환하는지 검증합니다.
func TestRunApprovalReject_InvalidID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runApprovalReject(client, &buf, "bad/id")
	if err == nil {
		t.Error("유효하지 않은 ID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

// TestRunApprovalListAPIError는 API 에러 경로를 테스트합니다.
func TestRunApprovalListAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"success":false,"error":"not found"}`, http.StatusNotFound)
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runApprovalList(client, &buf, "proj-nonexistent", false)
	if err == nil {
		t.Fatal("에러가 예상되었으나 nil 반환")
	}
}

// TestRunApprovalApproveAPIError는 approve API 에러 경로를 테스트합니다.
func TestRunApprovalApproveAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"success":false,"error":"not found"}`, http.StatusNotFound)
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runApprovalApprove(client, &buf, "apr-uuid-nonexist-0000")
	if err == nil {
		t.Fatal("에러가 예상되었으나 nil 반환")
	}
}
