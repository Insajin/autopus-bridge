package hook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

// TestIntegration_HookServer_ApprovalFlow는 전체 통합 흐름을 검증합니다:
// HTTP 서버 시작 -> POST /hooks/pre-tool-use -> ApprovalManager로 결정 전달 -> HTTP 응답 확인
func TestIntegration_HookServer_ApprovalFlow(t *testing.T) {
	t.Parallel()

	// 1. 컴포넌트 생성
	approvalMgr := NewApprovalManager(5*time.Second, zerolog.Nop())

	approvalCh := make(chan HookRequest, 1)
	onApproval := func(_ context.Context, req HookRequest) {
		approvalCh <- req
	}

	handler := NewHookHandler(approvalMgr, WithOnApproval(onApproval))
	token := "integration-test-token"
	server := NewHookServer(handler, token)

	// 2. 서버 시작
	port, err := server.Start(context.Background())
	if err != nil {
		t.Fatalf("서버 시작 에러: %v", err)
	}
	defer func() {
		if stopErr := server.Stop(); stopErr != nil {
			t.Errorf("서버 중지 에러: %v", stopErr)
		}
	}()

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)

	// 3. 결정을 비동기로 전달하는 고루틴
	go func() {
		// OnApproval 콜백에서 요청 수신 대기
		cbReq := <-approvalCh

		// approval_id가 존재하는지 확인
		if cbReq.ApprovalID == "" {
			t.Error("통합 테스트: approval_id가 비어있습니다")
			return
		}

		// allow 결정 전달
		if deliverErr := approvalMgr.DeliverDecision(cbReq.ApprovalID, ApprovalDecision{
			Allow:  true,
			Reason: "통합 테스트 승인",
		}); deliverErr != nil {
			t.Errorf("DeliverDecision 에러: %v", deliverErr)
		}
	}()

	// 4. HTTP 요청 전송
	hookBody := map[string]any{
		"session_id": "integration-sess-001",
		"tool_name":  "Bash",
		"tool_input": map[string]string{"command": "echo integration test"},
	}
	bodyBytes, _ := json.Marshal(hookBody)

	req, _ := http.NewRequest(
		http.MethodPost,
		baseURL+"/hooks/pre-tool-use",
		bytes.NewReader(bodyBytes),
	)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Session-Token", token)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("HTTP 요청 에러: %v", err)
	}
	defer resp.Body.Close()

	// 5. 응답 검증
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
	if hookResp.ID == "" {
		t.Error("응답에 approval_id가 비어있습니다")
	}
}

// TestIntegration_HookServer_DenyFlow는 거부 흐름의 통합 테스트입니다.
func TestIntegration_HookServer_DenyFlow(t *testing.T) {
	t.Parallel()

	approvalMgr := NewApprovalManager(5*time.Second, zerolog.Nop())

	approvalCh := make(chan HookRequest, 1)
	onApproval := func(_ context.Context, req HookRequest) {
		approvalCh <- req
	}

	handler := NewHookHandler(approvalMgr, WithOnApproval(onApproval))
	token := "deny-integration-token"
	server := NewHookServer(handler, token)

	port, err := server.Start(context.Background())
	if err != nil {
		t.Fatalf("서버 시작 에러: %v", err)
	}
	defer func() { _ = server.Stop() }()

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)

	go func() {
		cbReq := <-approvalCh
		_ = approvalMgr.DeliverDecision(cbReq.ApprovalID, ApprovalDecision{
			Allow:  false,
			Reason: "위험한 명령 거부",
		})
	}()

	hookBody := map[string]any{
		"session_id": "integration-deny-001",
		"tool_name":  "Bash",
		"tool_input": map[string]string{"command": "rm -rf /"},
	}
	bodyBytes, _ := json.Marshal(hookBody)

	req, _ := http.NewRequest(
		http.MethodPost,
		baseURL+"/hooks/pre-tool-use",
		bytes.NewReader(bodyBytes),
	)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Session-Token", token)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("HTTP 요청 에러: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var hookResp HookResponse
	if err := json.NewDecoder(resp.Body).Decode(&hookResp); err != nil {
		t.Fatalf("응답 JSON 파싱 에러: %v", err)
	}

	if hookResp.Decision != "deny" {
		t.Errorf("Decision = %q, want %q", hookResp.Decision, "deny")
	}
	if hookResp.Reason != "위험한 명령 거부" {
		t.Errorf("Reason = %q, want %q", hookResp.Reason, "위험한 명령 거부")
	}
}

// TestIntegration_HookServer_TimeoutFlow는 타임아웃 거부 흐름의 통합 테스트입니다.
func TestIntegration_HookServer_TimeoutFlow(t *testing.T) {
	t.Parallel()

	// 짧은 타임아웃 설정 (결정을 전달하지 않음)
	approvalMgr := NewApprovalManager(200*time.Millisecond, zerolog.Nop())
	handler := NewHookHandler(approvalMgr) // OnApproval 없음
	token := "timeout-integration-token"
	server := NewHookServer(handler, token)

	port, err := server.Start(context.Background())
	if err != nil {
		t.Fatalf("서버 시작 에러: %v", err)
	}
	defer func() { _ = server.Stop() }()

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)

	hookBody := map[string]any{
		"session_id": "integration-timeout-001",
		"tool_name":  "Bash",
		"tool_input": map[string]string{"command": "sleep 100"},
	}
	bodyBytes, _ := json.Marshal(hookBody)

	req, _ := http.NewRequest(
		http.MethodPost,
		baseURL+"/hooks/pre-tool-use",
		bytes.NewReader(bodyBytes),
	)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Session-Token", token)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("HTTP 요청 에러: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var hookResp HookResponse
	if err := json.NewDecoder(resp.Body).Decode(&hookResp); err != nil {
		t.Fatalf("응답 JSON 파싱 에러: %v", err)
	}

	if hookResp.Decision != "deny" {
		t.Errorf("Decision = %q, want %q (타임아웃으로 deny 예상)", hookResp.Decision, "deny")
	}
}

// TestIntegration_HookServer_HealthAndAuth는 health와 auth 엔드포인트의
// 연동 동작을 검증합니다.
func TestIntegration_HookServer_HealthAndAuth(t *testing.T) {
	t.Parallel()

	approvalMgr := NewApprovalManager(1*time.Second, zerolog.Nop())
	handler := NewHookHandler(approvalMgr)
	token := "multi-endpoint-token"
	server := NewHookServer(handler, token)

	port, err := server.Start(context.Background())
	if err != nil {
		t.Fatalf("서버 시작 에러: %v", err)
	}
	defer func() { _ = server.Stop() }()

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	client := &http.Client{Timeout: 2 * time.Second}

	// 1. Health는 인증 없이 접근 가능
	healthResp, err := client.Get(baseURL + "/hooks/health")
	if err != nil {
		t.Fatalf("health 에러: %v", err)
	}
	healthResp.Body.Close()
	if healthResp.StatusCode != http.StatusOK {
		t.Errorf("health status = %d, want %d", healthResp.StatusCode, http.StatusOK)
	}

	// 2. Pre-tool-use는 인증 없이 401
	preReq, _ := http.NewRequest(http.MethodPost, baseURL+"/hooks/pre-tool-use", nil)
	preResp, err := client.Do(preReq)
	if err != nil {
		t.Fatalf("pre-tool-use 에러: %v", err)
	}
	preResp.Body.Close()
	if preResp.StatusCode != http.StatusUnauthorized {
		t.Errorf("pre-tool-use (no auth) status = %d, want %d", preResp.StatusCode, http.StatusUnauthorized)
	}

	// 3. Post-tool-use는 인증 없이 401
	postReq, _ := http.NewRequest(http.MethodPost, baseURL+"/hooks/post-tool-use", nil)
	postResp, err := client.Do(postReq)
	if err != nil {
		t.Fatalf("post-tool-use 에러: %v", err)
	}
	postResp.Body.Close()
	if postResp.StatusCode != http.StatusUnauthorized {
		t.Errorf("post-tool-use (no auth) status = %d, want %d", postResp.StatusCode, http.StatusUnauthorized)
	}
}
