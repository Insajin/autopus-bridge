//go:build !(linux || darwin || freebsd)

package plugin

import (
	"fmt"
)

// loadPlugin은 지원되지 않는 플랫폼에서 경고를 로그에 기록하고 에러를 반환합니다.
// Go의 plugin 패키지는 Linux, macOS, FreeBSD에서만 동작합니다.
func (l *Loader) loadPlugin(path string) (Plugin, error) {
	l.logger.Warn().
		Str("path", path).
		Msg("현재 플랫폼에서는 Go 플러그인 로딩을 지원하지 않습니다")
	return nil, fmt.Errorf("%w: cannot load %s", ErrPluginsUnsupported, path)
}
