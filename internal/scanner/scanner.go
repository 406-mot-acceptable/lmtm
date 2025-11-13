package scanner

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jaco/tunneler/internal/ssh"
)

// DiscoveredDevice represents a device found on the network
type DiscoveredDevice struct {
	IP         string
	MACAddress string
	Vendor     string
	Online     bool
	OpenPorts  []int
	Services   map[int]string // port -> service name
	DeviceType string         // Camera, NVR, Network Device, etc
}

// ScanMethod defines the type of network scan
type ScanMethod string

const (
	ScanMethodARP  ScanMethod = "arp"  // Fast: uses ARP cache
	ScanMethodPing ScanMethod = "ping" // Medium: ping sweep
	ScanMethodNmap ScanMethod = "nmap" // Slow: full nmap scan
)

// Scanner performs network discovery
type Scanner struct {
	siteTunnel  *ssh.SiteTunnel
	subnet      string
	gatewayType string
	macCache    map[string]string // IP -> MAC address mapping
}

// NewScanner creates a new network scanner
func NewScanner(siteTunnel *ssh.SiteTunnel, subnet, gatewayType string) *Scanner {
	return &Scanner{
		siteTunnel:  siteTunnel,
		subnet:      subnet,
		gatewayType: gatewayType,
		macCache:    make(map[string]string),
	}
}

// DiscoverHosts finds active devices on the network
func (s *Scanner) DiscoverHosts(method ScanMethod) ([]string, error) {
	switch method {
	case ScanMethodARP:
		return s.discoverViaARP()
	case ScanMethodPing:
		return s.discoverViaPing()
	case ScanMethodNmap:
		return s.discoverViaNmap()
	default:
		return nil, fmt.Errorf("unknown scan method: %s", method)
	}
}

// discoverViaARP uses ARP cache for instant discovery
func (s *Scanner) discoverViaARP() ([]string, error) {
	// Use gateway-specific ARP command
	cmd := ssh.BuildARPCommand(s.gatewayType)
	output, err := s.siteTunnel.ExecuteCommand(cmd)
	if err != nil {
		return nil, fmt.Errorf("ARP scan failed: %w", err)
	}

	// Parse based on gateway type
	var entries []ssh.ARPEntry
	if s.gatewayType == "mikrotik" {
		entries = ssh.ParseMikroTikARP(output)
	} else {
		entries = ssh.ParseARPCache(output)
	}

	ips := make([]string, 0, len(entries))

	// Filter to subnet and active states
	for _, entry := range entries {
		if strings.HasPrefix(entry.IP, s.subnet+".") {
			// Include active entries
			// Linux: REACHABLE, STALE
			// MikroTik: D (dynamic)
			if s.gatewayType == "mikrotik" {
				// Any non-invalid entry (D, DP, DH flags)
				if strings.Contains(entry.State, "D") {
					ips = append(ips, entry.IP)
					s.macCache[entry.IP] = entry.MACAddress
				}
			} else {
				// Linux: REACHABLE or STALE
				if entry.State == "REACHABLE" || entry.State == "STALE" {
					ips = append(ips, entry.IP)
					s.macCache[entry.IP] = entry.MACAddress
				}
			}
		}
	}

	return ips, nil
}

// discoverViaPing performs ping sweep
func (s *Scanner) discoverViaPing() ([]string, error) {
	cmd := ssh.BuildPingSweepCommand(s.subnet, s.gatewayType)

	output, err := s.siteTunnel.ExecuteCommand(cmd)
	if err != nil {
		// Ping sweep might partially fail but still have results
		if output == "" {
			return nil, fmt.Errorf("ping sweep failed: %w", err)
		}
	}

	ips := ssh.ParsePingResults(output)
	return ips, nil
}

// discoverViaNmap uses nmap if available
func (s *Scanner) discoverViaNmap() ([]string, error) {
	// Check if nmap is available
	if !ssh.CheckToolAvailable(s.siteTunnel, "nmap") {
		return nil, fmt.Errorf("nmap not available on gateway")
	}

	cmd := fmt.Sprintf("nmap -sn %s.0/24 -oG - | grep 'Host:' | awk '{print $2}'", s.subnet)

	output, err := s.siteTunnel.ExecuteCommand(cmd)
	if err != nil {
		return nil, fmt.Errorf("nmap scan failed: %w", err)
	}

	ips := ssh.ParsePingResults(output)
	return ips, nil
}

// ScanPorts scans common ports on a host
// Falls back to client-side scanning if remote scanning fails
func (s *Scanner) ScanPorts(ip string, ports []int) ([]int, error) {
	// MikroTik RouterOS doesn't have netcat or bash /dev/tcp
	// Skip remote scanning and go straight to client-side
	if s.gatewayType == "mikrotik" {
		return s.ScanPortsClientSide(ip, ports)
	}

	// Try remote scanning first on Linux-based gateways (Ubiquiti, etc)
	cmd := ssh.BuildPortScanCommand(ip, ports)
	output, err := s.siteTunnel.ExecuteCommand(cmd)

	// If remote scan succeeded, parse and return results
	if err == nil && output != "" {
		openPorts := ssh.ParsePortScanResults(output)
		if len(openPorts) > 0 {
			return openPorts, nil
		}
	}

	// Fall back to client-side scanning
	// This works on all gateway types since it uses SSH tunnel's Dial
	return s.ScanPortsClientSide(ip, ports)
}

// ScanPortsClientSide scans ports by dialing through the SSH tunnel
// This is slower but works on all gateway types (MikroTik, Ubiquiti, etc)
func (s *Scanner) ScanPortsClientSide(ip string, ports []int) ([]int, error) {
	openPorts := make([]int, 0)

	// Test each port with 1 second timeout
	for _, port := range ports {
		if s.siteTunnel.DialWithTimeout(ip, port, 1*time.Second) {
			openPorts = append(openPorts, port)
		}
	}

	return openPorts, nil
}

// ScanNetwork performs full network discovery with port scanning
func (s *Scanner) ScanNetwork(method ScanMethod, scanPorts []int) ([]DiscoveredDevice, error) {
	// Step 1: Discover hosts
	ips, err := s.DiscoverHosts(method)
	if err != nil {
		return nil, err
	}

	if len(ips) == 0 {
		return []DiscoveredDevice{}, nil
	}

	// Step 2: Scan ports on discovered hosts
	devices := make([]DiscoveredDevice, 0, len(ips))

	for _, ip := range ips {
		openPorts, err := s.ScanPorts(ip, scanPorts)

		// Build services map
		services := make(map[int]string)
		for _, port := range openPorts {
			services[port] = ssh.GetServiceName(port)
		}

		// Look up MAC address and vendor
		macAddress := s.macCache[ip]
		vendor := ssh.LookupVendor(macAddress)

		device := DiscoveredDevice{
			IP:         ip,
			MACAddress: macAddress,
			Vendor:     vendor,
			Online:     true,
			OpenPorts:  openPorts,
			Services:   services,
			DeviceType: ssh.GuessDeviceType(openPorts, vendor),
		}

		// Include device even if port scan failed (host is still online)
		if err != nil {
			device.OpenPorts = []int{}
			device.Services = make(map[int]string)
			device.DeviceType = "Unknown (port scan failed)"
		}

		devices = append(devices, device)
	}

	// Sort by IP address for consistent ordering
	sort.Slice(devices, func(i, j int) bool {
		return compareIPs(devices[i].IP, devices[j].IP)
	})

	return devices, nil
}

// compareIPs compares two IP addresses for sorting
func compareIPs(ip1, ip2 string) bool {
	parts1 := strings.Split(ip1, ".")
	parts2 := strings.Split(ip2, ".")

	if len(parts1) != 4 || len(parts2) != 4 {
		return ip1 < ip2
	}

	for i := 0; i < 4; i++ {
		var n1, n2 int
		fmt.Sscanf(parts1[i], "%d", &n1)
		fmt.Sscanf(parts2[i], "%d", &n2)

		if n1 != n2 {
			return n1 < n2
		}
	}

	return false
}

// FormatServicesString returns a comma-separated list of services
func (d *DiscoveredDevice) FormatServicesString() string {
	if len(d.Services) == 0 {
		return "No services detected"
	}

	services := make([]string, 0, len(d.Services))
	for _, service := range d.Services {
		services = append(services, service)
	}

	// Remove duplicates
	uniqueServices := make(map[string]bool)
	for _, s := range services {
		uniqueServices[s] = true
	}

	result := make([]string, 0, len(uniqueServices))
	for s := range uniqueServices {
		result = append(result, s)
	}

	sort.Strings(result)
	return strings.Join(result, ", ")
}

// FormatPortsString returns a comma-separated list of open ports
func (d *DiscoveredDevice) FormatPortsString() string {
	if len(d.OpenPorts) == 0 {
		return "None"
	}

	ports := make([]string, len(d.OpenPorts))
	for i, port := range d.OpenPorts {
		ports[i] = fmt.Sprintf("%d", port)
	}

	return strings.Join(ports, ",")
}
