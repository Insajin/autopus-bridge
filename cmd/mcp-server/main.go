// Package main은 autopus-mcp-server 독립 바이너리의 진입점입니다.
// Claude Code MCP 플러그인으로 사용되며, stdio 트랜스포트로 MCP 서버를 실행합니다.
//
// 사용 예시 (Claude Code MCP 설정):
//
//	{
//	  "mcpServers": {
//	    "autopus": {
//	      "command": "autopus-mcp-server"
//	    }
//	  }
//	}
package main

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
	"github.com/spf13/viper"
)

// 빌드 시 ldflags로 주입되는 버전 정보
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "오류: %v\n", err)
		os.Exit(1)
	}
}

// run은 MCP 서버의 메인 로직을 실행합니다.
func run() error {
	// 설정 초기화
	initConfig()

	// 로거 초기화 (stderr로 출력, stdout은 MCP stdio에서 사용)
	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).
		With().
		Timestamp().
		Str("component", "autopus-mcp-server").
		Str("version", version).
		Logger()

	logger.Info().
		Str("commit", commit).
		Str("date", date).
		Msg("Autopus MCP 서버를 시작합니다...")

	// 1. 인증 정보 로드
	creds, err := auth.Load()
	if err != nil {
		return fmt.Errorf("인증 정보를 읽을 수 없습니다: %w", err)
	}
	if creds == nil {
		return fmt.Errorf("인증 정보가 없습니다. 먼저 'autopus-bridge login'으로 로그인하세요")
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

// initConfig는 MCP 서버에 필요한 설정을 초기화합니다.
func initConfig() {
	// 환경변수 자동 바인딩 (LAB_ 접두사)
	viper.SetEnvPrefix("LAB")
	viper.AutomaticEnv()

	// 설정 파일 경로
	home, err := os.UserHomeDir()
	if err == nil {
		viper.AddConfigPath(home + "/.config/local-agent-bridge")
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}

	// MCP 서버 기본 설정
	viper.SetDefault("mcpserver.backend_url", "https://api.autopus.co")
	viper.SetDefault("mcpserver.timeout", "30s")
	viper.SetDefault("mcpserver.cache_ttl", "30s")

	// 설정 파일 읽기 (없어도 오류 아님)
	_ = viper.ReadInConfig()
}
