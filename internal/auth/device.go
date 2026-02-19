// device.go는 RFC 8628 Device Authorization Flow 클라이언트를 구현합니다.
// POST /api/v1/auth/device/code 로 디바이스 코드를 요청하고,
// POST /api/v1/auth/device/token 을 폴링하여 토큰을 수신합니다.
package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// DeviceCodeResponse는 디바이스 코드 요청 응답 구조체입니다.
// POST /api/v1/auth/device/code 에서 반환됩니다.
type DeviceCodeResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

// DeviceTokenResponse는 디바이스 토큰 폴링 응답 구조체입니다.
// POST /api/v1/auth/device/token 에서 반환됩니다.
type DeviceTokenResponse struct {
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	TokenType    string `json:"token_type,omitempty"`
	ExpiresIn    int    `json:"expires_in,omitempty"`
	Error        string `json:"error,omitempty"`
	// 서버가 성공 시 사용자 정보를 함께 반환할 수 있음
	UserEmail     string `json:"user_email,omitempty"`
	WorkspaceID   string `json:"workspace_id,omitempty"`
	WorkspaceSlug string `json:"workspace_slug,omitempty"`
}

const (
	// deviceHTTPTimeout은 개별 HTTP 요청의 타임아웃입니다.
	deviceHTTPTimeout = 10 * time.Second
	// defaultPollInterval은 기본 폴링 간격(초)입니다.
	defaultPollInterval = 5
	// slowDownIncrement은 slow_down 응답 시 간격 증가량(초)입니다.
	slowDownIncrement = 5
)

// RequestDeviceCode는 디바이스 코드를 요청합니다.
// apiBaseURL은 "http://127.0.0.1:8080" 또는 "https://api.autopus.co" 형식입니다.
func RequestDeviceCode(apiBaseURL string) (*DeviceCodeResponse, error) {
	url := apiBaseURL + "/api/v1/auth/device/code"

	client := &http.Client{Timeout: deviceHTTPTimeout}
	resp, err := client.Post(url, "application/json", bytes.NewReader([]byte("{}")))
	if err != nil {
		return nil, fmt.Errorf("디바이스 코드 요청 실패: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("디바이스 코드 요청 실패 (HTTP %d)", resp.StatusCode)
	}

	var deviceResp DeviceCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&deviceResp); err != nil {
		return nil, fmt.Errorf("디바이스 코드 응답 파싱 실패: %w", err)
	}

	if deviceResp.DeviceCode == "" || deviceResp.UserCode == "" {
		return nil, fmt.Errorf("서버가 유효한 디바이스 코드를 반환하지 않았습니다")
	}

	// 기본 폴링 간격 설정
	if deviceResp.Interval <= 0 {
		deviceResp.Interval = defaultPollInterval
	}

	return &deviceResp, nil
}

// PollDeviceToken은 디바이스 토큰 엔드포인트를 폴링하여 인증 완료를 기다립니다.
// 사용자가 브라우저에서 인증을 완료하면 토큰을 반환합니다.
// ctx가 취소되면 폴링을 중단합니다.
func PollDeviceToken(ctx context.Context, apiBaseURL, deviceCode string, interval int) (*DeviceTokenResponse, error) {
	url := apiBaseURL + "/api/v1/auth/device/token"
	client := &http.Client{Timeout: deviceHTTPTimeout}

	if interval <= 0 {
		interval = defaultPollInterval
	}

	reqBody := struct {
		DeviceCode string `json:"device_code"`
		GrantType  string `json:"grant_type"`
	}{
		DeviceCode: deviceCode,
		GrantType:  "urn:ietf:params:oauth:grant-type:device_code",
	}

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Duration(interval) * time.Second):
			// 폴링 요청 전송
			body, err := json.Marshal(reqBody)
			if err != nil {
				return nil, fmt.Errorf("요청 생성 실패: %w", err)
			}

			resp, err := client.Post(url, "application/json", bytes.NewReader(body))
			if err != nil {
				// 네트워크 오류 시 폴링 계속
				continue
			}

			var tokenResp DeviceTokenResponse
			decodeErr := json.NewDecoder(resp.Body).Decode(&tokenResp)
			_ = resp.Body.Close()

			if decodeErr != nil {
				// 파싱 오류 시 폴링 계속
				continue
			}

			// 오류 응답 처리 (RFC 8628 Section 3.5)
			switch tokenResp.Error {
			case "":
				// 토큰 수신 성공
				if tokenResp.AccessToken != "" {
					return &tokenResp, nil
				}
				// access_token이 없으면 계속 폴링
				continue

			case "authorization_pending":
				// 사용자가 아직 인증하지 않음 - 계속 폴링
				continue

			case "slow_down":
				// 폴링 간격 증가
				interval += slowDownIncrement
				continue

			case "expired_token":
				return nil, fmt.Errorf("인증 코드가 만료되었습니다. 다시 시도하세요")

			case "access_denied":
				return nil, fmt.Errorf("인증이 거부되었습니다")

			default:
				return nil, fmt.Errorf("인증 오류: %s", tokenResp.Error)
			}
		}
	}
}
