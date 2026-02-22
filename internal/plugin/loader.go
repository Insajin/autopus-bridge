package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/rs/zerolog"
)

// DefaultPluginsDir는 기본 플러그인 디렉토리 경로를 반환합니다.
// ~/.config/autopus/plugins/
func DefaultPluginsDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(homeDir, ".config", "autopus", "plugins")
}

// Loader는 플러그인 디렉토리에서 플러그인을 탐색하고 로드합니다.
type Loader struct {
	// pluginsDir는 플러그인 파일을 탐색할 디렉토리 경로입니다.
	pluginsDir string
	// plugins는 로드된 플러그인을 이름으로 인덱싱한 맵입니다.
	plugins map[string]Plugin
	// logger는 구조화된 로거입니다.
	logger zerolog.Logger
}

// LoaderOption은 Loader 설정 옵션입니다.
type LoaderOption func(*Loader)

// WithPluginsDir은 플러그인 디렉토리 경로를 설정합니다.
func WithPluginsDir(dir string) LoaderOption {
	return func(l *Loader) {
		l.pluginsDir = dir
	}
}

// WithLogger는 로거를 설정합니다.
func WithLogger(logger zerolog.Logger) LoaderOption {
	return func(l *Loader) {
		l.logger = logger
	}
}

// NewLoader는 새로운 플러그인 로더를 생성합니다.
func NewLoader(opts ...LoaderOption) *Loader {
	l := &Loader{
		pluginsDir: DefaultPluginsDir(),
		plugins:    make(map[string]Plugin),
		logger:     zerolog.Nop(),
	}

	for _, opt := range opts {
		opt(l)
	}

	return l
}

// pluginFileExtension은 현재 OS에 맞는 플러그인 파일 확장자를 반환합니다.
func pluginFileExtension() string {
	switch runtime.GOOS {
	case "windows":
		return ".dll"
	default:
		// Linux, macOS, FreeBSD 등
		return ".so"
	}
}

// DiscoverAndLoad는 플러그인 디렉토리에서 플러그인을 탐색하고 로드합니다.
// 로드된 플러그인을 반환하며, 개별 플러그인 로드 실패는 로그로 기록하고 계속 진행합니다.
func (l *Loader) DiscoverAndLoad() ([]Plugin, error) {
	// 플러그인 디렉토리 존재 확인
	if l.pluginsDir == "" {
		l.logger.Debug().Msg("플러그인 디렉토리가 설정되지 않았습니다")
		return nil, nil
	}

	info, err := os.Stat(l.pluginsDir)
	if os.IsNotExist(err) {
		l.logger.Debug().Str("dir", l.pluginsDir).Msg("플러그인 디렉토리가 존재하지 않습니다")
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("플러그인 디렉토리 확인 실패: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("플러그인 경로가 디렉토리가 아닙니다: %s", l.pluginsDir)
	}

	// 플러그인 파일 탐색
	ext := pluginFileExtension()
	pattern := filepath.Join(l.pluginsDir, "*"+ext)
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("플러그인 파일 탐색 실패: %w", err)
	}

	if len(matches) == 0 {
		l.logger.Debug().Str("dir", l.pluginsDir).Str("ext", ext).Msg("플러그인 파일을 찾을 수 없습니다")
		return nil, nil
	}

	l.logger.Info().Int("count", len(matches)).Str("dir", l.pluginsDir).Msg("플러그인 파일 발견")

	// 각 플러그인 파일 로드
	var loaded []Plugin
	for _, path := range matches {
		p, err := l.loadPlugin(path)
		if err != nil {
			l.logger.Warn().Err(err).Str("path", path).Msg("플러그인 로드 실패, 건너뜁니다")
			continue
		}

		// 중복 이름 검사
		if _, exists := l.plugins[p.Name()]; exists {
			l.logger.Warn().
				Str("name", p.Name()).
				Str("path", path).
				Msg("동일한 이름의 플러그인이 이미 로드되어 있습니다, 건너뜁니다")
			continue
		}

		l.plugins[p.Name()] = p
		loaded = append(loaded, p)

		l.logger.Info().
			Str("name", p.Name()).
			Str("version", p.Version()).
			Str("path", path).
			Msg("플러그인 로드 완료")
	}

	return loaded, nil
}

// LoadedPlugins는 로드된 모든 플러그인을 반환합니다.
func (l *Loader) LoadedPlugins() map[string]Plugin {
	return l.plugins
}
