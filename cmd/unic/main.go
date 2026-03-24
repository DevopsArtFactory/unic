package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"unic/internal/app"
	"unic/internal/cli"
	"unic/internal/config"
)

func main() {
	rootCmd := cli.NewRootCmd()

	rootCmd.RunE = func(cmd *cobra.Command, args []string) error {
		configPath, err := config.DefaultPath()
		if err != nil {
			return err
		}

		if err := config.EnsureConfigExists(configPath); err != nil {
			return fmt.Errorf("config error: %w", err)
		}

		cfg, err := config.Load(cli.Profile(), cli.Region(), configPath)
		if err != nil {
			return fmt.Errorf("config load error: %w", err)
		}

		p := tea.NewProgram(app.New(cfg, configPath), tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			return fmt.Errorf("TUI error: %w", err)
		}
		return nil
	}

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
