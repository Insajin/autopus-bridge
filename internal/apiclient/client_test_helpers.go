// client_test_helpers.go는 테스트 전용 Client 생성 헬퍼를 제공합니다.
// 이 파일은 프로덕션 코드에 포함되지만, 테스트 환경에서만 사용합니다.
package apiclient

import (
	"net/http"
	"time"

	"github.com/insajin/autopus-bridge/internal/auth"
)

// NewClientForTest는 테스트용 Client를 생성합니다.
// 실제 BackendClient 대신 nil을 사용하며, 토큰 갱신 기능은 유지합니다.
// 업로드 등 HTTP 직접 요청을 테스트할 때 사용합니다.
func NewClientForTest(baseURL string, creds *auth.Credentials, tokenRefresher *auth.TokenRefresher) *Client {
	return &Client{
		backend:        nil,
		creds:          creds,
		tokenRefresher: tokenRefresher,
		httpClient:     &http.Client{Timeout: 30 * time.Second},
		workspaceID:    creds.WorkspaceID,
		baseURL:        baseURL,
	}
}
