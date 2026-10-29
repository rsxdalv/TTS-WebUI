package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/rsxdalv/TTS-WebUI/core/internal/engine/manifest"
)

func validCatalog() catalogFile {
	return catalogFile{
		Schema: 1,
		Engines: []manifest.Manifest{
			{PackageName: "engine-a", Name: "Engine A"},
			{PackageName: "engine-b", Name: "Engine B"},
		},
	}
}

func serveCatalog(t *testing.T, cf catalogFile) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(cf)
	}))
}

func TestFetch_Success(t *testing.T) {
	srv := serveCatalog(t, validCatalog())
	defer srv.Close()

	cacheDir := t.TempDir()
	manifests, err := Fetch(context.Background(), srv.URL, cacheDir)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(manifests) != 2 {
		t.Errorf("expected 2 engines, got %d", len(manifests))
	}
	if manifests[0].PackageName != "engine-a" {
		t.Errorf("expected engine-a, got %q", manifests[0].PackageName)
	}

	// Cache file should exist.
	if _, err := os.Stat(filepath.Join(cacheDir, "catalog.json")); os.IsNotExist(err) {
		t.Error("cache file not written")
	}
}

func TestFetch_UnknownSchema(t *testing.T) {
	srv := serveCatalog(t, catalogFile{Schema: 99, Engines: []manifest.Manifest{{PackageName: "x"}}})
	defer srv.Close()

	// Pre-populate cache with a valid catalog.
	cacheDir := t.TempDir()
	_ = writeCatalog(cacheDir, validCatalog())

	manifests, err := Fetch(context.Background(), srv.URL, cacheDir)
	if !errors.Is(err, ErrUnknownSchema) {
		t.Errorf("expected ErrUnknownSchema, got %v", err)
	}
	// Should still return the cached copy.
	if len(manifests) != 2 {
		t.Errorf("expected 2 cached engines, got %d", len(manifests))
	}
}

func TestFetch_NetworkFailureFallback(t *testing.T) {
	cacheDir := t.TempDir()
	_ = writeCatalog(cacheDir, validCatalog())

	// Use a port that will definitely refuse connections.
	manifests, err := Fetch(context.Background(), "http://127.0.0.1:1", cacheDir)
	if err != nil {
		t.Fatalf("expected nil error with cache fallback, got %v", err)
	}
	if len(manifests) != 2 {
		t.Errorf("expected 2 cached engines, got %d", len(manifests))
	}
}

func TestFetch_NetworkFailureNoCache(t *testing.T) {
	cacheDir := t.TempDir()
	_, err := Fetch(context.Background(), "http://127.0.0.1:1", cacheDir)
	if err == nil {
		t.Error("expected error with no cache and network failure")
	}
}

func TestLoad_MissingCache(t *testing.T) {
	cacheDir := t.TempDir()
	_, err := Load(cacheDir)
	if err == nil {
		t.Error("expected error loading missing cache")
	}
}

func TestLoad_AfterFetch(t *testing.T) {
	srv := serveCatalog(t, validCatalog())
	defer srv.Close()

	cacheDir := t.TempDir()
	if _, err := Fetch(context.Background(), srv.URL, cacheDir); err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	manifests, err := Load(cacheDir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(manifests) != 2 {
		t.Errorf("expected 2 engines, got %d", len(manifests))
	}
}

func TestFetch_SchemaMissing(t *testing.T) {
	// Schema == 0 (missing) should succeed (legacy extensions.json format).
	cf := catalogFile{Schema: 0, Engines: []manifest.Manifest{{PackageName: "x"}}}
	srv := serveCatalog(t, cf)
	defer srv.Close()

	cacheDir := t.TempDir()
	manifests, err := Fetch(context.Background(), srv.URL, cacheDir)
	if err != nil {
		t.Fatalf("Fetch with schema=0: %v", err)
	}
	if len(manifests) != 1 {
		t.Errorf("expected 1 engine, got %d", len(manifests))
	}
}
