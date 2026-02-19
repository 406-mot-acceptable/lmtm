package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/406-mot-acceptable/lmtm/internal/discovery"
	"github.com/406-mot-acceptable/lmtm/internal/gateway"
	"github.com/406-mot-acceptable/lmtm/internal/portmap"
	"github.com/406-mot-acceptable/lmtm/internal/ssh"
	"github.com/406-mot-acceptable/lmtm/internal/stats"
)

// SurveyDataMsg carries WAN/LAN info from the async survey command.
type SurveyDataMsg struct {
	WAN      *gateway.WANConfig
	LAN      *gateway.LANConfig
	Hostname string
	Err      error
}

// wizardState mirrors wizardState to avoid import cycle.
type wizardState int

const (
	stateConnect   wizardState = iota
	stateDetecting
	stateSurvey
	stateScanning
	stateDevices
	stateBuilding
	stateTunnels
	stateError
)

// errMsg wraps a generic error for state transitions.
type errMsg struct {
	err error
}

// AppModel is the root Bubbletea model that wires all sub-models
// and drives the wizard state machine.
type AppModel struct {
	state     wizardState
	prevState wizardState

	// Sub-models.
	connect  ConnectModel
	detect   DetectModel
	survey   SurveyModel
	scan     ScanModel
	devices  DevicesModel
	building BuildingModel
	tunnels  TunnelsModel

	// Backend state.
	sshClient   *ssh.Client
	gw          gateway.Gateway
	manager     *ssh.Manager
	scanner     *discovery.Scanner
	allocator   *portmap.PortAllocator
	lanSubnet   string
	gatewayAddr string
	gatewayType string
	hostname    string

	// Error state.
	lastErr error

	// Terminal size.
	width, height int
}

// NewAppModel creates the initial application model.
func NewAppModel() AppModel {
	return AppModel{
		state:   stateConnect,
		connect: NewConnectModel(),
	}
}

// Init starts the connect screen.
func (m AppModel) Init() tea.Cmd {
	return m.connect.Init()
}

// Update dispatches messages to the current state's handler.
func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle global keys first.
	if kmsg, ok := msg.(tea.KeyMsg); ok {
		// Ctrl+C always force-quits.
		if kmsg.String() == "ctrl+c" {
			return m, m.cleanup()
		}
		// Esc goes back or disconnects depending on state.
		if key.Matches(kmsg, DefaultGlobalKeys.Back) {
			return m.handleBack()
		}
	}

	// Handle window size.
	if msg, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	}

	switch m.state {
	case stateConnect:
		return m.updateConnect(msg)
	case stateDetecting:
		return m.updateDetecting(msg)
	case stateSurvey:
		return m.updateSurvey(msg)
	case stateScanning:
		return m.updateScanning(msg)
	case stateDevices:
		return m.updateDevices(msg)
	case stateBuilding:
		return m.updateBuilding(msg)
	case stateTunnels:
		return m.updateTunnels(msg)
	case stateError:
		return m.updateError(msg)
	}

	return m, nil
}

// View renders the current state's view.
func (m AppModel) View() string {
	switch m.state {
	case stateConnect:
		return m.connect.View()
	case stateDetecting:
		return m.detect.View()
	case stateSurvey:
		return m.survey.View()
	case stateScanning:
		return m.scan.View()
	case stateDevices:
		return m.devices.View()
	case stateBuilding:
		return m.building.View()
	case stateTunnels:
		return m.tunnels.View()
	case stateError:
		return m.errorView()
	default:
		return "Unknown state"
	}
}

// --- State update handlers ---

func (m AppModel) updateConnect(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case ConnectMsg:
		cm := msg.(ConnectMsg)
		m.gatewayAddr = cm.Gateway
		m.detect = NewDetectModel(cm.Gateway)
		m.state = stateDetecting
		return m, tea.Batch(
			m.detect.Init(),
			m.connectCmd(cm.Gateway, cm.Username, cm.Password),
		)
	}

	var cmd tea.Cmd
	m.connect, cmd = m.connect.Update(msg)
	return m, cmd
}

func (m AppModel) updateDetecting(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case sshConnectedMsg:
		// Store backend state from the connection.
		m.sshClient = msg.client
		m.gw = msg.gw
		m.hostname = msg.hostname
		m.gatewayType = msg.gwType
		// Forward to detect sub-model as DetectDoneMsg.
		doneMsg := DetectDoneMsg{
			GatewayType: msg.gwType,
			Hostname:    msg.hostname,
		}
		m.detect, _ = m.detect.Update(doneMsg)
		// Start async survey.
		return m, m.surveyCmd()

	case DetectDoneMsg:
		m.detect, _ = m.detect.Update(msg)
		if msg.Err != nil {
			return m.toError(msg.Err)
		}
		m.gatewayType = msg.GatewayType
		m.hostname = msg.Hostname
		return m, m.surveyCmd()

	case SurveyDataMsg:
		if msg.Err != nil {
			return m.toError(msg.Err)
		}
		var wan *WANConfig
		if msg.WAN != nil {
			wan = &WANConfig{
				Interface: msg.WAN.InterfaceName,
				PublicIP:  msg.WAN.PublicIP,
				Gateway:   msg.WAN.Gateway,
			}
		}
		var lan *LANConfig
		if msg.LAN != nil {
			lan = &LANConfig{
				Interface: msg.LAN.InterfaceName,
				Subnet:    msg.LAN.CIDR,
				Gateway:   msg.LAN.GatewayIP,
				DHCPStart: msg.LAN.DHCPStart,
				DHCPEnd:   msg.LAN.DHCPEnd,
			}
			m.lanSubnet = msg.LAN.Subnet
		}
		m.survey = NewSurveyModel(m.gatewayAddr, m.gatewayType, m.hostname, wan, lan)
		m.state = stateSurvey
		return m, m.survey.Init()
	}

	var cmd tea.Cmd
	m.detect, cmd = m.detect.Update(msg)
	return m, cmd
}

func (m AppModel) updateSurvey(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case ScanRequestMsg:
		m.scan = NewScanModel()
		m.state = stateScanning
		return m, tea.Batch(
			m.scan.Init(),
			m.scanCmd(),
		)
	}

	var cmd tea.Cmd
	m.survey, cmd = m.survey.Update(msg)
	return m, cmd
}

func (m AppModel) updateScanning(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case scanDevicesMsg:
		// Scan finished successfully with devices.
		doneMsg := ScanDoneMsg{DevicesFound: len(msg.devices)}
		m.scan, _ = m.scan.Update(doneMsg)
		m.devices = NewDevicesModel(msg.devices)
		m.state = stateDevices
		return m, m.devices.Init()

	case ScanDoneMsg:
		m.scan, _ = m.scan.Update(msg)
		if msg.Err != nil {
			return m.toError(msg.Err)
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.scan, cmd = m.scan.Update(msg)
	return m, cmd
}

func (m AppModel) updateDevices(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case DeviceSelectMsg:
		// Allocate ports and build tunnel specs.
		m.allocator = portmap.NewPortAllocator()
		var specs []ssh.TunnelSpec
		for _, d := range msg.Devices {
			for _, port := range d.Ports {
				localPort, err := m.allocator.Allocate(d.IP, port)
				if err != nil {
					continue
				}
				specs = append(specs, ssh.TunnelSpec{
					RemoteHost: d.IP,
					RemotePort: port,
					LocalPort:  localPort,
				})
			}
		}
		if len(specs) == 0 {
			return m.toError(fmt.Errorf("no tunnels could be allocated"))
		}

		m.manager = ssh.NewManager(m.sshClient, len(specs)*2)
		gwTag := m.hostname
		if gwTag == "" {
			gwTag = m.gatewayAddr
		}
		m.building = NewBuildingModel(specs, gwTag)
		m.state = stateBuilding
		return m, tea.Batch(
			m.building.Init(),
			m.buildCmd(specs),
		)
	}

	var cmd tea.Cmd
	m.devices, cmd = m.devices.Update(msg)
	return m, cmd
}

func (m AppModel) updateBuilding(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case TunnelBuildMsg:
		var cmd tea.Cmd
		m.building, cmd = m.building.Update(msg)
		// Chain to read the next event from the manager.
		return m, tea.Batch(cmd, m.nextEventCmd())

	case BuildDoneMsg:
		m.building, _ = m.building.Update(msg)
		// Record tunnel stats and check for milestones.
		active := msg.(BuildDoneMsg).Active
		milestone := ""
		if active > 0 {
			milestone = stats.AddTunnels(active)
		}
		// Brief pause to show final animation state, then transition.
		return m, tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
			return transitionToTunnelsMsg{milestone: milestone}
		})

	case transitionToTunnelsMsg:
		tunnels := m.manager.Tunnels()
		tmsg := msg.(transitionToTunnelsMsg)
		m.tunnels = NewTunnelsModel(tunnels)
		m.tunnels.milestone = tmsg.milestone
		m.state = stateTunnels
		return m, m.tunnels.Init()
	}

	var cmd tea.Cmd
	m.building, cmd = m.building.Update(msg)
	return m, cmd
}

func (m AppModel) updateTunnels(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case DisconnectMsg:
		return m.disconnect()
	case ReconnectMsg:
		// TODO: reconnect failed tunnels
		return m, nil
	}

	var cmd tea.Cmd
	m.tunnels, cmd = m.tunnels.Update(msg)
	return m, cmd
}

func (m AppModel) updateError(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "r":
			// Retry: go back to connect.
			return m.disconnect()
		case "q":
			return m, m.cleanup()
		}
	}
	return m, nil
}

// --- Navigation ---

func (m AppModel) handleBack() (tea.Model, tea.Cmd) {
	switch m.state {
	case stateConnect:
		return m, m.cleanup()
	case stateSurvey:
		return m.disconnect()
	case stateDevices:
		// Go back to survey.
		m.state = stateSurvey
		return m, nil
	case stateError:
		return m.disconnect()
	default:
		return m, nil
	}
}

// --- Async commands ---

func (m AppModel) connectCmd(host, user, pass string) tea.Cmd {
	return func() tea.Msg {
		client := ssh.NewClient()

		// Try connecting. If it fails with default algos, retry with ssh-rsa for Ubiquiti.
		err := client.Connect(host, "22", user, pass, nil)
		if err != nil {
			// Retry with ssh-rsa host key algorithm for Ubiquiti devices.
			client = ssh.NewClient()
			if err2 := client.Connect(host, "22", user, pass, []string{"ssh-rsa"}); err2 != nil {
				return DetectDoneMsg{Err: fmt.Errorf("connection failed: %w", err)}
			}
		}

		client.StartKeepalive(30 * time.Second)

		// Detect gateway type.
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		banner := client.ServerVersion()
		runner := client.Exec
		gw, err := gateway.Detect(ctx, banner, runner)
		if err != nil {
			client.Close()
			return DetectDoneMsg{Err: fmt.Errorf("detection failed: %w", err)}
		}

		// Get identity.
		hostname, _ := gw.Identity(ctx)

		// Store client and gateway on the model via a closure trick:
		// We can't modify m directly, so we send the data via the msg.
		// The AppModel will store these in updateDetecting via sshConnectedMsg.
		return sshConnectedMsg{
			client:   client,
			gw:       gw,
			hostname: hostname,
			gwType:   gwDisplayName(gw.Type()),
		}
	}
}

// sshConnectedMsg carries the SSH client and gateway after successful connection.
type sshConnectedMsg struct {
	client   *ssh.Client
	gw       gateway.Gateway
	hostname string
	gwType   string
}

// scanDevicesMsg carries discovered devices from the scan.
type scanDevicesMsg struct {
	devices []discovery.DiscoveredDevice
}

// transitionToTunnelsMsg triggers the transition from building to tunnels view.
type transitionToTunnelsMsg struct {
	milestone string
}

func (m AppModel) surveyCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		wan, _ := m.gw.WANInfo(ctx)
		lan, _ := m.gw.LANInfo(ctx)

		return SurveyDataMsg{
			WAN:      wan,
			LAN:      lan,
			Hostname: m.hostname,
		}
	}
}

func (m AppModel) scanCmd() tea.Cmd {
	// Capture gateway and subnet by value for the closure. Do not assign
	// back to m.scanner inside the closure -- m is a value receiver copy
	// and the assignment would be silently lost.
	gw := m.gw
	subnet := m.lanSubnet
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		scanner := discovery.NewScanner(gw)
		devices, err := scanner.Scan(ctx, subnet, nil)
		if err != nil {
			return ScanDoneMsg{Err: err}
		}

		return scanDevicesMsg{devices: devices}
	}
}

func (m AppModel) buildCmd(specs []ssh.TunnelSpec) tea.Cmd {
	// Capture manager before the closure to avoid value-copy issues.
	mgr := m.manager
	eventCh := mgr.Events()
	return func() tea.Msg {
		go mgr.BuildTunnels(specs)
		// Read the first event; subsequent reads are chained via nextEventCmd.
		ev, ok := <-eventCh
		if !ok {
			// Channel closed (CloseAll called) -- signal build is done.
			return BuildDoneMsg{}
		}
		return TunnelBuildMsg{Event: ev}
	}
}

func (m AppModel) nextEventCmd() tea.Cmd {
	// Capture manager before the closure to avoid value-copy issues.
	mgr := m.manager
	if mgr == nil {
		return func() tea.Msg { return BuildDoneMsg{} }
	}
	eventCh := mgr.Events()
	return func() tea.Msg {
		ev, ok := <-eventCh
		if !ok {
			// Channel closed (CloseAll called) -- signal build is done.
			return BuildDoneMsg{}
		}
		return TunnelBuildMsg{Event: ev}
	}
}

// --- Cleanup ---

func (m AppModel) disconnect() (tea.Model, tea.Cmd) {
	if m.manager != nil {
		m.manager.CloseAll()
		m.manager = nil
	} else if m.sshClient != nil {
		m.sshClient.Close()
	}
	m.sshClient = nil
	m.gw = nil
	m.scanner = nil
	m.allocator = nil
	m.lanSubnet = ""

	m.connect = NewConnectModel()
	m.state = stateConnect
	return m, m.connect.Init()
}

func (m AppModel) cleanup() tea.Cmd {
	if m.manager != nil {
		m.manager.CloseAll()
		m.manager = nil
	} else if m.sshClient != nil {
		m.sshClient.Close()
		m.sshClient = nil
	}
	return tea.Quit
}

func (m AppModel) toError(err error) (tea.Model, tea.Cmd) {
	m.lastErr = err
	m.prevState = m.state
	m.state = stateError
	return m, nil
}

// errorView renders the error screen with context about where the error occurred.
func (m AppModel) errorView() string {
	var b strings.Builder

	// Show which stage the error occurred in.
	stage := stateLabel(m.prevState)
	if stage != "" {
		b.WriteString(DimStyle.Render("During: "))
		b.WriteString(WarningStyle.Render(stage))
		b.WriteString("\n\n")
	}

	if m.lastErr != nil {
		b.WriteString(ErrorStyle.Render(m.lastErr.Error()))
	} else {
		b.WriteString(ErrorStyle.Render("An unknown error occurred"))
	}

	panel := renderPanel("Error", b.String())
	bar := renderStatusBar("r: retry", "q: quit", "Esc: back")

	return ContentStyle.Render(panel + "\n" + bar)
}

// stateLabel returns a human-readable label for a wizard state.
func stateLabel(s wizardState) string {
	switch s {
	case stateConnect:
		return "Connection"
	case stateDetecting:
		return "Gateway Detection"
	case stateSurvey:
		return "Network Survey"
	case stateScanning:
		return "Network Scan"
	case stateDevices:
		return "Device Selection"
	case stateBuilding:
		return "Tunnel Construction"
	case stateTunnels:
		return "Active Tunnels"
	default:
		return ""
	}
}

// gwDisplayName returns a human-readable name for the gateway type.
func gwDisplayName(t gateway.Type) string {
	switch t {
	case gateway.TypeMikroTik:
		return "MikroTik"
	case gateway.TypeUbiquiti:
		return "Ubiquiti"
	default:
		return string(t)
	}
}
