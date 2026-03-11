package knowledgesync

import (
	"testing"
)

// TestIsExcluded_MatchesDefaultPatterns 는 기본 제외 패턴에 매칭되는지 검증합니다.
func TestIsExcluded_MatchesDefaultPatterns(t *testing.T) {
	t.Parallel()

	cases := []struct {
		path     string
		patterns []string
		want     bool
	}{
		{".env", DefaultExcludePatterns, true},
		{".env.local", DefaultExcludePatterns, true},
		{"secret.key", DefaultExcludePatterns, true},
		{"cert.pem", DefaultExcludePatterns, true},
		{"credentials.json", DefaultExcludePatterns, true},
		{".git", DefaultExcludePatterns, true},
		{".git/config", DefaultExcludePatterns, true},
		{"node_modules/lodash/index.js", DefaultExcludePatterns, true},
		{"__pycache__/module.pyc", DefaultExcludePatterns, true},
		{"app.pyc", DefaultExcludePatterns, true},
		{".DS_Store", DefaultExcludePatterns, true},
		{"Thumbs.db", DefaultExcludePatterns, true},
		// 정상 파일
		{"docs/readme.md", DefaultExcludePatterns, false},
		{"src/main.go", DefaultExcludePatterns, false},
		{"config.yaml", DefaultExcludePatterns, false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.path, func(t *testing.T) {
			t.Parallel()
			got := IsExcluded(tc.path, tc.patterns)
			if got != tc.want {
				t.Errorf("IsExcluded(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

// TestIsExcluded_CustomPatterns 는 사용자 정의 패턴 매칭을 검증합니다.
func TestIsExcluded_CustomPatterns(t *testing.T) {
	t.Parallel()

	custom := []string{"*.log", "tmp/**", "build"}
	cases := []struct {
		path string
		want bool
	}{
		{"app.log", true},
		{"tmp/cache/file.txt", true},
		{"build", true},
		{"src/main.go", false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.path, func(t *testing.T) {
			t.Parallel()
			got := IsExcluded(tc.path, custom)
			if got != tc.want {
				t.Errorf("IsExcluded(%q, custom) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

// TestIsExcluded_EmptyPatterns 는 패턴이 없을 때 false 를 반환하는지 검증합니다.
func TestIsExcluded_EmptyPatterns(t *testing.T) {
	t.Parallel()
	if IsExcluded("any/path/file.md", nil) {
		t.Error("IsExcluded() = true for nil patterns, want false")
	}
}

// TestMergePatterns_CombinesDefaultAndCustom 는 기본+사용자 패턴 병합을 검증합니다.
func TestMergePatterns_CombinesDefaultAndCustom(t *testing.T) {
	t.Parallel()
	custom := []string{"*.log", "tmp/**"}
	merged := MergePatterns(DefaultExcludePatterns, custom)

	// 기본 패턴이 포함되어야 합니다
	if !contains(merged, ".env") {
		t.Error("MergePatterns() missing default pattern '.env'")
	}
	// 사용자 패턴이 포함되어야 합니다
	if !contains(merged, "*.log") {
		t.Error("MergePatterns() missing custom pattern '*.log'")
	}
	// 중복은 없어야 합니다
	if countOccurrences(merged, ".env") > 1 {
		t.Error("MergePatterns() has duplicate '.env'")
	}
}

// TestMergePatterns_NilCustom 은 custom 이 nil 일 때 기본 패턴만 반환하는지 검증합니다.
func TestMergePatterns_NilCustom(t *testing.T) {
	t.Parallel()
	merged := MergePatterns(DefaultExcludePatterns, nil)
	if len(merged) != len(DefaultExcludePatterns) {
		t.Errorf("MergePatterns(nil custom) len = %d, want %d", len(merged), len(DefaultExcludePatterns))
	}
}

// contains 는 슬라이스에 요소가 있는지 확인합니다.
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// countOccurrences 는 슬라이스에서 항목의 출현 횟수를 셉니다.
func countOccurrences(slice []string, item string) int {
	count := 0
	for _, s := range slice {
		if s == item {
			count++
		}
	}
	return count
}
