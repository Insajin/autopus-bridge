// Package auth provides authentication utilities for Local Agent Bridge.
// It handles credential storage, loading, and token refresh.
package auth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Credentials stores authentication tokens for the CLI.
type Credentials struct {
	AccessToken   string    `json:"access_token"`
	RefreshToken  string    `json:"refresh_token"`
	ExpiresAt     time.Time `json:"expires_at"`
	ServerURL     string    `json:"server_url"`
	UserEmail     string    `json:"user_email,omitempty"`
	WorkspaceID   string    `json:"workspace_id,omitempty"`
	WorkspaceSlug string    `json:"workspace_slug,omitempty"`
	WorkspaceName string    `json:"workspace_name,omitempty"`
}

// IsExpired checks if the access token has expired.
func (c *Credentials) IsExpired() bool {
	// Add 30 second buffer for clock skew
	return time.Now().Add(30 * time.Second).After(c.ExpiresAt)
}

// IsValid checks if credentials are valid (non-empty and not expired).
func (c *Credentials) IsValid() bool {
	return c.AccessToken != "" && !c.IsExpired()
}

// ParseJWTExpiry JWT 토큰에서 exp 클레임을 추출하여 만료 시간을 파싱합니다.
// JWT 서명을 검증하지 않고 exp 클레임만 추출합니다 (서명 검증은 서버의 책임).
// 만료 시간을 기준으로 토큰 새로고침을 스케줄링하기 위한 용도입니다.
func ParseJWTExpiry(token string) (time.Time, error) {
	// JWT는 header.payload.signature 형식이므로 '.'으로 분리
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return time.Time{}, fmt.Errorf("유효하지 않은 JWT 형식: %d개 부분 (예상: 3개)", len(parts))
	}

	// payload는 두 번째 부분
	payload := parts[1]

	// base64url 디코딩 (패딩 추가)
	// base64url은 '-'과 '_'를 사용하고 패딩을 생략하므로 복구 필요
	padded := payload
	switch len(payload) % 4 {
	case 1:
		padded += "==="
	case 2:
		padded += "=="
	case 3:
		padded += "="
	}

	decoded, err := base64.URLEncoding.DecodeString(padded)
	if err != nil {
		return time.Time{}, fmt.Errorf("JWT payload base64 디코딩 실패: %w", err)
	}

	// JSON 파싱하여 exp 클레임 추출
	var claims map[string]interface{}
	if err := json.Unmarshal(decoded, &claims); err != nil {
		return time.Time{}, fmt.Errorf("JWT payload JSON 파싱 실패: %w", err)
	}

	// exp 클레임 확인
	expValue, ok := claims["exp"]
	if !ok {
		return time.Time{}, fmt.Errorf("JWT payload에 exp 클레임이 없습니다")
	}

	// exp는 Unix 타임스탐프 (정수)
	expFloat, ok := expValue.(float64)
	if !ok {
		return time.Time{}, fmt.Errorf("JWT exp 클레임이 숫자가 아닙니다: %T", expValue)
	}

	// Unix 타임스탬프를 time.Time으로 변환
	return time.Unix(int64(expFloat), 0), nil
}

// ExpiresAtFromJWT JWT 토큰에서 exp 클레임을 기반으로 만료 시간을 반환합니다.
func (c *Credentials) ExpiresAtFromJWT() (time.Time, error) {
	return ParseJWTExpiry(c.AccessToken)
}

// credentialsDir returns the directory for storing credentials.
// Uses ~/.config/autopus on Unix-like systems.
func credentialsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}

	// Use XDG_CONFIG_HOME if set, otherwise ~/.config
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		configDir = filepath.Join(home, ".config")
	}

	dir := filepath.Join(configDir, "autopus")
	return dir, nil
}

// credentialsPath returns the full path to the credentials file.
func credentialsPath() (string, error) {
	dir, err := credentialsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "credentials.json"), nil
}

// Save stores credentials to the filesystem.
// The file is created with 0600 permissions for security.
func Save(creds *Credentials) error {
	dir, err := credentialsDir()
	if err != nil {
		return err
	}

	// Create directory if it doesn't exist (0700 for security)
	if mkdirErr := os.MkdirAll(dir, 0700); mkdirErr != nil {
		return fmt.Errorf("create config directory: %w", mkdirErr)
	}

	path, err := credentialsPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal credentials: %w", err)
	}

	// Write with restrictive permissions (owner read/write only)
	if writeErr := os.WriteFile(path, data, 0600); writeErr != nil {
		return fmt.Errorf("write credentials file: %w", writeErr)
	}

	// SEC-P2-01: 명시적 권한 설정 (umask에 의한 권한 완화 방지)
	if chmodErr := os.Chmod(path, 0600); chmodErr != nil {
		return fmt.Errorf("set credentials file permissions: %w", chmodErr)
	}

	return nil
}

// Load reads credentials from the filesystem.
// Returns nil if no credentials file exists.
func Load() (*Credentials, error) {
	path, err := credentialsPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No credentials stored
		}
		return nil, fmt.Errorf("read credentials file: %w", err)
	}

	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("parse credentials file: %w", err)
	}

	return &creds, nil
}

// Clear removes stored credentials.
func Clear() error {
	path, err := credentialsPath()
	if err != nil {
		return err
	}

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove credentials file: %w", err)
	}

	return nil
}

// Exists checks if credentials file exists.
func Exists() bool {
	path, err := credentialsPath()
	if err != nil {
		return false
	}

	_, err = os.Stat(path)
	return err == nil
}

// MaskToken masks a token for safe logging (SEC-P2-04).
// Only the first 8 characters are shown, followed by "...".
// Tokens shorter than or equal to 8 characters are fully masked.
func MaskToken(token string) string {
	if len(token) <= 8 {
		return "***"
	}
	return token[:8] + "..."
}
