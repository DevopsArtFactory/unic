package app

import (
	"testing"
	"unicode/utf8"

	"unic/internal/inspector"
)

func TestInspectorEnsureWorkflowsKeepsExistingWorkflows(t *testing.T) {
	im := inspectorModel{
		checklistPath: "new-checklist.yaml",
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

	if len(im.workflows) == 0 {
		t.Fatal("expected empty workflow list to be populated")
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
