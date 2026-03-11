package knowledgesync

import (
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

const (
	// debounceWindow 는 디바운싱 윈도우 크기입니다 (빠른 연속 변경 배치).
	debounceWindow = 500 * time.Millisecond
	// eventChannelBuffer 는 이벤트 채널 버퍼 크기입니다.
	eventChannelBuffer = 100
)

// WatcherChangeEvent 는 파일 시스템 변경 이벤트를 담습니다.
// SPEC-KHSOURCE-001 TASK-010
type WatcherChangeEvent struct {
	// Type 은 "create", "modify", "delete" 중 하나입니다.
	Type string
	// FilePath 는 변경된 파일의 경로입니다.
	FilePath string
}

// Watcher 는 fsnotify 기반 재귀 파일 감시자입니다.
// 디바운싱으로 빠른 연속 변경을 배치 처리하고 민감한 파일을 필터링합니다.
// SPEC-KHSOURCE-001 TASK-010
//
// @MX:ANCHOR: Watcher — 로컬 파일시스템 변경 감지 진입점
// @MX:REASON: knowledge sync 커맨드에서 --watch 플래그 시 사용하는 핵심 컴포넌트
// @MX:SPEC: SPEC-KHSOURCE-001
type Watcher struct {
	fsWatcher       *fsnotify.Watcher
	excludePatterns []string
	events          chan WatcherChangeEvent
	done            chan struct{}
	mu              sync.Mutex
	// debounce 맵: 파일 경로 → 마지막 이벤트 시각
	debounceMap map[string]time.Time
}

// NewWatcher 는 새 Watcher 를 생성합니다.
// excludePatterns: 이벤트에서 제외할 파일 패턴 목록
func NewWatcher(excludePatterns []string) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	w := &Watcher{
		fsWatcher:       fsw,
		excludePatterns: excludePatterns,
		events:          make(chan WatcherChangeEvent, eventChannelBuffer),
		done:            make(chan struct{}),
		debounceMap:     make(map[string]time.Time),
	}

	go w.run()

	return w, nil
}

// Add 는 감시 대상 디렉토리를 추가합니다.
func (w *Watcher) Add(path string) error {
	return w.fsWatcher.Add(path)
}

// Events 는 파일 변경 이벤트 채널을 반환합니다.
func (w *Watcher) Events() <-chan WatcherChangeEvent {
	return w.events
}

// Close 는 파일 감시자를 종료합니다.
func (w *Watcher) Close() error {
	close(w.done)
	return w.fsWatcher.Close()
}

// run 은 fsnotify 이벤트를 처리하는 고루틴입니다.
//
// @MX:WARN: 고루틴 — Watcher.done 채널로 생명주기를 관리합니다
// @MX:REASON: Close() 호출 없이 watcher 를 버리면 고루틴 누수가 발생합니다
// @MX:SPEC: SPEC-KHSOURCE-001
func (w *Watcher) run() {
	ticker := time.NewTicker(debounceWindow)
	defer ticker.Stop()

	// 배치 맵: 파일 경로 → 이벤트 타입
	batch := make(map[string]string)

	for {
		select {
		case <-w.done:
			return

		case event, ok := <-w.fsWatcher.Events:
			if !ok {
				return
			}

			path := filepath.ToSlash(event.Name)

			// 제외 패턴 검사
			if IsExcluded(path, w.excludePatterns) {
				continue
			}

			// 이벤트 타입 매핑
			eventType := fsnotifyOpToType(event.Op)
			if eventType == "" {
				continue
			}

			w.mu.Lock()
			batch[path] = eventType
			w.mu.Unlock()

		case <-ticker.C:
			// 디바운스 윈도우 경과 후 배치 방출
			w.mu.Lock()
			if len(batch) == 0 {
				w.mu.Unlock()
				continue
			}
			// 배치 복사 후 초기화
			currentBatch := make(map[string]string, len(batch))
			for k, v := range batch {
				currentBatch[k] = v
			}
			batch = make(map[string]string)
			w.mu.Unlock()

			// 채널에 이벤트 방출
			for path, eventType := range currentBatch {
				select {
				case w.events <- WatcherChangeEvent{Type: eventType, FilePath: path}:
				case <-w.done:
					return
				}
			}

		case err, ok := <-w.fsWatcher.Errors:
			if !ok {
				return
			}
			// 에러 로깅 (현재는 무시)
			_ = err
		}
	}
}

// fsnotifyOpToType 는 fsnotify 작업을 문자열 이벤트 타입으로 변환합니다.
func fsnotifyOpToType(op fsnotify.Op) string {
	switch {
	case op&fsnotify.Create != 0:
		return "create"
	case op&fsnotify.Write != 0:
		return "modify"
	case op&fsnotify.Remove != 0, op&fsnotify.Rename != 0:
		return "delete"
	default:
		return ""
	}
}
