// Package cmd는 Local Agent Bridge CLI의 명령어를 정의합니다.
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/insajin/autopus-bridge/internal/config"
	"github.com/insajin/autopus-bridge/internal/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	// 전역 플래그
	cfgFile string
	verbose bool

	// 버전 정보 (main에서 주입)
	appVersion   string
	appCommit    string
	appBuildDate string
)

// rootCmd는 CLI의 루트 명령어입니다.
var rootCmd = &cobra.Command{
	Use:   "autopus",
	Short: "Autopus Local Bridge CLI",
	Long: `Autopus Local Bridge는 Autopus 서버와 WebSocket으로 통신하여
사용자의 로컬 AI 에이전트를 연결합니다.

사용자의 Claude, Gemini, Codex 등의 API 키를 활용하여
로컬에서 AI 작업을 실행합니다.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// 로거 초기화
		return initLogger()
	},
}

// Execute는 루트 명령어를 실행합니다.
func Execute() error {
	return rootCmd.Execute()
}

// SetVersionInfo는 버전 정보를 설정합니다.
func SetVersionInfo(version, commit, buildDate string) {
	appVersion = version
	appCommit = commit
	appBuildDate = buildDate
}

// GetVersionInfo는 버전 정보를 반환합니다.
func GetVersionInfo() (version, commit, buildDate string) {
	return appVersion, appCommit, appBuildDate
}

func init() {
	cobra.OnInitialize(initConfig)

	// 전역 플래그 정의
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "",
		"설정 파일 경로 (기본값: ~/.config/autopus/config.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false,
		"상세 로그 출력 (debug 레벨)")
}

// initConfig는 설정 파일을 초기화합니다.
// REQ-U-04: 설정 우선순위 - 환경변수 > 설정파일 > 기본값
func initConfig() {
	if cfgFile != "" {
		// 명시적 설정 파일 사용
		viper.SetConfigFile(cfgFile)
	} else {
		// 기본 설정 경로: ~/.config/autopus/config.yaml
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "홈 디렉토리를 찾을 수 없습니다: %v\n", err)
			os.Exit(1)
		}

		configDir := filepath.Join(home, ".config", "autopus")
		viper.AddConfigPath(configDir)
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}

	// 환경변수 자동 바인딩 (LAB_ 접두사)
	viper.SetEnvPrefix("LAB")
	viper.AutomaticEnv()

	// 기본값 설정
	setDefaults()

	// 설정 파일 읽기 (없어도 오류 아님)
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			// 설정 파일이 있지만 읽기 실패한 경우만 오류
			fmt.Fprintf(os.Stderr, "설정 파일 읽기 실패: %v\n", err)
		}
	}
}

// setDefaults는 기본 설정값을 정의합니다.
func setDefaults() {
	// 서버 설정
	viper.SetDefault("server.url", "wss://api.autopus.co/ws/agent")
	viper.SetDefault("server.timeout_seconds", 30)

	// 인증 설정
	home, _ := os.UserHomeDir()
	viper.SetDefault("auth.token_file", filepath.Join(home, ".config", "autopus", "token"))

	// Claude 프로바이더 설정
	viper.SetDefault("providers.claude.api_key_env", "CLAUDE_API_KEY")
	viper.SetDefault("providers.claude.default_model", "claude-sonnet-4-20250514")

	// Gemini 프로바이더 설정
	viper.SetDefault("providers.gemini.api_key_env", "GEMINI_API_KEY")
	viper.SetDefault("providers.gemini.default_model", "gemini-2.0-flash")

	// 로깅 설정
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "json")
	viper.SetDefault("logging.file", "")

	// 재연결 설정
	viper.SetDefault("reconnection.max_attempts", 10)
	viper.SetDefault("reconnection.initial_delay_ms", 1000)
	viper.SetDefault("reconnection.max_delay_ms", 60000)
	viper.SetDefault("reconnection.backoff_multiplier", 2.0)

	// 보안 설정 - 샌드박스 (SEC-P2-03)
	viper.SetDefault("security.sandbox.enabled", true)
	viper.SetDefault("security.sandbox.allowed_paths", []string{"~/projects", "~/workspace"})
	viper.SetDefault("security.sandbox.denied_paths", []string{"~/.ssh", "~/.gnupg", "~/.config", "~/.aws", "/etc", "/var"})
	viper.SetDefault("security.sandbox.deny_hidden_dirs", true)

	// Computer Use 기본값 (SPEC-COMPUTER-USE-002)
	viper.SetDefault("computer_use.isolation", "auto")
	viper.SetDefault("computer_use.max_containers", 5)
	viper.SetDefault("computer_use.warm_pool_size", 2)
	viper.SetDefault("computer_use.image", "autopus/chromium-sandbox:latest")
	viper.SetDefault("computer_use.container_memory", "512m")
	viper.SetDefault("computer_use.container_cpu", "1.0")
	viper.SetDefault("computer_use.idle_timeout", "5m")
	viper.SetDefault("computer_use.network", "autopus-sandbox-net")
}

// initLogger는 로거를 초기화합니다.
func initLogger() error {
	// 설정에서 로그 레벨 가져오기
	level := viper.GetString("logging.level")
	if verbose {
		level = "debug"
	}

	// 설정 로드
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("설정 로드 실패: %w", err)
	}

	// verbose 플래그가 설정되면 debug 레벨로 오버라이드
	if verbose {
		cfg.Logging.Level = "debug"
	} else {
		cfg.Logging.Level = level
	}

	// 로거 설정
	logger.Setup(cfg.Logging)
	return nil
}
