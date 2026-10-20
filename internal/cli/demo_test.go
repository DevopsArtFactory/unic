package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestDemoCommandHelpAndArguments(t *testing.T) {
	root := NewRootCmd()
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetErr(&output)
	root.SetArgs([]string{"demo", "--help"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "no AWS setup") {
		t.Fatal("demo not discoverable")
	}
	root = NewRootCmd()
	cmd, _, err := root.Find([]string{"demo"})
	if err != nil || cmd.Name() != "demo" || cmd.RunE == nil {
		t.Fatal("demo command not registered")
	}
	if cmd.Annotations[annotationReadOnly] != "true" || cmd.Annotations[annotationDestructive] != "false" {
		t.Fatal("demo contract must be read-only")
	}
	if err := cmd.ValidateArgs([]string{"unexpected"}); err == nil {
		t.Fatal("demo accepted unexpected arguments")
	}
}
