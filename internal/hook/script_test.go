package hook

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGenerateHookConfig는 설정 구조가 올바른지 검증합니다.
func TestGenerateHookConfig(t *testing.T) {
	t.Parallel()

	config := GenerateHookConfig(8080, "test-token")

	// hooks 키 존재 확인
	hooks, ok := config["hooks"]
	if !ok {
		t.Fatal("config에 'hooks' 키가 없습니다")
	}

	hooksMap, ok := hooks.(map[string]any)
	if !ok {
		t.Fatal("hooks가 map[string]any 타입이 아닙니다")
	}

	// PreToolUse 키 존재 확인
	preToolUse, ok := hooksMap["PreToolUse"]
	if !ok {
		t.Fatal("hooks에 'PreToolUse' 키가 없습니다")
	}

	// PreToolUse가 배열인지 확인
	preToolUseArr, ok := preToolUse.([]map[string]any)
	if !ok {
		t.Fatal("PreToolUse가 []map[string]any 타입이 아닙니다")
	}
	if len(preToolUseArr) == 0 {
		t.Fatal("PreToolUse 배열이 비어있습니다")
	}

	// matcher 확인
	matcher, ok := preToolUseArr[0]["matcher"]
	if !ok {
		t.Fatal("PreToolUse[0]에 'matcher' 키가 없습니다")
	}
	if matcher != "*" {
		t.Errorf("matcher = %q, want %q", matcher, "*")
	}

	// hooks 내부 배열 확인
	innerHooks, ok := preToolUseArr[0]["hooks"]
	if !ok {
		t.Fatal("PreToolUse[0]에 'hooks' 키가 없습니다")
	}
	innerArr, ok := innerHooks.([]map[string]any)
	if !ok || len(innerArr) == 0 {
		t.Fatal("inner hooks가 비어있거나 타입이 잘못되었습니다")
	}

	// type이 command인지 확인
	hookType, ok := innerArr[0]["type"]
	if !ok || hookType != "command" {
		t.Errorf("hook type = %v, want %q", hookType, "command")
	}
}

// TestGenerateHookConfig_ContainsPortAndToken은 command에 포트와 토큰이 포함되어 있는지 검증합니다.
func TestGenerateHookConfig_ContainsPortAndToken(t *testing.T) {
	t.Parallel()

	port := 9999
	token := "my-secret-token"
	config := GenerateHookConfig(port, token)

	// JSON으로 직렬화하여 포트와 토큰이 포함되어 있는지 확인
	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("JSON 직렬화 에러: %v", err)
	}

	configStr := string(data)
	if !strings.Contains(configStr, "9999") {
		t.Errorf("config에 포트 '9999'가 포함되어 있지 않습니다: %s", configStr)
	}
	if !strings.Contains(configStr, token) {
		t.Errorf("config에 토큰 '%s'가 포함되어 있지 않습니다: %s", token, configStr)
	}
}

// TestGenerateSessionDir는 디렉토리 생성과 settings.json 파일 존재를 검증합니다.
func TestGenerateSessionDir(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	sessionID := "test-session-001"
	port := 8080
	token := "test-token"

	sessionDir, cleanup, err := GenerateSessionDir(baseDir, sessionID, port, token)
	if err != nil {
		t.Fatalf("GenerateSessionDir 에러: %v", err)
	}
	defer cleanup()

	// sessionDir이 존재하는지 확인
	if _, statErr := os.Stat(sessionDir); os.IsNotExist(statErr) {
		t.Fatal("sessionDir이 존재하지 않습니다")
	}

	// .claude 디렉토리 존재 확인
	claudeDir := filepath.Join(sessionDir, ".claude")
	if _, statErr := os.Stat(claudeDir); os.IsNotExist(statErr) {
		t.Fatal(".claude 디렉토리가 존재하지 않습니다")
	}

	// settings.json 파일 존재 확인
	settingsPath := filepath.Join(claudeDir, "settings.json")
	if _, statErr := os.Stat(settingsPath); os.IsNotExist(statErr) {
		t.Fatal("settings.json 파일이 존재하지 않습니다")
	}

	// settings.json 내용이 유효한 JSON인지 확인
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("settings.json 읽기 에러: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("settings.json JSON 파싱 에러: %v", err)
	}

	// hooks 키가 존재하는지 확인
	if _, ok := parsed["hooks"]; !ok {
		t.Error("settings.json에 'hooks' 키가 없습니다")
	}
}

// TestGenerateSessionDir_Cleanup는 정리 함수가 디렉토리를 삭제하는지 검증합니다.
func TestGenerateSessionDir_Cleanup(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	sessionID := "test-cleanup-001"

	sessionDir, cleanup, err := GenerateSessionDir(baseDir, sessionID, 8080, "token")
	if err != nil {
		t.Fatalf("GenerateSessionDir 에러: %v", err)
	}

	// sessionDir 존재 확인
	if _, statErr := os.Stat(sessionDir); os.IsNotExist(statErr) {
		t.Fatal("cleanup 호출 전 sessionDir이 존재해야 합니다")
	}

	// 정리 실행
	cleanup()

	// sessionDir이 삭제되었는지 확인
	if _, statErr := os.Stat(sessionDir); !os.IsNotExist(statErr) {
		t.Error("cleanup 호출 후 sessionDir이 삭제되어야 합니다")
	}
}

// TestGenerateSessionDir_InvalidBaseDir는 잘못된 기본 디렉토리에서 에러를 반환하는지 검증합니다.
func TestGenerateSessionDir_InvalidBaseDir(t *testing.T) {
	t.Parallel()

	// 존재하지 않는 부모 경로 내에서 읽기 전용으로 만들 수 없으므로,
	// null 바이트가 포함된 경로를 사용하여 디렉토리 생성 실패 유도
	_, _, err := GenerateSessionDir("/dev/null/invalid", "test", 8080, "token")
	if err == nil {
		t.Error("잘못된 baseDir에서 에러가 반환되어야 합니다")
	}
}

// TestGenerateHookScript는 스크립트에 curl 명령과 올바른 URL이 포함되어 있는지 검증합니다.
func TestGenerateHookScript(t *testing.T) {
	t.Parallel()

	port := 7777
	token := "hook-script-token"
	script := GenerateHookScript(port, token)

	// curl 명령 포함 확인
	if !strings.Contains(script, "curl") {
		t.Error("스크립트에 'curl'이 포함되어 있지 않습니다")
	}

	// 올바른 URL 포함 확인
	expectedURL := "http://127.0.0.1:7777/hooks/pre-tool-use"
	if !strings.Contains(script, expectedURL) {
		t.Errorf("스크립트에 URL '%s'가 포함되어 있지 않습니다", expectedURL)
	}

	// 토큰 포함 확인
	if !strings.Contains(script, token) {
		t.Errorf("스크립트에 토큰 '%s'가 포함되어 있지 않습니다", token)
	}

	// X-Session-Token 헤더 포함 확인
	if !strings.Contains(script, "X-Session-Token") {
		t.Error("스크립트에 'X-Session-Token' 헤더가 포함되어 있지 않습니다")
	}

	// bash -c 시작 확인
	if !strings.HasPrefix(script, "bash -c") {
		t.Errorf("스크립트가 'bash -c'로 시작하지 않습니다: %s", script[:20])
	}

	// deny 시 exit 2 포함 확인
	if !strings.Contains(script, "exit 2") {
		t.Error("스크립트에 'exit 2' (deny 종료코드)가 포함되어 있지 않습니다")
	}

	// allow 시 exit 0 포함 확인
	if !strings.Contains(script, "exit 0") {
		t.Error("스크립트에 'exit 0' (allow 종료코드)가 포함되어 있지 않습니다")
	}
}
