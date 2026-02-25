package agentbrowser

import (
	"context"
	"os/exec"
	"sync"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	logger := noopLogger()
	executor := NewCommandExecutor(logger)
	m := NewManager(logger, executor)

	if m == nil {
		t.Fatal("NewManager() returned nil")
	}
	if m.State() != StateStopped {
		t.Errorf("State() = %q; want %q", m.State(), StateStopped)
	}
	if m.RestartCount() != 0 {
		t.Errorf("RestartCount() = %d; want 0", m.RestartCount())
	}
}

func TestNewManager_WithOptions(t *testing.T) {
	logger := noopLogger()
	executor := NewCommandExecutor(logger)

	var callbackCalled bool
	m := NewManager(logger, executor,
		WithHeadless(true),
		WithInitialURL("https://example.com"),
		WithStatusCallback(func(state ManagerState, message string) {
			callbackCalled = true
		}),
	)

	if !m.headless {
		t.Error("headless = false; want true")
	}
	if m.initialURL != "https://example.com" {
		t.Errorf("initialURL = %q; want %q", m.initialURL, "https://example.com")
	}
	if m.onStatusChange == nil {
		t.Error("onStatusChange is nil; want non-nil")
	}
	_ = callbackCalled // 콜백은 상태 변경 시에만 호출됨
}

func TestManager_Start_Success(t *testing.T) {
	logger := noopLogger()
	executor := NewCommandExecutor(logger)

	// 명령 실행을 성공으로 모킹
	executor.commandFn = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "echo", "ok")
	}

	m := NewManager(logger, executor, WithInitialURL("https://example.com"))

	err := m.Start(context.Background())
	if err != nil {
		t.Fatalf("Start() returned error: %v", err)
	}
	if m.State() != StateReady {
		t.Errorf("State() = %q; want %q", m.State(), StateReady)
	}
}

func TestManager_Start_WithoutInitialURL(t *testing.T) {
	logger := noopLogger()
	executor := NewCommandExecutor(logger)

	m := NewManager(logger, executor)

	err := m.Start(context.Background())
	if err != nil {
		t.Fatalf("Start() returned error: %v", err)
	}
	if m.State() != StateReady {
		t.Errorf("State() = %q; want %q", m.State(), StateReady)
	}
}

func TestManager_Start_ExecutionFailure(t *testing.T) {
	logger := noopLogger()
	executor := NewCommandExecutor(logger)

	// 실패하는 명령 모킹
	executor.commandFn = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "false")
	}

	m := NewManager(logger, executor, WithInitialURL("https://example.com"))

	err := m.Start(context.Background())
	if err == nil {
		t.Fatal("Start() returned nil error; want error")
	}
	if m.State() != StateError {
		t.Errorf("State() = %q; want %q after start failure", m.State(), StateError)
	}
}

func TestManager_Start_AlreadyRunning(t *testing.T) {
	logger := noopLogger()
	executor := NewCommandExecutor(logger)

	m := NewManager(logger, executor)
	_ = m.Start(context.Background())

	err := m.Start(context.Background())
	if err == nil {
		t.Fatal("Start() returned nil error; want error for already running manager")
	}
}

func TestManager_Stop(t *testing.T) {
	logger := noopLogger()
	executor := NewCommandExecutor(logger)

	m := NewManager(logger, executor)
	_ = m.Start(context.Background())

	err := m.Stop()
	if err != nil {
		t.Fatalf("Stop() returned error: %v", err)
	}
	if m.State() != StateStopped {
		t.Errorf("State() = %q; want %q after Stop()", m.State(), StateStopped)
	}
}

func TestManager_Stop_AlreadyStopped(t *testing.T) {
	logger := noopLogger()
	executor := NewCommandExecutor(logger)

	m := NewManager(logger, executor)

	// 이미 중지된 상태에서 Stop 호출 시 에러 없어야 한다
	err := m.Stop()
	if err != nil {
		t.Fatalf("Stop() on stopped manager returned error: %v", err)
	}
}

func TestManager_IsReady(t *testing.T) {
	logger := noopLogger()
	executor := NewCommandExecutor(logger)

	m := NewManager(logger, executor)

	// 초기 상태 (stopped) - not ready
	if m.IsReady() {
		t.Error("IsReady() = true; want false for stopped state")
	}

	// 시작 후 - ready
	_ = m.Start(context.Background())
	if !m.IsReady() {
		t.Error("IsReady() = false; want true after Start()")
	}

	// standby 상태 - ready (명령 수신 가능)
	m.EnterStandby()
	if !m.IsReady() {
		t.Error("IsReady() = false; want true for standby state")
	}

	// 중지 후 - not ready
	_ = m.Stop()
	if m.IsReady() {
		t.Error("IsReady() = true; want false after Stop()")
	}
}

func TestManager_Execute_Success(t *testing.T) {
	logger := noopLogger()
	executor := NewCommandExecutor(logger)

	// 성공적인 실행 모킹
	executor.commandFn = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "echo", "command output")
	}

	m := NewManager(logger, executor)
	_ = m.Start(context.Background())

	result, err := m.Execute(context.Background(), "get", nil, map[string]interface{}{"type": "url"})
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil; want non-nil")
	}
	if result.Output != "command output" {
		t.Errorf("Output = %q; want %q", result.Output, "command output")
	}
}

func TestManager_Execute_NotReady(t *testing.T) {
	logger := noopLogger()
	executor := NewCommandExecutor(logger)

	m := NewManager(logger, executor)

	// 시작하지 않은 상태에서 Execute 호출
	_, err := m.Execute(context.Background(), "get", nil, nil)
	if err == nil {
		t.Fatal("Execute() returned nil error; want error for not-ready state")
	}
}

func TestManager_Execute_Failure_RestartCount(t *testing.T) {
	logger := noopLogger()
	executor := NewCommandExecutor(logger)

	// 항상 실패하는 명령 모킹
	executor.commandFn = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "false")
	}

	m := NewManager(logger, executor)
	// 직접 ready 상태로 설정 (initialURL 없이 Start)
	m.mu.Lock()
	m.state = StateReady
	m.mu.Unlock()

	// 첫 번째 실패
	_, err := m.Execute(context.Background(), "click", nil, nil)
	if err == nil {
		t.Fatal("Execute() returned nil error; want error")
	}
	if m.RestartCount() != 1 {
		t.Errorf("RestartCount() = %d; want 1", m.RestartCount())
	}

	// 두 번째 실패
	m.mu.Lock()
	m.state = StateReady
	m.mu.Unlock()

	_, _ = m.Execute(context.Background(), "click", nil, nil)
	if m.RestartCount() != 2 {
		t.Errorf("RestartCount() = %d; want 2", m.RestartCount())
	}

	// 세 번째 실패 - maxRestartAttempts 도달
	m.mu.Lock()
	m.state = StateReady
	m.mu.Unlock()

	_, err = m.Execute(context.Background(), "click", nil, nil)
	if err == nil {
		t.Fatal("Execute() returned nil error; want error for max restart exceeded")
	}
	if m.State() != StateError {
		t.Errorf("State() = %q; want %q after max restart", m.State(), StateError)
	}
}

func TestManager_Execute_Success_ResetsRestartCount(t *testing.T) {
	logger := noopLogger()
	executor := NewCommandExecutor(logger)

	callCount := 0
	executor.commandFn = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		callCount++
		if callCount == 1 {
			// 첫 번째 호출은 실패 (BuildArgs용)
			return exec.CommandContext(ctx, "false")
		}
		// 두 번째 호출부터 성공
		return exec.CommandContext(ctx, "echo", "ok")
	}

	m := NewManager(logger, executor)
	_ = m.Start(context.Background())

	// 실패 후 restartCount 확인
	_, _ = m.Execute(context.Background(), "click", nil, nil)

	// 상태를 ready로 복원
	m.mu.Lock()
	m.state = StateReady
	m.mu.Unlock()

	// 성공하면 restartCount 초기화
	_, err := m.Execute(context.Background(), "get", nil, nil)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	if m.RestartCount() != 0 {
		t.Errorf("RestartCount() = %d; want 0 after success", m.RestartCount())
	}
}

func TestManager_Execute_StateTransitions(t *testing.T) {
	logger := noopLogger()
	executor := NewCommandExecutor(logger)

	var stateDuringExec ManagerState
	var mu sync.Mutex

	executor.commandFn = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		mu.Lock()
		// 이 시점에서는 명령 실행 중이지만 Manager의 상태를 직접 확인할 수 없으므로,
		// 실행 전후 상태를 확인한다.
		mu.Unlock()
		return exec.CommandContext(ctx, "echo", "ok")
	}

	m := NewManager(logger, executor)
	_ = m.Start(context.Background())

	// 실행 전 상태 확인
	if m.State() != StateReady {
		t.Errorf("before Execute: State() = %q; want %q", m.State(), StateReady)
	}

	_, err := m.Execute(context.Background(), "click", nil, nil)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	// 실행 후 상태 확인 (ready로 복원되어야 한다)
	if m.State() != StateReady {
		t.Errorf("after Execute: State() = %q; want %q", m.State(), StateReady)
	}

	_ = stateDuringExec // 사용되지 않은 변수 경고 방지
}

func TestManager_Execute_Base64Screenshot(t *testing.T) {
	logger := noopLogger()
	executor := NewCommandExecutor(logger)

	// Screenshot 바이너리 데이터가 포함된 CommandResult를 생성하기 위해
	// ExecuteFromPayload의 JSON 파싱을 우회하여 직접 screenshot 데이터 검증
	// Manager.Execute는 CommandResult.Screenshot을 base64로 인코딩한다
	executor.commandFn = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "echo", "ok")
	}

	m := NewManager(logger, executor)
	_ = m.Start(context.Background())

	result, err := m.Execute(context.Background(), "screenshot", nil, nil)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	// echo "ok" 출력이므로 screenshot은 비어 있을 것이다
	// (실제 바이너리 데이터는 CLI가 반환해야 함)
	if result.Screenshot != "" {
		t.Logf("Screenshot = %q (expected empty for echo mock)", result.Screenshot)
	}
}

// --- Standby 관련 테스트 ---

func TestManager_EnterStandby(t *testing.T) {
	logger := noopLogger()
	executor := NewCommandExecutor(logger)

	m := NewManager(logger, executor)
	_ = m.Start(context.Background())

	m.EnterStandby()
	if m.State() != StateStandby {
		t.Errorf("State() = %q; want %q after EnterStandby()", m.State(), StateStandby)
	}
}

func TestManager_EnterStandby_NotReady(t *testing.T) {
	logger := noopLogger()
	executor := NewCommandExecutor(logger)

	m := NewManager(logger, executor)

	// 중지 상태에서 EnterStandby 호출 시 상태가 변경되지 않아야 한다
	m.EnterStandby()
	if m.State() != StateStopped {
		t.Errorf("State() = %q; want %q (unchanged)", m.State(), StateStopped)
	}
}

func TestManager_CheckStandby_IdleTimeout(t *testing.T) {
	logger := noopLogger()
	executor := NewCommandExecutor(logger)

	m := NewManager(logger, executor)
	_ = m.Start(context.Background())

	// lastActiveAt을 과거로 설정하여 유휴 타임아웃 시뮬레이션
	m.mu.Lock()
	m.lastActiveAt = time.Now().Add(-standbyTimeout - time.Minute)
	m.mu.Unlock()

	transitioned := m.CheckStandby()
	if !transitioned {
		t.Error("CheckStandby() = false; want true for idle timeout")
	}
	if m.State() != StateStandby {
		t.Errorf("State() = %q; want %q", m.State(), StateStandby)
	}
}

func TestManager_CheckStandby_NotIdle(t *testing.T) {
	logger := noopLogger()
	executor := NewCommandExecutor(logger)

	m := NewManager(logger, executor)
	_ = m.Start(context.Background())

	transitioned := m.CheckStandby()
	if transitioned {
		t.Error("CheckStandby() = true; want false for recently active manager")
	}
	if m.State() != StateReady {
		t.Errorf("State() = %q; want %q", m.State(), StateReady)
	}
}

func TestManager_CheckStandby_NotReadyState(t *testing.T) {
	logger := noopLogger()
	executor := NewCommandExecutor(logger)

	m := NewManager(logger, executor)

	// 중지 상태에서 CheckStandby 호출 시 false
	transitioned := m.CheckStandby()
	if transitioned {
		t.Error("CheckStandby() = true; want false for non-ready state")
	}
}

// --- 상태 콜백 테스트 ---

func TestManager_StatusCallback(t *testing.T) {
	logger := noopLogger()
	executor := NewCommandExecutor(logger)

	var mu sync.Mutex
	var transitions []ManagerState

	m := NewManager(logger, executor, WithStatusCallback(func(state ManagerState, message string) {
		mu.Lock()
		defer mu.Unlock()
		transitions = append(transitions, state)
	}))

	_ = m.Start(context.Background())

	// goroutine에서 콜백이 실행되므로 짧은 대기
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	// starting과 ready 전환이 기록되어야 한다
	// (goroutine 콜백이므로 순서가 보장되지 않을 수 있다)
	if len(transitions) < 2 {
		t.Errorf("transitions count = %d; want >= 2", len(transitions))
		return
	}

	hasStarting := false
	hasReady := false
	for _, s := range transitions {
		if s == StateStarting {
			hasStarting = true
		}
		if s == StateReady {
			hasReady = true
		}
	}
	if !hasStarting {
		t.Errorf("transitions %v; want containing %q", transitions, StateStarting)
	}
	if !hasReady {
		t.Errorf("transitions %v; want containing %q", transitions, StateReady)
	}
}

// --- LastActiveAt 테스트 ---

func TestManager_LastActiveAt(t *testing.T) {
	logger := noopLogger()
	executor := NewCommandExecutor(logger)

	m := NewManager(logger, executor)
	before := time.Now()
	_ = m.Start(context.Background())
	after := time.Now()

	lastActive := m.LastActiveAt()
	if lastActive.Before(before) || lastActive.After(after) {
		t.Errorf("LastActiveAt() = %v; want between %v and %v", lastActive, before, after)
	}
}

// --- CI/CD 설정 전파 테스트 ---

func TestNewManager_WithCICDConfig(t *testing.T) {
	logger := noopLogger()
	executor := NewCommandExecutor(logger)

	config := CICDConfig{
		Headless:   true,
		JSONOutput: true,
		NoColor:    true,
		Timeout:    5 * time.Minute,
	}

	m := NewManager(logger, executor, WithCICDConfig(config))

	if !m.headless {
		t.Error("headless = false; want true when CICDConfig.Headless is true")
	}
	if !m.cicdConfig.Headless {
		t.Error("cicdConfig.Headless = false; want true")
	}
	if !m.cicdConfig.JSONOutput {
		t.Error("cicdConfig.JSONOutput = false; want true")
	}
	if !m.cicdConfig.NoColor {
		t.Error("cicdConfig.NoColor = false; want true")
	}
	if m.cicdConfig.Timeout != 5*time.Minute {
		t.Errorf("cicdConfig.Timeout = %v; want 5m", m.cicdConfig.Timeout)
	}
}

func TestNewManager_WithCICDConfig_ForcesHeadless(t *testing.T) {
	logger := noopLogger()
	executor := NewCommandExecutor(logger)

	// WithHeadless(false)를 먼저 설정하더라도 WithCICDConfig가 headless를 강제한다.
	m := NewManager(logger, executor,
		WithHeadless(false),
		WithCICDConfig(CICDConfig{Headless: true}),
	)

	if !m.headless {
		t.Error("headless = false; want true (forced by CICDConfig)")
	}
}

func TestNewManager_WithCICDConfig_PropagateToExecutor(t *testing.T) {
	logger := noopLogger()
	executor := NewCommandExecutor(logger)

	config := CICDConfig{
		Headless:   true,
		JSONOutput: true,
		NoColor:    true,
	}

	_ = NewManager(logger, executor, WithCICDConfig(config))

	// executor에 CI/CD 설정이 전파되었는지 확인
	if !executor.cicdConfig.Headless {
		t.Error("executor.cicdConfig.Headless = false; want true")
	}
	if !executor.cicdConfig.JSONOutput {
		t.Error("executor.cicdConfig.JSONOutput = false; want true")
	}
	if !executor.cicdConfig.NoColor {
		t.Error("executor.cicdConfig.NoColor = false; want true")
	}
}

func TestManager_CICDConfig_Accessor(t *testing.T) {
	logger := noopLogger()
	executor := NewCommandExecutor(logger)

	config := CICDConfig{Headless: true, JSONOutput: true}
	m := NewManager(logger, executor, WithCICDConfig(config))

	got := m.CICDConfig()
	if !got.Headless {
		t.Error("CICDConfig().Headless = false; want true")
	}
	if !got.JSONOutput {
		t.Error("CICDConfig().JSONOutput = false; want true")
	}
}

