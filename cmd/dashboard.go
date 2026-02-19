// Package cmd defines the Local Agent Bridge CLI commands.
// dashboard.go implements the TUI dashboard command (FR-P4-01).
package cmd

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/insajin/autopus-bridge/internal/tui"
	"github.com/spf13/cobra"
)

// dashboardCmd opens the interactive TUI dashboard.
var dashboardCmd = &cobra.Command{
	Use:   "dashboard",
	Short: "Open TUI dashboard for monitoring",
	Long: `Opens an interactive TUI dashboard showing connection status,
task history, and resource usage in real-time.

Panels:
  - Connection: current state, server URL, uptime, heartbeat
  - Tasks: recent task history with status and duration
  - Resources: memory, goroutines, message counters

Keyboard shortcuts:
  q          quit dashboard
  r          manual refresh
  t          toggle task detail view
  tab        switch between panels
  up/down    scroll task list`,
	RunE: runDashboard,
}

func init() {
	rootCmd.AddCommand(dashboardCmd)
}

// runDashboard initializes and runs the Bubble Tea TUI program.
func runDashboard(cmd *cobra.Command, args []string) error {
	// Use mock data provider for standalone operation.
	// This will be replaced with a real provider connected to the
	// WebSocket client when integrating with the connect workflow.
	provider := tui.NewMockDataProvider()
	model := tui.NewModel(provider)

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("dashboard error: %w", err)
	}

	return nil
}
