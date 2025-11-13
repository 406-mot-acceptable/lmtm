package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	cfgFile string
	rootCmd = &cobra.Command{
		Use:   "tunneler",
		Short: "SSH tunnel manager for accessing devices behind NAT",
		Long: `The Tunneler - A fast SSH tunnel manager with TUI support.

Quickly create multiple SSH port forwards through gateway devices
to access internal equipment at customer sites.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Default behavior: launch TUI
			return runTUI(cfgFile)
		},
	}
)

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default is ./tunneler.yaml)")
}

// Execute runs the root command
func Execute() error {
	return rootCmd.Execute()
}

func er(msg interface{}) {
	fmt.Fprintln(os.Stderr, "Error:", msg)
	os.Exit(1)
}
