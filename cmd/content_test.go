// content_test.go는 content 서브커맨드 핸들러 함수를 테스트합니다.
package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunContentList(t *testing.T) {
	items := []ContentItem{
		{ID: "c-1", Title: "첫 번째 콘텐츠", ContentType: "blog", Platform: "medium", Status: "draft"},
		{ID: "c-2", Title: "두 번째 콘텐츠", ContentType: "video", Platform: "youtube", Status: "published"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/content-calendar" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(items))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runContentList(client, &buf, 0, 0, false)
	if err != nil {
		t.Fatalf("runContentList 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "첫 번째 콘텐츠") {
		t.Errorf("출력에 '첫 번째 콘텐츠'가 없습니다: %s", out)
	}
	if !strings.Contains(out, "두 번째 콘텐츠") {
		t.Errorf("출력에 '두 번째 콘텐츠'가 없습니다: %s", out)
	}
}

func TestRunContentListJSON(t *testing.T) {
	items := []ContentItem{
		{ID: "c-1", Title: "테스트 콘텐츠", ContentType: "blog", Platform: "medium", Status: "draft"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(items))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runContentList(client, &buf, 0, 0, true)
	if err != nil {
		t.Fatalf("runContentList JSON 오류: %v", err)
	}

	var parsed []ContentItem
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if len(parsed) != 1 || parsed[0].ID != "c-1" {
		t.Errorf("예상치 않은 결과: %v", parsed)
	}
}

func TestRunContentShow(t *testing.T) {
	item := ContentItem{ID: "c-1", Title: "테스트 콘텐츠", ContentType: "blog", Platform: "medium", Status: "draft"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/content-calendar/c-1" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(item))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runContentShow(client, &buf, "c-1", false)
	if err != nil {
		t.Fatalf("runContentShow 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "테스트 콘텐츠") {
		t.Errorf("출력에 '테스트 콘텐츠'가 없습니다: %s", out)
	}
	if !strings.Contains(out, "blog") {
		t.Errorf("출력에 'blog'가 없습니다: %s", out)
	}
}

func TestRunContentShowJSON(t *testing.T) {
	item := ContentItem{ID: "c-1", Title: "테스트 콘텐츠", ContentType: "blog", Platform: "medium", Status: "draft"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(item))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runContentShow(client, &buf, "c-1", true)
	if err != nil {
		t.Fatalf("runContentShow JSON 오류: %v", err)
	}

	var parsed ContentItem
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if parsed.ID != "c-1" {
		t.Errorf("예상치 않은 ID: %s", parsed.ID)
	}
}

func TestRunContentCreate(t *testing.T) {
	newItem := ContentItem{ID: "c-new", Title: "새 콘텐츠", ContentType: "blog", Platform: "medium", Status: "draft"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/content-calendar" || r.Method != http.MethodPost {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(newItem))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runContentCreate(client, &buf, "새 콘텐츠", "blog", "medium", "콘텐츠 본문", "agent-1", "goal-1", false)
	if err != nil {
		t.Fatalf("runContentCreate 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "새 콘텐츠") {
		t.Errorf("출력에 '새 콘텐츠'가 없습니다: %s", out)
	}
}

func TestRunContentUpdate(t *testing.T) {
	updatedItem := ContentItem{ID: "c-1", Title: "수정된 콘텐츠", ContentType: "blog", Platform: "medium", Status: "draft"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/content-calendar/c-1" || r.Method != http.MethodPut {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(updatedItem))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runContentUpdate(client, &buf, "c-1", "수정된 콘텐츠", "", "", "", "", "", false)
	if err != nil {
		t.Fatalf("runContentUpdate 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "수정된 콘텐츠") {
		t.Errorf("출력에 '수정된 콘텐츠'가 없습니다: %s", out)
	}
}

func TestRunContentDelete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/content-calendar/c-1" || r.Method != http.MethodDelete {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(map[string]string{"message": "deleted"}))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runContentDelete(client, &buf, "c-1")
	if err != nil {
		t.Fatalf("runContentDelete 오류: %v", err)
	}
}

func TestRunContentSchedule(t *testing.T) {
	scheduledItem := ContentItem{ID: "c-1", Title: "예약된 콘텐츠", Status: "scheduled"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/content-calendar/c-1/schedule" || r.Method != http.MethodPost {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(scheduledItem))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runContentSchedule(client, &buf, "c-1", false)
	if err != nil {
		t.Fatalf("runContentSchedule 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "예약된 콘텐츠") {
		t.Errorf("출력에 '예약된 콘텐츠'가 없습니다: %s", out)
	}
}

func TestRunContentApprove(t *testing.T) {
	approvedItem := ContentItem{ID: "c-1", Title: "승인된 콘텐츠", Status: "approved"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/content-calendar/c-1/approve" || r.Method != http.MethodPost {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(approvedItem))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runContentApprove(client, &buf, "c-1", false)
	if err != nil {
		t.Fatalf("runContentApprove 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "승인된 콘텐츠") {
		t.Errorf("출력에 '승인된 콘텐츠'가 없습니다: %s", out)
	}
}

func TestRunContentShow_InvalidID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runContentShow(client, &buf, "bad/id", false)
	if err == nil {
		t.Error("유효하지 않은 ID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

func TestRunContentDelete_InvalidID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runContentDelete(client, &buf, "bad id")
	if err == nil {
		t.Error("유효하지 않은 ID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

func TestRunContentSchedule_InvalidID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runContentSchedule(client, &buf, "../bad", false)
	if err == nil {
		t.Error("유효하지 않은 ID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

func TestRunContentApprove_InvalidID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runContentApprove(client, &buf, "id with space", false)
	if err == nil {
		t.Error("유효하지 않은 ID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

func TestRunContentDeleteError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"success":false,"error":"not found"}`, http.StatusNotFound)
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runContentDelete(client, &buf, "c-nonexistent")
	if err == nil {
		t.Fatal("에러가 예상되었으나 nil 반환")
	}
}
