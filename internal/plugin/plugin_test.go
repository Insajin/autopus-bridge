package plugin

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/insajin/autopus-agent-protocol"
	"github.com/rs/zerolog"
)

// --- 테스트용 목(mock) 플러그인 구현 ---

// mockPlugin은 기본 Plugin 인터페이스만 구현하는 목 플러그인입니다.
type mockPlugin struct {
	name    string
	version string
	closed  bool
}

func (m *mockPlugin) Name() string                             { return m.name }
func (m *mockPlugin) Version() string                          { return m.version }
func (m *mockPlugin) Init(config map[string]interface{}) error { return nil }
func (m *mockPlugin) Close() error                             { m.closed = true; return nil }

// mockProviderPlugin은 ProviderPlugin 인터페이스를 구현하는 목 플러그인입니다.
type mockProviderPlugin struct {
	mockPlugin
	supportedModel string
}

func (m *mockProviderPlugin) Execute(ctx context.Context, req ProviderRequest) (*ProviderResponse, error) {
	return &ProviderResponse{Output: "mock output", DurationMs: 100}, nil
}

func (m *mockProviderPlugin) Supports(model string) bool {
	return model == m.supportedModel
}

// mockTaskTypePlugin은 TaskTypePlugin 인터페이스를 구현하는 목 플러그인입니다.
type mockTaskTypePlugin struct {
	mockPlugin
	taskType string
}

func (m *mockTaskTypePlugin) TaskType() string { return m.taskType }

func (m *mockTaskTypePlugin) HandleTask(ctx context.Context, payload []byte) ([]byte, error) {
	return []byte(`{"status":"ok"}`), nil
}

// mockPostProcessorPlugin은 PostProcessorPlugin 인터페이스를 구현하는 목 플러그인입니다.
type mockPostProcessorPlugin struct {
	mockPlugin
	appliesTo string
}

func (m *mockPostProcessorPlugin) Process(ctx context.Context, result *ws.TaskResultPayload) (*ws.TaskResultPayload, error) {
	result.Output = "[processed] " + result.Output
	return result, nil
}

func (m *mockPostProcessorPlugin) Applies(taskType string) bool {
	return taskType == m.appliesTo
}

// --- Registry 테스트 ---

// TestNewRegistry는 새로운 레지스트리 생성을 테스트합니다.
func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	if r == nil {
		t.Fatal("NewRegistry()가 nil을 반환했습니다")
	}
	if r.Count() != 0 {
		t.Errorf("새 레지스트리의 플러그인 수가 0이 아닙니다: %d", r.Count())
	}
}

// TestRegisterBasePlugin은 기본 플러그인 등록을 테스트합니다.
func TestRegisterBasePlugin(t *testing.T) {
	r := NewRegistry()
	p := &mockPlugin{name: "test-plugin", version: "1.0.0"}

	err := r.Register(p)
	if err != nil {
		t.Fatalf("플러그인 등록 실패: %v", err)
	}

	if r.Count() != 1 {
		t.Errorf("등록 후 플러그인 수가 1이 아닙니다: %d", r.Count())
	}
}

// TestRegisterProviderPlugin은 프로바이더 플러그인 등록 및 조회를 테스트합니다.
func TestRegisterProviderPlugin(t *testing.T) {
	r := NewRegistry()
	p := &mockProviderPlugin{
		mockPlugin:     mockPlugin{name: "custom-llm", version: "2.0.0"},
		supportedModel: "custom-model-v1",
	}

	err := r.Register(p)
	if err != nil {
		t.Fatalf("프로바이더 플러그인 등록 실패: %v", err)
	}

	// 이름으로 조회
	got, ok := r.GetProvider("custom-llm")
	if !ok {
		t.Fatal("등록된 프로바이더 플러그인을 찾을 수 없습니다")
	}
	if got.Name() != "custom-llm" {
		t.Errorf("프로바이더 이름이 일치하지 않습니다: got %s, want custom-llm", got.Name())
	}
	if !got.Supports("custom-model-v1") {
		t.Error("프로바이더가 등록된 모델을 지원하지 않습니다")
	}
}

// TestGetProviderNotFound는 존재하지 않는 프로바이더 조회를 테스트합니다.
func TestGetProviderNotFound(t *testing.T) {
	r := NewRegistry()

	_, ok := r.GetProvider("nonexistent")
	if ok {
		t.Error("존재하지 않는 프로바이더가 반환되었습니다")
	}
}

// TestRegisterTaskTypePlugin은 작업 유형 플러그인 등록 및 라우팅을 테스트합니다.
func TestRegisterTaskTypePlugin(t *testing.T) {
	r := NewRegistry()
	p := &mockTaskTypePlugin{
		mockPlugin: mockPlugin{name: "deploy-handler", version: "1.0.0"},
		taskType:   "deploy_request",
	}

	err := r.Register(p)
	if err != nil {
		t.Fatalf("작업 유형 플러그인 등록 실패: %v", err)
	}

	// 메시지 유형으로 조회
	handler, ok := r.GetTaskHandler("deploy_request")
	if !ok {
		t.Fatal("등록된 작업 유형 핸들러를 찾을 수 없습니다")
	}
	if handler.Name() != "deploy-handler" {
		t.Errorf("핸들러 이름이 일치하지 않습니다: got %s, want deploy-handler", handler.Name())
	}

	// 존재하지 않는 메시지 유형 조회
	_, ok = r.GetTaskHandler("unknown_request")
	if ok {
		t.Error("존재하지 않는 메시지 유형에 대한 핸들러가 반환되었습니다")
	}
}

// TestRegisterPostProcessorPlugin은 후처리 플러그인 등록 및 필터링을 테스트합니다.
func TestRegisterPostProcessorPlugin(t *testing.T) {
	r := NewRegistry()

	// 두 개의 후처리 플러그인 등록 (적용 대상이 다름)
	pp1 := &mockPostProcessorPlugin{
		mockPlugin: mockPlugin{name: "logger-pp", version: "1.0.0"},
		appliesTo:  "task_request",
	}
	pp2 := &mockPostProcessorPlugin{
		mockPlugin: mockPlugin{name: "metrics-pp", version: "1.0.0"},
		appliesTo:  "build_request",
	}

	if err := r.Register(pp1); err != nil {
		t.Fatalf("후처리 플러그인 1 등록 실패: %v", err)
	}
	if err := r.Register(pp2); err != nil {
		t.Fatalf("후처리 플러그인 2 등록 실패: %v", err)
	}

	// task_request에 적용되는 후처리 플러그인만 조회
	processors := r.GetPostProcessors("task_request")
	if len(processors) != 1 {
		t.Fatalf("task_request에 적용되는 후처리 플러그인 수가 1이 아닙니다: %d", len(processors))
	}
	if processors[0].Name() != "logger-pp" {
		t.Errorf("후처리 플러그인 이름이 일치하지 않습니다: got %s, want logger-pp", processors[0].Name())
	}

	// 적용되는 플러그인이 없는 경우
	processors = r.GetPostProcessors("unknown_type")
	if len(processors) != 0 {
		t.Errorf("적용되는 후처리 플러그인이 없어야 하는데 %d개가 반환되었습니다", len(processors))
	}
}

// TestDuplicatePluginName은 중복 이름 등록 시 에러를 반환하는지 테스트합니다.
func TestDuplicatePluginName(t *testing.T) {
	r := NewRegistry()

	p1 := &mockPlugin{name: "duplicate", version: "1.0.0"}
	p2 := &mockPlugin{name: "duplicate", version: "2.0.0"}

	err := r.Register(p1)
	if err != nil {
		t.Fatalf("첫 번째 플러그인 등록 실패: %v", err)
	}

	err = r.Register(p2)
	if err == nil {
		t.Fatal("중복 이름으로 등록했는데 에러가 반환되지 않았습니다")
	}

	// 에러 타입 확인
	if !isErrDuplicatePlugin(err) {
		t.Errorf("예상된 ErrDuplicatePlugin 에러가 아닙니다: %v", err)
	}

	// 원래 플러그인이 보존되는지 확인
	if r.Count() != 1 {
		t.Errorf("중복 등록 시도 후 플러그인 수가 1이 아닙니다: %d", r.Count())
	}
}

// TestListPlugins는 플러그인 목록 조회를 테스트합니다.
func TestListPlugins(t *testing.T) {
	r := NewRegistry()

	_ = r.Register(&mockPlugin{name: "base-plugin", version: "1.0.0"})
	_ = r.Register(&mockProviderPlugin{
		mockPlugin:     mockPlugin{name: "provider-plugin", version: "2.0.0"},
		supportedModel: "test-model",
	})
	_ = r.Register(&mockTaskTypePlugin{
		mockPlugin: mockPlugin{name: "task-plugin", version: "3.0.0"},
		taskType:   "custom_task",
	})

	infos := r.ListPlugins()
	if len(infos) != 3 {
		t.Fatalf("플러그인 목록 길이가 3이 아닙니다: %d", len(infos))
	}

	// 유형별 확인
	typeMap := make(map[string]string)
	for _, info := range infos {
		typeMap[info.Name] = info.Type
	}

	if typeMap["base-plugin"] != "base" {
		t.Errorf("base-plugin의 유형이 base가 아닙니다: %s", typeMap["base-plugin"])
	}
	if typeMap["provider-plugin"] != "provider" {
		t.Errorf("provider-plugin의 유형이 provider가 아닙니다: %s", typeMap["provider-plugin"])
	}
	if typeMap["task-plugin"] != "task_type" {
		t.Errorf("task-plugin의 유형이 task_type이 아닙니다: %s", typeMap["task-plugin"])
	}
}

// TestRegistryClose는 모든 플러그인 닫기를 테스트합니다.
func TestRegistryClose(t *testing.T) {
	r := NewRegistry()

	p1 := &mockPlugin{name: "plugin-1", version: "1.0.0"}
	p2 := &mockPlugin{name: "plugin-2", version: "1.0.0"}

	_ = r.Register(p1)
	_ = r.Register(p2)

	err := r.Close()
	if err != nil {
		t.Fatalf("레지스트리 닫기 실패: %v", err)
	}

	// 플러그인이 닫혔는지 확인
	if !p1.closed {
		t.Error("plugin-1이 닫히지 않았습니다")
	}
	if !p2.closed {
		t.Error("plugin-2가 닫히지 않았습니다")
	}

	// 레지스트리가 비워졌는지 확인
	if r.Count() != 0 {
		t.Errorf("Close() 후 플러그인 수가 0이 아닙니다: %d", r.Count())
	}
}

// TestRegistryConcurrentAccess는 동시 접근 안전성을 테스트합니다.
func TestRegistryConcurrentAccess(t *testing.T) {
	r := NewRegistry()
	done := make(chan bool, 20)

	// 동시에 여러 고루틴에서 등록 및 조회
	for i := 0; i < 10; i++ {
		go func(n int) {
			p := &mockPlugin{name: "concurrent-" + string(rune('a'+n)), version: "1.0.0"}
			_ = r.Register(p)
			_ = r.ListPlugins()
			_ = r.Count()
			done <- true
		}(i)
	}

	// 동시에 여러 고루틴에서 조회
	for i := 0; i < 10; i++ {
		go func() {
			r.GetProvider("nonexistent")
			r.GetTaskHandler("nonexistent")
			r.GetPostProcessors("nonexistent")
			done <- true
		}()
	}

	// 모든 고루틴 완료 대기
	for i := 0; i < 20; i++ {
		<-done
	}

	// 패닉 없이 완료되면 성공
}

// TestPluginClassification은 플러그인 유형 분류를 테스트합니다.
func TestPluginClassification(t *testing.T) {
	tests := []struct {
		name     string
		plugin   Plugin
		wantType string
	}{
		{
			name:     "base plugin",
			plugin:   &mockPlugin{name: "base", version: "1.0.0"},
			wantType: "base",
		},
		{
			name:     "provider plugin",
			plugin:   &mockProviderPlugin{mockPlugin: mockPlugin{name: "provider", version: "1.0.0"}},
			wantType: "provider",
		},
		{
			name:     "task type plugin",
			plugin:   &mockTaskTypePlugin{mockPlugin: mockPlugin{name: "task", version: "1.0.0"}, taskType: "custom"},
			wantType: "task_type",
		},
		{
			name:     "post processor plugin",
			plugin:   &mockPostProcessorPlugin{mockPlugin: mockPlugin{name: "pp", version: "1.0.0"}, appliesTo: "all"},
			wantType: "post_processor",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyPlugin(tt.plugin)
			if got != tt.wantType {
				t.Errorf("classifyPlugin() = %s, want %s", got, tt.wantType)
			}
		})
	}
}

// --- Loader 테스트 ---

// TestNewLoader는 로더 생성을 테스트합니다.
func TestNewLoader(t *testing.T) {
	l := NewLoader()
	if l == nil {
		t.Fatal("NewLoader()가 nil을 반환했습니다")
	}
	if l.pluginsDir == "" {
		t.Log("기본 플러그인 디렉토리가 비어있습니다 (홈 디렉토리 접근 실패 가능)")
	}
}

// TestNewLoaderWithOptions는 옵션과 함께 로더 생성을 테스트합니다.
func TestNewLoaderWithOptions(t *testing.T) {
	customDir := "/tmp/test-plugins"
	logger := zerolog.Nop()

	l := NewLoader(
		WithPluginsDir(customDir),
		WithLogger(logger),
	)

	if l.pluginsDir != customDir {
		t.Errorf("플러그인 디렉토리가 설정되지 않았습니다: got %s, want %s", l.pluginsDir, customDir)
	}
}

// TestLoaderNonexistentDir은 존재하지 않는 디렉토리에서의 로딩을 테스트합니다.
func TestLoaderNonexistentDir(t *testing.T) {
	l := NewLoader(WithPluginsDir("/nonexistent/path/plugins"))

	plugins, err := l.DiscoverAndLoad()
	if err != nil {
		t.Fatalf("존재하지 않는 디렉토리에서 에러가 발생했습니다: %v", err)
	}
	if len(plugins) != 0 {
		t.Errorf("존재하지 않는 디렉토리에서 플러그인이 발견되었습니다: %d", len(plugins))
	}
}

// TestLoaderEmptyDir은 빈 디렉토리에서의 로딩을 테스트합니다.
func TestLoaderEmptyDir(t *testing.T) {
	// 임시 빈 디렉토리 생성
	tmpDir := t.TempDir()

	l := NewLoader(WithPluginsDir(tmpDir))

	plugins, err := l.DiscoverAndLoad()
	if err != nil {
		t.Fatalf("빈 디렉토리에서 에러가 발생했습니다: %v", err)
	}
	if len(plugins) != 0 {
		t.Errorf("빈 디렉토리에서 플러그인이 발견되었습니다: %d", len(plugins))
	}
}

// TestLoaderEmptyPluginsDir은 플러그인 디렉토리가 빈 문자열인 경우를 테스트합니다.
func TestLoaderEmptyPluginsDir(t *testing.T) {
	l := NewLoader(WithPluginsDir(""))

	plugins, err := l.DiscoverAndLoad()
	if err != nil {
		t.Fatalf("빈 디렉토리 경로에서 에러가 발생했습니다: %v", err)
	}
	if len(plugins) != 0 {
		t.Errorf("빈 디렉토리 경로에서 플러그인이 발견되었습니다: %d", len(plugins))
	}
}

// TestLoaderNotADirectory는 디렉토리가 아닌 경로를 테스트합니다.
func TestLoaderNotADirectory(t *testing.T) {
	// 임시 파일 생성
	tmpFile := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(tmpFile, []byte("test"), 0644); err != nil {
		t.Fatalf("임시 파일 생성 실패: %v", err)
	}

	l := NewLoader(WithPluginsDir(tmpFile))

	_, err := l.DiscoverAndLoad()
	if err == nil {
		t.Fatal("디렉토리가 아닌 경로에서 에러가 발생하지 않았습니다")
	}
}

// TestPluginFileExtension은 플랫폼별 확장자를 테스트합니다.
func TestPluginFileExtension(t *testing.T) {
	ext := pluginFileExtension()
	// 현재 테스트 환경에서 확장자가 비어있지 않은지 확인
	if ext == "" {
		t.Error("플러그인 파일 확장자가 비어있습니다")
	}
	// .so 또는 .dll 중 하나여야 함
	if ext != ".so" && ext != ".dll" {
		t.Errorf("예상하지 못한 파일 확장자: %s", ext)
	}
}

// TestDefaultPluginsDir는 기본 플러그인 디렉토리 경로를 테스트합니다.
func TestDefaultPluginsDir(t *testing.T) {
	dir := DefaultPluginsDir()
	// 홈 디렉토리를 가져올 수 있으면 경로가 비어있지 않아야 함
	if homeDir, err := os.UserHomeDir(); err == nil {
		expected := filepath.Join(homeDir, ".config", "autopus", "plugins")
		if dir != expected {
			t.Errorf("기본 플러그인 디렉토리가 일치하지 않습니다: got %s, want %s", dir, expected)
		}
	}
}

// TestProviderPluginExecute는 프로바이더 플러그인 실행을 테스트합니다.
func TestProviderPluginExecute(t *testing.T) {
	r := NewRegistry()
	p := &mockProviderPlugin{
		mockPlugin:     mockPlugin{name: "test-provider", version: "1.0.0"},
		supportedModel: "test-model",
	}

	_ = r.Register(p)

	provider, ok := r.GetProvider("test-provider")
	if !ok {
		t.Fatal("프로바이더를 찾을 수 없습니다")
	}

	resp, err := provider.Execute(context.Background(), ProviderRequest{
		Prompt:    "test prompt",
		Model:     "test-model",
		MaxTokens: 100,
	})
	if err != nil {
		t.Fatalf("프로바이더 실행 실패: %v", err)
	}
	if resp.Output != "mock output" {
		t.Errorf("출력이 일치하지 않습니다: got %s, want mock output", resp.Output)
	}
}

// TestPostProcessorProcess는 후처리 플러그인의 결과 처리를 테스트합니다.
func TestPostProcessorProcess(t *testing.T) {
	r := NewRegistry()
	pp := &mockPostProcessorPlugin{
		mockPlugin: mockPlugin{name: "test-pp", version: "1.0.0"},
		appliesTo:  "task_request",
	}

	_ = r.Register(pp)

	processors := r.GetPostProcessors("task_request")
	if len(processors) != 1 {
		t.Fatalf("후처리 플러그인 수가 1이 아닙니다: %d", len(processors))
	}

	result := &ws.TaskResultPayload{
		ExecutionID: "test-exec",
		Output:      "original output",
	}

	processed, err := processors[0].Process(context.Background(), result)
	if err != nil {
		t.Fatalf("후처리 실패: %v", err)
	}
	if processed.Output != "[processed] original output" {
		t.Errorf("후처리 결과가 일치하지 않습니다: got %s", processed.Output)
	}
}

// isErrDuplicatePlugin은 에러가 ErrDuplicatePlugin인지 확인하는 헬퍼입니다.
func isErrDuplicatePlugin(err error) bool {
	return err != nil && err.Error() != "" && contains(err.Error(), ErrDuplicatePlugin.Error())
}

// contains는 문자열 포함 여부를 확인하는 헬퍼입니다.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

// searchString은 문자열 검색 헬퍼입니다.
func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
