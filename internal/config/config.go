package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents the main configuration
type Config struct {
	Defaults       Defaults          `yaml:"defaults"`
	Presets        map[string]Preset `yaml:"presets,omitempty"`
	DeviceProfiles map[string]DeviceProfile `yaml:"device_profiles,omitempty"`
	Sites          []Site            `yaml:"sites"`
}

// Defaults contains default settings
type Defaults struct {
	Username       string `yaml:"username"`
	Subnet         string `yaml:"subnet"`
	PasswordPrompt bool   `yaml:"password_prompt"`
	DefaultPreset  string `yaml:"default_preset,omitempty"`
}

// Preset defines a reusable tunnel configuration
type Preset struct {
	Name        string       `yaml:"name"`
	Type        string       `yaml:"type,omitempty"`        // "tunnel" (default), "scan"
	Range       *DeviceRange `yaml:"range,omitempty"`
	Devices     []string     `yaml:"devices,omitempty"`     // Specific IPs
	Ports       []int        `yaml:"ports"`
	BrowserTabs bool         `yaml:"browser_tabs,omitempty"`
	Protocol    string       `yaml:"protocol,omitempty"`    // "https", "rtsp", etc.

	// Scan-specific options
	ScanMethod  string       `yaml:"scan_method,omitempty"` // "arp", "ping", "nmap"
	ScanPorts   []int        `yaml:"scan_ports,omitempty"`  // Ports to scan on discovered devices
	Subnets     []string     `yaml:"subnets,omitempty"`     // Multiple subnets to scan (e.g. ["10.0.0", "192.168.1"])
	AutoTunnel  bool         `yaml:"auto_tunnel,omitempty"` // Auto-tunnel to all discovered devices
}

// DeviceProfile defines characteristics of device types
type DeviceProfile struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description,omitempty"`
	Ports       []int    `yaml:"ports"`
	Protocol    string   `yaml:"protocol,omitempty"`
	BrowserURLs []string `yaml:"browser_urls,omitempty"`
	Tags        []string `yaml:"tags,omitempty"`
}

// DeviceInventory represents an explicitly configured device
type DeviceInventory struct {
	IP      string `yaml:"ip"`
	Name    string `yaml:"name"`
	Profile string `yaml:"profile,omitempty"` // References DeviceProfiles
	Notes   string `yaml:"notes,omitempty"`
}

// Site represents a customer site
type Site struct {
	Name          string            `yaml:"name"`
	Gateway       string            `yaml:"gateway"`
	Type          string            `yaml:"type"` // "ubiquiti" or "mikrotik"
	Username      string            `yaml:"username,omitempty"`
	Subnet        string            `yaml:"subnet,omitempty"`     // Override default subnet (e.g. "192.168.1")
	DeviceRange   *DeviceRange      `yaml:"device_range,omitempty"`
	Favorite      bool              `yaml:"favorite,omitempty"`
	DefaultPreset string            `yaml:"default_preset,omitempty"`
	Devices       []DeviceInventory `yaml:"devices,omitempty"`
}

// DeviceRange specifies a range of devices to tunnel
type DeviceRange struct {
	Start int `yaml:"start"`
	End   int `yaml:"end"`
}

// Device represents a tunneled device
type Device struct {
	IP        string
	Name      string
	Port      int
	LocalPort int
}

// GetUsername returns the username for this site (with fallback to default)
func (s *Site) GetUsername(defaults Defaults) string {
	if s.Username != "" {
		return s.Username
	}
	return defaults.Username
}

// GetSubnet returns the subnet for this site (with fallback to default)
func (s *Site) GetSubnet(defaults Defaults) string {
	if s.Subnet != "" {
		return s.Subnet
	}
	return defaults.Subnet
}

// GetSSHOptions returns SSH options based on gateway type
func (s *Site) GetSSHOptions() []string {
	if s.Type == "ubiquiti" {
		return []string{"-o", "HostKeyAlgorithm=ssh-rsa"}
	}
	return []string{}
}

// GenerateDevices generates device list based on range or defaults
func (s *Site) GenerateDevices(subnet string, defaultStart, defaultEnd int) []Device {
	start := defaultStart
	end := defaultEnd

	if s.DeviceRange != nil {
		start = s.DeviceRange.Start
		end = s.DeviceRange.End
	}

	devices := make([]Device, 0, end-start+1)
	for i := start; i <= end; i++ {
		ip := fmt.Sprintf("%s.%d", subnet, i)
		device := Device{
			IP:        ip,
			Name:      fmt.Sprintf("Device %d", i),
			Port:      443,
			LocalPort: 4430 + i, // 10.0.0.2 -> localhost:4432
		}
		devices = append(devices, device)
	}

	return devices
}

// Load loads configuration from a YAML file
func Load(path string) (*Config, error) {
	// Expand home directory
	if len(path) > 0 && path[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		path = filepath.Join(home, path[1:])
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Set defaults
	if cfg.Defaults.Username == "" {
		cfg.Defaults.Username = "dato"
	}
	if cfg.Defaults.Subnet == "" {
		cfg.Defaults.Subnet = "10.0.0"
	}
	if !cfg.Defaults.PasswordPrompt {
		cfg.Defaults.PasswordPrompt = true
	}

	return &cfg, nil
}

// GetSiteByName finds a site by name
func (c *Config) GetSiteByName(name string) *Site {
	for i := range c.Sites {
		if c.Sites[i].Name == name {
			return &c.Sites[i]
		}
	}
	return nil
}

// GetSitesByFavorite returns sites sorted by favorite status
func (c *Config) GetSitesByFavorite() []Site {
	favorites := make([]Site, 0)
	others := make([]Site, 0)

	for _, site := range c.Sites {
		if site.Favorite {
			favorites = append(favorites, site)
		} else {
			others = append(others, site)
		}
	}

	return append(favorites, others...)
}

// GetPreset retrieves a preset by key
func (c *Config) GetPreset(key string) *Preset {
	if preset, ok := c.Presets[key]; ok {
		return &preset
	}
	return nil
}

// GetPresetKeys returns sorted list of preset keys
func (c *Config) GetPresetKeys() []string {
	keys := make([]string, 0, len(c.Presets))
	for k := range c.Presets {
		keys = append(keys, k)
	}
	return keys
}

// ApplyPreset generates devices based on a preset
func (p *Preset) ApplyPreset(subnet string) []Device {
	devices := make([]Device, 0)

	// If specific devices are listed
	if len(p.Devices) > 0 {
		for _, deviceIP := range p.Devices {
			// Extract last octet from IP
			var lastOctet int
			fmt.Sscanf(deviceIP, subnet+".%d", &lastOctet)

			// For each port in the preset, create a separate tunnel
			for portIdx, port := range p.Ports {
				// Generate unique local port: base (4430 + octet) + port offset
				// E.g., 10.0.0.2:80 -> 4432, 10.0.0.2:443 -> 4875, 10.0.0.2:554 -> 4986
				localPort := 4430 + lastOctet + port

				// Generate descriptive name with port
				name := fmt.Sprintf("%s:%d", deviceIP, port)
				if len(p.Ports) > 1 {
					// Multi-port device - add protocol hint
					protocol := p.Protocol
					if protocol == "" {
						protocol = getProtocolHint(port)
					}
					name = fmt.Sprintf("%s:%d (%s)", deviceIP, port, protocol)
				}

				devices = append(devices, Device{
					IP:        deviceIP,
					Name:      name,
					Port:      port,
					LocalPort: localPort,
				})
				_ = portIdx // Avoid unused variable warning
			}
		}
		return devices
	}

	// If range is specified
	if p.Range != nil {
		for i := p.Range.Start; i <= p.Range.End; i++ {
			ip := fmt.Sprintf("%s.%d", subnet, i)

			// Use first port if only one specified, otherwise default to 443
			port := 443
			if len(p.Ports) > 0 {
				port = p.Ports[0]
			}

			devices = append(devices, Device{
				IP:        ip,
				Name:      fmt.Sprintf("Device %d", i),
				Port:      port,
				LocalPort: 4430 + i,
			})
		}
	}

	return devices
}

// getProtocolHint returns a protocol hint based on common port numbers
func getProtocolHint(port int) string {
	switch port {
	case 80:
		return "HTTP"
	case 443:
		return "HTTPS"
	case 554:
		return "RTSP"
	case 8080:
		return "HTTP-ALT"
	case 8443:
		return "HTTPS-ALT"
	default:
		return fmt.Sprintf("Port %d", port)
	}
}

// GetDeviceProfile retrieves a device profile by name
func (c *Config) GetDeviceProfile(name string) *DeviceProfile {
	if profile, ok := c.DeviceProfiles[name]; ok {
		return &profile
	}
	return nil
}

// IsScanPreset returns true if this is a scan/discovery preset
func (p *Preset) IsScanPreset() bool {
	return p.Type == "scan"
}

// GetScanMethod returns the scan method or default
func (p *Preset) GetScanMethod() string {
	if p.ScanMethod == "" {
		return "arp" // Default to ARP (fastest)
	}
	return p.ScanMethod
}

// GetScanPorts returns ports to scan or defaults
func (p *Preset) GetScanPorts() []int {
	if len(p.ScanPorts) > 0 {
		return p.ScanPorts
	}
	// Default common ports
	return []int{22, 80, 443, 554, 8080, 8443}
}

// GetScanSubnets returns the list of subnets to scan
// If preset has subnets defined, use those; otherwise use the provided default
func (p *Preset) GetScanSubnets(defaultSubnet string) []string {
	if len(p.Subnets) > 0 {
		return p.Subnets
	}
	// Fall back to single default subnet
	return []string{defaultSubnet}
}
