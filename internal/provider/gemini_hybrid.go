// Package provider는 AI 프로바이더 통합 레이어를 제공합니다.
package provider

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
)

// HybridGeminiProvider는 CLI와 API를 함께 사용하는 하이브리드 Gemini 프로바이더입니다.
// CLI를 우선 사용하고, 실패 시 API로 폴백합니다.
type HybridGeminiProvider struct {
	cli    *GeminiCLIProvider
	api    *GeminiProvider
	logger zerolog.Logger

	// 통계 추적
	cliSuccess uint64
	cliFailed  uint64
	apiSuccess uint64
	apiFailed  uint64
}

// HybridGeminiProviderOption은 HybridGeminiProvider 설정 옵션입니다.
type HybridGeminiProviderOption func(*HybridGeminiProvider)

// WithGeminiHybridLogger는 로거를 설정합니다.
func WithGeminiHybridLogger(logger zerolog.Logger) HybridGeminiProviderOption {
	return func(p *HybridGeminiProvider) {
		p.logger = logger
	}
}

// NewHybridGeminiProvider는 새로운 HybridGeminiProvider를 생성합니다.
// CLI와 API 프로바이더를 모두 초기화합니다.
// NewGeminiProvider가 context.Context를 필요로 하므로 ctx 파라미터가 필수입니다.
func NewHybridGeminiProvider(
	ctx context.Context,
	cliOpts []GeminiCLIProviderOption,
	apiOpts []GeminiProviderOption,
	hybridOpts ...HybridGeminiProviderOption,
) (*HybridGeminiProvider, error) {
	p := &HybridGeminiProvider{
		logger: zerolog.Nop(),
	}

	// 하이브리드 옵션 적용
	for _, opt := range hybridOpts {
		opt(p)
	}

	// CLI 프로바이더 초기화 (선택적)
	cli, err := NewGeminiCLIProvider(cliOpts...)
	if err != nil {
		p.logger.Warn().Err(err).Msg("Gemini CLI 프로바이더 초기화 실패, API만 사용")
	} else {
		p.cli = cli
	}

	// API 프로바이더 초기화 (선택적)
	api, err := NewGeminiProvider(ctx, apiOpts...)
	if err != nil {
		p.logger.Warn().Err(err).Msg("Gemini API 프로바이더 초기화 실패, CLI만 사용")
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
func (p *HybridGeminiProvider) Name() string {
	return "gemini"
}

// ValidateConfig는 프로바이더 설정의 유효성을 검사합니다.
func (p *HybridGeminiProvider) ValidateConfig() error {
	// 최소 하나의 프로바이더가 유효해야 함
	if p.cli == nil && p.api == nil {
		return ErrNoAPIKey
	}

	// 각 프로바이더의 설정 검증 (에러 무시 - 하나만 작동해도 됨)
	if p.cli != nil {
		if err := p.cli.ValidateConfig(); err != nil {
			p.logger.Debug().Err(err).Msg("Gemini CLI 설정 검증 실패")
			p.cli = nil
		}
	}

	if p.api != nil {
		if err := p.api.ValidateConfig(); err != nil {
			p.logger.Debug().Err(err).Msg("Gemini API 설정 검증 실패")
			p.api = nil
		}
	}

	if p.cli == nil && p.api == nil {
		return ErrNoAPIKey
	}

	return nil
}

// Supports는 주어진 모델명을 지원하는지 확인합니다.
func (p *HybridGeminiProvider) Supports(model string) bool {
	if p.cli != nil && p.cli.Supports(model) {
		return true
	}
	if p.api != nil && p.api.Supports(model) {
		return true
	}
	return false
}

// Execute는 CLI를 우선 시도하고, 실패 시 API로 폴백합니다.
func (p *HybridGeminiProvider) Execute(ctx context.Context, req ExecuteRequest) (*ExecuteResponse, error) {
	startTime := time.Now()

	// CLI 우선 시도
	if p.cli != nil {
		p.logger.Debug().
			Str("model", req.Model).
			Msg("Gemini CLI로 실행 시도")

		resp, err := p.cli.Execute(ctx, req)
		if err == nil {
			atomic.AddUint64(&p.cliSuccess, 1)
			p.logger.Debug().
				Int64("duration_ms", time.Since(startTime).Milliseconds()).
				Msg("Gemini CLI 실행 성공")
			return resp, nil
		}

		atomic.AddUint64(&p.cliFailed, 1)
		p.logger.Warn().
			Err(err).
			Int64("duration_ms", time.Since(startTime).Milliseconds()).
			Msg("Gemini CLI 실행 실패, API로 폴백")
	}

	// API 폴백
	if p.api != nil {
		p.logger.Debug().
			Str("model", req.Model).
			Msg("Gemini API로 실행 시도")

		resp, err := p.api.Execute(ctx, req)
		if err == nil {
			atomic.AddUint64(&p.apiSuccess, 1)
			p.logger.Debug().
				Int64("duration_ms", time.Since(startTime).Milliseconds()).
				Msg("Gemini API 실행 성공")
			return resp, nil
		}

		atomic.AddUint64(&p.apiFailed, 1)
		p.logger.Error().
			Err(err).
			Int64("duration_ms", time.Since(startTime).Milliseconds()).
			Msg("Gemini API 실행도 실패")
		return nil, err
	}

	return nil, ErrNoAPIKey
}

// Stats는 프로바이더 사용 통계를 반환합니다.
func (p *HybridGeminiProvider) Stats() HybridStats {
	return HybridStats{
		CLISuccess:   atomic.LoadUint64(&p.cliSuccess),
		CLIFailed:    atomic.LoadUint64(&p.cliFailed),
		APISuccess:   atomic.LoadUint64(&p.apiSuccess),
		APIFailed:    atomic.LoadUint64(&p.apiFailed),
		CLIAvailable: p.cli != nil,
		APIAvailable: p.api != nil,
	}
}

// HasCLI는 CLI 프로바이더가 사용 가능한지 반환합니다.
func (p *HybridGeminiProvider) HasCLI() bool {
	return p.cli != nil
}

// HasAPI는 API 프로바이더가 사용 가능한지 반환합니다.
func (p *HybridGeminiProvider) HasAPI() bool {
	return p.api != nil
}
