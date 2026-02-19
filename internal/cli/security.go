package cli

// CLI 명령어 실행을 위한 보안 검증기.
// SPEC-SKILL-V2-001 Block C: 명령어 화이트리스트/블랙리스트 기반 보안 검증
//
// 보안 검증 순서:
// 1. 빈 명령어 거부
// 2. 블랙리스트 명령어 차단 (위험 명령어)
// 3. 블랙리스트 패턴 차단 (위험 하위문자열)
// 4. 화이트리스트 접두사 확인 (허용된 명령어만 통과)
// 5. 작업 디렉토리 유효성 검증

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SecurityChecker는 CLI 명령어를 실행하기 전에 보안 검증을 수행합니다.
type SecurityChecker struct {
	// allowedPrefixes는 실행이 허용된 명령어 접두사 목록입니다.
	allowedPrefixes []string
	// blockedCommands는 항상 거부되는 위험 명령어 목록입니다.
	blockedCommands []string
	// blockedPatterns는 위험한 동작을 나타내는 하위문자열 패턴 목록입니다.
	blockedPatterns []string
}

// NewSecurityChecker는 안전한 기본값을 갖춘 SecurityChecker를 생성합니다.
func NewSecurityChecker() *SecurityChecker {
	return &SecurityChecker{
		allowedPrefixes: []string{
			// Go 관련
			"go ", "go test", "go build", "go vet", "go fmt",
			// Node.js 관련
			"npm ", "npx ", "yarn ", "pnpm ", "bun ",
			// Rust 관련
			"cargo ", "rustc ",
			// Python 관련
			"python ", "python3 ", "pip ", "pytest ",
			// Ruby 관련
			"ruby ", "bundle ", "rake ", "rspec ",
			// Elixir 관련
			"mix ", "elixir ",
			// Dart/Flutter 관련
			"dart ", "flutter ",
			// Java 관련
			"java ", "javac ", "mvn ", "gradle ",
			// 빌드 도구
			"make ",
			// 컨테이너
			"docker ", "docker-compose ",
			// Git (읽기 전용)
			"git status", "git log", "git diff", "git show",
			// 파일 조회 (읽기 전용)
			"ls ", "cat ", "head ", "tail ", "wc ",
			// 검색 도구
			"find ", "grep ", "rg ", "ag ",
			// 유틸리티
			"echo ", "date ", "env ",
			// 네트워크 (읽기 전용)
			"curl ", "wget ",
		},
		blockedCommands: []string{
			// 파일 시스템 파괴
			"rm -rf /", "rm -rf /*",
			// 디스크 포맷/덮어쓰기
			"mkfs", "dd if=",
			// 포크 폭탄
			":(){ :|:& };:",
			// 무차별 권한 변경
			"chmod -R 777 /",
			// 디바이스 직접 쓰기
			"> /dev/sda",
		},
		blockedPatterns: []string{
			// 권한 상승
			"sudo ",
			"su -",
			"passwd",
			// 시스템 제어
			"shutdown",
			"reboot",
			"halt",
			"init ",
			"systemctl",
			"service ",
			// 네트워크/방화벽
			"iptables",
			// 원격 접속
			"ssh ",
			"scp ",
			"rsync ",
			// 민감 파일 접근
			"/etc/shadow",
			"/etc/passwd",
			// 동적 코드 실행 (셸 이스케이프 방지)
			"eval ",
			"exec ",
		},
	}
}

// Validate는 명령어와 작업 디렉토리의 안전성을 검증합니다.
// 검증 실패 시 에러를 반환합니다.
func (s *SecurityChecker) Validate(command string, workDir string) error {
	command = strings.TrimSpace(command)
	if command == "" {
		return fmt.Errorf("빈 명령어")
	}

	// 블랙리스트 명령어 확인 (정확한 접두사 매칭)
	lowerCmd := strings.ToLower(command)
	for _, blocked := range s.blockedCommands {
		if strings.HasPrefix(lowerCmd, strings.ToLower(blocked)) {
			return fmt.Errorf("차단된 명령어: %s", blocked)
		}
	}

	// 블랙리스트 패턴 확인 (하위문자열 매칭)
	for _, pattern := range s.blockedPatterns {
		if strings.Contains(lowerCmd, strings.ToLower(pattern)) {
			return fmt.Errorf("차단된 패턴: %s", pattern)
		}
	}

	// 화이트리스트 접두사 확인
	allowed := false
	for _, prefix := range s.allowedPrefixes {
		if strings.HasPrefix(lowerCmd, strings.ToLower(prefix)) {
			allowed = true
			break
		}
	}
	if !allowed {
		return fmt.Errorf("허용되지 않은 명령어: %s", firstWord(command))
	}

	// 작업 디렉토리 유효성 검증
	if workDir != "" {
		absDir, err := filepath.Abs(workDir)
		if err != nil {
			return fmt.Errorf("잘못된 작업 디렉토리: %w", err)
		}
		info, err := os.Stat(absDir)
		if err != nil {
			return fmt.Errorf("작업 디렉토리 접근 불가: %w", err)
		}
		if !info.IsDir() {
			return fmt.Errorf("작업 디렉토리가 디렉토리가 아님: %s", absDir)
		}
	}

	return nil
}

// firstWord는 문자열의 첫 번째 단어(공백 기준)를 반환합니다.
func firstWord(s string) string {
	parts := strings.Fields(s)
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}
