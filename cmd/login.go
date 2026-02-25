// Package cmd provides CLI commands for Local Agent Bridge.
// login.go implements authentication via Device Authorization Flow (RFC 8628) with PKCE (RFC 7636).
package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
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
	// loginTimeout은 인증 대기 타임아웃 (10분)
	loginTimeout = 10 * time.Minute
)

// loginDeviceCodeOnly 플래그: 하위 호환성을 위해 유지 (Device Code가 유일한 방법이므로 no-op)
var loginDeviceCodeOnly bool

// loginCmd handles the login command
var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Autopus 서버에 로그인합니다",
	Long: `Autopus 서버에 로그인합니다.

Device Code Flow를 사용하여 브라우저에서 인증합니다.
토큰은 ~/.config/autopus/credentials.json에 저장됩니다.
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
	loginCmd.Flags().BoolVar(&loginDeviceCodeOnly, "device-code", false, "Device Code Flow 사용 (기본값, 하위 호환성)")
}

// runLogin executes the Device Authorization Flow login
func runLogin(cmd *cobra.Command, args []string) error {
	existing, err := auth.Load()
	if err != nil {
		return fmt.Errorf("기존 인증 정보 확인 실패: %w", err)
	}

	if existing != nil && existing.IsValid() {
		logger.Info().
			Str("email", existing.UserEmail).
			Msg("이미 로그인되어 있습니다.")
		return nil
	}

	return performDeviceCodeLogin(cmd)
}

// performDeviceCodeLogin executes the Device Authorization Flow with PKCE
func performDeviceCodeLogin(cmd *cobra.Command) error {
	apiBaseURL := getAPIBaseURL()

	// Step 1: PKCE 키 쌍 생성
	pkce, err := auth.GeneratePKCE()
	if err != nil {
		return fmt.Errorf("PKCE 생성 실패: %w", err)
	}

	// Step 2: 디바이스 코드 요청 (PKCE code_challenge 포함)
	fmt.Println("기기 인증을 시작합니다...")
	fmt.Println()

	deviceResp, err := auth.RequestDeviceCode(apiBaseURL, pkce)
	if err != nil {
		return fmt.Errorf("디바이스 코드 요청 실패: %w", err)
	}

	// Step 3: 인증 코드 표시
	fmt.Printf("  인증 코드: %s\n", deviceResp.UserCode)
	fmt.Println()
	fmt.Printf("  다음 URL에서 위 코드를 입력하세요:\n")
	fmt.Printf("  %s\n", deviceResp.VerificationURI)
	fmt.Println()

	// Step 4: 브라우저 자동 열기
	openURL := deviceResp.VerificationURIComplete
	if openURL == "" {
		openURL = deviceResp.VerificationURI
	}
	if browserErr := openBrowser(openURL); browserErr != nil {
		logger.Warn().Err(browserErr).Msg("브라우저 자동 열기 실패")
		fmt.Printf("  브라우저에서 직접 위 URL을 열어주세요.\n\n")
	}

	// Step 5: 토큰 폴링 (PKCE code_verifier 포함)
	ctx, cancel := context.WithTimeout(context.Background(), loginTimeout)
	defer cancel()

	expiresAt := time.Now().Add(time.Duration(deviceResp.ExpiresIn) * time.Second)
	stopSpinner := make(chan struct{})
	go spinnerLoop(stopSpinner, expiresAt)

	tokenResp, err := auth.PollDeviceToken(ctx, apiBaseURL, deviceResp.DeviceCode, deviceResp.Interval, pkce.CodeVerifier)
	close(stopSpinner)
	fmt.Printf("\r%s\r", strings.Repeat(" ", 50))

	if err != nil {
		return fmt.Errorf("인증 실패: %w", err)
	}

	// Step 6: 인증 정보 저장
	tokenExpiresAt := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	if jwtExpiry, parseErr := auth.ParseJWTExpiry(tokenResp.AccessToken); parseErr == nil {
		tokenExpiresAt = jwtExpiry
	}

	creds := &auth.Credentials{
		AccessToken:   tokenResp.AccessToken,
		RefreshToken:  tokenResp.RefreshToken,
		ExpiresAt:     tokenExpiresAt,
		ServerURL:     getServerURL(),
		UserEmail:     tokenResp.UserEmail,
		WorkspaceID:   tokenResp.WorkspaceID,
		WorkspaceSlug: tokenResp.WorkspaceSlug,
	}

	// Step 7: 워크스페이스 선택
	if err := selectWorkspace(creds, tokenResp); err != nil {
		logger.Warn().Err(err).Msg("워크스페이스 선택 실패")
	}

	if saveErr := auth.Save(creds); saveErr != nil {
		return fmt.Errorf("인증 정보 저장 실패: %w", saveErr)
	}

	// 성공 출력
	logger.Info().Str("email", creds.UserEmail).Msg("로그인 성공!")
	if creds.UserEmail != "" {
		fmt.Printf("  인증 성공! 이메일: %s\n", creds.UserEmail)
	} else {
		fmt.Println("  인증 성공!")
	}
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

// selectWorkspace는 토큰 응답의 워크스페이스 목록에서 선택하거나 API로 조회합니다.
func selectWorkspace(creds *auth.Credentials, tokenResp *auth.DeviceTokenResponse) error {
	// 이미 워크스페이스가 설정된 경우 스킵
	if creds.WorkspaceID != "" {
		return nil
	}

	// 토큰 응답에서 워크스페이스 목록 가져오기
	workspaces := tokenResp.Workspaces

	// 토큰 응답에 워크스페이스가 없으면 API 조회
	if len(workspaces) == 0 {
		apiBaseURL := getAPIBaseURL()
		apiWorkspaces, err := fetchWorkspaces(apiBaseURL, creds.AccessToken)
		if err != nil {
			return err
		}
		for _, ws := range apiWorkspaces {
			workspaces = append(workspaces, auth.TokenWorkspace{
				ID:   ws.ID,
				Name: ws.Name,
				Slug: ws.Slug,
				Role: ws.Role,
			})
		}
	}

	if len(workspaces) == 0 {
		return nil // 워크스페이스가 없으면 나중에 선택
	}

	if len(workspaces) == 1 {
		// 워크스페이스가 1개면 자동 선택
		creds.WorkspaceID = workspaces[0].ID
		creds.WorkspaceSlug = workspaces[0].Slug
		creds.WorkspaceName = workspaces[0].Name
		return nil
	}

	// 2개 이상이면 사용자에게 선택 요청
	fmt.Println()
	fmt.Println("  사용 가능한 워크스페이스:")
	for i, ws := range workspaces {
		role := ""
		if ws.Role != "" {
			role = fmt.Sprintf(" [%s]", ws.Role)
		}
		fmt.Printf("    %d) %s (%s)%s\n", i+1, ws.Name, ws.Slug, role)
	}
	fmt.Printf("\n  선택 [1]: ")

	scanner := bufio.NewScanner(os.Stdin)
	choice := 1
	if scanner.Scan() {
		input := strings.TrimSpace(scanner.Text())
		if input != "" {
			if _, scanErr := fmt.Sscanf(input, "%d", &choice); scanErr != nil {
				choice = 1
			}
		}
	}
	if choice < 1 || choice > len(workspaces) {
		choice = 1
	}

	selected := workspaces[choice-1]
	creds.WorkspaceID = selected.ID
	creds.WorkspaceSlug = selected.Slug
	creds.WorkspaceName = selected.Name
	return nil
}

// performAuthFlow는 Device Auth Flow를 실행하고 인증 정보를 반환합니다.
// up.go 등 다른 명령에서 호출됩니다.
func performAuthFlow() (*auth.Credentials, error) {
	return performDeviceAuthFlow()
}

// ─────────────────────────────────────────────────────────────────────────────
// Logout / Helpers
// ─────────────────────────────────────────────────────────────────────────────

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

// isTTY checks if stdin is connected to a terminal.
func isTTY() bool {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) != 0
}

// getBaseURL returns the frontend base URL for constructing login URLs.
func getBaseURL() string {
	serverURL := viper.GetString("server.url")
	if strings.Contains(serverURL, "localhost") || strings.Contains(serverURL, "127.0.0.1") {
		return "http://localhost:3000"
	}
	return "https://autopus.co"
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

// spinnerLoop displays a countdown spinner until stopCh is closed.
func spinnerLoop(stopCh <-chan struct{}, expiresAt time.Time) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-stopCh:
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
}
