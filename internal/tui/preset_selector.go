package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/jaco/tunneler/internal/config"
)

// PresetSelectorModel handles preset selection UI
type PresetSelectorModel struct {
	config        *config.Config
	presetKeys    []string
	cursor        int
	selectedKey   string
	customRange   bool
	rangeStart    string
	rangeEnd      string
	rangeInputIdx int // 0 = start, 1 = end
}

// NewPresetSelector creates a new preset selector
func NewPresetSelector(cfg *config.Config) PresetSelectorModel {
	keys := cfg.GetPresetKeys()
	return PresetSelectorModel{
		config:     cfg,
		presetKeys: keys,
		cursor:     0,
	}
}

// GetSelectedPreset returns the selected preset or nil
func (m PresetSelectorModel) GetSelectedPreset() *config.Preset {
	if m.selectedKey != "" {
		return m.config.GetPreset(m.selectedKey)
	}
	return nil
}

// IsCustomRange returns true if user selected custom range
func (m PresetSelectorModel) IsCustomRange() bool {
	return m.customRange
}

// GetCustomRange returns start and end if custom range selected
func (m PresetSelectorModel) GetCustomRange() (int, int) {
	var start, end int
	fmt.Sscanf(m.rangeStart, "%d", &start)
	fmt.Sscanf(m.rangeEnd, "%d", &end)
	return start, end
}

// View renders the preset selector
func (m PresetSelectorModel) View() string {
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("12")).
		MarginBottom(1)

	b.WriteString(titleStyle.Render("Select Tunnel Preset"))
	b.WriteString("\n\n")

	// Show presets
	if len(m.presetKeys) > 0 {
		for i, key := range m.presetKeys {
			preset := m.config.GetPreset(key)
			if preset == nil {
				continue
			}

			cursor := "  "
			if m.cursor == i {
				cursor = "> "
			}

			// Format preset info
			info := ""
			if preset.Range != nil {
				info = fmt.Sprintf("%s.%d-%d", m.config.Defaults.Subnet, preset.Range.Start, preset.Range.End)
			} else if len(preset.Devices) > 0 {
				info = fmt.Sprintf("%d devices", len(preset.Devices))
			}

			ports := ""
			if len(preset.Ports) > 0 {
				ports = fmt.Sprintf("ports: %v", preset.Ports)
			}

			browser := ""
			if preset.BrowserTabs {
				browser = " [auto-browser]"
			}

			b.WriteString(fmt.Sprintf("%s%d. %s - %s %s%s\n", cursor, i+1, preset.Name, info, ports, browser))
		}
		b.WriteString("\n")
	}

	// Custom range option
	cursor := "  "
	customIdx := len(m.presetKeys)
	if m.cursor == customIdx {
		cursor = "> "
	}
	b.WriteString(fmt.Sprintf("%sc. Custom range\n", cursor))

	// Help text
	b.WriteString("\n")
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	b.WriteString(helpStyle.Render("↑/↓: navigate • 1-9/c: select • enter: confirm • esc: cancel"))

	return b.String()
}

// CustomRangeView renders custom range input UI
func (m PresetSelectorModel) CustomRangeView() string {
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("12")).
		MarginBottom(1)

	b.WriteString(titleStyle.Render("Custom Range"))
	b.WriteString("\n\n")

	// Start input
	startLabel := "Start IP (last octet): "
	if m.rangeInputIdx == 0 {
		startLabel = "> " + startLabel
	} else {
		startLabel = "  " + startLabel
	}
	b.WriteString(startLabel)
	b.WriteString(m.rangeStart)
	if m.rangeInputIdx == 0 {
		b.WriteString("_")
	}
	b.WriteString("\n")

	// End input
	endLabel := "End IP (last octet):   "
	if m.rangeInputIdx == 1 {
		endLabel = "> " + endLabel
	} else {
		endLabel = "  " + endLabel
	}
	b.WriteString(endLabel)
	b.WriteString(m.rangeEnd)
	if m.rangeInputIdx == 1 {
		b.WriteString("_")
	}
	b.WriteString("\n\n")

	// Help
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	b.WriteString(helpStyle.Render("0-9: input • enter: confirm • esc: cancel"))

	return b.String()
}
