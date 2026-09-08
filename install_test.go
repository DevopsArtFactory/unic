package unic_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestInstallerValidatesBothBinariesBeforeInstalling(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("installer supports macOS and Linux")
	}
	if runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64" {
		t.Skip("installer supports amd64 and arm64")
	}
	for _, tool := range []string{"sh", "uname", "grep", "sed", "mktemp", "rm", "cp", "tar", "install"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("installer requires %s: %v", tool, err)
		}
	}
	for _, tc := range []struct {
		name    string
		files   map[string]string
		wantErr bool
	}{
		{"missing unic", map[string]string{"unic-mcp": "new mcp"}, true},
		{"missing unic-mcp", map[string]string{"unic": "new tui"}, true},
		{"empty unic", map[string]string{"unic": "", "unic-mcp": "new mcp"}, true},
		{"empty unic-mcp", map[string]string{"unic": "new tui", "unic-mcp": ""}, true},
		{"both binaries", map[string]string{"unic": "new tui", "unic-mcp": "new mcp"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			installDir := filepath.Join(dir, "installed")
			fakeBin := filepath.Join(dir, "bin")
			for _, path := range []string{installDir, fakeBin} {
				if err := os.Mkdir(path, 0755); err != nil {
					t.Fatal(err)
				}
			}
			for _, name := range []string{"unic", "unic-mcp"} {
				if err := os.WriteFile(filepath.Join(installDir, name), []byte("existing "+name), 0700); err != nil {
					t.Fatal(err)
				}
			}

			var archive bytes.Buffer
			gz := gzip.NewWriter(&archive)
			tw := tar.NewWriter(gz)
			for name, content := range tc.files {
				if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0644, Size: int64(len(content))}); err != nil {
					t.Fatal(err)
				}
				if _, err := tw.Write([]byte(content)); err != nil {
					t.Fatal(err)
				}
			}
			if err := tw.Close(); err != nil {
				t.Fatal(err)
			}
			if err := gz.Close(); err != nil {
				t.Fatal(err)
			}
			archivePath := filepath.Join(dir, "release.tar.gz")
			if err := os.WriteFile(archivePath, archive.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
			const fakeCurl = `#!/bin/sh
set -eu
while [ "$#" -gt 0 ]; do
  if [ "$1" = "-o" ]; then
    cp "$UNIC_TEST_ARCHIVE" "$2"
    exit
  fi
  shift
done
printf '%s\n' '{"tag_name":"v0.0.0-test"}'
`
			if err := os.WriteFile(filepath.Join(fakeBin, "curl"), []byte(fakeCurl), 0755); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, "sh", "install.sh")
			// Bound output draining if a subprocess keeps the shell's pipes open.
			cmd.WaitDelay = 5 * time.Second
			cmd.Env = append(os.Environ(),
				"PATH="+fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"),
				"INSTALL_DIR="+installDir,
				"UNIC_TEST_ARCHIVE="+archivePath,
			)
			output, err := cmd.CombinedOutput()
			if ctx.Err() != nil || errors.Is(err, exec.ErrWaitDelay) {
				t.Fatalf("installer exceeded time limit: %v (context: %v); output:\n%s", err, ctx.Err(), output)
			}
			if (err != nil) != tc.wantErr {
				t.Errorf("installer error = %v, want error %v; output:\n%s", err, tc.wantErr, output)
			}
			for _, name := range []string{"unic", "unic-mcp"} {
				wantContent, wantMode := tc.files[name], os.FileMode(0755)
				if tc.wantErr {
					wantContent, wantMode = "existing "+name, 0700
				}
				path := filepath.Join(installDir, name)
				content, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if string(content) != wantContent {
					t.Errorf("installed %s = %q, want %q", name, content, wantContent)
				}
				info, err := os.Stat(path)
				if err != nil {
					t.Fatal(err)
				}
				if info.Mode().Perm() != wantMode {
					t.Errorf("installed %s mode = %04o, want %04o", name, info.Mode().Perm(), wantMode)
				}
			}
		})
	}
}
