package app

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"unic/internal/inspector"
)

func TestInspectorWarningsStayBoundedWithPartialFindings(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.height = 14
	m.screen = screenInspectorResults
	m.inspector.report = &inspector.SecurityScanReport{
		ScannedAt:    time.Now(),
		ScannerCount: 1,
		Warnings:     []string{"cost-waste: failed lookup 1\nfailed lookup 2\nfailed lookup 3"},
	}
	m.inspector.findings = []inspector.SecurityFinding{{
		RuleName:     "Empty target group",
		Severity:     inspector.RuleSeverityLow,
		ResourceType: "ELBTargetGroup",
		ResourceID:   "target-group-1",
	}}

	view := stripANSI(m.inspector.viewResults(m))
	if !strings.Contains(view, "Warnings: 1 rule pack errors") || !strings.Contains(view, "failed lookup 1") ||
		strings.Contains(view, "failed lookup 2") || !strings.Contains(view, "target-group-1") ||
		!strings.Contains(view, "esc: Inspector mode") {
		t.Fatalf("expected bounded warnings with findings and help visible:\n%s", view)
	}
}

func TestInspectorEnsureWorkflowsKeepsExistingWorkflows(t *testing.T) {
	im := inspectorModel{
		workflows: []inspector.Workflow{
			{
				Kind:        inspector.WorkflowSecurity,
				Title:       "Existing Workflow",
				Description: "already loaded",
				Available:   true,
			},
		},
	}

	im.ensureWorkflows()

	if len(im.workflows) != 1 {
		t.Fatalf("expected existing workflow list to be preserved, got %d workflows", len(im.workflows))
	}
	if im.workflows[0].Title != "Existing Workflow" {
		t.Fatalf("expected existing workflow to be preserved, got %#v", im.workflows[0])
	}
}

func TestInspectorEnsureWorkflowsRefreshesEmptyWorkflows(t *testing.T) {
	im := inspectorModel{}

	im.ensureWorkflows()

	if len(im.workflows) != 2 {
		t.Fatalf("expected empty workflow list to be populated with 2 workflows, got %d", len(im.workflows))
	}
}

func TestInspectorShortenHandlesUnicodeSafely(t *testing.T) {
	tests := []struct {
		name  string
		value string
		width int
	}{
		{name: "cjk narrow", value: "보안점검", width: 2},
		{name: "emoji narrow", value: "🔒security", width: 3},
		{name: "cjk with ellipsis", value: "보안점검결과", width: 6},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := inspectorShorten(tc.value, tc.width)
			if !utf8.ValidString(got) {
				t.Fatalf("expected valid UTF-8, got %q", got)
			}
		})
	}
}

func TestInspectorShortenUsesEllipsisForWiderColumns(t *testing.T) {
	got := inspectorShorten("security-group-public-ssh", 10)
	if got != "securit..." {
		t.Fatalf("expected truncated value with ellipsis, got %q", got)
	}
}
