package cli

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"unic/internal/config"
)

func TestRootIncludesContextAndEnvCommands(t *testing.T) {
	cmd := NewRootCmd()
	if _, _, err := cmd.Find([]string{"context", "setup"}); err != nil {
		t.Fatalf("expected context setup command: %v", err)
	}
	if _, _, err := cmd.Find([]string{"context", "unset"}); err != nil {
		t.Fatalf("expected context unset command: %v", err)
	}
	if _, _, err := cmd.Find([]string{"env"}); err != nil {
		t.Fatalf("expected env command: %v", err)
	}
}

func TestEnvCommandPrintsExports(t *testing.T) {
	origDefaultPath := defaultPathFn
	origEnsure := ensureConfigExistsFn
	origLoadNamed := loadNamedContextFn
	origBuildEnv := buildEnvFn
	defer func() {
		defaultPathFn = origDefaultPath
		ensureConfigExistsFn = origEnsure
		loadNamedContextFn = origLoadNamed
		buildEnvFn = origBuildEnv
	}()

	defaultPathFn = func() (string, error) { return "/tmp/config.yaml", nil }
	ensureConfigExistsFn = func(string) error { return nil }
	loadNamedContextFn = func(string, string) (*config.Config, error) {
		return &config.Config{ContextName: "dev", AuthType: config.AuthTypeCredential, Region: "ap-northeast-2", Profile: "dev"}, nil
	}
	buildEnvFn = func(context.Context, *config.Config) (string, error) {
		return "export AWS_PROFILE='dev'", nil
	}

	cmd := NewRootCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"env", "dev"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(stdout.String()) != "export AWS_PROFILE='dev'" {
		t.Fatalf("unexpected stdout: %q", stdout.String())
	}
}

func TestContextSetupCopiesExportsToClipboardWithoutStdout(t *testing.T) {
	origDefaultPath := defaultPathFn
	origEnsure := ensureConfigExistsFn
	origSetup := setupContextFn
	origClipboard := copyClipboardFn
	defer func() {
		defaultPathFn = origDefaultPath
		ensureConfigExistsFn = origEnsure
		setupContextFn = origSetup
		copyClipboardFn = origClipboard
	}()

	defaultPathFn = func() (string, error) { return "/tmp/config.yaml", nil }
	ensureConfigExistsFn = func(string) error { return nil }
	setupContextFn = func(context.Context, string, io.Reader, io.Writer) (string, error) {
		return "export AWS_REGION='us-east-1'", nil
	}
	copyClipboardFn = func(text string) error {
		if text != "export AWS_REGION='us-east-1'" {
			t.Fatalf("unexpected clipboard text: %q", text)
		}
		return nil
	}

	cmd := NewRootCmd()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"context", "setup"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(stdout.String()) != "" {
		t.Fatalf("expected no stdout output, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "Exports copied to clipboard.") {
		t.Fatalf("expected clipboard success message in stderr, got %q", stderr.String())
	}
}

func TestContextUnsetClearsCurrentAndCopiesCleanupCommands(t *testing.T) {
	origDefaultPath := defaultPathFn
	origEnsure := ensureConfigExistsFn
	origUnset := unsetCurrentFn
	origClipboard := copyClipboardFn
	origCleanup := buildCleanupEnvFn
	defer func() {
		defaultPathFn = origDefaultPath
		ensureConfigExistsFn = origEnsure
		unsetCurrentFn = origUnset
		copyClipboardFn = origClipboard
		buildCleanupEnvFn = origCleanup
	}()

	defaultPathFn = func() (string, error) { return "/tmp/config.yaml", nil }
	ensureConfigExistsFn = func(string) error { return nil }
	unsetCurrentFn = func(path string) error {
		if path != "/tmp/config.yaml" {
			t.Fatalf("unexpected config path: %q", path)
		}
		return nil
	}
	buildCleanupEnvFn = func() string {
		return strings.Join([]string{
			"unset AWS_PROFILE",
			"unset AWS_SESSION_TOKEN",
			"unset UNIC_CONTEXT",
		}, "\n")
	}
	copyClipboardFn = func(text string) error {
		if !strings.Contains(text, "unset AWS_PROFILE") {
			t.Fatalf("expected cleanup commands, got %q", text)
		}
		if !strings.Contains(text, "unset AWS_SESSION_TOKEN") {
			t.Fatalf("expected session token cleanup, got %q", text)
		}
		if !strings.Contains(text, "unset UNIC_CONTEXT") {
			t.Fatalf("expected UNIC_CONTEXT cleanup, got %q", text)
		}
		return nil
	}

	cmd := NewRootCmd()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"context", "unset"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(stdout.String()) != "" {
		t.Fatalf("expected no stdout output, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "Cleanup commands copied to clipboard.") {
		t.Fatalf("expected clipboard message, got %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "Current context cleared.") {
		t.Fatalf("expected clear message, got %q", stderr.String())
	}
}
