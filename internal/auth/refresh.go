// refresh.go는 만료된 access token을 자동으로 갱신하는 기능을 제공합니다.
package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// cliRefreshRequest는 CLI refresh 엔드포인트 요청 구조체입니다.
type cliRefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// cliRefreshResponse는 CLI refresh 엔드포인트 응답 구조체입니다.
// 백엔드 response 패키지의 Body { Success, Data } 형식을 따릅니다.
type cliRefreshResponse struct {
	Success bool `json:"success"`
	Data    struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
	} `json:"data"`
}

// RefreshAccessToken은 백엔드 CLI refresh 엔드포인트를 호출하여 새로운 access token을 발급받습니다.
// 성공 시 자격 증명을 업데이트하고 파일에 저장합니다.
func RefreshAccessToken(creds *Credentials) error {
	if creds.RefreshToken == "" {
		return fmt.Errorf("refresh token이 없습니다. 'lab login'으로 다시 로그인하세요")
	}

	// WebSocket URL을 HTTP API URL로 변환
	apiURL := wsURLToHTTPAPI(creds.ServerURL)

	reqBody, err := json.Marshal(cliRefreshRequest{
		RefreshToken: creds.RefreshToken,
	})
	if err != nil {
		return fmt.Errorf("요청 생성 실패: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(apiURL+"/auth/cli-refresh", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("서버 통신 실패: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("토큰 갱신 실패 (HTTP %d)", resp.StatusCode)
	}

	var refreshResp cliRefreshResponse
	if err := json.NewDecoder(resp.Body).Decode(&refreshResp); err != nil {
		return fmt.Errorf("응답 파싱 실패: %w", err)
	}

	if !refreshResp.Success || refreshResp.Data.AccessToken == "" {
		return fmt.Errorf("토큰 갱신 실패: 서버가 유효한 토큰을 반환하지 않았습니다")
	}

	// 자격 증명 업데이트
	creds.AccessToken = refreshResp.Data.AccessToken
	creds.RefreshToken = refreshResp.Data.RefreshToken
	creds.ExpiresAt = time.Now().Add(time.Duration(refreshResp.Data.ExpiresIn) * time.Second)

	// 파일에 저장
	if err := Save(creds); err != nil {
		return fmt.Errorf("자격 증명 저장 실패: %w", err)
	}

	return nil
}

// wsURLToHTTPAPI는 WebSocket URL을 HTTP API URL로 변환합니다.
//
//	"ws://localhost:8080/ws/agent"  -> "http://localhost:8080/api/v1"
//	"wss://api.autopus.co/ws/agent"   -> "https://api.autopus.co/api/v1"
func wsURLToHTTPAPI(wsURL string) string {
	httpURL := wsURL
	if strings.HasPrefix(httpURL, "wss://") {
		httpURL = "https://" + strings.TrimPrefix(httpURL, "wss://")
	} else if strings.HasPrefix(httpURL, "ws://") {
		httpURL = "http://" + strings.TrimPrefix(httpURL, "ws://")
	}

	// /ws/agent 등 WebSocket 경로 제거
	if idx := strings.Index(httpURL, "/ws"); idx != -1 {
		httpURL = httpURL[:idx]
	}

	return httpURL + "/api/v1"
}
