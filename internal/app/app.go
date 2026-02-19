package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/406-mot-acceptable/lmtm/internal/tui"
)

// Run starts the Tunneler TUI application.
func Run() error {
	model := tui.NewAppModel()
	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
