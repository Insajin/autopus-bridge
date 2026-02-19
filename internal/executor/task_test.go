package executor

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/insajin/autopus-agent-protocol"
	"github.com/insajin/autopus-bridge/internal/provider"
	"github.com/rs/zerolog"
)

// ====================
// TaskQueue 테스트
// ====================

func TestTaskQueue_NewTaskQueue(t *testing.T) {
	// 기본 큐 생성 테스트
	q := NewTaskQueue()
	if q == nil {
		t.Fatal("NewTaskQueue가 nil을 반환함")
	}
	if q.Capacity() != DefaultQueueCapacity {
		t.Errorf("기본 용량이 잘못됨: got %d, want %d", q.Capacity(), DefaultQueueCapacity)
	}
	if !q.IsEmpty() {
		t.Error("새 큐가 비어있지 않음")
	}
}

func TestTaskQueue_WithCapacity(t *testing.T) {
	// 용량 옵션 테스트
	capacity := 50
	q := NewTaskQueue(WithQueueCapacity(capacity))
	if q.Capacity() != capacity {
		t.Errorf("용량 설정 실패: got %d, want %d", q.Capacity(), capacity)
	}
}

func TestTaskQueue_AddAndGet(t *testing.T) {
	q := NewTaskQueue()

	// 작업 추가
	task1 := ws.TaskRequestPayload{ExecutionID: "exec-1", Prompt: "test1"}
	task2 := ws.TaskRequestPayload{ExecutionID: "exec-2", Prompt: "test2"}

	if err := q.Add(task1); err != nil {
		t.Fatalf("작업 추가 실패: %v", err)
	}
	if err := q.Add(task2); err != nil {
		t.Fatalf("작업 추가 실패: %v", err)
	}

	// 크기 확인
	if q.Size() != 2 {
		t.Errorf("큐 크기 오류: got %d, want 2", q.Size())
	}

	// FIFO 순서로 가져오기
	got1, err := q.Get()
	if err != nil {
		t.Fatalf("작업 가져오기 실패: %v", err)
	}
	if got1.ExecutionID != task1.ExecutionID {
		t.Errorf("FIFO 순서 오류: got %s, want %s", got1.ExecutionID, task1.ExecutionID)
	}

	got2, err := q.Get()
	if err != nil {
		t.Fatalf("작업 가져오기 실패: %v", err)
	}
	if got2.ExecutionID != task2.ExecutionID {
		t.Errorf("FIFO 순서 오류: got %s, want %s", got2.ExecutionID, task2.ExecutionID)
	}

	// 빈 큐에서 가져오기
	_, err = q.Get()
	if !errors.Is(err, ErrQueueEmpty) {
		t.Errorf("빈 큐 에러 오류: got %v, want %v", err, ErrQueueEmpty)
	}
}

func TestTaskQueue_IsFull(t *testing.T) {
	capacity := 3
	q := NewTaskQueue(WithQueueCapacity(capacity))

	// 용량까지 채우기
	for i := 0; i < capacity; i++ {
		task := ws.TaskRequestPayload{ExecutionID: string(rune('0' + i))}
		if err := q.Add(task); err != nil {
			t.Fatalf("작업 추가 실패: %v", err)
		}
	}

	// 가득 찼는지 확인
	if !q.IsFull() {
		t.Error("큐가 가득 찬 상태여야 함")
	}

	// 추가 시도
	err := q.Add(ws.TaskRequestPayload{ExecutionID: "overflow"})
	if !errors.Is(err, ErrQueueFull) {
		t.Errorf("가득 찬 큐 에러 오류: got %v, want %v", err, ErrQueueFull)
	}
}

func TestTaskQueue_Peek(t *testing.T) {
	q := NewTaskQueue()

	// 빈 큐에서 Peek
	_, err := q.Peek()
	if !errors.Is(err, ErrQueueEmpty) {
		t.Errorf("빈 큐 Peek 에러 오류: got %v, want %v", err, ErrQueueEmpty)
	}

	// 작업 추가 후 Peek
	task := ws.TaskRequestPayload{ExecutionID: "peek-test"}
	_ = q.Add(task)

	peeked, err := q.Peek()
	if err != nil {
		t.Fatalf("Peek 실패: %v", err)
	}
	if peeked.ExecutionID != task.ExecutionID {
		t.Errorf("Peek 결과 오류: got %s, want %s", peeked.ExecutionID, task.ExecutionID)
	}

	// Peek 후 크기가 변하지 않아야 함
	if q.Size() != 1 {
		t.Error("Peek 후 크기가 변함")
	}
}

func TestTaskQueue_Clear(t *testing.T) {
	q := NewTaskQueue()

	// 작업 추가
	for i := 0; i < 5; i++ {
		_ = q.Add(ws.TaskRequestPayload{ExecutionID: string(rune('0' + i))})
	}

	// Clear
	q.Clear()

	if !q.IsEmpty() {
		t.Error("Clear 후 큐가 비어있지 않음")
	}
}

func TestTaskQueue_List(t *testing.T) {
	q := NewTaskQueue()

	ids := []string{"id-1", "id-2", "id-3"}
	for _, id := range ids {
		_ = q.Add(ws.TaskRequestPayload{ExecutionID: id})
	}

	list := q.List()
	if len(list) != len(ids) {
		t.Fatalf("List 길이 오류: got %d, want %d", len(list), len(ids))
	}

	for i, id := range list {
		if id != ids[i] {
			t.Errorf("List[%d] 오류: got %s, want %s", i, id, ids[i])
		}
	}
}

func TestTaskQueue_Concurrent(t *testing.T) {
	// 동시성 테스트
	q := NewTaskQueue(WithQueueCapacity(1000))
	var wg sync.WaitGroup
	numGoroutines := 10
	tasksPerGoroutine := 50

	// 동시 추가
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(base int) {
			defer wg.Done()
			for j := 0; j < tasksPerGoroutine; j++ {
				task := ws.TaskRequestPayload{
					ExecutionID: string(rune(base*100 + j)),
				}
				_ = q.Add(task)
			}
		}(i)
	}

	wg.Wait()

	expectedSize := numGoroutines * tasksPerGoroutine
	if q.Size() != expectedSize {
		t.Errorf("동시 추가 후 크기 오류: got %d, want %d", q.Size(), expectedSize)
	}
}

// ====================
// TaskExecutor 테스트
// ====================

// mockSender는 테스트용 TaskSender 모의 객체입니다.
type mockSender struct {
	progressCalls []ws.TaskProgressPayload
	resultCalls   []ws.TaskResultPayload
	errorCalls    []ws.TaskErrorPayload
	lastExecID    string
	mu            sync.Mutex
}

func newMockSender() *mockSender {
	return &mockSender{
		progressCalls: make([]ws.TaskProgressPayload, 0),
		resultCalls:   make([]ws.TaskResultPayload, 0),
		errorCalls:    make([]ws.TaskErrorPayload, 0),
	}
}

func (m *mockSender) SendTaskProgress(payload ws.TaskProgressPayload) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.progressCalls = append(m.progressCalls, payload)
	return nil
}

func (m *mockSender) SendTaskResult(payload ws.TaskResultPayload) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.resultCalls = append(m.resultCalls, payload)
	return nil
}

func (m *mockSender) SendTaskError(payload ws.TaskErrorPayload) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errorCalls = append(m.errorCalls, payload)
	return nil
}

func (m *mockSender) SetLastExecID(execID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastExecID = execID
}

func (m *mockSender) GetProgressCalls() []ws.TaskProgressPayload {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]ws.TaskProgressPayload, len(m.progressCalls))
	copy(result, m.progressCalls)
	return result
}

func (m *mockSender) GetResultCalls() []ws.TaskResultPayload {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]ws.TaskResultPayload, len(m.resultCalls))
	copy(result, m.resultCalls)
	return result
}

func (m *mockSender) GetErrorCalls() []ws.TaskErrorPayload {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]ws.TaskErrorPayload, len(m.errorCalls))
	copy(result, m.errorCalls)
	return result
}

// mockProvider는 테스트용 Provider 모의 객체입니다.
type mockProvider struct {
	name         string
	executeFunc  func(ctx context.Context, req provider.ExecuteRequest) (*provider.ExecuteResponse, error)
	executeDelay time.Duration
}

func (m *mockProvider) Name() string {
	return m.name
}

func (m *mockProvider) Execute(ctx context.Context, req provider.ExecuteRequest) (*provider.ExecuteResponse, error) {
	if m.executeDelay > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(m.executeDelay):
		}
	}
	if m.executeFunc != nil {
		return m.executeFunc(ctx, req)
	}
	return &provider.ExecuteResponse{
		Output:     "mock output",
		DurationMs: 100,
		TokenUsage: provider.TokenUsage{
			InputTokens:  10,
			OutputTokens: 20,
			TotalTokens:  30,
		},
	}, nil
}

func (m *mockProvider) ValidateConfig() error {
	return nil
}

func (m *mockProvider) Supports(model string) bool {
	return true
}

func TestTaskExecutor_NewTaskExecutor(t *testing.T) {
	registry := provider.NewRegistry()
	sender := newMockSender()

	executor := NewTaskExecutor(registry, sender)
	if executor == nil {
		t.Fatal("NewTaskExecutor가 nil을 반환함")
	}

	if executor.IsExecuting() {
		t.Error("새 실행기가 실행 중 상태임")
	}
}

func TestTaskExecutor_Execute_Success(t *testing.T) {
	registry := provider.NewRegistry()
	registry.Register(&mockProvider{name: "claude"})
	sender := newMockSender()
	logger := zerolog.Nop()

	executor := NewTaskExecutor(registry, sender, WithLogger(logger))

	ctx := context.Background()
	task := ws.TaskRequestPayload{
		ExecutionID: "test-exec-1",
		Prompt:      "Hello",
		Model:       "claude-sonnet",
		MaxTokens:   100,
		Timeout:     60,
	}

	result, err := executor.Execute(ctx, task)
	if err != nil {
		t.Fatalf("Execute 실패: %v", err)
	}

	if result.ExecutionID != task.ExecutionID {
		t.Errorf("ExecutionID 오류: got %s, want %s", result.ExecutionID, task.ExecutionID)
	}
	if result.Output != "mock output" {
		t.Errorf("Output 오류: got %s, want mock output", result.Output)
	}
	if result.ExitCode != 0 {
		t.Errorf("ExitCode 오류: got %d, want 0", result.ExitCode)
	}
}

func TestTaskExecutor_Execute_ProviderNotFound(t *testing.T) {
	registry := provider.NewRegistry() // 빈 레지스트리
	sender := newMockSender()
	logger := zerolog.Nop()

	executor := NewTaskExecutor(registry, sender, WithLogger(logger))

	ctx := context.Background()
	task := ws.TaskRequestPayload{
		ExecutionID: "test-exec-2",
		Prompt:      "Hello",
		Model:       "unknown-model",
		Timeout:     60,
	}

	_, err := executor.Execute(ctx, task)
	if err == nil {
		t.Fatal("프로바이더 없이 성공하면 안 됨")
	}

	var taskErr *TaskError
	if !errors.As(err, &taskErr) {
		t.Fatalf("TaskError가 아님: %v", err)
	}
	if taskErr.Code != ErrorCodeProviderNotFound {
		t.Errorf("에러 코드 오류: got %s, want %s", taskErr.Code, ErrorCodeProviderNotFound)
	}
}

func TestTaskExecutor_Execute_Timeout(t *testing.T) {
	registry := provider.NewRegistry()
	registry.Register(&mockProvider{
		name:         "claude",
		executeDelay: 5 * time.Second, // 긴 지연
	})
	sender := newMockSender()
	logger := zerolog.Nop()

	executor := NewTaskExecutor(registry, sender, WithLogger(logger))

	ctx := context.Background()
	task := ws.TaskRequestPayload{
		ExecutionID: "test-exec-3",
		Prompt:      "Hello",
		Model:       "claude-sonnet",
		Timeout:     1, // 1초 타임아웃
	}

	_, err := executor.Execute(ctx, task)
	if err == nil {
		t.Fatal("타임아웃 시 에러가 발생해야 함")
	}

	var taskErr *TaskError
	if !errors.As(err, &taskErr) {
		t.Fatalf("TaskError가 아님: %v", err)
	}
	if taskErr.Code != ErrorCodeTimeout {
		t.Errorf("에러 코드 오류: got %s, want %s", taskErr.Code, ErrorCodeTimeout)
	}
	if !taskErr.Retryable {
		t.Error("타임아웃은 재시도 가능해야 함")
	}
}

func TestTaskExecutor_Execute_Cancelled(t *testing.T) {
	registry := provider.NewRegistry()
	registry.Register(&mockProvider{
		name:         "claude",
		executeDelay: 10 * time.Second,
	})
	sender := newMockSender()
	logger := zerolog.Nop()

	executor := NewTaskExecutor(registry, sender, WithLogger(logger))

	ctx, cancel := context.WithCancel(context.Background())
	task := ws.TaskRequestPayload{
		ExecutionID: "test-exec-4",
		Prompt:      "Hello",
		Model:       "claude-sonnet",
		Timeout:     60,
	}

	// 100ms 후 취소
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	_, err := executor.Execute(ctx, task)
	if err == nil {
		t.Fatal("취소 시 에러가 발생해야 함")
	}

	var taskErr *TaskError
	if !errors.As(err, &taskErr) {
		t.Fatalf("TaskError가 아님: %v", err)
	}
	if taskErr.Code != ErrorCodeCancelled {
		t.Errorf("에러 코드 오류: got %s, want %s", taskErr.Code, ErrorCodeCancelled)
	}
	if taskErr.Retryable {
		t.Error("취소는 재시도 불가능해야 함")
	}
}

func TestTaskExecutor_Submit_And_Process(t *testing.T) {
	registry := provider.NewRegistry()
	registry.Register(&mockProvider{name: "claude"})
	sender := newMockSender()
	logger := zerolog.Nop()

	executor := NewTaskExecutor(registry, sender, WithLogger(logger))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 실행기 시작
	executor.Start(ctx)
	defer executor.Stop()

	// 작업 제출
	task := ws.TaskRequestPayload{
		ExecutionID: "test-exec-5",
		Prompt:      "Hello",
		Model:       "claude-sonnet",
		Timeout:     60,
	}

	err := executor.Submit(task)
	if err != nil {
		t.Fatalf("Submit 실패: %v", err)
	}

	// 결과 대기 (최대 2초)
	timeout := time.After(2 * time.Second)
	for {
		select {
		case <-timeout:
			t.Fatal("작업 처리 타임아웃")
		default:
			results := sender.GetResultCalls()
			if len(results) > 0 {
				if results[0].ExecutionID != task.ExecutionID {
					t.Errorf("결과 ExecutionID 오류: got %s, want %s",
						results[0].ExecutionID, task.ExecutionID)
				}
				return // 성공
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
}

func TestTaskExecutor_ProgressReporting(t *testing.T) {
	// 진행 상황 보고 간격을 테스트용으로 짧게 설정할 수 없으므로
	// Execute 함수가 진행 상황 고루틴을 시작하는지만 확인
	registry := provider.NewRegistry()
	var executeStarted atomic.Bool
	registry.Register(&mockProvider{
		name: "claude",
		executeFunc: func(ctx context.Context, req provider.ExecuteRequest) (*provider.ExecuteResponse, error) {
			executeStarted.Store(true)
			// 짧은 지연
			time.Sleep(100 * time.Millisecond)
			return &provider.ExecuteResponse{
				Output:     "done",
				DurationMs: 100,
			}, nil
		},
	})
	sender := newMockSender()
	logger := zerolog.Nop()

	executor := NewTaskExecutor(registry, sender, WithLogger(logger))

	ctx := context.Background()
	task := ws.TaskRequestPayload{
		ExecutionID: "test-exec-6",
		Prompt:      "Hello",
		Model:       "claude-sonnet",
		Timeout:     60,
	}

	_, err := executor.Execute(ctx, task)
	if err != nil {
		t.Fatalf("Execute 실패: %v", err)
	}

	if !executeStarted.Load() {
		t.Error("프로바이더 실행이 호출되지 않음")
	}
}

func TestTaskExecutor_QueueOperations(t *testing.T) {
	registry := provider.NewRegistry()
	sender := newMockSender()
	logger := zerolog.Nop()

	executor := NewTaskExecutor(registry, sender, WithLogger(logger))

	// 초기 큐 크기
	if executor.QueueSize() != 0 {
		t.Errorf("초기 큐 크기 오류: got %d, want 0", executor.QueueSize())
	}

	// 실행기 시작 전 Submit은 실패해야 함
	err := executor.Submit(ws.TaskRequestPayload{ExecutionID: "test"})
	if err == nil {
		t.Error("실행기 중지 상태에서 Submit이 성공하면 안 됨")
	}

	// 실행기 시작
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	executor.Start(ctx)

	// 프로바이더 없이 Submit (큐에는 추가됨)
	task := ws.TaskRequestPayload{
		ExecutionID: "test-queue",
		Prompt:      "Hello",
		Model:       "claude-sonnet",
		Timeout:     60,
	}

	err = executor.Submit(task)
	if err != nil {
		t.Fatalf("Submit 실패: %v", err)
	}

	executor.Stop()
}

func TestTaskExecutor_CancelCurrent(t *testing.T) {
	registry := provider.NewRegistry()
	var executing atomic.Bool
	registry.Register(&mockProvider{
		name: "claude",
		executeFunc: func(ctx context.Context, req provider.ExecuteRequest) (*provider.ExecuteResponse, error) {
			executing.Store(true)
			<-ctx.Done() // 취소될 때까지 대기
			return nil, ctx.Err()
		},
	})
	sender := newMockSender()
	logger := zerolog.Nop()

	executor := NewTaskExecutor(registry, sender, WithLogger(logger))

	ctx := context.Background()
	task := ws.TaskRequestPayload{
		ExecutionID: "test-cancel",
		Prompt:      "Hello",
		Model:       "claude-sonnet",
		Timeout:     60,
	}

	// 비동기로 실행 시작
	done := make(chan struct{})
	go func() {
		_, _ = executor.Execute(ctx, task)
		close(done)
	}()

	// 실행 시작 대기
	for !executing.Load() {
		time.Sleep(10 * time.Millisecond)
	}

	// 실행 중 확인
	if !executor.IsExecuting() {
		t.Error("실행 중이어야 함")
	}
	if executor.CurrentExecutionID() != task.ExecutionID {
		t.Errorf("현재 실행 ID 오류: got %s, want %s",
			executor.CurrentExecutionID(), task.ExecutionID)
	}

	// 취소
	cancelled := executor.CancelCurrent()
	if !cancelled {
		t.Error("취소가 성공해야 함")
	}

	// 완료 대기
	select {
	case <-done:
		// 성공
	case <-time.After(2 * time.Second):
		t.Fatal("취소 후 완료 대기 타임아웃")
	}
}

func TestTaskError(t *testing.T) {
	err := &TaskError{
		Code:      ErrorCodeProviderError,
		Message:   "테스트 에러",
		Retryable: true,
	}

	// Error() 메서드 테스트
	errStr := err.Error()
	if errStr != "[PROVIDER_ERROR] 테스트 에러" {
		t.Errorf("에러 문자열 오류: got %s", errStr)
	}

	// errors.As 테스트
	var taskErr *TaskError
	if !errors.As(err, &taskErr) {
		t.Error("errors.As가 실패함")
	}
}
