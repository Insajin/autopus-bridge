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

func TestRunChatHistory(t *testing.T) {
	messages := []ChatMessage{
		{ID: "msg-1", Content: "Hello", Role: "user"},
		{ID: "msg-2", Content: "Hi there!", Role: "assistant"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/messages") {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		payload, _ := json.Marshal(messages)
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
