package executor

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/insajin/autopus-agent-protocol"
	"github.com/insajin/autopus-bridge/internal/config"
	"github.com/insajin/autopus-bridge/internal/provider"
)

// testHome은 테스트에서 사용할 홈 디렉토리를 반환합니다.
func testHome(t *testing.T) string {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("홈 디렉토리를 찾을 수 없습니다: %v", err)
	}
	return home
}

func TestSandbox_AllowedPathAccess(t *testing.T) {
	home := testHome(t)

	s := NewSandbox(config.SandboxConfig{
		Enabled:        true,
		AllowedPaths:   []string{"~/projects", "~/workspace"},
		DeniedPaths:    []string{},
		DenyHiddenDirs: false,
	})

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "허용된 경로 - projects",
			path:    filepath.Join(home, "projects", "my-app"),
			wantErr: false,
		},
		{
			name:    "허용된 경로 - workspace",
			path:    filepath.Join(home, "workspace", "repo"),
			wantErr: false,
		},
		{
			name:    "허용된 경로 - projects 루트",
			path:    filepath.Join(home, "projects"),
			wantErr: false,
		},
		{
			name:    "허용되지 않은 경로 - Desktop",
			path:    filepath.Join(home, "Desktop"),
			wantErr: true,
		},
		{
			name:    "허용되지 않은 경로 - 루트",
			path:    "/tmp/something",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := s.ValidatePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
		})
	}
}

func TestSandbox_DeniedPathAccess(t *testing.T) {
	home := testHome(t)

	s := NewSandbox(config.SandboxConfig{
		Enabled:        true,
		AllowedPaths:   []string{home}, // 홈 전체 허용
		DeniedPaths:    []string{"~/secret"},
		DenyHiddenDirs: false,
	})

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "허용된 일반 경로",
			path:    filepath.Join(home, "documents"),
			wantErr: false,
		},
		{
			name:    "거부된 사용자 설정 경로",
			path:    filepath.Join(home, "secret", "data"),
			wantErr: true,
		},
		{
			name:    "거부 경로가 허용보다 우선",
			path:    filepath.Join(home, "secret"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := s.ValidatePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
		})
	}
}

func TestSandbox_SSHDirectoryAlwaysDenied(t *testing.T) {
	home := testHome(t)

	// .ssh를 명시적으로 허용 목록에 넣어도 기본 거부 규칙에 의해 차단됨
	s := NewSandbox(config.SandboxConfig{
		Enabled:        true,
		AllowedPaths:   []string{home}, // 홈 전체 허용
		DeniedPaths:    []string{},     // 사용자 거부 목록 비어있음
		DenyHiddenDirs: false,          // 숨김 디렉토리 허용
	})

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    ".ssh 디렉토리 접근 차단",
			path:    filepath.Join(home, ".ssh"),
			wantErr: true,
		},
		{
			name:    ".ssh 하위 파일 접근 차단",
			path:    filepath.Join(home, ".ssh", "id_rsa"),
			wantErr: true,
		},
		{
			name:    ".gnupg 디렉토리 접근 차단",
			path:    filepath.Join(home, ".gnupg"),
			wantErr: true,
		},
		{
			name:    ".aws 디렉토리 접근 차단",
			path:    filepath.Join(home, ".aws", "credentials"),
			wantErr: true,
		},
		{
			name:    ".config 디렉토리 접근 차단",
			path:    filepath.Join(home, ".config", "sensitive"),
			wantErr: true,
		},
		{
			name:    "/etc 디렉토리 접근 차단",
			path:    "/etc/passwd",
			wantErr: true,
		},
		{
			name:    "/var 디렉토리 접근 차단",
			path:    "/var/log/syslog",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := s.ValidatePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
		})
	}
}

func TestSandbox_HiddenDirectoryDenied(t *testing.T) {
	home := testHome(t)

	s := NewSandbox(config.SandboxConfig{
		Enabled:        true,
		AllowedPaths:   []string{home},
		DeniedPaths:    []string{},
		DenyHiddenDirs: true,
	})

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "숨김 디렉토리 차단 - .hidden",
			path:    filepath.Join(home, ".hidden", "file.txt"),
			wantErr: true,
		},
		{
			name:    "숨김 디렉토리 차단 - .local",
			path:    filepath.Join(home, ".local", "share"),
			wantErr: true,
		},
		{
			name:    "일반 디렉토리 허용",
			path:    filepath.Join(home, "projects", "app"),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := s.ValidatePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
		})
	}
}

func TestSandbox_HiddenDirectoryAllowed(t *testing.T) {
	home := testHome(t)

	// deny_hidden_dirs가 false이면 숨김 디렉토리도 기본 거부 외에는 허용
	s := NewSandbox(config.SandboxConfig{
		Enabled:        true,
		AllowedPaths:   []string{home},
		DeniedPaths:    []string{},
		DenyHiddenDirs: false,
	})

	// .ssh 등 기본 거부 경로는 여전히 차단되지만
	// 일반 숨김 디렉토리는 허용됨
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "일반 숨김 디렉토리 허용 - .hidden",
			path:    filepath.Join(home, ".hidden", "file.txt"),
			wantErr: false,
		},
		{
			name:    "기본 거부 숨김 디렉토리 차단 - .ssh",
			path:    filepath.Join(home, ".ssh", "id_rsa"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := s.ValidatePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
		})
	}
}

func TestSandbox_PathTraversal(t *testing.T) {
	home := testHome(t)

	s := NewSandbox(config.SandboxConfig{
		Enabled:        true,
		AllowedPaths:   []string{"~/projects"},
		DeniedPaths:    []string{},
		DenyHiddenDirs: false,
	})

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "정상 경로",
			path:    filepath.Join(home, "projects", "app"),
			wantErr: false,
		},
		{
			name:    "경로 순회 공격 - ../etc/passwd",
			path:    filepath.Join(home, "projects", "..", "..", "etc", "passwd"),
			wantErr: true,
		},
		{
			name:    "경로 순회 공격 - ../../.ssh",
			path:    filepath.Join(home, "projects", "..", ".ssh", "id_rsa"),
			wantErr: true,
		},
		{
			name:    "경로 순회 공격 - 허용 범위 탈출",
			path:    filepath.Join(home, "projects", "..", "Desktop", "secret"),
			wantErr: true,
		},
		{
			name:    "경로 내 상대 참조 (허용 범위 내)",
			path:    filepath.Join(home, "projects", "app", "..", "other-app"),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := s.ValidatePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
		})
	}
}

func TestSandbox_Disabled(t *testing.T) {
	s := NewSandbox(config.SandboxConfig{
		Enabled:        false,
		AllowedPaths:   []string{"~/projects"},
		DeniedPaths:    []string{},
		DenyHiddenDirs: true,
	})

	// 비활성화된 샌드박스는 모든 경로를 허용
	tests := []struct {
		name string
		path string
	}{
		{"루트 경로", "/"},
		{"SSH 디렉토리", "~/.ssh/id_rsa"},
		{"/etc/passwd", "/etc/passwd"},
		{"숨김 디렉토리", "~/.hidden/file"},
		{"허용 범위 외 경로", "/tmp/something"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := s.ValidatePath(tt.path)
			if err != nil {
				t.Errorf("비활성화된 샌드박스가 경로를 차단함: %v", err)
			}
		})
	}
}

func TestSandbox_EmptyAllowlistDeniesEverything(t *testing.T) {
	home := testHome(t)

	s := NewSandbox(config.SandboxConfig{
		Enabled:        true,
		AllowedPaths:   []string{}, // 빈 허용 목록
		DeniedPaths:    []string{},
		DenyHiddenDirs: false,
	})

	tests := []struct {
		name string
		path string
	}{
		{"홈 디렉토리", home},
		{"프로젝트 디렉토리", filepath.Join(home, "projects")},
		{"임시 디렉토리", "/tmp"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := s.ValidatePath(tt.path)
			if err == nil {
				t.Errorf("빈 허용 목록에서 경로 '%s'가 허용됨", tt.path)
			}
			if !strings.Contains(err.Error(), "허용된 경로가 설정되지 않았습니다") {
				t.Errorf("예상과 다른 에러 메시지: %v", err)
			}
		})
	}
}

func TestSandbox_ValidateWorkDir(t *testing.T) {
	s := NewSandbox(config.SandboxConfig{
		Enabled:        true,
		AllowedPaths:   []string{"~/projects"},
		DeniedPaths:    []string{},
		DenyHiddenDirs: false,
	})

	// 빈 WorkDir은 허용 (기본 디렉토리 사용)
	if err := s.ValidateWorkDir(""); err != nil {
		t.Errorf("빈 WorkDir이 거부됨: %v", err)
	}

	// 허용 범위 외 WorkDir은 거부
	if err := s.ValidateWorkDir("/tmp/malicious"); err == nil {
		t.Error("허용 범위 외 WorkDir이 허용됨")
	}
}

func TestSandbox_DeniedOverridesAllowed(t *testing.T) {
	home := testHome(t)

	// 허용: ~/projects 전체
	// 거부: ~/projects/restricted
	s := NewSandbox(config.SandboxConfig{
		Enabled:        true,
		AllowedPaths:   []string{"~/projects"},
		DeniedPaths:    []string{"~/projects/restricted"},
		DenyHiddenDirs: false,
	})

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "허용된 일반 프로젝트",
			path:    filepath.Join(home, "projects", "my-app"),
			wantErr: false,
		},
		{
			name:    "거부가 허용을 오버라이드",
			path:    filepath.Join(home, "projects", "restricted", "secret"),
			wantErr: true,
		},
		{
			name:    "거부 경로 자체도 차단",
			path:    filepath.Join(home, "projects", "restricted"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := s.ValidatePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
		})
	}
}

func TestSandbox_TildeExpansion(t *testing.T) {
	home := testHome(t)

	s := NewSandbox(config.SandboxConfig{
		Enabled:        true,
		AllowedPaths:   []string{"~/projects"},
		DeniedPaths:    []string{},
		DenyHiddenDirs: false,
	})

	// 틸드 경로와 절대 경로 모두 동일하게 처리
	absPath := filepath.Join(home, "projects", "test")
	if err := s.ValidatePath(absPath); err != nil {
		t.Errorf("절대 경로 검증 실패: %v", err)
	}
}

func TestSandbox_PrefixMatchBoundary(t *testing.T) {
	home := testHome(t)

	s := NewSandbox(config.SandboxConfig{
		Enabled:        true,
		AllowedPaths:   []string{"~/projects"},
		DeniedPaths:    []string{},
		DenyHiddenDirs: false,
	})

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "정확한 접두사 매칭 - projects",
			path:    filepath.Join(home, "projects", "app"),
			wantErr: false,
		},
		{
			name:    "유사 접두사 거부 - projects-backup",
			path:    filepath.Join(home, "projects-backup", "app"),
			wantErr: true,
		},
		{
			name:    "유사 접두사 거부 - projectsX",
			path:    filepath.Join(home, "projectsX"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := s.ValidatePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
		})
	}
}

func TestSandbox_SymlinkResolution(t *testing.T) {
	// macOS에서 /var -> /private/var 심볼릭 링크 문제를 회피하기 위해
	// 홈 디렉토리 아래에 테스트 디렉토리 생성
	home := testHome(t)
	testBase := filepath.Join(home, ".sandbox-test-tmp")
	if err := os.MkdirAll(testBase, 0755); err != nil {
		t.Fatalf("테스트 디렉토리 생성 실패: %v", err)
	}
	defer func() { _ = os.RemoveAll(testBase) }()

	targetDir := filepath.Join(testBase, "actual-projects")
	symlinkDir := filepath.Join(testBase, "link-to-projects")
	secretDir := filepath.Join(testBase, "secret-data")

	// 디렉토리 생성
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatalf("디렉토리 생성 실패: %v", err)
	}
	if err := os.MkdirAll(secretDir, 0755); err != nil {
		t.Fatalf("디렉토리 생성 실패: %v", err)
	}

	// 심볼릭 링크 생성: link-to-projects -> actual-projects
	if err := os.Symlink(targetDir, symlinkDir); err != nil {
		t.Skipf("심볼릭 링크를 생성할 수 없습니다 (권한 부족): %v", err)
	}

	// 허용된 경로 외부를 가리키는 심볼릭 링크
	escapeLinkDir := filepath.Join(targetDir, "escape-link")
	if err := os.Symlink(secretDir, escapeLinkDir); err != nil {
		t.Skipf("심볼릭 링크를 생성할 수 없습니다: %v", err)
	}

	// 실제 경로를 해석하여 allowedPaths에 설정 (macOS /private 경로 등 고려)
	resolvedTarget, err := filepath.EvalSymlinks(targetDir)
	if err != nil {
		t.Fatalf("심볼릭 링크 해석 실패: %v", err)
	}

	s := NewSandbox(config.SandboxConfig{
		Enabled:        true,
		AllowedPaths:   []string{resolvedTarget},
		DeniedPaths:    []string{},
		DenyHiddenDirs: false,
	})

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "실제 경로 접근 허용",
			path:    targetDir,
			wantErr: false,
		},
		{
			name:    "심볼릭 링크를 통한 허용 경로 접근",
			path:    symlinkDir,
			wantErr: false,
		},
		{
			name:    "심볼릭 링크를 통한 샌드박스 탈출 차단",
			path:    escapeLinkDir,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := s.ValidatePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
		})
	}
}

func TestSandbox_ErrorMessages(t *testing.T) {
	home := testHome(t)

	s := NewSandbox(config.SandboxConfig{
		Enabled:        true,
		AllowedPaths:   []string{"~/projects"},
		DeniedPaths:    []string{"~/projects/restricted"},
		DenyHiddenDirs: true,
	})

	tests := []struct {
		name        string
		path        string
		errContains string
	}{
		{
			name:        "거부 경로 에러 메시지",
			path:        filepath.Join(home, "projects", "restricted", "file"),
			errContains: "보안 정책에 의해 차단",
		},
		{
			name:        "숨김 디렉토리 에러 메시지",
			path:        filepath.Join(home, "projects", ".hidden", "file"),
			errContains: "숨김 디렉토리",
		},
		{
			name:        "허용 범위 외 에러 메시지",
			path:        filepath.Join(home, "Desktop", "file"),
			errContains: "허용된 작업 디렉토리 범위를 벗어납니다",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := s.ValidatePath(tt.path)
			if err == nil {
				t.Fatalf("에러가 예상되었지만 nil 반환")
			}
			if !strings.Contains(err.Error(), tt.errContains) {
				t.Errorf("에러 메시지에 '%s'가 포함되지 않음: %v", tt.errContains, err)
			}
		})
	}
}

// ====================
// 헬퍼 함수 테스트
// ====================

func TestIsSubPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		basePath string
		expected bool
	}{
		{
			name:     "정확히 같은 경로",
			path:     "/home/user/projects",
			basePath: "/home/user/projects",
			expected: true,
		},
		{
			name:     "하위 경로",
			path:     "/home/user/projects/app",
			basePath: "/home/user/projects",
			expected: true,
		},
		{
			name:     "유사하지만 다른 경로",
			path:     "/home/user/projects-backup",
			basePath: "/home/user/projects",
			expected: false,
		},
		{
			name:     "상위 경로",
			path:     "/home/user",
			basePath: "/home/user/projects",
			expected: false,
		},
		{
			name:     "완전히 다른 경로",
			path:     "/tmp/data",
			basePath: "/home/user/projects",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isSubPath(tt.path, tt.basePath)
			if result != tt.expected {
				t.Errorf("isSubPath(%q, %q) = %v, want %v", tt.path, tt.basePath, result, tt.expected)
			}
		})
	}
}

func TestHasHiddenComponent(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		wantHidden bool
		wantComp   string
	}{
		{
			name:       "숨김 없음",
			path:       "/home/user/projects/app",
			wantHidden: false,
			wantComp:   "",
		},
		{
			name:       "숨김 디렉토리 포함",
			path:       "/home/user/.config/app",
			wantHidden: true,
			wantComp:   ".config",
		},
		{
			name:       "중첩 숨김 디렉토리",
			path:       "/home/user/projects/.hidden/deep",
			wantHidden: true,
			wantComp:   ".hidden",
		},
		{
			name:       "루트만",
			path:       "/",
			wantHidden: false,
			wantComp:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hidden, comp := hasHiddenComponent(tt.path)
			if hidden != tt.wantHidden {
				t.Errorf("hasHiddenComponent(%q) hidden = %v, want %v", tt.path, hidden, tt.wantHidden)
			}
			if comp != tt.wantComp {
				t.Errorf("hasHiddenComponent(%q) component = %q, want %q", tt.path, comp, tt.wantComp)
			}
		})
	}
}

func TestExpandTilde(t *testing.T) {
	home := testHome(t)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "틸드만",
			input:    "~",
			expected: home,
		},
		{
			name:     "틸드와 하위 경로",
			input:    "~/projects/app",
			expected: filepath.Join(home, "projects", "app"),
		},
		{
			name:     "절대 경로",
			input:    "/etc/config",
			expected: "/etc/config",
		},
		{
			name:     "빈 문자열",
			input:    "",
			expected: "",
		},
		{
			name:     "상대 경로",
			input:    "relative/path",
			expected: "relative/path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandTilde(tt.input)
			if result != tt.expected {
				t.Errorf("expandTilde(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestDeduplicate(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "중복 없음",
			input:    []string{"a", "b", "c"},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "중복 있음",
			input:    []string{"a", "b", "a", "c", "b"},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "빈 슬라이스",
			input:    []string{},
			expected: []string{},
		},
		{
			name:     "모두 같음",
			input:    []string{"x", "x", "x"},
			expected: []string{"x"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := deduplicate(tt.input)
			if len(result) != len(tt.expected) {
				t.Fatalf("deduplicate() 길이 = %d, want %d", len(result), len(tt.expected))
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("deduplicate()[%d] = %q, want %q", i, v, tt.expected[i])
				}
			}
		})
	}
}

// ====================
// TaskExecutor 샌드박스 통합 테스트
// ====================

func TestTaskExecutor_SandboxViolation(t *testing.T) {
	home := testHome(t)

	registry := provider.NewRegistry()
	registry.Register(&mockProvider{name: "claude"})
	sender := newMockSender()

	sandbox := NewSandbox(config.SandboxConfig{
		Enabled:        true,
		AllowedPaths:   []string{"~/projects"},
		DeniedPaths:    []string{},
		DenyHiddenDirs: true,
	})

	executor := NewTaskExecutor(registry, sender, WithSandbox(sandbox))

	ctx := context.Background()

	tests := []struct {
		name     string
		workDir  string
		wantErr  bool
		wantCode string
	}{
		{
			name:     "허용된 작업 디렉토리",
			workDir:  filepath.Join(home, "projects", "my-app"),
			wantErr:  false,
			wantCode: "",
		},
		{
			name:     "빈 작업 디렉토리 (허용)",
			workDir:  "",
			wantErr:  false,
			wantCode: "",
		},
		{
			name:     "거부된 작업 디렉토리",
			workDir:  "/etc/sensitive",
			wantErr:  true,
			wantCode: ErrorCodeSandboxViolationTask,
		},
		{
			name:     "숨김 디렉토리 차단",
			workDir:  filepath.Join(home, "projects", ".secret"),
			wantErr:  true,
			wantCode: ErrorCodeSandboxViolationTask,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := ws.TaskRequestPayload{
				ExecutionID: "sandbox-test",
				Prompt:      "test",
				Model:       "claude-sonnet",
				Timeout:     60,
				WorkDir:     tt.workDir,
			}

			_, err := executor.Execute(ctx, task)
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.wantCode != "" {
				var taskErr *TaskError
				if !errors.As(err, &taskErr) {
					t.Fatalf("TaskError가 아님: %v", err)
				}
				if taskErr.Code != tt.wantCode {
					t.Errorf("에러 코드 오류: got %s, want %s", taskErr.Code, tt.wantCode)
				}
				if taskErr.Retryable {
					t.Error("샌드박스 위반은 재시도 불가능해야 함")
				}
			}
		})
	}
}

func TestTaskExecutor_WithoutSandbox(t *testing.T) {
	// 샌드박스 없이 실행기 생성 시 모든 WorkDir 허용
	registry := provider.NewRegistry()
	registry.Register(&mockProvider{name: "claude"})
	sender := newMockSender()

	executor := NewTaskExecutor(registry, sender)

	ctx := context.Background()
	task := ws.TaskRequestPayload{
		ExecutionID: "no-sandbox-test",
		Prompt:      "test",
		Model:       "claude-sonnet",
		Timeout:     60,
		WorkDir:     "/etc/sensitive", // 일반적으로 거부되지만 샌드박스 없으면 허용
	}

	result, err := executor.Execute(ctx, task)
	if err != nil {
		t.Fatalf("샌드박스 없이 Execute 실패: %v", err)
	}
	if result.ExecutionID != task.ExecutionID {
		t.Errorf("ExecutionID 오류: got %s, want %s", result.ExecutionID, task.ExecutionID)
	}
}
