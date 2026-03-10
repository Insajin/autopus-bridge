// Package authwatch는 AI 프로바이더 인증 파일 변경을 감지하고
// capabilities 업데이트를 트리거하는 파일 시스템 감시자를 제공합니다.
// SPEC-HOTSWAP-001: fsnotify 기반 실시간 인증 파일 감지 및 hot-swap 지원
package authwatch

import (
	"context"
	"maps"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/rs/zerolog"
)

// Provider는 authwatch가 감시하는 프로바이더 인터페이스입니다.
// provider.Provider의 서브셋으로, 순환 의존성을 방지합니다.
type Provider interface {
	// Name은 프로바이더 식별자를 반환합니다 (예: "claude", "gemini").
	Name() string
	// ValidateConfig는 현재 인증 설정의 유효성을 검사합니다.
	ValidateConfig() error
	// AuthFilePath는 이 프로바이더의 인증 파일 경로를 반환합니다.
	// 디렉토리 감시에 사용되며, 파일이 존재하지 않아도 됩니다.
	// 빈 문자열을 반환하면 디렉토리 감시를 건너뜁니다.
	AuthFilePath() string
}


// watchedFileNames는 변경 이벤트를 트리거하는 파일 이름 화이트리스트입니다.
// REQ-N-003: 화이트리스트 기반 필터링으로 불필요한 재빌드 방지
var watchedFileNames = map[string]bool{
	"credentials.json":                        true,
	"auth.json":                               true,
	".claude.json":                            true,
	"application_default_credentials.json":    true,
}

// defaultDebounce는 기본 디바운스 지연 시간입니다.
// REQ-E-001: 1초 디바운스로 연속 이벤트 병합
const defaultDebounce = 1 * time.Second

// Watcher는 프로바이더 인증 파일을 감시하고 변경 시 콜백을 호출합니다.
// @MX:ANCHOR: [AUTO] SPEC-HOTSWAP-001 핵심 감시자 - Start/Stop/rebuildCapabilities 3개 호출자
// @MX:REASON: authwatch 패키지의 공개 API 진입점으로 connect.go, 테스트에서 사용
type Watcher struct {
	fsWatcher    *fsnotify.Watcher
	providers    []Provider
	onChange     func(caps map[string]bool)
	debounce     time.Duration
	logger       zerolog.Logger
	stopCh       chan struct{}
	stopOnce     sync.Once
	watchedFiles map[string]bool // 화이트리스트 파일명
	extraDirs    []string        // 추가 감시 디렉토리 (WithWatchDirs 옵션으로 설정)

	// prevCaps는 마지막으로 전송된 capabilities입니다 (변경 감지용).
	prevCaps   map[string]bool
	prevCapsMu sync.Mutex

	// timer는 디바운스 타이머입니다.
	timer   *time.Timer
	timerMu sync.Mutex
}

// Option은 Watcher 생성 옵션입니다.
type Option func(*Watcher)

// WithLogger는 커스텀 로거를 설정합니다.
func WithLogger(logger zerolog.Logger) Option {
	return func(w *Watcher) {
		w.logger = logger
	}
}

// WithDebounce는 커스텀 디바운스 지연 시간을 설정합니다.
func WithDebounce(d time.Duration) Option {
	return func(w *Watcher) {
		w.debounce = d
	}
}

// WithWatchDirs는 추가 감시 디렉토리를 설정합니다.
// Provider.AuthFilePath()를 구현하지 않는 프로바이더를 위해 직접 디렉토리를 지정합니다.
func WithWatchDirs(dirs ...string) Option {
	return func(w *Watcher) {
		w.extraDirs = append(w.extraDirs, dirs...)
	}
}

// New는 새로운 Watcher를 생성합니다.
// providers에서 감시할 디렉토리를 추출하고, 존재하는 디렉토리만 감시합니다.
// REQ-E-005: 존재하지 않는 디렉토리는 조용히 건너뜁니다.
func New(providers []Provider, onChange func(caps map[string]bool), opts ...Option) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	w := &Watcher{
		fsWatcher:    fsw,
		providers:    providers,
		onChange:     onChange,
		debounce:     defaultDebounce,
		logger:       zerolog.Nop(),
		stopCh:       make(chan struct{}),
		watchedFiles: watchedFileNames,
		prevCaps:     make(map[string]bool),
	}

	for _, opt := range opts {
		opt(w)
	}

	// 감시할 디렉토리 추출 및 등록
	watchedDirs := make(map[string]bool)

	// Provider.AuthFilePath()에서 디렉토리 추출
	for _, p := range providers {
		authPath := p.AuthFilePath()
		if authPath == "" {
			continue
		}
		dir := filepath.Dir(authPath)
		addWatchDir(fsw, dir, p.Name(), watchedDirs, w.logger)
	}

	// WithWatchDirs()로 지정된 추가 디렉토리 등록
	for _, dir := range w.extraDirs {
		addWatchDir(fsw, dir, "extra", watchedDirs, w.logger)
	}

	return w, nil
}

// addWatchDir는 존재하는 디렉토리를 fsnotify 감시 목록에 추가합니다.
// REQ-E-005: 존재하지 않는 디렉토리는 조용히 건너뜁니다.
func addWatchDir(fsw *fsnotify.Watcher, dir, providerName string, watchedDirs map[string]bool, logger zerolog.Logger) {
	if watchedDirs[dir] {
		return
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		logger.Debug().
			Str("provider", providerName).
			Str("dir", dir).
			Msg("authwatch: 디렉토리가 존재하지 않아 건너뜁니다")
		return
	}
	if err := fsw.Add(dir); err != nil {
		logger.Warn().
			Err(err).
			Str("provider", providerName).
			Str("dir", dir).
			Msg("authwatch: 디렉토리 감시 등록 실패")
		return
	}
	watchedDirs[dir] = true
	logger.Debug().
		Str("provider", providerName).
		Str("dir", dir).
		Msg("authwatch: 디렉토리 감시 시작")
}

// Start는 백그라운드 이벤트 루프를 시작합니다.
// ctx가 취소되면 감시자를 종료합니다.
func (w *Watcher) Start(ctx context.Context) error {
	go w.eventLoop(ctx)
	return nil
}

// Stop은 감시자를 정리하고 종료합니다.
// 중복 호출은 안전합니다.
func (w *Watcher) Stop() {
	w.stopOnce.Do(func() {
		close(w.stopCh)
		_ = w.fsWatcher.Close()
	})
}

// eventLoop는 fsnotify 이벤트를 처리하는 메인 루프입니다.
// @MX:WARN: [AUTO] 백그라운드 고루틴 - stopCh와 ctx 두 종료 조건 사용
// @MX:REASON: 이중 종료 신호(ctx, stopCh)를 모두 처리해야 하는 복잡한 패턴
func (w *Watcher) eventLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		case event, ok := <-w.fsWatcher.Events:
			if !ok {
				return
			}
			w.handleEvent(event)
		case err, ok := <-w.fsWatcher.Errors:
			if !ok {
				return
			}
			w.logger.Warn().Err(err).Msg("authwatch: fsnotify 에러")
		}
	}
}

// handleEvent는 단일 fsnotify 이벤트를 처리합니다.
// 화이트리스트 파일만 처리하고, 디바운스 타이머를 리셋합니다.
func (w *Watcher) handleEvent(event fsnotify.Event) {
	// 파일명 추출
	fileName := filepath.Base(event.Name)

	// REQ-N-003: 화이트리스트에 없는 파일은 무시
	if !w.watchedFiles[fileName] {
		return
	}

	// 로그: 파일 경로만 기록 (파일 내용/인증 값은 절대 기록하지 않음)
	w.logger.Debug().
		Str("file", event.Name).
		Str("op", event.Op.String()).
		Msg("authwatch: 인증 파일 변경 감지")

	// REQ-E-001: 디바운스 - 타이머를 리셋하여 연속 이벤트를 병합
	w.timerMu.Lock()
	if w.timer != nil {
		w.timer.Stop()
	}
	w.timer = time.AfterFunc(w.debounce, w.fireCallback)
	w.timerMu.Unlock()
}

// fireCallback은 디바운스 타이머 만료 시 호출됩니다.
// capabilities를 재빌드하고, 변경이 있을 때만 onChange를 호출합니다.
// REQ-S-002: 동일한 capabilities면 전송하지 않음
func (w *Watcher) fireCallback() {
	newCaps := w.rebuildCapabilities()

	w.prevCapsMu.Lock()
	changed := !maps.Equal(w.prevCaps, newCaps)
	if changed {
		// 복사본 저장 (원본 맵 보호)
		copy := make(map[string]bool, len(newCaps))
		for k, v := range newCaps {
			copy[k] = v
		}
		w.prevCaps = copy
	}
	w.prevCapsMu.Unlock()

	if changed {
		w.logger.Info().
			Interface("capabilities", newCaps).
			Msg("authwatch: capabilities 변경 감지, 업데이트 전송")
		w.onChange(newCaps)
	}
}

// rebuildCapabilities는 모든 프로바이더를 재검증하여 현재 capabilities를 반환합니다.
// 일부 프로바이더가 실패해도 나머지는 정상 처리됩니다.
func (w *Watcher) rebuildCapabilities() map[string]bool {
	caps := make(map[string]bool, len(w.providers))
	for _, p := range w.providers {
		valid := p.ValidateConfig() == nil
		caps[p.Name()] = valid
		if !valid {
			w.logger.Debug().
				Str("provider", p.Name()).
				Msg("authwatch: 프로바이더 설정 검증 실패")
		}
	}
	return caps
}

