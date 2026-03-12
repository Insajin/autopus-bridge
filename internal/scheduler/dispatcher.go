package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// ScheduleInfo는 서버에서 가져온 스케줄 정보입니다.
type ScheduleInfo struct {
	ID             string          `json:"id"`
	Name           string          `json:"name"`
	CronExpression string          `json:"cron_expression,omitempty"`
	Timezone       string          `json:"timezone,omitempty"`
	TargetAgentID  string          `json:"target_agent_id,omitempty"`
	TaskTemplate   json.RawMessage `json:"task_template,omitempty"`
	IsActive       bool            `json:"is_active"`
}

// GetPrompt는 TaskTemplate에서 프롬프트 문자열을 추출합니다.
// string인 경우 그대로 반환하고, object인 경우 "prompt" 필드를 추출합니다.
func (s *ScheduleInfo) GetPrompt() string {
	if len(s.TaskTemplate) == 0 {
		return ""
	}

	// string 시도
	var str string
	if err := json.Unmarshal(s.TaskTemplate, &str); err == nil {
		return str
	}

	// object 시도 — prompt 필드 추출
	var obj map[string]interface{}
	if err := json.Unmarshal(s.TaskTemplate, &obj); err == nil {
		if p, ok := obj["prompt"]; ok {
			if ps, ok := p.(string); ok {
				return ps
			}
		}
		// prompt 필드 없으면 description 시도
		if d, ok := obj["description"]; ok {
			if ds, ok := d.(string); ok {
				return ds
			}
		}
	}

	return ""
}

// ScheduleFetcher는 서버에서 스케줄 목록을 가져오는 인터페이스입니다.
type ScheduleFetcher interface {
	FetchSchedules(ctx context.Context) ([]ScheduleInfo, error)
}

// TaskTrigger는 에이전트 태스크 실행을 트리거하는 인터페이스입니다.
type TaskTrigger interface {
	TriggerExecution(ctx context.Context, agentID, prompt string) error
}

// Dispatcher는 스케줄 기반으로 에이전트 태스크를 디스패치합니다.
// 주기적으로 서버에서 스케줄을 가져와 cron 표현식을 평가하고,
// 매칭 시 태스크 실행을 트리거합니다.
type Dispatcher struct {
	fetcher  ScheduleFetcher
	trigger  TaskTrigger
	logger   zerolog.Logger
	interval time.Duration

	mu          sync.Mutex
	lastRun     map[string]time.Time // 스케줄 ID -> 마지막 실행 시각 (분 단위 중복 방지)
	cronCache   map[string]*CronExpr // 스케줄 ID -> 파싱된 cron (캐시)
	cronExprMap map[string]string    // 스케줄 ID -> 원본 cron 문자열 (캐시 무효화용)
}

// NewDispatcher는 새 Dispatcher를 생성합니다.
// interval은 스케줄 체크 간격입니다 (기본 60초 권장).
func NewDispatcher(fetcher ScheduleFetcher, trigger TaskTrigger, logger zerolog.Logger, interval time.Duration) *Dispatcher {
	if interval <= 0 {
		interval = 60 * time.Second
	}
	return &Dispatcher{
		fetcher:     fetcher,
		trigger:     trigger,
		logger:      logger,
		interval:    interval,
		lastRun:     make(map[string]time.Time),
		cronCache:   make(map[string]*CronExpr),
		cronExprMap: make(map[string]string),
	}
}

// Start는 디스패처 루프를 시작합니다. 컨텍스트가 취소될 때까지 실행됩니다.
func (d *Dispatcher) Start(ctx context.Context) {
	d.logger.Info().
		Dur("interval", d.interval).
		Msg("스케줄 디스패처 시작")

	// 시작 직후 첫 체크
	d.checkAndDispatch(ctx)

	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			d.logger.Info().Msg("스케줄 디스패처 종료")
			return
		case <-ticker.C:
			d.checkAndDispatch(ctx)
		}
	}
}

// checkAndDispatch는 스케줄을 가져와 평가하고 매칭된 스케줄을 실행합니다.
func (d *Dispatcher) checkAndDispatch(ctx context.Context) {
	schedules, err := d.fetcher.FetchSchedules(ctx)
	if err != nil {
		d.logger.Error().Err(err).Msg("스케줄 목록 가져오기 실패")
		return
	}

	if len(schedules) == 0 {
		return
	}

	now := time.Now()

	for _, s := range schedules {
		if !s.IsActive || s.CronExpression == "" || s.TargetAgentID == "" {
			continue
		}

		// 타임존 적용
		evalTime := now
		if s.Timezone != "" {
			loc, err := time.LoadLocation(s.Timezone)
			if err != nil {
				d.logger.Warn().
					Str("schedule_id", s.ID).
					Str("timezone", s.Timezone).
					Err(err).
					Msg("타임존 로드 실패, UTC 사용")
			} else {
				evalTime = now.In(loc)
			}
		}

		// cron 파싱 (캐시)
		cronExpr, err := d.getCronExpr(s.ID, s.CronExpression)
		if err != nil {
			d.logger.Warn().
				Str("schedule_id", s.ID).
				Str("cron", s.CronExpression).
				Err(err).
				Msg("cron 표현식 파싱 실패")
			continue
		}

		if !cronExpr.Matches(evalTime) {
			continue
		}

		// 같은 분 내 중복 실행 방지
		if d.wasRunInSameMinute(s.ID, evalTime) {
			continue
		}

		// 실행 트리거
		prompt := s.GetPrompt()
		if prompt == "" {
			prompt = fmt.Sprintf("[스케줄 자동 실행] %s", s.Name)
		}

		d.logger.Info().
			Str("schedule_id", s.ID).
			Str("schedule_name", s.Name).
			Str("agent_id", s.TargetAgentID).
			Str("cron", s.CronExpression).
			Msg("스케줄 매칭 - 태스크 실행 트리거")

		if err := d.trigger.TriggerExecution(ctx, s.TargetAgentID, prompt); err != nil {
			d.logger.Error().
				Err(err).
				Str("schedule_id", s.ID).
				Str("agent_id", s.TargetAgentID).
				Msg("태스크 실행 트리거 실패")
			continue
		}

		d.markRun(s.ID, evalTime)

		d.logger.Info().
			Str("schedule_id", s.ID).
			Str("agent_id", s.TargetAgentID).
			Msg("태스크 실행 트리거 완료")
	}
}

// getCronExpr은 cron 표현식을 파싱하되, 캐시를 활용합니다.
func (d *Dispatcher) getCronExpr(id, expr string) (*CronExpr, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// 캐시된 표현식이 동일하면 재사용
	if cached, ok := d.cronCache[id]; ok && d.cronExprMap[id] == expr {
		return cached, nil
	}

	parsed, err := ParseCron(expr)
	if err != nil {
		return nil, err
	}

	d.cronCache[id] = parsed
	d.cronExprMap[id] = expr
	return parsed, nil
}

// wasRunInSameMinute는 같은 분에 이미 실행되었는지 확인합니다.
func (d *Dispatcher) wasRunInSameMinute(id string, t time.Time) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	last, ok := d.lastRun[id]
	if !ok {
		return false
	}

	return last.Year() == t.Year() &&
		last.Month() == t.Month() &&
		last.Day() == t.Day() &&
		last.Hour() == t.Hour() &&
		last.Minute() == t.Minute()
}

// markRun은 스케줄의 마지막 실행 시각을 기록합니다.
func (d *Dispatcher) markRun(id string, t time.Time) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.lastRun[id] = t
}

// LastRunTimes는 마지막 실행 시각 맵의 스냅샷을 반환합니다 (테스트용).
func (d *Dispatcher) LastRunTimes() map[string]time.Time {
	d.mu.Lock()
	defer d.mu.Unlock()

	result := make(map[string]time.Time, len(d.lastRun))
	for k, v := range d.lastRun {
		result[k] = v
	}
	return result
}
