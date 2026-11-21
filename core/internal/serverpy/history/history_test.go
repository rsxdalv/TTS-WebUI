package history

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func gen(t *testing.T, root, coll, name string) {
	t.Helper()
	for ext, body := range map[string]string{".wav": "RIFF", ".png": "PNG", ".json": `{"text":"` + name + `"}`} {
		write(t, root, coll+"/"+name+"/"+name+ext, body)
	}
}

func newStore(t *testing.T) (*Store, string) {
	t.Helper()
	root := t.TempDir()
	return New(Options{Root: root, Token: "tok"}), root
}

func exists(root, rel string) bool {
	_, err := os.Lstat(filepath.Join(root, filepath.FromSlash(rel)))
	return err == nil
}

func TestCollections(t *testing.T) {
	s, root := newStore(t)
	c, err := s.Collections()
	if err != nil || strings.Join(c, ",") != "outputs,favorites" {
		t.Fatalf("empty: %v %v", c, err)
	}
	for _, d := range []string{"collections/zeta", "collections/alpha", "collections/.hidden"} {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(d)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write(t, root, "collections/file.txt", "x")
	c, _ = s.Collections()
	if got := strings.Join(c, ","); got != "outputs,favorites,collections/alpha,collections/zeta" {
		t.Fatalf("got %s", got)
	}
}

func TestItems(t *testing.T) {
	s, root := newStore(t)
	gen(t, root, "outputs", "b")
	gen(t, root, "outputs", "a")
	write(t, root, "outputs/nowav/nowav.png", "PNG")
	write(t, root, "outputs/.hid/x/x.png", "PNG")
	write(t, root, "outputs/.hid/x/x.wav", "RIFF")
	got, err := s.Items("outputs")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, ",") != "outputs/a/a.png,outputs/b/b.png" {
		t.Fatalf("got %v", got)
	}
	if _, err := s.Items("collections/missing"); !errors.Is(err, errNotFound) {
		t.Fatalf("missing collection: %v", err)
	}
	if _, err := s.Items("outputs/a"); !errors.Is(err, errInvalid) {
		t.Fatalf("non-collection: %v", err)
	}
}

func TestItemJSON(t *testing.T) {
	s, root := newStore(t)
	gen(t, root, "outputs", "a")
	for _, p := range []string{"outputs/a/a.wav", `outputs\a\a.png`} {
		b, err := s.ItemJSON(p)
		if err != nil || !strings.Contains(string(b), `"a"`) {
			t.Fatalf("%s: %s %v", p, b, err)
		}
	}
	if _, err := s.ItemJSON("outputs/a/a.txt"); !errors.Is(err, errInvalid) {
		t.Fatalf("ext: %v", err)
	}
	if _, err := s.ItemJSON("outputs/zz/zz.wav"); !errors.Is(err, errNotFound) {
		t.Fatalf("missing: %v", err)
	}
	if _, err := s.ItemJSON("../outside/a.wav"); !errors.Is(err, errInvalid) {
		t.Fatalf("traversal: %v", err)
	}
}

func TestBadPaths(t *testing.T) {
	s, root := newStore(t)
	gen(t, root, "outputs", "a")
	write(t, root, "secret/keep.txt", "x")
	abs := filepath.ToSlash(root) + "/outputs/a"
	bad := []string{
		"", ".", "..", "../secret", "outputs/..", "outputs/../secret", "outputs/a/../../secret",
		`outputs\..\secret`, `..\secret`, "/outputs/a", `\outputs\a`, abs, "C:/Windows", `C:\Windows`,
		"outputs/a:stream", "outputs//a", "outputs/a/", "outputs/./a", "outputs/a.", "outputs/a ",
		"outputs/CON", "outputs/nul.txt", "secret", "secret/keep.txt", "collections", "collections/",
		"outputs", "favorites", "collections/x", "outputs/a\x00", "other/a",
	}
	for _, p := range bad {
		if err := s.Delete(p); err == nil {
			t.Errorf("Delete(%q) succeeded", p)
		}
		if _, err := s.Copy(p, "favorites"); err == nil {
			t.Errorf("Copy(%q) succeeded", p)
		}
	}
	for _, to := range []string{"", "..", "secret", "outputs/a", "../secret", `..\secret`, "collections", "C:/x", "collections/nope"} {
		if _, err := s.Copy("outputs/a", to); err == nil {
			t.Errorf("Copy to %q succeeded", to)
		}
	}
	if !exists(root, "outputs/a/a.wav") || !exists(root, "secret/keep.txt") {
		t.Fatal("something was deleted")
	}
}

func TestCreateCollectionNames(t *testing.T) {
	s, root := newStore(t)
	for _, n := range []string{"", ".", "..", "a/b", `a\b`, "../x", ".hidden", "a.", "a ", "con", "NUL", "a:b", "a*", "a?", `a"b`, "a<b", "a|b", "a\nb", strings.Repeat("x", 201)} {
		if err := s.CreateCollection(n); !errors.Is(err, errInvalid) {
			t.Errorf("CreateCollection(%q) = %v", n, err)
		}
	}
	if err := s.CreateCollection("Mine 1"); err != nil {
		t.Fatal(err)
	}
	if !exists(root, "collections/Mine 1") {
		t.Fatal("not created")
	}
	if err := s.CreateCollection("Mine 1"); !errors.Is(err, errExists) {
		t.Fatalf("duplicate: %v", err)
	}
}

func TestDeleteAndCopy(t *testing.T) {
	s, root := newStore(t)
	gen(t, root, "outputs", "a")
	if err := s.CreateCollection("c"); err != nil {
		t.Fatal(err)
	}
	p, err := s.Copy(`outputs\a`, "favorites")
	if err != nil || p != "favorites/a" || !exists(root, "favorites/a/a.json") {
		t.Fatalf("favorites: %q %v", p, err)
	}
	if _, err := s.Copy("outputs/a", "favorites"); !errors.Is(err, errExists) {
		t.Fatalf("overwrite: %v", err)
	}
	if _, err := s.Copy("outputs/a", "outputs"); !errors.Is(err, errExists) {
		t.Fatalf("into itself: %v", err)
	}
	if p, err = s.Copy("favorites/a", `collections\c`); err != nil || p != "collections/c/a" {
		t.Fatalf("collection: %q %v", p, err)
	}
	if entries, _ := os.ReadDir(filepath.Join(root, "collections", "c")); len(entries) != 1 {
		t.Fatalf("temp dir left behind: %v", entries)
	}
	if err := s.Delete("outputs/a"); err != nil {
		t.Fatal(err)
	}
	if exists(root, "outputs/a") || !exists(root, "favorites/a/a.wav") || !exists(root, "collections/c/a/a.wav") {
		t.Fatal("wrong files deleted")
	}
	if err := s.Delete("outputs/a"); !errors.Is(err, errNotFound) {
		t.Fatalf("delete twice: %v", err)
	}
}

func TestSymlinks(t *testing.T) {
	s, root := newStore(t)
	outside := t.TempDir()
	write(t, outside, "victim/v.wav", "x")
	gen(t, root, "outputs", "a")
	if err := os.Symlink(filepath.Join(outside, "victim"), filepath.Join(root, "outputs", "link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "collections"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "collections", "escape")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := s.Delete("outputs/link"); err == nil {
		t.Error("deleted through a link")
	}
	if _, err := s.Copy("outputs/link", "favorites"); err == nil {
		t.Error("copied through a link")
	}
	if _, err := s.Copy("outputs/a", "collections/escape"); err == nil {
		t.Error("copied into a linked collection")
	}
	if _, err := s.Items("collections/escape"); err == nil {
		t.Error("listed a linked collection")
	}
	if err := s.Delete("collections/escape/victim"); err == nil {
		t.Error("deleted below a linked collection")
	}
	if !exists(outside, "victim/v.wav") {
		t.Fatal("victim was touched")
	}
	// A symlink inside a generation is refused by Copy, not followed.
	if err := os.Symlink(filepath.Join(outside, "victim", "v.wav"), filepath.Join(root, "outputs", "a", "evil.wav")); err != nil {
		t.Skip()
	}
	if _, err := s.Copy("outputs/a", "favorites"); err == nil {
		t.Error("copied a symlink")
	}
	if exists(root, "favorites/a") {
		t.Error("partial copy left behind")
	}
}

func TestOutputsLinkedByUser(t *testing.T) {
	root := t.TempDir()
	disk := t.TempDir()
	gen(t, disk, ".", "a")
	if err := os.Symlink(disk, filepath.Join(root, "outputs")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "favorites"), 0o755); err != nil {
		t.Fatal(err)
	}
	s := New(Options{Root: root, Token: "tok"})
	if it, err := s.Items("outputs"); err != nil || len(it) != 1 {
		t.Fatalf("%v %v", it, err)
	}
	if _, err := s.Copy("outputs/a", "favorites"); err != nil {
		t.Fatal(err)
	}
}

func call(t *testing.T, h http.Handler, method, path, body, tok string) (int, map[string]json.RawMessage) {
	t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.RemoteAddr = "127.0.0.1:5555"
	if body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	if tok != "" {
		r.Header.Set(TokenHeader, tok)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	var m map[string]json.RawMessage
	_ = json.Unmarshal(w.Body.Bytes(), &m)
	return w.Code, m
}

func TestHTTP(t *testing.T) {
	s, root := newStore(t)
	h := s.Handler()
	gen(t, root, "outputs", "a")

	if c, _ := call(t, h, "GET", "/api/history/collections", "", ""); c != 403 {
		t.Errorf("no token: %d", c)
	}
	if c, _ := call(t, h, "GET", "/api/history/collections", "", "wrong"); c != 403 {
		t.Errorf("wrong token: %d", c)
	}
	r := httptest.NewRequest("GET", "/api/history/collections", nil)
	r.RemoteAddr = "10.0.0.5:1"
	r.Header.Set(TokenHeader, "tok")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Errorf("non-loopback: %d", w.Code)
	}
	if c, _ := call(t, h, "GET", "/api/history/nope", "", "tok"); c != 404 {
		t.Errorf("unknown op: %d", c)
	}
	if c, _ := call(t, h, "POST", "/api/history/collections", "{}", "tok"); c != 405 {
		t.Errorf("wrong method: %d", c)
	}
	if c, _ := call(t, h, "GET", "/api/history/delete", "", "tok"); c != 405 {
		t.Errorf("GET delete: %d", c)
	}
	c, m := call(t, h, "GET", "/api/history/items?collection=outputs", "", "tok")
	if c != 200 || string(m["items"]) != "[\"outputs/a/a.png\"]\n" && !strings.Contains(string(m["items"]), "outputs/a/a.png") {
		t.Errorf("items: %d %s", c, m["items"])
	}
	c, m = call(t, h, "GET", "/api/history/item?path=outputs/a/a.wav", "", "tok")
	if c != 200 || !strings.Contains(string(m["json"]), `"text"`) {
		t.Errorf("item: %d %s", c, m["json"])
	}
	if c, _ = call(t, h, "GET", "/api/history/item?path=../x.wav", "", "tok"); c != 400 {
		t.Errorf("item traversal: %d", c)
	}
	c, m = call(t, h, "POST", "/api/history/create-collection", `{"name":"mine"}`, "tok")
	if c != 200 || !strings.Contains(string(m["collections"]), "collections/mine") {
		t.Errorf("create: %d %s", c, m["collections"])
	}
	if c, _ = call(t, h, "POST", "/api/history/create-collection", `{"name":"mine"}`, "tok"); c != 409 {
		t.Errorf("create twice: %d", c)
	}
	if c, _ = call(t, h, "POST", "/api/history/create-collection", `{"name":".."}`, "tok"); c != 400 {
		t.Errorf("create ..: %d", c)
	}
	c, m = call(t, h, "POST", "/api/history/copy", `{"directory":"outputs/a","to":"favorites"}`, "tok")
	if c != 200 || len(m) != 1 ||
		!strings.Contains(string(m["ui"]), `"__gradio__":"Button"`) || !strings.Contains(string(m["ui"]), `"value":"Saved"`) {
		t.Errorf("copy: %d %v", c, m)
	}
	c, m = call(t, h, "POST", "/api/history/copy", `{"directory":"outputs/a","to":"collections/mine"}`, "tok")
	if c != 200 || !strings.Contains(string(m["ui"]), `"__gradio__":"Dropdown"`) {
		t.Errorf("copy to collection: %d %s", c, m["ui"])
	}
	if c, _ = call(t, h, "POST", "/api/history/copy", `{"directory":"outputs/a","to":"favorites"}`, "tok"); c != 409 {
		t.Errorf("copy twice: %d", c)
	}
	if c, _ = call(t, h, "POST", "/api/history/copy", `{"directory":"outputs/a","extra":1}`, "tok"); c != 400 {
		t.Errorf("unknown field: %d", c)
	}
	if c, _ = call(t, h, "POST", "/api/history/delete", `{"directory":"../../etc"}`, "tok"); c != 400 {
		t.Errorf("delete traversal: %d", c)
	}
	if c, _ = call(t, h, "POST", "/api/history/delete", `{not json`, "tok"); c != 400 {
		t.Errorf("bad json: %d", c)
	}
	r = httptest.NewRequest("POST", "/api/history/delete", strings.NewReader(`{"directory":"outputs/a"}`))
	r.RemoteAddr = "127.0.0.1:1"
	r.Header.Set(TokenHeader, "tok")
	r.Header.Set("Content-Type", "text/plain")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 400 || !exists(root, "outputs/a/a.wav") {
		t.Errorf("text/plain delete: %d", w.Code)
	}
	if c, _ = call(t, h, "POST", "/api/history/delete", `{"directory":"outputs/a"}`, "tok"); c != 200 || exists(root, "outputs/a") {
		t.Errorf("delete: %d", c)
	}
	if c, _ = call(t, h, "POST", "/api/history/delete", `{"directory":"outputs/a"}`, "tok"); c != 404 {
		t.Errorf("delete again: %d", c)
	}
}

func TestEmptyTokenRefusesAll(t *testing.T) {
	h := New(Options{Root: t.TempDir()}).Handler()
	if c, _ := call(t, h, "GET", "/api/history/collections", "", ""); c != 403 {
		t.Fatalf("got %d", c)
	}
}
