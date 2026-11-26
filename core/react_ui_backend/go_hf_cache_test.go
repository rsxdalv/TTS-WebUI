package react_ui_backend

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	hfH1 = "1111111111111111111111111111111111111111"
	hfH2 = "2222222222222222222222222222222222222222"
)

// hfFixture writes a cache with one repo of two revisions that share a blob.
func hfFixture(t *testing.T) (cache, repo string) {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cache = filepath.Join(dir, "hub")
	repo = filepath.Join(cache, "models--org--tiny")
	for name, body := range map[string]string{"shared": "0123456789", "only1": "abc"} {
		hfWrite(t, filepath.Join(repo, "blobs", name), body)
	}
	for rev, files := range map[string]map[string]string{hfH1: {"config.json": "shared", "w.bin": "only1"}, hfH2: {"config.json": "shared"}} {
		for f, blob := range files {
			link := filepath.Join(repo, "snapshots", rev, f)
			if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(filepath.Join("..", "..", "blobs", blob), link); err != nil {
				t.Skipf("symlinks are not available here: %v", err)
			}
		}
	}
	hfWrite(t, filepath.Join(repo, "refs", "main"), hfH1)
	hfWrite(t, filepath.Join(repo, "refs", "dev"), hfH2)
	return cache, repo
}

func hfWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func hfServer(t *testing.T, cache string) *httptest.Server {
	t.Helper()
	root := t.TempDir()
	h := newHFCache(root, func(string) []string { return []string{"HF_HUB_CACHE=" + cache} })
	mux := http.NewServeMux()
	for _, c := range []string{"GET /api/go-data/hf-cache/scan", "POST /api/go-data/hf-cache/delete"} {
		mux.Handle(c, h)
	}
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func hfDo(t *testing.T, method, url, contentType, body string) (int, map[string]any) {
	t.Helper()
	req, _ := http.NewRequest(method, url, strings.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	return resp.StatusCode, out
}

func TestHFCacheScanRoute(t *testing.T) {
	cache, _ := hfFixture(t)
	srv := hfServer(t, cache)
	code, got := hfDo(t, "GET", srv.URL+"/api/go-data/hf-cache/scan", "", "")
	if code != 200 || got["exists"] != true || got["cache_dir"] != cache {
		t.Fatalf("scan: %d %v", code, got)
	}
	rows := got["rows"].([]any)
	if len(rows) != 2 || len(got["headers"].([]any)) != 13 {
		t.Fatalf("rows: %v", got)
	}
	first := rows[0].(map[string]any)
	for _, k := range got["headers"].([]any) {
		if _, ok := first[k.(string)]; !ok {
			t.Errorf("row has no %q", k)
		}
	}
	if first["commit_hash"] != hfH1 || first["refs"] != "main" || first["revision_size_on_disk"].(float64) != 13 || first["repo_size_on_disk"].(float64) != 13 {
		t.Errorf("row: %v", first)
	}
}

func TestHFCacheScanOfAMissingCacheIsEmpty(t *testing.T) {
	srv := hfServer(t, filepath.Join(t.TempDir(), "nope"))
	code, got := hfDo(t, "GET", srv.URL+"/api/go-data/hf-cache/scan", "", "")
	if code != 200 || got["exists"] != false || len(got["rows"].([]any)) != 0 || len(got["warnings"].([]any)) != 1 {
		t.Fatalf("missing: %d %v", code, got)
	}
}

func TestHFCacheDeleteRoute(t *testing.T) {
	cache, repo := hfFixture(t)
	srv := hfServer(t, cache)
	url := srv.URL + "/api/go-data/hf-cache/delete"

	if code, _ := hfDo(t, "POST", url, "text/plain", `{"commit_hash":"`+hfH1+`"}`); code != 415 {
		t.Errorf("not json: %d", code)
	}
	if code, _ := hfDo(t, "POST", url, "application/json", `{"commit_hash":"`+hfH1+`","path":"/etc"}`); code != 400 {
		t.Errorf("unknown field: %d", code)
	}
	if code, _ := hfDo(t, "POST", url, "application/json", `{}`); code != 400 {
		t.Errorf("empty: %d", code)
	}
	if code, _ := hfDo(t, "GET", url, "", ""); code != 405 && code != 404 {
		t.Errorf("GET on delete: %d", code)
	}
	for _, bad := range []string{"../../etc", "/etc/passwd", "..", "a/b", `a\b`, "C:\\x", ""} {
		body, _ := json.Marshal(map[string]string{"commit_hash": bad})
		if code, _ := hfDo(t, "POST", url, "application/json", string(body)); code != 400 {
			t.Errorf("hash %q: %d", bad, code)
		}
	}
	if _, err := os.Stat(filepath.Join(repo, "snapshots", hfH1)); err != nil {
		t.Fatal("something was deleted by a refused request")
	}

	// the same body the Gradio endpoint took
	code, got := hfDo(t, "POST", url, "application/json", `{"commit_hash":"`+hfH1+`"}`)
	if code != 200 || got["expected_freed_size"].(float64) != 3 || len(got["deleted"].([]any)) != 1 || len(got["errors"].([]any)) != 0 {
		t.Fatalf("delete: %d %v", code, got)
	}
	for _, gone := range []string{filepath.Join("snapshots", hfH1), filepath.Join("blobs", "only1"), filepath.Join("refs", "main")} {
		if _, err := os.Lstat(filepath.Join(repo, gone)); err == nil {
			t.Errorf("%s is still there", gone)
		}
	}
	if _, err := os.Lstat(filepath.Join(repo, "blobs", "shared")); err != nil {
		t.Error("the shared blob was removed")
	}
	// the list form; an unknown hash is reported, not an error
	code, got = hfDo(t, "POST", url, "application/json", `{"revisions":[{"commit_hash":"`+hfH2+`","repo_id":"org/tiny","repo_type":"model"},{"commit_hash":"`+strings.Repeat("a", 40)+`"}]}`)
	if code != 200 || len(got["not_found"].([]any)) != 1 {
		t.Fatalf("list: %d %v", code, got)
	}
	if _, err := os.Lstat(repo); err == nil {
		t.Error("the last revision did not remove the repo folder")
	}
	if _, err := os.Lstat(cache); err != nil {
		t.Error("the cache folder is gone")
	}
}

func TestHFCacheDeleteNeedsTheRepoForAnAmbiguousHash(t *testing.T) {
	cache, _ := hfFixture(t)
	other := filepath.Join(cache, "models--other--fork")
	hfWrite(t, filepath.Join(other, "blobs", "b"), "x")
	if err := os.MkdirAll(filepath.Join(other, "snapshots", hfH1), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("..", "..", "blobs", "b"), filepath.Join(other, "snapshots", hfH1, "f")); err != nil {
		t.Skip(err)
	}
	srv := hfServer(t, cache)
	url := srv.URL + "/api/go-data/hf-cache/delete"
	if code, got := hfDo(t, "POST", url, "application/json", `{"commit_hash":"`+hfH1+`"}`); code != 409 {
		t.Fatalf("ambiguous: %d %v", code, got)
	}
	if _, err := os.Lstat(filepath.Join(other, "snapshots", hfH1)); err != nil {
		t.Fatal("an ambiguous request deleted something")
	}
	if code, got := hfDo(t, "POST", url, "application/json", `{"commit_hash":"`+hfH1+`","repo_id":"other/fork"}`); code != 200 {
		t.Fatalf("narrowed: %d %v", code, got)
	}
}

// A symlink in the snapshot that points outside the cache: not scanned, not
// followed, never deleted, whatever the request says.
func TestHFCacheDeleteDoesNotTouchWhatALinkPointsAt(t *testing.T) {
	cache, repo := hfFixture(t)
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret.txt")
	hfWrite(t, secret, "do not delete")
	if err := os.Symlink(secret, filepath.Join(repo, "snapshots", hfH1, "evil.txt")); err != nil {
		t.Skip(err)
	}
	srv := hfServer(t, cache)
	if code, got := hfDo(t, "GET", srv.URL+"/api/go-data/hf-cache/scan", "", ""); code != 200 || len(got["warnings"].([]any)) != 1 {
		t.Fatalf("scan: %d %v", code, got)
	}
	if code, got := hfDo(t, "POST", srv.URL+"/api/go-data/hf-cache/delete", "application/json", `{"commit_hash":"`+hfH1+`"}`); code != 200 {
		t.Fatalf("delete: %d %v", code, got)
	}
	if b, err := os.ReadFile(secret); err != nil || string(b) != "do not delete" {
		t.Fatalf("the file behind the link was touched: %v %q", err, b)
	}
}

func TestHFCacheClaimIsRegistered(t *testing.T) {
	found := false
	for _, n := range ClaimNames() {
		found = found || n == "go-hf-cache"
	}
	if !found {
		t.Fatal("go-hf-cache is not a registered claim")
	}
}
