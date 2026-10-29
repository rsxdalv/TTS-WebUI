// Package tools downloads and verifies pinned tool binaries (uv, ffmpeg, MinGit).
package tools

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
)

// Tool describes a single pinned binary.
type Tool struct {
	Name        string // binary name, e.g. "uv"
	Version     string // semver string without "v"
	OS          string // "windows", "linux", "darwin", or "*" for all
	Arch        string // "amd64", "arm64", or "*" for all
	URLTemplate string // {version}, {os}, {arch}, {ext} are substituted
	SHA256      string // hex-encoded SHA-256 of the downloaded archive; "" skips check
}

// pinned is the tool table embedded at build time.
// SHA256 values are listed per platform below; "" means "TODO: fill in after first download".
var pinned = []Tool{
	// uv — Python package manager
	{
		Name:    "uv",
		Version: "0.7.8",
		OS:      "windows",
		Arch:    "amd64",
		// TODO: fill SHA256 from https://github.com/astral-sh/uv/releases/tag/0.7.8
		URLTemplate: "https://github.com/astral-sh/uv/releases/download/{version}/uv-x86_64-pc-windows-msvc.zip",
		SHA256:      "",
	},
	{
		Name:        "uv",
		Version:     "0.7.8",
		OS:          "linux",
		Arch:        "amd64",
		URLTemplate: "https://github.com/astral-sh/uv/releases/download/{version}/uv-x86_64-unknown-linux-gnu.tar.gz",
		SHA256:      "",
	},
	{
		Name:        "uv",
		Version:     "0.7.8",
		OS:          "darwin",
		Arch:        "amd64",
		URLTemplate: "https://github.com/astral-sh/uv/releases/download/{version}/uv-x86_64-apple-darwin.tar.gz",
		SHA256:      "",
	},
	{
		Name:        "uv",
		Version:     "0.7.8",
		OS:          "darwin",
		Arch:        "arm64",
		URLTemplate: "https://github.com/astral-sh/uv/releases/download/{version}/uv-aarch64-apple-darwin.tar.gz",
		SHA256:      "",
	},
	// ffmpeg — audio/video processing
	{
		Name:        "ffmpeg",
		Version:     "7.1",
		OS:          "windows",
		Arch:        "amd64",
		URLTemplate: "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-n7.1-latest-win64-gpl-7.1.zip",
		SHA256:      "", // TODO: fill from BtbN release page (varies by rebuild date)
	},
	{
		Name:        "ffmpeg",
		Version:     "7.1",
		OS:          "linux",
		Arch:        "amd64",
		URLTemplate: "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-n7.1-latest-linux64-gpl-7.1.tar.xz",
		SHA256:      "",
	},
	// MinGit — Windows only
	{
		Name:        "mingit",
		Version:     "2.49.0",
		OS:          "windows",
		Arch:        "amd64",
		URLTemplate: "https://github.com/git-for-windows/git/releases/download/v{version}.windows.1/MinGit-{version}-64-bit.zip",
		SHA256:      "",
	},
}

// toolsDir returns installer_files/tools/ inside root.
func toolsDir(root paths.Root) string {
	return filepath.Join(root.InstallerFiles(), "tools")
}

// EnsureAll downloads and installs every pinned tool applicable to the
// current OS/arch, skipping ones already present with a matching version.
func EnsureAll(ctx context.Context, root paths.Root) error {
	return EnsureAllWithClient(ctx, root, http.DefaultClient)
}

// EnsureAllWithClient is like EnsureAll but accepts an injectable HTTP client.
func EnsureAllWithClient(ctx context.Context, root paths.Root, client *http.Client) error {
	dir := toolsDir(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating tools dir: %w", err)
	}
	for _, t := range pinned {
		if !t.appliesToCurrent() {
			continue
		}
		if err := ensureTool(ctx, dir, t, client); err != nil {
			return fmt.Errorf("tool %s: %w", t.Name, err)
		}
	}
	return nil
}

// appliesToCurrent returns true when the tool entry matches the running OS/arch.
func (t Tool) appliesToCurrent() bool {
	goos := runtime.GOOS
	goarch := runtime.GOARCH
	if t.OS != "*" && t.OS != goos {
		return false
	}
	if t.Arch != "*" && t.Arch != goarch {
		return false
	}
	return true
}

// resolvedURL substitutes {version}, {os}, {arch}, {ext} in URLTemplate.
func (t Tool) resolvedURL() string {
	ext := "tar.gz"
	if runtime.GOOS == "windows" {
		ext = "zip"
	}
	r := strings.NewReplacer(
		"{version}", t.Version,
		"{os}", runtime.GOOS,
		"{arch}", runtime.GOARCH,
		"{ext}", ext,
	)
	return r.Replace(t.URLTemplate)
}

func ensureTool(ctx context.Context, dir string, t Tool, client *http.Client) error {
	versionFile := filepath.Join(dir, t.Name+".version")
	if data, err := os.ReadFile(versionFile); err == nil {
		if strings.TrimSpace(string(data)) == t.Version {
			return nil // already installed, skip
		}
	}

	rawURL := t.resolvedURL()
	data, err := download(ctx, client, rawURL)
	if err != nil {
		return fmt.Errorf("downloading %s: %w", rawURL, err)
	}

	if t.SHA256 != "" {
		sum := sha256.Sum256(data)
		got := hex.EncodeToString(sum[:])
		if got != strings.ToLower(t.SHA256) {
			return fmt.Errorf("SHA256 mismatch for %s: got %s, want %s", t.Name, got, t.SHA256)
		}
	}

	destDir := filepath.Join(dir, t.Name)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("creating dest dir: %w", err)
	}

	switch {
	case strings.HasSuffix(rawURL, ".zip"):
		if err := extractZip(data, destDir); err != nil {
			return fmt.Errorf("extracting zip: %w", err)
		}
	case strings.HasSuffix(rawURL, ".tar.gz"):
		if err := extractTarGz(data, destDir); err != nil {
			return fmt.Errorf("extracting tar.gz: %w", err)
		}
	case strings.HasSuffix(rawURL, ".tar.xz"):
		if err := extractTarXZ(destDir, data); err != nil {
			return fmt.Errorf("extracting tar.xz: %w", err)
		}
	default:
		return fmt.Errorf("unknown archive format: %s", rawURL)
	}

	if err := os.WriteFile(versionFile, []byte(t.Version+"\n"), 0o644); err != nil {
		return fmt.Errorf("writing version file: %w", err)
	}
	return nil
}

// download fetches url and returns the response body.
func download(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}
	return io.ReadAll(resp.Body)
}

// extractZip extracts all files from data (a ZIP archive) into destDir.
func extractZip(data []byte, destDir string) error {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}
	for _, f := range zr.File {
		if err := extractZipFile(f, destDir); err != nil {
			return err
		}
	}
	return nil
}

func extractZipFile(f *zip.File, destDir string) error {
	// Security: reject entries with path traversal.
	name := filepath.FromSlash(f.Name)
	target := filepath.Join(destDir, name)
	if !strings.HasPrefix(target, filepath.Clean(destDir)+string(os.PathSeparator)) && target != filepath.Clean(destDir) {
		return fmt.Errorf("zip path traversal: %s", f.Name)
	}
	if f.FileInfo().IsDir() {
		return os.MkdirAll(target, 0o755)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode())
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, rc)
	return err
}

// extractTarGz extracts a gzip-compressed tar archive.
func extractTarGz(data []byte, destDir string) error {
	gr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer gr.Close()
	return extractTar(tar.NewReader(gr), destDir)
}

// extractTarXZ shells out to the system tar to handle xz compression, which
// is not available in the Go stdlib.
func extractTarXZ(destDir string, data []byte) error {
	// Write to a temp file because tar needs seekable input for xz.
	tmp, err := os.CreateTemp("", "tts-tool-*.tar.xz")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	tmp.Close()

	// `tar` is available on Linux/macOS and Windows 10+ (bsdtar).
	cmd := tarCommand(tmp.Name(), destDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("tar: %w (output: %s)", err, out)
	}
	return nil
}

func extractTar(tr *tar.Reader, destDir string) error {
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		target := filepath.Join(destDir, filepath.FromSlash(hdr.Name))
		if !strings.HasPrefix(target, filepath.Clean(destDir)+string(os.PathSeparator)) && target != filepath.Clean(destDir) {
			return fmt.Errorf("tar path traversal: %s", hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
		}
	}
}
