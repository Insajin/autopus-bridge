package agentbrowser

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

func TestNewCommandExecutor(t *testing.T) {
	logger := noopLogger()
	ce := NewCommandExecutor(logger)
	if ce == nil {
		t.Fatal("NewCommandExecutor() returned nil")
	}
	if ce.commandFn == nil {
		t.Error("commandFn is nil; want non-nil")
	}
	if ce.binaryName != "agent-browser" {
		t.Errorf("binaryName = %q; want %q", ce.binaryName, "agent-browser")
	}
}

func TestCommandExecutor_Execute_Success(t *testing.T) {
	logger := noopLogger()
	ce := NewCommandExecutor(logger)

	// echo 명령을 모킹하여 성공적인 실행 시뮬레이션
	ce.commandFn = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "echo", "hello world")
	}

	result, err := ce.Execute(context.Background(), "get", "url")
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	if result.Output != "hello world" {
		t.Errorf("Output = %q; want %q", result.Output, "hello world")
	}
	if result.DurationMs < 0 {
		t.Errorf("DurationMs = %d; want >= 0", result.DurationMs)
	}
	if result.Error != "" {
		t.Errorf("Error = %q; want empty", result.Error)
	}
}

func TestCommandExecutor_Execute_Failure(t *testing.T) {
	logger := noopLogger()
	ce := NewCommandExecutor(logger)

	// 실패하는 명령 모킹
	ce.commandFn = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "false")
	}

	result, err := ce.Execute(context.Background(), "get", "url")
	if err == nil {
		t.Fatal("Execute() returned nil error; want error")
	}
	if !strings.Contains(err.Error(), "agent-browser 명령 실행 실패") {
		t.Errorf("error = %q; want containing 'agent-browser 명령 실행 실패'", err.Error())
	}
	if result == nil {
		t.Fatal("result is nil; want non-nil even on failure")
	}
	if result.DurationMs < 0 {
		t.Errorf("DurationMs = %d; want >= 0", result.DurationMs)
	}
}

func TestCommandExecutor_Execute_StderrOutput(t *testing.T) {
	logger := noopLogger()
	ce := NewCommandExecutor(logger)

	// stderr에 에러 메시지를 출력하고 실패하는 명령 모킹
	ce.commandFn = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		// sh -c를 사용하여 stderr 출력 후 실패
		return exec.CommandContext(ctx, "sh", "-c", "echo 'custom error' >&2; exit 1")
	}

	result, err := ce.Execute(context.Background(), "open", "https://example.com")
	if err == nil {
		t.Fatal("Execute() returned nil error; want error")
	}
	if result.Error != "custom error" {
		t.Errorf("Error = %q; want %q", result.Error, "custom error")
	}
}

func TestCommandExecutor_Execute_JSONOutput(t *testing.T) {
	logger := noopLogger()
	ce := NewCommandExecutor(logger)

	// JSON 출력을 반환하는 명령 모킹
	jsonOutput := `{"snapshot": "accessibility-tree-data", "output": "parsed output", "error": ""}`
	ce.commandFn = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "echo", jsonOutput)
	}

	result, err := ce.Execute(context.Background(), "get", "snapshot")
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	if result.Snapshot != "accessibility-tree-data" {
		t.Errorf("Snapshot = %q; want %q", result.Snapshot, "accessibility-tree-data")
	}
	if result.Output != "parsed output" {
		t.Errorf("Output = %q; want %q", result.Output, "parsed output")
	}
}

func TestCommandExecutor_Execute_JSONWithError(t *testing.T) {
	logger := noopLogger()
	ce := NewCommandExecutor(logger)

	// 에러를 포함한 JSON 출력 모킹
	jsonOutput := `{"error": "element not found", "output": ""}`
	ce.commandFn = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "echo", jsonOutput)
	}

	result, err := ce.Execute(context.Background(), "click", "@e42")
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	if result.Error != "element not found" {
		t.Errorf("Error = %q; want %q", result.Error, "element not found")
	}
}

func TestCommandExecutor_Execute_NonJSONOutput(t *testing.T) {
	logger := noopLogger()
	ce := NewCommandExecutor(logger)

	// JSON이 아닌 일반 텍스트 출력 모킹
	ce.commandFn = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "echo", "https://example.com/page")
	}

	result, err := ce.Execute(context.Background(), "get", "url")
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	if result.Output != "https://example.com/page" {
		t.Errorf("Output = %q; want %q", result.Output, "https://example.com/page")
	}
	// JSON이 아니므로 Snapshot은 비어 있어야 한다
	if result.Snapshot != "" {
		t.Errorf("Snapshot = %q; want empty for non-JSON output", result.Snapshot)
	}
}

func TestCommandExecutor_Execute_EmptyOutput(t *testing.T) {
	logger := noopLogger()
	ce := NewCommandExecutor(logger)

	// 빈 출력을 반환하는 명령 모킹
	ce.commandFn = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "echo", "-n", "")
	}

	result, err := ce.Execute(context.Background(), "click", "@e1")
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	if result.Snapshot != "" {
		t.Errorf("Snapshot = %q; want empty", result.Snapshot)
	}
}

func TestCommandExecutor_Execute_ContextCancelled(t *testing.T) {
	logger := noopLogger()
	ce := NewCommandExecutor(logger)

	// 실제 exec.CommandContext 사용 (취소된 컨텍스트로)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := ce.Execute(ctx, "get", "url")
	if err == nil {
		t.Fatal("Execute() returned nil error; want error for cancelled context")
	}
}

// --- BuildArgs 테스트 ---

func TestCommandExecutor_BuildArgs_OpenCommand(t *testing.T) {
	ce := NewCommandExecutor(noopLogger())

	payload := BrowserActionPayload{
		Command: "open",
		Params:  map[string]interface{}{"url": "https://example.com"},
	}

	args := ce.BuildArgs(payload)
	expected := []string{"open", "https://example.com"}
	assertSliceEqual(t, args, expected)
}

func TestCommandExecutor_BuildArgs_ClickWithRef(t *testing.T) {
	ce := NewCommandExecutor(noopLogger())

	ref := 42
	payload := BrowserActionPayload{
		Command: "click",
		Ref:     &ref,
	}

	args := ce.BuildArgs(payload)
	expected := []string{"click", "@e42"}
	assertSliceEqual(t, args, expected)
}

func TestCommandExecutor_BuildArgs_FillWithRefAndText(t *testing.T) {
	ce := NewCommandExecutor(noopLogger())

	ref := 15
	payload := BrowserActionPayload{
		Command: "fill",
		Ref:     &ref,
		Params:  map[string]interface{}{"text": "hello world"},
	}

	args := ce.BuildArgs(payload)
	expected := []string{"fill", "@e15", "hello world"}
	assertSliceEqual(t, args, expected)
}

func TestCommandExecutor_BuildArgs_PressKey(t *testing.T) {
	ce := NewCommandExecutor(noopLogger())

	payload := BrowserActionPayload{
		Command: "press",
		Params:  map[string]interface{}{"key": "Enter"},
	}

	args := ce.BuildArgs(payload)
	expected := []string{"press", "Enter"}
	assertSliceEqual(t, args, expected)
}

func TestCommandExecutor_BuildArgs_ScrollWithDirection(t *testing.T) {
	ce := NewCommandExecutor(noopLogger())

	payload := BrowserActionPayload{
		Command: "scroll",
		Params:  map[string]interface{}{"direction": "down", "amount": 3},
	}

	args := ce.BuildArgs(payload)
	// direction + amount
	if len(args) != 3 {
		t.Fatalf("len(args) = %d; want 3", len(args))
	}
	if args[0] != "scroll" {
		t.Errorf("args[0] = %q; want %q", args[0], "scroll")
	}
	if args[1] != "down" {
		t.Errorf("args[1] = %q; want %q", args[1], "down")
	}
}

func TestCommandExecutor_BuildArgs_GetSubCommand(t *testing.T) {
	ce := NewCommandExecutor(noopLogger())

	payload := BrowserActionPayload{
		Command: "get",
		Params:  map[string]interface{}{"type": "snapshot", "selector": "#main"},
	}

	args := ce.BuildArgs(payload)
	expected := []string{"get", "snapshot", "#main"}
	assertSliceEqual(t, args, expected)
}

func TestCommandExecutor_BuildArgs_WaitSelector(t *testing.T) {
	ce := NewCommandExecutor(noopLogger())

	payload := BrowserActionPayload{
		Command: "wait",
		Params:  map[string]interface{}{"selector": ".loading"},
	}

	args := ce.BuildArgs(payload)
	expected := []string{"wait", ".loading"}
	assertSliceEqual(t, args, expected)
}

func TestCommandExecutor_BuildArgs_SetViewport(t *testing.T) {
	ce := NewCommandExecutor(noopLogger())

	payload := BrowserActionPayload{
		Command: "set",
		Params:  map[string]interface{}{"type": "viewport", "width": 1920, "height": 1080},
	}

	args := ce.BuildArgs(payload)
	if len(args) < 4 {
		t.Fatalf("len(args) = %d; want >= 4", len(args))
	}
	if args[0] != "set" {
		t.Errorf("args[0] = %q; want %q", args[0], "set")
	}
	if args[1] != "viewport" {
		t.Errorf("args[1] = %q; want %q", args[1], "viewport")
	}
}

func TestCommandExecutor_BuildArgs_NavigateCommand(t *testing.T) {
	ce := NewCommandExecutor(noopLogger())

	payload := BrowserActionPayload{
		Command: "navigate",
		Params:  map[string]interface{}{"url": "https://example.com/page"},
	}

	args := ce.BuildArgs(payload)
	expected := []string{"navigate", "https://example.com/page"}
	assertSliceEqual(t, args, expected)
}

func TestCommandExecutor_BuildArgs_TypeCommand(t *testing.T) {
	ce := NewCommandExecutor(noopLogger())

	ref := 10
	payload := BrowserActionPayload{
		Command: "type",
		Ref:     &ref,
		Params:  map[string]interface{}{"text": "search query"},
	}

	args := ce.BuildArgs(payload)
	expected := []string{"type", "@e10", "search query"}
	assertSliceEqual(t, args, expected)
}

func TestCommandExecutor_BuildArgs_NoParams(t *testing.T) {
	ce := NewCommandExecutor(noopLogger())

	payload := BrowserActionPayload{
		Command: "screenshot",
	}

	args := ce.BuildArgs(payload)
	expected := []string{"screenshot"}
	assertSliceEqual(t, args, expected)
}

func TestCommandExecutor_BuildArgs_UnknownCommand(t *testing.T) {
	ce := NewCommandExecutor(noopLogger())

	payload := BrowserActionPayload{
		Command: "custom_command",
		Params:  map[string]interface{}{"foo": "bar"},
	}

	args := ce.BuildArgs(payload)
	// 알 수 없는 명령은 command 이름만 포함해야 한다
	expected := []string{"custom_command"}
	assertSliceEqual(t, args, expected)
}

// --- CI/CD 플래그 주입 테스트 ---

func TestCommandExecutor_BuildArgs_CICDHeadless(t *testing.T) {
	ce := NewCommandExecutor(noopLogger())
	ce.SetCICDConfig(CICDConfig{Headless: true})

	payload := BrowserActionPayload{
		Command: "open",
		Params:  map[string]interface{}{"url": "https://example.com"},
	}

	args := ce.BuildArgs(payload)
	expected := []string{"open", "--headless", "https://example.com"}
	assertSliceEqual(t, args, expected)
}

func TestCommandExecutor_BuildArgs_CICDAllFlags(t *testing.T) {
	ce := NewCommandExecutor(noopLogger())
	ce.SetCICDConfig(CICDConfig{
		Headless:   true,
		JSONOutput: true,
		NoColor:    true,
	})

	payload := BrowserActionPayload{
		Command: "screenshot",
	}

	args := ce.BuildArgs(payload)
	expected := []string{"screenshot", "--headless", "--json", "--no-color"}
	assertSliceEqual(t, args, expected)
}

func TestCommandExecutor_BuildArgs_CICDWithRef(t *testing.T) {
	ce := NewCommandExecutor(noopLogger())
	ce.SetCICDConfig(CICDConfig{Headless: true, JSONOutput: true})

	ref := 42
	payload := BrowserActionPayload{
		Command: "click",
		Ref:     &ref,
	}

	args := ce.BuildArgs(payload)
	expected := []string{"click", "--headless", "--json", "@e42"}
	assertSliceEqual(t, args, expected)
}

func TestCommandExecutor_BuildArgs_NoCICDFlags(t *testing.T) {
	ce := NewCommandExecutor(noopLogger())
	// CI/CD 설정 없음 (기본값)

	payload := BrowserActionPayload{
		Command: "open",
		Params:  map[string]interface{}{"url": "https://example.com"},
	}

	args := ce.BuildArgs(payload)
	expected := []string{"open", "https://example.com"}
	assertSliceEqual(t, args, expected)
}

func TestCommandExecutor_BuildArgs_CICDJSONOnly(t *testing.T) {
	ce := NewCommandExecutor(noopLogger())
	ce.SetCICDConfig(CICDConfig{JSONOutput: true})

	payload := BrowserActionPayload{
		Command: "get",
		Params:  map[string]interface{}{"type": "url"},
	}

	args := ce.BuildArgs(payload)
	expected := []string{"get", "--json", "url"}
	assertSliceEqual(t, args, expected)
}

func TestCommandExecutor_SetCICDConfig(t *testing.T) {
	ce := NewCommandExecutor(noopLogger())

	config := CICDConfig{
		Headless:   true,
		JSONOutput: true,
		NoColor:    true,
	}
	ce.SetCICDConfig(config)

	if !ce.cicdConfig.Headless {
		t.Error("cicdConfig.Headless = false; want true")
	}
	if !ce.cicdConfig.JSONOutput {
		t.Error("cicdConfig.JSONOutput = false; want true")
	}
	if !ce.cicdConfig.NoColor {
		t.Error("cicdConfig.NoColor = false; want true")
	}
}

// --- ExecuteFromPayload 테스트 ---

func TestCommandExecutor_ExecuteFromPayload(t *testing.T) {
	logger := noopLogger()
	ce := NewCommandExecutor(logger)

	// 명령 실행을 모킹
	var capturedArgs []string
	ce.commandFn = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		capturedArgs = args
		return exec.CommandContext(ctx, "echo", "ok")
	}

	ref := 5
	payload := BrowserActionPayload{
		Command: "click",
		Ref:     &ref,
	}

	result, err := ce.ExecuteFromPayload(context.Background(), payload)
	if err != nil {
		t.Fatalf("ExecuteFromPayload() returned error: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil; want non-nil")
	}

	// BuildArgs가 올바르게 호출되었는지 확인
	expectedArgs := []string{"click", "@e5"}
	assertSliceEqual(t, capturedArgs, expectedArgs)
}

// --- parseJSONOutput 테스트 ---

func TestParseJSONOutput_ValidJSON(t *testing.T) {
	ce := NewCommandExecutor(noopLogger())
	result := &CommandResult{}

	ce.parseJSONOutput(result, `{"snapshot": "tree-data", "output": "some output", "error": ""}`)

	if result.Snapshot != "tree-data" {
		t.Errorf("Snapshot = %q; want %q", result.Snapshot, "tree-data")
	}
	if result.Output != "some output" {
		t.Errorf("Output = %q; want %q", result.Output, "some output")
	}
}

func TestParseJSONOutput_InvalidJSON(t *testing.T) {
	ce := NewCommandExecutor(noopLogger())
	result := &CommandResult{Output: "original output"}

	ce.parseJSONOutput(result, "not json at all")

	// JSON 파싱 실패 시 기존 필드가 유지되어야 한다
	if result.Output != "original output" {
		t.Errorf("Output = %q; want %q (unchanged)", result.Output, "original output")
	}
}

func TestParseJSONOutput_EmptyString(t *testing.T) {
	ce := NewCommandExecutor(noopLogger())
	result := &CommandResult{Output: "original"}

	ce.parseJSONOutput(result, "")

	if result.Output != "original" {
		t.Errorf("Output = %q; want %q (unchanged)", result.Output, "original")
	}
}

func TestParseJSONOutput_ArrayJSON(t *testing.T) {
	ce := NewCommandExecutor(noopLogger())
	result := &CommandResult{Output: "original"}

	// 배열 JSON은 map으로 파싱할 수 없으므로 무시되어야 한다
	ce.parseJSONOutput(result, `[1, 2, 3]`)

	if result.Output != "original" {
		t.Errorf("Output = %q; want %q (unchanged)", result.Output, "original")
	}
}

func TestParseJSONOutput_WithWhitespace(t *testing.T) {
	ce := NewCommandExecutor(noopLogger())
	result := &CommandResult{}

	ce.parseJSONOutput(result, `  {"snapshot": "data"}  `)

	if result.Snapshot != "data" {
		t.Errorf("Snapshot = %q; want %q", result.Snapshot, "data")
	}
}

// --- 헬퍼 함수 ---

func assertSliceEqual(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("len = %d; want %d\n  got:  %v\n  want: %v", len(got), len(want), got, want)
		return
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("index %d: got %q; want %q\n  got:  %v\n  want: %v", i, got[i], want[i], got, want)
			return
		}
	}
}

// noopLogger는 테스트용 무출력 로거를 반환한다.
// 같은 패키지의 다른 테스트 파일에서도 사용된다.
func noopLogger() zerolog.Logger {
	return zerolog.Nop()
}
