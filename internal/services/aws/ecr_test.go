package aws

import "testing"

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

func TestParseECRRuntimeRejectsUnknown(t *testing.T) {
	if _, err := ParseECRRuntime("nerdctl"); err == nil {
		t.Fatal("expected unsupported runtime error")
	}
}
