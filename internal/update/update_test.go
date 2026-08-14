package update

import (
	"errors"
	"strings"
	"testing"
)

func TestIsNewer(t *testing.T) {
	tests := []struct {
		name    string
		current string
		latest  string
		want    bool
	}{
		{"newer patch", "0.1.0", "0.1.1", true},
		{"newer minor", "0.1.0", "0.2.0", true},
		{"newer major", "0.1.0", "1.0.0", true},
		{"same version", "0.1.0", "0.1.0", false},
		{"older version", "0.2.0", "0.1.0", false},
		{"with v prefix", "v0.1.0", "v0.2.0", true},
		{"mixed prefix", "0.1.0", "v0.2.0", true},
		{"multi-digit minor", "0.9.0", "0.10.0", true},
		{"multi-digit patch", "0.1.9", "0.1.10", true},
		{"major double digit", "2.0.0", "10.0.0", true},
		{"non-numeric segment", "0.1.0", "0.1.0-beta", false},
		{"dev version", "dev", "", false},
		{"empty latest", "0.1.0", "", false},
		{"empty current", "", "0.1.0", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsNewer(tt.current, tt.latest)
			if got != tt.want {
				t.Errorf("IsNewer(%q, %q) = %v, want %v", tt.current, tt.latest, got, tt.want)
			}
		})
	}
}

func TestNormalizeVersion(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"v0.1.0", "0.1.0"},
		{"0.1.0", "0.1.0"},
		{"v1.2.3", "1.2.3"},
		{"", ""},
	}

	for _, tt := range tests {
		got := normalizeVersion(tt.input)
		if got != tt.want {
			t.Errorf("normalizeVersion(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestArchiveName(t *testing.T) {
	name := archiveName("v0.2.0")
	// Just check it's not empty and has expected format
	if name == "" {
		t.Error("archiveName returned empty string")
	}
	if len(name) < 10 {
		t.Errorf("archiveName returned unexpectedly short: %q", name)
	}
}

func TestDownloadURL(t *testing.T) {
	url := DownloadURL("v0.2.0")
	if url == "" {
		t.Error("DownloadURL returned empty string")
	}
	expected := "https://github.com/DevopsArtFactory/unic/releases/download/v0.2.0/"
	if len(url) < len(expected) {
		t.Errorf("DownloadURL too short: %q", url)
	}
}

func TestShouldCheck_NoCacheFile(t *testing.T) {
	// With no cache file, should always return true
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if !ShouldCheck() {
		t.Error("ShouldCheck should return true when no cache exists")
	}
}

func TestWriteAndReadCache(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	// Write cache
	err := writeCache("v0.2.0")
	if err != nil {
		t.Fatalf("writeCache failed: %v", err)
	}

	// Read cache
	c, err := readCache()
	if err != nil {
		t.Fatalf("readCache failed: %v", err)
	}
	if c.Version != "v0.2.0" {
		t.Errorf("expected version v0.2.0, got %s", c.Version)
	}

	// Should not need to check again (cache is fresh)
	if ShouldCheck() {
		t.Error("ShouldCheck should return false with fresh cache")
	}
}

func TestCheckForUpdate_DevVersion(t *testing.T) {
	// dev version should never trigger update check
	result, err := CheckForUpdate("dev")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty string for dev version, got %q", result)
	}
}

func TestCheckForUpdate_PropagatesCheckError(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	orig := checkLatestVersionFn
	t.Cleanup(func() { checkLatestVersionFn = orig })
	checkLatestVersionFn = func() (string, error) {
		return "", errors.New("api down")
	}

	result, err := CheckForUpdate("0.1.0")
	if err == nil || !strings.Contains(err.Error(), "api down") {
		t.Fatalf("expected wrapped check error, got %v", err)
	}
	if result != "" {
		t.Errorf("expected empty version on error, got %q", result)
	}
}

func TestCheckForUpdate_ReturnsNewerVersion(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	orig := checkLatestVersionFn
	t.Cleanup(func() { checkLatestVersionFn = orig })
	checkLatestVersionFn = func() (string, error) {
		return "v0.2.0", nil
	}

	result, err := CheckForUpdate("0.1.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "v0.2.0" {
		t.Errorf("expected v0.2.0, got %q", result)
	}
}
