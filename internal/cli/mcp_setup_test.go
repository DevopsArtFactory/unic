package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdateJSONConfigPreservesUnrelatedServers(t *testing.T) {
	before := []byte(`{"theme":"dark","mcpServers":{"other":{"command":"other"}}}`)
	after, err := updateJSONConfig(before, "/opt/bin/unic-mcp")
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(after, &got); err != nil {
		t.Fatal(err)
	}
	servers := got["mcpServers"].(map[string]any)
	if got["theme"] != "dark" || servers["other"] == nil || servers["unic"].(map[string]any)["command"] != "/opt/bin/unic-mcp" {
		t.Fatalf("config not preserved: %s", after)
	}
	if strings.Contains(string(after), "AWS_SECRET_ACCESS_KEY=") {
		t.Fatal("credential value persisted")
	}
}

func TestUpdateJSONConfigRejectsMalformed(t *testing.T) {
	if _, err := updateJSONConfig([]byte(`{broken`), "unic-mcp"); err == nil {
		t.Fatal("malformed JSON must fail")
	}
}

func TestUpdateCodexConfigIsIdempotentAndPreservesOtherTables(t *testing.T) {
	before := []byte("model = \"gpt\"\n\n[mcp_servers.other]\ncommand = \"other\"\n\n[mcp_servers.unic]\ncommand = \"old\"\n")
	one, err := updateCodexConfig(before, "/opt/bin/unic-mcp")
	if err != nil {
		t.Fatal(err)
	}
	two, err := updateCodexConfig(one, "/opt/bin/unic-mcp")
	if err != nil {
		t.Fatal(err)
	}
	if string(one) != string(two) || !strings.Contains(string(one), "[mcp_servers.other]") || strings.Count(string(one), "[mcp_servers.unic]") != 1 {
		t.Fatalf("not idempotent:\n%s", one)
	}
	if !strings.Contains(string(one), `"AWS_SECRET_ACCESS_KEY"`) || strings.Contains(string(one), "AWS_SECRET_ACCESS_KEY=") {
		t.Fatal("expected names only")
	}
}

func TestMCPSetupPlanUsesClientPaths(t *testing.T) {
	home := t.TempDir()
	path, before, after, err := mcpSetupPlan("kiro", home, "/bin/unic-mcp")
	if err != nil {
		t.Fatal(err)
	}
	if path != filepath.Join(home, ".kiro", "settings", "mcp.json") || len(before) != 0 || len(after) == 0 {
		t.Fatalf("unexpected plan: %s %q", path, after)
	}
}

func TestMCPSetupWritesBackup(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".claude.json")
	if err := os.WriteFile(path, []byte(`{"unrelated":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	originalHome, originalLook, originalCheck := userHomeDir, lookPath, checkMCPStartup
	defer func() { userHomeDir, lookPath, checkMCPStartup = originalHome, originalLook, originalCheck }()
	userHomeDir = func() (string, error) { return home, nil }
	lookPath = func(string) (string, error) { return "/bin/unic-mcp", nil }
	checkMCPStartup = func(string) error { return nil }
	cmd := NewRootCmd()
	cmd.SetArgs([]string{"mcp", "setup", "claude"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	backups, err := filepath.Glob(path + ".bak-*")
	if err != nil || len(backups) != 1 {
		t.Fatalf("backups=%v err=%v", backups, err)
	}
}

func TestMCPSetupDryRunDoesNotWrite(t *testing.T) {
	home := t.TempDir()
	originalHome, originalLook, originalCheck := userHomeDir, lookPath, checkMCPStartup
	defer func() { userHomeDir, lookPath, checkMCPStartup = originalHome, originalLook, originalCheck }()
	userHomeDir = func() (string, error) { return home, nil }
	lookPath = func(string) (string, error) { return "/bin/unic-mcp", nil }
	checkMCPStartup = func(string) error { return nil }
	cmd := NewRootCmd()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"mcp", "setup", "kiro", "--dry-run"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, ".kiro", "settings", "mcp.json")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("dry run wrote %s", path)
	}
	if !strings.Contains(output.String(), path) || !strings.Contains(output.String(), `"command": "/bin/unic-mcp"`) {
		t.Fatalf("incomplete preview: %s", output.String())
	}
}
