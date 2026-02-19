// Package tui provides a Bubble Tea TUI dashboard for the Local Agent Bridge.
// styles.go defines lipgloss styles for the dashboard panels and status indicators.
package tui

import "github.com/charmbracelet/lipgloss"

// Panel border and title styles.
var (
	// panelStyle defines the base panel with a rounded border.
	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#52525B")).
			Padding(0, 1)

	// activePanelStyle highlights the currently focused panel.
	activePanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#8B5CF6")).
				Padding(0, 1)

	// titleStyle formats panel titles.
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#4C1D95")).
			Padding(0, 1)
)

// Status color styles for connection states.
var (
	// statusConnected renders ocean teal text for the Connected state.
	statusConnected = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#14B8A6")).
			Bold(true)

	// statusDisconnected renders deep coral text for the Disconnected state.
	statusDisconnected = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#E11D48")).
				Bold(true)

	// statusConnecting renders primary purple text for Connecting/Reconnecting states.
	statusConnecting = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#8B5CF6")).
				Bold(true)

	// statusAuthenticating renders dark teal text for the Authenticating state.
	statusAuthenticating = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#0D9488")).
				Bold(true)
)

// Table formatting styles.
var (
	// headerStyle formats table column headers.
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			BorderBottom(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#6D28D9"))

	// selectedRowStyle highlights the currently selected table row.
	selectedRowStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#4C1D95")).
				Foreground(lipgloss.Color("#FFFFFF"))

	// normalRowStyle formats a normal (unselected) table row.
	normalRowStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A1A1AA"))
)

// Label and value styles for key-value pairs.
var (
	// labelStyle formats labels in key-value displays.
	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A1A1AA")).
			Width(16)

	// valueStyle formats values in key-value displays.
	valueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF"))
)

// Task status indicator styles.
var (
	// taskStatusCompleted renders ocean teal text for completed tasks.
	taskStatusCompleted = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#14B8A6"))

	// taskStatusRunning renders primary purple text for running tasks.
	taskStatusRunning = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#8B5CF6"))

	// taskStatusFailed renders deep coral text for failed tasks.
	taskStatusFailed = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#E11D48"))

	// taskStatusPending renders muted gray text for pending tasks.
	taskStatusPending = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#71717A"))
)

// Footer and help styles.
var (
	// helpStyle renders keyboard shortcut hints in the footer.
	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#71717A"))

	// helpKeyStyle renders keyboard shortcut keys.
	helpKeyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#14B8A6")).
			Bold(true)
)
