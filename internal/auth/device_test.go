package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestRequestDeviceCode_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/auth/device/code" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(DeviceCodeResponse{
			DeviceCode:              "test-device-code",
			UserCode:                "ABCD-EFGH",
			VerificationURI:         "https://autopus.co/device",
			VerificationURIComplete: "https://autopus.co/device?code=ABCD-EFGH",
			ExpiresIn:               600,
			Interval:                5,
		})
	}))
	defer server.Close()

	resp, err := RequestDeviceCode(server.URL, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.DeviceCode != "test-device-code" {
		t.Errorf("expected device_code 'test-device-code', got '%s'", resp.DeviceCode)
	}
	if resp.UserCode != "ABCD-EFGH" {
		t.Errorf("expected user_code 'ABCD-EFGH', got '%s'", resp.UserCode)
	}
	if resp.VerificationURI != "https://autopus.co/device" {
		t.Errorf("expected verification_uri 'https://autopus.co/device', got '%s'", resp.VerificationURI)
	}
	if resp.ExpiresIn != 600 {
		t.Errorf("expected expires_in 600, got %d", resp.ExpiresIn)
	}
	if resp.Interval != 5 {
		t.Errorf("expected interval 5, got %d", resp.Interval)
	}
}

func TestRequestDeviceCode_WithPKCE(t *testing.T) {
	var receivedChallenge, receivedMethod string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}
		receivedChallenge = body["code_challenge"]
		receivedMethod = body["code_challenge_method"]

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(DeviceCodeResponse{
			DeviceCode:             "test-device-code",
			UserCode:               "PKCE-TEST",
			VerificationURI:        "https://autopus.co/device",
			ExpiresIn:              600,
			Interval:               5,
			CodeChallengeSupported: true,
		})
	}))
	defer server.Close()

	pkce, err := GeneratePKCE()
	if err != nil {
		t.Fatalf("GeneratePKCE failed: %v", err)
	}

	resp, err := RequestDeviceCode(server.URL, pkce)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedChallenge != pkce.CodeChallenge {
		t.Errorf("expected code_challenge '%s', got '%s'", pkce.CodeChallenge, receivedChallenge)
	}
	if receivedMethod != "S256" {
		t.Errorf("expected code_challenge_method 'S256', got '%s'", receivedMethod)
	}
	if resp.UserCode != "PKCE-TEST" {
		t.Errorf("expected user_code 'PKCE-TEST', got '%s'", resp.UserCode)
	}
}

func TestRequestDeviceCode_BridgeVersionHeader(t *testing.T) {
	originalVersion := BridgeVersion
	defer func() { BridgeVersion = originalVersion }()

	BridgeVersion = "1.2.3"

	var receivedVersion string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedVersion = r.Header.Get("X-Bridge-Version")

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(DeviceCodeResponse{
			DeviceCode:      "test-code",
			UserCode:        "TEST-CODE",
			VerificationURI: "https://autopus.co/device",
			ExpiresIn:       600,
			Interval:        5,
		})
	}))
	defer server.Close()

	_, err := RequestDeviceCode(server.URL, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedVersion != "1.2.3" {
		t.Errorf("expected X-Bridge-Version '1.2.3', got '%s'", receivedVersion)
	}
}

func TestRequestDeviceCode_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	_, err := RequestDeviceCode(server.URL, nil)
	if err == nil {
		t.Fatal("expected error for server error response")
	}
}

func TestRequestDeviceCode_EmptyDeviceCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(DeviceCodeResponse{
			DeviceCode: "",
			UserCode:   "",
		})
	}))
	defer server.Close()

	_, err := RequestDeviceCode(server.URL, nil)
	if err == nil {
		t.Fatal("expected error for empty device code")
	}
}

func TestRequestDeviceCode_DefaultInterval(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(DeviceCodeResponse{
			DeviceCode:      "test-code",
			UserCode:        "TEST-CODE",
			VerificationURI: "https://autopus.co/device",
			Interval:        0, // 서버가 interval을 설정하지 않은 경우
		})
	}))
	defer server.Close()

	resp, err := RequestDeviceCode(server.URL, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Interval != defaultPollInterval {
		t.Errorf("expected default interval %d, got %d", defaultPollInterval, resp.Interval)
	}
}

func TestPollDeviceToken_Success(t *testing.T) {
	var callCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/auth/device/token" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		count := callCount.Add(1)
		w.Header().Set("Content-Type", "application/json")

		if count < 3 {
			// 처음 2번은 authorization_pending
			_ = json.NewEncoder(w).Encode(DeviceTokenResponse{
				Error: "authorization_pending",
			})
		} else {
			// 3번째에 성공
			_ = json.NewEncoder(w).Encode(DeviceTokenResponse{
				AccessToken:  "test-access-token",
				RefreshToken: "test-refresh-token",
				TokenType:    "Bearer",
				ExpiresIn:    3600,
				UserEmail:    "user@example.com",
			})
		}
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := PollDeviceToken(ctx, server.URL, "test-device-code", 1, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.AccessToken != "test-access-token" {
		t.Errorf("expected access_token 'test-access-token', got '%s'", resp.AccessToken)
	}
	if resp.RefreshToken != "test-refresh-token" {
		t.Errorf("expected refresh_token 'test-refresh-token', got '%s'", resp.RefreshToken)
	}
	if resp.UserEmail != "user@example.com" {
		t.Errorf("expected user_email 'user@example.com', got '%s'", resp.UserEmail)
	}
}

func TestPollDeviceToken_WithCodeVerifier(t *testing.T) {
	var receivedVerifier string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			DeviceCode   string `json:"device_code"`
			GrantType    string `json:"grant_type"`
			CodeVerifier string `json:"code_verifier"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}
		receivedVerifier = body.CodeVerifier

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(DeviceTokenResponse{
			AccessToken: "ok",
			ExpiresIn:   3600,
		})
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := PollDeviceToken(ctx, server.URL, "test-device-code", 1, "my-code-verifier")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedVerifier != "my-code-verifier" {
		t.Errorf("expected code_verifier 'my-code-verifier', got '%s'", receivedVerifier)
	}
}

func TestPollDeviceToken_WithWorkspaces(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(DeviceTokenResponse{
			AccessToken:  "test-token",
			RefreshToken: "test-refresh",
			ExpiresIn:    3600,
			UserEmail:    "user@example.com",
			User: &TokenUser{
				ID:          "user-123",
				Email:       "user@example.com",
				DisplayName: "Test User",
			},
			Workspaces: []TokenWorkspace{
				{ID: "ws-1", Name: "My Workspace", Slug: "my-workspace", Role: "owner"},
				{ID: "ws-2", Name: "Team Workspace", Slug: "team-workspace", Role: "member"},
			},
		})
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := PollDeviceToken(ctx, server.URL, "test-device-code", 1, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.User == nil {
		t.Fatal("expected User to be non-nil")
	}
	if resp.User.ID != "user-123" {
		t.Errorf("expected user ID 'user-123', got '%s'", resp.User.ID)
	}
	if len(resp.Workspaces) != 2 {
		t.Fatalf("expected 2 workspaces, got %d", len(resp.Workspaces))
	}
	if resp.Workspaces[0].Slug != "my-workspace" {
		t.Errorf("expected first workspace slug 'my-workspace', got '%s'", resp.Workspaces[0].Slug)
	}
}

func TestPollDeviceToken_ExpiredToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(DeviceTokenResponse{
			Error: "expired_token",
		})
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := PollDeviceToken(ctx, server.URL, "test-device-code", 1, "")
	if err == nil {
		t.Fatal("expected error for expired token")
	}
	if err.Error() != "인증 코드가 만료되었습니다. 다시 시도하세요" {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}

func TestPollDeviceToken_AccessDenied(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(DeviceTokenResponse{
			Error: "access_denied",
		})
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := PollDeviceToken(ctx, server.URL, "test-device-code", 1, "")
	if err == nil {
		t.Fatal("expected error for access denied")
	}
}

func TestPollDeviceToken_SlowDown(t *testing.T) {
	var callCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := callCount.Add(1)
		w.Header().Set("Content-Type", "application/json")

		if count == 1 {
			// 첫 요청에 slow_down
			_ = json.NewEncoder(w).Encode(DeviceTokenResponse{
				Error: "slow_down",
			})
		} else {
			// 이후 성공
			_ = json.NewEncoder(w).Encode(DeviceTokenResponse{
				AccessToken:  "test-token",
				RefreshToken: "test-refresh",
				TokenType:    "Bearer",
				ExpiresIn:    3600,
			})
		}
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := PollDeviceToken(ctx, server.URL, "test-device-code", 1, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.AccessToken != "test-token" {
		t.Errorf("expected access_token 'test-token', got '%s'", resp.AccessToken)
	}
}

func TestPollDeviceToken_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(DeviceTokenResponse{
			Error: "authorization_pending",
		})
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	// 즉시 취소
	cancel()

	_, err := PollDeviceToken(ctx, server.URL, "test-device-code", 1, "")
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestPollDeviceToken_RequestBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			DeviceCode string `json:"device_code"`
			GrantType  string `json:"grant_type"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}

		if body.DeviceCode != "my-device-code" {
			t.Errorf("expected device_code 'my-device-code', got '%s'", body.DeviceCode)
		}
		if body.GrantType != "urn:ietf:params:oauth:grant-type:device_code" {
			t.Errorf("expected correct grant_type, got '%s'", body.GrantType)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(DeviceTokenResponse{
			AccessToken: "ok",
			ExpiresIn:   3600,
		})
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := PollDeviceToken(ctx, server.URL, "my-device-code", 1, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
