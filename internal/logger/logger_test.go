package logger

import (
	"testing"
)

// TestMaskSensitive는 민감 정보 마스킹 기능을 테스트합니다.
// REQ-U-02: 시스템은 항상 민감 정보(API 키, JWT 토큰)를 마스킹하여 로그에 기록해야 한다
func TestMaskSensitive(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Claude API 키 마스킹",
			input:    "API key: sk-ant-api03-abcdefghijklmnopqrstuvwxyz123456",
			expected: "API key: sk-a***3456",
		},
		{
			name:     "Gemini API 키 마스킹",
			input:    "Using AIzaSyAbcdefghijklmnopqrstuvwxyz123456789",
			expected: "Using AIza***6789",
		},
		{
			name:     "JWT 토큰 마스킹",
			input:    "Token: eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U",
			expected: "Token: eyJh***sR8U",
		},
		{
			name:     "Bearer 토큰 마스킹",
			input:    "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxIn0.rTCH8cLoGxAm_xw68z-zXVKi9ie6xJn9tnVWjd_9ftE",
			expected: "Authorization: Bearer ******9ftE",
		},
		{
			name:     "api_key= 패턴 마스킹",
			input:    "config api_key=sk-proj-abcdefghijklmnop",
			expected: "config api_key=sk-p***mnop",
		},
		{
			name:     "일반 텍스트는 변경 없음",
			input:    "This is a normal log message",
			expected: "This is a normal log message",
		},
		{
			name:     "빈 문자열",
			input:    "",
			expected: "",
		},
		{
			name:     "짧은 토큰",
			input:    "short key=abc123",
			expected: "short key=abc123", // 10자 미만이므로 마스킹 안 함
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskSensitive(tt.input)
			if result != tt.expected {
				t.Errorf("MaskSensitive() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestMaskValue는 개별 값 마스킹을 테스트합니다.
func TestMaskValue(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "긴 값 마스킹",
			input:    "abcdefghijklmnop",
			expected: "abcd***mnop",
		},
		{
			name:     "정확히 8자",
			input:    "12345678",
			expected: "***",
		},
		{
			name:     "8자 미만",
			input:    "short",
			expected: "***",
		},
		{
			name:     "빈 문자열",
			input:    "",
			expected: "***",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := maskValue(tt.input)
			if result != tt.expected {
				t.Errorf("maskValue() = %q, want %q", result, tt.expected)
			}
		})
	}
}
