package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jaco/tunneler/internal/browser"
	"github.com/jaco/tunneler/internal/config"
	"github.com/jaco/tunneler/internal/scanner"
	"github.com/jaco/tunneler/internal/ssh"
)

type Model struct {
	config        *config.Config
	manager       *ssh.Manager
	browserOpener *browser.Opener
	logger        *Logger

	siteList       list.Model
	tunnelTable    table.Model
	passwordInput  textinput.Model
	presetSelector PresetSelectorModel
	deviceSelector DeviceSelectorModel

	mode           string // "list", "preset", "password", "tunnels", "custom_range", "scanning", "device_selection"
	selectedSite   *config.Site
	selectedPreset *config.Preset
	scanResults    []config.Device // Devices from scan to tunnel
	currentPassword string          // Temporarily store password for scanning
	status         string
	err            error
	width          int
	height         int
	showDebug      bool // Toggle debug view with 'l' key
}

type siteItem struct {
	site config.Site
}

func (s siteItem) FilterValue() string { return s.site.Name }
func (s siteItem) Title() string {
	prefix := ""
	if s.site.Favorite {
		prefix = "★ "
	}
	return prefix + s.site.Name
}
func (s siteItem) Description() string {
	return fmt.Sprintf("%s (%s) - %s", s.site.Gateway, s.site.GetUsername(config.Defaults{}), s.site.Type)
}

// Messages
type tunnelStatusMsg struct {
	info *ssh.TunnelInfo
}

type connectCompleteMsg struct {
	err error
}

type scanCompleteMsg struct {
	devices []scanner.DiscoveredDevice
	err     error
}

func NewModel(cfg *config.Config) Model {
	// Create site list
	items := make([]list.Item, 0, len(cfg.Sites))
	sites := cfg.GetSitesByFavorite()
	for _, site := range sites {
		items = append(items, siteItem{site: site})
	}

	delegate := list.NewDefaultDelegate()
	delegate.SetHeight(2)
	siteList := list.New(items, delegate, 0, 0)
	siteList.Title = "Customer Sites"
	siteList.SetShowStatusBar(false)
	siteList.SetFilteringEnabled(true)

	// Create tunnel table
	columns := []table.Column{
		{Title: "Site", Width: 20},
		{Title: "Device", Width: 25},
		{Title: "Remote", Width: 20},
		{Title: "Local", Width: 15},
		{Title: "Status", Width: 12},
	}
	tunnelTable := table.New(
		table.WithColumns(columns),
		table.WithFocused(false),
		table.WithHeight(10),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	tunnelTable.SetStyles(s)

	// Create password input
	pwInput := textinput.New()
	pwInput.Placeholder = "Password"
	pwInput.EchoMode = textinput.EchoPassword
	pwInput.EchoCharacter = '•'

	logger := NewLogger(100) // Keep last 100 log entries

	return Model{
		config:         cfg,
		manager:        ssh.NewManager(),
		browserOpener:  browser.NewOpener(),
		logger:         logger,
		siteList:       siteList,
		tunnelTable:    tunnelTable,
		passwordInput:  pwInput,
		presetSelector: NewPresetSelector(cfg),
		mode:           "list",
		status:         "Select a site to connect",
		showDebug:      false,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.siteList.SetSize(msg.Width/2-4, msg.Height-6)
		return m, nil

	case tea.KeyMsg:
		switch m.mode {
		case "list":
			return m.updateListMode(msg)
		case "preset":
			return m.updatePresetMode(msg)
		case "custom_range":
			return m.updateCustomRangeMode(msg)
		case "password":
			return m.updatePasswordMode(msg)
		case "device_selection":
			return m.updateDeviceSelectionMode(msg)
		case "tunnels":
			return m.updateTunnelsMode(msg)
		}

	case tunnelStatusMsg:
		return m.handleTunnelStatus(msg), nil

	case scanCompleteMsg:
		if msg.err != nil {
			m.logger.Error("Scan failed: %v", msg.err)
			m.status = fmt.Sprintf("Scan error: %v", msg.err)
			m.mode = "preset"
		} else {
			m.logger.Info("Scan complete - found %d devices", len(msg.devices))
			m.deviceSelector = NewDeviceSelector(msg.devices)
			m.mode = "device_selection"
			m.status = "Select devices to tunnel (space: toggle, enter: connect)"
		}
		return m, nil

	case connectCompleteMsg:
		if msg.err != nil {
			m.logger.Error("connectCompleteMsg received with error: %v", msg.err)
			m.status = fmt.Sprintf("Error: %v", msg.err)
			m.mode = "list"
		} else {
			m.logger.Info("connectCompleteMsg received - switching to tunnels view")
			m.status = fmt.Sprintf("Connected to %s", m.selectedSite.Name)
			m.mode = "tunnels"

			// Immediately update tunnel table
			m = m.handleTunnelStatus(tunnelStatusMsg{info: nil})

			// Auto-open browser if preset has browser_tabs enabled
			if m.selectedPreset != nil && m.selectedPreset.BrowserTabs {
				m.logger.Info("Auto-opening browser tabs (browser_tabs=true)")
				tunnels := m.manager.GetAllTunnels()
				protocol := m.selectedPreset.Protocol
				if err := m.browserOpener.OpenTunnels(tunnels, protocol); err == nil {
					m.status += " (opened browser tabs)"
					m.logger.Info("Browser tabs opened successfully")
				} else {
					m.logger.Error("Failed to open browser tabs: %v", err)
				}
			}
		}
		return m, nil
	}

	var cmd tea.Cmd
	switch m.mode {
	case "list":
		m.siteList, cmd = m.siteList.Update(msg)
	case "password":
		m.passwordInput, cmd = m.passwordInput.Update(msg)
	case "tunnels":
		m.tunnelTable, cmd = m.tunnelTable.Update(msg)
	case "device_selection":
		if m.deviceSelector.editingPort {
			m.deviceSelector.portInput, cmd = m.deviceSelector.portInput.Update(msg)
		}
	}

	return m, cmd
}

func (m Model) updateListMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		m.manager.DisconnectAll()
		return m, tea.Quit

	case "enter":
		// Get selected site
		if item, ok := m.siteList.SelectedItem().(siteItem); ok {
			m.selectedSite = &item.site
			// Check if presets are available
			if len(m.config.Presets) > 0 {
				m.mode = "preset"
				m.presetSelector = NewPresetSelector(m.config)
				m.status = "Select a preset or custom range"
			} else {
				// No presets, go directly to password
				m.mode = "password"
				m.passwordInput.Focus()
				m.status = fmt.Sprintf("Enter password for %s@%s",
					m.selectedSite.GetUsername(m.config.Defaults),
					m.selectedSite.Gateway)
			}
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.siteList, cmd = m.siteList.Update(msg)
	return m, cmd
}

func (m Model) updatePresetMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = "list"
		m.status = "Select a site to connect"
		return m, nil

	case "up", "k":
		if m.presetSelector.cursor > 0 {
			m.presetSelector.cursor--
		}
		return m, nil

	case "down", "j":
		maxIdx := len(m.presetSelector.presetKeys) // +1 for custom option
		if m.presetSelector.cursor < maxIdx {
			m.presetSelector.cursor++
		}
		return m, nil

	case "c":
		// Custom range
		m.mode = "custom_range"
		m.presetSelector.customRange = true
		m.presetSelector.rangeStart = "2"
		m.presetSelector.rangeEnd = "11"
		m.presetSelector.rangeInputIdx = 0
		m.status = "Enter custom range"
		return m, nil

	case "enter", "1", "2", "3", "4", "5", "6", "7", "8", "9":
		// Select preset by number or enter
		selectedIdx := m.presetSelector.cursor
		if msg.String() != "enter" {
			// Number key pressed
			num := int(msg.String()[0] - '0')
			if num > 0 && num <= len(m.presetSelector.presetKeys) {
				selectedIdx = num - 1
			}
		}

		// Check if custom option selected
		if selectedIdx == len(m.presetSelector.presetKeys) {
			m.mode = "custom_range"
			m.presetSelector.customRange = true
			m.presetSelector.rangeStart = "2"
			m.presetSelector.rangeEnd = "11"
			m.presetSelector.rangeInputIdx = 0
			m.status = "Enter custom range"
			return m, nil
		}

		// Select preset
		if selectedIdx >= 0 && selectedIdx < len(m.presetSelector.presetKeys) {
			key := m.presetSelector.presetKeys[selectedIdx]
			m.presetSelector.selectedKey = key
			m.selectedPreset = m.config.GetPreset(key)

			// Go to password
			m.mode = "password"
			m.passwordInput.Focus()
			m.status = fmt.Sprintf("Enter password for %s@%s (preset: %s)",
				m.selectedSite.GetUsername(m.config.Defaults),
				m.selectedSite.Gateway,
				m.selectedPreset.Name)
		}
		return m, nil
	}

	return m, nil
}

func (m Model) updateCustomRangeMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = "preset"
		m.status = "Select a preset or custom range"
		return m, nil

	case "tab", "down":
		// Switch input field
		m.presetSelector.rangeInputIdx = (m.presetSelector.rangeInputIdx + 1) % 2
		return m, nil

	case "up":
		// Switch input field backwards
		m.presetSelector.rangeInputIdx = (m.presetSelector.rangeInputIdx + 1) % 2
		return m, nil

	case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9":
		// Add digit to current input
		if m.presetSelector.rangeInputIdx == 0 {
			if len(m.presetSelector.rangeStart) < 3 {
				m.presetSelector.rangeStart += msg.String()
			}
		} else {
			if len(m.presetSelector.rangeEnd) < 3 {
				m.presetSelector.rangeEnd += msg.String()
			}
		}
		return m, nil

	case "backspace":
		// Remove last digit
		if m.presetSelector.rangeInputIdx == 0 {
			if len(m.presetSelector.rangeStart) > 0 {
				m.presetSelector.rangeStart = m.presetSelector.rangeStart[:len(m.presetSelector.rangeStart)-1]
			}
		} else {
			if len(m.presetSelector.rangeEnd) > 0 {
				m.presetSelector.rangeEnd = m.presetSelector.rangeEnd[:len(m.presetSelector.rangeEnd)-1]
			}
		}
		return m, nil

	case "enter":
		// Validate and go to password
		start, end := m.presetSelector.GetCustomRange()
		if start >= 1 && end <= 254 && start <= end {
			m.mode = "password"
			m.passwordInput.Focus()
			m.status = fmt.Sprintf("Enter password for %s@%s (range: %d-%d)",
				m.selectedSite.GetUsername(m.config.Defaults),
				m.selectedSite.Gateway,
				start, end)
		} else {
			m.status = "Invalid range (must be 1-254, start <= end)"
		}
		return m, nil
	}

	return m, nil
}

func (m Model) updatePasswordMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = "list"
		m.passwordInput.SetValue("")
		m.status = "Select a site to connect"
		return m, nil

	case "enter":
		password := m.passwordInput.Value()
		m.currentPassword = password
		m.manager.SetPassword(password)
		m.passwordInput.SetValue("")

		// Check if this is a scan preset
		if m.selectedPreset != nil && m.selectedPreset.IsScanPreset() {
			m.mode = "scanning"
			m.status = "Scanning network..."
			m.logger.Info("Starting network scan with method: %s", m.selectedPreset.GetScanMethod())
			return m, m.scanNetwork()
		} else {
			m.mode = "list"
			m.status = "Connecting..."
			return m, m.connectToSite()
		}
	}

	var cmd tea.Cmd
	m.passwordInput, cmd = m.passwordInput.Update(msg)
	return m, cmd
}

func (m Model) updateDeviceSelectionMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle port editing mode
	if m.deviceSelector.editingPort {
		switch msg.String() {
		case "enter":
			m.deviceSelector.FinishPortEdit()
			m.status = "Port updated"
			return m, nil
		case "esc":
			m.deviceSelector.CancelPortEdit()
			m.status = "Port edit cancelled"
			return m, nil
		}
		// Let textinput handle other keys
		return m, nil
	}

	// Normal selection mode
	switch msg.String() {
	case "esc":
		m.mode = "preset"
		m.status = "Select a preset or custom range"
		return m, nil

	case "up", "k":
		m.deviceSelector.MoveCursor(-1)
		return m, nil

	case "down", "j":
		m.deviceSelector.MoveCursor(1)
		return m, nil

	case " ": // Space bar
		m.deviceSelector.ToggleSelection()
		return m, nil

	case "a":
		m.deviceSelector.SelectAll()
		m.status = "All devices selected"
		return m, nil

	case "n":
		m.deviceSelector.SelectNone()
		m.status = "All devices deselected"
		return m, nil

	case "p":
		m.deviceSelector.StartPortEdit()
		m.status = "Editing port (enter to save, esc to cancel)"
		return m, nil

	case "enter":
		// Get selected devices and connect
		// Note: For multi-subnet scans, we don't actually use the subnet parameter
		// as the full IP is already in the discovered device
		m.scanResults = m.deviceSelector.GetSelectedDevices(m.selectedSite.GetSubnet(m.config.Defaults))
		if len(m.scanResults) == 0 {
			m.status = "No devices selected!"
			return m, nil
		}

		m.logger.Info("Creating tunnels for %d selected devices", len(m.scanResults))
		m.mode = "list"
		m.status = "Connecting..."
		return m, m.connectToSiteWithDevices(m.scanResults)
	}

	return m, nil
}

func (m Model) updateTunnelsMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.logger.Info("Disconnecting all tunnels (user requested)")
		m.manager.DisconnectAll()
		m.mode = "list"
		m.status = "Disconnected"
		m.tunnelTable.SetRows([]table.Row{})
		return m, nil

	case "d":
		m.logger.Info("Disconnecting all tunnels (user requested)")
		m.manager.DisconnectAll()
		m.mode = "list"
		m.status = "Disconnected all tunnels"
		m.tunnelTable.SetRows([]table.Row{})
		return m, nil

	case "l":
		// Toggle debug log view
		m.showDebug = !m.showDebug
		if m.showDebug {
			m.status = "Debug view enabled (press 'l' to hide)"
		} else {
			m.status = "Debug view disabled"
		}
		return m, nil

	case "b":
		m.logger.Info("Opening browser tabs for active tunnels")
		// Open browser tabs for all active tunnels
		tunnels := m.manager.GetAllTunnels()
		protocol := ""
		if m.selectedPreset != nil {
			protocol = m.selectedPreset.Protocol
		}

		if err := m.browserOpener.OpenTunnels(tunnels, protocol); err != nil {
			m.logger.Error("Browser error: %v", err)
			m.status = fmt.Sprintf("Browser error: %v", err)
		} else {
			browserCmd := m.browserOpener.GetBrowserCommand()
			m.logger.Info("Opened %d tabs in %s", countActiveTunnels(tunnels), browserCmd)
			m.status = fmt.Sprintf("Opened %d tabs in %s", countActiveTunnels(tunnels), browserCmd)
		}
		return m, nil
	}

	return m, nil
}

// countActiveTunnels counts active tunnels across all sites
func countActiveTunnels(tunnels map[string][]*ssh.TunnelInfo) int {
	count := 0
	for _, siteTunnels := range tunnels {
		for _, tunnel := range siteTunnels {
			if tunnel.Status == ssh.StatusActive {
				count++
			}
		}
	}
	return count
}

func (m Model) connectToSite() tea.Cmd {
	return func() tea.Msg {
		var devices []config.Device

		m.logger.Info("=== Starting connection to %s ===", m.selectedSite.Name)
		m.logger.Debug("Site gateway: %s, type: %s, username: %s",
			m.selectedSite.Gateway,
			m.selectedSite.Type,
			m.selectedSite.GetUsername(m.config.Defaults))

		// Use preset if selected
		if m.selectedPreset != nil {
			m.logger.Info("Using preset: %s", m.selectedPreset.Name)
			m.logger.Debug("Preset config: range=%v, devices=%v, ports=%v",
				m.selectedPreset.Range,
				m.selectedPreset.Devices,
				m.selectedPreset.Ports)
			devices = m.selectedPreset.ApplyPreset(m.config.Defaults.Subnet)
		} else if m.presetSelector.IsCustomRange() {
			// Custom range
			start, end := m.presetSelector.GetCustomRange()
			m.logger.Info("Using custom range: %d-%d", start, end)
			devices = m.selectedSite.GenerateDevices(m.config.Defaults.Subnet, start, end)
		} else {
			// Default fallback
			m.logger.Info("Using default range: 2-11")
			devices = m.selectedSite.GenerateDevices(m.config.Defaults.Subnet, 2, 11)
		}

		m.logger.Info("Generated %d device tunnels", len(devices))
		for i, dev := range devices {
			m.logger.Debug("Device %d: %s:%d -> localhost:%d (%s)",
				i+1, dev.IP, dev.Port, dev.LocalPort, dev.Name)
		}

		// No status callback - we'll poll tunnel status instead
		username := m.selectedSite.GetUsername(m.config.Defaults)
		m.logger.Info("Connecting to SSH gateway with username: %s", username)
		m.logger.Debug("Gateway: %s:22, Type: %s", m.selectedSite.Gateway, m.selectedSite.Type)

		err := m.manager.ConnectSite(m.selectedSite, devices, m.config.Defaults, nil)

		if err != nil {
			m.logger.Error("Connection failed: %v", err)
			return connectCompleteMsg{err: err}
		} else {
			m.logger.Info("SSH connection established successfully")
			tunnelCount := len(m.manager.GetAllTunnels()[m.selectedSite.Name])
			m.logger.Info("Created %d tunnels", tunnelCount)
		}

		m.logger.Info("=== Connection complete ===")
		return connectCompleteMsg{err: nil}
	}
}

func (m Model) handleTunnelStatus(msg tunnelStatusMsg) Model {
	// Update tunnel table
	allTunnels := m.manager.GetAllTunnels()
	rows := make([]table.Row, 0)

	for siteName, tunnels := range allTunnels {
		for _, tunnel := range tunnels {
			symbol := getStatusSymbol(tunnel.Status)
			rows = append(rows, table.Row{
				siteName,
				tunnel.DeviceName,
				fmt.Sprintf("%s:%d", tunnel.DeviceIP, tunnel.DevicePort),
				fmt.Sprintf("localhost:%d", tunnel.LocalPort),
				fmt.Sprintf("%s %s", symbol, tunnel.Status),
			})
		}
	}

	m.tunnelTable.SetRows(rows)
	return m
}

func (m Model) scanNetwork() tea.Cmd {
	return func() tea.Msg {
		m.logger.Info("=== Starting network scan ===")

		// First, connect to the gateway to run scan commands
		m.logger.Info("Connecting to gateway for scanning...")

		// Create a temporary tunnel just for scanning (no port forwards)
		siteTunnel := ssh.NewSiteTunnel(
			m.selectedSite.Name,
			m.selectedSite.Gateway,
			m.selectedSite.GetUsername(m.config.Defaults),
			m.currentPassword,
			m.selectedSite.GetSSHOptions(),
		)

		// Connect without any devices (just SSH connection)
		if err := siteTunnel.Connect([]config.Device{}); err != nil {
			m.logger.Error("Failed to connect to gateway: %v", err)
			return scanCompleteMsg{err: err}
		}
		defer siteTunnel.Disconnect()

		m.logger.Info("Connected to gateway, starting scan...")

		// Get subnets to scan (may be multiple for preset, or site-specific override)
		siteSubnet := m.selectedSite.GetSubnet(m.config.Defaults)
		subnetsToScan := m.selectedPreset.GetScanSubnets(siteSubnet)

		m.logger.Info("Scanning %d subnet(s): %v", len(subnetsToScan), subnetsToScan)

		// Perform scan on each subnet
		scanMethod := scanner.ScanMethod(m.selectedPreset.GetScanMethod())
		scanPorts := m.selectedPreset.GetScanPorts()

		m.logger.Info("Scanning with method=%s, ports=%v", scanMethod, scanPorts)

		allDevices := make([]scanner.DiscoveredDevice, 0)

		for _, subnet := range subnetsToScan {
			m.logger.Info("Scanning subnet: %s.0/24", subnet)

			// Create scanner for this subnet
			scan := scanner.NewScanner(siteTunnel, subnet, m.selectedSite.Type)

			devices, err := scan.ScanNetwork(scanMethod, scanPorts)
			if err != nil {
				m.logger.Warning("Scan failed for subnet %s: %v", subnet, err)
				continue
			}

			m.logger.Info("Found %d devices on subnet %s", len(devices), subnet)
			allDevices = append(allDevices, devices...)
		}

		m.logger.Info("Scan complete: found %d total devices across all subnets", len(allDevices))
		for i, dev := range allDevices {
			m.logger.Debug("Device %d: %s (%s) - ports: %v", i+1, dev.IP, dev.DeviceType, dev.OpenPorts)
		}

		return scanCompleteMsg{devices: allDevices, err: nil}
	}
}

func (m Model) connectToSiteWithDevices(devices []config.Device) tea.Cmd {
	return func() tea.Msg {
		m.logger.Info("=== Connecting with %d devices ===", len(devices))

		for i, dev := range devices {
			m.logger.Debug("Device %d: %s:%d -> localhost:%d",
				i+1, dev.IP, dev.Port, dev.LocalPort)
		}

		username := m.selectedSite.GetUsername(m.config.Defaults)
		m.logger.Info("Connecting to SSH gateway with username: %s", username)
		m.logger.Debug("Gateway: %s:22, Type: %s", m.selectedSite.Gateway, m.selectedSite.Type)

		err := m.manager.ConnectSite(m.selectedSite, devices, m.config.Defaults, nil)

		if err != nil {
			m.logger.Error("Connection failed: %v", err)
			return connectCompleteMsg{err: err}
		} else {
			m.logger.Info("SSH connection established successfully")
			tunnelCount := len(m.manager.GetAllTunnels()[m.selectedSite.Name])
			m.logger.Info("Created %d tunnels", tunnelCount)
		}

		m.logger.Info("=== Connection complete ===")
		return connectCompleteMsg{err: nil}
	}
}

func (m Model) View() string {
	var b strings.Builder

	// Title
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	b.WriteString(titleStyle.Render("The Tunneler (Go Edition)"))
	b.WriteString("\n\n")

	switch m.mode {
	case "list":
		b.WriteString(m.siteList.View())
	case "preset":
		b.WriteString(m.presetSelector.View())
	case "custom_range":
		b.WriteString(m.presetSelector.CustomRangeView())
	case "password":
		b.WriteString("Enter password:\n\n")
		b.WriteString(m.passwordInput.View())
	case "scanning":
		scanStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
		b.WriteString(scanStyle.Render("Scanning network...\n\n"))
		b.WriteString("This may take a few seconds. Please wait.")
	case "device_selection":
		b.WriteString(m.deviceSelector.View())
	case "tunnels":
		if m.showDebug {
			// Split view: tunnels on left, logs on right
			tunnelView := m.renderTunnelsView()
			logView := m.renderLogView()

			// Simple side-by-side layout
			leftWidth := m.width / 2
			rightWidth := m.width - leftWidth

			// Add borders
			leftStyle := lipgloss.NewStyle().Width(leftWidth - 2).Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("240"))
			rightStyle := lipgloss.NewStyle().Width(rightWidth - 2).Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("240"))

			b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, leftStyle.Render(tunnelView), rightStyle.Render(logView)))
		} else {
			// Full width tunnel view
			b.WriteString("Active Tunnels\n\n")
			b.WriteString(m.tunnelTable.View())
		}
	}

	// Status bar
	b.WriteString("\n\n")
	statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	b.WriteString(statusStyle.Render(m.status))

	// Help
	b.WriteString("\n\n")
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	switch m.mode {
	case "list":
		b.WriteString(helpStyle.Render("↑/↓: navigate • enter: connect • /: filter • q: quit"))
	case "password":
		b.WriteString(helpStyle.Render("enter: connect • esc: cancel"))
	case "device_selection":
		b.WriteString(helpStyle.Render(m.deviceSelector.HelpView()))
	case "tunnels":
		if m.showDebug {
			b.WriteString(helpStyle.Render("l: hide logs • b: open browser • d: disconnect all • esc/q: back"))
		} else {
			b.WriteString(helpStyle.Render("l: show logs • b: open browser • d: disconnect all • esc/q: back"))
		}
	}

	return b.String()
}

func getStatusSymbol(status ssh.TunnelStatus) string {
	switch status {
	case ssh.StatusActive:
		return "✓"
	case ssh.StatusConnecting:
		return "⋯"
	case ssh.StatusFailed:
		return "✗"
	case ssh.StatusDisconnected:
		return "○"
	default:
		return "?"
	}
}

func (m Model) renderTunnelsView() string {
	var b strings.Builder
	b.WriteString("Active Tunnels\n\n")
	b.WriteString(m.tunnelTable.View())
	return b.String()
}

func (m Model) renderLogView() string {
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	b.WriteString(titleStyle.Render("Debug Logs"))
	b.WriteString("\n\n")

	entries := m.logger.GetRecent(20) // Show last 20 entries
	if len(entries) == 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("No logs yet..."))
		return b.String()
	}

	for _, entry := range entries {
		// Format: [HH:MM:SS] LEVEL message
		timestamp := entry.Time.Format("15:04:05")
		levelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(entry.Level.Color())).Bold(true)
		level := levelStyle.Render(fmt.Sprintf("%-5s", entry.Level.String()))

		b.WriteString(fmt.Sprintf("[%s] %s %s\n", timestamp, level, entry.Message))
	}

	return b.String()
}
