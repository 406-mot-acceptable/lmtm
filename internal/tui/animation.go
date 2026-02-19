package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/406-mot-acceptable/lmtm/internal/ssh"
)

// animTickMsg is the frame ticker for pipe animation.
type animTickMsg time.Time

// pipeState tracks the visual state of a single tunnel pipe.
type pipeState int

const (
	pipePending  pipeState = iota // Waiting to build
	pipeDrawing                   // Currently animating
	pipeActive                    // Built successfully
	pipeFailed                    // Build failed
)

// animPipe represents one tunnel's visual pipe in the animation.
type animPipe struct {
	LocalPort  int
	RemoteHost string
	RemotePort int
	State      pipeState
	Frame      int // Animation frame counter (0-4)
}

// AnimationModel renders the ASCII pipe construction animation.
type AnimationModel struct {
	pipes      []animPipe
	gatewayTag string
	active     int // Index of currently-drawing pipe (-1 if none)
}

// NewAnimationModel creates an animation for the given tunnel specs.
func NewAnimationModel(specs []ssh.TunnelSpec, gatewayTag string) AnimationModel {
	pipes := make([]animPipe, len(specs))
	for i, s := range specs {
		pipes[i] = animPipe{
			LocalPort:  s.LocalPort,
			RemoteHost: s.RemoteHost,
			RemotePort: s.RemotePort,
			State:      pipePending,
		}
	}
	if gatewayTag == "" {
		gatewayTag = "GW"
	}
	return AnimationModel{
		pipes:      pipes,
		gatewayTag: gatewayTag,
		active:     -1,
	}
}

// Init starts the frame ticker.
func (m AnimationModel) Init() tea.Cmd {
	return m.tickCmd()
}

// Update advances the animation frame.
func (m AnimationModel) Update(msg tea.Msg) (AnimationModel, tea.Cmd) {
	switch msg.(type) {
	case animTickMsg:
		if m.active >= 0 && m.active < len(m.pipes) {
			p := &m.pipes[m.active]
			if p.State == pipeDrawing && p.Frame < 4 {
				p.Frame++
				return m, m.tickCmd()
			}
		}
		// Check if any pipe is still drawing.
		for i := range m.pipes {
			if m.pipes[i].State == pipeDrawing {
				return m, m.tickCmd()
			}
		}
		return m, nil
	}
	return m, nil
}

// MarkStarted transitions a tunnel to the drawing state.
func (m *AnimationModel) MarkStarted(localPort int) {
	for i := range m.pipes {
		if m.pipes[i].LocalPort == localPort {
			m.pipes[i].State = pipeDrawing
			m.pipes[i].Frame = 0
			m.active = i
			return
		}
	}
}

// MarkActive transitions a tunnel to the active (completed) state.
func (m *AnimationModel) MarkActive(localPort int) {
	for i := range m.pipes {
		if m.pipes[i].LocalPort == localPort {
			m.pipes[i].State = pipeActive
			m.pipes[i].Frame = 4
			return
		}
	}
}

// MarkFailed transitions a tunnel to the failed state.
func (m *AnimationModel) MarkFailed(localPort int) {
	for i := range m.pipes {
		if m.pipes[i].LocalPort == localPort {
			m.pipes[i].State = pipeFailed
			return
		}
	}
}

// AllDone returns true when no pipes are pending or drawing.
func (m AnimationModel) AllDone() bool {
	for _, p := range m.pipes {
		if p.State == pipePending || p.State == pipeDrawing {
			return false
		}
	}
	return len(m.pipes) > 0
}

// View renders the tunnel construction diagram showing LOCAL:PORT --> REMOTE:PORT.
//
// Layout per tunnel:
//
//   localhost:4435 ====[ GW ]==== 192.168.1.5:443   [ OK ]
//
// During animation, the pipe builds progressively with dots becoming equals:
//
//   localhost:4435 ==..[ GW ]..== 192.168.1.5:443   [....]
//
func (m AnimationModel) View() string {
	if len(m.pipes) == 0 {
		return ""
	}

	// Calculate column widths for alignment.
	leftWidth := 0
	rightWidth := 0
	for _, p := range m.pipes {
		l := len(fmt.Sprintf("localhost:%d", p.LocalPort))
		if l > leftWidth {
			leftWidth = l
		}
		r := len(fmt.Sprintf("%s:%d", p.RemoteHost, p.RemotePort))
		if r > rightWidth {
			rightWidth = r
		}
	}

	gwLabel := fmt.Sprintf("[ %s ]", m.gatewayTag)
	pipeWidth := 4

	var b strings.Builder

	for i, p := range m.pipes {
		left := padRight(fmt.Sprintf("localhost:%d", p.LocalPort), leftWidth)
		right := padRight(fmt.Sprintf("%s:%d", p.RemoteHost, p.RemotePort), rightWidth)

		var pipe, status string

		switch p.State {
		case pipePending:
			lp := strings.Repeat(".", pipeWidth)
			rp := strings.Repeat(".", pipeWidth)
			pipe = DimStyle.Render(lp + gwLabel + rp)
			status = DimStyle.Render("[    ]")
		case pipeDrawing:
			built := p.Frame
			rest := pipeWidth - built
			lp := strings.Repeat("=", built) + strings.Repeat(".", rest)
			rp := strings.Repeat(".", rest) + strings.Repeat("=", built)
			pipe = WarningStyle.Render(lp + gwLabel + rp)
			status = WarningStyle.Render("[....]")
		case pipeActive:
			lp := strings.Repeat("=", pipeWidth)
			rp := strings.Repeat("=", pipeWidth)
			pipe = SuccessStyle.Render(lp + gwLabel + rp)
			status = SuccessStyle.Render("[ OK ]")
		case pipeFailed:
			lp := strings.Repeat("-", pipeWidth)
			rp := strings.Repeat("-", pipeWidth)
			pipe = ErrorStyle.Render(lp + " XX " + rp)
			status = ErrorStyle.Render("[FAIL]")
		}

		// Render: LOCAL:PORT ==[ GW ]==  REMOTE:PORT  [STATUS]
		b.WriteString(ActiveStyle.Render(left))
		b.WriteString(" ")
		b.WriteString(pipe)
		b.WriteString(" ")
		b.WriteString(right)
		b.WriteString("  ")
		b.WriteString(status)

		if i < len(m.pipes)-1 {
			b.WriteByte('\n')
		}
		b.WriteByte('\n')
	}

	return b.String()
}

func (m AnimationModel) tickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return animTickMsg(t)
	})
}

// padRight pads a string with spaces to the given width.
func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}
