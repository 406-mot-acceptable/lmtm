package ssh

import (
	"fmt"
	"sync"

	"github.com/jaco/tunneler/internal/config"
)

// Manager manages multiple site tunnels
type Manager struct {
	activeSites map[string]*SiteTunnel // siteName -> SiteTunnel
	password    string
	mu          sync.RWMutex
}

// NewManager creates a new tunnel manager
func NewManager() *Manager {
	return &Manager{
		activeSites: make(map[string]*SiteTunnel),
	}
}

// SetPassword sets the cached password
func (m *Manager) SetPassword(password string) {
	m.password = password
}

// ConnectSite connects to a site and sets up all device tunnels
func (m *Manager) ConnectSite(site *config.Site, devices []config.Device, defaults config.Defaults, statusCallback func(*TunnelInfo)) error {
	m.mu.Lock()
	// Close existing connection if any
	if existing, ok := m.activeSites[site.Name]; ok {
		existing.Disconnect()
		delete(m.activeSites, site.Name)
	}
	m.mu.Unlock()

	// Create new site tunnel
	siteTunnel := NewSiteTunnel(
		site.Name,
		site.Gateway,
		site.GetUsername(defaults),
		m.password,
		site.GetSSHOptions(),
	)

	if statusCallback != nil {
		siteTunnel.SetStatusCallback(statusCallback)
	}

	// Connect
	if err := siteTunnel.Connect(devices); err != nil {
		return err
	}

	m.mu.Lock()
	m.activeSites[site.Name] = siteTunnel
	m.mu.Unlock()

	return nil
}

// DisconnectSite disconnects a specific site
func (m *Manager) DisconnectSite(siteName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if siteTunnel, ok := m.activeSites[siteName]; ok {
		if err := siteTunnel.Disconnect(); err != nil {
			return err
		}
		delete(m.activeSites, siteName)
	}

	return nil
}

// DisconnectAll disconnects all active sites
func (m *Manager) DisconnectAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, siteTunnel := range m.activeSites {
		siteTunnel.Disconnect()
	}

	m.activeSites = make(map[string]*SiteTunnel)
	return nil
}

// GetAllTunnels returns all active tunnels across all sites
func (m *Manager) GetAllTunnels() map[string][]*TunnelInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string][]*TunnelInfo)
	for siteName, siteTunnel := range m.activeSites {
		result[siteName] = siteTunnel.GetTunnels()
	}

	return result
}

// IsSiteConnected checks if a site is connected
func (m *Manager) IsSiteConnected(siteName string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if siteTunnel, ok := m.activeSites[siteName]; ok {
		return siteTunnel.IsConnected()
	}

	return false
}

// QuickConnect creates a quick tunnel without config file
func (m *Manager) QuickConnect(gateway, username, password, gatewayType string, subnet string, start, end int, statusCallback func(*TunnelInfo)) error {
	// Generate devices
	devices := make([]config.Device, 0, end-start+1)
	for i := start; i <= end; i++ {
		ip := fmt.Sprintf("%s.%d", subnet, i)
		device := config.Device{
			IP:        ip,
			Name:      fmt.Sprintf("Device %d", i),
			Port:      443,
			LocalPort: 4430 + i,
		}
		devices = append(devices, device)
	}

	// Create temporary site config
	site := &config.Site{
		Name:     fmt.Sprintf("Quick: %s", gateway),
		Gateway:  gateway,
		Type:     gatewayType,
		Username: username,
	}

	// Create minimal defaults (QuickConnect already has username)
	defaults := config.Defaults{
		Username: username,
		Subnet:   subnet,
	}

	return m.ConnectSite(site, devices, defaults, statusCallback)
}
