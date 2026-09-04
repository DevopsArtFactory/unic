package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"unic/internal/clipboard"
	"unic/internal/config"
	awsservice "unic/internal/services/aws"
)

var (
	ecrDefaultPathFn        = config.DefaultPath
	ecrEnsureConfigExistsFn = config.EnsureConfigExists
	ecrLoadConfigFn         = config.Load
	ecrResolveRegistryURIFn = awsservice.ResolvePrivateECRRegistryURI
	ecrBuildLoginCommandFn  = awsservice.BuildECRLoginCommand
	ecrCopyClipboardFn      = clipboard.Copy
)

func newECRCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ecr",
		Short: "ECR helper commands",
	}

	cmd.AddCommand(newECRLoginCmd())
	return cmd
}

func newECRLoginCmd() *cobra.Command {
	var runtime string
	var copyToClipboard bool

	cmd := &cobra.Command{
		Use:         "login",
		Short:       "Print an ECR login command for the current AWS context",
		Annotations: map[string]string{annotationReadOnly: "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			configPath, err := ecrDefaultPathFn()
			if err != nil {
				return err
			}
			if err := ecrEnsureConfigExistsFn(configPath); err != nil {
				return err
			}

			cfg, err := ecrLoadConfigFn(Profile(), Region(), configPath)
			if err != nil {
				return err
			}

			parsedRuntime, err := awsservice.ParseECRRuntime(runtime)
			if err != nil {
				return err
			}

			registryURI, _, err := ecrResolveRegistryURIFn(context.Background(), cfg)
			if err != nil {
				return err
			}

			command, err := ecrBuildLoginCommandFn(registryURI, cfg.Region, parsedRuntime)
			if err != nil {
				return err
			}

			if _, err := fmt.Fprintln(cmd.OutOrStdout(), command); err != nil {
				return err
			}

			if copyToClipboard {
				if err := ecrCopyClipboardFn(command); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "Clipboard unavailable: %v\n", err)
				} else {
					fmt.Fprintf(cmd.ErrOrStderr(), "Login command copied to clipboard for %s.\n", registryURI)
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&runtime, "runtime", string(awsservice.ECRRuntimeDocker), "Container runtime to target (docker or podman)")
	cmd.Flags().BoolVar(&copyToClipboard, "copy", false, "Copy the generated login command to the clipboard")
	return cmd
}
