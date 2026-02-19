package gateway

import (
	"context"
	"fmt"
	"regexp"
)

// Type identifies the gateway vendor.
type Type string

const (
	TypeMikroTik Type = "mikrotik"
	TypeUbiquiti Type = "ubiquiti"
	TypeUnknown  Type = "unknown"
)

// CommandRunner executes a command on the remote gateway and returns its
// combined stdout/stderr output. This is provided by the ssh package --
// gateway does NOT import ssh directly.
type CommandRunner func(ctx context.Context, cmd string) (string, error)

// Gateway abstracts vendor-specific operations on a network gateway.
type Gateway interface {
	// Type returns the detected gateway vendor.
	Type() Type

	// Identity returns the device hostname / identity string.
	Identity(ctx context.Context) (string, error)

	// WANInfo returns the WAN-facing interface configuration.
	WANInfo(ctx context.Context) (*WANConfig, error)

	// LANInfo returns the LAN-side configuration including DHCP range.
	LANInfo(ctx context.Context) (*LANConfig, error)

	// FloodPing sends a broadcast or sweep ping to populate the ARP table.
	FloodPing(ctx context.Context, subnet string) error

	// ARPTable returns the current ARP entries, optionally filtered to a subnet.
	ARPTable(ctx context.Context, subnet string) ([]ARPEntry, error)
}

// WANConfig holds the WAN-facing interface details.
type WANConfig struct {
	PublicIP      string
	InterfaceName string
	Gateway       string
}

// LANConfig holds the LAN-side network details.
type LANConfig struct {
	Subnet        string // e.g., "10.0.0"
	CIDR          string // e.g., "10.0.0.0/24"
	GatewayIP     string // e.g., "10.0.0.1"
	DHCPStart     string
	DHCPEnd       string
	InterfaceName string
}

// ARPEntry represents a single row from the gateway ARP table.
type ARPEntry struct {
	IP    string
	MAC   string
	Iface string
	Flags string // "D", "DH", etc. for MikroTik
}

// subnetRe matches a 3-octet subnet prefix like "10.0.0" or "192.168.1".
// Each octet must be 0-255 (regex allows 0-999 -- ValidateSubnet enforces range).
var subnetRe = regexp.MustCompile(`^\d{1,3}\.\d{1,3}\.\d{1,3}$`)

// ValidateSubnet checks that a subnet string is exactly 3 decimal octets
// (e.g., "10.0.0") with no shell metacharacters. This MUST be called before
// interpolating subnet into any command string to prevent command injection.
func ValidateSubnet(subnet string) error {
	if !subnetRe.MatchString(subnet) {
		return fmt.Errorf("invalid subnet format %q: must be 3 decimal octets (e.g., 10.0.0)", subnet)
	}
	// Verify each octet is in 0-255 range.
	var a, b, c int
	n, _ := fmt.Sscanf(subnet, "%d.%d.%d", &a, &b, &c)
	if n != 3 || a > 255 || b > 255 || c > 255 {
		return fmt.Errorf("invalid subnet %q: octets must be 0-255", subnet)
	}
	return nil
}
