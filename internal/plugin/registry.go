package plugin

import (
	"fmt"
	"sync"
)

// Registry는 로드된 모든 플러그인을 유형별로 관리합니다.
// 스레드 안전하게 플러그인을 등록하고 조회할 수 있습니다.
type Registry struct {
	// providers는 이름으로 인덱싱된 프로바이더 플러그인 맵입니다.
	providers map[string]ProviderPlugin
	// taskTypes는 메시지 유형으로 인덱싱된 작업 유형 플러그인 맵입니다.
	taskTypes map[string]TaskTypePlugin
	// postProcessors는 등록된 후처리 플러그인 슬라이스입니다.
	postProcessors []PostProcessorPlugin
	// allPlugins는 모든 등록된 플러그인을 이름으로 인덱싱한 맵입니다.
	allPlugins map[string]Plugin
	// mu는 모든 맵 접근을 보호하는 뮤텍스입니다.
	mu sync.RWMutex
}

// NewRegistry는 새로운 플러그인 레지스트리를 생성합니다.
func NewRegistry() *Registry {
	return &Registry{
		providers:      make(map[string]ProviderPlugin),
		taskTypes:      make(map[string]TaskTypePlugin),
		postProcessors: make([]PostProcessorPlugin, 0),
		allPlugins:     make(map[string]Plugin),
	}
}

// Register는 플러그인을 레지스트리에 등록합니다.
// 플러그인이 구현하는 인터페이스에 따라 자동으로 분류됩니다.
// 동일한 이름의 플러그인이 이미 있으면 에러를 반환합니다.
func (r *Registry) Register(p Plugin) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := p.Name()

	// 중복 이름 검사
	if _, exists := r.allPlugins[name]; exists {
		return fmt.Errorf("%w: %s", ErrDuplicatePlugin, name)
	}

	// 기본 등록
	r.allPlugins[name] = p

	// 인터페이스에 따라 분류 등록
	if pp, ok := p.(ProviderPlugin); ok {
		r.providers[name] = pp
	}
	if tp, ok := p.(TaskTypePlugin); ok {
		r.taskTypes[tp.TaskType()] = tp
	}
	if pp, ok := p.(PostProcessorPlugin); ok {
		r.postProcessors = append(r.postProcessors, pp)
	}

	return nil
}

// GetProvider는 이름으로 프로바이더 플러그인을 조회합니다.
func (r *Registry) GetProvider(name string) (ProviderPlugin, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[name]
	return p, ok
}

// GetTaskHandler는 메시지 유형으로 작업 유형 플러그인을 조회합니다.
func (r *Registry) GetTaskHandler(msgType string) (TaskTypePlugin, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	h, ok := r.taskTypes[msgType]
	return h, ok
}

// GetPostProcessors는 주어진 작업 유형에 적용되는 후처리 플러그인 목록을 반환합니다.
func (r *Registry) GetPostProcessors(taskType string) []PostProcessorPlugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []PostProcessorPlugin
	for _, pp := range r.postProcessors {
		if pp.Applies(taskType) {
			result = append(result, pp)
		}
	}
	return result
}

// ListPlugins는 등록된 모든 플러그인의 정보를 반환합니다.
func (r *Registry) ListPlugins() []PluginInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	infos := make([]PluginInfo, 0, len(r.allPlugins))
	for _, p := range r.allPlugins {
		info := PluginInfo{
			Name:    p.Name(),
			Version: p.Version(),
			Type:    classifyPlugin(p),
		}
		infos = append(infos, info)
	}
	return infos
}

// Count는 등록된 플러그인 수를 반환합니다.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.allPlugins)
}

// Close는 모든 등록된 플러그인을 닫고 리소스를 정리합니다.
func (r *Registry) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var lastErr error
	for name, p := range r.allPlugins {
		if err := p.Close(); err != nil {
			lastErr = fmt.Errorf("plugin '%s' close failed: %w", name, err)
		}
	}

	// 모든 맵 초기화
	r.providers = make(map[string]ProviderPlugin)
	r.taskTypes = make(map[string]TaskTypePlugin)
	r.postProcessors = nil
	r.allPlugins = make(map[string]Plugin)

	return lastErr
}

// classifyPlugin은 플러그인이 구현하는 인터페이스에 따라 유형 문자열을 반환합니다.
func classifyPlugin(p Plugin) string {
	switch {
	case isProviderPlugin(p):
		return "provider"
	case isTaskTypePlugin(p):
		return "task_type"
	case isPostProcessorPlugin(p):
		return "post_processor"
	default:
		return "base"
	}
}

// isProviderPlugin은 플러그인이 ProviderPlugin 인터페이스를 구현하는지 확인합니다.
func isProviderPlugin(p Plugin) bool {
	_, ok := p.(ProviderPlugin)
	return ok
}

// isTaskTypePlugin은 플러그인이 TaskTypePlugin 인터페이스를 구현하는지 확인합니다.
func isTaskTypePlugin(p Plugin) bool {
	_, ok := p.(TaskTypePlugin)
	return ok
}

// isPostProcessorPlugin은 플러그인이 PostProcessorPlugin 인터페이스를 구현하는지 확인합니다.
func isPostProcessorPlugin(p Plugin) bool {
	_, ok := p.(PostProcessorPlugin)
	return ok
}
