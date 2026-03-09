package apiclient

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

const (
	// eventTypeAgentTyping은 에이전트 타이핑 이벤트 타입입니다.
	eventTypeAgentTyping = "agent_typing"
)

const (
	// sseReadBufferSize는 SSE 스트림 읽기 버퍼 크기입니다 (1MB).
	sseReadBufferSize = 1024 * 1024
	// sseMaxRetry는 최대 재연결 시도 횟수입니다.
	sseMaxRetry = 3
)

// SSESubscriber는 Unified SSE 엔드포인트에 연결합니다.
// 자동 재연결(지수 백오프)을 지원합니다.
type SSESubscriber struct {
	baseURL     string
	workspaceID string
	token       string
	logger      zerolog.Logger
	maxRetry    int
	httpClient  *http.Client
}

// NewSSESubscriber는 새 SSESubscriber를 생성합니다.
// SSE 엔드포인트: GET /api/v1/workspaces/{workspaceID}/stream
// 로그 출력이 필요하면 NewSSESubscriberWithLogger를 사용하세요.
func NewSSESubscriber(baseURL, workspaceID, token string) *SSESubscriber {
	return NewSSESubscriberWithOptions(baseURL, workspaceID, token, sseMaxRetry)
}

// NewSSESubscriberWithLogger는 로거를 지정하여 새 SSESubscriber를 생성합니다.
func NewSSESubscriberWithLogger(baseURL, workspaceID, token string, logger zerolog.Logger) *SSESubscriber {
	sub := NewSSESubscriberWithOptions(baseURL, workspaceID, token, sseMaxRetry)
	sub.logger = logger.With().Str("component", "apiclient.sse").Logger()
	return sub
}

// NewSSESubscriberWithOptions는 재시도 횟수를 지정하여 새 SSESubscriber를 생성합니다.
// maxRetry가 0이면 재시도 없이 첫 번째 실패 시 에러를 반환합니다.
func NewSSESubscriberWithOptions(baseURL, workspaceID, token string, maxRetry int) *SSESubscriber {
	return &SSESubscriber{
		baseURL:     strings.TrimRight(baseURL, "/"),
		workspaceID: workspaceID,
		token:       token,
		logger:      zerolog.Nop().With().Str("component", "apiclient.sse").Logger(),
		maxRetry:    maxRetry,
		httpClient: &http.Client{
			Transport: &http.Transport{
				DialContext: (&net.Dialer{
					Timeout: 10 * time.Second,
				}).DialContext,
			},
		},
	}
}

// Subscribe는 SSE 엔드포인트에 연결하여 이벤트 채널을 반환합니다.
// 연결 실패 시 최대 3회까지 지수 백오프(1s, 2s, 4s)로 재연결을 시도합니다.
// 컨텍스트가 취소되면 채널이 닫힙니다.
//
// @MX:NOTE: eventCh와 errCh는 고루틴 내부에서 close됩니다.
// // 컨텍스트 취소 또는 모든 재시도 실패 시 자동으로 닫힙니다.
func (s *SSESubscriber) Subscribe(ctx context.Context) (<-chan SSEEvent, <-chan error) {
	eventCh := make(chan SSEEvent, 100)
	errCh := make(chan error, 1)

	go s.run(ctx, eventCh, errCh)

	return eventCh, errCh
}

// run은 SSE 스트림을 읽는 고루틴 본체입니다.
func (s *SSESubscriber) run(ctx context.Context, eventCh chan<- SSEEvent, errCh chan<- error) {
	defer close(eventCh)
	defer close(errCh)

	endpoint := s.buildEndpoint()
	backoff := time.Second

	for attempt := 0; attempt <= s.maxRetry; attempt++ {
		if attempt > 0 {
			s.logger.Debug().
				Int("attempt", attempt).
				Dur("backoff", backoff).
				Msg("SSE 재연결 대기")

			timer := time.NewTimer(backoff)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
				backoff *= 2
			}
		}

		err := s.connect(ctx, endpoint, eventCh)
		if err == nil {
			// 정상 종료 (컨텍스트 취소 등)
			return
		}

		if ctx.Err() != nil {
			// 컨텍스트 취소면 재시도 불필요
			return
		}

		s.logger.Warn().
			Err(err).
			Int("attempt", attempt).
			Msg("SSE 연결 실패")

		if attempt == s.maxRetry {
			errCh <- fmt.Errorf("SSE 연결 실패 (최대 재시도 횟수 초과): %w", err)
			return
		}
	}
}

// connect는 단일 SSE 연결을 시도하고 이벤트를 채널로 전달합니다.
func (s *SSESubscriber) connect(ctx context.Context, endpoint string, eventCh chan<- SSEEvent) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("SSE 요청 생성 실패: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+s.token)
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil // 컨텍스트 취소
		}
		return fmt.Errorf("SSE 연결 실패: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("SSE 서버 오류 (HTTP %d)", resp.StatusCode)
	}

	// bufio.Scanner로 SSE 스트림 읽기 (cmd/execute.go 패턴 참조)
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, sseReadBufferSize), sseReadBufferSize)

	for scanner.Scan() {
		if ctx.Err() != nil {
			return nil
		}

		line := strings.TrimSpace(scanner.Text())

		// 빈 줄 또는 주석 줄 무시
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}

		// "data: {json}" 형식만 처리
		if !strings.HasPrefix(line, "data:") {
			continue
		}

		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" {
			continue
		}

		var event SSEEvent
		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			s.logger.Debug().
				Str("payload", payload).
				Err(err).
				Msg("SSE 이벤트 파싱 실패, 건너뜀")
			continue
		}

		select {
		case eventCh <- event:
		case <-ctx.Done():
			return nil
		}
	}

	if err := scanner.Err(); err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("SSE 스트림 읽기 실패: %w", err)
	}

	return nil
}

// buildEndpoint는 SSE 엔드포인트 URL을 구성합니다.
func (s *SSESubscriber) buildEndpoint() string {
	return s.baseURL + "/api/v1/workspaces/" + s.workspaceID + "/stream"
}

// FilterAgentTyping은 SSE 이벤트 채널에서 agent_typing 이벤트를 필터링합니다.
// channelID가 지정되면 해당 채널의 이벤트만 반환합니다.
// channelID가 빈 문자열이면 모든 채널의 이벤트를 반환합니다.
// 고루틴에서 실행되며, 입력 채널이 닫히면 출력 채널도 닫힙니다.
func FilterAgentTyping(events <-chan SSEEvent, channelID string) <-chan AgentTypingPayload {
	typingCh := make(chan AgentTypingPayload, 100)

	go func() {
		defer close(typingCh)

		for event := range events {
			if event.Type != eventTypeAgentTyping {
				continue
			}

			var payload AgentTypingPayload
			if err := json.Unmarshal(event.Data, &payload); err != nil {
				continue
			}

			// channelID가 지정된 경우 해당 채널만 허용
			if channelID != "" && payload.ChannelID != channelID {
				continue
			}

			typingCh <- payload
		}
	}()

	return typingCh
}
