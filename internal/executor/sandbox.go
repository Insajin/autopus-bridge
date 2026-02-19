// Package executor는 Local Agent Bridge의 작업 실행 엔진을 제공합니다.
// SEC-P2-03: 작업 디렉토리 샌드박싱으로 비인가 파일 접근 방지
package executor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/insajin/autopus-bridge/internal/config"
)

// 샌드박스 에러 코드
const (
	// ErrorCodeSandboxViolation은 샌드박스 정책 위반 시 사용됩니다.
	ErrorCodeSandboxViolation = "SANDBOX_VIOLATION"
)

// defaultDeniedPaths는 항상 거부되는 경로 목록입니다.
// allowlist에 포함되어 있어도 이 경로들은 접근이 차단됩니다.
var defaultDeniedPaths = []string{
	"~/.ssh",
	"~/.gnupg",
	"~/.config",
	"~/.aws",
	"/etc",
	"/var",
}

// Sandbox는 파일시스템 접근을 제한하는 샌드박스입니다.
// SEC-P2-03: 작업 디렉토리 샌드박싱
type Sandbox struct {
	// enabled는 샌드박스 활성화 여부입니다.
	enabled bool
	// allowedPaths는 접근이 허용된 절대 경로 목록입니다.
	allowedPaths []string
	// deniedPaths는 접근이 거부된 절대 경로 목록입니다.
	deniedPaths []string
	// denyHidden는 숨김 디렉토리 접근 거부 여부입니다.
	denyHidden bool
}

// NewSandbox는 설정에서 새로운 Sandbox를 생성합니다.
func NewSandbox(cfg config.SandboxConfig) *Sandbox {
	s := &Sandbox{
		enabled:    cfg.Enabled,
		denyHidden: cfg.DenyHiddenDirs,
	}

	// 허용 경로 확장 및 정규화
	s.allowedPaths = expandAndCleanPaths(cfg.AllowedPaths)

	// 거부 경로: 사용자 설정 + 기본 거부 경로 병합
	allDenied := make([]string, 0, len(cfg.DeniedPaths)+len(defaultDeniedPaths))
	allDenied = append(allDenied, cfg.DeniedPaths...)
	allDenied = append(allDenied, defaultDeniedPaths...)
	s.deniedPaths = expandAndCleanPaths(deduplicate(allDenied))

	return s
}

// ValidatePath는 주어진 경로가 샌드박스 정책을 충족하는지 검증합니다.
// 비활성화된 경우 항상 nil을 반환합니다.
func (s *Sandbox) ValidatePath(path string) error {
	if !s.enabled {
		return nil
	}

	// 경로 정규화: 절대 경로로 변환 및 정리
	absPath, err := normalizePath(path)
	if err != nil {
		return fmt.Errorf("경로 정규화 실패 '%s': %w", path, err)
	}

	// 심볼릭 링크 해석하여 실제 경로 확인
	resolvedPath, err := resolveSymlinks(absPath)
	if err != nil {
		// 경로가 존재하지 않는 경우 absPath 사용 (새 디렉토리 생성 가능)
		resolvedPath = absPath
	}

	// 1단계: 거부 경로 확인 (거부가 허용보다 우선)
	if denied, deniedPath := s.isDeniedPath(resolvedPath); denied {
		return fmt.Errorf("접근 거부: '%s' 경로는 보안 정책에 의해 차단됩니다 (거부 규칙: %s)", path, deniedPath)
	}

	// 2단계: 숨김 디렉토리 확인
	if s.denyHidden {
		if hidden, component := hasHiddenComponent(resolvedPath); hidden {
			return fmt.Errorf("접근 거부: '%s' 경로에 숨김 디렉토리 '%s'가 포함되어 있습니다", path, component)
		}
	}

	// 3단계: 허용 경로 확인
	if len(s.allowedPaths) == 0 {
		return fmt.Errorf("접근 거부: '%s' - 허용된 경로가 설정되지 않았습니다", path)
	}

	if !s.isAllowedPath(resolvedPath) {
		return fmt.Errorf("접근 거부: '%s' 경로는 허용된 작업 디렉토리 범위를 벗어납니다", path)
	}

	return nil
}

// ValidateWorkDir는 작업 디렉토리가 샌드박스 정책을 충족하는지 검증합니다.
// 빈 WorkDir는 허용됩니다 (기본 작업 디렉토리 사용).
func (s *Sandbox) ValidateWorkDir(workDir string) error {
	if workDir == "" {
		return nil
	}
	return s.ValidatePath(workDir)
}

// isDeniedPath는 경로가 거부 목록에 해당하는지 확인합니다.
// 거부된 경우 true와 매칭된 거부 경로를 반환합니다.
func (s *Sandbox) isDeniedPath(absPath string) (bool, string) {
	for _, denied := range s.deniedPaths {
		if isSubPath(absPath, denied) {
			return true, denied
		}
	}
	return false, ""
}

// isAllowedPath는 경로가 허용 목록에 해당하는지 확인합니다.
func (s *Sandbox) isAllowedPath(absPath string) bool {
	for _, allowed := range s.allowedPaths {
		if isSubPath(absPath, allowed) {
			return true
		}
	}
	return false
}

// isSubPath는 path가 basePath의 하위 경로(또는 동일)인지 확인합니다.
// prefix 매칭을 사용하되, 디렉토리 경계를 올바르게 처리합니다.
func isSubPath(path, basePath string) bool {
	// 정확히 같은 경로
	if path == basePath {
		return true
	}

	// basePath가 path의 접두사이고, 다음 문자가 경로 구분자인지 확인
	// 예: /home/user/projects 는 /home/user/project 의 하위가 아님
	prefix := basePath
	if !strings.HasSuffix(prefix, string(filepath.Separator)) {
		prefix += string(filepath.Separator)
	}
	return strings.HasPrefix(path, prefix)
}

// hasHiddenComponent는 경로에 숨김 디렉토리(. 으로 시작하는 컴포넌트)가 있는지 확인합니다.
// 루트 디렉토리와 현재/상위 디렉토리 (., ..)는 제외합니다.
func hasHiddenComponent(absPath string) (bool, string) {
	// 경로를 컴포넌트로 분리
	parts := strings.Split(absPath, string(filepath.Separator))
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			continue
		}
		if strings.HasPrefix(part, ".") {
			return true, part
		}
	}
	return false, ""
}

// normalizePath는 경로를 정규화합니다.
// 상대 경로를 절대 경로로 변환하고, Clean을 적용합니다.
func normalizePath(path string) (string, error) {
	// ~ 확장
	expanded := expandTilde(path)

	// 절대 경로로 변환
	absPath, err := filepath.Abs(expanded)
	if err != nil {
		return "", err
	}

	// 경로 정리 (., .., 중복 구분자 제거)
	cleaned := filepath.Clean(absPath)

	return cleaned, nil
}

// resolveSymlinks는 심볼릭 링크를 해석하여 실제 경로를 반환합니다.
// 샌드박스 탈출을 방지하기 위해 사용됩니다.
func resolveSymlinks(path string) (string, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	return filepath.Clean(resolved), nil
}

// expandTilde는 경로의 ~ 를 사용자 홈 디렉토리로 확장합니다.
func expandTilde(path string) string {
	if path == "" {
		return ""
	}
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		if path == "~" {
			return home
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

// expandAndCleanPaths는 경로 목록의 각 항목을 확장하고 정규화합니다.
func expandAndCleanPaths(paths []string) []string {
	result := make([]string, 0, len(paths))
	for _, p := range paths {
		expanded := expandTilde(p)
		cleaned := filepath.Clean(expanded)
		// 절대 경로로 변환 (가능한 경우)
		if abs, err := filepath.Abs(cleaned); err == nil {
			cleaned = abs
		}
		result = append(result, cleaned)
	}
	return result
}

// deduplicate는 문자열 슬라이스에서 중복을 제거합니다.
func deduplicate(items []string) []string {
	seen := make(map[string]bool, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	return result
}
