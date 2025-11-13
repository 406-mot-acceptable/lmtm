package ssh

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/jaco/tunneler/internal/config"
)

// TunnelStatus represents the status of a tunnel
type TunnelStatus int

const (
	StatusDisconnected TunnelStatus = iota
	StatusConnecting
	StatusActive
	StatusFailed
)

func (s TunnelStatus) String() string {
	switch s {
	case StatusDisconnected:
		return "disconnected"
	case StatusConnecting:
		return "connecting"
	case StatusActive:
		return "active"
	case StatusFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// TunnelInfo contains information about a tunnel
type TunnelInfo struct {
	DeviceName string
	DeviceIP   string
	DevicePort int
	LocalPort  int
	Status     TunnelStatus
	Error      error
}

// SiteTunnel manages tunnels for a single site
type SiteTunnel struct {
	SiteName   string
	Gateway    string
	Username   string
	Password   string
	SSHOptions []string

	client    *ssh.Client
	tunnels   map[int]*TunnelInfo // localPort -> TunnelInfo
	listeners []net.Listener
	ctx       context.Context
	cancel    context.CancelFunc
	mu        sync.RWMutex
	wg        sync.WaitGroup

	statusCallback func(*TunnelInfo)
}

// NewSiteTunnel creates a new site tunnel manager
func NewSiteTunnel(siteName, gateway, username, password string, sshOptions []string) *SiteTunnel {
	ctx, cancel := context.WithCancel(context.Background())
	return &SiteTunnel{
		SiteName:   siteName,
		Gateway:    gateway,
		Username:   username,
		Password:   password,
		SSHOptions: sshOptions,
		tunnels:    make(map[int]*TunnelInfo),
		ctx:        ctx,
		cancel:     cancel,
	}
}

// SetStatusCallback sets the callback for status updates
func (st *SiteTunnel) SetStatusCallback(cb func(*TunnelInfo)) {
	st.statusCallback = cb
}

// notifyStatus sends status update via callback
func (st *SiteTunnel) notifyStatus(info *TunnelInfo) {
	if st.statusCallback != nil {
		st.statusCallback(info)
	}
}

// Connect establishes SSH connection and sets up tunnels
func (st *SiteTunnel) Connect(devices []config.Device) error {
	// Update all tunnels to connecting status
	st.mu.Lock()
	for _, device := range devices {
		info := &TunnelInfo{
			DeviceName: device.Name,
			DeviceIP:   device.IP,
			DevicePort: device.Port,
			LocalPort:  device.LocalPort,
			Status:     StatusConnecting,
		}
		st.tunnels[device.LocalPort] = info
		st.notifyStatus(info)
	}
	st.mu.Unlock()

	// Build SSH client config
	sshConfig := &ssh.ClientConfig{
		User: st.Username,
		Auth: []ssh.AuthMethod{
			ssh.Password(st.Password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // TODO: Implement proper host key checking
		Timeout:         10 * time.Second,
	}

	// Handle Ubiquiti ssh-rsa requirement
	for i := 0; i < len(st.SSHOptions)-1; i++ {
		if st.SSHOptions[i] == "-o" && st.SSHOptions[i+1] == "HostKeyAlgorithm=ssh-rsa" {
			sshConfig.HostKeyAlgorithms = []string{"ssh-rsa"}
			break
		}
	}

	// Connect to gateway
	client, err := ssh.Dial("tcp", st.Gateway+":22", sshConfig)
	if err != nil {
		// Mark all tunnels as failed
		st.mu.Lock()
		for _, info := range st.tunnels {
			info.Status = StatusFailed
			info.Error = err
			st.notifyStatus(info)
		}
		st.mu.Unlock()
		return fmt.Errorf("failed to connect to gateway: %w", err)
	}

	st.client = client

	// Set up port forwards for each device
	for _, device := range devices {
		if err := st.setupForward(device); err != nil {
			// Mark this device as failed but continue with others
			st.mu.Lock()
			if info, ok := st.tunnels[device.LocalPort]; ok {
				info.Status = StatusFailed
				info.Error = err
				st.notifyStatus(info)
			}
			st.mu.Unlock()
		}
	}

	return nil
}

// setupForward sets up a single port forward
func (st *SiteTunnel) setupForward(device config.Device) error {
	// Listen on local port
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", device.LocalPort))
	if err != nil {
		return fmt.Errorf("failed to listen on local port: %w", err)
	}

	st.listeners = append(st.listeners, listener)

	// Update status to active
	st.mu.Lock()
	if info, ok := st.tunnels[device.LocalPort]; ok {
		info.Status = StatusActive
		st.notifyStatus(info)
	}
	st.mu.Unlock()

	// Start accepting connections
	st.wg.Add(1)
	go st.handleForward(listener, device)

	return nil
}

// handleForward handles forwarding for a single tunnel
func (st *SiteTunnel) handleForward(listener net.Listener, device config.Device) {
	defer st.wg.Done()
	defer listener.Close()

	for {
		select {
		case <-st.ctx.Done():
			return
		default:
			// Set accept deadline to allow periodic context checking
			listener.(*net.TCPListener).SetDeadline(time.Now().Add(1 * time.Second))

			localConn, err := listener.Accept()
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue // Deadline exceeded, check context and retry
				}
				return
			}

			// Connect to remote device through SSH
			go st.forward(localConn, device)
		}
	}
}

// forward handles a single connection forward
func (st *SiteTunnel) forward(localConn net.Conn, device config.Device) {
	defer localConn.Close()

	remoteAddr := fmt.Sprintf("%s:%d", device.IP, device.Port)
	remoteConn, err := st.client.Dial("tcp", remoteAddr)
	if err != nil {
		return
	}
	defer remoteConn.Close()

	// Bidirectional copy
	done := make(chan struct{}, 2)

	go func() {
		io.Copy(remoteConn, localConn)
		done <- struct{}{}
	}()

	go func() {
		io.Copy(localConn, remoteConn)
		done <- struct{}{}
	}()

	<-done
}

// Disconnect closes all tunnels and the SSH connection
func (st *SiteTunnel) Disconnect() error {
	st.cancel()

	// Close all listeners
	for _, listener := range st.listeners {
		listener.Close()
	}

	// Wait for all forwards to finish
	st.wg.Wait()

	// Close SSH connection
	if st.client != nil {
		st.client.Close()
	}

	// Update all tunnel statuses
	st.mu.Lock()
	for _, info := range st.tunnels {
		info.Status = StatusDisconnected
		st.notifyStatus(info)
	}
	st.mu.Unlock()

	return nil
}

// GetTunnels returns all tunnel info
func (st *SiteTunnel) GetTunnels() []*TunnelInfo {
	st.mu.RLock()
	defer st.mu.RUnlock()

	tunnels := make([]*TunnelInfo, 0, len(st.tunnels))
	for _, info := range st.tunnels {
		tunnels = append(tunnels, info)
	}
	return tunnels
}

// IsConnected returns true if the SSH connection is active
func (st *SiteTunnel) IsConnected() bool {
	return st.client != nil
}

// ExecuteCommand runs a command on the gateway and returns output
func (st *SiteTunnel) ExecuteCommand(cmd string) (string, error) {
	if st.client == nil {
		return "", fmt.Errorf("not connected to gateway")
	}

	session, err := st.client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	output, err := session.CombinedOutput(cmd)
	if err != nil {
		return string(output), fmt.Errorf("command failed: %w", err)
	}

	return string(output), nil
}

// DialWithTimeout attempts to connect to a remote host:port through the SSH tunnel
// Returns true if the connection succeeds (port is open), false otherwise
func (st *SiteTunnel) DialWithTimeout(host string, port int, timeout time.Duration) bool {
	if st.client == nil {
		return false
	}

	// Create a channel to signal completion
	done := make(chan bool, 1)

	go func() {
		addr := fmt.Sprintf("%s:%d", host, port)
		conn, err := st.client.Dial("tcp", addr)
		if err != nil {
			done <- false
			return
		}
		conn.Close()
		done <- true
	}()

	// Wait for either completion or timeout
	select {
	case result := <-done:
		return result
	case <-time.After(timeout):
		return false
	}
}
