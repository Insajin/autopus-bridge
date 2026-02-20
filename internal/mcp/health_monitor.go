package mcp

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// ServerStats는 MCP 서버의 런타임 메트릭을 추적합니다.
type ServerStats struct {
	TotalCalls    int
	ErrorCount    int
	TotalRespMs   int64 // 평균 계산용 응답 시간 합계
	LastError     string
	LastErrorTime time.Time
}

// ServerHealth는 MCP 서버의 건강 상태 스냅샷입니다.
type ServerHealth struct {
	Name          string
	Status        string // "running", "stopped", "error"
	UptimeSeconds int64
	TotalCalls    int
	ErrorCount    int
	ErrorRate     float64
	AvgResponseMs int64
	MemoryMB      int
	LastError     *string
}

// HealthReport는 모든 MCP 서버의 건강 상태 보고서입니다.
type HealthReport struct {
	Servers    []ServerHealth
	ReportedAt time.Time
}

// HealthMonitor는 MCP 서버의 건강 상태를 주기적으로 수집하고 보고합니다.
// SPEC-SELF-EXPAND-001: MCP 서버 헬스 모니터링
type HealthMonitor struct {
	manager  *Manager
	interval time.Duration

	// stats는 서버별 호출 메트릭을 추적합니다.
	stats   map[string]*ServerStats
	statsMu sync.RWMutex

	cancel context.CancelFunc
}

// NewHealthMonitor는 새로운 HealthMonitor를 생성합니다.
func NewHealthMonitor(manager *Manager, interval time.Duration) *HealthMonitor {
	if interval <= 0 {
		interval = 60 * time.Second
	}

	return &HealthMonitor{
		manager:  manager,
		interval: interval,
		stats:    make(map[string]*ServerStats),
	}
}

// Start는 주기적인 헬스 모니터링 루프를 시작합니다.
// reportFn은 각 틱마다 수집된 HealthReport와 함께 호출됩니다.
// context 취소 또는 Stop() 호출로 종료됩니다.
func (h *HealthMonitor) Start(ctx context.Context, reportFn func(report HealthReport)) {
	monitorCtx, cancel := context.WithCancel(ctx)
	h.cancel = cancel

	go func() {
		ticker := time.NewTicker(h.interval)
		defer ticker.Stop()

		log.Info().
			Dur("interval", h.interval).
			Msg("[mcp-health] 헬스 모니터링 시작")

		for {
			select {
			case <-monitorCtx.Done():
				log.Info().Msg("[mcp-health] 헬스 모니터링 종료")
				return
			case <-ticker.C:
				report := h.CollectHealth()
				if len(report.Servers) > 0 {
					log.Debug().
						Int("servers", len(report.Servers)).
						Msg("[mcp-health] 헬스 리포트 수집 완료")
				}
				if reportFn != nil {
					reportFn(report)
				}
			}
		}
	}()
}

// Stop은 모니터링 루프를 종료합니다.
func (h *HealthMonitor) Stop() {
	if h.cancel != nil {
		h.cancel()
		h.cancel = nil
	}
}

// RecordCall은 MCP 서버 호출 결과를 기록합니다.
// 스레드 안전하게 호출 횟수, 에러 수, 응답 시간을 업데이트합니다.
func (h *HealthMonitor) RecordCall(serverName string, durationMs int64, err error) {
	h.statsMu.Lock()
	defer h.statsMu.Unlock()

	stats, ok := h.stats[serverName]
	if !ok {
		stats = &ServerStats{}
		h.stats[serverName] = stats
	}

	stats.TotalCalls++
	stats.TotalRespMs += durationMs

	if err != nil {
		stats.ErrorCount++
		stats.LastError = err.Error()
		stats.LastErrorTime = time.Now()
	}
}

// CollectHealth는 모든 실행 중인 MCP 서버의 건강 상태를 수집합니다.
func (h *HealthMonitor) CollectHealth() HealthReport {
	now := time.Now()
	running := h.manager.ListRunning()

	h.statsMu.RLock()
	defer h.statsMu.RUnlock()

	// 실행 중인 서버 이름을 빠르게 조회하기 위한 맵 (mutex 복사 방지)
	runningSet := make(map[string]struct{}, len(running))
	for i := range running {
		runningSet[running[i].Name] = struct{}{}
	}

	var servers []ServerHealth

	// 1. 실행 중인 서버 처리
	for i := range running {
		sh := ServerHealth{
			Name:          running[i].Name,
			Status:        "running",
			UptimeSeconds: int64(now.Sub(running[i].StartedAt).Seconds()),
			MemoryMB:      0, // OS별 메모리 조회는 생략, 0 반환
		}

		if stats, ok := h.stats[running[i].Name]; ok {
			sh.TotalCalls = stats.TotalCalls
			sh.ErrorCount = stats.ErrorCount
			sh.AvgResponseMs = calcAvgResponseMs(stats)
			sh.ErrorRate = calcErrorRate(stats)

			if stats.LastError != "" {
				lastErr := stats.LastError
				sh.LastError = &lastErr
			}

			// 에러율이 높으면 status를 "error"로 변경
			if sh.ErrorRate > 0.5 && sh.TotalCalls >= 10 {
				sh.Status = "error"
			}
		}

		servers = append(servers, sh)
	}

	// 2. 통계는 있지만 실행 중이 아닌 서버 (중지된 서버)
	for name, stats := range h.stats {
		if _, isRunning := runningSet[name]; isRunning {
			continue
		}

		sh := ServerHealth{
			Name:          name,
			Status:        "stopped",
			UptimeSeconds: 0,
			TotalCalls:    stats.TotalCalls,
			ErrorCount:    stats.ErrorCount,
			ErrorRate:     calcErrorRate(stats),
			AvgResponseMs: calcAvgResponseMs(stats),
			MemoryMB:      0,
		}

		if stats.LastError != "" {
			lastErr := stats.LastError
			sh.LastError = &lastErr
		}

		servers = append(servers, sh)
	}

	return HealthReport{
		Servers:    servers,
		ReportedAt: now,
	}
}

// calcAvgResponseMs는 평균 응답 시간을 계산합니다.
func calcAvgResponseMs(stats *ServerStats) int64 {
	if stats.TotalCalls == 0 {
		return 0
	}
	return stats.TotalRespMs / int64(stats.TotalCalls)
}

// calcErrorRate는 에러율을 계산합니다 (0.0 ~ 1.0).
func calcErrorRate(stats *ServerStats) float64 {
	if stats.TotalCalls == 0 {
		return 0
	}
	return float64(stats.ErrorCount) / float64(stats.TotalCalls)
}
