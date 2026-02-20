// gemini.go는 Gemini CLI 감지 및 MCP 설정 기능을 제공합니다.
package aitools

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// DetectGeminiCLI는 Gemini CLI의 설치 여부를 감지합니다.
func DetectGeminiCLI() (*AIToolInfo, error) {
	info := &AIToolInfo{
		Name: "Gemini CLI",
	}

	// CLI 바이너리 감지
	cliPath, err := exec.LookPath("gemini")
	if err != nil {
		return info, nil
	}
	info.Installed = true
	info.CLIPath = cliPath

	// API 키 확인 (GEMINI_API_KEY 또는 GOOGLE_API_KEY)
	if os.Getenv("GEMINI_API_KEY") != "" || os.Getenv("GOOGLE_API_KEY") != "" {
		info.HasAPIKey = true
	}

	// 버전 확인
	out, err := exec.Command("gemini", "--version").Output()
	if err == nil {
		info.Version = strings.TrimSpace(strings.Split(string(out), "\n")[0])
	}

	// 설정 파일 경로
	home, err := os.UserHomeDir()
	if err == nil {
		info.ConfigPath = filepath.Join(home, ".gemini", "settings.json")
	}

	return info, nil
}

// ConfigureGeminiMCP는 Gemini CLI의 settings.json에 Autopus MCP 서버를 설정합니다.
func ConfigureGeminiMCP() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("홈 디렉토리 확인 실패: %w", err)
	}

	settingsPath := filepath.Join(home, ".gemini", "settings.json")

	// 기존 설정 읽기 또는 새로 생성
	var config map[string]interface{}

	if _, statErr := os.Stat(settingsPath); statErr == nil {
		// 기존 파일 백업
		if backupErr := BackupFile(settingsPath); backupErr != nil {
			return fmt.Errorf("백업 실패: %w", backupErr)
		}

		config, err = ReadJSONConfig(settingsPath)
		if err != nil {
			config = make(map[string]interface{})
		}
	} else {
		config = make(map[string]interface{})
	}

	// Autopus MCP 서버 추가
	addMCPServerToJSON(config, "autopus", DefaultAutopusMCPServer())

	// 저장
	return WriteJSONConfig(settingsPath, config)
}
