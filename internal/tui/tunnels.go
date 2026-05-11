package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
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
	RemoteMAC  string
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

	viewport viewport.Model
	width    int
	height   int
	ready    bool
}

// NewTunnelsModel creates the active tunnel dashboard from the current tunnels.
// macByHost maps RemoteHost -> MAC address for display next to each device.
func NewTunnelsModel(tunnels []*ssh.Tunnel, macByHost map[string]string, width, height int) TunnelsModel {
	groups := groupTunnels(tunnels, macByHost)
	m := TunnelsModel{
		groups:     groups,
		startTime:  time.Now(),
		tunnelKeys: DefaultTunnelKeys,
		globals:    DefaultGlobalKeys,
		width:      width,
		height:     height,
	}
	m.initViewport()
	return m
}

// Init starts the elapsed time ticker.
func (m TunnelsModel) Init() tea.Cmd {
	return m.tickCmd()
}

// Update handles tunnel updates, user input, and elapsed ticks.
func (m TunnelsModel) Update(msg tea.Msg) (TunnelsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.initViewport()
		return m, nil

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.globals.Quit):
			return m, func() tea.Msg { return DisconnectMsg{} }
		case key.Matches(msg, m.tunnelKeys.Reconnect):
			return m, func() tea.Msg { return ReconnectMsg{} }
		}
		// Forward navigation keys to the viewport.
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd

	case TunnelUpdateMsg:
		m.applyUpdate(msg.Event)
		m.refreshContent()
		return m, nil

	case tunnelTickMsg:
		m.elapsed = time.Since(m.startTime)
		return m, m.tickCmd()
	}

	return m, nil
}

// initViewport sizes the viewport based on the current terminal dimensions
// and seeds it with the current device list rendering.
func (m *TunnelsModel) initViewport() {
	// Reserve rows for: outer padding (top+bottom = 2), panel border (top+bottom = 2),
	// panel padding (top+bottom = 2), milestone (1), status bar (1).
	const chrome = 8
	vw := m.width - 8 // account for ContentStyle (1,2) + PanelStyle (1,2) horizontal padding
	if vw < 20 {
		vw = 20
	}
	vh := m.height - chrome
	if vh < 3 {
		vh = 3
	}
	if !m.ready {
		m.viewport = viewport.New(vw, vh)
		m.ready = true
	} else {
		m.viewport.Width = vw
		m.viewport.Height = vh
	}
	m.refreshContent()
}

// refreshContent rebuilds the viewport content from the current groups.
func (m *TunnelsModel) refreshContent() {
	if !m.ready {
		return
	}
	m.viewport.SetContent(m.renderDeviceList())
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

// renderDeviceList builds the scrollable body: one line per device with
// subnet IP, local-port list, MAC, and a per-device status summary.
func (m TunnelsModel) renderDeviceList() string {
	var b strings.Builder
	for i, g := range m.groups {
		b.WriteString(m.renderDeviceLine(g))
		if i < len(m.groups)-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// renderDeviceLine renders a single device as one compact line.
// Format: "<IP>  <port> <port> <port>  <MAC>  <status>"
// Each local port is colored by its tunnel status and wrapped in an
// OSC8 hyperlink when the remote port has a natural URL scheme.
func (m TunnelsModel) renderDeviceLine(g tunnelGroup) string {
	var b strings.Builder

	// Subnet IP, left-padded for alignment.
	b.WriteString(ActiveStyle.Render(fmt.Sprintf("%-15s", g.RemoteHost)))
	b.WriteString("  ")

	// Local ports, colored by status, clickable when web-ish.
	ports := make([]string, len(g.Tunnels))
	active, failed, total := 0, 0, len(g.Tunnels)
	for i, t := range g.Tunnels {
		ports[i] = renderPortToken(t)
		switch t.Status {
		case ssh.StatusActive:
			active++
		case ssh.StatusFailed:
			failed++
		}
	}
	b.WriteString(strings.Join(ports, " "))
	b.WriteString("  ")

	// MAC (blank if unknown, e.g. for the gateway auto-tunnel).
	mac := g.RemoteMAC
	if mac == "" {
		mac = strings.Repeat(" ", 17)
	}
	b.WriteString(DimStyle.Render(fmt.Sprintf("%-17s", mac)))
	b.WriteString("  ")

	// Per-device status summary.
	switch {
	case failed == 0 && active == total:
		b.WriteString(SuccessStyle.Render(fmt.Sprintf("[%d/%d up]", active, total)))
	case failed > 0 && active > 0:
		b.WriteString(WarningStyle.Render(fmt.Sprintf("[%d up, %d failed]", active, failed)))
	case failed == total:
		b.WriteString(ErrorStyle.Render(fmt.Sprintf("[%d failed]", failed)))
	default:
		b.WriteString(DimStyle.Render(fmt.Sprintf("[%d/%d]", active, total)))
	}

	return b.String()
}

// renderPortToken renders a single local-port token, colored by status and
// wrapped in an OSC8 hyperlink for HTTP/HTTPS remote ports.
func renderPortToken(t tunnelEntry) string {
	style := DimStyle
	switch t.Status {
	case ssh.StatusActive:
		style = SuccessStyle
	case ssh.StatusFailed:
		style = ErrorStyle
	case ssh.StatusConnecting:
		style = WarningStyle
	}

	text := fmt.Sprintf("%d", t.LocalPort)
	styled := style.Render(text)

	switch t.RemotePort {
	case 443, 8443:
		return components.Hyperlink(fmt.Sprintf("https://localhost:%d", t.LocalPort), styled)
	case 80, 8080, 8291:
		return components.Hyperlink(fmt.Sprintf("http://localhost:%d", t.LocalPort), styled)
	}
	return styled
}

// View renders the active tunnel dashboard.
func (m TunnelsModel) View() string {
	body := m.viewport.View()
	// Scroll hint when content overflows.
	if m.viewport.TotalLineCount() > m.viewport.VisibleLineCount() {
		pct := int(m.viewport.ScrollPercent() * 100)
		body += "\n" + DimStyle.Render(fmt.Sprintf("  -- %d%% --  scroll: ↑/↓ j/k pgup/pgdn", pct))
	}

	panel := renderPanel("Active Tunnels", body)

	if m.milestone != "" {
		panel += "\n" + SubtitleStyle.Render("  "+m.milestone)
	}

	// Status bar.
	activeCount, failedCount := m.statusCounts()
	uptime := fmt.Sprintf("UP %s", formatDuration(m.elapsed))
	summary := fmt.Sprintf("%d active", activeCount)
	if failedCount > 0 {
		summary += fmt.Sprintf(", %d failed", failedCount)
	}
	bar := renderStatusBar(uptime, summary, "q: disconnect", "r: reconnect")

	return ContentStyle.Render(panel + "\n" + bar)
}

// statusCounts returns the total active and failed tunnel counts across all groups.
func (m TunnelsModel) statusCounts() (active, failed int) {
	for _, g := range m.groups {
		for _, t := range g.Tunnels {
			switch t.Status {
			case ssh.StatusActive:
				active++
			case ssh.StatusFailed:
				failed++
			}
		}
	}
	return active, failed
}

// groupTunnels organizes tunnels by their remote host and attaches MAC info.
// Within each group tunnels are sorted by local port for stable display.
func groupTunnels(tunnels []*ssh.Tunnel, macByHost map[string]string) []tunnelGroup {
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
		entries := byHost[host]
		sort.Slice(entries, func(a, b int) bool {
			return entries[a].LocalPort < entries[b].LocalPort
		})
		groups[i] = tunnelGroup{
			RemoteHost: host,
			RemoteMAC:  macByHost[host],
			Tunnels:    entries,
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
