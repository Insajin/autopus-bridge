// Package provider는 AI 프로바이더 통합 레이어를 제공합니다.
package provider

import (
	"encoding/json"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// Claude CLI stream-json 이벤트 타입 상수
const (
	StreamEventMessageStart      = "message_start"
	StreamEventContentBlockStart = "content_block_start"
	StreamEventContentBlockDelta = "content_block_delta"
	StreamEventContentBlockStop  = "content_block_stop"
	StreamEventMessageDelta      = "message_delta"
	StreamEventMessageStop       = "message_stop"
	StreamEventResult            = "result"
)

// StreamLine은 Claude CLI stream-json의 NDJSON 한 줄을 파싱한 결과입니다.
type StreamLine struct {
	Type  string `json:"type"`
	Index int    `json:"index,omitempty"`

	// content_block_delta 전용
	Delta *StreamDelta `json:"delta,omitempty"`

	// result 전용 (최종 결과)
	Result            string  `json:"result,omitempty"`
	Subtype           string  `json:"subtype,omitempty"`
	CostUSD           float64 `json:"cost_usd,omitempty"`
	DurationMS        int64   `json:"duration_ms,omitempty"`
	SessionID         string  `json:"session_id,omitempty"`
	NumTurns          int     `json:"num_turns,omitempty"`
	TotalInputTokens  int     `json:"total_input_tokens,omitempty"`
	TotalOutputTokens int     `json:"total_output_tokens,omitempty"`
}

// StreamDelta는 content_block_delta 이벤트의 delta 필드입니다.
type StreamDelta struct {
	Type string `json:"type"` // "text_delta", "input_json_delta", etc.
	Text string `json:"text,omitempty"`
}

// ParseStreamLine은 NDJSON 한 줄을 StreamLine으로 파싱합니다.
func ParseStreamLine(line []byte) (*StreamLine, error) {
	var sl StreamLine
	if err := json.Unmarshal(line, &sl); err != nil {
		return nil, err
	}
	return &sl, nil
}

// IsTextDelta는 이 이벤트가 텍스트 델타인지 확인합니다.
func (sl *StreamLine) IsTextDelta() bool {
	return sl.Type == StreamEventContentBlockDelta &&
		sl.Delta != nil &&
		sl.Delta.Type == "text_delta" &&
		sl.Delta.Text != ""
}

// IsResult는 이 이벤트가 최종 결과인지 확인합니다.
func (sl *StreamLine) IsResult() bool {
	return sl.Type == StreamEventResult
}

// StreamAccumulator는 텍스트 토큰을 누적하고 블록 스트리밍 로직을 구현합니다.
// 문장 경계, 줄바꿈, 시간 제한, 또는 버퍼 크기에 따라 플러시합니다.
type StreamAccumulator struct {
	mu            sync.Mutex
	buffer        strings.Builder // 아직 플러시되지 않은 텍스트
	accumulated   strings.Builder // 전체 누적 텍스트
	lastFlushTime time.Time
	flushTimeout  time.Duration // 플러시 타임아웃 (기본 300ms)
	maxBufferSize int           // 최대 버퍼 크기 (기본 200자)
}

// NewStreamAccumulator는 새로운 StreamAccumulator를 생성합니다.
func NewStreamAccumulator() *StreamAccumulator {
	return &StreamAccumulator{
		lastFlushTime: time.Now(),
		flushTimeout:  300 * time.Millisecond,
		maxBufferSize: 200,
	}
}

// Add는 텍스트 토큰을 버퍼에 추가합니다.
func (a *StreamAccumulator) Add(text string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.buffer.WriteString(text)
	a.accumulated.WriteString(text)
}

// ShouldFlush는 버퍼를 플러시해야 하는지 확인합니다.
// 조건: 문장 경계, 줄바꿈, 타임아웃(300ms), 또는 버퍼 크기 초과(200자)
func (a *StreamAccumulator) ShouldFlush() bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	bufStr := a.buffer.String()
	if bufStr == "" {
		return false
	}

	// 조건 1: 버퍼 크기 초과
	if utf8.RuneCountInString(bufStr) >= a.maxBufferSize {
		return true
	}

	// 조건 2: 타임아웃
	if time.Since(a.lastFlushTime) >= a.flushTimeout {
		return true
	}

	// 조건 3: 줄바꿈
	if strings.HasSuffix(bufStr, "\n") {
		return true
	}

	// 조건 4: 문장 경계 (마침표/물음표/느낌표 + 공백 또는 끝)
	trimmed := strings.TrimRight(bufStr, " ")
	if len(trimmed) > 0 {
		lastRune, _ := utf8.DecodeLastRuneInString(trimmed)
		if lastRune == '.' || lastRune == '?' || lastRune == '!' ||
			lastRune == '。' || lastRune == '？' || lastRune == '！' {
			return true
		}
	}

	return false
}

// Flush는 버퍼의 내용을 반환하고 버퍼를 비웁니다.
func (a *StreamAccumulator) Flush() string {
	a.mu.Lock()
	defer a.mu.Unlock()

	result := a.buffer.String()
	a.buffer.Reset()
	a.lastFlushTime = time.Now()
	return result
}

// GetAccumulated는 지금까지 누적된 전체 텍스트를 반환합니다.
func (a *StreamAccumulator) GetAccumulated() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.accumulated.String()
}

// HasPending는 플러시되지 않은 텍스트가 있는지 확인합니다.
func (a *StreamAccumulator) HasPending() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.buffer.Len() > 0
}

// FlushAll은 남은 버퍼를 모두 플러시합니다. 스트리밍 종료 시 호출합니다.
func (a *StreamAccumulator) FlushAll() string {
	return a.Flush()
}
