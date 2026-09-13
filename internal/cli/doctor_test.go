package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"unic/internal/config"
)

func stubDoctor(t *testing.T) {
	t.Helper()
	origLookPath, origExecutable, origStat := doctorLookPath, doctorExecutable, doctorStat
	origDefaultPath, origLoad := doctorDefaultPath, doctorLoadConfig
	origCredentials, origMCP := doctorCredentials, doctorRunMCPCheck
	origVersion := Version
	t.Cleanup(func() {
		doctorLookPath, doctorExecutable, doctorStat = origLookPath, origExecutable, origStat
		doctorDefaultPath, doctorLoadConfig = origDefaultPath, origLoad
		doctorCredentials, doctorRunMCPCheck = origCredentials, origMCP
		Version = origVersion
	})
	doctorLookPath = func(string) (string, error) { return "/bin/unic-mcp", nil }
	doctorRunMCPCheck = func(context.Context, string) (string, int, error) { return "1.2.3", 4, nil }
	doctorDefaultPath = func() (string, error) { return "/config.yaml", nil }
	doctorStat = func(string) (os.FileInfo, error) { return fakeFileInfo{}, nil }
	doctorLoadConfig = func(*string, *string, string) (*config.Config, error) {
		return &config.Config{ContextName: "dev", Profile: "dev-profile", Region: "us-east-1"}, nil
	}
	doctorCredentials = func(context.Context, *config.Config) error { return nil }
	Version = "1.2.3"
}

func TestDoctorJSONHealthy(t *testing.T) {
	stubDoctor(t)
	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"doctor", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"schema_version":"v1"`, `"status":"pass"`, `"context":"dev"`, `"credential chain resolved"`} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output missing %q: %s", want, out.String())
		}
	}
}

func TestDoctorReportsMissingMCPAndCredentialsWithoutSecrets(t *testing.T) {
	stubDoctor(t)
	doctorLookPath = func(string) (string, error) { return "", errors.New("missing") }
	doctorExecutable = func() (string, error) { return "/bin/unic", nil }
	doctorStat = func(path string) (os.FileInfo, error) {
		if filepath.Base(path) == "unic-mcp" {
			return nil, os.ErrNotExist
		}
		return fakeFileInfo{}, nil
	}
	doctorCredentials = func(context.Context, *config.Config) error { return errors.New("AKIA-DO-NOT-PRINT secret-token") }
	report := buildDoctorReport(context.Background())
	data := bytes.Buffer{}
	if err := jsonEncode(&data, report); err != nil {
		t.Fatal(err)
	}
	if report.Status != "fail" || !strings.Contains(data.String(), "binary not found") || !strings.Contains(data.String(), "credentials unavailable") {
		t.Fatalf("unexpected report: %s", data.String())
	}
	if strings.Contains(data.String(), "AKIA") || strings.Contains(data.String(), "secret-token") {
		t.Fatalf("credential material leaked: %s", data.String())
	}
}

func TestDoctorWarnsWhenConfigMissing(t *testing.T) {
	stubDoctor(t)
	doctorStat = func(path string) (os.FileInfo, error) {
		if path == "/config.yaml" {
			return nil, os.ErrNotExist
		}
		return fakeFileInfo{}, nil
	}
	report := buildDoctorReport(context.Background())
	if report.Status != "warn" || report.Checks[len(report.Checks)-1].Name != "config" {
		t.Fatalf("unexpected report: %#v", report)
	}
}

func TestDoctorReportsVersionSkew(t *testing.T) {
	stubDoctor(t)
	doctorRunMCPCheck = func(context.Context, string) (string, int, error) { return "1.2.4", 4, nil }
	report := buildDoctorReport(context.Background())
	if report.Status != "fail" || !strings.Contains(report.Checks[0].Message, "version mismatch") {
		t.Fatalf("unexpected report: %#v", report)
	}
}

type fakeFileInfo struct{}

func (fakeFileInfo) Name() string       { return "file" }
func (fakeFileInfo) Size() int64        { return 0 }
func (fakeFileInfo) Mode() os.FileMode  { return 0600 }
func (fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (fakeFileInfo) IsDir() bool        { return false }
func (fakeFileInfo) Sys() any           { return nil }

func jsonEncode(out *bytes.Buffer, value any) error {
	return json.NewEncoder(out).Encode(value)
}
