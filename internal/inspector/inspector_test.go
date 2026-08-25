package inspector

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

	report, err := RunSecurityScan(context.Background(), &AwsRepository{})
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

	report, err := RunSecurityScan(context.Background(), &AwsRepository{})
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

func TestRunSecurityScanKeepsPartialFindingsWithWarning(t *testing.T) {
	withInspectorScanners(t, []InspectorScanner{{
		Name: "partial",
		Run: func(context.Context, *AwsRepository) ([]SecurityFinding, error) {
			return []SecurityFinding{{RuleID: "kept", ResourceID: "resource-1"}}, fmt.Errorf("resource-2: access denied")
		},
	}})

	report, err := RunSecurityScan(context.Background(), &AwsRepository{})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Findings) != 1 || report.Findings[0].ResourceID != "resource-1" {
		t.Fatalf("expected partial finding to be preserved, got %+v", report.Findings)
	}
	if len(report.Warnings) != 1 || report.Warnings[0] != "partial: resource-2: access denied" {
		t.Fatalf("expected partial warning, got %v", report.Warnings)
	}
}

func TestRunSecurityScanStopsWhenContextCanceledBetweenScanners(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runCount := 0
	withInspectorScanners(t, []InspectorScanner{
		{
			Name: "first",
			Run: func(context.Context, *AwsRepository) ([]SecurityFinding, error) {
				runCount++
				cancel()
				return nil, nil
			},
		},
		{
			Name: "second",
			Run: func(context.Context, *AwsRepository) ([]SecurityFinding, error) {
				runCount++
				return nil, nil
			},
		},
	})

	report, err := RunSecurityScan(ctx, &AwsRepository{})
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
	if err != context.Canceled {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if report != nil {
		t.Fatalf("expected nil report on cancellation, got %+v", report)
	}
	if runCount != 1 {
		t.Fatalf("expected only the first scanner to run, got %d runs", runCount)
	}
}
