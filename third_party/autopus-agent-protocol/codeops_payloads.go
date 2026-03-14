package ws

// CodeOpsRequestPayload는 서버가 Bridge에 보내는 코드 수정 요청입니다.
// Message type: codeops_request (Server -> Bridge)
// REQ-003.7: CodeOpsRequest WebSocket 메시지 타입
// SPEC-CODEOPS-001
type CodeOpsRequestPayload struct {
	// RequestID는 요청 ID입니다 (서버에서 생성).
	RequestID string `json:"request_id"`
	// WorkspaceID는 워크스페이스 ID입니다.
	WorkspaceID string `json:"workspace_id"`
	// WorkspaceSlug는 워크스페이스 슬러그입니다 (브랜치 정책용).
	WorkspaceSlug string `json:"workspace_slug"`
	// RepoURL은 GitHub 레포지토리 URL입니다.
	RepoURL string `json:"repo_url"`
	// Branch는 기반 브랜치입니다 (예: main).
	Branch string `json:"branch"`
	// AgentBranch는 에이전트 작업 브랜치입니다 (agent/ 프리픽스 필수).
	AgentBranch string `json:"agent_branch"`
	// AgentName은 에이전트 이름입니다 (커밋 메시지용).
	AgentName string `json:"agent_name"`
	// Description은 변경 설명입니다.
	Description string `json:"description"`
	// TargetFiles는 수정 대상 파일 목록입니다.
	TargetFiles []string `json:"target_files"`
	// ChangeType은 변경 타입입니다 (feature/bugfix/refactor/test/docs).
	ChangeType string `json:"change_type"`
	// OAuthToken은 GitHub OAuth 토큰입니다 (일회성, Bridge에 저장 안됨).
	OAuthToken string `json:"oauth_token"`
	// CorrelationID는 실행 그래프 추적 ID입니다.
	CorrelationID string `json:"correlation_id"`
	// InitiativeID는 Initiative ID (선택)입니다.
	InitiativeID string `json:"initiative_id,omitempty"`
	// GoalID는 Goal ID (선택)입니다.
	GoalID string `json:"goal_id,omitempty"`
	// TaskID는 Task ID (선택)입니다.
	TaskID string `json:"task_id,omitempty"`
}

// CodeOpsResultPayload는 Bridge가 서버에 보내는 코드 수정 결과입니다.
// Message type: codeops_result (Bridge -> Server)
// REQ-003.8: CodeOpsResult WebSocket 메시지 타입
// SPEC-CODEOPS-001
type CodeOpsResultPayload struct {
	// RequestID는 요청 ID입니다.
	RequestID string `json:"request_id"`
	// WorkspaceID는 워크스페이스 ID입니다.
	WorkspaceID string `json:"workspace_id"`
	// Success는 성공 여부입니다.
	Success bool `json:"success"`
	// PushedBranch는 push된 브랜치명입니다.
	PushedBranch string `json:"pushed_branch,omitempty"`
	// CommitSHA는 생성된 커밋 SHA입니다.
	CommitSHA string `json:"commit_sha,omitempty"`
	// ChangedFiles는 실제 변경된 파일 목록입니다.
	ChangedFiles []string `json:"changed_files,omitempty"`
	// DiffSummary는 변경사항 요약입니다.
	DiffSummary string `json:"diff_summary,omitempty"`
	// TestOutput은 테스트 실행 결과입니다.
	TestOutput string `json:"test_output,omitempty"`
	// Error는 실패 시 에러 메시지입니다.
	Error string `json:"error,omitempty"`
	// RetryCount는 재시도 횟수입니다.
	RetryCount int `json:"retry_count"`
	// CorrelationID는 실행 그래프 추적 ID입니다.
	CorrelationID string `json:"correlation_id"`
}
