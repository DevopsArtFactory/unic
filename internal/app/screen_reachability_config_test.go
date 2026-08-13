package app

import (
	"testing"

	awsservice "unic/internal/services/aws"
)

func manualIPReachabilityModel() reachabilityModel {
	rm := newReachabilityModel()
	rm.destination = &awsservice.ReachabilityTarget{ManualIP: true}
	return rm
}

func TestReachabilityConfigMaxField(t *testing.T) {
	rm := newReachabilityModel()
	if rm.configMaxField() != 1 {
		t.Fatalf("expected max field 1 without manual IP, got %d", rm.configMaxField())
	}
	rm = manualIPReachabilityModel()
	if rm.configMaxField() != 2 {
		t.Fatalf("expected max field 2 with manual IP destination, got %d", rm.configMaxField())
	}
}

func TestReachabilityConfigProtocolAdjustClamps(t *testing.T) {
	rm := newReachabilityModel()
	rm.configField = 0
	rm.protocolIdx = 0

	rm.adjustConfigProtocol(-1)
	if rm.protocolIdx != 0 {
		t.Fatalf("expected protocol index to clamp at 0, got %d", rm.protocolIdx)
	}
	rm.adjustConfigProtocol(1)
	if rm.protocolIdx != 1 {
		t.Fatalf("expected protocol index 1, got %d", rm.protocolIdx)
	}

	rm.configField = 1
	rm.adjustConfigProtocol(1)
	if rm.protocolIdx != 1 {
		t.Fatalf("expected protocol unchanged off the protocol field, got %d", rm.protocolIdx)
	}
}

func TestReachabilityConfigCharacterInputValidation(t *testing.T) {
	rm := manualIPReachabilityModel()

	rm.configField = 1
	rm.portInput = ""
	rm.appendConfigChar("4")
	rm.appendConfigChar("x")
	rm.appendConfigChar("3")
	if rm.portInput != "43" {
		t.Fatalf("expected digits-only port input, got %q", rm.portInput)
	}
	rm.deleteConfigChar()
	if rm.portInput != "4" {
		t.Fatalf("expected backspace on port input, got %q", rm.portInput)
	}

	rm.configField = 2
	rm.destinationIP = ""
	rm.appendConfigChar("1")
	rm.appendConfigChar(".")
	rm.appendConfigChar("a")
	if rm.destinationIP != "1." {
		t.Fatalf("expected IP charset validation, got %q", rm.destinationIP)
	}
}

func TestReachabilityConfigSubmitAdvancesFromProtocolField(t *testing.T) {
	m := Model{screen: screenReachabilityConfig}
	m.reachability = newReachabilityModel()
	m.reachability.configField = 0

	_, cmd := m.reachability.submitConfig(&m)
	if m.reachability.configField != 1 {
		t.Fatalf("expected enter on protocol field to advance, got field %d", m.reachability.configField)
	}
	if cmd != nil {
		t.Fatal("expected no analysis to start when advancing fields")
	}
}
