package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// ScanRequestMsg is sent when the user presses Enter to start scanning.
type ScanRequestMsg struct{}

// WANConfig holds WAN interface details for display.
type WANConfig struct {
	Interface string
	PublicIP  string
	Gateway   string
}

// LANConfig holds LAN interface details for display.
type LANConfig struct {
	Interface string
	Subnet    string
	Gateway   string
	DHCPStart string
	DHCPEnd   string
}

// SurveyModel displays the network survey results.
type SurveyModel struct {
	gateway     string
	gatewayType string
	hostname    string
	wan         *WANConfig
	lan         *LANConfig
	keys        NavigationKeys
	globals     GlobalKeys
}

// NewSurveyModel creates the survey display screen.
func NewSurveyModel(gateway, gatewayType, hostname string, wan *WANConfig, lan *LANConfig) SurveyModel {
	return SurveyModel{
		gateway:     gateway,
		gatewayType: gatewayType,
		hostname:    hostname,
		wan:         wan,
		lan:         lan,
		keys:        DefaultNavigationKeys,
		globals:     DefaultGlobalKeys,
	}
}

// Init does nothing for the survey screen.
func (m SurveyModel) Init() tea.Cmd {
	return nil
}

// Update handles key events on the survey screen.
func (m SurveyModel) Update(msg tea.Msg) (SurveyModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Enter):
			return m, func() tea.Msg { return ScanRequestMsg{} }
		}
	}
	return m, nil
}

// View renders the tree-style network survey display.
func (m SurveyModel) View() string {
	var b strings.Builder

	// Gateway summary line.
	gwInfo := ActiveStyle.Render(m.gateway)
	gwInfo += DimStyle.Render(fmt.Sprintf(" (%s", m.gatewayType))
	if m.hostname != "" {
		gwInfo += DimStyle.Render(fmt.Sprintf(" - %q", m.hostname))
	}
	gwInfo += DimStyle.Render(")")
	b.WriteString(LabelStyle.Render("Gateway"))
	b.WriteString(gwInfo)
	b.WriteString("\n\n")

	// WAN section in inner panel.
	var wan strings.Builder
	if m.wan != nil {
		wan.WriteString(m.treeLine(false, "Interface", m.wan.Interface))
		wan.WriteString(m.treeLine(false, "Public IP", m.wan.PublicIP))
		wan.WriteString(m.treeLine(true, "Gateway", m.wan.Gateway))
	} else {
		wan.WriteString(m.treeLine(true, "Status", "not available"))
	}
	b.WriteString(InnerPanelStyle.Render(
		ActiveStyle.Render("WAN") + "\n" + wan.String(),
	))
	b.WriteByte('\n')

	// LAN section in inner panel.
	var lan strings.Builder
	if m.lan != nil {
		lan.WriteString(m.treeLine(false, "Interface", m.lan.Interface))
		lan.WriteString(m.treeLine(false, "Subnet", m.lan.Subnet))
		lan.WriteString(m.treeLine(false, "Gateway", m.lan.Gateway))
		dhcp := m.lan.DHCPStart + " - " + m.lan.DHCPEnd
		lan.WriteString(m.treeLine(true, "DHCP Pool", dhcp))
	} else {
		lan.WriteString(m.treeLine(true, "Status", "not available"))
	}
	b.WriteString(InnerPanelStyle.Render(
		ActiveStyle.Render("LAN") + "\n" + lan.String(),
	))

	panel := renderPanel("Network Survey", b.String())

	// Status bar.
	bar := renderStatusBar("Enter: scan network", "Esc: disconnect")

	return ContentStyle.Render(panel + "\n" + bar)
}

// treeLine renders a single tree line with the box-drawing connector.
func (m SurveyModel) treeLine(last bool, label, value string) string {
	connector := "├─ "
	if last {
		connector = "└─ "
	}
	return DimStyle.Render(connector) +
		LabelStyle.Render(fmt.Sprintf("%-12s", label)) +
		value + "\n"
}
