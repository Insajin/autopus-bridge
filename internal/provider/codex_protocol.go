package provider

import (
	"encoding/json"
	"fmt"
)

// ===== JSON-RPC 2.0 기본 타입 =====

// JSONRPCRequest는 JSON-RPC 2.0 요청 메시지입니다.
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	ID      int64           `json:"id"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSONRPCResponse는 JSON-RPC 2.0 응답 메시지입니다.
type JSONRPCResponse struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      int64            `json:"id"`
	Result  *json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError    `json:"error,omitempty"`
}

// JSONRPCNotification은 JSON-RPC 2.0 알림 메시지입니다 (id 없음).
type JSONRPCNotification struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSONRPCError는 JSON-RPC 에러 객체입니다.
type JSONRPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// Error는 JSONRPCError를 Go error 인터페이스로 구현합니다.
func (e *JSONRPCError) Error() string {
	return fmt.Sprintf("JSON-RPC 에러 [%d]: %s", e.Code, e.Message)
}

// ===== JSON-RPC 에러 코드 상수 =====

const (
	// ErrCodeParseError는 JSON 파싱 에러입니다.
	ErrCodeParseError = -32700
	// ErrCodeInvalidRequest는 잘못된 요청입니다.
	ErrCodeInvalidRequest = -32600
	// ErrCodeMethodNotFound는 메서드를 찾을 수 없을 때입니다.
	ErrCodeMethodNotFound = -32601
	// ErrCodeInvalidParams는 잘못된 파라미터입니다.
	ErrCodeInvalidParams = -32602
	// ErrCodeInternalError는 내부 에러입니다.
	ErrCodeInternalError = -32603
	// ErrCodeContextWindowExceeded는 컨텍스트 윈도우 초과입니다.
	ErrCodeContextWindowExceeded = -32001
	// ErrCodeUsageLimitExceeded는 사용량 제한 초과입니다.
	ErrCodeUsageLimitExceeded = -32002
	// ErrCodeUnauthorized는 인증 실패입니다.
	ErrCodeUnauthorized = -32003
	// ErrCodeConnectionFailed는 연결 실패입니다.
	ErrCodeConnectionFailed = -32004
)

// Codex App Server 관련 에러 변수
var (
	// ErrConnectionClosed는 JSON-RPC 연결이 종료되었을 때 반환됩니다.
	ErrConnectionClosed = fmt.Errorf("JSON-RPC 연결이 종료되었습니다")
	// ErrMaxRestartsExceeded는 최대 재시작 횟수를 초과했을 때 반환됩니다.
	ErrMaxRestartsExceeded = fmt.Errorf("최대 재시작 횟수를 초과했습니다")
	// ErrHandshakeTimeout은 초기화 핸드셰이크 타임아웃입니다.
	ErrHandshakeTimeout = fmt.Errorf("초기화 핸드셰이크 타임아웃")
	// ErrProcessNotRunning은 App Server 프로세스가 실행 중이 아닐 때 반환됩니다.
	ErrProcessNotRunning = fmt.Errorf("App Server 프로세스가 실행 중이 아닙니다")
)

// MapJSONRPCError는 JSON-RPC 에러 코드를 Go error로 변환합니다.
func MapJSONRPCError(rpcErr *JSONRPCError) error {
	if rpcErr == nil {
		return nil
	}
	switch rpcErr.Code {
	case ErrCodeContextWindowExceeded:
		return fmt.Errorf("컨텍스트 윈도우 초과: %s", rpcErr.Message)
	case ErrCodeUsageLimitExceeded:
		return fmt.Errorf("사용량 제한 초과: %s", rpcErr.Message)
	case ErrCodeUnauthorized:
		return fmt.Errorf("%w: %s", ErrNoAPIKey, rpcErr.Message)
	case ErrCodeConnectionFailed:
		return fmt.Errorf("%w: %s", ErrConnectionClosed, rpcErr.Message)
	default:
		return rpcErr
	}
}

// ===== Codex App Server 도메인 타입 =====

// InitializeParams는 초기화 핸드셰이크 요청 파라미터입니다.
type InitializeParams struct {
	// ClientVersion은 클라이언트 버전입니다.
	ClientVersion string `json:"clientVersion,omitempty"`
}

// InitializeResult는 초기화 핸드셰이크 결과입니다.
type InitializeResult struct {
	// ServerVersion은 서버 버전입니다.
	ServerVersion string `json:"serverVersion,omitempty"`
}

// ThreadStartParams는 Thread 생성 파라미터입니다.
type ThreadStartParams struct {
	// Model은 사용할 모델입니다.
	Model string `json:"model"`
	// Cwd는 작업 디렉토리입니다.
	Cwd string `json:"cwd"`
	// ApprovalPolicy는 승인 정책입니다 ("auto-approve", "deny-all").
	ApprovalPolicy string `json:"approvalPolicy,omitempty"`
}

// ThreadStartResult는 Thread 생성 결과입니다.
type ThreadStartResult struct {
	// ThreadID는 생성된 Thread의 ID입니다.
	ThreadID string `json:"threadId"`
}

// TurnStartParams는 Turn 시작 파라미터입니다.
type TurnStartParams struct {
	// ThreadID는 Turn을 시작할 Thread의 ID입니다.
	ThreadID string `json:"threadId"`
	// Input은 Turn 입력 목록입니다.
	Input []TurnInput `json:"input"`
}

// TurnStartResult는 Turn 시작 결과입니다.
type TurnStartResult struct {
	// TurnID는 시작된 Turn의 ID입니다.
	TurnID string `json:"turnId,omitempty"`
}

// TurnInput은 Turn 입력 단위입니다.
type TurnInput struct {
	// Type은 입력 타입입니다 (예: "text").
	Type string `json:"type"`
	// Text는 입력 텍스트입니다.
	Text string `json:"text"`
}

// AccountLoginParams는 인증 요청 파라미터입니다.
type AccountLoginParams struct {
	// Method는 인증 방식입니다 ("apiKey", "chatgptAuthTokens").
	Method string `json:"method"`
	// APIKey는 API 키입니다 (method가 "apiKey"일 때).
	APIKey string `json:"apiKey,omitempty"`
	// ChatGPTAuthTokens는 ChatGPT 인증 토큰입니다.
	ChatGPTAuthTokens string `json:"chatgptAuthTokens,omitempty"`
}

// AccountLoginResult는 인증 결과입니다.
type AccountLoginResult struct {
	// Success는 인증 성공 여부입니다.
	Success bool `json:"success"`
}

// ===== Item 이벤트 타입 =====

// AgentMessageDelta는 에이전트 메시지 텍스트 청크입니다.
type AgentMessageDelta struct {
	// Text는 텍스트 청크입니다.
	Text string `json:"text"`
}

// CommandExecutionOutputDelta는 명령 실행 출력 청크입니다.
type CommandExecutionOutputDelta struct {
	// Output은 명령 실행 출력입니다.
	Output string `json:"output"`
}

// CommandExecutionItem은 명령 실행 완료 아이템입니다.
type CommandExecutionItem struct {
	// ID는 아이템 ID입니다.
	ID string `json:"id,omitempty"`
	// Command는 실행된 명령입니다.
	Command string `json:"command,omitempty"`
	// Output은 명령 실행 출력입니다.
	Output string `json:"output,omitempty"`
	// ExitCode는 종료 코드입니다.
	ExitCode int `json:"exitCode,omitempty"`
}

// MCPToolCallItem은 MCP 도구 호출 아이템입니다.
type MCPToolCallItem struct {
	// ID는 아이템 ID입니다.
	ID string `json:"id,omitempty"`
	// ToolName은 호출된 도구 이름입니다.
	ToolName string `json:"toolName,omitempty"`
	// Input은 도구 입력 파라미터입니다.
	Input json.RawMessage `json:"input,omitempty"`
	// Output은 도구 실행 결과입니다.
	Output string `json:"output,omitempty"`
}

// RequestApprovalParams는 승인 요청 파라미터입니다.
type RequestApprovalParams struct {
	// ExecutionID는 실행 ID입니다.
	ExecutionID string `json:"executionId"`
	// Type은 승인 요청 타입입니다.
	Type string `json:"type"`
}

// ApprovalResponse는 승인 응답입니다.
type ApprovalResponse struct {
	// ExecutionID는 실행 ID입니다.
	ExecutionID string `json:"executionId"`
	// Decision은 승인 결정입니다 ("accept", "decline").
	Decision string `json:"decision"`
}

// TurnCompletedParams는 Turn 완료 알림 파라미터입니다.
type TurnCompletedParams struct {
	// ThreadID는 완료된 Turn의 Thread ID입니다.
	ThreadID string `json:"threadId"`
}

// ===== JSON-RPC 알림 메서드 상수 =====

const (
	// MethodInitialize는 초기화 요청 메서드입니다.
	MethodInitialize = "initialize"
	// MethodInitialized는 초기화 완료 알림 메서드입니다.
	MethodInitialized = "initialized"
	// MethodAccountLoginStart는 인증 시작 메서드입니다.
	MethodAccountLoginStart = "account/login/start"
	// MethodThreadStart는 Thread 생성 메서드입니다.
	MethodThreadStart = "thread/start"
	// MethodTurnStart는 Turn 시작 메서드입니다.
	MethodTurnStart = "turn/start"
	// MethodAgentMessageDelta는 에이전트 메시지 델타 알림입니다.
	MethodAgentMessageDelta = "item/agentMessage/delta"
	// MethodCommandExecutionOutputDelta는 명령 실행 출력 델타 알림입니다.
	MethodCommandExecutionOutputDelta = "item/commandExecution/outputDelta"
	// MethodCommandExecution은 명령 실행 완료 알림입니다.
	MethodCommandExecution = "item/commandExecution"
	// MethodMCPToolCall은 MCP 도구 호출 알림입니다.
	MethodMCPToolCall = "item/mcpToolCall"
	// MethodRequestApproval은 승인 요청 알림입니다.
	MethodRequestApproval = "requestApproval"
	// MethodTurnCompleted는 Turn 완료 알림입니다.
	MethodTurnCompleted = "turn/completed"
)
