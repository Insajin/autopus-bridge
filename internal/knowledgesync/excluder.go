// Package knowledgesync 는 autopus-bridge 의 Knowledge Hub 파일 동기화 모듈입니다.
// SPEC-KHSOURCE-001 TASK-011
package knowledgesync

import (
	"path/filepath"
	"strings"
)

// DefaultExcludePatterns 는 기본 제외 파일 패턴 목록입니다.
// 민감한 파일과 빌드 아티팩트를 포함합니다.
// SPEC-KHSOURCE-001 TASK-011
var DefaultExcludePatterns = []string{
	// 환경 변수 / 시크릿
	".env",
	".env.*",
	// 인증서 / 키
	"*.key",
	"*.pem",
	"*.p12",
	"*.pfx",
	// 자격증명
	"credentials.*",
	"*_credentials.*",
	// VCS
	".git",
	".git/**",
	// 패키지 디렉토리
	"node_modules",
	"node_modules/**",
	// Python 캐시
	"__pycache__",
	"__pycache__/**",
	"*.pyc",
	// OS 메타데이터
	".DS_Store",
	"Thumbs.db",
}

// IsExcluded 는 path 가 patterns 중 하나와 매칭되면 true 를 반환합니다.
// glob 패턴을 사용하며, 경로의 모든 구성 요소를 검사합니다.
// SPEC-KHSOURCE-001 TASK-011
func IsExcluded(path string, patterns []string) bool {
	if len(patterns) == 0 {
		return false
	}

	// 경로 구성 요소를 슬래시로 정규화
	normalPath := filepath.ToSlash(path)
	base := filepath.Base(normalPath)

	for _, pattern := range patterns {
		// 패턴이 "/" 를 포함하면 전체 경로 매칭 시도
		if strings.Contains(pattern, "/") {
			// 접미사 "/**" 패턴: 디렉토리 하위 경로 매칭
			if strings.HasSuffix(pattern, "/**") {
				prefix := strings.TrimSuffix(pattern, "/**")
				if normalPath == prefix || strings.HasPrefix(normalPath, prefix+"/") {
					return true
				}
			} else {
				// 일반 슬래시 포함 패턴: 전체 경로 glob 매칭
				if matched, _ := filepath.Match(pattern, normalPath); matched {
					return true
				}
			}
		} else {
			// 슬래시 없는 패턴: 파일명(base) 매칭
			if matched, _ := filepath.Match(pattern, base); matched {
				return true
			}
			// 경로에 패턴 디렉토리가 포함되어 있는지 확인
			// 예: ".git" 패턴이 ".git/config" 를 매칭해야 함
			parts := strings.Split(normalPath, "/")
			for _, part := range parts {
				if matched, _ := filepath.Match(pattern, part); matched {
					return true
				}
			}
		}
	}

	return false
}

// MergePatterns 는 기본 패턴과 사용자 정의 패턴을 중복 없이 병합합니다.
// SPEC-KHSOURCE-001 TASK-011
func MergePatterns(defaults, custom []string) []string {
	seen := make(map[string]struct{}, len(defaults)+len(custom))
	result := make([]string, 0, len(defaults)+len(custom))

	for _, p := range defaults {
		if _, ok := seen[p]; !ok {
			seen[p] = struct{}{}
			result = append(result, p)
		}
	}

	for _, p := range custom {
		if _, ok := seen[p]; !ok {
			seen[p] = struct{}{}
			result = append(result, p)
		}
	}

	return result
}
