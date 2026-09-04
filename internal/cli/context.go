package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
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
	listContextsFn       = config.Contexts
	buildSyncPlanFn      = auth.BuildContextSyncPlan
	applySyncPlanFn      = auth.ApplyContextSyncPlan
)

func newContextCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "context",
		Short: "Manage authentication contexts",
	}

	cmd.AddCommand(newContextSetupCmd())
	cmd.AddCommand(newContextOrderCmd())
	cmd.AddCommand(newContextUnsetCmd())
	cmd.AddCommand(newContextSyncCmd())
	return cmd
}

func newContextSyncCmd() *cobra.Command {
	var prune, dryRun, apply, jsonOutput bool
	var confirmation string
	cmd := &cobra.Command{
		Use:   "sync [base-context]",
		Short: "Plan or apply contexts from the accounts and roles visible to an SSO base context",
		Long: "Build a plan for the accounts and roles visible to an SSO base context. Apply it only with --apply and --confirm. " +
			"Existing contexts are never rewritten; sync-managed contexts whose account/role disappeared are removed only when applying with --prune.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRun && apply {
				return writeMutationError(cmd, fmt.Errorf("--dry-run and --apply cannot be used together"), jsonOutput, "")
			}
			configPath, err := defaultPathFn()
			if err != nil {
				return writeMutationError(cmd, err, jsonOutput, "")
			}
			if err := ensureConfigExistsFn(configPath); err != nil {
				return writeMutationError(cmd, err, jsonOutput, "")
			}
			base, err := resolveSyncBase(configPath, args)
			if err != nil {
				return writeMutationError(cmd, err, jsonOutput, "")
			}
			plan, err := buildSyncPlanFn(context.Background(), configPath, base)
			if err != nil {
				return writeMutationError(cmd, err, jsonOutput, "sso:ListAccounts,sso:ListAccountRoles")
			}
			contract := contextSyncMutationPlan(plan, prune)
			if !apply {
				if jsonOutput {
					return json.NewEncoder(cmd.OutOrStdout()).Encode(contract)
				}
				printSyncPlan(cmd.OutOrStdout(), plan, prune, true)
				return nil
			}
			if confirmation != base.Name {
				err := fmt.Errorf("%w: pass --confirm %s", errConfirmationMismatch, base.Name)
				return writeMutationError(cmd, err, jsonOutput, "")
			}
			if err := applySyncPlanFn(configPath, plan, prune); err != nil {
				return writeMutationError(cmd, err, jsonOutput, "")
			}
			if jsonOutput {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(MutationResult{
					Version: "v1", Operation: contract.Operation, Resource: contract.Resource,
					Changed: len(plan.Add) > 0 || prune && len(plan.Orphans) > 0, Before: contract.Before, After: contract.After,
					RollbackHint: "restore the previous config file from backup or version control",
				})
			}
			printSyncPlan(cmd.OutOrStdout(), plan, prune, false)
			return nil
		},
	}
	cmd.Flags().BoolVar(&prune, "prune", false, "remove sync-managed contexts whose SSO account/role is no longer visible")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show the sync plan without writing config")
	cmd.Flags().BoolVar(&apply, "apply", false, "apply the sync plan")
	cmd.Flags().StringVar(&confirmation, "confirm", "", "confirm execution by repeating the base context name")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "write the plan, result, or error as JSON")
	return cmd
}

type contextSyncSnapshot struct {
	Contexts []string `json:"contexts"`
}

func contextSyncMutationPlan(plan auth.ContextSyncPlan, prune bool) MutationPlan {
	before := append(append([]string{}, plan.Keep...), plan.Orphans...)
	after := append([]string{}, plan.Keep...)
	for _, entry := range plan.Add {
		after = append(after, entry.Name)
	}
	if !prune {
		after = append(after, plan.Orphans...)
	}
	sort.Strings(before)
	sort.Strings(after)
	return MutationPlan{
		Version: "v1", Operation: "context.sync", Resource: plan.Base,
		Before: contextSyncSnapshot{Contexts: before}, After: contextSyncSnapshot{Contexts: after}, Confirmation: plan.Base,
	}
}

func writeMutationError(cmd *cobra.Command, err error, jsonOutput bool, permission string) error {
	if jsonOutput {
		if encodeErr := json.NewEncoder(cmd.OutOrStdout()).Encode(mutationError(err, permission)); encodeErr != nil {
			return encodeErr
		}
	}
	return err
}

func resolveSyncBase(configPath string, args []string) (config.ContextInfo, error) {
	contexts, err := listContextsFn(configPath)
	if err != nil {
		return config.ContextInfo{}, err
	}

	if len(args) == 1 {
		for _, ctx := range contexts {
			if ctx.Name != args[0] {
				continue
			}
			if !auth.IsBaseSSOContext(ctx) {
				return config.ContextInfo{}, fmt.Errorf("context %q is not an SSO base context", ctx.Name)
			}
			return ctx, nil
		}
		return config.ContextInfo{}, fmt.Errorf("context %q not found", args[0])
	}

	var bases []config.ContextInfo
	for _, ctx := range contexts {
		if auth.IsBaseSSOContext(ctx) {
			bases = append(bases, ctx)
		}
	}
	switch len(bases) {
	case 0:
		return config.ContextInfo{}, fmt.Errorf("no SSO base context found; add one with sso_start_url and no sso_account_id/sso_role_name")
	case 1:
		return bases[0], nil
	default:
		names := make([]string, 0, len(bases))
		for _, base := range bases {
			names = append(names, base.Name)
		}
		return config.ContextInfo{}, fmt.Errorf("multiple SSO base contexts found (%s); pass one as an argument", strings.Join(names, ", "))
	}
}

func printSyncPlan(out io.Writer, plan auth.ContextSyncPlan, prune, dryRun bool) {
	for _, entry := range plan.Add {
		fmt.Fprintf(out, "add:    %s\n", entry.Name)
	}
	for _, name := range plan.Orphans {
		action := "orphan"
		if prune {
			action = "remove"
		}
		fmt.Fprintf(out, "%s: %s\n", action, name)
	}
	suffix := ""
	if dryRun {
		suffix = " (dry run, nothing written)"
	}
	fmt.Fprintf(out, "sync %s: %d added, %d kept, %d orphaned%s\n", plan.Base, len(plan.Add), len(plan.Keep), len(plan.Orphans), suffix)
	if !prune && len(plan.Orphans) > 0 {
		fmt.Fprintln(out, "use --prune to remove orphaned sync-managed contexts")
	}
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
	if order < 1 {
		return 0, fmt.Errorf("order must be a positive integer, got %d", order)
	}
	return order, nil
}

func newContextSetupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "setup",
		Short: "Interactively select a context and copy shell exports to the clipboard",
		Long:  "Select a context, resolve any required SSO account/role and resource region, set it as current, and copy shell export commands to the clipboard.",
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
