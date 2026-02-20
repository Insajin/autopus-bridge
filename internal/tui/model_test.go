package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// staticDataProvider returns fixed data for deterministic tests.
type staticDataProvider struct {
	data DashboardData
}

func (s *staticDataProvider) FetchData() DashboardData {
	return s.data
}

func newTestProvider() *staticDataProvider {
	return &staticDataProvider{
		data: DashboardData{
			ConnectionStatus: StatusConnected,
			ServerURL:        "wss://test.example.com/ws",
			StartTime:        time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			LastHeartbeat:    time.Date(2026, 1, 1, 0, 0, 5, 0, time.UTC),
			Tasks: []TaskEntry{
				{ID: "t-001", Type: "chat", Status: TaskCompleted, Duration: 2 * time.Second, Time: time.Now()},
				{ID: "t-002", Type: "code", Status: TaskRunning, Duration: 0, Time: time.Now()},
				{ID: "t-003", Type: "chat", Status: TaskFailed, Duration: 500 * time.Millisecond, Time: time.Now()},
			},
			MemoryUsageMB:    25.5,
			GoroutineCount:   12,
			MessagesSent:     100,
			MessagesReceived: 95,
		},
	}
}

func TestNewModel_InitialState(t *testing.T) {
	provider := newTestProvider()
	m := NewModel(provider)

	if m.activePanel != PanelConnection {
		t.Errorf("expected initial panel to be PanelConnection, got %d", m.activePanel)
	}
	if m.selectedTask != 0 {
		t.Errorf("expected initial selectedTask to be 0, got %d", m.selectedTask)
	}
	if m.showTaskDetail {
		t.Error("expected showTaskDetail to be false initially")
	}
	if m.quitting {
		t.Error("expected quitting to be false initially")
	}
	if m.data.ConnectionStatus != StatusConnected {
		t.Errorf("expected StatusConnected, got %v", m.data.ConnectionStatus)
	}
	if len(m.data.Tasks) != 3 {
		t.Errorf("expected 3 tasks, got %d", len(m.data.Tasks))
	}
}

func TestKeyBinding_QuitQ(t *testing.T) {
	provider := newTestProvider()
	m := NewModel(provider)

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	model := updated.(Model)

	if !model.quitting {
		t.Error("expected quitting to be true after pressing 'q'")
	}
	if cmd == nil {
		t.Error("expected a tea.Quit command, got nil")
	}
}

func TestKeyBinding_QuitCtrlC(t *testing.T) {
	provider := newTestProvider()
	m := NewModel(provider)

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	model := updated.(Model)

	if !model.quitting {
		t.Error("expected quitting to be true after pressing ctrl+c")
	}
	if cmd == nil {
		t.Error("expected a tea.Quit command, got nil")
	}
}

func TestKeyBinding_Refresh(t *testing.T) {
	provider := newTestProvider()
	m := NewModel(provider)

	// Modify provider data after model creation.
	provider.data.MessagesSent = 200

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	model := updated.(Model)

	if model.data.MessagesSent != 200 {
		t.Errorf("expected MessagesSent=200 after refresh, got %d", model.data.MessagesSent)
	}
}

func TestKeyBinding_ToggleTaskDetail(t *testing.T) {
	provider := newTestProvider()
	m := NewModel(provider)

	if m.showTaskDetail {
		t.Error("expected showTaskDetail to start false")
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	model := updated.(Model)

	if !model.showTaskDetail {
		t.Error("expected showTaskDetail to be true after pressing 't'")
	}

	// Toggle again.
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	model = updated.(Model)

	if model.showTaskDetail {
		t.Error("expected showTaskDetail to be false after second press of 't'")
	}
}

func TestKeyBinding_TabSwitchPanel(t *testing.T) {
	provider := newTestProvider()
	m := NewModel(provider)

	if m.activePanel != PanelConnection {
		t.Errorf("expected PanelConnection, got %d", m.activePanel)
	}

	// Tab forward through all panels.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	model := updated.(Model)
	if model.activePanel != PanelTasks {
		t.Errorf("expected PanelTasks after first tab, got %d", model.activePanel)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model = updated.(Model)
	if model.activePanel != PanelResources {
		t.Errorf("expected PanelResources after second tab, got %d", model.activePanel)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model = updated.(Model)
	if model.activePanel != PanelConnection {
		t.Errorf("expected PanelConnection after third tab (wrap around), got %d", model.activePanel)
	}
}

func TestKeyBinding_ShiftTabSwitchPanel(t *testing.T) {
	provider := newTestProvider()
	m := NewModel(provider)

	// Shift+Tab should go backwards.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	model := updated.(Model)
	if model.activePanel != PanelResources {
		t.Errorf("expected PanelResources after shift+tab from Connection, got %d", model.activePanel)
	}
}

func TestKeyBinding_ArrowNavigation(t *testing.T) {
	provider := newTestProvider()
	m := NewModel(provider)

	// Switch to tasks panel first.
	m.activePanel = PanelTasks

	if m.selectedTask != 0 {
		t.Errorf("expected selectedTask=0, got %d", m.selectedTask)
	}

	// Move down.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	model := updated.(Model)
	if model.selectedTask != 1 {
		t.Errorf("expected selectedTask=1 after down, got %d", model.selectedTask)
	}

	// Move down again.
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updated.(Model)
	if model.selectedTask != 2 {
		t.Errorf("expected selectedTask=2 after second down, got %d", model.selectedTask)
	}

	// Move down at bottom should stay at 2 (3 tasks, index 0-2).
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updated.(Model)
	if model.selectedTask != 2 {
		t.Errorf("expected selectedTask=2 at bottom boundary, got %d", model.selectedTask)
	}

	// Move up.
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyUp})
	model = updated.(Model)
	if model.selectedTask != 1 {
		t.Errorf("expected selectedTask=1 after up, got %d", model.selectedTask)
	}

	// Move up to top.
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyUp})
	model = updated.(Model)
	if model.selectedTask != 0 {
		t.Errorf("expected selectedTask=0 after up to top, got %d", model.selectedTask)
	}

	// Move up at top should stay at 0.
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyUp})
	model = updated.(Model)
	if model.selectedTask != 0 {
		t.Errorf("expected selectedTask=0 at top boundary, got %d", model.selectedTask)
	}
}

func TestKeyBinding_VimNavigation(t *testing.T) {
	provider := newTestProvider()
	m := NewModel(provider)
	m.activePanel = PanelTasks

	// 'j' moves down.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	model := updated.(Model)
	if model.selectedTask != 1 {
		t.Errorf("expected selectedTask=1 after 'j', got %d", model.selectedTask)
	}

	// 'k' moves up.
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	model = updated.(Model)
	if model.selectedTask != 0 {
		t.Errorf("expected selectedTask=0 after 'k', got %d", model.selectedTask)
	}
}

func TestKeyBinding_ArrowsIgnoredWhenNotTaskPanel(t *testing.T) {
	provider := newTestProvider()
	m := NewModel(provider)

	// On PanelConnection, arrow keys should not change selectedTask.
	m.activePanel = PanelConnection

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	model := updated.(Model)
	if model.selectedTask != 0 {
		t.Errorf("expected selectedTask=0 on non-task panel, got %d", model.selectedTask)
	}
}

func TestWindowSizeMsg(t *testing.T) {
	provider := newTestProvider()
	m := NewModel(provider)

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	model := updated.(Model)

	if model.width != 120 {
		t.Errorf("expected width=120, got %d", model.width)
	}
	if model.height != 40 {
		t.Errorf("expected height=40, got %d", model.height)
	}
}

func TestTickMsg_RefreshesData(t *testing.T) {
	provider := newTestProvider()
	m := NewModel(provider)

	provider.data.GoroutineCount = 99

	updated, cmd := m.Update(tickMsg(time.Now()))
	model := updated.(Model)

	if model.data.GoroutineCount != 99 {
		t.Errorf("expected GoroutineCount=99 after tick, got %d", model.data.GoroutineCount)
	}
	if cmd == nil {
		t.Error("expected tickCmd to be returned after tick, got nil")
	}
}

func TestConnectionStatus_String(t *testing.T) {
	tests := []struct {
		status   ConnectionStatus
		expected string
	}{
		{StatusDisconnected, "Disconnected"},
		{StatusConnecting, "Connecting"},
		{StatusAuthenticating, "Authenticating"},
		{StatusConnected, "Connected"},
		{StatusReconnecting, "Reconnecting"},
		{ConnectionStatus(99), "Unknown"},
	}

	for _, tc := range tests {
		if tc.status.String() != tc.expected {
			t.Errorf("ConnectionStatus(%d).String() = %q, want %q", tc.status, tc.status.String(), tc.expected)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d        time.Duration
		expected string
	}{
		{500 * time.Millisecond, "500ms"},
		{5 * time.Second, "5s"},
		{90 * time.Second, "1m 30s"},
		{3661 * time.Second, "1h 1m 1s"},
		{90061 * time.Second, "1d 1h 1m"},
	}

	for _, tc := range tests {
		result := formatDuration(tc.d)
		if result != tc.expected {
			t.Errorf("formatDuration(%v) = %q, want %q", tc.d, result, tc.expected)
		}
	}
}

func TestFormatTaskDuration(t *testing.T) {
	tests := []struct {
		d        time.Duration
		expected string
	}{
		{0, "--"},
		{500 * time.Millisecond, "500ms"},
		{2500 * time.Millisecond, "2.5s"},
	}

	for _, tc := range tests {
		result := formatTaskDuration(tc.d)
		if result != tc.expected {
			t.Errorf("formatTaskDuration(%v) = %q, want %q", tc.d, result, tc.expected)
		}
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"short", 10, "short"},
		{"exactly10!", 10, "exactly10!"},
		{"this is a long string", 10, "this is..."},
		{"ab", 2, "ab"},
		{"abc", 2, "ab"},
	}

	for _, tc := range tests {
		result := truncate(tc.input, tc.maxLen)
		if result != tc.expected {
			t.Errorf("truncate(%q, %d) = %q, want %q", tc.input, tc.maxLen, result, tc.expected)
		}
	}
}

func TestView_ContainsPanelTitles(t *testing.T) {
	provider := newTestProvider()
	m := NewModel(provider)
	m.width = 80
	m.height = 24

	view := m.View()

	if !strings.Contains(view, "Connection") {
		t.Error("expected view to contain 'Connection' panel title")
	}
	if !strings.Contains(view, "Tasks") {
		t.Error("expected view to contain 'Tasks' panel title")
	}
	if !strings.Contains(view, "Resources") {
		t.Error("expected view to contain 'Resources' panel title")
	}
}

func TestView_ContainsServerURL(t *testing.T) {
	provider := newTestProvider()
	m := NewModel(provider)
	m.width = 80
	m.height = 24

	view := m.View()

	if !strings.Contains(view, "wss://test.example.com/ws") {
		t.Error("expected view to contain server URL")
	}
}

func TestView_ContainsResourceData(t *testing.T) {
	provider := newTestProvider()
	m := NewModel(provider)
	m.width = 80
	m.height = 24

	view := m.View()

	if !strings.Contains(view, "25.5 MB") {
		t.Error("expected view to contain memory usage '25.5 MB'")
	}
	if !strings.Contains(view, "12") {
		t.Error("expected view to contain goroutine count '12'")
	}
}

func TestView_QuittingMessage(t *testing.T) {
	provider := newTestProvider()
	m := NewModel(provider)
	m.quitting = true

	view := m.View()

	if view != "Autopus Local Bridge Dashboard closed.\n" {
		t.Errorf("expected quitting view to be 'Autopus Local Bridge Dashboard closed.\\n', got %q", view)
	}
}

func TestView_ContainsKeyboardHelp(t *testing.T) {
	provider := newTestProvider()
	m := NewModel(provider)
	m.width = 120
	m.height = 30

	view := m.View()

	if !strings.Contains(view, "quit") {
		t.Error("expected view to contain 'quit' help text")
	}
	if !strings.Contains(view, "refresh") {
		t.Error("expected view to contain 'refresh' help text")
	}
}

func TestMockDataProvider_ReturnsValidData(t *testing.T) {
	provider := NewMockDataProvider()
	data := provider.FetchData()

	if data.ConnectionStatus != StatusConnected {
		t.Errorf("expected StatusConnected, got %v", data.ConnectionStatus)
	}
	if data.ServerURL == "" {
		t.Error("expected non-empty ServerURL")
	}
	if len(data.Tasks) == 0 {
		t.Error("expected non-empty Tasks list")
	}
	if data.GoroutineCount == 0 {
		t.Error("expected non-zero GoroutineCount")
	}
}

func TestInit_ReturnsTickCmd(t *testing.T) {
	provider := newTestProvider()
	m := NewModel(provider)

	cmd := m.Init()
	if cmd == nil {
		t.Error("expected Init() to return a tick command, got nil")
	}
}
