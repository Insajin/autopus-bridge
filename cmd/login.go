// Package cmd provides CLI commands for Local Agent Bridge.
// login.go implements authentication flows: Browser OAuth Callback and Device Authorization Flow (RFC 8628).
package cmd

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/insajin/autopus-bridge/internal/auth"
	"github.com/insajin/autopus-bridge/internal/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	// 로그인 타임아웃 (10분)
	loginTimeout = 10 * time.Minute

	// 브라우저 콜백 서버 포트 범위
	callbackPortStart = 19280
	callbackPortEnd   = 19290

	// CSRF state 토큰 바이트 길이
	stateByteLength = 32
)

// loginDeviceCodeOnly 플래그: 비대화형 환경에서 Device Code Flow만 사용
var loginDeviceCodeOnly bool

// loginCmd handles the login command
var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Autopus 서버에 로그인합니다",
	Long: `Autopus 서버에 로그인합니다.

브라우저 로그인 또는 Device Code Flow를 선택할 수 있습니다.
비대화형 환경에서는 --device-code 플래그를 사용하세요.

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
	loginCmd.Flags().BoolVar(&loginDeviceCodeOnly, "device-code", false, "Device Code Flow만 사용합니다 (비대화형 환경용)")
}

// runLogin executes the login flow with TUI selector or device code fallback
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

	// --device-code 플래그 또는 비대화형 환경인지 확인
	if loginDeviceCodeOnly || !isTTY() {
		return performDeviceCodeLogin(cmd)
	}

	// TUI 선택기 표시
	var loginMethod string
	err = huh.NewSelect[string]().
		Title("로그인 방법을 선택하세요").
		Options(
			huh.NewOption("브라우저 로그인 (추천)", "browser"),
			huh.NewOption("Device Code", "device"),
		).
		Value(&loginMethod).
		Run()

	if err != nil {
		// TUI 실패 시 (예: 파이프된 입력) Device Code로 폴백
		logger.Warn().Err(err).Msg("TUI 선택기 실패, Device Code Flow로 전환")
		return performDeviceCodeLogin(cmd)
	}

	switch loginMethod {
	case "browser":
		return performBrowserAuthFlow(cmd)
	case "device":
		return performDeviceCodeLogin(cmd)
	default:
		return performDeviceCodeLogin(cmd)
	}
}

// performDeviceCodeLogin executes the Device Authorization Flow login
func performDeviceCodeLogin(cmd *cobra.Command) error {
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

// ─────────────────────────────────────────────────────────────────────────────
// Browser OAuth Callback Flow
// ─────────────────────────────────────────────────────────────────────────────

// callbackResponse represents the JSON payload sent by the frontend to the CLI callback server.
type callbackResponse struct {
	AccessToken   string `json:"access_token"`
	RefreshToken  string `json:"refresh_token"`
	ExpiresIn     int    `json:"expires_in"`
	UserEmail     string `json:"user_email"`
	WorkspaceID   string `json:"workspace_id"`
	WorkspaceSlug string `json:"workspace_slug"`
	State         string `json:"state"`
	Error         string `json:"error,omitempty"`
}

// performBrowserAuthFlow executes the Browser OAuth Callback login flow.
func performBrowserAuthFlow(cmd *cobra.Command) error {
	// 1. 사용 가능한 포트 찾기
	port, err := findAvailablePort()
	if err != nil {
		fmt.Println("  콜백 서버 포트를 찾을 수 없습니다. Device Code Flow로 전환합니다...")
		return performDeviceCodeLogin(cmd)
	}

	// 2. CSRF 방지용 state 생성
	state, err := generateState()
	if err != nil {
		return fmt.Errorf("state 토큰 생성 실패: %w", err)
	}

	// 3. 콜백 서버 시작
	resultCh := make(chan *auth.Credentials, 1)
	errCh := make(chan error, 1)
	server := startCallbackServer(port, state, resultCh, errCh)

	// 서버 종료 보장
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}()

	// 4. 로그인 URL 생성 및 브라우저 열기
	baseURL := getBaseURL()
	loginURL := fmt.Sprintf("%s/login?source=cli&callback_port=%d&state=%s", baseURL, port, state)

	fmt.Println("브라우저에서 로그인을 진행합니다...")
	fmt.Println()

	if browserErr := openBrowser(loginURL); browserErr != nil {
		logger.Warn().Err(browserErr).Msg("브라우저 자동 열기 실패")
		fmt.Printf("  브라우저에서 다음 URL을 열어주세요:\n")
		fmt.Printf("  %s\n\n", loginURL)
	} else {
		fmt.Println("  브라우저가 열렸습니다. 로그인을 완료해주세요.")
		fmt.Println()
	}

	// 5. 콜백 대기
	fmt.Println("  인증 완료를 기다리는 중...")

	ctx, cancel := context.WithTimeout(context.Background(), loginTimeout)
	defer cancel()

	select {
	case creds := <-resultCh:
		// 6. 인증 정보 저장
		if saveErr := auth.Save(creds); saveErr != nil {
			return fmt.Errorf("인증 정보 저장 실패: %w", saveErr)
		}

		logger.Info().
			Str("email", creds.UserEmail).
			Msg("로그인 성공!")

		fmt.Println()
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

	case err := <-errCh:
		return fmt.Errorf("브라우저 인증 실패: %w", err)

	case <-ctx.Done():
		return fmt.Errorf("인증 시간 초과 (%v)", loginTimeout)
	}
}

// startCallbackServer starts an HTTP server to receive the OAuth callback from the browser.
// 서버는 127.0.0.1에만 바인딩합니다 (외부 접근 차단).
func startCallbackServer(port int, expectedState string, resultCh chan<- *auth.Credentials, errCh chan<- error) *http.Server {
	mux := http.NewServeMux()
	origin := getBaseURL()

	// 헬스체크 엔드포인트
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// 콜백 엔드포인트
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		// CORS 헤더 설정
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		// Preflight 요청 처리
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var resp callbackResponse
		if decodeErr := json.NewDecoder(r.Body).Decode(&resp); decodeErr != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			errCh <- fmt.Errorf("콜백 요청 파싱 실패: %w", decodeErr)
			return
		}

		// State 검증 (CSRF 방지)
		if resp.State != expectedState {
			http.Error(w, "Invalid state parameter", http.StatusForbidden)
			errCh <- fmt.Errorf("state 불일치: CSRF 공격 가능성")
			return
		}

		// 에러 응답 처리
		if resp.Error != "" {
			http.Error(w, resp.Error, http.StatusBadRequest)
			errCh <- fmt.Errorf("인증 서버 에러: %s", resp.Error)
			return
		}

		// 성공 응답 - HTML 페이지 반환
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<!DOCTYPE html>
<html>
<head><meta charset="utf-8"><title>인증 완료</title></head>
<body style="display:flex;justify-content:center;align-items:center;height:100vh;font-family:sans-serif;background:#f8f9fa;">
<div style="text-align:center;">
<h1 style="color:#22c55e;">인증 완료!</h1>
<p>이 탭을 닫아도 됩니다.</p>
</div>
</body>
</html>`))

		// Credentials 생성
		tokenExpiresAt := time.Now().Add(time.Duration(resp.ExpiresIn) * time.Second)
		creds := &auth.Credentials{
			AccessToken:   resp.AccessToken,
			RefreshToken:  resp.RefreshToken,
			ExpiresAt:     tokenExpiresAt,
			ServerURL:     getServerURL(),
			UserEmail:     resp.UserEmail,
			WorkspaceID:   resp.WorkspaceID,
			WorkspaceSlug: resp.WorkspaceSlug,
		}

		resultCh <- creds
	})

	server := &http.Server{
		Addr:              fmt.Sprintf("127.0.0.1:%d", port),
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		if listenErr := server.ListenAndServe(); listenErr != nil && listenErr != http.ErrServerClosed {
			errCh <- fmt.Errorf("콜백 서버 시작 실패: %w", listenErr)
		}
	}()

	return server
}

// generateState generates a cryptographically random state string for CSRF prevention.
func generateState() (string, error) {
	b := make([]byte, stateByteLength)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("랜덤 바이트 생성 실패: %w", err)
	}
	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(b), nil
}

// findAvailablePort tries to find an available port in the callback port range.
func findAvailablePort() (int, error) {
	for port := callbackPortStart; port <= callbackPortEnd; port++ {
		listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err != nil {
			continue
		}
		_ = listener.Close()
		return port, nil
	}
	return 0, fmt.Errorf("포트 %d-%d 범위에서 사용 가능한 포트를 찾을 수 없습니다", callbackPortStart, callbackPortEnd)
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

// performAuthFlow selects the appropriate auth flow based on the environment.
// TTY 환경에서는 TUI 선택기를 표시하고, 비대화형 환경에서는 Device Code Flow를 사용합니다.
func performAuthFlow() (*auth.Credentials, error) {
	if !isTTY() {
		return performDeviceAuthFlow()
	}

	// TUI 선택기 표시
	var loginMethod string
	err := huh.NewSelect[string]().
		Title("로그인 방법을 선택하세요").
		Options(
			huh.NewOption("브라우저 로그인 (추천)", "browser"),
			huh.NewOption("Device Code", "device"),
		).
		Value(&loginMethod).
		Run()

	if err != nil {
		// TUI 실패 시 Device Code로 폴백
		logger.Warn().Err(err).Msg("TUI 선택기 실패, Device Code Flow로 전환")
		return performDeviceAuthFlow()
	}

	switch loginMethod {
	case "browser":
		return performBrowserAuthFlowStandalone()
	default:
		return performDeviceAuthFlow()
	}
}

// performBrowserAuthFlowStandalone is the standalone version of browser auth flow
// that returns credentials directly (used by up.go).
func performBrowserAuthFlowStandalone() (*auth.Credentials, error) {
	// 1. 사용 가능한 포트 찾기
	port, err := findAvailablePort()
	if err != nil {
		fmt.Println("  콜백 서버 포트를 찾을 수 없습니다. Device Code Flow로 전환합니다...")
		return performDeviceAuthFlow()
	}

	// 2. CSRF 방지용 state 생성
	state, err := generateState()
	if err != nil {
		return nil, fmt.Errorf("state 토큰 생성 실패: %w", err)
	}

	// 3. 콜백 서버 시작
	resultCh := make(chan *auth.Credentials, 1)
	errCh := make(chan error, 1)
	server := startCallbackServer(port, state, resultCh, errCh)

	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}()

	// 4. 로그인 URL 생성 및 브라우저 열기
	baseURL := getBaseURL()
	loginURL := fmt.Sprintf("%s/login?source=cli&callback_port=%d&state=%s", baseURL, port, state)

	fmt.Println("  브라우저에서 로그인을 진행합니다...")
	fmt.Println()

	if browserErr := openBrowser(loginURL); browserErr != nil {
		logger.Warn().Err(browserErr).Msg("브라우저 자동 열기 실패")
		fmt.Printf("  브라우저에서 다음 URL을 열어주세요:\n")
		fmt.Printf("  %s\n\n", loginURL)
	} else {
		fmt.Println("  브라우저가 열렸습니다. 로그인을 완료해주세요.")
		fmt.Println()
	}

	// 5. 콜백 대기
	fmt.Println("  인증 완료를 기다리는 중...")

	ctx, cancel := context.WithTimeout(context.Background(), loginTimeout)
	defer cancel()

	select {
	case creds := <-resultCh:
		if saveErr := auth.Save(creds); saveErr != nil {
			return nil, fmt.Errorf("인증 정보 저장 실패: %w", saveErr)
		}
		return creds, nil

	case err := <-errCh:
		return nil, fmt.Errorf("브라우저 인증 실패: %w", err)

	case <-ctx.Done():
		return nil, fmt.Errorf("인증 시간 초과 (%v)", loginTimeout)
	}
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
