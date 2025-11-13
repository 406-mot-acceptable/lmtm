package cli

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jaco/tunneler/internal/config"
	"github.com/jaco/tunneler/internal/tui"
)

func runTUI(cfgFile string) error {
	// Load config if provided
	var cfg *config.Config
	var err error

	if cfgFile != "" {
		cfg, err = config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
	} else {
		// Try default locations
		for _, path := range []string{"./tunneler.yaml", "~/.config/tunneler/config.yaml"} {
			cfg, err = config.Load(path)
			if err == nil {
				break
			}
		}

		if cfg == nil {
			return fmt.Errorf("no config file found. Use --config to specify one, or create ./tunneler.yaml")
		}
	}

	// Create and run TUI
	model := tui.NewModel(cfg)
	p := tea.NewProgram(model, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	return nil
}
