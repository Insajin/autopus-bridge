// Package provider는 AI 프로바이더 통합 레이어를 제공합니다.
package provider

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// Registry는 AI 프로바이더를 관리하는 레지스트리입니다.
// 스레드 안전하게 프로바이더를 등록하고 조회할 수 있습니다.
type Registry struct {
	// providers는 이름으로 인덱싱된 프로바이더 맵입니다.
	providers map[string]Provider
	// mu는 providers 맵 접근을 보호하는 뮤텍스입니다.
	mu sync.RWMutex
}

// NewRegistry는 새로운 프로바이더 레지스트리를 생성합니다.
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]Provider),
	}
}

// Register는 프로바이더를 레지스트리에 등록합니다.
// 동일한 이름의 프로바이더가 이미 있으면 덮어씁니다.
func (r *Registry) Register(provider Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[provider.Name()] = provider
}

// Get은 이름으로 프로바이더를 조회합니다.
// 프로바이더가 없으면 nil을 반환합니다.
func (r *Registry) Get(name string) Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.providers[name]
}

// GetForModel은 모델명에 맞는 프로바이더를 반환합니다.
// 모델명 패턴에 따라 적절한 프로바이더를 선택합니다.
func (r *Registry) GetForModel(model string) (Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// 모델명 기반 프로바이더 매핑
	// claude-* -> claude 프로바이더
	// gemini-* -> gemini 프로바이더
	if strings.HasPrefix(model, "claude-") {
		if p, ok := r.providers["claude"]; ok {
			return p, nil
		}
		return nil, fmt.Errorf("%w: claude 프로바이더가 등록되지 않았습니다", ErrProviderNotFound)
	}

	if strings.HasPrefix(model, "gemini-") {
		if p, ok := r.providers["gemini"]; ok {
			return p, nil
		}
		return nil, fmt.Errorf("%w: gemini 프로바이더가 등록되지 않았습니다", ErrProviderNotFound)
	}

	// Codex (OpenAI) 모델 매핑
	if strings.HasPrefix(model, "gpt-") || strings.HasPrefix(model, "o4-") {
		if p, ok := r.providers["codex"]; ok {
			return p, nil
		}
		return nil, fmt.Errorf("%w: codex 프로바이더가 등록되지 않았습니다", ErrProviderNotFound)
	}

	// 등록된 모든 프로바이더에서 지원 여부 확인
	for _, provider := range r.providers {
		if provider.Supports(model) {
			return provider, nil
		}
	}

	return nil, fmt.Errorf("%w: 모델 '%s'을 지원하는 프로바이더를 찾을 수 없습니다", ErrProviderNotFound, model)
}

// List는 등록된 모든 프로바이더 이름을 반환합니다.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}

// ListProviders는 등록된 모든 프로바이더를 반환합니다.
func (r *Registry) ListProviders() []Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	providers := make([]Provider, 0, len(r.providers))
	for _, provider := range r.providers {
		providers = append(providers, provider)
	}
	return providers
}

// Has는 특정 이름의 프로바이더가 등록되어 있는지 확인합니다.
func (r *Registry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.providers[name]
	return ok
}

// Count는 등록된 프로바이더 수를 반환합니다.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.providers)
}

// Remove는 프로바이더를 레지스트리에서 제거합니다.
func (r *Registry) Remove(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.providers, name)
}

// Clear는 모든 프로바이더를 레지스트리에서 제거합니다.
func (r *Registry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers = make(map[string]Provider)
}

// ValidateAll은 모든 등록된 프로바이더의 설정 유효성을 검사합니다.
func (r *Registry) ValidateAll() error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for name, provider := range r.providers {
		if err := provider.ValidateConfig(); err != nil {
			return fmt.Errorf("프로바이더 '%s' 설정 검증 실패: %w", name, err)
		}
	}
	return nil
}

// DefaultRegistry는 기본 전역 프로바이더 레지스트리입니다.
var DefaultRegistry = NewRegistry()

// RegistryConfig는 레지스트리 초기화 설정입니다.
type RegistryConfig struct {
	// ClaudeAPIKey는 Claude API 키입니다.
	ClaudeAPIKey string
	// ClaudeDefaultModel은 Claude 기본 모델입니다.
	ClaudeDefaultModel string
	// ClaudeMode는 Claude 프로바이더 모드입니다 ("api", "cli", "hybrid").
	// 기본값: "api"
	ClaudeMode string
	// ClaudeCLIPath는 claude CLI 바이너리 경로입니다.
	// 기본값: "claude"
	ClaudeCLIPath string
	// ClaudeCLITimeout은 CLI 실행 타임아웃(초)입니다.
	// 기본값: 300
	ClaudeCLITimeout int

	// GeminiAPIKey는 Gemini API 키입니다.
	GeminiAPIKey string
	// GeminiDefaultModel은 Gemini 기본 모델입니다.
	GeminiDefaultModel string
	// GeminiMode는 Gemini 프로바이더 모드입니다 ("api", "cli", "hybrid").
	GeminiMode string
	// GeminiCLIPath는 gemini CLI 바이너리 경로입니다.
	GeminiCLIPath string
	// GeminiCLITimeout은 Gemini CLI 실행 타임아웃(초)입니다.
	GeminiCLITimeout int
	// GeminiCLIFallbackCommand는 gemini CLI 폴백 명령어입니다.
	GeminiCLIFallbackCommand string

	// CodexAPIKey는 Codex (OpenAI) API 키입니다.
	CodexAPIKey string
	// CodexDefaultModel은 Codex 기본 모델입니다.
	CodexDefaultModel string
	// CodexMode는 Codex 프로바이더 모드입니다 ("api", "cli", "hybrid").
	CodexMode string
	// CodexCLIPath는 codex CLI 바이너리 경로입니다.
	CodexCLIPath string
	// CodexCLITimeout은 Codex CLI 실행 타임아웃(초)입니다.
	CodexCLITimeout int
	// CodexApprovalPolicy는 App Server 모드의 승인 정책입니다.
	CodexApprovalPolicy string
	// CodexChatGPTAuthEnv는 ChatGPT 인증 토큰 환경변수명입니다.
	CodexChatGPTAuthEnv string
}

// InitializeRegistry는 설정에 따라 프로바이더를 초기화하고 레지스트리에 등록합니다.
// REQ-S-05: 설정된 AI 프로바이더가 없으면 에러 반환
func InitializeRegistry(ctx context.Context, cfg RegistryConfig) (*Registry, error) {
	return InitializeRegistryWithLogger(ctx, cfg, zerolog.Nop())
}

// InitializeRegistryWithLogger는 로거와 함께 레지스트리를 초기화합니다.
func InitializeRegistryWithLogger(ctx context.Context, cfg RegistryConfig, logger zerolog.Logger) (*Registry, error) {
	registry := NewRegistry()
	providerCount := 0

	// Claude 프로바이더 초기화 (모드에 따라 분기)
	claudeProvider, err := initializeClaudeProvider(cfg, logger)
	if err != nil {
		// 에러가 발생해도 다른 프로바이더 시도
		logger.Warn().Err(err).Msg("Claude 프로바이더 초기화 실패")
	} else if claudeProvider != nil {
		registry.Register(claudeProvider)
		providerCount++
	}

	// Gemini 프로바이더 초기화 (모드에 따라 분기)
	geminiProvider, err := initializeGeminiProvider(ctx, cfg, logger)
	if err != nil {
		logger.Warn().Err(err).Msg("Gemini 프로바이더 초기화 실패")
	} else if geminiProvider != nil {
		registry.Register(geminiProvider)
		providerCount++
	}

	// Codex 프로바이더 초기화 (모드에 따라 분기)
	codexProvider, err := initializeCodexProvider(cfg, logger)
	if err != nil {
		logger.Warn().Err(err).Msg("Codex 프로바이더 초기화 실패")
	} else if codexProvider != nil {
		registry.Register(codexProvider)
		providerCount++
	}

	// REQ-S-05: 설정된 AI 프로바이더가 없으면 에러
	if providerCount == 0 {
		return nil, fmt.Errorf("AI 프로바이더가 설정되지 않았습니다. ANTHROPIC_API_KEY, GEMINI_API_KEY, 또는 OPENAI_API_KEY 환경변수를 설정하거나, claude/gemini/codex CLI를 설치하세요")
	}

	return registry, nil
}

// initializeClaudeProvider는 설정된 모드에 따라 Claude 프로바이더를 초기화합니다.
func initializeClaudeProvider(cfg RegistryConfig, logger zerolog.Logger) (Provider, error) {
	// 모드 기본값 설정
	mode := cfg.ClaudeMode
	if mode == "" {
		mode = "api"
	}

	// CLI 경로 기본값
	cliPath := cfg.ClaudeCLIPath
	if cliPath == "" {
		cliPath = "claude"
	}

	// CLI 타임아웃 기본값 (300초 = 5분)
	cliTimeout := cfg.ClaudeCLITimeout
	if cliTimeout <= 0 {
		cliTimeout = 300
	}

	switch mode {
	case "api":
		// API 모드: API 키 필수
		if cfg.ClaudeAPIKey == "" {
			return nil, nil // API 키가 없으면 건너뜀
		}
		return initializeClaudeAPIProvider(cfg)

	case "cli":
		// CLI 모드: claude CLI 필수
		if !IsCLIAvailable(cliPath) {
			return nil, fmt.Errorf("%w: %s", ErrCLINotFound, cliPath)
		}
		return initializeClaudeCLIProvider(cfg, cliPath, cliTimeout)

	case "hybrid":
		// 하이브리드 모드: CLI 우선, API 폴백
		return initializeClaudeHybridProvider(cfg, cliPath, cliTimeout, logger)

	default:
		return nil, fmt.Errorf("지원하지 않는 Claude 모드: %s (api, cli, hybrid 중 하나)", mode)
	}
}

// initializeClaudeAPIProvider는 API 기반 Claude 프로바이더를 초기화합니다.
func initializeClaudeAPIProvider(cfg RegistryConfig) (*ClaudeProvider, error) {
	opts := []ClaudeProviderOption{
		WithClaudeAPIKey(cfg.ClaudeAPIKey),
	}
	if cfg.ClaudeDefaultModel != "" {
		opts = append(opts, WithClaudeDefaultModel(cfg.ClaudeDefaultModel))
	}

	return NewClaudeProvider(opts...)
}

// initializeClaudeCLIProvider는 CLI 기반 Claude 프로바이더를 초기화합니다.
func initializeClaudeCLIProvider(cfg RegistryConfig, cliPath string, cliTimeout int) (*ClaudeCLIProvider, error) {
	opts := []ClaudeCLIProviderOption{
		WithCLIPath(cliPath),
		WithCLITimeout(time.Duration(cliTimeout) * time.Second),
	}
	if cfg.ClaudeDefaultModel != "" {
		opts = append(opts, WithCLIDefaultModel(cfg.ClaudeDefaultModel))
	}

	return NewClaudeCLIProvider(opts...)
}

// initializeClaudeHybridProvider는 하이브리드 Claude 프로바이더를 초기화합니다.
func initializeClaudeHybridProvider(cfg RegistryConfig, cliPath string, cliTimeout int, logger zerolog.Logger) (*HybridClaudeProvider, error) {
	// CLI 옵션 구성
	cliOpts := []ClaudeCLIProviderOption{
		WithCLIPath(cliPath),
		WithCLITimeout(time.Duration(cliTimeout) * time.Second),
	}
	if cfg.ClaudeDefaultModel != "" {
		cliOpts = append(cliOpts, WithCLIDefaultModel(cfg.ClaudeDefaultModel))
	}

	// API 옵션 구성 (API 키가 있는 경우에만)
	var apiOpts []ClaudeProviderOption
	if cfg.ClaudeAPIKey != "" {
		apiOpts = []ClaudeProviderOption{
			WithClaudeAPIKey(cfg.ClaudeAPIKey),
		}
		if cfg.ClaudeDefaultModel != "" {
			apiOpts = append(apiOpts, WithClaudeDefaultModel(cfg.ClaudeDefaultModel))
		}
	}

	// 하이브리드 프로바이더 생성
	return NewHybridClaudeProvider(
		cliOpts,
		apiOpts,
		WithHybridLogger(logger),
	)
}

// GetForTask resolves a provider using explicit provider name first,
// falling back to model-based resolution.
//
//  1. Explicit provider name (from agent's provider field)
//  2. Model name prefix matching (claude-*, gemini-*)
//  3. Provider.Supports() check
func (r *Registry) GetForTask(providerName, model string) (Provider, error) {
	// 1. Explicit provider name takes priority.
	if providerName != "" {
		if p := r.Get(providerName); p != nil {
			return p, nil
		}
	}

	// 2. Fall back to model-based resolution.
	return r.GetForModel(model)
}

// initializeGeminiProvider는 설정된 모드에 따라 Gemini 프로바이더를 초기화합니다.
func initializeGeminiProvider(ctx context.Context, cfg RegistryConfig, logger zerolog.Logger) (Provider, error) {
	mode := cfg.GeminiMode
	if mode == "" {
		mode = "api"
	}

	cliPath := cfg.GeminiCLIPath
	if cliPath == "" {
		cliPath = "gemini"
	}

	cliTimeout := cfg.GeminiCLITimeout
	if cliTimeout <= 0 {
		cliTimeout = 300
	}

	fallbackCmd := cfg.GeminiCLIFallbackCommand
	if fallbackCmd == "" {
		fallbackCmd = "npx @google/gemini-cli"
	}

	switch mode {
	case "api":
		if cfg.GeminiAPIKey == "" {
			return nil, nil
		}
		return initializeGeminiAPIProvider(ctx, cfg)

	case "cli":
		if !IsGeminiCLIAvailable(cliPath, fallbackCmd) {
			return nil, fmt.Errorf("%w: %s", ErrCLINotFound, cliPath)
		}
		return initializeGeminiCLIProvider(cfg, cliPath, cliTimeout, fallbackCmd)

	case "hybrid":
		return initializeGeminiHybridProvider(ctx, cfg, cliPath, cliTimeout, fallbackCmd, logger)

	default:
		return nil, fmt.Errorf("지원하지 않는 Gemini 모드: %s (api, cli, hybrid 중 하나)", mode)
	}
}

// initializeGeminiAPIProvider는 API 기반 Gemini 프로바이더를 초기화합니다.
func initializeGeminiAPIProvider(ctx context.Context, cfg RegistryConfig) (*GeminiProvider, error) {
	opts := []GeminiProviderOption{
		WithGeminiAPIKey(cfg.GeminiAPIKey),
	}
	if cfg.GeminiDefaultModel != "" {
		opts = append(opts, WithGeminiDefaultModel(cfg.GeminiDefaultModel))
	}
	return NewGeminiProvider(ctx, opts...)
}

// initializeGeminiCLIProvider는 CLI 기반 Gemini 프로바이더를 초기화합니다.
func initializeGeminiCLIProvider(cfg RegistryConfig, cliPath string, cliTimeout int, fallbackCmd string) (*GeminiCLIProvider, error) {
	opts := []GeminiCLIProviderOption{
		WithGeminiCLIPath(cliPath),
		WithGeminiCLITimeout(time.Duration(cliTimeout) * time.Second),
		WithGeminiCLIFallbackCommand(fallbackCmd),
	}
	if cfg.GeminiDefaultModel != "" {
		opts = append(opts, WithGeminiCLIDefaultModel(cfg.GeminiDefaultModel))
	}
	return NewGeminiCLIProvider(opts...)
}

// initializeGeminiHybridProvider는 하이브리드 Gemini 프로바이더를 초기화합니다.
func initializeGeminiHybridProvider(ctx context.Context, cfg RegistryConfig, cliPath string, cliTimeout int, fallbackCmd string, logger zerolog.Logger) (*HybridGeminiProvider, error) {
	cliOpts := []GeminiCLIProviderOption{
		WithGeminiCLIPath(cliPath),
		WithGeminiCLITimeout(time.Duration(cliTimeout) * time.Second),
		WithGeminiCLIFallbackCommand(fallbackCmd),
	}
	if cfg.GeminiDefaultModel != "" {
		cliOpts = append(cliOpts, WithGeminiCLIDefaultModel(cfg.GeminiDefaultModel))
	}

	var apiOpts []GeminiProviderOption
	if cfg.GeminiAPIKey != "" {
		apiOpts = []GeminiProviderOption{
			WithGeminiAPIKey(cfg.GeminiAPIKey),
		}
		if cfg.GeminiDefaultModel != "" {
			apiOpts = append(apiOpts, WithGeminiDefaultModel(cfg.GeminiDefaultModel))
		}
	}

	return NewHybridGeminiProvider(
		ctx,
		cliOpts,
		apiOpts,
		WithGeminiHybridLogger(logger),
	)
}

// initializeCodexProvider는 설정된 모드에 따라 Codex 프로바이더를 초기화합니다.
func initializeCodexProvider(cfg RegistryConfig, logger zerolog.Logger) (Provider, error) {
	mode := cfg.CodexMode
	if mode == "" {
		mode = "api"
	}

	cliPath := cfg.CodexCLIPath
	if cliPath == "" {
		cliPath = "codex"
	}

	cliTimeout := cfg.CodexCLITimeout
	if cliTimeout <= 0 {
		cliTimeout = 300
	}

	switch mode {
	case "api":
		if cfg.CodexAPIKey == "" {
			return nil, nil
		}
		return initializeCodexAPIProvider(cfg)

	case "cli":
		if !IsCodexCLIAvailable(cliPath) {
			return nil, fmt.Errorf("%w: %s", ErrCLINotFound, cliPath)
		}
		return initializeCodexCLIProvider(cfg, cliPath, cliTimeout)

	case "hybrid":
		return initializeCodexHybridProvider(cfg, cliPath, cliTimeout, logger)

	case "app-server":
		return initializeCodexAppServerProvider(cfg, cliPath, logger)

	default:
		return nil, fmt.Errorf("지원하지 않는 Codex 모드: %s (api, cli, hybrid, app-server 중 하나)", mode)
	}
}

// initializeCodexAPIProvider는 API 기반 Codex 프로바이더를 초기화합니다.
func initializeCodexAPIProvider(cfg RegistryConfig) (*CodexProvider, error) {
	opts := []CodexProviderOption{
		WithCodexAPIKey(cfg.CodexAPIKey),
	}
	if cfg.CodexDefaultModel != "" {
		opts = append(opts, WithCodexDefaultModel(cfg.CodexDefaultModel))
	}
	return NewCodexProvider(opts...)
}

// initializeCodexCLIProvider는 CLI 기반 Codex 프로바이더를 초기화합니다.
func initializeCodexCLIProvider(cfg RegistryConfig, cliPath string, cliTimeout int) (*CodexCLIProvider, error) {
	opts := []CodexCLIProviderOption{
		WithCodexCLIPath(cliPath),
		WithCodexCLITimeout(time.Duration(cliTimeout) * time.Second),
	}
	if cfg.CodexDefaultModel != "" {
		opts = append(opts, WithCodexCLIDefaultModel(cfg.CodexDefaultModel))
	}
	return NewCodexCLIProvider(opts...)
}

// initializeCodexHybridProvider는 하이브리드 Codex 프로바이더를 초기화합니다.
func initializeCodexHybridProvider(cfg RegistryConfig, cliPath string, cliTimeout int, logger zerolog.Logger) (*HybridCodexProvider, error) {
	cliOpts := []CodexCLIProviderOption{
		WithCodexCLIPath(cliPath),
		WithCodexCLITimeout(time.Duration(cliTimeout) * time.Second),
	}
	if cfg.CodexDefaultModel != "" {
		cliOpts = append(cliOpts, WithCodexCLIDefaultModel(cfg.CodexDefaultModel))
	}

	var apiOpts []CodexProviderOption
	if cfg.CodexAPIKey != "" {
		apiOpts = []CodexProviderOption{
			WithCodexAPIKey(cfg.CodexAPIKey),
		}
		if cfg.CodexDefaultModel != "" {
			apiOpts = append(apiOpts, WithCodexDefaultModel(cfg.CodexDefaultModel))
		}
	}

	return NewHybridCodexProvider(
		cliOpts,
		apiOpts,
		WithCodexHybridLogger(logger),
	)
}

// initializeCodexAppServerProvider는 App Server 기반 Codex 프로바이더를 초기화합니다.
func initializeCodexAppServerProvider(cfg RegistryConfig, cliPath string, logger zerolog.Logger) (*CodexAppServerProvider, error) {
	// 인증 방법 및 키 결정
	// 우선순위: API 키 우선, ChatGPT 인증 차순
	authMethod := ""
	authKey := ""

	if cfg.CodexAPIKey != "" {
		authMethod = "apiKey"
		authKey = cfg.CodexAPIKey
	} else if cfg.CodexChatGPTAuthEnv != "" {
		chatGPTKey := os.Getenv(cfg.CodexChatGPTAuthEnv)
		if chatGPTKey != "" {
			authMethod = "chatgptAuthTokens"
			authKey = chatGPTKey
		}
	}

	if authKey == "" {
		return nil, fmt.Errorf("%w: app-server 모드에는 API 키 또는 ChatGPT 인증이 필요합니다", ErrNoAPIKey)
	}

	approvalPolicy := cfg.CodexApprovalPolicy
	if approvalPolicy == "" {
		approvalPolicy = "auto-approve"
	}

	opts := []CodexAppServerOption{
		WithAppServerLogger(logger),
		WithAppServerApprovalPolicy(approvalPolicy),
		WithAppServerAuth(authMethod, authKey),
	}

	if cfg.CodexDefaultModel != "" {
		// Note: model is set per-request, not at provider level for app-server
	}

	return NewCodexAppServerProvider(cliPath, opts...)
}

// Execute는 모델에 맞는 프로바이더를 찾아 실행합니다.
// 편의 메서드로, GetForModel + Execute를 한 번에 수행합니다.
func (r *Registry) Execute(ctx context.Context, req ExecuteRequest) (*ExecuteResponse, error) {
	provider, err := r.GetForModel(req.Model)
	if err != nil {
		return nil, err
	}
	return provider.Execute(ctx, req)
}
