package tui

import "github.com/charmbracelet/bubbles/key"

// GlobalKeys handles quit and back navigation.
type GlobalKeys struct {
	Quit key.Binding
	Back key.Binding
}

// ShortHelp returns keybindings for the short help view.
func (k GlobalKeys) ShortHelp() []key.Binding {
	return []key.Binding{k.Quit, k.Back}
}

// FullHelp returns keybindings for the full help view.
func (k GlobalKeys) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Quit, k.Back}}
}

// NavigationKeys handles list navigation.
type NavigationKeys struct {
	Up    key.Binding
	Down  key.Binding
	Enter key.Binding
}

// ShortHelp returns keybindings for the short help view.
func (k NavigationKeys) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Enter}
}

// FullHelp returns keybindings for the full help view.
func (k NavigationKeys) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Up, k.Down, k.Enter}}
}

// SelectionKeys handles multi-select in device lists.
type SelectionKeys struct {
	Toggle  key.Binding
	All     key.Binding
	None    key.Binding
	FirstN  key.Binding
}

// ShortHelp returns keybindings for the short help view.
func (k SelectionKeys) ShortHelp() []key.Binding {
	return []key.Binding{k.Toggle, k.All, k.None}
}

// FullHelp returns keybindings for the full help view.
func (k SelectionKeys) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Toggle, k.All, k.None, k.FirstN}}
}

// TunnelKeys handles the active tunnel dashboard.
type TunnelKeys struct {
	Reconnect key.Binding
	EditPorts key.Binding
}

// ShortHelp returns keybindings for the short help view.
func (k TunnelKeys) ShortHelp() []key.Binding {
	return []key.Binding{k.Reconnect, k.EditPorts}
}

// FullHelp returns keybindings for the full help view.
func (k TunnelKeys) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Reconnect, k.EditPorts}}
}

// ConnectKeys handles the connection input screen.
type ConnectKeys struct {
	NextField key.Binding
	PrevField key.Binding
	Connect   key.Binding
}

// ShortHelp returns keybindings for the short help view.
func (k ConnectKeys) ShortHelp() []key.Binding {
	return []key.Binding{k.NextField, k.Connect}
}

// FullHelp returns keybindings for the full help view.
func (k ConnectKeys) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.NextField, k.PrevField, k.Connect}}
}

// DefaultGlobalKeys returns the default global keybindings.
var DefaultGlobalKeys = GlobalKeys{
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	Back: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "back"),
	),
}

// DefaultNavigationKeys returns the default navigation keybindings.
var DefaultNavigationKeys = NavigationKeys{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("up/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("down/j", "down"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "select"),
	),
}

// DefaultSelectionKeys returns the default selection keybindings.
var DefaultSelectionKeys = SelectionKeys{
	Toggle: key.NewBinding(
		key.WithKeys(" "),
		key.WithHelp("space", "toggle"),
	),
	All: key.NewBinding(
		key.WithKeys("a"),
		key.WithHelp("a", "select all"),
	),
	None: key.NewBinding(
		key.WithKeys("n"),
		key.WithHelp("n", "select none"),
	),
	FirstN: key.NewBinding(
		key.WithKeys("f"),
		key.WithHelp("f", "first 10"),
	),
}

// DefaultTunnelKeys returns the default tunnel dashboard keybindings.
var DefaultTunnelKeys = TunnelKeys{
	Reconnect: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "reconnect"),
	),
	EditPorts: key.NewBinding(
		key.WithKeys("p"),
		key.WithHelp("p", "edit ports"),
	),
}

// DefaultConnectKeys returns the default connect screen keybindings.
var DefaultConnectKeys = ConnectKeys{
	NextField: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "next field"),
	),
	PrevField: key.NewBinding(
		key.WithKeys("shift+tab"),
		key.WithHelp("shift+tab", "prev field"),
	),
	Connect: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "connect"),
	),
}
