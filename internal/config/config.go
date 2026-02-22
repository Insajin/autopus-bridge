// Package config는 Local Agent Bridge의 설정 관리를 담당합니다.
// REQ-U-04: 설정 우선순위 (환경변수 > 설정파일 > 기본값)
package config

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config는 전체 애플리케이션 설정을 나타냅니다.
// SPEC Section 4.4의 설정 파일 구조를 따릅니다.
type Config struct {
	Server       ServerConfig       `mapstructure:"server"`
	Auth         AuthConfig         `mapstructure:"auth"`
	Providers    ProvidersConfig    `mapstructure:"providers"`
	Logging      LoggingConfig      `mapstructure:"logging"`
	Reconnection ReconnectionConfig `mapstructure:"reconnection"`
	Security     SecurityConfig     `mapstructure:"security"`
	ComputerUse  ComputerUseConfig  `mapstructure:"computer_use"`
}

// ComputerUseConfig는 Computer Use 컨테이너 격리 설정입니다.
// SPEC-COMPUTER-USE-002: REQ-C6-01
type ComputerUseConfig struct {
	// Isolation은 격리 모드입니다 ("container", "local", "auto").
	Isolation string `mapstructure:"isolation"`
	// MaxContainers는 최대 동시 컨테이너 수입니다.
	MaxContainers int `mapstructure:"max_containers"`
	// WarmPoolSize는 warm 풀 크기입니다.
	WarmPoolSize int `mapstructure:"warm_pool_size"`
	// Image는 Docker 이미지 이름입니다.
	Image string `mapstructure:"image"`
	// ContainerMemory는 컨테이너 메모리 제한입니다 (예: "512m").
	ContainerMemory string `mapstructure:"container_memory"`
	// ContainerCPU는 컨테이너 CPU 제한입니다 (예: "1.0").
	ContainerCPU string `mapstructure:"container_cpu"`
	// IdleTimeout은 warm 컨테이너 유휴 타임아웃입니다 (예: "5m").
	IdleTimeout string `mapstructure:"idle_timeout"`
	// Network는 Docker 네트워크 이름입니다.
	Network string `mapstructure:"network"`
}

// SecurityConfig는 보안 관련 설정입니다.
type SecurityConfig struct {
	// Sandbox는 작업 디렉토리 샌드박스 설정입니다.
	Sandbox SandboxConfig `yaml:"sandbox" mapstructure:"sandbox"`
}

// SandboxConfig는 파일시스템 샌드박스 설정입니다.
// SEC-P2-03: 작업 디렉토리 샌드박싱으로 비인가 파일 접근 방지
type SandboxConfig struct {
	// Enabled는 샌드박스 활성화 여부입니다.
	Enabled bool `yaml:"enabled" mapstructure:"enabled"`
	// AllowedPaths는 접근이 허용된 경로 목록입니다.
	AllowedPaths []string `yaml:"allowed_paths" mapstructure:"allowed_paths"`
	// DeniedPaths는 접근이 거부된 경로 목록입니다.
	DeniedPaths []string `yaml:"denied_paths" mapstructure:"denied_paths"`
	// DenyHiddenDirs는 숨김 디렉토리 (. 으로 시작) 접근 거부 여부입니다.
	DenyHiddenDirs bool `yaml:"deny_hidden_dirs" mapstructure:"deny_hidden_dirs"`
}

// ServerConfig는 서버 연결 설정입니다.
type ServerConfig struct {
	// URL은 WebSocket 서버 주소입니다.
	URL string `mapstructure:"url"`
	// TimeoutSeconds는 연결 타임아웃(초)입니다.
	TimeoutSeconds int `mapstructure:"timeout_seconds"`
}

// AuthConfig는 인증 설정입니다.
type AuthConfig struct {
	// TokenFile은 JWT 토큰을 저장할 파일 경로입니다.
	TokenFile string `mapstructure:"token_file"`
}

// ProvidersConfig는 AI 프로바이더 설정입니다.
type ProvidersConfig struct {
	Claude ProviderConfig `mapstructure:"claude"`
	Gemini ProviderConfig `mapstructure:"gemini"`
	Codex  ProviderConfig `mapstructure:"codex"`
}

// ProviderConfig는 개별 AI 프로바이더 설정입니다.
type ProviderConfig struct {
	// APIKeyEnv는 API 키를 가져올 환경변수 이름입니다.
	// REQ-N-01: API 키를 평문으로 파일에 저장하지 않음
	APIKeyEnv string `mapstructure:"api_key_env"`
	// DefaultModel은 기본 사용 모델입니다.
	DefaultModel string `mapstructure:"default_model"`
	// Mode는 프로바이더 실행 모드입니다 ("api", "cli", "hybrid").
	// - "api": API 키를 사용한 직접 API 호출 (기본값)
	// - "cli": claude CLI 바이너리를 서브프로세스로 실행
	// - "hybrid": CLI 우선, API 폴백
	Mode string `mapstructure:"mode"`
	// CLIPath는 claude CLI 바이너리 경로입니다.
	// 기본값: "claude" (PATH에서 검색)
	CLIPath string `mapstructure:"cli_path"`
	// CLITimeout은 CLI 실행 타임아웃(초)입니다.
	// 기본값: 300 (5분)
	CLITimeout int `mapstructure:"cli_timeout"`
	// ApprovalPolicy는 App Server 모드의 승인 정책입니다 ("auto-approve", "deny-all").
	// 기본값: "auto-approve"
	ApprovalPolicy string `mapstructure:"approval_policy"`
	// ChatGPTAuthEnv는 ChatGPT 인증 토큰을 가져올 환경변수 이름입니다.
	ChatGPTAuthEnv string `mapstructure:"chatgpt_auth_env"`
}

// LoggingConfig는 로깅 설정입니다.
type LoggingConfig struct {
	// Level은 로그 레벨입니다 (debug, info, warn, error).
	Level string `mapstructure:"level"`
	// Format은 로그 포맷입니다 (json, text).
	// REQ-U-01: 구조화된 JSON 로그
	Format string `mapstructure:"format"`
	// File은 로그 파일 경로입니다. 비어있으면 stdout으로 출력합니다.
	File string `mapstructure:"file"`
}

// ReconnectionConfig는 재연결 설정입니다.
type ReconnectionConfig struct {
	// MaxAttempts는 최대 재연결 시도 횟수입니다.
	// REQ-N-05: 재연결 시도를 무한 반복하지 않음 (최대 10회)
	MaxAttempts int `mapstructure:"max_attempts"`
	// InitialDelayMs는 초기 재연결 지연 시간(밀리초)입니다.
	InitialDelayMs int `mapstructure:"initial_delay_ms"`
	// MaxDelayMs는 최대 재연결 지연 시간(밀리초)입니다.
	MaxDelayMs int `mapstructure:"max_delay_ms"`
	// BackoffMultiplier는 지수 백오프 배수입니다.
	BackoffMultiplier float64 `mapstructure:"backoff_multiplier"`
}

// Load는 설정을 로드하고 Config 구조체를 반환합니다.
func Load() (*Config, error) {
	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("설정 파싱 실패: %w", err)
	}

	// 홈 디렉토리 경로 확장
	cfg.Auth.TokenFile = expandPath(cfg.Auth.TokenFile)
	cfg.Logging.File = expandPath(cfg.Logging.File)

	return &cfg, nil
}

// GetAPIKey는 환경변수에서 API 키를 가져옵니다.
// REQ-N-01: API 키를 평문으로 파일에 저장하지 않음
func (p *ProviderConfig) GetAPIKey() string {
	if p.APIKeyEnv == "" {
		return ""
	}
	return os.Getenv(p.APIKeyEnv)
}

// HasAPIKey는 API 키가 설정되어 있는지 확인합니다.
func (p *ProviderConfig) HasAPIKey() bool {
	return p.GetAPIKey() != ""
}

// GetMode는 프로바이더 모드를 반환합니다.
// 설정되지 않은 경우 기본값 "api"를 반환합니다.
func (p *ProviderConfig) GetMode() string {
	if p.Mode == "" {
		return "api"
	}
	return p.Mode
}

// GetCLIPath는 CLI 바이너리 경로를 반환합니다.
// 설정되지 않은 경우 기본값 "claude"를 반환합니다.
func (p *ProviderConfig) GetCLIPath() string {
	if p.CLIPath == "" {
		return "claude"
	}
	return p.CLIPath
}

// GetCLITimeout은 CLI 타임아웃(초)을 반환합니다.
// 설정되지 않은 경우 기본값 300을 반환합니다.
func (p *ProviderConfig) GetCLITimeout() int {
	if p.CLITimeout <= 0 {
		return 300
	}
	return p.CLITimeout
}

// GetApprovalPolicy는 승인 정책을 반환합니다.
// 설정되지 않은 경우 기본값 "auto-approve"를 반환합니다.
func (p *ProviderConfig) GetApprovalPolicy() string {
	if p.ApprovalPolicy == "" {
		return "auto-approve"
	}
	return p.ApprovalPolicy
}

// GetChatGPTAuthKey는 ChatGPT 인증 토큰을 환경변수에서 가져옵니다.
func (p *ProviderConfig) GetChatGPTAuthKey() string {
	if p.ChatGPTAuthEnv == "" {
		return ""
	}
	return os.Getenv(p.ChatGPTAuthEnv)
}

// HasChatGPTAuth는 ChatGPT 인증이 설정되어 있는지 확인합니다.
func (p *ProviderConfig) HasChatGPTAuth() bool {
	return p.GetChatGPTAuthKey() != ""
}

// IsCLIAvailable은 CLI 바이너리가 사용 가능한지 확인합니다.
func (p *ProviderConfig) IsCLIAvailable() bool {
	cliPath := p.GetCLIPath()
	_, err := exec.LookPath(cliPath)
	return err == nil
}

// IsAvailable은 프로바이더가 사용 가능한지 확인합니다.
// 모드에 따라 API 키 또는 CLI 가용성을 확인합니다.
func (p *ProviderConfig) IsAvailable() bool {
	mode := p.GetMode()
	switch mode {
	case "api":
		return p.HasAPIKey()
	case "cli":
		return p.IsCLIAvailable()
	case "hybrid":
		// 하이브리드 모드는 CLI 또는 API 중 하나만 있으면 됨
		return p.IsCLIAvailable() || p.HasAPIKey()
	case "app-server":
		// App Server 모드는 CLI 바이너리와 인증(API Key 또는 ChatGPT 토큰)이 필요
		return p.IsCLIAvailable() && (p.HasAPIKey() || p.HasChatGPTAuth())
	default:
		return false
	}
}

// Validate는 설정의 유효성을 검사합니다.
func (c *Config) Validate() error {
	// REQ-S-05: 설정된 AI 프로바이더가 없으면 연결 시도를 거부
	// Claude: 모드에 따라 API 키 또는 CLI 가용성 확인
	// Gemini: API 키만 확인
	if !c.Providers.Claude.IsAvailable() && !c.Providers.Gemini.IsAvailable() && !c.Providers.Codex.IsAvailable() {
		return fmt.Errorf("AI 프로바이더가 설정되지 않았습니다. ANTHROPIC_API_KEY, GEMINI_API_KEY, 또는 OPENAI_API_KEY 환경변수를 설정하거나, claude/gemini/codex CLI를 설치하세요")
	}

	// Claude 모드 검증
	claudeMode := c.Providers.Claude.GetMode()
	validModes := map[string]bool{
		"api":    true,
		"cli":    true,
		"hybrid": true,
	}
	if !validModes[claudeMode] {
		return fmt.Errorf("유효하지 않은 Claude 모드: %s (api, cli, hybrid 중 하나)", claudeMode)
	}

	// CLI 모드인 경우 CLI 가용성 확인
	if claudeMode == "cli" && !c.Providers.Claude.IsCLIAvailable() {
		return fmt.Errorf("claude CLI 모드가 설정되었지만 claude 바이너리를 찾을 수 없습니다: %s", c.Providers.Claude.GetCLIPath())
	}

	// Gemini 모드 검증
	geminiMode := c.Providers.Gemini.GetMode()
	if !validModes[geminiMode] {
		return fmt.Errorf("유효하지 않은 Gemini 모드: %s (api, cli, hybrid 중 하나)", geminiMode)
	}
	if geminiMode == "cli" && !c.Providers.Gemini.IsCLIAvailable() {
		return fmt.Errorf("gemini CLI 모드가 설정되었지만 gemini 바이너리를 찾을 수 없습니다: %s", c.Providers.Gemini.GetCLIPath())
	}

	// Codex 모드 검증 (app-server 추가)
	codexMode := c.Providers.Codex.GetMode()
	validCodexModes := map[string]bool{
		"api":        true,
		"cli":        true,
		"hybrid":     true,
		"app-server": true,
	}
	if !validCodexModes[codexMode] {
		return fmt.Errorf("유효하지 않은 Codex 모드: %s (api, cli, hybrid, app-server 중 하나)", codexMode)
	}
	if codexMode == "cli" && !c.Providers.Codex.IsCLIAvailable() {
		return fmt.Errorf("codex CLI 모드가 설정되었지만 codex 바이너리를 찾을 수 없습니다: %s", c.Providers.Codex.GetCLIPath())
	}
	if codexMode == "app-server" && !c.Providers.Codex.IsCLIAvailable() {
		return fmt.Errorf("codex app-server 모드가 설정되었지만 codex 바이너리를 찾을 수 없습니다: %s", c.Providers.Codex.GetCLIPath())
	}

	// 로그 레벨 검증
	validLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}
	if !validLevels[c.Logging.Level] {
		return fmt.Errorf("유효하지 않은 로그 레벨: %s (debug, info, warn, error 중 하나)", c.Logging.Level)
	}

	// 로그 포맷 검증
	validFormats := map[string]bool{
		"json": true,
		"text": true,
	}
	if !validFormats[c.Logging.Format] {
		return fmt.Errorf("유효하지 않은 로그 포맷: %s (json, text 중 하나)", c.Logging.Format)
	}

	// 재연결 설정 검증 (0 = 무제한)
	if c.Reconnection.MaxAttempts < 0 {
		return fmt.Errorf("max_attempts는 0 이상이어야 합니다 (0 = 무제한)")
	}

	return nil
}

// GetAvailableProviders는 사용 가능한 프로바이더 목록을 반환합니다.
// Claude의 경우 모드에 따라 API 키 또는 CLI 가용성을 확인합니다.
func (c *Config) GetAvailableProviders() []string {
	var providers []string
	if c.Providers.Claude.IsAvailable() {
		providers = append(providers, "claude")
	}
	if c.Providers.Gemini.IsAvailable() {
		providers = append(providers, "gemini")
	}
	if c.Providers.Codex.IsAvailable() {
		providers = append(providers, "codex")
	}
	return providers
}

// GetClaudeProviderInfo는 Claude 프로바이더의 상세 정보를 반환합니다.
func (c *Config) GetClaudeProviderInfo() map[string]interface{} {
	return map[string]interface{}{
		"mode":          c.Providers.Claude.GetMode(),
		"cli_path":      c.Providers.Claude.GetCLIPath(),
		"cli_timeout":   c.Providers.Claude.GetCLITimeout(),
		"cli_available": c.Providers.Claude.IsCLIAvailable(),
		"api_available": c.Providers.Claude.HasAPIKey(),
		"available":     c.Providers.Claude.IsAvailable(),
	}
}

// GetIsolationMode는 격리 모드를 반환합니다.
// 설정되지 않은 경우 기본값 "auto"를 반환합니다.
func (c *ComputerUseConfig) GetIsolationMode() string {
	if c.Isolation == "" {
		return "auto"
	}
	return c.Isolation
}

// IsContainerMode는 컨테이너 모드 여부를 확인합니다.
func (c *ComputerUseConfig) IsContainerMode() bool {
	mode := c.GetIsolationMode()
	return mode == "container" || mode == "auto"
}

// expandPath는 ~를 홈 디렉토리로 확장합니다.
func expandPath(path string) string {
	if path == "" {
		return ""
	}
	if path[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[1:])
	}
	return path
}

// EnsureConfigDir는 설정 디렉토리가 존재하는지 확인하고 없으면 생성합니다.
func EnsureConfigDir() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("홈 디렉토리를 찾을 수 없습니다: %w", err)
	}

	configDir := filepath.Join(home, ".config", "autopus")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf("설정 디렉토리 생성 실패: %w", err)
	}

	return nil
}

// DefaultConfigPath는 기본 설정 파일 경로를 반환합니다.
func DefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "autopus", "config.yaml")
}
