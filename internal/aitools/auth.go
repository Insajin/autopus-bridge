// auth.go는 AI 프로바이더별 CLI 인증 상태를 확인하는 기능을 제공합니다.
// 보안 원칙: 실제 API 키 값은 절대 로깅하거나 반환하지 않습니다.
package aitools

import (
	"os"
	"path/filepath"
	"strings"
)

// AuthStatus는 프로바이더의 인증 상태를 나타냅니다.
type AuthStatus string

const (
	// AuthStatusAuthenticated는 CLI 인증이 완료된 상태입니다.
	AuthStatusAuthenticated AuthStatus = "authenticated"
	// AuthStatusNotAuthenticated는 인증이 되지 않은 상태입니다.
	AuthStatusNotAuthenticated AuthStatus = "not_authenticated"
	// AuthStatusAPIKeyOnly는 API 키만 설정된 상태입니다 (CLI 로그인 미완료).
	AuthStatusAPIKeyOnly AuthStatus = "api_key_only"
	// AuthStatusUnknown는 인증 상태를 확인할 수 없는 상태입니다.
	AuthStatusUnknown AuthStatus = "unknown"
)

// AuthCheckResult는 단일 프로바이더의 인증 확인 결과를 담는 구조체입니다.
type AuthCheckResult struct {
	ProviderName     string     // "Claude", "Codex", "Gemini"
	Status           AuthStatus // 인증 상태
	CLIAuthenticated bool       // CLI 로그인 완료 여부
	HasAPIKey        bool       // API 키 환경변수 설정 여부
	APIKeyEnvName    string     // 발견된 환경변수 이름 (예: "ANTHROPIC_API_KEY")
	Message          string     // 사람이 읽을 수 있는 상태 메시지
}

// CheckClaudeAuth는 Claude CLI의 인증 상태를 확인합니다.
// ~/.claude/ 디렉토리 및 인증 파일, ANTHROPIC_API_KEY/CLAUDE_API_KEY 환경변수를 확인합니다.
func CheckClaudeAuth() AuthCheckResult {
	home, err := os.UserHomeDir()
	if err != nil {
		return AuthCheckResult{
			ProviderName: "Claude",
			Status:       AuthStatusUnknown,
			Message:      "홈 디렉토리를 확인할 수 없습니다",
		}
	}
	return checkClaudeAuthWithHome(home)
}

// CheckCodexAuth는 Codex CLI의 인증 상태를 확인합니다.
// ~/.codex/ 디렉토리 내 인증 파일, OPENAI_API_KEY 환경변수를 확인합니다.
func CheckCodexAuth() AuthCheckResult {
	home, err := os.UserHomeDir()
	if err != nil {
		return AuthCheckResult{
			ProviderName: "Codex",
			Status:       AuthStatusUnknown,
			Message:      "홈 디렉토리를 확인할 수 없습니다",
		}
	}
	return checkCodexAuthWithHome(home)
}

// CheckGeminiAuth는 Gemini CLI의 인증 상태를 확인합니다.
// ~/.gemini/ 디렉토리, gcloud 인증 정보, GEMINI_API_KEY/GOOGLE_API_KEY 환경변수를 확인합니다.
func CheckGeminiAuth() AuthCheckResult {
	home, err := os.UserHomeDir()
	if err != nil {
		return AuthCheckResult{
			ProviderName: "Gemini",
			Status:       AuthStatusUnknown,
			Message:      "홈 디렉토리를 확인할 수 없습니다",
		}
	}
	return checkGeminiAuthWithHome(home)
}

// CheckAllAuth는 요청된 프로바이더들의 인증 상태를 일괄 확인합니다.
// providerNames에 "Claude", "Codex", "Gemini"를 지정할 수 있습니다.
// 결과는 요청 순서와 동일한 순서로 반환됩니다.
func CheckAllAuth(providerNames []string) []AuthCheckResult {
	results := make([]AuthCheckResult, 0, len(providerNames))

	for _, name := range providerNames {
		switch strings.ToLower(name) {
		case "claude":
			results = append(results, CheckClaudeAuth())
		case "codex":
			results = append(results, CheckCodexAuth())
		case "gemini":
			results = append(results, CheckGeminiAuth())
		default:
			results = append(results, AuthCheckResult{
				ProviderName: name,
				Status:       AuthStatusUnknown,
				Message:      "지원하지 않는 프로바이더: " + name,
			})
		}
	}

	return results
}

// checkClaudeAuthWithHome는 지정된 홈 디렉토리를 기준으로 Claude 인증 상태를 확인합니다.
// Claude 인증 파일: ~/.claude/credentials.json, ~/.claude/auth.json, ~/.claude.json
func checkClaudeAuthWithHome(homeDir string) AuthCheckResult {
	claudeDir := filepath.Join(homeDir, ".claude")

	// CLI 인증 파일 확인 (~/.claude/ 내부 + ~/.claude.json)
	cliAuth := anyFileExists(
		filepath.Join(claudeDir, "credentials.json"),
		filepath.Join(claudeDir, "auth.json"),
		filepath.Join(homeDir, ".claude.json"),
	)

	// API 키 환경변수 확인 (보안: 값은 저장하지 않음)
	apiKeyEnv, hasKey := findAPIKeyEnv("ANTHROPIC_API_KEY", "CLAUDE_API_KEY")

	return buildAuthResult("Claude", cliAuth, hasKey, apiKeyEnv)
}

// checkCodexAuthWithHome는 지정된 홈 디렉토리를 기준으로 Codex 인증 상태를 확인합니다.
// Codex 인증 파일: ~/.codex/ 디렉토리 내 인증 관련 파일
func checkCodexAuthWithHome(homeDir string) AuthCheckResult {
	codexDir := filepath.Join(homeDir, ".codex")

	// CLI 인증 파일 확인
	cliAuth := anyFileExists(
		filepath.Join(codexDir, "auth.json"),
		filepath.Join(codexDir, "credentials.json"),
	)

	// API 키 환경변수 확인
	apiKeyEnv, hasKey := findAPIKeyEnv("OPENAI_API_KEY")

	return buildAuthResult("Codex", cliAuth, hasKey, apiKeyEnv)
}

// checkGeminiAuthWithHome는 지정된 홈 디렉토리를 기준으로 Gemini 인증 상태를 확인합니다.
// Gemini 인증 경로: ~/.gemini/ 디렉토리, ~/.config/gcloud/application_default_credentials.json
func checkGeminiAuthWithHome(homeDir string) AuthCheckResult {
	geminiDir := filepath.Join(homeDir, ".gemini")

	// CLI 인증 파일 확인 (gcloud ADC + ~/.gemini/ 내부)
	cliAuth := anyFileExists(
		filepath.Join(homeDir, ".config", "gcloud", "application_default_credentials.json"),
		filepath.Join(geminiDir, "auth.json"),
		filepath.Join(geminiDir, "credentials.json"),
	)

	// API 키 환경변수 확인
	apiKeyEnv, hasKey := findAPIKeyEnv("GEMINI_API_KEY", "GOOGLE_API_KEY")

	return buildAuthResult("Gemini", cliAuth, hasKey, apiKeyEnv)
}

// ---------------------------------------------------------------------------
// 공통 헬퍼 함수
// ---------------------------------------------------------------------------

// buildAuthResult는 CLI 인증 여부와 API 키 상태를 바탕으로 AuthCheckResult를 생성합니다.
func buildAuthResult(providerName string, cliAuth, hasAPIKey bool, apiKeyEnvName string) AuthCheckResult {
	result := AuthCheckResult{
		ProviderName:     providerName,
		CLIAuthenticated: cliAuth,
		HasAPIKey:        hasAPIKey,
		APIKeyEnvName:    apiKeyEnvName,
	}

	switch {
	case cliAuth:
		result.Status = AuthStatusAuthenticated
		result.Message = providerName + " CLI 인증이 완료되었습니다"
	case hasAPIKey:
		result.Status = AuthStatusAPIKeyOnly
		result.Message = apiKeyEnvName + " 환경변수가 설정되어 있습니다 (CLI 로그인은 미완료)"
	default:
		result.Status = AuthStatusNotAuthenticated
		result.Message = providerName + " 인증이 설정되지 않았습니다"
	}

	return result
}

// findAPIKeyEnv는 주어진 환경변수 이름 목록에서 값이 설정된 첫 번째 이름을 반환합니다.
// 보안: 환경변수의 값은 반환하지 않고 이름만 반환합니다.
func findAPIKeyEnv(envNames ...string) (envName string, found bool) {
	for _, name := range envNames {
		if os.Getenv(name) != "" {
			return name, true
		}
	}
	return "", false
}

// anyFileExists는 주어진 경로 중 하나라도 파일로 존재하면 true를 반환합니다.
// 디렉토리인 경우 false로 취급합니다.
func anyFileExists(paths ...string) bool {
	for _, p := range paths {
		if fileExists(p) {
			return true
		}
	}
	return false
}

// fileExists는 지정된 경로에 파일이 존재하는지 확인합니다.
// 디렉토리인 경우 false를 반환합니다.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}
