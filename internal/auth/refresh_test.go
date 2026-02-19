package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestWsURLToHTTPAPI는 WebSocket URL을 HTTP API URL로 올바르게 변환하는지 테스트합니다.
func TestWsURLToHTTPAPI(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "ws를 http로 변환",
			input:    "ws://localhost:8080/ws/agent",
			expected: "http://localhost:8080/api/v1",
		},
		{
			name:     "wss를 https로 변환",
			input:    "wss://api.autopus.co/ws/agent",
			expected: "https://api.autopus.co/api/v1",
		},
		{
			name:     "경로 없는 ws URL",
			input:    "ws://localhost:8080",
			expected: "http://localhost:8080/api/v1",
		},
		{
			name:     "경로 없는 wss URL",
			input:    "wss://api.autopus.co",
			expected: "https://api.autopus.co/api/v1",
		},
		{
			name:     "http URL 그대로 유지",
			input:    "http://localhost:8080",
			expected: "http://localhost:8080/api/v1",
		},
		{
			name:     "https URL 그대로 유지",
			input:    "https://api.autopus.co",
			expected: "https://api.autopus.co/api/v1",
		},
		{
			name:     "/ws 하위 경로가 있는 경우",
			input:    "wss://api.autopus.co/ws/agent/v2",
			expected: "https://api.autopus.co/api/v1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wsURLToHTTPAPI(tt.input)
			if got != tt.expected {
				t.Errorf("wsURLToHTTPAPI(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// TestRefreshAccessToken_Success는 토큰 갱신 성공 시나리오를 테스트합니다.
func TestRefreshAccessToken_Success(t *testing.T) {
	setupTestEnv(t)

	// 테스트 서버 생성
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 요청 검증
		if r.Method != http.MethodPost {
			t.Errorf("Method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/api/v1/auth/cli-refresh" {
			t.Errorf("Path = %q, want /api/v1/auth/cli-refresh", r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", r.Header.Get("Content-Type"))
		}

		// 요청 본문 검증
		var req cliRefreshRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("요청 본문 파싱 실패: %v", err)
		}
		if req.RefreshToken != "old-refresh-token" {
			t.Errorf("RefreshToken = %q, want %q", req.RefreshToken, "old-refresh-token")
		}

		// 성공 응답
		resp := cliRefreshResponse{
			Success: true,
		}
		resp.Data.AccessToken = "new-access-token"
		resp.Data.RefreshToken = "new-refresh-token"
		resp.Data.ExpiresIn = 900

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	creds := &Credentials{
		AccessToken:  "expired-access-token",
		RefreshToken: "old-refresh-token",
		ExpiresAt:    time.Now().Add(-1 * time.Hour),
		ServerURL:    server.URL, // http://127.0.0.1:PORT (ws 접두사 없음)
		UserEmail:    "test@example.com",
	}

	err := RefreshAccessToken(creds)
	if err != nil {
		t.Fatalf("RefreshAccessToken() error = %v", err)
	}

	if creds.AccessToken != "new-access-token" {
		t.Errorf("AccessToken = %q, want %q", creds.AccessToken, "new-access-token")
	}
	if creds.RefreshToken != "new-refresh-token" {
		t.Errorf("RefreshToken = %q, want %q", creds.RefreshToken, "new-refresh-token")
	}
	if time.Until(creds.ExpiresAt) < 890*time.Second || time.Until(creds.ExpiresAt) > 910*time.Second {
		t.Errorf("ExpiresAt이 약 900초 후가 아닙니다: %v", time.Until(creds.ExpiresAt))
	}
}

// TestRefreshAccessToken_EmptyRefreshToken는 refresh token이 없을 때 에러를 반환하는지 테스트합니다.
func TestRefreshAccessToken_EmptyRefreshToken(t *testing.T) {
	creds := &Credentials{
		AccessToken:  "expired-token",
		RefreshToken: "",
		ServerURL:    "ws://localhost:8080/ws/agent",
	}

	err := RefreshAccessToken(creds)
	if err == nil {
		t.Fatal("RefreshAccessToken() error = nil, want error for empty refresh token")
	}
}

// TestRefreshAccessToken_ServerError는 서버 오류 시 에러를 반환하는지 테스트합니다.
func TestRefreshAccessToken_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	creds := &Credentials{
		AccessToken:  "expired-token",
		RefreshToken: "invalid-refresh-token",
		ExpiresAt:    time.Now().Add(-1 * time.Hour),
		ServerURL:    server.URL,
	}

	err := RefreshAccessToken(creds)
	if err == nil {
		t.Fatal("RefreshAccessToken() error = nil, want error for server error")
	}
}

// TestRefreshAccessToken_InvalidJSON는 서버가 잘못된 JSON을 반환할 때 에러를 반환하는지 테스트합니다.
func TestRefreshAccessToken_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{invalid json}"))
	}))
	defer server.Close()

	creds := &Credentials{
		AccessToken:  "expired-token",
		RefreshToken: "some-refresh-token",
		ExpiresAt:    time.Now().Add(-1 * time.Hour),
		ServerURL:    server.URL,
	}

	err := RefreshAccessToken(creds)
	if err == nil {
		t.Fatal("RefreshAccessToken() error = nil, want error for invalid JSON")
	}
}

// TestRefreshAccessToken_EmptyAccessTokenInResponse는 서버가 빈 access token을 반환할 때 에러를 반환하는지 테스트합니다.
func TestRefreshAccessToken_EmptyAccessTokenInResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := cliRefreshResponse{
			Success: true,
		}
		// Data.AccessToken이 빈 상태
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	creds := &Credentials{
		AccessToken:  "expired-token",
		RefreshToken: "some-refresh-token",
		ExpiresAt:    time.Now().Add(-1 * time.Hour),
		ServerURL:    server.URL,
	}

	err := RefreshAccessToken(creds)
	if err == nil {
		t.Fatal("RefreshAccessToken() error = nil, want error for empty access token in response")
	}
}
