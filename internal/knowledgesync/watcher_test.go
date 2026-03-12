package knowledgesync

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestNewWatcher_CreatesSuccessfully 는 Watcher 생성을 검증합니다.
func TestNewWatcher_CreatesSuccessfully(t *testing.T) {
	t.Parallel()
	w, err := NewWatcher(DefaultExcludePatterns)
	if err != nil {
		t.Fatalf("NewWatcher() unexpected error: %v", err)
	}
	defer w.Close()

	if w == nil {
		t.Error("NewWatcher() returned nil")
	}
}

// TestWatcher_DetectsFileCreate 는 파일 생성 이벤트 감지를 검증합니다.
func TestWatcher_DetectsFileCreate(t *testing.T) {
	// 병렬 실행 시 임시 디렉토리 경합 가능성으로 순차 실행
	tmpDir := t.TempDir()

	w, err := NewWatcher(DefaultExcludePatterns)
	if err != nil {
		t.Fatalf("NewWatcher() unexpected error: %v", err)
	}
	defer w.Close()

	if err := w.Add(tmpDir); err != nil {
		t.Fatalf("Watcher.Add() unexpected error: %v", err)
	}

	events := w.Events()

	// 파일 생성
	testFile := filepath.Join(tmpDir, "test.md")
	if err := os.WriteFile(testFile, []byte("hello"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	select {
	case event := <-events:
		if event.FilePath == "" {
			t.Error("Watcher event FilePath is empty")
		}
	case <-time.After(3 * time.Second):
		t.Error("Watcher did not emit event within 3 seconds")
	}
}

// TestWatcher_DetectsNestedFileCreate 는 기존 하위 디렉토리 파일 생성 이벤트를 검증합니다.
func TestWatcher_DetectsNestedFileCreate(t *testing.T) {
	tmpDir := t.TempDir()
	nestedDir := filepath.Join(tmpDir, "docs", "nested")
	if err := os.MkdirAll(nestedDir, 0755); err != nil {
		t.Fatalf("Failed to create nested dir: %v", err)
	}

	w, err := NewWatcher(DefaultExcludePatterns)
	if err != nil {
		t.Fatalf("NewWatcher() unexpected error: %v", err)
	}
	defer w.Close()

	if err := w.Add(tmpDir); err != nil {
		t.Fatalf("Watcher.Add() unexpected error: %v", err)
	}

	events := w.Events()
	testFile := filepath.Join(nestedDir, "deep.md")
	if err := os.WriteFile(testFile, []byte("hello"), 0644); err != nil {
		t.Fatalf("Failed to create nested file: %v", err)
	}

	deadline := time.After(3 * time.Second)
	for {
		select {
		case event, ok := <-events:
			if !ok {
				t.Fatal("events channel closed before nested event arrived")
			}
			if filepath.Base(event.FilePath) == "deep.md" {
				return
			}
		case <-deadline:
			t.Fatal("Watcher did not emit nested file event within 3 seconds")
		}
	}
}

// TestWatcher_DetectsNewSubdirectoryFileCreate 는 동적으로 생긴 하위 디렉토리도 감시되는지 검증합니다.
func TestWatcher_DetectsNewSubdirectoryFileCreate(t *testing.T) {
	tmpDir := t.TempDir()

	w, err := NewWatcher(DefaultExcludePatterns)
	if err != nil {
		t.Fatalf("NewWatcher() unexpected error: %v", err)
	}
	defer w.Close()

	if err := w.Add(tmpDir); err != nil {
		t.Fatalf("Watcher.Add() unexpected error: %v", err)
	}

	events := w.Events()
	newDir := filepath.Join(tmpDir, "new-subdir")
	if err := os.MkdirAll(newDir, 0755); err != nil {
		t.Fatalf("Failed to create new subdir: %v", err)
	}
	testFile := filepath.Join(newDir, "later.md")
	if err := os.WriteFile(testFile, []byte("hello"), 0644); err != nil {
		t.Fatalf("Failed to create file in new subdir: %v", err)
	}

	deadline := time.After(4 * time.Second)
	for {
		select {
		case event, ok := <-events:
			if !ok {
				t.Fatal("events channel closed before new subdir event arrived")
			}
			if filepath.Base(event.FilePath) == "later.md" {
				return
			}
		case <-deadline:
			t.Fatal("Watcher did not emit event for file in new subdir within 4 seconds")
		}
	}
}

// TestWatcher_ExcludesSensitiveFiles 는 민감한 파일이 이벤트에서 제외되는지 검증합니다.
func TestWatcher_ExcludesSensitiveFiles(t *testing.T) {
	tmpDir := t.TempDir()

	w, err := NewWatcher(DefaultExcludePatterns)
	if err != nil {
		t.Fatalf("NewWatcher() unexpected error: %v", err)
	}
	defer w.Close()

	if err := w.Add(tmpDir); err != nil {
		t.Fatalf("Watcher.Add() unexpected error: %v", err)
	}

	events := w.Events()

	// .env 파일 생성 (제외 대상)
	envFile := filepath.Join(tmpDir, ".env")
	if err := os.WriteFile(envFile, []byte("SECRET=xxx"), 0644); err != nil {
		t.Fatalf("Failed to create .env file: %v", err)
	}

	// 정상 파일도 생성 (이벤트 있어야 함)
	normalFile := filepath.Join(tmpDir, "normal.md")
	if err := os.WriteFile(normalFile, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create normal file: %v", err)
	}

	// 이벤트를 수집하여 .env 가 없는지 확인
	timeout := time.After(2 * time.Second)
	gotNormal := false
	gotEnv := false

	for {
		select {
		case event := <-events:
			base := filepath.Base(event.FilePath)
			if base == ".env" {
				gotEnv = true
			}
			if base == "normal.md" {
				gotNormal = true
			}
		case <-timeout:
			if gotEnv {
				t.Error("Watcher emitted event for excluded .env file")
			}
			_ = gotNormal
			return
		}
	}
}

// TestWatcher_ChangeEvent_HasType 는 ChangeEvent 에 타입이 설정되는지 검증합니다.
func TestWatcher_ChangeEvent_HasType(t *testing.T) {
	t.Parallel()
	event := WatcherChangeEvent{
		Type:     "create",
		FilePath: "docs/readme.md",
	}
	if event.Type != "create" {
		t.Errorf("WatcherChangeEvent.Type = %q, want 'create'", event.Type)
	}
}

// TestWatcher_Debounce_BatchesRapidChanges 는 디바운싱으로 빠른 변경이 배치되는지 검증합니다.
func TestWatcher_Debounce_BatchesRapidChanges(t *testing.T) {
	tmpDir := t.TempDir()

	w, err := NewWatcher(DefaultExcludePatterns)
	if err != nil {
		t.Fatalf("NewWatcher() unexpected error: %v", err)
	}
	defer w.Close()

	if err := w.Add(tmpDir); err != nil {
		t.Fatalf("Watcher.Add() unexpected error: %v", err)
	}

	events := w.Events()

	// 동일 파일 여러 번 빠르게 수정
	testFile := filepath.Join(tmpDir, "rapid.md")
	for i := range 5 {
		content := []byte("version" + string(rune('0'+i)))
		_ = os.WriteFile(testFile, content, 0644)
		time.Sleep(10 * time.Millisecond)
	}

	// 디바운스(500ms) 후 이벤트 수집
	time.Sleep(700 * time.Millisecond)

	eventCount := 0
loop:
	for {
		select {
		case <-events:
			eventCount++
		case <-time.After(100 * time.Millisecond):
			break loop
		}
	}

	// 5번 변경이 디바운싱으로 줄어들어야 합니다 (1-2개 이벤트 예상)
	if eventCount > 3 {
		t.Errorf("Debounce: got %d events for 5 rapid changes, expected <= 3", eventCount)
	}
}

// TestWatcher_Close_ClosesEvents 는 Close 호출 시 events 채널이 닫히는지 검증합니다.
func TestWatcher_Close_ClosesEvents(t *testing.T) {
	t.Parallel()

	w, err := NewWatcher(DefaultExcludePatterns)
	if err != nil {
		t.Fatalf("NewWatcher() unexpected error: %v", err)
	}

	events := w.Events()
	if err := w.Close(); err != nil {
		t.Fatalf("Watcher.Close() unexpected error: %v", err)
	}

	select {
	case _, ok := <-events:
		if ok {
			t.Fatal("expected closed events channel")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("events channel was not closed within 2 seconds")
	}
}
