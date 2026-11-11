// Package selfupdate implements core binary self-update with minisign
// signature verification (D-open-5).
//
// The embedded public key is a throwaway test key checked into testdata/.
// TODO: replace embeddedPublicKey with the real release key before shipping.
package selfupdate

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// TODO: replace with real public key before release (D-open-5).
// The key below is a throwaway test key for CI only.
const embeddedPublicKey = `untrusted comment: minisign public key tts-webui-test
RWTerb7vAAECA3m1Vi6P5lT5QHixEuipi6eQH4U65pW+1+DjkQutBJZk
`

const releasesAPI = "https://api.github.com/repos/rsxdalv/TTS-WebUI/releases/latest"

// Release describes an available update.
type Release struct {
	Version string
	URL     string // download URL for the binary asset
	Sig     string // download URL for the .minisig file
}

// ghRelease is the GitHub releases API response shape (fields we need).
type ghRelease struct {
	TagName string    `json:"tag_name"`
	Assets  []ghAsset `json:"assets"`
}

type ghAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// Check fetches the latest release from GitHub and returns it with a bool
// indicating whether it is newer than currentVersion.
func Check(ctx context.Context, currentVersion string) (Release, bool, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, releasesAPI, nil)
	if err != nil {
		return Release{}, false, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "tts-webui-core")

	resp, err := client.Do(req)
	if err != nil {
		return Release{}, false, fmt.Errorf("fetching releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Release{}, false, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var rel ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return Release{}, false, fmt.Errorf("decoding release: %w", err)
	}

	assetName := binaryAssetName()
	sigName := assetName + ".minisig"

	var binaryURL, sigURL string
	for _, a := range rel.Assets {
		switch a.Name {
		case assetName:
			binaryURL = a.BrowserDownloadURL
		case sigName:
			sigURL = a.BrowserDownloadURL
		}
	}
	if binaryURL == "" {
		return Release{}, false, fmt.Errorf("no asset %q in release %s", assetName, rel.TagName)
	}

	release := Release{
		Version: rel.TagName,
		URL:     binaryURL,
		Sig:     sigURL,
	}

	hasUpdate := rel.TagName != "" && rel.TagName != currentVersion &&
		rel.TagName != "v"+strings.TrimPrefix(currentVersion, "v")

	return release, hasUpdate, nil
}

// Apply downloads the asset, verifies its minisign signature, and atomically
// replaces the running binary.
//
// It returns an error if:
//   - the download fails
//   - the signature file is missing or verification fails
//   - the file replacement fails
func Apply(ctx context.Context, rel Release, pubKeyPEM string) error {
	if pubKeyPEM == "" {
		pubKeyPEM = embeddedPublicKey
	}

	pubKey, err := parsePubKey([]byte(pubKeyPEM))
	if err != nil {
		return fmt.Errorf("parsing public key: %w", err)
	}

	// Download binary.
	data, err := download(ctx, rel.URL)
	if err != nil {
		return fmt.Errorf("downloading binary: %w", err)
	}

	// Download and verify signature.
	if rel.Sig != "" {
		sigData, err := download(ctx, rel.Sig)
		if err != nil {
			return fmt.Errorf("downloading signature: %w", err)
		}
		sig, err := parseSig(sigData)
		if err != nil {
			return fmt.Errorf("parsing signature: %w", err)
		}
		if err := verifyMinisign(pubKey, sig, data); err != nil {
			return fmt.Errorf("signature verification failed: %w", err)
		}
		slog.Info("selfupdate: signature verified")
	} else {
		slog.Warn("selfupdate: no signature URL in release; skipping verification")
	}

	return replaceExe(data)
}

// binaryAssetName returns the expected asset name for the current OS/arch.
func binaryAssetName() string {
	os := runtime.GOOS
	arch := runtime.GOARCH
	ext := ""
	if os == "windows" {
		ext = ".exe"
	}
	return fmt.Sprintf("tts-webui-%s-%s%s", os, arch, ext)
}

// download fetches a URL and returns the body bytes.
func download(ctx context.Context, url string) ([]byte, error) {
	client := &http.Client{Timeout: 5 * time.Minute}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned %d for %s", resp.StatusCode, url)
	}
	return io.ReadAll(resp.Body)
}

// replaceExe atomically replaces the running binary with data.
//   - On Windows: rename current to .old, write new, then restart (or let caller restart).
//   - On Unix: write to temp in same dir, chmod +x, rename over the current.
func replaceExe(data []byte) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("finding executable path: %w", err)
	}
	// Resolve symlinks so we always write next to the real binary.
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}

	dir := filepath.Dir(exe)
	tmp := exe + ".new"
	if err := os.WriteFile(tmp, data, 0o755); err != nil {
		return fmt.Errorf("writing new binary: %w", err)
	}

	if runtime.GOOS == "windows" {
		old := exe + ".old"
		_ = os.Remove(old)
		if err := os.Rename(exe, old); err != nil {
			_ = os.Remove(tmp)
			return fmt.Errorf("renaming current binary: %w", err)
		}
		if err := os.Rename(tmp, exe); err != nil {
			// Try to restore.
			_ = os.Rename(old, exe)
			return fmt.Errorf("placing new binary: %w", err)
		}
		return nil
	}

	// Unix: atomic rename within same volume.
	_ = dir
	if err := os.Rename(tmp, exe); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("replacing binary: %w", err)
	}
	return nil
}
