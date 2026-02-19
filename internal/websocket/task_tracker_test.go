package websocket

import (
	"fmt"
	"sync"
	"testing"
)

func TestTaskTracker_TrackAndComplete(t *testing.T) {
	tracker := NewTaskTracker()

	// 작업 추적 시작
	tracker.Track("exec-001", "task")
	tracker.Track("exec-002", "build")
	tracker.Track("exec-003", "test")

	// 활성 작업 수 확인
	if count := tracker.GetActiveTaskCount(); count != 3 {
		t.Errorf("활성 작업 수 = %d, 기대값 3", count)
	}

	// IsActive 확인
	if !tracker.IsActive("exec-001") {
		t.Error("exec-001이 활성 상태여야 합니다")
	}
	if !tracker.IsActive("exec-002") {
		t.Error("exec-002가 활성 상태여야 합니다")
	}
	if tracker.IsActive("exec-999") {
		t.Error("exec-999가 비활성 상태여야 합니다")
	}

	// 작업 완료 처리
	tracker.Complete("exec-002")

	if count := tracker.GetActiveTaskCount(); count != 2 {
		t.Errorf("완료 후 활성 작업 수 = %d, 기대값 2", count)
	}

	if tracker.IsActive("exec-002") {
		t.Error("exec-002가 완료 후 비활성 상태여야 합니다")
	}

	// 아직 활성인 작업 확인
	if !tracker.IsActive("exec-001") {
		t.Error("exec-001이 여전히 활성 상태여야 합니다")
	}
	if !tracker.IsActive("exec-003") {
		t.Error("exec-003이 여전히 활성 상태여야 합니다")
	}

	// 존재하지 않는 작업 완료 (panic 없어야 함)
	tracker.Complete("exec-999")

	// 기존 활성 작업 수에 영향 없음
	if count := tracker.GetActiveTaskCount(); count != 2 {
		t.Errorf("존재하지 않는 작업 완료 후 활성 작업 수 = %d, 기대값 2", count)
	}
}

func TestTaskTracker_GetActiveTasks(t *testing.T) {
	tracker := NewTaskTracker()

	// 빈 상태에서 호출
	tasks := tracker.GetActiveTasks()
	if len(tasks) != 0 {
		t.Errorf("빈 상태에서 활성 작업 수 = %d, 기대값 0", len(tasks))
	}

	// 작업 추가
	tracker.Track("exec-001", "task")
	tracker.Track("exec-002", "build")
	tracker.Track("exec-003", "qa")

	tasks = tracker.GetActiveTasks()
	if len(tasks) != 3 {
		t.Errorf("활성 작업 수 = %d, 기대값 3", len(tasks))
	}

	// 모든 exec ID가 포함되어 있는지 확인
	taskMap := make(map[string]bool)
	for _, id := range tasks {
		taskMap[id] = true
	}
	if !taskMap["exec-001"] || !taskMap["exec-002"] || !taskMap["exec-003"] {
		t.Errorf("활성 작업 목록에 기대한 ID가 없습니다: %v", tasks)
	}

	// 하나 완료 후 확인
	tracker.Complete("exec-001")

	tasks = tracker.GetActiveTasks()
	if len(tasks) != 2 {
		t.Errorf("완료 후 활성 작업 수 = %d, 기대값 2", len(tasks))
	}

	taskMap = make(map[string]bool)
	for _, id := range tasks {
		taskMap[id] = true
	}
	if taskMap["exec-001"] {
		t.Error("완료된 exec-001이 활성 목록에 포함되면 안 됩니다")
	}
}

func TestTaskTracker_Clear(t *testing.T) {
	tracker := NewTaskTracker()

	tracker.Track("exec-001", "task")
	tracker.Track("exec-002", "build")
	tracker.Track("exec-003", "test")

	if count := tracker.GetActiveTaskCount(); count != 3 {
		t.Errorf("Clear 전 활성 작업 수 = %d, 기대값 3", count)
	}

	tracker.Clear()

	if count := tracker.GetActiveTaskCount(); count != 0 {
		t.Errorf("Clear 후 활성 작업 수 = %d, 기대값 0", count)
	}

	// Clear 후 새 작업 추가 가능
	tracker.Track("exec-004", "task")
	if count := tracker.GetActiveTaskCount(); count != 1 {
		t.Errorf("Clear 후 새 작업 추가 후 활성 작업 수 = %d, 기대값 1", count)
	}
}

func TestTaskTracker_DuplicateTrack(t *testing.T) {
	tracker := NewTaskTracker()

	// 동일 ID로 두 번 Track 호출 (덮어쓰기 됨)
	tracker.Track("exec-001", "task")
	tracker.Track("exec-001", "build")

	if count := tracker.GetActiveTaskCount(); count != 1 {
		t.Errorf("중복 Track 후 활성 작업 수 = %d, 기대값 1", count)
	}
}

func TestTaskTracker_ConcurrentAccess(t *testing.T) {
	tracker := NewTaskTracker()

	var wg sync.WaitGroup

	// 동시에 100개의 작업 추적
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			execID := fmt.Sprintf("exec-%03d", id)
			tracker.Track(execID, "task")
		}(i)
	}

	wg.Wait()

	if count := tracker.GetActiveTaskCount(); count != 100 {
		t.Errorf("동시 추가 후 활성 작업 수 = %d, 기대값 100", count)
	}

	// 동시에 50개 완료
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			execID := fmt.Sprintf("exec-%03d", id)
			tracker.Complete(execID)
		}(i)
	}

	wg.Wait()

	if count := tracker.GetActiveTaskCount(); count != 50 {
		t.Errorf("동시 완료 후 활성 작업 수 = %d, 기대값 50", count)
	}

	// 동시에 읽기와 쓰기 혼합
	for i := 50; i < 75; i++ {
		wg.Add(2)
		go func(id int) {
			defer wg.Done()
			execID := fmt.Sprintf("exec-%03d", id)
			tracker.Complete(execID)
		}(i)
		go func() {
			defer wg.Done()
			_ = tracker.GetActiveTasks()
			_ = tracker.GetActiveTaskCount()
		}()
	}

	wg.Wait()

	if count := tracker.GetActiveTaskCount(); count != 25 {
		t.Errorf("혼합 동시 접근 후 활성 작업 수 = %d, 기대값 25", count)
	}
}
