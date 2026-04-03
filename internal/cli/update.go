package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"unic/internal/update"
)

func newUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update unic to the latest version",
		Long:  "Check for the latest release on GitHub and replace the current binary in-place.",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Current version: %s\n", Version)
			fmt.Println("Checking for updates...")

			latest, err := update.CheckLatestVersion()
			if err != nil {
				return fmt.Errorf("failed to check for updates: %w", err)
			}

			if !update.IsNewer(Version, latest) {
				fmt.Printf("Already on the latest version (%s)\n", Version)
				return nil
			}

			fmt.Printf("New version available: %s\n", latest)

			method := update.DetectInstallMethod()
			if method == update.InstallBrew {
				fmt.Println("\nunic was installed via Homebrew. To update, run:")
				fmt.Println("  brew upgrade unic")
				return nil
			}

			fmt.Println("Downloading...")

			if err := update.DownloadAndReplace(latest); err != nil {
				return fmt.Errorf("update failed: %w", err)
			}

			fmt.Printf("Successfully updated to %s\n", latest)
			return nil
		},
	}
	return cmd
}
