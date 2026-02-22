// Package cmd는 Local Agent Bridge CLI의 명령어를 정의합니다.
// setup.go는 초기 설정 마법사를 구현합니다.
// FR-P1-07: setup 명령으로 자동 환경 설정
package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/insajin/autopus-bridge/internal/aitools"
	"github.com/insajin/autopus-bridge/internal/branding"
	"github.com/insajin/autopus-bridge/internal/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// providerInfo는 감지된 프로바이더 정보를 담는 구조체입니다.
type providerInfo struct {
	Name      string
	CLIFound  bool
	APIKeyEnv string // 감지된 API 키 환경변수 이름 (비어있으면 미감지)
	CLIPath   string // CLI 바이너리 경로
	HasCLI    bool
	HasAPIKey bool
}

// setupCmd는 초기 설정 마법사 명령어입니다.
var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "초기 설정 마법사를 실행합니다",
	Long: `Autopus Local Bridge의 초기 설정을 안내합니다.

설정 마법사가 수행하는 작업:
  1. 설치된 AI CLI 도구 자동 감지 (claude, gemini, codex)
  2. API 키 환경변수 확인
  3. 서버 URL 설정
  4. 작업 디렉토리 설정
  5. 설정 파일 생성 (~/.config/autopus/config.yaml)

이미 설정 파일이 존재하면 덮어쓸지 확인합니다.`,
	RunE: runSetup,
}

func init() {
	rootCmd.AddCommand(setupCmd)
}

// runSetup은 setup 명령의 실행 로직입니다.
func runSetup(cmd *cobra.Command, args []string) error {
	scanner := bufio.NewScanner(os.Stdin)

	// 1. 환영 메시지 출력
	printWelcomeBanner()

	// 2. 기존 설정 파일 확인
	configPath := config.DefaultConfigPath()
	if _, err := os.Stat(configPath); err == nil {
		fmt.Printf("\n설정 파일이 이미 존재합니다: %s\n", configPath)
		fmt.Print("덮어쓰시겠습니까? (y/N): ")
		if !scanYesNo(scanner) {
			fmt.Println("설정을 취소합니다.")
			return nil
		}
	}

	// 3. 프로바이더 감지
	fmt.Println("\n========================================")
	fmt.Println(" AI 프로바이더 감지")
	fmt.Println("========================================")
	providers := detectProviders()
	printProviderStatus(providers)

	// 4. 서버 URL 설정
	fmt.Println("\n========================================")
	fmt.Println(" 서버 설정")
	fmt.Println("========================================")
	serverURL := promptServerURL(scanner)

	// 5. 작업 디렉토리 설정
	fmt.Println("\n========================================")
	fmt.Println(" 작업 디렉토리 설정")
	fmt.Println("========================================")
	workDir := promptWorkDirectory(scanner)

	// 6. 설정 파일 생성
	fmt.Println("\n========================================")
	fmt.Println(" 설정 파일 생성")
	fmt.Println("========================================")
	if err := writeSetupConfig(configPath, providers, serverURL, workDir); err != nil {
		return fmt.Errorf("설정 파일 생성 실패: %w", err)
	}
	fmt.Printf("설정 파일이 저장되었습니다: %s\n", configPath)

	// 7. 설정 요약 출력
	printSetupSummary(providers, serverURL, workDir, configPath)

	return nil
}

// printWelcomeBanner는 환영 배너를 출력합니다.
func printWelcomeBanner() {
	fmt.Println()
	fmt.Println(branding.StartupBanner())
	fmt.Println("========================================")
	fmt.Println(" 초기 설정")
	fmt.Println("========================================")
	fmt.Println()
	fmt.Println("이 마법사가 Autopus Local Bridge를 설정합니다.")
	fmt.Println()
	fmt.Println("수행 항목:")
	fmt.Println("  1. 설치된 AI CLI 도구 자동 감지")
	fmt.Println("  2. API 키 환경변수 확인")
	fmt.Println("  3. Autopus 서버 URL 설정")
	fmt.Println("  4. 기본 작업 디렉토리 설정")
	fmt.Println("  5. 설정 파일 생성")
}

// detectProviders는 설치된 AI 프로바이더를 감지합니다.
func detectProviders() []providerInfo {
	providers := []providerInfo{
		{
			Name:      "Claude",
			HasCLI:    detectCLI("claude"),
			APIKeyEnv: detectAPIKey("CLAUDE_API_KEY", "ANTHROPIC_API_KEY"),
		},
		{
			Name:      "Gemini",
			HasCLI:    detectCLI("gemini"),
			APIKeyEnv: detectAPIKey("GEMINI_API_KEY", "GOOGLE_API_KEY"),
		},
		{
			Name:      "Codex",
			HasCLI:    detectCLI("codex"),
			APIKeyEnv: detectAPIKey("OPENAI_API_KEY"),
		},
	}

	// CLI 경로 기록
	for i := range providers {
		if providers[i].HasCLI {
			cliName := strings.ToLower(providers[i].Name)
			if path, err := exec.LookPath(cliName); err == nil {
				providers[i].CLIPath = path
			}
		}
		providers[i].HasAPIKey = providers[i].APIKeyEnv != ""
	}

	return providers
}

// detectCLI는 주어진 CLI 도구가 PATH에 있는지 확인합니다.
func detectCLI(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// detectAPIKey는 주어진 환경변수 중 설정된 것을 찾아 반환합니다.
// 설정된 환경변수가 없으면 빈 문자열을 반환합니다.
func detectAPIKey(envVars ...string) string {
	for _, env := range envVars {
		if val := os.Getenv(env); val != "" {
			return env
		}
	}
	return ""
}

// printProviderStatus는 감지된 프로바이더 상태를 출력합니다.
func printProviderStatus(providers []providerInfo) {
	fmt.Println()
	anyFound := false

	for _, p := range providers {
		if p.HasCLI || p.HasAPIKey {
			anyFound = true
		}

		// CLI 상태
		if p.HasCLI {
			fmt.Printf("  [v] %s CLI: 감지됨", p.Name)
			if p.CLIPath != "" {
				fmt.Printf(" (%s)", p.CLIPath)
			}
			fmt.Println()
		} else {
			fmt.Printf("  [ ] %s CLI: 미감지\n", p.Name)
		}

		// API 키 상태
		if p.HasAPIKey {
			fmt.Printf("  [v] %s API 키: 설정됨 (%s)\n", p.Name, p.APIKeyEnv)
		} else {
			fmt.Printf("  [ ] %s API 키: 미설정\n", p.Name)
		}

		fmt.Println()
	}

	if !anyFound {
		fmt.Println("  주의: 감지된 프로바이더가 없습니다.")
		fmt.Println("  AI CLI를 설치하거나 API 키 환경변수를 설정하세요.")
		fmt.Println()
		fmt.Println("  환경변수 설정 예시:")
		fmt.Println("    export ANTHROPIC_API_KEY=<your-key>")
		fmt.Println("    export GEMINI_API_KEY=<your-key>")
		fmt.Println("    export OPENAI_API_KEY=<your-key>")
	}
}

// promptServerURL은 서버 URL을 사용자에게 입력받습니다.
func promptServerURL(scanner *bufio.Scanner) string {
	defaultURL := "wss://api.autopus.co/ws/agent"
	localURL := "ws://127.0.0.1:8080/ws/agent"

	fmt.Println()
	fmt.Println("Autopus 서버 URL을 선택하세요:")
	fmt.Printf("  1) 프로덕션 서버 (기본값: %s)\n", defaultURL)
	fmt.Printf("  2) 로컬 개발 서버 (%s)\n", localURL)
	fmt.Println("  3) 직접 입력")
	fmt.Print("\n선택 [1]: ")

	if !scanner.Scan() {
		return defaultURL
	}
	input := strings.TrimSpace(scanner.Text())

	switch input {
	case "", "1":
		return defaultURL
	case "2":
		return localURL
	case "3":
		fmt.Print("서버 URL을 입력하세요: ")
		if scanner.Scan() {
			customURL := strings.TrimSpace(scanner.Text())
			if customURL != "" {
				return customURL
			}
		}
		return defaultURL
	default:
		fmt.Printf("잘못된 선택입니다. 기본값을 사용합니다: %s\n", defaultURL)
		return defaultURL
	}
}

// promptWorkDirectory는 작업 디렉토리를 사용자에게 입력받습니다.
func promptWorkDirectory(scanner *bufio.Scanner) string {
	cwd, _ := os.Getwd()
	home, _ := os.UserHomeDir()
	defaultDir := filepath.Join(home, "projects")

	fmt.Println()
	fmt.Println("기본 작업 디렉토리를 선택하세요:")
	fmt.Printf("  1) 현재 디렉토리 (%s)\n", cwd)
	fmt.Printf("  2) 홈 프로젝트 디렉토리 (%s)\n", defaultDir)
	fmt.Println("  3) 직접 입력")
	fmt.Print("\n선택 [1]: ")

	if !scanner.Scan() {
		return cwd
	}
	input := strings.TrimSpace(scanner.Text())

	switch input {
	case "", "1":
		return cwd
	case "2":
		// 디렉토리가 없으면 생성 여부 확인
		if _, err := os.Stat(defaultDir); os.IsNotExist(err) {
			fmt.Printf("디렉토리가 존재하지 않습니다: %s\n", defaultDir)
			fmt.Print("생성하시겠습니까? (Y/n): ")
			if scanYesNoDefault(scanner, true) {
				if mkErr := os.MkdirAll(defaultDir, 0755); mkErr != nil {
					fmt.Printf("디렉토리 생성 실패: %v. 현재 디렉토리를 사용합니다.\n", mkErr)
					return cwd
				}
				fmt.Printf("디렉토리가 생성되었습니다: %s\n", defaultDir)
			} else {
				return cwd
			}
		}
		return defaultDir
	case "3":
		fmt.Print("작업 디렉토리 경로를 입력하세요: ")
		if scanner.Scan() {
			customDir := strings.TrimSpace(scanner.Text())
			if customDir != "" {
				// ~ 확장
				if strings.HasPrefix(customDir, "~/") {
					customDir = filepath.Join(home, customDir[2:])
				}
				// 경로 유효성 확인
				if _, err := os.Stat(customDir); os.IsNotExist(err) {
					fmt.Printf("경로가 존재하지 않습니다: %s\n", customDir)
					fmt.Print("생성하시겠습니까? (Y/n): ")
					if scanYesNoDefault(scanner, true) {
						if mkErr := os.MkdirAll(customDir, 0755); mkErr != nil {
							fmt.Printf("디렉토리 생성 실패: %v. 현재 디렉토리를 사용합니다.\n", mkErr)
							return cwd
						}
						fmt.Printf("디렉토리가 생성되었습니다: %s\n", customDir)
					} else {
						return cwd
					}
				}
				return customDir
			}
		}
		return cwd
	default:
		fmt.Printf("잘못된 선택입니다. 현재 디렉토리를 사용합니다: %s\n", cwd)
		return cwd
	}
}

// setupConfig는 설정 마법사에서 생성하는 설정 구조체입니다.
type setupConfig struct {
	Server       setupServerConfig    `yaml:"server"`
	Auth         setupAuthConfig      `yaml:"auth"`
	Providers    setupProvidersConfig `yaml:"providers"`
	Logging      setupLoggingConfig   `yaml:"logging"`
	Reconnection setupReconnConfig    `yaml:"reconnection"`
	WorkDir      string               `yaml:"work_dir,omitempty"`
}

type setupServerConfig struct {
	URL            string `yaml:"url"`
	TimeoutSeconds int    `yaml:"timeout_seconds"`
}

type setupAuthConfig struct {
	TokenFile string `yaml:"token_file"`
}

type setupProvidersConfig struct {
	Claude setupProviderEntry `yaml:"claude"`
	Gemini setupProviderEntry `yaml:"gemini"`
	Codex  setupProviderEntry `yaml:"codex,omitempty"`
}

type setupProviderEntry struct {
	APIKeyEnv    string `yaml:"api_key_env"`
	DefaultModel string `yaml:"default_model"`
	Mode         string `yaml:"mode,omitempty"`
}

type setupLoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
	File   string `yaml:"file"`
}

type setupReconnConfig struct {
	MaxAttempts       int     `yaml:"max_attempts"`
	InitialDelayMs    int     `yaml:"initial_delay_ms"`
	MaxDelayMs        int     `yaml:"max_delay_ms"`
	BackoffMultiplier float64 `yaml:"backoff_multiplier"`
}

// writeSetupConfig는 설정을 YAML 파일로 저장합니다.
func writeSetupConfig(configPath string, providers []providerInfo, serverURL, workDir string) error {
	// 설정 디렉토리 생성 (0700 권한)
	if err := config.EnsureConfigDir(); err != nil {
		return fmt.Errorf("설정 디렉토리 생성 실패: %w", err)
	}

	// Claude API 키 환경변수 결정
	claudeAPIKeyEnv := "ANTHROPIC_API_KEY"
	for _, p := range providers {
		if p.Name == "Claude" && p.APIKeyEnv != "" {
			claudeAPIKeyEnv = p.APIKeyEnv
			break
		}
	}

	// Gemini API 키 환경변수 결정
	geminiAPIKeyEnv := "GEMINI_API_KEY"
	for _, p := range providers {
		if p.Name == "Gemini" && p.APIKeyEnv != "" {
			geminiAPIKeyEnv = p.APIKeyEnv
			break
		}
	}

	// Codex API 키 환경변수 결정
	codexAPIKeyEnv := "OPENAI_API_KEY"
	for _, p := range providers {
		if p.Name == "Codex" && p.APIKeyEnv != "" {
			codexAPIKeyEnv = p.APIKeyEnv
			break
		}
	}

	// Claude 모드 결정
	claudeMode := "api"
	for _, p := range providers {
		if p.Name == "Claude" {
			if p.HasCLI && p.HasAPIKey {
				claudeMode = "hybrid"
			} else if p.HasCLI {
				claudeMode = "cli"
			}
			break
		}
	}

	// Gemini 모드 결정
	geminiMode := "api"
	for _, p := range providers {
		if p.Name == "Gemini" {
			if p.HasCLI && p.HasAPIKey {
				geminiMode = "hybrid"
			} else if p.HasCLI {
				geminiMode = "cli"
			}
			break
		}
	}

	// Codex 모드 결정
	codexMode := "api"
	for _, p := range providers {
		if p.Name == "Codex" {
			if p.HasCLI && p.HasAPIKey {
				codexMode = "hybrid"
			} else if p.HasCLI {
				codexMode = "cli"
			}
			break
		}
	}

	cfg := setupConfig{
		Server: setupServerConfig{
			URL:            serverURL,
			TimeoutSeconds: 30,
		},
		Auth: setupAuthConfig{
			TokenFile: "~/.config/autopus/token",
		},
		Providers: setupProvidersConfig{
			Claude: setupProviderEntry{
				APIKeyEnv:    claudeAPIKeyEnv,
				DefaultModel: "claude-sonnet-4-20250514",
				Mode:         claudeMode,
			},
			Gemini: setupProviderEntry{
				APIKeyEnv:    geminiAPIKeyEnv,
				DefaultModel: "gemini-2.0-flash",
				Mode:         geminiMode,
			},
			Codex: setupProviderEntry{
				APIKeyEnv:    codexAPIKeyEnv,
				DefaultModel: "o3-mini",
				Mode:         codexMode,
			},
		},
		Logging: setupLoggingConfig{
			Level:  "info",
			Format: "json",
			File:   "",
		},
		Reconnection: setupReconnConfig{
			MaxAttempts:       10,
			InitialDelayMs:    1000,
			MaxDelayMs:        60000,
			BackoffMultiplier: 2.0,
		},
		WorkDir: workDir,
	}

	// YAML 직렬화
	yamlData, err := yaml.Marshal(&cfg)
	if err != nil {
		return fmt.Errorf("YAML 직렬화 실패: %w", err)
	}

	// 헤더 주석 추가
	header := "# Autopus Local Bridge 설정 파일\n"
	header += "# 생성됨: autopus-bridge setup\n"
	header += "# 문서: https://docs.autopus.co/autopus\n\n"

	content := header + string(yamlData)

	// 파일 저장 (0644 권한)
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("설정 파일 저장 실패: %w", err)
	}

	return nil
}

// printSetupSummary는 설정 완료 요약을 출력합니다.
func printSetupSummary(providers []providerInfo, serverURL, workDir, configPath string) {
	fmt.Println()
	fmt.Println("========================================")
	fmt.Println(" 설정 완료")
	fmt.Println("========================================")
	fmt.Println()

	// 감지된 프로바이더
	fmt.Println("감지된 프로바이더:")
	hasProvider := false
	for _, p := range providers {
		if p.HasCLI || p.HasAPIKey {
			hasProvider = true
			modes := []string{}
			if p.HasCLI {
				modes = append(modes, "CLI")
			}
			if p.HasAPIKey {
				modes = append(modes, "API")
			}
			fmt.Printf("  [v] %s (%s)\n", p.Name, strings.Join(modes, " + "))
		}
	}
	if !hasProvider {
		fmt.Println("  (없음 - CLI 설치 또는 API 키 설정 필요)")
	}
	fmt.Println()

	// MCP 설정 상태
	fmt.Println("\nMCP 설정 상태:")
	claudeInfo, _ := aitools.DetectClaudeCode()
	codexInfo, _ := aitools.DetectCodexCLI()
	geminiInfo, _ := aitools.DetectGeminiCLI()

	if claudeInfo != nil && claudeInfo.Installed {
		if aitools.IsPluginInstalled() {
			fmt.Println("  [v] Claude Code: 플러그인 설치됨")
		} else {
			fmt.Println("  [ ] Claude Code: 플러그인 미설치")
		}
	}
	if codexInfo != nil && codexInfo.Installed {
		fmt.Println("  [v] Codex CLI: 감지됨")
	}
	if geminiInfo != nil && geminiInfo.Installed {
		fmt.Println("  [v] Gemini CLI: 감지됨")
	}
	fmt.Println()

	// 서버 URL
	fmt.Printf("서버 URL:     %s\n", serverURL)
	fmt.Printf("작업 디렉토리: %s\n", workDir)
	fmt.Printf("설정 파일:     %s\n", configPath)
	fmt.Println()

	// 다음 단계 안내
	fmt.Println("다음 단계:")
	fmt.Println("  1. autopus-bridge login    - Autopus 서버에 로그인")
	fmt.Println("  2. autopus-bridge up       - MCP 설정 포함 통합 시작")
	fmt.Println("  3. autopus-bridge connect  - 서버에 연결하여 작업 대기")
	fmt.Println()

	if !hasProvider {
		fmt.Println("주의: 프로바이더가 감지되지 않았습니다.")
		fmt.Println("연결 전에 다음 중 하나를 설정하세요:")
		fmt.Println()
		fmt.Println("  # Claude API 키 설정")
		fmt.Println("  export ANTHROPIC_API_KEY=<your-key>")
		fmt.Println()
		fmt.Println("  # 또는 Gemini API 키 설정")
		fmt.Println("  export GEMINI_API_KEY=<your-key>")
		fmt.Println()
		fmt.Println("  # 또는 OpenAI API 키 설정")
		fmt.Println("  export OPENAI_API_KEY=<your-key>")
		fmt.Println()
	}
}

// scanYesNo는 사용자 입력을 읽어 y/Y이면 true, 그 외이면 false를 반환합니다.
// 기본값은 No입니다.
func scanYesNo(scanner *bufio.Scanner) bool {
	if !scanner.Scan() {
		return false
	}
	input := strings.TrimSpace(strings.ToLower(scanner.Text()))
	return input == "y" || input == "yes"
}

// scanYesNoDefault는 사용자 입력을 읽어 y/n을 반환합니다.
// 빈 입력 시 defaultVal을 반환합니다.
func scanYesNoDefault(scanner *bufio.Scanner, defaultVal bool) bool {
	if !scanner.Scan() {
		return defaultVal
	}
	input := strings.TrimSpace(strings.ToLower(scanner.Text()))
	if input == "" {
		return defaultVal
	}
	return input == "y" || input == "yes"
}
