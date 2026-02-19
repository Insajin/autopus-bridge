package websocket

import (
	"testing"
	"time"
)

// TestDefaultReconnectStrategy는 기본 재연결 전략의 설정값을 검증합니다.
func TestDefaultReconnectStrategy(t *testing.T) {
	strategy := DefaultReconnectStrategy()

	// 기본값 확인
	if strategy.initialDelay != time.Second {
		t.Errorf("initialDelay = %v, want %v", strategy.initialDelay, time.Second)
	}
	if strategy.maxDelay != 120*time.Second {
		t.Errorf("maxDelay = %v, want %v", strategy.maxDelay, 120*time.Second)
	}
	if strategy.multiplier != 2.0 {
		t.Errorf("multiplier = %v, want %v", strategy.multiplier, 2.0)
	}
	// maxAttempts: 0 = 무제한
	if strategy.maxAttempts != 0 {
		t.Errorf("maxAttempts = %v, want %v (0 = 무제한)", strategy.maxAttempts, 0)
	}
}

// TestNewReconnectStrategy는 사용자 정의 재연결 전략 생성을 검증합니다.
func TestNewReconnectStrategy(t *testing.T) {
	strategy := NewReconnectStrategy(
		2*time.Second,
		30*time.Second,
		1.5,
		5,
	)

	if strategy.initialDelay != 2*time.Second {
		t.Errorf("initialDelay = %v, want %v", strategy.initialDelay, 2*time.Second)
	}
	if strategy.maxDelay != 30*time.Second {
		t.Errorf("maxDelay = %v, want %v", strategy.maxDelay, 30*time.Second)
	}
	if strategy.multiplier != 1.5 {
		t.Errorf("multiplier = %v, want %v", strategy.multiplier, 1.5)
	}
	if strategy.maxAttempts != 5 {
		t.Errorf("maxAttempts = %v, want %v", strategy.maxAttempts, 5)
	}
}

// TestNewReconnectStrategy_MaxAttemptsLimit는 무제한 모드를 검증합니다.
func TestNewReconnectStrategy_MaxAttemptsLimit(t *testing.T) {
	// maxAttempts=0은 무제한 (그대로 유지)
	strategy := NewReconnectStrategy(time.Second, time.Minute, 2.0, 0)
	if strategy.maxAttempts != 0 {
		t.Errorf("maxAttempts = %v, want %v (0 = 무제한)", strategy.maxAttempts, 0)
	}

	// 음수는 0(무제한)으로 보정
	strategy = NewReconnectStrategy(time.Second, time.Minute, 2.0, -5)
	if strategy.maxAttempts != 0 {
		t.Errorf("maxAttempts = %v, want %v (음수는 0으로 보정)", strategy.maxAttempts, 0)
	}

	// 큰 값도 그대로 허용 (10 제한 제거됨)
	strategy = NewReconnectStrategy(time.Second, time.Minute, 2.0, 100)
	if strategy.maxAttempts != 100 {
		t.Errorf("maxAttempts = %v, want %v (큰 값 허용)", strategy.maxAttempts, 100)
	}
}

// TestUnlimitedRetries는 무제한 재시도 모드를 검증합니다.
func TestUnlimitedRetries(t *testing.T) {
	strategy := NewReconnectStrategy(time.Second, time.Minute, 2.0, 0)

	// 무제한 모드에서 CanRetry는 항상 true
	for i := 0; i < 100; i++ {
		if !strategy.CanRetry() {
			t.Errorf("무제한 모드에서 %d번째 시도 후 CanRetry() = false, want true", i)
		}
		_ = strategy.NextDelay()
	}

	// RemainingAttempts는 -1 반환
	if strategy.RemainingAttempts() != -1 {
		t.Errorf("무제한 모드 RemainingAttempts() = %v, want -1", strategy.RemainingAttempts())
	}
}

// TestFastRetry는 처음 FastRetryCount회가 초기 지연을 사용하는지 검증합니다.
func TestFastRetry(t *testing.T) {
	initialDelay := 2 * time.Second
	strategy := NewReconnectStrategy(initialDelay, time.Minute, 2.0, 0)

	// 처음 FastRetryCount(3)회는 모두 initialDelay를 사용
	for i := 0; i < FastRetryCount; i++ {
		delay := strategy.NextDelay()
		if delay != initialDelay {
			t.Errorf("Fast retry 시도 %d: NextDelay() = %v, want %v", i+1, delay, initialDelay)
		}
	}

	// FastRetryCount 이후 첫 번째는 initialDelay * multiplier^0 = initialDelay
	delay := strategy.NextDelay()
	if delay != initialDelay {
		t.Errorf("FastRetryCount 이후 첫 시도: NextDelay() = %v, want %v", delay, initialDelay)
	}

	// 그 다음은 지수 백오프 적용: initialDelay * 2^1 = 4s
	delay = strategy.NextDelay()
	expected := 4 * time.Second
	if delay != expected {
		t.Errorf("FastRetryCount+2 시도: NextDelay() = %v, want %v", delay, expected)
	}
}

// TestNextDelay_ExponentialBackoff는 지수 백오프 계산을 검증합니다.
func TestNextDelay_ExponentialBackoff(t *testing.T) {
	strategy := NewReconnectStrategy(
		time.Second,     // initialDelay: 1초
		120*time.Second, // maxDelay: 120초
		2.0,             // multiplier: 2.0
		10,
	)

	// FastRetryCount(3)회는 initialDelay, 이후 지수 백오프
	// 시도 1-3: 1s (fast retry)
	// 시도 4: 1s * 2^0 = 1s
	// 시도 5: 1s * 2^1 = 2s
	// 시도 6: 1s * 2^2 = 4s
	// 시도 7: 1s * 2^3 = 8s
	// 시도 8: 1s * 2^4 = 16s
	// 시도 9: 1s * 2^5 = 32s
	// 시도 10: 1s * 2^6 = 64s
	expectedDelays := []time.Duration{
		1 * time.Second,  // fast retry 1
		1 * time.Second,  // fast retry 2
		1 * time.Second,  // fast retry 3
		1 * time.Second,  // backoff 2^0
		2 * time.Second,  // backoff 2^1
		4 * time.Second,  // backoff 2^2
		8 * time.Second,  // backoff 2^3
		16 * time.Second, // backoff 2^4
		32 * time.Second, // backoff 2^5
		64 * time.Second, // backoff 2^6
	}

	for i, expected := range expectedDelays {
		got := strategy.NextDelay()
		if got != expected {
			t.Errorf("시도 %d: NextDelay() = %v, want %v", i+1, got, expected)
		}
	}
}

// TestNextDelay_FirstAttemptIsInitialDelay는 첫 시도가 초기 지연을 사용하는지 검증합니다.
func TestNextDelay_FirstAttemptIsInitialDelay(t *testing.T) {
	strategy := NewReconnectStrategy(5*time.Second, time.Minute, 2.0, 10)

	delay := strategy.NextDelay()
	if delay != 5*time.Second {
		t.Errorf("첫 번째 NextDelay() = %v, want %v", delay, 5*time.Second)
	}
}

// TestReset은 재연결 시도 횟수 초기화를 검증합니다.
func TestReset(t *testing.T) {
	strategy := DefaultReconnectStrategy()

	// 몇 번 시도 후
	_ = strategy.NextDelay()
	_ = strategy.NextDelay()
	_ = strategy.NextDelay()

	if strategy.CurrentAttempt() != 3 {
		t.Errorf("시도 후 CurrentAttempt() = %v, want %v", strategy.CurrentAttempt(), 3)
	}

	// Reset 호출
	strategy.Reset()

	if strategy.CurrentAttempt() != 0 {
		t.Errorf("Reset 후 CurrentAttempt() = %v, want %v", strategy.CurrentAttempt(), 0)
	}
	if strategy.LastDelay() != 0 {
		t.Errorf("Reset 후 LastDelay() = %v, want %v", strategy.LastDelay(), time.Duration(0))
	}

	// Reset 후 첫 번째 시도는 다시 초기 지연 사용
	delay := strategy.NextDelay()
	if delay != time.Second {
		t.Errorf("Reset 후 첫 번째 NextDelay() = %v, want %v", delay, time.Second)
	}
}

// TestCanRetry는 재시도 가능 여부 확인을 검증합니다.
func TestCanRetry(t *testing.T) {
	strategy := NewReconnectStrategy(time.Second, time.Minute, 2.0, 3)

	// 초기 상태: 재시도 가능
	if !strategy.CanRetry() {
		t.Error("초기 상태에서 CanRetry() = false, want true")
	}

	// 3회 시도 후
	_ = strategy.NextDelay()
	_ = strategy.NextDelay()
	_ = strategy.NextDelay()

	// 최대 시도 횟수 도달: 재시도 불가
	if strategy.CanRetry() {
		t.Error("최대 시도 후 CanRetry() = true, want false")
	}

	// Reset 후: 다시 재시도 가능
	strategy.Reset()
	if !strategy.CanRetry() {
		t.Error("Reset 후 CanRetry() = false, want true")
	}
}

// TestRemainingAttempts는 남은 재시도 횟수 계산을 검증합니다.
func TestRemainingAttempts(t *testing.T) {
	strategy := NewReconnectStrategy(time.Second, time.Minute, 2.0, 5)

	if strategy.RemainingAttempts() != 5 {
		t.Errorf("초기 RemainingAttempts() = %v, want %v", strategy.RemainingAttempts(), 5)
	}

	_ = strategy.NextDelay()
	if strategy.RemainingAttempts() != 4 {
		t.Errorf("1회 시도 후 RemainingAttempts() = %v, want %v", strategy.RemainingAttempts(), 4)
	}

	_ = strategy.NextDelay()
	_ = strategy.NextDelay()
	if strategy.RemainingAttempts() != 2 {
		t.Errorf("3회 시도 후 RemainingAttempts() = %v, want %v", strategy.RemainingAttempts(), 2)
	}
}

// TestConcurrentAccess는 동시 접근 안전성을 검증합니다.
func TestConcurrentAccess(t *testing.T) {
	strategy := DefaultReconnectStrategy()

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				_ = strategy.NextDelay()
				_ = strategy.CurrentAttempt()
				_ = strategy.CanRetry()
				_ = strategy.RemainingAttempts()
				_ = strategy.LastDelay()
				if j%10 == 0 {
					strategy.Reset()
				}
			}
			done <- true
		}()
	}

	// 모든 고루틴 완료 대기
	for i := 0; i < 10; i++ {
		<-done
	}
}

// TestLastDelay는 마지막 지연 시간 추적을 검증합니다.
func TestLastDelay(t *testing.T) {
	strategy := NewReconnectStrategy(time.Second, time.Minute, 2.0, 10)

	// 초기 상태
	if strategy.LastDelay() != 0 {
		t.Errorf("초기 LastDelay() = %v, want %v", strategy.LastDelay(), time.Duration(0))
	}

	// 첫 번째 시도 후
	delay1 := strategy.NextDelay()
	if strategy.LastDelay() != delay1 {
		t.Errorf("첫 번째 시도 후 LastDelay() = %v, want %v", strategy.LastDelay(), delay1)
	}

	// 두 번째 시도 후
	delay2 := strategy.NextDelay()
	if strategy.LastDelay() != delay2 {
		t.Errorf("두 번째 시도 후 LastDelay() = %v, want %v", strategy.LastDelay(), delay2)
	}
}

// TestMaxDelayCap은 최대 지연 시간 제한을 검증합니다.
func TestMaxDelayCap(t *testing.T) {
	strategy := NewReconnectStrategy(
		10*time.Second, // 초기 지연이 크면 빠르게 최대에 도달
		20*time.Second, // 낮은 최대 지연
		2.0,
		10,
	)

	// FastRetryCount(3)회는 모두 initialDelay(10s)
	for i := 0; i < FastRetryCount; i++ {
		delay := strategy.NextDelay()
		if delay != 10*time.Second {
			t.Errorf("Fast retry 시도 %d: NextDelay() = %v, want %v", i+1, delay, 10*time.Second)
		}
	}

	// FastRetryCount 이후: 10s * 2^0 = 10s
	delay := strategy.NextDelay()
	if delay != 10*time.Second {
		t.Errorf("backoff 시도 1: NextDelay() = %v, want %v", delay, 10*time.Second)
	}

	// 다음: 10s * 2^1 = 20s (maxDelay 도달)
	delay = strategy.NextDelay()
	if delay != 20*time.Second {
		t.Errorf("backoff 시도 2: NextDelay() = %v, want %v (maxDelay)", delay, 20*time.Second)
	}

	// 이후에도 maxDelay 유지
	delay = strategy.NextDelay()
	if delay != 20*time.Second {
		t.Errorf("backoff 시도 3: NextDelay() = %v, want %v (maxDelay 유지)", delay, 20*time.Second)
	}
}
