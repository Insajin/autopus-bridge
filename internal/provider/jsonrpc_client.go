package provider

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"sync/atomic"

	"github.com/rs/zerolog"
)

// NotificationHandler는 JSON-RPC 알림 핸들러 함수 타입입니다.
type NotificationHandler func(method string, params json.RawMessage)

// JSONRPCClient는 stdio 기반 JSON-RPC 2.0 클라이언트입니다.
// 동시성 안전하게 요청/응답을 관리하며, 알림을 핸들러로 디스패치합니다.
type JSONRPCClient struct {
	stdin          io.WriteCloser
	stdout         *bufio.Scanner
	nextID         atomic.Int64
	pending        map[int64]chan *JSONRPCResponse
	pendingMu      sync.Mutex
	notifyHandlers map[string]NotificationHandler
	handlersMu     sync.RWMutex
	writeMu        sync.Mutex
	ctx            context.Context
	cancel         context.CancelFunc
	done           chan struct{}
	logger         zerolog.Logger
}

// NewJSONRPCClient는 새로운 JSON-RPC 클라이언트를 생성하고 readLoop을 시작합니다.
// stdin은 프로세스의 표준 입력으로 요청을 전송하는 데 사용됩니다.
// stdout은 프로세스의 표준 출력에서 응답을 수신하는 데 사용됩니다.
func NewJSONRPCClient(stdin io.WriteCloser, stdout io.Reader, logger zerolog.Logger) *JSONRPCClient {
	ctx, cancel := context.WithCancel(context.Background())
	scanner := bufio.NewScanner(stdout)
	// 기본 버퍼 크기를 1MB로 확장 (큰 JSON 응답 처리)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	c := &JSONRPCClient{
		stdin:          stdin,
		stdout:         scanner,
		pending:        make(map[int64]chan *JSONRPCResponse),
		notifyHandlers: make(map[string]NotificationHandler),
		ctx:            ctx,
		cancel:         cancel,
		done:           make(chan struct{}),
		logger:         logger.With().Str("component", "jsonrpc-client").Logger(),
	}

	go c.readLoop()
	return c
}

// Call은 JSON-RPC 요청을 전송하고 응답을 대기합니다.
// ctx의 타임아웃이나 취소에 의해 중단될 수 있습니다.
func (c *JSONRPCClient) Call(ctx context.Context, method string, params interface{}) (*json.RawMessage, error) {
	// 클라이언트 종료 확인
	select {
	case <-c.done:
		return nil, ErrConnectionClosed
	default:
	}

	// 자동 증가 ID 생성
	id := c.nextID.Add(1)

	// 요청 구성
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		ID:      id,
	}

	// params 직렬화
	if params != nil {
		paramsData, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("params 직렬화 실패: %w", err)
		}
		req.Params = paramsData
	}

	// 응답 채널 등록
	respCh := make(chan *JSONRPCResponse, 1)
	c.pendingMu.Lock()
	c.pending[id] = respCh
	c.pendingMu.Unlock()

	// 정리 보장
	defer func() {
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
	}()

	// 요청 전송
	if err := c.writeRequest(req); err != nil {
		return nil, err
	}

	c.logger.Debug().
		Int64("id", id).
		Str("method", method).
		Msg("JSON-RPC 요청 전송")

	// 응답 대기
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("JSON-RPC 호출 타임아웃 (method=%s): %w", method, ctx.Err())
	case <-c.done:
		return nil, ErrConnectionClosed
	case resp := <-respCh:
		if resp == nil {
			return nil, ErrConnectionClosed
		}
		if resp.Error != nil {
			return nil, MapJSONRPCError(resp.Error)
		}
		return resp.Result, nil
	}
}

// Notify는 JSON-RPC 알림을 전송합니다 (응답을 기대하지 않음).
func (c *JSONRPCClient) Notify(method string, params interface{}) error {
	// 클라이언트 종료 확인
	select {
	case <-c.done:
		return ErrConnectionClosed
	default:
	}

	notif := JSONRPCNotification{
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

	data, err := json.Marshal(notif)
	if err != nil {
		return fmt.Errorf("알림 직렬화 실패: %w", err)
	}

	// 줄바꿈 추가
	data = append(data, '\n')

	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	_, err = c.stdin.Write(data)
	if err != nil {
		return fmt.Errorf("알림 전송 실패: %w", err)
	}

	c.logger.Debug().
		Str("method", method).
		Msg("JSON-RPC 알림 전송")

	return nil
}

// OnNotification은 지정된 메서드에 대한 알림 핸들러를 등록합니다.
func (c *JSONRPCClient) OnNotification(method string, handler NotificationHandler) {
	c.handlersMu.Lock()
	defer c.handlersMu.Unlock()
	c.notifyHandlers[method] = handler
}

// Close는 클라이언트를 종료하고 대기 중인 모든 요청을 취소합니다.
func (c *JSONRPCClient) Close() error {
	c.cancel()

	// 모든 대기 중인 채널에 nil 전송하여 종료 알림
	c.pendingMu.Lock()
	for id, ch := range c.pending {
		close(ch)
		delete(c.pending, id)
	}
	c.pendingMu.Unlock()

	// stdin 닫기
	err := c.stdin.Close()

	// readLoop 종료 대기
	<-c.done

	return err
}

// writeRequest는 JSON-RPC 요청을 stdin에 기록합니다.
func (c *JSONRPCClient) writeRequest(req JSONRPCRequest) error {
	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("요청 직렬화 실패: %w", err)
	}

	// 줄바꿈 추가 (newline-delimited JSON)
	data = append(data, '\n')

	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	_, err = c.stdin.Write(data)
	if err != nil {
		return fmt.Errorf("요청 전송 실패: %w", err)
	}

	return nil
}

// readLoop은 stdout에서 JSON-RPC 메시지를 읽고 디스패치하는 고루틴입니다.
// 각 줄을 JSON으로 파싱하여:
//   - "id" 필드가 있으면 응답으로 처리하여 pending 채널에 전송
//   - "id" 필드가 없으면 알림으로 처리하여 등록된 핸들러에 디스패치
//
// EOF 또는 에러 시 모든 pending 채널을 닫고 종료합니다.
func (c *JSONRPCClient) readLoop() {
	defer close(c.done)

	for c.stdout.Scan() {
		line := c.stdout.Bytes()
		if len(line) == 0 {
			continue
		}

		// JSON인지 빠르게 확인
		if line[0] != '{' {
			c.logger.Trace().
				Str("line", string(line)).
				Msg("JSON이 아닌 라인 무시")
			continue
		}

		// id 필드 유무로 응답/알림 구분
		// 먼저 가볍게 raw 메시지로 파싱
		var raw struct {
			ID     *json.RawMessage `json:"id"`
			Method string           `json:"method"`
		}
		if err := json.Unmarshal(line, &raw); err != nil {
			c.logger.Warn().
				Err(err).
				Str("line", string(line)).
				Msg("JSON 파싱 실패")
			continue
		}

		if raw.ID != nil && raw.Method == "" {
			// 응답 메시지 (id가 있고 method가 없음)
			c.handleResponse(line)
		} else if raw.Method != "" && raw.ID == nil {
			// 알림 메시지 (method가 있고 id가 없음)
			c.handleNotification(line)
		} else if raw.ID != nil && raw.Method != "" {
			// 요청 메시지 (서버에서 클라이언트로의 요청 - 현재 무시)
			c.logger.Debug().
				Str("method", raw.Method).
				Msg("서버 요청 수신 (현재 미지원)")
		}
	}

	// 스캐너 에러 확인
	if err := c.stdout.Err(); err != nil {
		c.logger.Debug().Err(err).Msg("readLoop 스캐너 에러")
	}

	// 모든 대기 중인 채널 닫기
	c.pendingMu.Lock()
	for id, ch := range c.pending {
		close(ch)
		delete(c.pending, id)
	}
	c.pendingMu.Unlock()
}

// handleResponse는 응답 메시지를 처리합니다.
func (c *JSONRPCClient) handleResponse(data []byte) {
	var resp JSONRPCResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		c.logger.Warn().
			Err(err).
			Msg("응답 파싱 실패")
		return
	}

	c.pendingMu.Lock()
	ch, ok := c.pending[resp.ID]
	c.pendingMu.Unlock()

	if !ok {
		c.logger.Warn().
			Int64("id", resp.ID).
			Msg("대기 중이 아닌 ID의 응답 수신")
		return
	}

	// 채널이 이미 닫혔을 수 있으므로 recover
	defer func() {
		if r := recover(); r != nil {
			c.logger.Debug().
				Int64("id", resp.ID).
				Msg("응답 채널이 이미 닫혀 있습니다")
		}
	}()

	ch <- &resp
}

// handleNotification은 알림 메시지를 처리합니다.
func (c *JSONRPCClient) handleNotification(data []byte) {
	var notif JSONRPCNotification
	if err := json.Unmarshal(data, &notif); err != nil {
		c.logger.Warn().
			Err(err).
			Msg("알림 파싱 실패")
		return
	}

	c.handlersMu.RLock()
	handler, ok := c.notifyHandlers[notif.Method]
	c.handlersMu.RUnlock()

	if ok {
		// 핸들러를 별도 고루틴으로 실행하여 readLoop 차단 방지
		go handler(notif.Method, notif.Params)
	} else {
		c.logger.Trace().
			Str("method", notif.Method).
			Msg("등록되지 않은 알림 메서드")
	}
}
