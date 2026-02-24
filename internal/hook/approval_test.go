package hook

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

// TestNewApprovalManager는 기본값과 지정된 타임아웃으로 생성을 검증합니다.
func TestNewApprovalManager(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		timeout         time.Duration
		expectedTimeout time.Duration
	}{
		{
			name:            "양수 타임아웃 설정",
			timeout:         30 * time.Second,
			expectedTimeout: 30 * time.Second,
		},
		{
			name:            "0 타임아웃은 기본 5분으로 설정됨",
			timeout:         0,
			expectedTimeout: 5 * time.Minute,
		},
		{
			name:            "음수 타임아웃은 기본 5분으로 설정됨",
			timeout:         -1 * time.Second,
			expectedTimeout: 5 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mgr := NewApprovalManager(tt.timeout, zerolog.Nop())
			if mgr == nil {
				t.Fatal("NewApprovalManager가 nil을 반환했습니다")
			}
			if mgr.timeout != tt.expectedTimeout {
				t.Errorf("timeout = %v, want %v", mgr.timeout, tt.expectedTimeout)
			}
			if mgr.PendingCount() != 0 {
				t.Errorf("초기 PendingCount = %d, want 0", mgr.PendingCount())
			}
		})
	}
}

// TestApprovalManager_RequestAndDeliver는 승인 요청 후 allow 결정 전달을 검증합니다.
func TestApprovalManager_RequestAndDeliver(t *testing.T) {
	t.Parallel()

	mgr := NewApprovalManager(5*time.Second, zerolog.Nop())
	approvalID := "request-allow-001"

	// 비동기로 결정 전달
	go func() {
		time.Sleep(50 * time.Millisecond)
		err := mgr.DeliverDecision(approvalID, ApprovalDecision{
			Allow:  true,
			Reason: "승인됨",
		})
		if err != nil {
			t.Errorf("DeliverDecision 에러: %v", err)
		}
	}()

	decision, err := mgr.RequestApproval(context.Background(), approvalID)
	if err != nil {
		t.Fatalf("RequestApproval 에러: %v", err)
	}
	if !decision.Allow {
		t.Error("Allow = false, want true")
	}
	if decision.Reason != "승인됨" {
		t.Errorf("Reason = %q, want %q", decision.Reason, "승인됨")
	}
}

// TestApprovalManager_RequestAndDeny는 승인 요청 후 deny 결정 전달을 검증합니다.
func TestApprovalManager_RequestAndDeny(t *testing.T) {
	t.Parallel()

	mgr := NewApprovalManager(5*time.Second, zerolog.Nop())
	approvalID := "request-deny-001"

	go func() {
		time.Sleep(50 * time.Millisecond)
		_ = mgr.DeliverDecision(approvalID, ApprovalDecision{
			Allow:  false,
			Reason: "위험한 명령",
		})
	}()

	decision, err := mgr.RequestApproval(context.Background(), approvalID)
	if err != nil {
		t.Fatalf("RequestApproval 에러: %v", err)
	}
	if decision.Allow {
		t.Error("Allow = true, want false")
	}
	if decision.Reason != "위험한 명령" {
		t.Errorf("Reason = %q, want %q", decision.Reason, "위험한 명령")
	}
}

// TestApprovalManager_Timeout은 타임아웃 시 ErrApprovalTimeout이 반환되는지 검증합니다.
func TestApprovalManager_Timeout(t *testing.T) {
	t.Parallel()

	mgr := NewApprovalManager(100*time.Millisecond, zerolog.Nop())

	start := time.Now()
	decision, err := mgr.RequestApproval(context.Background(), "timeout-001")
	elapsed := time.Since(start)

	if !errors.Is(err, ErrApprovalTimeout) {
		t.Fatalf("err = %v, want ErrApprovalTimeout", err)
	}
	if decision.Allow {
		t.Error("타임아웃 시 Allow = true, want false")
	}
	if elapsed < 80*time.Millisecond {
		t.Errorf("타임아웃 전에 반환되었습니다: %v", elapsed)
	}
}

// TestApprovalManager_ContextCancel은 컨텍스트 취소 시 동작을 검증합니다.
func TestApprovalManager_ContextCancel(t *testing.T) {
	t.Parallel()

	mgr := NewApprovalManager(5*time.Second, zerolog.Nop())
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	decision, err := mgr.RequestApproval(ctx, "cancel-001")
	if err == nil {
		t.Fatal("컨텍스트 취소 시 에러가 반환되어야 합니다")
	}
	if decision.Allow {
		t.Error("컨텍스트 취소 시 Allow = true, want false")
	}
}

// TestApprovalManager_DeliverNotFound는 존재하지 않는 ID에
// 결정 전달 시 ErrApprovalNotFound가 반환되는지 검증합니다.
func TestApprovalManager_DeliverNotFound(t *testing.T) {
	t.Parallel()

	mgr := NewApprovalManager(5*time.Second, zerolog.Nop())

	err := mgr.DeliverDecision("nonexistent-id", ApprovalDecision{
		Allow:  true,
		Reason: "test",
	})
	if !errors.Is(err, ErrApprovalNotFound) {
		t.Fatalf("err = %v, want ErrApprovalNotFound", err)
	}
}

// TestApprovalManager_CancelAll은 모든 대기 중인 승인이 취소되는지 검증합니다.
func TestApprovalManager_CancelAll(t *testing.T) {
	t.Parallel()

	mgr := NewApprovalManager(5*time.Second, zerolog.Nop())

	var wg sync.WaitGroup
	const numRequests = 3
	decisions := make([]ApprovalDecision, numRequests)
	errs := make([]error, numRequests)

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			id := "cancel-all-" + string(rune('a'+idx))
			decisions[idx], errs[idx] = mgr.RequestApproval(context.Background(), id)
		}(i)
	}

	// 채널이 등록될 때까지 대기
	time.Sleep(50 * time.Millisecond)

	mgr.CancelAll()
	wg.Wait()

	// 모든 요청이 deny로 종료되었는지 확인
	for i := 0; i < numRequests; i++ {
		if errs[i] != nil {
			t.Errorf("요청 %d 에러: %v", i, errs[i])
		}
		if decisions[i].Allow {
			t.Errorf("요청 %d: Allow = true, want false", i)
		}
	}
}

// TestApprovalManager_PendingCount는 카운트 추적이 올바른지 검증합니다.
func TestApprovalManager_PendingCount(t *testing.T) {
	t.Parallel()

	mgr := NewApprovalManager(5*time.Second, zerolog.Nop())

	if mgr.PendingCount() != 0 {
		t.Fatalf("초기 PendingCount = %d, want 0", mgr.PendingCount())
	}

	// 승인 요청을 시작하고 pending 카운트 증가 확인
	started := make(chan struct{})
	go func() {
		// RequestApproval 내부에서 채널 등록 후 count 증가됨
		// 바로 시그널을 보낼 수 없으므로, 짧은 시간 대기
		time.Sleep(30 * time.Millisecond)
		close(started)
		// 이 요청은 타임아웃으로 종료될 것
	}()

	go func() {
		mgr.RequestApproval(context.Background(), "pending-count-001")
	}()

	<-started
	if count := mgr.PendingCount(); count != 1 {
		t.Errorf("PendingCount = %d, want 1", count)
	}

	// 결정을 전달하여 요청 완료
	_ = mgr.DeliverDecision("pending-count-001", ApprovalDecision{Allow: true})
	time.Sleep(30 * time.Millisecond)

	if count := mgr.PendingCount(); count != 0 {
		t.Errorf("결정 전달 후 PendingCount = %d, want 0", count)
	}
}

// TestApprovalManager_ConcurrentRequests는 여러 고루틴에서
// 동시에 요청을 처리할 수 있는지 검증합니다.
func TestApprovalManager_ConcurrentRequests(t *testing.T) {
	t.Parallel()

	mgr := NewApprovalManager(5*time.Second, zerolog.Nop())

	const numRequests = 10
	var wg sync.WaitGroup
	decisions := make([]ApprovalDecision, numRequests)
	errs := make([]error, numRequests)

	// 동시 요청 시작
	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			id := "concurrent-" + string(rune('A'+idx))
			decisions[idx], errs[idx] = mgr.RequestApproval(context.Background(), id)
		}(i)
	}

	// 등록 대기
	time.Sleep(50 * time.Millisecond)

	// 각 요청에 대해 결정 전달
	for i := 0; i < numRequests; i++ {
		id := "concurrent-" + string(rune('A'+i))
		allow := i%2 == 0 // 짝수 인덱스만 allow
		err := mgr.DeliverDecision(id, ApprovalDecision{
			Allow:  allow,
			Reason: "concurrent test",
		})
		if err != nil {
			t.Errorf("DeliverDecision(%s) 에러: %v", id, err)
		}
	}

	wg.Wait()

	for i := 0; i < numRequests; i++ {
		if errs[i] != nil {
			t.Errorf("요청 %d 에러: %v", i, errs[i])
		}
		expectedAllow := i%2 == 0
		if decisions[i].Allow != expectedAllow {
			t.Errorf("요청 %d: Allow = %v, want %v", i, decisions[i].Allow, expectedAllow)
		}
	}
}
