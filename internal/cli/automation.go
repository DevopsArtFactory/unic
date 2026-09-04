package cli

import (
	"bytes"
	"context"
)

// ExecuteAutomation runs an existing machine-readable CLI command and returns
// its stdout. It keeps automation adapters on the same command contracts as
// the unic CLI instead of duplicating their business logic.
func ExecuteAutomation(ctx context.Context, args ...string) ([]byte, error) {
	cmd := NewRootCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetArgs(args)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	err := cmd.ExecuteContext(ctx)
	return stdout.Bytes(), err
}
