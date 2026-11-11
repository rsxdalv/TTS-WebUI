// Package app manages the tts-webui Python application package via uv.
//
// It replaces the git-pull-based update in init_app.js with a pinned
// PyPI package install (D-open-3). Existing git-based installs are detected
// by the presence of .git_version without .app_version and are migrated
// transparently on the first Update call.
package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
	"github.com/rsxdalv/TTS-WebUI/core/internal/tools"
)

const defaultPyPIURL = "https://pypi.org/pypi/tts-webui/json"

// ErrUpdateAvailable is returned by callers when --check-only is set and a
// newer version is available. Use errors.Is to distinguish from a genuine failure.
var ErrUpdateAvailable = errors.New("update available")

// pypiResponse is the subset of the PyPI JSON API we need.
type pypiResponse struct {
	Info struct {
		Version string `json:"version"`
	} `json:"info"`
}

// CurrentVersion reads the installed app version from installer_scripts/.app_version.
// Returns ("", nil) when the marker does not exist (first install or unmigrated git install).
func CurrentVersion(root paths.Root) (string, error) {
	data, err := os.ReadFile(root.MarkerFile(".app_version"))
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("reading .app_version: %w", err)
	}
	// The marker may contain a migration note on the first line; the version is
	// on the first non-empty line that looks like a semver.
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "migrated") {
			return line, nil
		}
	}
	return strings.TrimSpace(string(data)), nil
}

// IsMigratingFromGit reports whether this is a git-based install that has not
// yet been converted to the uv package model.
// True when .git_version exists but .app_version does not.
func IsMigratingFromGit(root paths.Root) bool {
	_, errGit := os.Stat(root.MarkerFile(".git_version"))
	_, errApp := os.Stat(root.MarkerFile(".app_version"))
	return errGit == nil && os.IsNotExist(errApp)
}

// LatestVersion queries PyPI for the latest published version of tts-webui.
func LatestVersion(ctx context.Context) (string, error) {
	return latestVersion(ctx, defaultPyPIURL)
}

func latestVersion(ctx context.Context, url string) (string, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("building PyPI request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "tts-webui-core")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching PyPI: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("PyPI returned HTTP %d", resp.StatusCode)
	}

	// Bound the response to prevent memory exhaustion from a misbehaving server.
	resp.Body = http.MaxBytesReader(nil, resp.Body, 1<<20) // 1 MiB

	var info pypiResponse
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return "", fmt.Errorf("decoding PyPI response: %w", err)
	}
	if info.Info.Version == "" {
		return "", fmt.Errorf("PyPI response missing version field")
	}
	return info.Info.Version, nil
}

// Update fetches the latest tts-webui version from PyPI and installs it.
// gpuLabel is accepted for forward compatibility (GPU-specific extras); currently
// the base package is installed and torch is expected to be present from C4.2.
// TODO: map gpuLabel to package extras (e.g. tts-webui[cuda]) once defined.
func Update(ctx context.Context, root paths.Root, gpuLabel string) error {
	version, err := LatestVersion(ctx)
	if err != nil {
		return fmt.Errorf("checking latest version: %w", err)
	}
	return UpdateTo(ctx, root, gpuLabel, version)
}

// UpdateTo installs a specific version of tts-webui into the main venv.
// Using a pre-fetched version avoids a second PyPI round-trip in the common
// update flow where LatestVersion was already called by the caller.
func UpdateTo(ctx context.Context, root paths.Root, gpuLabel, version string) error {
	_ = gpuLabel // reserved for GPU-specific extras; not yet used

	if err := tools.EnsureAll(ctx, root); err != nil {
		return fmt.Errorf("ensuring tools: %w", err)
	}

	venvPath := filepath.Join(root.Venvs(), "main")
	uv := uvBinary(root)

	pkg := "tts-webui==" + version
	cmd := exec.CommandContext(ctx, uv, "pip", "install", "--python", venvPath, pkg)
	cmd.Env = uvEnviron(root)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("uv pip install %s: %w\n%s", pkg, err, out)
	}

	var markerContent string
	if IsMigratingFromGit(root) {
		markerContent = fmt.Sprintf("migrated from git install\n%s\n", version)
	} else {
		markerContent = version + "\n"
	}
	return writeMarkerAtomic(root.MarkerFile(".app_version"), markerContent)
}

// uvBinary returns the path to the uv executable within the tools directory.
func uvBinary(root paths.Root) string {
	name := "uv"
	if runtime.GOOS == "windows" {
		name = "uv.exe"
	}
	return filepath.Join(root.InstallerFiles(), "tools", "uv", name)
}

// uvEnviron returns os.Environ() with UV_CACHE_DIR set for this install root.
// Any existing UV_CACHE_DIR entry is removed before appending to avoid duplicates.
func uvEnviron(root paths.Root) []string {
	cacheDir := filepath.Join(root.InstallerFiles(), "uvcache")
	base := os.Environ()
	filtered := make([]string, 0, len(base)+1)
	for _, kv := range base {
		if !strings.HasPrefix(kv, "UV_CACHE_DIR=") {
			filtered = append(filtered, kv)
		}
	}
	return append(filtered, "UV_CACHE_DIR="+cacheDir)
}

// writeMarkerAtomic writes content to path atomically via a temp-file rename.
func writeMarkerAtomic(path, content string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating marker dir: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".app_version-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temp marker: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op after successful rename

	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		return fmt.Errorf("writing temp marker: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing temp marker: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("renaming marker: %w", err)
	}
	return nil
}
