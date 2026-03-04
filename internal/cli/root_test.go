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
}
