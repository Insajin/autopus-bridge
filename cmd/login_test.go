// Package cmd는 Bridge CLI 명령어의 정적 분석 테스트를 포함합니다.
// SPEC-BRIDGE-DEVAUTH-001 Milestone 5: 리팩토링 완료 후 코드 품질 검증
package cmd

import (
	"os"
	"strings"
	"testing"
)

// loginGoSourcePath는 테스트 대상 소스 파일 경로입니다.
const loginGoSourcePath = "login.go"

// readLoginGoSource는 login.go 소스 파일을 읽어서 반환합니다.
func readLoginGoSource(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(loginGoSourcePath)
	if err != nil {
		t.Fatalf("login.go 파일 읽기 실패: %v", err)
	}
	return string(data)
}

// TestAC017_NoBrowserAuthFlow는 performBrowserAuthFlow 함수가 제거되었는지 검증합니다.
// AC-017: Browser OAuth 콜백 코드가 완전히 제거되어야 합니다.
func TestAC017_NoBrowserAuthFlow(t *testing.T) {
	source := readLoginGoSource(t)

	// performBrowserAuthFlow 함수가 존재하지 않아야 함
	forbiddenPatterns := []struct {
		pattern string
		desc    string
	}{
		{"performBrowserAuthFlow", "Browser Auth Flow 함수"},
		{"func performBrowserAuth", "Browser Auth 함수 정의"},
	}

	for _, tt := range forbiddenPatterns {
		if strings.Contains(source, tt.pattern) {
			t.Errorf("login.go에 %s가 존재합니다 (패턴: %q). Browser Auth Flow는 제거되어야 합니다.",
				tt.desc, tt.pattern)
		}
	}
}

// TestAC018_NoTUIAuthSelector는 TUI 인증 선택기 관련 코드가 제거되었는지 검증합니다.
// AC-018: huh.NewSelect 등 TUI 선택기 코드가 없어야 합니다.
func TestAC018_NoTUIAuthSelector(t *testing.T) {
	source := readLoginGoSource(t)

	// TUI 인증 선택기 관련 패턴이 존재하지 않아야 함
	forbiddenPatterns := []struct {
		pattern string
		desc    string
	}{
		{"huh.NewSelect", "huh TUI 선택기"},
		{`"Browser login"`, "Browser login 옵션 문자열"},
		{`"Device code"`, "Device code 선택 옵션 문자열"},
		{"charmbracelet/huh", "huh 패키지 임포트"},
	}

	for _, tt := range forbiddenPatterns {
		if strings.Contains(source, tt.pattern) {
			t.Errorf("login.go에 %s가 존재합니다 (패턴: %q). TUI 선택기는 제거되어야 합니다.",
				tt.desc, tt.pattern)
		}
	}
}

// TestAC019_FileSizeLimit는 login.go 파일이 350줄 이하인지 검증합니다.
// AC-019: 리팩토링 후 파일 크기가 적절해야 합니다.
func TestAC019_FileSizeLimit(t *testing.T) {
	source := readLoginGoSource(t)

	const maxLines = 350
	lineCount := strings.Count(source, "\n") + 1

	if lineCount > maxLines {
		t.Errorf("login.go가 %d줄입니다. 최대 %d줄 이하여야 합니다.", lineCount, maxLines)
	}
}

// TestAC020_NoLocalhostCallbackServer는 localhost 콜백 서버 코드가 제거되었는지 검증합니다.
// AC-020: Browser OAuth의 localhost 콜백 서버가 완전히 제거되어야 합니다.
func TestAC020_NoLocalhostCallbackServer(t *testing.T) {
	source := readLoginGoSource(t)

	// localhost 콜백 서버 관련 패턴이 존재하지 않아야 함
	forbiddenPatterns := []struct {
		pattern string
		desc    string
	}{
		{"net.Listen", "net.Listen 콜백 서버"},
		{"http.Serve", "http.Serve 콜백 서버"},
		{"19280", "OAuth 콜백 포트 19280"},
		{"callbackServer", "콜백 서버 변수/함수"},
		{"startCallbackServer", "콜백 서버 시작 함수"},
		{"/callback", "OAuth 콜백 경로"},
	}

	for _, tt := range forbiddenPatterns {
		if strings.Contains(source, tt.pattern) {
			t.Errorf("login.go에 %s가 존재합니다 (패턴: %q). localhost 콜백 서버는 제거되어야 합니다.",
				tt.desc, tt.pattern)
		}
	}
}

// TestAC023_DeviceCodeFlagExists는 --device-code 플래그가 존재하는지 검증합니다.
// AC-023: 하위 호환성을 위해 --device-code 플래그가 유지되어야 합니다.
func TestAC023_DeviceCodeFlagExists(t *testing.T) {
	source := readLoginGoSource(t)

	// --device-code 플래그 등록 코드가 존재해야 함
	requiredPatterns := []struct {
		pattern string
		desc    string
	}{
		{"device-code", "--device-code 플래그 이름"},
		{"loginDeviceCodeOnly", "device-code 플래그 변수"},
	}

	for _, tt := range requiredPatterns {
		if !strings.Contains(source, tt.pattern) {
			t.Errorf("login.go에 %s가 존재하지 않습니다 (패턴: %q). --device-code 플래그는 유지되어야 합니다.",
				tt.desc, tt.pattern)
		}
	}
}
