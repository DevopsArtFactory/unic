package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"unic/internal/config"
	awsservice "unic/internal/services/aws"
)

type jsonEnvelope[T any] struct {
	SchemaVersion string         `json:"schema_version"`
	Data          T              `json:"data"`
	Warnings      []string       `json:"warnings"`
	Pagination    jsonPagination `json:"pagination"`
}

type jsonPagination struct {
	Complete  bool    `json:"complete"`
	NextToken *string `json:"next_token"`
}

type backupVaultJSON struct {
	ARN                string `json:"arn"`
	Name               string `json:"name"`
	Region             string `json:"region"`
	State              string `json:"state"`
	Type               string `json:"type"`
	EncryptionKeyARN   string `json:"encryption_key_arn,omitempty"`
	RecoveryPointCount int64  `json:"recovery_point_count"`
	Locked             bool   `json:"locked"`
}

type cloudFormationStackJSON struct {
	ID                    string                    `json:"id"`
	Name                  string                    `json:"name"`
	Description           string                    `json:"description,omitempty"`
	Status                string                    `json:"status"`
	StatusReason          string                    `json:"status_reason,omitempty"`
	DriftStatus           string                    `json:"drift_status"`
	Region                string                    `json:"region"`
	LastDriftCheck        string                    `json:"last_drift_check,omitempty"`
	CreatedAt             string                    `json:"created_at"`
	UpdatedAt             string                    `json:"updated_at,omitempty"`
	TerminationProtection bool                      `json:"termination_protection"`
	Parameters            []cloudFormationValueJSON `json:"parameters"`
	Outputs               []cloudFormationValueJSON `json:"outputs"`
}

type cloudFormationValueJSON struct {
	Key         string `json:"key"`
	Value       string `json:"value"`
	Description string `json:"description,omitempty"`
	ExportName  string `json:"export_name,omitempty"`
}

func cloudFormationValuesJSON(values []awsservice.CloudFormationValue) []cloudFormationValueJSON {
	result := make([]cloudFormationValueJSON, 0, len(values))
	for _, value := range values {
		result = append(result, cloudFormationValueJSON{
			Key: value.Key, Value: value.Value, Description: value.Description, ExportName: value.ExportName,
		})
	}
	return result
}

func cloudFormationTimeJSON(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

var loadBackupVaults = func(ctx context.Context) ([]awsservice.BackupVault, []error, error) {
	configPath, err := config.DefaultPath()
	if err != nil {
		return nil, nil, err
	}
	if err := config.EnsureConfigExists(configPath); err != nil {
		return nil, nil, err
	}
	cfg, err := config.Load(Profile(), Region(), configPath)
	if err != nil {
		return nil, nil, err
	}
	repo, err := awsservice.NewAwsRepository(ctx, cfg)
	if err != nil {
		return nil, nil, err
	}
	return repo.ListBackupVaults(ctx)
}

func newResourcesCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "resources", Short: "Read-only resource queries for automation"}
	cmd.AddCommand(newBackupVaultsCmd())
	cmd.AddCommand(newEC2InstancesCmd(), newRDSInstancesCmd(), newCloudFormationStacksCmd(), newAlarmsCmd())
	cmd.AddCommand(newECSRolloutCmd(), newCloudTrailEventsCmd(), newELBTargetHealthCmd())
	return cmd
}

func newBackupVaultsCmd() *cobra.Command {
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "backup-vaults",
		Short: "List AWS Backup vaults",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			vaults, warningErrors, err := loadBackupVaults(cmd.Context())
			if err != nil {
				return err
			}
			warnings := make([]string, 0, len(warningErrors))
			for _, warning := range warningErrors {
				warnings = append(warnings, warning.Error())
			}
			if jsonOutput {
				data := make([]backupVaultJSON, 0, len(vaults))
				for _, vault := range vaults {
					data = append(data, backupVaultJSON{
						ARN: vault.ARN, Name: vault.Name, Region: vault.Region, State: vault.State,
						Type: vault.Type, EncryptionKeyARN: vault.EncryptionKeyARN,
						RecoveryPointCount: vault.RecoveryPointCount, Locked: vault.Locked,
					})
				}
				return json.NewEncoder(cmd.OutOrStdout()).Encode(jsonEnvelope[[]backupVaultJSON]{
					SchemaVersion: "v1", Data: data, Warnings: warnings,
					Pagination: jsonPagination{Complete: len(warnings) == 0},
				})
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
			if _, err := fmt.Fprintln(w, "NAME\tSTATE\tTYPE\tRECOVERY POINTS\tREGION"); err != nil {
				return err
			}
			for _, vault := range vaults {
				if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n", vault.Name, vault.State, vault.Type, vault.RecoveryPointCount, vault.Region); err != nil {
					return err
				}
			}
			if err := w.Flush(); err != nil {
				return err
			}
			for _, warning := range warnings {
				if _, err := fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s\n", warning); err != nil {
					return err
				}
			}
			return nil
		},
	}
	cmd.Annotations = map[string]string{annotationReadOnly: "true", annotationOutputVersion: "v1"}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Emit stable machine-readable JSON")
	return cmd
}
