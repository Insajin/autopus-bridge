// Package aitools는 AI CLI 도구의 MCP 자동 설정 기능을 제공합니다.
// Claude Code, Codex CLI, Gemini CLI의 감지 및 MCP 설정을 지원합니다.
package aitools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// AIToolInfo는 감지된 AI 도구 정보를 담는 구조체입니다.
type AIToolInfo struct {
	Name       string `json:"name"`
	Installed  bool   `json:"installed"`
	Version    string `json:"version,omitempty"`
	CLIPath    string `json:"cli_path,omitempty"`
	ConfigPath string `json:"config_path,omitempty"`
	HasAPIKey  bool   `json:"has_api_key"`
}

// MCPServerConfig는 MCP 서버 설정 항목입니다.
type MCPServerConfig struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

// DefaultAutopusMCPServer는 Autopus MCP 서버 기본 설정을 반환합니다.
func DefaultAutopusMCPServer() MCPServerConfig {
	return MCPServerConfig{
		Command: "autopus-bridge",
		Args:    []string{"mcp-serve"},
	}
}

// ReadJSONConfig는 JSON 설정 파일을 읽어 map으로 반환합니다.
func ReadJSONConfig(path string) (map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("설정 파일 읽기 실패: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("JSON 파싱 실패: %w", err)
	}

	return result, nil
}

// WriteJSONConfig는 map을 JSON 설정 파일로 저장합니다.
func WriteJSONConfig(path string, data map[string]interface{}) error {
	if err := EnsureDir(path); err != nil {
		return err
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("JSON 직렬화 실패: %w", err)
	}

	jsonData = append(jsonData, '\n')

	if err := os.WriteFile(path, jsonData, 0644); err != nil {
		return fmt.Errorf("설정 파일 저장 실패: %w", err)
	}

	return nil
}

// BackupFile는 파일의 .bak 백업을 생성합니다.
func BackupFile(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil // 파일이 없으면 백업 불필요
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("백업 원본 읽기 실패: %w", err)
	}

	backupPath := path + ".bak"
	if err := os.WriteFile(backupPath, data, 0644); err != nil {
		return fmt.Errorf("백업 파일 저장 실패: %w", err)
	}

	return nil
}

// EnsureDir는 파일 경로의 상위 디렉토리가 존재하는지 확인하고, 없으면 생성합니다.
func EnsureDir(filePath string) error {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("디렉토리 생성 실패: %w", err)
	}
	return nil
}

// addMCPServerToJSON는 JSON 설정에 MCP 서버 항목을 추가합니다.
func addMCPServerToJSON(config map[string]interface{}, serverName string, server MCPServerConfig) {
	mcpServers, ok := config["mcpServers"].(map[string]interface{})
	if !ok {
		mcpServers = make(map[string]interface{})
		config["mcpServers"] = mcpServers
	}

	mcpServers[serverName] = map[string]interface{}{
		"command": server.Command,
		"args":    server.Args,
	}
}
