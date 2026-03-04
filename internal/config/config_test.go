package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestProviderConfig_GetAPIKey는 환경변수에서 API 키를 가져오는 기능을 테스트합니다.
func TestProviderConfig_GetAPIKey(t *testing.T) {
	// 테스트용 환경변수 설정
	testKey := "test-api-key-12345"
	t.Setenv("TEST_API_KEY", testKey)

	tests := []struct {
		name      string
		apiKeyEnv string
		expected  string
	}{
		{
			name:      "환경변수가 설정된 경우",
			apiKeyEnv: "TEST_API_KEY",
			expected:  testKey,
		},
		{
			name:      "환경변수가 없는 경우",
			apiKeyEnv: "NONEXISTENT_KEY",
			expected:  "",
		},
		{
			name:      "환경변수 이름이 빈 문자열인 경우",
			apiKeyEnv: "",
			expected:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &ProviderConfig{APIKeyEnv: tt.apiKeyEnv}
			result := p.GetAPIKey()
			if result != tt.expected {
				t.Errorf("GetAPIKey() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestProviderConfig_HasAPIKey는 API 키 존재 여부 확인을 테스트합니다.
func TestProviderConfig_HasAPIKey(t *testing.T) {
	t.Setenv("TEST_API_KEY", "some-key")

	tests := []struct {
		name      string
		apiKeyEnv string
		expected  bool
	}{
		{
			name:      "API 키가 있는 경우",
			apiKeyEnv: "TEST_API_KEY",
			expected:  true,
		},
		{
			name:      "API 키가 없는 경우",
			apiKeyEnv: "NONEXISTENT_KEY",
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &ProviderConfig{APIKeyEnv: tt.apiKeyEnv}
			result := p.HasAPIKey()
			if result != tt.expected {
				t.Errorf("HasAPIKey() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestConfig_Validate는 설정 검증을 테스트합니다.
func TestConfig_Validate(t *testing.T) {
	// Claude API 키 설정
	t.Setenv("CLAUDE_API_KEY", "test-key")

	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "유효한 설정",
			config: &Config{
				Providers: ProvidersConfig{
					Claude: ProviderConfig{APIKeyEnv: "CLAUDE_API_KEY"},
				},
				Logging: LoggingConfig{
					Level:  "info",
					Format: "json",
				},
				Reconnection: ReconnectionConfig{
					MaxAttempts: 10,
				},
			},
			wantErr: false,
		},
		{
			name: "유효하지 않은 로그 레벨",
			config: &Config{
				Providers: ProvidersConfig{
					Claude: ProviderConfig{APIKeyEnv: "CLAUDE_API_KEY"},
				},
				Logging: LoggingConfig{
					Level:  "invalid",
					Format: "json",
				},
				Reconnection: ReconnectionConfig{
					MaxAttempts: 10,
				},
			},
			wantErr: true,
		},
		{
			name: "유효하지 않은 로그 포맷",
			config: &Config{
				Providers: ProvidersConfig{
					Claude: ProviderConfig{APIKeyEnv: "CLAUDE_API_KEY"},
				},
				Logging: LoggingConfig{
					Level:  "info",
					Format: "invalid",
				},
				Reconnection: ReconnectionConfig{
					MaxAttempts: 10,
				},
			},
			wantErr: true,
		},
		{
			name: "MaxAttempts가 10 초과 (허용됨)",
			config: &Config{
				Providers: ProvidersConfig{
					Claude: ProviderConfig{APIKeyEnv: "CLAUDE_API_KEY"},
				},
				Logging: LoggingConfig{
					Level:  "info",
					Format: "json",
				},
				Reconnection: ReconnectionConfig{
					MaxAttempts: 15, // 10 초과도 허용
				},
			},
			wantErr: false,
		},
		{
			name: "MaxAttempts가 0 (무제한)",
			config: &Config{
				Providers: ProvidersConfig{
					Claude: ProviderConfig{APIKeyEnv: "CLAUDE_API_KEY"},
				},
				Logging: LoggingConfig{
					Level:  "info",
					Format: "json",
				},
				Reconnection: ReconnectionConfig{
					MaxAttempts: 0, // 무제한
				},
			},
			wantErr: false,
		},
		{
			name: "MaxAttempts가 음수",
			config: &Config{
				Providers: ProvidersConfig{
					Claude: ProviderConfig{APIKeyEnv: "CLAUDE_API_KEY"},
				},
				Logging: LoggingConfig{
					Level:  "info",
					Format: "json",
				},
				Reconnection: ReconnectionConfig{
					MaxAttempts: -1, // 음수는 에러
				},
			},
			wantErr: true,
		},
		{
			name: "AI 프로바이더 미설정",
			config: &Config{
				Providers: ProvidersConfig{
					Claude: ProviderConfig{APIKeyEnv: "NONEXISTENT_KEY"},
					Gemini: ProviderConfig{APIKeyEnv: "NONEXISTENT_KEY2"},
				},
				Logging: LoggingConfig{
					Level:  "info",
					Format: "json",
				},
				Reconnection: ReconnectionConfig{
					MaxAttempts: 10,
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestExpandPath는 경로 확장을 테스트합니다.
func TestExpandPath(t *testing.T) {
	home, _ := os.UserHomeDir()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "틸드로 시작하는 경로",
			input:    "~/config/test.yaml",
			expected: home + "/config/test.yaml",
		},
		{
			name:     "절대 경로",
			input:    "/etc/config.yaml",
			expected: "/etc/config.yaml",
		},
		{
			name:     "상대 경로",
			input:    "config/test.yaml",
			expected: "config/test.yaml",
		},
		{
			name:     "빈 문자열",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandPath(tt.input)
			if result != tt.expected {
				t.Errorf("expandPath() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestConfig_GetAvailableProviders는 사용 가능한 프로바이더 목록을 테스트합니다.
func TestConfig_GetAvailableProviders(t *testing.T) {
	t.Setenv("CLAUDE_API_KEY", "test-claude")
	t.Setenv("GEMINI_API_KEY", "test-gemini")

	tests := []struct {
		name     string
		config   *Config
		expected []string
	}{
		{
			name: "두 프로바이더 모두 설정",
			config: &Config{
				Providers: ProvidersConfig{
					Claude: ProviderConfig{APIKeyEnv: "CLAUDE_API_KEY"},
					Gemini: ProviderConfig{APIKeyEnv: "GEMINI_API_KEY"},
				},
			},
			expected: []string{"anthropic", "google"},
		},
		{
			name: "Claude만 설정",
			config: &Config{
				Providers: ProvidersConfig{
					Claude: ProviderConfig{APIKeyEnv: "CLAUDE_API_KEY"},
					Gemini: ProviderConfig{APIKeyEnv: "NONEXISTENT"},
				},
			},
			expected: []string{"anthropic"},
		},
		{
			name: "프로바이더 없음",
			config: &Config{
				Providers: ProvidersConfig{
					Claude: ProviderConfig{APIKeyEnv: "NONEXISTENT1"},
					Gemini: ProviderConfig{APIKeyEnv: "NONEXISTENT2"},
				},
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.GetAvailableProviders()
			if len(result) != len(tt.expected) {
				t.Errorf("GetAvailableProviders() = %v, want %v", result, tt.expected)
				return
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("GetAvailableProviders()[%d] = %q, want %q", i, v, tt.expected[i])
				}
			}
		})
	}
}

// TestProviderConfig_IsAvailable_AppServerWithCodexSavedAuth는
// codex login(auth.json)만으로 app-server 모드가 사용 가능해지는지 검증합니다.
func TestProviderConfig_IsAvailable_AppServerWithCodexSavedAuth(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	codexDir := filepath.Join(tmpHome, ".codex")
	if err := os.MkdirAll(codexDir, 0o755); err != nil {
		t.Fatalf(".codex 디렉토리 생성 실패: %v", err)
	}

	authPath := filepath.Join(codexDir, "auth.json")
	authJSON := `{"auth_mode":"chatgpt","tokens":{"access_token":"test-token"}}`
	if err := os.WriteFile(authPath, []byte(authJSON), 0o644); err != nil {
		t.Fatalf("auth.json 생성 실패: %v", err)
	}

	p := &ProviderConfig{
		Mode:           "app-server",
		CLIPath:        "sh",
		APIKeyEnv:      "NONEXISTENT_OPENAI_API_KEY",
		ChatGPTAuthEnv: "NONEXISTENT_CHATGPT_AUTH",
	}

	if !p.IsAvailable() {
		t.Fatal("app-server가 codex saved auth로 available=true 이어야 합니다")
	}
}

// TestConfig_Validate_CodexAppServerSavedAuth는 codex-only 환경에서
// auth.json 기반 app-server 설정 검증이 통과하는지 확인합니다.
func TestConfig_Validate_CodexAppServerSavedAuth(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	codexDir := filepath.Join(tmpHome, ".codex")
	if err := os.MkdirAll(codexDir, 0o755); err != nil {
		t.Fatalf(".codex 디렉토리 생성 실패: %v", err)
	}

	authPath := filepath.Join(codexDir, "auth.json")
	authJSON := `{"auth_mode":"chatgpt","tokens":{"access_token":"test-token"}}`
	if err := os.WriteFile(authPath, []byte(authJSON), 0o644); err != nil {
		t.Fatalf("auth.json 생성 실패: %v", err)
	}

	disabled := false
	cfg := &Config{
		Providers: ProvidersConfig{
			Claude: ProviderConfig{
				Enabled: &disabled,
			},
			Gemini: ProviderConfig{
				Enabled: &disabled,
			},
			Codex: ProviderConfig{
				Mode:      "app-server",
				CLIPath:   "sh",
				APIKeyEnv: "NONEXISTENT_OPENAI_API_KEY",
			},
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
		Reconnection: ReconnectionConfig{
			MaxAttempts: 1,
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() 실패: %v", err)
	}

	providers := cfg.GetAvailableProviders()
	if len(providers) != 1 || providers[0] != "openai" {
		t.Fatalf("GetAvailableProviders() = %v, want [openai]", providers)
	}
}
