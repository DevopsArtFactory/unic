package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"unic/internal/app"
	"unic/internal/cli"
	"unic/internal/config"
)

func main() {
	rootCmd := cli.NewRootCmd()

	rootCmd.RunE = func(cmd *cobra.Command, args []string) error {
		configDir, err := os.UserConfigDir()
		if err != nil {
			return fmt.Errorf("could not determine config directory: %w", err)
		}
		configPath := filepath.Join(configDir, "unic", "config.yaml")

		if err := config.EnsureConfigExists(configPath); err != nil {
			return fmt.Errorf("config error: %w", err)
		}

		_, err = config.Load(cli.Profile(), cli.Region(), configPath)
		if err != nil {
			return fmt.Errorf("config load error: %w", err)
		}

		p := tea.NewProgram(app.New(), tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			return fmt.Errorf("TUI error: %w", err)
		}
		return nil
	}

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
