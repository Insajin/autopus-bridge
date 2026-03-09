// testhelpers_test.go는 cmd 패키지 테스트에서 공통으로 사용하는 헬퍼 함수를 제공합니다.
package cmd

import (
	"encoding/json"
	"time"

	"github.com/insajin/autopus-bridge/internal/apiclient"
	"github.com/insajin/autopus-bridge/internal/auth"
	"github.com/insajin/autopus-bridge/internal/mcpserver"
	"github.com/rs/zerolog"
)

// makeTestClient는 테스트용 apiclient.Client를 생성합니다.
// 유효한 ExpiresAt을 설정하여 토큰 갱신이 트리거되지 않도록 합니다.
func makeTestClient(serverURL, workspaceID string) *apiclient.Client {
	creds := &auth.Credentials{
		AccessToken: "test-token",
		ServerURL:   serverURL,
		WorkspaceID: workspaceID,
		ExpiresAt:   time.Now().Add(1 * time.Hour),
	}
	tr := auth.NewTokenRefresher(creds)
	backend := mcpserver.NewBackendClient(serverURL, tr, 0, zerolog.Nop())
	return apiclient.New(backend, creds, tr)
}

// buildAPIResponse는 표준 성공 APIResponse JSON을 생성합니다.
func buildAPIResponse(data interface{}) []byte {
	payload, _ := json.Marshal(data)
	resp := map[string]interface{}{
		"success": true,
		"data":    json.RawMessage(payload),
	}
	b, _ := json.Marshal(resp)
	return b
}
