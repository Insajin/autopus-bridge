// Package websocket는 Local Agent Bridge의 WebSocket 통신을 담당합니다.
// 이 파일은 HMAC-SHA256 기반 메시지 서명 및 검증을 제공합니다 (SEC-P2-02).
package websocket

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/insajin/autopus-agent-protocol"
)

// criticalMessageTypes는 HMAC 서명이 필요한 중요 메시지 타입 목록입니다 (SEC-P2-02).
var criticalMessageTypes = map[string]bool{
	ws.AgentMsgTaskReq:     true,
	ws.AgentMsgTaskResult:  true,
	ws.AgentMsgTaskError:   true,
	ws.AgentMsgBuildReq:    true,
	ws.AgentMsgBuildResult: true,
	ws.AgentMsgTestReq:     true,
	ws.AgentMsgTestResult:  true,
	ws.AgentMsgQAReq:       true,
	ws.AgentMsgQAResult:    true,
	ws.AgentMsgCLIRequest:  true, // SPEC-SKILL-V2-001 Block C: CLI 실행 요청
	ws.AgentMsgCLIResult:   true, // SPEC-SKILL-V2-001 Block C: CLI 실행 결과
	ws.AgentMsgMCPStart:    true, // SPEC-SKILL-V2-001 Block D: MCP 서버 시작 요청
	ws.AgentMsgMCPReady:    true, // SPEC-SKILL-V2-001 Block D: MCP 서버 준비 완료
	ws.AgentMsgMCPStop:     true, // SPEC-SKILL-V2-001 Block D: MCP 서버 중지 요청
	ws.AgentMsgMCPError:    true, // SPEC-SKILL-V2-001 Block D: MCP 서버 에러
}

// MessageSigner는 HMAC-SHA256 기반 메시지 서명 및 검증을 담당합니다 (SEC-P2-02).
// 스레드 안전하게 설계되어 있으며, 시크릿은 agent_connect_ack 수신 후 설정됩니다.
type MessageSigner struct {
	secret []byte
	mu     sync.RWMutex
}

// NewMessageSigner는 새로운 MessageSigner를 생성합니다.
// 시크릿은 SetSecret()을 통해 나중에 설정됩니다.
func NewMessageSigner() *MessageSigner {
	return &MessageSigner{}
}

// SetSecret은 HMAC 서명에 사용할 공유 시크릿을 설정합니다 (SEC-P2-02).
// agent_connect_ack에서 수신한 hex 인코딩된 시크릿을 디코딩하여 저장합니다.
func (s *MessageSigner) SetSecret(secret []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.secret = secret
}

// HasSecret은 시크릿이 설정되어 있는지 확인합니다.
func (s *MessageSigner) HasSecret() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.secret) > 0
}

// Sign은 메시지에 HMAC-SHA256 서명을 추가합니다 (SEC-P2-02).
// 시크릿이 없거나 비중요 메시지인 경우 서명을 건너뜁니다.
func (s *MessageSigner) Sign(msg *ws.AgentMessage) error {
	s.mu.RLock()
	secret := s.secret
	s.mu.RUnlock()

	if len(secret) == 0 {
		return nil
	}

	if !s.IsCriticalMessage(msg.Type) {
		return nil
	}

	data := buildSigningPayload(msg.Type, msg.ID, msg.Timestamp, msg.Payload)
	msg.Signature = computeHMAC(secret, data)
	log.Printf("[HMAC-SIGN] type=%s id=%s ts=%s", msg.Type, msg.ID, msg.Timestamp.Format(time.RFC3339Nano))
	log.Printf("[HMAC-SIGN] payload_len=%d payload_prefix=%s", len(msg.Payload), string(msg.Payload[:min(len(msg.Payload), 100)]))
	log.Printf("[HMAC-SIGN] signature=%s", msg.Signature)
	log.Printf("[HMAC-SIGN] signing_data_len=%d signing_prefix=%s", len(data), string(data[:min(len(data), 200)]))
	return nil
}

// Verify는 메시지의 HMAC-SHA256 서명을 검증합니다 (SEC-P2-02).
// 시크릿이 없거나 비중요 메시지인 경우 true를 반환합니다 (하위 호환성).
// 중요 메시지에 서명이 없거나 유효하지 않으면 false를 반환합니다.
func (s *MessageSigner) Verify(msg *ws.AgentMessage) bool {
	s.mu.RLock()
	secret := s.secret
	s.mu.RUnlock()

	if len(secret) == 0 {
		// 시크릿 미설정: 레거시 연결 하위 호환
		return true
	}

	if !s.IsCriticalMessage(msg.Type) {
		// 비중요 메시지는 서명 검증 불필요
		return true
	}

	if msg.Signature == "" {
		// 중요 메시지인데 서명이 없으면 거부
		return false
	}

	data := buildSigningPayload(msg.Type, msg.ID, msg.Timestamp, msg.Payload)
	expected := computeHMAC(secret, data)
	return hmac.Equal([]byte(msg.Signature), []byte(expected))
}

// IsCriticalMessage는 메시지 타입이 HMAC 서명을 요구하는지 확인합니다 (SEC-P2-02).
func (s *MessageSigner) IsCriticalMessage(msgType string) bool {
	return criticalMessageTypes[msgType]
}

// SetSecretFromHex는 hex 인코딩된 문자열에서 시크릿을 디코딩하여 설정합니다 (SEC-P2-02).
func (s *MessageSigner) SetSecretFromHex(hexSecret string) error {
	if hexSecret == "" {
		return nil
	}
	secret, err := hex.DecodeString(hexSecret)
	if err != nil {
		return fmt.Errorf("HMAC 시크릿 hex 디코딩 실패: %w", err)
	}
	s.SetSecret(secret)
	return nil
}

// buildSigningPayload는 메시지 서명에 사용할 정규화된 페이로드를 생성합니다 (SEC-P2-02).
// 형식: Type|ID|Timestamp(RFC3339Nano)|Payload
func buildSigningPayload(msgType, msgID string, timestamp time.Time, payload []byte) []byte {
	data := msgType + "|" + msgID + "|" + timestamp.Format(time.RFC3339Nano) + "|" + string(payload)
	return []byte(data)
}

// computeHMAC는 주어진 데이터에 대해 HMAC-SHA256을 계산하고 hex 인코딩된 문자열을 반환합니다.
func computeHMAC(secret, data []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write(data)
	return hex.EncodeToString(mac.Sum(nil))
}
