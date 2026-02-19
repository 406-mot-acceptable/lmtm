package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// ConnectMsg is sent when the user submits the connection form.
type ConnectMsg struct {
	Gateway  string
	Username string
	Password string
}

// ConnectModel is the gateway connection input screen.
type ConnectModel struct {
	gatewayInput  textinput.Model
	usernameInput textinput.Model
	passwordInput textinput.Model
	focusIndex    int
	err           error
	keys          ConnectKeys
	globals       GlobalKeys
}

// NewConnectModel creates the connection input screen with default values.
func NewConnectModel() ConnectModel {
	gi := textinput.New()
	gi.Placeholder = "192.168.1.1"
	gi.CharLimit = 45 // IPv6 max
	gi.Width = 30
	gi.Focus()

	ui := textinput.New()
	ui.Placeholder = "admin"
	ui.SetValue("dato")
	ui.CharLimit = 32
	ui.Width = 30

	pi := textinput.New()
	pi.Placeholder = "password"
	pi.EchoMode = textinput.EchoPassword
	pi.EchoCharacter = '*'
	pi.CharLimit = 128
	pi.Width = 30

	return ConnectModel{
		gatewayInput:  gi,
		usernameInput: ui,
		passwordInput: pi,
		focusIndex:    0,
		keys:          DefaultConnectKeys,
		globals:       DefaultGlobalKeys,
	}
}

// Gateway returns the entered gateway address.
func (m ConnectModel) Gateway() string {
	return strings.TrimSpace(m.gatewayInput.Value())
}

// Username returns the entered username.
func (m ConnectModel) Username() string {
	return strings.TrimSpace(m.usernameInput.Value())
}

// Password returns the entered password.
func (m ConnectModel) Password() string {
	return m.passwordInput.Value()
}

// SetError sets an error to display on the connect screen.
func (m *ConnectModel) SetError(err error) {
	m.err = err
}

// Init initializes the text input blink.
func (m ConnectModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles input events for the connect screen.
func (m ConnectModel) Update(msg tea.Msg) (ConnectModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.NextField):
			m.focusIndex = (m.focusIndex + 1) % 3
			return m, m.updateFocus()

		case key.Matches(msg, m.keys.PrevField):
			m.focusIndex = (m.focusIndex + 2) % 3 // +2 wraps backwards
			return m, m.updateFocus()

		case key.Matches(msg, m.keys.Connect):
			// Only trigger connect if we have at least gateway and password.
			if m.Gateway() != "" && m.Password() != "" {
				username := m.Username()
				if username == "" {
					username = "dato"
				}
				cmsg := ConnectMsg{
					Gateway:  m.Gateway(),
					Username: username,
					Password: m.Password(),
				}
				// Clear password from the input model immediately after
				// capturing it, to reduce the window of plaintext retention.
				m.passwordInput.SetValue("")
				return m, func() tea.Msg {
					return cmsg
				}
			}
			// If fields are missing, advance to the next empty field.
			if m.Gateway() == "" {
				m.focusIndex = 0
			} else if m.Username() == "" {
				m.focusIndex = 1
			} else {
				m.focusIndex = 2
			}
			return m, m.updateFocus()
		}
	}

	// Forward to the focused input.
	var cmd tea.Cmd
	switch m.focusIndex {
	case 0:
		m.gatewayInput, cmd = m.gatewayInput.Update(msg)
	case 1:
		m.usernameInput, cmd = m.usernameInput.Update(msg)
	case 2:
		m.passwordInput, cmd = m.passwordInput.Update(msg)
	}
	return m, cmd
}

// updateFocus sets focus on the correct input field.
func (m *ConnectModel) updateFocus() tea.Cmd {
	cmds := make([]tea.Cmd, 3)
	inputs := []*textinput.Model{&m.gatewayInput, &m.usernameInput, &m.passwordInput}
	for i, input := range inputs {
		if i == m.focusIndex {
			cmds[i] = input.Focus()
		} else {
			input.Blur()
		}
	}
	return tea.Batch(cmds...)
}

// View renders the connect screen.
func (m ConnectModel) View() string {
	var b strings.Builder

	// LMTM banner.
	b.WriteString(Banner())
	b.WriteString("\n\n")

	// Input fields.
	var form strings.Builder
	fields := []struct {
		label string
		input textinput.Model
	}{
		{"Gateway", m.gatewayInput},
		{"Username", m.usernameInput},
		{"Password", m.passwordInput},
	}

	for i, f := range fields {
		label := LabelStyle.Render(f.label)
		cursor := "  "
		if i == m.focusIndex {
			cursor = AccentStyle.Render("> ")
		}
		form.WriteString(cursor + label + " " + f.input.View())
		if i < len(fields)-1 {
			form.WriteByte('\n')
		}
		form.WriteByte('\n')
	}

	// Error display.
	if m.err != nil {
		form.WriteByte('\n')
		form.WriteString(ErrorStyle.Render("Error: " + m.err.Error()))
	}

	b.WriteString(renderPanel("Connect", form.String()))

	// Status bar.
	b.WriteByte('\n')
	b.WriteString(renderStatusBar(
		"Tab/Shift+Tab: navigate",
		"Enter: connect",
		"Ctrl+C: quit",
	))

	return ContentStyle.Render(b.String())
}
