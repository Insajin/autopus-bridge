package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/insajin/autopus-bridge/internal/auth"
	"github.com/insajin/autopus-bridge/internal/mcpserver"
	"github.com/rs/zerolog"

	"github.com/insajin/autopus-bridge/internal/apiclient"
)

func makeAPITestClient(serverURL, workspaceID string) *apiclient.Client {
	creds := &auth.Credentials{
		AccessToken: "test-token",
		ServerURL:   serverURL,
		WorkspaceID: workspaceID,
		ExpiresAt:   time.Now().Add(1 * time.Hour), // 만료되지 않은 토큰
	}
	tr := auth.NewTokenRefresher(creds)
	backend := mcpserver.NewBackendClient(serverURL, tr, 0, zerolog.Nop())
	return apiclient.New(backend, creds, tr)
}

func TestRunRawAPI_GET(t *testing.T) {
	// Mock 서버
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path != "/api/v1/workspaces" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"data":    []string{"ws-1", "ws-2"},
		})
	}))
	defer srv.Close()

	client := makeAPITestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runRawAPI(context.Background(), client, &buf, http.MethodGet, "/api/v1/workspaces", "", nil)
	if err != nil {
		t.Fatalf("runRawAPI GET 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "ws-1") {
		t.Errorf("출력에 'ws-1'이 없습니다: %s", out)
	}
}

func TestRunRawAPI_POST(t *testing.T) {
	var receivedBody map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"data":    map[string]string{"id": "msg-1"},
		})
	}))
	defer srv.Close()

	client := makeAPITestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	body := `{"content":"hello"}`
	err := runRawAPI(context.Background(), client, &buf, http.MethodPost, "/api/v1/channels/ch-1/messages", body, nil)
	if err != nil {
		t.Fatalf("runRawAPI POST 오류: %v", err)
	}

	if receivedBody == nil {
		t.Fatal("요청 본문이 수신되지 않았습니다")
	}
	if content, ok := receivedBody["content"].(string); !ok || content != "hello" {
		t.Errorf("수신된 본문: %v", receivedBody)
	}
}

func TestRunRawAPI_WorkspaceIDReplace(t *testing.T) {
	// :workspaceId가 실제 workspace ID로 치환되는지 확인
	var capturedPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"data":    map[string]string{},
		})
	}))
	defer srv.Close()

	client := makeAPITestClient(srv.URL, "actual-ws-id")
	var buf bytes.Buffer

	err := runRawAPI(context.Background(), client, &buf, http.MethodGet, "/api/v1/workspaces/:workspaceId/agents", "", nil)
	if err != nil {
		t.Fatalf("runRawAPI 오류: %v", err)
	}

	if !strings.Contains(capturedPath, "actual-ws-id") {
		t.Errorf("워크스페이스 ID 치환 실패. 경로: %s", capturedPath)
	}
	if strings.Contains(capturedPath, ":workspaceId") {
		t.Errorf(":workspaceId가 치환되지 않았습니다. 경로: %s", capturedPath)
	}
}

func TestRunRawAPI_ShowsStatusCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"data":    nil,
		})
	}))
	defer srv.Close()

	client := makeAPITestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runRawAPI(context.Background(), client, &buf, http.MethodGet, "/api/v1/test", "", nil)
	if err != nil {
		t.Fatalf("runRawAPI 오류: %v", err)
	}

	// HTTP 상태 코드가 출력에 포함되어야 합니다
	out := buf.String()
	if !strings.Contains(out, "200") {
		t.Errorf("출력에 HTTP 상태 코드 200이 없습니다: %s", out)
	}
}
