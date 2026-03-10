// label_test.go는 label 서브커맨드 핸들러 함수를 테스트합니다.
package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunLabelList(t *testing.T) {
	labels := []Label{
		{ID: "lbl-1", Name: "bug", Color: "#ff0000"},
		{ID: "lbl-2", Name: "feature", Color: "#00ff00"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/projects/proj-1/labels" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(labels))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runLabelList(client, &buf, "proj-1", false)
	if err != nil {
		t.Fatalf("runLabelList 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "bug") {
		t.Errorf("출력에 'bug'가 없습니다: %s", out)
	}
	if !strings.Contains(out, "feature") {
		t.Errorf("출력에 'feature'가 없습니다: %s", out)
	}
}

func TestRunLabelListJSON(t *testing.T) {
	labels := []Label{
		{ID: "lbl-1", Name: "bug", Color: "#ff0000"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(labels))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runLabelList(client, &buf, "proj-1", true)
	if err != nil {
		t.Fatalf("runLabelList JSON 오류: %v", err)
	}

	var parsed []Label
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if len(parsed) != 1 || parsed[0].ID != "lbl-1" {
		t.Errorf("예상치 않은 결과: %v", parsed)
	}
}

func TestRunLabelList_InvalidProjectID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runLabelList(client, &buf, "bad/id", false)
	if err == nil {
		t.Error("유효하지 않은 projectID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

func TestRunLabelCreate(t *testing.T) {
	newLabel := Label{ID: "lbl-new", Name: "urgent", Color: "#ff0000", ProjectID: "proj-1"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/projects/proj-1/labels" || r.Method != http.MethodPost {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(newLabel))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runLabelCreate(client, &buf, "proj-1", "urgent", "#ff0000", false)
	if err != nil {
		t.Fatalf("runLabelCreate 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "urgent") {
		t.Errorf("출력에 'urgent'가 없습니다: %s", out)
	}
}

func TestRunLabelCreate_InvalidProjectID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runLabelCreate(client, &buf, "bad id", "name", "#000", false)
	if err == nil {
		t.Error("유효하지 않은 projectID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

func TestRunLabelUpdate(t *testing.T) {
	updated := Label{ID: "lbl-1", Name: "critical", Color: "#ff0000"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/labels/lbl-1" || r.Method != http.MethodPatch {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(updated))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runLabelUpdate(client, &buf, "lbl-1", "critical", "", false)
	if err != nil {
		t.Fatalf("runLabelUpdate 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "critical") {
		t.Errorf("출력에 'critical'이 없습니다: %s", out)
	}
}

func TestRunLabelUpdate_InvalidLabelID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runLabelUpdate(client, &buf, "bad/id", "name", "", false)
	if err == nil {
		t.Error("유효하지 않은 labelID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

func TestRunLabelDelete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/labels/lbl-1" || r.Method != http.MethodDelete {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(map[string]string{"message": "deleted"}))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runLabelDelete(client, &buf, "lbl-1")
	if err != nil {
		t.Fatalf("runLabelDelete 오류: %v", err)
	}
}

func TestRunLabelDelete_InvalidLabelID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runLabelDelete(client, &buf, "bad id")
	if err == nil {
		t.Error("유효하지 않은 labelID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

func TestRunLabelAdd(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/issues/issue-1/labels/lbl-1" || r.Method != http.MethodPost {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(map[string]string{"message": "added"}))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runLabelAdd(client, &buf, "issue-1", "lbl-1")
	if err != nil {
		t.Fatalf("runLabelAdd 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "라벨 추가 완료") {
		t.Errorf("출력에 '라벨 추가 완료'가 없습니다: %s", out)
	}
}

func TestRunLabelAdd_InvalidIssueID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runLabelAdd(client, &buf, "bad/id", "lbl-1")
	if err == nil {
		t.Error("유효하지 않은 issueID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

func TestRunLabelAdd_InvalidLabelID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runLabelAdd(client, &buf, "issue-1", "bad id")
	if err == nil {
		t.Error("유효하지 않은 labelID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

func TestRunLabelRemove(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/issues/issue-1/labels/lbl-1" || r.Method != http.MethodDelete {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(map[string]string{"message": "removed"}))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runLabelRemove(client, &buf, "issue-1", "lbl-1")
	if err != nil {
		t.Fatalf("runLabelRemove 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "라벨 제거 완료") {
		t.Errorf("출력에 '라벨 제거 완료'가 없습니다: %s", out)
	}
}

func TestRunLabelRemove_InvalidIssueID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runLabelRemove(client, &buf, "bad/id", "lbl-1")
	if err == nil {
		t.Error("유효하지 않은 issueID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

func TestRunLabelRemove_InvalidLabelID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runLabelRemove(client, &buf, "issue-1", "bad id")
	if err == nil {
		t.Error("유효하지 않은 labelID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

func TestRunLabelDelete_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"success":false,"error":"not found"}`, http.StatusNotFound)
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runLabelDelete(client, &buf, "lbl-nonexistent")
	if err == nil {
		t.Fatal("에러가 예상되었으나 nil 반환")
	}
}

func TestRunLabelCreateJSON(t *testing.T) {
	newLabel := Label{ID: "lbl-new", Name: "urgent", Color: "#ff0000"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(newLabel))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runLabelCreate(client, &buf, "proj-1", "urgent", "#ff0000", true)
	if err != nil {
		t.Fatalf("runLabelCreate JSON 오류: %v", err)
	}

	var parsed Label
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if parsed.ID != "lbl-new" {
		t.Errorf("예상치 않은 ID: %s", parsed.ID)
	}
}
