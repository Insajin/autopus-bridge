// Package authwatch는 AI 프로바이더 인증 파일 변경을 감지하고
// capabilities 업데이트를 트리거하는 파일 시스템 감시자를 제공합니다.
// SPEC-HOTSWAP-001: fsnotify 기반 실시간 인증 파일 감지
package authwatch

import (
	"context"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

// safeWriter는 race detector 통과를 위한 뮤텍스 기반 안전한 io.Writer입니다.
type safeWriter struct {
	mu  *sync.Mutex
	buf *strings.Builder
}

func (sw *safeWriter) Write(p []byte) (n int, err error) {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	return sw.buf.Write(p)
}

// mockProvider는 테스트용 프로바이더 구현입니다.
// valid 필드는 뮤텍스로 보호됩니다 (race detector를 통과하기 위해).
type mockProvider struct {
	name         string
	mu           sync.Mutex
	valid        bool
	authFilePath string // 감시할 인증 파일 경로 (디렉토리 추출용)
}

func (m *mockProvider) Name() string { return m.name }
func (m *mockProvider) ValidateConfig() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.valid {
		return errors.New("API 키가 설정되지 않았습니다")
	}
	return nil
}
func (m *mockProvider) SetValid(v bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.valid = v
}
func (m *mockProvider) AuthFilePath() string { return m.authFilePath }

// noopLogger는 테스트에서 로그 출력을 억제합니다.
var noopLogger = zerolog.Nop()

// TestNew_SkipsNonExistentDirs는 존재하지 않는 디렉토리를 건너뛰는지 검증합니다.
// REQ-E-005: 존재하지 않는 디렉토리는 조용히 건너뜀
func TestNew_SkipsNonExistentDirs(t *testing.T) {
	t.Parallel()

	// 존재하지 않는 경로를 가진 프로바이더
	p := &mockProvider{
		name:         "test",
		valid:        true,
		authFilePath: "/nonexistent/path/that/does/not/exist/credentials.json",
	}

	onChange := func(caps map[string]bool) {}
	w, err := New([]Provider{p}, onChange, WithLogger(noopLogger))

	// 에러 없이 생성되어야 합니다 (존재하지 않는 디렉토리는 건너뜀)
	if err != nil {
		t.Fatalf("New()가 에러를 반환했습니다: %v", err)
	}
	if w == nil {
		t.Fatal("New()가 nil을 반환했습니다")
	}
	defer w.Stop()
}

// TestNew_WatchesExistingDirs는 존재하는 디렉토리를 감시하는지 검증합니다.
func TestNew_WatchesExistingDirs(t *testing.T) {
	t.Parallel()

	// 임시 디렉토리 생성
	tmpDir := t.TempDir()
	credFile := filepath.Join(tmpDir, "credentials.json")

	p := &mockProvider{
		name:         "test",
		valid:        true,
		authFilePath: credFile,
	}

	onChange := func(caps map[string]bool) {}
	w, err := New([]Provider{p}, onChange, WithLogger(noopLogger))
	if err != nil {
		t.Fatalf("New()가 에러를 반환했습니다: %v", err)
	}
	if w == nil {
		t.Fatal("New()가 nil을 반환했습니다")
	}
	defer w.Stop()
}

// TestDebounce_SingleCallback은 파일 변경이 1회 콜백으로 디바운싱되는지 검증합니다.
// REQ-E-001: 1초 디바운스
func TestDebounce_SingleCallback(t *testing.T) {
	tmpDir := t.TempDir()
	credFile := filepath.Join(tmpDir, "credentials.json")

	// 인증 파일 초기 생성 (프로바이더가 유효한 상태)
	if err := os.WriteFile(credFile, []byte(`{"key": "value"}`), 0600); err != nil {
		t.Fatal(err)
	}

	callCount := 0
	var mu sync.Mutex
	onChange := func(caps map[string]bool) {
		mu.Lock()
		callCount++
		mu.Unlock()
	}

	p := &mockProvider{
		name:         "test",
		valid:        true,
		authFilePath: credFile,
	}

	w, err := New([]Provider{p}, onChange,
		WithLogger(noopLogger),
		WithDebounce(200*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("New()가 에러를 반환했습니다: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := w.Start(ctx); err != nil {
		t.Fatalf("Start()가 에러를 반환했습니다: %v", err)
	}
	defer w.Stop()

	// 파일 변경
	if err := os.WriteFile(credFile, []byte(`{"key": "updated"}`), 0600); err != nil {
		t.Fatal(err)
	}

	// 디바운스 + 처리 시간 대기
	time.Sleep(600 * time.Millisecond)

	mu.Lock()
	count := callCount
	mu.Unlock()

	// 콜백이 1회 호출되어야 합니다
	if count < 1 {
		t.Errorf("콜백이 최소 1회 호출되어야 하나 %d회 호출됨", count)
	}
}

// TestDebounce_RapidChanges는 빠른 연속 변경이 1번의 콜백으로 합쳐지는지 검증합니다.
// REQ-E-001: 디바운스 - 타이머 리셋
func TestDebounce_RapidChanges(t *testing.T) {
	tmpDir := t.TempDir()
	credFile := filepath.Join(tmpDir, "credentials.json")

	// 초기 파일 생성
	if err := os.WriteFile(credFile, []byte(`{"key": "initial"}`), 0600); err != nil {
		t.Fatal(err)
	}

	callCount := 0
	var mu sync.Mutex
	onChange := func(caps map[string]bool) {
		mu.Lock()
		callCount++
		mu.Unlock()
	}

	p := &mockProvider{
		name:         "test",
		valid:        true,
		authFilePath: credFile,
	}

	w, err := New([]Provider{p}, onChange,
		WithLogger(noopLogger),
		WithDebounce(300*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("New()가 에러를 반환했습니다: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := w.Start(ctx); err != nil {
		t.Fatalf("Start()가 에러를 반환했습니다: %v", err)
	}
	defer w.Stop()

	// 0.1초 간격으로 3번 변경
	for i := 0; i < 3; i++ {
		if err := os.WriteFile(credFile, []byte(`{"key": "change"}`), 0600); err != nil {
			t.Fatal(err)
		}
		time.Sleep(100 * time.Millisecond)
	}

	// 디바운스 만료까지 대기
	time.Sleep(800 * time.Millisecond)

	mu.Lock()
	count := callCount
	mu.Unlock()

	// 3번 변경이 1-2번의 콜백으로 합쳐져야 합니다
	if count > 2 {
		t.Errorf("빠른 변경이 디바운싱되어야 하나 %d번의 콜백 발생", count)
	}
}

// TestFileCreate_ActivatesProvider는 파일 생성 시 프로바이더가 활성화되는지 검증합니다.
func TestFileCreate_ActivatesProvider(t *testing.T) {
	tmpDir := t.TempDir()
	credFile := filepath.Join(tmpDir, "credentials.json")

	// 초기에는 파일이 없어서 invalid
	received := make(chan map[string]bool, 5)
	onChange := func(caps map[string]bool) {
		copyMap := make(map[string]bool, len(caps))
		for k, v := range caps {
			copyMap[k] = v
		}
		select {
		case received <- copyMap:
		default:
		}
	}

	p := &mockProvider{
		name:         "claude",
		authFilePath: credFile,
	}
	// 초기에는 파일이 없어서 invalid
	p.SetValid(false)

	w, err := New([]Provider{p}, onChange,
		WithLogger(noopLogger),
		WithDebounce(100*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("New()가 에러를 반환했습니다: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := w.Start(ctx); err != nil {
		t.Fatalf("Start()가 에러를 반환했습니다: %v", err)
	}
	defer w.Stop()

	// 파일 생성 - 프로바이더를 valid로 변경
	p.SetValid(true)
	if err := os.WriteFile(credFile, []byte(`{"key": "value"}`), 0600); err != nil {
		t.Fatal(err)
	}

	// 콜백 대기
	select {
	case caps := <-received:
		if !caps["claude"] {
			t.Errorf("claude 프로바이더가 활성화되어야 하나 caps=%v", caps)
		}
	case <-time.After(2 * time.Second):
		t.Error("파일 생성 후 콜백이 호출되지 않았습니다")
	}
}

// TestFileDelete_DeactivatesProvider는 파일 삭제 시 프로바이더가 비활성화되는지 검증합니다.
func TestFileDelete_DeactivatesProvider(t *testing.T) {
	tmpDir := t.TempDir()
	credFile := filepath.Join(tmpDir, "credentials.json")

	// 초기에 파일 생성
	if err := os.WriteFile(credFile, []byte(`{"key": "value"}`), 0600); err != nil {
		t.Fatal(err)
	}

	received := make(chan map[string]bool, 5)
	onChange := func(caps map[string]bool) {
		copyMap := make(map[string]bool, len(caps))
		for k, v := range caps {
			copyMap[k] = v
		}
		select {
		case received <- copyMap:
		default:
		}
	}

	p := &mockProvider{
		name:         "claude",
		authFilePath: credFile,
	}
	p.SetValid(true)

	w, err := New([]Provider{p}, onChange,
		WithLogger(noopLogger),
		WithDebounce(100*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("New()가 에러를 반환했습니다: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := w.Start(ctx); err != nil {
		t.Fatalf("Start()가 에러를 반환했습니다: %v", err)
	}
	defer w.Stop()

	// 파일 삭제 - 프로바이더를 invalid로 변경
	p.SetValid(false)
	if err := os.Remove(credFile); err != nil {
		t.Fatal(err)
	}

	// 콜백 대기
	select {
	case caps := <-received:
		if caps["claude"] {
			t.Errorf("claude 프로바이더가 비활성화되어야 하나 caps=%v", caps)
		}
	case <-time.After(2 * time.Second):
		t.Error("파일 삭제 후 콜백이 호출되지 않았습니다")
	}
}

// TestFilter_IgnoresUnwatchedFiles는 화이트리스트 외 파일은 무시하는지 검증합니다.
// REQ-N-003: 화이트리스트 기반 파일 필터링
func TestFilter_IgnoresUnwatchedFiles(t *testing.T) {
	tmpDir := t.TempDir()
	credFile := filepath.Join(tmpDir, "credentials.json")

	// 초기 파일 생성
	if err := os.WriteFile(credFile, []byte(`{"key": "value"}`), 0600); err != nil {
		t.Fatal(err)
	}

	callCount := 0
	var mu sync.Mutex
	onChange := func(caps map[string]bool) {
		mu.Lock()
		callCount++
		mu.Unlock()
	}

	p := &mockProvider{
		name:         "test",
		valid:        true,
		authFilePath: credFile,
	}

	w, err := New([]Provider{p}, onChange,
		WithLogger(noopLogger),
		WithDebounce(100*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("New()가 에러를 반환했습니다: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := w.Start(ctx); err != nil {
		t.Fatalf("Start()가 에러를 반환했습니다: %v", err)
	}
	defer w.Stop()

	// 화이트리스트에 없는 파일 변경
	unwatchedFile := filepath.Join(tmpDir, "some_other_file.txt")
	if err := os.WriteFile(unwatchedFile, []byte("data"), 0600); err != nil {
		t.Fatal(err)
	}

	// 충분히 기다림
	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	count := callCount
	mu.Unlock()

	// 화이트리스트 외 파일은 콜백을 트리거하지 않아야 합니다
	if count > 0 {
		t.Errorf("화이트리스트 외 파일 변경으로 콜백이 %d번 호출됨", count)
	}
}

// TestRebuildCapabilities_PartialFailure는 일부 프로바이더 실패 시 나머지가 작동하는지 검증합니다.
func TestRebuildCapabilities_PartialFailure(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	credFile := filepath.Join(tmpDir, "credentials.json")
	if err := os.WriteFile(credFile, []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}

	claudeP := &mockProvider{name: "claude", authFilePath: credFile}
	claudeP.SetValid(true)
	geminiP := &mockProvider{name: "gemini", authFilePath: credFile}
	geminiP.SetValid(false)
	codexP := &mockProvider{name: "codex", authFilePath: credFile}
	codexP.SetValid(true)
	providers := []Provider{claudeP, geminiP, codexP}

	onChange := func(caps map[string]bool) {}
	w, err := New(providers, onChange, WithLogger(noopLogger))
	if err != nil {
		t.Fatalf("New()가 에러를 반환했습니다: %v", err)
	}
	defer w.Stop()

	caps := w.rebuildCapabilities()

	if !caps["claude"] {
		t.Error("claude는 활성화되어야 합니다")
	}
	if caps["gemini"] {
		t.Error("gemini는 비활성화되어야 합니다")
	}
	if !caps["codex"] {
		t.Error("codex는 활성화되어야 합니다")
	}
}

// TestStop_CleansUp는 Stop()이 정리를 수행하는지 검증합니다.
func TestStop_CleansUp(t *testing.T) {
	t.Parallel()

	p := &mockProvider{
		name:         "test",
		valid:        true,
		authFilePath: "/nonexistent/path/creds.json",
	}

	onChange := func(caps map[string]bool) {}
	w, err := New([]Provider{p}, onChange, WithLogger(noopLogger))
	if err != nil {
		t.Fatalf("New()가 에러를 반환했습니다: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := w.Start(ctx); err != nil {
		t.Fatalf("Start()가 에러를 반환했습니다: %v", err)
	}

	// Stop이 패닉 없이 완료되어야 합니다
	w.Stop()

	// 중복 Stop도 안전해야 합니다
	w.Stop()
}

// TestLogNoCredentials는 로그에 인증 정보가 포함되지 않는지 검증합니다.
// SPEC-HOTSWAP-001: 인증 값은 절대 로그에 기록하지 않음
func TestLogNoCredentials(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	credFile := filepath.Join(tmpDir, "credentials.json")
	sensitiveData := `{"api_key": "SECRET_API_KEY_12345", "token": "VERY_SECRET_TOKEN"}`
	if err := os.WriteFile(credFile, []byte(sensitiveData), 0600); err != nil {
		t.Fatal(err)
	}

	// 로그 버퍼 (뮤텍스로 race 방지)
	var logMu sync.Mutex
	var logBuf strings.Builder
	safeWriter := &safeWriter{mu: &logMu, buf: &logBuf}
	logger := zerolog.New(safeWriter)

	p := &mockProvider{
		name:         "test",
		authFilePath: credFile,
	}
	p.SetValid(true)

	onChange := func(caps map[string]bool) {}
	w, err := New([]Provider{p}, onChange, WithLogger(logger))
	if err != nil {
		t.Fatalf("New()가 에러를 반환했습니다: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := w.Start(ctx); err != nil {
		t.Fatalf("Start()가 에러를 반환했습니다: %v", err)
	}
	defer w.Stop()

	// 파일 변경 트리거
	if err := os.WriteFile(credFile, []byte(`{"api_key": "NEW_SECRET"}`), 0600); err != nil {
		t.Fatal(err)
	}

	time.Sleep(500 * time.Millisecond)

	// 로그에 민감한 데이터가 없어야 합니다
	logMu.Lock()
	logOutput := logBuf.String()
	logMu.Unlock()
	if strings.Contains(logOutput, "SECRET_API_KEY_12345") {
		t.Error("로그에 민감한 API 키가 포함되어 있습니다")
	}
	if strings.Contains(logOutput, "VERY_SECRET_TOKEN") {
		t.Error("로그에 민감한 토큰이 포함되어 있습니다")
	}
	if strings.Contains(logOutput, "NEW_SECRET") {
		t.Error("로그에 새로운 민감한 데이터가 포함되어 있습니다")
	}
}

// TestMapsEqual은 maps.Equal을 사용한 capability 비교 로직을 검증합니다.
func TestMapsEqual(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		a, b map[string]bool
		want bool
	}{
		{"both nil", nil, nil, true},
		{"both empty", map[string]bool{}, map[string]bool{}, true},
		{"same", map[string]bool{"claude": true, "gemini": false}, map[string]bool{"claude": true, "gemini": false}, true},
		{"different length", map[string]bool{"claude": true}, map[string]bool{"claude": true, "gemini": false}, false},
		{"different value", map[string]bool{"claude": true}, map[string]bool{"claude": false}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maps.Equal(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("maps.Equal(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

// TestWithWatchDirs는 추가 감시 디렉토리 옵션을 검증합니다.
func TestWithWatchDirs(t *testing.T) {
	t.Parallel()

	tmpDir1 := t.TempDir()
	tmpDir2 := t.TempDir()

	p := &mockProvider{
		name:         "test",
		valid:        true,
		authFilePath: filepath.Join(tmpDir1, "credentials.json"),
	}

	onChange := func(caps map[string]bool) {}
	w, err := New([]Provider{p}, onChange,
		WithLogger(noopLogger),
		WithWatchDirs(tmpDir2),
	)
	if err != nil {
		t.Fatalf("New()가 에러를 반환했습니다: %v", err)
	}
	defer w.Stop()

	// extraDirs가 설정되었는지 확인
	if len(w.extraDirs) != 1 || w.extraDirs[0] != tmpDir2 {
		t.Errorf("extraDirs가 %v이어야 하나 %v", []string{tmpDir2}, w.extraDirs)
	}
}

// TestEventLoop_ContextCancel은 ctx 취소 시 이벤트 루프가 종료되는지 검증합니다.
func TestEventLoop_ContextCancel(t *testing.T) {
	tmpDir := t.TempDir()
	credFile := filepath.Join(tmpDir, "credentials.json")
	if err := os.WriteFile(credFile, []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}

	p := &mockProvider{name: "test", valid: true, authFilePath: credFile}
	onChange := func(caps map[string]bool) {}

	w, err := New([]Provider{p}, onChange, WithLogger(noopLogger))
	if err != nil {
		t.Fatalf("New()가 에러를 반환했습니다: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	if err := w.Start(ctx); err != nil {
		t.Fatalf("Start()가 에러를 반환했습니다: %v", err)
	}

	// ctx 취소로 이벤트 루프가 종료되어야 합니다
	cancel()
	time.Sleep(100 * time.Millisecond)

	// Stop이 패닉 없이 완료되어야 합니다
	w.Stop()
}

// TestEmptyAuthFilePath는 AuthFilePath가 빈 문자열인 프로바이더를 건너뛰는지 검증합니다.
func TestEmptyAuthFilePath(t *testing.T) {
	t.Parallel()

	p := &mockProvider{
		name:         "nopath",
		valid:        true,
		authFilePath: "", // 빈 경로
	}

	onChange := func(caps map[string]bool) {}
	w, err := New([]Provider{p}, onChange, WithLogger(noopLogger))
	if err != nil {
		t.Fatalf("New()가 에러를 반환했습니다: %v", err)
	}
	defer w.Stop()
}
