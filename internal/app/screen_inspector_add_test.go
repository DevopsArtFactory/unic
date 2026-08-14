package app

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"unic/internal/config"
	"unic/internal/inspector"
)

func inspectorAddTestModel(t *testing.T) Model {
	t.Helper()
	m := New(testConfig(), "", "dev")
	m.cfg = &config.Config{Region: "us-east-1", ContextName: "dev"}
	m.inspector.checklistPath = filepath.Join(t.TempDir(), "readiness.yaml")
	m.screen = screenInspectorChecklistResults
	return m
}

func typeIntoChecklistAdd(m *Model, text string) {
	for _, r := range text {
		m.inspector.updateChecklistAdd(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
}

func selectChecklistType(t *testing.T, m *Model, want inspector.ChecklistCheckType) {
	t.Helper()
	for i, checkType := range checklistPromptTypes {
		if checkType == want {
			m.inspector.addTypeIdx = i
			m.inspector.updateChecklistAdd(m, tea.KeyMsg{Type: tea.KeyEnter})
			return
		}
	}
	t.Fatalf("check type %s not offered", want)
}

func TestChecklistAddWizardPersistsAndReruns(t *testing.T) {
	m := inspectorAddTestModel(t)

	m.inspector.openChecklistAdd(&m)
	if m.screen != screenInspectorChecklistAdd {
		t.Fatalf("expected add screen, got %v", m.screen)
	}

	selectChecklistType(t, &m, inspector.ChecklistCheckCloudWatchLogGroup)
	typeIntoChecklistAdd(&m, "/aws/ecs/app")
	m.inspector.updateChecklistAdd(&m, tea.KeyMsg{Type: tea.KeyEnter}) // resource
	typeIntoChecklistAdd(&m, "30")
	newM, cmd := m.inspector.updateChecklistAdd(&m, tea.KeyMsg{Type: tea.KeyEnter}) // retention -> save
	model := newM.(Model)
	if cmd == nil || model.screen != screenInspectorScanning {
		t.Fatalf("expected checklist rerun after save, got screen=%v", model.screen)
	}

	loaded, err := inspector.LoadChecklist(m.inspector.checklistPath)
	if err != nil {
		t.Fatalf("expected generated checklist to load through validation: %v", err)
	}
	if len(loaded.Checks) != 1 || loaded.Checks[0].Type != inspector.ChecklistCheckCloudWatchLogGroup ||
		loaded.Checks[0].Resource != "/aws/ecs/app" || loaded.Checks[0].Expect.RetentionDays == nil {
		t.Fatalf("expected persisted log-group check, got %+v", loaded.Checks)
	}
}

func TestChecklistAddSecurityGroupRuleMapping(t *testing.T) {
	values := map[string]string{
		"resource":  "sg-web",
		"rule_mode": "ingress_absent",
		"protocol":  "tcp",
		"from_port": "22",
		"to_port":   "22",
		"cidr":      "0.0.0.0/0",
	}
	check, err := buildChecklistCheck(inspector.ChecklistCheckSecurityGroup, values)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(check.Expect.IngressAbsent) != 1 {
		t.Fatalf("expected one forbidden ingress rule, got %+v", check.Expect)
	}
	rule := check.Expect.IngressAbsent[0]
	if rule.Protocol != "tcp" || rule.CIDR != "0.0.0.0/0" || rule.FromPort == nil || *rule.FromPort != 22 {
		t.Fatalf("expected mapped rule fields, got %+v", rule)
	}

	values["rule_mode"] = "sideways"
	if _, err := buildChecklistCheck(inspector.ChecklistCheckSecurityGroup, values); err == nil {
		t.Fatal("expected invalid rule_mode rejection")
	}
}

func TestChecklistAddValidationFailureKeepsWizardOpen(t *testing.T) {
	m := inspectorAddTestModel(t)
	m.inspector.openChecklistAdd(&m)
	selectChecklistType(t, &m, inspector.ChecklistCheckRDS)

	typeIntoChecklistAdd(&m, "prod-db")
	m.inspector.updateChecklistAdd(&m, tea.KeyMsg{Type: tea.KeyEnter}) // resource
	// Skip every expectation: AppendCheck must reject a check with none.
	for range checklistPromptFields[inspector.ChecklistCheckRDS] {
		m.inspector.updateChecklistAdd(&m, tea.KeyMsg{Type: tea.KeyEnter})
	}
	if m.screen != screenInspectorChecklistAdd || m.inspector.addError == "" {
		t.Fatalf("expected validation error to keep the wizard open, screen=%v err=%q", m.screen, m.inspector.addError)
	}
	if _, err := inspector.LoadChecklist(m.inspector.checklistPath); err == nil {
		t.Fatal("expected nothing to be written on validation failure")
	}

	view, ok := m.inspector.View(m)
	if !ok || !strings.Contains(view, m.inspector.addError) {
		t.Fatalf("expected the error to render in the wizard, got:\n%s", view)
	}
}

func TestChecklistAddBadNumberReturnsFieldError(t *testing.T) {
	values := map[string]string{"resource": "/aws/ecs/app", "retention_days": "thirty"}
	if _, err := buildChecklistCheck(inspector.ChecklistCheckCloudWatchLogGroup, values); err == nil ||
		!strings.Contains(err.Error(), "retention_days") {
		t.Fatalf("expected numeric validation error, got %v", err)
	}
}

func TestChecklistAddDefaultsTargetPathWithoutLoadedChecklist(t *testing.T) {
	m := inspectorAddTestModel(t)
	m.inspector.checklistPath = ""

	target := m.inspector.checklistAddTargetPath()
	if !strings.HasSuffix(target, "unic-checklist.yaml") {
		t.Fatalf("expected default checklist target, got %q", target)
	}
}

func TestChecklistAddReachableFromPickerWithoutLoadedChecklist(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.cfg = &config.Config{Region: "us-east-1", ContextName: "dev"}
	m.inspector.checklistPath = ""
	m.inspector.checklistDir = t.TempDir()
	m.screen = screenInspectorChecklistPicker

	// `a` on the picker opens the wizard even with nothing loaded.
	newM, _, handled := m.inspector.HandleKey(&m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	model := newM.(Model)
	if !handled || model.screen != screenInspectorChecklistAdd {
		t.Fatalf("expected wizard from the picker, got %v", model.screen)
	}

	// Complete the wizard end to end: a baseline check needs only a resource.
	m = model
	selectChecklistType(t, &m, inspector.ChecklistCheckCloudTrailBaseline)
	typeIntoChecklistAdd(&m, "cloudtrail")
	newM2, cmd := m.inspector.updateChecklistAdd(&m, tea.KeyMsg{Type: tea.KeyEnter})
	model = newM2.(Model)
	if cmd == nil || model.screen != screenInspectorScanning {
		t.Fatalf("expected rerun after first-check save, got %v", model.screen)
	}

	target := filepath.Join(m.inspector.checklistDir, "unic-checklist.yaml")
	loaded, err := inspector.LoadChecklist(target)
	if err != nil || len(loaded.Checks) != 1 {
		t.Fatalf("expected the default checklist to be created at %s, got %+v err=%v", target, loaded, err)
	}

	// esc from the wizard's type step returns to where it was opened.
	m = model
	m.inspector.openChecklistAdd(&m)
	m.screen = screenInspectorChecklistAdd
	m.inspector.addPrevScreen = screenInspectorChecklistPicker
	m.inspector.updateChecklistAdd(&m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.screen != screenInspectorChecklistPicker {
		t.Fatalf("expected esc to return to the picker, got %v", m.screen)
	}
}

func TestChecklistAddSecurityGroupExtendedFields(t *testing.T) {
	values := map[string]string{
		"resource":         "sg-web",
		"rule_mode":        "ingress_present",
		"cidr_v6":          "::/0",
		"referenced_sg_id": "sg-db",
	}
	check, err := buildChecklistCheck(inspector.ChecklistCheckSecurityGroup, values)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rule := check.Expect.IngressPresent[0]
	if rule.CIDRv6 != "::/0" || rule.ReferencedSGID != "sg-db" {
		t.Fatalf("expected IPv6 and referenced-SG fields mapped, got %+v", rule)
	}
}

func TestChecklistAddSchemaCoverageExtras(t *testing.T) {
	check, err := buildChecklistCheck(inspector.ChecklistCheckRDS, map[string]string{
		"resource": "prod-db", "engine_version": "16.3",
	})
	if err != nil || check.Expect.EngineVersion == nil || *check.Expect.EngineVersion != "16.3" {
		t.Fatalf("expected engine_version mapped, got %+v err=%v", check.Expect, err)
	}

	check, err = buildChecklistCheck(inspector.ChecklistCheckRoute53Record, map[string]string{
		"resource": "api.example.com", "zone": "example.com", "alias_hosted_zone_id": "Z123",
	})
	if err != nil || check.Expect.AliasHostedZoneID == nil || *check.Expect.AliasHostedZoneID != "Z123" {
		t.Fatalf("expected alias_hosted_zone_id mapped, got %+v err=%v", check.Expect, err)
	}
}
