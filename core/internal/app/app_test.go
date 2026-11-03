package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
)

// makeRoot creates a temporary directory with installer_scripts/ and returns a Root.
func makeRoot(t *testing.T) paths.Root {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "installer_scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	root, err := paths.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func TestCurrentVersion_Missing(t *testing.T) {
	root := makeRoot(t)
	ver, err := CurrentVersion(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ver != "" {
		t.Errorf("expected empty version, got %q", ver)
	}
}

func TestCurrentVersion_Present(t *testing.T) {
	root := makeRoot(t)
	marker := root.MarkerFile(".app_version")
	if err := os.WriteFile(marker, []byte("0.3.7\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ver, err := CurrentVersion(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ver != "0.3.7" {
		t.Errorf("expected %q, got %q", "0.3.7", ver)
	}
}

func TestCurrentVersion_MigrationNote(t *testing.T) {
	root := makeRoot(t)
	marker := root.MarkerFile(".app_version")
	// Migration note format: first line is "migrated ...", second is version.
	if err := os.WriteFile(marker, []byte("migrated from git install\n0.3.5\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ver, err := CurrentVersion(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ver != "0.3.5" {
		t.Errorf("expected %q, got %q", "0.3.5", ver)
	}
}

func TestIsMigratingFromGit_NeitherFile(t *testing.T) {
	root := makeRoot(t)
	if IsMigratingFromGit(root) {
		t.Error("expected false when neither .git_version nor .app_version exist")
	}
}

func TestIsMigratingFromGit_OnlyAppVersion(t *testing.T) {
	root := makeRoot(t)
	if err := os.WriteFile(root.MarkerFile(".app_version"), []byte("0.3.7\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if IsMigratingFromGit(root) {
		t.Error("expected false when .app_version exists (already migrated)")
	}
}

func TestIsMigratingFromGit_OnlyGitVersion(t *testing.T) {
	root := makeRoot(t)
	if err := os.WriteFile(root.MarkerFile(".git_version"), []byte("abc123\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !IsMigratingFromGit(root) {
		t.Error("expected true when .git_version exists but .app_version does not")
	}
}

func TestIsMigratingFromGit_BothFiles(t *testing.T) {
	root := makeRoot(t)
	if err := os.WriteFile(root.MarkerFile(".git_version"), []byte("abc123\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(root.MarkerFile(".app_version"), []byte("0.3.7\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if IsMigratingFromGit(root) {
		t.Error("expected false when both files exist (migration already complete)")
	}
}

func TestLatestVersion_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"info": map[string]any{
				"version": "1.2.3",
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	ver, err := latestVersion(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ver != "1.2.3" {
		t.Errorf("expected %q, got %q", "1.2.3", ver)
	}
}

func TestLatestVersion_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := latestVersion(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected error on HTTP 404")
	}
}

func TestLatestVersion_MissingVersion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Response with empty version field.
		_, _ = w.Write([]byte(`{"info":{"version":""}}`))
	}))
	defer srv.Close()

	_, err := latestVersion(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected error when version is empty")
	}
}
