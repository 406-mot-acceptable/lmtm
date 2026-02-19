package portmap

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
)

// PortMapping describes a single local-to-remote port forward.
type PortMapping struct {
	LocalPort  int
	RemoteHost string
	RemotePort int
}

// PortBase returns the base local port for a given remote service port.
//
//	443 -> 4430
//	80  -> 8030
//	22  -> 2230
//	554 -> 5540
//
// For unrecognized ports, it returns 10000 + remotePort*10 to keep them
// in a distinct range.
func PortBase(remotePort int) int {
	switch remotePort {
	case 443:
		return 4430
	case 80:
		return 8030
	case 22:
		return 2230
	case 554:
		return 5540
	default:
		return 10000 + remotePort*10
	}
}

// LocalPort calculates the local port for a given remote IP and service port.
// It adds the last octet of the IP to the port base.
// For example: remoteIP="192.168.1.5", remotePort=443 -> 4430 + 5 = 4435
func LocalPort(remoteIP string, remotePort int) int {
	return PortBase(remotePort) + lastOctet(remoteIP)
}

// PortAllocator tracks allocated local ports and handles collisions.
type PortAllocator struct {
	mu        sync.Mutex
	allocated map[int]PortMapping
}

// NewPortAllocator creates a PortAllocator ready for use.
func NewPortAllocator() *PortAllocator {
	return &PortAllocator{
		allocated: make(map[int]PortMapping),
	}
}

// Allocate assigns a local port for the given remote host and port.
// It uses the standard formula (PortBase + last octet) and bumps to the
// next available port if a collision is detected.
func (pa *PortAllocator) Allocate(remoteIP string, remotePort int) (int, error) {
	pa.mu.Lock()
	defer pa.mu.Unlock()

	port := LocalPort(remoteIP, remotePort)

	// Try up to 256 consecutive ports to find an open slot.
	for i := 0; i < 256; i++ {
		candidate := port + i
		if candidate > 65535 {
			break
		}
		if _, taken := pa.allocated[candidate]; !taken {
			pa.allocated[candidate] = PortMapping{
				LocalPort:  candidate,
				RemoteHost: remoteIP,
				RemotePort: remotePort,
			}
			return candidate, nil
		}
	}

	return 0, fmt.Errorf("no available local port for %s:%d", remoteIP, remotePort)
}

// Release frees a previously allocated local port.
func (pa *PortAllocator) Release(localPort int) {
	pa.mu.Lock()
	defer pa.mu.Unlock()
	delete(pa.allocated, localPort)
}

// Mappings returns a copy of all current port mappings.
func (pa *PortAllocator) Mappings() []PortMapping {
	pa.mu.Lock()
	defer pa.mu.Unlock()

	result := make([]PortMapping, 0, len(pa.allocated))
	for _, m := range pa.allocated {
		result = append(result, m)
	}
	return result
}

// lastOctet extracts the last octet from an IPv4 address string.
func lastOctet(ip string) int {
	parsed := net.ParseIP(ip)
	if parsed != nil {
		v4 := parsed.To4()
		if v4 != nil {
			return int(v4[3])
		}
	}

	// Fallback: split on dot and parse the last segment.
	parts := strings.Split(ip, ".")
	if len(parts) == 4 {
		if n, err := strconv.Atoi(parts[3]); err == nil && n >= 0 && n <= 255 {
			return n
		}
	}
	return 0
}
