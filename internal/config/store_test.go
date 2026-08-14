package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteConfigBytesIsAtomicAndLeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "config.yaml")

	if err := writeConfigBytes(path, []byte("current: a\n")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := writeConfigBytes(path, []byte("current: b\n")); err != nil {
		t.Fatalf("unexpected error overwriting: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil || string(data) != "current: b\n" {
		t.Fatalf("expected replaced content, got %q err=%v", data, err)
	}

	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".tmp") {
			t.Fatalf("expected no temp files to linger, found %q", entry.Name())
		}
	}

	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0644 {
		t.Fatalf("expected 0644 permissions, got %v err=%v", info.Mode(), err)
	}
}

func TestMutateFileConfigErrorDoesNotTouchFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	original := "current: keep\ncontexts:\n  - name: keep\n"
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	wantErr := errors.New("mutation rejected")
	if err := mutateFileConfig(path, failIfMissing, func(*fileConfig) error {
		return wantErr
	}); !errors.Is(err, wantErr) {
		t.Fatalf("expected mutation error to propagate, got %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil || string(data) != original {
		t.Fatalf("expected file untouched after mutation error, got %q err=%v", data, err)
	}
}

func TestMutateFileConfigMissingPolicies(t *testing.T) {
	dir := t.TempDir()

	missing := filepath.Join(dir, "missing.yaml")
	if err := mutateFileConfig(missing, failIfMissing, func(*fileConfig) error { return nil }); err == nil {
		t.Fatal("expected failIfMissing to reject an absent file")
	}

	seeded := filepath.Join(dir, "seeded.yaml")
	err := mutateFileConfig(seeded, defaultsIfMissing, func(fc *fileConfig) error {
		fc.Favorites.Services = []string{"ECS"}
		return nil
	})
	if err != nil {
		t.Fatalf("expected defaultsIfMissing to seed and write, got %v", err)
	}
	if _, err := os.Stat(seeded); err != nil {
		t.Fatalf("expected seeded file to exist: %v", err)
	}
}

func TestMutateConfigNodePreservesComments(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := "# keep this comment\ncurrent: a\ncontexts:\n  - name: a\n  - name: b\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	if err := SetCurrent(path, "b"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "# keep this comment") {
		t.Fatalf("expected comment to survive the node path, got:\n%s", data)
	}
	if !strings.Contains(string(data), "current: b") {
		t.Fatalf("expected current to be updated, got:\n%s", data)
	}
}

func TestWriteConfigBytesPreservesExistingPermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("current: a\n"), 0600); err != nil {
		t.Fatal(err)
	}

	if err := writeConfigBytes(path, []byte("current: b\n")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("expected 0600 to be preserved on overwrite, got %v err=%v", info.Mode(), err)
	}
}
