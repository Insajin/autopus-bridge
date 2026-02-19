package websocket

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/insajin/autopus-agent-protocol"
)

// TestMessageSigner_SignAndVerify는 메시지 서명 후 검증이 성공하는지 테스트합니다.
func TestMessageSigner_SignAndVerify(t *testing.T) {
	signer := NewMessageSigner()
	secret := []byte("test-secret-32-bytes-long-value!")
	signer.SetSecret(secret)

	payload, _ := json.Marshal(map[string]string{"execution_id": "exec-001", "output": "hello"})
	msg := &ws.AgentMessage{
		Type:      ws.AgentMsgTaskResult,
		ID:        "msg-001",
		Timestamp: time.Now(),
		Payload:   payload,
	}

	// 서명 추가
	err := signer.Sign(msg)
	if err != nil {
		t.Fatalf("Sign() 에러 발생: %v", err)
	}

	// 서명이 설정되었는지 확인
	if msg.Signature == "" {
		t.Fatal("Sign() 후 Signature가 비어 있음")
	}

	// 검증 성공
	if !signer.Verify(msg) {
		t.Fatal("Verify()가 유효한 서명에 대해 false 반환")
	}
}

// TestMessageSigner_TamperedMessage는 페이로드 변조 시 검증이 실패하는지 테스트합니다.
func TestMessageSigner_TamperedMessage(t *testing.T) {
	signer := NewMessageSigner()
	secret := []byte("test-secret-32-bytes-long-value!")
	signer.SetSecret(secret)

	payload, _ := json.Marshal(map[string]string{"execution_id": "exec-001", "output": "hello"})
	msg := &ws.AgentMessage{
		Type:      ws.AgentMsgTaskResult,
		ID:        "msg-001",
		Timestamp: time.Now(),
		Payload:   payload,
	}

	// 서명 추가
	_ = signer.Sign(msg)
	originalSig := msg.Signature

	if originalSig == "" {
		t.Fatal("Sign() 후 Signature가 비어 있음")
	}

	// 페이로드 변조
	tamperedPayload, _ := json.Marshal(map[string]string{"execution_id": "exec-001", "output": "tampered"})
	msg.Payload = tamperedPayload

	// 검증 실패 확인
	if signer.Verify(msg) {
		t.Fatal("Verify()가 변조된 메시지에 대해 true 반환")
	}
}

// TestMessageSigner_TamperedType은 메시지 타입 변조 시 검증이 실패하는지 테스트합니다.
func TestMessageSigner_TamperedType(t *testing.T) {
	signer := NewMessageSigner()
	secret := []byte("test-secret-32-bytes-long-value!")
	signer.SetSecret(secret)

	payload, _ := json.Marshal(map[string]string{"execution_id": "exec-001"})
	msg := &ws.AgentMessage{
		Type:      ws.AgentMsgTaskResult,
		ID:        "msg-001",
		Timestamp: time.Now(),
		Payload:   payload,
	}

	_ = signer.Sign(msg)

	// 타입 변조 (task_result -> task_error, 둘 다 critical)
	msg.Type = ws.AgentMsgTaskError

	// 검증 실패 확인
	if signer.Verify(msg) {
		t.Fatal("Verify()가 타입 변조된 메시지에 대해 true 반환")
	}
}

// TestMessageSigner_IsCriticalMessage는 중요 메시지 타입 분류가 올바른지 테스트합니다.
func TestMessageSigner_IsCriticalMessage(t *testing.T) {
	signer := NewMessageSigner()

	// 중요 메시지 타입
	criticalTypes := []string{
		ws.AgentMsgTaskReq,
		ws.AgentMsgTaskResult,
		ws.AgentMsgTaskError,
		ws.AgentMsgBuildReq,
		ws.AgentMsgBuildResult,
		ws.AgentMsgTestReq,
		ws.AgentMsgTestResult,
		ws.AgentMsgQAReq,
		ws.AgentMsgQAResult,
	}

	for _, msgType := range criticalTypes {
		if !signer.IsCriticalMessage(msgType) {
			t.Errorf("IsCriticalMessage(%q) = false, true 기대", msgType)
		}
	}

	// 비중요 메시지 타입
	nonCriticalTypes := []string{
		ws.AgentMsgConnect,
		ws.AgentMsgConnectAck,
		ws.AgentMsgDisconnect,
		ws.AgentMsgHeartbeat,
		ws.AgentMsgTaskProg,
	}

	for _, msgType := range nonCriticalTypes {
		if signer.IsCriticalMessage(msgType) {
			t.Errorf("IsCriticalMessage(%q) = true, false 기대", msgType)
		}
	}
}

// TestMessageSigner_NoSecret은 시크릿 미설정 시 서명/검증 동작을 테스트합니다.
func TestMessageSigner_NoSecret(t *testing.T) {
	signer := NewMessageSigner()

	payload, _ := json.Marshal(map[string]string{"execution_id": "exec-001"})
	msg := &ws.AgentMessage{
		Type:      ws.AgentMsgTaskResult,
		ID:        "msg-001",
		Timestamp: time.Now(),
		Payload:   payload,
	}

	// 시크릿 없이 서명 시도 - 에러 없이 건너뜀
	err := signer.Sign(msg)
	if err != nil {
		t.Fatalf("Sign() 시크릿 없을 때 에러 발생: %v", err)
	}

	// 서명이 추가되지 않아야 함
	if msg.Signature != "" {
		t.Fatal("시크릿 없을 때 Signature가 설정됨")
	}

	// 시크릿 없이 검증 - 항상 true (하위 호환)
	if !signer.Verify(msg) {
		t.Fatal("시크릿 없을 때 Verify()가 false 반환")
	}
}

// TestMessageSigner_NonCriticalMessage는 비중요 메시지에 서명이 추가되지 않는지 테스트합니다.
func TestMessageSigner_NonCriticalMessage(t *testing.T) {
	signer := NewMessageSigner()
	secret := []byte("test-secret-32-bytes-long-value!")
	signer.SetSecret(secret)

	payload, _ := json.Marshal(map[string]string{"timestamp": "2026-01-01T00:00:00Z"})
	msg := &ws.AgentMessage{
		Type:      ws.AgentMsgHeartbeat,
		ID:        "msg-001",
		Timestamp: time.Now(),
		Payload:   payload,
	}

	// 비중요 메시지 서명 시도
	err := signer.Sign(msg)
	if err != nil {
		t.Fatalf("Sign() 비중요 메시지에서 에러 발생: %v", err)
	}

	// 서명이 추가되지 않아야 함
	if msg.Signature != "" {
		t.Fatal("비중요 메시지에 Signature가 설정됨")
	}

	// 비중요 메시지 검증 - 항상 true
	if !signer.Verify(msg) {
		t.Fatal("비중요 메시지 Verify()가 false 반환")
	}
}

// TestMessageSigner_SetSecretFromHex는 hex 인코딩된 시크릿 설정을 테스트합니다.
func TestMessageSigner_SetSecretFromHex(t *testing.T) {
	signer := NewMessageSigner()

	// 유효한 hex 시크릿 설정
	hexSecret := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	err := signer.SetSecretFromHex(hexSecret)
	if err != nil {
		t.Fatalf("SetSecretFromHex() 에러 발생: %v", err)
	}

	if !signer.HasSecret() {
		t.Fatal("SetSecretFromHex() 후 HasSecret()가 false 반환")
	}

	// 빈 문자열 설정 - 에러 없이 건너뜀
	err = signer.SetSecretFromHex("")
	if err != nil {
		t.Fatalf("SetSecretFromHex(\"\") 에러 발생: %v", err)
	}

	// 유효하지 않은 hex 문자열
	err = signer.SetSecretFromHex("not-valid-hex")
	if err == nil {
		t.Fatal("SetSecretFromHex()가 유효하지 않은 hex에서 에러를 반환하지 않음")
	}
}

// TestMessageSigner_MissingSignature는 중요 메시지에 서명이 없을 때 검증 실패를 테스트합니다.
func TestMessageSigner_MissingSignature(t *testing.T) {
	signer := NewMessageSigner()
	secret := []byte("test-secret-32-bytes-long-value!")
	signer.SetSecret(secret)

	payload, _ := json.Marshal(map[string]string{"execution_id": "exec-001"})
	msg := &ws.AgentMessage{
		Type:      ws.AgentMsgTaskResult,
		ID:        "msg-001",
		Timestamp: time.Now(),
		Payload:   payload,
		// Signature 없음
	}

	// 서명 없이 중요 메시지 검증 - 실패해야 함
	if signer.Verify(msg) {
		t.Fatal("Verify()가 서명 없는 중요 메시지에 대해 true 반환")
	}
}

// TestMessageSigner_WrongSecret은 다른 시크릿으로 서명된 메시지 검증 실패를 테스트합니다.
func TestMessageSigner_WrongSecret(t *testing.T) {
	signer1 := NewMessageSigner()
	signer1.SetSecret([]byte("secret-1-32-bytes-long-value-aa"))

	signer2 := NewMessageSigner()
	signer2.SetSecret([]byte("secret-2-32-bytes-long-value-bb"))

	payload, _ := json.Marshal(map[string]string{"execution_id": "exec-001"})
	msg := &ws.AgentMessage{
		Type:      ws.AgentMsgTaskResult,
		ID:        "msg-001",
		Timestamp: time.Now(),
		Payload:   payload,
	}

	// signer1으로 서명
	_ = signer1.Sign(msg)

	// signer2로 검증 - 실패해야 함
	if signer2.Verify(msg) {
		t.Fatal("Verify()가 다른 시크릿으로 서명된 메시지에 대해 true 반환")
	}

	// signer1으로 검증 - 성공해야 함
	if !signer1.Verify(msg) {
		t.Fatal("Verify()가 올바른 시크릿으로 서명된 메시지에 대해 false 반환")
	}
}

// TestMessageSigner_AllCriticalTypes는 모든 중요 메시지 타입에 서명/검증이 동작하는지 테스트합니다.
func TestMessageSigner_AllCriticalTypes(t *testing.T) {
	signer := NewMessageSigner()
	secret := []byte("test-secret-32-bytes-long-value!")
	signer.SetSecret(secret)

	criticalTypes := []string{
		ws.AgentMsgTaskReq,
		ws.AgentMsgTaskResult,
		ws.AgentMsgTaskError,
		ws.AgentMsgBuildReq,
		ws.AgentMsgBuildResult,
		ws.AgentMsgTestReq,
		ws.AgentMsgTestResult,
		ws.AgentMsgQAReq,
		ws.AgentMsgQAResult,
	}

	for _, msgType := range criticalTypes {
		payload, _ := json.Marshal(map[string]string{"execution_id": "exec-001"})
		msg := &ws.AgentMessage{
			Type:      msgType,
			ID:        "msg-001",
			Timestamp: time.Now(),
			Payload:   payload,
		}

		err := signer.Sign(msg)
		if err != nil {
			t.Fatalf("Sign() type=%s 에러: %v", msgType, err)
		}
		if msg.Signature == "" {
			t.Fatalf("Sign() type=%s 서명 미설정", msgType)
		}
		if !signer.Verify(msg) {
			t.Fatalf("Verify() type=%s 실패", msgType)
		}
	}
}

// TestMessageSigner_HasSecret은 HasSecret 메서드를 테스트합니다.
func TestMessageSigner_HasSecret(t *testing.T) {
	signer := NewMessageSigner()

	if signer.HasSecret() {
		t.Fatal("초기 상태에서 HasSecret()이 true 반환")
	}

	signer.SetSecret([]byte("some-secret"))

	if !signer.HasSecret() {
		t.Fatal("시크릿 설정 후 HasSecret()이 false 반환")
	}
}
