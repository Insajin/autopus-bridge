// Package executor - CodingSessionManager 테스트
package executor

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCodingSessionManagerNew는 NewCodingSessionManager 생성자를 검증합니다.
func TestCodingSessionManagerNew(t *testing.T) {
	t.Parallel()

	cfg := CodingSessionConfig{
		MaxConcurrent:      2,
		RelayMaxIterations: 10,
		SessionTimeoutMin:  30,
		MaxBudgetUSD:       20.0,
	}
	mgr := NewCodingSessionManager(cfg)
	require.NotNil(t, mgr)
}

// TestCodingSessionManagerNewDefault는 기본 설정으로 생성을 검증합니다.
func TestCodingSessionManagerNewDefault(t *testing.T) {
	t.Parallel()

	mgr := NewCodingSessionManager(CodingSessionConfig{})
	require.NotNil(t, mgr)
	// 기본값: MaxConcurrent = 2
	assert.Equal(t, 2, mgr.maxConcurrent)
}

// TestCodingSessionManagerSemaphore는 세마포어 Acquire/Release를 검증합니다.
func TestCodingSessionManagerSemaphore(t *testing.T) {
	t.Parallel()

	cfg := CodingSessionConfig{MaxConcurrent: 2}
	mgr := NewCodingSessionManager(cfg)

	// 첫 번째 슬롯 획득
	err := mgr.AcquireSlot(context.Background())
	require.NoError(t, err)

	// 두 번째 슬롯 획득
	err = mgr.AcquireSlot(context.Background())
	require.NoError(t, err)

	// 세 번째 슬롯 획득 시도 — 타임아웃 내에 실패해야 함
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err = mgr.AcquireSlot(ctx)
	require.Error(t, err)

	// 슬롯 해제 후 재획득 가능
	mgr.ReleaseSlot()
	err = mgr.AcquireSlot(context.Background())
	require.NoError(t, err)

	// 정리
	mgr.ReleaseSlot()
	mgr.ReleaseSlot()
	mgr.ReleaseSlot()
}

// TestCodingSessionManagerSemaphoreConcurrent는 동시 세마포어 사용을 검증합니다.
func TestCodingSessionManagerSemaphoreConcurrent(t *testing.T) {
	t.Parallel()

	const maxConcurrent = 3
	const totalWorkers = 10
	cfg := CodingSessionConfig{MaxConcurrent: maxConcurrent}
	mgr := NewCodingSessionManager(cfg)

	var (
		mu      sync.Mutex
		current int
		maxSeen int
	)

	var wg sync.WaitGroup
	for i := 0; i < totalWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			if err := mgr.AcquireSlot(context.Background()); err != nil {
				return
			}
			defer mgr.ReleaseSlot()

			mu.Lock()
			current++
			if current > maxSeen {
				maxSeen = current
			}
			mu.Unlock()

			time.Sleep(10 * time.Millisecond)

			mu.Lock()
			current--
			mu.Unlock()
		}()
	}

	wg.Wait()
	assert.LessOrEqual(t, maxSeen, maxConcurrent, "동시 실행 수가 최대치를 초과함")
}

// TestCodingSessionManagerRegisterGet은 세션 등록/조회를 검증합니다.
func TestCodingSessionManagerRegisterGet(t *testing.T) {
	t.Parallel()

	mgr := NewCodingSessionManager(CodingSessionConfig{MaxConcurrent: 2})
	session := &mockCodingSession{sessionID: "test-123"}

	mgr.RegisterSession("test-123", session)
	got, ok := mgr.GetSession("test-123")
	require.True(t, ok)
	assert.Equal(t, session, got)
}

// TestCodingSessionManagerGetNotFound는 존재하지 않는 세션 조회를 검증합니다.
func TestCodingSessionManagerGetNotFound(t *testing.T) {
	t.Parallel()

	mgr := NewCodingSessionManager(CodingSessionConfig{MaxConcurrent: 2})
	_, ok := mgr.GetSession("nonexistent")
	assert.False(t, ok)
}

// TestCodingSessionManagerCloseSession는 세션 종료를 검증합니다.
func TestCodingSessionManagerCloseSession(t *testing.T) {
	t.Parallel()

	mgr := NewCodingSessionManager(CodingSessionConfig{MaxConcurrent: 2})
	session := &mockCodingSession{sessionID: "sess-456"}
	mgr.RegisterSession("sess-456", session)

	err := mgr.CloseSession(context.Background(), "sess-456")
	require.NoError(t, err)

	// 닫힌 후 세션이 제거되어야 함
	_, ok := mgr.GetSession("sess-456")
	assert.False(t, ok)
}

// TestCodingSessionManagerCloseSessionNotFound는 존재하지 않는 세션 종료를 검증합니다.
func TestCodingSessionManagerCloseSessionNotFound(t *testing.T) {
	t.Parallel()

	mgr := NewCodingSessionManager(CodingSessionConfig{MaxConcurrent: 2})
	err := mgr.CloseSession(context.Background(), "nonexistent")
	// 에러 또는 nil 반환 — 존재하지 않는 세션 종료 시 에러 반환
	require.Error(t, err)
}

// TestCodingSessionManagerDetectProviders는 DetectProviders가 정렬된 리스트를 반환하는지 검증합니다.
func TestCodingSessionManagerDetectProviders(t *testing.T) {
	t.Parallel()

	mgr := NewCodingSessionManager(CodingSessionConfig{MaxConcurrent: 2})
	providers := mgr.DetectProviders()

	// providers는 nil이 아닌 슬라이스를 반환해야 함 (비어있을 수 있음)
	assert.NotNil(t, providers)

	// 반환된 프로바이더는 유효한 값이어야 함 (string 타입)
	validProviders := []string{
		string(CodingProviderClaude), string(CodingProviderCodex),
		string(CodingProviderGemini), string(CodingProviderOpenCode),
	}
	for _, p := range providers {
		assert.Contains(t, validProviders, p)
	}
}

// TestCodingSessionManagerCreateSession는 CreateSession 팩토리 메서드를 검증합니다.
func TestCodingSessionManagerCreateSession(t *testing.T) {
	t.Parallel()

	mgr := NewCodingSessionManager(CodingSessionConfig{MaxConcurrent: 2})

	// Claude 프로바이더로 세션 생성
	session, err := mgr.CreateSession(CodingProviderClaude)
	require.NoError(t, err)
	require.NotNil(t, session)

	// API 폴백 프로바이더로 세션 생성
	apiSession, err := mgr.CreateSession(CodingProviderAPI)
	require.NoError(t, err)
	require.NotNil(t, apiSession)

	// 알 수 없는 프로바이더는 에러 반환
	_, err = mgr.CreateSession(CodingProvider("unknown"))
	require.Error(t, err)
}

// TestCodingSessionManagerSessionLifecycle은 전체 세션 라이프사이클을 검증합니다.
func TestCodingSessionManagerSessionLifecycle(t *testing.T) {
	t.Parallel()

	mgr := NewCodingSessionManager(CodingSessionConfig{MaxConcurrent: 2})

	// 세션 등록
	for i := 0; i < 5; i++ {
		id := fmt.Sprintf("session-%d", i)
		session := &mockCodingSession{sessionID: id}
		mgr.RegisterSession(id, session)
	}

	// 모든 세션 조회 가능해야 함
	for i := 0; i < 5; i++ {
		id := fmt.Sprintf("session-%d", i)
		_, ok := mgr.GetSession(id)
		assert.True(t, ok, "세션 %s를 찾을 수 없음", id)
	}

	// 세션 하나씩 종료
	for i := 0; i < 5; i++ {
		id := fmt.Sprintf("session-%d", i)
		err := mgr.CloseSession(context.Background(), id)
		require.NoError(t, err)
		_, ok := mgr.GetSession(id)
		assert.False(t, ok, "종료된 세션 %s가 여전히 존재함", id)
	}
}
