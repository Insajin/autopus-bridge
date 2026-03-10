// attachment_test.go는 attachment 서브커맨드 핸들러 함수를 테스트합니다.
package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunAttachmentList(t *testing.T) {
	attachments := []Attachment{
		{ID: "att-1", Filename: "report.pdf", Size: 1024, MimeType: "application/pdf"},
		{ID: "att-2", Filename: "notes.txt", Size: 256, MimeType: "text/plain"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/issues/issue-1/attachments" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(attachments))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runAttachmentList(client, &buf, "issue-1", false)
	if err != nil {
		t.Fatalf("runAttachmentList 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "report.pdf") {
		t.Errorf("출력에 'report.pdf'가 없습니다: %s", out)
	}
	if !strings.Contains(out, "notes.txt") {
		t.Errorf("출력에 'notes.txt'가 없습니다: %s", out)
	}
}

func TestRunAttachmentListJSON(t *testing.T) {
	attachments := []Attachment{
		{ID: "att-1", Filename: "report.pdf", Size: 1024, MimeType: "application/pdf"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(attachments))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runAttachmentList(client, &buf, "issue-1", true)
	if err != nil {
		t.Fatalf("runAttachmentList JSON 오류: %v", err)
	}

	var parsed []Attachment
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if len(parsed) != 1 || parsed[0].ID != "att-1" {
		t.Errorf("예상치 않은 결과: %v", parsed)
	}
}

func TestRunAttachmentList_InvalidIssueID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runAttachmentList(client, &buf, "bad/id", false)
	if err == nil {
		t.Error("유효하지 않은 issueID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

func TestRunAttachmentShow(t *testing.T) {
	att := Attachment{ID: "att-1", Filename: "report.pdf", Size: 1024, MimeType: "application/pdf", IssueID: "issue-1"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/attachments/att-1" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(att))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runAttachmentShow(client, &buf, "att-1", false)
	if err != nil {
		t.Fatalf("runAttachmentShow 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "report.pdf") {
		t.Errorf("출력에 'report.pdf'가 없습니다: %s", out)
	}
	if !strings.Contains(out, "application/pdf") {
		t.Errorf("출력에 'application/pdf'가 없습니다: %s", out)
	}
}

func TestRunAttachmentShowJSON(t *testing.T) {
	att := Attachment{ID: "att-1", Filename: "report.pdf", Size: 1024, MimeType: "application/pdf"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(att))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runAttachmentShow(client, &buf, "att-1", true)
	if err != nil {
		t.Fatalf("runAttachmentShow JSON 오류: %v", err)
	}

	var parsed Attachment
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if parsed.ID != "att-1" {
		t.Errorf("예상치 않은 ID: %s", parsed.ID)
	}
}

func TestRunAttachmentShow_InvalidID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runAttachmentShow(client, &buf, "bad/id", false)
	if err == nil {
		t.Error("유효하지 않은 attachmentID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

func TestRunAttachmentUpload(t *testing.T) {
	returnedAtt := Attachment{ID: "att-new", Filename: "test.txt", Size: 11, MimeType: "text/plain", IssueID: "issue-1"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/issues/issue-1/attachments" || r.Method != http.MethodPost {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(returnedAtt))
	}))
	defer srv.Close()

	// 임시 파일 생성
	tmpFile, err := os.CreateTemp("", "test-upload-*.txt")
	if err != nil {
		t.Fatalf("임시 파일 생성 실패: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString("hello world"); err != nil {
		t.Fatalf("임시 파일 쓰기 실패: %v", err)
	}
	tmpFile.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err = runAttachmentUpload(client, &buf, "issue-1", tmpFile.Name(), false)
	if err != nil {
		t.Fatalf("runAttachmentUpload 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "att-new") && !strings.Contains(out, filepath.Base(tmpFile.Name())) {
		t.Errorf("출력에 업로드된 파일 정보가 없습니다: %s", out)
	}
}

func TestRunAttachmentUpload_InvalidIssueID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runAttachmentUpload(client, &buf, "bad/id", "/tmp/test.txt", false)
	if err == nil {
		t.Error("유효하지 않은 issueID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

func TestRunAttachmentUpload_FileNotFound(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runAttachmentUpload(client, &buf, "issue-1", "/nonexistent/file.txt", false)
	if err == nil {
		t.Error("존재하지 않는 파일에서 에러가 발생해야 합니다")
	}
}

func TestRunAttachmentDelete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/attachments/att-1" || r.Method != http.MethodDelete {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(map[string]string{"message": "deleted"}))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runAttachmentDelete(client, &buf, "att-1")
	if err != nil {
		t.Fatalf("runAttachmentDelete 오류: %v", err)
	}
}

func TestRunAttachmentDelete_InvalidID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runAttachmentDelete(client, &buf, "bad id")
	if err == nil {
		t.Error("유효하지 않은 attachmentID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

func TestRunAttachmentDelete_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"success":false,"error":"not found"}`, http.StatusNotFound)
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runAttachmentDelete(client, &buf, "att-nonexistent")
	if err == nil {
		t.Fatal("에러가 예상되었으나 nil 반환")
	}
}

func TestRunAttachmentDownload_WithoutOutput(t *testing.T) {
	fileContent := []byte("file content here")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/attachments/att-1/download" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write(fileContent)
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runAttachmentDownload(client, &buf, "att-1", "")
	if err != nil {
		t.Fatalf("runAttachmentDownload 오류: %v", err)
	}
}

func TestRunAttachmentDownload_WithOutput(t *testing.T) {
	fileContent := []byte("file content here")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/attachments/att-1/download" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write(fileContent)
	}))
	defer srv.Close()

	// 임시 출력 파일 경로
	tmpOut := filepath.Join(os.TempDir(), "att-download-test.bin")
	defer os.Remove(tmpOut)

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runAttachmentDownload(client, &buf, "att-1", tmpOut)
	if err != nil {
		t.Fatalf("runAttachmentDownload --output 오류: %v", err)
	}

	// 저장된 파일 확인
	saved, err := os.ReadFile(tmpOut)
	if err != nil {
		t.Fatalf("저장된 파일 읽기 실패: %v", err)
	}
	if !bytes.Equal(saved, fileContent) {
		t.Errorf("저장된 파일 내용이 다릅니다: %v vs %v", saved, fileContent)
	}
}

func TestRunAttachmentDownload_InvalidID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runAttachmentDownload(client, &buf, "bad/id", "")
	if err == nil {
		t.Error("유효하지 않은 attachmentID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}
