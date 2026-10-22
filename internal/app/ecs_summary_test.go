package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	awsservice "unic/internal/services/aws"
)

func TestECSSummaryAllowlistAndWarnings(t *testing.T) {
	const privateMarker = "fixture-only-private-marker"
	result := awsservice.ECSInvestigation{ObservedAt: time.Date(2026, 10, 22, 1, 2, 3, 0, time.UTC),
		Rollout:      &awsservice.ECSServiceDetail{Name: privateMarker, ARN: privateMarker, Status: privateMarker, DesiredCount: 3, RunningCount: 1, TaskDefinitionARN: privateMarker, ContainerImages: []awsservice.ECSContainerImage{{Image: privateMarker}}, Events: []awsservice.ECSServiceEvent{{Message: privateMarker}}, Deployments: []awsservice.ECSDeployment{{RolloutState: "FAILED", RolloutStateReason: privateMarker, FailedTasks: 2}}},
		StoppedTasks: []awsservice.ECSStoppedTask{{ARN: privateMarker, Reason: privateMarker, Containers: []string{privateMarker}}},
		Logs:         []awsservice.ECSInvestigationLog{{Group: privateMarker, Events: []awsservice.LogEvent{{Message: privateMarker}}, Warning: "AccessDenied " + privateMarker}},
		Health:       []awsservice.ECSServiceTargetHealth{{Group: awsservice.ELBTargetGroupHealth{ARN: privateMarker, Name: privateMarker, UnhealthyCount: 1}}},
		Changes:      []awsservice.CloudTrailEvent{{Name: privateMarker, RawJSON: privateMarker, Username: privateMarker, SourceIP: privateMarker}}, Warnings: []string{"CloudTrail truncated " + privateMarker, "deadline exceeded " + privateMarker},
	}
	text := ecsSummary(result, "private-cluster", "private-service", "private-region", false)
	for _, forbidden := range []string{privateMarker, "private-cluster", "private-service", "private-region"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("leaked %s", forbidden)
		}
	}
	for _, want := range []string{"2026-10-22T01:02:03Z", "desired 3, running 1", "FAILED", "Permission denied", "limit was reached", "timed out", "YOUR_LOCAL_PROFILE", "not establish root cause", "unavailable or incomplete"} {
		if !strings.Contains(text, want) {
			t.Fatalf("lost %q: %s", want, text)
		}
	}
	text = ecsSummary(result, "private-cluster", "private-service", "private-region", true)
	if !strings.Contains(text, "private-cluster") || !strings.Contains(text, "private-service") || strings.Contains(text, privateMarker) {
		t.Fatal("optional disclosure scope incorrect")
	}
}

func TestSaveECSSummaryExclusivePrivateFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "summary.md")
	if err := saveECSSummary(path, "first"); err != nil {
		t.Fatal(err)
	}
	if err := saveECSSummary(path, "overwritten"); !errors.Is(err, os.ErrExist) {
		t.Fatalf("existing file not protected: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "first" {
		t.Fatal("existing content changed")
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("export not private: %v %v", info, err)
	}
	if err := saveECSSummary(filepath.Join(t.TempDir(), "missing", "summary.md"), "data"); err == nil {
		t.Fatal("missing-directory failure hidden")
	}
}

func TestECSSummaryPreviewActionsAndReturn(t *testing.T) {
	originalCopy, originalSave := ecsSummaryCopy, ecsSummarySave
	t.Cleanup(func() { ecsSummaryCopy, ecsSummarySave = originalCopy, originalSave })
	var copied, saved string
	ecsSummaryCopy = func(text string) error { copied = text; return errors.New("clipboard unavailable") }
	ecsSummarySave = func(path, text string) error { saved = text; return os.ErrPermission }
	m := New(testConfig(), "", "dev")
	m.screen = screenECSInvestigation
	m.width = 80
	m.height = 24
	m.ecs.selectedCluster = &awsservice.ECSCluster{ARN: "target-cluster"}
	m.ecs.selectedDetail = &awsservice.ECSServiceDetail{ARN: "target-service", Name: "target-name"}
	m.ecs.investigation = awsservice.ECSInvestigation{ObservedAt: time.Now()}
	m.ecs.detailScroll = 4
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("m")})
	m = updated.(Model)
	if cmd != nil || m.screen != screenECSSummary || m.ecs.summaryIdentifiers || m.ecs.summaryPath == "" {
		t.Fatal("preview did not open safely")
	}
	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	m = updated.(Model)
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	if copied == "" || strings.Contains(copied, "target-service") || !strings.Contains(m.ecs.summaryNotice, "failed") {
		t.Fatal("copy leaked identifiers or hid failure")
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	m = updated.(Model)
	if !m.ecs.summaryIdentifiers || !strings.Contains(strings.Join(m.ecs.summaryLines(m), " "), "target-service") {
		t.Fatal("identifier disclosure not previewed")
	}
	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	m = updated.(Model)
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	if !strings.Contains(saved, "target-service") || !strings.Contains(m.ecs.summaryNotice, "failed") {
		t.Fatal("save did not match preview or failure hidden")
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.screen != screenECSInvestigation || m.ecs.detailScroll != 4 {
		t.Fatal("return lost investigation scroll")
	}
	resetECSContextState(&m)
	if m.ecs.summaryIdentifiers || m.ecs.summaryPath != "" {
		t.Fatal("identity switch retained disclosure state")
	}
}
