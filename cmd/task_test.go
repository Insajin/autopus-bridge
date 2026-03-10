// task_test.go는 task 서브커맨드 핸들러 함수를 테스트합니다.
package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunTaskList(t *testing.T) {
	// 테스트 에이전트 태스크 목록
	tasks := []AgentTask{
		{ID: "task-1", Title: "Fix bug", TaskType: "code", Priority: 1, Status: "in_progress", AgentName: "Alice"},
		{ID: "task-2", Title: "Write tests", TaskType: "test", Priority: 2, Status: "pending", AgentName: "Bob"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/agent-tasks" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(tasks))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runTaskList(client, &buf, "", "", 0, false)
	if err != nil {
		t.Fatalf("runTaskList 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Fix bug") {
		t.Errorf("출력에 'Fix bug'가 없습니다: %s", out)
	}
	if !strings.Contains(out, "Write tests") {
		t.Errorf("출력에 'Write tests'가 없습니다: %s", out)
	}
}

func TestRunTaskListJSON(t *testing.T) {
	tasks := []AgentTask{
		{ID: "task-1", Title: "Fix bug", TaskType: "code", Status: "pending"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(tasks))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runTaskList(client, &buf, "", "", 0, true)
	if err != nil {
		t.Fatalf("runTaskList JSON 오류: %v", err)
	}

	var parsed []AgentTask
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if len(parsed) != 1 || parsed[0].ID != "task-1" {
		t.Errorf("예상치 않은 결과: %v", parsed)
	}
}

func TestRunTaskListWithFilters(t *testing.T) {
	// 필터 파라미터가 쿼리 스트링에 포함되는지 확인
	tasks := []AgentTask{
		{ID: "task-1", Title: "Fix bug", Status: "pending"},
	}

	var receivedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.RequestURI()
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(tasks))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runTaskList(client, &buf, "pending", "code", 1, false)
	if err != nil {
		t.Fatalf("runTaskList 필터 오류: %v", err)
	}

	if !strings.Contains(receivedPath, "status=pending") {
		t.Errorf("쿼리 스트링에 'status=pending'이 없습니다: %s", receivedPath)
	}
	if !strings.Contains(receivedPath, "type=code") {
		t.Errorf("쿼리 스트링에 'type=code'가 없습니다: %s", receivedPath)
	}
}

func TestRunTaskShow(t *testing.T) {
	task := AgentTask{ID: "task-1", Title: "Fix bug", TaskType: "code", Priority: 1, Status: "in_progress", AgentName: "Alice"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/agent-tasks/task-1" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(task))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runTaskShow(client, &buf, "task-1", false)
	if err != nil {
		t.Fatalf("runTaskShow 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Fix bug") {
		t.Errorf("출력에 'Fix bug'가 없습니다: %s", out)
	}
	if !strings.Contains(out, "Alice") {
		t.Errorf("출력에 'Alice'가 없습니다: %s", out)
	}
}

func TestRunTaskShowJSON(t *testing.T) {
	task := AgentTask{ID: "task-1", Title: "Fix bug", Status: "pending"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(task))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	client.SetJSONOutput(true)
	var buf bytes.Buffer

	err := runTaskShow(client, &buf, "task-1", true)
	if err != nil {
		t.Fatalf("runTaskShow JSON 오류: %v", err)
	}

	var parsed AgentTask
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &parsed); err != nil {
		t.Fatalf("JSON 파싱 실패: %v, 출력: %s", err, buf.String())
	}
	if parsed.ID != "task-1" {
		t.Errorf("예상치 않은 ID: %s", parsed.ID)
	}
}

func TestRunTaskCreate(t *testing.T) {
	newTask := AgentTask{ID: "task-new", Title: "New task", TaskType: "code", Priority: 1}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/agent-tasks" || r.Method != http.MethodPost {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(newTask))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runTaskCreate(client, &buf, "New task", "code", 1, "", "", false)
	if err != nil {
		t.Fatalf("runTaskCreate 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "New task") {
		t.Errorf("출력에 'New task'가 없습니다: %s", out)
	}
}

func TestRunTaskAssign(t *testing.T) {
	task := AgentTask{ID: "task-1", Title: "Fix bug", AgentID: "agent-1"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/agent-tasks/task-1/assign" || r.Method != http.MethodPatch {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(task))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runTaskAssign(client, &buf, "task-1", "agent-1", false)
	if err != nil {
		t.Fatalf("runTaskAssign 오류: %v", err)
	}
}

func TestRunTaskStart(t *testing.T) {
	task := AgentTask{ID: "task-1", Title: "Fix bug", Status: "in_progress"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/agent-tasks/task-1/start" || r.Method != http.MethodPost {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(task))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runTaskStart(client, &buf, "task-1")
	if err != nil {
		t.Fatalf("runTaskStart 오류: %v", err)
	}
}

func TestRunTaskComplete(t *testing.T) {
	task := AgentTask{ID: "task-1", Title: "Fix bug", Status: "completed"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/agent-tasks/task-1/complete" || r.Method != http.MethodPost {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(task))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runTaskComplete(client, &buf, "task-1", "")
	if err != nil {
		t.Fatalf("runTaskComplete 오류: %v", err)
	}
}

func TestRunTaskFail(t *testing.T) {
	task := AgentTask{ID: "task-1", Title: "Fix bug", Status: "failed"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/agent-tasks/task-1/fail" || r.Method != http.MethodPost {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(task))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runTaskFail(client, &buf, "task-1", "네트워크 오류")
	if err != nil {
		t.Fatalf("runTaskFail 오류: %v", err)
	}
}

func TestRunTaskCancel(t *testing.T) {
	task := AgentTask{ID: "task-1", Title: "Fix bug", Status: "cancelled"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/agent-tasks/task-1/cancel" || r.Method != http.MethodPost {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(task))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runTaskCancel(client, &buf, "task-1")
	if err != nil {
		t.Fatalf("runTaskCancel 오류: %v", err)
	}
}

func TestRunTaskStats(t *testing.T) {
	stats := TaskQueueStats{
		Total:    10,
		ByStatus: map[string]int{"pending": 5, "in_progress": 3, "completed": 2},
		AvgTime:  "5m30s",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/workspaces/ws-1/agent-tasks/queue-stats" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildAPIResponse(stats))
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runTaskStats(client, &buf)
	if err != nil {
		t.Fatalf("runTaskStats 오류: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "10") {
		t.Errorf("출력에 '10'이 없습니다: %s", out)
	}
}

// TestRunTaskShow_InvalidID는 유효하지 않은 taskID에서 ValidateID 에러를 반환하는지 검증합니다.
func TestRunTaskShow_InvalidID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runTaskShow(client, &buf, "bad/id", false)
	if err == nil {
		t.Error("유효하지 않은 taskID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

// TestRunTaskStart_InvalidID는 유효하지 않은 taskID에서 ValidateID 에러를 반환하는지 검증합니다.
func TestRunTaskStart_InvalidID(t *testing.T) {
	client := makeTestClient("http://localhost:9999", "ws-1")
	var buf bytes.Buffer

	err := runTaskStart(client, &buf, "bad id")
	if err == nil {
		t.Error("유효하지 않은 taskID에서 에러가 발생해야 합니다")
	} else if !strings.Contains(err.Error(), "유효하지 않은 ID") {
		t.Errorf("에러에 '유효하지 않은 ID'가 없습니다: %v", err)
	}
}

// TestRunTaskListError는 runTaskList의 API 에러 경로를 테스트합니다.
func TestRunTaskListError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"success":false,"error":"unauthorized"}`, http.StatusUnauthorized)
	}))
	defer srv.Close()

	client := makeTestClient(srv.URL, "ws-1")
	var buf bytes.Buffer

	err := runTaskList(client, &buf, "", "", 0, false)
	if err == nil {
		t.Fatal("에러가 예상되었으나 nil 반환")
	}
	if !strings.Contains(err.Error(), "태스크 목록 조회 실패") {
		t.Errorf("에러 메시지에 '태스크 목록 조회 실패'가 없습니다: %v", err)
	}
}
