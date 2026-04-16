package cli

import (
	"testing"
)

func TestNewRootCmdNotNil(t *testing.T) {
	cmd := NewRootCmd()
	if cmd == nil {
		t.Fatal("root command should not be nil")
	}
	if cmd.Use != "unic" {
		t.Errorf("expected Use 'unic', got '%s'", cmd.Use)
	}
}

func TestRootCmdHasFlags(t *testing.T) {
	cmd := NewRootCmd()
	pf := cmd.PersistentFlags().Lookup("profile")
	if pf == nil {
		t.Error("expected --profile flag")
	}
	rf := cmd.PersistentFlags().Lookup("region")
	if rf == nil {
		t.Error("expected --region flag")
	}
	vf := cmd.PersistentFlags().Lookup("verbose")
	if vf == nil {
		t.Error("expected --verbose flag")
	}
	cf := cmd.PersistentFlags().Lookup("checklist")
	if cf == nil {
		t.Error("expected --checklist flag")
	}
	if vf != nil && vf.Shorthand != "v" {
		t.Errorf("expected --verbose shorthand 'v', got '%s'", vf.Shorthand)
	}
}
