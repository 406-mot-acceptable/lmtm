package gateway

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

type ubiquitiGateway struct {
	run CommandRunner
}

func newUbiquiti(run CommandRunner) *ubiquitiGateway {
	return &ubiquitiGateway{run: run}
}

func (g *ubiquitiGateway) Type() Type { return TypeUbiquiti }

func (g *ubiquitiGateway) Identity(ctx context.Context) (string, error) {
	out, err := g.run(ctx, "hostname")
	if err != nil {
		return "", fmt.Errorf("ubiquiti identity: %w", err)
	}
	return strings.TrimSpace(out), nil
}

func (g *ubiquitiGateway) WANInfo(ctx context.Context) (*WANConfig, error) {
	cfg := &WANConfig{}

	// Strategy 1: airOS system.cfg -- has explicit interface roles.
	out, err := g.run(ctx, "cat /tmp/system.cfg 2>/dev/null")
	if err == nil {
		wanIface, wanIP := parseSystemCfgWAN(out)
		if wanIP != "" {
			cfg.PublicIP = wanIP
			cfg.InterfaceName = wanIface
		}
	}

	// Strategy 2: Try PPPoE/WAN interfaces with `ip addr show`.
	if cfg.PublicIP == "" {
		for _, iface := range []string{"ppp0", "pppoe0", "eth0"} {
			out, err := g.run(ctx, fmt.Sprintf("ip addr show %s 2>/dev/null", iface))
			if err != nil {
				continue
			}
			ip := parseLinuxInetAddr(out)
			if ip != "" && !isPrivateIPv4(stripCIDRSuffix(ip)) {
				cfg.PublicIP = ip
				cfg.InterfaceName = iface
				break
			}
		}
	}

	// Strategy 3: ifconfig fallback (airOS BusyBox).
	if cfg.PublicIP == "" {
		for _, iface := range []string{"ppp0", "pppoe0", "eth0"} {
			out, err := g.run(ctx, fmt.Sprintf("ifconfig %s 2>/dev/null", iface))
			if err != nil {
				continue
			}
			ip := parseIfconfigInetAddr(out)
			if ip != "" && !isPrivateIPv4(ip) {
				cfg.PublicIP = ip
				cfg.InterfaceName = iface
				break
			}
		}
	}

	// Get default route gateway.
	out, err = g.run(ctx, "ip route show default 2>/dev/null")
	if err == nil {
		cfg.Gateway = parseLinuxDefaultGateway(out)
	}

	if cfg.PublicIP == "" && cfg.Gateway == "" {
		return nil, fmt.Errorf("ubiquiti WANInfo: could not determine WAN configuration")
	}
	return cfg, nil
}

func (g *ubiquitiGateway) LANInfo(ctx context.Context) (*LANConfig, error) {
	cfg := &LANConfig{}

	// Strategy 1: airOS system.cfg -- has explicit interface roles and DHCP.
	out, err := g.run(ctx, "cat /tmp/system.cfg 2>/dev/null")
	if err == nil {
		lanIface, lanIP, lanMask := parseSystemCfgLAN(out)
		if lanIP != "" {
			cidr := lanIP + cidrFromMask(lanMask)
			cfg.InterfaceName = lanIface
			cfg.GatewayIP = lanIP
			cfg.CIDR = cidr
			cfg.Subnet = subnetFromCIDR(cidr)
			// DHCP from system.cfg.
			cfg.DHCPStart, cfg.DHCPEnd = parseSystemCfgDHCP(out)
		}
	}

	// Strategy 2: Dynamic discovery via `ip -o addr show` (EdgeOS).
	if cfg.GatewayIP == "" {
		out, err := g.run(ctx, "ip -o addr show 2>/dev/null")
		if err == nil {
			// Detect if a PPP/PPPoE interface exists -- if so, eth0 is LAN.
			hasPPP := strings.Contains(out, "ppp0") || strings.Contains(out, "pppoe0")
			for _, candidate := range discoverLANInterfaces(out, hasPPP) {
				cfg.InterfaceName = candidate.iface
				cfg.GatewayIP = stripCIDRSuffix(candidate.addr)
				cfg.CIDR = candidate.addr
				cfg.Subnet = subnetFromCIDR(candidate.addr)
				break
			}
		}
	}

	// Strategy 3: ifconfig fallback (airOS BusyBox).
	if cfg.GatewayIP == "" {
		for _, iface := range []string{"eth0", "br0", "eth1", "switch0"} {
			out, err := g.run(ctx, fmt.Sprintf("ifconfig %s 2>/dev/null", iface))
			if err != nil {
				continue
			}
			ip := parseIfconfigInetAddr(out)
			mask := parseIfconfigMask(out)
			if ip != "" && isPrivateIPv4(ip) {
				cidr := ip + cidrFromMask(mask)
				cfg.InterfaceName = iface
				cfg.GatewayIP = ip
				cfg.CIDR = cidr
				cfg.Subnet = subnetFromCIDR(cidr)
				break
			}
		}
	}

	// Strategy 4: Hardcoded interface names with `ip addr show` (legacy).
	if cfg.GatewayIP == "" {
		for _, iface := range []string{"br0", "eth1", "switch0"} {
			out, err := g.run(ctx, fmt.Sprintf("ip addr show %s 2>/dev/null", iface))
			if err != nil {
				continue
			}
			ip := parseLinuxInetAddr(out)
			if ip != "" {
				cfg.InterfaceName = iface
				cfg.GatewayIP = stripCIDRSuffix(ip)
				cfg.CIDR = ip
				cfg.Subnet = subnetFromCIDR(ip)
				break
			}
		}
	}

	if cfg.GatewayIP == "" {
		return nil, fmt.Errorf("ubiquiti LANInfo: could not determine LAN configuration")
	}

	// DHCP: try EdgeOS sources if system.cfg didn't provide it.
	if cfg.DHCPStart == "" {
		out, err = g.run(ctx, "cat /etc/dnsmasq.d/dhcpd.conf 2>/dev/null || cat /config/dhcpd.conf 2>/dev/null")
		if err == nil {
			cfg.DHCPStart, cfg.DHCPEnd = parseDnsmasqRange(out)
		}
	}
	if cfg.DHCPStart == "" {
		out, err = g.run(ctx, "cat /config/config.boot 2>/dev/null")
		if err == nil {
			cfg.DHCPStart, cfg.DHCPEnd = parseConfigBootDHCP(out, cfg.Subnet)
		}
	}

	return cfg, nil
}

func (g *ubiquitiGateway) FloodPing(ctx context.Context, subnet string) error {
	if err := ValidateSubnet(subnet); err != nil {
		return fmt.Errorf("ubiquiti flood ping: %w", err)
	}
	// Parallel ping sweep of the /24 to populate ARP table.
	cmd := fmt.Sprintf(
		"for i in $(seq 1 254); do ping -c1 -W1 %s.$i &>/dev/null & done; wait",
		subnet,
	)
	_, err := g.run(ctx, cmd)
	if err != nil {
		return fmt.Errorf("ubiquiti flood ping: %w", err)
	}
	return nil
}

// neighRe matches `ip neigh show` output.
// Example: "10.0.0.2 dev eth1 lladdr AA:BB:CC:DD:EE:FF REACHABLE"
var neighRe = regexp.MustCompile(
	`(?m)^(\d+\.\d+\.\d+\.\d+)\s+dev\s+(\S+)\s+lladdr\s+([0-9A-Fa-f:]{17})\s+(\S+)`,
)

func (g *ubiquitiGateway) ARPTable(ctx context.Context, subnet string) ([]ARPEntry, error) {
	if subnet != "" {
		if err := ValidateSubnet(subnet); err != nil {
			return nil, fmt.Errorf("ubiquiti ARP: %w", err)
		}
	}

	// Try `ip neigh show` first (EdgeOS).
	out, err := g.run(ctx, "ip neigh show 2>/dev/null")
	if err == nil && strings.TrimSpace(out) != "" {
		matches := neighRe.FindAllStringSubmatch(out, -1)
		if len(matches) == 0 {
			return parseNeighFallback(out, subnet), nil
		}
		var entries []ARPEntry
		for _, m := range matches {
			ip := m[1]
			if subnet != "" && !strings.HasPrefix(ip, subnet+".") {
				continue
			}
			state := m[4]
			if strings.EqualFold(state, "FAILED") {
				continue
			}
			entries = append(entries, ARPEntry{
				IP:    ip,
				Iface: m[2],
				MAC:   strings.ToUpper(m[3]),
				Flags: state,
			})
		}
		return entries, nil
	}

	// Fallback: `arp -a` (airOS BusyBox).
	out, err = g.run(ctx, "arp -a 2>/dev/null")
	if err != nil {
		return nil, fmt.Errorf("ubiquiti ARP: neither ip neigh nor arp available")
	}
	return parseBusyBoxARP(out, subnet), nil
}

// ---------------------------------------------------------------------------
// airOS system.cfg parsers
// ---------------------------------------------------------------------------

// parseSystemCfgWAN finds the WAN interface from /tmp/system.cfg.
// PPPoE (ppp.1) takes priority, then looks for interfaces without role=lan.
func parseSystemCfgWAN(cfg string) (iface, ip string) {
	lines := strings.Split(cfg, "\n")
	kv := make(map[string]string)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if k, v, ok := strings.Cut(line, "="); ok {
			kv[k] = v
		}
	}

	// Check for PPPoE -- the WAN IP will be on the ppp0 interface at runtime.
	if kv["ppp.1.status"] == "enabled" {
		return "ppp0", "" // IP is dynamic, WANInfo will get it from ifconfig
	}

	return "", ""
}

// parseSystemCfgLAN finds the LAN interface from /tmp/system.cfg.
// Looks for netconf entries with role=lan.
func parseSystemCfgLAN(cfg string) (iface, ip, mask string) {
	lines := strings.Split(cfg, "\n")
	kv := make(map[string]string)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if k, v, ok := strings.Cut(line, "="); ok {
			kv[k] = v
		}
	}

	// Scan netconf entries for role=lan.
	for i := 1; i <= 10; i++ {
		prefix := fmt.Sprintf("netconf.%d", i)
		role := kv[prefix+".role"]
		dev := kv[prefix+".devname"]
		ipAddr := kv[prefix+".ip"]
		netmask := kv[prefix+".netmask"]
		if role == "lan" && ipAddr != "" {
			return dev, ipAddr, netmask
		}
	}

	// Fallback: look for DHCP device (airOS puts DHCP on the LAN interface).
	if dev := kv["dhcpd.1.devname"]; dev != "" {
		// Find the matching netconf entry for this device.
		for i := 1; i <= 10; i++ {
			prefix := fmt.Sprintf("netconf.%d", i)
			if kv[prefix+".devname"] == dev {
				return dev, kv[prefix+".ip"], kv[prefix+".netmask"]
			}
		}
	}

	return "", "", ""
}

// parseSystemCfgDHCP extracts DHCP range from /tmp/system.cfg.
func parseSystemCfgDHCP(cfg string) (start, end string) {
	lines := strings.Split(cfg, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if k, v, ok := strings.Cut(line, "="); ok {
			switch k {
			case "dhcpd.1.start":
				start = v
			case "dhcpd.1.end":
				end = v
			}
		}
	}
	return start, end
}

// cidrFromMask converts a dotted netmask to CIDR suffix.
// E.g., "255.255.255.0" -> "/24". Returns "" if mask is empty or unparseable.
func cidrFromMask(mask string) string {
	if mask == "" {
		return "/24" // default assumption
	}
	var a, b, c, d int
	n, _ := fmt.Sscanf(mask, "%d.%d.%d.%d", &a, &b, &c, &d)
	if n != 4 {
		return "/24"
	}
	bits := 0
	for _, octet := range []int{a, b, c, d} {
		for i := 7; i >= 0; i-- {
			if octet&(1<<uint(i)) != 0 {
				bits++
			} else {
				return fmt.Sprintf("/%d", bits)
			}
		}
	}
	return fmt.Sprintf("/%d", bits)
}

// ---------------------------------------------------------------------------
// ifconfig parsers (BusyBox / airOS)
// ---------------------------------------------------------------------------

// ifconfigInetRe matches "inet addr:10.0.0.1" from BusyBox ifconfig.
var ifconfigInetRe = regexp.MustCompile(`inet addr:(\d+\.\d+\.\d+\.\d+)`)

// parseIfconfigInetAddr extracts the inet address from ifconfig output.
func parseIfconfigInetAddr(out string) string {
	m := ifconfigInetRe.FindStringSubmatch(out)
	if m == nil {
		return ""
	}
	return m[1]
}

// ifconfigMaskRe matches "Mask:255.255.255.0" from BusyBox ifconfig.
var ifconfigMaskRe = regexp.MustCompile(`Mask:(\d+\.\d+\.\d+\.\d+)`)

// parseIfconfigMask extracts the netmask from ifconfig output.
func parseIfconfigMask(out string) string {
	m := ifconfigMaskRe.FindStringSubmatch(out)
	if m == nil {
		return ""
	}
	return m[1]
}

// ---------------------------------------------------------------------------
// BusyBox arp parser
// ---------------------------------------------------------------------------

// busyBoxARPRe matches `arp -a` output.
// Example: "? (10.0.0.5) at AA:BB:CC:DD:EE:FF [ether] on eth0"
var busyBoxARPRe = regexp.MustCompile(
	`\((\d+\.\d+\.\d+\.\d+)\)\s+at\s+([0-9A-Fa-f:]{17})\s+\[(\w+)\]\s+on\s+(\S+)`,
)

// parseBusyBoxARP parses `arp -a` output from BusyBox.
func parseBusyBoxARP(out, subnet string) []ARPEntry {
	var entries []ARPEntry
	for _, m := range busyBoxARPRe.FindAllStringSubmatch(out, -1) {
		ip := m[1]
		mac := m[2]
		if subnet != "" && !strings.HasPrefix(ip, subnet+".") {
			continue
		}
		entries = append(entries, ARPEntry{
			IP:    ip,
			MAC:   strings.ToUpper(mac),
			Iface: m[4],
		})
	}
	return entries
}

// ---------------------------------------------------------------------------
// Linux output parsers (iproute2 / EdgeOS)
// ---------------------------------------------------------------------------

// inetRe matches "inet 10.0.0.1/24" from `ip addr show`.
var inetRe = regexp.MustCompile(`inet\s+(\d+\.\d+\.\d+\.\d+(?:/\d+)?)`)

// parseLinuxInetAddr extracts the first inet address from `ip addr show` output.
func parseLinuxInetAddr(out string) string {
	m := inetRe.FindStringSubmatch(out)
	if m == nil {
		return ""
	}
	return m[1]
}

// ipOAddrRe matches lines from `ip -o addr show` output.
// Example: "3: br0    inet 192.168.1.1/24 brd 192.168.1.255 scope global br0"
var ipOAddrRe = regexp.MustCompile(`\d+:\s+(\S+)\s+inet\s+(\d+\.\d+\.\d+\.\d+(?:/\d+)?)`)

// lanCandidate holds an interface name and its IP address from discovery.
type lanCandidate struct {
	iface string
	addr  string
}

// discoverLANInterfaces parses `ip -o addr show` output and returns interfaces
// with private IPv4 addresses. If hasPPP is true, eth0 is allowed as a LAN
// candidate (because PPPoE means eth0 is the LAN, not the WAN).
func discoverLANInterfaces(out string, hasPPP bool) []lanCandidate {
	wanIfaces := map[string]bool{"lo": true, "ppp0": true, "pppoe0": true}
	if !hasPPP {
		// Only exclude eth0 as WAN when there's no PPPoE.
		wanIfaces["eth0"] = true
	}
	var results []lanCandidate
	for _, m := range ipOAddrRe.FindAllStringSubmatch(out, -1) {
		iface := m[1]
		addr := m[2]
		if wanIfaces[iface] {
			continue
		}
		ip := stripCIDRSuffix(addr)
		if !isPrivateIPv4(ip) {
			continue
		}
		results = append(results, lanCandidate{iface, addr})
	}
	return results
}

// isPrivateIPv4 checks if an IP is in RFC1918 private address ranges.
func isPrivateIPv4(ip string) bool {
	var a, b int
	n, _ := fmt.Sscanf(ip, "%d.%d.", &a, &b)
	if n < 2 {
		return false
	}
	return a == 10 || (a == 172 && b >= 16 && b <= 31) || (a == 192 && b == 168)
}

// parseLinuxDefaultGateway extracts the gateway IP from `ip route show default`.
// Example: "default via 192.168.1.1 dev eth0"
func parseLinuxDefaultGateway(out string) string {
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		for i, f := range fields {
			if f == "via" && i+1 < len(fields) {
				return fields[i+1]
			}
		}
	}
	return ""
}

// parseDnsmasqRange extracts dhcp-range from dnsmasq config.
// Example line: "dhcp-range=10.0.0.100,10.0.0.200,24h"
func parseDnsmasqRange(out string) (start, end string) {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "dhcp-range") {
			continue
		}
		if _, v, ok := strings.Cut(line, "="); ok {
			parts := strings.Split(v, ",")
			if len(parts) >= 2 {
				return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
			}
		}
	}
	return "", ""
}

// parseConfigBootDHCP extracts DHCP start/stop from EdgeOS config.boot.
// Looks for lines like:
//
//	start 10.0.0.100 {
//	    stop 10.0.0.200
func parseConfigBootDHCP(out, subnet string) (start, end string) {
	lines := strings.Split(out, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "start ") {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) < 2 {
			continue
		}
		candidate := fields[1]
		// Only match ranges in the same subnet.
		if subnet != "" && !strings.HasPrefix(candidate, subnet+".") {
			continue
		}
		start = candidate
		// Look for "stop" in subsequent lines within the same block.
		for j := i + 1; j < len(lines) && j < i+10; j++ {
			inner := strings.TrimSpace(lines[j])
			if strings.HasPrefix(inner, "stop ") {
				parts := strings.Fields(inner)
				if len(parts) >= 2 {
					end = parts[1]
				}
				break
			}
			if inner == "}" {
				break
			}
		}
		if start != "" {
			return start, end
		}
	}
	return "", ""
}

// Fallback regexes for non-standard `ip neigh` output.
var (
	neighFallbackIPRe  = regexp.MustCompile(`\d+\.\d+\.\d+\.\d+`)
	neighFallbackMACRe = regexp.MustCompile(`[0-9A-Fa-f]{2}:[0-9A-Fa-f]{2}:[0-9A-Fa-f]{2}:[0-9A-Fa-f]{2}:[0-9A-Fa-f]{2}:[0-9A-Fa-f]{2}`)
)

// parseNeighFallback handles non-standard `ip neigh` output line by line.
func parseNeighFallback(out, subnet string) []ARPEntry {
	var entries []ARPEntry

	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Skip FAILED entries.
		if strings.Contains(line, "FAILED") {
			continue
		}
		ip := neighFallbackIPRe.FindString(line)
		mac := neighFallbackMACRe.FindString(line)
		if ip == "" || mac == "" {
			continue
		}
		if subnet != "" && !strings.HasPrefix(ip, subnet+".") {
			continue
		}
		entries = append(entries, ARPEntry{
			IP:  ip,
			MAC: strings.ToUpper(mac),
		})
	}
	return entries
}
