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

	"github.com/insajin/autopus-bridge/internal/apiclient"
	"github.com/insajin/autopus-bridge/internal/auth"
	"github.com/insajin/autopus-bridge/internal/mcpserver"
	"github.com/rs/zerolog"
)

// makeWorkspaceTestClient는 테스트용 apiclient.Client를 생성합니다.
func makeWorkspaceTestClient(serverURL, workspaceID string) *apiclient.Client {
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

// makeAPIResponse는 표준 APIResponse JSON 바이트를 생성합니다.
func makeAPIResponse(data interface{}) []byte {
	payload, _ := json.Marshal(data)
	resp := map[string]interface{}{
		"success": true,
		"data":    json.RawMessage(payload),
	}
	b, _ := json.Marshal(resp)
	return b
}

func TestRunWorkspaceList(t *testing.T) {
	// 테스트 워크스페이스 목록
	workspaces := []Workspace{
		{ID: "ws-1", Name: "Alpha", Slug: "alpha"},
		{ID: "ws-2", Name: "Beta", Slug: "beta"},
	}

	// Mock 서버: GET /api/v1/workspaces
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(makeAPIResponse(workspaces))
	}))
	defer srv.Close()

	client := makeWorkspaceTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runWorkspaceList(client, &buf, false)
	if err != nil {
		t.Fatalf("runWorkspaceList 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Alpha") {
		t.Errorf("출력에 'Alpha'가 없습니다: %s", out)
	}
	if !strings.Contains(out, "Beta") {
		t.Errorf("출력에 'Beta'가 없습니다: %s", out)
	}
}

func TestRunWorkspaceListJSON(t *testing.T) {
	workspaces := []Workspace{
		{ID: "ws-1", Name: "Alpha", Slug: "alpha"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(makeAPIResponse(workspaces))
	}))
	defer srv.Close()

	client := makeWorkspaceTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runWorkspaceList(client, &buf, true)
	if err != nil {
		t.Fatalf("runWorkspaceList JSON 오류: %v", err)
	}

	// JSON 출력이어야 합니다
	out := buf.String()
	var parsed []Workspace
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, out)
	}
	if len(parsed) != 1 || parsed[0].ID != "ws-1" {
		t.Errorf("예상치 않은 결과: %v", parsed)
	}
}

func TestRunWorkspaceShow(t *testing.T) {
	ws := Workspace{ID: "ws-1", Name: "Alpha", Slug: "alpha"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(makeAPIResponse(ws))
	}))
	defer srv.Close()

	client := makeWorkspaceTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runWorkspaceShow(client, &buf, "ws-1", false)
	if err != nil {
		t.Fatalf("runWorkspaceShow 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Alpha") {
		t.Errorf("출력에 'Alpha'가 없습니다: %s", out)
	}
}

func TestRunWorkspaceShowUsesCurrentWorkspace(t *testing.T) {
	ws := Workspace{ID: "ws-current", Name: "Current WS", Slug: "current"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-current" {
			http.Error(w, "not found: "+r.URL.Path, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(makeAPIResponse(ws))
	}))
	defer srv.Close()

	// workspaceID를 ws-current로 설정 (인자 없이 현재 workspace 사용)
	client := makeWorkspaceTestClient(srv.URL, "ws-current")
	var buf bytes.Buffer

	// 빈 workspaceID 전달 → 현재 workspace 사용
	err := runWorkspaceShow(client, &buf, "", false)
	if err != nil {
		t.Fatalf("runWorkspaceShow 현재 workspace 오류: %v", err)
	}
}

func TestRunWorkspaceMembers(t *testing.T) {
	members := []WorkspaceMember{
		{ID: "u1", Name: "Alice", Email: "alice@example.com", Role: "admin"},
		{ID: "u2", Name: "Bob", Email: "bob@example.com", Role: "member"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/members" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(makeAPIResponse(members))
	}))
	defer srv.Close()

	client := makeWorkspaceTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runWorkspaceMembers(client, &buf, "ws-1", false)
	if err != nil {
		t.Fatalf("runWorkspaceMembers 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Alice") {
		t.Errorf("출력에 'Alice'가 없습니다: %s", out)
	}
}

func TestRunWorkspaceCreate(t *testing.T) {
	ws := Workspace{ID: "ws-new", Name: "New Workspace", Slug: "new-workspace"}

	var capturedBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces" || r.Method != http.MethodPost {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		json.NewDecoder(r.Body).Decode(&capturedBody)
		w.Header().Set("Content-Type", "application/json")
		w.Write(makeAPIResponse(ws))
	}))
	defer srv.Close()

	client := makeWorkspaceTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runWorkspaceCreate(client, &buf, "New Workspace", false)
	if err != nil {
		t.Fatalf("runWorkspaceCreate 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "New Workspace") {
		t.Errorf("출력에 'New Workspace'가 없습니다: %s", out)
	}
	if capturedBody["name"] != "New Workspace" {
		t.Errorf("요청 본문 name = %v, want 'New Workspace'", capturedBody["name"])
	}
}

func TestRunWorkspaceUpdate(t *testing.T) {
	ws := Workspace{ID: "ws-1", Name: "Updated WS", Slug: "ws-1"}

	var capturedBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1" || r.Method != http.MethodPatch {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		json.NewDecoder(r.Body).Decode(&capturedBody)
		w.Header().Set("Content-Type", "application/json")
		w.Write(makeAPIResponse(ws))
	}))
	defer srv.Close()

	client := makeWorkspaceTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runWorkspaceUpdate(client, &buf, "Updated WS", "", false)
	if err != nil {
		t.Fatalf("runWorkspaceUpdate 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Updated WS") {
		t.Errorf("출력에 'Updated WS'가 없습니다: %s", out)
	}
	if capturedBody["name"] != "Updated WS" {
		t.Errorf("요청 본문 name = %v, want 'Updated WS'", capturedBody["name"])
	}
}

func TestRunWorkspaceDelete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1" || r.Method != http.MethodDelete {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(makeAPIResponse(map[string]interface{}{}))
	}))
	defer srv.Close()

	client := makeWorkspaceTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runWorkspaceDelete(client, &buf)
	if err != nil {
		t.Fatalf("runWorkspaceDelete 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "워크스페이스 삭제 완료") {
		t.Errorf("출력에 '워크스페이스 삭제 완료'가 없습니다: %s", out)
	}
}

func TestRunWorkspaceMission(t *testing.T) {
	ws := Workspace{ID: "ws-1", Name: "Alpha", Slug: "alpha"}

	var capturedBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/mission" || r.Method != http.MethodPatch {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		json.NewDecoder(r.Body).Decode(&capturedBody)
		w.Header().Set("Content-Type", "application/json")
		w.Write(makeAPIResponse(ws))
	}))
	defer srv.Close()

	client := makeWorkspaceTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runWorkspaceMission(client, &buf, "우리의 미션", "우리의 비전", false)
	if err != nil {
		t.Fatalf("runWorkspaceMission 오류: %v", err)
	}

	if capturedBody["mission"] != "우리의 미션" {
		t.Errorf("요청 본문 mission = %v, want '우리의 미션'", capturedBody["mission"])
	}
	if capturedBody["vision"] != "우리의 비전" {
		t.Errorf("요청 본문 vision = %v, want '우리의 비전'", capturedBody["vision"])
	}
}

func TestRunWorkspaceAddMember(t *testing.T) {
	member := WorkspaceMember{ID: "u-new", Name: "Charlie", Email: "charlie@example.com", Role: "member"}

	var capturedBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/members" || r.Method != http.MethodPost {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		json.NewDecoder(r.Body).Decode(&capturedBody)
		w.Header().Set("Content-Type", "application/json")
		w.Write(makeAPIResponse(member))
	}))
	defer srv.Close()

	client := makeWorkspaceTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runWorkspaceAddMember(client, &buf, "u-new", "member")
	if err != nil {
		t.Fatalf("runWorkspaceAddMember 오류: %v", err)
	}

	if capturedBody["user_id"] != "u-new" {
		t.Errorf("요청 본문 user_id = %v, want 'u-new'", capturedBody["user_id"])
	}
	if capturedBody["role"] != "member" {
		t.Errorf("요청 본문 role = %v, want 'member'", capturedBody["role"])
	}
}

func TestRunWorkspaceRemoveMember(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/members/u-1" || r.Method != http.MethodDelete {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(makeAPIResponse(map[string]interface{}{}))
	}))
	defer srv.Close()

	client := makeWorkspaceTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runWorkspaceRemoveMember(client, &buf, "u-1")
	if err != nil {
		t.Fatalf("runWorkspaceRemoveMember 오류: %v", err)
	}
}

func TestRunWorkspaceUpdateRole(t *testing.T) {
	member := WorkspaceMember{ID: "u-1", Name: "Alice", Email: "alice@example.com", Role: "admin"}

	var capturedBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/members/u-1" || r.Method != http.MethodPatch {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		json.NewDecoder(r.Body).Decode(&capturedBody)
		w.Header().Set("Content-Type", "application/json")
		w.Write(makeAPIResponse(member))
	}))
	defer srv.Close()

	client := makeWorkspaceTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runWorkspaceUpdateRole(client, &buf, "u-1", "admin")
	if err != nil {
		t.Fatalf("runWorkspaceUpdateRole 오류: %v", err)
	}

	if capturedBody["role"] != "admin" {
		t.Errorf("요청 본문 role = %v, want 'admin'", capturedBody["role"])
	}
}

func TestRunWorkspaceSwitch(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	workspaces := []Workspace{
		{ID: "ws-1", Name: "Alpha", Slug: "alpha"},
		{ID: "ws-2", Name: "Beta", Slug: "beta"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(makeAPIResponse(workspaces))
	}))
	defer srv.Close()

	creds := &auth.Credentials{
		AccessToken: "test-token",
		ServerURL:   srv.URL,
		WorkspaceID: "ws-1",
		ExpiresAt:   time.Now().Add(1 * time.Hour),
	}
	tr := auth.NewTokenRefresher(creds)
	backend := mcpserver.NewBackendClient(srv.URL, tr, 0, zerolog.Nop())
	client := apiclient.New(backend, creds, tr)

	var buf bytes.Buffer
	// 사용자 입력을 "2"로 시뮬레이션 (Beta 선택)
	input := strings.NewReader("2\n")

	err := runWorkspaceSwitch(context.Background(), client, creds, input, &buf)
	if err != nil {
		t.Fatalf("runWorkspaceSwitch 오류: %v", err)
	}

	// creds.WorkspaceID가 변경되었어야 합니다
	if creds.WorkspaceID != "ws-2" {
		t.Errorf("WorkspaceID = %q, want %q", creds.WorkspaceID, "ws-2")
	}
}
