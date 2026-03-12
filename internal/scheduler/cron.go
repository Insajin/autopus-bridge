// Package scheduler는 스케줄 기반 에이전트 실행을 제공합니다.
// 서버에서 스케줄 목록을 주기적으로 가져와 cron 표현식을 평가하고,
// 매칭 시 execute API를 호출하여 에이전트 태스크를 트리거합니다.
package scheduler

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// CronExpr은 파싱된 5-필드 cron 표현식을 나타냅니다.
// 필드 순서: minute, hour, day-of-month, month, day-of-week
type CronExpr struct {
	Minute     fieldMatcher
	Hour       fieldMatcher
	DayOfMonth fieldMatcher
	Month      fieldMatcher
	DayOfWeek  fieldMatcher
}

// fieldMatcher는 cron 필드의 매칭 로직을 캡슐화합니다.
type fieldMatcher struct {
	values map[int]bool // 허용된 값 집합, nil이면 모든 값 허용 (*)
}

// matches는 주어진 값이 이 필드와 매칭되는지 반환합니다.
func (f fieldMatcher) matches(val int) bool {
	if f.values == nil {
		return true // wildcard (*)
	}
	return f.values[val]
}

// ParseCron은 표준 5-필드 cron 표현식을 파싱합니다.
// 지원: *(와일드카드), 숫자, 범위(1-5), 목록(1,3,5), 스텝(*/2, 1-10/3)
func ParseCron(expr string) (*CronExpr, error) {
	fields := strings.Fields(strings.TrimSpace(expr))
	if len(fields) != 5 {
		return nil, fmt.Errorf("cron 표현식은 5개 필드가 필요합니다: %q", expr)
	}

	minute, err := parseField(fields[0], 0, 59)
	if err != nil {
		return nil, fmt.Errorf("minute 필드 파싱 실패: %w", err)
	}
	hour, err := parseField(fields[1], 0, 23)
	if err != nil {
		return nil, fmt.Errorf("hour 필드 파싱 실패: %w", err)
	}
	dom, err := parseField(fields[2], 1, 31)
	if err != nil {
		return nil, fmt.Errorf("day-of-month 필드 파싱 실패: %w", err)
	}
	month, err := parseField(fields[3], 1, 12)
	if err != nil {
		return nil, fmt.Errorf("month 필드 파싱 실패: %w", err)
	}
	dow, err := parseField(fields[4], 0, 6)
	if err != nil {
		return nil, fmt.Errorf("day-of-week 필드 파싱 실패: %w", err)
	}

	return &CronExpr{
		Minute:     minute,
		Hour:       hour,
		DayOfMonth: dom,
		Month:      month,
		DayOfWeek:  dow,
	}, nil
}

// Matches는 주어진 시각이 이 cron 표현식과 매칭되는지 반환합니다.
func (c *CronExpr) Matches(t time.Time) bool {
	return c.Minute.matches(t.Minute()) &&
		c.Hour.matches(t.Hour()) &&
		c.DayOfMonth.matches(t.Day()) &&
		c.Month.matches(int(t.Month())) &&
		c.DayOfWeek.matches(int(t.Weekday()))
}

// parseField는 단일 cron 필드를 파싱합니다.
func parseField(field string, min, max int) (fieldMatcher, error) {
	if field == "*" {
		return fieldMatcher{values: nil}, nil
	}

	values := make(map[int]bool)

	// 쉼표로 분리된 목록 처리
	parts := strings.Split(field, ",")
	for _, part := range parts {
		if err := parsePart(part, min, max, values); err != nil {
			return fieldMatcher{}, err
		}
	}

	if len(values) == 0 {
		return fieldMatcher{}, fmt.Errorf("빈 필드: %q", field)
	}

	return fieldMatcher{values: values}, nil
}

// parsePart는 단일 cron 필드 파트를 파싱합니다 (범위, 스텝, 숫자).
func parsePart(part string, min, max int, values map[int]bool) error {
	// 스텝 처리: */2, 1-10/3
	step := 1
	if idx := strings.Index(part, "/"); idx != -1 {
		var err error
		step, err = strconv.Atoi(part[idx+1:])
		if err != nil || step <= 0 {
			return fmt.Errorf("유효하지 않은 스텝: %q", part)
		}
		part = part[:idx]
	}

	// 와일드카드 + 스텝: */2
	if part == "*" {
		for i := min; i <= max; i += step {
			values[i] = true
		}
		return nil
	}

	// 범위: 1-5
	if idx := strings.Index(part, "-"); idx != -1 {
		start, err := strconv.Atoi(part[:idx])
		if err != nil {
			return fmt.Errorf("유효하지 않은 범위 시작: %q", part)
		}
		end, err := strconv.Atoi(part[idx+1:])
		if err != nil {
			return fmt.Errorf("유효하지 않은 범위 끝: %q", part)
		}
		if start < min || end > max || start > end {
			return fmt.Errorf("범위 초과: %d-%d (허용: %d-%d)", start, end, min, max)
		}
		for i := start; i <= end; i += step {
			values[i] = true
		}
		return nil
	}

	// 단일 숫자
	val, err := strconv.Atoi(part)
	if err != nil {
		return fmt.Errorf("유효하지 않은 값: %q", part)
	}
	if val < min || val > max {
		return fmt.Errorf("값 범위 초과: %d (허용: %d-%d)", val, min, max)
	}
	values[val] = true
	return nil
}
