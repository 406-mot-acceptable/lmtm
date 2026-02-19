package discovery

import (
	"regexp"
	"strings"

	"github.com/406-mot-acceptable/lmtm/internal/gateway"
)

// MikroTik terse ARP format (from `/ip arp print terse`):
//   0 DH 10.0.0.2 AA:BB:CC:DD:EE:FF bridge1
//   1  D 10.0.0.3 11:22:33:44:55:66 ether1
//
// Fields: index, flags, IP, MAC, interface.
// Flags may be empty, single char, or multi-char (D, DH, etc.).
var mikrotikARPRe = regexp.MustCompile(
	`^\s*\d+\s+([A-Z]*)\s+` + // index + flags
		`(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})\s+` + // IP
		`([0-9A-Fa-f]{2}(?::[0-9A-Fa-f]{2}){5})\s+` + // MAC
		`(\S+)`, // interface
)

// ParseMikroTikARP parses the output of `/ip arp print terse`.
func ParseMikroTikARP(output string) []gateway.ARPEntry {
	var entries []gateway.ARPEntry
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		m := mikrotikARPRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		entries = append(entries, gateway.ARPEntry{
			Flags: m[1],
			IP:    m[2],
			MAC:   strings.ToUpper(m[3]),
			Iface: m[4],
		})
	}
	return entries
}

// Linux `ip neigh show` format:
//   10.0.0.2 dev eth1 lladdr AA:BB:CC:DD:EE:FF REACHABLE
//   10.0.0.3 dev eth1 lladdr 11:22:33:44:55:66 STALE
//   10.0.0.4 dev eth1  FAILED
//
// Lines with FAILED or INCOMPLETE have no lladdr and are skipped.
var linuxARPRe = regexp.MustCompile(
	`^(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})\s+` + // IP
		`dev\s+(\S+)\s+` + // interface
		`lladdr\s+([0-9A-Fa-f]{2}(?::[0-9A-Fa-f]{2}){5})\s+` + // MAC
		`(\S+)`, // state (REACHABLE, STALE, DELAY, etc.)
)

// ParseLinuxARP parses the output of `ip neigh show`.
func ParseLinuxARP(output string) []gateway.ARPEntry {
	var entries []gateway.ARPEntry
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		m := linuxARPRe.FindStringSubmatch(line)
		if m == nil {
			// Skip lines without lladdr (FAILED, INCOMPLETE).
			continue
		}
		entries = append(entries, gateway.ARPEntry{
			IP:    m[1],
			Iface: m[2],
			MAC:   strings.ToUpper(m[3]),
			Flags: m[4],
		})
	}
	return entries
}
