package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

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
	setContextOrderFn    = config.SetContextOrder
	setContextOrdersFn   = config.SetContextOrders
	reorderContextsFn    = reorderContexts
	copyClipboardFn      = clipboard.Copy
)

func newContextCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "context",
		Short: "Manage authentication contexts",
	}

	cmd.AddCommand(newContextSetupCmd())
	cmd.AddCommand(newContextOrderCmd())
	cmd.AddCommand(newContextUnsetCmd())
	return cmd
}

func newContextOrderCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "order [context-name] [number]",
		Short: "Set the display order for a context",
		Args:  cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			configPath, err := defaultPathFn()
			if err != nil {
				return err
			}
			if err := ensureConfigExistsFn(configPath); err != nil {
				return err
			}

			message, err := applyContextOrder(configPath, cmd.InOrStdin(), cmd.ErrOrStderr(), args)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.ErrOrStderr(), message)
			return nil
		},
	}
}

func applyContextOrder(configPath string, in io.Reader, errOut io.Writer, args []string) (string, error) {
	switch len(args) {
	case 2:
		order, err := parseContextOrder(args[1])
		if err != nil {
			return "", err
		}
		if err := setContextOrderFn(configPath, args[0], order); err != nil {
			return "", err
		}
		return fmt.Sprintf("Updated context %s to order %d.", args[0], order), nil
	case 0:
		names, err := reorderContextsFn(configPath, in, errOut)
		if err != nil {
			return "", err
		}
		if err := setContextOrdersFn(configPath, names); err != nil {
			return "", err
		}
		return "Updated context order.", nil
	default:
		return "", fmt.Errorf("expected either no arguments or <context-name> <number>")
	}
}

func parseContextOrder(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	order := 0
	if _, err := fmt.Sscanf(raw, "%d", &order); err != nil {
		return 0, fmt.Errorf("invalid order %q", raw)
	}
	return order, nil
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
