// claude.go는 Claude Code 감지, 플러그인 설치, MCP 설정 기능을 제공합니다.
package aitools

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// DetectClaudeCode는 Claude Code CLI의 설치 여부를 감지합니다.
func DetectClaudeCode() (*AIToolInfo, error) {
	info := &AIToolInfo{
		Name: "Claude Code",
	}

	// CLI 바이너리 감지
	cliPath, err := exec.LookPath("claude")
	if err != nil {
		return info, nil // 미설치 -- 에러가 아님
	}
	info.Installed = true
	info.CLIPath = cliPath

	// 버전 확인
	out, err := exec.Command("claude", "--version").Output()
	if err == nil {
		info.Version = strings.TrimSpace(strings.Split(string(out), "\n")[0])
	}

	// 설정 디렉토리 확인
	home, err := os.UserHomeDir()
	if err == nil {
		configDir := filepath.Join(home, ".claude")
		if _, statErr := os.Stat(configDir); statErr == nil {
			info.ConfigPath = configDir
		}
	}

	return info, nil
}

// IsPluginInstalled는 Autopus 플러그인이 이미 설치되어 있는지 확인합니다.
func IsPluginInstalled() bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}

	pluginJSON := filepath.Join(home, ".claude", "plugins", "autopus", ".claude-plugin", "plugin.json")
	_, err = os.Stat(pluginJSON)
	return err == nil
}

// InstallPlugin은 Autopus Claude Code 플러그인을 ~/.claude/plugins/autopus/에 설치합니다.
func InstallPlugin() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("홈 디렉토리 확인 실패: %w", err)
	}

	pluginDir := filepath.Join(home, ".claude", "plugins", "autopus")

	// 이미 설치된 경우 건너뜀
	if IsPluginInstalled() {
		return nil
	}

	// 임베디드 파일을 대상 디렉토리에 복사
	return installPluginTo(pluginDir)
}

// InstallPluginTo는 지정된 경로에 플러그인을 설치합니다 (테스트용).
func InstallPluginTo(targetDir string) error {
	return installPluginTo(targetDir)
}

// installPluginTo는 임베디드 플러그인 파일을 대상 디렉토리에 복사합니다.
func installPluginTo(targetDir string) error {
	return fs.WalkDir(pluginFiles, "plugin-dist", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		// "plugin-dist" 접두사 제거하여 상대 경로 계산
		relPath, relErr := filepath.Rel("plugin-dist", path)
		if relErr != nil {
			return fmt.Errorf("상대 경로 계산 실패 (%s): %w", path, relErr)
		}
		if relPath == "." {
			return nil
		}
		targetPath := filepath.Join(targetDir, relPath)

		if d.IsDir() {
			return os.MkdirAll(targetPath, 0755)
		}

		data, readErr := pluginFiles.ReadFile(path)
		if readErr != nil {
			return fmt.Errorf("임베디드 파일 읽기 실패 (%s): %w", path, readErr)
		}

		// 상위 디렉토리 확인
		if mkErr := os.MkdirAll(filepath.Dir(targetPath), 0755); mkErr != nil {
			return fmt.Errorf("디렉토리 생성 실패: %w", mkErr)
		}

		return os.WriteFile(targetPath, data, 0644)
	})
}

// PluginVersion은 임베디드 플러그인의 버전을 반환합니다.
func PluginVersion() string {
	data, err := pluginFiles.ReadFile("plugin-dist/.claude-plugin/plugin.json")
	if err != nil {
		return "unknown"
	}

	var manifest struct {
		Version string `json:"version"`
	}
	if jsonErr := json.Unmarshal(data, &manifest); jsonErr != nil {
		return "unknown"
	}

	return manifest.Version
}

// ConfigureClaudeCodeMCP는 Claude Code의 .mcp.json에 Autopus MCP 서버를 설정합니다.
// projectDir이 비어있으면 글로벌 설정(~/.claude/.mcp.json)을 사용합니다.
func ConfigureClaudeCodeMCP(projectDir string) error {
	var mcpPath string

	if projectDir != "" {
		mcpPath = filepath.Join(projectDir, ".mcp.json")
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("홈 디렉토리 확인 실패: %w", err)
		}
		mcpPath = filepath.Join(home, ".claude", ".mcp.json")
	}

	// 기존 설정 읽기 또는 새로 생성
	var config map[string]interface{}

	if _, err := os.Stat(mcpPath); err == nil {
		// 기존 파일 백업
		if backupErr := BackupFile(mcpPath); backupErr != nil {
			return fmt.Errorf("백업 실패: %w", backupErr)
		}

		config, err = ReadJSONConfig(mcpPath)
		if err != nil {
			// 파싱 실패 시 새로 생성
			config = make(map[string]interface{})
		}
	} else {
		config = make(map[string]interface{})
	}

	// Autopus MCP 서버 추가
	addMCPServerToJSON(config, "autopus", DefaultAutopusMCPServer())

	// 저장
	return WriteJSONConfig(mcpPath, config)
}
