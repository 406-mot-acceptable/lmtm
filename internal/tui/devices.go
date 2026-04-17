package tui

import (
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/406-mot-acceptable/lmtm/internal/discovery"
	"github.com/406-mot-acceptable/lmtm/internal/gateway"
)

// devicesMode tracks the current input mode of the devices screen.
type devicesMode int

const (
	modeList   devicesMode = iota // Normal device list browsing
	modeSubnet                    // Subnet input for rescanning
	modeManual                    // Manual IP:Port entry
)

// PortPreset cycles through port assignment modes for a device.
type PortPreset int

const (
	PresetDefault PortPreset = iota // Use DeviceClass defaults
	PresetCamera                    // 22,80,443,554
	PresetRouter                    // 22,80,443
	PresetWeb                       // 80,443
)

func (p PortPreset) String() string {
	switch p {
	case PresetCamera:
		return "Camera"
	case PresetRouter:
		return "Router"
	case PresetWeb:
		return "Web"
	default:
		return "Default"
	}
}

// Ports returns the port list for this preset.
func (p PortPreset) Ports() []int {
	switch p {
	case PresetCamera:
		return []int{22, 80, 443, 554}
	case PresetRouter:
		return []int{22, 80, 443}
	case PresetWeb:
		return []int{80, 443}
	default:
		return nil // caller uses DeviceClass defaults
	}
}

// deviceEntry tracks selection and port override state per device.
type deviceEntry struct {
	Device   discovery.DiscoveredDevice
	Selected bool
	Preset   PortPreset
}

// effectivePorts returns the active port list for this entry.
func (e deviceEntry) effectivePorts() []int {
	if ports := e.Preset.Ports(); ports != nil {
		return ports
	}
	return e.Device.DefaultPorts
}

// DeviceSelectMsg is sent when the user confirms their device selection.
type DeviceSelectMsg struct {
	Devices []SelectedDevice
}

// SubnetScanRequestMsg is emitted when the user submits a subnet for scanning.
type SubnetScanRequestMsg struct {
	Subnet string
}

// SelectedDevice is a device chosen for tunneling with its port list.
type SelectedDevice struct {
	IP    string
	MAC   string
	Ports []int
}

// DevicesModel handles the device selection list.
type DevicesModel struct {
	entries    []deviceEntry
	cursor     int
	viewStart  int
	viewHeight int
	selKeys    SelectionKeys
	navKeys    NavigationKeys
	globals    GlobalKeys

	// Input mode state.
	mode        devicesMode
	subnetInput textinput.Model
	ipInput     textinput.Model
	portInput   textinput.Model
	manualFocus int    // 0=IP, 1=Port
	inputErr    string
}

// NewDevicesModel creates the device selection screen from scan results.
func NewDevicesModel(devices []discovery.DiscoveredDevice) DevicesModel {
	entries := make([]deviceEntry, len(devices))
	for i, d := range devices {
		entries[i] = deviceEntry{Device: d}
	}
	return DevicesModel{
		entries:     entries,
		viewHeight:  20,
		selKeys:     DefaultSelectionKeys,
		navKeys:     DefaultNavigationKeys,
		globals:     DefaultGlobalKeys,
		subnetInput: newSubnetInput(),
		ipInput:     newIPInput(),
		portInput:   newPortInput(),
	}
}

// NewDevicesModelFromEntries creates the device selection screen from existing
// entries, preserving selection state. Used after merging scan results.
func NewDevicesModelFromEntries(entries []deviceEntry) DevicesModel {
	return DevicesModel{
		entries:     entries,
		viewHeight:  20,
		selKeys:     DefaultSelectionKeys,
		navKeys:     DefaultNavigationKeys,
		globals:     DefaultGlobalKeys,
		subnetInput: newSubnetInput(),
		ipInput:     newIPInput(),
		portInput:   newPortInput(),
	}
}

// Entries returns a copy of the current device entries.
func (m DevicesModel) Entries() []deviceEntry {
	result := make([]deviceEntry, len(m.entries))
	copy(result, m.entries)
	return result
}

// SelectedDevices returns all selected devices with their effective ports.
func (m DevicesModel) SelectedDevices() []SelectedDevice {
	var result []SelectedDevice
	for _, e := range m.entries {
		if e.Selected {
			result = append(result, SelectedDevice{
				IP:    e.Device.IP,
				MAC:   e.Device.MAC,
				Ports: e.effectivePorts(),
			})
		}
	}
	return result
}

// Init does nothing for the device list.
func (m DevicesModel) Init() tea.Cmd {
	return nil
}

// Update handles input events for device selection.
func (m DevicesModel) Update(msg tea.Msg) (DevicesModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.mode {
		case modeSubnet:
			return m.updateSubnetMode(msg)
		case modeManual:
			return m.updateManualMode(msg)
		default:
			return m.updateListMode(msg)
		}
	}
	return m, nil
}

// updateListMode handles keys in normal device list mode.
func (m DevicesModel) updateListMode(msg tea.KeyMsg) (DevicesModel, tea.Cmd) {
	switch {
	case key.Matches(msg, m.navKeys.Up):
		if m.cursor > 0 {
			m.cursor--
			if m.cursor < m.viewStart {
				m.viewStart = m.cursor
			}
		}

	case key.Matches(msg, m.navKeys.Down):
		if m.cursor < len(m.entries)-1 {
			m.cursor++
			if m.cursor >= m.viewStart+m.viewHeight {
				m.viewStart = m.cursor - m.viewHeight + 1
			}
		}

	case key.Matches(msg, m.selKeys.Toggle):
		if len(m.entries) > 0 {
			m.entries[m.cursor].Selected = !m.entries[m.cursor].Selected
		}

	case key.Matches(msg, m.selKeys.All):
		for i := range m.entries {
			m.entries[i].Selected = true
		}

	case key.Matches(msg, m.selKeys.None):
		for i := range m.entries {
			m.entries[i].Selected = false
		}

	case key.Matches(msg, m.selKeys.FirstN):
		for i := range m.entries {
			m.entries[i].Selected = i < 10
		}

	case key.Matches(msg, key.NewBinding(key.WithKeys("p"))):
		// Cycle port preset on current device.
		if len(m.entries) > 0 {
			e := &m.entries[m.cursor]
			e.Preset = (e.Preset + 1) % 4
		}

	case key.Matches(msg, key.NewBinding(key.WithKeys("s"))):
		m.mode = modeSubnet
		m.inputErr = ""
		m.subnetInput.SetValue("")
		return m, m.subnetInput.Focus()

	case key.Matches(msg, key.NewBinding(key.WithKeys("+"))):
		m.mode = modeManual
		m.manualFocus = 0
		m.inputErr = ""
		m.ipInput.SetValue("")
		m.portInput.SetValue("")
		return m, m.ipInput.Focus()

	case key.Matches(msg, m.navKeys.Enter):
		selected := m.SelectedDevices()
		if len(selected) > 0 {
			return m, func() tea.Msg {
				return DeviceSelectMsg{Devices: selected}
			}
		}
	}

	return m, nil
}

// updateSubnetMode handles keys in subnet input mode.
func (m DevicesModel) updateSubnetMode(msg tea.KeyMsg) (DevicesModel, tea.Cmd) {
	switch {
	case key.Matches(msg, m.navKeys.Enter):
		subnet := strings.TrimSpace(m.subnetInput.Value())
		if err := gateway.ValidateSubnet(subnet); err != nil {
			m.inputErr = err.Error()
			return m, nil
		}
		m.mode = modeList
		m.inputErr = ""
		m.subnetInput.Blur()
		return m, func() tea.Msg {
			return SubnetScanRequestMsg{Subnet: subnet}
		}
	}

	// Forward to text input.
	var cmd tea.Cmd
	m.subnetInput, cmd = m.subnetInput.Update(msg)
	return m, cmd
}

// updateManualMode handles keys in manual IP:Port input mode.
func (m DevicesModel) updateManualMode(msg tea.KeyMsg) (DevicesModel, tea.Cmd) {
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("tab", "shift+tab"))):
		m.manualFocus = (m.manualFocus + 1) % 2
		if m.manualFocus == 0 {
			m.portInput.Blur()
			return m, m.ipInput.Focus()
		}
		m.ipInput.Blur()
		return m, m.portInput.Focus()

	case key.Matches(msg, m.navKeys.Enter):
		ip := strings.TrimSpace(m.ipInput.Value())
		portStr := strings.TrimSpace(m.portInput.Value())

		// Validate IP.
		if net.ParseIP(ip) == nil {
			m.inputErr = "invalid IP address"
			return m, nil
		}

		// Validate port.
		port, err := strconv.Atoi(portStr)
		if err != nil || port < 1 || port > 65535 {
			m.inputErr = "port must be 1-65535"
			return m, nil
		}

		// Check if IP already exists in the list.
		found := false
		for i, e := range m.entries {
			if e.Device.IP == ip {
				found = true
				if !hasDupePort(m.entries[i].Device.DefaultPorts, port) {
					m.entries[i].Device.DefaultPorts = append(m.entries[i].Device.DefaultPorts, port)
				}
				break
			}
		}

		if !found {
			m.entries = append(m.entries, deviceEntry{
				Device: discovery.DiscoveredDevice{
					IP:           ip,
					Vendor:       "Manual",
					DeviceType:   discovery.ClassUnknown,
					DefaultPorts: []int{port},
					Online:       true,
				},
				Selected: true,
			})
			sortEntriesByIP(m.entries)
			// Reset cursor to the newly added device.
			for i, e := range m.entries {
				if e.Device.IP == ip {
					m.cursor = i
					break
				}
			}
			if m.cursor >= m.viewStart+m.viewHeight {
				m.viewStart = m.cursor - m.viewHeight + 1
			} else if m.cursor < m.viewStart {
				m.viewStart = m.cursor
			}
		}

		m.mode = modeList
		m.inputErr = ""
		m.ipInput.Blur()
		m.portInput.Blur()
		return m, nil
	}

	// Forward to focused text input.
	var cmd tea.Cmd
	if m.manualFocus == 0 {
		m.ipInput, cmd = m.ipInput.Update(msg)
	} else {
		m.portInput, cmd = m.portInput.Update(msg)
	}
	return m, cmd
}

// View renders the device selection list.
func (m DevicesModel) View() string {
	var b strings.Builder

	if len(m.entries) == 0 {
		b.WriteString(DimStyle.Render("No devices found."))
	} else {
		// Column header.
		header := fmt.Sprintf("  %-3s %-16s %-14s %-18s %-10s %s",
			" ", "IP", "MAC", "Vendor", "Type", "Ports")
		b.WriteString(TableHeaderStyle.Render(header))
		b.WriteByte('\n')

		// Visible rows.
		end := m.viewStart + m.viewHeight
		if end > len(m.entries) {
			end = len(m.entries)
		}

		for i := m.viewStart; i < end; i++ {
			e := m.entries[i]
			b.WriteString(m.renderRow(i, e))
			b.WriteByte('\n')
		}

		// Scroll indicator.
		if len(m.entries) > m.viewHeight {
			b.WriteString(DimStyle.Render(fmt.Sprintf(
				"  [%d-%d of %d]", m.viewStart+1, end, len(m.entries))))
			b.WriteByte('\n')
		}
	}

	panel := renderPanel("Select Devices", b.String())

	// Input bar and status bar depend on mode.
	var bar string
	switch m.mode {
	case modeSubnet:
		bar = m.subnetBar()
	case modeManual:
		bar = m.manualBar()
	default:
		selCount, portCount := m.selectionCounts()
		summary := fmt.Sprintf("%d/%d devices, %d ports",
			selCount, len(m.entries), portCount)
		bar = renderStatusBar(summary, "Space: toggle", "a/n: all/none",
			"p: preset", "s: scan subnet", "+: add device", "Enter: build")
	}

	return ContentStyle.Render(panel + "\n" + bar)
}

// subnetBar renders the subnet input bar and status hints.
func (m DevicesModel) subnetBar() string {
	var b strings.Builder
	label := AccentStyle.Render("Subnet")
	b.WriteString("  " + label + " " + m.subnetInput.View())
	if m.inputErr != "" {
		b.WriteString("  " + ErrorStyle.Render(m.inputErr))
	}
	b.WriteByte('\n')
	b.WriteString(renderStatusBar("Enter: scan", "Esc: cancel"))
	return b.String()
}

// manualBar renders the manual IP:Port input bar and status hints.
func (m DevicesModel) manualBar() string {
	var b strings.Builder
	ipLabel := AccentStyle.Render("IP")
	portLabel := AccentStyle.Render("Port")
	b.WriteString("  " + ipLabel + " " + m.ipInput.View())
	b.WriteString("   " + portLabel + " " + m.portInput.View())
	if m.inputErr != "" {
		b.WriteString("  " + ErrorStyle.Render(m.inputErr))
	}
	b.WriteByte('\n')
	b.WriteString(renderStatusBar("Tab: next field", "Enter: add", "Esc: cancel"))
	return b.String()
}

// renderRow renders a single device row.
func (m DevicesModel) renderRow(idx int, e deviceEntry) string {
	check := "[ ]"
	if e.Selected {
		check = "[x]"
	}

	// Truncate MAC to first 8 chars (vendor prefix).
	mac := e.Device.MAC
	if len(mac) > 8 {
		mac = mac[:8] + "..."
	}

	// Truncate vendor.
	vendor := e.Device.Vendor
	if len(vendor) > 16 {
		vendor = vendor[:16] + ".."
	}

	ports := formatPorts(e.effectivePorts())

	line := fmt.Sprintf("%s %-16s %-14s %-18s %-10s %s",
		check, e.Device.IP, mac, vendor, e.Device.DeviceType, ports)

	switch {
	case idx == m.cursor && e.Selected:
		return SelectedStyle.Render("> " + line)
	case idx == m.cursor:
		return ActiveStyle.Render("> " + line)
	case e.Selected:
		return SuccessStyle.Render("  " + line)
	default:
		return "  " + line
	}
}

// selectionCounts returns the number of selected devices and total ports.
func (m DevicesModel) selectionCounts() (int, int) {
	var devices, ports int
	for _, e := range m.entries {
		if e.Selected {
			devices++
			ports += len(e.effectivePorts())
		}
	}
	return devices, ports
}

// formatPorts renders a port list compactly.
func formatPorts(ports []int) string {
	strs := make([]string, len(ports))
	for i, p := range ports {
		strs[i] = fmt.Sprintf("%d", p)
	}
	return strings.Join(strs, ",")
}

// mergeEntries combines previous entries with newly scanned devices.
// Previous entries keep their selection state. New devices not already
// in the list are appended. The result is sorted by last IP octet.
func mergeEntries(previous []deviceEntry, newDevices []discovery.DiscoveredDevice) []deviceEntry {
	existing := make(map[string]bool, len(previous))
	for _, e := range previous {
		existing[e.Device.IP] = true
	}

	merged := make([]deviceEntry, len(previous))
	copy(merged, previous)

	for _, d := range newDevices {
		if !existing[d.IP] {
			merged = append(merged, deviceEntry{Device: d})
		}
	}

	sortEntriesByIP(merged)
	return merged
}

// --- helpers ---

func newSubnetInput() textinput.Model {
	ti := textinput.New()
	ti.Placeholder = "10.0.0"
	ti.CharLimit = 11 // "255.255.255"
	ti.Width = 15
	return ti
}

func newIPInput() textinput.Model {
	ti := textinput.New()
	ti.Placeholder = "192.168.1.100"
	ti.CharLimit = 15
	ti.Width = 18
	return ti
}

func newPortInput() textinput.Model {
	ti := textinput.New()
	ti.Placeholder = "443"
	ti.CharLimit = 5
	ti.Width = 8
	return ti
}

func sortEntriesByIP(entries []deviceEntry) {
	sort.Slice(entries, func(i, j int) bool {
		return lastOctet(entries[i].Device.IP) < lastOctet(entries[j].Device.IP)
	})
}

func lastOctet(ip string) int {
	parts := strings.Split(ip, ".")
	if len(parts) != 4 {
		return 0
	}
	n, _ := strconv.Atoi(parts[3])
	return n
}

func hasDupePort(ports []int, port int) bool {
	for _, p := range ports {
		if p == port {
			return true
		}
	}
	return false
}
