package cli

import (
	"context"
	"errors"

	"github.com/spf13/cobra"

	"unic/internal/config"
	"unic/internal/inspector"
)

type inspectFindingJSON struct {
	RuleID         string `json:"rule_id"`
	RuleName       string `json:"rule_name"`
	Severity       string `json:"severity"`
	ResourceType   string `json:"resource_type"`
	ResourceID     string `json:"resource_id"`
	Summary        string `json:"summary"`
	Recommendation string `json:"recommendation"`
}

type inspectReportJSON struct {
	ScannedAt      string               `json:"scanned_at"`
	ScannerCount   int                  `json:"scanner_count"`
	FindingCount   int                  `json:"finding_count"`
	SeverityCounts map[string]int       `json:"severity_counts"`
	Findings       []inspectFindingJSON `json:"findings"`
}

// runSecurityInspector is a variable so tests can exercise the command's
// contract without reaching AWS. It reuses resourceRepository so the agent
// surface resolves its AWS context exactly like every other automation
// command.
var runSecurityInspector = func(ctx context.Context) (*inspector.SecurityScanReport, error) {
	repo, err := resourceRepository(ctx)
	if err != nil {
		return nil, err
	}
	configPath, err := config.DefaultPath()
	if err != nil {
		return nil, err
	}
	cfg, err := config.Load(Profile(), Region(), configPath)
	if err != nil {
		return nil, err
	}
	return inspector.RunSecurityScan(ctx, repo, inspector.SecurityScanOptions{
		ACMExpiryWindowDays: cfg.ACMExpiryWindowDays,
		RequiredTags:        cfg.RequiredTags,
	})
}

func newInspectCmd() *cobra.Command {
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "inspect",
		Short: "Run the built-in security and cost/waste rule packs as JSON",
		Args:  cobra.NoArgs,
		Annotations: map[string]string{
			annotationReadOnly:      "true",
			annotationOutputVersion: "v1",
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !jsonOutput {
				return errors.New("this automation command supports JSON output only")
			}
			// --checklist is a root persistent flag, so it is inherited here and
			// advertised by `unic schema inspect`. Running the security packs
			// while silently ignoring it would hand back the wrong report, so
			// reject it until the checklist output contract is defined.
			if Checklist() != "" {
				return errors.New("--checklist is not exposed to automation yet; this command runs the built-in security and cost/waste rule packs only")
			}
			report, err := runSecurityInspector(cmd.Context())
			if err != nil {
				return err
			}
			if report == nil {
				return errors.New("security scan returned no report")
			}
			return writeResourceJSON(cmd, newInspectReportJSON(report), true, report.Warnings)
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", true, "Emit stable machine-readable JSON")
	return cmd
}

func newInspectReportJSON(report *inspector.SecurityScanReport) inspectReportJSON {
	findings := make([]inspectFindingJSON, 0, len(report.Findings))
	severityCounts := map[string]int{}
	for _, finding := range report.Findings {
		severity := string(finding.Severity)
		severityCounts[severity]++
		findings = append(findings, inspectFindingJSON{
			RuleID:         finding.RuleID,
			RuleName:       finding.RuleName,
			Severity:       severity,
			ResourceType:   finding.ResourceType,
			ResourceID:     finding.ResourceID,
			Summary:        finding.Summary,
			Recommendation: finding.Recommendation,
		})
	}
	return inspectReportJSON{
		ScannedAt:      report.ScannedAt.UTC().Format("2006-01-02T15:04:05Z"),
		ScannerCount:   report.ScannerCount,
		FindingCount:   len(findings),
		SeverityCounts: severityCounts,
		Findings:       findings,
	}
}
