// Package provider는 AI 프로바이더 통합 레이어를 제공합니다.
package provider

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
)

// HybridClaudeProvider는 CLI와 API를 함께 사용하는 하이브리드 프로바이더입니다.
// CLI를 우선 사용하고, 실패 시 API로 폴백합니다.
type HybridClaudeProvider struct {
	cli    *ClaudeCLIProvider
	api    *ClaudeProvider
	logger zerolog.Logger

	// 통계 추적
	cliSuccess uint64
	cliFailed  uint64
	apiSuccess uint64
	apiFailed  uint64
}

// HybridClaudeProviderOption은 HybridClaudeProvider 설정 옵션입니다.
type HybridClaudeProviderOption func(*HybridClaudeProvider)

// WithHybridLogger는 로거를 설정합니다.
func WithHybridLogger(logger zerolog.Logger) HybridClaudeProviderOption {
	return func(p *HybridClaudeProvider) {
		p.logger = logger
	}
}

// NewHybridClaudeProvider는 새로운 HybridClaudeProvider를 생성합니다.
// CLI와 API 프로바이더를 모두 초기화합니다.
func NewHybridClaudeProvider(
	cliOpts []ClaudeCLIProviderOption,
	apiOpts []ClaudeProviderOption,
	hybridOpts ...HybridClaudeProviderOption,
) (*HybridClaudeProvider, error) {
	p := &HybridClaudeProvider{
		logger: zerolog.Nop(),
	}

	// 하이브리드 옵션 적용
	for _, opt := range hybridOpts {
		opt(p)
	}

	// CLI 프로바이더 초기화 (선택적)
	cli, err := NewClaudeCLIProvider(cliOpts...)
	if err != nil {
		p.logger.Warn().Err(err).Msg("CLI 프로바이더 초기화 실패, API만 사용")
	} else {
		p.cli = cli
	}

	// API 프로바이더 초기화 (선택적)
	api, err := NewClaudeProvider(apiOpts...)
	if err != nil {
		p.logger.Warn().Err(err).Msg("API 프로바이더 초기화 실패, CLI만 사용")
	} else {
		p.api = api
	}

	// 최소 하나의 프로바이더가 필요
	if p.cli == nil && p.api == nil {
		return nil, ErrNoAPIKey
	}

	return p, nil
}

// Name은 프로바이더 식별자를 반환합니다.
func (p *HybridClaudeProvider) Name() string {
	return "claude"
}

// ValidateConfig는 프로바이더 설정의 유효성을 검사합니다.
func (p *HybridClaudeProvider) ValidateConfig() error {
	// 최소 하나의 프로바이더가 유효해야 함
	if p.cli == nil && p.api == nil {
		return ErrNoAPIKey
	}

	// 각 프로바이더의 설정 검증 (에러 무시 - 하나만 작동해도 됨)
	if p.cli != nil {
		if err := p.cli.ValidateConfig(); err != nil {
			p.logger.Debug().Err(err).Msg("CLI 설정 검증 실패")
			p.cli = nil
		}
	}

	if p.api != nil {
		if err := p.api.ValidateConfig(); err != nil {
			p.logger.Debug().Err(err).Msg("API 설정 검증 실패")
			p.api = nil
		}
	}

	if p.cli == nil && p.api == nil {
		return ErrNoAPIKey
	}

	return nil
}

// Supports는 주어진 모델명을 지원하는지 확인합니다.
func (p *HybridClaudeProvider) Supports(model string) bool {
	if p.cli != nil && p.cli.Supports(model) {
		return true
	}
	if p.api != nil && p.api.Supports(model) {
		return true
	}
	return false
}

// Execute는 CLI를 우선 시도하고, 실패 시 API로 폴백합니다.
func (p *HybridClaudeProvider) Execute(ctx context.Context, req ExecuteRequest) (*ExecuteResponse, error) {
	startTime := time.Now()

	// CLI 우선 시도
	if p.cli != nil {
		p.logger.Debug().
			Str("model", req.Model).
			Msg("CLI로 실행 시도")

		resp, err := p.cli.Execute(ctx, req)
		if err == nil {
			atomic.AddUint64(&p.cliSuccess, 1)
			p.logger.Debug().
				Int64("duration_ms", time.Since(startTime).Milliseconds()).
				Msg("CLI 실행 성공")
			return resp, nil
		}

		atomic.AddUint64(&p.cliFailed, 1)
		p.logger.Warn().
			Err(err).
			Int64("duration_ms", time.Since(startTime).Milliseconds()).
			Msg("CLI 실행 실패, API로 폴백")
	}

	// API 폴백
	if p.api != nil {
		p.logger.Debug().
			Str("model", req.Model).
			Msg("API로 실행 시도")

		resp, err := p.api.Execute(ctx, req)
		if err == nil {
			atomic.AddUint64(&p.apiSuccess, 1)
			p.logger.Debug().
				Int64("duration_ms", time.Since(startTime).Milliseconds()).
				Msg("API 실행 성공")
			return resp, nil
		}

		atomic.AddUint64(&p.apiFailed, 1)
		p.logger.Error().
			Err(err).
			Int64("duration_ms", time.Since(startTime).Milliseconds()).
			Msg("API 실행도 실패")
		return nil, err
	}

	return nil, ErrNoAPIKey
}

// Stats는 프로바이더 사용 통계를 반환합니다.
func (p *HybridClaudeProvider) Stats() HybridStats {
	return HybridStats{
		CLISuccess:   atomic.LoadUint64(&p.cliSuccess),
		CLIFailed:    atomic.LoadUint64(&p.cliFailed),
		APISuccess:   atomic.LoadUint64(&p.apiSuccess),
		APIFailed:    atomic.LoadUint64(&p.apiFailed),
		CLIAvailable: p.cli != nil,
		APIAvailable: p.api != nil,
	}
}

// HybridStats는 하이브리드 프로바이더의 사용 통계입니다.
type HybridStats struct {
	// CLISuccess는 CLI 성공 횟수입니다.
	CLISuccess uint64 `json:"cli_success"`
	// CLIFailed는 CLI 실패 횟수입니다.
	CLIFailed uint64 `json:"cli_failed"`
	// APISuccess는 API 성공 횟수입니다.
	APISuccess uint64 `json:"api_success"`
	// APIFailed는 API 실패 횟수입니다.
	APIFailed uint64 `json:"api_failed"`
	// CLIAvailable는 CLI 사용 가능 여부입니다.
	CLIAvailable bool `json:"cli_available"`
	// APIAvailable는 API 사용 가능 여부입니다.
	APIAvailable bool `json:"api_available"`
}

// HasCLI는 CLI 프로바이더가 사용 가능한지 반환합니다.
func (p *HybridClaudeProvider) HasCLI() bool {
	return p.cli != nil
}

// HasAPI는 API 프로바이더가 사용 가능한지 반환합니다.
func (p *HybridClaudeProvider) HasAPI() bool {
	return p.api != nil
}
