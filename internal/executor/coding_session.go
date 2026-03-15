// Package executor - 코딩 세션 인터페이스 및 타입 정의
package executor

import "context"

// CodingSession은 코딩 에이전트(Claude Code, Codex, Gemini CLI, OpenCode)와의 상호작용을 추상화합니다.
// @MX:ANCHOR: [AUTO] CodingSession 릴레이 아키텍처의 핵심 인터페이스 (fan_in >= 3)
// @MX:REASON: ClaudeCodeSession, CodexSession, APIFallbackSession 및 CodingSessionManager에서 구현/사용
type CodingSession interface {
	// Open은 코딩 세션을 엽니다.
	Open(ctx context.Context, req CodingSessionOpenRequest) error
	// Send는 메시지를 전송하고 응답을 반환합니다.
	Send(ctx context.Context, message string) (*CodingSessionResponse, error)
	// SessionID는 현재 세션 ID를 반환합니다.
	SessionID() string
	// Close는 세션을 종료하고 리소스를 해제합니다.
	Close(ctx context.Context) error
}

// CodingSessionOpenRequest는 코딩 세션 시작 요청입니다.
type CodingSessionOpenRequest struct {
	// WorkDir은 작업 디렉토리입니다.
	WorkDir string
	// Model은 사용할 AI 모델입니다.
	Model string
	// MaxBudgetUSD는 최대 예산(USD)입니다.
	MaxBudgetUSD float64
	// Tools는 사용 가능한 도구 목록입니다.
	Tools []string
	// AllowedTools는 허용된 도구 목록입니다.
	AllowedTools []string
	// ResumeSession은 재개할 세션 ID입니다 (비어있으면 신규 세션).
	ResumeSession string
	// SystemPrompt는 시스템 프롬프트입니다.
	SystemPrompt string
}

// CodingSessionResponse는 코딩 세션 응답입니다.
type CodingSessionResponse struct {
	// Content는 에이전트 응답 내용입니다.
	Content string
	// DiffSummary는 변경사항 요약입니다.
	DiffSummary string
	// TestOutput은 테스트 실행 결과입니다.
	TestOutput string
	// FilesChanged는 변경된 파일 목록입니다.
	FilesChanged []string
	// SessionID는 세션 ID입니다.
	SessionID string
	// CostUSD는 이번 응답의 비용(USD)입니다.
	CostUSD float64
	// TurnCount는 총 턴 수입니다.
	TurnCount int
}

// CodingSessionConfig는 코딩 세션 관리자 설정입니다.
type CodingSessionConfig struct {
	// MaxConcurrent는 동시 실행 가능한 최대 세션 수입니다.
	MaxConcurrent int
	// RelayMaxIterations는 릴레이 최대 반복 횟수입니다.
	RelayMaxIterations int
	// SessionTimeoutMin은 세션 타임아웃(분)입니다.
	SessionTimeoutMin int
	// MaxBudgetUSD는 세션당 최대 예산(USD)입니다.
	MaxBudgetUSD float64
	// PreferredProvider는 선호하는 코딩 프로바이더입니다 (auto | claude | codex | gemini | opencode).
	PreferredProvider string
	// GitConfig는 GitExecutor 설정입니다 (HandleSessionComplete에서 사용).
	GitConfig GitExecutorConfig
}

// CodingProvider는 코딩 에이전트 프로바이더 타입입니다.
type CodingProvider string

const (
	// CodingProviderClaude는 Claude Code CLI 프로바이더입니다.
	CodingProviderClaude CodingProvider = "claude"
	// CodingProviderCodex는 Codex CLI 프로바이더입니다.
	CodingProviderCodex CodingProvider = "codex"
	// CodingProviderGemini는 Gemini CLI 프로바이더입니다.
	CodingProviderGemini CodingProvider = "gemini"
	// CodingProviderOpenCode는 OpenCode CLI 프로바이더입니다.
	CodingProviderOpenCode CodingProvider = "opencode"
	// CodingProviderAPI는 HTTP API 폴백 프로바이더입니다.
	CodingProviderAPI CodingProvider = "api"
)
