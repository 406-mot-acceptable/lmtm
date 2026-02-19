package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/406-mot-acceptable/lmtm/internal/tui/components"
)

// ScanProgressMsg updates the scan progress display.
type ScanProgressMsg struct {
	DevicesFound int
	Status       string
}

// ScanDoneMsg signals the scan is complete.
type ScanDoneMsg struct {
	DevicesFound int
	Err          error
}

// scanTickMsg is an internal tick for updating elapsed time.
type scanTickMsg time.Time

// ScanModel displays network scan progress.
type ScanModel struct {
	spinner      components.SpinnerModel
	elapsed      time.Duration
	startTime    time.Time
	devicesFound int
	status       string
	done         bool
	err          error
}

// NewScanModel creates the scan progress screen.
func NewScanModel() ScanModel {
	return ScanModel{
		spinner:   components.NewSpinner("Scanning network..."),
		startTime: time.Now(),
		status:    "Scanning network...",
	}
}

// Init starts the spinner and the elapsed time ticker.
func (m ScanModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Init(),
		m.tickCmd(),
	)
}

// Update handles scan progress and tick messages.
func (m ScanModel) Update(msg tea.Msg) (ScanModel, tea.Cmd) {
	switch msg := msg.(type) {
	case ScanProgressMsg:
		m.devicesFound = msg.DevicesFound
		if msg.Status != "" {
			m.status = msg.Status
		}
		m.spinner.SetMessage(m.statusLine())
		return m, nil

	case ScanDoneMsg:
		m.done = true
		m.devicesFound = msg.DevicesFound
		m.elapsed = time.Since(m.startTime)
		if msg.Err != nil {
			m.err = msg.Err
			m.status = "Scan failed"
		} else {
			m.status = "Scan complete"
		}
		return m, nil

	case scanTickMsg:
		if m.done {
			return m, nil
		}
		m.elapsed = time.Since(m.startTime)
		m.spinner.SetMessage(m.statusLine())
		return m, m.tickCmd()
	}

	// Forward spinner ticks.
	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

// Done returns whether the scan has completed.
func (m ScanModel) Done() bool {
	return m.done
}

// Err returns any scan error.
func (m ScanModel) Err() error {
	return m.err
}

// DevicesFound returns the number of discovered devices.
func (m ScanModel) DevicesFound() int {
	return m.devicesFound
}

// View renders the scan progress screen.
func (m ScanModel) View() string {
	var b strings.Builder

	if m.err != nil {
		b.WriteString(ErrorStyle.Render("Error: " + m.err.Error()))
		b.WriteByte('\n')
		b.WriteString(DimStyle.Render("[Esc] back"))
	} else if m.done {
		b.WriteString(SuccessStyle.Render(fmt.Sprintf(
			"Found %d devices in %.1fs",
			m.devicesFound,
			m.elapsed.Seconds(),
		)))
		b.WriteByte('\n')
	} else {
		b.WriteString(m.spinner.View())
	}

	return ContentStyle.Render(renderPanel("Network Scan", b.String()))
}

// statusLine builds the dynamic status text.
func (m ScanModel) statusLine() string {
	elapsed := fmt.Sprintf("%.1fs", m.elapsed.Seconds())
	if m.devicesFound > 0 {
		return fmt.Sprintf("%s... %s (%d devices found)", m.status, elapsed, m.devicesFound)
	}
	return fmt.Sprintf("%s... %s", m.status, elapsed)
}

// tickCmd returns a command that sends a tick after 100ms.
func (m ScanModel) tickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return scanTickMsg(t)
	})
}
