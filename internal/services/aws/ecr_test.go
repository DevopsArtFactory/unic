package aws

import (
	"strings"
	"testing"
)

func TestBuildPrivateECRRegistryURI(t *testing.T) {
	got, err := BuildPrivateECRRegistryURI("123456789012", "ap-northeast-2")
	if err != nil {
		t.Fatal(err)
	}
	want := "123456789012.dkr.ecr.ap-northeast-2.amazonaws.com"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestBuildPrivateECRRegistryURIRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name      string
		accountID string
		region    string
	}{
		{name: "invalid account", accountID: "1234abc", region: "ap-northeast-2"},
		{name: "invalid region", accountID: "123456789012", region: "ap-northeast-2; rm -rf /"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := BuildPrivateECRRegistryURI(tc.accountID, tc.region); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestBuildECRLoginCommandDocker(t *testing.T) {
	got, err := BuildECRLoginCommand("123456789012.dkr.ecr.ap-northeast-2.amazonaws.com", "ap-northeast-2", ECRRuntimeDocker)
	if err != nil {
		t.Fatal(err)
	}
	want := "aws ecr get-login-password --region ap-northeast-2 | docker login --username AWS --password-stdin 123456789012.dkr.ecr.ap-northeast-2.amazonaws.com"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestBuildECRLoginCommandPodman(t *testing.T) {
	got, err := BuildECRLoginCommand("123456789012.dkr.ecr.us-east-1.amazonaws.com", "us-east-1", ECRRuntimePodman)
	if err != nil {
		t.Fatal(err)
	}
	want := "aws ecr get-login-password --region us-east-1 | podman login --username AWS --password-stdin 123456789012.dkr.ecr.us-east-1.amazonaws.com"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestBuildECRLoginCommandRejectsInjectionAttempts(t *testing.T) {
	tests := []struct {
		name        string
		registryURI string
		region      string
	}{
		{
			name:        "region injection",
			registryURI: "123456789012.dkr.ecr.ap-northeast-2.amazonaws.com",
			region:      "ap-northeast-2; rm -rf /",
		},
		{
			name:        "registry injection",
			registryURI: "123456789012.dkr.ecr.ap-northeast-2.amazonaws.com; curl bad",
			region:      "ap-northeast-2",
		},
		{
			name:        "registry region mismatch",
			registryURI: "123456789012.dkr.ecr.us-east-1.amazonaws.com",
			region:      "ap-northeast-2",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := BuildECRLoginCommand(tc.registryURI, tc.region, ECRRuntimeDocker)
			if err == nil {
				t.Fatal("expected validation error")
			}
			if strings.Contains(err.Error(), "; rm -rf /") || strings.Contains(err.Error(), "curl bad") {
				t.Fatalf("error should not echo malicious shell payload verbatim: %v", err)
			}
		})
	}
}

func TestParseECRRuntimeRejectsUnknown(t *testing.T) {
	if _, err := ParseECRRuntime("nerdctl"); err == nil {
		t.Fatal("expected unsupported runtime error")
	}
}
