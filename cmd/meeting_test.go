// meeting_test.go는 meeting 서브커맨드 핸들러 함수를 테스트합니다.
package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestRunMeetingList는 미팅 목록 조회를 테스트합니다.
func TestRunMeetingList(t *testing.T) {
	meetings := []Meeting{
		{ID: "meet-1", Title: "팀 스탠드업", Status: "scheduled", ScheduledAt: "2026-03-10T09:00:00Z"},
		{ID: "meet-2", Title: "스프린트 리뷰", Status: "ended", ScheduledAt: "2026-03-09T14:00:00Z"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/meetings" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(meetings))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runMeetingList(client, &buf, "", 0, 0, false)
	if err != nil {
		t.Fatalf("runMeetingList 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "팀 스탠드업") {
		t.Errorf("출력에 '팀 스탠드업'이 없습니다: %s", out)
	}
	if !strings.Contains(out, "스프린트 리뷰") {
		t.Errorf("출력에 '스프린트 리뷰'가 없습니다: %s", out)
	}
}

// TestRunMeetingListJSON는 미팅 목록 JSON 출력을 테스트합니다.
func TestRunMeetingListJSON(t *testing.T) {
	meetings := []Meeting{
		{ID: "meet-1", Title: "팀 스탠드업", Status: "scheduled"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(meetings))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runMeetingList(client, &buf, "", 0, 0, true)
	if err != nil {
		t.Fatalf("runMeetingList JSON 오류: %v", err)
	}

	var parsed []Meeting
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if len(parsed) != 1 || parsed[0].ID != "meet-1" {
		t.Errorf("예상치 않은 결과: %v", parsed)
	}
}

// TestRunMeetingListWithStatus는 상태 필터로 목록 조회를 테스트합니다.
func TestRunMeetingListWithStatus(t *testing.T) {
	meetings := []Meeting{
		{ID: "meet-1", Title: "팀 스탠드업", Status: "scheduled"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// status 쿼리 파라미터 확인
		if r.URL.Query().Get("status") != "scheduled" {
			http.Error(w, "bad status param", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(meetings))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runMeetingList(client, &buf, "scheduled", 0, 0, false)
	if err != nil {
		t.Fatalf("runMeetingList with status 오류: %v", err)
	}
}

// TestRunMeetingListWithPagination는 페이지네이션 파라미터 전달을 테스트합니다.
func TestRunMeetingListWithPagination(t *testing.T) {
	meetings := []Meeting{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("page") != "2" || q.Get("per_page") != "5" {
			http.Error(w, "bad pagination params", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(meetings))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runMeetingList(client, &buf, "", 2, 5, false)
	if err != nil {
		t.Fatalf("runMeetingList with pagination 오류: %v", err)
	}
}

// TestRunMeetingShow는 미팅 상세 조회를 테스트합니다.
func TestRunMeetingShow(t *testing.T) {
	meeting := Meeting{
		ID:     "meet-1",
		Title:  "팀 스탠드업",
		Agenda: "진행 상황 공유",
		Status: "scheduled",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/meetings/meet-1" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(meeting))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runMeetingShow(client, &buf, "meet-1", false)
	if err != nil {
		t.Fatalf("runMeetingShow 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "팀 스탠드업") {
		t.Errorf("출력에 '팀 스탠드업'이 없습니다: %s", out)
	}
	if !strings.Contains(out, "진행 상황 공유") {
		t.Errorf("출력에 '진행 상황 공유'가 없습니다: %s", out)
	}
}

// TestRunMeetingShowJSON는 미팅 상세 JSON 출력을 테스트합니다.
func TestRunMeetingShowJSON(t *testing.T) {
	meeting := Meeting{ID: "meet-1", Title: "팀 스탠드업"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(meeting))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runMeetingShow(client, &buf, "meet-1", true)
	if err != nil {
		t.Fatalf("runMeetingShow JSON 오류: %v", err)
	}

	var parsed Meeting
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if parsed.ID != "meet-1" {
		t.Errorf("예상치 않은 ID: %s", parsed.ID)
	}
}

// TestRunMeetingShow_InvalidID는 유효하지 않은 ID에서 에러를 반환하는지 테스트합니다.
func TestRunMeetingShow_InvalidID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runMeetingShow(client, &buf, "bad/id", false)
	if err == nil {
		t.Error("유효하지 않은 ID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

// TestRunMeetingCreate는 미팅 생성을 테스트합니다.
func TestRunMeetingCreate(t *testing.T) {
	newMeeting := Meeting{ID: "meet-new", Title: "신규 미팅", Status: "scheduled"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/meetings" || r.Method != http.MethodPost {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(newMeeting))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runMeetingCreate(client, &buf, "신규 미팅", "", "ch-1", "agent-1,agent-2", "", false)
	if err != nil {
		t.Fatalf("runMeetingCreate 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "신규 미팅") {
		t.Errorf("출력에 '신규 미팅'이 없습니다: %s", out)
	}
}

// TestRunMeetingCreateJSON는 미팅 생성 JSON 출력을 테스트합니다.
func TestRunMeetingCreateJSON(t *testing.T) {
	newMeeting := Meeting{ID: "meet-new", Title: "신규 미팅"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(newMeeting))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runMeetingCreate(client, &buf, "신규 미팅", "", "", "", "", true)
	if err != nil {
		t.Fatalf("runMeetingCreate JSON 오류: %v", err)
	}

	var parsed Meeting
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if parsed.ID != "meet-new" {
		t.Errorf("예상치 않은 ID: %s", parsed.ID)
	}
}

// TestRunMeetingStart는 미팅 시작을 테스트합니다.
func TestRunMeetingStart(t *testing.T) {
	meeting := Meeting{ID: "meet-1", Title: "팀 스탠드업", Status: "in_progress"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/meetings/meet-1/start" || r.Method != http.MethodPatch {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(meeting))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runMeetingStart(client, &buf, "meet-1")
	if err != nil {
		t.Fatalf("runMeetingStart 오류: %v", err)
	}
}

// TestRunMeetingStart_InvalidID는 유효하지 않은 ID에서 에러를 반환하는지 테스트합니다.
func TestRunMeetingStart_InvalidID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runMeetingStart(client, &buf, "bad id")
	if err == nil {
		t.Error("유효하지 않은 ID에서 에러가 발생해야 합니다")
	}
}

// TestRunMeetingEnd는 미팅 종료를 테스트합니다.
func TestRunMeetingEnd(t *testing.T) {
	meeting := Meeting{ID: "meet-1", Title: "팀 스탠드업", Status: "ended"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/meetings/meet-1/end" || r.Method != http.MethodPatch {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(meeting))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runMeetingEnd(client, &buf, "meet-1")
	if err != nil {
		t.Fatalf("runMeetingEnd 오류: %v", err)
	}
}

// TestRunMeetingEnd_InvalidID는 유효하지 않은 ID에서 에러를 반환하는지 테스트합니다.
func TestRunMeetingEnd_InvalidID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runMeetingEnd(client, &buf, "bad/id")
	if err == nil {
		t.Error("유효하지 않은 ID에서 에러가 발생해야 합니다")
	}
}

// TestRunMeetingCancel는 미팅 취소를 테스트합니다.
func TestRunMeetingCancel(t *testing.T) {
	meeting := Meeting{ID: "meet-1", Title: "팀 스탠드업", Status: "cancelled"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/meetings/meet-1/cancel" || r.Method != http.MethodPatch {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(meeting))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runMeetingCancel(client, &buf, "meet-1")
	if err != nil {
		t.Fatalf("runMeetingCancel 오류: %v", err)
	}
}

// TestRunMeetingCancel_InvalidID는 유효하지 않은 ID에서 에러를 반환하는지 테스트합니다.
func TestRunMeetingCancel_InvalidID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runMeetingCancel(client, &buf, "../bad")
	if err == nil {
		t.Error("유효하지 않은 ID에서 에러가 발생해야 합니다")
	}
}

// TestRunMeetingMessages는 미팅 메시지 목록 조회를 테스트합니다.
func TestRunMeetingMessages(t *testing.T) {
	messages := []MeetingMessage{
		{ID: "msg-1", MeetingID: "meet-1", Sender: "agent-1", Content: "안녕하세요"},
		{ID: "msg-2", MeetingID: "meet-1", Sender: "agent-2", Content: "반갑습니다"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/meetings/meet-1/messages" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(messages))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runMeetingMessages(client, &buf, "meet-1", false)
	if err != nil {
		t.Fatalf("runMeetingMessages 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "안녕하세요") {
		t.Errorf("출력에 '안녕하세요'가 없습니다: %s", out)
	}
	if !strings.Contains(out, "반갑습니다") {
		t.Errorf("출력에 '반갑습니다'가 없습니다: %s", out)
	}
}

// TestRunMeetingMessagesJSON는 미팅 메시지 JSON 출력을 테스트합니다.
func TestRunMeetingMessagesJSON(t *testing.T) {
	messages := []MeetingMessage{
		{ID: "msg-1", MeetingID: "meet-1", Sender: "agent-1", Content: "안녕하세요"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(messages))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runMeetingMessages(client, &buf, "meet-1", true)
	if err != nil {
		t.Fatalf("runMeetingMessages JSON 오류: %v", err)
	}

	var parsed []MeetingMessage
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if len(parsed) != 1 || parsed[0].ID != "msg-1" {
		t.Errorf("예상치 않은 결과: %v", parsed)
	}
}

// TestRunMeetingMessages_InvalidID는 유효하지 않은 ID에서 에러를 반환하는지 테스트합니다.
func TestRunMeetingMessages_InvalidID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runMeetingMessages(client, &buf, "bad/id", false)
	if err == nil {
		t.Error("유효하지 않은 ID에서 에러가 발생해야 합니다")
	}
}

// TestRunMeetingRegenerateMinutes는 회의록 재생성을 테스트합니다.
func TestRunMeetingRegenerateMinutes(t *testing.T) {
	meeting := Meeting{ID: "meet-1", Title: "팀 스탠드업", Status: "ended"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/meetings/meet-1/regenerate-minutes" || r.Method != http.MethodPost {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(meeting))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runMeetingRegenerateMinutes(client, &buf, "meet-1")
	if err != nil {
		t.Fatalf("runMeetingRegenerateMinutes 오류: %v", err)
	}
}

// TestRunMeetingRegenerateMinutes_InvalidID는 유효하지 않은 ID에서 에러를 반환하는지 테스트합니다.
func TestRunMeetingRegenerateMinutes_InvalidID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runMeetingRegenerateMinutes(client, &buf, "bad id")
	if err == nil {
		t.Error("유효하지 않은 ID에서 에러가 발생해야 합니다")
	}
}

// TestRunMeetingScheduleCreate는 미팅 스케줄 생성을 테스트합니다.
func TestRunMeetingScheduleCreate(t *testing.T) {
	schedule := MeetingSchedule{
		ID:             "sched-1",
		Title:          "주간 스탠드업",
		CronExpression: "0 9 * * 1",
		RecurrenceType: "weekly",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/meeting-schedules" || r.Method != http.MethodPost {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(schedule))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runMeetingScheduleCreate(client, &buf, "주간 스탠드업", "", "0 9 * * 1", "agent-1,agent-2", "weekly", false)
	if err != nil {
		t.Fatalf("runMeetingScheduleCreate 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "주간 스탠드업") {
		t.Errorf("출력에 '주간 스탠드업'이 없습니다: %s", out)
	}
}

// TestRunMeetingScheduleCreateJSON는 스케줄 생성 JSON 출력을 테스트합니다.
func TestRunMeetingScheduleCreateJSON(t *testing.T) {
	schedule := MeetingSchedule{ID: "sched-1", Title: "주간 스탠드업"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(schedule))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runMeetingScheduleCreate(client, &buf, "주간 스탠드업", "", "0 9 * * 1", "", "weekly", true)
	if err != nil {
		t.Fatalf("runMeetingScheduleCreate JSON 오류: %v", err)
	}

	var parsed MeetingSchedule
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if parsed.ID != "sched-1" {
		t.Errorf("예상치 않은 ID: %s", parsed.ID)
	}
}

// TestRunMeetingSchedules는 미팅 스케줄 목록 조회를 테스트합니다.
func TestRunMeetingSchedules(t *testing.T) {
	schedules := []MeetingSchedule{
		{ID: "sched-1", Title: "주간 스탠드업", CronExpression: "0 9 * * 1", RecurrenceType: "weekly"},
		{ID: "sched-2", Title: "월간 회의", CronExpression: "0 10 1 * *", RecurrenceType: "monthly"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/meeting-schedules" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(schedules))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runMeetingSchedules(client, &buf, false)
	if err != nil {
		t.Fatalf("runMeetingSchedules 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "주간 스탠드업") {
		t.Errorf("출력에 '주간 스탠드업'이 없습니다: %s", out)
	}
	if !strings.Contains(out, "월간 회의") {
		t.Errorf("출력에 '월간 회의'가 없습니다: %s", out)
	}
}

// TestRunMeetingSchedulesJSON는 스케줄 목록 JSON 출력을 테스트합니다.
func TestRunMeetingSchedulesJSON(t *testing.T) {
	schedules := []MeetingSchedule{
		{ID: "sched-1", Title: "주간 스탠드업"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(schedules))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runMeetingSchedules(client, &buf, true)
	if err != nil {
		t.Fatalf("runMeetingSchedules JSON 오류: %v", err)
	}

	var parsed []MeetingSchedule
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if len(parsed) != 1 || parsed[0].ID != "sched-1" {
		t.Errorf("예상치 않은 결과: %v", parsed)
	}
}

// TAG-001 RED: SuccessPage 포맷(백엔드 변경 후) 응답에서 미팅 목록을 파싱하는 테스트.
// 백엔드가 SuccessPage로 변경되면 {success:true, data:[...], total, limit, offset} 형태로 응답한다.
// CLI의 DoList[Meeting]은 data 배열만 읽으므로 이 형태에서도 동작해야 한다.
func TestRunMeetingList_SuccessPageFormat(t *testing.T) {
	meetings := []Meeting{
		{ID: "meet-1", Title: "팀 스탠드업", Status: "scheduled", ScheduledAt: "2026-03-10T09:00:00Z"},
		{ID: "meet-2", Title: "스프린트 리뷰", Status: "ended", ScheduledAt: "2026-03-09T14:00:00Z"},
	}

	// SuccessPage 포맷 응답 시뮬레이션: data가 배열, total/limit/offset이 최상위 필드
	buildSuccessPageResponse := func(data interface{}, total int64, limit, offset int) []byte {
		payload, _ := json.Marshal(data)
		resp := map[string]interface{}{
			"success": true,
			"data":    json.RawMessage(payload),
			"total":   total,
			"limit":   limit,
			"offset":  offset,
		}
		b, _ := json.Marshal(resp)
		return b
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/meetings" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildSuccessPageResponse(meetings, 2, 20, 0))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runMeetingList(client, &buf, "", 0, 0, false)
	if err != nil {
		t.Fatalf("runMeetingList SuccessPage 포맷 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "팀 스탠드업") {
		t.Errorf("출력에 '팀 스탠드업'이 없습니다: %s", out)
	}
	if !strings.Contains(out, "스프린트 리뷰") {
		t.Errorf("출력에 '스프린트 리뷰'가 없습니다: %s", out)
	}
}

// TestRunMeetingList_OldWrappedFormat은 현재 백엔드 응답(data가 객체)이 CLI 파싱 실패를 유발함을 검증한다.
// REQ-CC-001: 현재 형태가 문제임을 재현하는 특성화 테스트
func TestRunMeetingList_OldWrappedFormat(t *testing.T) {
	meetings := []Meeting{
		{ID: "meet-1", Title: "팀 스탠드업"},
	}

	// 현재 백엔드 형태: data가 객체
	buildOldResponse := func(data []Meeting, total, page, perPage int) []byte {
		resp := map[string]interface{}{
			"success": true,
			"data": map[string]interface{}{
				"meetings": data,
				"total":    total,
				"page":     page,
				"per_page": perPage,
			},
		}
		b, _ := json.Marshal(resp)
		return b
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildOldResponse(meetings, 1, 1, 20))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	// 현재 코드: DoList[Meeting]은 data를 배열로 파싱 시도 → 실패해야 함
	err := runMeetingList(client, &buf, "", 0, 0, false)
	if err == nil {
		t.Error("구형 래핑 포맷에서는 파싱 오류가 발생해야 합니다 (data가 배열이 아닌 객체)")
	}
}

// TestRunMeetingListError는 API 에러 경로를 테스트합니다.
func TestRunMeetingListError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"success":false,"error":"internal error"}`, http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runMeetingList(client, &buf, "", 0, 0, false)
	if err == nil {
		t.Fatal("에러가 예상되었으나 nil 반환")
	}
}

// TestRunMeetingShowError는 미팅 조회 API 에러 경로를 테스트합니다.
func TestRunMeetingShowError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"success":false,"error":"not found"}`, http.StatusNotFound)
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runMeetingShow(client, &buf, "meet-nonexistent", false)
	if err == nil {
		t.Fatal("에러가 예상되었으나 nil 반환")
	}
}
