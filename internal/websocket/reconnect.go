// Package websocket는 Local Agent Bridge의 WebSocket 통신을 담당합니다.
// REQ-E-08: 지수 백오프 재연결 전략
package websocket

import (
	"math"
	"sync"
	"time"
)

// FastRetryCount는 빠른 재시도 횟수입니다 (지수 백오프 적용 전).
const FastRetryCount = 3

// ReconnectStrategy는 지수 백오프 재연결 전략을 구현합니다.
// SPEC 4.3 타이밍 요구사항:
// - 초기 지연: 1초
// - 최대 지연: 120초
// - 배수: 2.0
// - maxAttempts: 0 = 무제한
type ReconnectStrategy struct {
	// initialDelay는 초기 재연결 지연 시간입니다.
	initialDelay time.Duration
	// maxDelay는 최대 재연결 지연 시간입니다.
	maxDelay time.Duration
	// multiplier는 지수 백오프 배수입니다.
	multiplier float64
	// maxAttempts는 최대 재연결 시도 횟수입니다 (0 = 무제한).
	maxAttempts int

	// mu는 상태 접근을 보호하는 뮤텍스입니다.
	mu sync.RWMutex
	// currentAttempt는 현재 재연결 시도 횟수입니다.
	currentAttempt int
	// lastDelay는 마지막 계산된 지연 시간입니다.
	lastDelay time.Duration
}

// DefaultReconnectStrategy는 기본값을 사용하는 전략을 생성합니다.
func DefaultReconnectStrategy() *ReconnectStrategy {
	return NewReconnectStrategy(
		time.Second,     // initialDelay: 1초
		120*time.Second, // maxDelay: 120초
		2.0,             // multiplier: 2.0
		0,               // maxAttempts: 0 = 무제한
	)
}

// NewReconnectStrategy는 새로운 재연결 전략을 생성합니다.
// maxAttempts가 0이면 무제한 재시도를 허용합니다.
func NewReconnectStrategy(initialDelay, maxDelay time.Duration, multiplier float64, maxAttempts int) *ReconnectStrategy {
	if maxAttempts < 0 {
		maxAttempts = 0
	}

	return &ReconnectStrategy{
		initialDelay:   initialDelay,
		maxDelay:       maxDelay,
		multiplier:     multiplier,
		maxAttempts:    maxAttempts,
		currentAttempt: 0,
		lastDelay:      0,
	}
}

// NextDelay는 다음 재연결까지 대기해야 할 시간을 반환합니다.
// 처음 FastRetryCount(3)회는 초기 지연 시간을 사용하고,
// 이후 지수 백오프 공식: delay = initialDelay * (multiplier ^ (attempt - FastRetryCount))
// 최대 지연 시간을 초과하면 maxDelay를 반환합니다.
func (r *ReconnectStrategy) NextDelay() time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Fast retry: 처음 FastRetryCount회는 초기 지연 시간 사용
	if r.currentAttempt < FastRetryCount {
		r.lastDelay = r.initialDelay
		r.currentAttempt++
		return r.lastDelay
	}

	// 지수 백오프 계산 (FastRetryCount 이후부터)
	backoffAttempt := r.currentAttempt - FastRetryCount
	delay := time.Duration(float64(r.initialDelay) * math.Pow(r.multiplier, float64(backoffAttempt)))

	// 최대 지연 시간 제한
	if delay > r.maxDelay {
		delay = r.maxDelay
	}

	r.lastDelay = delay
	r.currentAttempt++
	return delay
}

// Reset은 재연결 시도 횟수를 초기화합니다.
// 연결 성공 시 호출해야 합니다.
func (r *ReconnectStrategy) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.currentAttempt = 0
	r.lastDelay = 0
}

// CurrentAttempt는 현재 재연결 시도 횟수를 반환합니다.
func (r *ReconnectStrategy) CurrentAttempt() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.currentAttempt
}

// CanRetry는 재연결 시도가 가능한지 확인합니다.
// maxAttempts가 0이면 무제한 재시도를 허용합니다.
func (r *ReconnectStrategy) CanRetry() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.maxAttempts == 0 {
		return true // 무제한
	}
	return r.currentAttempt < r.maxAttempts
}

// MaxAttempts는 최대 재연결 시도 횟수를 반환합니다.
// 0은 무제한을 의미합니다.
func (r *ReconnectStrategy) MaxAttempts() int {
	return r.maxAttempts
}

// RemainingAttempts는 남은 재연결 시도 횟수를 반환합니다.
// 무제한 모드(maxAttempts == 0)에서는 -1을 반환합니다.
func (r *ReconnectStrategy) RemainingAttempts() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.maxAttempts == 0 {
		return -1 // 무제한
	}
	remaining := r.maxAttempts - r.currentAttempt
	if remaining < 0 {
		return 0
	}
	return remaining
}

// LastDelay는 마지막으로 계산된 지연 시간을 반환합니다.
func (r *ReconnectStrategy) LastDelay() time.Duration {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.lastDelay
}
