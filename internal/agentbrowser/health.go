package agentbrowser

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

const (
	// healthCheckInterval은 헬스 체크 주기이다.
	healthCheckInterval = 30 * time.Second
	// healthCheckTimeout은 헬스 체크 명령의 타임아웃이다.
	healthCheckTimeout = 10 * time.Second
)

// HealthChecker는 agent-browser 데몬의 상태를 모니터링한다.
type HealthChecker struct {
	logger         zerolog.Logger
	executor       *CommandExecutor
	onStatusChange StatusCallback

	// healthy는 마지막 헬스 체크 결과이다.
	healthy bool
	// consecutiveFailures는 연속 헬스 체크 실패 횟수이다.
	consecutiveFailures int
	mu                  sync.Mutex
}

// NewHealthChecker는 새로운 HealthChecker를 생성한다.
func NewHealthChecker(logger zerolog.Logger, executor *CommandExecutor, onStatusChange StatusCallback) *HealthChecker {
	return &HealthChecker{
		logger:         logger,
		executor:       executor,
		onStatusChange: onStatusChange,
		healthy:        false,
	}
}

// CheckHealth는 agent-browser 데몬의 응답성을 확인한다.
// `agent-browser get url` 명령으로 데몬이 응답하는지 확인한다.
func (hc *HealthChecker) CheckHealth(ctx context.Context) bool {
	timeoutCtx, cancel := context.WithTimeout(ctx, healthCheckTimeout)
	defer cancel()

	// 간단한 명령으로 응답성 확인
	_, err := hc.executor.Execute(timeoutCtx, "get", "url")

	hc.mu.Lock()
	defer hc.mu.Unlock()

	if err != nil {
		hc.consecutiveFailures++
		hc.healthy = false

		hc.logger.Warn().
			Err(err).
			Int("consecutive_failures", hc.consecutiveFailures).
			Msg("agent-browser 헬스 체크 실패")

		// 연속 실패 시 상태 콜백 호출
		if hc.onStatusChange != nil && hc.consecutiveFailures >= maxRestartAttempts {
			hc.onStatusChange(StateError, "agent-browser 헬스 체크 연속 실패")
		}

		return false
	}

	// 이전에 실패했다가 복구된 경우
	if !hc.healthy && hc.consecutiveFailures > 0 {
		hc.logger.Info().
			Int("previous_failures", hc.consecutiveFailures).
			Msg("agent-browser 헬스 체크 복구")
		if hc.onStatusChange != nil {
			hc.onStatusChange(StateReady, "agent-browser 헬스 체크 복구")
		}
	}

	hc.consecutiveFailures = 0
	hc.healthy = true

	return true
}

// IsHealthy는 마지막 헬스 체크 결과를 반환한다.
func (hc *HealthChecker) IsHealthy() bool {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	return hc.healthy
}

// ConsecutiveFailures는 연속 실패 횟수를 반환한다.
func (hc *HealthChecker) ConsecutiveFailures() int {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	return hc.consecutiveFailures
}

// StartMonitoring은 주기적 헬스 체크 goroutine을 시작한다.
// 컨텍스트가 취소되면 중지된다.
func (hc *HealthChecker) StartMonitoring(ctx context.Context) {
	ticker := time.NewTicker(healthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			hc.CheckHealth(ctx)
		}
	}
}

// NeedsRestart는 자동 재시작이 필요한지 판단한다.
// 연속 실패가 1회 이상이고 maxRestartAttempts 미만인 경우 true를 반환한다.
func (hc *HealthChecker) NeedsRestart() bool {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	return hc.consecutiveFailures > 0 && hc.consecutiveFailures < maxRestartAttempts
}

// Reset은 헬스 체커 상태를 초기화한다.
func (hc *HealthChecker) Reset() {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.consecutiveFailures = 0
	hc.healthy = false
}
