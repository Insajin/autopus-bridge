// Package cmd는 Local Agent Bridge CLI의 명령어를 정의합니다.
// config.go는 설정 관리 명령을 구현합니다.
// REQ-E-10: config 명령으로 API 키 설정 인터페이스 제공
package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/insajin/autopus-bridge/internal/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// configCmd는 설정 관리를 위한 상위 명령어입니다.
// REQ-E-10: config 명령 시 API 키 설정 인터페이스 제공
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "설정을 관리합니다",
	Long: `설정 파일의 값을 조회하거나 수정합니다.

설정 파일 위치: ~/.config/autopus/config.yaml

주의: API 키는 환경변수로 설정하는 것을 권장합니다.
  - CLAUDE_API_KEY: Claude API 키
  - GEMINI_API_KEY: Gemini API 키
  - LAB_TOKEN: JWT 인증 토큰`,
}

// configSetCmd는 설정 값을 저장하는 명령어입니다.
var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "설정 값을 저장합니다",
	Long: `설정 파일에 값을 저장합니다.

키는 점(.)으로 구분된 경로를 사용합니다.
예시:
  autopus config set server.url wss://custom.server.io/ws/agent
  autopus config set logging.level debug
  autopus config set reconnection.max_attempts 5

지원하는 설정 키:
  server.url              - WebSocket 서버 URL
  server.timeout_seconds  - 연결 타임아웃(초)
  logging.level           - 로그 레벨 (debug, info, warn, error)
  logging.format          - 로그 포맷 (json, text)
  logging.file            - 로그 파일 경로 (비어있으면 stdout)
  reconnection.max_attempts     - 최대 재연결 시도 횟수 (최대 10)
  reconnection.initial_delay_ms - 초기 재연결 지연(밀리초)
  reconnection.max_delay_ms     - 최대 재연결 지연(밀리초)
  reconnection.backoff_multiplier - 지수 백오프 배수`,
	Args: cobra.ExactArgs(2),
	RunE: runConfigSet,
}

// configGetCmd는 설정 값을 조회하는 명령어입니다.
var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "설정 값을 조회합니다",
	Long: `설정 파일에서 특정 키의 값을 조회합니다.

키는 점(.)으로 구분된 경로를 사용합니다.
예시:
  autopus config get server.url
  autopus config get logging.level`,
	Args: cobra.ExactArgs(1),
	RunE: runConfigGet,
}

// configListCmd는 전체 설정을 출력하는 명령어입니다.
var configListCmd = &cobra.Command{
	Use:   "list",
	Short: "전체 설정을 출력합니다",
	Long: `현재 적용된 모든 설정을 YAML 포맷으로 출력합니다.

API 키 관련 환경변수 설정 여부도 함께 표시됩니다.
민감한 정보(API 키)는 마스킹 처리되어 표시됩니다.`,
	RunE: runConfigList,
}

// configPathCmd는 설정 파일 경로를 출력하는 명령어입니다.
var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "설정 파일 경로를 출력합니다",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println(config.DefaultConfigPath())
		return nil
	},
}

// configInitCmd는 기본 설정 파일을 생성하는 명령어입니다.
var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "기본 설정 파일을 생성합니다",
	Long: `기본 설정 파일을 ~/.config/autopus/config.yaml에 생성합니다.

이미 파일이 존재하면 덮어쓰지 않습니다.
강제로 덮어쓰려면 --force 플래그를 사용하세요.`,
	RunE: runConfigInit,
}

var forceInit bool

func init() {
	rootCmd.AddCommand(configCmd)

	// 하위 명령 등록
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configListCmd)
	configCmd.AddCommand(configPathCmd)
	configCmd.AddCommand(configInitCmd)

	// init 명령 플래그
	configInitCmd.Flags().BoolVar(&forceInit, "force", false, "기존 파일을 덮어씁니다")
}

// runConfigSet은 설정 값을 저장합니다.
func runConfigSet(cmd *cobra.Command, args []string) error {
	key := args[0]
	value := args[1]

	// 유효한 키인지 확인
	if !isValidConfigKey(key) {
		return fmt.Errorf("알 수 없는 설정 키: %s", key)
	}

	// 값 변환 (숫자, 불리언 등)
	parsedValue := parseConfigValue(value)

	// viper에 설정
	viper.Set(key, parsedValue)

	// 설정 디렉토리 확인/생성
	if err := config.EnsureConfigDir(); err != nil {
		return fmt.Errorf("설정 디렉토리 생성 실패: %w", err)
	}

	// 설정 파일 저장
	configPath := config.DefaultConfigPath()
	if err := viper.WriteConfigAs(configPath); err != nil {
		return fmt.Errorf("설정 파일 저장 실패: %w", err)
	}

	fmt.Printf("%s = %v\n", key, parsedValue)
	fmt.Printf("설정이 저장되었습니다: %s\n", configPath)
	return nil
}

// runConfigGet은 설정 값을 조회합니다.
func runConfigGet(cmd *cobra.Command, args []string) error {
	key := args[0]

	value := viper.Get(key)
	if value == nil {
		return fmt.Errorf("설정 키를 찾을 수 없습니다: %s", key)
	}

	// API 키 관련 환경변수는 마스킹 처리
	if strings.Contains(key, "api_key") {
		if strVal, ok := value.(string); ok && strVal != "" {
			// 환경변수 이름이면 그대로 출력, 아니면 마스킹
			if !strings.HasSuffix(strVal, "_KEY") {
				value = maskSensitiveValue(strVal)
			}
		}
	}

	fmt.Printf("%s = %v\n", key, value)
	return nil
}

// runConfigList는 전체 설정을 출력합니다.
func runConfigList(cmd *cobra.Command, args []string) error {
	// 설정 로드
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("설정 로드 실패: %w", err)
	}

	// 설정 파일 경로 출력
	configFile := viper.ConfigFileUsed()
	if configFile != "" {
		fmt.Printf("# 설정 파일: %s\n", configFile)
	} else {
		fmt.Printf("# 설정 파일: (기본값 사용 중)\n")
	}
	fmt.Println()

	// YAML로 직렬화
	yamlData, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("YAML 직렬화 실패: %w", err)
	}

	fmt.Println(string(yamlData))

	// API 키 환경변수 상태 출력
	fmt.Println("# 환경변수 상태:")
	printEnvStatus("CLAUDE_API_KEY", cfg.Providers.Claude.APIKeyEnv)
	printEnvStatus("GEMINI_API_KEY", cfg.Providers.Gemini.APIKeyEnv)
	printEnvStatus("LAB_TOKEN", "LAB_TOKEN")

	return nil
}

// runConfigInit은 기본 설정 파일을 생성합니다.
func runConfigInit(cmd *cobra.Command, args []string) error {
	configPath := config.DefaultConfigPath()

	// 기존 파일 확인
	if !forceInit {
		if _, err := os.Stat(configPath); err == nil {
			return fmt.Errorf("설정 파일이 이미 존재합니다: %s\n--force 플래그로 덮어쓸 수 있습니다", configPath)
		}
	}

	// 설정 디렉토리 생성
	if err := config.EnsureConfigDir(); err != nil {
		return fmt.Errorf("설정 디렉토리 생성 실패: %w", err)
	}

	// 기본 설정 파일 내용
	defaultConfig := `# Autopus Local Bridge 설정 파일
# 생성됨: autopus config init

server:
  url: "wss://api.autopus.co/ws/agent"
  timeout_seconds: 30

auth:
  token_file: "~/.config/autopus/token"

providers:
  claude:
    # API 키는 환경변수로 설정하세요 (CLAUDE_API_KEY)
    api_key_env: "CLAUDE_API_KEY"
    default_model: "claude-sonnet-4-20250514"
  gemini:
    # API 키는 환경변수로 설정하세요 (GEMINI_API_KEY)
    api_key_env: "GEMINI_API_KEY"
    default_model: "gemini-2.0-flash"

logging:
  level: "info"    # debug, info, warn, error
  format: "json"   # json, text
  file: ""         # 비어있으면 stdout

reconnection:
  max_attempts: 10
  initial_delay_ms: 1000
  max_delay_ms: 60000
  backoff_multiplier: 2.0
`

	if err := os.WriteFile(configPath, []byte(defaultConfig), 0600); err != nil {
		return fmt.Errorf("설정 파일 생성 실패: %w", err)
	}

	fmt.Printf("설정 파일이 생성되었습니다: %s\n", configPath)
	fmt.Println("\n다음 환경변수를 설정하세요:")
	fmt.Println("  export CLAUDE_API_KEY=<your-claude-api-key>")
	fmt.Println("  export GEMINI_API_KEY=<your-gemini-api-key>")
	fmt.Println("  export LAB_TOKEN=<your-jwt-token>")
	return nil
}

// isValidConfigKey는 유효한 설정 키인지 확인합니다.
func isValidConfigKey(key string) bool {
	validKeys := map[string]bool{
		"server.url":                      true,
		"server.timeout_seconds":          true,
		"auth.token_file":                 true,
		"providers.claude.api_key_env":    true,
		"providers.claude.default_model":  true,
		"providers.claude.mode":           true, // api, cli, hybrid
		"providers.claude.cli_path":       true, // claude CLI 바이너리 경로
		"providers.claude.cli_timeout":    true, // CLI 실행 타임아웃(초)
		"providers.gemini.api_key_env":    true,
		"providers.gemini.default_model":  true,
		"providers.gemini.mode":           true,
		"providers.gemini.cli_path":       true,
		"providers.gemini.cli_timeout":    true,
		"logging.level":                   true,
		"logging.format":                  true,
		"logging.file":                    true,
		"reconnection.max_attempts":       true,
		"reconnection.initial_delay_ms":   true,
		"reconnection.max_delay_ms":       true,
		"reconnection.backoff_multiplier": true,
	}
	return validKeys[key]
}

// parseConfigValue는 문자열 값을 적절한 타입으로 변환합니다.
func parseConfigValue(value string) interface{} {
	// 불리언
	if value == "true" {
		return true
	}
	if value == "false" {
		return false
	}

	// 정수
	var intVal int
	if _, err := fmt.Sscanf(value, "%d", &intVal); err == nil {
		// 소수점이 없으면 정수로 처리
		if !strings.Contains(value, ".") {
			return intVal
		}
	}

	// 실수
	var floatVal float64
	if _, err := fmt.Sscanf(value, "%f", &floatVal); err == nil {
		return floatVal
	}

	// 기본: 문자열
	return value
}

// maskSensitiveValue는 민감한 값을 마스킹합니다.
func maskSensitiveValue(value string) string {
	if len(value) <= 8 {
		return "***"
	}
	return value[:4] + "***" + value[len(value)-4:]
}

// printEnvStatus는 환경변수 설정 상태를 출력합니다.
func printEnvStatus(displayName, envVar string) {
	value := os.Getenv(envVar)
	if value != "" {
		masked := maskSensitiveValue(value)
		fmt.Printf("  %s: 설정됨 (%s)\n", displayName, masked)
	} else {
		fmt.Printf("  %s: 설정되지 않음\n", displayName)
	}
}
