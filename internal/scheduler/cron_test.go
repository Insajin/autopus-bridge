package scheduler

import (
	"testing"
	"time"
)

func TestParseCron_Valid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		expr string
	}{
		{name: "모든 와일드카드", expr: "* * * * *"},
		{name: "매시 정각", expr: "0 * * * *"},
		{name: "매일 9시", expr: "0 9 * * *"},
		{name: "평일 9시", expr: "0 9 * * 1-5"},
		{name: "월요일 9시", expr: "0 9 * * 1"},
		{name: "매월 1일 10시", expr: "0 10 1 * *"},
		{name: "스텝", expr: "*/15 * * * *"},
		{name: "목록", expr: "0,30 * * * *"},
		{name: "범위+스텝", expr: "0-30/10 * * * *"},
		{name: "복합", expr: "0 9 * * 1,3,5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			expr, err := ParseCron(tt.expr)
			if err != nil {
				t.Fatalf("ParseCron(%q) 실패: %v", tt.expr, err)
			}
			if expr == nil {
				t.Fatal("ParseCron이 nil을 반환했습니다")
			}
		})
	}
}

func TestParseCron_Invalid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		expr string
	}{
		{name: "필드 부족", expr: "0 9 * *"},
		{name: "필드 초과", expr: "0 9 * * * *"},
		{name: "빈 문자열", expr: ""},
		{name: "잘못된 숫자", expr: "abc * * * *"},
		{name: "범위 초과 minute", expr: "60 * * * *"},
		{name: "범위 초과 hour", expr: "0 24 * * *"},
		{name: "범위 초과 day", expr: "0 0 32 * *"},
		{name: "범위 초과 month", expr: "0 0 * 13 *"},
		{name: "범위 초과 dow", expr: "0 0 * * 7"},
		{name: "잘못된 스텝", expr: "*/0 * * * *"},
		{name: "역전된 범위", expr: "5-1 * * * *"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := ParseCron(tt.expr)
			if err == nil {
				t.Errorf("ParseCron(%q) 에러를 기대했지만 성공했습니다", tt.expr)
			}
		})
	}
}

func TestCronExpr_Matches(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		expr  string
		time  time.Time
		match bool
	}{
		{
			name:  "매일 9시 정각 매칭",
			expr:  "0 9 * * *",
			time:  time.Date(2026, 3, 12, 9, 0, 0, 0, time.UTC),
			match: true,
		},
		{
			name:  "매일 9시 정각 불일치 (9:01)",
			expr:  "0 9 * * *",
			time:  time.Date(2026, 3, 12, 9, 1, 0, 0, time.UTC),
			match: false,
		},
		{
			name:  "평일 9시 월요일 매칭",
			expr:  "0 9 * * 1-5",
			time:  time.Date(2026, 3, 9, 9, 0, 0, 0, time.UTC), // 월요일
			match: true,
		},
		{
			name:  "평일 9시 일요일 불일치",
			expr:  "0 9 * * 1-5",
			time:  time.Date(2026, 3, 8, 9, 0, 0, 0, time.UTC), // 일요일
			match: false,
		},
		{
			name:  "매월 1일 10시 매칭",
			expr:  "0 10 1 * *",
			time:  time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC),
			match: true,
		},
		{
			name:  "매월 1일 10시 2일 불일치",
			expr:  "0 10 1 * *",
			time:  time.Date(2026, 4, 2, 10, 0, 0, 0, time.UTC),
			match: false,
		},
		{
			name:  "15분 간격 매칭 (0분)",
			expr:  "*/15 * * * *",
			time:  time.Date(2026, 3, 12, 14, 0, 0, 0, time.UTC),
			match: true,
		},
		{
			name:  "15분 간격 매칭 (30분)",
			expr:  "*/15 * * * *",
			time:  time.Date(2026, 3, 12, 14, 30, 0, 0, time.UTC),
			match: true,
		},
		{
			name:  "15분 간격 불일치 (7분)",
			expr:  "*/15 * * * *",
			time:  time.Date(2026, 3, 12, 14, 7, 0, 0, time.UTC),
			match: false,
		},
		{
			name:  "목록 매칭 (월,수,금)",
			expr:  "0 9 * * 1,3,5",
			time:  time.Date(2026, 3, 11, 9, 0, 0, 0, time.UTC), // 수요일
			match: true,
		},
		{
			name:  "목록 불일치 (화요일)",
			expr:  "0 9 * * 1,3,5",
			time:  time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC), // 화요일
			match: false,
		},
		{
			name:  "와일드카드 전체 매칭",
			expr:  "* * * * *",
			time:  time.Date(2026, 3, 12, 15, 42, 0, 0, time.UTC),
			match: true,
		},
		{
			name:  "범위+스텝 매칭 (0분)",
			expr:  "0-30/10 * * * *",
			time:  time.Date(2026, 3, 12, 14, 0, 0, 0, time.UTC),
			match: true,
		},
		{
			name:  "범위+스텝 매칭 (20분)",
			expr:  "0-30/10 * * * *",
			time:  time.Date(2026, 3, 12, 14, 20, 0, 0, time.UTC),
			match: true,
		},
		{
			name:  "범위+스텝 불일치 (25분)",
			expr:  "0-30/10 * * * *",
			time:  time.Date(2026, 3, 12, 14, 25, 0, 0, time.UTC),
			match: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			expr, err := ParseCron(tt.expr)
			if err != nil {
				t.Fatalf("ParseCron(%q) 실패: %v", tt.expr, err)
			}
			got := expr.Matches(tt.time)
			if got != tt.match {
				t.Errorf("CronExpr.Matches(%v) = %v, want %v", tt.time, got, tt.match)
			}
		})
	}
}
