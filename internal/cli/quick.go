package cli

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/jaco/tunneler/internal/ssh"
)

var (
	gateway     string
	username    string
	gatewayType string
	subnet      string
	first10     bool
	rangeStart  int
	rangeEnd    int
	device      string

	quickCmd = &cobra.Command{
		Use:   "quick",
		Short: "Quick tunnel without config file",
		Long: `Create SSH tunnels quickly without editing config file.

Examples:
  # Tunnel to first 10 devices (10.0.0.2-11)
  tunneler quick --gateway 102.217.230.33 --first-10

  # Tunnel to specific range
  tunneler quick --gateway 102.217.230.33 --range 5-15

  # Tunnel to single device
  tunneler quick --gateway 102.217.230.33 --device 10.0.0.5`,
		RunE: runQuick,
	}
)

func init() {
	quickCmd.Flags().StringVarP(&gateway, "gateway", "g", "", "Gateway IP address (required)")
	quickCmd.Flags().StringVarP(&username, "user", "u", "dato", "SSH username")
	quickCmd.Flags().StringVarP(&gatewayType, "type", "t", "ubiquiti", "Gateway type (ubiquiti or mikrotik)")
	quickCmd.Flags().StringVarP(&subnet, "subnet", "s", "10.0.0", "Device subnet (e.g., 10.0.0)")
	quickCmd.Flags().BoolVar(&first10, "first-10", false, "Tunnel to first 10 devices (10.0.0.2-11)")
	quickCmd.Flags().StringVar(&device, "device", "", "Single device IP to tunnel")
	quickCmd.Flags().IntVar(&rangeStart, "range-start", 0, "Device range start")
	quickCmd.Flags().IntVar(&rangeEnd, "range-end", 0, "Device range end")

	quickCmd.MarkFlagRequired("gateway")

	rootCmd.AddCommand(quickCmd)
}

func runQuick(cmd *cobra.Command, args []string) error {
	// Determine device range
	start := 2
	end := 11

	if first10 {
		start = 2
		end = 11
	} else if device != "" {
		// Parse single device
		fmt.Printf("Single device mode not yet implemented\n")
		return nil
	} else if rangeStart > 0 && rangeEnd > 0 {
		start = rangeStart
		end = rangeEnd
	}

	// Prompt for password
	fmt.Printf("Password for %s@%s: ", username, gateway)
	passwordBytes, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		return fmt.Errorf("failed to read password: %w", err)
	}
	password := string(passwordBytes)

	// Create tunnel manager
	manager := ssh.NewManager()
	manager.SetPassword(password)

	// Status callback
	statusCallback := func(info *ssh.TunnelInfo) {
		symbol := getStatusSymbol(info.Status)
		fmt.Printf("%s %s (%s:%d) -> localhost:%d\n",
			symbol,
			info.DeviceName,
			info.DeviceIP,
			info.DevicePort,
			info.LocalPort,
		)
	}

	fmt.Printf("\nConnecting to %s...\n", gateway)
	fmt.Printf("Creating tunnels for %s.%d-%d\n\n", subnet, start, end)

	// Quick connect
	err = manager.QuickConnect(gateway, username, password, gatewayType, subnet, start, end, statusCallback)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	fmt.Println("\n✓ Tunnels active. Press Ctrl+C to disconnect.")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\n\nDisconnecting...")
	manager.DisconnectAll()
	fmt.Println("✓ Disconnected")

	return nil
}

func getStatusSymbol(status ssh.TunnelStatus) string {
	switch status {
	case ssh.StatusActive:
		return "✓"
	case ssh.StatusConnecting:
		return "⋯"
	case ssh.StatusFailed:
		return "✗"
	case ssh.StatusDisconnected:
		return "○"
	default:
		return "?"
	}
}
