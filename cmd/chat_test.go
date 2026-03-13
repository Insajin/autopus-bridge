package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/insajin/autopus-bridge/internal/auth"
	"github.com/insajin/autopus-bridge/internal/mcpserver"
	"github.com/rs/zerolog"

	"github.com/insajin/autopus-bridge/internal/apiclient"
)

func makeChatTestClient(serverURL, workspaceID string) *apiclient.Client {
	creds := &auth.Credentials{
		AccessToken: "test-token",
		ServerURL:   serverURL,
		WorkspaceID: workspaceID,
		ExpiresAt:   time.Now().Add(1 * time.Hour), // 만료되지 않은 토큰
	}
	tr := auth.NewTokenRefresher(creds)
	backend := mcpserver.NewBackendClient(serverURL, tr, 0, zerolog.Nop())
	return apiclient.New(backend, creds, tr)
}

func TestResolveDMChannel_CEO(t *testing.T) {
	// CEO 에이전트의 경우 /dm-channels/ensure-ceo 엔드포인트 사용
	channel := DMChannel{ID: "ch-ceo", AgentID: "ag-ceo", ChannelType: "dm"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || !strings.Contains(r.URL.Path, "ensure-ceo") {
			http.Error(w, "not found: "+r.Method+" "+r.URL.Path, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		payload, _ := json.Marshal(channel)
		resp := map[string]interface{}{
			"success": true,
			"data":    json.RawMessage(payload),
		}
		b, _ := json.Marshal(resp)
		w.Write(b)
	}))
	defer srv.Close()

	client := makeChatTestClient(srv.URL, "ws-1")

	agent := &DashboardAgent{ID: "ag-ceo", Name: "CEO"}
	ch, err := resolveDMChannel(client, agent)
	if err != nil {
		t.Fatalf("resolveDMChannel CEO 오류: %v", err)
	}
	if ch.ID != "ch-ceo" {
		t.Errorf("채널 ID = %q, want %q", ch.ID, "ch-ceo")
	}
}

func TestResolveDMChannel_NonCEO(t *testing.T) {
	// 일반 에이전트의 경우 DM 채널 목록 조회 후 필터링
	channels := []DMChannel{
		{ID: "ch-1", AgentID: "ag-cto", ChannelType: "dm"},
		{ID: "ch-2", AgentID: "ag-other", ChannelType: "dm"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "dm-channels") {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		payload, _ := json.Marshal(channels)
		resp := map[string]interface{}{
			"success": true,
			"data":    json.RawMessage(payload),
		}
		b, _ := json.Marshal(resp)
		w.Write(b)
	}))
	defer srv.Close()

	client := makeChatTestClient(srv.URL, "ws-1")

	agent := &DashboardAgent{ID: "ag-cto", Name: "CTO"}
	ch, err := resolveDMChannel(client, agent)
	if err != nil {
		t.Fatalf("resolveDMChannel 오류: %v", err)
	}
	if ch.ID != "ch-1" {
		t.Errorf("채널 ID = %q, want %q", ch.ID, "ch-1")
	}
}

func TestSendChatMessage(t *testing.T) {
	var receivedBody map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || !strings.Contains(r.URL.Path, "/messages") {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"success": true,
			"data":    map[string]string{"id": "msg-1"},
		}
		b, _ := json.Marshal(resp)
		w.Write(b)
	}))
	defer srv.Close()

	client := makeChatTestClient(srv.URL, "ws-1")

	err := sendChatMessage(client, "ch-1", "Hello, World!")
	if err != nil {
		t.Fatalf("sendChatMessage 오류: %v", err)
	}

	if receivedBody == nil {
		t.Fatal("요청 본문이 수신되지 않았습니다")
	}
	if content, ok := receivedBody["content"].(string); !ok || content != "Hello, World!" {
		t.Errorf("수신된 본문 content: %v", receivedBody["content"])
	}
}

// TestRunChatHistory는 실제 백엔드 응답 포맷(wrapper struct)으로 채팅 히스토리를 파싱하는 테스트.
// 백엔드는 {data: {messages:[], has_more, first_unread_id}} 형태로 응답한다.
func TestRunChatHistory(t *testing.T) {
	messages := []ChatMessage{
		{ID: "msg-1", Content: "Hello", Role: "user"},
		{ID: "msg-2", Content: "Hi there!", Role: "assistant"},
	}

	// 실제 백엔드와 동일한 형태: data 안에 messages 배열이 중첩된 구조
	type wrappedResponse struct {
		Messages      []ChatMessage `json:"messages"`
		HasMore       bool          `json:"has_more"`
		FirstUnreadID *string       `json:"first_unread_id"`
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/messages") {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		wrapped := wrappedResponse{Messages: messages, HasMore: false}
		payload, _ := json.Marshal(wrapped)
		resp := map[string]interface{}{
			"success": true,
			"data":    json.RawMessage(payload),
		}
		b, _ := json.Marshal(resp)
		w.Write(b)
	}))
	defer srv.Close()

	client := makeChatTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runChatHistory(client, &buf, "ch-1", 10, false)
	if err != nil {
		t.Fatalf("runChatHistory 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Hello") {
		t.Errorf("출력에 'Hello'가 없습니다: %s", out)
	}
}

// TAG-002 RED: 실제 백엔드 응답 포맷(wrapper struct)으로 채팅 히스토리를 파싱하는 테스트
// 백엔드는 {data: {messages:[], has_more, first_unread_id}} 형태로 응답한다.
// 현재 DoList[ChatMessage]는 data를 직접 배열로 파싱하려 하므로 실패한다.
// GREEN에서는 Do[chatHistoryResponse]로 변경해야 이 테스트가 통과한다.
func TestRunChatHistory_WrappedFormat(t *testing.T) {
	messages := []ChatMessage{
		{ID: "msg-1", Content: "Hello", Role: "user"},
		{ID: "msg-2", Content: "Hi there!", Role: "assistant"},
	}

	// 실제 백엔드 응답 형태: data 안에 messages 배열이 중첩된 구조
	type wrappedResponse struct {
		Messages      []ChatMessage `json:"messages"`
		HasMore       bool          `json:"has_more"`
		FirstUnreadID *string       `json:"first_unread_id"`
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/messages") {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		// 실제 백엔드와 동일: data가 배열이 아닌 객체
		wrapped := wrappedResponse{
			Messages: messages,
			HasMore:  false,
		}
		payload, _ := json.Marshal(wrapped)
		resp := map[string]interface{}{
			"success": true,
			"data":    json.RawMessage(payload),
		}
		b, _ := json.Marshal(resp)
		w.Write(b)
	}))
	defer srv.Close()

	client := makeChatTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	// GREEN 단계에서 runChatHistory가 wrapper struct를 사용해야 이 테스트가 통과한다
	err := runChatHistory(client, &buf, "ch-1", 10, false)
	if err != nil {
		t.Fatalf("runChatHistory wrapper 포맷 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Hello") {
		t.Errorf("출력에 'Hello'가 없습니다: %s", out)
	}
	if !strings.Contains(out, "Hi there!") {
		t.Errorf("출력에 'Hi there!'가 없습니다: %s", out)
	}
}
