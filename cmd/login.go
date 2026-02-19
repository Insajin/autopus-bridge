// Package cmd provides CLI commands for Local Agent Bridge.
// login.go implements Device Authorization Flow (RFC 8628) for authentication.
package cmd

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/insajin/autopus-bridge/internal/auth"
	"github.com/insajin/autopus-bridge/internal/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	// 로그인 타임아웃 (10분)
	loginTimeout = 10 * time.Minute
)

// loginCmd handles the login command
var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Autopus 서버에 로그인합니다",
	Long: `Device Authorization Flow를 사용하여 Autopus 서버에 로그인합니다.

브라우저에서 인증 코드를 입력하면 로그인이 완료됩니다.
토큰은 ~/.config/local-agent-bridge/credentials.json에 저장됩니다.
이후 'lab connect' 명령 시 저장된 토큰이 자동으로 사용됩니다.`,
	RunE: runLogin,
}

// logoutCmd handles the logout command
var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "저장된 인증 정보를 삭제합니다",
	Long:  `로컬에 저장된 인증 토큰을 삭제합니다.`,
	RunE:  runLogout,
}

func init() {
	rootCmd.AddCommand(loginCmd)
	rootCmd.AddCommand(logoutCmd)
}

// runLogin executes the Device Authorization Flow login
func runLogin(cmd *cobra.Command, args []string) error {
	// 이미 로그인되어 있는지 확인
	existing, err := auth.Load()
	if err != nil {
		return fmt.Errorf("기존 인증 정보 확인 실패: %w", err)
	}

	if existing != nil && existing.IsValid() {
		logger.Info().
			Str("email", existing.UserEmail).
			Msg("이미 로그인되어 있습니다. 다시 로그인하려면 'lab logout' 후 다시 시도하세요.")
		return nil
	}

	apiBaseURL := getAPIBaseURL()

	// Step 1: 디바이스 코드 요청
	fmt.Println("기기 인증을 시작합니다...")
	fmt.Println()

	deviceResp, err := auth.RequestDeviceCode(apiBaseURL)
	if err != nil {
		return fmt.Errorf("디바이스 코드 요청 실패: %w", err)
	}

	// Step 2: 인증 코드 표시
	fmt.Printf("  인증 코드: %s\n", deviceResp.UserCode)
	fmt.Println()
	fmt.Printf("  다음 URL에서 위 코드를 입력하세요:\n")
	fmt.Printf("  %s\n", deviceResp.VerificationURI)
	fmt.Println()

	// Step 3: 브라우저 자동 열기
	openURL := deviceResp.VerificationURIComplete
	if openURL == "" {
		openURL = deviceResp.VerificationURI
	}

	if browserErr := openBrowser(openURL); browserErr != nil {
		logger.Warn().Err(browserErr).Msg("브라우저 자동 열기 실패")
		fmt.Printf("  브라우저에서 직접 위 URL을 열어주세요.\n\n")
	}

	// Step 4: 토큰 폴링 시작
	ctx, cancel := context.WithTimeout(context.Background(), loginTimeout)
	defer cancel()

	expiresAt := time.Now().Add(time.Duration(deviceResp.ExpiresIn) * time.Second)

	// 폴링 진행 표시를 위한 고루틴
	stopSpinner := make(chan struct{})
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stopSpinner:
				return
			case <-ticker.C:
				remaining := time.Until(expiresAt).Truncate(time.Second)
				if remaining < 0 {
					remaining = 0
				}
				minutes := int(remaining.Minutes())
				seconds := int(remaining.Seconds()) % 60
				fmt.Printf("\r  인증 대기 중... (남은 시간: %d분 %02d초)  ", minutes, seconds)
			}
		}
	}()

	tokenResp, err := auth.PollDeviceToken(ctx, apiBaseURL, deviceResp.DeviceCode, deviceResp.Interval)
	close(stopSpinner)
	// 진행 표시 줄 정리
	fmt.Printf("\r%s\r", strings.Repeat(" ", 50))

	if err != nil {
		return fmt.Errorf("인증 실패: %w", err)
	}

	// Step 5: 인증 정보 저장
	tokenExpiresAt := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)

	creds := &auth.Credentials{
		AccessToken:   tokenResp.AccessToken,
		RefreshToken:  tokenResp.RefreshToken,
		ExpiresAt:     tokenExpiresAt,
		ServerURL:     getServerURL(),
		UserEmail:     tokenResp.UserEmail,
		WorkspaceID:   tokenResp.WorkspaceID,
		WorkspaceSlug: tokenResp.WorkspaceSlug,
	}

	if saveErr := auth.Save(creds); saveErr != nil {
		return fmt.Errorf("인증 정보 저장 실패: %w", saveErr)
	}

	logger.Info().
		Str("email", creds.UserEmail).
		Msg("로그인 성공!")

	// 성공 메시지 출력
	if creds.UserEmail != "" {
		fmt.Printf("  인증 성공! 이메일: %s\n", creds.UserEmail)
	} else {
		fmt.Println("  인증 성공!")
	}

	// 워크스페이스 연결 상태 출력
	if creds.WorkspaceSlug != "" {
		fmt.Printf("  워크스페이스 '%s'에 연결되었습니다.\n", creds.WorkspaceSlug)
	}
	fmt.Println()

	// 자동 연결 시도
	fmt.Println("서버에 자동 연결을 시도합니다...")
	if connectErr := runConnect(cmd, nil); connectErr != nil {
		logger.Warn().Err(connectErr).Msg("자동 연결 실패. 'lab connect'로 수동 연결하세요.")
	}

	return nil
}

// runLogout clears stored credentials
func runLogout(cmd *cobra.Command, args []string) error {
	if !auth.Exists() {
		fmt.Println("저장된 인증 정보가 없습니다.")
		return nil
	}

	if err := auth.Clear(); err != nil {
		return fmt.Errorf("인증 정보 삭제 실패: %w", err)
	}

	fmt.Println("로그아웃 완료. 인증 정보가 삭제되었습니다.")
	return nil
}

// openBrowser opens the URL in the default browser
func openBrowser(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		return fmt.Errorf("지원하지 않는 운영체제: %s", runtime.GOOS)
	}

	return cmd.Start()
}

// getAPIBaseURL returns the backend API base URL (without path)
func getAPIBaseURL() string {
	serverURL := viper.GetString("server.url")
	if strings.Contains(serverURL, "localhost") || strings.Contains(serverURL, "127.0.0.1") {
		return "http://127.0.0.1:8080"
	}
	return "https://api.autopus.co"
}

// getServerURL returns the WebSocket server URL
func getServerURL() string {
	serverURL := viper.GetString("server.url")
	if serverURL != "" {
		return serverURL
	}
	return "wss://api.autopus.co/ws/agent"
}
