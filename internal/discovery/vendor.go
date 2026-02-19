package discovery

import "github.com/endobit/oui"

// LookupVendor returns the manufacturer name for a MAC address.
// The endobit/oui package uses a compiled-in IEEE OUI database,
// so no runtime initialization or file loading is needed.
// Returns "Unknown" if the OUI prefix is not found.
func LookupVendor(mac string) string {
	vendor := oui.Vendor(mac)
	if vendor == "" {
		return "Unknown"
	}
	return vendor
}
