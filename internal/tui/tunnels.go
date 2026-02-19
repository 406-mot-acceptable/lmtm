package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/406-mot-acceptable/lmtm/internal/ssh"
	"github.com/406-mot-acceptable/lmtm/internal/tui/components"
)

// TunnelUpdateMsg carries a tunnel status change to the dashboard.
type TunnelUpdateMsg struct {
	Event ssh.TunnelEvent
}

// DisconnectMsg signals the user wants to disconnect.
type DisconnectMsg struct{}

// ReconnectMsg signals the user wants to reconnect failed tunnels.
type ReconnectMsg struct{}

// tunnelTickMsg is the elapsed time ticker.
type tunnelTickMsg time.Time

// tunnelGroup groups tunnels by remote device.
type tunnelGroup struct {
	RemoteHost string
	Tunnels    []tunnelEntry
}

// tunnelEntry is a single tunnel in the dashboard.
type tunnelEntry struct {
	LocalPort  int
	RemotePort int
	Status     ssh.TunnelStatus
	Error      string
}

// TunnelsModel is the active tunnel dashboard.
type TunnelsModel struct {
	groups     []tunnelGroup
	startTime  time.Time
	elapsed    time.Duration
	tunnelKeys TunnelKeys
	globals    GlobalKeys
	milestone  string
}

// NewTunnelsModel creates the active tunnel dashboard from the current tunnels.
func NewTunnelsModel(tunnels []*ssh.Tunnel) TunnelsModel {
	groups := groupTunnels(tunnels)
	return TunnelsModel{
		groups:     groups,
		startTime:  time.Now(),
		tunnelKeys: DefaultTunnelKeys,
		globals:    DefaultGlobalKeys,
	}
}

// Init starts the elapsed time ticker.
func (m TunnelsModel) Init() tea.Cmd {
	return m.tickCmd()
}

// Update handles tunnel updates, user input, and elapsed ticks.
func (m TunnelsModel) Update(msg tea.Msg) (TunnelsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.globals.Quit):
			return m, func() tea.Msg { return DisconnectMsg{} }
		case key.Matches(msg, m.tunnelKeys.Reconnect):
			return m, func() tea.Msg { return ReconnectMsg{} }
		}

	case TunnelUpdateMsg:
		m.applyUpdate(msg.Event)
		return m, nil

	case tunnelTickMsg:
		m.elapsed = time.Since(m.startTime)
		return m, m.tickCmd()
	}

	return m, nil
}

// applyUpdate updates a tunnel entry's status from an event.
func (m *TunnelsModel) applyUpdate(ev ssh.TunnelEvent) {
	port := ev.Tunnel.LocalPort
	for gi := range m.groups {
		for ti := range m.groups[gi].Tunnels {
			if m.groups[gi].Tunnels[ti].LocalPort == port {
				switch ev.Type {
				case ssh.EventActive:
					m.groups[gi].Tunnels[ti].Status = ssh.StatusActive
					m.groups[gi].Tunnels[ti].Error = ""
				case ssh.EventFailed:
					m.groups[gi].Tunnels[ti].Status = ssh.StatusFailed
					if ev.Tunnel.Error != nil {
						m.groups[gi].Tunnels[ti].Error = ev.Tunnel.Error.Error()
					}
				case ssh.EventClosed:
					m.groups[gi].Tunnels[ti].Status = ssh.StatusDisconnected
				}
				return
			}
		}
	}
}

// View renders the active tunnel dashboard.
func (m TunnelsModel) View() string {
	var b strings.Builder

	// Tunnel groups by device.
	activeCount := 0
	failedCount := 0

	for gi, g := range m.groups {
		var group strings.Builder
		for i, t := range g.Tunnels {
			last := i == len(g.Tunnels)-1
			connector := "├─ "
			if last {
				connector = "└─ "
			}
			group.WriteString(DimStyle.Render(connector))

			// LOCAL:PORT --> REMOTE:PORT with clickable hyperlink.
			link := portLink(t.LocalPort, t.RemotePort)
			group.WriteString(link)
			group.WriteString(DimStyle.Render(" --> "))
			group.WriteString(fmt.Sprintf("%s:%d", g.RemoteHost, t.RemotePort))

			// Status indicator.
			group.WriteString("  ")
			switch t.Status {
			case ssh.StatusActive:
				group.WriteString(SuccessStyle.Render("[active]"))
				activeCount++
			case ssh.StatusFailed:
				group.WriteString(ErrorStyle.Render("[failed]"))
				failedCount++
				if t.Error != "" {
					group.WriteString(DimStyle.Render(" " + t.Error))
				}
			case ssh.StatusConnecting:
				group.WriteString(WarningStyle.Render("[connecting]"))
			default:
				group.WriteString(DimStyle.Render("[closed]"))
			}
			group.WriteByte('\n')
		}

		b.WriteString(InnerPanelStyle.Render(
			ActiveStyle.Render(g.RemoteHost) + "\n" + group.String(),
		))
		if gi < len(m.groups)-1 {
			b.WriteByte('\n')
		}
	}

	panel := renderPanel("Active Tunnels", b.String())

	// Milestone easter egg.
	if m.milestone != "" {
		panel += "\n" + SubtitleStyle.Render("  "+m.milestone)
	}

	// Status bar.
	uptime := fmt.Sprintf("UP %s", formatDuration(m.elapsed))
	summary := fmt.Sprintf("%d active", activeCount)
	if failedCount > 0 {
		summary += fmt.Sprintf(", %d failed", failedCount)
	}
	bar := renderStatusBar(uptime, summary, "q: disconnect", "r: reconnect")

	return ContentStyle.Render(panel + "\n" + bar)
}

// portLink returns a clickable OSC8 hyperlink appropriate for the remote port.
func portLink(localPort, remotePort int) string {
	switch remotePort {
	case 443:
		return components.HTTPSLink(localPort)
	default:
		return components.HTTPLink(localPort)
	}
}

// groupTunnels organizes tunnels by their remote host.
func groupTunnels(tunnels []*ssh.Tunnel) []tunnelGroup {
	order := make([]string, 0)
	byHost := make(map[string][]tunnelEntry)

	for _, t := range tunnels {
		entry := tunnelEntry{
			LocalPort:  t.LocalPort,
			RemotePort: t.RemotePort,
			Status:     t.Status,
		}
		if t.Error != nil {
			entry.Error = t.Error.Error()
		}

		if _, exists := byHost[t.RemoteHost]; !exists {
			order = append(order, t.RemoteHost)
		}
		byHost[t.RemoteHost] = append(byHost[t.RemoteHost], entry)
	}

	groups := make([]tunnelGroup, len(order))
	for i, host := range order {
		groups[i] = tunnelGroup{
			RemoteHost: host,
			Tunnels:    byHost[host],
		}
	}
	return groups
}

// formatDuration renders a duration as "Xm Ys" or "Xh Ym".
func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60

	if h > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

func (m TunnelsModel) tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tunnelTickMsg(t)
	})
}
