// approval_chain_test.go는 approval-chain 서브커맨드 핸들러 함수를 테스트합니다.
package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestRunApprovalChainTemplates는 템플릿 목록 조회를 테스트합니다.
func TestRunApprovalChainTemplates(t *testing.T) {
	templates := []ApprovalChainTemplate{
		{ID: "tmpl-1", Name: "Standard Review", Steps: 3},
		{ID: "tmpl-2", Name: "Fast Track", Steps: 1},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/approval-chains/templates" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(templates))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runApprovalChainTemplates(client, &buf, false)
	if err != nil {
		t.Fatalf("runApprovalChainTemplates 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Standard Review") {
		t.Errorf("출력에 'Standard Review'가 없습니다: %s", out)
	}
	if !strings.Contains(out, "Fast Track") {
		t.Errorf("출력에 'Fast Track'이 없습니다: %s", out)
	}
}

// TestRunApprovalChainTemplatesJSON는 템플릿 목록의 JSON 출력을 테스트합니다.
func TestRunApprovalChainTemplatesJSON(t *testing.T) {
	templates := []ApprovalChainTemplate{
		{ID: "tmpl-1", Name: "Standard Review", Steps: 3},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(templates))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runApprovalChainTemplates(client, &buf, true)
	if err != nil {
		t.Fatalf("runApprovalChainTemplates JSON 오류: %v", err)
	}

	var parsed []ApprovalChainTemplate
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if len(parsed) != 1 || parsed[0].ID != "tmpl-1" {
		t.Errorf("예상치 않은 결과: %v", parsed)
	}
}

// TestRunApprovalChainCreateTemplate는 템플릿 생성을 테스트합니다.
func TestRunApprovalChainCreateTemplate(t *testing.T) {
	newTemplate := ApprovalChainTemplate{ID: "tmpl-new", Name: "My Template", Steps: 2}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/approval-chains/templates" || r.Method != http.MethodPost {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(newTemplate))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runApprovalChainCreateTemplate(client, &buf, "My Template", "A description", "", false)
	if err != nil {
		t.Fatalf("runApprovalChainCreateTemplate 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "My Template") {
		t.Errorf("출력에 'My Template'이 없습니다: %s", out)
	}
}

// TestRunApprovalChainCreateTemplateJSON는 템플릿 생성의 JSON 출력을 테스트합니다.
func TestRunApprovalChainCreateTemplateJSON(t *testing.T) {
	newTemplate := ApprovalChainTemplate{ID: "tmpl-new", Name: "My Template", Steps: 2}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(newTemplate))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runApprovalChainCreateTemplate(client, &buf, "My Template", "", "", true)
	if err != nil {
		t.Fatalf("runApprovalChainCreateTemplate JSON 오류: %v", err)
	}

	var parsed ApprovalChainTemplate
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if parsed.ID != "tmpl-new" {
		t.Errorf("예상치 않은 ID: %s", parsed.ID)
	}
}

// TestRunApprovalChainList는 체인 목록 조회를 테스트합니다.
func TestRunApprovalChainList(t *testing.T) {
	chains := []ApprovalChain{
		{ID: "chain-1", TemplateID: "tmpl-1", Status: "in_progress", CurrentStep: 2, TotalSteps: 3},
		{ID: "chain-2", TemplateID: "tmpl-2", Status: "approved", CurrentStep: 1, TotalSteps: 1},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/approval-chains" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(chains))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runApprovalChainList(client, &buf, false)
	if err != nil {
		t.Fatalf("runApprovalChainList 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "chain-1") {
		t.Errorf("출력에 'chain-1'이 없습니다: %s", out)
	}
	if !strings.Contains(out, "in_progress") {
		t.Errorf("출력에 'in_progress'가 없습니다: %s", out)
	}
}

// TestRunApprovalChainListJSON는 체인 목록의 JSON 출력을 테스트합니다.
func TestRunApprovalChainListJSON(t *testing.T) {
	chains := []ApprovalChain{
		{ID: "chain-1", TemplateID: "tmpl-1", Status: "in_progress"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(chains))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runApprovalChainList(client, &buf, true)
	if err != nil {
		t.Fatalf("runApprovalChainList JSON 오류: %v", err)
	}

	var parsed []ApprovalChain
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if len(parsed) != 1 || parsed[0].ID != "chain-1" {
		t.Errorf("예상치 않은 결과: %v", parsed)
	}
}

// TestRunApprovalChainStart는 체인 시작을 테스트합니다.
func TestRunApprovalChainStart(t *testing.T) {
	newChain := ApprovalChain{ID: "chain-new", TemplateID: "tmpl-1", Status: "in_progress"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/approval-chains" || r.Method != http.MethodPost {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(newChain))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runApprovalChainStart(client, &buf, "tmpl-1", false)
	if err != nil {
		t.Fatalf("runApprovalChainStart 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "chain-new") {
		t.Errorf("출력에 'chain-new'가 없습니다: %s", out)
	}
}

// TestRunApprovalChainShow는 체인 상세 조회를 테스트합니다.
func TestRunApprovalChainShow(t *testing.T) {
	chain := ApprovalChain{ID: "chain-1", TemplateID: "tmpl-1", Status: "in_progress", CurrentStep: 1, TotalSteps: 3}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/approval-chains/chain-1" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(chain))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runApprovalChainShow(client, &buf, "chain-1", false)
	if err != nil {
		t.Fatalf("runApprovalChainShow 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "chain-1") {
		t.Errorf("출력에 'chain-1'이 없습니다: %s", out)
	}
	if !strings.Contains(out, "in_progress") {
		t.Errorf("출력에 'in_progress'가 없습니다: %s", out)
	}
}

// TestRunApprovalChainShowJSON는 체인 상세 조회의 JSON 출력을 테스트합니다.
func TestRunApprovalChainShowJSON(t *testing.T) {
	chain := ApprovalChain{ID: "chain-1", TemplateID: "tmpl-1", Status: "in_progress"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(chain))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runApprovalChainShow(client, &buf, "chain-1", true)
	if err != nil {
		t.Fatalf("runApprovalChainShow JSON 오류: %v", err)
	}

	var parsed ApprovalChain
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if parsed.ID != "chain-1" {
		t.Errorf("예상치 않은 ID: %s", parsed.ID)
	}
}

// TestRunApprovalChainApprove는 체인 단계 승인을 테스트합니다.
func TestRunApprovalChainApprove(t *testing.T) {
	result := map[string]string{"message": "approved"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/approval-chains/chain-1/steps/step-1/approve" || r.Method != http.MethodPost {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(result))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runApprovalChainApprove(client, &buf, "chain-1", "step-1")
	if err != nil {
		t.Fatalf("runApprovalChainApprove 오류: %v", err)
	}
}

// TestRunApprovalChainReject는 체인 단계 거절을 테스트합니다.
func TestRunApprovalChainReject(t *testing.T) {
	result := map[string]string{"message": "rejected"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/approval-chains/chain-1/steps/step-1/reject" || r.Method != http.MethodPost {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(result))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runApprovalChainReject(client, &buf, "chain-1", "step-1")
	if err != nil {
		t.Fatalf("runApprovalChainReject 오류: %v", err)
	}
}

// TestRunApprovalChainShow_InvalidID는 유효하지 않은 ID에서 ValidateID 에러를 검증합니다.
func TestRunApprovalChainShow_InvalidID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runApprovalChainShow(client, &buf, "bad/id", false)
	if err == nil {
		t.Error("유효하지 않은 ID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

// TestRunApprovalChainApprove_InvalidChainID는 유효하지 않은 chain-id에서 ValidateID 에러를 검증합니다.
func TestRunApprovalChainApprove_InvalidChainID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runApprovalChainApprove(client, &buf, "bad id", "step-1")
	if err == nil {
		t.Error("유효하지 않은 chain-id에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

// TestRunApprovalChainApprove_InvalidStepID는 유효하지 않은 step-id에서 ValidateID 에러를 검증합니다.
func TestRunApprovalChainApprove_InvalidStepID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runApprovalChainApprove(client, &buf, "chain-1", "bad/step")
	if err == nil {
		t.Error("유효하지 않은 step-id에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

// TestRunApprovalChainReject_InvalidChainID는 유효하지 않은 chain-id에서 ValidateID 에러를 검증합니다.
func TestRunApprovalChainReject_InvalidChainID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runApprovalChainReject(client, &buf, "../bad", "step-1")
	if err == nil {
		t.Error("유효하지 않은 chain-id에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

// TestRunApprovalChainTemplates_APIError는 API 에러 경로를 테스트합니다.
func TestRunApprovalChainTemplates_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"success":false,"error":"internal server error"}`, http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runApprovalChainTemplates(client, &buf, false)
	if err == nil {
		t.Fatal("에러가 예상되었으나 nil 반환")
	}
}

// TestRunApprovalChainList_APIError는 목록 API 에러 경로를 테스트합니다.
func TestRunApprovalChainList_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"success":false,"error":"not found"}`, http.StatusNotFound)
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runApprovalChainList(client, &buf, false)
	if err == nil {
		t.Fatal("에러가 예상되었으나 nil 반환")
	}
}
