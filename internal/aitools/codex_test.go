package aitools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDetectCodexCLI는 Codex CLI 감지 함수가 정상 동작하는지 검증합니다.
func TestDetectCodexCLI(t *testing.T) {
	info, err := DetectCodexCLI()
	if err != nil {
		t.Fatalf("DetectCodexCLI() 예상치 못한 에러: %v", err)
	}

	if info == nil {
		t.Fatal("DetectCodexCLI() 결과가 nil이면 안 됩니다")
	}

	if info.Name != "Codex CLI" {
		t.Errorf("Name = %q, want %q", info.Name, "Codex CLI")
	}
}

// TestDetectCodexCLI_API키감지는 OPENAI_API_KEY 환경변수 감지를 검증합니다.
func TestDetectCodexCLI_API키감지(t *testing.T) {
	tests := []struct {
		name       string
		envKey     string
		envValue   string
		wantAPIKey bool
	}{
		{
			name:       "OPENAI_API_KEY가 설정된 경우",
			envKey:     "OPENAI_API_KEY",
			envValue:   "sk-test-key-123",
			wantAPIKey: true,
		},
		{
			name:       "OPENAI_API_KEY가 비어있는 경우",
			envKey:     "OPENAI_API_KEY",
			envValue:   "",
			wantAPIKey: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(tt.envKey, tt.envValue)

			info, err := DetectCodexCLI()
			if err != nil {
				t.Fatalf("DetectCodexCLI() error = %v", err)
			}

			// CLI가 설치되어 있지 않으면 HasAPIKey 검증을 건너뜁니다.
			// (설치되어 있지 않으면 감지 로직이 API 키까지 진행하지 않을 수 있음)
			if info.Installed && info.HasAPIKey != tt.wantAPIKey {
				t.Errorf("HasAPIKey = %v, want %v", info.HasAPIKey, tt.wantAPIKey)
			}
		})
	}
}

// TestConfigureCodexMCP는 Codex CLI MCP 설정 기능을 테스트합니다.
func TestConfigureCodexMCP(t *testing.T) {
	tests := []struct {
		name            string
		setupExisting   bool
		existingContent string
		description     string
	}{
		{
			name:          "설정 파일이 없는 경우 새로 생성",
			setupExisting: false,
			description:   "config.toml이 없으면 새로 생성해야 합니다",
		},
		{
			name:          "기존 설정에 autopus 섹션 추가",
			setupExisting: true,
			existingContent: `# 기존 설정
[settings]
model = "o3"
`,
			description: "기존 설정을 유지하면서 MCP 섹션을 추가해야 합니다",
		},
		{
			name:          "이미 autopus 섹션이 있는 경우",
			setupExisting: true,
			existingContent: `# 기존 설정
[mcp_servers.autopus]
command = "autopus-bridge"
args = ["mcp-serve"]
`,
			description: "이미 설정되어 있으면 중복 추가하지 않아야 합니다",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			t.Setenv("HOME", tmpDir)

			configPath := filepath.Join(tmpDir, ".codex", "config.toml")

			if tt.setupExisting {
				if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
					t.Fatalf("디렉토리 생성 실패: %v", err)
				}
				if err := os.WriteFile(configPath, []byte(tt.existingContent), 0644); err != nil {
					t.Fatalf("기존 설정 파일 생성 실패: %v", err)
				}
			}

			err := ConfigureCodexMCP()
			if err != nil {
				t.Fatalf("ConfigureCodexMCP() error = %v", err)
			}

			// 파일이 생성되었는지 확인
			content, readErr := os.ReadFile(configPath)
			if readErr != nil {
				t.Fatalf("설정 파일 읽기 실패: %v", readErr)
			}

			contentStr := string(content)

			// [mcp_servers.autopus] 섹션이 존재하는지 확인
			if !strings.Contains(contentStr, "[mcp_servers.autopus]") {
				t.Error("설정 파일에 [mcp_servers.autopus] 섹션이 없습니다")
			}

			// autopus-bridge 명령어가 포함되어 있는지 확인
			if !strings.Contains(contentStr, `"autopus-bridge"`) {
				t.Error("설정 파일에 autopus-bridge 명령어가 없습니다")
			}

			// 기존 설정이 있었다면 유지되는지 확인
			if tt.setupExisting && !strings.Contains(tt.existingContent, "[mcp_servers.autopus]") {
				// 기존 내용 중 autopus가 아닌 부분이 유지되는지 확인
				for _, line := range strings.Split(tt.existingContent, "\n") {
					trimmed := strings.TrimSpace(line)
					if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
						if !strings.Contains(contentStr, trimmed) {
							t.Errorf("기존 설정이 유실되었습니다: %q", trimmed)
						}
					}
				}
			}
		})
	}
}

// TestConfigureCodexMCP_멱등성은 두 번 호출해도 섹션이 중복되지 않는지 검증합니다.
func TestConfigureCodexMCP_멱등성(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// 첫 번째 호출
	err := ConfigureCodexMCP()
	if err != nil {
		t.Fatalf("첫 번째 ConfigureCodexMCP() error = %v", err)
	}

	// 두 번째 호출
	err = ConfigureCodexMCP()
	if err != nil {
		t.Fatalf("두 번째 ConfigureCodexMCP() error = %v", err)
	}

	configPath := filepath.Join(tmpDir, ".codex", "config.toml")
	content, readErr := os.ReadFile(configPath)
	if readErr != nil {
		t.Fatalf("설정 파일 읽기 실패: %v", readErr)
	}

	// [mcp_servers.autopus]가 정확히 한 번만 등장하는지 확인
	count := strings.Count(string(content), "[mcp_servers.autopus]")
	if count != 1 {
		t.Errorf("[mcp_servers.autopus] 등장 횟수 = %d, want 1", count)
	}
}

// TestConfigureCodexMCP_백업생성은 기존 파일이 있을 때 백업이 생성되는지 검증합니다.
func TestConfigureCodexMCP_백업생성(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	configPath := filepath.Join(tmpDir, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		t.Fatalf("디렉토리 생성 실패: %v", err)
	}

	originalContent := `# Original config
[settings]
model = "o3"
`
	if err := os.WriteFile(configPath, []byte(originalContent), 0644); err != nil {
		t.Fatalf("기존 파일 생성 실패: %v", err)
	}

	err := ConfigureCodexMCP()
	if err != nil {
		t.Fatalf("ConfigureCodexMCP() error = %v", err)
	}

	// .bak 파일이 생성되었는지 확인
	backupPath := configPath + ".bak"
	backupContent, readErr := os.ReadFile(backupPath)
	if readErr != nil {
		t.Fatalf("백업 파일 읽기 실패: %v", readErr)
	}

	if string(backupContent) != originalContent {
		t.Errorf("백업 파일 내용이 원본과 다릅니다: got %q, want %q", string(backupContent), originalContent)
	}
}

// TestConfigureCodexMCP_디렉토리자동생성은 .codex 디렉토리가 없을 때 자동 생성되는지 검증합니다.
func TestConfigureCodexMCP_디렉토리자동생성(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// .codex 디렉토리를 미리 만들지 않음
	err := ConfigureCodexMCP()
	if err != nil {
		t.Fatalf("ConfigureCodexMCP() error = %v", err)
	}

	configPath := filepath.Join(tmpDir, ".codex", "config.toml")
	if _, statErr := os.Stat(configPath); os.IsNotExist(statErr) {
		t.Error("config.toml이 생성되지 않았습니다")
	}
}

// TestConfigureCodexMCP_읽기실패는 기존 파일이 읽기 불가할 때 에러를 반환하는지 검증합니다.
func TestConfigureCodexMCP_읽기실패(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	configPath := filepath.Join(tmpDir, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		t.Fatalf("디렉토리 생성 실패: %v", err)
	}

	// 파일 생성 후 읽기 권한 제거
	if err := os.WriteFile(configPath, []byte("[settings]\nmodel = \"o3\"\n"), 0644); err != nil {
		t.Fatalf("파일 생성 실패: %v", err)
	}
	if err := os.Chmod(configPath, 0000); err != nil {
		t.Fatalf("권한 변경 실패: %v", err)
	}
	t.Cleanup(func() {
		os.Chmod(configPath, 0644)
	})

	err := ConfigureCodexMCP()
	if err == nil {
		t.Error("읽기 불가 파일에서 ConfigureCodexMCP()가 에러를 반환하지 않았습니다")
	}
}

// TestConfigureCodexMCP_쓰기실패는 기존 설정에 추가 시 쓰기가 실패하는 경우를 검증합니다.
func TestConfigureCodexMCP_쓰기실패(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	configPath := filepath.Join(tmpDir, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		t.Fatalf("디렉토리 생성 실패: %v", err)
	}

	// 기존 내용 작성
	existingContent := "[settings]\nmodel = \"o3\"\n"
	if err := os.WriteFile(configPath, []byte(existingContent), 0644); err != nil {
		t.Fatalf("파일 생성 실패: %v", err)
	}

	// 파일을 읽기 전용으로 변경 (읽기는 가능하지만 쓰기 불가)
	if err := os.Chmod(configPath, 0444); err != nil {
		t.Fatalf("권한 변경 실패: %v", err)
	}
	t.Cleanup(func() {
		os.Chmod(configPath, 0644)
	})

	err := ConfigureCodexMCP()
	if err == nil {
		t.Error("쓰기 불가 파일에서 ConfigureCodexMCP()가 에러를 반환하지 않았습니다")
	}
}

// TestCodexMCPSection은 codexMCPSection 상수의 내용을 검증합니다.
func TestCodexMCPSection(t *testing.T) {
	if !strings.Contains(codexMCPSection, "[mcp_servers.autopus]") {
		t.Error("codexMCPSection에 [mcp_servers.autopus] 섹션이 없습니다")
	}

	if !strings.Contains(codexMCPSection, `"autopus-bridge"`) {
		t.Error("codexMCPSection에 autopus-bridge 명령어가 없습니다")
	}

	if !strings.Contains(codexMCPSection, `"mcp-serve"`) {
		t.Error("codexMCPSection에 mcp-serve 인자가 없습니다")
	}
}
