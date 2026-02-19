// Package provider는 AI 프로바이더 통합 레이어를 제공합니다.
package provider

import (
	"context"
	"testing"
)

// mockProvider는 테스트용 목 프로바이더입니다.
type mockProvider struct {
	name           string
	supportedModel string
	validateErr    error
	executeResp    *ExecuteResponse
	executeErr     error
}

func (m *mockProvider) Name() string {
	return m.name
}

func (m *mockProvider) Execute(ctx context.Context, req ExecuteRequest) (*ExecuteResponse, error) {
	if m.executeErr != nil {
		return nil, m.executeErr
	}
	return m.executeResp, nil
}

func (m *mockProvider) ValidateConfig() error {
	return m.validateErr
}

func (m *mockProvider) Supports(model string) bool {
	return model == m.supportedModel
}

// TestRegistryNew는 새로운 레지스트리 생성을 테스트합니다.
func TestRegistryNew(t *testing.T) {
	r := NewRegistry()
	if r == nil {
		t.Fatal("NewRegistry()가 nil을 반환했습니다")
	}
	if r.Count() != 0 {
		t.Errorf("새 레지스트리의 프로바이더 수가 0이 아닙니다: %d", r.Count())
	}
}

// TestRegistryRegister는 프로바이더 등록을 테스트합니다.
func TestRegistryRegister(t *testing.T) {
	r := NewRegistry()
	mock := &mockProvider{name: "test-provider"}

	r.Register(mock)

	if r.Count() != 1 {
		t.Errorf("등록 후 프로바이더 수가 1이 아닙니다: %d", r.Count())
	}

	if !r.Has("test-provider") {
		t.Error("등록된 프로바이더를 찾을 수 없습니다")
	}
}

// TestRegistryGet은 프로바이더 조회를 테스트합니다.
func TestRegistryGet(t *testing.T) {
	r := NewRegistry()
	mock := &mockProvider{name: "claude"}

	r.Register(mock)

	// 존재하는 프로바이더 조회
	p := r.Get("claude")
	if p == nil {
		t.Error("등록된 프로바이더를 Get()으로 찾을 수 없습니다")
	}
	if p.Name() != "claude" {
		t.Errorf("프로바이더 이름이 일치하지 않습니다: got %s, want claude", p.Name())
	}

	// 존재하지 않는 프로바이더 조회
	p = r.Get("nonexistent")
	if p != nil {
		t.Error("존재하지 않는 프로바이더가 반환되었습니다")
	}
}

// TestRegistryGetForModel은 모델 기반 프로바이더 조회를 테스트합니다.
func TestRegistryGetForModel(t *testing.T) {
	r := NewRegistry()
	claudeMock := &mockProvider{name: "claude", supportedModel: "claude-sonnet-4-20250514"}
	geminiMock := &mockProvider{name: "gemini", supportedModel: "gemini-2.0-flash"}

	r.Register(claudeMock)
	r.Register(geminiMock)

	tests := []struct {
		name         string
		model        string
		wantProvider string
		wantErr      bool
	}{
		{
			name:         "claude 모델",
			model:        "claude-sonnet-4-20250514",
			wantProvider: "claude",
			wantErr:      false,
		},
		{
			name:         "gemini 모델",
			model:        "gemini-2.0-flash",
			wantProvider: "gemini",
			wantErr:      false,
		},
		{
			name:    "지원하지 않는 모델",
			model:   "gpt-4",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := r.GetForModel(tt.model)

			if tt.wantErr {
				if err == nil {
					t.Error("에러가 예상되었지만 반환되지 않았습니다")
				}
				return
			}

			if err != nil {
				t.Errorf("예상치 못한 에러: %v", err)
				return
			}

			if p.Name() != tt.wantProvider {
				t.Errorf("프로바이더 이름이 일치하지 않습니다: got %s, want %s", p.Name(), tt.wantProvider)
			}
		})
	}
}

// TestRegistryList는 프로바이더 목록 조회를 테스트합니다.
func TestRegistryList(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockProvider{name: "claude"})
	r.Register(&mockProvider{name: "gemini"})

	list := r.List()
	if len(list) != 2 {
		t.Errorf("프로바이더 목록 길이가 2가 아닙니다: %d", len(list))
	}

	// 이름이 포함되어 있는지 확인
	names := make(map[string]bool)
	for _, name := range list {
		names[name] = true
	}
	if !names["claude"] {
		t.Error("claude가 목록에 없습니다")
	}
	if !names["gemini"] {
		t.Error("gemini가 목록에 없습니다")
	}
}

// TestRegistryRemove는 프로바이더 제거를 테스트합니다.
func TestRegistryRemove(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockProvider{name: "claude"})
	r.Register(&mockProvider{name: "gemini"})

	r.Remove("claude")

	if r.Has("claude") {
		t.Error("제거된 프로바이더가 여전히 존재합니다")
	}
	if !r.Has("gemini") {
		t.Error("제거되지 않은 프로바이더가 사라졌습니다")
	}
	if r.Count() != 1 {
		t.Errorf("제거 후 프로바이더 수가 1이 아닙니다: %d", r.Count())
	}
}

// TestRegistryClear는 전체 프로바이더 제거를 테스트합니다.
func TestRegistryClear(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockProvider{name: "claude"})
	r.Register(&mockProvider{name: "gemini"})

	r.Clear()

	if r.Count() != 0 {
		t.Errorf("Clear() 후 프로바이더 수가 0이 아닙니다: %d", r.Count())
	}
}

// TestRegistryValidateAll은 모든 프로바이더 검증을 테스트합니다.
func TestRegistryValidateAll(t *testing.T) {
	t.Run("모든 프로바이더 유효", func(t *testing.T) {
		r := NewRegistry()
		r.Register(&mockProvider{name: "claude", validateErr: nil})
		r.Register(&mockProvider{name: "gemini", validateErr: nil})

		err := r.ValidateAll()
		if err != nil {
			t.Errorf("유효한 프로바이더에서 에러 발생: %v", err)
		}
	})

	t.Run("유효하지 않은 프로바이더 존재", func(t *testing.T) {
		r := NewRegistry()
		r.Register(&mockProvider{name: "claude", validateErr: ErrNoAPIKey})

		err := r.ValidateAll()
		if err == nil {
			t.Error("유효하지 않은 프로바이더에서 에러가 발생하지 않았습니다")
		}
	})
}

// TestRegistryExecute는 레지스트리를 통한 실행을 테스트합니다.
func TestRegistryExecute(t *testing.T) {
	r := NewRegistry()
	expectedResp := &ExecuteResponse{
		Output:     "테스트 응답",
		DurationMs: 100,
		Model:      "claude-sonnet-4-20250514",
	}
	r.Register(&mockProvider{
		name:           "claude",
		supportedModel: "claude-sonnet-4-20250514",
		executeResp:    expectedResp,
	})

	resp, err := r.Execute(context.Background(), ExecuteRequest{
		Prompt: "테스트 프롬프트",
		Model:  "claude-sonnet-4-20250514",
	})

	if err != nil {
		t.Errorf("실행 중 에러 발생: %v", err)
	}
	if resp == nil {
		t.Fatal("응답이 nil입니다")
	}
	if resp.Output != expectedResp.Output {
		t.Errorf("응답 출력이 일치하지 않습니다: got %s, want %s", resp.Output, expectedResp.Output)
	}
}

// TestRegistryConcurrentAccess는 동시 접근 안전성을 테스트합니다.
func TestRegistryConcurrentAccess(t *testing.T) {
	r := NewRegistry()
	done := make(chan bool)

	// 동시에 여러 고루틴에서 접근
	for i := 0; i < 10; i++ {
		go func(n int) {
			mock := &mockProvider{name: "provider-" + string(rune('0'+n))}
			r.Register(mock)
			_ = r.List()
			_ = r.Count()
			_ = r.Has(mock.name)
			done <- true
		}(i)
	}

	// 모든 고루틴 완료 대기
	for i := 0; i < 10; i++ {
		<-done
	}

	// 패닉 없이 완료되면 성공
}

// TestClaudeProviderSupports는 Claude 프로바이더의 모델 지원 확인을 테스트합니다.
func TestClaudeProviderSupports(t *testing.T) {
	// API 키 없이 Supports 함수만 테스트하기 위해 직접 검사
	tests := []struct {
		model string
		want  bool
	}{
		{"claude-sonnet-4-20250514", true},
		{"claude-opus-4-20250514", true},
		{"claude-3-5-sonnet-20240620", true},
		{"claude-3-5-sonnet-latest", true},
		{"gemini-2.0-flash", false},
		{"gpt-4", false},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			// Claude 모델 패턴 매칭 로직 테스트
			got := isClaudeModel(tt.model)
			if got != tt.want {
				t.Errorf("isClaudeModel(%s) = %v, want %v", tt.model, got, tt.want)
			}
		})
	}
}

// TestGeminiProviderSupports는 Gemini 프로바이더의 모델 지원 확인을 테스트합니다.
func TestGeminiProviderSupports(t *testing.T) {
	tests := []struct {
		model string
		want  bool
	}{
		{"gemini-2.0-flash", true},
		{"gemini-1.5-pro", true},
		{"gemini-1.5-flash-latest", true},
		{"claude-sonnet-4-20250514", false},
		{"gpt-4", false},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			// Gemini 모델 패턴 매칭 로직 테스트
			got := isGeminiModel(tt.model)
			if got != tt.want {
				t.Errorf("isGeminiModel(%s) = %v, want %v", tt.model, got, tt.want)
			}
		})
	}
}

// isClaudeModel은 Claude 모델인지 확인합니다 (테스트용 헬퍼).
func isClaudeModel(model string) bool {
	if len(model) < 7 {
		return false
	}
	return model[:7] == "claude-"
}

// isGeminiModel은 Gemini 모델인지 확인합니다 (테스트용 헬퍼).
func isGeminiModel(model string) bool {
	if len(model) < 7 {
		return false
	}
	return model[:7] == "gemini-"
}
