// Package executor는 Local Agent Bridge의 작업 실행 엔진을 제공합니다.
package executor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSecretScanner_DetectsSecrets는 시크릿 패턴 감지를 검증합니다.
// REQ-006.4: 커밋 전 시크릿 스캔
func TestSecretScanner_DetectsSecrets(t *testing.T) {
	t.Parallel()

	scanner := NewSecretScanner()

	tests := []struct {
		name      string
		content   string
		wantFound bool
		wantType  string
	}{
		{
			name:      ".env 파일 패턴 감지",
			content:   "SECRET=abc123\nDB_PASSWORD=mysecret",
			wantFound: true,
		},
		{
			name:      "AWS Access Key 패턴 감지",
			content:   "AKIA" + "IOSFODNN7EXAMPLE",
			wantFound: true,
			wantType:  "aws_access_key",
		},
		{
			name:      "GitHub PAT 패턴 감지",
			content:   "ghp_" + "ABCDEFGHIJKLMNOPabcd0123456789",
			wantFound: true,
			wantType:  "github_pat",
		},
		{
			name:      "일반 코드 - 시크릿 없음",
			content:   "func main() {\n  fmt.Println(\"hello world\")\n}",
			wantFound: false,
		},
		{
			name:      "password 변수 감지",
			content:   "password = \"super_secret_123\"",
			wantFound: true,
		},
		{
			name:      "api_key 할당 감지",
			content:   "api_key = \"sk-1234567890abcdef\"",
			wantFound: true,
		},
		{
			name:      "변수명만 있고 값 없음 - 허용",
			content:   "// API_KEY 환경변수를 설정하세요",
			wantFound: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			findings := scanner.Scan(tc.content)
			if tc.wantFound {
				assert.NotEmpty(t, findings, "시크릿이 감지되어야 합니다")
				if tc.wantType != "" {
					found := false
					for _, f := range findings {
						if f.Type == tc.wantType {
							found = true
							break
						}
					}
					assert.True(t, found, "특정 타입 '%s' 시크릿이 감지되어야 합니다", tc.wantType)
				}
			} else {
				assert.Empty(t, findings, "시크릿이 감지되지 않아야 합니다")
			}
		})
	}
}

// TestSecretScanner_ScanFilePath는 파일 경로 기반 스캔을 검증합니다.
func TestSecretScanner_ScanFilePath(t *testing.T) {
	t.Parallel()

	scanner := NewSecretScanner()

	// .env 파일은 항상 차단
	findings := scanner.ScanFilePath(".env")
	assert.NotEmpty(t, findings, ".env 파일은 항상 차단되어야 합니다")

	// .env.local도 차단
	findings = scanner.ScanFilePath(".env.local")
	assert.NotEmpty(t, findings)

	// 일반 Go 파일은 통과
	findings = scanner.ScanFilePath("main.go")
	assert.Empty(t, findings)

	// credentials 파일 차단
	findings = scanner.ScanFilePath("credentials.json")
	assert.NotEmpty(t, findings)
}

// TestSecretScanner_ScanFileList는 변경 파일 목록 전체를 스캔합니다.
func TestSecretScanner_ScanFileList(t *testing.T) {
	t.Parallel()

	scanner := NewSecretScanner()

	files := []string{"main.go", "service.go", ".env", "handler.go"}
	violations := scanner.ScanFileList(files)
	assert.NotEmpty(t, violations)
	assert.Contains(t, violations[0].Path, ".env")
}
