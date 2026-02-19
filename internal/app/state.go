package app

import "fmt"

// WizardState represents the current phase of the tunnel-building wizard.
type WizardState int

const (
	StateConnect   WizardState = iota // Enter IP + password
	StateDetecting                    // Auto-detect gateway type
	StateSurvey                       // Display WAN/LAN info
	StateScanning                     // Ping sweep + ARP read
	StateDevices                      // Device list + selection
	StateBuilding                     // Tunnel construction
	StateTunnels                      // Active tunnel dashboard
	StateError                        // Error recovery
)

func (s WizardState) String() string {
	switch s {
	case StateConnect:
		return "Connect"
	case StateDetecting:
		return "Detecting"
	case StateSurvey:
		return "Survey"
	case StateScanning:
		return "Scanning"
	case StateDevices:
		return "Devices"
	case StateBuilding:
		return "Building"
	case StateTunnels:
		return "Tunnels"
	case StateError:
		return "Error"
	default:
		return fmt.Sprintf("Unknown(%d)", int(s))
	}
}

// ValidTransition checks whether moving from one state to another is allowed.
// The state machine enforces the wizard flow:
//
//	Connect -> Detecting -> Survey -> Scanning -> Devices -> Building -> Tunnels
//	                \-> Error                       \-> Error
//	Tunnels -> Connect (disconnect)
//	Error   -> Connect (start over)
//	Error   -> previous state (retry, handled by caller)
func ValidTransition(from, to WizardState) bool {
	switch from {
	case StateConnect:
		return to == StateDetecting
	case StateDetecting:
		return to == StateSurvey || to == StateError
	case StateSurvey:
		return to == StateScanning
	case StateScanning:
		return to == StateDevices || to == StateError
	case StateDevices:
		return to == StateBuilding
	case StateBuilding:
		return to == StateTunnels
	case StateTunnels:
		return to == StateConnect
	case StateError:
		// Error can go back to Connect or retry to the previous state.
		// Since we don't track the previous state here, we allow any
		// non-error state as a valid target from Error.
		return to != StateError
	default:
		return false
	}
}
