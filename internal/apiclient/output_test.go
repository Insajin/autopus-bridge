package apiclient_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/insajin/autopus-bridge/internal/apiclient"
)

// TestPrintTable은 tabwriter를 이용한 테이블 출력을 검증합니다.
func TestPrintTable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		headers []string
		rows    [][]string
		checks  []string
	}{
		{
			name:    "헤더와 행 출력",
			headers: []string{"ID", "Name", "Status"},
			rows: [][]string{
				{"1", "Alice", "active"},
				{"2", "Bob", "inactive"},
			},
			checks: []string{"ID", "Name", "Status", "Alice", "active", "Bob", "inactive"},
		},
		{
			name:    "헤더만 있는 경우",
			headers: []string{"ID", "Name"},
			rows:    nil,
			checks:  []string{"ID", "Name"},
		},
		{
			name:    "빈 테이블",
			headers: nil,
			rows:    nil,
			checks:  []string{},
		},
		{
			name:    "긴 텍스트가 있는 행",
			headers: []string{"Key", "Value"},
			rows: [][]string{
				{"very-long-key", "short"},
				{"k", "very long value that might affect column width"},
			},
			checks: []string{"very-long-key", "very long value that might affect column width"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			apiclient.PrintTable(&buf, tt.headers, tt.rows)
			output := buf.String()
			for _, check := range tt.checks {
				if !strings.Contains(output, check) {
					t.Errorf("출력에 %q가 없습니다\n출력: %s", check, output)
				}
			}
		})
	}
}

// TestPrintTable_ColumnAlignment는 열 정렬을 검증합니다.
func TestPrintTable_ColumnAlignment(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	apiclient.PrintTable(&buf, []string{"ID", "Name"}, [][]string{
		{"1", "Alice"},
		{"100", "Bob"},
	})

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	// 최소 2줄 (헤더 + 2개 행) 이상 출력되어야 함
	if len(lines) < 2 {
		t.Errorf("출력 라인 수 = %d, want >= 2\n출력: %s", len(lines), output)
	}
}

// TestPrintJSON은 JSON 형식 출력을 검증합니다.
func TestPrintJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  interface{}
		checks []string
	}{
		{
			name:   "맵 출력",
			input:  map[string]string{"name": "test", "id": "123"},
			checks: []string{"name", "test", "id", "123"},
		},
		{
			name: "구조체 출력",
			input: struct {
				ID   int    `json:"id"`
				Name string `json:"name"`
			}{ID: 1, Name: "테스트"},
			checks: []string{`"id"`, `"name"`, `"테스트"`},
		},
		{
			name:   "배열 출력",
			input:  []string{"a", "b", "c"},
			checks: []string{"a", "b", "c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			err := apiclient.PrintJSON(&buf, tt.input)
			if err != nil {
				t.Fatalf("PrintJSON() 오류: %v", err)
			}
			output := buf.String()
			for _, check := range tt.checks {
				if !strings.Contains(output, check) {
					t.Errorf("출력에 %q가 없습니다\n출력: %s", check, output)
				}
			}
		})
	}
}

// TestPrintJSON_PrettyFormat은 들여쓰기가 있는 pretty print를 검증합니다.
func TestPrintJSON_PrettyFormat(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := apiclient.PrintJSON(&buf, map[string]int{"count": 42})
	if err != nil {
		t.Fatalf("PrintJSON() 오류: %v", err)
	}

	output := buf.String()
	// pretty print이면 줄바꿈이 포함되어야 함
	if !strings.Contains(output, "\n") {
		t.Errorf("pretty print 출력에 줄바꿈이 없습니다\n출력: %s", output)
	}
}

// TestPrintJSON_InvalidInput은 직렬화 불가능한 입력 처리를 검증합니다.
func TestPrintJSON_InvalidInput(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	// 채널은 JSON 직렬화 불가
	err := apiclient.PrintJSON(&buf, make(chan int))
	if err == nil {
		t.Error("직렬화 불가능한 입력에서 에러를 기대했지만 성공했습니다")
	}
}

// TestPrintDetail은 키-값 쌍 출력을 검증합니다.
func TestPrintDetail(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		fields []apiclient.KeyValue
		checks []string
	}{
		{
			name: "기본 키-값 출력",
			fields: []apiclient.KeyValue{
				{Key: "ID", Value: "ws-123"},
				{Key: "Name", Value: "My Workspace"},
				{Key: "Status", Value: "active"},
			},
			checks: []string{"ID", "ws-123", "Name", "My Workspace", "Status", "active"},
		},
		{
			name:   "빈 필드 목록",
			fields: []apiclient.KeyValue{},
			checks: []string{},
		},
		{
			name: "빈 값",
			fields: []apiclient.KeyValue{
				{Key: "Key", Value: ""},
			},
			checks: []string{"Key"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			apiclient.PrintDetail(&buf, tt.fields)
			output := buf.String()
			for _, check := range tt.checks {
				if !strings.Contains(output, check) {
					t.Errorf("출력에 %q가 없습니다\n출력: %s", check, output)
				}
			}
		})
	}
}

// TestPrintDetail_Format은 상세 출력 형식을 검증합니다 (콜론 구분자).
func TestPrintDetail_Format(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	apiclient.PrintDetail(&buf, []apiclient.KeyValue{
		{Key: "Name", Value: "Alice"},
	})

	output := buf.String()
	// 키와 값이 구분자(: 또는 공백)로 구분되어야 함
	if !strings.Contains(output, "Name") || !strings.Contains(output, "Alice") {
		t.Errorf("출력 형식이 잘못되었습니다\n출력: %s", output)
	}
}
