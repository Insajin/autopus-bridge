// validate_test.go는 ValidateID와 NewContextWithTimeout의 단위 테스트를 제공합니다.
package apiclient_test

import (
	"strings"
	"testing"
	"time"

	"github.com/insajin/autopus-bridge/internal/apiclient"
)

// TestValidateID_Valid는 유효한 ID 패턴을 검증합니다.
func TestValidateID_Valid(t *testing.T) {
	t.Parallel()

	// UUID 형식 및 슬러그 형식의 유효한 ID 목록
	validIDs := []string{
		"abc123",
		"ABC-123",
		"uuid-1234-5678",
		"proj_001",
		"a",
		"a1b2c3d4-e5f6-7890-abcd-ef1234567890", // UUID 형식
		"UPPERCASE",
		"mixed_Case-123",
	}

	for _, id := range validIDs {
		t.Run(id, func(t *testing.T) {
			t.Parallel()
			if err := apiclient.ValidateID(id); err != nil {
				t.Errorf("ValidateID(%q)이 에러를 반환했습니다: %v", id, err)
			}
		})
	}
}

// TestValidateID_Invalid는 유효하지 않은 ID 패턴을 검증합니다.
func TestValidateID_Invalid(t *testing.T) {
	t.Parallel()

	// 유효하지 않아야 하는 ID 목록 (빈 문자열, 특수문자, 경로 순회, 공백 포함)
	invalidIDs := []struct {
		id   string
		desc string
	}{
		{"", "빈 문자열"},
		{"../etc/passwd", "경로 순회 시도"},
		{"id with space", "공백 포함"},
		{"id/slash", "슬래시 포함"},
		{"id.dot", "점 포함"},
		{"id@at", "@ 포함"},
		{"id#hash", "# 포함"},
		{"id!excl", "! 포함"},
		{"\t탭문자", "탭 문자"},
		{"한글ID", "한글 포함"},
	}

	for _, tc := range invalidIDs {
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()
			err := apiclient.ValidateID(tc.id)
			if err == nil {
				t.Errorf("ValidateID(%q)가 에러를 반환해야 하지만 nil을 반환했습니다", tc.id)
			}
			// 에러 메시지에 '유효하지 않은 ID' 포함 여부 확인
			if err != nil && !strings.Contains(err.Error(), "유효하지 않은 ID") {
				t.Errorf("에러 메시지에 '유효하지 않은 ID'가 없습니다: %v", err)
			}
		})
	}
}

// TestNewContextWithTimeout은 타임아웃 컨텍스트 생성을 검증합니다.
func TestNewContextWithTimeout(t *testing.T) {
	t.Parallel()

	// 컨텍스트 생성 및 cancel 함수 확인
	ctx, cancel := apiclient.NewContextWithTimeout(10 * time.Second)
	if ctx == nil {
		t.Fatal("NewContextWithTimeout()이 nil 컨텍스트를 반환했습니다")
	}
	if cancel == nil {
		t.Fatal("NewContextWithTimeout()이 nil cancel 함수를 반환했습니다")
	}
	defer cancel()

	// 데드라인이 설정되었는지 확인
	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("컨텍스트에 데드라인이 설정되지 않았습니다")
	}

	// 데드라인이 현재 시간보다 미래인지 확인
	remaining := time.Until(deadline)
	if remaining <= 0 {
		t.Errorf("데드라인이 현재 시간보다 과거입니다: remaining=%v", remaining)
	}
	if remaining > 10*time.Second+100*time.Millisecond {
		t.Errorf("데드라인이 너무 멀리 설정되었습니다: remaining=%v", remaining)
	}
}

// TestNewContextWithTimeout_Cancel은 cancel 호출 시 컨텍스트가 취소되는지 검증합니다.
func TestNewContextWithTimeout_Cancel(t *testing.T) {
	t.Parallel()

	ctx, cancel := apiclient.NewContextWithTimeout(10 * time.Second)
	// 즉시 cancel 호출
	cancel()

	// cancel 호출 후 Done 채널이 닫혀야 함
	select {
	case <-ctx.Done():
		// 정상: 컨텍스트가 취소됨
	default:
		t.Error("cancel 호출 후 ctx.Done() 채널이 닫혀야 합니다")
	}
}

// TestNewContextWithTimeout_DifferentDurations는 다양한 타임아웃 값을 검증합니다.
func TestNewContextWithTimeout_DifferentDurations(t *testing.T) {
	t.Parallel()

	durations := []time.Duration{
		1 * time.Second,
		5 * time.Second,
		30 * time.Second,
		10 * time.Minute,
	}

	for _, d := range durations {
		d := d // 루프 변수 캡처
		t.Run(d.String(), func(t *testing.T) {
			t.Parallel()
			ctx, cancel := apiclient.NewContextWithTimeout(d)
			defer cancel()

			deadline, ok := ctx.Deadline()
			if !ok {
				t.Fatal("컨텍스트에 데드라인이 설정되지 않았습니다")
			}

			remaining := time.Until(deadline)
			// 타이밍 오차 허용 (100ms)
			tolerance := 100 * time.Millisecond
			if remaining > d+tolerance {
				t.Errorf("데드라인이 너무 멀리 설정됨: want<=%v, got remaining=%v", d, remaining)
			}
			if remaining <= 0 {
				t.Errorf("데드라인이 즉시 만료됨: remaining=%v", remaining)
			}
		})
	}
}
