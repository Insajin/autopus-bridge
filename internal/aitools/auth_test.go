package aitools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// UT-001: CheckClaudeAuth - 인증 디렉토리가 존재하면 authenticated
// ---------------------------------------------------------------------------

func TestCheckClaudeAuth(t *testing.T) {
	tests := []struct {
		name          string
		setup         func(t *testing.T, homeDir string)
		envVars       map[string]string
		wantStatus    AuthStatus
		wantCLI       bool
		wantAPI       bool
		wantEnvName   string
		wantNonEmpty  bool // Message가 비어있지 않아야 하는지
	}{
		{
			// UT-001: 인증 디렉토리와 인증 파일이 존재하면 authenticated
			name: "CLI 인증 완료 상태 (credentials.json 존재)",
			setup: func(t *testing.T, homeDir string) {
				claudeDir := filepath.Join(homeDir, ".claude")
				if err := os.MkdirAll(claudeDir, 0755); err != nil {
					t.Fatalf(".claude 디렉토리 생성 실패: %v", err)
				}
				credPath := filepath.Join(claudeDir, "credentials.json")
				if err := os.WriteFile(credPath, []byte(`{"token":"test"}`), 0644); err != nil {
					t.Fatalf("credentials.json 생성 실패: %v", err)
				}
			},
			envVars:      nil,
			wantStatus:   AuthStatusAuthenticated,
			wantCLI:      true,
			wantAPI:      false,
			wantEnvName:  "",
			wantNonEmpty: true,
		},
		{
			// UT-001 변형: auth.json이 존재하는 경우도 authenticated
			name: "CLI 인증 완료 상태 (auth.json 존재)",
			setup: func(t *testing.T, homeDir string) {
				claudeDir := filepath.Join(homeDir, ".claude")
				if err := os.MkdirAll(claudeDir, 0755); err != nil {
					t.Fatalf(".claude 디렉토리 생성 실패: %v", err)
				}
				authPath := filepath.Join(claudeDir, "auth.json")
				if err := os.WriteFile(authPath, []byte(`{"session":"test"}`), 0644); err != nil {
					t.Fatalf("auth.json 생성 실패: %v", err)
				}
			},
			envVars:      nil,
			wantStatus:   AuthStatusAuthenticated,
			wantCLI:      true,
			wantAPI:      false,
			wantEnvName:  "",
			wantNonEmpty: true,
		},
		{
			// UT-001 변형: 홈 디렉토리의 .claude.json이 존재하는 경우도 authenticated
			name: "CLI 인증 완료 상태 (.claude.json 존재)",
			setup: func(t *testing.T, homeDir string) {
				dotClaudePath := filepath.Join(homeDir, ".claude.json")
				if err := os.WriteFile(dotClaudePath, []byte(`{"auth":true}`), 0644); err != nil {
					t.Fatalf(".claude.json 생성 실패: %v", err)
				}
			},
			envVars:      nil,
			wantStatus:   AuthStatusAuthenticated,
			wantCLI:      true,
			wantAPI:      false,
			wantEnvName:  "",
			wantNonEmpty: true,
		},
		{
			// UT-002: 인증 디렉토리가 존재하지 않으면 not_authenticated
			name: "인증 없음 (디렉토리 미존재, API 키 없음)",
			setup: func(t *testing.T, homeDir string) {
				// 아무것도 설정하지 않음
			},
			envVars:      nil,
			wantStatus:   AuthStatusNotAuthenticated,
			wantCLI:      false,
			wantAPI:      false,
			wantEnvName:  "",
			wantNonEmpty: true,
		},
		{
			// UT-003: API 키만 설정된 경우 (CLI 인증 없음) -> api_key_only
			name: "API 키만 설정됨 (ANTHROPIC_API_KEY)",
			setup: func(t *testing.T, homeDir string) {
				// CLI 인증 디렉토리/파일 없음
			},
			envVars: map[string]string{
				"ANTHROPIC_API_KEY": "sk-ant-test-key",
			},
			wantStatus:   AuthStatusAPIKeyOnly,
			wantCLI:      false,
			wantAPI:      true,
			wantEnvName:  "ANTHROPIC_API_KEY",
			wantNonEmpty: true,
		},
		{
			// UT-003 변형: CLAUDE_API_KEY만 설정된 경우
			name: "API 키만 설정됨 (CLAUDE_API_KEY)",
			setup: func(t *testing.T, homeDir string) {
				// CLI 인증 디렉토리/파일 없음
			},
			envVars: map[string]string{
				"CLAUDE_API_KEY": "sk-claude-test-key",
			},
			wantStatus:   AuthStatusAPIKeyOnly,
			wantCLI:      false,
			wantAPI:      true,
			wantEnvName:  "CLAUDE_API_KEY",
			wantNonEmpty: true,
		},
		{
			// CLI 인증 + API 키 모두 있는 경우 -> authenticated (CLI가 우선)
			name: "CLI 인증 + API 키 모두 존재",
			setup: func(t *testing.T, homeDir string) {
				claudeDir := filepath.Join(homeDir, ".claude")
				if err := os.MkdirAll(claudeDir, 0755); err != nil {
					t.Fatalf(".claude 디렉토리 생성 실패: %v", err)
				}
				credPath := filepath.Join(claudeDir, "credentials.json")
				if err := os.WriteFile(credPath, []byte(`{"token":"test"}`), 0644); err != nil {
					t.Fatalf("credentials.json 생성 실패: %v", err)
				}
			},
			envVars: map[string]string{
				"ANTHROPIC_API_KEY": "sk-ant-test-key",
			},
			wantStatus:   AuthStatusAuthenticated,
			wantCLI:      true,
			wantAPI:      true,
			wantEnvName:  "ANTHROPIC_API_KEY",
			wantNonEmpty: true,
		},
		{
			// .claude 디렉토리만 있고 인증 파일이 없는 경우 -> not_authenticated
			name: ".claude 디렉토리만 존재 (인증 파일 없음)",
			setup: func(t *testing.T, homeDir string) {
				claudeDir := filepath.Join(homeDir, ".claude")
				if err := os.MkdirAll(claudeDir, 0755); err != nil {
					t.Fatalf(".claude 디렉토리 생성 실패: %v", err)
				}
			},
			envVars:      nil,
			wantStatus:   AuthStatusNotAuthenticated,
			wantCLI:      false,
			wantAPI:      false,
			wantEnvName:  "",
			wantNonEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			homeDir := t.TempDir()

			if tt.setup != nil {
				tt.setup(t, homeDir)
			}

			// 환경변수를 클린하게 설정 (다른 테스트의 잔여 값 방지)
			t.Setenv("ANTHROPIC_API_KEY", "")
			t.Setenv("CLAUDE_API_KEY", "")
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			result := checkClaudeAuthWithHome(homeDir)

			if result.ProviderName != "Claude" {
				t.Errorf("ProviderName = %q, want %q", result.ProviderName, "Claude")
			}
			if result.Status != tt.wantStatus {
				t.Errorf("Status = %q, want %q", result.Status, tt.wantStatus)
			}
			if result.CLIAuthenticated != tt.wantCLI {
				t.Errorf("CLIAuthenticated = %v, want %v", result.CLIAuthenticated, tt.wantCLI)
			}
			if result.HasAPIKey != tt.wantAPI {
				t.Errorf("HasAPIKey = %v, want %v", result.HasAPIKey, tt.wantAPI)
			}
			if result.APIKeyEnvName != tt.wantEnvName {
				t.Errorf("APIKeyEnvName = %q, want %q", result.APIKeyEnvName, tt.wantEnvName)
			}
			if tt.wantNonEmpty && result.Message == "" {
				t.Error("Message가 비어있으면 안 됩니다")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// UT-004: CheckCodexAuth - 인증 파일이 존재하면 authenticated
// UT-005: CheckCodexAuth - 인증 파일/API 키 없으면 not_authenticated
// ---------------------------------------------------------------------------

func TestCheckCodexAuth(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T, homeDir string)
		envVars     map[string]string
		wantStatus  AuthStatus
		wantCLI     bool
		wantAPI     bool
		wantEnvName string
	}{
		{
			// UT-004: 인증 디렉토리/파일이 존재하면 authenticated
			name: "CLI 인증 완료 상태 (.codex 디렉토리 내 인증 파일 존재)",
			setup: func(t *testing.T, homeDir string) {
				codexDir := filepath.Join(homeDir, ".codex")
				if err := os.MkdirAll(codexDir, 0755); err != nil {
					t.Fatalf(".codex 디렉토리 생성 실패: %v", err)
				}
				authPath := filepath.Join(codexDir, "auth.json")
				if err := os.WriteFile(authPath, []byte(`{"token":"test"}`), 0644); err != nil {
					t.Fatalf("auth.json 생성 실패: %v", err)
				}
			},
			envVars:     nil,
			wantStatus:  AuthStatusAuthenticated,
			wantCLI:     true,
			wantAPI:     false,
			wantEnvName: "",
		},
		{
			// UT-005: 인증 파일도 없고 API 키도 없으면 not_authenticated
			name: "인증 없음 (디렉토리 미존재, API 키 없음)",
			setup: func(t *testing.T, homeDir string) {
				// 아무것도 설정하지 않음
			},
			envVars:     nil,
			wantStatus:  AuthStatusNotAuthenticated,
			wantCLI:     false,
			wantAPI:     false,
			wantEnvName: "",
		},
		{
			// API 키만 설정된 경우 -> api_key_only
			name: "API 키만 설정됨 (OPENAI_API_KEY)",
			setup: func(t *testing.T, homeDir string) {
				// CLI 인증 없음
			},
			envVars: map[string]string{
				"OPENAI_API_KEY": "sk-openai-test-key",
			},
			wantStatus:  AuthStatusAPIKeyOnly,
			wantCLI:     false,
			wantAPI:     true,
			wantEnvName: "OPENAI_API_KEY",
		},
		{
			// CLI 인증 + API 키 모두 있는 경우 -> authenticated
			name: "CLI 인증 + API 키 모두 존재",
			setup: func(t *testing.T, homeDir string) {
				codexDir := filepath.Join(homeDir, ".codex")
				if err := os.MkdirAll(codexDir, 0755); err != nil {
					t.Fatalf(".codex 디렉토리 생성 실패: %v", err)
				}
				authPath := filepath.Join(codexDir, "auth.json")
				if err := os.WriteFile(authPath, []byte(`{"token":"test"}`), 0644); err != nil {
					t.Fatalf("auth.json 생성 실패: %v", err)
				}
			},
			envVars: map[string]string{
				"OPENAI_API_KEY": "sk-openai-test-key",
			},
			wantStatus:  AuthStatusAuthenticated,
			wantCLI:     true,
			wantAPI:     true,
			wantEnvName: "OPENAI_API_KEY",
		},
		{
			// .codex 디렉토리만 있고 인증 파일이 없는 경우 -> not_authenticated
			name: ".codex 디렉토리만 존재 (인증 파일 없음)",
			setup: func(t *testing.T, homeDir string) {
				codexDir := filepath.Join(homeDir, ".codex")
				if err := os.MkdirAll(codexDir, 0755); err != nil {
					t.Fatalf(".codex 디렉토리 생성 실패: %v", err)
				}
			},
			envVars:     nil,
			wantStatus:  AuthStatusNotAuthenticated,
			wantCLI:     false,
			wantAPI:     false,
			wantEnvName: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			homeDir := t.TempDir()

			if tt.setup != nil {
				tt.setup(t, homeDir)
			}

			// 환경변수 클린 설정
			t.Setenv("OPENAI_API_KEY", "")
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			result := checkCodexAuthWithHome(homeDir)

			if result.ProviderName != "Codex" {
				t.Errorf("ProviderName = %q, want %q", result.ProviderName, "Codex")
			}
			if result.Status != tt.wantStatus {
				t.Errorf("Status = %q, want %q", result.Status, tt.wantStatus)
			}
			if result.CLIAuthenticated != tt.wantCLI {
				t.Errorf("CLIAuthenticated = %v, want %v", result.CLIAuthenticated, tt.wantCLI)
			}
			if result.HasAPIKey != tt.wantAPI {
				t.Errorf("HasAPIKey = %v, want %v", result.HasAPIKey, tt.wantAPI)
			}
			if result.APIKeyEnvName != tt.wantEnvName {
				t.Errorf("APIKeyEnvName = %q, want %q", result.APIKeyEnvName, tt.wantEnvName)
			}
			if result.Message == "" {
				t.Error("Message가 비어있으면 안 됩니다")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// UT-006: CheckGeminiAuth - gcloud credentials가 존재하면 authenticated
// UT-007: CheckGeminiAuth - API 키만 설정된 경우 api_key_only
// ---------------------------------------------------------------------------

func TestCheckGeminiAuth(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T, homeDir string)
		envVars     map[string]string
		wantStatus  AuthStatus
		wantCLI     bool
		wantAPI     bool
		wantEnvName string
	}{
		{
			// UT-006: gcloud application_default_credentials.json이 존재하면 authenticated
			name: "gcloud 인증 완료 상태",
			setup: func(t *testing.T, homeDir string) {
				gcloudDir := filepath.Join(homeDir, ".config", "gcloud")
				if err := os.MkdirAll(gcloudDir, 0755); err != nil {
					t.Fatalf("gcloud 디렉토리 생성 실패: %v", err)
				}
				credPath := filepath.Join(gcloudDir, "application_default_credentials.json")
				if err := os.WriteFile(credPath, []byte(`{"type":"authorized_user"}`), 0644); err != nil {
					t.Fatalf("credentials 파일 생성 실패: %v", err)
				}
			},
			envVars:     nil,
			wantStatus:  AuthStatusAuthenticated,
			wantCLI:     true,
			wantAPI:     false,
			wantEnvName: "",
		},
		{
			// .gemini 디렉토리 내 인증 파일이 존재하면 authenticated
			name: "Gemini CLI 인증 완료 상태 (.gemini 디렉토리)",
			setup: func(t *testing.T, homeDir string) {
				geminiDir := filepath.Join(homeDir, ".gemini")
				if err := os.MkdirAll(geminiDir, 0755); err != nil {
					t.Fatalf(".gemini 디렉토리 생성 실패: %v", err)
				}
				authPath := filepath.Join(geminiDir, "auth.json")
				if err := os.WriteFile(authPath, []byte(`{"token":"test"}`), 0644); err != nil {
					t.Fatalf("auth.json 생성 실패: %v", err)
				}
			},
			envVars:     nil,
			wantStatus:  AuthStatusAuthenticated,
			wantCLI:     true,
			wantAPI:     false,
			wantEnvName: "",
		},
		{
			// UT-007: API 키만 설정된 경우 -> api_key_only
			name: "API 키만 설정됨 (GEMINI_API_KEY)",
			setup: func(t *testing.T, homeDir string) {
				// CLI 인증 없음
			},
			envVars: map[string]string{
				"GEMINI_API_KEY": "test-gemini-key",
			},
			wantStatus:  AuthStatusAPIKeyOnly,
			wantCLI:     false,
			wantAPI:     true,
			wantEnvName: "GEMINI_API_KEY",
		},
		{
			// UT-007 변형: GOOGLE_API_KEY만 설정된 경우
			name: "API 키만 설정됨 (GOOGLE_API_KEY)",
			setup: func(t *testing.T, homeDir string) {
				// CLI 인증 없음
			},
			envVars: map[string]string{
				"GOOGLE_API_KEY": "test-google-key",
			},
			wantStatus:  AuthStatusAPIKeyOnly,
			wantCLI:     false,
			wantAPI:     true,
			wantEnvName: "GOOGLE_API_KEY",
		},
		{
			// 인증 없음
			name: "인증 없음 (디렉토리 미존재, API 키 없음)",
			setup: func(t *testing.T, homeDir string) {
				// 아무것도 설정하지 않음
			},
			envVars:     nil,
			wantStatus:  AuthStatusNotAuthenticated,
			wantCLI:     false,
			wantAPI:     false,
			wantEnvName: "",
		},
		{
			// gcloud 인증 + API 키 모두 있는 경우 -> authenticated
			name: "gcloud 인증 + API 키 모두 존재",
			setup: func(t *testing.T, homeDir string) {
				gcloudDir := filepath.Join(homeDir, ".config", "gcloud")
				if err := os.MkdirAll(gcloudDir, 0755); err != nil {
					t.Fatalf("gcloud 디렉토리 생성 실패: %v", err)
				}
				credPath := filepath.Join(gcloudDir, "application_default_credentials.json")
				if err := os.WriteFile(credPath, []byte(`{"type":"authorized_user"}`), 0644); err != nil {
					t.Fatalf("credentials 파일 생성 실패: %v", err)
				}
			},
			envVars: map[string]string{
				"GEMINI_API_KEY": "test-gemini-key",
			},
			wantStatus:  AuthStatusAuthenticated,
			wantCLI:     true,
			wantAPI:     true,
			wantEnvName: "GEMINI_API_KEY",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			homeDir := t.TempDir()

			if tt.setup != nil {
				tt.setup(t, homeDir)
			}

			// 환경변수 클린 설정
			t.Setenv("GEMINI_API_KEY", "")
			t.Setenv("GOOGLE_API_KEY", "")
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			result := checkGeminiAuthWithHome(homeDir)

			if result.ProviderName != "Gemini" {
				t.Errorf("ProviderName = %q, want %q", result.ProviderName, "Gemini")
			}
			if result.Status != tt.wantStatus {
				t.Errorf("Status = %q, want %q", result.Status, tt.wantStatus)
			}
			if result.CLIAuthenticated != tt.wantCLI {
				t.Errorf("CLIAuthenticated = %v, want %v", result.CLIAuthenticated, tt.wantCLI)
			}
			if result.HasAPIKey != tt.wantAPI {
				t.Errorf("HasAPIKey = %v, want %v", result.HasAPIKey, tt.wantAPI)
			}
			if result.APIKeyEnvName != tt.wantEnvName {
				t.Errorf("APIKeyEnvName = %q, want %q", result.APIKeyEnvName, tt.wantEnvName)
			}
			if result.Message == "" {
				t.Error("Message가 비어있으면 안 됩니다")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// CheckAllAuth 테스트
// ---------------------------------------------------------------------------

func TestCheckAllAuth(t *testing.T) {
	tests := []struct {
		name           string
		providerNames  []string
		wantCount      int
		wantProviders  []string
	}{
		{
			name:          "모든 프로바이더 확인",
			providerNames: []string{"Claude", "Codex", "Gemini"},
			wantCount:     3,
			wantProviders: []string{"Claude", "Codex", "Gemini"},
		},
		{
			name:          "단일 프로바이더 확인",
			providerNames: []string{"Claude"},
			wantCount:     1,
			wantProviders: []string{"Claude"},
		},
		{
			name:          "빈 목록",
			providerNames: []string{},
			wantCount:     0,
			wantProviders: []string{},
		},
		{
			name:          "알 수 없는 프로바이더 포함",
			providerNames: []string{"Claude", "Unknown"},
			wantCount:     2,
			wantProviders: []string{"Claude", "Unknown"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := CheckAllAuth(tt.providerNames)

			if len(results) != tt.wantCount {
				t.Errorf("결과 수 = %d, want %d", len(results), tt.wantCount)
			}

			for i, wantProvider := range tt.wantProviders {
				if i >= len(results) {
					break
				}
				if results[i].ProviderName != wantProvider {
					t.Errorf("results[%d].ProviderName = %q, want %q", i, results[i].ProviderName, wantProvider)
				}
			}

			// 알 수 없는 프로바이더는 unknown 상태여야 함
			for _, r := range results {
				if r.ProviderName == "Unknown" && r.Status != AuthStatusUnknown {
					t.Errorf("알 수 없는 프로바이더의 Status = %q, want %q", r.Status, AuthStatusUnknown)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// CheckAllAuth 순서 보장 테스트
// ---------------------------------------------------------------------------

func TestCheckAllAuth_순서보장(t *testing.T) {
	providers := []string{"Gemini", "Claude", "Codex"}
	results := CheckAllAuth(providers)

	if len(results) != 3 {
		t.Fatalf("결과 수 = %d, want 3", len(results))
	}

	for i, wantProvider := range providers {
		if results[i].ProviderName != wantProvider {
			t.Errorf("results[%d].ProviderName = %q, want %q (순서 보장 실패)", i, results[i].ProviderName, wantProvider)
		}
	}
}

// ---------------------------------------------------------------------------
// AuthStatus 상수값 검증
// ---------------------------------------------------------------------------

func TestAuthStatus_상수값(t *testing.T) {
	tests := []struct {
		name     string
		status   AuthStatus
		wantStr  string
	}{
		{"authenticated", AuthStatusAuthenticated, "authenticated"},
		{"not_authenticated", AuthStatusNotAuthenticated, "not_authenticated"},
		{"api_key_only", AuthStatusAPIKeyOnly, "api_key_only"},
		{"unknown", AuthStatusUnknown, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.status) != tt.wantStr {
				t.Errorf("AuthStatus = %q, want %q", string(tt.status), tt.wantStr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 보안 테스트: API 키 값이 결과에 노출되지 않는지 확인
// ---------------------------------------------------------------------------

func TestCheckAuth_API키값미노출(t *testing.T) {
	homeDir := t.TempDir()

	apiKey := "sk-ant-super-secret-key-12345"
	t.Setenv("ANTHROPIC_API_KEY", apiKey)
	t.Setenv("CLAUDE_API_KEY", "")

	result := checkClaudeAuthWithHome(homeDir)

	// Message에 실제 API 키 값이 포함되면 안 됨
	if result.Message != "" && strings.Contains(result.Message, apiKey) {
		t.Errorf("Message에 실제 API 키 값이 노출되었습니다: %q", result.Message)
	}

	// APIKeyEnvName에는 환경변수 이름만 있어야 함 (값이 아닌)
	if result.APIKeyEnvName == apiKey {
		t.Error("APIKeyEnvName에 API 키 값이 저장되었습니다 (환경변수 이름이어야 합니다)")
	}
}

