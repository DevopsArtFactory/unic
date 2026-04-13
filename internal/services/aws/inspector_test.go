package aws

import (
	"context"
	"fmt"
	"sync"
	"testing"
)

var inspectorScannerTestMu sync.Mutex

func withInspectorScanners(t *testing.T, scanners []InspectorScanner) {
	t.Helper()
	inspectorScannerTestMu.Lock()
	securityInspectorScannersMu.Lock()
	original := append([]InspectorScanner(nil), securityInspectorScanners...)
	securityInspectorScanners = append([]InspectorScanner(nil), scanners...)
	securityInspectorScannersMu.Unlock()
	t.Cleanup(func() {
		securityInspectorScannersMu.Lock()
		securityInspectorScanners = original
		securityInspectorScannersMu.Unlock()
		inspectorScannerTestMu.Unlock()
	})
}

func TestRunSecurityScanSortsFindingsBySeverityAndResource(t *testing.T) {
	withInspectorScanners(t, []InspectorScanner{
		{
			Name: "network",
			Run: func(context.Context, *AwsRepository) ([]SecurityFinding, error) {
				return []SecurityFinding{
					{
						RuleID:       "sg-public-ssh",
						RuleName:     "SSH exposed to the internet",
						Severity:     RuleSeverityHigh,
						ResourceType: "SecurityGroup",
						ResourceID:   "sg-zeta",
					},
					{
						RuleID:       "sg-public-rdp",
						RuleName:     "RDP exposed to the internet",
						Severity:     RuleSeverityCritical,
						ResourceType: "SecurityGroup",
						ResourceID:   "sg-alpha",
					},
				}, nil
			},
		},
		{
			Name: "database",
			Run: func(context.Context, *AwsRepository) ([]SecurityFinding, error) {
				return []SecurityFinding{
					{
						RuleID:       "rds-no-backup",
						RuleName:     "Automated backups disabled",
						Severity:     RuleSeverityHigh,
						ResourceType: "RDS",
						ResourceID:   "db-alpha",
					},
				}, nil
			},
		},
	})

	report, err := (&AwsRepository{}).RunSecurityScan(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.ScannerCount != 2 {
		t.Fatalf("expected 2 scanners, got %d", report.ScannerCount)
	}
	if len(report.Findings) != 3 {
		t.Fatalf("expected 3 findings, got %d", len(report.Findings))
	}
	if report.Findings[0].Severity != RuleSeverityCritical || report.Findings[0].ResourceID != "sg-alpha" {
		t.Fatalf("expected critical finding first, got %+v", report.Findings[0])
	}
	if report.Findings[1].ResourceID != "db-alpha" {
		t.Fatalf("expected high finding with alphabetical resource sort second, got %+v", report.Findings[1])
	}
	if report.Findings[2].ResourceID != "sg-zeta" {
		t.Fatalf("expected remaining high finding last, got %+v", report.Findings[2])
	}
}

func TestRunSecurityScanCollectsWarningsAndContinues(t *testing.T) {
	withInspectorScanners(t, []InspectorScanner{
		{
			Name: "broken",
			Run: func(context.Context, *AwsRepository) ([]SecurityFinding, error) {
				return nil, fmt.Errorf("scan failed")
			},
		},
		{
			Name: "healthy",
			Run: func(context.Context, *AwsRepository) ([]SecurityFinding, error) {
				return []SecurityFinding{
					{
						RuleID:       "secret-rotation",
						RuleName:     "Secret rotation overdue",
						Severity:     RuleSeverityMedium,
						ResourceType: "Secret",
						ResourceID:   "/prod/db",
					},
				}, nil
			},
		},
	})

	report, err := (&AwsRepository{}).RunSecurityScan(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(report.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(report.Findings))
	}
	if len(report.Warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(report.Warnings))
	}
	if report.Warnings[0] != "broken: scan failed" {
		t.Fatalf("unexpected warning text: %q", report.Warnings[0])
	}
}
