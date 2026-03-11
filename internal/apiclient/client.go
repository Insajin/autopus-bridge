package apiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/insajin/autopus-bridge/internal/auth"
	"github.com/insajin/autopus-bridge/internal/mcpserver"
)

// ValidateID는 ID 문자열이 허용된 형식인지 검사합니다.
// UUID 및 슬러그 형식 (영문, 숫자, -, _)만 허용합니다.
// 빈 문자열, 공백, 특수문자, 경로 순회 시도 등은 거부합니다.
// @MX:ANCHOR: [AUTO] issue/channel/message/project cmd 모든 run* 함수에서 호출하는 ID 검증 게이트웨이
// @MX:REASON: fan_in >= 14 (issue 7, channel 4, message 4 호출), 경로 순회 방어의 단일 진입점
// @MX:SPEC: SPEC-CLI-003 C2 ID 검증 요구사항
func ValidateID(id string) error {
	if id == "" {
		return fmt.Errorf("유효하지 않은 ID 형식입니다: 빈 문자열은 허용되지 않습니다")
	}
	for _, r := range id {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_') {
			return fmt.Errorf("유효하지 않은 ID 형식입니다: 허용되지 않는 문자 %q 포함", r)
		}
	}
	return nil
}

// DefaultAPITimeout은 CLI API 호출의 기본 타임아웃입니다.
const DefaultAPITimeout = 10 * time.Second

// NewContextWithTimeout은 타임아웃이 설정된 컨텍스트와 취소 함수를 반환합니다.
// 기본 사용: ctx, cancel := NewContextWithTimeout(DefaultAPITimeout); defer cancel()
// @MX:ANCHOR: [AUTO] 모든 run* API 호출 함수의 컨텍스트 타임아웃 생성 표준 패턴
// @MX:REASON: fan_in >= 20 (issue 7, channel 6, message 4, project 3 호출), W5 context timeout 보안 요구사항의 단일 진입점
// @MX:SPEC: SPEC-CLI-003 W5 컨텍스트 타임아웃 요구사항
func NewContextWithTimeout(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), timeout)
}

// Client는 CLI 명령에서 사용하는 API 클라이언트입니다.
// BackendClient를 감싸 편의 메서드와 JSON 출력 제어를 제공합니다.
// Meta(페이지네이션)를 포함한 전체 API 응답을 파싱하기 위해 내부 HTTP 클라이언트를 직접 사용합니다.
type Client struct {
	// @MX:NOTE: BackendClient는 고수준 메서드(ExecuteTask 등)를 위해 유지합니다.
	// // 기본 HTTP 요청은 Client가 직접 구현하여 Meta 필드를 포함한 전체 응답을 파싱합니다.
	backend        *mcpserver.BackendClient
	creds          *auth.Credentials
	tokenRefresher *auth.TokenRefresher
	httpClient     *http.Client
	workspaceID    string
	baseURL        string
	jsonOutput     bool
}

// New는 새 Client를 생성합니다.
// backend는 고수준 API 메서드에 사용하고, tokenRefresher는 토큰 자동 갱신에 사용합니다.
func New(backend *mcpserver.BackendClient, creds *auth.Credentials, tokenRefresher *auth.TokenRefresher) *Client {
	baseURL := ""
	if creds != nil {
		baseURL = wsURLToHTTPBase(creds.ServerURL)
	}
	workspaceID := ""
	if creds != nil {
		workspaceID = creds.WorkspaceID
	}
	return &Client{
		backend:        backend,
		creds:          creds,
		tokenRefresher: tokenRefresher,
		httpClient:     &http.Client{Timeout: 30 * time.Second},
		workspaceID:    workspaceID,
		baseURL:        baseURL,
	}
}

// Get은 GET 요청을 실행합니다.
func (c *Client) Get(ctx context.Context, path string) (*APIResponse, error) {
	return c.do(ctx, http.MethodGet, path, nil)
}

// Post는 POST 요청을 실행합니다.
func (c *Client) Post(ctx context.Context, path string, body interface{}) (*APIResponse, error) {
	return c.do(ctx, http.MethodPost, path, body)
}

// Patch는 PATCH 요청을 실행합니다.
func (c *Client) Patch(ctx context.Context, path string, body interface{}) (*APIResponse, error) {
	return c.do(ctx, http.MethodPatch, path, body)
}

// Delete는 DELETE 요청을 실행합니다.
func (c *Client) Delete(ctx context.Context, path string) (*APIResponse, error) {
	return c.do(ctx, http.MethodDelete, path, nil)
}

// do는 직접 HTTP 요청을 실행하고 APIResponse(Meta 포함)를 반환합니다.
// BackendClient.Do()는 Meta 필드가 없는 내부 타입을 반환하므로, Client가 직접 요청을 구현합니다.
func (c *Client) do(ctx context.Context, method, path string, body interface{}) (*APIResponse, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("요청 본문 직렬화 실패: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	url := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("HTTP 요청 생성 실패: %w", err)
	}

	// TokenRefresher에서 유효한 토큰 가져오기
	token, err := c.tokenRefresher.GetToken()
	if err != nil {
		return nil, fmt.Errorf("인증 토큰 획득 실패: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("백엔드 통신 실패: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("응답 읽기 실패: %w", err)
	}

	if resp.StatusCode >= 500 {
		return nil, fmt.Errorf("백엔드 서버 오류 (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	var apiResp APIResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("응답 파싱 실패 (HTTP %d): %w", resp.StatusCode, err)
	}

	if resp.StatusCode >= 400 {
		errMsg := apiResp.Error.String()
		if errMsg == "" {
			errMsg = apiResp.Message
		}
		if errMsg == "" {
			errMsg = fmt.Sprintf("HTTP %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("API 오류: %s", errMsg)
	}

	return &apiResp, nil
}

// Do는 제네릭 단건 응답 파싱 함수입니다.
// APIResponse.Data 필드를 T 타입으로 언마샬링합니다.
func Do[T any](c *Client, ctx context.Context, method, path string, body interface{}) (*T, error) {
	resp, err := c.do(ctx, method, path, body)
	if err != nil {
		return nil, err
	}
	var result T
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("응답 데이터 파싱 실패: %w", err)
	}
	return &result, nil
}

// DoList는 제네릭 배열 응답 파싱 함수입니다.
// APIResponse.Data 필드 (배열)를 []T로 언마샬링합니다.
func DoList[T any](c *Client, ctx context.Context, method, path string, body interface{}) ([]T, error) {
	resp, err := c.do(ctx, method, path, body)
	if err != nil {
		return nil, err
	}
	var result []T
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("응답 배열 파싱 실패: %w", err)
	}
	return result, nil
}

// DoPage는 제네릭 페이지네이션 응답 파싱 함수입니다.
// APIResponse.Data (배열)와 Meta를 함께 반환합니다.
func DoPage[T any](c *Client, ctx context.Context, method, path string, body interface{}) ([]T, *PageMeta, error) {
	resp, err := c.do(ctx, method, path, body)
	if err != nil {
		return nil, nil, err
	}
	var result []T
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, nil, fmt.Errorf("응답 배열 파싱 실패: %w", err)
	}
	return result, resp.Meta, nil
}

// DoRaw는 HTTP 요청을 실행하고 상태 코드와 응답 본문을 반환합니다.
// api 명령어 등에서 HTTP 상태 코드를 직접 캡처할 때 사용합니다.
func (c *Client) DoRaw(ctx context.Context, method, path string, body interface{}, extraHeaders map[string]string) (int, []byte, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return 0, nil, fmt.Errorf("요청 본문 직렬화 실패: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	url := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return 0, nil, fmt.Errorf("HTTP 요청 생성 실패: %w", err)
	}

	token, tokenErr := c.tokenRefresher.GetToken()
	if tokenErr != nil {
		return 0, nil, fmt.Errorf("인증 토큰 획득 실패: %w", tokenErr)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}

	resp, doErr := c.httpClient.Do(req)
	if doErr != nil {
		return 0, nil, fmt.Errorf("API 요청 실패: %w", doErr)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return resp.StatusCode, nil, fmt.Errorf("응답 읽기 실패: %w", readErr)
	}

	return resp.StatusCode, respBody, nil
}

// ResolvePath는 경로의 :workspaceId를 실제 워크스페이스 ID로 치환합니다.
func (c *Client) ResolvePath(path string) string {
	if c.workspaceID == "" {
		return path
	}
	return strings.ReplaceAll(path, ":workspaceId", c.workspaceID)
}

// SetJSONOutput은 JSON 출력 여부를 설정합니다.
func (c *Client) SetJSONOutput(json bool) {
	c.jsonOutput = json
}

// IsJSONOutput은 JSON 출력 여부를 반환합니다.
func (c *Client) IsJSONOutput() bool {
	return c.jsonOutput
}

// BaseURL은 백엔드 기본 URL을 반환합니다. SSE 연결 시 사용합니다.
func (c *Client) BaseURL() string {
	return c.baseURL
}

// WorkspaceID는 현재 워크스페이스 ID를 반환합니다.
func (c *Client) WorkspaceID() string {
	return c.workspaceID
}

// wsURLToHTTPBase는 WebSocket URL을 HTTP 기본 URL로 변환합니다.
// 예: "wss://api.autopus.co/ws/agent" → "https://api.autopus.co"
func wsURLToHTTPBase(wsURL string) string {
	httpURL := strings.TrimRight(wsURL, "/")
	if strings.HasPrefix(httpURL, "wss://") {
		httpURL = "https://" + strings.TrimPrefix(httpURL, "wss://")
	} else if strings.HasPrefix(httpURL, "ws://") {
		httpURL = "http://" + strings.TrimPrefix(httpURL, "ws://")
	}
	// /ws/agent 등 WebSocket 경로 제거
	if idx := strings.Index(httpURL, "/ws"); idx != -1 {
		httpURL = httpURL[:idx]
	}
	return httpURL
}

// Token은 현재 유효한 인증 토큰을 반환합니다. SSE 연결 시 사용합니다.
// TokenRefresher.GetToken()을 호출하여 만료된 토큰을 자동 갱신합니다.
func (c *Client) Token() (string, error) {
	return c.tokenRefresher.GetToken()
}
