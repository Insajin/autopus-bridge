package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/insajin/autopus-bridge/internal/mcpserver"
)

func TestServerURLToHTTPBase(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "wss url",
			in:   "wss://api.autopus.co/ws/agent",
			want: "https://api.autopus.co",
		},
		{
			name: "ws url",
			in:   "ws://localhost:8080/ws/agent",
			want: "http://localhost:8080",
		},
		{
			name: "plain http url",
			in:   "https://api.autopus.co",
			want: "https://api.autopus.co",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := serverURLToHTTPBase(tt.in); got != tt.want {
				t.Fatalf("serverURLToHTTPBase(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestFindAgentByName(t *testing.T) {
	agents := []mcpserver.AgentInfo{
		{ID: "a1", Name: "CTO"},
		{ID: "a2", Name: "CMO"},
		{ID: "a3", Name: "Growth PM"},
	}

	agent, err := findAgentByName(agents, "cto")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if agent.ID != "a1" {
		t.Fatalf("agent.ID = %q, want %q", agent.ID, "a1")
	}

	agent, err = findAgentByName(agents, "growth")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if agent.ID != "a3" {
		t.Fatalf("agent.ID = %q, want %q", agent.ID, "a3")
	}
}

func TestChooseAgentInteractively(t *testing.T) {
	in := bytes.NewBufferString("2\n")
	out := &bytes.Buffer{}
	agents := []mcpserver.AgentInfo{
		{ID: "a1", Name: "CTO"},
		{ID: "a2", Name: "CMO"},
	}

	agent, err := chooseAgentInteractively(in, out, agents)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if agent.ID != "a2" {
		t.Fatalf("agent.ID = %q, want %q", agent.ID, "a2")
	}
	if out.Len() == 0 {
		t.Fatal("expected interactive prompt output")
	}
}

func TestIsTerminalExecutionStatus(t *testing.T) {
	tests := []struct {
		status string
		want   bool
	}{
		{status: "pending", want: false},
		{status: "running", want: false},
		{status: "completed", want: true},
		{status: "failed", want: true},
		{status: "approved", want: true},
		{status: "rejected", want: true},
	}

	for _, tt := range tests {
		if got := isTerminalExecutionStatus(tt.status); got != tt.want {
			t.Fatalf("isTerminalExecutionStatus(%q) = %v, want %v", tt.status, got, tt.want)
		}
	}
}

func TestWaitForExecution(t *testing.T) {
	fetcher := &stubExecutionStatusFetcher{
		statuses: []*mcpserver.ExecutionStatus{
			{ExecutionID: "exec-1", Status: "running"},
			{ExecutionID: "exec-1", Status: "completed", Result: []byte(`{"output":"done"}`)},
		},
	}

	out := &bytes.Buffer{}
	status, err := waitForExecution(context.Background(), fetcher, "exec-1", 1, out, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Status != "completed" {
		t.Fatalf("status = %q, want %q", status.Status, "completed")
	}
	if out.Len() == 0 {
		t.Fatal("expected progress output")
	}
}

func TestWaitForExecution_PropagatesError(t *testing.T) {
	fetcher := &stubExecutionStatusFetcher{
		err: errors.New("backend down"),
	}

	_, err := waitForExecution(context.Background(), fetcher, "exec-1", 1, &bytes.Buffer{}, true)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateExecuteMode(t *testing.T) {
	reset := saveExecuteFlags()
	defer reset()

	executeWait = true
	executeStream = true
	if err := validateExecuteMode(); err == nil {
		t.Fatal("expected mutual exclusion error")
	}

	reset = saveExecuteFlags()
	defer reset()
	executeStream = true
	executeJSON = true
	if err := validateExecuteMode(); err == nil {
		t.Fatal("expected stream/json incompatibility")
	}
}

func TestImmediateExecutionStatus_UsesInlineResult(t *testing.T) {
	status, ok := immediateExecutionStatus(&mcpserver.ExecuteTaskResponse{
		ExecutionID: "exec-inline",
		Result:      []byte(`{"output":"done"}`),
	})
	if !ok {
		t.Fatal("expected immediate status")
	}
	if status.Status != "completed" {
		t.Fatalf("status = %q, want %q", status.Status, "completed")
	}
	if status.ExecutionID != "exec-inline" {
		t.Fatalf("execution_id = %q, want %q", status.ExecutionID, "exec-inline")
	}
	if string(status.Result) != `{"output":"done"}` {
		t.Fatalf("result = %s", string(status.Result))
	}
}

func TestImmediateExecutionStatus_UsesTerminalStatus(t *testing.T) {
	status, ok := immediateExecutionStatus(&mcpserver.ExecuteTaskResponse{
		ExecutionID: "exec-terminal",
		Status:      "failed",
	})
	if !ok {
		t.Fatal("expected immediate status")
	}
	if status.Status != "failed" {
		t.Fatalf("status = %q, want %q", status.Status, "failed")
	}
}

func TestImmediateExecutionStatus_PendingResponseFallsBackToPolling(t *testing.T) {
	status, ok := immediateExecutionStatus(&mcpserver.ExecuteTaskResponse{
		ExecutionID: "exec-pending",
		Status:      "running",
	})
	if ok || status != nil {
		t.Fatal("expected polling fallback")
	}
}

func TestBuildExecutionStreamURL(t *testing.T) {
	reset := saveExecuteFlags()
	defer reset()
	executeProvider = "claude"
	executeModel = "claude-sonnet"
	executeTools = []string{"bash", "git"}
	executeTimeoutSec = 120
	executeMaxTokens = 4096

	got, err := buildExecutionStreamURL(
		"https://api.autopus.co",
		"ws-1",
		"exec-1",
		"agent-1",
		"hello world",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, wantPart := range []string{
		"https://api.autopus.co/api/v1/workspaces/ws-1/executions/exec-1/stream?",
		"agent_id=agent-1",
		"prompt=hello+world",
		"provider=claude",
		"model=claude-sonnet",
		"tools=bash%2Cgit",
		"timeout_seconds=120",
		"max_tokens=4096",
	} {
		if !strings.Contains(got, wantPart) {
			t.Fatalf("buildExecutionStreamURL() = %q, missing %q", got, wantPart)
		}
	}
}

func TestStreamPrinter(t *testing.T) {
	buf := &bytes.Buffer{}
	printer := &streamPrinter{out: buf}

	err := printer.printEvent(sseStreamEvent{
		Type:    "content",
		Content: []byte(`"hello"`),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = printer.printEvent(sseStreamEvent{
		Type:    "tool_use",
		Content: []byte(`"Read"`),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := printer.finish(); err != nil {
		t.Fatalf("unexpected finish error: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "hello") {
		t.Fatalf("expected content output, got %q", got)
	}
	if !strings.Contains(got, "[tool_use]") {
		t.Fatalf("expected tagged event output, got %q", got)
	}
}

func TestStreamPrinter_SummaryEvent(t *testing.T) {
	printer := &streamPrinter{out: &bytes.Buffer{}}

	err := printer.printEvent(sseStreamEvent{
		Type:    "summary",
		Content: []byte(`{"execution_id":"exec-1","status":"completed","result":{"output":"done"}}`),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if printer.summary == nil {
		t.Fatal("expected summary to be captured")
	}
	if printer.summary.Status != "completed" {
		t.Fatalf("summary status = %q, want %q", printer.summary.Status, "completed")
	}
}

func TestFetchStreamExecutionSummary(t *testing.T) {
	fetcher := &stubExecutionStatusFetcher{
		statuses: []*mcpserver.ExecutionStatus{
			{ExecutionID: "exec-1", Status: "running"},
			{ExecutionID: "exec-1", Status: "completed", Result: []byte(`{"output":"done"}`)},
		},
	}

	status, err := fetchStreamExecutionSummary(context.Background(), fetcher, "exec-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Status != "completed" {
		t.Fatalf("status = %q, want %q", status.Status, "completed")
	}
}

func TestRecordExecutionStatus_PreservesExistingStatusAndIncrementsCompleted(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	startTime := time.Now().Add(-time.Minute)
	initial := &StatusInfo{
		Connected:      true,
		ServerURL:      "wss://api.autopus.co/ws/agent",
		StartTime:      &startTime,
		TasksCompleted: 2,
		TasksFailed:    1,
		PID:            1234,
		WorkspaceID:    "ws-1",
	}
	if err := SaveStatus(initial); err != nil {
		t.Fatalf("SaveStatus() error = %v", err)
	}

	if err := recordExecutionStatus("ws-1", "completed"); err != nil {
		t.Fatalf("recordExecutionStatus() error = %v", err)
	}

	data, err := os.ReadFile(getScopedStatusFilePath("ws-1"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	var got StatusInfo
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if !got.Connected {
		t.Fatal("Connected = false, want true")
	}
	if got.PID != 1234 {
		t.Fatalf("PID = %d, want %d", got.PID, 1234)
	}
	if got.TasksCompleted != 3 {
		t.Fatalf("TasksCompleted = %d, want %d", got.TasksCompleted, 3)
	}
	if got.TasksFailed != 1 {
		t.Fatalf("TasksFailed = %d, want %d", got.TasksFailed, 1)
	}
}

func TestRecordExecutionStatus_CreatesStatusFileForFailures(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := recordExecutionStatus("ws-2", "failed"); err != nil {
		t.Fatalf("recordExecutionStatus() error = %v", err)
	}

	data, err := os.ReadFile(getScopedStatusFilePath("ws-2"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	var got StatusInfo
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if got.WorkspaceID != "ws-2" {
		t.Fatalf("WorkspaceID = %q, want %q", got.WorkspaceID, "ws-2")
	}
	if got.TasksCompleted != 0 {
		t.Fatalf("TasksCompleted = %d, want %d", got.TasksCompleted, 0)
	}
	if got.TasksFailed != 1 {
		t.Fatalf("TasksFailed = %d, want %d", got.TasksFailed, 1)
	}
}

func saveExecuteFlags() func() {
	prevAgentID := executeAgentID
	prevAgentName := executeAgentName
	prevWorkspace := executeWorkspace
	prevModel := executeModel
	prevProvider := executeProvider
	prevTools := append([]string(nil), executeTools...)
	prevTimeout := executeTimeoutSec
	prevMaxTokens := executeMaxTokens
	prevWait := executeWait
	prevWaitPoll := executeWaitPoll
	prevWaitLimit := executeWaitLimit
	prevStream := executeStream
	prevJSON := executeJSON

	return func() {
		executeAgentID = prevAgentID
		executeAgentName = prevAgentName
		executeWorkspace = prevWorkspace
		executeModel = prevModel
		executeProvider = prevProvider
		executeTools = prevTools
		executeTimeoutSec = prevTimeout
		executeMaxTokens = prevMaxTokens
		executeWait = prevWait
		executeWaitPoll = prevWaitPoll
		executeWaitLimit = prevWaitLimit
		executeStream = prevStream
		executeJSON = prevJSON
	}
}

type stubExecutionStatusFetcher struct {
	statuses []*mcpserver.ExecutionStatus
	err      error
	index    int
}

func (s *stubExecutionStatusFetcher) GetExecutionStatus(_ context.Context, executionID string) (*mcpserver.ExecutionStatus, error) {
	if s.err != nil {
		return nil, s.err
	}
	if len(s.statuses) == 0 {
		return &mcpserver.ExecutionStatus{ExecutionID: executionID, Status: "completed"}, nil
	}
	if s.index >= len(s.statuses) {
		return s.statuses[len(s.statuses)-1], nil
	}
	status := s.statuses[s.index]
	s.index++
	return status, nil
}
