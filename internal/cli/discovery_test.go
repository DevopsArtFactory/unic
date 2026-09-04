package cli

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
)

func TestCapabilitiesJSONIsDeterministicAndUsesCatalog(t *testing.T) {
	first := executeDiscovery(t, "capabilities", "--json")
	second := executeDiscovery(t, "capabilities", "--json")
	if first != second {
		t.Fatal("capabilities output should be deterministic")
	}

	var document capabilityDocument
	if err := json.Unmarshal([]byte(first), &document); err != nil {
		t.Fatalf("decode capabilities: %v", err)
	}
	if document.SchemaVersion != discoverySchemaVersion {
		t.Fatalf("schema version = %q, want %q", document.SchemaVersion, discoverySchemaVersion)
	}
	if len(document.Services) == 0 || len(document.Commands) == 0 {
		t.Fatalf("expected services and commands, got %#v", document)
	}
	for i := 1; i < len(document.Services); i++ {
		if document.Services[i-1].Name > document.Services[i].Name {
			t.Fatalf("services are not sorted: %q before %q", document.Services[i-1].Name, document.Services[i].Name)
		}
	}
}

func TestSchemaDescribesRegisteredCommand(t *testing.T) {
	output := executeDiscovery(t, "schema", "context", "sync", "--json")
	var contract commandContract
	if err := json.Unmarshal([]byte(output), &contract); err != nil {
		t.Fatalf("decode schema: %v", err)
	}

	wantArgs := []commandArgument{{Name: "base-context", Required: false}}
	if contract.Path != "unic context sync" || !reflect.DeepEqual(contract.Arguments, wantArgs) {
		t.Fatalf("unexpected contract: %#v", contract)
	}
	if contract.ReadOnly || !contract.Destructive {
		t.Fatalf("context sync safety classification = read_only:%t destructive:%t", contract.ReadOnly, contract.Destructive)
	}
	if !hasFlag(contract.Flags, "dry-run") || !hasFlag(contract.Flags, "profile") {
		t.Fatalf("expected local and inherited flags, got %#v", contract.Flags)
	}
}

func TestSchemaRejectsUnknownOrNonExecutableCommand(t *testing.T) {
	for _, args := range [][]string{{"schema", "missing", "--json"}, {"schema", "context", "--json"}} {
		cmd := NewRootCmd()
		cmd.SetArgs(args)
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		if err := cmd.Execute(); err == nil {
			t.Fatalf("Execute(%v) succeeded, want error", args)
		}
	}
}

func executeDiscovery(t *testing.T, args ...string) string {
	t.Helper()
	cmd := NewRootCmd()
	var output bytes.Buffer
	cmd.SetArgs(args)
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute(%v): %v\n%s", args, err, output.String())
	}
	return output.String()
}

func hasFlag(flags []commandFlag, name string) bool {
	for _, flag := range flags {
		if flag.Name == name {
			return true
		}
	}
	return false
}
