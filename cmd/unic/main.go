package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"unic/internal/app"
	"unic/internal/cli"
	"unic/internal/config"
	uniclog "unic/internal/log"
)

func main() {
	rootCmd := cli.NewRootCmd()

	rootCmd.RunE = func(cmd *cobra.Command, args []string) error {
		if err := uniclog.Init(cli.Verbose()); err != nil {
			return fmt.Errorf("logger init error: %w", err)
		}
		defer uniclog.Close()

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

		uniclog.Info("config", "config loaded",
			"profile", cfg.Profile,
			"region", cfg.Region,
			"context", cfg.ContextName,
			"auth_type", string(cfg.AuthType),
		)

		p := tea.NewProgram(app.New(cfg, configPath, cli.Version), tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			return fmt.Errorf("TUI error: %w", err)
		}
		return nil
	}

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
