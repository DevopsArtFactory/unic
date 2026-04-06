package update

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	repoOwner    = "DevopsArtFactory"
	repoName     = "unic"
	cacheFile    = "update-check.json"
	cacheTTL     = 24 * time.Hour
	apiURL       = "https://api.github.com/repos/" + repoOwner + "/" + repoName + "/releases/latest"
	downloadBase = "https://github.com/" + repoOwner + "/" + repoName + "/releases/download"
)

// InstallMethod represents how unic was installed.
type InstallMethod int

const (
	InstallUnknown InstallMethod = iota
	InstallBrew
	InstallBinary
)

// DetectInstallMethod checks if unic was installed via Homebrew.
func DetectInstallMethod() InstallMethod {
	execPath, err := os.Executable()
	if err != nil {
		return InstallUnknown
	}
	resolved, err := filepath.EvalSymlinks(execPath)
	if err != nil {
		return InstallUnknown
	}
	if strings.Contains(resolved, "Cellar") || strings.Contains(resolved, "homebrew") {
		return InstallBrew
	}
	return InstallBinary
}

// cache stores the last update check result.
type cache struct {
	Version   string    `json:"version"`
	CheckedAt time.Time `json:"checked_at"`
}

// cacheDir returns the unic config directory (~/.config/unic/).
func cacheDir() (string, error) {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "unic"), nil
}

// cachePath returns the full path to the update check cache file.
func cachePath() (string, error) {
	dir, err := cacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, cacheFile), nil
}

// readCache reads the cached update check result.
func readCache() (*cache, error) {
	path, err := cachePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c cache
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// writeCache writes the update check result to the cache file.
func writeCache(version string) error {
	path, err := cachePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	c := cache{Version: version, CheckedAt: time.Now()}
	data, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// ShouldCheck returns true if the cache is missing or older than 24h.
func ShouldCheck() bool {
	c, err := readCache()
	if err != nil {
		return true
	}
	return time.Since(c.CheckedAt) > cacheTTL
}

// CheckLatestVersion queries the GitHub releases API and returns the latest version tag.
func CheckLatestVersion() (string, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}
	return release.TagName, nil
}

// CheckForUpdate checks if a newer version is available, using cache when possible.
// Returns the new version string if available, or empty string if up to date.
func CheckForUpdate(currentVersion string) string {
	if currentVersion == "dev" {
		return ""
	}

	// Try cache first
	if !ShouldCheck() {
		c, err := readCache()
		if err == nil && IsNewer(currentVersion, c.Version) {
			return c.Version
		}
		return ""
	}

	latest, err := CheckLatestVersion()
	if err != nil {
		return ""
	}

	// Update cache regardless of result
	_ = writeCache(latest)

	if IsNewer(currentVersion, latest) {
		return latest
	}
	return ""
}

// IsNewer returns true if latest is a newer version than current.
// Both may have a "v" prefix (e.g., "v0.1.0" or "0.1.0").
func IsNewer(current, latest string) bool {
	c := normalizeVersion(current)
	l := normalizeVersion(latest)
	if c == "" || l == "" {
		return false
	}
	return compareVersions(l, c) > 0
}

// compareVersions compares two dot-separated version strings segment by segment.
// Returns positive if a > b, negative if a < b, zero if equal.
func compareVersions(a, b string) int {
	aParts := strings.Split(a, ".")
	bParts := strings.Split(b, ".")
	maxLen := max(len(aParts), len(bParts))
	for i := 0; i < maxLen; i++ {
		ai := parseVersionSegment(aParts, i)
		bi := parseVersionSegment(bParts, i)
		if ai != bi {
			return ai - bi
		}
	}
	return 0
}

// parseVersionSegment extracts a numeric segment from a version parts slice.
// Returns 0 for out-of-range indices or non-numeric segments.
func parseVersionSegment(parts []string, i int) int {
	if i >= len(parts) {
		return 0
	}
	var n int
	if _, err := fmt.Sscanf(parts[i], "%d", &n); err != nil {
		return 0
	}
	return n
}

// normalizeVersion strips "v" prefix for comparison.
func normalizeVersion(v string) string {
	return strings.TrimPrefix(v, "v")
}

// archiveName returns the expected archive filename for the current platform.
func archiveName(version string) string {
	os := runtime.GOOS
	arch := runtime.GOARCH
	v := strings.TrimPrefix(version, "v")
	if os == "windows" {
		return fmt.Sprintf("unic-%s-%s.zip", os, arch)
	}
	_ = v
	return fmt.Sprintf("unic-%s-%s.tar.gz", os, arch)
}

// DownloadURL returns the download URL for the given version and current platform.
func DownloadURL(version string) string {
	return fmt.Sprintf("%s/%s/%s", downloadBase, version, archiveName(version))
}

// DownloadAndReplace downloads the given version and replaces the current binary.
func DownloadAndReplace(version string) error {
	url := DownloadURL(version)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	// Extract binary from tar.gz
	binary, err := extractBinaryFromTarGz(resp.Body)
	if err != nil {
		return fmt.Errorf("extract failed: %w", err)
	}

	// Get current binary path
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not determine executable path: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("could not resolve symlinks: %w", err)
	}

	// Write to temp file in same directory, then atomic rename
	dir := filepath.Dir(execPath)
	tmp, err := os.CreateTemp(dir, "unic-update-*")
	if err != nil {
		return fmt.Errorf("could not create temp file: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(binary); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("write failed: %w", err)
	}
	tmp.Close()

	if err := os.Chmod(tmpPath, 0755); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("chmod failed: %w", err)
	}

	if err := os.Rename(tmpPath, execPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("replace failed (may need sudo): %w", err)
	}

	return nil
}

// extractBinaryFromTarGz reads a tar.gz stream and returns the "unic" binary contents.
func extractBinaryFromTarGz(r io.Reader) ([]byte, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if filepath.Base(hdr.Name) == "unic" && hdr.Typeflag == tar.TypeReg {
			return io.ReadAll(tr)
		}
	}
	return nil, fmt.Errorf("binary 'unic' not found in archive")
}
