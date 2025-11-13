package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"

	"github.com/jaco/tunneler/internal/config"
	"github.com/jaco/tunneler/internal/scanner"
)

// DeviceSelectorModel handles device selection UI
type DeviceSelectorModel struct {
	devices       []scanner.DiscoveredDevice
	selected      map[int]bool // index -> selected
	customPorts   map[int]int  // index -> custom port
	cursor        int
	editingPort   bool
	portInput     textinput.Model
	editIndex     int
}

// NewDeviceSelector creates a new device selector
func NewDeviceSelector(devices []scanner.DiscoveredDevice) DeviceSelectorModel {
	portInput := textinput.New()
	portInput.Placeholder = "Port"
	portInput.CharLimit = 5
	portInput.Width = 10

	return DeviceSelectorModel{
		devices:     devices,
		selected:    make(map[int]bool),
		customPorts: make(map[int]int),
		cursor:      0,
		portInput:   portInput,
		editingPort: false,
		editIndex:   -1,
	}
}

// ToggleSelection toggles selection for current cursor position
func (m *DeviceSelectorModel) ToggleSelection() {
	m.selected[m.cursor] = !m.selected[m.cursor]
}

// SelectAll selects all devices
func (m *DeviceSelectorModel) SelectAll() {
	for i := range m.devices {
		m.selected[i] = true
	}
}

// SelectNone deselects all devices
func (m *DeviceSelectorModel) SelectNone() {
	m.selected = make(map[int]bool)
}

// MoveCursor moves the cursor up or down
func (m *DeviceSelectorModel) MoveCursor(delta int) {
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.devices) {
		m.cursor = len(m.devices) - 1
	}
}

// StartPortEdit starts editing port for current device
func (m *DeviceSelectorModel) StartPortEdit() {
	m.editingPort = true
	m.editIndex = m.cursor

	// Pre-fill with existing custom port or first open port
	device := m.devices[m.cursor]
	if customPort, ok := m.customPorts[m.cursor]; ok {
		m.portInput.SetValue(fmt.Sprintf("%d", customPort))
	} else if len(device.OpenPorts) > 0 {
		m.portInput.SetValue(fmt.Sprintf("%d", device.OpenPorts[0]))
	} else {
		m.portInput.SetValue("443")
	}

	m.portInput.Focus()
}

// FinishPortEdit saves the edited port
func (m *DeviceSelectorModel) FinishPortEdit() {
	var port int
	fmt.Sscanf(m.portInput.Value(), "%d", &port)

	if port > 0 && port <= 65535 {
		m.customPorts[m.editIndex] = port
	}

	m.editingPort = false
	m.editIndex = -1
	m.portInput.Blur()
	m.portInput.SetValue("")
}

// CancelPortEdit cancels port editing
func (m *DeviceSelectorModel) CancelPortEdit() {
	m.editingPort = false
	m.editIndex = -1
	m.portInput.Blur()
	m.portInput.SetValue("")
}

// GetPort returns the port to use for a device (custom or default)
func (m *DeviceSelectorModel) GetPort(index int) int {
	if customPort, ok := m.customPorts[index]; ok {
		return customPort
	}

	device := m.devices[index]
	if len(device.OpenPorts) > 0 {
		// Prefer HTTPS > HTTP > other
		for _, port := range []int{443, 8443, 80, 8080} {
			for _, openPort := range device.OpenPorts {
				if openPort == port {
					return port
				}
			}
		}
		// Return first open port
		return device.OpenPorts[0]
	}

	// Default to 443
	return 443
}

// GetSelectedDevices returns list of selected devices as config.Device
func (m *DeviceSelectorModel) GetSelectedDevices(subnet string) []config.Device {
	devices := make([]config.Device, 0)

	for i, device := range m.devices {
		if m.selected[i] {
			port := m.GetPort(i)

			// Extract last octet from IP (works with any subnet)
			parts := strings.Split(device.IP, ".")
			var lastOctet int
			if len(parts) == 4 {
				fmt.Sscanf(parts[3], "%d", &lastOctet)
			}

			devices = append(devices, config.Device{
				IP:        device.IP,
				Name:      fmt.Sprintf("%s:%d (%s)", device.IP, port, device.DeviceType),
				Port:      port,
				LocalPort: 4430 + lastOctet + port,
			})
		}
	}

	return devices
}

// View renders the device selector
func (m DeviceSelectorModel) View() string {
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	b.WriteString(titleStyle.Render(fmt.Sprintf("Discovered Devices (%d found)", len(m.devices))))
	b.WriteString("\n\n")

	if len(m.devices) == 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("No devices discovered."))
		return b.String()
	}

	// Table headers
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	b.WriteString(headerStyle.Render("   IP Address      Status   Ports            Vendor                    Device Type\n"))
	b.WriteString(strings.Repeat("─", 100) + "\n")

	// Device rows
	for i, device := range m.devices {
		// Cursor indicator
		cursor := "  "
		if i == m.cursor {
			cursor = "> "
		}

		// Checkbox
		checkbox := "[ ]"
		if m.selected[i] {
			checkbox = "[✓]"
		}

		// Port info (show custom port if set)
		portInfo := device.FormatPortsString()
		if customPort, ok := m.customPorts[i]; ok {
			portInfo = fmt.Sprintf("%s (→%d)", portInfo, customPort)
		}

		// Highlight current row
		rowStyle := lipgloss.NewStyle()
		if i == m.cursor {
			rowStyle = rowStyle.Background(lipgloss.Color("235"))
		}

		// Truncate vendor if too long
		vendorDisplay := device.Vendor
		if len(vendorDisplay) > 25 {
			vendorDisplay = vendorDisplay[:22] + "..."
		}

		row := fmt.Sprintf("%s%s %-15s %-8s %-16s %-25s %s",
			cursor,
			checkbox,
			device.IP,
			"Online",
			portInfo,
			vendorDisplay,
			device.DeviceType,
		)

		b.WriteString(rowStyle.Render(row))
		b.WriteString("\n")

		// Show port edit input if editing this device
		if m.editingPort && m.editIndex == i {
			b.WriteString(rowStyle.Render(fmt.Sprintf("      Port: %s", m.portInput.View())))
			b.WriteString("\n")
		}
	}

	// Summary
	b.WriteString("\n")
	selectedCount := len(m.selected)
	summaryStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	b.WriteString(summaryStyle.Render(fmt.Sprintf("Selected: %d/%d devices", selectedCount, len(m.devices))))

	return b.String()
}

// HelpView renders help text
func (m DeviceSelectorModel) HelpView() string {
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	if m.editingPort {
		return helpStyle.Render("enter: save port • esc: cancel")
	}

	return helpStyle.Render("space: toggle • a: select all • n: none • p: edit port • s: save preset • enter: connect • esc: cancel")
}
