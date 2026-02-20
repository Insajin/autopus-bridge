// Package websocket는 Local Agent Bridge의 WebSocket 통신을 담당합니다.
// REQ-E-01: WebSocket 연결 관리
// REQ-E-06: 30초 하트비트 간격
package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/insajin/autopus-agent-protocol"
)

// ReconnectionHandler is an optional interface that message handlers can
// implement to receive notifications when the WebSocket connection is
// re-established. This enables stateful subsystems (e.g. Computer Use
// sessions) to restore their state after a reconnection (REQ-M3-04).
type ReconnectionHandler interface {
	OnReconnected(ctx context.Context) error
}

// SPEC 4.3 타이밍 상수
const (
	// HeartbeatInterval은 하트비트 전송 간격입니다 (REQ-E-06).
	HeartbeatInterval = 30 * time.Second

	// HeartbeatTimeout은 하트비트 응답 대기 시간입니다.
	HeartbeatTimeout = 60 * time.Second

	// MaxMessageSize는 최대 메시지 크기입니다 (1MB).
	MaxMessageSize = 1024 * 1024

	// WriteTimeout은 메시지 쓰기 타임아웃입니다.
	WriteTimeout = 10 * time.Second

	// ReadTimeout은 메시지 읽기 타임아웃입니다.
	ReadTimeout = HeartbeatTimeout

	// ConnectTimeout은 연결 타임아웃입니다.
	ConnectTimeout = 30 * time.Second

	// AuthTimeout은 인증 응답 대기 타임아웃입니다 (FR-P2-01).
	AuthTimeout = 10 * time.Second
)

// ConnectionState는 WebSocket 연결 상태를 나타냅니다.
type ConnectionState int32

const (
	// StateDisconnected는 연결되지 않은 상태입니다.
	StateDisconnected ConnectionState = iota
	// StateConnecting은 연결 중인 상태입니다.
	StateConnecting
	// StateAuthenticating은 WebSocket 연결 후 인증 진행 중인 상태입니다 (FR-P2-01).
	// Flow: Disconnected -> Connecting -> Authenticating -> Connected
	StateAuthenticating
	// StateConnected는 연결된 상태입니다.
	StateConnected
	// StateReconnecting은 재연결 중인 상태입니다.
	StateReconnecting
	// StateClosed는 명시적으로 종료된 상태입니다.
	StateClosed
)

// String은 ConnectionState의 문자열 표현을 반환합니다.
func (s ConnectionState) String() string {
	switch s {
	case StateDisconnected:
		return "disconnected"
	case StateConnecting:
		return "connecting"
	case StateAuthenticating:
		return "authenticating"
	case StateConnected:
		return "connected"
	case StateReconnecting:
		return "reconnecting"
	case StateClosed:
		return "closed"
	default:
		return "unknown"
	}
}

// Client는 WebSocket 클라이언트를 나타냅니다.
type Client struct {
	// serverURL은 WebSocket 서버 URL입니다.
	serverURL string
	// token은 JWT 인증 토큰입니다.
	token string
	// version은 에이전트 버전입니다.
	version string
	// capabilities는 지원하는 CLI 목록입니다.
	capabilities []string

	// conn은 WebSocket 연결입니다.
	conn *websocket.Conn
	// connMu는 연결 접근을 보호하는 뮤텍스입니다.
	connMu sync.RWMutex

	// state는 현재 연결 상태입니다.
	state atomic.Int32

	// messages는 수신된 메시지를 전달하는 채널입니다.
	messages chan ws.AgentMessage
	// done은 클라이언트 종료를 알리는 채널입니다.
	done chan struct{}

	// reconnectStrategy는 재연결 전략입니다.
	reconnectStrategy *ReconnectStrategy

	// lastExecID는 마지막으로 처리한 실행 ID입니다 (재연결 시 복구용).
	lastExecID string
	// lastExecIDMu는 lastExecID 접근을 보호하는 뮤텍스입니다.
	lastExecIDMu sync.RWMutex

	// writeMu는 WebSocket 쓰기 접근을 보호하는 뮤텍스입니다.
	// gorilla/websocket은 동시 쓰기를 지원하지 않으므로 모든 WriteMessage 호출을 직렬화합니다.
	writeMu sync.Mutex

	// heartbeatCancel은 하트비트 고루틴을 취소하는 함수입니다.
	heartbeatCancel context.CancelFunc
	// heartbeatMu는 하트비트 취소 함수 접근을 보호하는 뮤텍스입니다.
	heartbeatMu sync.Mutex

	// lastHeartbeat는 마지막 하트비트 시간입니다.
	lastHeartbeat time.Time
	// lastHeartbeatMu는 lastHeartbeat 접근을 보호하는 뮤텍스입니다.
	lastHeartbeatMu sync.RWMutex

	// handler는 메시지 핸들러입니다.
	handler MessageHandler

	// signer는 HMAC-SHA256 메시지 서명기입니다 (SEC-P2-02).
	signer *MessageSigner

	// taskTracker는 진행 중인 태스크를 추적합니다 (FR-P2-04).
	taskTracker *TaskTracker

	// tokenRefreshFn은 재연결 전 토큰을 갱신하는 콜백입니다.
	tokenRefreshFn func() (string, error)

	// heartbeatEnricher는 하트비트 전송 시 추가 정보를 제공하는 콜백입니다 (SPEC-AI-003 M3 T-26).
	heartbeatEnricher func() (mcpServeStatus string)
}

// ClientOption은 Client 설정 옵션입니다.
type ClientOption func(*Client)

// WithReconnectStrategy는 재연결 전략을 설정합니다.
func WithReconnectStrategy(strategy *ReconnectStrategy) ClientOption {
	return func(c *Client) {
		c.reconnectStrategy = strategy
	}
}

// WithCapabilities는 지원하는 CLI 목록을 설정합니다.
func WithCapabilities(capabilities []string) ClientOption {
	return func(c *Client) {
		c.capabilities = capabilities
	}
}

// WithMessageHandler는 메시지 핸들러를 설정합니다.
func WithMessageHandler(handler MessageHandler) ClientOption {
	return func(c *Client) {
		c.handler = handler
	}
}

// SetMessageHandler는 메시지 핸들러를 런타임에 설정합니다.
// Client 생성 후 Router 등의 핸들러를 연결할 때 사용합니다.
func (c *Client) SetMessageHandler(handler MessageHandler) {
	c.handler = handler
}

// Signer는 HMAC-SHA256 메시지 서명기를 반환합니다 (SEC-P2-02).
func (c *Client) Signer() *MessageSigner {
	return c.signer
}

// NewClient는 새로운 WebSocket 클라이언트를 생성합니다.
func NewClient(serverURL, token, version string, opts ...ClientOption) *Client {
	c := &Client{
		serverURL:         serverURL,
		token:             token,
		version:           version,
		capabilities:      []string{"claude"},
		messages:          make(chan ws.AgentMessage, 500),
		done:              make(chan struct{}),
		reconnectStrategy: DefaultReconnectStrategy(),
		signer:            NewMessageSigner(), // SEC-P2-02
		taskTracker:       NewTaskTracker(),   // FR-P2-04
	}

	for _, opt := range opts {
		opt(c)
	}

	c.state.Store(int32(StateDisconnected))
	return c
}

// Connect는 WebSocket 서버에 연결합니다.
// REQ-E-01: connect 커맨드 시 WebSocket 연결
// FR-P2-02: 메시지 기반 JWT 인증 (URL에서 토큰 제거)
//
// State flow: Disconnected -> Connecting -> Authenticating -> Connected
func (c *Client) Connect(ctx context.Context) error {
	if c.State() == StateClosed {
		return errors.New("클라이언트가 닫혔습니다")
	}

	c.state.Store(int32(StateConnecting))

	// 연결 타임아웃 컨텍스트 생성
	connectCtx, cancel := context.WithTimeout(ctx, ConnectTimeout)
	defer cancel()

	// FR-P2-02: 토큰을 URL에 포함하지 않음
	u, err := url.Parse(c.serverURL)
	if err != nil {
		c.state.Store(int32(StateDisconnected))
		return fmt.Errorf("서버 URL 파싱 실패: %w", err)
	}

	// WebSocket 다이얼
	dialer := websocket.Dialer{
		HandshakeTimeout: ConnectTimeout,
	}

	conn, _, err := dialer.DialContext(connectCtx, u.String(), nil)
	if err != nil {
		c.state.Store(int32(StateDisconnected))
		return fmt.Errorf("WebSocket 연결 실패: %w", err)
	}

	// 연결 설정
	conn.SetReadLimit(MaxMessageSize)

	// 서버 PING 메시지 처리 - PONG 응답 전송 및 연결 활성 상태 유지
	conn.SetPingHandler(func(appData string) error {
		_ = conn.SetWriteDeadline(time.Now().Add(WriteTimeout))
		if err := conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(WriteTimeout)); err != nil {
			return err
		}
		return nil
	})

	c.connMu.Lock()
	c.conn = conn
	c.connMu.Unlock()

	// FR-P2-01: Authenticating 상태로 전환
	c.state.Store(int32(StateAuthenticating))

	// FR-P2-02: agent_connect 메시지에 토큰을 포함하여 전송
	if err := c.sendConnect(); err != nil {
		c.closeConnection()
		c.state.Store(int32(StateDisconnected))
		return fmt.Errorf("연결 메시지 전송 실패: %w", err)
	}

	// FR-P2-02: 서버의 인증 응답(agent_connect_ack) 대기
	if err := c.waitForAuthAck(); err != nil {
		c.closeConnection()
		c.state.Store(int32(StateDisconnected))
		return fmt.Errorf("인증 실패: %w", err)
	}

	c.state.Store(int32(StateConnected))
	c.reconnectStrategy.Reset()

	// 재연결 시 done 채널이 닫혀 있을 수 있으므로 재생성
	c.ResetDone()

	// 메시지 수신 고루틴 시작
	// 연결 타임아웃 컨텍스트(ctx)가 아닌 done 채널 기반 컨텍스트를 사용합니다.
	readCtx, readCancel := context.WithCancel(context.Background())
	go func() {
		<-c.done
		readCancel()
	}()
	go c.readLoop(readCtx)

	return nil
}

// waitForAuthAck는 서버의 agent_connect_ack 응답을 대기합니다 (FR-P2-02).
// AuthTimeout 이내에 성공 응답을 받지 못하면 에러를 반환합니다.
func (c *Client) waitForAuthAck() error {
	c.connMu.RLock()
	conn := c.conn
	c.connMu.RUnlock()

	if conn == nil {
		return errors.New("연결이 없습니다")
	}

	// 인증 응답 대기 타임아웃 설정
	_ = conn.SetReadDeadline(time.Now().Add(AuthTimeout))

	_, data, err := conn.ReadMessage()
	if err != nil {
		return fmt.Errorf("인증 응답 수신 실패: %w", err)
	}

	// 읽기 데드라인 초기화
	_ = conn.SetReadDeadline(time.Time{})

	var msg ws.AgentMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return fmt.Errorf("인증 응답 파싱 실패: %w", err)
	}

	if msg.Type != ws.AgentMsgConnectAck {
		return fmt.Errorf("예상치 못한 메시지 타입: %s (agent_connect_ack 기대)", msg.Type)
	}

	var ackPayload ws.ConnectAckPayload
	if err := json.Unmarshal(msg.Payload, &ackPayload); err != nil {
		return fmt.Errorf("인증 응답 페이로드 파싱 실패: %w", err)
	}

	if !ackPayload.Success {
		return fmt.Errorf("서버 인증 거부: %s", ackPayload.Message)
	}

	// SEC-P2-02: HMAC 공유 시크릿 추출 및 설정
	if ackPayload.HMACSecret != "" {
		if err := c.signer.SetSecretFromHex(ackPayload.HMACSecret); err != nil {
			return fmt.Errorf("HMAC 시크릿 설정 실패: %w", err)
		}
	}

	return nil
}

// sendConnect는 연결 메시지를 전송합니다.
// 이 함수는 연결 과정 중에 호출되므로 StateConnected 체크를 우회합니다.
// FR-P2-02: JWT 토큰을 페이로드에 포함하여 메시지 기반 인증 수행.
func (c *Client) sendConnect() error {
	c.lastExecIDMu.RLock()
	lastExecID := c.lastExecID
	c.lastExecIDMu.RUnlock()

	payload := ws.AgentConnectPayload{
		Version:      c.version,
		Capabilities: c.capabilities,
		LastExecID:   lastExecID,
		Token:        c.token,
	}

	// sendMessage 대신 직접 전송 (연결 과정 중이므로)
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("페이로드 직렬화 실패: %w", err)
	}

	msg := ws.AgentMessage{
		Type:      ws.AgentMsgConnect,
		ID:        uuid.New().String(),
		Timestamp: time.Now(),
		Payload:   payloadBytes,
	}

	c.connMu.RLock()
	conn := c.conn
	c.connMu.RUnlock()

	if conn == nil {
		return errors.New("연결이 없습니다")
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("메시지 직렬화 실패: %w", err)
	}

	c.writeMu.Lock()
	_ = conn.SetWriteDeadline(time.Now().Add(WriteTimeout))
	err = conn.WriteMessage(websocket.TextMessage, data)
	c.writeMu.Unlock()

	if err != nil {
		return fmt.Errorf("메시지 전송 실패: %w", err)
	}

	return nil
}

// Disconnect는 서버와의 연결을 종료합니다.
func (c *Client) Disconnect(reason string) error {
	if c.State() == StateDisconnected || c.State() == StateClosed {
		return nil
	}

	// 연결 해제 메시지 전송
	payload := ws.AgentDisconnectPayload{
		Reason: reason,
	}
	_ = c.sendMessage(ws.AgentMsgDisconnect, payload)

	c.state.Store(int32(StateClosed))

	// 하트비트 중지
	c.stopHeartbeat()

	// 연결 종료
	c.closeConnection()

	// done 채널 닫기
	select {
	case <-c.done:
		// 이미 닫혀 있음
	default:
		close(c.done)
	}

	return nil
}

// closeConnection은 WebSocket 연결을 닫습니다.
func (c *Client) closeConnection() {
	c.connMu.Lock()
	defer c.connMu.Unlock()

	if c.conn != nil {
		c.writeMu.Lock()
		_ = c.conn.WriteMessage(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
		)
		c.writeMu.Unlock()
		_ = c.conn.Close()
		c.conn = nil
	}
}

// Send는 메시지를 서버로 전송합니다.
func (c *Client) Send(msg ws.AgentMessage) error {
	if c.State() != StateConnected {
		return errors.New("연결되지 않은 상태입니다")
	}

	c.connMu.RLock()
	conn := c.conn
	c.connMu.RUnlock()

	if conn == nil {
		return errors.New("연결이 없습니다")
	}

	// SEC-P2-02: 중요 메시지에 HMAC-SHA256 서명 추가
	if c.signer != nil {
		if err := c.signer.Sign(&msg); err != nil {
			return fmt.Errorf("HMAC 서명 실패: %w", err)
		}
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("메시지 직렬화 실패: %w", err)
	}

	c.writeMu.Lock()
	_ = conn.SetWriteDeadline(time.Now().Add(WriteTimeout))
	err = conn.WriteMessage(websocket.TextMessage, data)
	c.writeMu.Unlock()

	if err != nil {
		return fmt.Errorf("메시지 전송 실패: %w", err)
	}

	return nil
}

// sendMessage는 타입과 페이로드로 메시지를 생성하여 전송합니다.
func (c *Client) sendMessage(msgType string, payload interface{}) error {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("페이로드 직렬화 실패: %w", err)
	}

	msg := ws.AgentMessage{
		Type:      msgType,
		ID:        uuid.New().String(),
		Timestamp: time.Now(),
		Payload:   payloadBytes,
	}

	return c.Send(msg)
}

// Messages는 수신된 메시지 채널을 반환합니다.
func (c *Client) Messages() <-chan ws.AgentMessage {
	return c.messages
}

// Done은 클라이언트 종료 채널을 반환합니다.
func (c *Client) Done() <-chan struct{} {
	return c.done
}

// ResetDone은 재연결 시 done 채널을 재생성합니다.
// handleDisconnect에서 재연결 성공 후 호출됩니다.
func (c *Client) ResetDone() {
	select {
	case <-c.done:
		// done 채널이 닫혀 있으면 새로 생성
		c.done = make(chan struct{})
	default:
		// 아직 열려 있으면 그대로 둠
	}
}

// State는 현재 연결 상태를 반환합니다.
func (c *Client) State() ConnectionState {
	return ConnectionState(c.state.Load())
}

// StartHeartbeat는 하트비트 전송을 시작합니다.
// REQ-E-06: 30초 간격 하트비트
func (c *Client) StartHeartbeat(ctx context.Context) {
	c.heartbeatMu.Lock()
	if c.heartbeatCancel != nil {
		c.heartbeatCancel()
	}
	heartbeatCtx, cancel := context.WithCancel(ctx)
	c.heartbeatCancel = cancel
	c.heartbeatMu.Unlock()

	// Initialize lastHeartbeat to prevent premature timeout before first response
	c.lastHeartbeatMu.Lock()
	c.lastHeartbeat = time.Now()
	c.lastHeartbeatMu.Unlock()

	go c.heartbeatLoop(heartbeatCtx)
}

// stopHeartbeat는 하트비트 전송을 중지합니다.
func (c *Client) stopHeartbeat() {
	c.heartbeatMu.Lock()
	defer c.heartbeatMu.Unlock()

	if c.heartbeatCancel != nil {
		c.heartbeatCancel()
		c.heartbeatCancel = nil
	}
}

// heartbeatLoop는 주기적으로 하트비트를 전송합니다.
func (c *Client) heartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.done:
			return
		case <-ticker.C:
			if c.State() != StateConnected {
				continue
			}

			// 하트비트 타임아웃 확인
			c.lastHeartbeatMu.RLock()
			lastHeartbeat := c.lastHeartbeat
			c.lastHeartbeatMu.RUnlock()

			if !lastHeartbeat.IsZero() && time.Since(lastHeartbeat) > HeartbeatTimeout {
				// 하트비트 타임아웃 - 재연결 시도
				go c.handleDisconnect(ctx, "하트비트 타임아웃")
				return
			}

			// 하트비트 전송 (SPEC-AI-003 M3 T-26: MCP 서버 상태 포함)
			payload := ws.AgentHeartbeatPayload{
				Timestamp: time.Now(),
			}
			if c.heartbeatEnricher != nil {
				payload.MCPServeStatus = c.heartbeatEnricher()
			}
			if err := c.sendMessage(ws.AgentMsgHeartbeat, payload); err != nil {
				// 전송 실패 시 재연결 시도
				go c.handleDisconnect(ctx, fmt.Sprintf("하트비트 전송 실패: %v", err))
				return
			}
		}
	}
}

// readLoop는 메시지를 지속적으로 수신합니다.
// gorilla/websocket은 ReadMessage() 에러 후 재시도 시 panic하므로,
// 에러 발생 시 즉시 루프를 종료하고 재연결을 시도합니다.
func (c *Client) readLoop(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[STABILITY] readLoop panic 복구: %v", r)
			go c.handleDisconnect(ctx, fmt.Sprintf("readLoop panic 복구: %v", r))
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.done:
			return
		default:
		}

		c.connMu.RLock()
		conn := c.conn
		c.connMu.RUnlock()

		if conn == nil {
			return
		}

		// gorilla/websocket은 ReadMessage() 에러 후 동일 conn에서 재시도하면 panic합니다.
		// ReadDeadline을 사용하지 않고 blocking read를 하되,
		// 종료 시 conn.Close()로 ReadMessage()를 unblock합니다.
		_, data, err := conn.ReadMessage()
		if err != nil {
			// 모든 read 에러는 연결 끊김으로 처리 (재시도 금지)
			go c.handleDisconnect(ctx, fmt.Sprintf("연결 에러: %v", err))
			return
		}

		var msg ws.AgentMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}

		// 하트비트 응답 처리
		if msg.Type == ws.AgentMsgHeartbeat {
			c.lastHeartbeatMu.Lock()
			c.lastHeartbeat = time.Now()
			c.lastHeartbeatMu.Unlock()
			continue
		}

		// SEC-P2-02: 수신된 중요 메시지의 HMAC-SHA256 서명 검증
		if c.signer != nil && !c.signer.Verify(&msg) {
			// 서명 검증 실패 시 메시지 무시
			continue
		}

		// 핸들러가 있으면 핸들러로 전달
		if c.handler != nil {
			go func() { _ = c.handler.HandleMessage(ctx, msg) }()
		}

		// 메시지 채널로 전달
		select {
		case c.messages <- msg:
		default:
			// 채널이 가득 찬 경우 경고 로깅
			log.Printf("[STABILITY] 메시지 채널 버퍼 오버플로 - 메시지 타입: %s, 채널 용량: %d", msg.Type, cap(c.messages))
		}
	}
}

// UpdateToken은 런타임에 토큰을 업데이트합니다.
// TokenRefresher가 갱신한 토큰을 반영할 때 사용합니다.
func (c *Client) UpdateToken(token string) {
	c.connMu.Lock()
	defer c.connMu.Unlock()
	c.token = token
}

// SetTokenRefreshFunc은 재연결 전 토큰을 갱신하는 콜백을 설정합니다.
func (c *Client) SetTokenRefreshFunc(fn func() (string, error)) {
	c.tokenRefreshFn = fn
}

// SetHeartbeatEnricher는 하트비트 전송 시 MCP 서버 상태를 포함하는 콜백을 설정합니다 (SPEC-AI-003 M3 T-26).
func (c *Client) SetHeartbeatEnricher(fn func() string) {
	c.heartbeatEnricher = fn
}

// handleDisconnect는 연결 끊김을 처리하고 재연결을 시도합니다.
// REQ-E-08: 지수 백오프 재연결
// CAS 가드: heartbeatLoop과 readLoop이 동시에 호출해도 하나만 진행
func (c *Client) handleDisconnect(ctx context.Context, reason string) {
	// CAS: StateConnected -> StateReconnecting 전환이 성공한 고루틴만 진행.
	// 이미 Reconnecting/Closed/Disconnected 상태면 즉시 반환하여 중복 재연결 방지.
	if !c.state.CompareAndSwap(int32(StateConnected), int32(StateReconnecting)) {
		return
	}

	log.Printf("[STABILITY] 연결 끊김 감지: %s", reason)
	c.closeConnection()

	// 재연결 시도
	for c.reconnectStrategy.CanRetry() {
		select {
		case <-ctx.Done():
			c.state.Store(int32(StateDisconnected))
			return
		case <-c.done:
			return
		default:
		}

		delay := c.reconnectStrategy.NextDelay()
		attempt := c.reconnectStrategy.CurrentAttempt()
		log.Printf("[STABILITY] 재연결 시도 %d (대기: %v)", attempt, delay)

		select {
		case <-ctx.Done():
			c.state.Store(int32(StateDisconnected))
			return
		case <-c.done:
			return
		case <-time.After(delay):
		}

		// 재연결 전 토큰 갱신 시도
		if c.tokenRefreshFn != nil {
			if newToken, err := c.tokenRefreshFn(); err == nil {
				c.UpdateToken(newToken)
			} else {
				log.Printf("재연결 전 토큰 갱신 실패: %v", err)
			}
		}

		if err := c.Connect(ctx); err == nil {
			log.Printf("[STABILITY] 재연결 성공 (시도 %d)", attempt)
			// 재연결 성공 - 하트비트 재시작
			c.StartHeartbeat(ctx)
			// FR-P2-04: 미완료 태스크 복구 시도
			go c.RecoverTasks(ctx)
			// REQ-M3-04: 핸들러가 ReconnectionHandler를 구현하면 상태 복구 수행
			if rh, ok := c.handler.(ReconnectionHandler); ok {
				go func() { _ = rh.OnReconnected(ctx) }()
			}
			return
		} else {
			log.Printf("[STABILITY] 재연결 실패 (시도 %d): %v", attempt, err)
		}
	}

	// 재연결 실패
	log.Printf("[STABILITY] 최대 재연결 시도 소진 - 연결 종료")
	c.state.Store(int32(StateDisconnected))
}

// SetLastExecID는 마지막으로 처리한 실행 ID를 설정합니다.
func (c *Client) SetLastExecID(execID string) {
	c.lastExecIDMu.Lock()
	defer c.lastExecIDMu.Unlock()
	c.lastExecID = execID
}

// GetLastExecID는 마지막으로 처리한 실행 ID를 반환합니다.
func (c *Client) GetLastExecID() string {
	c.lastExecIDMu.RLock()
	defer c.lastExecIDMu.RUnlock()
	return c.lastExecID
}

// SendTaskProgress는 작업 진행 상황을 서버로 전송합니다.
func (c *Client) SendTaskProgress(payload ws.TaskProgressPayload) error {
	return c.sendMessage(ws.AgentMsgTaskProg, payload)
}

// SendTaskResult는 작업 결과를 서버로 전송합니다.
func (c *Client) SendTaskResult(payload ws.TaskResultPayload) error {
	c.SetLastExecID(payload.ExecutionID)
	return c.sendMessage(ws.AgentMsgTaskResult, payload)
}

// SendTaskError는 작업 오류를 서버로 전송합니다.
func (c *Client) SendTaskError(payload ws.TaskErrorPayload) error {
	c.SetLastExecID(payload.ExecutionID)
	return c.sendMessage(ws.AgentMsgTaskError, payload)
}

// SendBuildResult는 빌드 결과를 서버로 전송합니다 (FR-P3-01).
func (c *Client) SendBuildResult(payload ws.BuildResultPayload) error {
	c.SetLastExecID(payload.ExecutionID)
	return c.sendMessage(ws.AgentMsgBuildResult, payload)
}

// SendTestResult는 테스트 결과를 서버로 전송합니다 (FR-P3-02).
func (c *Client) SendTestResult(payload ws.TestResultPayload) error {
	c.SetLastExecID(payload.ExecutionID)
	return c.sendMessage(ws.AgentMsgTestResult, payload)
}

// SendQAResult는 QA 결과를 서버로 전송합니다 (FR-P3-03).
func (c *Client) SendQAResult(payload ws.QAResultPayload) error {
	c.SetLastExecID(payload.ExecutionID)
	return c.sendMessage(ws.AgentMsgQAResult, payload)
}

// SendComputerResult는 Computer Use 결과를 서버로 전송합니다 (SPEC-COMPUTER-USE-001).
func (c *Client) SendComputerResult(payload ws.ComputerResultPayload) error {
	c.SetLastExecID(payload.ExecutionID)
	return c.sendMessage(ws.AgentMsgComputerResult, payload)
}

// SendMCPCodegenProgress는 코드 생성 진행 상황을 서버로 전송합니다 (SPEC-SELF-EXPAND-001).
func (c *Client) SendMCPCodegenProgress(msgID string, payload ws.MCPCodegenProgressPayload) error {
	return c.sendMessageWithID(ws.AgentMsgMCPCodegenProgress, msgID, payload)
}

// SendMCPCodegenResult는 코드 생성 결과를 서버로 전송합니다 (SPEC-SELF-EXPAND-001).
func (c *Client) SendMCPCodegenResult(msgID string, payload ws.MCPCodegenResultPayload) error {
	return c.sendMessageWithID(ws.AgentMsgMCPCodegenResult, msgID, payload)
}

// SendMCPDeployResult는 배포 결과를 서버로 전송합니다 (SPEC-SELF-EXPAND-001).
func (c *Client) SendMCPDeployResult(msgID string, payload ws.MCPDeployResultPayload) error {
	return c.sendMessageWithID(ws.AgentMsgMCPDeployResult, msgID, payload)
}

// SendMCPHealthReport는 헬스 리포트를 서버로 전송합니다 (SPEC-SELF-EXPAND-001).
func (c *Client) SendMCPHealthReport(payload ws.MCPHealthReportPayload) error {
	return c.sendMessage(ws.AgentMsgMCPHealthReport, payload)
}

// sendMessageWithID는 특정 ID를 가진 메시지를 생성하여 전송합니다.
func (c *Client) sendMessageWithID(msgType, id string, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("%s 직렬화 실패: %w", msgType, err)
	}

	msg := ws.AgentMessage{
		Type:      msgType,
		ID:        id,
		Timestamp: time.Now(),
		Payload:   data,
	}

	return c.Send(msg)
}

// TaskTracker는 태스크 추적기를 반환합니다 (FR-P2-04).
func (c *Client) TaskTracker() *TaskTracker {
	return c.taskTracker
}

// RecoverTasks는 재연결 후 미완료 태스크의 상태를 서버에 조회하여 복구합니다 (FR-P2-04).
// 서버가 이미 완료한 태스크는 추적 목록에서 제거하고,
// 서버에서 찾을 수 없는 태스크(not_found)도 제거합니다.
// "pending" 상태인 태스크만 추적 목록에 유지합니다.
func (c *Client) RecoverTasks(ctx context.Context) {
	activeTasks := c.taskTracker.GetActiveTasks()
	if len(activeTasks) == 0 {
		return
	}

	log.Printf("[FR-P2-04] 재연결 후 미완료 태스크 %d개 복구 시도", len(activeTasks))

	httpBaseURL := wsToHTTPURL(c.serverURL)

	for _, execID := range activeTasks {
		select {
		case <-ctx.Done():
			return
		default:
		}

		statusURL := httpBaseURL + "/api/v1/agent/tasks/" + execID + "/status"

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, statusURL, nil)
		if err != nil {
			log.Printf("[FR-P2-04] 태스크 상태 조회 요청 생성 실패 (exec=%s): %v", execID, err)
			continue
		}
		req.Header.Set("Authorization", "Bearer "+c.token)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Printf("[FR-P2-04] 태스크 상태 조회 실패 (exec=%s): %v", execID, err)
			continue
		}

		var result struct {
			Success bool `json:"success"`
			Data    struct {
				Status string `json:"status"`
			} `json:"data"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			_ = resp.Body.Close()
			log.Printf("[FR-P2-04] 태스크 상태 응답 파싱 실패 (exec=%s): %v", execID, err)
			continue
		}
		_ = resp.Body.Close()

		switch result.Data.Status {
		case "completed", "not_found":
			// 서버에서 완료됐거나 찾을 수 없는 태스크는 추적 목록에서 제거
			c.taskTracker.Complete(execID)
			log.Printf("[FR-P2-04] 태스크 추적 제거 (exec=%s, status=%s)", execID, result.Data.Status)
		case "pending":
			// 아직 진행 중인 태스크는 유지
			log.Printf("[FR-P2-04] 태스크 계속 대기 (exec=%s, status=pending)", execID)
		default:
			log.Printf("[FR-P2-04] 알 수 없는 태스크 상태 (exec=%s, status=%s)", execID, result.Data.Status)
		}
	}

	log.Printf("[FR-P2-04] 태스크 복구 완료, 남은 활성 태스크: %d", c.taskTracker.GetActiveTaskCount())
}

// wsToHTTPURL은 WebSocket URL을 HTTP URL로 변환합니다 (FR-P2-04).
// wss:// -> https://, ws:// -> http://
// 경로 부분은 제거하고 호스트까지만 반환합니다.
func wsToHTTPURL(wsURL string) string {
	u, err := url.Parse(wsURL)
	if err != nil {
		// 파싱 실패 시 단순 문자열 치환
		result := strings.Replace(wsURL, "wss://", "https://", 1)
		result = strings.Replace(result, "ws://", "http://", 1)
		return result
	}

	scheme := "http"
	if u.Scheme == "wss" {
		scheme = "https"
	}

	return scheme + "://" + u.Host
}
