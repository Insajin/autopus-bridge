package client

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"sync/atomic"

	"github.com/insajin/autopus-codex-rpc/protocol"
)

// NotificationHandler는 JSON-RPC 알림을 처리하는 핸들러 함수 타입이다.
type NotificationHandler func(method string, params json.RawMessage)

// Client는 stdio 기반 JSON-RPC 2.0 클라이언트이다.
// 동시성 안전하게 요청/응답을 관리하며, 알림을 등록된 핸들러로 디스패치한다.
// 외부 의존성 없이 stdlib만 사용한다.
type Client struct {
	stdin  io.WriteCloser
	stdout *bufio.Scanner
	logger Logger

	nextID    atomic.Int64
	pending   map[int64]chan *protocol.JSONRPCResponse
	pendingMu sync.Mutex

	notifyHandlers map[string]NotificationHandler
	handlersMu     sync.RWMutex

	writeMu sync.Mutex

	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{} // readLoop이 종료되면 닫힌다.
}

// NewJSONRPCClient는 새로운 JSON-RPC 클라이언트를 생성하고 readLoop을 시작한다.
// stdin은 서버로 요청을 전송하는 WriteCloser이다.
// stdout은 서버 응답을 읽는 Reader이다.
// logger는 로깅 인터페이스이다. 로깅이 필요 없으면 NopLogger()를 사용한다.
func NewJSONRPCClient(stdin io.WriteCloser, stdout io.Reader, logger Logger) *Client {
	ctx, cancel := context.WithCancel(context.Background())

	scanner := bufio.NewScanner(stdout)
	// 기본 버퍼를 1MB로 확장하여 큰 JSON 응답을 처리한다.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	c := &Client{
		stdin:          stdin,
		stdout:         scanner,
		logger:         logger,
		pending:        make(map[int64]chan *protocol.JSONRPCResponse),
		notifyHandlers: make(map[string]NotificationHandler),
		ctx:            ctx,
		cancel:         cancel,
		done:           make(chan struct{}),
	}

	go c.readLoop()
	return c
}

// Call은 JSON-RPC 요청을 전송하고 응답을 대기한다.
// ctx 타임아웃 또는 취소로 중단될 수 있다.
// params는 json.Marshal로 직렬화되어 전송된다.
func (c *Client) Call(ctx context.Context, method string, params interface{}) (*json.RawMessage, error) {
	// 클라이언트 종료 여부 확인
	select {
	case <-c.ctx.Done():
		return nil, fmt.Errorf("JSON-RPC 클라이언트가 종료되었습니다")
	default:
	}

	id := c.nextID.Add(1)

	req := protocol.JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		ID:      id,
	}

	// params를 json.RawMessage로 직렬화
	if params != nil {
		paramsData, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("params 직렬화 실패: %w", err)
		}
		req.Params = paramsData
	}

	// 응답 채널 등록
	respCh := make(chan *protocol.JSONRPCResponse, 1)
	c.pendingMu.Lock()
	c.pending[id] = respCh
	c.pendingMu.Unlock()

	// 요청 완료 후 채널 정리 보장
	defer func() {
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
	}()

	// 요청 전송
	if err := c.writeJSON(req); err != nil {
		return nil, err
	}

	c.logger.Debug("JSON-RPC 요청 전송", "id", id, "method", method)

	// 응답 대기
	// c.ctx.Done(): Close() 호출 또는 다른 취소 시 종료
	// ctx.Done(): 호출자 타임아웃 또는 취소
	// respCh: 실제 응답 수신
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("JSON-RPC 호출 타임아웃 (method=%s): %w", method, ctx.Err())
	case <-c.ctx.Done():
		return nil, fmt.Errorf("JSON-RPC 클라이언트가 종료되었습니다")
	case resp, ok := <-respCh:
		if !ok || resp == nil {
			return nil, fmt.Errorf("JSON-RPC 클라이언트가 종료되었습니다")
		}
		if resp.Error != nil {
			return nil, protocol.MapJSONRPCError(resp.Error)
		}
		return resp.Result, nil
	}
}

// Notify는 JSON-RPC 알림을 전송한다 (응답을 기대하지 않음).
// params가 nil이면 params 필드 없이 전송한다.
func (c *Client) Notify(method string, params interface{}) error {
	select {
	case <-c.ctx.Done():
		return fmt.Errorf("JSON-RPC 클라이언트가 종료되었습니다")
	default:
	}

	notif := protocol.JSONRPCNotification{
		JSONRPC: "2.0",
		Method:  method,
	}

	if params != nil {
		paramsData, err := json.Marshal(params)
		if err != nil {
			return fmt.Errorf("params 직렬화 실패: %w", err)
		}
		notif.Params = paramsData
	}

	c.logger.Debug("JSON-RPC 알림 전송", "method", method)
	return c.writeJSON(notif)
}

// Done은 readLoop가 종료되면 닫히는 채널을 반환한다.
// 이를 통해 호출자가 연결 종료를 감지할 수 있다.
func (c *Client) Done() <-chan struct{} {
	return c.done
}

// OnNotification은 지정된 메서드에 대한 알림 핸들러를 등록한다.
// 같은 메서드에 핸들러를 중복 등록하면 마지막 핸들러가 사용된다.
func (c *Client) OnNotification(method string, handler NotificationHandler) {
	c.handlersMu.Lock()
	defer c.handlersMu.Unlock()
	c.notifyHandlers[method] = handler
}

// Close는 클라이언트를 종료하고 대기 중인 모든 요청을 취소한다.
// 호출 후 모든 진행 중인 Call은 에러와 함께 반환된다.
// stdin을 닫아 readLoop에 EOF를 전달한다.
func (c *Client) Close() error {
	c.cancel()

	// 대기 중인 모든 채널을 닫아 종료 신호 전달
	c.pendingMu.Lock()
	for id, ch := range c.pending {
		close(ch)
		delete(c.pending, id)
	}
	c.pendingMu.Unlock()

	// stdin 닫기 (서버에 EOF 신호 전달하여 readLoop 종료 유도)
	return c.stdin.Close()
}

// writeJSON은 임의의 구조체를 JSON으로 직렬화하여 stdin에 줄바꿈과 함께 기록한다.
func (c *Client) writeJSON(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("직렬화 실패: %w", err)
	}

	// newline-delimited JSON 형식
	data = append(data, '\n')

	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	_, err = c.stdin.Write(data)
	if err != nil {
		return fmt.Errorf("전송 실패: %w", err)
	}
	return nil
}

// readLoop은 stdout에서 JSON-RPC 메시지를 읽고 라우팅하는 고루틴이다.
// id 필드가 있고 method가 없으면 응답으로, method가 있고 id가 없으면 알림으로 처리한다.
// EOF 또는 에러 시 모든 대기 중인 채널을 닫고 종료한다.
func (c *Client) readLoop() {
	defer close(c.done)

	for c.stdout.Scan() {
		line := c.stdout.Bytes()
		if len(line) == 0 {
			continue
		}

		// JSON 메시지인지 빠르게 확인
		if line[0] != '{' {
			c.logger.Debug("JSON이 아닌 라인 무시", "line", string(line))
			continue
		}

		// id와 method 필드로 응답/알림 구분
		var raw struct {
			ID     *json.RawMessage `json:"id"`
			Method string           `json:"method"`
		}
		if err := json.Unmarshal(line, &raw); err != nil {
			c.logger.Warn("JSON 파싱 실패", "err", err)
			continue
		}

		if raw.ID != nil && raw.Method == "" {
			// 응답 메시지: id가 있고 method가 없음
			c.handleResponse(line)
		} else if raw.Method != "" && raw.ID == nil {
			// 알림 메시지: method가 있고 id가 없음
			c.handleNotification(line)
		}
	}

	if err := c.stdout.Err(); err != nil {
		c.logger.Debug("readLoop 스캐너 에러", "err", err)
	}

	// 남은 대기 중인 채널 모두 닫기
	c.pendingMu.Lock()
	for id, ch := range c.pending {
		close(ch)
		delete(c.pending, id)
	}
	c.pendingMu.Unlock()
}

// handleResponse는 응답 메시지를 파싱하여 해당 pending 채널로 전달한다.
func (c *Client) handleResponse(data []byte) {
	var resp protocol.JSONRPCResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		c.logger.Warn("응답 파싱 실패", "err", err)
		return
	}

	if resp.ID == nil {
		c.logger.Warn("응답에 ID가 없음")
		return
	}

	c.pendingMu.Lock()
	ch, ok := c.pending[*resp.ID]
	c.pendingMu.Unlock()

	if !ok {
		c.logger.Warn("대기 중이 아닌 ID의 응답 수신", "id", *resp.ID)
		return
	}

	// 채널이 이미 닫혀 있을 경우를 대비해 recover
	defer func() {
		if r := recover(); r != nil {
			c.logger.Debug("응답 채널이 이미 닫혀 있음", "id", *resp.ID)
		}
	}()

	ch <- &resp
}

// handleNotification은 알림 메시지를 파싱하여 등록된 핸들러를 호출한다.
// 핸들러는 readLoop 고루틴에서 직접 호출되어 알림 순서가 보장된다.
// 핸들러는 블록킹 작업을 수행하지 않아야 한다 (예: 버퍼 채널 전송).
func (c *Client) handleNotification(data []byte) {
	var notif protocol.JSONRPCNotification
	if err := json.Unmarshal(data, &notif); err != nil {
		c.logger.Warn("알림 파싱 실패", "err", err)
		return
	}

	c.handlersMu.RLock()
	handler, ok := c.notifyHandlers[notif.Method]
	c.handlersMu.RUnlock()

	if ok {
		// readLoop에서 직접 호출하여 알림 도착 순서 보장
		// 핸들러는 논블로킹이어야 한다 (버퍼 채널 사용 권장)
		handler(notif.Method, notif.Params)
	} else {
		c.logger.Debug("등록되지 않은 알림 메서드", "method", notif.Method)
	}
}
