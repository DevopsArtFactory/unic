package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"unic/internal/config"
)

func newInitCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:         "init",
		Short:       "Initialize unic config file",
		Long:        "Create the default config file at ~/.config/unic/config.yaml (or $XDG_CONFIG_HOME/unic/config.yaml).",
		Annotations: map[string]string{annotationDestructive: "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			configPath, err := config.DefaultPath()
			if err != nil {
				return err
			}

			created, err := config.CreateConfig(configPath, force)
			if err != nil {
				return err
			}

			if created {
				fmt.Printf("Config file created: %s\n", configPath)
			} else {
				fmt.Printf("Config file already exists: %s\n", configPath)
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Overwrite existing config file")

	return cmd
}
