package discovery

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"

	"github.com/406-mot-acceptable/lmtm/internal/gateway"
)

// ProgressFunc is called during scanning with the number of devices processed so far.
type ProgressFunc func(found int)

// Scanner orchestrates device discovery on a gateway's LAN.
type Scanner struct {
	gw gateway.Gateway
}

// NewScanner creates a Scanner that discovers devices through the given gateway.
func NewScanner(gw gateway.Gateway) *Scanner {
	return &Scanner{gw: gw}
}

// Scan performs full device discovery on the given subnet.
//
// Flow:
//  1. Flood ping to populate the ARP table (failure is non-fatal).
//  2. Read the ARP table (required).
//  3. For each entry: vendor lookup, classification, build DiscoveredDevice.
//  4. Sort by IP (last octet, numerically).
func (s *Scanner) Scan(ctx context.Context, subnet string, progress ProgressFunc) ([]DiscoveredDevice, error) {
	// Step 1: flood ping to populate ARP -- best effort.
	_ = s.gw.FloodPing(ctx, subnet)

	// Step 2: read ARP table -- required.
	arpEntries, err := s.gw.ARPTable(ctx, subnet)
	if err != nil {
		return nil, fmt.Errorf("ARP table read failed: %w", err)
	}

	// Step 3: build device list from ARP entries.
	devices := make([]DiscoveredDevice, 0, len(arpEntries))
	for i, entry := range arpEntries {
		vendor := LookupVendor(entry.MAC)
		class := ClassifyByVendor(vendor)

		devices = append(devices, DiscoveredDevice{
			IP:           entry.IP,
			MAC:          entry.MAC,
			Vendor:       vendor,
			DeviceType:   class,
			DefaultPorts: class.DefaultPorts(),
			Online:       true,
		})

		if progress != nil {
			progress(i + 1)
		}
	}

	// Step 4: sort by last octet of IP address.
	sort.Slice(devices, func(i, j int) bool {
		return parseLastOctet(devices[i].IP) < parseLastOctet(devices[j].IP)
	})

	return devices, nil
}

// parseLastOctet extracts the last octet from an IPv4 address as an integer.
// Returns 0 if the IP cannot be parsed.
func parseLastOctet(ip string) int {
	parsed := net.ParseIP(ip)
	if parsed != nil {
		if v4 := parsed.To4(); v4 != nil {
			return int(v4[3])
		}
	}

	// Fallback: split on dot.
	parts := strings.Split(ip, ".")
	if len(parts) == 4 {
		if n, err := strconv.Atoi(parts[3]); err == nil {
			return n
		}
	}
	return 0
}
