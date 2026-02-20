// codex.go는 Codex CLI 감지 및 MCP 설정 기능을 제공합니다.
package aitools

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const codexMCPSection = `
[mcp_servers.autopus]
command = "autopus-bridge"
args = ["mcp-serve"]
`

// DetectCodexCLI는 Codex CLI의 설치 여부를 감지합니다.
func DetectCodexCLI() (*AIToolInfo, error) {
	info := &AIToolInfo{
		Name: "Codex CLI",
	}

	// CLI 바이너리 감지
	cliPath, err := exec.LookPath("codex")
	if err != nil {
		return info, nil
	}
	info.Installed = true
	info.CLIPath = cliPath

	// API 키 확인
	if os.Getenv("OPENAI_API_KEY") != "" {
		info.HasAPIKey = true
	}

	// 설정 파일 경로
	home, err := os.UserHomeDir()
	if err == nil {
		info.ConfigPath = filepath.Join(home, ".codex", "config.toml")
	}

	return info, nil
}

// ConfigureCodexMCP는 Codex CLI의 config.toml에 Autopus MCP 서버를 설정합니다.
func ConfigureCodexMCP() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("홈 디렉토리 확인 실패: %w", err)
	}

	configPath := filepath.Join(home, ".codex", "config.toml")

	// 기존 설정 파일 확인
	if _, statErr := os.Stat(configPath); statErr == nil {
		// 기존 파일이 있으면 백업 후 수정
		if backupErr := BackupFile(configPath); backupErr != nil {
			return fmt.Errorf("백업 실패: %w", backupErr)
		}

		content, readErr := os.ReadFile(configPath)
		if readErr != nil {
			return fmt.Errorf("설정 파일 읽기 실패: %w", readErr)
		}

		// 이미 autopus 섹션이 있는지 확인
		if strings.Contains(string(content), "[mcp_servers.autopus]") {
			return nil // 이미 설정됨
		}

		// 기존 내용에 추가
		newContent := string(content) + codexMCPSection
		if writeErr := os.WriteFile(configPath, []byte(newContent), 0644); writeErr != nil {
			return fmt.Errorf("설정 파일 저장 실패: %w", writeErr)
		}

		return nil
	}

	// 새 파일 생성
	if err := EnsureDir(configPath); err != nil {
		return err
	}

	content := "# Codex CLI 설정 파일\n# autopus-bridge setup에 의해 생성됨\n" + codexMCPSection
	if writeErr := os.WriteFile(configPath, []byte(content), 0644); writeErr != nil {
		return fmt.Errorf("설정 파일 생성 실패: %w", writeErr)
	}

	return nil
}
