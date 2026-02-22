package auth

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// setupTestEnv는 XDG_CONFIG_HOME을 t.TempDir()로 설정하여
// 실제 설정 파일에 영향을 주지 않도록 테스트 환경을 구성합니다.
func setupTestEnv(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	return tmpDir
}

// newTestCredentials는 테스트용 Credentials를 생성합니다.
func newTestCredentials(expiresAt time.Time) *Credentials {
	return &Credentials{
		AccessToken:   "test-access-token",
		RefreshToken:  "test-refresh-token",
		ExpiresAt:     expiresAt,
		ServerURL:     "https://example.com",
		UserEmail:     "test@example.com",
		WorkspaceID:   "ws-123",
		WorkspaceSlug: "test-workspace",
	}
}

// TestSave_SavesCredentialsToFile는 자격 증명이 파일에 올바르게 저장되는지 테스트합니다.
func TestSave_SavesCredentialsToFile(t *testing.T) {
	tmpDir := setupTestEnv(t)

	creds := newTestCredentials(time.Now().Add(1 * time.Hour))

	if err := Save(creds); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// 파일이 생성되었는지 확인
	expectedPath := filepath.Join(tmpDir, "autopus", "credentials.json")
	data, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("credentials 파일 읽기 실패: %v", err)
	}

	// JSON 내용 검증
	var loaded Credentials
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("JSON 파싱 실패: %v", err)
	}

	if loaded.AccessToken != creds.AccessToken {
		t.Errorf("AccessToken = %q, want %q", loaded.AccessToken, creds.AccessToken)
	}
	if loaded.RefreshToken != creds.RefreshToken {
		t.Errorf("RefreshToken = %q, want %q", loaded.RefreshToken, creds.RefreshToken)
	}
	if loaded.ServerURL != creds.ServerURL {
		t.Errorf("ServerURL = %q, want %q", loaded.ServerURL, creds.ServerURL)
	}
	if loaded.UserEmail != creds.UserEmail {
		t.Errorf("UserEmail = %q, want %q", loaded.UserEmail, creds.UserEmail)
	}
	if loaded.WorkspaceID != creds.WorkspaceID {
		t.Errorf("WorkspaceID = %q, want %q", loaded.WorkspaceID, creds.WorkspaceID)
	}
	if loaded.WorkspaceSlug != creds.WorkspaceSlug {
		t.Errorf("WorkspaceSlug = %q, want %q", loaded.WorkspaceSlug, creds.WorkspaceSlug)
	}
}

// TestSave_CreatesDirectoryIfNeeded는 디렉터리가 없을 때 자동으로 생성하는지 테스트합니다.
func TestSave_CreatesDirectoryIfNeeded(t *testing.T) {
	tmpDir := setupTestEnv(t)

	// 디렉터리가 아직 존재하지 않는 것을 확인
	dirPath := filepath.Join(tmpDir, "autopus")
	if _, err := os.Stat(dirPath); !os.IsNotExist(err) {
		t.Fatal("테스트 전에 디렉터리가 이미 존재합니다")
	}

	creds := newTestCredentials(time.Now().Add(1 * time.Hour))
	if err := Save(creds); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// 디렉터리가 생성되었는지 확인
	info, err := os.Stat(dirPath)
	if err != nil {
		t.Fatalf("디렉터리 확인 실패: %v", err)
	}
	if !info.IsDir() {
		t.Error("생성된 경로가 디렉터리가 아닙니다")
	}
}

// TestSave_FilePermissions는 파일이 0600 권한으로 생성되는지 테스트합니다.
func TestSave_FilePermissions(t *testing.T) {
	// Windows에서는 Unix 파일 권한이 적용되지 않으므로 건너뜁니다.
	if runtime.GOOS == "windows" {
		t.Skip("Windows에서는 Unix 파일 권한 테스트를 건너뜁니다")
	}

	tmpDir := setupTestEnv(t)

	creds := newTestCredentials(time.Now().Add(1 * time.Hour))
	if err := Save(creds); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	filePath := filepath.Join(tmpDir, "autopus", "credentials.json")
	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("파일 정보 확인 실패: %v", err)
	}

	// 파일 권한이 0600인지 확인 (owner read/write only)
	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("파일 권한 = %o, want %o", perm, 0600)
	}
}

// TestLoad_LoadsValidCredentials는 유효한 자격 증명 파일을 올바르게 로드하는지 테스트합니다.
func TestLoad_LoadsValidCredentials(t *testing.T) {
	setupTestEnv(t)

	// 먼저 저장
	original := newTestCredentials(time.Now().Add(1 * time.Hour))
	if err := Save(original); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// 로드
	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded == nil {
		t.Fatal("Load() returned nil, want non-nil")
	}

	if loaded.AccessToken != original.AccessToken {
		t.Errorf("AccessToken = %q, want %q", loaded.AccessToken, original.AccessToken)
	}
	if loaded.RefreshToken != original.RefreshToken {
		t.Errorf("RefreshToken = %q, want %q", loaded.RefreshToken, original.RefreshToken)
	}
	if loaded.ServerURL != original.ServerURL {
		t.Errorf("ServerURL = %q, want %q", loaded.ServerURL, original.ServerURL)
	}
	if loaded.UserEmail != original.UserEmail {
		t.Errorf("UserEmail = %q, want %q", loaded.UserEmail, original.UserEmail)
	}
}

// TestLoad_ReturnsNilForMissingFile는 파일이 없을 때 nil을 반환하는지 테스트합니다.
func TestLoad_ReturnsNilForMissingFile(t *testing.T) {
	setupTestEnv(t)

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if loaded != nil {
		t.Errorf("Load() = %v, want nil", loaded)
	}
}

// TestLoad_HandlesInvalidJSON는 잘못된 JSON 파일 처리를 테스트합니다.
func TestLoad_HandlesInvalidJSON(t *testing.T) {
	tmpDir := setupTestEnv(t)

	// 잘못된 JSON을 직접 작성
	dir := filepath.Join(tmpDir, "autopus")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatalf("디렉터리 생성 실패: %v", err)
	}
	filePath := filepath.Join(dir, "credentials.json")
	if err := os.WriteFile(filePath, []byte("{invalid json}"), 0600); err != nil {
		t.Fatalf("파일 작성 실패: %v", err)
	}

	loaded, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want error for invalid JSON")
	}
	if loaded != nil {
		t.Errorf("Load() = %v, want nil on error", loaded)
	}
}

// TestClear_RemovesCredentialsFile는 자격 증명 파일이 삭제되는지 테스트합니다.
func TestClear_RemovesCredentialsFile(t *testing.T) {
	tmpDir := setupTestEnv(t)

	// 먼저 저장
	creds := newTestCredentials(time.Now().Add(1 * time.Hour))
	if err := Save(creds); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// 파일 존재 확인
	filePath := filepath.Join(tmpDir, "autopus", "credentials.json")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Fatal("Clear 전에 파일이 존재하지 않습니다")
	}

	// 삭제
	if err := Clear(); err != nil {
		t.Fatalf("Clear() error = %v", err)
	}

	// 파일이 삭제되었는지 확인
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Error("Clear() 후에도 파일이 존재합니다")
	}
}

// TestClear_NoErrorIfFileNotExists는 파일이 없을 때 에러가 발생하지 않는지 테스트합니다.
func TestClear_NoErrorIfFileNotExists(t *testing.T) {
	setupTestEnv(t)

	// 파일이 없는 상태에서 Clear 호출
	if err := Clear(); err != nil {
		t.Errorf("Clear() error = %v, want nil", err)
	}
}

// TestExists_TrueWhenFileExists는 파일이 존재할 때 true를 반환하는지 테스트합니다.
func TestExists_TrueWhenFileExists(t *testing.T) {
	setupTestEnv(t)

	creds := newTestCredentials(time.Now().Add(1 * time.Hour))
	if err := Save(creds); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if !Exists() {
		t.Error("Exists() = false, want true")
	}
}

// TestExists_FalseWhenFileNotExists는 파일이 없을 때 false를 반환하는지 테스트합니다.
func TestExists_FalseWhenFileNotExists(t *testing.T) {
	setupTestEnv(t)

	if Exists() {
		t.Error("Exists() = true, want false")
	}
}

// TestIsExpired는 토큰 만료 여부를 테스트합니다.
func TestIsExpired(t *testing.T) {
	tests := []struct {
		name      string
		expiresAt time.Time
		expected  bool
	}{
		{
			name:      "이미 만료된 토큰",
			expiresAt: time.Now().Add(-1 * time.Hour),
			expected:  true,
		},
		{
			name:      "30초 이내 만료 예정 (버퍼 내)",
			expiresAt: time.Now().Add(10 * time.Second),
			expected:  true,
		},
		{
			name:      "아직 유효한 토큰 (버퍼 밖)",
			expiresAt: time.Now().Add(5 * time.Minute),
			expected:  false,
		},
		{
			name:      "먼 미래 만료",
			expiresAt: time.Now().Add(24 * time.Hour),
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			creds := &Credentials{ExpiresAt: tt.expiresAt}
			if got := creds.IsExpired(); got != tt.expected {
				t.Errorf("IsExpired() = %v, want %v (expiresAt: %v, now: %v)",
					got, tt.expected, tt.expiresAt, time.Now())
			}
		})
	}
}

// TestIsValid는 자격 증명의 유효성을 테스트합니다.
func TestIsValid(t *testing.T) {
	tests := []struct {
		name     string
		creds    *Credentials
		expected bool
	}{
		{
			name: "유효한 자격 증명 (토큰 있음 + 만료 안 됨)",
			creds: &Credentials{
				AccessToken: "valid-token",
				ExpiresAt:   time.Now().Add(1 * time.Hour),
			},
			expected: true,
		},
		{
			name: "빈 AccessToken",
			creds: &Credentials{
				AccessToken: "",
				ExpiresAt:   time.Now().Add(1 * time.Hour),
			},
			expected: false,
		},
		{
			name: "만료된 토큰",
			creds: &Credentials{
				AccessToken: "expired-token",
				ExpiresAt:   time.Now().Add(-1 * time.Hour),
			},
			expected: false,
		},
		{
			name: "빈 토큰 + 만료됨",
			creds: &Credentials{
				AccessToken: "",
				ExpiresAt:   time.Now().Add(-1 * time.Hour),
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.creds.IsValid(); got != tt.expected {
				t.Errorf("IsValid() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestSave_ThenLoad_RoundTrip은 Save와 Load의 왕복 일관성을 테스트합니다.
func TestSave_ThenLoad_RoundTrip(t *testing.T) {
	setupTestEnv(t)

	original := newTestCredentials(time.Now().Add(1 * time.Hour).Truncate(time.Millisecond))

	if err := Save(original); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded == nil {
		t.Fatal("Load() returned nil")
	}

	// 모든 필드 비교
	if loaded.AccessToken != original.AccessToken {
		t.Errorf("AccessToken = %q, want %q", loaded.AccessToken, original.AccessToken)
	}
	if loaded.RefreshToken != original.RefreshToken {
		t.Errorf("RefreshToken = %q, want %q", loaded.RefreshToken, original.RefreshToken)
	}
	if loaded.ServerURL != original.ServerURL {
		t.Errorf("ServerURL = %q, want %q", loaded.ServerURL, original.ServerURL)
	}
	if loaded.UserEmail != original.UserEmail {
		t.Errorf("UserEmail = %q, want %q", loaded.UserEmail, original.UserEmail)
	}
	if loaded.WorkspaceID != original.WorkspaceID {
		t.Errorf("WorkspaceID = %q, want %q", loaded.WorkspaceID, original.WorkspaceID)
	}
	if loaded.WorkspaceSlug != original.WorkspaceSlug {
		t.Errorf("WorkspaceSlug = %q, want %q", loaded.WorkspaceSlug, original.WorkspaceSlug)
	}
	// ExpiresAt은 JSON 직렬화/역직렬화로 나노초 정밀도가 달라질 수 있으므로
	// Truncate(time.Millisecond) 처리 후 비교
	if !loaded.ExpiresAt.Truncate(time.Millisecond).Equal(original.ExpiresAt) {
		t.Errorf("ExpiresAt = %v, want %v", loaded.ExpiresAt, original.ExpiresAt)
	}
}

// TestSave_OverwritesExistingFile는 기존 파일을 덮어쓰는지 테스트합니다.
func TestSave_OverwritesExistingFile(t *testing.T) {
	setupTestEnv(t)

	// 첫 번째 저장
	first := newTestCredentials(time.Now().Add(1 * time.Hour))
	if err := Save(first); err != nil {
		t.Fatalf("첫 번째 Save() error = %v", err)
	}

	// 두 번째 저장 (다른 토큰)
	second := &Credentials{
		AccessToken:  "updated-token",
		RefreshToken: "updated-refresh",
		ExpiresAt:    time.Now().Add(2 * time.Hour),
		ServerURL:    "https://updated.example.com",
	}
	if err := Save(second); err != nil {
		t.Fatalf("두 번째 Save() error = %v", err)
	}

	// 로드하여 두 번째 값인지 확인
	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.AccessToken != second.AccessToken {
		t.Errorf("AccessToken = %q, want %q", loaded.AccessToken, second.AccessToken)
	}
	if loaded.ServerURL != second.ServerURL {
		t.Errorf("ServerURL = %q, want %q", loaded.ServerURL, second.ServerURL)
	}
}

// TestClear_ThenExists는 Clear 후 Exists가 false를 반환하는지 테스트합니다.
func TestClear_ThenExists(t *testing.T) {
	setupTestEnv(t)

	creds := newTestCredentials(time.Now().Add(1 * time.Hour))
	if err := Save(creds); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if !Exists() {
		t.Fatal("Save 후 Exists() = false, want true")
	}

	if err := Clear(); err != nil {
		t.Fatalf("Clear() error = %v", err)
	}

	if Exists() {
		t.Error("Clear 후 Exists() = true, want false")
	}
}

// TestClear_ThenLoad는 Clear 후 Load가 nil을 반환하는지 테스트합니다.
func TestClear_ThenLoad(t *testing.T) {
	setupTestEnv(t)

	creds := newTestCredentials(time.Now().Add(1 * time.Hour))
	if err := Save(creds); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if err := Clear(); err != nil {
		t.Fatalf("Clear() error = %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded != nil {
		t.Errorf("Clear 후 Load() = %v, want nil", loaded)
	}
}
