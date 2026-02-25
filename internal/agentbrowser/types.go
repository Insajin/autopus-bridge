// Package agentbrowser는 Vercel의 agent-browser CLI 도구를 서브프로세스로 관리하여
// AI 기반 웹 자동화를 제공한다.
package agentbrowser

import "time"

// BrowserActionPayload는 백엔드에서 수신하는 브라우저 액션 요청이다.
// autopus-agent-protocol 패키지에 정의될 때까지 로컬 복사본을 유지한다.
type BrowserActionPayload struct {
	ExecutionID string                 `json:"execution_id"`
	SessionID   string                 `json:"session_id"`
	Command     string                 `json:"command"`
	Ref         *int                   `json:"ref,omitempty"`
	Params      map[string]interface{} `json:"params,omitempty"`
}

// BrowserResultPayload는 브라우저 액션 결과를 서버로 전송하는 페이로드이다.
type BrowserResultPayload struct {
	ExecutionID string `json:"execution_id"`
	SessionID   string `json:"session_id"`
	Success     bool   `json:"success"`
	Snapshot    string `json:"snapshot,omitempty"`
	Screenshot  string `json:"screenshot,omitempty"`
	Output      string `json:"output,omitempty"`
	Error       string `json:"error,omitempty"`
	DurationMs  int64  `json:"duration_ms"`
}

// BrowserSessionPayload는 브라우저 세션 관련 메시지 페이로드이다.
type BrowserSessionPayload struct {
	ExecutionID string `json:"execution_id"`
	SessionID   string `json:"session_id"`
	URL         string `json:"url,omitempty"`
	Headless    bool   `json:"headless"`
	Status      string `json:"status"` // starting, ready, busy, error, stopped
}

// CommandResult는 agent-browser CLI 명령 실행 결과이다.
type CommandResult struct {
	// Output은 명령의 텍스트 출력이다.
	Output string
	// Snapshot은 접근성 트리 스냅샷 데이터이다.
	Snapshot string
	// Screenshot은 스크린샷 바이너리 데이터이다 (PNG).
	Screenshot []byte
	// Error는 명령 실행 중 발생한 오류 메시지이다.
	Error string
	// DurationMs는 명령 실행에 걸린 시간 (밀리초)이다.
	DurationMs int64
}

// ExecutionResult는 Manager를 통한 명령 실행 결과이다.
type ExecutionResult struct {
	// Output은 명령의 텍스트 출력이다.
	Output string
	// Snapshot은 접근성 트리 스냅샷 데이터이다.
	Snapshot string
	// Screenshot은 base64 인코딩된 스크린샷 데이터이다.
	Screenshot string
	// Error는 명령 실행 오류 메시지이다.
	Error string
	// DurationMs는 명령 실행에 걸린 시간 (밀리초)이다.
	DurationMs int64
}

// ManagerState는 agent-browser 데몬의 상태를 나타낸다.
type ManagerState string

const (
	// StateStarting은 데몬이 시작 중인 상태이다.
	StateStarting ManagerState = "starting"
	// StateReady는 데몬이 명령을 받을 준비가 된 상태이다.
	StateReady ManagerState = "ready"
	// StateBusy는 데몬이 명령을 실행 중인 상태이다.
	StateBusy ManagerState = "busy"
	// StateError는 데몬에 오류가 발생한 상태이다.
	StateError ManagerState = "error"
	// StateStopped는 데몬이 중지된 상태이다.
	StateStopped ManagerState = "stopped"
	// StateStandby는 유휴 대기 상태이다 (브라우저 중지, 데몬 유지).
	StateStandby ManagerState = "standby"
)

// StatusCallback은 상태 변경 시 호출되는 콜백 함수 타입이다.
type StatusCallback func(state ManagerState, message string)

// CICDConfig는 CI/CD 환경에서의 agent-browser 실행 설정이다 (REQ-M5-04).
type CICDConfig struct {
	// Headless는 항상 headless 모드를 강제한다.
	Headless bool
	// JSONOutput은 JSON 형식 출력을 강제한다.
	JSONOutput bool
	// NoColor는 색상 출력을 비활성화한다.
	NoColor bool
	// Timeout은 글로벌 타임아웃이다 (기본 5분).
	Timeout time.Duration
}

// DefaultCICDTimeout은 CI/CD 환경의 기본 글로벌 타임아웃이다.
const DefaultCICDTimeout = 5 * time.Minute

// IsEnabled는 CI/CD 모드가 활성화되었는지 반환한다.
// Headless, JSONOutput, NoColor 중 하나라도 설정되었으면 활성화된 것으로 판단한다.
func (c CICDConfig) IsEnabled() bool {
	return c.Headless || c.JSONOutput || c.NoColor
}
