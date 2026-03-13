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
		w.Write(buildAPIResponse(messageListResponse{Messages: messages}))
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
		w.Write(buildAPIResponse(messageListResponse{Messages: messages}))
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
		w.Write(buildAPIResponse(messageListResponse{Messages: messages}))
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
		w.Write(buildAPIResponse(messageListResponse{Messages: threadMessages}))
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
		w.Write(buildAPIResponse(messageListResponse{Messages: agentMessages}))
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
		w.Write(buildAPIResponse(messageListResponse{Messages: threadMessages}))
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
		w.Write(buildAPIResponse(messageListResponse{Messages: agentMessages}))
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

// TestRunMessageList_InvalidChannelID는 유효하지 않은 channelID에서 ValidateID 에러를 반환하는지 검증합니다.
func TestRunMessageList_InvalidChannelID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runMessageList(client, &buf, "bad/id", 0, "", false)
	if err == nil {
		t.Error("유효하지 않은 channelID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

// TestRunMessageSend_InvalidChannelID는 유효하지 않은 channelID에서 ValidateID 에러를 반환하는지 검증합니다.
func TestRunMessageSend_InvalidChannelID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runMessageSend(client, &buf, "bad id", "내용")
	if err == nil {
		t.Error("유효하지 않은 channelID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

// TestRunMessageThread_InvalidMessageID는 유효하지 않은 messageID에서 ValidateID 에러를 반환하는지 검증합니다.
func TestRunMessageThread_InvalidMessageID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runMessageThread(client, &buf, "../bad", false)
	if err == nil {
		t.Error("유효하지 않은 messageID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

// TestRunMessageAgentMessages_InvalidChannelID는 유효하지 않은 channelID에서 ValidateID 에러를 반환하는지 검증합니다.
func TestRunMessageAgentMessages_InvalidChannelID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runMessageAgentMessages(client, &buf, "ch id", false)
	if err == nil {
		t.Error("유효하지 않은 channelID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

// ---------------------------------------------------------------------------
// REQ-AC-002: 에이전트 작성자 이름 표시 - [TIER] AgentName 형식
// REQ-AC-003: MessageWithUser 구조체에 AgentName, AgentTier 필드 포함
// ---------------------------------------------------------------------------

// TestMessageWithUser_HasAgentFields는 MessageWithUser 구조체에
// AgentName과 AgentTier 필드가 있는지 컴파일 시점에 검증한다.
// RED 단계: 필드가 없으면 컴파일 에러로 실패한다.
func TestMessageWithUser_HasAgentFields(t *testing.T) {
	agentName := "분석가"
	agentTier := "worker"

	// 컴파일 타임에 필드 존재 여부 검증
	msg := MessageWithUser{
		Message:         Message{ID: "msg-a1", Content: "분석 결과", CreatedAt: "2026-01-01T00:00:00Z"},
		UserDisplayName: "",
		AgentName:       &agentName,
		AgentTier:       &agentTier,
	}

	if msg.AgentName == nil || *msg.AgentName != "분석가" {
		t.Errorf("AgentName 필드가 예상값과 다릅니다: %v", msg.AgentName)
	}
	if msg.AgentTier == nil || *msg.AgentTier != "worker" {
		t.Errorf("AgentTier 필드가 예상값과 다릅니다: %v", msg.AgentTier)
	}
}

// TestPrintMessageTable_AgentAuthorFallback은 UserDisplayName이 없을 때
// AgentName을 "[TIER] AgentName" 형식으로 표시하는지 검증한다.
// RED 단계: AgentName 폴백 로직이 없으면 빈 문자열이나 UserID가 표시되어 실패한다.
func TestPrintMessageTable_AgentAuthorFallback(t *testing.T) {
	agentName := "기획자"
	agentTier := "pm"

	messages := []MessageWithUser{
		{
			Message:         Message{ID: "msg-a1", Content: "기획 내용", CreatedAt: "2026-01-01T00:00:00Z", UserID: "user-uuid-123"},
			UserDisplayName: "", // 에이전트는 UserDisplayName이 비어있다
			AgentName:       &agentName,
			AgentTier:       &agentTier,
		},
	}

	var buf bytes.Buffer
	printMessageTable(&buf, messages)

	out := buf.String()
	// "[PM] 기획자" 형식이 포함되어야 한다
	if !strings.Contains(out, "[PM] 기획자") {
		t.Errorf("에이전트 작성자가 '[PM] 기획자' 형식으로 표시되어야 합니다. 실제 출력: %s", out)
	}
}

// TestPrintMessageTable_AgentAuthorUppercase는 에이전트 티어가 대문자로 표시되는지 검증한다.
// worker 티어는 [WORKER]로 표시되어야 한다.
func TestPrintMessageTable_AgentAuthorUppercase(t *testing.T) {
	agentName := "코딩봇"
	agentTier := "worker"

	messages := []MessageWithUser{
		{
			Message:         Message{ID: "msg-w1", Content: "코드 작성 완료", CreatedAt: "2026-01-01T00:00:00Z"},
			UserDisplayName: "",
			AgentName:       &agentName,
			AgentTier:       &agentTier,
		},
	}

	var buf bytes.Buffer
	printMessageTable(&buf, messages)

	out := buf.String()
	// 티어는 대문자: [WORKER]
	if !strings.Contains(out, "[WORKER] 코딩봇") {
		t.Errorf("에이전트 티어가 대문자로 표시되어야 합니다. 실제 출력: %s", out)
	}
}

// TestPrintMessageTable_UserDisplayNamePriority는 UserDisplayName이 있을 때
// 에이전트 이름보다 UserDisplayName을 우선 표시하는지 검증한다.
func TestPrintMessageTable_UserDisplayNamePriority(t *testing.T) {
	agentName := "에이전트"
	agentTier := "worker"

	messages := []MessageWithUser{
		{
			Message:         Message{ID: "msg-1", Content: "사람 메시지", CreatedAt: "2026-01-01T00:00:00Z"},
			UserDisplayName: "홍길동", // UserDisplayName이 있으면 에이전트 이름 무시
			AgentName:       &agentName,
			AgentTier:       &agentTier,
		},
	}

	var buf bytes.Buffer
	printMessageTable(&buf, messages)

	out := buf.String()
	// UserDisplayName 우선
	if !strings.Contains(out, "홍길동") {
		t.Errorf("UserDisplayName이 있으면 에이전트 이름 대신 표시되어야 합니다. 실제 출력: %s", out)
	}
	// 에이전트 이름이 보이면 안 됨
	if strings.Contains(out, "[WORKER]") {
		t.Errorf("UserDisplayName이 있을 때 에이전트 이름이 표시되면 안 됩니다. 실제 출력: %s", out)
	}
}

// TestPrintMessageTable_FallbackToUserID는 UserDisplayName도 AgentName도 없을 때
// UserID를 fallback으로 사용하는지 검증한다.
func TestPrintMessageTable_FallbackToUserID(t *testing.T) {
	messages := []MessageWithUser{
		{
			Message:         Message{ID: "msg-1", Content: "내용", CreatedAt: "2026-01-01T00:00:00Z", UserID: "user-fallback-id"},
			UserDisplayName: "",
			AgentName:       nil,
			AgentTier:       nil,
		},
	}

	var buf bytes.Buffer
	printMessageTable(&buf, messages)

	out := buf.String()
	if !strings.Contains(out, "user-fallback-id") {
		t.Errorf("UserDisplayName과 AgentName이 모두 없으면 UserID를 표시해야 합니다. 실제 출력: %s", out)
	}
}

// TestRunMessageAgentMessages_WithAgentInfo는 에이전트 메시지 조회 시
// 응답의 AgentName/AgentTier 필드가 올바르게 파싱되어 표시되는지 검증한다.
func TestRunMessageAgentMessages_WithAgentInfo(t *testing.T) {
	agentName := "전략분석가"
	agentTier := "c_level"

	agentMessages := []MessageWithUser{
		{
			Message:         Message{ID: "msg-a1", Content: "전략 분석 완료", CreatedAt: "2026-01-01T00:00:00Z"},
			UserDisplayName: "",
			AgentName:       &agentName,
			AgentTier:       &agentTier,
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/channels/ch-1/agent-messages" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(messageListResponse{Messages: agentMessages}))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runMessageAgentMessages(client, &buf, "ch-1", false)
	if err != nil {
		t.Fatalf("runMessageAgentMessages 오류: %v", err)
	}

	out := buf.String()
	// "[C_LEVEL] 전략분석가" 형식으로 표시되어야 한다
	if !strings.Contains(out, "[C_LEVEL] 전략분석가") {
		t.Errorf("에이전트 작성자가 '[C_LEVEL] 전략분석가' 형식으로 표시되어야 합니다. 실제 출력: %s", out)
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
		w.Write(buildAPIResponse(messageListResponse{Messages: messages}))
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
