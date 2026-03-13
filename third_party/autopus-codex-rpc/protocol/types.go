// Package protocol은 JSON-RPC 2.0 기본 타입과 Codex App Server 도메인 타입을 정의한다.
// Backend와 Bridge 간 공유 프로토콜 계층으로, 외부 의존성 없이 stdlib만 사용한다.
package protocol

import "encoding/json"

// --- JSON-RPC 2.0 기본 타입 ---

// JSONRPCRequest는 JSON-RPC 2.0 요청 메시지이다.
type JSONRPCRequest struct {
	// JSONRPC는 항상 "2.0"이다.
	JSONRPC string `json:"jsonrpc"`
	// Method는 호출할 메서드 이름이다.
	Method string `json:"method"`
	// ID는 요청 식별자이다. 응답과 매칭하는 데 사용한다.
	ID int64 `json:"id"`
	// Params는 메서드 파라미터이다. json.RawMessage로 지연 파싱을 지원한다.
	Params json.RawMessage `json:"params,omitempty"`
}

// JSONRPCResponse는 JSON-RPC 2.0 응답 메시지이다.
type JSONRPCResponse struct {
	// JSONRPC는 항상 "2.0"이다.
	JSONRPC string `json:"jsonrpc"`
	// ID는 요청 ID와 매칭된다. 알림 응답의 경우 nil이다.
	ID *int64 `json:"id"`
	// Result는 성공 응답 결과이다. 포인터로 null 구분을 지원한다.
	Result *json.RawMessage `json:"result,omitempty"`
	// Error는 에러 응답이다.
	Error *JSONRPCError `json:"error,omitempty"`
}

// JSONRPCNotification은 id가 없는 서버 -> 클라이언트 알림 메시지이다.
type JSONRPCNotification struct {
	// JSONRPC는 항상 "2.0"이다.
	JSONRPC string `json:"jsonrpc"`
	// Method는 알림 메서드 이름이다.
	Method string `json:"method"`
	// Params는 알림 파라미터이다.
	Params json.RawMessage `json:"params,omitempty"`
}

// --- Codex App Server 도메인 타입 ---

// ClientInfo는 클라이언트 식별 정보이다.
type ClientInfo struct {
	// Name은 클라이언트 이름이다 (예: "autopus-bridge").
	Name string `json:"name"`
	// Version은 클라이언트 버전이다.
	Version string `json:"version"`
}

// Capabilities는 클라이언트 능력 정보이다.
type Capabilities struct {
	// ExperimentalApi는 실험적 API(dynamicTools 등) 지원 여부이다.
	ExperimentalApi bool `json:"experimentalApi,omitempty"`
	// OptOutNotificationMethods는 수신 거부할 서버 알림 메서드 목록이다.
	// 예: ["turn/plan/updated", "turn/diff/updated"]
	OptOutNotificationMethods []string `json:"optOutNotificationMethods,omitempty"`
}

// InitializeParams는 initialize 핸드셰이크 요청 파라미터이다.
type InitializeParams struct {
	// ClientInfo는 클라이언트 식별 정보이다.
	ClientInfo ClientInfo `json:"clientInfo"`
	// Capabilities는 클라이언트 능력 정보이다.
	Capabilities Capabilities `json:"capabilities,omitempty"`
}

// InitializeResult는 initialize 핸드셰이크 응답 결과이다.
type InitializeResult struct {
	// ServerName은 서버 이름이다.
	ServerName string `json:"serverName,omitempty"`
	// ServerVersion은 서버 버전이다.
	ServerVersion string `json:"serverVersion,omitempty"`
}

// AccountLoginParams는 account/login/start 요청 파라미터이다.
// oneOf: apiKey | chatgpt | chatgptAuthTokens
type AccountLoginParams struct {
	// Type은 인증 방식이다 ("apiKey", "chatgpt", "chatgptAuthTokens").
	Type string `json:"type"`
	// APIKey는 API 키이다 (type="apiKey"일 때 필수).
	APIKey string `json:"apiKey,omitempty"`
	// AccessToken은 ChatGPT access_token이다 (type="chatgptAuthTokens"일 때 필수).
	AccessToken string `json:"accessToken,omitempty"`
	// ChatGPTAccountID는 ChatGPT 계정 ID이다 (type="chatgptAuthTokens"일 때 필수).
	ChatGPTAccountID string `json:"chatgptAccountId,omitempty"`
}

// AccountLoginResult는 account/login/start 응답 결과이다.
type AccountLoginResult struct {
	// Success는 인증 성공 여부이다.
	Success bool `json:"success"`
}

// DynamicToolDefinition은 dynamicTools로 전달되는 커스텀 도구 정의이다.
type DynamicToolDefinition struct {
	// Name은 도구 이름이다.
	Name string `json:"name"`
	// Description은 도구 설명이다.
	Description string `json:"description,omitempty"`
	// InputSchema는 도구 입력 스키마이다 (JSON Schema).
	InputSchema json.RawMessage `json:"inputSchema,omitempty"`
}

// ThreadStartParams는 thread/start 요청 파라미터이다.
// Codex App Server v2 스키마에 맞춰 정의한다.
type ThreadStartParams struct {
	// Model은 사용할 모델이다 (예: "gpt-5.4").
	Model string `json:"model,omitempty"`
	// Cwd는 작업 디렉토리이다.
	Cwd string `json:"cwd,omitempty"`
	// ApprovalPolicy는 승인 정책이다 ("never", "on-failure", "on-request", "untrusted").
	ApprovalPolicy string `json:"approvalPolicy,omitempty"`
	// Sandbox는 샌드박스 모드이다 ("read-only", "workspace-write", "danger-full-access").
	Sandbox string `json:"sandbox,omitempty"`
	// DeveloperInstructions는 개발자 제공 지침이다.
	DeveloperInstructions string `json:"developerInstructions,omitempty"`
	// BaseInstructions는 기본 지침이다.
	BaseInstructions string `json:"baseInstructions,omitempty"`
	// Config는 추가 설정 맵이다 (Codex config overrides).
	Config map[string]interface{} `json:"config,omitempty"`
	// DynamicTools는 실험적 API로 도구 정의를 등록한다.
	// capabilities.experimentalApi = true 필요.
	DynamicTools []DynamicToolDefinition `json:"dynamicTools,omitempty"`
}

// ThreadStartResult는 thread/start 응답 결과이다.
type ThreadStartResult struct {
	// ThreadID는 생성된 Thread의 ID이다.
	ThreadID string `json:"threadId"`
}

// ThreadResumeParams는 thread/resume 요청 파라미터이다.
type ThreadResumeParams struct {
	// ThreadID는 재개할 Thread의 ID이다.
	ThreadID string `json:"threadId"`
}

// TurnSettings는 Turn 시작 시 적용할 설정이다.
type TurnSettings struct {
	// DeveloperInstructions는 개발자 제공 지침이다 (시스템 프롬프트 추가).
	DeveloperInstructions *string `json:"developer_instructions,omitempty"`
}

// TurnStartParams는 turn/start 요청 파라미터이다.
type TurnStartParams struct {
	// ThreadID는 Turn을 시작할 Thread의 ID이다.
	ThreadID string `json:"threadId"`
	// Input은 Turn 입력 목록이다.
	Input []TurnInput `json:"input"`
	// Effort는 모델 추론 노력 수준이다 ("low", "medium", "high").
	Effort string `json:"effort,omitempty"`
	// Summary는 스트리밍 요약 모드이다 ("auto", "concise", "detailed").
	Summary string `json:"summary,omitempty"`
	// CollaborationMode는 협업 모드이다 ("sequential", "parallel").
	CollaborationMode string `json:"collaborationMode,omitempty"`
	// OutputSchema는 구조화 출력을 위한 JSON Schema이다.
	OutputSchema json.RawMessage `json:"outputSchema,omitempty"`
	// Settings는 Turn별 추가 설정이다.
	Settings *TurnSettings `json:"settings,omitempty"`
}

// TurnSteerParams는 turn/steer 요청 파라미터이다.
// 진행 중인 Turn의 방향을 전환할 때 사용한다.
type TurnSteerParams struct {
	// ThreadID는 방향을 전환할 Thread의 ID이다.
	ThreadID string `json:"threadId"`
	// Input은 새 입력 데이터이다.
	Input json.RawMessage `json:"input"`
}

// CollabToolCallItem은 협업 도구 호출 아이템이다.
// dynamicTools 협업 모드에서 서버가 클라이언트에게 전달하는 도구 호출 단위이다.
type CollabToolCallItem struct {
	// Type은 아이템 타입이다 (예: "tool_call").
	Type string `json:"type"`
	// ID는 아이템 고유 ID이다.
	ID string `json:"id,omitempty"`
	// Name은 도구 이름이다.
	Name string `json:"name"`
	// Arguments는 도구 실행 인자이다 (임의의 JSON).
	Arguments json.RawMessage `json:"arguments,omitempty"`
	// CallID는 도구 호출 상관관계 ID이다.
	CallID string `json:"callId,omitempty"`
}

// TurnStartResult는 turn/start 응답 결과이다.
type TurnStartResult struct {
	// TurnID는 시작된 Turn의 ID이다.
	TurnID string `json:"turnId,omitempty"`
}

// TurnInput은 Turn 입력 단위이다.
type TurnInput struct {
	// Type은 입력 타입이다 ("text", "image", "skill").
	Type string `json:"type"`
	// Text는 입력 텍스트이다. Codex v0.114.0은 text 필드를 필수로 요구한다.
	Text string `json:"text"`
}

// TurnCompletedParams는 turn/completed 알림 파라미터이다.
type TurnCompletedParams struct {
	// ThreadID는 완료된 Turn의 Thread ID이다.
	ThreadID string `json:"threadId"`
	// TurnID는 완료된 Turn의 ID이다.
	TurnID string `json:"turnId,omitempty"`
}

// ItemNotification은 item 관련 알림의 공통 구조이다.
type ItemNotification struct {
	// ThreadID는 Thread ID이다.
	ThreadID string `json:"threadId"`
	// ItemID는 Item ID이다.
	ItemID string `json:"itemId"`
	// ItemType은 Item 타입이다 ("agentMessage", "commandExecution", "fileChange", "mcpToolCall").
	ItemType string `json:"itemType"`
	// Data는 Item 타입별 데이터이다.
	Data json.RawMessage `json:"data,omitempty"`
}

// ItemCompletedParams는 item/completed 알림 파라미터이다.
type ItemCompletedParams struct {
	// ThreadID는 Thread ID이다.
	ThreadID string `json:"threadId"`
	// ItemID는 Item ID이다.
	ItemID string `json:"itemId"`
	// ItemType은 Item 타입이다.
	ItemType string `json:"itemType"`
	// Data는 Item 완료 데이터이다.
	Data json.RawMessage `json:"data,omitempty"`
}

// AgentMessageDelta는 agentMessage 텍스트 증분이다.
type AgentMessageDelta struct {
	// Delta는 텍스트 청크이다.
	Delta string `json:"delta"`
}

// AgentMessageCompleted는 완료된 agentMessage의 데이터이다.
type AgentMessageCompleted struct {
	// Text는 전체 텍스트이다.
	Text string `json:"text"`
}

// CommandExecutionDelta는 commandExecution 출력 증분이다.
type CommandExecutionDelta struct {
	// Delta는 출력 청크이다.
	Delta string `json:"delta"`
}

// CommandExecutionCompleted는 완료된 commandExecution의 데이터이다.
type CommandExecutionCompleted struct {
	// Command는 실행된 명령이다.
	Command string `json:"command"`
	// ExitCode는 종료 코드이다.
	ExitCode int `json:"exitCode"`
	// Output은 명령 실행 출력이다.
	Output string `json:"output"`
}

// FileChangeDelta는 fileChange 출력 증분이다.
type FileChangeDelta struct {
	// Delta는 변경 청크이다.
	Delta string `json:"delta"`
}

// FileChangeCompleted는 완료된 fileChange의 데이터이다.
type FileChangeCompleted struct {
	// FilePath는 변경된 파일 경로이다.
	FilePath string `json:"filePath"`
	// Diff는 변경 내용이다.
	Diff string `json:"diff,omitempty"`
}

// MCPToolCallCompleted는 완료된 mcpToolCall의 데이터이다.
type MCPToolCallCompleted struct {
	// ToolName은 호출된 도구 이름이다.
	ToolName string `json:"toolName"`
	// Input은 도구 입력 파라미터이다.
	Input string `json:"input"`
	// Output은 도구 실행 결과이다.
	Output string `json:"output"`
}

// ApprovalRequest는 승인 요청 이벤트의 데이터이다.
type ApprovalRequest struct {
	// ThreadID는 Thread ID이다.
	ThreadID string `json:"threadId"`
	// ItemID는 승인 대상 Item ID이다.
	ItemID string `json:"itemId"`
	// ItemType은 Item 타입이다 ("commandExecution", "fileChange").
	ItemType string `json:"itemType"`
	// Command는 실행될 명령이다 (commandExecution 타입일 때).
	Command string `json:"command,omitempty"`
	// FilePath는 변경될 파일 경로이다 (fileChange 타입일 때).
	FilePath string `json:"filePath,omitempty"`
}

// DynamicToolCallRequest는 서버가 클라이언트에게 동적 도구 실행을 요청할 때의 파라미터이다.
// v2 스키마: callId, turnId 필드 필수.
type DynamicToolCallRequest struct {
	// ThreadID는 Thread ID이다.
	ThreadID string `json:"threadId"`
	// TurnID는 Turn ID이다.
	TurnID string `json:"turnId"`
	// CallID는 도구 호출 ID이다.
	CallID string `json:"callId"`
	// Tool은 실행할 도구 이름이다.
	Tool string `json:"tool"`
	// Arguments는 도구 실행 인자이다.
	Arguments json.RawMessage `json:"arguments"`
	// ItemID는 레거시 호환 Item ID이다 (v1에서만 사용).
	ItemID string `json:"itemId,omitempty"`
}

// ContentItem은 동적 도구 응답의 콘텐츠 항목이다.
// v2 스키마: type은 "inputText" 또는 "inputImage"이다.
type ContentItem struct {
	// Type은 콘텐츠 타입이다 ("inputText" 또는 "inputImage").
	Type string `json:"type"`
	// Text는 텍스트 콘텐츠이다 (type="inputText"일 때).
	Text string `json:"text,omitempty"`
	// ImageURL은 이미지 URL이다 (type="inputImage"일 때).
	ImageURL string `json:"imageUrl,omitempty"`
}

// DynamicToolCallResponse는 클라이언트가 동적 도구 실행 결과를 반환할 때의 응답이다.
type DynamicToolCallResponse struct {
	// ContentItems는 응답 콘텐츠 항목 목록이다.
	ContentItems []ContentItem `json:"contentItems,omitempty"`
	// Success는 도구 실행 성공 여부이다.
	Success bool `json:"success"`
}

// ApprovalResponseParams는 승인 응답 파라미터이다.
type ApprovalResponseParams struct {
	// ThreadID는 Thread ID이다.
	ThreadID string `json:"threadId"`
	// ItemID는 승인 대상 Item ID이다.
	ItemID string `json:"itemId"`
	// Decision은 승인 결정이다 ("accept", "acceptForSession", "decline", "cancel").
	Decision string `json:"decision"`
}

// --- Thread 관리 타입 (REQ-009) ---

// ThreadForkParams는 thread/fork 요청 파라미터이다.
type ThreadForkParams struct {
	// ThreadID는 포크할 원본 Thread의 ID이다.
	ThreadID string `json:"threadId"`
}

// ThreadForkResult는 thread/fork 응답 결과이다.
type ThreadForkResult struct {
	// ThreadID는 새로 생성된 포크 Thread의 ID이다.
	ThreadID string `json:"threadId,omitempty"`
}

// ThreadReadParams는 thread/read 요청 파라미터이다.
type ThreadReadParams struct {
	// ThreadID는 읽을 Thread의 ID이다.
	ThreadID string `json:"threadId"`
}

// ThreadReadResult는 thread/read 응답 결과이다.
type ThreadReadResult struct {
	// ThreadID는 Thread의 ID이다.
	ThreadID string `json:"threadId,omitempty"`
	// Status는 Thread 상태이다 ("idle", "running", "completed", "error").
	Status string `json:"status,omitempty"`
	// Items는 Thread에 포함된 아이템 목록이다 (JSON 배열).
	Items json.RawMessage `json:"items,omitempty"`
}

// ThreadListParams는 thread/list 요청 파라미터이다.
type ThreadListParams struct {
	// Limit은 반환할 최대 Thread 수이다.
	Limit int `json:"limit,omitempty"`
	// Cursor는 페이지네이션 커서이다.
	Cursor string `json:"cursor,omitempty"`
}

// ThreadListResult는 thread/list 응답 결과이다.
type ThreadListResult struct {
	// Threads는 Thread 목록이다 (JSON 배열).
	Threads json.RawMessage `json:"threads,omitempty"`
	// NextCursor는 다음 페이지 커서이다.
	NextCursor string `json:"nextCursor,omitempty"`
}

// ThreadRollbackParams는 thread/rollback 요청 파라미터이다.
type ThreadRollbackParams struct {
	// ThreadID는 롤백할 Thread의 ID이다.
	ThreadID string `json:"threadId"`
	// TurnID는 롤백 대상 Turn의 ID이다 (해당 Turn 이후를 제거).
	TurnID string `json:"turnId,omitempty"`
}

// ThreadRollbackResult는 thread/rollback 응답 결과이다.
type ThreadRollbackResult struct {
	// ThreadID는 롤백된 Thread의 ID이다.
	ThreadID string `json:"threadId,omitempty"`
}

// --- Model 목록 타입 (REQ-016) ---

// ModelInfo는 모델 정보이다.
type ModelInfo struct {
	// ID는 모델 식별자이다 (예: "o4-mini").
	ID string `json:"id"`
	// Name은 모델 표시 이름이다.
	Name string `json:"name,omitempty"`
	// Description은 모델 설명이다.
	Description string `json:"description,omitempty"`
}

// ModelListResult는 model/list 응답 결과이다.
type ModelListResult struct {
	// Models는 사용 가능한 모델 목록이다.
	Models []ModelInfo `json:"models,omitempty"`
}

// --- Review 타입 (REQ-011) ---

// ReviewStartParams는 review/start 요청 파라미터이다.
type ReviewStartParams struct {
	// ThreadID는 리뷰를 시작할 Thread의 ID이다.
	ThreadID string `json:"threadId"`
	// Prompt는 리뷰 요청 프롬프트이다.
	Prompt string `json:"prompt,omitempty"`
}

// ReviewStartResult는 review/start 응답 결과이다.
type ReviewStartResult struct {
	// ReviewID는 생성된 리뷰의 ID이다.
	ReviewID string `json:"reviewId,omitempty"`
}

// --- Config 타입 (REQ-012) ---

// ConfigReadResult는 config/read 응답 결과이다.
type ConfigReadResult struct {
	// Config는 현재 설정 값이다 (임의의 JSON 객체).
	Config json.RawMessage `json:"config,omitempty"`
}

// ConfigWriteParams는 config/value/write 요청 파라미터이다.
type ConfigWriteParams struct {
	// Key는 설정 키이다.
	Key string `json:"key"`
	// Value는 설정 값이다 (임의의 JSON).
	Value json.RawMessage `json:"value,omitempty"`
}

// ConfigBatchWriteParams는 config/batchWrite 요청 파라미터이다.
type ConfigBatchWriteParams struct {
	// Updates는 키-값 쌍으로 된 설정 업데이트 맵이다.
	Updates map[string]json.RawMessage `json:"updates,omitempty"`
}

// --- Utility 목록 타입 (REQ-012) ---

// SkillInfo는 스킬 정보이다.
type SkillInfo struct {
	// ID는 스킬 식별자이다.
	ID string `json:"id"`
	// Name은 스킬 이름이다.
	Name string `json:"name,omitempty"`
	// Enabled는 스킬 활성화 여부이다.
	Enabled bool `json:"enabled,omitempty"`
}

// SkillsListResult는 skills/list 응답 결과이다.
type SkillsListResult struct {
	// Skills는 사용 가능한 스킬 목록이다.
	Skills []SkillInfo `json:"skills,omitempty"`
}

// McpServerStatus는 MCP 서버 상태 정보이다.
type McpServerStatus struct {
	// Name은 서버 이름이다.
	Name string `json:"name"`
	// Status는 서버 상태이다 ("connected", "disconnected", "error").
	Status string `json:"status,omitempty"`
}

// McpServerStatusListResult는 mcpServerStatus/list 응답 결과이다.
type McpServerStatusListResult struct {
	// Servers는 MCP 서버 상태 목록이다.
	Servers []McpServerStatus `json:"servers,omitempty"`
}

// ExperimentalFeature는 실험적 기능 정보이다.
type ExperimentalFeature struct {
	// ID는 기능 식별자이다.
	ID string `json:"id"`
	// Name은 기능 이름이다.
	Name string `json:"name,omitempty"`
	// Enabled는 기능 활성화 여부이다.
	Enabled bool `json:"enabled,omitempty"`
}

// ExperimentalFeatureListResult는 experimentalFeature/list 응답 결과이다.
type ExperimentalFeatureListResult struct {
	// Features는 실험적 기능 목록이다.
	Features []ExperimentalFeature `json:"features,omitempty"`
}
