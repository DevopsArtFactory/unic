package cli

import (
	"context"
	"encoding/json"
	"testing"
)

func TestExecuteAutomationUsesExistingJSONCommand(t *testing.T) {
	output, err := ExecuteAutomation(context.Background(), "schema", "context", "sync", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var contract commandContract
	if err := json.Unmarshal(output, &contract); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if contract.Path != "unic context sync" {
		t.Fatalf("path = %q", contract.Path)
	}
}
