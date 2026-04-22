package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"unic/internal/config"
	awsservice "unic/internal/services/aws"
)

func TestRootIncludesECRLoginCommand(t *testing.T) {
	cmd := NewRootCmd()
	if _, _, err := cmd.Find([]string{"ecr", "login"}); err != nil {
		t.Fatalf("expected ecr login command: %v", err)
	}
}

func TestECRLoginPrintsCommand(t *testing.T) {
	origDefaultPath := ecrDefaultPathFn
	origEnsure := ecrEnsureConfigExistsFn
	origLoad := ecrLoadConfigFn
	origResolve := ecrResolveRegistryURIFn
	origBuild := ecrBuildLoginCommandFn
	defer func() {
		ecrDefaultPathFn = origDefaultPath
		ecrEnsureConfigExistsFn = origEnsure
		ecrLoadConfigFn = origLoad
		ecrResolveRegistryURIFn = origResolve
		ecrBuildLoginCommandFn = origBuild
	}()

	ecrDefaultPathFn = func() (string, error) { return "/tmp/config.yaml", nil }
	ecrEnsureConfigExistsFn = func(string) error { return nil }
	ecrLoadConfigFn = func(*string, *string, string) (*config.Config, error) {
		return &config.Config{Region: "ap-northeast-2"}, nil
	}
	ecrResolveRegistryURIFn = func(context.Context, *config.Config) (string, string, error) {
		return "123456789012.dkr.ecr.ap-northeast-2.amazonaws.com", "123456789012", nil
	}
	ecrBuildLoginCommandFn = func(registryURI, region string, runtime awsservice.ECRRuntime) (string, error) {
		if runtime != awsservice.ECRRuntimePodman {
			t.Fatalf("expected podman runtime, got %s", runtime)
		}
		return "aws ecr get-login-password --region ap-northeast-2 | podman login --username AWS --password-stdin 123456789012.dkr.ecr.ap-northeast-2.amazonaws.com", nil
	}

	cmd := NewRootCmd()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"ecr", "login", "--runtime", "podman"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(stdout.String()); !strings.Contains(got, "podman login") {
		t.Fatalf("expected podman login command, got %q", got)
	}
	if strings.TrimSpace(stderr.String()) != "" {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestECRLoginCopyReportsSuccessOnStderr(t *testing.T) {
	origDefaultPath := ecrDefaultPathFn
	origEnsure := ecrEnsureConfigExistsFn
	origLoad := ecrLoadConfigFn
	origResolve := ecrResolveRegistryURIFn
	origBuild := ecrBuildLoginCommandFn
	origCopy := ecrCopyClipboardFn
	defer func() {
		ecrDefaultPathFn = origDefaultPath
		ecrEnsureConfigExistsFn = origEnsure
		ecrLoadConfigFn = origLoad
		ecrResolveRegistryURIFn = origResolve
		ecrBuildLoginCommandFn = origBuild
		ecrCopyClipboardFn = origCopy
	}()

	ecrDefaultPathFn = func() (string, error) { return "/tmp/config.yaml", nil }
	ecrEnsureConfigExistsFn = func(string) error { return nil }
	ecrLoadConfigFn = func(*string, *string, string) (*config.Config, error) {
		return &config.Config{Region: "us-east-1"}, nil
	}
	ecrResolveRegistryURIFn = func(context.Context, *config.Config) (string, string, error) {
		return "123456789012.dkr.ecr.us-east-1.amazonaws.com", "123456789012", nil
	}
	ecrBuildLoginCommandFn = func(string, string, awsservice.ECRRuntime) (string, error) {
		return "aws ecr get-login-password --region us-east-1 | docker login --username AWS --password-stdin 123456789012.dkr.ecr.us-east-1.amazonaws.com", nil
	}
	var copied string
	ecrCopyClipboardFn = func(text string) error {
		copied = text
		return nil
	}

	cmd := NewRootCmd()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"ecr", "login", "--copy"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(copied, "docker login") {
		t.Fatalf("expected copied login command, got %q", copied)
	}
	if !strings.Contains(stderr.String(), "copied to clipboard") {
		t.Fatalf("expected clipboard success message, got %q", stderr.String())
	}
}
