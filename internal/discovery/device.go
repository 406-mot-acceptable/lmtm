package discovery

import "fmt"

// DeviceClass categorizes a discovered network device.
type DeviceClass int

const (
	ClassUnknown       DeviceClass = iota
	ClassCamera                    // IP camera (Hikvision, Dahua, Axis, etc.)
	ClassNVR                       // Network video recorder (Synology, QNAP)
	ClassRouter                    // Gateway/router (MikroTik)
	ClassNetworkDevice             // Switch, AP, firewall (Ubiquiti, Cisco)
	ClassServer
	ClassCustom
)

func (c DeviceClass) String() string {
	switch c {
	case ClassUnknown:
		return "Unknown"
	case ClassCamera:
		return "Camera"
	case ClassNVR:
		return "NVR"
	case ClassRouter:
		return "Router"
	case ClassNetworkDevice:
		return "Network Device"
	case ClassServer:
		return "Server"
	case ClassCustom:
		return "Custom"
	default:
		return fmt.Sprintf("DeviceClass(%d)", int(c))
	}
}

// DefaultPorts returns the standard ports to tunnel for this device class.
func (c DeviceClass) DefaultPorts() []int {
	switch c {
	case ClassCamera:
		return []int{22, 80, 443, 554}
	case ClassNVR:
		return []int{22, 80, 443, 554}
	case ClassRouter:
		return []int{22, 80, 443}
	case ClassNetworkDevice:
		return []int{22, 80, 443}
	case ClassServer:
		return []int{22, 80, 443}
	default:
		return []int{80, 443}
	}
}

// DiscoveredDevice represents a host found on the gateway's LAN.
type DiscoveredDevice struct {
	IP           string
	MAC          string
	Vendor       string
	DeviceType   DeviceClass
	DefaultPorts []int
	Online       bool
}
