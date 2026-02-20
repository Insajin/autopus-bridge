package aitools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestDetectClaudeCode는 Claude Code CLI 감지 함수가 정상 동작하는지 검증합니다.
func TestDetectClaudeCode(t *testing.T) {
	info, err := DetectClaudeCode()
	if err != nil {
		t.Fatalf("DetectClaudeCode() 예상치 못한 에러: %v", err)
	}

	if info == nil {
		t.Fatal("DetectClaudeCode() 결과가 nil이면 안 됩니다")
	}

	if info.Name != "Claude Code" {
		t.Errorf("Name = %q, want %q", info.Name, "Claude Code")
	}

	// CLI가 설치되어 있지 않아도 에러 없이 반환되어야 합니다.
	// info.Installed 값은 환경에 따라 다를 수 있으므로 값 자체는 검증하지 않습니다.
}

// TestIsPluginInstalled는 플러그인 설치 여부 확인 기능을 테스트합니다.
func TestIsPluginInstalled(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(t *testing.T, homeDir string)
		expected bool
	}{
		{
			name: "플러그인이 설치되지 않은 경우",
			setup: func(t *testing.T, homeDir string) {
				// 아무것도 설정하지 않음
			},
			expected: false,
		},
		{
			name: "플러그인이 설치된 경우",
			setup: func(t *testing.T, homeDir string) {
				pluginDir := filepath.Join(homeDir, ".claude", "plugins", "autopus", ".claude-plugin")
				if err := os.MkdirAll(pluginDir, 0755); err != nil {
					t.Fatalf("플러그인 디렉토리 생성 실패: %v", err)
				}
				pluginJSON := filepath.Join(pluginDir, "plugin.json")
				content := `{"name": "autopus", "version": "1.0.0"}`
				if err := os.WriteFile(pluginJSON, []byte(content), 0644); err != nil {
					t.Fatalf("plugin.json 생성 실패: %v", err)
				}
			},
			expected: true,
		},
		{
			name: ".claude 디렉토리만 있고 플러그인이 없는 경우",
			setup: func(t *testing.T, homeDir string) {
				claudeDir := filepath.Join(homeDir, ".claude")
				if err := os.MkdirAll(claudeDir, 0755); err != nil {
					t.Fatalf(".claude 디렉토리 생성 실패: %v", err)
				}
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			t.Setenv("HOME", tmpDir)

			tt.setup(t, tmpDir)

			result := IsPluginInstalled()
			if result != tt.expected {
				t.Errorf("IsPluginInstalled() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestConfigureClaudeCodeMCP는 Claude Code MCP 설정 기능을 테스트합니다.
func TestConfigureClaudeCodeMCP(t *testing.T) {
	tests := []struct {
		name          string
		useProjectDir bool
		setupExisting bool
		existingData  map[string]interface{}
		description   string
	}{
		{
			name:          "프로젝트 디렉토리에 새 .mcp.json 생성",
			useProjectDir: true,
			setupExisting: false,
			description:   "프로젝트 디렉토리에 .mcp.json이 없으면 새로 생성해야 합니다",
		},
		{
			name:          "기존 .mcp.json에 autopus 항목 추가",
			useProjectDir: true,
			setupExisting: true,
			existingData: map[string]interface{}{
				"mcpServers": map[string]interface{}{
					"other-server": map[string]interface{}{
						"command": "other-cmd",
						"args":    []string{"arg1"},
					},
				},
			},
			description: "기존 설정의 다른 서버를 유지하면서 autopus를 추가해야 합니다",
		},
		{
			name:          "글로벌 설정에 새 .mcp.json 생성",
			useProjectDir: false,
			setupExisting: false,
			description:   "projectDir이 비어있으면 글로벌 경로에 생성해야 합니다",
		},
		{
			name:          "기존 글로벌 .mcp.json 업데이트",
			useProjectDir: false,
			setupExisting: true,
			existingData: map[string]interface{}{
				"mcpServers": map[string]interface{}{
					"context7": map[string]interface{}{
						"command": "npx",
						"args":    []string{"@context7/mcp"},
					},
				},
			},
			description: "기존 글로벌 설정을 유지하면서 autopus를 추가해야 합니다",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			t.Setenv("HOME", tmpDir)

			var mcpPath string
			var projectDir string

			if tt.useProjectDir {
				projectDir = filepath.Join(tmpDir, "project")
				if err := os.MkdirAll(projectDir, 0755); err != nil {
					t.Fatalf("프로젝트 디렉토리 생성 실패: %v", err)
				}
				mcpPath = filepath.Join(projectDir, ".mcp.json")
			} else {
				projectDir = ""
				claudeDir := filepath.Join(tmpDir, ".claude")
				if err := os.MkdirAll(claudeDir, 0755); err != nil {
					t.Fatalf(".claude 디렉토리 생성 실패: %v", err)
				}
				mcpPath = filepath.Join(claudeDir, ".mcp.json")
			}

			if tt.setupExisting {
				jsonData, _ := json.MarshalIndent(tt.existingData, "", "  ")
				if err := os.WriteFile(mcpPath, jsonData, 0644); err != nil {
					t.Fatalf("기존 설정 파일 생성 실패: %v", err)
				}
			}

			// 실행
			err := ConfigureClaudeCodeMCP(projectDir)
			if err != nil {
				t.Fatalf("ConfigureClaudeCodeMCP() error = %v", err)
			}

			// 결과 파일 확인
			result, readErr := ReadJSONConfig(mcpPath)
			if readErr != nil {
				t.Fatalf("결과 파일 읽기 실패: %v", readErr)
			}

			// autopus 서버가 추가되었는지 확인
			mcpServers, ok := result["mcpServers"].(map[string]interface{})
			if !ok {
				t.Fatal("mcpServers 필드가 없거나 올바른 타입이 아닙니다")
			}

			autopus, ok := mcpServers["autopus"].(map[string]interface{})
			if !ok {
				t.Fatal("autopus 서버가 추가되지 않았습니다")
			}

			if autopus["command"] != "autopus-bridge" {
				t.Errorf("autopus command = %v, want %q", autopus["command"], "autopus-bridge")
			}

			// 기존 서버가 유지되는지 확인
			if tt.setupExisting {
				for serverName := range tt.existingData["mcpServers"].(map[string]interface{}) {
					if _, exists := mcpServers[serverName]; !exists {
						t.Errorf("기존 서버 %q가 삭제되었습니다", serverName)
					}
				}
			}
		})
	}
}

// TestConfigureClaudeCodeMCP_백업생성은 기존 파일이 있을 때 백업이 생성되는지 검증합니다.
func TestConfigureClaudeCodeMCP_백업생성(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("프로젝트 디렉토리 생성 실패: %v", err)
	}

	mcpPath := filepath.Join(projectDir, ".mcp.json")
	originalContent := `{"mcpServers": {}}`
	if err := os.WriteFile(mcpPath, []byte(originalContent), 0644); err != nil {
		t.Fatalf("기존 파일 생성 실패: %v", err)
	}

	err := ConfigureClaudeCodeMCP(projectDir)
	if err != nil {
		t.Fatalf("ConfigureClaudeCodeMCP() error = %v", err)
	}

	// .bak 파일이 생성되었는지 확인
	backupPath := mcpPath + ".bak"
	backupContent, readErr := os.ReadFile(backupPath)
	if readErr != nil {
		t.Fatalf("백업 파일 읽기 실패: %v", readErr)
	}

	if string(backupContent) != originalContent {
		t.Errorf("백업 파일 내용이 원본과 다릅니다: got %q, want %q", string(backupContent), originalContent)
	}
}

// TestConfigureClaudeCodeMCP_잘못된JSON은 기존 파일이 유효하지 않은 JSON일 때의 동작을 검증합니다.
func TestConfigureClaudeCodeMCP_잘못된JSON(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("프로젝트 디렉토리 생성 실패: %v", err)
	}

	mcpPath := filepath.Join(projectDir, ".mcp.json")
	// 유효하지 않은 JSON 작성
	if err := os.WriteFile(mcpPath, []byte(`{invalid`), 0644); err != nil {
		t.Fatalf("파일 생성 실패: %v", err)
	}

	// 잘못된 JSON이어도 새 설정으로 생성되어야 합니다
	err := ConfigureClaudeCodeMCP(projectDir)
	if err != nil {
		t.Fatalf("ConfigureClaudeCodeMCP() error = %v", err)
	}

	// 결과 파일이 유효한 JSON인지 확인
	result, readErr := ReadJSONConfig(mcpPath)
	if readErr != nil {
		t.Fatalf("결과 파일 읽기 실패: %v", readErr)
	}

	mcpServers, ok := result["mcpServers"].(map[string]interface{})
	if !ok {
		t.Fatal("mcpServers 필드가 없습니다")
	}

	if _, ok := mcpServers["autopus"]; !ok {
		t.Error("autopus 서버가 추가되지 않았습니다")
	}
}
