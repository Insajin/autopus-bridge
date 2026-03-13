package provider

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/insajin/autopus-codex-rpc/client"
	"github.com/insajin/autopus-codex-rpc/protocol"
)

// newTestCodexProvider는 테스트용 CodexProvider를 생성합니다 (httpClient 포함).
func newTestCodexProvider(appServer *CodexAppServerProvider) *CodexProvider {
	return &CodexProvider{
		httpClient: &http.Client{Timeout: 5 * time.Second},
		config:     ProviderConfig{DefaultModel: "gpt-5.4", APIKey: "test-key"},
		baseURL:    "http://unreachable.test",
		appServer:  appServer,
	}
}

// TestCodexProvider_DelegateToAppServer는
// App Server가 사용 가능할 때 Execute()가 App Server로 위임되는지 검증합니다.
func TestCodexProvider_DelegateToAppServer(t *testing.T) {
	mock := newMockAppServer()
	appServerClient := mock.createClient()
	appServerProv := createMockProvider(appServerClient, "auto-approve")

	// CodexProvider에 App Server 주입
	codexProv := newTestCodexProvider(appServerProv)

	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		remaining := make([]byte, 0, 4096)

		// thread/start 응답
		req, err := mock.readRequest(&remaining)
		if err != nil {
			return
		}
		if err := mock.sendResponse(req.ID, protocol.ThreadStartResult{ThreadID: "thread-delegate"}); err != nil {
			return
		}

		// turn/start 응답
		req, err = mock.readRequest(&remaining)
		if err != nil {
			return
		}
		if err := mock.sendResponse(req.ID, protocol.TurnStartResult{TurnID: "turn-delegate"}); err != nil {
			return
		}

		// 에이전트 메시지 델타
		if err := mock.sendNotification(protocol.MethodAgentMessageDelta, protocol.AgentMessageDelta{Delta: "App Server 응답"}); err != nil {
			return
		}

		time.Sleep(20 * time.Millisecond)

		// Turn 완료
		if err := mock.sendNotification(protocol.MethodTurnCompleted, protocol.TurnCompletedParams{ThreadID: "thread-delegate"}); err != nil {
			return
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := codexProv.Execute(ctx, ExecuteRequest{
		Prompt: "위임 테스트",
		Model:  "gpt-5-codex",
	})

	mock.close()
	<-serverDone
	appServerClient.Close()

	if err != nil {
		t.Fatalf("Execute 실패: %v", err)
	}

	if resp.Output != "App Server 응답" {
		t.Errorf("Output: got %q, want %q", resp.Output, "App Server 응답")
	}

	// App Server 프로바이더 이름이 반환되어야 함
	if resp.Provider != "codex" {
		t.Errorf("Provider: got %q, want %q", resp.Provider, "codex")
	}
}

// TestCodexProvider_FallbackToHTTP는
// App Server가 없을 때 HTTP 경로로 폴백되는지 검증합니다.
// (실제 HTTP 호출은 발생하지 않도록 baseURL을 비어두지 않고 nil appServer로 확인)
func TestCodexProvider_FallbackToHTTP_NoAppServer(t *testing.T) {
	codexProv := newTestCodexProvider(nil) // App Server 없음

	// App Server가 없으면 HTTP 경로를 시도해야 한다.
	// HTTP 호출은 실패하겠지만, App Server로 위임하려는 시도는 없어야 함.
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	_, err := codexProv.Execute(ctx, ExecuteRequest{
		Prompt: "HTTP 폴백 테스트",
		Model:  "gpt-5-codex",
	})

	// HTTP 에러(연결 실패)가 예상됨 (App Server 위임 에러가 아님)
	if err == nil {
		t.Fatal("에러를 기대했지만 nil을 받았습니다")
	}

	// ErrProcessNotRunning이 아닌 HTTP 에러여야 함
	if err == ErrProcessNotRunning {
		t.Error("ErrProcessNotRunning이 반환됨 — HTTP 폴백이 아니라 App Server 경로를 탄 것")
	}
}

// TestCodexProvider_SetAppServer는
// SetAppServer 메서드로 App Server를 동적으로 등록할 수 있는지 검증합니다.
func TestCodexProvider_SetAppServer(t *testing.T) {
	codexProv := newTestCodexProvider(nil)

	if codexProv.appServer != nil {
		t.Error("초기 상태에서 appServer가 nil이 아닙니다")
	}

	mock := newMockAppServer()
	appServerClient := mock.createClient()
	appServerProv := createMockProvider(appServerClient, "auto-approve")

	codexProv.SetAppServer(appServerProv)

	if codexProv.appServer == nil {
		t.Error("SetAppServer 후 appServer가 nil입니다")
	}

	if codexProv.appServer != appServerProv {
		t.Error("SetAppServer 후 appServer가 올바르지 않습니다")
	}

	// App Server가 실행 중이면 IsAppServerAvailable이 true여야 함
	if !codexProv.IsAppServerAvailable() {
		t.Error("App Server가 실행 중이지만 IsAppServerAvailable()이 false입니다")
	}

	mock.close()
	appServerClient.Close()
}

// TestCodexProvider_AppServerUnavailable_FallbackToHTTP는
// App Server가 등록되었지만 실행 중이지 않을 때 HTTP로 폴백되는지 검증합니다.
func TestCodexProvider_AppServerUnavailable_FallbackToHTTP(t *testing.T) {
	mock := newMockAppServer()
	appServerClient := mock.createClient()
	appServerProv := createMockProvider(appServerClient, "auto-approve")
	// App Server 프로세스를 중지 상태로 설정
	appServerProv.process.running.Store(false)

	codexProv := newTestCodexProvider(appServerProv)

	// App Server가 중지 상태이면 IsAppServerAvailable이 false여야 함
	if codexProv.IsAppServerAvailable() {
		t.Error("App Server가 중지 상태인데 IsAppServerAvailable()이 true입니다")
	}

	// HTTP 폴백이 시도됨 (연결 실패가 예상됨)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	_, err := codexProv.Execute(ctx, ExecuteRequest{
		Prompt: "App Server 미실행 폴백 테스트",
		Model:  "gpt-5-codex",
	})

	mock.close()
	appServerClient.Close()

	// HTTP 에러가 예상됨
	if err == nil {
		t.Fatal("에러를 기대했지만 nil을 받았습니다")
	}

	// App Server를 통한 ErrProcessNotRunning이 아닌 HTTP 에러여야 함
	if err == ErrProcessNotRunning {
		t.Error("ErrProcessNotRunning이 반환됨 — HTTP 폴백이 아님")
	}
}

// TestCodexProvider_WithAppServerOption는
// WithCodexAppServer 옵션으로 App Server를 초기화 시 설정할 수 있는지 검증합니다.
func TestCodexProvider_WithAppServerOption(t *testing.T) {
	mock := newMockAppServer()
	c := mock.createClient()
	appServerProv := createMockProvider(c, "auto-approve")

	// WithCodexAppServer 옵션으로 생성 (API 키 검증 없이 직접 구성)
	codexProv := newTestCodexProvider(nil)

	opt := WithCodexAppServer(appServerProv)
	opt(codexProv)

	if codexProv.appServer != appServerProv {
		t.Error("WithCodexAppServer 적용 후 appServer가 올바르지 않습니다")
	}

	mock.close()
	c.Close()
}

// TestCodexProvider_IsAppServerAvailable는
// appServer 등록 여부와 실행 상태에 따른 IsAppServerAvailable 동작을 검증합니다.
func TestCodexProvider_IsAppServerAvailable(t *testing.T) {
	t.Run("appServer nil이면 false", func(t *testing.T) {
		p := &CodexProvider{}
		if p.IsAppServerAvailable() {
			t.Error("appServer가 nil인데 IsAppServerAvailable()이 true입니다")
		}
	})

	t.Run("appServer 실행 중이면 true", func(t *testing.T) {
		mock := newMockAppServer()
		c := mock.createClient()
		appServerProv := createMockProvider(c, "auto-approve")

		p := &CodexProvider{appServer: appServerProv}
		if !p.IsAppServerAvailable() {
			t.Error("appServer가 실행 중인데 IsAppServerAvailable()이 false입니다")
		}

		mock.close()
		c.Close()
	})

	t.Run("appServer 중지 상태면 false", func(t *testing.T) {
		mock := newMockAppServer()
		c := mock.createClient()
		appServerProv := createMockProvider(c, "auto-approve")
		appServerProv.process.running.Store(false)

		p := &CodexProvider{appServer: appServerProv}
		if p.IsAppServerAvailable() {
			t.Error("appServer가 중지 상태인데 IsAppServerAvailable()이 true입니다")
		}

		mock.close()
		c.Close()
	})
}

// Ensure unused client import is avoided by using it in the test
var _ = client.NopLogger
