// debug_test.go는 debug ping/ws/token 명령어의 테스트를 정의합니다.
package cmd

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/insajin/autopus-bridge/internal/apiclient"
	"github.com/insajin/autopus-bridge/internal/auth"
	"github.com/insajin/autopus-bridge/internal/mcpserver"
	"github.com/rs/zerolog"
)

// makeTestJWT는 테스트용 JWT 토큰을 생성합니다.
// 실제 서명은 하지 않고 exp 클레임만 포함합니다.
func makeTestJWT(expiry time.Time) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payload := map[string]interface{}{
		"sub": "test-user",
		"exp": expiry.Unix(),
	}
	payloadBytes, _ := json.Marshal(payload)
	payloadEncoded := base64.RawURLEncoding.EncodeToString(payloadBytes)
	// 서명은 테스트용 더미 값
	sig := base64.RawURLEncoding.EncodeToString([]byte("test-signature"))
	return fmt.Sprintf("%s.%s.%s", header, payloadEncoded, sig)
}

// makeTestClientWithToken은 특정 토큰을 가진 테스트용 Client를 생성합니다.
func makeTestClientWithToken(token, serverURL, workspaceID string) *apiclient.Client {
	creds := &auth.Credentials{
		AccessToken: token,
		ServerURL:   serverURL,
		WorkspaceID: workspaceID,
		ExpiresAt:   time.Now().Add(2 * time.Hour), // TokenRefresher가 갱신 시도하지 않도록
	}
	tr := auth.NewTokenRefresher(creds)
	backend := mcpserver.NewBackendClient("http://localhost:8080", tr, 0, zerolog.Nop())
	return apiclient.New(backend, creds, tr)
}

// TestRunDebugPing_Success는 ping 성공 시 PONG 출력을 검증합니다.
func TestRunDebugPing_Success(t *testing.T) {
	health := OrgHealth{Status: "healthy"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(buildAPIResponse(health))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runDebugPing(client, &buf)
	if err != nil {
		t.Fatalf("runDebugPing 실패: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "PONG") {
		t.Errorf("ping 성공 시 PONG이 출력되어야 합니다: %s", out)
	}
	if !strings.Contains(out, "ms") {
		t.Errorf("ping 응답에 응답 시간(ms)이 포함되어야 합니다: %s", out)
	}
}

// TestRunDebugPing_Failure는 ping 실패 시 에러를 반환하는지 검증합니다.
func TestRunDebugPing_Failure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"서비스 불가"}`))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runDebugPing(client, &buf)
	if err == nil {
		t.Fatal("서버 오류 시 에러를 반환해야 합니다")
	}

	out := buf.String()
	if !strings.Contains(out, "PING FAILED") {
		t.Errorf("ping 실패 시 PING FAILED가 출력되어야 합니다: %s", out)
	}
}

// TestRunDebugWS_Success는 ws 연결 테스트 성공을 검증합니다.
func TestRunDebugWS_Success(t *testing.T) {
	health := OrgHealth{Status: "healthy"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(buildAPIResponse(health))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runDebugWS(client, &buf)
	if err != nil {
		t.Fatalf("runDebugWS 실패: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "WebSocket URL") {
		t.Errorf("출력에 WebSocket URL 정보가 없습니다: %s", out)
	}
	if !strings.Contains(out, "성공") {
		t.Errorf("연결 성공 메시지가 없습니다: %s", out)
	}
}

// TestRunDebugWS_Failure는 ws 연결 실패를 검증합니다.
func TestRunDebugWS_Failure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"서비스 불가"}`))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runDebugWS(client, &buf)
	if err == nil {
		t.Fatal("서버 오류 시 에러를 반환해야 합니다")
	}

	out := buf.String()
	if !strings.Contains(out, "실패") {
		t.Errorf("연결 실패 메시지가 없습니다: %s", out)
	}
}

// TestRunDebugToken_TableOutput은 token 명령의 테이블 출력을 검증합니다.
func TestRunDebugToken_TableOutput(t *testing.T) {
	// 유효한 JWT 토큰 생성 (만료 1시간 후)
	token := makeTestJWT(time.Now().Add(1 * time.Hour))
	client := makeTestClientWithToken(token, "ws://localhost:8080/ws/agent", "ws-123")

	var buf bytes.Buffer
	err := runDebugToken(client, &buf, false)
	if err != nil {
		t.Fatalf("runDebugToken 실패: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Token") {
		t.Errorf("출력에 Token 필드가 없습니다: %s", out)
	}
	if !strings.Contains(out, "WorkspaceID") {
		t.Errorf("출력에 WorkspaceID 필드가 없습니다: %s", out)
	}
	if !strings.Contains(out, "ExpiresAt") {
		t.Errorf("출력에 ExpiresAt 필드가 없습니다: %s", out)
	}
}

// TestRunDebugToken_JSONOutput은 token 명령의 JSON 출력을 검증합니다.
func TestRunDebugToken_JSONOutput(t *testing.T) {
	token := makeTestJWT(time.Now().Add(1 * time.Hour))
	client := makeTestClientWithToken(token, "ws://localhost:8080/ws/agent", "ws-999")
	var buf bytes.Buffer

	err := runDebugToken(client, &buf, true)
	if err != nil {
		t.Fatalf("runDebugToken JSON 실패: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &result); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if _, ok := result["token"]; !ok {
		t.Errorf("JSON 출력에 token 필드가 없습니다: %v", result)
	}
	if _, ok := result["workspace_id"]; !ok {
		t.Errorf("JSON 출력에 workspace_id 필드가 없습니다: %v", result)
	}
}

// TestRunDebugToken_MaskToken은 토큰이 마스킹되는지 검증합니다.
func TestRunDebugToken_MaskToken(t *testing.T) {
	token := makeTestJWT(time.Now().Add(1 * time.Hour))
	client := makeTestClientWithToken(token, "ws://localhost:8080/ws/agent", "ws-1")
	var buf bytes.Buffer

	_ = runDebugToken(client, &buf, false)
	out := buf.String()

	// 실제 토큰이 출력에 포함되면 안 됨 (마스킹 확인)
	if strings.Contains(out, token) {
		t.Errorf("출력에 실제 토큰이 노출되었습니다 (마스킹 실패): %s", out)
	}
	// 마스킹된 형태 (처음 8자 + "...")가 있어야 함
	masked := token[:8] + "..."
	if !strings.Contains(out, masked) {
		t.Errorf("출력에 마스킹된 토큰이 없습니다. 기대값: %s, 출력: %s", masked, out)
	}
}

// TestRunDebugToken_ExpiredWarning은 만료된 토큰에 경고를 출력하는지 검증합니다.
func TestRunDebugToken_ExpiredWarning(t *testing.T) {
	// 만료된 JWT 토큰 생성 (1시간 전 만료)
	expiredToken := makeTestJWT(time.Now().Add(-1 * time.Hour))
	// ExpiresAt을 미래로 설정하여 TokenRefresher의 자동 갱신을 방지
	// (실제 토큰 갱신은 테스트 범위 밖)
	client := makeTestClientWithToken(expiredToken, "ws://localhost:8080/ws/agent", "ws-1")
	var buf bytes.Buffer

	// 만료된 토큰이라도 token 정보는 출력 (오류는 없음)
	_ = runDebugToken(client, &buf, false)
	out := buf.String()

	if !strings.Contains(out, "만료") {
		t.Errorf("만료된 토큰에 경고 메시지가 없습니다: %s", out)
	}
}
