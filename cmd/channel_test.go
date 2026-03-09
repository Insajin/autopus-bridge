// channel_test.go는 channel 서브커맨드 핸들러 함수를 테스트합니다.
package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunChannelList(t *testing.T) {
	// 테스트 채널 목록 (기본: group/all 채널)
	channels := []ChannelWithUnreadCount{
		{Channel: Channel{ID: "ch-1", Name: "general", Type: "group"}, MemberCount: 5, UnreadCount: 3},
		{Channel: Channel{ID: "ch-2", Name: "random", Type: "group"}, MemberCount: 10, UnreadCount: 0},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/channels" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(channels))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runChannelList(client, &buf, "", false)
	if err != nil {
		t.Fatalf("runChannelList 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "general") {
		t.Errorf("출력에 'general'이 없습니다: %s", out)
	}
	if !strings.Contains(out, "random") {
		t.Errorf("출력에 'random'이 없습니다: %s", out)
	}
}

func TestRunChannelListDM(t *testing.T) {
	// DM 채널 목록
	dmChannels := []DMChannel{
		{ID: "dm-1", Name: "Alice", Type: "dm"},
		{ID: "dm-2", Name: "Bob", Type: "dm"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/dm-channels" {
			http.Error(w, "not found: "+r.URL.Path, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(dmChannels))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runChannelList(client, &buf, "dm", false)
	if err != nil {
		t.Fatalf("runChannelList DM 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Alice") {
		t.Errorf("출력에 'Alice'가 없습니다: %s", out)
	}
}

func TestRunChannelListJSON(t *testing.T) {
	channels := []ChannelWithUnreadCount{
		{Channel: Channel{ID: "ch-1", Name: "general", Type: "group"}, MemberCount: 2, UnreadCount: 1},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(channels))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runChannelList(client, &buf, "", true)
	if err != nil {
		t.Fatalf("runChannelList JSON 오류: %v", err)
	}

	out := buf.String()
	var parsed []ChannelWithUnreadCount
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, out)
	}
	if len(parsed) != 1 || parsed[0].ID != "ch-1" {
		t.Errorf("예상치 않은 결과: %v", parsed)
	}
}

func TestRunChannelShow(t *testing.T) {
	ch := Channel{ID: "ch-1", Name: "general", Type: "group", Description: "General channel"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/channels/ch-1" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(ch))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runChannelShow(client, &buf, "ch-1", false)
	if err != nil {
		t.Fatalf("runChannelShow 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "general") {
		t.Errorf("출력에 'general'이 없습니다: %s", out)
	}
	if !strings.Contains(out, "General channel") {
		t.Errorf("출력에 'General channel'이 없습니다: %s", out)
	}
}

func TestRunChannelCreate(t *testing.T) {
	newChannel := Channel{ID: "ch-new", Name: "new-channel", Type: "group"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/channels" || r.Method != http.MethodPost {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(newChannel))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runChannelCreate(client, &buf, "new-channel", "", false)
	if err != nil {
		t.Fatalf("runChannelCreate 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "new-channel") {
		t.Errorf("출력에 'new-channel'이 없습니다: %s", out)
	}
}

func TestRunChannelDelete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/channels/ch-1" || r.Method != http.MethodDelete {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		// DELETE는 빈 데이터 또는 성공 메시지 반환
		w.Write(buildAPIResponse(map[string]string{"message": "deleted"}))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runChannelDelete(client, &buf, "ch-1")
	if err != nil {
		t.Fatalf("runChannelDelete 오류: %v", err)
	}
}

func TestRunChannelMembers(t *testing.T) {
	members := []ChannelMember{
		{ID: "u-1", Name: "Alice", Role: "admin"},
		{ID: "u-2", Name: "Bob", Role: "member"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/channels/ch-1/members" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(members))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runChannelMembers(client, &buf, "ch-1", false)
	if err != nil {
		t.Fatalf("runChannelMembers 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Alice") {
		t.Errorf("출력에 'Alice'가 없습니다: %s", out)
	}
	if !strings.Contains(out, "Bob") {
		t.Errorf("출력에 'Bob'이 없습니다: %s", out)
	}
}

func TestRunChannelConfig(t *testing.T) {
	config := ChannelConfig{
		ID:     "ch-1",
		Config: map[string]interface{}{"notifications": true, "retention_days": 30},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/channels/ch-1/config" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(config))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runChannelConfig(client, &buf, "ch-1")
	if err != nil {
		t.Fatalf("runChannelConfig 오류: %v", err)
	}

	out := buf.String()
	// config는 JSON으로 출력되어야 합니다
	if !strings.Contains(out, "notifications") {
		t.Errorf("출력에 'notifications'가 없습니다: %s", out)
	}
}

// TestRunChannelDeleteError는 runChannelDelete의 API 에러 경로를 테스트합니다.
func TestRunChannelDeleteError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 404 응답으로 에러 유발
		http.Error(w, `{"success":false,"error":"not found"}`, http.StatusNotFound)
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runChannelDelete(client, &buf, "ch-nonexistent")
	if err == nil {
		t.Fatal("에러가 예상되었으나 nil 반환")
	}
}

// TestRunChannelConfigError는 runChannelConfig의 API 에러 경로를 테스트합니다.
func TestRunChannelConfigError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 404 응답으로 에러 유발
		http.Error(w, `{"success":false,"error":"not found"}`, http.StatusNotFound)
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runChannelConfig(client, &buf, "ch-nonexistent")
	if err == nil {
		t.Fatal("에러가 예상되었으나 nil 반환")
	}
}

// TestRunChannelListDMJSON는 runChannelList DM 채널의 JSON 출력 경로를 테스트합니다.
func TestRunChannelListDMJSON(t *testing.T) {
	dmChannels := []DMChannel{
		{ID: "dm-1", Name: "Alice", Type: "dm"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(dmChannels))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runChannelList(client, &buf, "dm", true)
	if err != nil {
		t.Fatalf("runChannelList DM JSON 오류: %v", err)
	}

	// 유효한 JSON 배열 출력 확인
	var parsed []DMChannel
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if len(parsed) != 1 || parsed[0].ID != "dm-1" {
		t.Errorf("예상치 않은 결과: %v", parsed)
	}
}

// TestRunChannelShowJSON는 runChannelShow의 JSON 출력 경로를 테스트합니다.
func TestRunChannelShowJSON(t *testing.T) {
	ch := Channel{ID: "ch-1", Name: "general", Type: "group", Description: "General channel"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(ch))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runChannelShow(client, &buf, "ch-1", true)
	if err != nil {
		t.Fatalf("runChannelShow JSON 오류: %v", err)
	}

	// 유효한 JSON 출력 확인
	var parsed Channel
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if parsed.ID != "ch-1" {
		t.Errorf("예상치 않은 ID: %s", parsed.ID)
	}
}

// TestRunChannelCreateJSON는 runChannelCreate의 JSON 출력 경로를 테스트합니다.
func TestRunChannelCreateJSON(t *testing.T) {
	newChannel := Channel{ID: "ch-new", Name: "new-channel", Type: "group"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(newChannel))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runChannelCreate(client, &buf, "new-channel", "", true)
	if err != nil {
		t.Fatalf("runChannelCreate JSON 오류: %v", err)
	}

	// 유효한 JSON 출력 확인
	var parsed Channel
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if parsed.ID != "ch-new" {
		t.Errorf("예상치 않은 ID: %s", parsed.ID)
	}
}

// TestRunChannelShow_InvalidChannelID는 유효하지 않은 channelID에서 ValidateID 에러를 반환하는지 검증합니다.
func TestRunChannelShow_InvalidChannelID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runChannelShow(client, &buf, "bad/id", false)
	if err == nil {
		t.Error("유효하지 않은 channelID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

// TestRunChannelDelete_InvalidChannelID는 유효하지 않은 channelID에서 ValidateID 에러를 반환하는지 검증합니다.
func TestRunChannelDelete_InvalidChannelID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runChannelDelete(client, &buf, "bad id")
	if err == nil {
		t.Error("유효하지 않은 channelID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

// TestRunChannelMembers_InvalidChannelID는 유효하지 않은 channelID에서 ValidateID 에러를 반환하는지 검증합니다.
func TestRunChannelMembers_InvalidChannelID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runChannelMembers(client, &buf, "../bad", false)
	if err == nil {
		t.Error("유효하지 않은 channelID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

// TestRunChannelConfig_InvalidChannelID는 유효하지 않은 channelID에서 ValidateID 에러를 반환하는지 검증합니다.
func TestRunChannelConfig_InvalidChannelID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runChannelConfig(client, &buf, "id with space")
	if err == nil {
		t.Error("유효하지 않은 channelID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

// TestRunChannelMembersJSON는 runChannelMembers의 JSON 출력 경로를 테스트합니다.
func TestRunChannelMembersJSON(t *testing.T) {
	members := []ChannelMember{
		{ID: "u-1", Name: "Alice", Role: "admin"},
		{ID: "u-2", Name: "Bob", Role: "member"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(members))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runChannelMembers(client, &buf, "ch-1", true)
	if err != nil {
		t.Fatalf("runChannelMembers JSON 오류: %v", err)
	}

	// 유효한 JSON 배열 출력 확인
	var parsed []ChannelMember
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if len(parsed) != 2 || parsed[0].ID != "u-1" {
		t.Errorf("예상치 않은 결과: %v", parsed)
	}
}
