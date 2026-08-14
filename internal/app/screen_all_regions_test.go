package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"unic/internal/config"
	awsservice "unic/internal/services/aws"
)

func multiRegionCfg() *config.Config {
	return &config.Config{Region: "us-east-1", Regions: []string{"us-east-1", "eu-west-1"}, ContextName: "dev"}
}

func TestRDSAllRegionsToggleAndRendering(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.cfg = multiRegionCfg()
	m.screen = screenRDSList

	next, cmd := m.rds.updateList(&m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}})
	model := next.(Model)
	if !model.rds.allRegions || cmd == nil || model.screen != screenLoading {
		t.Fatalf("expected all-regions reload, allRegions=%v screen=%v", model.rds.allRegions, model.screen)
	}

	m = model
	m.rds.HandleMessage(&m, rdsInstancesLoadedMsg{
		instances: []awsservice.RDSInstance{
			{DBInstanceID: "prod-db", Region: "eu-west-1"},
		},
		regionErrors: []awsservice.RegionError{{Region: "us-east-1", Err: errRegionTest}},
	})
	view, _ := m.rds.View(m)
	for _, want := range []string{"(all regions)", "[eu-west-1]", "us-east-1: access denied"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected %q in RDS all-regions view, got:\n%s", want, view)
		}
	}
}

func TestRDSAllRegionsToggleIgnoredForSingleRegion(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.cfg = &config.Config{Region: "us-east-1", Regions: []string{"us-east-1"}, ContextName: "dev"}
	m.screen = screenRDSList

	next, cmd := m.rds.updateList(&m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}})
	model := next.(Model)
	if model.rds.allRegions || cmd != nil {
		t.Fatal("expected no-op for single-region contexts")
	}
}

func TestLambdaAllRegionsToggleAndRendering(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.cfg = multiRegionCfg()
	m.screen = screenLambdaFunctionList

	next, cmd := m.lambda.updateFunctionList(&m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}})
	model := next.(Model)
	if !model.lambda.allRegions || cmd == nil {
		t.Fatal("expected all-regions reload for lambda")
	}

	m = model
	m.screen = screenLambdaFunctionList
	m.lambda.HandleMessage(&m, lambdaFunctionsLoadedMsg{
		functions: []awsservice.LambdaFunction{
			{Name: "checkout", Region: "eu-west-1"},
		},
		regionErrors: []awsservice.RegionError{{Region: "us-east-1", Err: errRegionTest}},
	})
	m.screen = screenLambdaFunctionList
	view, _ := m.lambda.View(m)
	for _, want := range []string{"(all regions)", "[eu-west-1]", "us-east-1: access denied"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected %q in Lambda all-regions view, got:\n%s", want, view)
		}
	}
}

var errRegionTest = errTest{}

type errTest struct{}

func (errTest) Error() string { return "access denied" }
