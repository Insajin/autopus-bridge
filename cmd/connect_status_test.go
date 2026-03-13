package cmd

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/insajin/autopus-agent-protocol"
)

type stubTaskSender struct {
	progress []ws.TaskProgressPayload
	results  []ws.TaskResultPayload
	errors   []ws.TaskErrorPayload
	lastExec string
}

func (s *stubTaskSender) SendTaskProgress(payload ws.TaskProgressPayload) error {
	s.progress = append(s.progress, payload)
	return nil
}

func (s *stubTaskSender) SendTaskResult(payload ws.TaskResultPayload) error {
	s.results = append(s.results, payload)
	return nil
}

func (s *stubTaskSender) SendTaskError(payload ws.TaskErrorPayload) error {
	s.errors = append(s.errors, payload)
	return nil
}

func (s *stubTaskSender) SetLastExecID(execID string) {
	s.lastExec = execID
}

func TestStatusTrackingTaskSender_TracksTaskCompletion(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	base := &stubTaskSender{}
	connState := NewConnectionState()
	connState.SetConnected(true)
	connState.SetWorkspaceID("ws-1")

	sender := &statusTrackingTaskSender{
		sender:    base,
		connState: connState,
	}

	if err := sender.SendTaskProgress(ws.TaskProgressPayload{
		ExecutionID: "exec-1",
		Progress:    0,
	}); err != nil {
		t.Fatalf("SendTaskProgress() error = %v", err)
	}
	if connState.CurrentTaskID() != "exec-1" {
		t.Fatalf("CurrentTaskID() = %q, want %q", connState.CurrentTaskID(), "exec-1")
	}

	if err := sender.SendTaskResult(ws.TaskResultPayload{
		ExecutionID: "exec-1",
		Output:      "done",
	}); err != nil {
		t.Fatalf("SendTaskResult() error = %v", err)
	}

	data, err := os.ReadFile(getScopedStatusFilePath("ws-1"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	var got StatusInfo
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if got.TasksCompleted != 1 {
		t.Fatalf("TasksCompleted = %d, want %d", got.TasksCompleted, 1)
	}
	if got.TasksFailed != 0 {
		t.Fatalf("TasksFailed = %d, want %d", got.TasksFailed, 0)
	}
	if got.CurrentTask != "" {
		t.Fatalf("CurrentTask = %q, want empty", got.CurrentTask)
	}
}

func TestStatusTrackingTaskSender_TracksTaskFailure(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	base := &stubTaskSender{}
	connState := NewConnectionState()
	connState.SetConnected(true)
	connState.SetWorkspaceID("ws-1")
	connState.SetCurrentTaskID("exec-2")

	sender := &statusTrackingTaskSender{
		sender:    base,
		connState: connState,
	}

	if err := sender.SendTaskError(ws.TaskErrorPayload{
		ExecutionID: "exec-2",
		Code:        "INTERNAL_ERROR",
		Message:     "boom",
	}); err != nil {
		t.Fatalf("SendTaskError() error = %v", err)
	}

	data, err := os.ReadFile(getScopedStatusFilePath("ws-1"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	var got StatusInfo
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if got.TasksCompleted != 0 {
		t.Fatalf("TasksCompleted = %d, want %d", got.TasksCompleted, 0)
	}
	if got.TasksFailed != 1 {
		t.Fatalf("TasksFailed = %d, want %d", got.TasksFailed, 1)
	}
	if got.CurrentTask != "" {
		t.Fatalf("CurrentTask = %q, want empty", got.CurrentTask)
	}
}
