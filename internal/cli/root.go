package cli

import (
	"github.com/spf13/cobra"
)

var (
	// Version is set via ldflags at build time.
	Version = "dev"

	profile string
	region  string
	verbose bool
)

func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "unic",
		Short:   "AWS DevOps TUI tool",
		Long:    "unic is a TUI tool for browsing and managing AWS resources in the terminal.",
		Version: Version,
	}

	cmd.PersistentFlags().StringVarP(&profile, "profile", "p", "", "AWS profile to use")
	cmd.PersistentFlags().StringVar(&region, "region", "", "AWS region to use")
	cmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose debug logging")

	cmd.AddCommand(newInitCmd())

	return cmd
}

// Profile returns the CLI profile flag value, or nil if not set.
func Profile() *string {
	if profile == "" {
		return nil
	}
	return &profile
}

// Region returns the CLI region flag value, or nil if not set.
func Region() *string {
	if region == "" {
		return nil
	}
	return &region
}

// Verbose returns true if --verbose was passed.
func Verbose() bool {
	return verbose
}
