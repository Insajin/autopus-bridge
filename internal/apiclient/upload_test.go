// upload_test.go는 DoUpload 함수의 단위 테스트를 제공합니다.
package apiclient_test

import (
	"encoding/json"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/insajin/autopus-bridge/internal/apiclient"
	"github.com/insajin/autopus-bridge/internal/auth"
	"time"
)

// makeTempFile은 테스트용 임시 파일을 생성합니다.
func makeTempFile(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("임시 파일 생성 실패: %v", err)
	}
	return path
}

// makeUploadTestClient는 업로드 테스트용 클라이언트를 생성합니다.
func makeUploadTestClient(t *testing.T, serverURL string) *apiclient.Client {
	t.Helper()
	creds := &auth.Credentials{
		AccessToken: "test-token",
		ServerURL:   serverURL,
		WorkspaceID: "ws-test",
		ExpiresAt:   time.Now().Add(1 * time.Hour),
	}
	tr := auth.NewTokenRefresher(creds)
	return apiclient.NewClientForTest(serverURL, creds, tr)
}

// TestDoUpload_Success는 정상적인 파일 업로드를 검증합니다.
func TestDoUpload_Success(t *testing.T) {
	t.Parallel()

	// 테스트 서버: multipart 업로드를 수신하고 성공 응답 반환
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Method = %s, want POST", r.Method)
		}

		// Content-Type이 multipart/form-data인지 확인
		ct := r.Header.Get("Content-Type")
		mediaType, params, err := mime.ParseMediaType(ct)
		if err != nil || mediaType != "multipart/form-data" {
			t.Errorf("Content-Type = %q, want multipart/form-data", ct)
		}

		// multipart 파싱
		mr := multipart.NewReader(r.Body, params["boundary"])
		form, err := mr.ReadForm(10 << 20)
		if err != nil {
			t.Fatalf("multipart 파싱 실패: %v", err)
		}
		defer form.RemoveAll()

		// file 파트 확인
		if files, ok := form.File["file"]; !ok || len(files) == 0 {
			t.Error("file 파트가 없습니다")
		}

		// 성공 응답 반환
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"success": true,
			"data":    map[string]string{"id": "uploaded-file-1", "url": "https://example.com/file"},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	// 테스트 파일 생성 (.txt 허용 확장자)
	filePath := makeTempFile(t, "test.txt", "hello upload content")

	client := makeUploadTestClient(t, srv.URL)
	ctx, cancel := apiclient.NewContextWithTimeout(10 * apiclient.SecondDuration)
	defer cancel()

	result, err := apiclient.DoUpload(client, ctx, "/api/v1/upload", filePath, nil)
	if err != nil {
		t.Fatalf("DoUpload() 오류: %v", err)
	}
	if result == nil {
		t.Fatal("DoUpload()가 nil을 반환했습니다")
	}
}

// TestDoUpload_WithExtraFields는 추가 필드가 포함된 업로드를 검증합니다.
func TestDoUpload_WithExtraFields(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ct := r.Header.Get("Content-Type")
		_, params, _ := mime.ParseMediaType(ct)
		mr := multipart.NewReader(r.Body, params["boundary"])
		form, err := mr.ReadForm(10 << 20)
		if err != nil {
			t.Fatalf("multipart 파싱 실패: %v", err)
		}
		defer form.RemoveAll()

		// 추가 필드 확인
		if vals, ok := form.Value["project_id"]; !ok || len(vals) == 0 || vals[0] != "proj-123" {
			t.Errorf("project_id 필드가 없거나 값이 다릅니다: %v", form.Value)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"data":    map[string]string{"id": "file-with-field"},
		})
	}))
	defer srv.Close()

	filePath := makeTempFile(t, "doc.md", "# 마크다운 문서")

	client := makeUploadTestClient(t, srv.URL)
	ctx, cancel := apiclient.NewContextWithTimeout(10 * apiclient.SecondDuration)
	defer cancel()

	extraFields := map[string]string{"project_id": "proj-123"}
	result, err := apiclient.DoUpload(client, ctx, "/api/v1/upload", filePath, extraFields)
	if err != nil {
		t.Fatalf("DoUpload() with extra fields 오류: %v", err)
	}
	if result == nil {
		t.Fatal("결과가 nil입니다")
	}
}

// TestDoUpload_InvalidExtension은 허용되지 않는 파일 확장자 거부를 검증합니다.
func TestDoUpload_InvalidExtension(t *testing.T) {
	t.Parallel()

	filePath := makeTempFile(t, "bad.exe", "binary content")
	client := makeUploadTestClient(t, "http://localhost:9999") // 서버에 도달하지 않아야 함
	ctx, cancel := apiclient.NewContextWithTimeout(10 * apiclient.SecondDuration)
	defer cancel()

	_, err := apiclient.DoUpload(client, ctx, "/api/v1/upload", filePath, nil)
	if err == nil {
		t.Fatal("허용되지 않는 확장자에서 에러가 발생해야 합니다")
	}
	if !strings.Contains(err.Error(), "허용되지 않는 파일 형식") {
		t.Errorf("에러 메시지에 '허용되지 않는 파일 형식'이 없습니다: %v", err)
	}
}

// TestDoUpload_FileTooLarge는 10MB 초과 파일 거부를 검증합니다.
func TestDoUpload_FileTooLarge(t *testing.T) {
	t.Parallel()

	// 10MB + 1 바이트 파일 생성
	dir := t.TempDir()
	filePath := filepath.Join(dir, "large.pdf")
	// 실제로 10MB+1 파일을 생성 (os.WriteFile로 크기 조절)
	largeContent := strings.Repeat("x", 10*1024*1024+1)
	if err := os.WriteFile(filePath, []byte(largeContent), 0600); err != nil {
		t.Fatalf("대용량 임시 파일 생성 실패: %v", err)
	}

	client := makeUploadTestClient(t, "http://localhost:9999")
	ctx, cancel := apiclient.NewContextWithTimeout(10 * apiclient.SecondDuration)
	defer cancel()

	_, err := apiclient.DoUpload(client, ctx, "/api/v1/upload", filePath, nil)
	if err == nil {
		t.Fatal("크기 초과 파일에서 에러가 발생해야 합니다")
	}
	if !strings.Contains(err.Error(), "파일 크기가 10MB를 초과합니다") {
		t.Errorf("에러 메시지에 '파일 크기가 10MB를 초과합니다'가 없습니다: %v", err)
	}
}

// TestDoUpload_FileNotFound는 존재하지 않는 파일 경로를 처리합니다.
func TestDoUpload_FileNotFound(t *testing.T) {
	t.Parallel()

	client := makeUploadTestClient(t, "http://localhost:9999")
	ctx, cancel := apiclient.NewContextWithTimeout(10 * apiclient.SecondDuration)
	defer cancel()

	_, err := apiclient.DoUpload(client, ctx, "/api/v1/upload", "/nonexistent/path/file.txt", nil)
	if err == nil {
		t.Fatal("존재하지 않는 파일에서 에러가 발생해야 합니다")
	}
}

// TestDoUpload_AllowedExtensions는 모든 허용 확장자를 검증합니다.
func TestDoUpload_AllowedExtensions(t *testing.T) {
	t.Parallel()

	// 허용 확장자 목록
	allowedExts := []string{".pdf", ".doc", ".docx", ".txt", ".md"}

	for _, ext := range allowedExts {
		ext := ext
		t.Run(ext, func(t *testing.T) {
			t.Parallel()

			// 각 서브테스트마다 독립된 서버를 생성하여 병렬 실행 시 서버 종료 경쟁 방지
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]interface{}{
					"success": true,
					"data":    map[string]string{"id": "ok"},
				})
			}))
			defer srv.Close()

			client := makeUploadTestClient(t, srv.URL)
			filePath := makeTempFile(t, "file"+ext, "content")
			ctx, cancel := apiclient.NewContextWithTimeout(10 * apiclient.SecondDuration)
			defer cancel()

			_, err := apiclient.DoUpload(client, ctx, "/api/v1/upload", filePath, nil)
			if err != nil {
				t.Errorf("허용된 확장자 %s에서 에러 발생: %v", ext, err)
			}
		})
	}
}
