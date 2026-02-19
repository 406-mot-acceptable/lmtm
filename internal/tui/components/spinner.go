package components

import (
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SpinnerModel wraps a bubbles spinner with an accompanying message.
type SpinnerModel struct {
	spinner spinner.Model
	message string
	style   lipgloss.Style
}

// NewSpinner creates a spinner with a message displayed beside it.
func NewSpinner(msg string) SpinnerModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{
		Dark:  "#AF87FF",
		Light: "#7B5FBF",
	})
	return SpinnerModel{
		spinner: s,
		message: msg,
		style:   lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Dark: "#E0E0E0", Light: "#1A1A1A"}),
	}
}

// SetMessage updates the spinner's message text.
func (m *SpinnerModel) SetMessage(msg string) {
	m.message = msg
}

// Init starts the spinner tick.
func (m SpinnerModel) Init() tea.Cmd {
	return m.spinner.Tick
}

// Update handles spinner tick messages.
func (m SpinnerModel) Update(msg tea.Msg) (SpinnerModel, tea.Cmd) {
	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

// View renders the spinner and its message.
func (m SpinnerModel) View() string {
	return m.spinner.View() + " " + m.style.Render(m.message)
}
