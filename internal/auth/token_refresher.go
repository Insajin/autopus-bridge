// token_refresher.go는 토큰 만료 전 자동 갱신을 담당하는 백그라운드 goroutine을 제공합니다.
package auth

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

const (
	// refreshBeforeExpiry는 만료 전 이 시간만큼 앞서 갱신을 시도합니다.
	refreshBeforeExpiry = 5 * time.Minute
	// minRefreshInterval은 갱신 시도 간 최소 간격입니다.
	minRefreshInterval = 30 * time.Second
)

// TokenRefresher는 토큰을 주기적으로 갱신하는 백그라운드 서비스입니다.
type TokenRefresher struct {
	creds  *Credentials
	mu     sync.RWMutex
	logger *slog.Logger
}

// NewTokenRefresher는 새 TokenRefresher를 생성합니다.
func NewTokenRefresher(creds *Credentials) *TokenRefresher {
	return &TokenRefresher{
		creds:  creds,
		logger: slog.Default(),
	}
}

// Start는 백그라운드 토큰 갱신 goroutine을 시작합니다.
// ctx가 취소되면 종료됩니다.
func (r *TokenRefresher) Start(ctx context.Context) {
	go r.run(ctx)
}

// GetToken은 현재 유효한 access token을 반환합니다.
// 만료되었으면 즉시 갱신을 시도합니다.
func (r *TokenRefresher) GetToken() (string, error) {
	r.mu.RLock()
	if r.creds.IsValid() {
		token := r.creds.AccessToken
		r.mu.RUnlock()
		return token, nil
	}
	r.mu.RUnlock()

	// 만료된 토큰 즉시 갱신
	r.mu.Lock()
	defer r.mu.Unlock()

	// Double-check: 다른 goroutine이 이미 갱신했을 수 있음
	if r.creds.IsValid() {
		return r.creds.AccessToken, nil
	}

	if err := RefreshAccessToken(r.creds); err != nil {
		return "", fmt.Errorf("토큰 갱신 실패: %w", err)
	}

	r.logger.Info("토큰 즉시 갱신 성공",
		"expires_at", r.creds.ExpiresAt.Format(time.RFC3339),
	)
	return r.creds.AccessToken, nil
}

// run은 토큰 만료 전 자동 갱신을 수행하는 루프입니다.
func (r *TokenRefresher) run(ctx context.Context) {
	for {
		sleepDuration := r.nextRefreshDuration()

		r.logger.Debug("다음 토큰 갱신 예약",
			"sleep", sleepDuration.String(),
		)

		select {
		case <-ctx.Done():
			r.logger.Info("토큰 갱신 goroutine 종료")
			return
		case <-time.After(sleepDuration):
			r.refreshToken()
		}
	}
}

// nextRefreshDuration은 다음 갱신까지 대기할 시간을 계산합니다.
func (r *TokenRefresher) nextRefreshDuration() time.Duration {
	r.mu.RLock()
	defer r.mu.RUnlock()

	timeUntilExpiry := time.Until(r.creds.ExpiresAt)
	refreshAt := timeUntilExpiry - refreshBeforeExpiry

	if refreshAt < minRefreshInterval {
		return minRefreshInterval
	}
	return refreshAt
}

// refreshToken은 토큰 갱신을 시도합니다.
func (r *TokenRefresher) refreshToken() {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 이미 유효하고 만료까지 충분한 시간이 남아있으면 스킵
	if time.Until(r.creds.ExpiresAt) > refreshBeforeExpiry {
		return
	}

	if err := RefreshAccessToken(r.creds); err != nil {
		r.logger.Error("백그라운드 토큰 갱신 실패", "error", err)
		return
	}

	r.logger.Info("백그라운드 토큰 갱신 성공",
		"expires_at", r.creds.ExpiresAt.Format(time.RFC3339),
	)
}
