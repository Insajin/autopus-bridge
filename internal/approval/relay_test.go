package approval

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// TestNewApprovalRouter는 다양한 정책과 타임아웃으로 ApprovalRouter 생성을 검증합니다.
func TestNewApprovalRouter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		policy  ApprovalPolicy
		timeout time.Duration
	}{
		{
			name:    "auto-execute 정책으로 생성",
			policy:  ApprovalPolicyAutoExecute,
			timeout: 30 * time.Second,
		},
		{
			name:    "auto-approve 정책으로 생성",
			policy:  ApprovalPolicyAutoApprove,
			timeout: 1 * time.Minute,
		},
		{
			name:    "human-approve 정책으로 생성",
			policy:  ApprovalPolicyHumanApprove,
			timeout: 5 * time.Minute,
		},
		{
			name:    "agent-approve 정책으로 생성",
			policy:  ApprovalPolicyAgentApprove,
			timeout: 2 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			router := NewApprovalRouter(tt.policy, tt.timeout)
			if router == nil {
				t.Fatal("NewApprovalRouter가 nil을 반환했습니다")
			}
			if router.policy != tt.policy {
				t.Errorf("policy = %q, want %q", router.policy, tt.policy)
			}
			if router.timeout != tt.timeout {
				t.Errorf("timeout = %v, want %v", router.timeout, tt.timeout)
			}
		})
	}
}

// TestApprovalRouter_AutoExecute는 auto-execute 정책이 즉시 allow를 반환하는지 검증합니다.
func TestApprovalRouter_AutoExecute(t *testing.T) {
	t.Parallel()

	router := NewApprovalRouter(ApprovalPolicyAutoExecute, 30*time.Second)
	req := ToolApprovalRequest{
		ApprovalID: "test-001",
		ToolName:   "Bash",
	}

	decision, err := router.HandleApproval(context.Background(), req)
	if err != nil {
		t.Fatalf("HandleApproval 에러: %v", err)
	}
	if decision.Decision != "allow" {
		t.Errorf("Decision = %q, want %q", decision.Decision, "allow")
	}
	if decision.DecidedBy != "policy" {
		t.Errorf("DecidedBy = %q, want %q", decision.DecidedBy, "policy")
	}
	if decision.DecidedAt.IsZero() {
		t.Error("DecidedAt가 zero 값입니다")
	}
}

// TestApprovalRouter_AutoApprove는 auto-approve 정책이 즉시 allow를 반환하는지 검증합니다.
func TestApprovalRouter_AutoApprove(t *testing.T) {
	t.Parallel()

	router := NewApprovalRouter(ApprovalPolicyAutoApprove, 30*time.Second)
	req := ToolApprovalRequest{
		ApprovalID: "test-002",
		ToolName:   "Write",
	}

	decision, err := router.HandleApproval(context.Background(), req)
	if err != nil {
		t.Fatalf("HandleApproval 에러: %v", err)
	}
	if decision.Decision != "allow" {
		t.Errorf("Decision = %q, want %q", decision.Decision, "allow")
	}
	if decision.DecidedBy != "policy" {
		t.Errorf("DecidedBy = %q, want %q", decision.DecidedBy, "policy")
	}
	if decision.Reason == "" {
		t.Error("Reason이 비어있습니다")
	}
}

// TestApprovalRouter_HumanApprove_DeliverAllow는 human-approve 정책에서
// allow 결정을 전달하면 올바르게 반환되는지 검증합니다.
func TestApprovalRouter_HumanApprove_DeliverAllow(t *testing.T) {
	t.Parallel()

	router := NewApprovalRouter(ApprovalPolicyHumanApprove, 5*time.Second)
	approvalID := "human-allow-001"
	req := ToolApprovalRequest{
		ApprovalID: approvalID,
		ToolName:   "Bash",
	}

	// 결정을 비동기로 전달
	go func() {
		time.Sleep(50 * time.Millisecond)
		err := router.DeliverDecision(approvalID, ToolApprovalDecision{
			Decision:  "allow",
			Reason:    "사용자가 승인함",
			DecidedBy: "human",
			DecidedAt: time.Now(),
		})
		if err != nil {
			t.Errorf("DeliverDecision 에러: %v", err)
		}
	}()

	decision, err := router.HandleApproval(context.Background(), req)
	if err != nil {
		t.Fatalf("HandleApproval 에러: %v", err)
	}
	if decision.Decision != "allow" {
		t.Errorf("Decision = %q, want %q", decision.Decision, "allow")
	}
	if decision.DecidedBy != "human" {
		t.Errorf("DecidedBy = %q, want %q", decision.DecidedBy, "human")
	}
}

// TestApprovalRouter_HumanApprove_DeliverDeny는 human-approve 정책에서
// deny 결정을 전달하면 올바르게 반환되는지 검증합니다.
func TestApprovalRouter_HumanApprove_DeliverDeny(t *testing.T) {
	t.Parallel()

	router := NewApprovalRouter(ApprovalPolicyHumanApprove, 5*time.Second)
	approvalID := "human-deny-001"
	req := ToolApprovalRequest{
		ApprovalID: approvalID,
		ToolName:   "Bash",
	}

	go func() {
		time.Sleep(50 * time.Millisecond)
		_ = router.DeliverDecision(approvalID, ToolApprovalDecision{
			Decision:  "deny",
			Reason:    "위험한 명령어",
			DecidedBy: "human",
			DecidedAt: time.Now(),
		})
	}()

	decision, err := router.HandleApproval(context.Background(), req)
	if err != nil {
		t.Fatalf("HandleApproval 에러: %v", err)
	}
	if decision.Decision != "deny" {
		t.Errorf("Decision = %q, want %q", decision.Decision, "deny")
	}
	if decision.Reason != "위험한 명령어" {
		t.Errorf("Reason = %q, want %q", decision.Reason, "위험한 명령어")
	}
}

// TestApprovalRouter_Timeout은 타임아웃 시 deny가 반환되는지 검증합니다.
func TestApprovalRouter_Timeout(t *testing.T) {
	t.Parallel()

	router := NewApprovalRouter(ApprovalPolicyHumanApprove, 100*time.Millisecond)
	req := ToolApprovalRequest{
		ApprovalID: "timeout-001",
		ToolName:   "Bash",
	}

	start := time.Now()
	decision, err := router.HandleApproval(context.Background(), req)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("HandleApproval 에러: %v (타임아웃 시 에러가 nil이어야 함)", err)
	}
	if decision.Decision != "deny" {
		t.Errorf("Decision = %q, want %q", decision.Decision, "deny")
	}
	if decision.Reason != "approval timed out" {
		t.Errorf("Reason = %q, want %q", decision.Reason, "approval timed out")
	}
	// 타임아웃 시간 근처에서 완료되었는지 확인
	if elapsed < 80*time.Millisecond {
		t.Errorf("타임아웃 전에 반환되었습니다: %v", elapsed)
	}
}

// TestApprovalRouter_ContextCancel은 컨텍스트 취소 시 deny와 에러가 반환되는지 검증합니다.
func TestApprovalRouter_ContextCancel(t *testing.T) {
	t.Parallel()

	router := NewApprovalRouter(ApprovalPolicyHumanApprove, 5*time.Second)
	ctx, cancel := context.WithCancel(context.Background())

	req := ToolApprovalRequest{
		ApprovalID: "cancel-001",
		ToolName:   "Bash",
	}

	// 50ms 후 컨텍스트 취소
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	decision, err := router.HandleApproval(ctx, req)
	if err == nil {
		t.Fatal("컨텍스트 취소 시 에러가 반환되어야 합니다")
	}
	if decision.Decision != "deny" {
		t.Errorf("Decision = %q, want %q", decision.Decision, "deny")
	}
}

// TestApprovalRouter_CancelAll은 모든 대기 중인 승인이 거부되는지 검증합니다.
func TestApprovalRouter_CancelAll(t *testing.T) {
	t.Parallel()

	router := NewApprovalRouter(ApprovalPolicyHumanApprove, 5*time.Second)

	var wg sync.WaitGroup
	results := make([]ToolApprovalDecision, 3)
	errs := make([]error, 3)

	// 3개의 동시 승인 요청 시작
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			req := ToolApprovalRequest{
				ApprovalID: fmt.Sprintf("cancel-all-%d", idx),
				ToolName:   "Bash",
			}
			results[idx], errs[idx] = router.HandleApproval(context.Background(), req)
		}(i)
	}

	// 채널이 등록될 때까지 약간 대기
	time.Sleep(50 * time.Millisecond)

	// 모든 대기 중인 승인 취소
	router.CancelAll()

	wg.Wait()

	for i := 0; i < 3; i++ {
		if errs[i] != nil {
			t.Errorf("요청 %d 에러: %v", i, errs[i])
		}
		if results[i].Decision != "deny" {
			t.Errorf("요청 %d: Decision = %q, want %q", i, results[i].Decision, "deny")
		}
	}
}

// TestApprovalRouter_DeliverNotFound는 존재하지 않는 승인 ID에
// 결정을 전달하면 에러가 반환되는지 검증합니다.
func TestApprovalRouter_DeliverNotFound(t *testing.T) {
	t.Parallel()

	router := NewApprovalRouter(ApprovalPolicyHumanApprove, 5*time.Second)

	err := router.DeliverDecision("nonexistent-id", ToolApprovalDecision{
		Decision:  "allow",
		Reason:    "test",
		DecidedBy: "human",
		DecidedAt: time.Now(),
	})
	if err == nil {
		t.Fatal("존재하지 않는 ID에 대해 에러가 반환되어야 합니다")
	}
}

