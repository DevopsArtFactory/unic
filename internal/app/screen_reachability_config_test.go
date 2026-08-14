package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	awsservice "unic/internal/services/aws"
)

func reachabilityConfigModel(manualIP bool) Model {
	m := Model{screen: screenReachabilityConfig}
	m.reachability = newReachabilityModel()
	if manualIP {
		m.reachability.destination = &awsservice.ReachabilityTarget{ManualIP: true}
	}
	return m
}

func configKey(key string) tea.KeyMsg {
	switch key {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "backspace":
		return tea.KeyMsg{Type: tea.KeyBackspace}
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
}

func TestReachabilityConfigMaxField(t *testing.T) {
	m := reachabilityConfigModel(false)
	if m.reachability.configMaxField() != 1 {
		t.Fatalf("expected max field 1 without manual IP, got %d", m.reachability.configMaxField())
	}
	m = reachabilityConfigModel(true)
	if m.reachability.configMaxField() != 2 {
		t.Fatalf("expected max field 2 with manual IP destination, got %d", m.reachability.configMaxField())
	}
}

func TestReachabilityConfigProtocolAdjustClamps(t *testing.T) {
	m := reachabilityConfigModel(false)
	m.reachability.configField = 0
	m.reachability.protocolIdx = 0

	m.reachability.updateConfig(&m, configKey("left"))
	if m.reachability.protocolIdx != 0 {
		t.Fatalf("expected protocol index to clamp at 0, got %d", m.reachability.protocolIdx)
	}
	m.reachability.updateConfig(&m, configKey("right"))
	if m.reachability.protocolIdx != 1 {
		t.Fatalf("expected protocol index 1, got %d", m.reachability.protocolIdx)
	}

	m.reachability.configField = 1
	m.reachability.updateConfig(&m, configKey("right"))
	if m.reachability.protocolIdx != 1 {
		t.Fatalf("expected protocol unchanged off the protocol field, got %d", m.reachability.protocolIdx)
	}
}

func TestReachabilityConfigCharacterInputValidation(t *testing.T) {
	m := reachabilityConfigModel(true)

	m.reachability.configField = 1
	m.reachability.portInput = ""
	for _, key := range []string{"4", "x", "3"} {
		m.reachability.updateConfig(&m, configKey(key))
	}
	if m.reachability.portInput != "43" {
		t.Fatalf("expected digits-only port input, got %q", m.reachability.portInput)
	}
	m.reachability.updateConfig(&m, configKey("backspace"))
	if m.reachability.portInput != "4" {
		t.Fatalf("expected backspace on port input, got %q", m.reachability.portInput)
	}

	m.reachability.configField = 2
	m.reachability.destinationIP = ""
	for _, key := range []string{"1", ".", "a"} {
		m.reachability.updateConfig(&m, configKey(key))
	}
	if m.reachability.destinationIP != "1." {
		t.Fatalf("expected IP charset validation, got %q", m.reachability.destinationIP)
	}
}

func TestReachabilityConfigEnterAdvancesFromProtocolField(t *testing.T) {
	m := reachabilityConfigModel(false)
	m.reachability.configField = 0

	_, cmd := m.reachability.updateConfig(&m, configKey("enter"))
	if m.reachability.configField != 1 {
		t.Fatalf("expected enter on protocol field to advance, got field %d", m.reachability.configField)
	}
	if cmd != nil {
		t.Fatal("expected no analysis to start when advancing fields")
	}
}
