package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Adaptive colors that work on both light and dark terminals.
// First value is for dark backgrounds, second for light.
var (
	colorPrimary  = lipgloss.AdaptiveColor{Dark: "#AF87FF", Light: "#7B5FBF"}
	colorGreen    = lipgloss.AdaptiveColor{Dark: "#5FD75F", Light: "#2E8B2E"}
	colorRed      = lipgloss.AdaptiveColor{Dark: "#FF5F5F", Light: "#CC3333"}
	colorYellow   = lipgloss.AdaptiveColor{Dark: "#FFD75F", Light: "#B8860B"}
	colorDim      = lipgloss.AdaptiveColor{Dark: "#585858", Light: "#999999"}
	colorSubtle   = lipgloss.AdaptiveColor{Dark: "#444444", Light: "#AAAAAA"}
	colorFg       = lipgloss.AdaptiveColor{Dark: "#E0E0E0", Light: "#1A1A1A"}
	colorHighBg   = lipgloss.AdaptiveColor{Dark: "#303030", Light: "#E0E0E0"}
	colorBorder   = lipgloss.AdaptiveColor{Dark: "#3A3A3A", Light: "#CCCCCC"}
	colorInputBg  = lipgloss.AdaptiveColor{Dark: "#1C1C1C", Light: "#F0F0F0"}
	colorStatusBg = lipgloss.AdaptiveColor{Dark: "#262626", Light: "#E8E8E8"}
)

// panelBorder is a rounded border for outer panels.
var panelBorder = lipgloss.RoundedBorder()

// innerPanelBorder is a lighter border for nested panels.
var innerPanelBorder = lipgloss.Border{
	Top:         "─",
	Bottom:      "─",
	Left:        "│",
	Right:       "│",
	TopLeft:     "┌",
	TopRight:    "┐",
	BottomLeft:  "└",
	BottomRight: "┘",
}

// HeaderStyle is a bold title box with a subtle border.
var HeaderStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(colorPrimary).
	BorderStyle(lipgloss.RoundedBorder()).
	BorderForeground(colorBorder).
	Padding(0, 2)

// SubtitleStyle is dimmed subtitle text.
var SubtitleStyle = lipgloss.NewStyle().
	Foreground(colorSubtle).
	Italic(true)

// ContentStyle is the main content area with padding.
var ContentStyle = lipgloss.NewStyle().
	Padding(1, 2)

// FooterStyle is bottom help text, dimmed.
var FooterStyle = lipgloss.NewStyle().
	Foreground(colorDim).
	Padding(1, 0, 0, 0)

// SuccessStyle is green text for OK/active status.
var SuccessStyle = lipgloss.NewStyle().
	Foreground(colorGreen).
	Bold(true)

// ErrorStyle is red text for failures.
var ErrorStyle = lipgloss.NewStyle().
	Foreground(colorRed).
	Bold(true)

// WarningStyle is yellow text for warnings.
var WarningStyle = lipgloss.NewStyle().
	Foreground(colorYellow)

// SelectedStyle is the highlighted row in lists.
var SelectedStyle = lipgloss.NewStyle().
	Foreground(colorFg).
	Background(colorHighBg).
	Bold(true)

// ActiveStyle is the currently focused item.
var ActiveStyle = lipgloss.NewStyle().
	Foreground(colorPrimary).
	Bold(true)

// DimStyle is de-emphasized text.
var DimStyle = lipgloss.NewStyle().
	Foreground(colorDim)

// TableHeaderStyle is bold underlined table headers.
var TableHeaderStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(colorPrimary).
	BorderStyle(lipgloss.NormalBorder()).
	BorderBottom(true).
	BorderForeground(colorBorder)

// BoxStyle is a bordered box for framing sections.
var BoxStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.RoundedBorder()).
	BorderForeground(colorBorder).
	Padding(1, 2)

// InputStyle is text input field styling.
var InputStyle = lipgloss.NewStyle().
	Foreground(colorFg).
	Background(colorInputBg).
	Padding(0, 1)

// LabelStyle is labels next to inputs.
var LabelStyle = lipgloss.NewStyle().
	Foreground(colorPrimary).
	Bold(true).
	Width(12)

// PanelStyle is the outer bordered panel wrapping each screen.
var PanelStyle = lipgloss.NewStyle().
	BorderStyle(panelBorder).
	BorderForeground(colorBorder).
	Padding(1, 2)

// InnerPanelStyle is for nested sub-sections within a panel.
var InnerPanelStyle = lipgloss.NewStyle().
	BorderStyle(innerPanelBorder).
	BorderForeground(colorDim).
	Padding(0, 1)

// StatusBarStyle is the bottom status bar.
var StatusBarStyle = lipgloss.NewStyle().
	Foreground(colorFg).
	Background(colorStatusBg).
	Padding(0, 1).
	Bold(true)

// BannerStyle is for the large ASCII art banner text.
var BannerStyle = lipgloss.NewStyle().
	Foreground(colorPrimary).
	Bold(true)

// AccentStyle is for highlighted accent text.
var AccentStyle = lipgloss.NewStyle().
	Foreground(colorPrimary).
	Bold(true)

// renderPanel wraps content in a bordered panel with a title in the top border.
func renderPanel(title, content string) string {
	titleStr := " " + AccentStyle.Render(title) + " "

	body := PanelStyle.Render(content)

	// Replace the top-left portion of the border with the title.
	lines := strings.Split(body, "\n")
	if len(lines) > 0 {
		topLine := lines[0]
		// Find position to insert title (after the corner + a few border chars).
		if len(topLine) > 4 {
			// Insert title after the rounded corner and two border chars.
			runes := []rune(topLine)
			// Build: corner + "─" + title + rest of border
			var b strings.Builder
			b.WriteRune(runes[0]) // corner char
			b.WriteString(titleStr)
			// Fill remaining border width.
			titleVisual := 2 + lipgloss.Width(title) // spaces + title text
			remaining := len(runes) - 1 - titleVisual
			if remaining > 0 {
				for i := 0; i < remaining; i++ {
					b.WriteRune('─')
				}
				// Restore the closing corner.
				b.WriteRune(runes[len(runes)-1])
				lines[0] = b.String()
			}
		}
		body = strings.Join(lines, "\n")
	}

	return body
}

// renderStatusBar renders a horizontal status bar with pipe-separated items.
func renderStatusBar(items ...string) string {
	sep := DimStyle.Render(" | ")
	return StatusBarStyle.Render(strings.Join(items, sep))
}
