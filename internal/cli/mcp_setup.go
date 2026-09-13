package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/spf13/cobra"

	manifest "unic"
)

var (
	userHomeDir     = os.UserHomeDir
	lookPath        = exec.LookPath
	checkMCPStartup = func(path string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _, err := runMCPCheck(ctx, path)
		return err
	}
)

func newMCPCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "mcp", Short: "Configure unic MCP clients"}
	cmd.AddCommand(newMCPSetupCmd())
	return cmd
}

func newMCPSetupCmd() *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use: "setup <codex|claude|kiro>", Short: "Safely register unic-mcp for an AI client", Args: cobra.ExactArgs(1),
		ValidArgs: []string{"codex", "claude", "kiro"},
		RunE: func(cmd *cobra.Command, args []string) error {
			client := strings.ToLower(args[0])
			if client != "codex" && client != "claude" && client != "kiro" {
				return fmt.Errorf("unsupported MCP client %q", args[0])
			}
			binary, err := lookPath("unic-mcp")
			if err != nil {
				return errors.New("unic-mcp not found in PATH; install unic with Homebrew or the release installer, then run unic doctor")
			}
			if err := checkMCPStartup(binary); err != nil {
				return fmt.Errorf("unic-mcp startup check failed: %w; run unic doctor for remediation", err)
			}
			home, err := userHomeDir()
			if err != nil {
				return err
			}
			path, before, after, err := mcpSetupPlan(client, home, binary)
			if err != nil {
				return err
			}
			if bytes.Equal(before, after) {
				_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s is already configured in %s\n", client, path)
				return err
			}
			if dryRun {
				_, err = fmt.Fprintf(cmd.OutOrStdout(), "Would update %s:\n%s", path, after)
				return err
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				return err
			}
			if len(before) > 0 {
				backup := path + ".bak-" + time.Now().Format("20060102-150405.000000000")
				if err := os.WriteFile(backup, before, 0o600); err != nil {
					return fmt.Errorf("back up config: %w", err)
				}
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Backup: %s\n", backup); err != nil {
					return err
				}
			}
			if err := os.WriteFile(path, after, 0o600); err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Configured %s in %s using %s\nRun `unic doctor` to verify the MCP handshake.\n", client, path, binary)
			return err
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview the exact resulting configuration")
	return cmd
}

func mcpSetupPlan(client, home, binary string) (string, []byte, []byte, error) {
	switch client {
	case "codex":
		path := filepath.Join(home, ".codex", "config.toml")
		before, err := readOptional(path)
		if err != nil {
			return "", nil, nil, err
		}
		after, err := updateCodexConfig(before, binary)
		return path, before, after, err
	case "claude", "kiro":
		path := filepath.Join(home, ".claude.json")
		if client == "kiro" {
			path = filepath.Join(home, ".kiro", "settings", "mcp.json")
		}
		before, err := readOptional(path)
		if err != nil {
			return "", nil, nil, err
		}
		after, err := updateJSONConfig(before, binary)
		return path, before, after, err
	default:
		return "", nil, nil, fmt.Errorf("unsupported MCP client %q", client)
	}
}

func readOptional(path string) ([]byte, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	return b, err
}

func updateJSONConfig(before []byte, binary string) ([]byte, error) {
	root := map[string]any{}
	if len(bytes.TrimSpace(before)) > 0 {
		if err := json.Unmarshal(before, &root); err != nil {
			return nil, fmt.Errorf("malformed MCP client config: %w", err)
		}
	}
	servers, ok := root["mcpServers"].(map[string]any)
	if !ok {
		if root["mcpServers"] != nil {
			return nil, errors.New("malformed MCP client config: mcpServers must be an object")
		}
		servers = map[string]any{}
		root["mcpServers"] = servers
	}
	var source struct {
		MCPServers map[string]map[string]any `json:"mcpServers"`
	}
	if err := json.Unmarshal(manifest.AgentMCPManifest, &source); err != nil {
		return nil, err
	}
	entry := source.MCPServers["unic"]
	entry["command"] = binary
	if reflect.DeepEqual(servers["unic"], entry) {
		return before, nil
	}
	servers["unic"] = entry
	out, err := json.MarshalIndent(root, "", "  ")
	return append(out, '\n'), err
}

func updateCodexConfig(before []byte, binary string) ([]byte, error) {
	var source struct {
		MCPServers map[string]struct {
			EnvVars []string `json:"env_vars"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(manifest.CodexMCPManifest, &source); err != nil {
		return nil, err
	}
	env, err := json.Marshal(source.MCPServers["unic"].EnvVars)
	if err != nil {
		return nil, err
	}
	section := fmt.Sprintf("[mcp_servers.unic]\ncommand = %q\nargs = []\nenv_vars = %s\n", binary, env)
	lines := strings.Split(strings.TrimRight(string(before), "\n"), "\n")
	start, end := -1, len(lines)
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && !strings.HasSuffix(trimmed, "]") {
			return nil, fmt.Errorf("malformed Codex config: invalid table header on line %d", i+1)
		}
		if trimmed == "[mcp_servers.unic]" {
			if start >= 0 {
				return nil, errors.New("malformed Codex config: duplicate [mcp_servers.unic] table")
			}
			start = i
			continue
		}
		if start >= 0 && i > start && strings.HasPrefix(trimmed, "[") {
			end = i
			break
		}
	}
	if start >= 0 {
		lines = append(lines[:start], lines[end:]...)
	}
	base := strings.TrimSpace(strings.Join(lines, "\n"))
	if base != "" {
		base += "\n\n"
	}
	return []byte(base + section), nil
}
