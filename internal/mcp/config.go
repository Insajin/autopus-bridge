// Package mcp는 Local Agent Bridge의 MCP 서버 프로세스 관리를 제공합니다.
// SPEC-SKILL-V2-001 Block D: Dynamic MCP Provisioning
package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ServerConfig는 로컬 MCP 서버의 설정을 나타냅니다.
type ServerConfig struct {
	Name           string            `json:"name"`
	Command        string            `json:"command"`
	Args           []string          `json:"args"`
	Env            map[string]string `json:"env,omitempty"`
	WorkingDir     string            `json:"working_dir,omitempty"`
	RequiredBinary string            `json:"required_binary,omitempty"`
}

// LocalConfig는 ~/.acos/mcp-servers.json 파일 구조입니다.
type LocalConfig struct {
	Servers map[string]ServerConfig `json:"servers"`
}

// DefaultConfigPath는 기본 MCP 서버 설정 파일 경로를 반환합니다.
func DefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".acos", "mcp-servers.json")
}

// LoadConfig는 로컬 MCP 서버 설정을 로드합니다.
func LoadConfig(path string) (*LocalConfig, error) {
	if path == "" {
		path = DefaultConfigPath()
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &LocalConfig{Servers: make(map[string]ServerConfig)}, nil
		}
		return nil, fmt.Errorf("MCP 설정 파일 읽기 실패: %w", err)
	}

	var cfg LocalConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("MCP 설정 파일 파싱 실패: %w", err)
	}

	if cfg.Servers == nil {
		cfg.Servers = make(map[string]ServerConfig)
	}

	return &cfg, nil
}

// GetServerConfig는 이름으로 서버 설정을 조회합니다.
func (c *LocalConfig) GetServerConfig(name string) (*ServerConfig, bool) {
	cfg, ok := c.Servers[name]
	if !ok {
		return nil, false
	}
	return &cfg, true
}
