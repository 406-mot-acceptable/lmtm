package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// TableModel renders a simple aligned table with styled headers.
type TableModel struct {
	Headers []string
	Rows    [][]string
	// Minimum column widths. If nil, widths are auto-calculated.
	MinWidths []int
}

// NewTable creates a table with the given column headers.
func NewTable(headers []string) TableModel {
	return TableModel{
		Headers: headers,
	}
}

// SetRows replaces all table rows.
func (t *TableModel) SetRows(rows [][]string) {
	t.Rows = rows
}

// View renders the table with aligned columns and styled headers.
func (t TableModel) View() string {
	if len(t.Headers) == 0 {
		return ""
	}

	colCount := len(t.Headers)
	widths := make([]int, colCount)

	// Calculate column widths from headers.
	for i, h := range t.Headers {
		if len(h) > widths[i] {
			widths[i] = len(h)
		}
	}

	// Expand widths from row data.
	for _, row := range t.Rows {
		for i := 0; i < colCount && i < len(row); i++ {
			if len(row[i]) > widths[i] {
				widths[i] = len(row[i])
			}
		}
	}

	// Apply minimum widths if set.
	for i := 0; i < colCount && i < len(t.MinWidths); i++ {
		if t.MinWidths[i] > widths[i] {
			widths[i] = t.MinWidths[i]
		}
	}

	// Add padding to each column.
	for i := range widths {
		widths[i] += 2
	}

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.AdaptiveColor{Dark: "#AF87FF", Light: "#7B5FBF"})

	dimStyle := lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Dark: "#585858", Light: "#999999"})

	var b strings.Builder

	// Render header row.
	for i, h := range t.Headers {
		b.WriteString(headerStyle.Render(pad(h, widths[i])))
	}
	b.WriteByte('\n')

	// Render separator.
	for i, w := range widths {
		if i > 0 {
			b.WriteString(dimStyle.Render("  "))
		}
		b.WriteString(dimStyle.Render(strings.Repeat("─", w-2)))
	}
	b.WriteByte('\n')

	// Render data rows.
	for _, row := range t.Rows {
		for i := 0; i < colCount; i++ {
			cell := ""
			if i < len(row) {
				cell = row[i]
			}
			b.WriteString(pad(cell, widths[i]))
		}
		b.WriteByte('\n')
	}

	return b.String()
}

// pad right-pads a string to the given width.
func pad(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}
