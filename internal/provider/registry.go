// Package provider는 AI 프로바이더 통합 레이어를 제공합니다.
package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/insajin/autopus-bridge/internal/approval"
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
// OpenRouter 형식(provider/model)과 레거시 형식 모두 지원합니다.
func (r *Registry) GetForModel(model string) (Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// 1. OpenRouter 형식 우선 확인 (예: "openai/o3-mini", "anthropic/claude-sonnet-4-6")
	if IsOpenRouterFormat(model) {
		prefix, _ := ParseOpenRouterID(model)
		providerName := ResolveProviderName(prefix)
		if providerName != "" {
			if p, ok := r.providers[providerName]; ok {
				return p, nil
			}
			return nil, fmt.Errorf("%w: %s 프로바이더가 등록되지 않았습니다", ErrProviderNotFound, providerName)
		}
	}

	// 2. 레거시 형식 폴백 (접두사 기반 매핑)
	// claude-* -> claude 프로바이더
	if strings.HasPrefix(model, "claude-") {
		if p, ok := r.providers["claude"]; ok {
			return p, nil
		}
		return nil, fmt.Errorf("%w: claude 프로바이더가 등록되지 않았습니다", ErrProviderNotFound)
	}

	// gemini-* -> gemini 프로바이더
	if strings.HasPrefix(model, "gemini-") {
		if p, ok := r.providers["gemini"]; ok {
			return p, nil
		}
		return nil, fmt.Errorf("%w: gemini 프로바이더가 등록되지 않았습니다", ErrProviderNotFound)
	}

	// gpt-*, o4-*, o3-* -> codex 프로바이더
	if strings.HasPrefix(model, "gpt-") || strings.HasPrefix(model, "o4-") || strings.HasPrefix(model, "o3-") {
		if p, ok := r.providers["codex"]; ok {
			return p, nil
		}
		return nil, fmt.Errorf("%w: codex 프로바이더가 등록되지 않았습니다", ErrProviderNotFound)
	}

	// 3. 등록된 모든 프로바이더에서 지원 여부 확인
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

// isProviderEnabled는 프로바이더 활성화 여부를 확인합니다.
// nil이면 기본값 true를 반환합니다.
func isProviderEnabled(enabled *bool) bool {
	if enabled == nil {
		return true
	}
	return *enabled
}

// GetRegisteredProviderNames는 등록된 프로바이더 이름 목록을 반환합니다.
func (r *Registry) GetRegisteredProviderNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}

// RegistryConfig는 레지스트리 초기화 설정입니다.
type RegistryConfig struct {
	// ClaudeEnabled는 Claude 프로바이더 활성화 여부입니다. nil이면 기본값 true입니다.
	ClaudeEnabled *bool
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
	// ClaudeExecutionMode는 Claude 실행 모드입니다 ("auto-execute" | "interactive").
	// "interactive"일 때 PTY + 훅 서버 기반의 인터랙티브 프로바이더를 사용합니다.
	// 기본값: "auto-execute"
	ClaudeExecutionMode string
	// ClaudeApprovalPolicy는 인터랙티브 모드의 승인 정책입니다.
	// "auto-execute", "auto-approve", "agent-approve", "human-approve" 중 하나.
	// 기본값: "auto-approve"
	ClaudeApprovalPolicy string
	// ClaudeHookServerPort는 인터랙티브 모드의 훅 서버 포트입니다 (0 = 자동 할당).
	ClaudeHookServerPort int
	// ClaudeApprovalTimeout은 인터랙티브 모드의 승인 타임아웃(초)입니다.
	// 기본값: 300
	ClaudeApprovalTimeout int

	// GeminiEnabled는 Gemini 프로바이더 활성화 여부입니다. nil이면 기본값 true입니다.
	GeminiEnabled *bool
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

	// CodexEnabled는 Codex 프로바이더 활성화 여부입니다. nil이면 기본값 true입니다.
	CodexEnabled *bool
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
	if !isProviderEnabled(cfg.ClaudeEnabled) {
		logger.Info().Msg("Claude 프로바이더 비활성화됨 (config.enabled=false)")
	} else {
		claudeProvider, err := initializeClaudeProvider(cfg, logger)
		if err != nil {
			// 에러가 발생해도 다른 프로바이더 시도
			logger.Warn().Err(err).Msg("Claude 프로바이더 초기화 실패")
		} else if claudeProvider != nil {
			registry.Register(claudeProvider)
			providerCount++
		}
	}

	// Gemini 프로바이더 초기화 (모드에 따라 분기)
	if !isProviderEnabled(cfg.GeminiEnabled) {
		logger.Info().Msg("Gemini 프로바이더 비활성화됨 (config.enabled=false)")
	} else {
		geminiProvider, err := initializeGeminiProvider(ctx, cfg, logger)
		if err != nil {
			logger.Warn().Err(err).Msg("Gemini 프로바이더 초기화 실패")
		} else if geminiProvider != nil {
			registry.Register(geminiProvider)
			providerCount++
		}
	}

	// Codex 프로바이더 초기화 (모드에 따라 분기)
	if !isProviderEnabled(cfg.CodexEnabled) {
		logger.Info().Msg("Codex 프로바이더 비활성화됨 (config.enabled=false)")
	} else {
		codexProvider, err := initializeCodexProvider(cfg, logger)
		if err != nil {
			logger.Warn().Err(err).Msg("Codex 프로바이더 초기화 실패")
		} else if codexProvider != nil {
			registry.Register(codexProvider)
			providerCount++
		}
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

	// 인터랙티브 모드 확인: execution_mode가 "interactive"이고 CLI 모드일 때
	executionMode := cfg.ClaudeExecutionMode
	if executionMode == "" {
		executionMode = "auto-execute"
	}

	switch mode {
	case "api":
		// API 모드: API 키 필수 (인터랙티브 모드 해당 없음)
		if cfg.ClaudeAPIKey == "" {
			return nil, nil // API 키가 없으면 건너뜀
		}
		return initializeClaudeAPIProvider(cfg)

	case "cli":
		// CLI 모드: claude CLI 필수
		if !IsCLIAvailable(cliPath) {
			return nil, fmt.Errorf("%w: %s", ErrCLINotFound, cliPath)
		}
		// 인터랙티브 모드일 경우 InteractiveClaudeCLIProvider 사용
		if executionMode == "interactive" {
			return initializeClaudeInteractiveProvider(cfg, cliPath, cliTimeout)
		}
		return initializeClaudeCLIProvider(cfg, cliPath, cliTimeout)

	case "hybrid":
		// 하이브리드 모드: 인터랙티브 모드일 경우 InteractiveClaudeCLIProvider 사용
		if executionMode == "interactive" && IsCLIAvailable(cliPath) {
			return initializeClaudeInteractiveProvider(cfg, cliPath, cliTimeout)
		}
		// 일반 하이브리드 모드: CLI 우선, API 폴백
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

// initializeClaudeInteractiveProvider는 인터랙티브 모드의 Claude 프로바이더를 초기화합니다.
// PTY + 훅 서버를 사용하여 Claude Code를 인터랙티브 모드로 실행합니다.
func initializeClaudeInteractiveProvider(cfg RegistryConfig, cliPath string, cliTimeout int) (*InteractiveClaudeCLIProvider, error) {
	// 승인 정책 파싱
	policyStr := cfg.ClaudeApprovalPolicy
	if policyStr == "" {
		policyStr = "auto-approve"
	}
	policy := approval.ApprovalPolicy(policyStr)

	// 승인 타임아웃 설정
	approvalTimeout := cfg.ClaudeApprovalTimeout
	if approvalTimeout <= 0 {
		approvalTimeout = 300 // 5분
	}

	opts := []InteractiveClaudeCLIProviderOption{
		WithInteractiveCLIPath(cliPath),
		WithInteractiveTimeout(time.Duration(cliTimeout) * time.Second),
		WithInteractiveApprovalPolicy(policy),
		WithInteractiveHookServerPort(cfg.ClaudeHookServerPort),
		WithInteractiveApprovalTimeout(time.Duration(approvalTimeout) * time.Second),
	}

	if cfg.ClaudeDefaultModel != "" {
		opts = append(opts, WithInteractiveDefaultModel(cfg.ClaudeDefaultModel))
	}

	return NewInteractiveClaudeCLIProvider(opts...)
}

// GetForTask resolves a provider using explicit provider name first,
// falling back to model-based resolution.
//
//  1. Explicit provider name (from agent's provider field)
//  2. OpenRouter canonical name resolution (anthropic->claude, openai->codex, google->gemini)
//  3. Model name prefix matching (claude-*, gemini-*)
//  4. Provider.Supports() check
func (r *Registry) GetForTask(providerName, model string) (Provider, error) {
	// 1. Explicit provider name takes priority.
	if providerName != "" {
		if p := r.Get(providerName); p != nil {
			return p, nil
		}
		// 2. 백엔드가 정규 이름(anthropic/openai/google)으로 보낼 수 있으므로
		// 내부 이름으로 변환하여 재시도
		internalName := ToInternalName(providerName)
		if internalName != providerName {
			if p := r.Get(internalName); p != nil {
				return p, nil
			}
		}
	}

	// 3. Fall back to model-based resolution.
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
	// 우선순위: 1) API 키 2) ChatGPT 환경변수 3) ~/.codex/auth.json 자동 감지
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

	// 명시적 인증이 없으면 ~/.codex/auth.json에서 자동 감지
	if authKey == "" {
		method, key, err := readCodexAuthFile(logger)
		if err != nil {
			logger.Debug().Err(err).Msg("codex auth.json 자동 감지 실패")
		} else {
			authMethod = method
			authKey = key
			logger.Info().Str("method", method).Msg("codex auth.json에서 인증 정보 자동 감지")
		}
	}

	if authKey == "" {
		return nil, fmt.Errorf("%w: app-server 모드에는 API 키 또는 ChatGPT 인증이 필요합니다 (codex login으로 로그인하세요)", ErrNoAPIKey)
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

// readCodexAuthFile은 ~/.codex/auth.json에서 인증 정보를 읽습니다.
// codex CLI가 로그인 상태이면 저장된 인증 정보를 반환합니다.
func readCodexAuthFile(logger zerolog.Logger) (method string, key string, err error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", fmt.Errorf("홈 디렉토리 확인 실패: %w", err)
	}

	authPath := filepath.Join(home, ".codex", "auth.json")
	data, err := os.ReadFile(authPath)
	if err != nil {
		return "", "", fmt.Errorf("auth.json 읽기 실패: %w", err)
	}

	var authData struct {
		AuthMode string          `json:"auth_mode"`
		APIKey   *string         `json:"OPENAI_API_KEY"`
		Tokens   json.RawMessage `json:"tokens"`
	}
	if err := json.Unmarshal(data, &authData); err != nil {
		return "", "", fmt.Errorf("auth.json 파싱 실패: %w", err)
	}

	// API 키가 있으면 우선 사용
	if authData.APIKey != nil && *authData.APIKey != "" {
		return "apiKey", *authData.APIKey, nil
	}

	// ChatGPT 인증이 있으면 사용 (app-server가 자체 처리)
	if authData.AuthMode == "chatgpt" && len(authData.Tokens) > 0 {
		return "chatgpt", "saved", nil
	}

	return "", "", fmt.Errorf("auth.json에 유효한 인증 정보가 없습니다 (auth_mode: %s)", authData.AuthMode)
}

// Execute는 모델에 맞는 프로바이더를 찾아 실행합니다.
// 편의 메서드로, GetForModel + Execute를 한 번에 수행합니다.
// OpenRouter 형식(openai/o3-mini)은 자동으로 프로바이더 접두사를 제거합니다.
func (r *Registry) Execute(ctx context.Context, req ExecuteRequest) (*ExecuteResponse, error) {
	provider, err := r.GetForModel(req.Model)
	if err != nil {
		return nil, err
	}
	// OpenRouter 형식이면 프로바이더 접두사 제거 (하위 API/CLI가 이해할 수 있는 형식으로)
	req.Model = StripProviderPrefix(req.Model)
	return provider.Execute(ctx, req)
}
