package aitools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestDetectGeminiCLI는 Gemini CLI 감지 함수가 정상 동작하는지 검증합니다.
func TestDetectGeminiCLI(t *testing.T) {
	info, err := DetectGeminiCLI()
	if err != nil {
		t.Fatalf("DetectGeminiCLI() 예상치 못한 에러: %v", err)
	}

	if info == nil {
		t.Fatal("DetectGeminiCLI() 결과가 nil이면 안 됩니다")
	}

	if info.Name != "Gemini CLI" {
		t.Errorf("Name = %q, want %q", info.Name, "Gemini CLI")
	}
}

// TestDetectGeminiCLI_API키감지는 GEMINI_API_KEY/GOOGLE_API_KEY 환경변수 감지를 검증합니다.
func TestDetectGeminiCLI_API키감지(t *testing.T) {
	tests := []struct {
		name       string
		geminiKey  string
		googleKey  string
		wantAPIKey bool
	}{
		{
			name:       "GEMINI_API_KEY가 설정된 경우",
			geminiKey:  "test-gemini-key",
			googleKey:  "",
			wantAPIKey: true,
		},
		{
			name:       "GOOGLE_API_KEY가 설정된 경우",
			geminiKey:  "",
			googleKey:  "test-google-key",
			wantAPIKey: true,
		},
		{
			name:       "두 키 모두 설정된 경우",
			geminiKey:  "test-gemini-key",
			googleKey:  "test-google-key",
			wantAPIKey: true,
		},
		{
			name:       "두 키 모두 없는 경우",
			geminiKey:  "",
			googleKey:  "",
			wantAPIKey: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GEMINI_API_KEY", tt.geminiKey)
			t.Setenv("GOOGLE_API_KEY", tt.googleKey)

			info, err := DetectGeminiCLI()
			if err != nil {
				t.Fatalf("DetectGeminiCLI() error = %v", err)
			}

			// CLI가 설치되어 있지 않으면 HasAPIKey 검증을 건너뜁니다.
			if info.Installed && info.HasAPIKey != tt.wantAPIKey {
				t.Errorf("HasAPIKey = %v, want %v", info.HasAPIKey, tt.wantAPIKey)
			}
		})
	}
}

// TestConfigureGeminiMCP는 Gemini CLI MCP 설정 기능을 테스트합니다.
func TestConfigureGeminiMCP(t *testing.T) {
	tests := []struct {
		name          string
		setupExisting bool
		existingData  map[string]interface{}
		description   string
	}{
		{
			name:          "설정 파일이 없는 경우 새로 생성",
			setupExisting: false,
			description:   "settings.json이 없으면 새로 생성해야 합니다",
		},
		{
			name:          "기존 설정에 autopus 항목 추가",
			setupExisting: true,
			existingData: map[string]interface{}{
				"mcpServers": map[string]interface{}{
					"filesystem": map[string]interface{}{
						"command": "npx",
						"args":    []string{"@modelcontextprotocol/server-filesystem"},
					},
				},
			},
			description: "기존 설정을 유지하면서 autopus를 추가해야 합니다",
		},
		{
			name:          "mcpServers가 없는 기존 설정",
			setupExisting: true,
			existingData: map[string]interface{}{
				"theme": "dark",
				"model": "gemini-2.5-pro",
			},
			description: "mcpServers 필드가 없는 기존 설정에도 정상 추가해야 합니다",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			t.Setenv("HOME", tmpDir)

			settingsPath := filepath.Join(tmpDir, ".gemini", "settings.json")

			if tt.setupExisting {
				if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
					t.Fatalf("디렉토리 생성 실패: %v", err)
				}
				jsonData, _ := json.MarshalIndent(tt.existingData, "", "  ")
				if err := os.WriteFile(settingsPath, jsonData, 0644); err != nil {
					t.Fatalf("기존 설정 파일 생성 실패: %v", err)
				}
			}

			err := ConfigureGeminiMCP()
			if err != nil {
				t.Fatalf("ConfigureGeminiMCP() error = %v", err)
			}

			// 파일이 생성되었는지 확인
			result, readErr := ReadJSONConfig(settingsPath)
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

			// 기존 설정이 유지되는지 확인
			if tt.setupExisting {
				for key := range tt.existingData {
					if key == "mcpServers" {
						// mcpServers 내부의 기존 서버가 유지되는지 확인
						if existingServers, ok := tt.existingData["mcpServers"].(map[string]interface{}); ok {
							for serverName := range existingServers {
								if _, exists := mcpServers[serverName]; !exists {
									t.Errorf("기존 서버 %q가 삭제되었습니다", serverName)
								}
							}
						}
					} else {
						if _, exists := result[key]; !exists {
							t.Errorf("기존 설정 키 %q가 삭제되었습니다", key)
						}
					}
				}
			}
		})
	}
}

// TestConfigureGeminiMCP_백업생성은 기존 파일이 있을 때 백업이 생성되는지 검증합니다.
func TestConfigureGeminiMCP_백업생성(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	settingsPath := filepath.Join(tmpDir, ".gemini", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
		t.Fatalf("디렉토리 생성 실패: %v", err)
	}

	originalContent := `{"theme": "dark"}`
	if err := os.WriteFile(settingsPath, []byte(originalContent), 0644); err != nil {
		t.Fatalf("기존 파일 생성 실패: %v", err)
	}

	err := ConfigureGeminiMCP()
	if err != nil {
		t.Fatalf("ConfigureGeminiMCP() error = %v", err)
	}

	// .bak 파일이 생성되었는지 확인
	backupPath := settingsPath + ".bak"
	backupContent, readErr := os.ReadFile(backupPath)
	if readErr != nil {
		t.Fatalf("백업 파일 읽기 실패: %v", readErr)
	}

	if string(backupContent) != originalContent {
		t.Errorf("백업 파일 내용이 원본과 다릅니다: got %q, want %q", string(backupContent), originalContent)
	}
}

// TestConfigureGeminiMCP_디렉토리자동생성은 .gemini 디렉토리가 없을 때 자동 생성되는지 검증합니다.
func TestConfigureGeminiMCP_디렉토리자동생성(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// .gemini 디렉토리를 미리 만들지 않음
	err := ConfigureGeminiMCP()
	if err != nil {
		t.Fatalf("ConfigureGeminiMCP() error = %v", err)
	}

	settingsPath := filepath.Join(tmpDir, ".gemini", "settings.json")
	if _, statErr := os.Stat(settingsPath); os.IsNotExist(statErr) {
		t.Error("settings.json이 생성되지 않았습니다")
	}
}

// TestConfigureGeminiMCP_잘못된JSON은 기존 파일이 유효하지 않은 JSON일 때의 동작을 검증합니다.
func TestConfigureGeminiMCP_잘못된JSON(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	settingsPath := filepath.Join(tmpDir, ".gemini", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
		t.Fatalf("디렉토리 생성 실패: %v", err)
	}

	// 유효하지 않은 JSON 작성
	if err := os.WriteFile(settingsPath, []byte(`{broken json`), 0644); err != nil {
		t.Fatalf("파일 생성 실패: %v", err)
	}

	// 잘못된 JSON이어도 새 설정으로 생성되어야 합니다
	err := ConfigureGeminiMCP()
	if err != nil {
		t.Fatalf("ConfigureGeminiMCP() error = %v", err)
	}

	// 결과 파일이 유효한 JSON인지 확인
	result, readErr := ReadJSONConfig(settingsPath)
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

// TestConfigureGeminiMCP_멱등성은 두 번 호출해도 정상 동작하는지 검증합니다.
func TestConfigureGeminiMCP_멱등성(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// 첫 번째 호출
	err := ConfigureGeminiMCP()
	if err != nil {
		t.Fatalf("첫 번째 ConfigureGeminiMCP() error = %v", err)
	}

	// 두 번째 호출
	err = ConfigureGeminiMCP()
	if err != nil {
		t.Fatalf("두 번째 ConfigureGeminiMCP() error = %v", err)
	}

	settingsPath := filepath.Join(tmpDir, ".gemini", "settings.json")
	result, readErr := ReadJSONConfig(settingsPath)
	if readErr != nil {
		t.Fatalf("설정 파일 읽기 실패: %v", readErr)
	}

	// autopus 서버가 여전히 존재하는지 확인
	mcpServers, ok := result["mcpServers"].(map[string]interface{})
	if !ok {
		t.Fatal("mcpServers 필드가 없습니다")
	}

	if _, ok := mcpServers["autopus"]; !ok {
		t.Error("autopus 서버가 없습니다")
	}

	// mcpServers에 autopus가 정확히 하나만 있는지 확인 (JSON은 키 중복 불가)
	autopusData, ok := mcpServers["autopus"].(map[string]interface{})
	if !ok {
		t.Fatal("autopus 서버 데이터가 올바르지 않습니다")
	}

	if autopusData["command"] != "autopus-bridge" {
		t.Errorf("command = %v, want %q", autopusData["command"], "autopus-bridge")
	}
}
