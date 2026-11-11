package api

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
)

// newTestRoot creates a minimal install root in a temp directory.
func newTestRoot(t *testing.T) paths.Root {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "installer_scripts"), 0o755); err != nil {
		t.Fatalf("mkdir installer_scripts: %v", err)
	}
	root, err := paths.New(dir)
	if err != nil {
		t.Fatalf("paths.New: %v", err)
	}
	return root
}

func TestHandleVersion(t *testing.T) {
	h := handleVersion()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/version", nil)
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, `"version"`) {
		t.Errorf("body %q missing version key", body)
	}
}

func TestHandleUpload_Success(t *testing.T) {
	root := newTestRoot(t)
	h := handleUpload(root)

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", "hello.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.WriteString(fw, "hello world")
	mw.Close()

	r := httptest.NewRequest(http.MethodPost, "/api/upload", &buf)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, "hello.txt") {
		t.Errorf("response %q does not mention filename", body)
	}
	// Verify the file was written.
	dest := filepath.Join(root.Outputs(), "uploads", "hello.txt")
	if _, err := os.Stat(dest); err != nil {
		t.Errorf("uploaded file not found at %s: %v", dest, err)
	}
}

func TestHandleUpload_SizeLimit(t *testing.T) {
	root := newTestRoot(t)
	h := handleUpload(root)

	// Build a body larger than uploadMaxBytes by providing a very large Content-Length.
	// MaxBytesReader will reject this.
	large := strings.NewReader(strings.Repeat("x", uploadMaxBytes+1))

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", "big.bin")
	_, _ = io.Copy(fw, large)
	mw.Close()

	r := httptest.NewRequest(http.MethodPost, "/api/upload", &buf)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code == http.StatusOK {
		t.Fatal("expected non-200 for oversized upload")
	}
}

func TestHandleGenerations_PathTraversal(t *testing.T) {
	root := newTestRoot(t)
	if err := os.MkdirAll(root.Outputs(), 0o755); err != nil {
		t.Fatal(err)
	}
	// Write a sentinel outside outputs/.
	secret := filepath.Join(root.Dir(), "secret.txt")
	if err := os.WriteFile(secret, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}

	h := handleGenerations(root)

	for _, attack := range []string{
		"../secret.txt",
		"sub/../../secret.txt",
		"%2e%2e%2fsecret.txt",
	} {
		url := fmt.Sprintf("/api/generations/outputs/%s", attack)
		r := httptest.NewRequest(http.MethodGet, url, nil)
		r.SetPathValue("collection", "outputs")
		r.SetPathValue("name", attack)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code == http.StatusOK {
			t.Errorf("path traversal %q returned 200, want 403/404", attack)
		}
	}
}

func TestHandleGenerations_Success(t *testing.T) {
	root := newTestRoot(t)
	dir := root.Outputs()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "test.wav"), []byte("RIFF"), 0o644); err != nil {
		t.Fatal(err)
	}

	h := handleGenerations(root)
	r := httptest.NewRequest(http.MethodGet, "/api/generations/outputs/test.wav", nil)
	r.SetPathValue("collection", "outputs")
	r.SetPathValue("name", "test.wav")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestHandleGenerations_UnknownCollection(t *testing.T) {
	root := newTestRoot(t)
	h := handleGenerations(root)
	r := httptest.NewRequest(http.MethodGet, "/api/generations/unknown/file.wav", nil)
	r.SetPathValue("collection", "unknown")
	r.SetPathValue("name", "file.wav")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}
