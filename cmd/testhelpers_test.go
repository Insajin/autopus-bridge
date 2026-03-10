// testhelpers_test.go는 cmd 패키지 테스트에서 공통으로 사용하는 헬퍼 함수를 제공합니다.
package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
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

// buildAPIListResponse는 배열 데이터를 표준 성공 APIResponse JSON으로 생성합니다.
func buildAPIListResponse(data interface{}) []byte {
	return buildAPIResponse(data)
}

// ServerStep은 stateful mock 서버의 단일 요청-응답 스텝을 정의합니다.
type ServerStep struct {
	Method       string
	PathPrefix   string
	ResponseData interface{}
	StatusCode   int
	Validate     func(t *testing.T, r *http.Request)
}

// buildStatefulServer는 요청 순서에 따라 상태를 전이하는 mock 서버를 생성합니다.
// 각 요청은 steps 배열의 순서대로 매칭되며, 매칭된 스텝의 응답을 반환합니다.
func buildStatefulServer(t *testing.T, steps []ServerStep) *httptest.Server {
	t.Helper()

	var mu sync.Mutex
	stepIdx := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		idx := stepIdx
		if idx < len(steps) {
			stepIdx++
		}
		mu.Unlock()

		if idx >= len(steps) {
			http.Error(w, fmt.Sprintf("unexpected request #%d: %s %s", idx, r.Method, r.URL.Path), http.StatusInternalServerError)
			return
		}

		step := steps[idx]

		if step.Method != "" && r.Method != step.Method {
			http.Error(w, fmt.Sprintf("step %d: expected method %s, got %s", idx, step.Method, r.Method), http.StatusMethodNotAllowed)
			return
		}

		if step.PathPrefix != "" && len(r.URL.Path) < len(step.PathPrefix) {
			http.Error(w, fmt.Sprintf("step %d: path %s does not match prefix %s", idx, r.URL.Path, step.PathPrefix), http.StatusNotFound)
			return
		}
		if step.PathPrefix != "" && r.URL.Path[:len(step.PathPrefix)] != step.PathPrefix {
			http.Error(w, fmt.Sprintf("step %d: path %s does not match prefix %s", idx, r.URL.Path, step.PathPrefix), http.StatusNotFound)
			return
		}

		if step.Validate != nil {
			step.Validate(t, r)
		}

		statusCode := step.StatusCode
		if statusCode == 0 {
			statusCode = http.StatusOK
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		w.Write(buildAPIResponse(step.ResponseData))
	}))

	t.Cleanup(func() {
		srv.Close()
		mu.Lock()
		remaining := len(steps) - stepIdx
		mu.Unlock()
		if remaining > 0 {
			t.Errorf("stateful server: %d step(s) were not consumed", remaining)
		}
	})

	return srv
}
