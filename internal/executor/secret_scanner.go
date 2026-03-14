// Package executor는 Local Agent Bridge의 작업 실행 엔진을 제공합니다.
// REQ-006.4: 커밋 전 시크릿 스캔
package executor

import (
	"regexp"
	"strings"
)

// 시크릿 탐지 패턴
var (
	// AWS Access Key 패턴
	awsAccessKeyPattern = regexp.MustCompile(`AKIA[0-9A-Z]{16}`)
	// GitHub PAT 패턴 (ghp_, gho_, ghu_, ghs_, ghr_)
	githubPATPattern = regexp.MustCompile(`gh[porsu]_[A-Za-z0-9]{20,}`)
	// 일반 password 할당 패턴 (password = "value" 형식)
	passwordPattern = regexp.MustCompile(`(?i)(password|passwd|pwd)\s*[=:]\s*["']?[A-Za-z0-9!@#$%^&*()_+\-=\[\]{};':"\\|,.<>?\/]{6,}["']?`)
	// api_key / api_secret 할당 패턴
	apiKeyPattern = regexp.MustCompile(`(?i)(api_key|api_secret|secret_key|access_key|auth_token|access_token)\s*[=:]\s*["']?[A-Za-z0-9\-_/+]{8,}["']?`)
	// sk- 형식 API 키 패턴 (OpenAI, Anthropic sk-ant- 등)
	skAPIKeyPattern = regexp.MustCompile(`sk(-ant)?-[A-Za-z0-9\-_]{20,}`)
)

// 차단할 파일 패턴
var blockedFilePatterns = []string{
	".env",
	".env.local",
	".env.production",
	".env.staging",
	".env.development",
	"credentials.json",
	"secrets.json",
	"service-account.json",
	"id_rsa",
	"id_ecdsa",
	"id_ed25519",
}

// SecretFinding은 탐지된 시크릿 정보입니다.
type SecretFinding struct {
	// Type은 탐지된 시크릿 타입입니다.
	Type string
	// Match는 탐지된 패턴 매치 문자열입니다 (마스킹 처리).
	Match string
	// Path는 파일 경로 기반 탐지의 경우 파일 경로입니다.
	Path string
}

// FileViolation은 차단된 파일 정보입니다.
type FileViolation struct {
	// Path는 차단된 파일 경로입니다.
	Path string
	// Reason은 차단 이유입니다.
	Reason string
}

// SecretScanner는 코드 내 시크릿 패턴을 탐지합니다.
// REQ-006.4: 커밋 전 .env, API 키, 비밀번호 패턴 검사
// @MX:ANCHOR: [AUTO] 모든 커밋 전 시크릿 검사 진입점 (CodeOpsWorker에서 호출)
// @MX:REASON: CodeOpsWorker.Commit 에서 항상 호출되는 보안 게이트
// @MX:SPEC: SPEC-CODEOPS-001 REQ-006
type SecretScanner struct{}

// NewSecretScanner는 새로운 SecretScanner를 생성합니다.
func NewSecretScanner() *SecretScanner {
	return &SecretScanner{}
}

// Scan은 텍스트 콘텐츠에서 시크릿 패턴을 탐지합니다.
func (s *SecretScanner) Scan(content string) []SecretFinding {
	var findings []SecretFinding

	// AWS Access Key
	if matches := awsAccessKeyPattern.FindAllString(content, -1); len(matches) > 0 {
		for _, m := range matches {
			findings = append(findings, SecretFinding{
				Type:  "aws_access_key",
				Match: maskSecret(m),
			})
		}
	}

	// GitHub PAT
	if matches := githubPATPattern.FindAllString(content, -1); len(matches) > 0 {
		for _, m := range matches {
			findings = append(findings, SecretFinding{
				Type:  "github_pat",
				Match: maskSecret(m),
			})
		}
	}

	// sk- 형식 API 키
	if matches := skAPIKeyPattern.FindAllString(content, -1); len(matches) > 0 {
		for _, m := range matches {
			findings = append(findings, SecretFinding{
				Type:  "sk_api_key",
				Match: maskSecret(m),
			})
		}
	}

	// password 할당 패턴
	if matches := passwordPattern.FindAllString(content, -1); len(matches) > 0 {
		for _, m := range matches {
			findings = append(findings, SecretFinding{
				Type:  "password_assignment",
				Match: maskSecret(m),
			})
		}
	}

	// api_key 할당 패턴
	if matches := apiKeyPattern.FindAllString(content, -1); len(matches) > 0 {
		for _, m := range matches {
			findings = append(findings, SecretFinding{
				Type:  "api_key_assignment",
				Match: maskSecret(m),
			})
		}
	}

	return findings
}

// ScanFilePath는 파일 경로 자체가 차단 대상인지 검사합니다.
// .env, credentials.json 등 민감한 파일명을 탐지합니다.
func (s *SecretScanner) ScanFilePath(filePath string) []SecretFinding {
	// 경로에서 파일명만 추출
	name := filePath
	if idx := strings.LastIndex(filePath, "/"); idx >= 0 {
		name = filePath[idx+1:]
	}

	for _, blocked := range blockedFilePatterns {
		if name == blocked || strings.HasSuffix(name, blocked) {
			return []SecretFinding{
				{
					Type:  "blocked_file",
					Match: filePath,
					Path:  filePath,
				},
			}
		}
	}

	return nil
}

// ScanFileList는 변경 파일 목록에서 차단 대상 파일을 찾습니다.
// REQ-006.2: .env 파일이 변경에 포함된 경우 커밋 차단
func (s *SecretScanner) ScanFileList(files []string) []FileViolation {
	var violations []FileViolation

	for _, f := range files {
		if findings := s.ScanFilePath(f); len(findings) > 0 {
			violations = append(violations, FileViolation{
				Path:   f,
				Reason: "차단된 파일 타입: " + findings[0].Type,
			})
		}
	}

	return violations
}

// maskSecret은 시크릿 문자열을 마스킹합니다 (로그 출력용).
func maskSecret(secret string) string {
	if len(secret) <= 8 {
		return strings.Repeat("*", len(secret))
	}
	// 앞 4자만 보여주고 나머지 마스킹
	return secret[:4] + strings.Repeat("*", len(secret)-4)
}
