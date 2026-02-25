package cmd

import "testing"

// TestDetermineBestMode는 프로바이더 인증 상태에 따른 모드 결정 로직을 검증합니다.
func TestDetermineBestMode(t *testing.T) {
	tests := []struct {
		name     string
		provider providerInfo
		want     string
	}{
		// UT-008: CLI 인증 + API 키 -> hybrid
		{
			name: "CLI인증+API키 -> hybrid",
			provider: providerInfo{
				Name:             "Claude",
				HasCLI:           true,
				CLIAuthenticated: true,
				HasAPIKey:        true,
			},
			want: "hybrid",
		},
		// UT-009: CLI 인증만 (API 키 없음) -> cli
		{
			name: "CLI인증만 -> cli",
			provider: providerInfo{
				Name:             "Claude",
				HasCLI:           true,
				CLIAuthenticated: true,
				HasAPIKey:        false,
			},
			want: "cli",
		},
		// UT-010: API 키만 (CLI 없음) -> api
		{
			name: "API키만 -> api",
			provider: providerInfo{
				Name:      "Claude",
				HasCLI:    false,
				HasAPIKey: true,
			},
			want: "api",
		},
		// UT-011: Codex + ChatGPT 인증 -> app-server
		{
			name: "Codex+ChatGPT인증 -> app-server",
			provider: providerInfo{
				Name:        "Codex",
				ChatGPTAuth: true,
			},
			want: "app-server",
		},
		// UT-012: CLI 설치 + 미인증 + API 키 -> api
		{
			name: "CLI설치+미인증+API키 -> api",
			provider: providerInfo{
				Name:             "Claude",
				HasCLI:           true,
				CLIAuthenticated: false,
				HasAPIKey:        true,
			},
			want: "api",
		},
		// 추가: CLI 설치 + 미인증 + API 키 없음 -> api (기본값)
		{
			name: "CLI설치+미인증+API키없음 -> api",
			provider: providerInfo{
				Name:             "Gemini",
				HasCLI:           true,
				CLIAuthenticated: false,
				HasAPIKey:        false,
			},
			want: "api",
		},
		// 추가: 아무것도 없음 -> api (기본값)
		{
			name: "아무것도없음 -> api",
			provider: providerInfo{
				Name: "Claude",
			},
			want: "api",
		},
		// 추가: Codex + ChatGPT 인증 + CLI 인증 + API 키 -> app-server (ChatGPT 우선)
		{
			name: "Codex+ChatGPT+CLI인증+API키 -> app-server우선",
			provider: providerInfo{
				Name:             "Codex",
				HasCLI:           true,
				CLIAuthenticated: true,
				HasAPIKey:        true,
				ChatGPTAuth:      true,
			},
			want: "app-server",
		},
		// 추가: Codex + ChatGPT 미인증 + CLI 인증 + API 키 -> hybrid
		{
			name: "Codex+ChatGPT미인증+CLI인증+API키 -> hybrid",
			provider: providerInfo{
				Name:             "Codex",
				HasCLI:           true,
				CLIAuthenticated: true,
				HasAPIKey:        true,
				ChatGPTAuth:      false,
			},
			want: "hybrid",
		},
		// 추가: Claude에 ChatGPTAuth는 무시됨 (Codex 전용)
		{
			name: "Claude+ChatGPTAuth무시 -> api",
			provider: providerInfo{
				Name:        "Claude",
				ChatGPTAuth: true,
			},
			want: "api",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := determineBestMode(tt.provider)
			if got != tt.want {
				t.Errorf("determineBestMode(%+v) = %q, want %q", tt.provider, got, tt.want)
			}
		})
	}
}
