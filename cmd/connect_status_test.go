package cmd

import (
	"encoding/json"
	"os"
	"testing"

	ws "github.com/insajin/autopus-agent-protocol"
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

// ---------------------------------------------------------------------------
// Tests: AI OAuth status in StatusInfo (SPEC-DOMAIN-PARALLEL-001 AC-9)
// ---------------------------------------------------------------------------

func TestUpdateStatusOAuthMode_Connected(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	connState := NewConnectionState()
	connState.SetConnected(true)
	connState.SetWorkspaceID("ws-oauth")

	// 초기 상태 파일 생성
	initial := &StatusInfo{Connected: true, WorkspaceID: "ws-oauth", PID: os.Getpid()}
	if err := SaveStatus(initial); err != nil {
		t.Fatalf("SaveStatus() error = %v", err)
	}

	payload := ws.AIOAuthStatusChangePayload{
		Provider: "openai",
		Status:   "connected",
		AIMode:   "oauth",
	}
	updateStatusOAuthMode(connState, payload)

	data, err := os.ReadFile(getScopedStatusFilePath("ws-oauth"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	var got StatusInfo
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if got.OAuthMode != "oauth" {
		t.Errorf("OAuthMode = %q, want %q", got.OAuthMode, "oauth")
	}
	if len(got.OAuthProviders) != 1 || got.OAuthProviders[0] != "openai" {
		t.Errorf("OAuthProviders = %v, want [openai]", got.OAuthProviders)
	}
}

func TestUpdateStatusOAuthMode_ConnectedTwice_NoDuplicate(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	connState := NewConnectionState()
	connState.SetWorkspaceID("ws-dedup")

	initial := &StatusInfo{Connected: true, WorkspaceID: "ws-dedup", PID: os.Getpid()}
	_ = SaveStatus(initial)

	payload := ws.AIOAuthStatusChangePayload{Provider: "google", Status: "connected", AIMode: "oauth"}
	updateStatusOAuthMode(connState, payload)
	updateStatusOAuthMode(connState, payload) // 두 번째 호출 — 중복 없어야 함

	data, _ := os.ReadFile(getScopedStatusFilePath("ws-dedup"))
	var got StatusInfo
	_ = json.Unmarshal(data, &got)

	if len(got.OAuthProviders) != 1 {
		t.Errorf("OAuthProviders len = %d, want 1 (no duplicate)", len(got.OAuthProviders))
	}
}

func TestUpdateStatusOAuthMode_Disconnected_RemovesProvider(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	connState := NewConnectionState()
	connState.SetWorkspaceID("ws-disc")

	initial := &StatusInfo{
		Connected:      true,
		WorkspaceID:    "ws-disc",
		OAuthMode:      "oauth",
		OAuthProviders: []string{"openai", "google"},
		PID:            os.Getpid(),
	}
	_ = SaveStatus(initial)

	payload := ws.AIOAuthStatusChangePayload{Provider: "openai", Status: "disconnected", AIMode: "bridge"}
	updateStatusOAuthMode(connState, payload)

	data, _ := os.ReadFile(getScopedStatusFilePath("ws-disc"))
	var got StatusInfo
	_ = json.Unmarshal(data, &got)

	if len(got.OAuthProviders) != 1 || got.OAuthProviders[0] != "google" {
		t.Errorf("OAuthProviders = %v, want [google]", got.OAuthProviders)
	}
	// google 아직 있으므로 mode 유지
	if got.OAuthMode == "" {
		t.Errorf("OAuthMode should not be cleared when other providers remain")
	}
}

func TestUpdateStatusOAuthMode_AllDisconnected_ClearsMode(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	connState := NewConnectionState()
	connState.SetWorkspaceID("ws-all-disc")

	initial := &StatusInfo{
		Connected:      true,
		WorkspaceID:    "ws-all-disc",
		OAuthMode:      "oauth",
		OAuthProviders: []string{"openai"},
		PID:            os.Getpid(),
	}
	_ = SaveStatus(initial)

	payload := ws.AIOAuthStatusChangePayload{Provider: "openai", Status: "expired", AIMode: "bridge"}
	updateStatusOAuthMode(connState, payload)

	data, _ := os.ReadFile(getScopedStatusFilePath("ws-all-disc"))
	var got StatusInfo
	_ = json.Unmarshal(data, &got)

	if got.OAuthMode != "" {
		t.Errorf("OAuthMode = %q, want empty when all providers removed", got.OAuthMode)
	}
	if len(got.OAuthProviders) != 0 {
		t.Errorf("OAuthProviders = %v, want empty", got.OAuthProviders)
	}
}

func TestStatusInfo_OAuthModeJSONRoundtrip(t *testing.T) {
	t.Parallel()

	s := StatusInfo{
		Connected:      true,
		OAuthMode:      "oauth",
		OAuthProviders: []string{"openai", "google"},
	}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	var got StatusInfo
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if got.OAuthMode != "oauth" {
		t.Errorf("OAuthMode = %q, want %q", got.OAuthMode, "oauth")
	}
	if len(got.OAuthProviders) != 2 {
		t.Errorf("OAuthProviders len = %d, want 2", len(got.OAuthProviders))
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
