// Package websocket는 Local Agent Bridge의 WebSocket 통신을 담당합니다.
// FR-P2-04: 멱등적 태스크 재시도를 위한 태스크 추적기.
package websocket

import (
	"sync"
	"time"
)

// TrackedTask는 추적 중인 작업 정보입니다 (FR-P2-04).
type TrackedTask struct {
	// ExecutionID는 서버에서 할당한 실행 ID입니다.
	ExecutionID string
	// TaskType은 작업 유형입니다 ("task", "build", "test", "qa").
	TaskType string
	// StartedAt은 작업 시작 시각입니다.
	StartedAt time.Time
}

// TaskTracker는 진행 중인 태스크를 추적하여 재연결 시 멱등적 재실행을 지원합니다 (FR-P2-04).
// 연결이 끊어졌다가 재연결되면, 활성 태스크 목록을 서버에 조회하여
// 완료 여부를 확인하고 필요 시 재실행합니다.
type TaskTracker struct {
	// activeTasks는 현재 진행 중인 태스크 맵입니다 (execution_id -> 태스크 정보).
	activeTasks map[string]*TrackedTask
	// mu는 activeTasks 접근을 보호하는 뮤텍스입니다.
	mu sync.RWMutex
}

// NewTaskTracker는 새로운 TaskTracker를 생성합니다.
func NewTaskTracker() *TaskTracker {
	return &TaskTracker{
		activeTasks: make(map[string]*TrackedTask),
	}
}

// Track은 새로운 작업을 활성 목록에 등록합니다.
// 작업 수신 시 호출하여 실행 ID와 작업 유형을 기록합니다.
func (t *TaskTracker) Track(executionID, taskType string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.activeTasks[executionID] = &TrackedTask{
		ExecutionID: executionID,
		TaskType:    taskType,
		StartedAt:   time.Now(),
	}
}

// Complete은 작업을 완료 처리하고 활성 목록에서 제거합니다.
// 작업 실행 완료(성공/실패 무관) 시 호출합니다.
func (t *TaskTracker) Complete(executionID string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	delete(t.activeTasks, executionID)
}

// GetActiveTasks는 미완료 작업의 실행 ID 목록을 반환합니다.
// 재연결 시 서버에 상태를 조회할 태스크 목록으로 사용됩니다.
func (t *TaskTracker) GetActiveTasks() []string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	ids := make([]string, 0, len(t.activeTasks))
	for id := range t.activeTasks {
		ids = append(ids, id)
	}
	return ids
}

// GetActiveTaskCount는 활성 작업 수를 반환합니다.
func (t *TaskTracker) GetActiveTaskCount() int {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return len(t.activeTasks)
}

// IsActive는 해당 실행 ID가 활성 상태인지 확인합니다.
func (t *TaskTracker) IsActive(executionID string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	_, exists := t.activeTasks[executionID]
	return exists
}

// Clear는 모든 추적 작업을 제거합니다.
func (t *TaskTracker) Clear() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.activeTasks = make(map[string]*TrackedTask)
}
