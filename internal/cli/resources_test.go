package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	awsservice "unic/internal/services/aws"
)

func TestBackupVaultsJSONIncludesDataWarningsAndPagination(t *testing.T) {
	original := loadBackupVaults
	defer func() { loadBackupVaults = original }()
	loadBackupVaults = func(context.Context) ([]awsservice.BackupVault, []error, error) {
		return []awsservice.BackupVault{{
			ARN: "arn:aws:backup:us-east-1:123456789012:backup-vault:prod", Name: "prod",
			Region: "us-east-1", State: "AVAILABLE", Type: "BACKUP_VAULT",
			RecoveryPointCount: 7, Locked: true,
		}}, []error{errors.New("second page unavailable")}, nil
	}

	cmd := NewRootCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"resources", "backup-vaults", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	var result struct {
		SchemaVersion string `json:"schema_version"`
		Data          []struct {
			Name string `json:"name"`
		} `json:"data"`
		Warnings   []string `json:"warnings"`
		Pagination struct {
			Complete  bool    `json:"complete"`
			NextToken *string `json:"next_token"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON output: %v\n%s", err, stdout.String())
	}
	if result.SchemaVersion != "v1" || len(result.Data) != 1 || result.Data[0].Name != "prod" {
		t.Fatalf("unexpected data envelope: %+v", result)
	}
	if len(result.Warnings) != 1 || result.Warnings[0] != "second page unavailable" {
		t.Fatalf("unexpected warnings: %v", result.Warnings)
	}
	if result.Pagination.Complete || result.Pagination.NextToken != nil {
		t.Fatalf("unexpected pagination: %+v", result.Pagination)
	}
	if stderr.Len() != 0 {
		t.Fatalf("JSON mode must keep diagnostics in the envelope, got stderr %q", stderr.String())
	}
}

func TestBackupVaultsHumanOutputSendsWarningsToStderr(t *testing.T) {
	original := loadBackupVaults
	defer func() { loadBackupVaults = original }()
	loadBackupVaults = func(context.Context) ([]awsservice.BackupVault, []error, error) {
		return []awsservice.BackupVault{{Name: "prod", State: "AVAILABLE", Region: "us-east-1"}}, []error{errors.New("partial result")}, nil
	}

	cmd := NewRootCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"resources", "backup-vaults"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "prod") || !strings.Contains(stderr.String(), "warning: partial result") {
		t.Fatalf("unexpected output: stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}
