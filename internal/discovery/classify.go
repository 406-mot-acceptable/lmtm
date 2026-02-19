package discovery

import "strings"

// ClassifyByVendor determines a DeviceClass from the MAC vendor string.
// Matching is case-insensitive and checks whether the vendor contains
// any of the known keywords.
func ClassifyByVendor(vendor string) DeviceClass {
	v := strings.ToLower(vendor)

	// IP cameras
	for _, kw := range []string{
		"hikvision", "dahua", "axis", "vivotek", "hanwha", "reolink",
	} {
		if strings.Contains(v, kw) {
			return ClassCamera
		}
	}

	// NVR / NAS
	for _, kw := range []string{"synology", "qnap"} {
		if strings.Contains(v, kw) {
			return ClassNVR
		}
	}

	// Routers
	if strings.Contains(v, "mikrotik") {
		return ClassRouter
	}

	// Network devices (switches, APs, firewalls)
	for _, kw := range []string{
		"ubiquiti", "ui.com", "cisco", "juniper", "aruba", "hpe",
	} {
		if strings.Contains(v, kw) {
			return ClassNetworkDevice
		}
	}

	return ClassUnknown
}
