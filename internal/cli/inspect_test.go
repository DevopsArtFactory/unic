package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"unic/internal/inspector"
)

func withStubbedInspector(t *testing.T, report *inspector.SecurityScanReport, err error) {
	t.Helper()
	original := runSecurityInspector
	t.Cleanup(func() { runSecurityInspector = original })
	runSecurityInspector = func(context.Context) (*inspector.SecurityScanReport, error) {
		return report, err
	}
}

func runInspect(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := NewRootCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetArgs(append([]string{"inspect"}, args...))
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	err := cmd.ExecuteContext(context.Background())
	return stdout.String(), err
}

func TestInspectEmitsStableEnvelopeAndSeverityCounts(t *testing.T) {
	withStubbedInspector(t, &inspector.SecurityScanReport{
		ScannerCount: 11,
		ScannedAt:    time.Date(2026, 9, 14, 3, 4, 5, 0, time.UTC),
		Warnings:     []string{"cost-waste: AccessDenied"},
		Findings: []inspector.SecurityFinding{
			{RuleID: "acm-certificate-expiry", RuleName: "ACM certificate expiring soon", Severity: inspector.RuleSeverityHigh,
				ResourceType: "ACM Certificate", ResourceID: "arn:cert", Summary: "expires in 3 days", Recommendation: "renew"},
			{RuleID: "cost-eip-unattached", RuleName: "Unattached Elastic IP", Severity: inspector.RuleSeverityMedium,
				ResourceType: "ElasticIP", ResourceID: "eipalloc-1", Summary: "not associated", Recommendation: "release"},
			{RuleID: "cost-ebs-snapshot-aged", RuleName: "Aged EBS snapshot", Severity: inspector.RuleSeverityMedium,
				ResourceType: "EBSSnapshot", ResourceID: "snap-1", Summary: "180 days old", Recommendation: "delete"},
		},
	}, nil)

	stdout, err := runInspect(t)
	if err != nil {
		t.Fatal(err)
	}

	var envelope struct {
		SchemaVersion string            `json:"schema_version"`
		Data          inspectReportJSON `json:"data"`
		Warnings      []string          `json:"warnings"`
		Pagination    struct {
			Complete bool `json:"complete"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("output is not the shared envelope: %v\n%s", err, stdout)
	}
	if envelope.SchemaVersion != "v1" || !envelope.Pagination.Complete {
		t.Fatalf("unexpected envelope: %+v", envelope)
	}
	if got := envelope.Data.ScannedAt; got != "2026-09-14T03:04:05Z" {
		t.Fatalf("expected a UTC RFC3339 timestamp, got %q", got)
	}
	if envelope.Data.ScannerCount != 11 || envelope.Data.FindingCount != 3 {
		t.Fatalf("unexpected counts: %+v", envelope.Data)
	}
	if envelope.Data.SeverityCounts["HIGH"] != 1 || envelope.Data.SeverityCounts["MEDIUM"] != 2 {
		t.Fatalf("unexpected severity counts: %+v", envelope.Data.SeverityCounts)
	}
	// Scanner warnings must reach the agent: a denied rule pack is a gap in
	// coverage, not a clean scan.
	if len(envelope.Warnings) != 1 || !strings.Contains(envelope.Warnings[0], "AccessDenied") {
		t.Fatalf("expected the scanner warning to survive, got %v", envelope.Warnings)
	}
}

func TestInspectReportsZeroFindingsAsEmptyListNotNull(t *testing.T) {
	withStubbedInspector(t, &inspector.SecurityScanReport{ScannerCount: 11, ScannedAt: time.Now()}, nil)

	stdout, err := runInspect(t)
	if err != nil {
		t.Fatal(err)
	}
	// A null findings array would make an agent branch on null vs empty.
	if !strings.Contains(stdout, `"findings":[]`) {
		t.Fatalf("expected an empty findings array, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, `"finding_count":0`) {
		t.Fatalf("expected a zero finding count, got:\n%s", stdout)
	}
}

func TestInspectSurfacesScanFailure(t *testing.T) {
	withStubbedInspector(t, nil, errors.New("AccessDenied: sts:GetCallerIdentity"))
	if _, err := runInspect(t); err == nil {
		t.Fatal("expected the scan failure to surface as a command error")
	}
}

func TestInspectRejectsNonJSONOutput(t *testing.T) {
	withStubbedInspector(t, &inspector.SecurityScanReport{}, nil)
	if _, err := runInspect(t, "--json=false"); err == nil {
		t.Fatal("expected --json=false to be rejected")
	}
}

func TestInspectAdvertisesReadOnlyContract(t *testing.T) {
	cmd, _, err := NewRootCmd().Find([]string{"inspect"})
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Annotations[annotationReadOnly] != "true" {
		t.Errorf("inspect must advertise read-only, got %q", cmd.Annotations[annotationReadOnly])
	}
	if cmd.Annotations[annotationOutputVersion] != "v1" {
		t.Errorf("inspect must advertise v1 output, got %q", cmd.Annotations[annotationOutputVersion])
	}
	if cmd.Flags().Lookup("json") == nil {
		t.Error("inspect must provide a --json flag")
	}
}

func TestInspectRejectsInheritedChecklistFlag(t *testing.T) {
	withStubbedInspector(t, &inspector.SecurityScanReport{}, nil)
	// --checklist is inherited from the root command and appears in
	// `unic schema inspect`, so it must fail loudly rather than return
	// security findings as if a checklist had run.
	cmd := NewRootCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetArgs([]string{"inspect", "--json", "--checklist", "/tmp/checklist.yaml"})
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	err := cmd.ExecuteContext(context.Background())
	if err == nil {
		t.Fatal("expected --checklist to be rejected")
	}
	if !strings.Contains(err.Error(), "not exposed to automation yet") {
		t.Fatalf("expected an explicit rejection, got %v", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no report on rejection, got %q", stdout.String())
	}
}
