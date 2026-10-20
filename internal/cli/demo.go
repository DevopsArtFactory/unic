package cli

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"unic/internal/app"
)

func newDemoCmd() *cobra.Command {
	return &cobra.Command{
		Use: "demo", Short: "Explore a synthetic ECS incident offline (no AWS setup)",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{annotationReadOnly: "true", annotationDestructive: "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := tea.NewProgram(app.NewDemo(), tea.WithAltScreen(), tea.WithContext(cmd.Context())).Run()
			if err != nil {
				return fmt.Errorf("demo TUI: %w", err)
			}
			return nil
		},
	}
}
