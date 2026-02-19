package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/406-mot-acceptable/lmtm/internal/tui/components"
)

// DetectStatusMsg updates the detection status text.
type DetectStatusMsg struct {
	Status string
}

// DetectDoneMsg signals detection is complete.
type DetectDoneMsg struct {
	GatewayType string // "MikroTik" or "Ubiquiti"
	Hostname    string
	Err         error
}

// DetectModel is the gateway detection progress screen.
type DetectModel struct {
	spinner     components.SpinnerModel
	gateway     string
	status      string
	gatewayType string
	hostname    string
	done        bool
	err         error
}

// NewDetectModel creates the detection screen for the given gateway address.
func NewDetectModel(gateway string) DetectModel {
	return DetectModel{
		spinner: components.NewSpinner("Connecting..."),
		gateway: gateway,
		status:  "Connecting...",
	}
}

// Init starts the spinner.
func (m DetectModel) Init() tea.Cmd {
	return m.spinner.Init()
}

// Update handles detection progress messages.
func (m DetectModel) Update(msg tea.Msg) (DetectModel, tea.Cmd) {
	switch msg := msg.(type) {
	case DetectStatusMsg:
		m.status = msg.Status
		m.spinner.SetMessage(msg.Status)
		return m, nil

	case DetectDoneMsg:
		m.done = true
		if msg.Err != nil {
			m.err = msg.Err
			m.status = "Detection failed"
		} else {
			m.gatewayType = msg.GatewayType
			m.hostname = msg.Hostname
			m.status = fmt.Sprintf("Detected %s - %q", msg.GatewayType, msg.Hostname)
		}
		return m, nil
	}

	// Forward spinner ticks.
	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

// Done returns whether detection has completed.
func (m DetectModel) Done() bool {
	return m.done
}

// Err returns any detection error.
func (m DetectModel) Err() error {
	return m.err
}

// GatewayType returns the detected gateway type.
func (m DetectModel) GatewayType() string {
	return m.gatewayType
}

// Hostname returns the detected gateway hostname.
func (m DetectModel) Hostname() string {
	return m.hostname
}

// View renders the detection progress screen.
func (m DetectModel) View() string {
	var b strings.Builder

	b.WriteString(LabelStyle.Render("Gateway"))
	b.WriteString(ActiveStyle.Render(m.gateway))
	b.WriteString("\n\n")

	if m.err != nil {
		b.WriteString(ErrorStyle.Render("Error: " + m.err.Error()))
		b.WriteByte('\n')
		b.WriteString(DimStyle.Render("[Esc] back"))
	} else if m.done {
		b.WriteString(SuccessStyle.Render("  " + m.gatewayType))
		if m.hostname != "" {
			b.WriteString(DimStyle.Render(fmt.Sprintf(" - %q", m.hostname)))
		}
		b.WriteByte('\n')
	} else {
		b.WriteString(m.spinner.View())
	}

	return ContentStyle.Render(renderPanel("Detecting Gateway", b.String()))
}
