package native

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rsxdalv/TTS-WebUI/core/internal/engine/manifest"
)

// makeZipArchive creates a minimal ZIP archive containing one file at relPath
// with the given content and returns the raw bytes.
func makeZipArchive(relPath, content string) []byte {
	buf := &bytes.Buffer{}
	zw := zip.NewWriter(buf)
	w, _ := zw.Create(relPath)
	_, _ = w.Write([]byte(content))
	zw.Close()
	return buf.Bytes()
}

// fakeAssetServer starts an httptest server serving a ZIP that contains
// binRelPath with binaryContent, and optionally a .sha256 sidecar.
// Returns the server, the asset URL (no trailing /), and cleanup func.
func fakeAssetServer(t *testing.T, assetFilename, binRelPath, binaryContent string, injectSidecar bool) (*httptest.Server, string) {
	t.Helper()
	archiveBytes := makeZipArchive(binRelPath, binaryContent)
	sum := sha256.Sum256(archiveBytes)
	hex256 := hex.EncodeToString(sum[:])

	mux := http.NewServeMux()
	mux.HandleFunc("/"+assetFilename, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/zip")
		_, _ = w.Write(archiveBytes)
	})
	if injectSidecar {
		mux.HandleFunc("/"+assetFilename+".sha256", func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(hex256 + "  " + assetFilename + "\n"))
		})
	}
	srv := httptest.NewServer(mux)
	return srv, hex256
}

// makeManifest constructs a minimal native manifest using the given template.
func makeManifest(urlTemplate, version, binRelPath string, checksums map[string]string) manifest.Manifest {
	return manifest.Manifest{
		PackageName: "test-engine",
		Name:        "Test Engine",
		Engine: manifest.Engine{
			Kind: manifest.KindNative,
			Health: &manifest.Health{
				Path: "/health",
			},
			NativeAsset: &manifest.NativeAsset{
				URLTemplate: urlTemplate,
				Version:     version,
				BinaryPath:  binRelPath,
				Checksums:   checksums,
			},
		},
	}
}

// TestResolve_ZipDownloadAndExtract verifies that Resolve downloads a fake ZIP,
// extracts it, and returns the binary path.
func TestResolve_ZipDownloadAndExtract(t *testing.T) {
	binRelPath := "bin/test-engine"
	if runtime.GOOS == "windows" {
		binRelPath = "bin/test-engine.exe"
	}
	assetFilename := "test-engine-" + runtime.GOOS + "-" + runtime.GOARCH + "-cpu.zip"
	binaryContent := "#!/bin/sh\necho ok\n"

	srv, _ := fakeAssetServer(t, assetFilename, binRelPath, binaryContent, false)
	defer srv.Close()

	// Use no checksums; just test download+extract.
	urlTemplate := srv.URL + "/test-engine-{os}-{arch}-{gpu}.zip"
	m := makeManifest(urlTemplate, "1.0.0", binRelPath, nil)

	origClient := HTTPClient
	HTTPClient = srv.Client()
	defer func() { HTTPClient = origClient }()

	cacheDir := t.TempDir()
	binPath, err := Resolve(context.Background(), m, "cpu", cacheDir)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !strings.HasSuffix(filepath.ToSlash(binPath), binRelPath) {
		t.Errorf("binary path %q does not end with %q", binPath, binRelPath)
	}
	got, err := os.ReadFile(binPath)
	if err != nil {
		t.Fatalf("reading binary: %v", err)
	}
	if string(got) != binaryContent {
		t.Errorf("binary content mismatch: got %q, want %q", got, binaryContent)
	}
}

// TestResolve_ChecksumMap verifies that a matching checksum in the checksums
// map passes and a wrong checksum returns an error.
func TestResolve_ChecksumMap(t *testing.T) {
	binRelPath := "mybin"
	if runtime.GOOS == "windows" {
		binRelPath = "mybin.exe"
	}
	assetFilename := "mybin-" + runtime.GOOS + "-" + runtime.GOARCH + "-cpu.zip"
	archiveBytes := makeZipArchive(binRelPath, "content")
	sum := sha256.Sum256(archiveBytes)
	correctHex := hex.EncodeToString(sum[:])

	tests := []struct {
		name      string
		checksum  string
		wantError bool
	}{
		{"correct checksum", correctHex, false},
		{"wrong checksum", strings.Repeat("a", 64), true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write(archiveBytes)
			}))
			defer srv.Close()

			urlTemplate := srv.URL + "/mybin-{os}-{arch}-{gpu}.zip"
			m := makeManifest(urlTemplate, "2.0.0", binRelPath, map[string]string{
				assetFilename: tc.checksum,
			})

			origClient := HTTPClient
			HTTPClient = srv.Client()
			defer func() { HTTPClient = origClient }()

			cacheDir := t.TempDir()
			_, err := Resolve(context.Background(), m, "cpu", cacheDir)
			if tc.wantError && err == nil {
				t.Error("expected checksum error, got nil")
			}
			if !tc.wantError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestResolve_SidecarChecksum verifies that the sidecar .sha256 URL is
// fetched and used when the checksums map is empty.
func TestResolve_SidecarChecksum(t *testing.T) {
	binRelPath := "engine"
	if runtime.GOOS == "windows" {
		binRelPath = "engine.exe"
	}
	assetFilename := "engine-" + runtime.GOOS + "-" + runtime.GOARCH + "-cuda.zip"
	srv, _ := fakeAssetServer(t, assetFilename, binRelPath, "sidecar-test", true)
	defer srv.Close()

	urlTemplate := srv.URL + "/" + "engine-{os}-{arch}-{gpu}.zip"
	m := makeManifest(urlTemplate, "0.5.0", binRelPath, nil)

	origClient := HTTPClient
	HTTPClient = srv.Client()
	defer func() { HTTPClient = origClient }()

	cacheDir := t.TempDir()
	_, err := Resolve(context.Background(), m, "cuda", cacheDir)
	if err != nil {
		t.Fatalf("Resolve with sidecar checksum: %v", err)
	}
}

// TestResolve_CacheHit verifies that a second Resolve call skips the download.
func TestResolve_CacheHit(t *testing.T) {
	downloads := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".sha256") {
			http.NotFound(w, r)
			return
		}
		downloads++
		binRelPath := "cached-bin"
		if runtime.GOOS == "windows" {
			binRelPath = "cached-bin.exe"
		}
		_, _ = w.Write(makeZipArchive(binRelPath, "cached"))
	}))
	defer srv.Close()

	binRelPath := "cached-bin"
	if runtime.GOOS == "windows" {
		binRelPath = "cached-bin.exe"
	}
	urlTemplate := srv.URL + "/cached-bin-{os}-{arch}-{gpu}.zip"
	m := makeManifest(urlTemplate, "1.1.0", binRelPath, nil)

	origClient := HTTPClient
	HTTPClient = srv.Client()
	defer func() { HTTPClient = origClient }()

	cacheDir := t.TempDir()
	if _, err := Resolve(context.Background(), m, "cpu", cacheDir); err != nil {
		t.Fatalf("first Resolve: %v", err)
	}
	if _, err := Resolve(context.Background(), m, "cpu", cacheDir); err != nil {
		t.Fatalf("second Resolve: %v", err)
	}
	if downloads != 1 {
		t.Errorf("expected 1 download, got %d", downloads)
	}
}

// TestResolve_MissingNativeAsset verifies that a manifest without NativeAsset
// returns an appropriate error.
func TestResolve_MissingNativeAsset(t *testing.T) {
	m := manifest.Manifest{
		PackageName: "no-asset",
		Engine:      manifest.Engine{Kind: manifest.KindNative},
	}
	_, err := Resolve(context.Background(), m, "cpu", t.TempDir())
	if err == nil {
		t.Fatal("expected error for missing native_asset")
	}
}
