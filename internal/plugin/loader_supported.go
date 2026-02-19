//go:build linux || darwin || freebsd

package plugin

import (
	"fmt"
	"plugin"
)

// loadPlugin은 지정된 경로에서 Go 공유 객체(.so)를 로드하고
// Plugin 인터페이스를 구현하는 인스턴스를 반환합니다.
func (l *Loader) loadPlugin(path string) (Plugin, error) {
	// Go plugin 패키지를 사용하여 공유 객체 로드
	raw, err := plugin.Open(path)
	if err != nil {
		return nil, fmt.Errorf("공유 객체 로드 실패 (%s): %w", path, err)
	}

	// 플러그인에서 export된 심볼 탐색
	sym, err := raw.Lookup(pluginExportSymbol)
	if err != nil {
		return nil, fmt.Errorf("%w: %s in %s", ErrPluginSymbolNotFound, pluginExportSymbol, path)
	}

	// 심볼이 Plugin 인터페이스를 구현하는지 확인
	p, ok := sym.(Plugin)
	if !ok {
		// 포인터일 수 있으므로 *Plugin도 확인
		pp, ok := sym.(*Plugin)
		if !ok {
			return nil, fmt.Errorf("%w: symbol '%s' in %s", ErrInvalidPlugin, pluginExportSymbol, path)
		}
		p = *pp
	}

	return p, nil
}
