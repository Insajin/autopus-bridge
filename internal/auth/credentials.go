// Package auth provides authentication utilities for Local Agent Bridge.
// It handles credential storage, loading, and token refresh.
package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

// credentialsDir returns the directory for storing credentials.
// Uses ~/.config/local-agent-bridge on Unix-like systems.
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

	dir := filepath.Join(configDir, "local-agent-bridge")
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
