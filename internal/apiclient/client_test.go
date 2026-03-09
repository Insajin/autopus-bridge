package apiclient_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/insajin/autopus-bridge/internal/apiclient"
	"github.com/insajin/autopus-bridge/internal/auth"
	"github.com/insajin/autopus-bridge/internal/mcpserver"
	"github.com/rs/zerolog"
)

// newTestBackendClient는 테스트용 BackendClient를 생성합니다.
func newTestBackendClient(baseURL string, creds *auth.Credentials) *mcpserver.BackendClient {
	tokenRefresher := auth.NewTokenRefresher(creds)
	return mcpserver.NewBackendClient(baseURL, tokenRefresher, 10*time.Second, zerolog.Nop())
}

// newTestCreds는 테스트용 Credentials를 생성합니다.
// 만료 시간을 1시간 후로 설정하여 유효한 상태를 유지합니다.
func newTestCreds(serverURL string) *auth.Credentials {
	return &auth.Credentials{
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		ExpiresAt:    time.Now().Add(1 * time.Hour),
		ServerURL:    serverURL,
		WorkspaceID:  "ws-123",
	}
}

// TestNew는 Client 생성자를 검증합니다.
func TestNew(t *testing.T) {
	t.Parallel()

	creds := newTestCreds("https://api.example.com")
	tokenRefresher := auth.NewTokenRefresher(creds)
	backend := mcpserver.NewBackendClient("https://api.example.com", tokenRefresher, 10*time.Second, zerolog.Nop())

	client := apiclient.New(backend, creds, tokenRefresher)
	if client == nil {
		t.Fatal("New()가 nil을 반환했습니다")
	}
}

// TestClient_WorkspaceID는 WorkspaceID 반환을 검증합니다.
func TestClient_WorkspaceID(t *testing.T) {
	t.Parallel()

	creds := newTestCreds("https://api.example.com")
	creds.WorkspaceID = "ws-test-456"
	tokenRefresher := auth.NewTokenRefresher(creds)
	backend := mcpserver.NewBackendClient("https://api.example.com", tokenRefresher, 10*time.Second, zerolog.Nop())

	client := apiclient.New(backend, creds, tokenRefresher)
	if got := client.WorkspaceID(); got != "ws-test-456" {
		t.Errorf("WorkspaceID() = %q, want ws-test-456", got)
	}
}

// TestClient_Token은 Token 반환을 검증합니다.
func TestClient_Token(t *testing.T) {
	t.Parallel()

	creds := newTestCreds("https://api.example.com")
	creds.AccessToken = "my-test-token"
	tokenRefresher := auth.NewTokenRefresher(creds)
	backend := mcpserver.NewBackendClient("https://api.example.com", tokenRefresher, 10*time.Second, zerolog.Nop())

	client := apiclient.New(backend, creds, tokenRefresher)
	token, err := client.Token()
	if err != nil {
		t.Fatalf("Token() 오류: %v", err)
	}
	if token != "my-test-token" {
		t.Errorf("Token() = %q, want my-test-token", token)
	}
}

// TestClient_JSONOutput는 JSON 출력 플래그 설정/조회를 검증합니다.
func TestClient_JSONOutput(t *testing.T) {
	t.Parallel()

	creds := newTestCreds("https://api.example.com")
	tokenRefresher := auth.NewTokenRefresher(creds)
	backend := mcpserver.NewBackendClient("https://api.example.com", tokenRefresher, 10*time.Second, zerolog.Nop())
	client := apiclient.New(backend, creds, tokenRefresher)

	if client.IsJSONOutput() {
		t.Error("기본값은 false여야 합니다")
	}

	client.SetJSONOutput(true)
	if !client.IsJSONOutput() {
		t.Error("SetJSONOutput(true) 후 IsJSONOutput()은 true여야 합니다")
	}

	client.SetJSONOutput(false)
	if client.IsJSONOutput() {
		t.Error("SetJSONOutput(false) 후 IsJSONOutput()은 false여야 합니다")
	}
}

// TestClient_ResolvePath는 경로의 :workspaceId 치환을 검증합니다.
func TestClient_ResolvePath(t *testing.T) {
	t.Parallel()

	creds := newTestCreds("https://api.example.com")
	creds.WorkspaceID = "ws-abc"
	tokenRefresher := auth.NewTokenRefresher(creds)
	backend := mcpserver.NewBackendClient("https://api.example.com", tokenRefresher, 10*time.Second, zerolog.Nop())
	client := apiclient.New(backend, creds, tokenRefresher)

	tests := []struct {
		input string
		want  string
	}{
		{"/api/v1/workspaces/:workspaceId/agents", "/api/v1/workspaces/ws-abc/agents"},
		{"/api/v1/workspaces/:workspaceId/channels/:workspaceId", "/api/v1/workspaces/ws-abc/channels/ws-abc"},
		{"/api/v1/agents", "/api/v1/agents"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got := client.ResolvePath(tt.input)
			if got != tt.want {
				t.Errorf("ResolvePath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestClient_BaseURL은 BaseURL 반환을 검증합니다.
func TestClient_BaseURL(t *testing.T) {
	t.Parallel()

	creds := newTestCreds("https://api.example.com")
	tokenRefresher := auth.NewTokenRefresher(creds)
	backend := mcpserver.NewBackendClient("https://api.example.com", tokenRefresher, 10*time.Second, zerolog.Nop())
	client := apiclient.New(backend, creds, tokenRefresher)

	if got := client.BaseURL(); got != "https://api.example.com" {
		t.Errorf("BaseURL() = %q, want https://api.example.com", got)
	}
}

// TestClient_Get는 GET 요청을 검증합니다.
func TestClient_Get(t *testing.T) {
	t.Parallel()

	// 테스트 서버 준비
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("Method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/api/v1/test" {
			t.Errorf("Path = %s, want /api/v1/test", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"data":    map[string]string{"id": "1"},
		})
	}))
	defer srv.Close()

	creds := newTestCreds(srv.URL)
	backend := newTestBackendClient(srv.URL, creds)
	tokenRefresher := auth.NewTokenRefresher(creds)
	client := apiclient.New(backend, creds, tokenRefresher)

	resp, err := client.Get(context.Background(), "/api/v1/test")
	if err != nil {
		t.Fatalf("Get() 오류: %v", err)
	}
	if !resp.Success {
		t.Errorf("Success = %v, want true", resp.Success)
	}
}

// TestClient_Post는 POST 요청을 검증합니다.
func TestClient_Post(t *testing.T) {
	t.Parallel()

	type reqBody struct {
		Name string `json:"name"`
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Method = %s, want POST", r.Method)
		}
		var body reqBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("요청 바디 파싱 실패: %v", err)
		}
		if body.Name != "test" {
			t.Errorf("Name = %q, want test", body.Name)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"data":    map[string]string{"id": "created-1"},
		})
	}))
	defer srv.Close()

	creds := newTestCreds(srv.URL)
	backend := newTestBackendClient(srv.URL, creds)
	tokenRefresher := auth.NewTokenRefresher(creds)
	client := apiclient.New(backend, creds, tokenRefresher)

	resp, err := client.Post(context.Background(), "/api/v1/items", reqBody{Name: "test"})
	if err != nil {
		t.Fatalf("Post() 오류: %v", err)
	}
	if !resp.Success {
		t.Errorf("Success = %v, want true", resp.Success)
	}
}

// TestClient_Patch는 PATCH 요청을 검증합니다.
func TestClient_Patch(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("Method = %s, want PATCH", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"data":    map[string]string{"id": "updated-1"},
		})
	}))
	defer srv.Close()

	creds := newTestCreds(srv.URL)
	backend := newTestBackendClient(srv.URL, creds)
	tokenRefresher := auth.NewTokenRefresher(creds)
	client := apiclient.New(backend, creds, tokenRefresher)

	resp, err := client.Patch(context.Background(), "/api/v1/items/1", map[string]string{"name": "updated"})
	if err != nil {
		t.Fatalf("Patch() 오류: %v", err)
	}
	if !resp.Success {
		t.Errorf("Success = %v, want true", resp.Success)
	}
}

// TestClient_Delete는 DELETE 요청을 검증합니다.
func TestClient_Delete(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("Method = %s, want DELETE", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"data":    map[string]string{"deleted": "true"},
		})
	}))
	defer srv.Close()

	creds := newTestCreds(srv.URL)
	backend := newTestBackendClient(srv.URL, creds)
	tokenRefresher := auth.NewTokenRefresher(creds)
	client := apiclient.New(backend, creds, tokenRefresher)

	resp, err := client.Delete(context.Background(), "/api/v1/items/1")
	if err != nil {
		t.Fatalf("Delete() 오류: %v", err)
	}
	if !resp.Success {
		t.Errorf("Success = %v, want true", resp.Success)
	}
}

// testItem은 Do[T] 테스트용 타입입니다.
type testItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// TestDo는 제네릭 단건 응답 파싱을 검증합니다.
func TestDo(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"data":    map[string]string{"id": "item-1", "name": "테스트 아이템"},
		})
	}))
	defer srv.Close()

	creds := newTestCreds(srv.URL)
	backend := newTestBackendClient(srv.URL, creds)
	tokenRefresher := auth.NewTokenRefresher(creds)
	client := apiclient.New(backend, creds, tokenRefresher)

	item, err := apiclient.Do[testItem](client, context.Background(), http.MethodGet, "/api/v1/items/1", nil)
	if err != nil {
		t.Fatalf("Do() 오류: %v", err)
	}
	if item.ID != "item-1" {
		t.Errorf("ID = %q, want item-1", item.ID)
	}
	if item.Name != "테스트 아이템" {
		t.Errorf("Name = %q, want 테스트 아이템", item.Name)
	}
}

// TestDoList는 제네릭 배열 응답 파싱을 검증합니다.
func TestDoList(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"data": []map[string]string{
				{"id": "1", "name": "아이템1"},
				{"id": "2", "name": "아이템2"},
			},
		})
	}))
	defer srv.Close()

	creds := newTestCreds(srv.URL)
	backend := newTestBackendClient(srv.URL, creds)
	tokenRefresher := auth.NewTokenRefresher(creds)
	client := apiclient.New(backend, creds, tokenRefresher)

	items, err := apiclient.DoList[testItem](client, context.Background(), http.MethodGet, "/api/v1/items", nil)
	if err != nil {
		t.Fatalf("DoList() 오류: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	if items[0].ID != "1" {
		t.Errorf("items[0].ID = %q, want 1", items[0].ID)
	}
	if items[1].Name != "아이템2" {
		t.Errorf("items[1].Name = %q, want 아이템2", items[1].Name)
	}
}

// TestDoPage는 제네릭 페이지네이션 응답 파싱을 검증합니다.
func TestDoPage(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"data": []map[string]string{
				{"id": "1", "name": "아이템1"},
			},
			"meta": map[string]int{
				"page":        1,
				"page_size":   10,
				"total":       50,
				"total_pages": 5,
			},
		})
	}))
	defer srv.Close()

	creds := newTestCreds(srv.URL)
	backend := newTestBackendClient(srv.URL, creds)
	tokenRefresher := auth.NewTokenRefresher(creds)
	client := apiclient.New(backend, creds, tokenRefresher)

	items, meta, err := apiclient.DoPage[testItem](client, context.Background(), http.MethodGet, "/api/v1/items", nil)
	if err != nil {
		t.Fatalf("DoPage() 오류: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	if meta == nil {
		t.Fatal("meta가 nil입니다")
	}
	if meta.Total != 50 {
		t.Errorf("meta.Total = %d, want 50", meta.Total)
	}
	if meta.TotalPages != 5 {
		t.Errorf("meta.TotalPages = %d, want 5", meta.TotalPages)
	}
}

// TestClient_ErrorResponse는 API 에러 응답 처리를 검증합니다.
func TestClient_ErrorResponse(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "NOT_FOUND",
			"message": "리소스를 찾을 수 없습니다",
		})
	}))
	defer srv.Close()

	creds := newTestCreds(srv.URL)
	backend := newTestBackendClient(srv.URL, creds)
	tokenRefresher := auth.NewTokenRefresher(creds)
	client := apiclient.New(backend, creds, tokenRefresher)

	_, err := client.Get(context.Background(), "/api/v1/missing")
	if err == nil {
		t.Fatal("에러를 기대했지만 성공했습니다")
	}
}

// TestClient_ErrorResponse_MessageFallback은 error 필드 없이 message만 있는 경우를 검증합니다.
func TestClient_ErrorResponse_MessageFallback(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		// error 필드 없이 message만 있는 경우
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "잘못된 요청입니다",
		})
	}))
	defer srv.Close()

	creds := newTestCreds(srv.URL)
	backend := newTestBackendClient(srv.URL, creds)
	tokenRefresher := auth.NewTokenRefresher(creds)
	client := apiclient.New(backend, creds, tokenRefresher)

	_, err := client.Get(context.Background(), "/api/v1/bad")
	if err == nil {
		t.Fatal("에러를 기대했지만 성공했습니다")
	}
}

// TestClient_ErrorResponse_StatusCodeFallback은 error, message 둘 다 없는 경우를 검증합니다.
func TestClient_ErrorResponse_StatusCodeFallback(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		// error, message 둘 다 없는 경우
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
		})
	}))
	defer srv.Close()

	creds := newTestCreds(srv.URL)
	backend := newTestBackendClient(srv.URL, creds)
	tokenRefresher := auth.NewTokenRefresher(creds)
	client := apiclient.New(backend, creds, tokenRefresher)

	_, err := client.Get(context.Background(), "/api/v1/invalid")
	if err == nil {
		t.Fatal("에러를 기대했지만 성공했습니다")
	}
}

// TestClient_ServerError는 500 에러 응답 처리를 검증합니다.
func TestClient_ServerError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Internal Server Error"))
	}))
	defer srv.Close()

	creds := newTestCreds(srv.URL)
	backend := newTestBackendClient(srv.URL, creds)
	tokenRefresher := auth.NewTokenRefresher(creds)
	client := apiclient.New(backend, creds, tokenRefresher)

	_, err := client.Get(context.Background(), "/api/v1/broken")
	if err == nil {
		t.Fatal("에러를 기대했지만 성공했습니다")
	}
}

// TestClient_InvalidJSONResponse는 파싱 불가능한 JSON 응답 처리를 검증합니다.
func TestClient_InvalidJSONResponse(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("this is not json"))
	}))
	defer srv.Close()

	creds := newTestCreds(srv.URL)
	backend := newTestBackendClient(srv.URL, creds)
	tokenRefresher := auth.NewTokenRefresher(creds)
	client := apiclient.New(backend, creds, tokenRefresher)

	_, err := client.Get(context.Background(), "/api/v1/bad-json")
	if err == nil {
		t.Fatal("에러를 기대했지만 성공했습니다")
	}
}

// TestDoList_HTTPError는 DoList에서 HTTP 에러 발생 시 에러를 반환하는지 검증합니다.
func TestDoList_HTTPError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "UNAUTHORIZED",
		})
	}))
	defer srv.Close()

	creds := newTestCreds(srv.URL)
	backend := newTestBackendClient(srv.URL, creds)
	tokenRefresher := auth.NewTokenRefresher(creds)
	client := apiclient.New(backend, creds, tokenRefresher)

	_, err := apiclient.DoList[testItem](client, context.Background(), http.MethodGet, "/api/v1/items", nil)
	if err == nil {
		t.Fatal("DoList()에서 에러를 기대했지만 성공했습니다")
	}
}

// TestDoList_InvalidDataJSON은 DoList에서 data 필드가 배열이 아닌 경우를 검증합니다.
func TestDoList_InvalidDataJSON(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// data가 배열이 아닌 객체인 경우
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"data":    map[string]string{"not": "array"},
		})
	}))
	defer srv.Close()

	creds := newTestCreds(srv.URL)
	backend := newTestBackendClient(srv.URL, creds)
	tokenRefresher := auth.NewTokenRefresher(creds)
	client := apiclient.New(backend, creds, tokenRefresher)

	_, err := apiclient.DoList[testItem](client, context.Background(), http.MethodGet, "/api/v1/items", nil)
	if err == nil {
		t.Fatal("DoList()에서 에러를 기대했지만 성공했습니다")
	}
}

// TestDoPage_HTTPError는 DoPage에서 HTTP 에러 발생 시 에러를 반환하는지 검증합니다.
func TestDoPage_HTTPError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "FORBIDDEN",
		})
	}))
	defer srv.Close()

	creds := newTestCreds(srv.URL)
	backend := newTestBackendClient(srv.URL, creds)
	tokenRefresher := auth.NewTokenRefresher(creds)
	client := apiclient.New(backend, creds, tokenRefresher)

	_, _, err := apiclient.DoPage[testItem](client, context.Background(), http.MethodGet, "/api/v1/items", nil)
	if err == nil {
		t.Fatal("DoPage()에서 에러를 기대했지만 성공했습니다")
	}
}

// TestDoPage_InvalidDataJSON은 DoPage에서 data 필드가 배열이 아닌 경우를 검증합니다.
func TestDoPage_InvalidDataJSON(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// data가 배열이 아닌 객체인 경우
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"data":    "not-an-array",
		})
	}))
	defer srv.Close()

	creds := newTestCreds(srv.URL)
	backend := newTestBackendClient(srv.URL, creds)
	tokenRefresher := auth.NewTokenRefresher(creds)
	client := apiclient.New(backend, creds, tokenRefresher)

	_, _, err := apiclient.DoPage[testItem](client, context.Background(), http.MethodGet, "/api/v1/items", nil)
	if err == nil {
		t.Fatal("DoPage()에서 에러를 기대했지만 성공했습니다")
	}
}

// TestClient_ResolvePath_EmptyWorkspaceID는 workspaceID가 빈 문자열일 때의 경로 반환을 검증합니다.
func TestClient_ResolvePath_EmptyWorkspaceID(t *testing.T) {
	t.Parallel()

	// workspaceID가 빈 문자열인 Credentials 생성
	creds := &auth.Credentials{
		AccessToken:  "test-token",
		RefreshToken: "test-refresh",
		ExpiresAt:    time.Now().Add(1 * time.Hour),
		ServerURL:    "https://api.example.com",
		WorkspaceID:  "", // 빈 문자열
	}
	tokenRefresher := auth.NewTokenRefresher(creds)
	backend := mcpserver.NewBackendClient("https://api.example.com", tokenRefresher, 10*time.Second, zerolog.Nop())
	client := apiclient.New(backend, creds, tokenRefresher)

	// workspaceID가 빈 문자열이면 원본 경로를 그대로 반환해야 함
	input := "/api/v1/workspaces/:workspaceId/agents"
	got := client.ResolvePath(input)
	if got != input {
		t.Errorf("ResolvePath(%q) = %q, want %q (workspaceID 빈 문자열 시 원본 반환)", input, got, input)
	}
}

// TestNew_NilCreds는 nil Credentials로 Client를 생성했을 때를 검증합니다.
func TestNew_NilCreds(t *testing.T) {
	t.Parallel()

	tokenRefresher := auth.NewTokenRefresher(nil)
	backend := mcpserver.NewBackendClient("", tokenRefresher, 10*time.Second, zerolog.Nop())

	// nil creds에서도 패닉 없이 생성되어야 함
	client := apiclient.New(backend, nil, tokenRefresher)
	if client == nil {
		t.Fatal("New()가 nil을 반환했습니다")
	}

	// baseURL과 workspaceID는 빈 문자열이어야 함
	if got := client.BaseURL(); got != "" {
		t.Errorf("BaseURL() = %q, want empty string", got)
	}
	if got := client.WorkspaceID(); got != "" {
		t.Errorf("WorkspaceID() = %q, want empty string", got)
	}
}

// TestClient_DoRaw는 DoRaw 메서드가 HTTP 상태 코드와 응답 본문을 반환하는지 검증합니다.
func TestClient_DoRaw(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		method     string
		statusCode int
		respBody   map[string]interface{}
		body       interface{}
		headers    map[string]string
	}{
		{
			name:       "GET 요청 성공",
			method:     http.MethodGet,
			statusCode: http.StatusOK,
			respBody:   map[string]interface{}{"success": true},
		},
		{
			name:       "POST 요청 본문 전송",
			method:     http.MethodPost,
			statusCode: http.StatusCreated,
			respBody:   map[string]interface{}{"id": "new-1"},
			body:       map[string]string{"content": "hello"},
		},
		{
			name:       "추가 헤더 전송",
			method:     http.MethodGet,
			statusCode: http.StatusOK,
			respBody:   map[string]interface{}{"ok": true},
			headers:    map[string]string{"X-Custom": "test-val"},
		},
		{
			name:       "4xx 상태 코드도 반환",
			method:     http.MethodGet,
			statusCode: http.StatusNotFound,
			respBody:   map[string]interface{}{"error": "not found"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// 인증 헤더 확인
				if auth := r.Header.Get("Authorization"); auth == "" {
					t.Error("Authorization 헤더가 없습니다")
				}
				// 추가 헤더 확인
				for k, v := range tt.headers {
					if got := r.Header.Get(k); got != v {
						t.Errorf("헤더 %s = %q, want %q", k, got, v)
					}
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				json.NewEncoder(w).Encode(tt.respBody)
			}))
			defer srv.Close()

			creds := newTestCreds(srv.URL)
			tokenRefresher := auth.NewTokenRefresher(creds)
			backend := newTestBackendClient(srv.URL, creds)
			client := apiclient.New(backend, creds, tokenRefresher)

			statusCode, body, err := client.DoRaw(context.Background(), tt.method, "/test", tt.body, tt.headers)
			if err != nil {
				t.Fatalf("DoRaw() 오류: %v", err)
			}
			if statusCode != tt.statusCode {
				t.Errorf("statusCode = %d, want %d", statusCode, tt.statusCode)
			}
			if body == nil {
				t.Fatal("응답 본문이 nil입니다")
			}
		})
	}
}

// TestClient_DoRaw_Error는 DoRaw의 에러 경로를 검증합니다.
func TestClient_DoRaw_Error(t *testing.T) {
	t.Parallel()

	// 유효하지 않은 URL로 연결 실패 유도
	creds := newTestCreds("http://invalid-host-that-does-not-exist:99999")
	tokenRefresher := auth.NewTokenRefresher(creds)
	backend := newTestBackendClient(creds.ServerURL, creds)
	client := apiclient.New(backend, creds, tokenRefresher)

	_, _, err := client.DoRaw(context.Background(), http.MethodGet, "/test", nil, nil)
	if err == nil {
		t.Error("유효하지 않은 호스트에서 에러가 발생해야 합니다")
	}
}
