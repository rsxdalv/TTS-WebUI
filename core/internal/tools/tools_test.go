package tools

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
)

// fakeRoot creates a temp dir that looks like a TTS WebUI install root.
func fakeRoot(t *testing.T) paths.Root {
	t.Helper()
	base := t.TempDir()
	if err := os.MkdirAll(filepath.Join(base, "installer_scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	root, err := paths.New(base)
	if err != nil {
		t.Fatal(err)
	}
	return root
}

// makeZip produces an in-memory ZIP archive containing one file.
func makeZip(t *testing.T, name, content string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, err := w.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	f.Write([]byte(content))
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// makeTarGz produces an in-memory .tar.gz archive containing one file.
func makeTarGz(t *testing.T, name, content string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	body := []byte(content)
	tw.WriteHeader(&tar.Header{Name: name, Size: int64(len(body)), Mode: 0o644})
	tw.Write(body)
	tw.Close()
	gw.Close()
	return buf.Bytes()
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func TestEnsureToolDownloadAndNoOp(t *testing.T) {
	payload := makeZip(t, "uv.exe", "fake-uv-binary")
	payloadHash := sha256Hex(payload)

	// Serve the fake archive.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/zip")
		w.Write(payload)
	}))
	defer srv.Close()

	root := fakeRoot(t)
	dir := toolsDir(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	tool := Tool{
		Name:        "uv",
		Version:     "0.7.8",
		OS:          "*",
		Arch:        "*",
		URLTemplate: srv.URL + "/uv.zip",
		SHA256:      payloadHash,
	}

	ctx := context.Background()

	// First run: should download and extract.
	if err := ensureTool(ctx, dir, tool, http.DefaultClient); err != nil {
		t.Fatalf("first run: %v", err)
	}
	// Version file should exist.
	vf := filepath.Join(dir, "uv.version")
	data, err := os.ReadFile(vf)
	if err != nil {
		t.Fatalf("version file missing: %v", err)
	}
	if strings.TrimSpace(string(data)) != "0.7.8" {
		t.Errorf("version = %q, want 0.7.8", data)
	}

	// Second run with a server that always fails — should be a no-op.
	failSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "should not be called", http.StatusInternalServerError)
	}))
	defer failSrv.Close()
	tool2 := tool
	tool2.URLTemplate = failSrv.URL + "/uv.zip"
	if err := ensureTool(ctx, dir, tool2, http.DefaultClient); err != nil {
		t.Fatalf("second run (no-op) failed: %v", err)
	}
}

func TestHashMismatch(t *testing.T) {
	payload := makeZip(t, "uv.exe", "fake-binary")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(payload)
	}))
	defer srv.Close()

	root := fakeRoot(t)
	dir := toolsDir(root)
	os.MkdirAll(dir, 0o755)

	tool := Tool{
		Name:        "uv",
		Version:     "0.7.8",
		OS:          "*",
		Arch:        "*",
		URLTemplate: srv.URL + "/uv.zip",
		SHA256:      "0000000000000000000000000000000000000000000000000000000000000000",
	}

	err := ensureTool(context.Background(), dir, tool, http.DefaultClient)
	if err == nil {
		t.Fatal("expected error for hash mismatch, got nil")
	}
	if !strings.Contains(err.Error(), "SHA256 mismatch") {
		t.Errorf("error = %q, want SHA256 mismatch", err)
	}
}

func TestEmptyHashSkipsVerification(t *testing.T) {
	payload := makeZip(t, "ffmpeg.exe", "fake-ffmpeg")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(payload)
	}))
	defer srv.Close()

	root := fakeRoot(t)
	dir := toolsDir(root)
	os.MkdirAll(dir, 0o755)

	tool := Tool{
		Name:        "ffmpeg",
		Version:     "7.1",
		OS:          "*",
		Arch:        "*",
		URLTemplate: srv.URL + "/ffmpeg.zip",
		SHA256:      "", // should not verify
	}

	if err := ensureTool(context.Background(), dir, tool, http.DefaultClient); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExtractTarGz(t *testing.T) {
	payload := makeTarGz(t, "uv/uv", "fake-uv")
	destDir := t.TempDir()
	if err := extractTarGz(payload, destDir); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(destDir, "uv", "uv"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "fake-uv" {
		t.Errorf("content = %q", data)
	}
}

func TestPinnedSHA256NonEmpty(t *testing.T) {
	for _, tool := range pinned {
		if tool.SHA256 == "" {
			t.Errorf("tool %s/%s/%s has empty SHA256", tool.Name, tool.OS, tool.Arch)
		}
	}
}
