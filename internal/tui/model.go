// Package tui provides a Bubble Tea TUI dashboard for the Local Agent Bridge.
// model.go implements the main Bubble Tea model with three panels:
// connection status, task history, and resource usage.
//
// FR-P4-01: TUI dashboard for monitoring Local Agent Bridge.
package tui

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Panel represents which dashboard panel is currently focused.
type Panel int

const (
	// PanelConnection is the connection status panel (top).
	PanelConnection Panel = iota
	// PanelTasks is the task history panel (middle).
	PanelTasks
	// PanelResources is the resource usage panel (bottom).
	PanelResources

	panelCount = 3
)

// ConnectionStatus represents the current connection state.
type ConnectionStatus int

const (
	StatusDisconnected ConnectionStatus = iota
	StatusConnecting
	StatusAuthenticating
	StatusConnected
	StatusReconnecting
)

// String returns a human-readable connection status label.
func (s ConnectionStatus) String() string {
	switch s {
	case StatusDisconnected:
		return "Disconnected"
	case StatusConnecting:
		return "Connecting"
	case StatusAuthenticating:
		return "Authenticating"
	case StatusConnected:
		return "Connected"
	case StatusReconnecting:
		return "Reconnecting"
	default:
		return "Unknown"
	}
}

// TaskStatus represents the status of a task.
type TaskStatus string

const (
	TaskPending   TaskStatus = "pending"
	TaskRunning   TaskStatus = "running"
	TaskCompleted TaskStatus = "completed"
	TaskFailed    TaskStatus = "failed"
)

// TaskEntry represents a single task in the history.
type TaskEntry struct {
	ID       string
	Type     string
	Status   TaskStatus
	Duration time.Duration
	Time     time.Time
}

// DashboardData holds all data displayed on the dashboard.
// This struct is designed to be populated externally so the TUI
// can be connected to the real WebSocket client later.
type DashboardData struct {
	// Connection panel data
	ConnectionStatus ConnectionStatus
	ServerURL        string
	StartTime        time.Time
	LastHeartbeat    time.Time

	// Task history panel data
	Tasks []TaskEntry

	// Resource usage panel data
	MemoryUsageMB    float64
	GoroutineCount   int
	MessagesSent     int64
	MessagesReceived int64
}

// DataProvider is an interface for fetching dashboard data.
// Implement this interface to connect the TUI to real data sources.
type DataProvider interface {
	// FetchData returns the current dashboard data snapshot.
	FetchData() DashboardData
}

// mockDataProvider provides sample data for standalone testing.
type mockDataProvider struct {
	startTime time.Time
}

// NewMockDataProvider creates a DataProvider with sample data.
func NewMockDataProvider() DataProvider {
	return &mockDataProvider{
		startTime: time.Now(),
	}
}

// FetchData returns mock dashboard data for demonstration.
func (m *mockDataProvider) FetchData() DashboardData {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	return DashboardData{
		ConnectionStatus: StatusConnected,
		ServerURL:        "wss://api.autopus.co/ws/agent",
		StartTime:        m.startTime,
		LastHeartbeat:    time.Now().Add(-3 * time.Second),
		Tasks: []TaskEntry{
			{ID: "task-001", Type: "chat", Status: TaskCompleted, Duration: 2350 * time.Millisecond, Time: time.Now().Add(-10 * time.Minute)},
			{ID: "task-002", Type: "code", Status: TaskCompleted, Duration: 5120 * time.Millisecond, Time: time.Now().Add(-8 * time.Minute)},
			{ID: "task-003", Type: "chat", Status: TaskFailed, Duration: 1200 * time.Millisecond, Time: time.Now().Add(-5 * time.Minute)},
			{ID: "task-004", Type: "analysis", Status: TaskRunning, Duration: 0, Time: time.Now().Add(-30 * time.Second)},
			{ID: "task-005", Type: "chat", Status: TaskPending, Duration: 0, Time: time.Now()},
		},
		MemoryUsageMB:    float64(memStats.Alloc) / 1024 / 1024,
		GoroutineCount:   runtime.NumGoroutine(),
		MessagesSent:     42,
		MessagesReceived: 38,
	}
}

// tickMsg signals a periodic data refresh.
type tickMsg time.Time

// Model is the main Bubble Tea model for the dashboard.
type Model struct {
	// data holds the current dashboard snapshot.
	data DashboardData
	// provider fetches fresh data on each tick.
	provider DataProvider
	// activePanel tracks the currently focused panel.
	activePanel Panel
	// selectedTask tracks the selected task row index.
	selectedTask int
	// taskScrollOffset tracks the scroll offset for the task list.
	taskScrollOffset int
	// showTaskDetail toggles expanded task detail view.
	showTaskDetail bool
	// width and height store the terminal dimensions.
	width  int
	height int
	// quitting signals the program should exit.
	quitting bool
}

// NewModel creates a new dashboard Model with the given DataProvider.
func NewModel(provider DataProvider) Model {
	data := provider.FetchData()
	return Model{
		data:        data,
		provider:    provider,
		activePanel: PanelConnection,
	}
}

// Init implements tea.Model. It starts the auto-refresh ticker.
func (m Model) Init() tea.Cmd {
	return tickCmd()
}

// tickCmd returns a command that sends a tickMsg every 2 seconds.
func tickCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// Update implements tea.Model. It processes messages and updates state.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tickMsg:
		m.data = m.provider.FetchData()
		return m, tickCmd()
	}

	return m, nil
}

// handleKeyPress processes keyboard input.
func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit

	case "r":
		m.data = m.provider.FetchData()
		return m, nil

	case "t":
		m.showTaskDetail = !m.showTaskDetail
		return m, nil

	case "tab":
		m.activePanel = (m.activePanel + 1) % panelCount
		return m, nil

	case "shift+tab":
		m.activePanel = (m.activePanel - 1 + panelCount) % panelCount
		return m, nil

	case "up", "k":
		if m.activePanel == PanelTasks && len(m.data.Tasks) > 0 {
			if m.selectedTask > 0 {
				m.selectedTask--
			}
			// Adjust scroll offset
			if m.selectedTask < m.taskScrollOffset {
				m.taskScrollOffset = m.selectedTask
			}
		}
		return m, nil

	case "down", "j":
		if m.activePanel == PanelTasks && len(m.data.Tasks) > 0 {
			if m.selectedTask < len(m.data.Tasks)-1 {
				m.selectedTask++
			}
			// Adjust scroll offset (show max 5 visible rows)
			maxVisible := 5
			if m.selectedTask >= m.taskScrollOffset+maxVisible {
				m.taskScrollOffset = m.selectedTask - maxVisible + 1
			}
		}
		return m, nil
	}

	return m, nil
}

// View implements tea.Model. It renders the dashboard.
func (m Model) View() string {
	if m.quitting {
		return "Autopus Local Bridge Dashboard closed.\n"
	}

	// Use sensible defaults if window size is not yet reported.
	w := m.width
	if w == 0 {
		w = 80
	}
	// Reserve space for header and footer.
	contentWidth := w - 2
	if contentWidth < 40 {
		contentWidth = 40
	}

	// Build each panel.
	connPanel := m.renderConnectionPanel(contentWidth)
	taskPanel := m.renderTaskPanel(contentWidth)
	resPanel := m.renderResourcePanel(contentWidth)

	// Build header.
	header := m.renderHeader(contentWidth)

	// Build footer.
	footer := m.renderFooter(contentWidth)

	// Compose full layout.
	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		connPanel,
		taskPanel,
		resPanel,
		footer,
	)
}

// renderHeader returns the dashboard title bar.
func (m Model) renderHeader(width int) string {
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(lipgloss.Color("#4C1D95")).
		Padding(0, 1).
		Width(width).
		Render("Autopus Local Bridge Dashboard")
	return title
}

// renderFooter returns the keyboard shortcut help bar.
func (m Model) renderFooter(width int) string {
	keys := []struct {
		key  string
		desc string
	}{
		{"q", "quit"},
		{"r", "refresh"},
		{"t", "toggle detail"},
		{"tab", "switch panel"},
		{"up/down", "scroll tasks"},
	}

	var parts []string
	for _, k := range keys {
		parts = append(parts,
			helpKeyStyle.Render(k.key)+" "+helpStyle.Render(k.desc),
		)
	}

	help := strings.Join(parts, helpStyle.Render("  |  "))
	return lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Render(help)
}

// renderConnectionPanel renders the connection status panel.
func (m Model) renderConnectionPanel(width int) string {
	// Format status with color.
	statusText := m.formatConnectionStatus()

	// Calculate uptime.
	var uptimeStr string
	if !m.data.StartTime.IsZero() {
		uptimeStr = formatDuration(time.Since(m.data.StartTime))
	} else {
		uptimeStr = "--"
	}

	// Format last heartbeat.
	var heartbeatStr string
	if !m.data.LastHeartbeat.IsZero() {
		ago := time.Since(m.data.LastHeartbeat)
		heartbeatStr = fmt.Sprintf("%s ago", formatDuration(ago))
	} else {
		heartbeatStr = "--"
	}

	// Build content lines.
	lines := []string{
		labelStyle.Render("Status:") + " " + statusText,
		labelStyle.Render("Server:") + " " + valueStyle.Render(m.data.ServerURL),
		labelStyle.Render("Uptime:") + " " + valueStyle.Render(uptimeStr),
		labelStyle.Render("Last Heartbeat:") + " " + valueStyle.Render(heartbeatStr),
	}
	content := strings.Join(lines, "\n")

	// Apply panel style.
	style := m.getPanelStyle(PanelConnection, width)
	title := titleStyle.Render(" Connection ")
	return title + "\n" + style.Render(content)
}

// renderTaskPanel renders the task history panel.
func (m Model) renderTaskPanel(width int) string {
	// Column widths.
	colID := 12
	colType := 10
	colStatus := 12
	colDuration := 10
	colTime := 18

	// Render header row.
	header := headerStyle.Render(
		fmt.Sprintf("%-*s %-*s %-*s %-*s %-*s",
			colID, "ID",
			colType, "Type",
			colStatus, "Status",
			colDuration, "Duration",
			colTime, "Time",
		),
	)

	// Render task rows.
	var rows []string
	rows = append(rows, header)

	if len(m.data.Tasks) == 0 {
		rows = append(rows, normalRowStyle.Render("  No tasks yet"))
	} else {
		maxVisible := 5
		end := m.taskScrollOffset + maxVisible
		if end > len(m.data.Tasks) {
			end = len(m.data.Tasks)
		}

		for i := m.taskScrollOffset; i < end; i++ {
			task := m.data.Tasks[i]
			row := fmt.Sprintf("%-*s %-*s %-*s %-*s %-*s",
				colID, truncate(task.ID, colID),
				colType, truncate(task.Type, colType),
				colStatus, m.formatTaskStatus(task.Status),
				colDuration, formatTaskDuration(task.Duration),
				colTime, task.Time.Format("15:04:05"),
			)

			if i == m.selectedTask && m.activePanel == PanelTasks {
				rows = append(rows, selectedRowStyle.Render(row))
			} else {
				rows = append(rows, normalRowStyle.Render(row))
			}
		}

		// Show scroll indicator if needed.
		if len(m.data.Tasks) > maxVisible {
			indicator := fmt.Sprintf("  [%d/%d tasks]", m.selectedTask+1, len(m.data.Tasks))
			rows = append(rows, helpStyle.Render(indicator))
		}
	}

	// Show task detail if toggled on and a task is selected.
	if m.showTaskDetail && len(m.data.Tasks) > 0 && m.selectedTask < len(m.data.Tasks) {
		task := m.data.Tasks[m.selectedTask]
		detail := fmt.Sprintf(
			"\n  Detail: ID=%s  Type=%s  Status=%s  Duration=%s  Time=%s",
			task.ID, task.Type, string(task.Status),
			formatTaskDuration(task.Duration),
			task.Time.Format("2006-01-02 15:04:05"),
		)
		rows = append(rows, helpStyle.Render(detail))
	}

	content := strings.Join(rows, "\n")
	style := m.getPanelStyle(PanelTasks, width)
	title := titleStyle.Render(" Tasks ")
	return title + "\n" + style.Render(content)
}

// renderResourcePanel renders the resource usage panel.
func (m Model) renderResourcePanel(width int) string {
	lines := []string{
		labelStyle.Render("Memory:") + " " + valueStyle.Render(fmt.Sprintf("%.1f MB", m.data.MemoryUsageMB)),
		labelStyle.Render("Goroutines:") + " " + valueStyle.Render(fmt.Sprintf("%d", m.data.GoroutineCount)),
		labelStyle.Render("Messages Sent:") + " " + valueStyle.Render(fmt.Sprintf("%d", m.data.MessagesSent)),
		labelStyle.Render("Messages Recv:") + " " + valueStyle.Render(fmt.Sprintf("%d", m.data.MessagesReceived)),
	}
	content := strings.Join(lines, "\n")

	style := m.getPanelStyle(PanelResources, width)
	title := titleStyle.Render(" Resources ")
	return title + "\n" + style.Render(content)
}

// getPanelStyle returns the appropriate panel style based on focus state.
func (m Model) getPanelStyle(panel Panel, width int) lipgloss.Style {
	if m.activePanel == panel {
		return activePanelStyle.Width(width - 2)
	}
	return panelStyle.Width(width - 2)
}

// formatConnectionStatus returns a color-coded connection status string.
func (m Model) formatConnectionStatus() string {
	switch m.data.ConnectionStatus {
	case StatusConnected:
		return statusConnected.Render("Connected")
	case StatusDisconnected:
		return statusDisconnected.Render("Disconnected")
	case StatusConnecting:
		return statusConnecting.Render("Connecting...")
	case StatusAuthenticating:
		return statusAuthenticating.Render("Authenticating...")
	case StatusReconnecting:
		return statusConnecting.Render("Reconnecting...")
	default:
		return statusDisconnected.Render("Unknown")
	}
}

// formatTaskStatus returns a color-coded task status string.
func (m Model) formatTaskStatus(status TaskStatus) string {
	switch status {
	case TaskCompleted:
		return taskStatusCompleted.Render(string(status))
	case TaskRunning:
		return taskStatusRunning.Render(string(status))
	case TaskFailed:
		return taskStatusFailed.Render(string(status))
	case TaskPending:
		return taskStatusPending.Render(string(status))
	default:
		return string(status)
	}
}

// formatDuration formats a duration into a human-readable string.
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}

	totalSeconds := int(d.Seconds())
	days := totalSeconds / 86400
	hours := (totalSeconds % 86400) / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}

// formatTaskDuration formats a task duration. Zero duration shows "--".
func formatTaskDuration(d time.Duration) string {
	if d == 0 {
		return "--"
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}

// truncate shortens a string to maxLen, adding an ellipsis if needed.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
