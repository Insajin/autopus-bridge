package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/insajin/autopus-bridge/internal/auth"
	"github.com/insajin/autopus-bridge/internal/mcpserver"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	rootCmd.AddCommand(mcpServeCmd)

	// MCP 서버 기본 설정
	viper.SetDefault("mcpserver.backend_url", "https://api.autopus.co")
	viper.SetDefault("mcpserver.timeout", "30s")
	viper.SetDefault("mcpserver.cache_ttl", "30s")
}

// mcpServeCmd는 MCP 서버를 시작하는 Cobra 서브커맨드입니다.
var mcpServeCmd = &cobra.Command{
	Use:   "mcp-serve",
	Short: "Start Autopus MCP server (stdio transport)",
	Long: `Autopus MCP 서버를 stdio 트랜스포트로 시작합니다.
Claude Code가 Autopus 플랫폼 기능에 MCP 도구/리소스로 접근할 수 있도록 합니다.

사용 예시 (Claude Code MCP 설정):
  {
    "mcpServers": {
      "autopus": {
        "command": "autopus",
        "args": ["mcp-serve"]
      }
    }
  }`,
	RunE: runMCPServe,
}

// runMCPServe는 MCP 서버를 시작합니다.
func runMCPServe(cmd *cobra.Command, args []string) error {
	// 로거 초기화 (stderr로 출력, stdout은 MCP stdio에서 사용)
	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).
		With().
		Timestamp().
		Str("component", "mcp-serve").
		Logger()

	logger.Info().Msg("Autopus MCP 서버를 시작합니다...")

	// 1. 인증 정보 로드
	creds, err := auth.Load()
	if err != nil {
		return fmt.Errorf("인증 정보를 읽을 수 없습니다: %w", err)
	}
	if creds == nil {
		return fmt.Errorf("인증 정보가 없습니다. 먼저 'autopus login'으로 로그인하세요")
	}

	logger.Info().
		Str("user", creds.UserEmail).
		Str("workspace", creds.WorkspaceSlug).
		Msg("인증 정보 로드 완료")

	// 2. TokenRefresher 초기화 (백그라운드 토큰 갱신)
	tokenRefresher := auth.NewTokenRefresher(creds)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tokenRefresher.Start(ctx)

	// 3. BackendClient 생성
	backendURL := viper.GetString("mcpserver.backend_url")
	timeoutStr := viper.GetString("mcpserver.timeout")
	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil {
		timeout = 30 * time.Second
		logger.Warn().
			Str("configured", timeoutStr).
			Str("fallback", "30s").
			Msg("유효하지 않은 타임아웃 설정, 기본값 사용")
	}

	client := mcpserver.NewBackendClient(backendURL, tokenRefresher, timeout, logger)

	// 4. MCP 서버 생성 (캐시 TTL 설정)
	cacheTTLStr := viper.GetString("mcpserver.cache_ttl")
	cacheTTL, cacheTTLErr := time.ParseDuration(cacheTTLStr)
	if cacheTTLErr != nil {
		cacheTTL = mcpserver.DefaultCacheTTL
		logger.Warn().
			Str("configured", cacheTTLStr).
			Str("fallback", mcpserver.DefaultCacheTTL.String()).
			Msg("유효하지 않은 캐시 TTL 설정, 기본값 사용")
	}
	srv := mcpserver.NewServer(client, logger, cacheTTL)

	// 5. 시그널 핸들링 (graceful shutdown)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		logger.Info().Str("signal", sig.String()).Msg("종료 시그널 수신, MCP 서버를 종료합니다")
		cancel()
		os.Exit(0)
	}()

	// 6. MCP 서버 시작 (stdio, 블로킹)
	logger.Info().
		Str("backend_url", backendURL).
		Str("timeout", timeout.String()).
		Msg("MCP 서버 준비 완료, stdio 대기 중...")

	if err := srv.Start(); err != nil {
		return fmt.Errorf("MCP 서버 실행 실패: %w", err)
	}

	return nil
}
