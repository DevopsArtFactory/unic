package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"unic/internal/auth"
	"unic/internal/clipboard"
	"unic/internal/config"
)

var (
	defaultPathFn        = config.DefaultPath
	ensureConfigExistsFn = config.EnsureConfigExists
	setupContextFn       = auth.SetupContext
	buildEnvFn           = auth.BuildEnvExports
	buildCleanupEnvFn    = auth.BuildEnvCleanupCommands
	loadNamedContextFn   = config.LoadNamedContext
	loadConfigFn         = config.Load
	unsetCurrentFn       = config.UnsetCurrent
	copyClipboardFn      = clipboard.Copy
)

func newContextCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "context",
		Short: "Manage authentication contexts",
	}

	cmd.AddCommand(newContextSetupCmd())
	cmd.AddCommand(newContextUnsetCmd())
	return cmd
}

func newContextSetupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "setup",
		Short: "Interactively select a context and copy shell exports to the clipboard",
		Long:  "Select a context, resolve any required SSO account/role, set it as current, and copy shell export commands to the clipboard.",
		RunE: func(cmd *cobra.Command, args []string) error {
			configPath, err := defaultPathFn()
			if err != nil {
				return err
			}
			if err := ensureConfigExistsFn(configPath); err != nil {
				return err
			}

			exports, err := setupContextFn(context.Background(), configPath, cmd.InOrStdin(), cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			if err := copyClipboardFn(exports); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Clipboard unavailable: %v\n", err)
			} else {
				fmt.Fprintln(cmd.ErrOrStderr(), "Exports copied to clipboard.")
			}
			return nil
		},
	}
}

func newContextUnsetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "unset",
		Short: "Clear the current context and copy shell cleanup commands to the clipboard",
		Long:  "Remove the current context selection from config and copy AWS environment cleanup commands to the clipboard.",
		RunE: func(cmd *cobra.Command, args []string) error {
			configPath, err := defaultPathFn()
			if err != nil {
				return err
			}
			if err := ensureConfigExistsFn(configPath); err != nil {
				return err
			}
			if err := unsetCurrentFn(configPath); err != nil {
				return err
			}

			exports := buildCleanupEnvFn()

			if err := copyClipboardFn(exports); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Clipboard unavailable: %v\n", err)
			} else {
				fmt.Fprintln(cmd.ErrOrStderr(), "Cleanup commands copied to clipboard.")
			}
			fmt.Fprintln(cmd.ErrOrStderr(), "Current context cleared.")
			return nil
		},
	}
}

func newEnvCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "env [context-name]",
		Short: "Print shell exports for the current or named context",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			configPath, err := defaultPathFn()
			if err != nil {
				return err
			}
			if err := ensureConfigExistsFn(configPath); err != nil {
				return err
			}

			var cfg *config.Config
			if len(args) == 1 {
				cfg, err = loadNamedContextFn(configPath, args[0])
			} else {
				cfg, err = loadConfigFn(Profile(), Region(), configPath)
			}
			if err != nil {
				return err
			}

			exports, err := buildEnvFn(context.Background(), cfg)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), exports)
			return err
		},
	}
}
