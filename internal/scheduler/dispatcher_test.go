package scheduler

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

// mockFetcher는 테스트용 ScheduleFetcher 구현입니다.
type mockFetcher struct {
	mu        sync.Mutex
	schedules []ScheduleInfo
	err       error
	callCount int
}

func (m *mockFetcher) FetchSchedules(_ context.Context) ([]ScheduleInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount++
	return m.schedules, m.err
}

func (m *mockFetcher) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

// mockTrigger는 테스트용 TaskTrigger 구현입니다.
type mockTrigger struct {
	mu         sync.Mutex
	executions []triggerCall
	err        error
}

type triggerCall struct {
	AgentID string
	Prompt  string
}

func (m *mockTrigger) TriggerExecution(_ context.Context, agentID, prompt string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.executions = append(m.executions, triggerCall{AgentID: agentID, Prompt: prompt})
	return m.err
}

func (m *mockTrigger) Executions() []triggerCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]triggerCall, len(m.executions))
	copy(result, m.executions)
	return result
}

func TestDispatcher_CheckAndDispatch_MatchingSchedule(t *testing.T) {
	t.Parallel()

	now := time.Now()
	// 현재 시각에 매칭되는 cron 생성
	cronExpr := minuteHourCron(now.Minute(), now.Hour())

	fetcher := &mockFetcher{
		schedules: []ScheduleInfo{
			{
				ID:             "sched-001",
				Name:           "Daily Standup",
				CronExpression: cronExpr,
				TargetAgentID:  "agent-ceo",
				TaskTemplate:   "일일 스탠드업 미팅 진행",
				IsActive:       true,
			},
		},
	}
	trigger := &mockTrigger{}
	logger := zerolog.Nop()

	d := NewDispatcher(fetcher, trigger, logger, 60*time.Second)
	d.checkAndDispatch(context.Background())

	execs := trigger.Executions()
	if len(execs) != 1 {
		t.Fatalf("실행 횟수 = %d, want 1", len(execs))
	}
	if execs[0].AgentID != "agent-ceo" {
		t.Errorf("AgentID = %q, want agent-ceo", execs[0].AgentID)
	}
	if execs[0].Prompt != "일일 스탠드업 미팅 진행" {
		t.Errorf("Prompt = %q, want 일일 스탠드업 미팅 진행", execs[0].Prompt)
	}
}

func TestDispatcher_CheckAndDispatch_NoMatch(t *testing.T) {
	t.Parallel()

	// 현재 시각과 절대 매칭되지 않는 cron (다른 시간대)
	now := time.Now()
	otherHour := (now.Hour() + 12) % 24

	fetcher := &mockFetcher{
		schedules: []ScheduleInfo{
			{
				ID:             "sched-002",
				Name:           "Off-hour Task",
				CronExpression: minuteHourCron(0, otherHour),
				TargetAgentID:  "agent-cto",
				IsActive:       true,
			},
		},
	}
	trigger := &mockTrigger{}
	logger := zerolog.Nop()

	d := NewDispatcher(fetcher, trigger, logger, 60*time.Second)
	d.checkAndDispatch(context.Background())

	if len(trigger.Executions()) != 0 {
		t.Errorf("매칭되지 않는 스케줄이 실행되었습니다")
	}
}

func TestDispatcher_CheckAndDispatch_InactiveSchedule(t *testing.T) {
	t.Parallel()

	now := time.Now()
	cronExpr := minuteHourCron(now.Minute(), now.Hour())

	fetcher := &mockFetcher{
		schedules: []ScheduleInfo{
			{
				ID:             "sched-003",
				Name:           "Inactive",
				CronExpression: cronExpr,
				TargetAgentID:  "agent-coo",
				IsActive:       false,
			},
		},
	}
	trigger := &mockTrigger{}
	logger := zerolog.Nop()

	d := NewDispatcher(fetcher, trigger, logger, 60*time.Second)
	d.checkAndDispatch(context.Background())

	if len(trigger.Executions()) != 0 {
		t.Errorf("비활성 스케줄이 실행되었습니다")
	}
}

func TestDispatcher_CheckAndDispatch_DuplicatePrevention(t *testing.T) {
	t.Parallel()

	now := time.Now()
	cronExpr := minuteHourCron(now.Minute(), now.Hour())

	fetcher := &mockFetcher{
		schedules: []ScheduleInfo{
			{
				ID:             "sched-004",
				Name:           "Duplicate Test",
				CronExpression: cronExpr,
				TargetAgentID:  "agent-cmo",
				IsActive:       true,
			},
		},
	}
	trigger := &mockTrigger{}
	logger := zerolog.Nop()

	d := NewDispatcher(fetcher, trigger, logger, 60*time.Second)

	// 첫 번째 체크 - 실행됨
	d.checkAndDispatch(context.Background())
	if len(trigger.Executions()) != 1 {
		t.Fatalf("첫 번째 실행 횟수 = %d, want 1", len(trigger.Executions()))
	}

	// 같은 분 내 두 번째 체크 - 중복 방지
	d.checkAndDispatch(context.Background())
	if len(trigger.Executions()) != 1 {
		t.Errorf("중복 실행 방지 실패: 실행 횟수 = %d, want 1", len(trigger.Executions()))
	}
}

func TestDispatcher_CheckAndDispatch_DefaultPrompt(t *testing.T) {
	t.Parallel()

	now := time.Now()
	cronExpr := minuteHourCron(now.Minute(), now.Hour())

	fetcher := &mockFetcher{
		schedules: []ScheduleInfo{
			{
				ID:             "sched-005",
				Name:           "No Template",
				CronExpression: cronExpr,
				TargetAgentID:  "agent-dev-pm",
				// TaskTemplate 비어있음
				IsActive: true,
			},
		},
	}
	trigger := &mockTrigger{}
	logger := zerolog.Nop()

	d := NewDispatcher(fetcher, trigger, logger, 60*time.Second)
	d.checkAndDispatch(context.Background())

	execs := trigger.Executions()
	if len(execs) != 1 {
		t.Fatalf("실행 횟수 = %d, want 1", len(execs))
	}
	want := "[스케줄 자동 실행] No Template"
	if execs[0].Prompt != want {
		t.Errorf("Prompt = %q, want %q", execs[0].Prompt, want)
	}
}

func TestDispatcher_CheckAndDispatch_MissingAgentID(t *testing.T) {
	t.Parallel()

	now := time.Now()
	cronExpr := minuteHourCron(now.Minute(), now.Hour())

	fetcher := &mockFetcher{
		schedules: []ScheduleInfo{
			{
				ID:             "sched-006",
				Name:           "No Agent",
				CronExpression: cronExpr,
				// TargetAgentID 비어있음
				IsActive: true,
			},
		},
	}
	trigger := &mockTrigger{}
	logger := zerolog.Nop()

	d := NewDispatcher(fetcher, trigger, logger, 60*time.Second)
	d.checkAndDispatch(context.Background())

	if len(trigger.Executions()) != 0 {
		t.Errorf("에이전트 ID가 없는 스케줄이 실행되었습니다")
	}
}

func TestDispatcher_CheckAndDispatch_MultipleSchedules(t *testing.T) {
	t.Parallel()

	now := time.Now()
	matchCron := minuteHourCron(now.Minute(), now.Hour())
	noMatchCron := minuteHourCron(0, (now.Hour()+12)%24)

	fetcher := &mockFetcher{
		schedules: []ScheduleInfo{
			{
				ID:             "sched-a",
				Name:           "Matching A",
				CronExpression: matchCron,
				TargetAgentID:  "agent-a",
				IsActive:       true,
			},
			{
				ID:             "sched-b",
				Name:           "Not Matching B",
				CronExpression: noMatchCron,
				TargetAgentID:  "agent-b",
				IsActive:       true,
			},
			{
				ID:             "sched-c",
				Name:           "Matching C",
				CronExpression: matchCron,
				TargetAgentID:  "agent-c",
				IsActive:       true,
			},
		},
	}
	trigger := &mockTrigger{}
	logger := zerolog.Nop()

	d := NewDispatcher(fetcher, trigger, logger, 60*time.Second)
	d.checkAndDispatch(context.Background())

	execs := trigger.Executions()
	if len(execs) != 2 {
		t.Fatalf("실행 횟수 = %d, want 2", len(execs))
	}

	agents := map[string]bool{}
	for _, e := range execs {
		agents[e.AgentID] = true
	}
	if !agents["agent-a"] || !agents["agent-c"] {
		t.Errorf("예상된 에이전트가 실행되지 않았습니다: %v", agents)
	}
}

func TestDispatcher_Start_ContextCancellation(t *testing.T) {
	t.Parallel()

	fetcher := &mockFetcher{
		schedules: []ScheduleInfo{},
	}
	trigger := &mockTrigger{}
	logger := zerolog.Nop()

	d := NewDispatcher(fetcher, trigger, logger, 50*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		d.Start(ctx)
		close(done)
	}()

	// 최소 1회 이상 실행 대기
	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// 정상 종료
	case <-time.After(2 * time.Second):
		t.Fatal("디스패처가 종료되지 않았습니다")
	}

	if fetcher.CallCount() < 1 {
		t.Errorf("FetchSchedules 호출 횟수 = %d, want >= 1", fetcher.CallCount())
	}
}

func TestDispatcher_LastRunTimes(t *testing.T) {
	t.Parallel()

	d := NewDispatcher(nil, nil, zerolog.Nop(), 60*time.Second)
	now := time.Now()

	d.markRun("sched-1", now)
	d.markRun("sched-2", now.Add(-5*time.Minute))

	times := d.LastRunTimes()
	if len(times) != 2 {
		t.Fatalf("LastRunTimes 길이 = %d, want 2", len(times))
	}
	if !times["sched-1"].Equal(now) {
		t.Errorf("sched-1 시각 불일치")
	}
}

func TestNewDispatcher_DefaultInterval(t *testing.T) {
	t.Parallel()

	d := NewDispatcher(nil, nil, zerolog.Nop(), 0)
	if d.interval != 60*time.Second {
		t.Errorf("기본 interval = %v, want 60s", d.interval)
	}
}

// minuteHourCron은 특정 minute, hour에만 매칭되는 cron 표현식을 생성합니다.
func minuteHourCron(minute, hour int) string {
	return fmt.Sprintf("%d %d * * *", minute, hour)
}
