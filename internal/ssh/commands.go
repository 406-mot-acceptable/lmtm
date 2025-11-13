package ssh

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/endobit/oui"
)

// CheckToolAvailable checks if a command/tool is available on the gateway
func CheckToolAvailable(st *SiteTunnel, tool string) bool {
	output, err := st.ExecuteCommand(fmt.Sprintf("which %s", tool))
	return err == nil && strings.TrimSpace(output) != ""
}

// ARPEntry represents a parsed ARP cache entry
type ARPEntry struct {
	IP         string
	State      string // REACHABLE, STALE, DELAY, etc (Linux) or D/I flags (MikroTik)
	MACAddress string // Hardware address
}

// BuildARPCommand returns the appropriate ARP command for the gateway type
func BuildARPCommand(gatewayType string) string {
	switch gatewayType {
	case "mikrotik":
		// MikroTik RouterOS command
		return "/ip arp print where !invalid"
	case "ubiquiti":
		fallthrough
	default:
		// Linux command (works on Ubiquiti EdgeOS)
		return "ip neigh show"
	}
}

// ParseARPCache parses output from "ip neigh show" (Linux)
func ParseARPCache(output string) []ARPEntry {
	entries := make([]ARPEntry, 0)

	// Example line: 10.0.0.2 dev eth0 lladdr aa:bb:cc:dd:ee:ff REACHABLE
	re := regexp.MustCompile(`(\d+\.\d+\.\d+\.\d+).*?lladdr\s+([0-9a-fA-F:]+).*?(REACHABLE|STALE|DELAY|PROBE)`)

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		matches := re.FindStringSubmatch(line)
		if len(matches) >= 4 {
			entries = append(entries, ARPEntry{
				IP:         matches[1],
				MACAddress: strings.ToUpper(matches[2]),
				State:      matches[3],
			})
		}
	}

	return entries
}

// ParseMikroTikARP parses output from "/ip arp print"
func ParseMikroTikARP(output string) []ARPEntry {
	entries := make([]ARPEntry, 0)

	// Example line: 0 D 10.0.0.2 AA:BB:CC:DD:EE:FF bridge1
	// Format: # FLAGS ADDRESS MAC-ADDRESS INTERFACE
	// Flags: D=dynamic, I=invalid, P=published
	re := regexp.MustCompile(`\s*\d+\s+([DHP]+)\s+(\d+\.\d+\.\d+\.\d+)\s+([0-9A-F:]+)\s+`)

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		matches := re.FindStringSubmatch(line)
		if len(matches) >= 4 {
			flags := matches[1]
			// Skip invalid entries (should be filtered by 'where !invalid' but double-check)
			if strings.Contains(flags, "I") {
				continue
			}

			entries = append(entries, ARPEntry{
				IP:         matches[2],
				MACAddress: matches[3],
				State:      flags, // D, DP, etc
			})
		}
	}

	return entries
}

// ParsePingResults parses output from ping sweep
func ParsePingResults(output string) []string {
	ips := make([]string, 0)

	// Match IP addresses in output
	re := regexp.MustCompile(`\b(\d+\.\d+\.\d+\.\d+)\b`)

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		matches := re.FindStringSubmatch(line)
		if len(matches) >= 2 {
			ips = append(ips, matches[1])
		}
	}

	return ips
}

// BuildPingSweepCommand generates a command to ping sweep a subnet
func BuildPingSweepCommand(subnet string, gatewayType string) string {
	if gatewayType == "mikrotik" {
		// MikroTik RouterOS doesn't have bash
		// Use RouterOS ping command with scripting
		return fmt.Sprintf(`:foreach i in=[/ip address find] do={
			:local subnet "%s"
			:for i from=2 to=254 do={
				:local ip ($subnet . "." . $i)
				:do {
					/ping $ip count=1 interval=100ms
					:put $ip
				} on-error={}
			}
		}`, subnet)
	}
	// Linux-based (Ubiquiti, etc) - uses bash
	return fmt.Sprintf(`for i in {2..254}; do (ping -c 1 -W 1 %s.$i >/dev/null 2>&1 && echo %s.$i) & done; wait`,
		subnet, subnet)
}

// BuildPortScanCommand generates a command to scan ports on a host
func BuildPortScanCommand(ip string, ports []int) string {
	portList := make([]string, len(ports))
	for i, port := range ports {
		portList[i] = fmt.Sprintf("%d", port)
	}

	// Try netcat first, fallback to simple TCP connect test
	return fmt.Sprintf(`
		if command -v nc >/dev/null 2>&1; then
			for port in %s; do
				nc -zv -w 1 %s $port 2>&1 | grep -q succeeded && echo "$port"
			done
		else
			for port in %s; do
				timeout 1 bash -c "echo >/dev/tcp/%s/$port" 2>/dev/null && echo "$port"
			done
		fi
	`, strings.Join(portList, " "), ip, strings.Join(portList, " "), ip)
}

// ParsePortScanResults parses port scan output
func ParsePortScanResults(output string) []int {
	ports := make([]int, 0)

	re := regexp.MustCompile(`\b(\d+)\b`)

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		matches := re.FindStringSubmatch(line)
		if len(matches) >= 2 {
			var port int
			fmt.Sscanf(matches[1], "%d", &port)
			if port > 0 && port <= 65535 {
				ports = append(ports, port)
			}
		}
	}

	return ports
}

// GetServiceName returns service name based on port number
func GetServiceName(port int) string {
	switch port {
	case 22:
		return "SSH"
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
	case 8081, 8082, 8083:
		return "HTTP"
	case 5000, 5001:
		return "UPnP"
	default:
		return fmt.Sprintf("Port %d", port)
	}
}

// LookupVendor returns the vendor name from a MAC address
func LookupVendor(macAddress string) string {
	if macAddress == "" {
		return "Unknown"
	}

	vendor := oui.Vendor(macAddress)
	if vendor == "" {
		return "Unknown"
	}

	return vendor
}

// GuessDeviceType returns likely device type based on open ports and vendor
func GuessDeviceType(openPorts []int, vendor string) string {
	hasRTSP := false
	hasHTTP := false
	hasHTTPS := false
	hasSSH := false

	for _, port := range openPorts {
		switch port {
		case 554:
			hasRTSP = true
		case 80, 8080:
			hasHTTP = true
		case 443, 8443:
			hasHTTPS = true
		case 22:
			hasSSH = true
		}
	}

	// Use vendor information to improve guessing
	vendorLower := strings.ToLower(vendor)

	// Check for camera vendors
	if strings.Contains(vendorLower, "hikvision") ||
		strings.Contains(vendorLower, "dahua") ||
		strings.Contains(vendorLower, "axis") ||
		strings.Contains(vendorLower, "vivotek") ||
		strings.Contains(vendorLower, "hanwha") {
		return "Camera (" + vendor + ")"
	}

	// Check for network equipment vendors
	if strings.Contains(vendorLower, "mikrotik") ||
		strings.Contains(vendorLower, "ubiquiti") ||
		strings.Contains(vendorLower, "cisco") ||
		strings.Contains(vendorLower, "juniper") {
		return "Network Device (" + vendor + ")"
	}

	// Port-based heuristics
	if hasRTSP && (hasHTTP || hasHTTPS) {
		if vendor != "Unknown" && vendor != "" {
			return "Camera/NVR (" + vendor + ")"
		}
		return "Camera/NVR"
	}
	if hasRTSP {
		return "Camera"
	}
	if hasSSH && hasHTTP && hasHTTPS {
		return "Network Device"
	}
	if hasHTTP || hasHTTPS {
		return "Web Server"
	}
	if hasSSH {
		return "SSH Server"
	}

	if vendor != "Unknown" && vendor != "" {
		return vendor
	}

	return "Unknown"
}
