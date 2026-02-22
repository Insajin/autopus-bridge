// Package cmd는 Local Agent Bridge CLI의 명령어를 정의합니다.
// connect.go는 서버 연결 명령을 구현합니다.
// REQ-E-01: connect 명령으로 WebSocket 연결 수립
// REQ-E-09: SIGINT/SIGTERM 시그널 시 graceful shutdown
// REQ-U-03: 종료 시 정상적인 연결 해제
package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/insajin/autopus-agent-protocol"
	"github.com/insajin/autopus-bridge/internal/auth"
	"github.com/insajin/autopus-bridge/internal/computeruse"
	"github.com/insajin/autopus-bridge/internal/config"
	"github.com/insajin/autopus-bridge/internal/executor"
	"github.com/insajin/autopus-bridge/internal/logger"
	"github.com/insajin/autopus-bridge/internal/mcp"
	"github.com/insajin/autopus-bridge/internal/provider"
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
)

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
}

// runConnect는 connect 명령의 실행 로직입니다.
func runConnect(cmd *cobra.Command, args []string) error {
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
					logger.Warn().Err(err).Msg("토큰 자동 갱신 실패. 'lab login'으로 다시 로그인하세요.")
				} else {
					authToken = creds.AccessToken
					logger.Info().
						Str("email", creds.UserEmail).
						Msg("토큰 자동 갱신 성공")
				}
			} else {
				logger.Warn().Msg("저장된 인증 정보가 만료되었습니다. 'lab login'으로 다시 로그인하세요.")
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

	logger.Info().
		Strs("providers", registry.List()).
		Msg("AI 프로바이더 초기화 완료")

	// SPEC-COMPUTER-USE-002: 컨테이너 풀 초기화 (Docker 사용 가능 시)
	var containerPool *computeruse.ContainerPool
	cuHandler := computeruse.NewHandler()

	if cfg.ComputerUse.IsContainerMode() {
		poolCtx, poolCancel := context.WithTimeout(ctx, 60*time.Second)
		pool, poolErr := computeruse.InitContainerPool(poolCtx, computeruse.ComputerUseConfigInput{
			MaxContainers:   cfg.ComputerUse.MaxContainers,
			WarmPoolSize:    cfg.ComputerUse.WarmPoolSize,
			Image:           cfg.ComputerUse.Image,
			ContainerMemory: cfg.ComputerUse.ContainerMemory,
			ContainerCPU:    cfg.ComputerUse.ContainerCPU,
			IdleTimeout:     cfg.ComputerUse.IdleTimeout,
			Network:         cfg.ComputerUse.Network,
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

	// WebSocket 클라이언트 생성 (단일 인스턴스)
	client := websocket.NewClient(
		srvURL,
		authToken,
		version,
		websocket.WithCapabilities(cfg.GetAvailableProviders()),
		websocket.WithReconnectStrategy(reconnectStrategy),
	)

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
			return tokenRefresher.GetToken()
		})
		logger.Info().Msg("토큰 자동 갱신 서비스 시작")
	}

	// 연결 상태 추적
	connState := NewConnectionState()

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

	// 종료 대기
	wg.Wait()

	logger.Info().
		Dur("uptime", connState.Uptime()).
		Int("tasks_completed", connState.TasksCompleted()).
		Int("tasks_failed", connState.TasksFailed()).
		Msg("연결 종료 완료")

	return nil
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

	case ws.AgentMsgTaskError:
		// 작업 오류 상태 업데이트
		connState.IncrementTasksFailed()

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
		ClaudeAPIKey:       cfg.Providers.Claude.GetAPIKey(),
		ClaudeDefaultModel: cfg.Providers.Claude.DefaultModel,
		ClaudeMode:         cfg.Providers.Claude.GetMode(),
		ClaudeCLIPath:      cfg.Providers.Claude.GetCLIPath(),
		ClaudeCLITimeout:   cfg.Providers.Claude.GetCLITimeout(),

		GeminiAPIKey:       cfg.Providers.Gemini.GetAPIKey(),
		GeminiDefaultModel: cfg.Providers.Gemini.DefaultModel,
		GeminiMode:         cfg.Providers.Gemini.GetMode(),
		GeminiCLIPath:      cfg.Providers.Gemini.GetCLIPath(),
		GeminiCLITimeout:   cfg.Providers.Gemini.GetCLITimeout(),

		CodexAPIKey:         cfg.Providers.Codex.GetAPIKey(),
		CodexDefaultModel:   cfg.Providers.Codex.DefaultModel,
		CodexMode:           cfg.Providers.Codex.GetMode(),
		CodexCLIPath:        cfg.Providers.Codex.GetCLIPath(),
		CodexCLITimeout:     cfg.Providers.Codex.GetCLITimeout(),
		CodexApprovalPolicy: cfg.Providers.Codex.GetApprovalPolicy(),
		CodexChatGPTAuthEnv: cfg.Providers.Codex.ChatGPTAuthEnv,
	}

	return provider.InitializeRegistry(ctx, registryConfig)
}

// ConnectionState는 연결 상태를 추적합니다.
type ConnectionState struct {
	connected      bool
	startTime      time.Time
	serverURL      string
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
	}

	if err := SaveStatus(status); err != nil {
		logger.Warn().Err(err).Msg("상태 파일 저장 실패")
	}
}

// clearConnectionStatus는 연결 상태 파일을 삭제합니다.
func clearConnectionStatus() {
	if err := ClearStatus(); err != nil {
		logger.Warn().Err(err).Msg("상태 파일 삭제 실패")
	}
}
