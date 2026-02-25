// Package cmd provides CLI commands for Local Agent Bridge.
// up.go implements the unified "up" command that combines login, setup, and connect.
// FR-P1-09: Single smart command for complete bridge startup.
package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/insajin/autopus-bridge/internal/aitools"
	"github.com/insajin/autopus-bridge/internal/auth"
	"github.com/insajin/autopus-bridge/internal/branding"
	"github.com/insajin/autopus-bridge/internal/config"
	"github.com/insajin/autopus-bridge/internal/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	totalUpSteps = 12
)

// upProgress tracks the completion state of each step for resume capability.
type upProgress struct {
	CompletedSteps []int     `json:"completed_steps"`
	LastError      string    `json:"last_error,omitempty"`
	LastStep       int       `json:"last_step"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// workspaceInfo represents a workspace returned from the API.
type workspaceInfo struct {
	ID   string `json:"id"`
	Slug string `json:"slug"`
	Name string `json:"name"`
	Role string `json:"role,omitempty"`
}

// workspacesResponse represents the API response for listing workspaces.
type workspacesResponse struct {
	Success bool            `json:"success"`
	Data    []workspaceInfo `json:"data"`
}

// upCmd is the unified startup command.
var upCmd = &cobra.Command{
	Use:   "up",
	Short: "인증, 설정, 연결을 한 번에 수행합니다",
	Long: `login, setup, connect를 통합한 스마트 명령입니다.

실행 단계:
  [1/12] 인증 확인
  [2/12] 토큰 갱신
  [3/12] 워크스페이스 선택
  [4/12] AI Provider 감지 및 설치
  [5/12] AI 구독 인증 확인
  [6/12] 비즈니스 도구 감지
  [7/12] Docker 감지 및 설정
  [8/12] Chromium Sandbox 이미지 준비
  [9/12] 미설치 도구 설치
  [10/12] AI 도구 MCP 설정
  [11/12] 설정 파일 업데이트
  [12/12] 서버 연결

각 단계가 실패하면 구체적인 해결 방법을 안내합니다.
재실행 시 완료된 단계는 자동으로 건너뜁니다.`,
	RunE: runUp,
}

var (
	upForceRestart bool
)

func init() {
	rootCmd.AddCommand(upCmd)

	upCmd.Flags().BoolVar(&upForceRestart, "force", false, "처음부터 다시 시작합니다 (진행 상태 초기화)")
}

// runUp executes the unified up command with 6 sequential steps.
func runUp(cmd *cobra.Command, args []string) error {
	fmt.Println()
	fmt.Println(branding.StartupBanner())
	fmt.Println("========================================")
	fmt.Println(" 시작")
	fmt.Println("========================================")
	fmt.Println()

	// Load or initialize progress
	progress := loadUpProgress()
	if upForceRestart {
		progress = &upProgress{}
		clearUpProgress()
	}

	scanner := bufio.NewScanner(os.Stdin)

	// ── Step 1: Auth Check ──
	var creds *auth.Credentials
	var err error

	if isStepCompleted(progress, 1) {
		printStep(1, totalUpSteps, "인증 확인 중...")
		// Even if step is "completed", we still need to load creds for subsequent steps
		creds, err = auth.Load()
		if err != nil || creds == nil || !creds.IsValid() {
			// Progress says done but creds are invalid - redo this step
			progress = removeStep(progress, 1)
			progress = removeStep(progress, 2)
		} else {
			printSkip(fmt.Sprintf("이미 인증됨 (%s)", creds.UserEmail))
		}
	}

	if !isStepCompleted(progress, 1) {
		printStep(1, totalUpSteps, "인증 확인 중...")
		creds, err = stepAuthCheck()
		if err != nil {
			printError(fmt.Sprintf("인증 실패: %v", err))
			saveUpProgress(progress, 1, err.Error())
			printFixSuggestion("auth", err)
			return err
		}
		markStepCompleted(progress, 1)
		saveUpProgress(progress, 0, "")
	}

	// ── Step 2: Token Refresh ──
	if isStepCompleted(progress, 2) {
		printStep(2, totalUpSteps, "토큰 갱신 중...")
		// Re-validate: creds might be stale
		if creds != nil && creds.IsValid() {
			printSkip("토큰이 유효합니다")
		} else {
			progress = removeStep(progress, 2)
		}
	}

	if !isStepCompleted(progress, 2) {
		printStep(2, totalUpSteps, "토큰 갱신 중...")
		creds, err = stepTokenRefresh(creds)
		if err != nil {
			printError(fmt.Sprintf("토큰 갱신 실패: %v", err))
			saveUpProgress(progress, 2, err.Error())
			printFixSuggestion("token_refresh", err)
			return err
		}
		markStepCompleted(progress, 2)
		saveUpProgress(progress, 0, "")
	}

	// ── Step 3: Workspace Selection ──
	if isStepCompleted(progress, 3) {
		printStep(3, totalUpSteps, "워크스페이스 선택 중...")
		if creds.WorkspaceID != "" {
			printSkip(fmt.Sprintf("워크스페이스: %s", creds.WorkspaceSlug))
		} else {
			progress = removeStep(progress, 3)
		}
	}

	if !isStepCompleted(progress, 3) {
		printStep(3, totalUpSteps, "워크스페이스 선택 중...")
		err = stepWorkspaceSelection(creds, scanner)
		if err != nil {
			printError(fmt.Sprintf("워크스페이스 선택 실패: %v", err))
			saveUpProgress(progress, 3, err.Error())
			printFixSuggestion("workspace", err)
			return err
		}
		markStepCompleted(progress, 3)
		saveUpProgress(progress, 0, "")
	}

	// ── Step 4: Provider Detection + AI CLI Installation ──
	printStep(4, totalUpSteps, "AI Provider 감지 및 설치 중...")
	providers := detectProviders()
	printProviderSummary(providers)
	providers = stepInstallMissingAICLI(providers, scanner)
	markStepCompleted(progress, 4)
	saveUpProgress(progress, 0, "")

	// ── Step 5: AI Subscription Auth Check ──
	printStep(5, totalUpSteps, "AI 구독 인증 확인 중...")
	providers = stepAISubscriptionAuth(providers, scanner)
	markStepCompleted(progress, 5)
	saveUpProgress(progress, 0, "")

	// ── Step 6: Business Tools Detection ──
	printStep(6, totalUpSteps, "비즈니스 도구 감지 중...")
	bizTools := detectBusinessTools()
	printBusinessToolSummary(bizTools)
	markStepCompleted(progress, 6)
	saveUpProgress(progress, 0, "")

	// ── Step 7: Docker Detection ──
	printStep(7, totalUpSteps, "Docker 감지 및 설정 중...")
	stepDockerDetection(scanner)
	markStepCompleted(progress, 7)
	saveUpProgress(progress, 0, "")

	// ── Step 8: Chromium Sandbox Image Preparation ──
	printStep(8, totalUpSteps, "Chromium Sandbox 이미지 준비 중...")
	stepChromiumSandboxImage()
	markStepCompleted(progress, 8)
	saveUpProgress(progress, 0, "")

	// ── Step 9: Missing Tools Installation ──
	printStep(9, totalUpSteps, "미설치 도구 확인 중...")
	stepInstallMissingTools(bizTools, scanner)
	markStepCompleted(progress, 9)
	saveUpProgress(progress, 0, "")

	// ── Step 10: AI Tool MCP Configuration ──
	printStep(10, totalUpSteps, "AI 도구 MCP 설정 중...")
	stepAIToolMCPConfig(providers, scanner)
	markStepCompleted(progress, 10)
	saveUpProgress(progress, 0, "")

	// ── Step 11: Config Update ──
	printStep(11, totalUpSteps, "설정 파일 업데이트 중...")
	err = stepConfigUpdate(providers, creds)
	if err != nil {
		printError(fmt.Sprintf("설정 업데이트 실패: %v", err))
		saveUpProgress(progress, 11, err.Error())
		printFixSuggestion("config", err)
		return err
	}
	markStepCompleted(progress, 11)
	saveUpProgress(progress, 0, "")

	// ── Step 12: Server Connection ──
	printStep(12, totalUpSteps, "서버 연결 중...")

	// Clear progress file before connecting (connection is the final step)
	clearUpProgress()

	fmt.Println()
	// Delegate to the existing connect logic
	connectErr := runConnect(cmd, nil)
	if connectErr == nil {
		return nil
	}

	// REQ-UX-001: 인증 실패 시 자동 재인증 후 1회 재시도
	if isAuthError(connectErr) {
		fmt.Println()
		fmt.Println("  서버 인증에 실패했습니다. 자동으로 재인증을 시도합니다...")
		fmt.Println()

		newCreds, authErr := performBrowserAuthWithFallback()
		if authErr != nil {
			printError(fmt.Sprintf("재인증 실패: %v", authErr))
			printFixSuggestion("connection", authErr)
			return fmt.Errorf("서버 연결 실패 (재인증 실패): %w", authErr)
		}

		printSuccess(fmt.Sprintf("재인증 성공: %s", newCreds.UserEmail))
		fmt.Println()
		fmt.Println("  서버에 다시 연결합니다...")
		fmt.Println()

		return runConnect(cmd, nil)
	}

	// 인증 외 에러는 그대로 반환
	printFixSuggestion("connection", connectErr)
	return connectErr
}

// ─────────────────────────────────────────────────────────────────────────────
// Auth Error Detection (REQ-UX-001)
// ─────────────────────────────────────────────────────────────────────────────

// isAuthError는 에러가 인증 관련 에러인지 판단합니다.
func isAuthError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	authPatterns := []string{
		"인증 실패",
		"인증 거부",
		"authentication failed",
		"authentication error",
		"unauthorized",
		"token expired",
		"토큰 만료",
		"토큰이 만료",
		"서버 인증",
	}
	for _, p := range authPatterns {
		if strings.Contains(msg, p) {
			return true
		}
	}
	return false
}

// ─────────────────────────────────────────────────────────────────────────────
// Step Implementations
// ─────────────────────────────────────────────────────────────────────────────

// performBrowserAuthWithFallback은 Device Code Flow로 인증합니다.
// 하위 호환성을 위해 함수명을 유지합니다.
func performBrowserAuthWithFallback() (*auth.Credentials, error) {
	return performDeviceAuthFlow()
}

// stepAuthCheck loads existing credentials or triggers browser auth flow.
// Returns valid credentials or error.
func stepAuthCheck() (*auth.Credentials, error) {
	creds, err := auth.Load()
	if err != nil {
		// auth.Load failed (corrupt file, permission issue, etc.)
		// Log warning and proceed to browser auth instead of stopping
		logger.Warn().Err(err).Msg("인증 정보 로드 실패, 새로 인증을 시작합니다")
		fmt.Println("  인증 정보를 로드할 수 없습니다. 새로 인증을 시작합니다...")
		fmt.Println()

		newCreds, authErr := performBrowserAuthWithFallback()
		if authErr != nil {
			return nil, authErr
		}
		printSuccess(fmt.Sprintf("인증 성공: %s", newCreds.UserEmail))
		return newCreds, nil
	}

	// Credentials exist and valid
	if creds != nil && creds.IsValid() {
		printSuccess(fmt.Sprintf("인증됨: %s", creds.UserEmail))
		return creds, nil
	}

	// Credentials exist but expired - will be handled in step 2
	if creds != nil && creds.AccessToken != "" {
		printSuccess("인증 정보 발견 (갱신 필요)")
		return creds, nil
	}

	// No credentials - directly open browser for login/signup
	fmt.Println("  저장된 인증 정보가 없습니다. 브라우저에서 로그인을 시작합니다...")
	fmt.Println()

	newCreds, err := performBrowserAuthWithFallback()
	if err != nil {
		return nil, err
	}

	printSuccess(fmt.Sprintf("인증 성공: %s", newCreds.UserEmail))
	return newCreds, nil
}

// stepTokenRefresh refreshes the token if expired. If refresh fails, triggers browser auth flow.
func stepTokenRefresh(creds *auth.Credentials) (*auth.Credentials, error) {
	if creds == nil {
		return nil, fmt.Errorf("인증 정보가 없습니다")
	}

	// Token still valid
	if creds.IsValid() {
		printSkip("토큰이 아직 유효합니다")
		return creds, nil
	}

	// Try refresh
	if creds.RefreshToken != "" {
		fmt.Println("  토큰이 만료되어 갱신을 시도합니다...")
		if err := auth.RefreshAccessToken(creds); err != nil {
			logger.Warn().Err(err).Msg("토큰 자동 갱신 실패, 재인증 시도")
			fmt.Println("  토큰 갱신 실패. 브라우저에서 재인증을 시작합니다...")
			fmt.Println()

			// Refresh failed - directly open browser, fallback to device code
			newCreds, authErr := performBrowserAuthWithFallback()
			if authErr != nil {
				return nil, authErr
			}
			printSuccess(fmt.Sprintf("재인증 성공: %s", newCreds.UserEmail))
			return newCreds, nil
		}

		printSuccess("토큰 갱신 성공")
		return creds, nil
	}

	// No refresh token - directly open browser, fallback to device code
	fmt.Println("  갱신 토큰이 없습니다. 브라우저에서 재인증을 시작합니다...")
	fmt.Println()

	newCreds, err := performBrowserAuthWithFallback()
	if err != nil {
		return nil, err
	}

	printSuccess(fmt.Sprintf("재인증 성공: %s", newCreds.UserEmail))
	return newCreds, nil
}

// stepWorkspaceSelection fetches workspaces from the API and selects one.
func stepWorkspaceSelection(creds *auth.Credentials, scanner *bufio.Scanner) error {
	// If credentials already have a workspace, use it
	if creds.WorkspaceID != "" && creds.WorkspaceSlug != "" {
		printSuccess(fmt.Sprintf("워크스페이스: %s (%s)", creds.WorkspaceSlug, creds.WorkspaceID[:8]+"..."))
		return nil
	}

	apiBaseURL := getAPIBaseURL()
	workspaces, err := fetchWorkspaces(apiBaseURL, creds.AccessToken)
	if err != nil {
		return fmt.Errorf("워크스페이스 목록 조회 실패: %w", err)
	}

	if len(workspaces) == 0 {
		return fmt.Errorf("사용 가능한 워크스페이스가 없습니다. 웹에서 워크스페이스를 생성하세요")
	}

	var selected workspaceInfo

	if len(workspaces) == 1 {
		// Auto-select the only workspace
		selected = workspaces[0]
		printSuccess(fmt.Sprintf("워크스페이스 자동 선택: %s", selected.Name))
	} else {
		// Present list for user selection
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
		selected = workspaces[choice-1]
		printSuccess(fmt.Sprintf("워크스페이스 선택: %s", selected.Name))
	}

	// Update credentials with workspace info
	creds.WorkspaceID = selected.ID
	creds.WorkspaceSlug = selected.Slug
	creds.WorkspaceName = selected.Name

	if err := auth.Save(creds); err != nil {
		return fmt.Errorf("인증 정보 업데이트 실패: %w", err)
	}

	return nil
}

// stepConfigUpdate updates the config file with detected providers and workspace info.
func stepConfigUpdate(providers []providerInfo, creds *auth.Credentials) error {
	configPath := config.DefaultConfigPath()

	// Determine server URL from existing config or default
	srvURL := viper.GetString("server.url")
	if srvURL == "" {
		srvURL = "wss://api.autopus.co/ws/agent"
	}

	// Determine work directory from existing config or current directory
	workDir := viper.GetString("work_dir")
	if workDir == "" {
		workDir, _ = os.Getwd()
	}

	if err := writeSetupConfig(configPath, providers, srvURL, workDir); err != nil {
		return fmt.Errorf("설정 파일 저장 실패: %w", err)
	}

	// Reload viper config after writing
	viper.SetConfigFile(configPath)
	if err := viper.ReadInConfig(); err != nil {
		logger.Warn().Err(err).Msg("설정 파일 재로드 실패")
	}

	printSuccess(fmt.Sprintf("설정 파일 저장: %s", configPath))
	return nil
}

// stepDockerDetection은 Docker 설치 여부를 확인하고, 미설치 시 자동 설치를 제안한다.
// Docker가 없어도 up 명령은 실패하지 않는다 (NON-BLOCKING).
// SPEC-COMPUTER-USE-002 Phase 2.
func stepDockerDetection(scanner *bufio.Scanner) {
	isolation := viper.GetString("computer_use.isolation")
	if isolation == "" {
		isolation = "auto"
	}

	// Docker CLI 존재 여부 확인
	dockerPath, err := exec.LookPath("docker")
	if err != nil {
		// Docker가 설치되어 있지 않음 - 자동 설치 제안
		fmt.Println("  Docker가 설치되어 있지 않습니다.")
		fmt.Printf("  Docker를 설치하시겠습니까? (Y/n): ")

		if scanYesNoDefault(scanner, true) {
			installed := false
			switch runtime.GOOS {
			case "darwin":
				// macOS: Homebrew를 통한 설치
				if _, brewErr := exec.LookPath("brew"); brewErr == nil {
					installCmd := "brew install --cask docker"
					fmt.Printf("  설치 중: %s\n", installCmd)
					if runErr := runInstallCommand(installCmd); runErr != nil {
						printError(fmt.Sprintf("Docker 설치 실패: %v", runErr))
					} else {
						installed = true
						printSuccess("Docker Desktop 설치 완료")
					}
				} else {
					fmt.Println("  ! Homebrew가 설치되어 있지 않아 자동 설치가 불가합니다.")
					fmt.Println("    수동 설치: https://docs.docker.com/desktop/install/mac-install/")
				}
			case "linux":
				installCmd := "curl -fsSL https://get.docker.com | sh"
				fmt.Printf("  설치 중: %s\n", installCmd)
				if runErr := runInstallCommand(installCmd); runErr != nil {
					printError(fmt.Sprintf("Docker 설치 실패: %v", runErr))
				} else {
					installed = true
					printSuccess("Docker 설치 완료")
					// docker 그룹에 사용자 추가 안내
					fmt.Println("  docker 그룹에 사용자를 추가하려면:")
					fmt.Println("    sudo usermod -aG docker $USER")
					fmt.Println("    (적용하려면 로그아웃 후 다시 로그인하세요)")
				}
			default:
				// Windows 등 기타 OS
				fmt.Println("  ! 이 OS에서는 자동 설치를 지원하지 않습니다.")
				fmt.Println("    수동 설치: https://docs.docker.com/desktop/install/windows-install/")
			}

			if installed {
				// 설치 후 Docker 데몬 시작 시도
				dockerPath, _ = exec.LookPath("docker")
				if dockerPath != "" {
					startDockerDaemon(dockerPath)
				}
				return
			}
		} else {
			if isolation == "container" {
				printError("Docker가 필요합니다 (isolation=container 모드)")
			} else {
				printSkip("Docker 설치 건너뜀 (컨테이너 격리 비활성화)")
			}
		}
		return
	}

	// Docker 데몬 실행 여부 확인
	infoCmd := exec.Command(dockerPath, "info")
	output, err := infoCmd.CombinedOutput()
	if err != nil {
		// Docker 설치됨, 데몬 미실행 - 시작 제안
		fmt.Println("  Docker가 설치되어 있지만 데몬이 실행되고 있지 않습니다.")
		startDockerDaemon(dockerPath)

		// 데몬 시작 후 재확인
		recheckCmd := exec.Command(dockerPath, "info")
		if recheckErr := recheckCmd.Run(); recheckErr != nil {
			if isolation == "container" {
				printError("Docker 데몬을 시작할 수 없습니다 (isolation=container 모드에 필요)")
			} else {
				fmt.Println("  ! Docker 데몬이 아직 실행되지 않았습니다 (컨테이너 격리 비활성화)")
			}
		} else {
			printDockerVersion(dockerPath)
		}
		return
	}

	// Docker 버전 정보 추출
	_ = output
	printDockerVersion(dockerPath)
}

// startDockerDaemon은 플랫폼에 맞게 Docker 데몬 시작을 시도한다.
func startDockerDaemon(dockerPath string) {
	switch runtime.GOOS {
	case "darwin":
		fmt.Println("  Docker Desktop을 시작합니다...")
		openCmd := exec.Command("open", "-a", "Docker")
		if openErr := openCmd.Run(); openErr != nil {
			fmt.Println("  ! Docker Desktop 시작 실패. 수동으로 시작하세요.")
			return
		}

		// Docker 데몬 시작 대기 (최대 60초)
		fmt.Print("  Docker 데몬 시작 대기 중...")
		for i := 0; i < 30; i++ {
			infoCmd := exec.Command(dockerPath, "info")
			if infoCmd.Run() == nil {
				fmt.Println()
				printSuccess("Docker 데몬 시작됨")
				return
			}
			time.Sleep(2 * time.Second)
			fmt.Printf("\r  Docker 데몬 시작 대기 중... (%d초)", (i+1)*2)
		}
		fmt.Println()
		fmt.Println("  ! Docker 데몬 시작 시간 초과 (60초)")

	case "linux":
		fmt.Println("  Docker 데몬을 시작하려면 다음을 실행하세요:")
		fmt.Println("    sudo systemctl start docker")
	}
}

// printDockerVersion은 Docker 버전 정보를 출력한다.
func printDockerVersion(dockerPath string) {
	versionCmd := exec.Command(dockerPath, "version", "--format", "{{.Server.Version}}")
	versionOutput, versionErr := versionCmd.Output()
	if versionErr == nil {
		printSuccess(fmt.Sprintf("Docker %s 감지됨", strings.TrimSpace(string(versionOutput))))
	} else {
		printSuccess("Docker 감지됨")
	}
}

// printDockerBuildFailureGuide는 Docker 빌드 실패 출력을 분석하여 원인별 해결 안내를 제공한다.
func printDockerBuildFailureGuide(buildOutput string) {
	lower := strings.ToLower(buildOutput)

	fmt.Println()
	switch {
	case strings.Contains(lower, "no space left on device"):
		fmt.Println("  원인: 디스크 공간이 부족합니다")
		fmt.Println()
		fmt.Println("  해결 방법:")
		fmt.Println("    1. docker system prune -a  (사용하지 않는 Docker 데이터 정리)")
		fmt.Println("    2. 정리 후 다시 시도: autopus up")

	case strings.Contains(lower, "network") || strings.Contains(lower, "timeout") ||
		strings.Contains(lower, "could not resolve") || strings.Contains(lower, "dial tcp"):
		fmt.Println("  원인: 네트워크 연결 문제 (베이스 이미지 다운로드 실패)")
		fmt.Println()
		fmt.Println("  해결 방법:")
		fmt.Println("    1. 인터넷 연결을 확인하세요")
		fmt.Println("    2. VPN을 사용 중이라면 잠시 끄고 다시 시도하세요")
		fmt.Println("    3. 다시 시도: autopus up")

	case strings.Contains(lower, "permission denied") || strings.Contains(lower, "access denied"):
		fmt.Println("  원인: Docker 권한 부족")
		fmt.Println()
		fmt.Println("  해결 방법:")
		if runtime.GOOS == "linux" {
			fmt.Println("    1. sudo usermod -aG docker $USER")
			fmt.Println("    2. 로그아웃 후 다시 로그인")
			fmt.Println("    3. 다시 시도: autopus up")
		} else {
			fmt.Println("    1. Docker Desktop이 실행 중인지 확인하세요")
			fmt.Println("    2. 다시 시도: autopus up")
		}

	case strings.Contains(lower, "daemon is not running") || strings.Contains(lower, "cannot connect"):
		fmt.Println("  원인: Docker 데몬이 실행되고 있지 않습니다")
		fmt.Println()
		fmt.Println("  해결 방법:")
		if runtime.GOOS == "darwin" {
			fmt.Println("    1. Docker Desktop 앱을 실행하세요")
		} else {
			fmt.Println("    1. sudo systemctl start docker")
		}
		fmt.Println("    2. 다시 시도: autopus up")

	default:
		// 알 수 없는 에러 - 원본 출력의 마지막 몇 줄을 보여줌
		fmt.Println("  원인을 자동으로 파악하지 못했습니다")
		fmt.Println()
		lines := strings.Split(strings.TrimSpace(buildOutput), "\n")
		// 마지막 5줄만 표시
		start := 0
		if len(lines) > 5 {
			start = len(lines) - 5
		}
		fmt.Println("  빌드 로그 (마지막 부분):")
		for _, line := range lines[start:] {
			fmt.Printf("    %s\n", strings.TrimSpace(line))
		}
		fmt.Println()
		fmt.Println("  수동 빌드를 시도하세요:")
		fmt.Println("    docker build -t autopus/chromium-sandbox:latest docker/chromium-sandbox/")
	}

	fmt.Println()
	fmt.Println("  이 단계는 선택사항입니다. Computer Use 없이 계속 진행합니다.")
}

// findDockerfileDir는 지정된 이미지 이름에 해당하는 Dockerfile 디렉토리를 탐색한다.
// 1. ~/.config/autopus/docker/<imageDirName>
// 2. 실행 파일 위치 기준
// 3. 현재 작업 디렉토리 기준
// 순서로 탐색한다.
func findDockerfileDir(imageDirName string) string {
	// 1. 홈 디렉토리 기반 캐시 경로 탐색
	home, homeErr := os.UserHomeDir()
	if homeErr == nil {
		candidate := filepath.Join(home, ".config", "autopus", "docker", imageDirName)
		if _, statErr := os.Stat(filepath.Join(candidate, "Dockerfile")); statErr == nil {
			return candidate
		}
	}

	// 2. 실행 파일 위치 기준 탐색
	execPath, err := os.Executable()
	if err == nil {
		execDir := filepath.Dir(execPath)
		// 실행 파일과 같은 레벨 또는 상위에서 docker/ 디렉토리 탐색
		candidates := []string{
			filepath.Join(execDir, "docker", imageDirName),
			filepath.Join(execDir, "..", "docker", imageDirName),
			filepath.Join(execDir, "..", "..", "docker", imageDirName),
		}
		for _, candidate := range candidates {
			if _, statErr := os.Stat(filepath.Join(candidate, "Dockerfile")); statErr == nil {
				return candidate
			}
		}
	}

	// 3. 현재 작업 디렉토리 기준
	cwd, cwdErr := os.Getwd()
	if cwdErr == nil {
		candidate := filepath.Join(cwd, "docker", imageDirName)
		if _, statErr := os.Stat(filepath.Join(candidate, "Dockerfile")); statErr == nil {
			return candidate
		}
	}

	return ""
}

// cacheDockerfile는 빌드된 Dockerfile을 홈 디렉토리의 캐시 위치에 복사한다.
// 향후 bridge 업데이트 후에도 Dockerfile을 찾을 수 있도록 한다.
func cacheDockerfile(dockerfilePath string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}

	cacheDir := filepath.Join(home, ".config", "autopus", "docker", "chromium-sandbox")
	if _, statErr := os.Stat(cacheDir); os.IsNotExist(statErr) {
		if mkdirErr := os.MkdirAll(cacheDir, 0755); mkdirErr != nil {
			return
		}

		srcDockerfile := filepath.Join(dockerfilePath, "Dockerfile")
		srcData, readErr := os.ReadFile(srcDockerfile)
		if readErr != nil {
			return
		}

		dstDockerfile := filepath.Join(cacheDir, "Dockerfile")
		_ = os.WriteFile(dstDockerfile, srcData, 0644)
	}
}

// stepChromiumSandboxImage는 Chromium Sandbox Docker 이미지와 네트워크를 준비한다.
// Docker가 없으면 건너뛴다 (NON-BLOCKING).
// SPEC-COMPUTER-USE-002 Phase 2.
func stepChromiumSandboxImage() {
	// Docker CLI 존재 여부 확인
	dockerPath, err := exec.LookPath("docker")
	if err != nil {
		printSkip("Docker 미설치 - 이미지 준비 건너뜀")
		return
	}

	// Docker 데몬 실행 여부 확인
	infoCmd := exec.Command(dockerPath, "info")
	if err := infoCmd.Run(); err != nil {
		printSkip("Docker 데몬 미실행 - 이미지 준비 건너뜀")
		return
	}

	const imageName = "autopus/chromium-sandbox:latest"
	const networkName = "autopus-sandbox-net"

	// 이미지 존재 여부 확인
	imgCmd := exec.Command(dockerPath, "images", "-q", imageName)
	imgOutput, imgErr := imgCmd.Output()
	if imgErr != nil || strings.TrimSpace(string(imgOutput)) == "" {
		// 이미지가 없으면 풀 시도
		fmt.Println("  이미지 가져오는 중:", imageName)
		pullCmd := exec.Command(dockerPath, "pull", imageName)
		pullOutput, pullErr := pullCmd.CombinedOutput()
		if pullErr != nil {
			// REQ-UX-004: 풀 실패 시 로컬 Dockerfile에서 자동 빌드 시도
			logger.Debug().Str("output", string(pullOutput)).Msg("이미지 풀 실패")
			fmt.Println("  이미지 다운로드 실패. 로컬에서 빌드를 시도합니다...")

			dockerfilePath := findDockerfileDir("chromium-sandbox")
			if dockerfilePath != "" {
				fmt.Printf("  빌드 중: %s (1~2분 소요)\n", dockerfilePath)
				buildCmd := exec.Command(dockerPath, "build", "-t", imageName, dockerfilePath)
				buildOutput, buildErr := buildCmd.CombinedOutput()
				if buildErr != nil {
					printError("이미지 빌드 실패")
					printDockerBuildFailureGuide(string(buildOutput))
				} else {
					printSuccess(fmt.Sprintf("이미지 빌드 완료: %s", imageName))
					// 빌드 성공 후 Dockerfile을 캐시 디렉토리에 복사
					cacheDockerfile(dockerfilePath)
				}
			} else {
				fmt.Println("  ! Dockerfile을 찾을 수 없습니다")
				fmt.Println("    수동 빌드 방법:")
				fmt.Println("    1. autopus-bridge 소스 디렉토리로 이동")
				fmt.Println("    2. docker build -t autopus/chromium-sandbox:latest docker/chromium-sandbox/")
				fmt.Println()
				fmt.Println("  Computer Use 없이 계속 진행합니다.")
			}
		} else {
			printSuccess(fmt.Sprintf("이미지 준비 완료: %s", imageName))
		}
	} else {
		printSuccess(fmt.Sprintf("이미지 확인됨: %s", imageName))
	}

	// 네트워크 존재 여부 확인
	netCmd := exec.Command(dockerPath, "network", "ls", "--filter", fmt.Sprintf("name=%s", networkName), "-q")
	netOutput, netErr := netCmd.Output()
	if netErr != nil || strings.TrimSpace(string(netOutput)) == "" {
		// 네트워크 생성
		createCmd := exec.Command(dockerPath, "network", "create", networkName)
		if createErr := createCmd.Run(); createErr != nil {
			fmt.Printf("  ! 네트워크 %s 생성 실패: %v\n", networkName, createErr)
		} else {
			printSuccess(fmt.Sprintf("네트워크 생성됨: %s", networkName))
		}
	} else {
		printSuccess(fmt.Sprintf("네트워크 확인됨: %s", networkName))
	}
}

// printBusinessToolSummary 비즈니스 도구 감지 결과를 요약 출력합니다.
func printBusinessToolSummary(tools []businessTool) {
	installed, total := countTools(tools)

	if installed == total {
		printSuccess(fmt.Sprintf("%d/%d 도구 설치됨 (건너뜀)", installed, total))
		return
	}

	for _, t := range tools {
		if t.Installed {
			fmt.Printf("  [v] %-14s %s\n", t.Name, t.Purpose)
		} else {
			fmt.Printf("  [ ] %-14s %s\n", t.Name, t.Purpose)
		}
	}
	fmt.Printf("  합계: %d/%d 설치됨\n", installed, total)
}

// stepInstallMissingTools 미설치 도구 설치를 안내합니다.
func stepInstallMissingTools(tools []businessTool, scanner *bufio.Scanner) {
	missing := filterMissing(tools)
	if len(missing) == 0 {
		printSkip("모든 도구 설치됨")
		return
	}

	// 필수 도구, 권장 도구, 개발자 도구 분리
	var essentialMissing, recommendedMissing []businessTool
	for _, t := range missing {
		switch t.Category {
		case toolCategoryEssential:
			essentialMissing = append(essentialMissing, t)
		case toolCategoryRecommended:
			recommendedMissing = append(recommendedMissing, t)
		case toolCategoryDeveloper:
			recommendedMissing = append(recommendedMissing, t)
		}
	}

	// 필수 도구 미설치 시 강조
	if len(essentialMissing) > 0 {
		names := make([]string, 0, len(essentialMissing))
		for _, t := range essentialMissing {
			names = append(names, t.Name)
		}
		fmt.Printf("  ! 필수 도구 미설치: %s\n", strings.Join(names, ", "))
	}

	targetTools := append(essentialMissing, recommendedMissing...)
	if len(targetTools) == 0 {
		printSkip("필수/권장 도구 모두 설치됨")
		return
	}

	fmt.Printf("  %d개 도구를 설치하시겠습니까? (Y/n): ", len(targetTools))
	if !scanYesNoDefault(scanner, true) {
		printSkip("설치를 건너뜁니다")
		return
	}

	osName := runtime.GOOS
	for _, t := range targetTools {
		installCmd, ok := t.InstallCmd[osName]
		if !ok {
			fmt.Printf("  ! %-14s %s 자동 설치 미지원\n", t.Name, osName)
			continue
		}

		// REQ-UX-003: pipx 명령이 필요한데 미설치인 경우 자동 설치
		if strings.HasPrefix(installCmd, "pipx ") {
			if _, pipxErr := exec.LookPath("pipx"); pipxErr != nil {
				fmt.Println("  pipx가 설치되어 있지 않습니다. 먼저 설치합니다...")
				var pipxInstalled bool
				switch osName {
				case "darwin":
					if _, brewErr := exec.LookPath("brew"); brewErr == nil {
						if brewInstallErr := runInstallCommand("brew install pipx"); brewInstallErr == nil {
							_ = runInstallCommand("pipx ensurepath")
							pipxInstalled = true
							printSuccess("pipx 설치 완료")
						}
					}
				case "linux":
					if aptErr := runInstallCommand("sudo apt-get install -y pipx"); aptErr == nil {
						_ = runInstallCommand("pipx ensurepath")
						pipxInstalled = true
						printSuccess("pipx 설치 완료")
					}
				}
				if !pipxInstalled {
					printError(fmt.Sprintf("pipx 설치 실패. %s 설치를 건너뜁니다", t.Name))
					continue
				}
			}
		}

		fmt.Printf("  설치 중: %s\n", installCmd)
		if err := runInstallCommand(installCmd); err != nil {
			printError(fmt.Sprintf("%s 설치 실패: %v", t.Name, err))
		} else {
			printSuccess(fmt.Sprintf("%s 설치 완료", t.Name))
		}
	}
}

// aiCLIInfo AI CLI 도구 정보
type aiCLIInfo struct {
	Name       string
	CLIName    string
	InstallCmd map[string]string
}

// stepInstallMissingAICLI 미설치 AI CLI 도구 설치를 제안합니다.
func stepInstallMissingAICLI(providers []providerInfo, scanner *bufio.Scanner) []providerInfo {
	aiCLIs := []aiCLIInfo{
		{
			Name:    "Claude Code",
			CLIName: "claude",
			InstallCmd: map[string]string{
				"darwin": "npm install -g @anthropic-ai/claude-code",
				"linux":  "npm install -g @anthropic-ai/claude-code",
			},
		},
		{
			Name:    "Codex CLI",
			CLIName: "codex",
			InstallCmd: map[string]string{
				"darwin": "npm install -g @openai/codex",
				"linux":  "npm install -g @openai/codex",
			},
		},
		{
			Name:    "Gemini CLI",
			CLIName: "gemini",
			InstallCmd: map[string]string{
				"darwin": "npm install -g @google/gemini-cli",
				"linux":  "npm install -g @google/gemini-cli",
			},
		},
	}

	// 미설치 CLI 탐색
	var missing []aiCLIInfo
	for _, cli := range aiCLIs {
		found := false
		for _, p := range providers {
			if (p.Name == "Claude" && cli.CLIName == "claude") ||
				(p.Name == "Codex" && cli.CLIName == "codex") ||
				(p.Name == "Gemini" && cli.CLIName == "gemini") {
				if p.HasCLI {
					found = true
				}
				break
			}
		}
		if !found {
			missing = append(missing, cli)
		}
	}

	if len(missing) == 0 {
		printSuccess("모든 AI CLI 설치됨")
		return providers
	}

	// npm 사용 가능 여부 확인
	if _, err := exec.LookPath("npm"); err != nil {
		// macOS에서 Homebrew를 통한 자동 설치 시도
		if runtime.GOOS == "darwin" {
			// Homebrew 사용 가능 여부 확인
			if _, brewErr := exec.LookPath("brew"); brewErr == nil {
				fmt.Println("  ! npm이 설치되어 있지 않습니다. Homebrew로 Node.js를 설치합니다.")
				fmt.Printf("    Node.js를 설치하시겠습니까? (Y/n): ")

				if scanYesNoDefault(scanner, true) {
					installCmd := "brew install node"
					fmt.Printf("  설치 중: %s\n", installCmd)
					if runErr := runInstallCommand(installCmd); runErr != nil {
						printError(fmt.Sprintf("Node.js 설치 실패: %v", runErr))
						return providers
					}

					// npm 재확인
					if _, npmCheckErr := exec.LookPath("npm"); npmCheckErr != nil {
						printError("Node.js 설치 후에도 npm을 찾을 수 없습니다")
						return providers
					}
					printSuccess("Node.js 설치 완료")
				} else {
					printSkip("Node.js 설치 건너뜀")
					return providers
				}
			} else {
				// Homebrew가 없는 경우
				fmt.Println("  ! npm이 설치되어 있지 않습니다. AI CLI 설치에 Node.js가 필요합니다.")
				fmt.Println("    Homebrew를 설치한 후 brew install node 로 Node.js를 설치하세요.")
				fmt.Println("    Homebrew 설치: https://brew.sh")
				return providers
			}
		} else {
			// 비 macOS 시스템
			fmt.Println("  ! npm이 설치되어 있지 않습니다. AI CLI 설치에 Node.js가 필요합니다.")
			fmt.Println("    https://nodejs.org 에서 Node.js를 먼저 설치하세요.")
			return providers
		}
	}

	// 미설치 목록 표시 및 설치 여부 확인
	fmt.Println()
	fmt.Println("  미설치 AI CLI:")
	for _, cli := range missing {
		fmt.Printf("    [ ] %-14s %s\n", cli.CLIName, cli.Name)
	}
	fmt.Printf("\n  %d개 AI CLI를 설치하시겠습니까? (Y/n): ", len(missing))

	if !scanYesNoDefault(scanner, true) {
		printSkip("AI CLI 설치 건너뜀")
		return providers
	}

	osName := runtime.GOOS
	for _, cli := range missing {
		installCmd, ok := cli.InstallCmd[osName]
		if !ok {
			fmt.Printf("  ! %-14s %s 자동 설치 미지원\n", cli.Name, osName)
			continue
		}

		fmt.Printf("  설치 중: %s\n", installCmd)
		if err := runInstallCommand(installCmd); err != nil {
			printError(fmt.Sprintf("%s 설치 실패: %v", cli.Name, err))
		} else {
			printSuccess(fmt.Sprintf("%s 설치 완료", cli.Name))
		}
	}

	// 설치 후 프로바이더 재감지
	fmt.Println()
	return detectProviders()
}

// stepAISubscriptionAuth는 AI CLI 인증 상태를 확인하고 미인증 시 안내합니다.
// 인증 실패 시에도 up 명령을 중단하지 않습니다 (NON-BLOCKING).
// SPEC-BRIDGE-AUTH-001
func stepAISubscriptionAuth(providers []providerInfo, scanner *bufio.Scanner) []providerInfo {
	anyDetected := false
	anyUnauthenticated := false

	for i, p := range providers {
		if !p.HasCLI && !p.HasAPIKey {
			continue
		}
		anyDetected = true

		var authResult aitools.AuthCheckResult
		switch p.Name {
		case "Claude":
			authResult = aitools.CheckClaudeAuth()
		case "Codex":
			authResult = aitools.CheckCodexAuth()
		case "Gemini":
			authResult = aitools.CheckGeminiAuth()
		default:
			continue
		}

		// providerInfo 업데이트
		providers[i].CLIAuthenticated = authResult.CLIAuthenticated

		// 상태 출력
		switch authResult.Status {
		case aitools.AuthStatusAuthenticated:
			printSuccess(fmt.Sprintf("%s: 인증됨", p.Name))
		case aitools.AuthStatusAPIKeyOnly:
			printSuccess(fmt.Sprintf("%s: API 키 설정됨 (%s)", p.Name, authResult.APIKeyEnvName))
		case aitools.AuthStatusNotAuthenticated:
			fmt.Printf("  [ ] %s: 미인증\n", p.Name)
			anyUnauthenticated = true
		default:
			fmt.Printf("  ? %s: 상태 불명\n", p.Name)
		}
	}

	if !anyDetected {
		fmt.Println("  ! 감지된 AI 프로바이더가 없습니다.")
		fmt.Println("    AI CLI를 설치하거나 API 키 환경변수를 설정하세요.")
		return providers
	}

	// 미인증 프로바이더에 대한 안내
	if anyUnauthenticated {
		fmt.Println()
		providers = showAuthGuide(providers, scanner)
	}

	// ChatGPT 구독 감지 (Codex)
	providers = detectChatGPTSubscription(providers, scanner)

	// 연결 테스트 제안 (기본: 건너뛰기)
	offerConnectionTest(providers, scanner)

	return providers
}

// showAuthGuide는 미인증 프로바이더에 대한 인증 가이드를 표시합니다.
func showAuthGuide(providers []providerInfo, scanner *bufio.Scanner) []providerInfo {
	for i, p := range providers {
		if !p.HasCLI || p.CLIAuthenticated || p.HasAPIKey {
			continue
		}

		switch p.Name {
		case "Claude":
			fmt.Println("  Claude Code가 인증되지 않았습니다.")
			fmt.Println("    인증 방법:")
			fmt.Println("    1) claude login 실행 (브라우저 인증)")
			fmt.Println("    2) export ANTHROPIC_API_KEY=<your-key> 설정")
			fmt.Println("       API 키 발급: https://console.anthropic.com/settings/keys")
		case "Codex":
			fmt.Println("  Codex CLI가 인증되지 않았습니다.")
			fmt.Println("    인증 방법:")
			fmt.Println("    1) codex auth 실행 (브라우저 인증)")
			fmt.Println("    2) export OPENAI_API_KEY=<your-key> 설정")
			fmt.Println("       API 키 발급: https://platform.openai.com/api-keys")
		case "Gemini":
			fmt.Println("  Gemini CLI가 인증되지 않았습니다.")
			fmt.Println("    인증 방법:")
			fmt.Println("    1) gemini auth 실행 (브라우저 인증)")
			fmt.Println("    2) export GEMINI_API_KEY=<your-key> 설정")
			fmt.Println("       API 키 발급: https://aistudio.google.com/apikey")
		}
		fmt.Println()

		// 인증 진행 여부 질문
		fmt.Printf("  %s 인증을 지금 진행하시겠습니까? (y/N): ", p.Name)
		if scanYesNo(scanner) {
			if runAuthCommand(p.Name) {
				// 인증 성공 후 상태 재확인
				var recheck aitools.AuthCheckResult
				switch p.Name {
				case "Claude":
					recheck = aitools.CheckClaudeAuth()
				case "Codex":
					recheck = aitools.CheckCodexAuth()
				case "Gemini":
					recheck = aitools.CheckGeminiAuth()
				}
				providers[i].CLIAuthenticated = recheck.CLIAuthenticated
				if recheck.CLIAuthenticated {
					printSuccess(fmt.Sprintf("%s 인증 완료", p.Name))
				} else {
					fmt.Printf("  ! %s 인증이 완료되지 않았습니다. 나중에 다시 시도하세요.\n", p.Name)
				}
			}
		} else {
			printSkip(fmt.Sprintf("%s 인증 건너뜀", p.Name))
		}
	}

	return providers
}

// runAuthCommand는 CLI 인증 명령을 실행합니다.
// 성공하면 true, 실패하면 false를 반환합니다.
func runAuthCommand(providerName string) bool {
	var cmdName string
	var cmdArgs []string

	switch providerName {
	case "Claude":
		cmdName = "claude"
		cmdArgs = []string{"login"}
	case "Codex":
		cmdName = "codex"
		cmdArgs = []string{"auth"}
	case "Gemini":
		cmdName = "gemini"
		cmdArgs = []string{"auth"}
	default:
		return false
	}

	fmt.Printf("  %s %s 실행 중...\n", cmdName, strings.Join(cmdArgs, " "))

	cmd := exec.Command(cmdName, cmdArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		printError(fmt.Sprintf("%s 인증 실행 실패: %v", providerName, err))
		return false
	}

	return true
}

// detectChatGPTSubscription는 Codex CLI가 미인증+API키 없을 때 ChatGPT 구독 여부를 확인합니다.
// SPEC-BRIDGE-AUTH-001 REQ-BA-003
func detectChatGPTSubscription(providers []providerInfo, scanner *bufio.Scanner) []providerInfo {
	for i, p := range providers {
		if p.Name != "Codex" || p.CLIAuthenticated || p.HasAPIKey {
			continue
		}
		if !p.HasCLI {
			continue
		}

		fmt.Println("  ChatGPT Plus/Pro 구독이 있으신가요?")
		fmt.Println("  구독이 있으면 Codex를 app-server 모드로 사용할 수 있습니다.")
		fmt.Printf("  ChatGPT Plus/Pro 구독 보유 (Y/n): ")

		if scanYesNoDefault(scanner, true) {
			providers[i].ChatGPTAuth = true
			printSuccess("Codex 모드: app-server (ChatGPT 인증 사용)")
			fmt.Println("    ChatGPT 인증 안내:")
			fmt.Println("    codex auth 실행 후 ChatGPT 계정으로 로그인하세요.")
			fmt.Println()

			// ChatGPT 인증 진행 여부
			fmt.Printf("  지금 ChatGPT 인증을 진행하시겠습니까? (y/N): ")
			if scanYesNo(scanner) {
				if runAuthCommand("Codex") {
					providers[i].CLIAuthenticated = true
					printSuccess("Codex ChatGPT 인증 완료")
				}
			}
		} else {
			fmt.Println("    API 키를 설정하세요:")
			fmt.Println("    export OPENAI_API_KEY=<your-key>")
			fmt.Println("    API 키 발급: https://platform.openai.com/api-keys")
		}
	}
	return providers
}

// offerConnectionTest는 인증된 프로바이더에 대한 연결 테스트를 제안합니다.
// 기본값은 건너뛰기(N)로 크레딧 소모를 방지합니다.
// SPEC-BRIDGE-AUTH-001 REQ-BA-004
func offerConnectionTest(providers []providerInfo, scanner *bufio.Scanner) {
	// 인증된 프로바이더 목록 생성
	var authenticatedProviders []string
	for _, p := range providers {
		if p.CLIAuthenticated || p.HasAPIKey {
			authenticatedProviders = append(authenticatedProviders, p.Name)
		}
	}

	if len(authenticatedProviders) == 0 {
		return
	}

	fmt.Println()
	fmt.Printf("  연결 테스트를 수행하시겠습니까? (y/N): ")
	if !scanYesNo(scanner) {
		printSkip("연결 테스트 건너뜀")
		return
	}

	fmt.Println("  연결 테스트 중...")
	validator := aitools.NewValidator()
	ctx := context.Background()

	for _, name := range authenticatedProviders {
		var result aitools.ValidationResult
		switch name {
		case "Claude":
			result = validator.ValidateClaudeCLI(ctx)
		case "Codex":
			result = validator.ValidateCodexCLI(ctx)
		case "Gemini":
			result = validator.ValidateGeminiCLI(ctx)
		default:
			continue
		}

		switch result.Status {
		case aitools.ValidationSuccess:
			printSuccess(fmt.Sprintf("%s: 연결 성공 (%.1f초)", name, result.ResponseTime.Seconds()))
		case aitools.ValidationAuthFailure:
			printError(fmt.Sprintf("%s: 인증 실패", name))
		case aitools.ValidationTimeout:
			printError(fmt.Sprintf("%s: 연결 시간 초과", name))
		case aitools.ValidationRateLimit:
			fmt.Printf("  ! %s: 요청 제한 (나중에 다시 시도하세요)\n", name)
		default:
			printError(fmt.Sprintf("%s: 연결 실패", name))
		}
	}
}

// stepAIToolMCPConfig는 감지된 AI CLI 도구에 Autopus MCP를 설정합니다.
func stepAIToolMCPConfig(providers []providerInfo, scanner *bufio.Scanner) {
	configured := 0

	for _, p := range providers {
		if !p.HasCLI {
			continue
		}

		switch p.Name {
		case "Claude":
			// MCP 설정
			fmt.Printf("  Claude Code MCP 자동 설정을 진행할까요? (Y/n): ")
			if scanYesNoDefault(scanner, true) {
				if err := aitools.ConfigureClaudeCodeMCP(""); err != nil {
					printError(fmt.Sprintf("Claude Code MCP 설정 실패: %v", err))
				} else {
					printSuccess("Claude Code MCP 설정 완료 (~/.claude/.mcp.json)")
					configured++
				}
			} else {
				printSkip("Claude Code MCP 설정 건너뜀")
			}

		case "Codex":
			fmt.Printf("  Codex CLI MCP 자동 설정을 진행할까요? (Y/n): ")
			if scanYesNoDefault(scanner, true) {
				if err := aitools.ConfigureCodexMCP(); err != nil {
					printError(fmt.Sprintf("Codex CLI MCP 설정 실패: %v", err))
				} else {
					printSuccess("Codex CLI MCP 설정 완료 (~/.codex/config.toml)")
					configured++
				}
			} else {
				printSkip("Codex CLI MCP 설정 건너뜀")
			}

			// Agent Skills 설치 (Gemini/Codex 공유 경로)
			if !aitools.IsAgentSkillInstalled() {
				fmt.Printf("  Autopus Agent Skill을 설치할까요? (Y/n): ")
				if scanYesNoDefault(scanner, true) {
					if err := aitools.InstallAgentSkill(); err != nil {
						printError(fmt.Sprintf("Agent Skill 설치 실패: %v", err))
					} else {
						printSuccess("Agent Skill 설치 완료 (~/.agents/skills/autopus-platform/)")
						configured++
					}
				} else {
					printSkip("Agent Skill 설치 건너뜀")
				}
			} else {
				printSuccess("Autopus Agent Skill 이미 설치됨")
			}

		case "Gemini":
			fmt.Printf("  Gemini CLI MCP 자동 설정을 진행할까요? (Y/n): ")
			if scanYesNoDefault(scanner, true) {
				if err := aitools.ConfigureGeminiMCP(); err != nil {
					printError(fmt.Sprintf("Gemini CLI MCP 설정 실패: %v", err))
				} else {
					printSuccess("Gemini CLI MCP 설정 완료 (~/.gemini/settings.json)")
					configured++
				}
			} else {
				printSkip("Gemini CLI MCP 설정 건너뜀")
			}

			// Agent Skills 설치 (Gemini/Codex 공유 경로)
			if !aitools.IsAgentSkillInstalled() {
				fmt.Printf("  Autopus Agent Skill을 설치할까요? (Y/n): ")
				if scanYesNoDefault(scanner, true) {
					if err := aitools.InstallAgentSkill(); err != nil {
						printError(fmt.Sprintf("Agent Skill 설치 실패: %v", err))
					} else {
						printSuccess("Agent Skill 설치 완료 (~/.agents/skills/autopus-platform/)")
						configured++
					}
				} else {
					printSkip("Agent Skill 설치 건너뜀")
				}
			} else {
				printSuccess("Autopus Agent Skill 이미 설치됨")
			}
		}
	}

	if configured == 0 {
		fmt.Println("  감지된 AI CLI가 없거나 MCP 설정을 건너뛰었습니다.")
	} else {
		printSuccess(fmt.Sprintf("%d개 AI 도구 MCP 설정 완료", configured))
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Device Auth Flow (reused from login.go logic, non-duplicated)
// ─────────────────────────────────────────────────────────────────────────────

// performDeviceAuthFlow runs the complete Device Authorization Flow with PKCE and returns credentials.
func performDeviceAuthFlow() (*auth.Credentials, error) {
	apiBaseURL := getAPIBaseURL()

	// Step 1: PKCE 키 쌍 생성
	pkce, err := auth.GeneratePKCE()
	if err != nil {
		return nil, fmt.Errorf("PKCE 생성 실패: %w", err)
	}

	// Step 2: Request device code (PKCE code_challenge 포함)
	deviceResp, err := auth.RequestDeviceCode(apiBaseURL, pkce)
	if err != nil {
		return nil, fmt.Errorf("디바이스 코드 요청 실패: %w", err)
	}

	// Step 3: Display auth code
	fmt.Printf("  인증 코드: %s\n", deviceResp.UserCode)
	fmt.Println()
	fmt.Printf("  다음 URL에서 위 코드를 입력하세요:\n")
	fmt.Printf("  %s\n", deviceResp.VerificationURI)
	fmt.Println()

	// Step 4: Open browser
	openURL := deviceResp.VerificationURIComplete
	if openURL == "" {
		openURL = deviceResp.VerificationURI
	}
	if browserErr := openBrowser(openURL); browserErr != nil {
		fmt.Printf("  브라우저에서 직접 위 URL을 열어주세요.\n\n")
	}

	// Step 5: Poll for token (PKCE code_verifier 포함)
	ctx, cancel := context.WithTimeout(context.Background(), loginTimeout)
	defer cancel()

	expiresAt := time.Now().Add(time.Duration(deviceResp.ExpiresIn) * time.Second)
	stopSpinner := make(chan struct{})
	go spinnerLoop(stopSpinner, expiresAt)

	tokenResp, err := auth.PollDeviceToken(ctx, apiBaseURL, deviceResp.DeviceCode, deviceResp.Interval, pkce.CodeVerifier)
	close(stopSpinner)
	fmt.Printf("\r%s\r", strings.Repeat(" ", 50))

	if err != nil {
		return nil, fmt.Errorf("인증 실패: %w", err)
	}

	// Step 6: Save credentials
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

	// 워크스페이스 자동 선택 (1개일 경우)
	if creds.WorkspaceID == "" && len(tokenResp.Workspaces) == 1 {
		creds.WorkspaceID = tokenResp.Workspaces[0].ID
		creds.WorkspaceSlug = tokenResp.Workspaces[0].Slug
		creds.WorkspaceName = tokenResp.Workspaces[0].Name
	}

	if saveErr := auth.Save(creds); saveErr != nil {
		return nil, fmt.Errorf("인증 정보 저장 실패: %w", saveErr)
	}

	return creds, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Workspace API
// ─────────────────────────────────────────────────────────────────────────────

// fetchWorkspaces retrieves the list of workspaces from the API.
func fetchWorkspaces(apiBaseURL, accessToken string) ([]workspaceInfo, error) {
	url := apiBaseURL + "/api/v1/workspaces"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("요청 생성 실패: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("서버 통신 실패: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("인증이 만료되었습니다. 'autopus-bridge login'으로 다시 로그인하세요")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("워크스페이스 조회 실패 (HTTP %d)", resp.StatusCode)
	}

	var wsResp workspacesResponse
	if err := json.NewDecoder(resp.Body).Decode(&wsResp); err != nil {
		return nil, fmt.Errorf("응답 파싱 실패: %w", err)
	}

	if !wsResp.Success {
		return nil, fmt.Errorf("서버가 실패 응답을 반환했습니다")
	}

	return wsResp.Data, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Visual Progress Helpers
// ─────────────────────────────────────────────────────────────────────────────

func printStep(step, total int, msg string) {
	fmt.Printf("\n[%d/%d] %s\n", step, total, msg)
}

func printSuccess(msg string) {
	fmt.Printf("  ✓ %s\n", msg)
}

func printSkip(msg string) {
	fmt.Printf("  - %s (건너뜀)\n", msg)
}

func printError(msg string) {
	fmt.Printf("  ✗ %s\n", msg)
}

// printProviderSummary displays a compact provider detection summary.
func printProviderSummary(providers []providerInfo) {
	anyFound := false
	for _, p := range providers {
		if p.HasCLI || p.HasAPIKey {
			anyFound = true
			modes := []string{}
			if p.HasCLI {
				modes = append(modes, "CLI")
			}
			if p.HasAPIKey {
				modes = append(modes, "API")
			}
			printSuccess(fmt.Sprintf("%s (%s)", p.Name, strings.Join(modes, "+")))
		}
	}

	if !anyFound {
		fmt.Println("  ! 감지된 프로바이더가 없습니다.")
		fmt.Println("    AI CLI를 설치하거나 API 키 환경변수를 설정하세요.")
		fmt.Println("    export ANTHROPIC_API_KEY=<your-key>")
	}
}

// printFixSuggestion prints context-specific fix suggestions for failures.
func printFixSuggestion(stepName string, err error) {
	fmt.Println()
	fmt.Println("  해결 방법:")

	switch stepName {
	case "auth":
		fmt.Println("    1. 인터넷 연결을 확인하세요")
		fmt.Println("    2. Autopus 서버가 실행 중인지 확인하세요")
		fmt.Println("    3. 'autopus-bridge login'으로 수동 로그인을 시도하세요")
	case "token_refresh":
		fmt.Println("    1. 'autopus-bridge logout && autopus-bridge login'으로 재로그인하세요")
		fmt.Println("    2. 서버 연결 상태를 확인하세요")
	case "workspace":
		fmt.Println("    1. 웹 대시보드에서 워크스페이스를 생성하세요")
		fmt.Println("    2. 계정에 워크스페이스 접근 권한이 있는지 확인하세요")
		fmt.Println("    3. 'autopus-bridge login'으로 재로그인 후 다시 시도하세요")
	case "config":
		fmt.Println("    1. ~/.config/autopus/ 디렉토리 쓰기 권한을 확인하세요")
		fmt.Println("    2. 'autopus-bridge setup'으로 수동 설정을 시도하세요")
	case "connection":
		fmt.Println("    서버 연결에 실패했습니다.")
		fmt.Println()
		fmt.Println("    다음을 확인해 주세요:")
		fmt.Println("    1. 인터넷에 연결되어 있는지 확인하세요")
		fmt.Println("    2. 'autopus-bridge up --force'로 처음부터 다시 시도하세요")
		fmt.Println("    3. 문제가 지속되면 'autopus-bridge up -v'로 상세 로그를 확인하세요")
	default:
		fmt.Println("    1. 'autopus-bridge up --force'로 처음부터 다시 시도하세요")
		fmt.Println("    2. 문제가 지속되면 'autopus-bridge up -v'로 상세 로그를 확인하세요")
	}

	fmt.Println()
	fmt.Println("  재실행 시 완료된 단계는 자동으로 건너뜁니다.")
	fmt.Println("  처음부터 다시 시작하려면: autopus-bridge up --force")
	fmt.Println()
}

// ─────────────────────────────────────────────────────────────────────────────
// Progress Tracking (resume from failed step)
// ─────────────────────────────────────────────────────────────────────────────

// upProgressFilePath returns the path to the progress tracking file.
func upProgressFilePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "autopus", ".up-progress.json")
}

// loadUpProgress reads progress from the temp file.
func loadUpProgress() *upProgress {
	path := upProgressFilePath()
	if path == "" {
		return &upProgress{}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return &upProgress{}
	}

	var p upProgress
	if err := json.Unmarshal(data, &p); err != nil {
		return &upProgress{}
	}

	// Expire progress older than 1 hour
	if time.Since(p.UpdatedAt) > 1*time.Hour {
		clearUpProgress()
		return &upProgress{}
	}

	return &p
}

// saveUpProgress writes progress to the temp file.
func saveUpProgress(p *upProgress, failedStep int, errMsg string) {
	path := upProgressFilePath()
	if path == "" {
		return
	}

	p.UpdatedAt = time.Now()
	if failedStep > 0 {
		p.LastStep = failedStep
		p.LastError = errMsg
	}

	// Ensure config directory exists
	if err := config.EnsureConfigDir(); err != nil {
		return
	}

	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return
	}

	_ = os.WriteFile(path, data, 0600)
}

// clearUpProgress removes the progress file.
func clearUpProgress() {
	path := upProgressFilePath()
	if path != "" {
		_ = os.Remove(path)
	}
}

// isStepCompleted checks if a step has been completed in this session.
func isStepCompleted(p *upProgress, step int) bool {
	for _, s := range p.CompletedSteps {
		if s == step {
			return true
		}
	}
	return false
}

// markStepCompleted adds a step to the completed list.
func markStepCompleted(p *upProgress, step int) {
	if !isStepCompleted(p, step) {
		p.CompletedSteps = append(p.CompletedSteps, step)
	}
}

// removeStep removes a step from the completed list (used when re-validation fails).
func removeStep(p *upProgress, step int) *upProgress {
	var newSteps []int
	for _, s := range p.CompletedSteps {
		if s != step {
			newSteps = append(newSteps, s)
		}
	}
	p.CompletedSteps = newSteps
	return p
}
