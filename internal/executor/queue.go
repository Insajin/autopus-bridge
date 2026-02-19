// Package executor는 Local Agent Bridge의 작업 실행 엔진을 제공합니다.
// REQ-S-03: 작업 실행 중에도 새로운 작업을 큐에 추가할 수 있습니다.
package executor

import (
	"errors"
	"sync"

	"github.com/insajin/autopus-agent-protocol"
)

// 큐 관련 에러 정의
var (
	// ErrQueueFull은 큐가 가득 찼을 때 반환됩니다.
	ErrQueueFull = errors.New("작업 큐가 가득 찼습니다")

	// ErrQueueEmpty는 큐가 비어있을 때 반환됩니다.
	ErrQueueEmpty = errors.New("작업 큐가 비어있습니다")
)

// 기본 큐 설정
const (
	// DefaultQueueCapacity는 기본 큐 용량입니다.
	DefaultQueueCapacity = 100
)

// TaskQueue는 스레드 안전한 작업 큐입니다.
// REQ-S-03: 작업 실행 중 새로운 요청을 큐에 저장
type TaskQueue struct {
	// tasks는 대기 중인 작업 목록입니다.
	tasks []ws.TaskRequestPayload
	// mu는 tasks 접근을 보호하는 뮤텍스입니다.
	mu sync.RWMutex
	// capacity는 큐의 최대 용량입니다.
	capacity int
	// cond는 새 작업 추가 시 대기 중인 고루틴에 알리는 조건 변수입니다.
	cond *sync.Cond
}

// TaskQueueOption은 TaskQueue 설정 옵션입니다.
type TaskQueueOption func(*TaskQueue)

// WithQueueCapacity는 큐 용량을 설정합니다.
func WithQueueCapacity(capacity int) TaskQueueOption {
	return func(q *TaskQueue) {
		if capacity > 0 {
			q.capacity = capacity
		}
	}
}

// NewTaskQueue는 새로운 작업 큐를 생성합니다.
func NewTaskQueue(opts ...TaskQueueOption) *TaskQueue {
	q := &TaskQueue{
		tasks:    make([]ws.TaskRequestPayload, 0, DefaultQueueCapacity),
		capacity: DefaultQueueCapacity,
	}

	for _, opt := range opts {
		opt(q)
	}

	// 조건 변수 초기화 (읽기 락 사용)
	q.cond = sync.NewCond(&q.mu)

	return q
}

// Add는 작업을 큐에 추가합니다.
// REQ-S-03: 새로운 작업 요청을 큐에 저장
// 큐가 가득 찬 경우 ErrQueueFull을 반환합니다.
func (q *TaskQueue) Add(task ws.TaskRequestPayload) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	// 큐 용량 확인
	if len(q.tasks) >= q.capacity {
		return ErrQueueFull
	}

	q.tasks = append(q.tasks, task)

	// 대기 중인 고루틴에 알림
	q.cond.Signal()

	return nil
}

// Get은 큐에서 가장 오래된 작업을 꺼냅니다.
// 큐가 비어있으면 ErrQueueEmpty를 반환합니다.
func (q *TaskQueue) Get() (ws.TaskRequestPayload, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.tasks) == 0 {
		return ws.TaskRequestPayload{}, ErrQueueEmpty
	}

	// FIFO: 첫 번째 작업을 꺼냄
	task := q.tasks[0]
	q.tasks = q.tasks[1:]

	return task, nil
}

// GetBlocking은 큐에서 작업을 꺼내되, 비어있으면 대기합니다.
// 컨텍스트 취소 등을 위해 done 채널을 모니터링합니다.
func (q *TaskQueue) GetBlocking(done <-chan struct{}) (ws.TaskRequestPayload, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	// 큐가 비어있으면 대기
	for len(q.tasks) == 0 {
		// done 채널 확인을 위한 비동기 체크
		select {
		case <-done:
			return ws.TaskRequestPayload{}, errors.New("큐 대기 중단됨")
		default:
		}

		// 조건 변수로 대기 (락 해제 후 재획득)
		q.cond.Wait()

		// 깨어난 후 done 채널 재확인
		select {
		case <-done:
			return ws.TaskRequestPayload{}, errors.New("큐 대기 중단됨")
		default:
		}
	}

	// FIFO: 첫 번째 작업을 꺼냄
	task := q.tasks[0]
	q.tasks = q.tasks[1:]

	return task, nil
}

// Peek은 큐의 첫 번째 작업을 확인하되 제거하지 않습니다.
func (q *TaskQueue) Peek() (ws.TaskRequestPayload, error) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	if len(q.tasks) == 0 {
		return ws.TaskRequestPayload{}, ErrQueueEmpty
	}

	return q.tasks[0], nil
}

// Size는 큐에 대기 중인 작업 수를 반환합니다.
func (q *TaskQueue) Size() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.tasks)
}

// IsFull은 큐가 가득 찼는지 확인합니다.
func (q *TaskQueue) IsFull() bool {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.tasks) >= q.capacity
}

// IsEmpty는 큐가 비어있는지 확인합니다.
func (q *TaskQueue) IsEmpty() bool {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.tasks) == 0
}

// Capacity는 큐의 최대 용량을 반환합니다.
func (q *TaskQueue) Capacity() int {
	return q.capacity
}

// Clear는 큐의 모든 작업을 제거합니다.
func (q *TaskQueue) Clear() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.tasks = q.tasks[:0]
}

// Wakeup은 대기 중인 모든 고루틴을 깨웁니다.
// 주로 종료 시 사용됩니다.
func (q *TaskQueue) Wakeup() {
	q.cond.Broadcast()
}

// List는 큐에 있는 모든 작업의 ExecutionID 목록을 반환합니다.
// 디버깅 및 모니터링용입니다.
func (q *TaskQueue) List() []string {
	q.mu.RLock()
	defer q.mu.RUnlock()

	ids := make([]string, len(q.tasks))
	for i, task := range q.tasks {
		ids[i] = task.ExecutionID
	}
	return ids
}
