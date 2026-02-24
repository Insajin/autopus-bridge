package hook

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

// newTestServer는 테스트용 HookServer를 생성하고 시작합니다.
// 반환된 정리 함수를 defer로 호출해야 합니다.
func newTestServer(t *testing.T, opts ...HookHandlerOption) (*HookServer, *ApprovalManager, string, func()) {
	t.Helper()

	mgr := NewApprovalManager(500*time.Millisecond, zerolog.Nop())
	handler := NewHookHandler(mgr, opts...)
	token := "test-session-token-12345"
	server := NewHookServer(handler, token)

	port, err := server.Start(context.Background())
	if err != nil {
		t.Fatalf("서버 시작 에러: %v", err)
	}

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)

	cleanup := func() {
		if stopErr := server.Stop(); stopErr != nil {
			t.Errorf("서버 중지 에러: %v", stopErr)
		}
	}

	return server, mgr, baseURL, cleanup
}

// TestHookServer_WithOptions는 WithPort, WithLogger 옵션이 적용되는지 검증합니다.
func TestHookServer_WithOptions(t *testing.T) {
	t.Parallel()

	mgr := NewApprovalManager(1*time.Second, zerolog.Nop())
	handler := NewHookHandler(mgr, WithHandlerLogger(zerolog.Nop()))
	logger := zerolog.Nop()
	server := NewHookServer(handler, "test-token", WithPort(0), WithLogger(logger))

	port, err := server.Start(context.Background())
	if err != nil {
		t.Fatalf("WithOptions Start 에러: %v", err)
	}
	defer func() { _ = server.Stop() }()

	if port <= 0 {
		t.Errorf("port = %d, 양수여야 합니다", port)
	}
}

// TestHookServer_StartAndStop는 서버 시작 및 중지를 검증합니다.
func TestHookServer_StartAndStop(t *testing.T) {
	t.Parallel()

	mgr := NewApprovalManager(1*time.Second, zerolog.Nop())
	handler := NewHookHandler(mgr)
	server := NewHookServer(handler, "test-token")

	port, err := server.Start(context.Background())
	if err != nil {
		t.Fatalf("Start 에러: %v", err)
	}

	if port <= 0 {
		t.Errorf("port = %d, 양수여야 합니다", port)
	}
	if server.Port() != port {
		t.Errorf("Port() = %d, Start 반환값 %d과 일치해야 합니다", server.Port(), port)
	}

	if err := server.Stop(); err != nil {
		t.Fatalf("Stop 에러: %v", err)
	}
}

// TestHookServer_StopWithoutStart는 시작하지 않은 서버 중지 시 에러가 없는지 검증합니다.
func TestHookServer_StopWithoutStart(t *testing.T) {
	t.Parallel()

	mgr := NewApprovalManager(1*time.Second, zerolog.Nop())
	handler := NewHookHandler(mgr)
	server := NewHookServer(handler, "test-token")

	if err := server.Stop(); err != nil {
		t.Fatalf("시작하지 않은 서버 Stop 에러: %v", err)
	}
}

// TestHookServer_HealthEndpoint는 /hooks/health가 200 + {"status":"ok"}를 반환하는지 검증합니다.
func TestHookServer_HealthEndpoint(t *testing.T) {
	t.Parallel()

	_, _, baseURL, cleanup := newTestServer(t)
	defer cleanup()

	resp, err := http.Get(baseURL + "/hooks/health")
	if err != nil {
		t.Fatalf("GET /hooks/health 에러: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	body, _ := io.ReadAll(resp.Body)
	bodyStr := strings.TrimSpace(string(body))
	if bodyStr != `{"status":"ok"}` {
		t.Errorf("body = %q, want %q", bodyStr, `{"status":"ok"}`)
	}
}

// TestHookServer_AuthMiddleware_ValidToken은 유효한 토큰으로 인증 통과를 검증합니다.
func TestHookServer_AuthMiddleware_ValidToken(t *testing.T) {
	t.Parallel()

	onApprovalCalled := make(chan HookRequest, 1)
	onApproval := func(_ context.Context, req HookRequest) {
		onApprovalCalled <- req
	}

	_, mgr, baseURL, cleanup := newTestServer(t, WithOnApproval(onApproval))
	defer cleanup()

	body := `{"session_id":"sess-auth","tool_name":"Read","tool_input":{}}`
	req, _ := http.NewRequest(http.MethodPost, baseURL+"/hooks/pre-tool-use", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Session-Token", "test-session-token-12345")

	// 결정을 비동기로 전달
	go func() {
		cbReq := <-onApprovalCalled
		_ = mgr.DeliverDecision(cbReq.ApprovalID, ApprovalDecision{Allow: true})
	}()

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST /hooks/pre-tool-use 에러: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

// TestHookServer_AuthMiddleware_InvalidToken은 잘못된 토큰이 401을 반환하는지 검증합니다.
func TestHookServer_AuthMiddleware_InvalidToken(t *testing.T) {
	t.Parallel()

	_, _, baseURL, cleanup := newTestServer(t)
	defer cleanup()

	body := `{"session_id":"sess","tool_name":"Read","tool_input":{}}`
	req, _ := http.NewRequest(http.MethodPost, baseURL+"/hooks/pre-tool-use", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Session-Token", "wrong-token")

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST 에러: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

// TestHookServer_AuthMiddleware_MissingToken은 토큰 미포함 시 401을 반환하는지 검증합니다.
func TestHookServer_AuthMiddleware_MissingToken(t *testing.T) {
	t.Parallel()

	_, _, baseURL, cleanup := newTestServer(t)
	defer cleanup()

	body := `{"session_id":"sess","tool_name":"Read","tool_input":{}}`
	req, _ := http.NewRequest(http.MethodPost, baseURL+"/hooks/pre-tool-use", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// X-Session-Token 헤더 없음

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST 에러: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

// TestHookServer_PreToolUse_Endpoint는 POST /hooks/pre-tool-use가
// 유효한 토큰으로 접근 가능한지 검증합니다.
func TestHookServer_PreToolUse_Endpoint(t *testing.T) {
	t.Parallel()

	onApprovalCh := make(chan HookRequest, 1)
	onApproval := func(_ context.Context, req HookRequest) {
		onApprovalCh <- req
	}

	_, mgr, baseURL, cleanup := newTestServer(t, WithOnApproval(onApproval))
	defer cleanup()

	body := `{"session_id":"sess-pre","tool_name":"Bash","tool_input":{"command":"echo hello"}}`
	req, _ := http.NewRequest(http.MethodPost, baseURL+"/hooks/pre-tool-use", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Session-Token", "test-session-token-12345")

	go func() {
		cbReq := <-onApprovalCh
		_ = mgr.DeliverDecision(cbReq.ApprovalID, ApprovalDecision{
			Allow:  true,
			Reason: "echo는 안전합니다",
		})
	}()

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST 에러: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var hookResp HookResponse
	if err := json.NewDecoder(resp.Body).Decode(&hookResp); err != nil {
		t.Fatalf("응답 JSON 파싱 에러: %v", err)
	}

	if hookResp.Decision != "allow" {
		t.Errorf("Decision = %q, want %q", hookResp.Decision, "allow")
	}
}

// TestHookServer_PostToolUse_Endpoint는 POST /hooks/post-tool-use가 접근 가능한지 검증합니다.
func TestHookServer_PostToolUse_Endpoint(t *testing.T) {
	t.Parallel()

	_, _, baseURL, cleanup := newTestServer(t)
	defer cleanup()

	body := `{"session_id":"sess-post","tool_name":"Bash","result":"exit 0"}`
	req, _ := http.NewRequest(http.MethodPost, baseURL+"/hooks/post-tool-use", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Session-Token", "test-session-token-12345")

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST 에러: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}
