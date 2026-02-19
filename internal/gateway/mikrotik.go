package gateway

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

type mikrotikGateway struct {
	run CommandRunner
}

func newMikroTik(run CommandRunner) *mikrotikGateway {
	return &mikrotikGateway{run: run}
}

func (g *mikrotikGateway) Type() Type { return TypeMikroTik }

func (g *mikrotikGateway) Identity(ctx context.Context) (string, error) {
	out, err := g.run(ctx, "/system identity print")
	if err != nil {
		return "", fmt.Errorf("mikrotik identity: %w", err)
	}
	out = strings.TrimSpace(out)
	if _, after, ok := strings.Cut(out, "name:"); ok {
		return strings.TrimSpace(after), nil
	}
	return out, nil
}

func (g *mikrotikGateway) WANInfo(ctx context.Context) (*WANConfig, error) {
	cfg := &WANConfig{}

	// Get WAN IP -- try ether1 and pppoe interfaces.
	out, err := g.run(ctx, `/ip address print terse where interface~"ether1|pppoe"`)
	if err == nil {
		cfg.PublicIP, cfg.InterfaceName = parseTerseAddress(out)
	}

	// Get default route gateway.
	out, err = g.run(ctx, `/ip route print terse where dst-address=0.0.0.0/0`)
	if err == nil {
		cfg.Gateway = parseTerseRouteGateway(out)
	}

	if cfg.PublicIP == "" && cfg.Gateway == "" {
		return nil, fmt.Errorf("mikrotik WANInfo: could not determine WAN configuration")
	}
	return cfg, nil
}

func (g *mikrotikGateway) LANInfo(ctx context.Context) (*LANConfig, error) {
	cfg := &LANConfig{}

	// Get LAN address -- try bridge and ether2.
	out, err := g.run(ctx, `/ip address print terse where interface~"bridge|ether2"`)
	if err == nil {
		ip, iface := parseTerseAddress(out)
		if ip != "" {
			cfg.InterfaceName = iface
			cfg.GatewayIP = stripCIDRSuffix(ip)
			cfg.CIDR = ip // includes /prefix
			cfg.Subnet = subnetFromCIDR(ip)
		}
	}

	if cfg.GatewayIP == "" {
		return nil, fmt.Errorf("mikrotik LANInfo: could not determine LAN configuration")
	}

	// Get DHCP pool range.
	out, err = g.run(ctx, `/ip pool print terse`)
	if err == nil {
		cfg.DHCPStart, cfg.DHCPEnd = parseTersePool(out)
	}

	return cfg, nil
}

func (g *mikrotikGateway) FloodPing(ctx context.Context, subnet string) error {
	if err := ValidateSubnet(subnet); err != nil {
		return fmt.Errorf("mikrotik flood ping: %w", err)
	}
	// MikroTik ARP is usually already populated from DHCP leases.
	// Run a lightweight sweep just in case -- scripted ping of the subnet.
	cmd := fmt.Sprintf(`:for i from=1 to=254 do={/ping %s.$i count=1 interval=0.1}`, subnet)
	_, err := g.run(ctx, cmd)
	if err != nil {
		return fmt.Errorf("mikrotik flood ping: %w", err)
	}
	return nil
}

// arpTerseRe matches terse ARP entries.
// Example line: " 0 DH 10.0.0.2 AA:BB:CC:DD:EE:FF bridge1"
// Fields: index, flags, address, mac-address, interface
var arpTerseRe = regexp.MustCompile(
	`(?m)^\s*\d+\s+(\S*)\s+(\d+\.\d+\.\d+\.\d+)\s+([0-9A-Fa-f:]{17})\s+(\S+)`,
)

func (g *mikrotikGateway) ARPTable(ctx context.Context, subnet string) ([]ARPEntry, error) {
	if subnet != "" {
		if err := ValidateSubnet(subnet); err != nil {
			return nil, fmt.Errorf("mikrotik ARP: %w", err)
		}
	}
	out, err := g.run(ctx, `/ip arp print terse where !invalid`)
	if err != nil {
		return nil, fmt.Errorf("mikrotik ARP: %w", err)
	}

	matches := arpTerseRe.FindAllStringSubmatch(out, -1)
	if len(matches) == 0 && strings.TrimSpace(out) != "" {
		// Fallback: try without regex in case format differs.
		return parseTerseARPFallback(out, subnet), nil
	}

	var entries []ARPEntry
	for _, m := range matches {
		ip := m[2]
		if subnet != "" && !strings.HasPrefix(ip, subnet+".") {
			continue
		}
		entries = append(entries, ARPEntry{
			Flags: m[1],
			IP:    ip,
			MAC:   strings.ToUpper(m[3]),
			Iface: m[4],
		})
	}
	return entries, nil
}

// ---------------------------------------------------------------------------
// MikroTik terse output parsers
// ---------------------------------------------------------------------------

// parseTerseAddress extracts the first address= and interface= from terse output.
// Terse lines look like: " 0 address=192.168.1.1/24 network=192.168.1.0 interface=bridge1"
func parseTerseAddress(out string) (addr, iface string) {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		for _, field := range strings.Fields(line) {
			if k, v, ok := strings.Cut(field, "="); ok {
				switch k {
				case "address":
					if addr == "" {
						addr = v
					}
				case "interface":
					if iface == "" {
						iface = v
					}
				}
			}
		}
		if addr != "" {
			return addr, iface
		}
	}
	return "", ""
}

// parseTerseRouteGateway extracts gateway= from terse route output.
func parseTerseRouteGateway(out string) string {
	for _, line := range strings.Split(out, "\n") {
		for _, field := range strings.Fields(line) {
			if k, v, ok := strings.Cut(field, "="); ok && k == "gateway" {
				return v
			}
		}
	}
	return ""
}

// parseTersePool extracts the first ranges= value from /ip pool print terse.
// Format: " 0 name=default-dhcp ranges=10.0.0.100-10.0.0.200"
func parseTersePool(out string) (start, end string) {
	for _, line := range strings.Split(out, "\n") {
		for _, field := range strings.Fields(line) {
			if k, v, ok := strings.Cut(field, "="); ok && k == "ranges" {
				if s, e, ok := strings.Cut(v, "-"); ok {
					return s, e
				}
				return v, ""
			}
		}
	}
	return "", ""
}

// stripCIDRSuffix removes the /prefix from an address like "10.0.0.1/24".
func stripCIDRSuffix(addr string) string {
	ip, _, _ := strings.Cut(addr, "/")
	return ip
}

// subnetFromCIDR extracts the first 3 octets from "10.0.0.1/24" -> "10.0.0".
func subnetFromCIDR(cidr string) string {
	ip := stripCIDRSuffix(cidr)
	parts := strings.Split(ip, ".")
	if len(parts) >= 3 {
		return strings.Join(parts[:3], ".")
	}
	return ip
}

// Fallback regexes for non-standard terse output.
var (
	fallbackIPRe  = regexp.MustCompile(`\d+\.\d+\.\d+\.\d+`)
	fallbackMACRe = regexp.MustCompile(`[0-9A-Fa-f]{2}:[0-9A-Fa-f]{2}:[0-9A-Fa-f]{2}:[0-9A-Fa-f]{2}:[0-9A-Fa-f]{2}:[0-9A-Fa-f]{2}`)
)

// parseTerseARPFallback handles non-standard terse formats line by line.
func parseTerseARPFallback(out, subnet string) []ARPEntry {
	var entries []ARPEntry

	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		ip := fallbackIPRe.FindString(line)
		mac := fallbackMACRe.FindString(line)
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
