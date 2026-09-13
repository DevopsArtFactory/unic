package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"unic/internal/config"
	awsservice "unic/internal/services/aws"
)

type doctorCheck struct {
	Name        string `json:"name"`
	Status      string `json:"status"`
	Message     string `json:"message"`
	Remediation string `json:"remediation,omitempty"`
}

type doctorReport struct {
	SchemaVersion string        `json:"schema_version"`
	Status        string        `json:"status"`
	Context       string        `json:"context,omitempty"`
	Profile       string        `json:"profile,omitempty"`
	Region        string        `json:"region,omitempty"`
	Checks        []doctorCheck `json:"checks"`
}

var (
	doctorLookPath    = exec.LookPath
	doctorExecutable  = os.Executable
	doctorStat        = os.Stat
	doctorDefaultPath = config.DefaultPath
	doctorLoadConfig  = config.Load
	doctorCredentials = checkDoctorCredentials
	doctorRunMCPCheck = runMCPCheck
)

func newDoctorCmd() *cobra.Command {
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check unic, AWS, and MCP setup",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			report := buildDoctorReport(cmd.Context())
			if jsonOutput {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(report)
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
			if _, err := fmt.Fprintln(w, "STATUS\tCHECK\tDETAIL"); err != nil {
				return err
			}
			for _, check := range report.Checks {
				if _, err := fmt.Fprintf(w, "%s\t%s\t%s\n", strings.ToUpper(check.Status), check.Name, check.Message); err != nil {
					return err
				}
				if check.Remediation != "" {
					if _, err := fmt.Fprintf(w, "\t\t→ %s\n", check.Remediation); err != nil {
						return err
					}
				}
			}
			return w.Flush()
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Emit stable machine-readable JSON")
	return cmd
}

func buildDoctorReport(ctx context.Context) doctorReport {
	report := doctorReport{SchemaVersion: "v1", Status: "pass", Checks: make([]doctorCheck, 0, 3)}
	mcpPath, err := findMCPBinary()
	if err != nil {
		report.Checks = append(report.Checks, doctorCheck{Name: "unic-mcp", Status: "fail", Message: "binary not found", Remediation: "install or upgrade unic so unic-mcp is beside unic or on PATH"})
	} else {
		checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		mcpVersion, toolCount, checkErr := doctorRunMCPCheck(checkCtx, mcpPath)
		cancel()
		if checkErr != nil {
			report.Checks = append(report.Checks, doctorCheck{Name: "unic-mcp", Status: "fail", Message: checkErr.Error(), Remediation: "run unic-mcp from a terminal and verify the client can execute the same path"})
		} else if Version != "dev" && mcpVersion != Version {
			report.Checks = append(report.Checks, doctorCheck{Name: "unic-mcp", Status: "fail", Message: fmt.Sprintf("version mismatch: unic=%s unic-mcp=%s", Version, mcpVersion), Remediation: "upgrade unic and unic-mcp together"})
		} else {
			report.Checks = append(report.Checks, doctorCheck{Name: "unic-mcp", Status: "pass", Message: fmt.Sprintf("%s; %d tools available", mcpVersion, toolCount)})
		}
	}

	configPath, pathErr := doctorDefaultPath()
	if pathErr != nil {
		report.Checks = append(report.Checks, doctorCheck{Name: "config", Status: "fail", Message: pathErr.Error()})
		return finalizeDoctorReport(report)
	}
	if _, err := doctorStat(configPath); errors.Is(err, os.ErrNotExist) {
		report.Checks = append(report.Checks, doctorCheck{Name: "config", Status: "warn", Message: "configuration not initialized", Remediation: "run unic init, then unic context setup"})
		return finalizeDoctorReport(report)
	} else if err != nil {
		report.Checks = append(report.Checks, doctorCheck{Name: "config", Status: "fail", Message: err.Error()})
		return finalizeDoctorReport(report)
	}
	cfg, err := doctorLoadConfig(Profile(), Region(), configPath)
	if err != nil {
		report.Checks = append(report.Checks, doctorCheck{Name: "config", Status: "fail", Message: err.Error(), Remediation: "fix the unic config file or rerun unic init --force"})
		return finalizeDoctorReport(report)
	}
	report.Context, report.Profile, report.Region = cfg.ContextName, cfg.Profile, cfg.Region
	report.Checks = append(report.Checks, doctorCheck{Name: "config", Status: "pass", Message: configPath})

	credentialCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	err = doctorCredentials(credentialCtx, cfg)
	cancel()
	if err != nil {
		report.Checks = append(report.Checks, doctorCheck{Name: "aws-credentials", Status: "fail", Message: "credentials unavailable", Remediation: "sign in to the selected AWS profile or run unic context setup"})
	} else {
		report.Checks = append(report.Checks, doctorCheck{Name: "aws-credentials", Status: "pass", Message: "credential chain resolved"})
	}
	return finalizeDoctorReport(report)
}

func checkDoctorCredentials(ctx context.Context, cfg *config.Config) error {
	repo, err := awsservice.NewAwsRepository(ctx, cfg)
	if err != nil {
		return err
	}
	_, err = repo.ResolveCredentialEnv(ctx)
	return err
}

func finalizeDoctorReport(report doctorReport) doctorReport {
	for _, check := range report.Checks {
		if check.Status == "fail" {
			report.Status = "fail"
			return report
		}
		if check.Status == "warn" {
			report.Status = "warn"
		}
	}
	return report
}

func findMCPBinary() (string, error) {
	if path, err := doctorLookPath("unic-mcp"); err == nil {
		return path, nil
	}
	executable, err := doctorExecutable()
	if err != nil {
		return "", err
	}
	path := filepath.Join(filepath.Dir(executable), "unic-mcp")
	if _, err := doctorStat(path); err != nil {
		return "", err
	}
	return path, nil
}

func runMCPCheck(ctx context.Context, path string) (string, int, error) {
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"unic-doctor","version":"1"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`,
	}, "\n") + "\n"
	command := exec.CommandContext(ctx, path)
	command.Stdin = strings.NewReader(input)
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		return "", 0, fmt.Errorf("MCP handshake failed: %w", err)
	}
	var version string
	toolCount := -1
	scanner := bufio.NewScanner(&stdout)
	for scanner.Scan() {
		var envelope struct {
			ID     int `json:"id"`
			Result struct {
				ServerInfo struct {
					Version string `json:"version"`
				} `json:"serverInfo"`
				Tools []json.RawMessage `json:"tools"`
			} `json:"result"`
		}
		if json.Unmarshal(scanner.Bytes(), &envelope) != nil {
			continue
		}
		switch envelope.ID {
		case 1:
			version = envelope.Result.ServerInfo.Version
		case 2:
			toolCount = len(envelope.Result.Tools)
		}
	}
	if err := scanner.Err(); err != nil {
		return "", 0, err
	}
	if version == "" || toolCount < 0 {
		return "", 0, fmt.Errorf("MCP returned incomplete handshake: %s", strings.TrimSpace(stderr.String()))
	}
	return version, toolCount, nil
}
