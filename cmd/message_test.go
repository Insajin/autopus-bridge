// message_test.go는 message 서브커맨드 핸들러 함수를 테스트합니다.
package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunMessageList(t *testing.T) {
	messages := []MessageWithUser{
		{Message: Message{ID: "msg-1", Content: "Hello world", CreatedAt: "2026-01-01T00:00:00Z"}, UserDisplayName: "Alice"},
		{Message: Message{ID: "msg-2", Content: "How are you", CreatedAt: "2026-01-01T01:00:00Z"}, UserDisplayName: "Bob"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/channels/ch-1/messages" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(messages))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runMessageList(client, &buf, "ch-1", 0, "", false)
	if err != nil {
		t.Fatalf("runMessageList 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Alice") {
		t.Errorf("출력에 'Alice'가 없습니다: %s", out)
	}
	if !strings.Contains(out, "Hello world") {
		t.Errorf("출력에 'Hello world'가 없습니다: %s", out)
	}
}

func TestRunMessageListWithParams(t *testing.T) {
	messages := []MessageWithUser{
		{Message: Message{ID: "msg-5", Content: "Earlier message", CreatedAt: "2026-01-01T00:00:00Z"}, UserDisplayName: "Charlie"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// limit, before 파라미터가 쿼리에 포함되어야 합니다
		if r.URL.Path != "/api/v1/channels/ch-1/messages" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(messages))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runMessageList(client, &buf, "ch-1", 10, "msg-10", false)
	if err != nil {
		t.Fatalf("runMessageList with params 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Charlie") {
		t.Errorf("출력에 'Charlie'가 없습니다: %s", out)
	}
}

func TestRunMessageListJSON(t *testing.T) {
	messages := []MessageWithUser{
		{Message: Message{ID: "msg-1", Content: "Hello world", CreatedAt: "2026-01-01T00:00:00Z"}, UserDisplayName: "Alice"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(messages))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runMessageList(client, &buf, "ch-1", 0, "", true)
	if err != nil {
		t.Fatalf("runMessageList JSON 오류: %v", err)
	}

	out := buf.String()
	var parsed []MessageWithUser
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, out)
	}
	if len(parsed) != 1 || parsed[0].ID != "msg-1" {
		t.Errorf("예상치 않은 결과: %v", parsed)
	}
}

func TestRunMessageSend(t *testing.T) {
	sentMessage := Message{ID: "msg-new", Content: "Hello from CLI!", CreatedAt: "2026-01-01T12:00:00Z"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/channels/ch-1/messages" || r.Method != http.MethodPost {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(sentMessage))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runMessageSend(client, &buf, "ch-1", "Hello from CLI!")
	if err != nil {
		t.Fatalf("runMessageSend 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "msg-new") {
		t.Errorf("출력에 'msg-new'가 없습니다: %s", out)
	}
}

func TestRunMessageThread(t *testing.T) {
	threadMessages := []MessageWithUser{
		{Message: Message{ID: "msg-1", Content: "Thread root", CreatedAt: "2026-01-01T00:00:00Z"}, UserDisplayName: "Alice"},
		{Message: Message{ID: "msg-2", Content: "Thread reply", CreatedAt: "2026-01-01T01:00:00Z"}, UserDisplayName: "Bob"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/messages/msg-1/thread" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(threadMessages))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runMessageThread(client, &buf, "msg-1", false)
	if err != nil {
		t.Fatalf("runMessageThread 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Thread root") {
		t.Errorf("출력에 'Thread root'가 없습니다: %s", out)
	}
	if !strings.Contains(out, "Thread reply") {
		t.Errorf("출력에 'Thread reply'가 없습니다: %s", out)
	}
}

func TestRunMessageAgentMessages(t *testing.T) {
	agentMessages := []MessageWithUser{
		{Message: Message{ID: "msg-a1", Content: "Agent response", CreatedAt: "2026-01-01T00:00:00Z", Type: "agent"}, UserDisplayName: "AI Agent"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/channels/ch-1/agent-messages" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(agentMessages))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runMessageAgentMessages(client, &buf, "ch-1", false)
	if err != nil {
		t.Fatalf("runMessageAgentMessages 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "AI Agent") {
		t.Errorf("출력에 'AI Agent'가 없습니다: %s", out)
	}
	if !strings.Contains(out, "Agent response") {
		t.Errorf("출력에 'Agent response'가 없습니다: %s", out)
	}
}

// TestRunMessageSendError는 runMessageSend의 API 에러 경로를 테스트합니다.
func TestRunMessageSendError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 500 응답으로 에러 유발
		http.Error(w, `{"success":false,"error":"server error"}`, http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runMessageSend(client, &buf, "ch-1", "test message")
	if err == nil {
		t.Fatal("에러가 예상되었으나 nil 반환")
	}
}

// TestRunMessageThreadJSON는 runMessageThread의 JSON 출력 경로를 테스트합니다.
func TestRunMessageThreadJSON(t *testing.T) {
	threadMessages := []MessageWithUser{
		{Message: Message{ID: "msg-1", Content: "Thread root", CreatedAt: "2026-01-01T00:00:00Z"}, UserDisplayName: "Alice"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(threadMessages))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runMessageThread(client, &buf, "msg-1", true)
	if err != nil {
		t.Fatalf("runMessageThread JSON 오류: %v", err)
	}

	// 유효한 JSON 배열 출력 확인
	var parsed []MessageWithUser
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if len(parsed) != 1 || parsed[0].ID != "msg-1" {
		t.Errorf("예상치 않은 결과: %v", parsed)
	}
}

// TestRunMessageAgentMessagesJSON는 runMessageAgentMessages의 JSON 출력 경로를 테스트합니다.
func TestRunMessageAgentMessagesJSON(t *testing.T) {
	agentMessages := []MessageWithUser{
		{Message: Message{ID: "msg-a1", Content: "Agent response", CreatedAt: "2026-01-01T00:00:00Z", Type: "agent"}, UserDisplayName: "AI Agent"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(agentMessages))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runMessageAgentMessages(client, &buf, "ch-1", true)
	if err != nil {
		t.Fatalf("runMessageAgentMessages JSON 오류: %v", err)
	}

	// 유효한 JSON 배열 출력 확인
	var parsed []MessageWithUser
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if len(parsed) != 1 || parsed[0].ID != "msg-a1" {
		t.Errorf("예상치 않은 결과: %v", parsed)
	}
}

func TestRunMessageContentTruncation(t *testing.T) {
	// 80자 초과 콘텐츠는 잘려야 합니다
	longContent := strings.Repeat("A", 100)
	messages := []MessageWithUser{
		{Message: Message{ID: "msg-1", Content: longContent, CreatedAt: "2026-01-01T00:00:00Z"}, UserDisplayName: "Alice"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(messages))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runMessageList(client, &buf, "ch-1", 0, "", false)
	if err != nil {
		t.Fatalf("runMessageList 오류: %v", err)
	}

	out := buf.String()
	// 출력에 원본 100자 문자열 전체가 그대로 있으면 안 됩니다
	if strings.Contains(out, longContent) {
		t.Errorf("긴 콘텐츠가 잘리지 않았습니다: %s", out)
	}
	// "..."이 포함되어야 합니다
	if !strings.Contains(out, "...") {
		t.Errorf("출력에 '...'이 없습니다: %s", out)
	}
}
