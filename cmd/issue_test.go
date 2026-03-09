// issue_test.go는 issue 서브커맨드 핸들러 함수의 단위 테스트를 제공합니다.
package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunIssueList(t *testing.T) {
	// 테스트 이슈 목록
	issues := []Issue{
		{ID: "issue-1111", Number: 1, Title: "첫 번째 이슈", Status: "open", Priority: "high", Type: "bug", AssigneeID: "agent-aaa"},
		{ID: "issue-2222", Number: 2, Title: "두 번째 이슈", Status: "closed", Priority: "low", Type: "feature"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/projects/proj-1/issues" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(issues))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runIssueList(client, &buf, "proj-1", "", "", "", false)
	if err != nil {
		t.Fatalf("runIssueList 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "첫 번째 이슈") {
		t.Errorf("출력에 '첫 번째 이슈'가 없습니다: %s", out)
	}
	if !strings.Contains(out, "두 번째 이슈") {
		t.Errorf("출력에 '두 번째 이슈'가 없습니다: %s", out)
	}
	// Number 컬럼 확인
	if !strings.Contains(out, "1") {
		t.Errorf("출력에 번호 '1'이 없습니다: %s", out)
	}
}

func TestRunIssueListWithFilters(t *testing.T) {
	issues := []Issue{
		{ID: "issue-3333", Number: 3, Title: "필터 이슈", Status: "open", Priority: "high", Type: "bug"},
	}

	var capturedQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(issues))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	// status, priority, type 필터 모두 지정
	err := runIssueList(client, &buf, "proj-1", "open", "high", "bug", false)
	if err != nil {
		t.Fatalf("runIssueList with filters 오류: %v", err)
	}

	// 쿼리 파라미터 확인
	if !strings.Contains(capturedQuery, "status=open") {
		t.Errorf("쿼리에 'status=open'이 없습니다: %s", capturedQuery)
	}
	if !strings.Contains(capturedQuery, "priority=high") {
		t.Errorf("쿼리에 'priority=high'이 없습니다: %s", capturedQuery)
	}
	if !strings.Contains(capturedQuery, "type=bug") {
		t.Errorf("쿼리에 'type=bug'이 없습니다: %s", capturedQuery)
	}
}

func TestRunIssueListJSON(t *testing.T) {
	issues := []Issue{
		{ID: "issue-1111", Number: 1, Title: "이슈 하나"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(issues))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runIssueList(client, &buf, "proj-1", "", "", "", true)
	if err != nil {
		t.Fatalf("runIssueList JSON 오류: %v", err)
	}

	out := buf.String()
	var parsed []Issue
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, out)
	}
	if len(parsed) != 1 || parsed[0].ID != "issue-1111" {
		t.Errorf("예상치 않은 결과: %v", parsed)
	}
}

func TestRunIssueShow(t *testing.T) {
	issue := Issue{
		ID:          "issue-1111",
		Number:      1,
		Title:       "버그 리포트",
		Status:      "open",
		Priority:    "high",
		Type:        "bug",
		AssigneeID:  "agent-aaa",
		Description: "상세 설명입니다",
		CreatedAt:   "2026-01-01T00:00:00Z",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/issues/issue-1111" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(issue))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runIssueShow(client, &buf, "issue-1111", false)
	if err != nil {
		t.Fatalf("runIssueShow 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "버그 리포트") {
		t.Errorf("출력에 '버그 리포트'가 없습니다: %s", out)
	}
	if !strings.Contains(out, "상세 설명입니다") {
		t.Errorf("출력에 설명이 없습니다: %s", out)
	}
}

func TestRunIssueCreate(t *testing.T) {
	issue := Issue{
		ID:     "issue-new-1",
		Number: 10,
		Title:  "새 이슈",
		Status: "open",
	}

	var capturedBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/projects/proj-1/issues" || r.Method != http.MethodPost {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		json.NewDecoder(r.Body).Decode(&capturedBody)
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(issue))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runIssueCreate(client, &buf, "proj-1", "새 이슈", "설명", "high", "bug", false)
	if err != nil {
		t.Fatalf("runIssueCreate 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "새 이슈") {
		t.Errorf("출력에 '새 이슈'가 없습니다: %s", out)
	}
	if capturedBody["title"] != "새 이슈" {
		t.Errorf("요청 본문 title = %v, want '새 이슈'", capturedBody["title"])
	}
}

func TestRunIssueUpdate(t *testing.T) {
	issue := Issue{
		ID:    "issue-1111",
		Title: "수정된 이슈 제목",
	}

	var capturedBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/issues/issue-1111" || r.Method != http.MethodPatch {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		json.NewDecoder(r.Body).Decode(&capturedBody)
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(issue))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runIssueUpdate(client, &buf, "issue-1111", "수정된 이슈 제목", "", "", false)
	if err != nil {
		t.Fatalf("runIssueUpdate 오류: %v", err)
	}

	if capturedBody["title"] != "수정된 이슈 제목" {
		t.Errorf("요청 본문 title = %v, want '수정된 이슈 제목'", capturedBody["title"])
	}
}

func TestRunIssueAssign(t *testing.T) {
	issue := Issue{
		ID:         "issue-1111",
		AssigneeID: "agent-bbb",
	}

	var capturedBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/issues/issue-1111/assignee" || r.Method != http.MethodPatch {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		json.NewDecoder(r.Body).Decode(&capturedBody)
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(issue))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runIssueAssign(client, &buf, "issue-1111", "agent-bbb", false)
	if err != nil {
		t.Fatalf("runIssueAssign 오류: %v", err)
	}

	if capturedBody["assignee_id"] != "agent-bbb" {
		t.Errorf("요청 본문 assignee_id = %v, want 'agent-bbb'", capturedBody["assignee_id"])
	}
}

func TestRunIssueCommentList(t *testing.T) {
	comments := []IssueComment{
		{ID: "cmt-1111-2222", Content: "첫 번째 댓글", AuthorID: "user-aaaa", CreatedAt: "2026-01-01T00:00:00Z"},
		{ID: "cmt-3333-4444", Content: "두 번째 댓글", AuthorID: "user-bbbb"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/issues/issue-1111/comments" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(comments))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runIssueCommentList(client, &buf, "issue-1111", false)
	if err != nil {
		t.Fatalf("runIssueCommentList 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "첫 번째 댓글") {
		t.Errorf("출력에 '첫 번째 댓글'이 없습니다: %s", out)
	}
	if !strings.Contains(out, "두 번째 댓글") {
		t.Errorf("출력에 '두 번째 댓글'이 없습니다: %s", out)
	}
	// ID 앞 8자 확인
	if !strings.Contains(out, "cmt-1111") {
		t.Errorf("출력에 'cmt-1111'이 없습니다: %s", out)
	}
}

// TestRunIssueShowJSON는 runIssueShow의 JSON 출력 경로를 테스트합니다.
func TestRunIssueShowJSON(t *testing.T) {
	issue := Issue{
		ID:     "issue-1111",
		Number: 1,
		Title:  "버그 리포트",
		Status: "open",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(issue))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runIssueShow(client, &buf, "issue-1111", true)
	if err != nil {
		t.Fatalf("runIssueShow JSON 오류: %v", err)
	}

	// 유효한 JSON 출력 확인
	var parsed Issue
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if parsed.ID != "issue-1111" {
		t.Errorf("예상치 않은 ID: %s", parsed.ID)
	}
}

// TestRunIssueUpdateJSON는 runIssueUpdate의 JSON 출력 경로를 테스트합니다.
func TestRunIssueUpdateJSON(t *testing.T) {
	issue := Issue{
		ID:    "issue-1111",
		Title: "수정된 이슈 제목",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(issue))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runIssueUpdate(client, &buf, "issue-1111", "수정된 이슈 제목", "", "", true)
	if err != nil {
		t.Fatalf("runIssueUpdate JSON 오류: %v", err)
	}

	// 유효한 JSON 출력 확인
	var parsed Issue
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if parsed.ID != "issue-1111" {
		t.Errorf("예상치 않은 ID: %s", parsed.ID)
	}
}

// TestRunIssueAssignJSON는 runIssueAssign의 JSON 출력 경로를 테스트합니다.
func TestRunIssueAssignJSON(t *testing.T) {
	issue := Issue{
		ID:         "issue-1111",
		AssigneeID: "agent-bbb",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(issue))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runIssueAssign(client, &buf, "issue-1111", "agent-bbb", true)
	if err != nil {
		t.Fatalf("runIssueAssign JSON 오류: %v", err)
	}

	// 유효한 JSON 출력 확인
	var parsed Issue
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if parsed.AssigneeID != "agent-bbb" {
		t.Errorf("예상치 않은 AssigneeID: %s", parsed.AssigneeID)
	}
}

// TestRunIssueUpdateAllFields는 runIssueUpdate의 description과 priority 분기를 테스트합니다.
func TestRunIssueUpdateAllFields(t *testing.T) {
	issue := Issue{
		ID:    "issue-1111",
		Title: "수정된 제목",
	}

	var capturedBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedBody)
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(issue))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	// title, description, priority 모두 지정하여 각 조건 분기 커버
	err := runIssueUpdate(client, &buf, "issue-1111", "수정된 제목", "새 설명", "medium", false)
	if err != nil {
		t.Fatalf("runIssueUpdate all fields 오류: %v", err)
	}

	// 요청 본문에 모든 필드 확인
	if capturedBody["description"] != "새 설명" {
		t.Errorf("요청 본문 description = %v, want '새 설명'", capturedBody["description"])
	}
	if capturedBody["priority"] != "medium" {
		t.Errorf("요청 본문 priority = %v, want 'medium'", capturedBody["priority"])
	}
}

// TestRunIssueList_InvalidProjectID는 유효하지 않은 projectID에서 ValidateID 에러를 반환하는지 검증합니다.
func TestRunIssueList_InvalidProjectID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	invalidIDs := []string{"", "id/slash", "id with space", "../etc/passwd"}
	for _, id := range invalidIDs {
		err := runIssueList(client, &buf, id, "", "", "", false)
		if err == nil {
			t.Errorf("유효하지 않은 projectID %q에서 에러가 발생해야 합니다", id)
			continue
		}
		if !strings.Contains(err.Error(), "유효하지 않은 ID") {
			t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
		}
	}
}

// TestRunIssueShow_InvalidIssueID는 유효하지 않은 issueID에서 ValidateID 에러를 반환하는지 검증합니다.
func TestRunIssueShow_InvalidIssueID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runIssueShow(client, &buf, "../bad", false)
	if err == nil {
		t.Error("유효하지 않은 issueID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

// TestRunIssueCreate_InvalidProjectID는 유효하지 않은 projectID에서 ValidateID 에러를 반환하는지 검증합니다.
func TestRunIssueCreate_InvalidProjectID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runIssueCreate(client, &buf, "bad/id", "제목", "", "", "", false)
	if err == nil {
		t.Error("유효하지 않은 projectID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

// TestRunIssueUpdate_InvalidIssueID는 유효하지 않은 issueID에서 ValidateID 에러를 반환하는지 검증합니다.
func TestRunIssueUpdate_InvalidIssueID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runIssueUpdate(client, &buf, "bad id", "", "", "", false)
	if err == nil {
		t.Error("유효하지 않은 issueID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

// TestRunIssueAssign_InvalidIssueID는 유효하지 않은 issueID에서 ValidateID 에러를 반환하는지 검증합니다.
func TestRunIssueAssign_InvalidIssueID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runIssueAssign(client, &buf, "bad.id", "agent-1", false)
	if err == nil {
		t.Error("유효하지 않은 issueID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

// TestRunIssueCommentList_InvalidIssueID는 유효하지 않은 issueID에서 ValidateID 에러를 반환하는지 검증합니다.
func TestRunIssueCommentList_InvalidIssueID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runIssueCommentList(client, &buf, "bad id", false)
	if err == nil {
		t.Error("유효하지 않은 issueID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

// TestRunIssueCommentAdd_InvalidIssueID는 유효하지 않은 issueID에서 ValidateID 에러를 반환하는지 검증합니다.
func TestRunIssueCommentAdd_InvalidIssueID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runIssueCommentAdd(client, &buf, "../bad", "내용", false)
	if err == nil {
		t.Error("유효하지 않은 issueID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

func TestRunIssueCommentAdd(t *testing.T) {
	comment := IssueComment{
		ID:      "cmt-new-1",
		Content: "새 댓글 내용",
	}

	var capturedBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/issues/issue-1111/comments" || r.Method != http.MethodPost {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		json.NewDecoder(r.Body).Decode(&capturedBody)
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(comment))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runIssueCommentAdd(client, &buf, "issue-1111", "새 댓글 내용", false)
	if err != nil {
		t.Fatalf("runIssueCommentAdd 오류: %v", err)
	}

	if capturedBody["content"] != "새 댓글 내용" {
		t.Errorf("요청 본문 content = %v, want '새 댓글 내용'", capturedBody["content"])
	}
}

// TestRunIssueCommentAddJSON는 runIssueCommentAdd의 JSON 출력 경로를 테스트합니다.
func TestRunIssueCommentAddJSON(t *testing.T) {
	comment := IssueComment{
		ID:      "cmt-new-1",
		Content: "새 댓글 내용",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(comment))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runIssueCommentAdd(client, &buf, "issue-1111", "새 댓글 내용", true)
	if err != nil {
		t.Fatalf("runIssueCommentAdd JSON 오류: %v", err)
	}

	// 유효한 JSON 출력 확인
	var parsed IssueComment
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if parsed.ID != "cmt-new-1" {
		t.Errorf("예상치 않은 ID: %s", parsed.ID)
	}
}
