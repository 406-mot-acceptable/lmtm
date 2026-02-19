package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/406-mot-acceptable/lmtm/internal/discovery"
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
}

// NewDevicesModel creates the device selection screen from scan results.
func NewDevicesModel(devices []discovery.DiscoveredDevice) DevicesModel {
	entries := make([]deviceEntry, len(devices))
	for i, d := range devices {
		entries[i] = deviceEntry{Device: d}
	}
	return DevicesModel{
		entries:    entries,
		viewHeight: 20,
		selKeys:    DefaultSelectionKeys,
		navKeys:    DefaultNavigationKeys,
		globals:    DefaultGlobalKeys,
	}
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

		case key.Matches(msg, m.navKeys.Enter):
			selected := m.SelectedDevices()
			if len(selected) > 0 {
				return m, func() tea.Msg {
					return DeviceSelectMsg{Devices: selected}
				}
			}
		}
	}

	return m, nil
}

// View renders the device selection list.
func (m DevicesModel) View() string {
	var b strings.Builder

	if len(m.entries) == 0 {
		b.WriteString(DimStyle.Render("No devices found."))
		return ContentStyle.Render(renderPanel("Select Devices", b.String()))
	}

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

	panel := renderPanel("Select Devices", b.String())

	// Status bar with selection summary and key hints.
	selCount, portCount := m.selectionCounts()
	summary := fmt.Sprintf("%d/%d devices, %d ports",
		selCount, len(m.entries), portCount)
	bar := renderStatusBar(summary, "Space: toggle", "a/n: all/none", "p: preset", "Enter: build")

	return ContentStyle.Render(panel + "\n" + bar)
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
