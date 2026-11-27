package launcher

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
)

func storageLauncher(t *testing.T) (*Launcher, string) {
	t.Helper()
	dir := t.TempDir()
	root, err := paths.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	return New(root), dir
}

func writeFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func callStorage(h http.Handler, method string) (int, map[string]any) {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, "/x", nil))
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	return rec.Code, body
}

func TestStorageStateOfAFreshFolder(t *testing.T) {
	l, _ := storageLauncher(t)
	code, body := callStorage(l.handleStorageState(), http.MethodGet)
	if code != 200 {
		t.Fatalf("code %d", code)
	}
	uvRow := body["uv"].(map[string]any)
	cache := body["cache"].(map[string]any)
	if uvRow["exists"] != false || cache["exists"] != false || len(body["venvs"].([]any)) != 0 {
		t.Fatalf("fresh folder: %v", body)
	}
}

func TestStorageStateCountsFilesAndVenvs(t *testing.T) {
	l, dir := storageLauncher(t)
	writeFile(t, filepath.Join(dir, "platforms", "uv", "bin", "uv.exe"), "12345")
	writeFile(t, filepath.Join(dir, "cache", "archive-v0", "w"), "1234567890")
	writeFile(t, filepath.Join(dir, ".venvs", "gradio-host", "pyvenv.cfg"), "x")
	_, body := callStorage(l.handleStorageState(), http.MethodGet)
	if body["uv"].(map[string]any)["size_bytes"] != float64(5) ||
		body["cache"].(map[string]any)["size_bytes"] != float64(10) {
		t.Fatalf("sizes: %v", body)
	}
	if v := body["venvs"].([]any); len(v) != 1 || v[0] != "gradio-host" {
		t.Fatalf("venvs: %v", v)
	}
}

func TestUninstallUVIsRefusedWhileAVenvExists(t *testing.T) {
	l, dir := storageLauncher(t)
	writeFile(t, filepath.Join(dir, "platforms", "uv", "bin", "uv.exe"), "x")
	writeFile(t, filepath.Join(dir, ".venvs", "kokoro", "pyvenv.cfg"), "x")
	code, body := callStorage(l.handleStorageAction("uv"), http.MethodPost)
	if code != http.StatusConflict {
		t.Fatalf("code %d body %v", code, body)
	}
	if _, err := os.Stat(filepath.Join(dir, "platforms", "uv")); err != nil {
		t.Fatalf("the platform was removed anyway: %v", err)
	}
}

func TestUninstallUVRemovesThePlatformAndKeepsTheCache(t *testing.T) {
	l, dir := storageLauncher(t)
	ro := filepath.Join(dir, "platforms", "uv", "python", "Lib", "ro.txt")
	writeFile(t, ro, "x")
	_ = os.Chmod(ro, 0o444) // a read-only file, as Python's tree has
	writeFile(t, filepath.Join(dir, "cache", "archive-v0", "w"), "x")
	code, body := callStorage(l.handleStorageAction("uv"), http.MethodPost)
	if code != 200 || body["ok"] != true {
		t.Fatalf("code %d body %v", code, body)
	}
	if _, err := os.Stat(filepath.Join(dir, "platforms", "uv")); !os.IsNotExist(err) {
		t.Fatalf("platforms/uv is still there: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "cache", "archive-v0", "w")); err != nil {
		t.Fatalf("the cache went too: %v", err)
	}
}

func TestClearCacheRemovesOnlyTheCache(t *testing.T) {
	l, dir := storageLauncher(t)
	writeFile(t, filepath.Join(dir, "cache", "archive-v0", "w"), "x")
	writeFile(t, filepath.Join(dir, "platforms", "uv", "bin", "uv.exe"), "x")
	writeFile(t, filepath.Join(dir, ".venvs", "kokoro", "pyvenv.cfg"), "x")
	code, body := callStorage(l.handleStorageAction("cache"), http.MethodPost)
	if code != 200 || body["ok"] != true {
		t.Fatalf("code %d body %v", code, body)
	}
	if _, err := os.Stat(filepath.Join(dir, "cache")); !os.IsNotExist(err) {
		t.Fatalf("cache is still there: %v", err)
	}
	for _, keep := range []string{"platforms/uv/bin/uv.exe", ".venvs/kokoro/pyvenv.cfg"} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(keep))); err != nil {
			t.Fatalf("%s went too: %v", keep, err)
		}
	}
}

func TestStorageActionsAreRefusedDuringAnInstall(t *testing.T) {
	l, dir := storageLauncher(t)
	writeFile(t, filepath.Join(dir, "cache", "archive-v0", "w"), "x")
	h, err := l.extensions()
	if err != nil {
		t.Fatal(err)
	}
	var code int
	_ = h.cat.WhileNoInstall(func() error { // an install holds the lock
		code, _ = callStorage(l.handleStorageAction("cache"), http.MethodPost)
		return nil
	})
	if code != http.StatusConflict {
		t.Fatalf("during an install: code %d, want 409", code)
	}
	if _, err := os.Stat(filepath.Join(dir, "cache")); err != nil {
		t.Fatalf("the cache was cleared under an install: %v", err)
	}
}
