// helpers.go는 CLI 명령어에서 공통으로 사용하는 헬퍼 함수를 정의합니다.
package cmd

import (
	"errors"
	"time"

	"github.com/insajin/autopus-bridge/internal/apiclient"
	"github.com/insajin/autopus-bridge/internal/auth"
	"github.com/insajin/autopus-bridge/internal/mcpserver"
	"github.com/rs/zerolog"
)

// newAPIClient는 인증된 API 클라이언트를 생성합니다.
// execute.go, status.go의 초기화 패턴을 공유합니다.
func newAPIClient() (*apiclient.Client, error) {
	// 저장된 인증 정보 로드
	creds, err := auth.Load()
	if err != nil {
		return nil, err
	}
	if creds == nil {
		return nil, errors.New("로그인이 필요합니다. 'autopus login'을 먼저 실행하세요")
	}

	// 서버 URL을 HTTP로 변환 (wss:// -> https://)
	baseURL := serverURLToHTTPBase(creds.ServerURL)
	if baseURL == "" {
		baseURL = "https://api.autopus.co"
	}

	// TokenRefresher와 BackendClient 생성
	tokenRefresher := auth.NewTokenRefresher(creds)
	backend := mcpserver.NewBackendClient(baseURL, tokenRefresher, 60*time.Second, zerolog.Nop())

	return apiclient.New(backend, creds, tokenRefresher), nil
}
