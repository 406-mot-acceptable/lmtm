package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/406-mot-acceptable/lmtm/internal/ssh"
)

// TunnelBuildMsg wraps a tunnel event from the manager for the TUI.
type TunnelBuildMsg struct {
	Event ssh.TunnelEvent
}

// BuildDoneMsg signals all tunnels have been built.
type BuildDoneMsg struct {
	Failed int
	Active int
}

// BuildingModel tracks tunnel construction and drives the animation.
type BuildingModel struct {
	animation AnimationModel
	specs     []ssh.TunnelSpec
	pending   int
	active    int
	failed    int
	done      bool
}

// NewBuildingModel creates the tunnel construction screen.
func NewBuildingModel(specs []ssh.TunnelSpec, gatewayTag string) BuildingModel {
	return BuildingModel{
		animation: NewAnimationModel(specs, gatewayTag),
		specs:     specs,
		pending:   len(specs),
	}
}

// Init starts the animation ticker.
func (m BuildingModel) Init() tea.Cmd {
	return m.animation.Init()
}

// Update handles tunnel build events and animation ticks.
func (m BuildingModel) Update(msg tea.Msg) (BuildingModel, tea.Cmd) {
	switch msg := msg.(type) {
	case TunnelBuildMsg:
		return m.handleEvent(msg.Event)

	case animTickMsg:
		var cmd tea.Cmd
		m.animation, cmd = m.animation.Update(msg)
		return m, cmd
	}

	return m, nil
}

// handleEvent processes a single tunnel event.
func (m BuildingModel) handleEvent(ev ssh.TunnelEvent) (BuildingModel, tea.Cmd) {
	port := ev.Tunnel.LocalPort

	switch ev.Type {
	case ssh.EventStarted:
		m.animation.MarkStarted(port)
		return m, m.animation.tickCmd()

	case ssh.EventActive:
		m.animation.MarkActive(port)
		m.pending--
		m.active++

	case ssh.EventFailed:
		m.animation.MarkFailed(port)
		m.pending--
		m.failed++

	case ssh.EventClosed:
		// Ignore during build phase.
	}

	// Check if all tunnels are done.
	if m.pending <= 0 && !m.done {
		m.done = true
		return m, func() tea.Msg {
			return BuildDoneMsg{
				Failed: m.failed,
				Active: m.active,
			}
		}
	}

	return m, nil
}

// Done returns whether building is complete.
func (m BuildingModel) Done() bool {
	return m.done
}

// View renders the building screen.
func (m BuildingModel) View() string {
	var b strings.Builder

	b.WriteString(m.animation.View())
	b.WriteByte('\n')

	// Progress counter.
	total := len(m.specs)
	completed := m.active + m.failed
	progress := fmt.Sprintf("[%d/%d]", completed, total)
	b.WriteString(DimStyle.Render("Progress: "))
	if m.done {
		b.WriteString(SuccessStyle.Render(progress))
	} else {
		b.WriteString(AccentStyle.Render(progress))
	}
	b.WriteByte('\n')

	// Summary.
	if m.done {
		b.WriteByte('\n')
		if m.failed == 0 {
			b.WriteString(SuccessStyle.Render(
				"All tunnels active"))
		} else {
			b.WriteString(WarningStyle.Render(
				formatBuildSummary(m.active, m.failed)))
		}
		b.WriteByte('\n')
	}

	return ContentStyle.Render(renderPanel("Building Tunnels", b.String()))
}

func formatBuildSummary(active, failed int) string {
	s := ""
	if active > 0 {
		s += SuccessStyle.Render(fmt.Sprintf("%d active", active))
	}
	if failed > 0 {
		if s != "" {
			s += ", "
		}
		s += ErrorStyle.Render(fmt.Sprintf("%d failed", failed))
	}
	return s
}
