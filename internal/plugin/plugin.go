// Package plugin은 Local Agent Bridge의 플러그인 시스템을 제공합니다.
// FR-P5-01: 플러그인 인터페이스 정의
// FR-P5-03: 플러그인 로딩 및 관리
package plugin

import (
	"context"
	"errors"

	"github.com/insajin/autopus-agent-protocol"
)

// 플러그인 관련 에러 정의
var (
	// ErrPluginNotFound는 플러그인을 찾을 수 없을 때 반환됩니다.
	ErrPluginNotFound = errors.New("plugin not found")

	// ErrDuplicatePlugin은 동일한 이름의 플러그인이 이미 등록되어 있을 때 반환됩니다.
	ErrDuplicatePlugin = errors.New("plugin with the same name already registered")

	// ErrInvalidPlugin은 플러그인이 올바른 인터페이스를 구현하지 않을 때 반환됩니다.
	ErrInvalidPlugin = errors.New("loaded symbol does not implement Plugin interface")

	// ErrPluginSymbolNotFound는 플러그인에서 필요한 심볼을 찾을 수 없을 때 반환됩니다.
	ErrPluginSymbolNotFound = errors.New("required symbol not found in plugin")

	// ErrPluginsUnsupported는 현재 플랫폼에서 플러그인 로딩을 지원하지 않을 때 반환됩니다.
	ErrPluginsUnsupported = errors.New("go plugin loading is not supported on this platform")
)

// Plugin은 모든 플러그인이 구현해야 하는 기본 인터페이스입니다.
type Plugin interface {
	// Name은 플러그인의 고유 이름을 반환합니다.
	Name() string
	// Version은 플러그인의 시맨틱 버전을 반환합니다.
	Version() string
	// Init은 주어진 설정으로 플러그인을 초기화합니다.
	Init(config map[string]interface{}) error
	// Close는 플러그인 리소스를 정리합니다.
	Close() error
}

// ProviderPlugin은 AI 프로바이더 기능을 제공하는 플러그인 인터페이스입니다.
type ProviderPlugin interface {
	Plugin
	// Execute는 이 프로바이더를 사용하여 작업을 실행합니다.
	Execute(ctx context.Context, req ProviderRequest) (*ProviderResponse, error)
	// Supports는 이 프로바이더가 주어진 모델을 지원하는지 반환합니다.
	Supports(model string) bool
}

// TaskTypePlugin은 사용자 정의 작업 유형을 처리하는 플러그인 인터페이스입니다.
type TaskTypePlugin interface {
	Plugin
	// TaskType은 이 플러그인이 처리하는 메시지 유형을 반환합니다 (예: "deploy_request").
	TaskType() string
	// HandleTask는 사용자 정의 작업을 처리하고 결과를 반환합니다.
	HandleTask(ctx context.Context, payload []byte) ([]byte, error)
}

// PostProcessorPlugin은 결과 후처리를 위한 플러그인 인터페이스입니다.
type PostProcessorPlugin interface {
	Plugin
	// Process는 서버로 전송하기 전에 작업 결과를 후처리합니다.
	Process(ctx context.Context, result *ws.TaskResultPayload) (*ws.TaskResultPayload, error)
	// Applies는 이 프로세서가 주어진 작업 유형에 적용되어야 하는지 반환합니다.
	Applies(taskType string) bool
}

// ProviderRequest는 프로바이더 플러그인에 대한 입력입니다.
type ProviderRequest struct {
	// Prompt는 AI에게 전달할 프롬프트입니다.
	Prompt string
	// Model은 사용할 모델명입니다.
	Model string
	// MaxTokens는 생성할 최대 토큰 수입니다.
	MaxTokens int
	// WorkDir는 작업 디렉토리입니다.
	WorkDir string
}

// ProviderResponse는 프로바이더 플러그인의 출력입니다.
type ProviderResponse struct {
	// Output은 AI의 응답 텍스트입니다.
	Output string
	// DurationMs는 실행 시간(밀리초)입니다.
	DurationMs int64
}

// PluginInfo는 플러그인 메타데이터를 제공합니다.
type PluginInfo struct {
	// Name은 플러그인 이름입니다.
	Name string
	// Version은 플러그인 버전입니다.
	Version string
	// Type은 플러그인 유형입니다 ("provider", "task_type", "post_processor", "base").
	Type string
}

// pluginExportSymbol은 플러그인 공유 객체에서 찾을 심볼 이름입니다.
// 모든 플러그인 .so 파일은 이 이름의 변수를 export해야 합니다.
const pluginExportSymbol = "PluginInstance"
