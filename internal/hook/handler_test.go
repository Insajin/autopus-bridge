package hook

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

// newTestHandler는 테스트용 HookHandler와 ApprovalManager를 생성합니다.
func newTestHandler(t *testing.T, opts ...HookHandlerOption) (*HookHandler, *ApprovalManager) {
	t.Helper()
	mgr := NewApprovalManager(500*time.Millisecond, zerolog.Nop())
	handler := NewHookHandler(mgr, opts...)
	return handler, mgr
}

// TestHandlePreToolUse_ValidRequest는 올바른 JSON 요청의 승인 흐름을 검증합니다.
func TestHandlePreToolUse_ValidRequest(t *testing.T) {
	t.Parallel()

	approvalCh := make(chan HookRequest, 1)
	onApproval := func(_ context.Context, req HookRequest) {
		approvalCh <- req
	}

	handler, mgr := newTestHandler(t, WithOnApproval(onApproval))

	body := `{"session_id":"sess-001","tool_name":"Bash","tool_input":{"command":"ls"}}`
	req := httptest.NewRequest(http.MethodPost, "/hooks/pre-tool-use", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	// OnApproval 콜백에서 approval_id를 채널로 수신 후 결정 전달
	go func() {
		cbReq := <-approvalCh
		_ = mgr.DeliverDecision(cbReq.ApprovalID, ApprovalDecision{
			Allow:  true,
			Reason: "테스트 승인",
		})
	}()

	handler.HandlePreToolUse(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp HookResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("응답 JSON 파싱 에러: %v", err)
	}

	if resp.Decision != "allow" {
		t.Errorf("Decision = %q, want %q", resp.Decision, "allow")
	}
	if resp.ID == "" {
		t.Error("응답에 approval_id가 비어있습니다")
	}
}

// TestHandlePreToolUse_InvalidJSON은 잘못된 JSON 바디에 대해 400 + deny를 반환하는지 검증합니다.
func TestHandlePreToolUse_InvalidJSON(t *testing.T) {
	t.Parallel()

	handler, _ := newTestHandler(t)

	body := `{invalid json`
	req := httptest.NewRequest(http.MethodPost, "/hooks/pre-tool-use", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.HandlePreToolUse(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	var resp HookResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("응답 JSON 파싱 에러: %v", err)
	}

	if resp.Decision != "deny" {
		t.Errorf("Decision = %q, want %q", resp.Decision, "deny")
	}
}

// TestHandlePreToolUse_EmptyBody는 빈 바디에 대해 400 + deny를 반환하는지 검증합니다.
func TestHandlePreToolUse_EmptyBody(t *testing.T) {
	t.Parallel()

	handler, _ := newTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/hooks/pre-tool-use", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.HandlePreToolUse(rec, req)

	// 빈 문자열도 JSON 파싱 에러이므로 400
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	var resp HookResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("응답 JSON 파싱 에러: %v", err)
	}

	if resp.Decision != "deny" {
		t.Errorf("Decision = %q, want %q", resp.Decision, "deny")
	}
}

// TestHandlePreToolUse_WithOnApproval은 OnApproval 콜백이 호출되는지 검증합니다.
func TestHandlePreToolUse_WithOnApproval(t *testing.T) {
	t.Parallel()

	callbackCalled := make(chan HookRequest, 1)
	onApproval := func(_ context.Context, req HookRequest) {
		callbackCalled <- req
	}

	handler, mgr := newTestHandler(t, WithOnApproval(onApproval))

	body := `{"session_id":"sess-cb","tool_name":"Write","tool_input":{"path":"/tmp/test"}}`
	req := httptest.NewRequest(http.MethodPost, "/hooks/pre-tool-use", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	// 콜백에서 캡처된 approval_id로 결정 전달
	go func() {
		cbReq := <-callbackCalled
		if cbReq.ToolName != "Write" {
			t.Errorf("콜백의 ToolName = %q, want %q", cbReq.ToolName, "Write")
		}
		if cbReq.ApprovalID == "" {
			t.Error("콜백의 ApprovalID가 비어있습니다")
		}
		_ = mgr.DeliverDecision(cbReq.ApprovalID, ApprovalDecision{Allow: true})
	}()

	handler.HandlePreToolUse(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

// TestHandlePreToolUse_ApprovalAllow는 allow 결정이 올바르게 응답되는지 검증합니다.
func TestHandlePreToolUse_ApprovalAllow(t *testing.T) {
	t.Parallel()

	approvalCh := make(chan HookRequest, 1)
	onApproval := func(_ context.Context, req HookRequest) {
		approvalCh <- req
	}

	handler, mgr := newTestHandler(t, WithOnApproval(onApproval))

	body := `{"session_id":"sess-allow","tool_name":"Read","tool_input":{}}`
	req := httptest.NewRequest(http.MethodPost, "/hooks/pre-tool-use", strings.NewReader(body))
	rec := httptest.NewRecorder()

	go func() {
		cbReq := <-approvalCh
		_ = mgr.DeliverDecision(cbReq.ApprovalID, ApprovalDecision{
			Allow:  true,
			Reason: "안전한 도구",
		})
	}()

	handler.HandlePreToolUse(rec, req)

	var resp HookResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("응답 JSON 파싱 에러: %v", err)
	}

	if resp.Decision != "allow" {
		t.Errorf("Decision = %q, want %q", resp.Decision, "allow")
	}
}

// TestHandlePreToolUse_ApprovalDeny는 deny 결정이 올바르게 응답되는지 검증합니다.
func TestHandlePreToolUse_ApprovalDeny(t *testing.T) {
	t.Parallel()

	approvalCh := make(chan HookRequest, 1)
	onApproval := func(_ context.Context, req HookRequest) {
		approvalCh <- req
	}

	handler, mgr := newTestHandler(t, WithOnApproval(onApproval))

	body := `{"session_id":"sess-deny","tool_name":"Bash","tool_input":{"command":"rm -rf /"}}`
	req := httptest.NewRequest(http.MethodPost, "/hooks/pre-tool-use", strings.NewReader(body))
	rec := httptest.NewRecorder()

	go func() {
		cbReq := <-approvalCh
		_ = mgr.DeliverDecision(cbReq.ApprovalID, ApprovalDecision{
			Allow:  false,
			Reason: "위험한 명령어 차단",
		})
	}()

	handler.HandlePreToolUse(rec, req)

	var resp HookResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("응답 JSON 파싱 에러: %v", err)
	}

	if resp.Decision != "deny" {
		t.Errorf("Decision = %q, want %q", resp.Decision, "deny")
	}
	if resp.Reason != "위험한 명령어 차단" {
		t.Errorf("Reason = %q, want %q", resp.Reason, "위험한 명령어 차단")
	}
}

// TestHandlePostToolUse는 PostToolUse 핸들러가 항상 200을 반환하는지 검증합니다.
func TestHandlePostToolUse(t *testing.T) {
	t.Parallel()

	handler, _ := newTestHandler(t)

	tests := []struct {
		name string
		body string
	}{
		{
			name: "유효한 JSON 바디",
			body: `{"session_id":"sess-001","tool_name":"Bash","result":"success"}`,
		},
		{
			name: "빈 바디",
			body: "",
		},
		{
			name: "잘못된 JSON",
			body: `{not valid}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var bodyReader *bytes.Reader
			if tt.body != "" {
				bodyReader = bytes.NewReader([]byte(tt.body))
			} else {
				bodyReader = bytes.NewReader(nil)
			}

			req := httptest.NewRequest(http.MethodPost, "/hooks/post-tool-use", bodyReader)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			handler.HandlePostToolUse(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
			}
		})
	}
}
