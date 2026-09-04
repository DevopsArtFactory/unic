package cli

import (
	"github.com/spf13/cobra"
)

var (
	// Version is set via ldflags at build time.
	Version = "dev"

	profile   string
	region    string
	verbose   bool
	checklist string
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
	cmd.PersistentFlags().StringVar(&checklist, "checklist", "", "Path to a checklist YAML file for Checklist Inspector")

	cmd.AddCommand(newInitCmd())
	cmd.AddCommand(newContextCmd())
	cmd.AddCommand(newECRCmd())
	cmd.AddCommand(newResourcesCmd())
	cmd.AddCommand(newEnvCmd())
	cmd.AddCommand(newUpdateCmd())

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

// Checklist returns the CLI checklist path, or an empty string if not set.
func Checklist() string {
	return checklist
}
