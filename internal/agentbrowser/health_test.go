package agentbrowser

import (
	"context"
	"os/exec"
	"sync"
	"testing"
	"time"
)

func TestNewHealthChecker(t *testing.T) {
	logger := noopLogger()
	executor := NewCommandExecutor(logger)
	hc := NewHealthChecker(logger, executor, nil)

	if hc == nil {
		t.Fatal("NewHealthChecker() returned nil")
	}
	if hc.IsHealthy() {
		t.Error("IsHealthy() = true; want false for initial state")
	}
	if hc.ConsecutiveFailures() != 0 {
		t.Errorf("ConsecutiveFailures() = %d; want 0", hc.ConsecutiveFailures())
	}
}

func TestHealthChecker_CheckHealth_Success(t *testing.T) {
	logger := noopLogger()
	executor := NewCommandExecutor(logger)

	// 성공적인 health check 모킹
	executor.commandFn = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "echo", "https://example.com")
	}

	hc := NewHealthChecker(logger, executor, nil)

	healthy := hc.CheckHealth(context.Background())
	if !healthy {
		t.Error("CheckHealth() = false; want true")
	}
	if !hc.IsHealthy() {
		t.Error("IsHealthy() = false; want true after successful check")
	}
	if hc.ConsecutiveFailures() != 0 {
		t.Errorf("ConsecutiveFailures() = %d; want 0", hc.ConsecutiveFailures())
	}
}

func TestHealthChecker_CheckHealth_Failure(t *testing.T) {
	logger := noopLogger()
	executor := NewCommandExecutor(logger)

	// 실패하는 health check 모킹
	executor.commandFn = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "false")
	}

	hc := NewHealthChecker(logger, executor, nil)

	healthy := hc.CheckHealth(context.Background())
	if healthy {
		t.Error("CheckHealth() = true; want false for failing command")
	}
	if hc.IsHealthy() {
		t.Error("IsHealthy() = true; want false after failed check")
	}
	if hc.ConsecutiveFailures() != 1 {
		t.Errorf("ConsecutiveFailures() = %d; want 1", hc.ConsecutiveFailures())
	}
}

func TestHealthChecker_CheckHealth_ConsecutiveFailures(t *testing.T) {
	logger := noopLogger()
	executor := NewCommandExecutor(logger)

	executor.commandFn = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "false")
	}

	hc := NewHealthChecker(logger, executor, nil)

	// 3번 연속 실패
	for i := 0; i < 3; i++ {
		hc.CheckHealth(context.Background())
	}

	if hc.ConsecutiveFailures() != 3 {
		t.Errorf("ConsecutiveFailures() = %d; want 3", hc.ConsecutiveFailures())
	}
}

func TestHealthChecker_CheckHealth_Recovery(t *testing.T) {
	logger := noopLogger()
	executor := NewCommandExecutor(logger)

	callCount := 0
	executor.commandFn = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		callCount++
		if callCount <= 2 {
			// 처음 2번은 실패
			return exec.CommandContext(ctx, "false")
		}
		// 3번째부터 성공
		return exec.CommandContext(ctx, "echo", "ok")
	}

	var mu sync.Mutex
	var recoveryState ManagerState
	hc := NewHealthChecker(logger, executor, func(state ManagerState, message string) {
		mu.Lock()
		defer mu.Unlock()
		recoveryState = state
	})

	// 2번 실패
	hc.CheckHealth(context.Background())
	hc.CheckHealth(context.Background())

	if hc.IsHealthy() {
		t.Error("IsHealthy() = true; want false after failures")
	}

	// 성공 (복구)
	healthy := hc.CheckHealth(context.Background())
	if !healthy {
		t.Error("CheckHealth() = false; want true for recovery")
	}
	if hc.ConsecutiveFailures() != 0 {
		t.Errorf("ConsecutiveFailures() = %d; want 0 after recovery", hc.ConsecutiveFailures())
	}

	// 복구 콜백이 호출되었는지 확인
	mu.Lock()
	defer mu.Unlock()
	if recoveryState != StateReady {
		t.Errorf("recovery state = %q; want %q", recoveryState, StateReady)
	}
}

func TestHealthChecker_CheckHealth_ErrorCallback_MaxFailures(t *testing.T) {
	logger := noopLogger()
	executor := NewCommandExecutor(logger)

	executor.commandFn = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "false")
	}

	var mu sync.Mutex
	var errorState ManagerState
	hc := NewHealthChecker(logger, executor, func(state ManagerState, message string) {
		mu.Lock()
		defer mu.Unlock()
		errorState = state
	})

	// maxRestartAttempts(3)번 실패하면 에러 콜백이 호출되어야 한다
	for i := 0; i < maxRestartAttempts; i++ {
		hc.CheckHealth(context.Background())
	}

	mu.Lock()
	defer mu.Unlock()
	if errorState != StateError {
		t.Errorf("error state = %q; want %q after max failures", errorState, StateError)
	}
}

func TestHealthChecker_CheckHealth_NoCallbackBeforeMaxFailures(t *testing.T) {
	logger := noopLogger()
	executor := NewCommandExecutor(logger)

	executor.commandFn = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "false")
	}

	callbackCalled := false
	hc := NewHealthChecker(logger, executor, func(state ManagerState, message string) {
		if state == StateError {
			callbackCalled = true
		}
	})

	// maxRestartAttempts - 1 번 실패 (에러 콜백이 호출되지 않아야 한다)
	for i := 0; i < maxRestartAttempts-1; i++ {
		hc.CheckHealth(context.Background())
	}

	if callbackCalled {
		t.Error("error callback was called before max failures; want not called")
	}
}

func TestHealthChecker_NeedsRestart(t *testing.T) {
	logger := noopLogger()
	executor := NewCommandExecutor(logger)

	executor.commandFn = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "false")
	}

	hc := NewHealthChecker(logger, executor, nil)

	// 초기 상태 - 재시작 불필요
	if hc.NeedsRestart() {
		t.Error("NeedsRestart() = true; want false for initial state")
	}

	// 1번 실패 - 재시작 필요
	hc.CheckHealth(context.Background())
	if !hc.NeedsRestart() {
		t.Error("NeedsRestart() = false; want true after 1 failure")
	}

	// 2번 실패 - 여전히 재시작 필요
	hc.CheckHealth(context.Background())
	if !hc.NeedsRestart() {
		t.Error("NeedsRestart() = false; want true after 2 failures")
	}

	// maxRestartAttempts번 실패 - 재시작 불필요 (포기)
	hc.CheckHealth(context.Background())
	if hc.NeedsRestart() {
		t.Error("NeedsRestart() = true; want false after max failures (give up)")
	}
}

func TestHealthChecker_Reset(t *testing.T) {
	logger := noopLogger()
	executor := NewCommandExecutor(logger)

	executor.commandFn = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "echo", "ok")
	}

	hc := NewHealthChecker(logger, executor, nil)

	// 성공 상태로 만들기
	hc.CheckHealth(context.Background())
	if !hc.IsHealthy() {
		t.Fatal("IsHealthy() = false; want true before reset")
	}

	hc.Reset()

	if hc.IsHealthy() {
		t.Error("IsHealthy() = true; want false after Reset()")
	}
	if hc.ConsecutiveFailures() != 0 {
		t.Errorf("ConsecutiveFailures() = %d; want 0 after Reset()", hc.ConsecutiveFailures())
	}
}

func TestHealthChecker_NilCallback(t *testing.T) {
	logger := noopLogger()
	executor := NewCommandExecutor(logger)

	executor.commandFn = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "false")
	}

	// nil 콜백으로 생성 - 패닉하지 않아야 한다
	hc := NewHealthChecker(logger, executor, nil)

	// maxRestartAttempts번 이상 실패해도 패닉하지 않아야 한다
	for i := 0; i < maxRestartAttempts+1; i++ {
		hc.CheckHealth(context.Background())
	}
}

func TestHealthChecker_StartMonitoring_Cancel(t *testing.T) {
	logger := noopLogger()
	executor := NewCommandExecutor(logger)

	executor.commandFn = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "echo", "ok")
	}

	hc := NewHealthChecker(logger, executor, nil)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		hc.StartMonitoring(ctx)
		close(done)
	}()

	cancel()

	select {
	case <-done:
		// 정상 종료
	case <-time.After(5 * time.Second):
		t.Fatal("StartMonitoring() did not stop after context cancellation")
	}
}

func TestHealthChecker_Concurrent(t *testing.T) {
	logger := noopLogger()
	executor := NewCommandExecutor(logger)

	executor.commandFn = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "echo", "ok")
	}

	hc := NewHealthChecker(logger, executor, nil)

	// 동시에 여러 goroutine에서 CheckHealth 호출
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			hc.CheckHealth(context.Background())
		}()
	}
	wg.Wait()

	// 데이터 레이스 없이 완료되어야 한다
	if !hc.IsHealthy() {
		t.Error("IsHealthy() = false; want true after concurrent successful checks")
	}
}
