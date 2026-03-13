// Package cmd는 Local Agent Bridge CLI의 명령어를 정의합니다.
// connect.go는 서버 연결 명령을 구현합니다.
// REQ-E-01: connect 명령으로 WebSocket 연결 수립
// REQ-E-09: SIGINT/SIGTERM 시그널 시 graceful shutdown
// REQ-U-03: 종료 시 정상적인 연결 해제
package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/insajin/autopus-agent-protocol"
	embeddedDocker "github.com/insajin/autopus-bridge/docker"
	"github.com/insajin/autopus-bridge/internal/auth"
	"github.com/insajin/autopus-bridge/internal/authwatch"
	"github.com/insajin/autopus-bridge/internal/bridgecontext"
	"github.com/insajin/autopus-bridge/internal/computeruse"
	"github.com/insajin/autopus-bridge/internal/config"
	"github.com/insajin/autopus-bridge/internal/executor"
	"github.com/insajin/autopus-bridge/internal/logger"
	"github.com/insajin/autopus-bridge/internal/mcp"
	"github.com/insajin/autopus-bridge/internal/project"
	"github.com/insajin/autopus-bridge/internal/provider"
	"github.com/insajin/autopus-bridge/internal/scheduler"
	"github.com/insajin/autopus-bridge/internal/websocket"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	// 연결 명령 플래그
	serverURL      string
	token          string
	connectTimeout int
	connectReplace bool

	connectProcessRunningFn = isProcessRunning
	connectStopProcessFn    = stopRunningConnectProcess
)

const connectLockFileName = "connect.lock"

// connectLock는 connect 단일 인스턴스 실행을 보장하기 위한 락 파일 핸들입니다.
type connectLock struct {
	path string
	pid  int
}

// connectCmd는 서버에 연결하는 명령어입니다.
// REQ-E-01: connect 명령 시 WebSocket 연결 수립 및 agent_connect 메시지 전송
var connectCmd = &cobra.Command{
	Use:   "connect",
	Short: "Autopus 서버에 연결합니다",
	Long: `WebSocket을 통해 Autopus 서버에 연결하고 작업 요청을 처리합니다.

연결이 수립되면 하트비트를 주기적으로 전송하고,
서버로부터 작업 요청을 수신하여 로컬 AI 프로바이더를 통해 실행합니다.

SIGINT(Ctrl+C) 또는 SIGTERM 시그널을 수신하면 정상적으로 연결을 종료합니다.`,
	RunE: runConnect,
}

func init() {
	rootCmd.AddCommand(connectCmd)

	// connect 명령 플래그
	connectCmd.Flags().StringVar(&serverURL, "server", "",
		"서버 URL (기본값: 설정 파일 또는 wss://api.autopus.co/ws/agent)")
	connectCmd.Flags().StringVar(&token, "token", "",
		"JWT 토큰 (또는 LAB_TOKEN 환경변수)")
	connectCmd.Flags().IntVar(&connectTimeout, "timeout", 30,
		"연결 타임아웃(초)")
	connectCmd.Flags().BoolVar(&connectReplace, "replace", false,
		"기존 bridge 연결 프로세스가 있으면 종료 후 새 세션으로 교체")
}

// runConnect는 connect 명령의 실행 로직입니다.
func runConnect(cmd *cobra.Command, args []string) error {
	return runConnectWithOptions(cmd, args, connectRunOptions{
		ReplaceExisting: connectReplace,
	})
}

type connectRunOptions struct {
	ReplaceExisting bool
}

func runConnectWithOptions(cmd *cobra.Command, args []string, opts connectRunOptions) error {
	scopeWorkspaceID := resolveCurrentWorkspaceScopeID()

	// 단일 인스턴스 보장: 기존 connect 프로세스가 있으면 중복 실행 방지
	lock, err := acquireConnectLock(scopeWorkspaceID, opts.ReplaceExisting)
	if err != nil {
		return err
	}
	defer lock.Release()

	// 설정 로드
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("설정 로드 실패: %w", err)
	}

	// 서버 URL 결정 (플래그 > 환경변수 > 설정파일)
	srvURL := serverURL
	if srvURL == "" {
		srvURL = viper.GetString("server.url")
	}
	if srvURL == "" {
		srvURL = "wss://api.autopus.co/ws/agent"
	}

	// JWT 토큰 결정 (플래그 > 환경변수 > 저장된 credential)
	authToken := token
	if authToken == "" {
		authToken = os.Getenv("LAB_TOKEN")
	}
	if authToken == "" {
		// Try to load from stored credentials
		creds, credErr := auth.Load()
		if credErr != nil {
			logger.Warn().Err(credErr).Msg("저장된 인증 정보 로드 실패")
		} else if creds != nil {
			if creds.IsValid() {
				authToken = creds.AccessToken
				logger.Info().
					Str("email", creds.UserEmail).
					Msg("저장된 인증 정보 사용")
			} else if creds.RefreshToken != "" {
				// 만료된 토큰 자동 갱신 시도
				logger.Info().Msg("토큰이 만료되어 자동 갱신을 시도합니다...")
				if err := auth.RefreshAccessToken(creds); err != nil {
					// REQ-UX-002: 갱신 실패 시 자동 재인증 시도
					logger.Warn().Err(err).Msg("토큰 자동 갱신 실패, 브라우저 재인증 시도")
					fmt.Println("  토큰 갱신에 실패했습니다. 브라우저에서 재인증을 시작합니다...")
					fmt.Println()
					newCreds, authErr := performBrowserAuthWithFallback()
					if authErr != nil {
						return fmt.Errorf("재인증 실패: %w", authErr)
					}
					authToken = newCreds.AccessToken
					creds = newCreds
					fmt.Printf("  ✓ 재인증 성공: %s\n", newCreds.UserEmail)
				} else {
					authToken = creds.AccessToken
					logger.Info().
						Str("email", creds.UserEmail).
						Msg("토큰 자동 갱신 성공")
				}
			} else {
				// REQ-UX-002: refresh token 없이 만료된 경우에도 자동 재인증
				logger.Warn().Msg("저장된 인증 정보가 만료되었습니다. 브라우저 재인증 시도")
				fmt.Println("  저장된 인증 정보가 만료되었습니다. 브라우저에서 재인증을 시작합니다...")
				fmt.Println()
				newCreds, authErr := performBrowserAuthWithFallback()
				if authErr != nil {
					return fmt.Errorf("재인증 실패: %w", authErr)
				}
				authToken = newCreds.AccessToken
				creds = newCreds
				fmt.Printf("  ✓ 재인증 성공: %s\n", newCreds.UserEmail)
			}
		}
	}
	if authToken == "" {
		return fmt.Errorf("JWT 토큰이 필요합니다. 'lab login'으로 로그인하거나 --token 플래그를 사용하세요")
	}

	// REQ-S-05: 설정된 AI 프로바이더가 없으면 연결 거부
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("설정 검증 실패: %w", err)
	}

	// 컨텍스트 생성 (graceful shutdown용)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// REQ-E-09: SIGINT/SIGTERM 시그널 핸들링
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// 프로바이더 레지스트리 초기화
	registry, err := initializeProviders(ctx, cfg)
	if err != nil {
		return fmt.Errorf("프로바이더 초기화 실패: %w", err)
	}

	// 프로바이더 오버라이드 설정 적용
	if cfg.Providers.Override.Provider != "" {
		internalName := provider.ToInternalName(cfg.Providers.Override.Provider)
		registry.SetOverride(provider.OverrideConfig{
			ProviderName: internalName,
			Model:        cfg.Providers.Override.Model,
		})
		logger.Info().
			Str("provider", internalName).
			Str("model", cfg.Providers.Override.Model).
			Msg("프로바이더 오버라이드 설정 적용")
	}

	logger.Info().
		Strs("providers", registry.List()).
		Msg("AI 프로바이더 초기화 완료")

	// SPEC-COMPUTER-USE-002: 컨테이너 풀 초기화 (Docker 사용 가능 시)
	var containerPool *computeruse.ContainerPool
	cuHandler := computeruse.NewHandler()

	if cfg.ComputerUse.IsContainerMode() {
		poolCtx, poolCancel := context.WithTimeout(ctx, 60*time.Second)
		pool, poolErr := computeruse.InitContainerPool(poolCtx, computeruse.ComputerUseConfigInput{
			MaxContainers:      cfg.ComputerUse.MaxContainers,
			WarmPoolSize:       cfg.ComputerUse.WarmPoolSize,
			Image:              cfg.ComputerUse.Image,
			ContainerMemory:    cfg.ComputerUse.ContainerMemory,
			ContainerCPU:       cfg.ComputerUse.ContainerCPU,
			IdleTimeout:        cfg.ComputerUse.IdleTimeout,
			Network:            cfg.ComputerUse.Network,
			EmbeddedDockerfile: embeddedDocker.ChromiumSandboxDockerfile,
		})
		poolCancel()

		if poolErr != nil {
			logger.Warn().Err(poolErr).Msg("컨테이너 풀 초기화 실패, 로컬 모드로 폴백")
		} else {
			containerPool = pool
			cuHandler = computeruse.NewHandler(computeruse.WithContainerPool(pool))
			logger.Info().
				Int("max_containers", cfg.ComputerUse.MaxContainers).
				Int("warm_pool_size", cfg.ComputerUse.WarmPoolSize).
				Msg("컨테이너 풀 초기화 완료")
		}
	}

	// 버전 정보 가져오기
	version, _, _ := GetVersionInfo()
	if version == "" {
		version = "dev"
	}

	// FR-P1-08: 백그라운드 업데이트 확인 (24시간 간격, 비차단)
	CheckUpdateInBackground(version)

	// 재연결 전략 설정
	// 인자 순서: initialDelay, maxDelay, multiplier, maxAttempts
	reconnectStrategy := websocket.NewReconnectStrategy(
		time.Duration(cfg.Reconnection.InitialDelayMs)*time.Millisecond,
		time.Duration(cfg.Reconnection.MaxDelayMs)*time.Millisecond,
		cfg.Reconnection.BackoffMultiplier,
		cfg.Reconnection.MaxAttempts,
	)

	// SPEC-BRIDGE-GATEWAY-001: 프로바이더 capabilities 맵 구성
	// 백엔드는 OpenRouter 정규 이름(anthropic/openai/google)을 사용하므로
	// 내부 이름(claude/codex/gemini)을 정규 이름으로 변환하여 전송.
	// ValidateConfig()를 통과하는 프로바이더만 capabilities에 포함하여
	// API 키 없는 프로바이더로의 태스크 라우팅을 방지합니다.
	providerCaps := make(map[string]bool)
	for _, p := range registry.ListProviders() {
		if err := p.ValidateConfig(); err == nil {
			canonicalName := provider.ToCanonicalName(p.Name())
			providerCaps[canonicalName] = true
		} else {
			logger.Warn().
				Str("provider", p.Name()).
				Err(err).
				Msg("프로바이더 설정 검증 실패, capabilities에서 제외")
		}
	}

	// WebSocket 클라이언트 생성 (단일 인스턴스)
	runtimeContext, runtimeRoot := loadBridgeRuntimeContext()
	connectWorkspaceID := resolveCurrentWorkspaceScopeID()
	client := websocket.NewClient(
		srvURL,
		authToken,
		version,
		websocket.WithCapabilities(cfg.GetAvailableProviders()),
		websocket.WithProviderCapabilities(providerCaps),
		websocket.WithWorkspaceID(connectWorkspaceID),
		websocket.WithRuntimeContext(runtimeContext),
		websocket.WithReconnectStrategy(reconnectStrategy),
	)

	// SPEC-HOTSWAP-001: authwatch 시작 - 인증 파일 변경 감지 및 hot-swap 지원
	authWatcher := startAuthWatcher(ctx, registry, client)
	if authWatcher != nil {
		defer authWatcher.Stop()
	}

	// 인증 실패 콜백 등록: 토큰 만료 시 자동 재인증 시도
	client.SetOnAuthFailure(func(authErr error) {
		if errors.Is(authErr, websocket.ErrAuthExpired) {
			fmt.Println("\n  세션 토큰이 만료되었습니다. 재인증을 시도합니다...")
			newCreds, reAuthErr := performBrowserAuthWithFallback()
			if reAuthErr != nil {
				fmt.Println("  재인증 실패. 'lab login'을 실행해 주세요.")
				cancel()
				return
			}
			client.UpdateToken(newCreds.AccessToken)
			fmt.Printf("  재인증 성공: %s\n", newCreds.UserEmail)
			go func() {
				if connErr := client.Connect(ctx); connErr != nil {
					logger.Error().Err(connErr).Msg("재인증 후 재연결 실패")
					cancel()
				} else {
					client.StartHeartbeat(ctx)
				}
			}()
		} else {
			fmt.Println("\n  인증 실패. 'lab login'으로 다시 로그인해 주세요.")
			cancel()
		}
	})

	// 작업 실행기 생성 (client를 sender로 사용)
	taskExecutor := executor.NewTaskExecutor(
		registry,
		client,
		executor.WithLogger(log.Logger),
	)

	// MCP 서버 관리자 초기화 (SPEC-SKILL-V2-001 Block D)
	mcpConfig, err := mcp.LoadConfig("")
	if err != nil {
		logger.Warn().Err(err).Msg("MCP 설정 로드 실패, MCP 기능 비활성화")
		mcpConfig = &mcp.LocalConfig{}
	}
	mcpManager := mcp.NewManager(mcpConfig)
	mcpAdapter := mcp.NewStarterAdapter(mcpManager)

	// 메시지 라우터 설정 (동일한 client 인스턴스 사용)
	router := websocket.NewRouter(
		client,
		websocket.WithTaskExecutor(taskExecutor),
		websocket.WithMCPStarter(mcpAdapter),
		websocket.WithComputerUseHandler(cuHandler),
		websocket.WithErrorHandler(func(err error) {
			logger.Error().Err(err).Msg("메시지 처리 오류")
		}),
	)

	// 동일한 client에 메시지 핸들러 등록 (재생성하지 않음)
	client.SetMessageHandler(router)

	// 토큰 자동 갱신 서비스 시작
	creds, _ := auth.Load()
	if creds != nil && creds.RefreshToken != "" {
		tokenRefresher := auth.NewTokenRefresher(creds)
		tokenRefresher.Start(ctx)
		// 재연결 시 갱신된 토큰을 사용하도록 콜백 등록
		client.SetTokenRefreshFunc(func() (string, error) {
			token, err := tokenRefresher.GetToken()
			if err != nil && errors.Is(err, auth.ErrRefreshTokenExpired) {
				return "", fmt.Errorf("%w: %v", websocket.ErrAuthExpired, err)
			}
			return token, err
		})
		logger.Info().Msg("토큰 자동 갱신 서비스 시작")
	}

	// 연결 상태 추적
	connState := NewConnectionState()
	connState.SetWorkspaceID(connectWorkspaceID)

	// 연결 및 실행 루프
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		defer mcpManager.StopAll() // MCP 서버 정리 (SPEC-SKILL-V2-001 Block D)
		// SPEC-COMPUTER-USE-002: 컨테이너 풀 종료
		if containerPool != nil {
			defer func() {
				shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer shutdownCancel()
				if err := containerPool.Shutdown(shutdownCtx); err != nil {
					logger.Error().Err(err).Msg("컨테이너 풀 종료 중 오류")
				}
			}()
		}
		runEventLoop(ctx, cancel, client, taskExecutor, connState, sigCh)
	}()

	// 서버 연결 시도
	logger.Info().
		Str("server", srvURL).
		Int("timeout", connectTimeout).
		Msg("서버에 연결 중...")

	connectCtx, connectCancel := context.WithTimeout(ctx, time.Duration(connectTimeout)*time.Second)
	defer connectCancel()

	if err := client.Connect(connectCtx); err != nil {
		cancel()
		return fmt.Errorf("서버 연결 실패: %w", err)
	}

	connState.SetConnected(true)
	connState.SetStartTime(time.Now())
	connState.SetServerURL(srvURL)
	logger.Info().
		Str("server", srvURL).
		Str("state", client.State().String()).
		Msg("서버 연결 성공")

	if runtimeRoot != "" {
		if err := router.SendProjectContext(runtimeRoot); err != nil {
			logger.Warn().Err(err).Str("workspace_root", runtimeRoot).Msg("project_context 전송 실패")
		}
	}

	// 상태 파일 저장
	saveConnectionStatus(connState)

	// 하트비트 시작 (REQ-E-06)
	client.StartHeartbeat(ctx)

	// SPEC-COMPUTER-USE-002: Computer Use 백그라운드 고루틴 시작
	go cuHandler.SessionManager().StartCleanupLoop(ctx)
	if containerPool != nil {
		go containerPool.StartReplenisher(ctx)
		go containerPool.StartHealthMonitor(ctx)
	}

	// FR-P2-03: 네트워크 변경 감지 시작
	networkMonitor := websocket.NewNetworkMonitor(client, 5*time.Second)
	networkMonitor.Start(ctx)

	// 작업 실행기 시작
	taskExecutor.Start(ctx)

	// 스케줄 디스패처 시작 (에이전트 자율 운영)
	apiClient, apiErr := newAPIClient()
	if apiErr != nil {
		logger.Warn().Err(apiErr).Msg("스케줄 디스패처용 API 클라이언트 생성 실패 - 디스패처 비활성화")
	} else {
		fetcher := scheduler.NewAPIScheduleFetcher(apiClient)
		trigger := scheduler.NewAPITaskTrigger(apiClient)
		schedDispatcher := scheduler.NewDispatcher(fetcher, trigger, log.Logger, 60*time.Second)
		go schedDispatcher.Start(ctx)
	}

	// 종료 대기
	wg.Wait()

	logger.Info().
		Dur("uptime", connState.Uptime()).
		Int("tasks_completed", connState.TasksCompleted()).
		Int("tasks_failed", connState.TasksFailed()).
		Msg("연결 종료 완료")

	return nil
}

func loadBridgeRuntimeContext() (*websocket.BridgeRuntimeContext, string) {
	root := resolveRuntimeWorkspaceRoot()
	if root == "" {
		return nil, ""
	}

	manifest, err := bridgecontext.LoadManifest(root)
	if err != nil {
		return nil, ""
	}

	ctx := bridgecontext.BuildRuntimeContext(root, manifest)
	if ctx == nil {
		return nil, ""
	}

	bindings := make([]websocket.KnowledgeSourceBinding, 0, len(ctx.KnowledgeSourceBindings))
	for _, binding := range ctx.KnowledgeSourceBindings {
		bindings = append(bindings, websocket.KnowledgeSourceBinding{
			SourceID:   binding.SourceID,
			SourceType: binding.SourceType,
			SourceRoot: binding.SourceRoot,
			SyncMode:   binding.SyncMode,
			WriteScope: append([]string(nil), binding.WriteScope...),
		})
	}

	return &websocket.BridgeRuntimeContext{
		WorkspaceRoot:           ctx.WorkspaceRoot,
		KnowledgeSourceBindings: bindings,
		SyncMode:                ctx.SyncMode,
		IgnoreRulesLoaded:       ctx.IgnoreRulesLoaded,
		PendingLocalChanges:     ctx.PendingLocalChanges,
		WriteScope:              append([]string(nil), ctx.WriteScope...),
	}, root
}

func resolveRuntimeWorkspaceRoot() string {
	configPath := config.DefaultConfigPath()
	configDir := filepath.Dir(configPath)

	manager := project.NewManager(configDir)
	if err := manager.LoadProjects(); err == nil {
		if active, ok := manager.GetActive(); ok {
			if active.Path != "" {
				return active.Path
			}
		}
	}

	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	if _, statErr := os.Stat(bridgecontext.ManifestPath(wd)); statErr == nil {
		return wd
	}
	return ""
}

// runEventLoop는 메인 이벤트 루프를 실행합니다.
// REQ-E-09: SIGINT/SIGTERM 시 agent_disconnect 전송 후 정상 종료
func runEventLoop(
	ctx context.Context,
	cancel context.CancelFunc,
	client *websocket.Client,
	taskExecutor *executor.TaskExecutor,
	connState *ConnectionState,
	sigCh <-chan os.Signal,
) {
	for {
		select {
		case <-ctx.Done():
			// 컨텍스트 취소 - 정상 종료
			gracefulShutdown(client, taskExecutor, "context_cancelled")
			return

		case sig := <-sigCh:
			// REQ-E-09: 시그널 수신 시 graceful shutdown
			logger.Info().
				Str("signal", sig.String()).
				Msg("종료 시그널 수신")
			gracefulShutdown(client, taskExecutor, "user_initiated")
			cancel()
			return

		case msg := <-client.Messages():
			// 메시지 수신 처리
			handleMessage(ctx, msg, taskExecutor, connState)

		case <-client.Done():
			// 연결 종료됨 - handleDisconnect가 재연결을 시도 중일 수 있으므로 대기
			logger.Warn().Msg("WebSocket 연결이 종료되었습니다")
			connState.SetConnected(false)

			// 최대 5분 대기 (5초 * 60) - 재연결 성공 여부 확인
			reconnected := false
			for i := 0; i < 60; i++ {
				time.Sleep(5 * time.Second)

				select {
				case <-ctx.Done():
					gracefulShutdown(client, taskExecutor, "context_cancelled")
					return
				case sig := <-sigCh:
					logger.Info().Str("signal", sig.String()).Msg("종료 시그널 수신")
					gracefulShutdown(client, taskExecutor, "user_initiated")
					cancel()
					return
				default:
				}

				state := client.State()
				if state == websocket.StateConnected {
					logger.Info().Msg("재연결 성공 - 이벤트 루프 재시작")
					connState.SetConnected(true)
					reconnected = true
					break
				}
				if state == websocket.StateDisconnected || state == websocket.StateClosed {
					logger.Error().
						Str("state", state.String()).
						Msg("재연결 실패 - 프로세스 종료")
					return
				}
				logger.Debug().
					Str("state", state.String()).
					Int("wait_iteration", i+1).
					Msg("재연결 대기 중...")
			}

			if !reconnected {
				logger.Error().Msg("재연결 대기 시간 초과 - 프로세스 종료")
				return
			}
		}
	}
}

// handleMessage는 수신된 메시지를 처리합니다.
func handleMessage(
	ctx context.Context,
	msg ws.AgentMessage,
	taskExecutor *executor.TaskExecutor,
	connState *ConnectionState,
) {
	switch msg.Type {
	case ws.AgentMsgTaskReq:
		// 작업 요청은 Router(MessageHandler)에서 처리됨.
		// 여기서는 상태 추적만 수행 (중복 실행 방지).
		logger.Debug().
			Str("type", msg.Type).
			Msg("작업 요청 수신 (Router에서 실행 처리)")

	case ws.AgentMsgTaskResult:
		// 작업 완료 상태 업데이트
		connState.IncrementTasksCompleted()
		saveConnectionStatus(connState)

	case ws.AgentMsgTaskError:
		// 작업 오류 상태 업데이트
		connState.IncrementTasksFailed()
		saveConnectionStatus(connState)

	default:
		logger.Debug().
			Str("type", msg.Type).
			Msg("알 수 없는 메시지 타입")
	}
}

// gracefulShutdown는 정상적인 연결 종료를 수행합니다.
// REQ-U-03: 종료 시 정상적인 연결 해제
func gracefulShutdown(client *websocket.Client, taskExecutor *executor.TaskExecutor, reason string) {
	logger.Info().
		Str("reason", reason).
		Msg("정상 종료 시작")

	// 작업 실행기 중지
	taskExecutor.Stop()

	// REQ-E-09: agent_disconnect 메시지 전송
	if err := client.Disconnect(reason); err != nil {
		logger.Error().
			Err(err).
			Msg("연결 종료 중 오류 발생")
	}

	// 상태 파일 삭제
	clearConnectionStatus()

	logger.Info().Msg("정상 종료 완료")
}

// initializeProviders는 설정에 따라 AI 프로바이더를 초기화합니다.
func initializeProviders(ctx context.Context, cfg *config.Config) (*provider.Registry, error) {
	registryConfig := provider.RegistryConfig{
		ClaudeEnabled:      cfg.Providers.Claude.Enabled,
		ClaudeAPIKey:       cfg.Providers.Claude.GetAPIKey(),
		ClaudeDefaultModel: cfg.Providers.Claude.DefaultModel,
		ClaudeMode:         cfg.Providers.Claude.GetMode(),
		ClaudeCLIPath:      cfg.Providers.Claude.GetCLIPath(),
		ClaudeCLITimeout:   cfg.Providers.Claude.GetCLITimeout(),

		GeminiEnabled:      cfg.Providers.Gemini.Enabled,
		GeminiAPIKey:       cfg.Providers.Gemini.GetAPIKey(),
		GeminiDefaultModel: cfg.Providers.Gemini.DefaultModel,
		GeminiMode:         cfg.Providers.Gemini.GetMode(),
		GeminiCLIPath:      getProviderCLIPath(cfg.Providers.Gemini, "gemini"),
		GeminiCLITimeout:   cfg.Providers.Gemini.GetCLITimeout(),

		CodexEnabled:        cfg.Providers.Codex.Enabled,
		CodexAPIKey:         cfg.Providers.Codex.GetAPIKey(),
		CodexDefaultModel:   cfg.Providers.Codex.DefaultModel,
		CodexMode:           cfg.Providers.Codex.GetMode(),
		CodexCLIPath:        getProviderCLIPath(cfg.Providers.Codex, "codex"),
		CodexCLITimeout:     cfg.Providers.Codex.GetCLITimeout(),
		CodexApprovalPolicy: cfg.Providers.Codex.GetApprovalPolicy(),
		CodexChatGPTAuthEnv: cfg.Providers.Codex.ChatGPTAuthEnv,
	}

	return provider.InitializeRegistryWithLogger(ctx, registryConfig, log.Logger)
}

// startAuthWatcher는 authwatch 인스턴스를 시작합니다.
// SPEC-HOTSWAP-001: 인증 파일 변경 감지로 연결 끊김 없이 capabilities 업데이트
//
// 에러 발생 시 nil을 반환하고 경고를 로깅합니다 (기능 비활성화로 폴백).
func startAuthWatcher(ctx context.Context, reg *provider.Registry, client *websocket.Client) *authwatch.Watcher {
	// provider.Registry의 프로바이더를 authwatch.Provider로 래핑
	providerList := reg.ListProviders()
	authProviders := make([]authwatch.Provider, 0, len(providerList))
	for _, p := range providerList {
		authProviders = append(authProviders, &registryProviderAdapter{p: p})
	}

	// 각 프로바이더의 표준 인증 디렉토리
	home, err := os.UserHomeDir()
	if err != nil {
		logger.Warn().Err(err).Msg("authwatch: 홈 디렉토리 조회 실패")
		home = ""
	}

	var watchDirs []string
	if home != "" {
		watchDirs = []string{
			filepath.Join(home, ".claude"),
			filepath.Join(home, ".codex"),
			filepath.Join(home, ".gemini"),
			filepath.Join(home, ".config", "gcloud"),
		}
	}

	w, err := authwatch.New(
		authProviders,
		func(caps map[string]bool) {
			// capabilities 변경을 canonical name으로 변환하여 전송
			canonicalCaps := make(map[string]bool, len(caps))
			for name, v := range caps {
				canonicalCaps[provider.ToCanonicalName(name)] = v
			}
			client.UpdateProviderCapabilities(canonicalCaps)
		},
		authwatch.WithWatchDirs(watchDirs...),
	)
	if err != nil {
		logger.Warn().Err(err).Msg("authwatch: 감시자 생성 실패, hot-swap 기능 비활성화")
		return nil
	}

	if err := w.Start(ctx); err != nil {
		logger.Warn().Err(err).Msg("authwatch: 감시자 시작 실패, hot-swap 기능 비활성화")
		return nil
	}

	logger.Info().Msg("authwatch: 인증 파일 감시 시작 (SPEC-HOTSWAP-001)")
	return w
}

// registryProviderAdapter는 provider.Provider를 authwatch.Provider로 래핑합니다.
// authwatch 패키지가 provider 패키지에 의존하지 않도록 어댑터 패턴을 사용합니다.
type registryProviderAdapter struct {
	p provider.Provider
}

func (a *registryProviderAdapter) Name() string          { return a.p.Name() }
func (a *registryProviderAdapter) ValidateConfig() error { return a.p.ValidateConfig() }

// AuthFilePath는 빈 문자열을 반환합니다.
// connect.go에서 WithWatchDirs()로 직접 디렉토리를 지정하므로 이 메서드는 사용되지 않습니다.
func (a *registryProviderAdapter) AuthFilePath() string { return "" }

// getProviderCLIPath는 프로바이더별 CLI 바이너리 경로를 반환합니다.
// config.yaml에 cli_path가 명시적으로 설정된 경우 그 값을 사용하고,
// 미설정 시 프로바이더별 기본 바이너리 이름을 반환합니다.
// (config.GetCLIPath()는 모든 프로바이더에 "claude"를 반환하는 버그가 있음)
func getProviderCLIPath(p config.ProviderConfig, defaultBinary string) string {
	if p.CLIPath != "" {
		return p.CLIPath
	}
	return defaultBinary
}

// ConnectionState는 연결 상태를 추적합니다.
type ConnectionState struct {
	connected      bool
	startTime      time.Time
	serverURL      string
	workspaceID    string
	tasksCompleted int
	tasksFailed    int
	currentTaskID  string
	mu             sync.RWMutex
}

// NewConnectionState는 새로운 ConnectionState를 생성합니다.
func NewConnectionState() *ConnectionState {
	return &ConnectionState{}
}

// SetConnected는 연결 상태를 설정합니다.
func (s *ConnectionState) SetConnected(connected bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.connected = connected
}

// IsConnected는 연결 상태를 반환합니다.
func (s *ConnectionState) IsConnected() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.connected
}

// SetStartTime는 시작 시간을 설정합니다.
func (s *ConnectionState) SetStartTime(t time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.startTime = t
}

// Uptime는 연결 유지 시간을 반환합니다.
func (s *ConnectionState) Uptime() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.startTime.IsZero() {
		return 0
	}
	return time.Since(s.startTime)
}

// IncrementTasksCompleted는 완료된 작업 수를 증가시킵니다.
func (s *ConnectionState) IncrementTasksCompleted() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tasksCompleted++
}

// TasksCompleted는 완료된 작업 수를 반환합니다.
func (s *ConnectionState) TasksCompleted() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tasksCompleted
}

// IncrementTasksFailed는 실패한 작업 수를 증가시킵니다.
func (s *ConnectionState) IncrementTasksFailed() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tasksFailed++
}

// TasksFailed는 실패한 작업 수를 반환합니다.
func (s *ConnectionState) TasksFailed() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tasksFailed
}

// SetServerURL은 서버 URL을 설정합니다.
func (s *ConnectionState) SetServerURL(url string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.serverURL = url
}

// ServerURL은 서버 URL을 반환합니다.
func (s *ConnectionState) ServerURL() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.serverURL
}

// SetWorkspaceID는 워크스페이스 ID를 설정합니다.
func (s *ConnectionState) SetWorkspaceID(workspaceID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.workspaceID = strings.TrimSpace(workspaceID)
}

// WorkspaceID는 워크스페이스 ID를 반환합니다.
func (s *ConnectionState) WorkspaceID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.workspaceID
}

// SetCurrentTaskID는 현재 작업 ID를 설정합니다.
func (s *ConnectionState) SetCurrentTaskID(taskID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.currentTaskID = taskID
}

// CurrentTaskID는 현재 작업 ID를 반환합니다.
func (s *ConnectionState) CurrentTaskID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentTaskID
}

// saveConnectionStatus는 연결 상태를 파일에 저장합니다.
func saveConnectionStatus(connState *ConnectionState) {
	startTime := connState.startTime
	status := &StatusInfo{
		Connected:      connState.IsConnected(),
		ServerURL:      connState.ServerURL(),
		StartTime:      &startTime,
		TasksCompleted: connState.TasksCompleted(),
		TasksFailed:    connState.TasksFailed(),
		CurrentTask:    connState.CurrentTaskID(),
		PID:            os.Getpid(),
		WorkspaceID:    connState.WorkspaceID(),
	}

	if err := SaveStatus(status); err != nil {
		logger.Warn().Err(err).Msg("상태 파일 저장 실패")
	}
}

// clearConnectionStatus는 연결 상태 파일을 삭제합니다.
func clearConnectionStatus() {
	statusFile := getStatusFilePath()
	if statusFile != "" {
		if data, err := os.ReadFile(statusFile); err == nil {
			var status StatusInfo
			if err := json.Unmarshal(data, &status); err == nil {
				if status.PID > 0 && status.PID != os.Getpid() {
					logger.Warn().
						Int("status_pid", status.PID).
						Int("current_pid", os.Getpid()).
						Msg("다른 프로세스의 상태 파일은 삭제하지 않음")
					return
				}
			}
		}
	}

	if err := ClearStatus(); err != nil {
		logger.Warn().Err(err).Msg("상태 파일 삭제 실패")
	}
}

// acquireConnectLock은 connect.lock을 생성해 connect 단일 인스턴스를 보장합니다.
func acquireConnectLock(workspaceID string, replaceExisting bool) (*connectLock, error) {
	if err := config.EnsureConfigDir(); err != nil {
		return nil, fmt.Errorf("설정 디렉토리 생성 실패: %w", err)
	}

	lockPath, err := getConnectLockPath(workspaceID)
	if err != nil {
		return nil, err
	}
	pid := os.Getpid()
	pidStr := strconv.Itoa(pid)

	createLock := func() error {
		f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		defer f.Close()

		if _, err := f.WriteString(pidStr + "\n"); err != nil {
			_ = os.Remove(lockPath)
			return fmt.Errorf("lock 파일 쓰기 실패: %w", err)
		}
		return nil
	}

	if err := createLock(); err != nil {
		if !os.IsExist(err) {
			return nil, fmt.Errorf("connect lock 생성 실패: %w", err)
		}

		existingPID, pidErr := readLockPID(lockPath)
		if pidErr == nil && existingPID > 0 && connectProcessRunningFn(existingPID) {
			if !replaceExisting {
				return nil, fmt.Errorf(
					"이미 실행 중인 bridge 연결 프로세스가 있습니다 (PID: %d). 새 세션으로 교체하려면 --replace를 사용하세요",
					existingPID,
				)
			}
			if err := connectStopProcessFn(existingPID); err != nil {
				return nil, fmt.Errorf("기존 bridge 연결 프로세스 종료 실패 (PID: %d): %w", existingPID, err)
			}
		}
		if pidErr != nil {
			// parse 불가 상태는 동시 생성 중일 수 있으므로 안전하게 실패 처리
			return nil, fmt.Errorf("connect lock 파일이 사용 중입니다: %w", pidErr)
		}

		// stale lock 정리 후 1회 재시도
		if err := os.Remove(lockPath); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("stale connect lock 삭제 실패: %w", err)
		}
		if err := createLock(); err != nil {
			if os.IsExist(err) {
				if existingPID, pidErr := readLockPID(lockPath); pidErr == nil && existingPID > 0 {
					return nil, fmt.Errorf(
						"이미 실행 중인 bridge 연결 프로세스가 있습니다 (PID: %d). 새 세션으로 교체하려면 --replace를 사용하세요",
						existingPID,
					)
				}
			}
			return nil, fmt.Errorf("connect lock 생성 재시도 실패: %w", err)
		}
	}

	return &connectLock{
		path: lockPath,
		pid:  pid,
	}, nil
}

func getConnectLockPath(workspaceID string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("홈 디렉토리 조회 실패: %w", err)
	}

	name := connectLockFileName
	if strings.TrimSpace(workspaceID) != "" {
		name = "connect-" + sanitizeWorkspaceScope(workspaceID) + ".lock"
	}

	return filepath.Join(home, ".config", "autopus", name), nil
}

// Release는 현재 프로세스가 소유한 connect.lock을 해제합니다.
func (l *connectLock) Release() {
	if l == nil || l.path == "" {
		return
	}

	existingPID, err := readLockPID(l.path)
	if err == nil && existingPID > 0 && existingPID != l.pid {
		return
	}
	_ = os.Remove(l.path)
}

// readLockPID는 connect.lock에 기록된 PID를 반환합니다.
func readLockPID(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}

	text := strings.TrimSpace(string(data))
	if text == "" {
		return 0, errors.New("lock 파일이 비어 있습니다")
	}

	pid, err := strconv.Atoi(text)
	if err != nil {
		return 0, fmt.Errorf("lock PID 파싱 실패: %w", err)
	}

	return pid, nil
}
