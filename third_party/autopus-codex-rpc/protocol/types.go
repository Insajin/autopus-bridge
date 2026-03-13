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

// InitializeParams는 initialize 핸드셰이크 요청 파라미터이다.
type InitializeParams struct {
	// ClientInfo는 클라이언트 식별 정보이다.
	ClientInfo ClientInfo `json:"clientInfo"`
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

// ThreadStartParams는 thread/start 요청 파라미터이다.
type ThreadStartParams struct {
	// Model은 사용할 모델이다 (예: "o4-mini").
	Model string `json:"model"`
	// Cwd는 작업 디렉토리이다.
	Cwd string `json:"cwd,omitempty"`
	// ApprovalPolicy는 승인 정책이다 ("auto-approve", "deny-all").
	ApprovalPolicy string `json:"approvalPolicy,omitempty"`
	// Sandbox는 샌드박스 모드 활성화 여부이다 (Backend에서 추가).
	Sandbox bool `json:"sandbox,omitempty"`
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

// TurnStartParams는 turn/start 요청 파라미터이다.
type TurnStartParams struct {
	// ThreadID는 Turn을 시작할 Thread의 ID이다.
	ThreadID string `json:"threadId"`
	// Input은 Turn 입력 목록이다.
	Input []TurnInput `json:"input"`
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
	// Text는 입력 텍스트이다.
	Text string `json:"text,omitempty"`
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

// ApprovalResponseParams는 승인 응답 파라미터이다.
type ApprovalResponseParams struct {
	// ThreadID는 Thread ID이다.
	ThreadID string `json:"threadId"`
	// ItemID는 승인 대상 Item ID이다.
	ItemID string `json:"itemId"`
	// Decision은 승인 결정이다 ("accept", "acceptForSession", "decline", "cancel").
	Decision string `json:"decision"`
}
