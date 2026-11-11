// Package native resolves, downloads, verifies, and launches native engine
// binaries (e.g. audio.cpp) distributed as GitHub release assets.
package native

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

	"github.com/rsxdalv/TTS-WebUI/core/internal/engine/lifecycle"
	"github.com/rsxdalv/TTS-WebUI/core/internal/engine/manifest"
)

// HTTPClient is used for all downloads; replace in tests.
var HTTPClient = http.DefaultClient

// Resolve downloads, verifies, and extracts the native engine binary for the
// current OS/arch and the given gpuLabel, then returns the absolute path to the
// main binary.  If a cached copy already exists, the download is skipped.
func Resolve(ctx context.Context, m manifest.Manifest, gpuLabel, cacheDir string) (string, error) {
	a := m.Engine.NativeAsset
	if a == nil {
		return "", fmt.Errorf("engine %q: missing native_asset in manifest", m.PackageName)
	}
	if a.URLTemplate == "" {
		return "", fmt.Errorf("engine %q: native_asset.url_template is empty", m.PackageName)
	}
	if a.Version == "" {
		return "", fmt.Errorf("engine %q: native_asset.version is empty", m.PackageName)
	}

	rawURL := resolveURL(a.URLTemplate, a.Version, gpuLabel)
	assetFilename := urlFilename(rawURL)
	versionDir := filepath.Join(cacheDir, m.PackageName, a.Version)
	binRelPath := a.BinaryPath
	if binRelPath == "" {
		binRelPath = defaultBinaryPath(m.PackageName)
	}
	binPath := filepath.Join(versionDir, filepath.FromSlash(binRelPath))

	// Fast path: binary already cached.
	if _, err := os.Stat(binPath); err == nil {
		return binPath, nil
	}

	data, err := downloadURL(ctx, rawURL)
	if err != nil {
		return "", fmt.Errorf("downloading %s: %w", rawURL, err)
	}

	if err := verifyChecksum(ctx, data, assetFilename, rawURL, a.Checksums); err != nil {
		return "", err
	}

	if err := os.MkdirAll(versionDir, 0o755); err != nil {
		return "", fmt.Errorf("creating cache dir: %w", err)
	}
	if err := extractArchive(rawURL, data, versionDir); err != nil {
		return "", fmt.Errorf("extracting %s: %w", assetFilename, err)
	}

	if _, err := os.Stat(binPath); err != nil {
		return "", fmt.Errorf("binary not found at %q after extraction: %w", binPath, err)
	}
	// Make executable on Unix; harmless no-op on Windows.
	_ = os.Chmod(binPath, 0o755)
	return binPath, nil
}

// Launch resolves the native binary, wires it into the manifest's launch command,
// registers it in mgr, and starts it.
func Launch(ctx context.Context, mgr *lifecycle.Manager, m manifest.Manifest, gpuLabel, cacheDir string) error {
	binPath, err := Resolve(ctx, m, gpuLabel, cacheDir)
	if err != nil {
		return err
	}

	// Patch the manifest so lifecycle.Manager uses the resolved binary.
	patched := m
	launch := manifest.Launch{}
	if m.Engine.Launch != nil {
		launch = *m.Engine.Launch
	}
	if len(launch.Cmd) == 0 {
		launch.Cmd = []string{binPath}
	} else {
		args := make([]string, len(launch.Cmd))
		copy(args, launch.Cmd)
		args[0] = binPath
		launch.Cmd = args
	}
	patched.Engine.Launch = &launch

	if err := mgr.Register(patched); err != nil {
		return fmt.Errorf("registering engine %q: %w", m.PackageName, err)
	}
	return mgr.Start(ctx, m.PackageName)
}

// resolveURL substitutes {version}, {os}, {arch}, {gpu}, {ext} in tmpl.
func resolveURL(tmpl, version, gpuLabel string) string {
	ext := archiveExt()
	r := strings.NewReplacer(
		"{version}", version,
		"{os}", runtime.GOOS,
		"{arch}", runtime.GOARCH,
		"{gpu}", gpuVariant(gpuLabel),
		"{ext}", ext,
	)
	return r.Replace(tmpl)
}

// gpuVariant maps a PyTorchChoice label to the URL-safe backend token used
// in native engine asset names (e.g. audio.cpp release assets).
func gpuVariant(label string) string {
	switch label {
	case "NVIDIA GPU":
		return "cuda12.4"
	case "AMD GPU (ROCM, Linux only)":
		return "rocm10.0.0-test"
	case "Apple M Series Chip":
		return "metal"
	case "Intel GPU (XPU)":
		return "vulkan-portable"
	case "CPU":
		return "cpu"
	default:
		return "cpu"
	}
}

func archiveExt() string {
	if runtime.GOOS == "windows" {
		return "zip"
	}
	return "tar.gz"
}

// urlFilename returns the last path segment of a URL.
func urlFilename(rawURL string) string {
	if i := strings.LastIndex(rawURL, "/"); i >= 0 {
		return rawURL[i+1:]
	}
	return rawURL
}

// defaultBinaryPath returns the expected binary name for pkgName on the current OS.
func defaultBinaryPath(pkgName string) string {
	name := pkgName
	if i := strings.LastIndex(pkgName, "/"); i >= 0 {
		name = pkgName[i+1:]
	}
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

// verifyChecksum checks the SHA256 of data against the checksums map, and
// falls back to fetching a sidecar .sha256 file.  If neither source provides
// a digest, verification is skipped (a warning is appropriate but not emitted
// here to keep the package side-effect-free).
func verifyChecksum(ctx context.Context, data []byte, filename, assetURL string, checksums map[string]string) error {
	expected := ""
	if cs, ok := checksums[filename]; ok {
		expected = strings.TrimSpace(cs)
	}
	if expected == "" {
		if cs, err := downloadURL(ctx, assetURL+".sha256"); err == nil {
			fields := strings.Fields(string(cs))
			if len(fields) > 0 {
				expected = strings.ToLower(fields[0])
			}
		}
	}
	if expected == "" {
		return fmt.Errorf("no checksum available for %s; refusing to install", filename)
	}
	sum := sha256.Sum256(data)
	got := hex.EncodeToString(sum[:])
	if got != strings.ToLower(expected) {
		return fmt.Errorf("SHA256 mismatch for %s: got %s, want %s", filename, got, strings.ToLower(expected))
	}
	return nil
}

// downloadURL fetches rawURL and returns the body bytes.
func downloadURL(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, rawURL)
	}
	return io.ReadAll(resp.Body)
}

// extractArchive dispatches to the correct extractor based on the URL suffix.
func extractArchive(rawURL string, data []byte, destDir string) error {
	switch {
	case strings.HasSuffix(rawURL, ".zip"):
		return extractZip(data, destDir)
	case strings.HasSuffix(rawURL, ".tar.gz"):
		return extractTarGz(data, destDir)
	case strings.HasSuffix(rawURL, ".tar.xz"):
		return extractTarXZ(data, destDir)
	default:
		return fmt.Errorf("unsupported archive format: %s", rawURL)
	}
}

func extractZip(data []byte, destDir string) error {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}
	for _, f := range zr.File {
		if err := extractZipEntry(f, destDir); err != nil {
			return err
		}
	}
	return nil
}

func extractZipEntry(f *zip.File, destDir string) error {
	name := filepath.FromSlash(f.Name)
	target := filepath.Join(destDir, name)
	// Security: reject path traversal.
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

func extractTarGz(data []byte, destDir string) error {
	gr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer gr.Close()
	return extractTarStream(tar.NewReader(gr), destDir)
}

func extractTarXZ(data []byte, destDir string) error {
	tmp, err := os.CreateTemp("", "tts-native-*.tar.xz")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	tmp.Close()
	cmd := tarXZCommand(tmp.Name(), destDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("tar: %w (output: %s)", err, out)
	}
	return nil
}

func extractTarStream(tr *tar.Reader, destDir string) error {
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
			_, copyErr := io.Copy(out, tr)
			out.Close()
			if copyErr != nil {
				return copyErr
			}
		}
	}
}
