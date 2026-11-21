package logview

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newStore(t *testing.T) (*Store, string) {
	t.Helper()
	root := t.TempDir()
	s := New(Options{Root: root, Token: "tok"})
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return s, root
}

func put(t *testing.T, s *Store, name, content string, age time.Duration) {
	t.Helper()
	p := filepath.Join(s.dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	at := time.Now().Add(-age)
	if err := os.Chtimes(p, at, at); err != nil {
		t.Fatal(err)
	}
}

func TestValidName(t *testing.T) {
	good := []string{"pip-install-x.log", "a.LOG", "pip-install-git+https___github.com_a_b@main.log", "x (dry run, update check).log", "é.log"}
	bad := []string{
		"", ".", "..", "../x.log", "..\\x.log", "a/b.log", "a\\b.log", "C:\\x.log", "C:x.log", "/etc/passwd.log",
		"x.txt", "x.log.exe", ".hidden.log", "x.log ", "con.log", "NUL.log", "com1.log", "lpt9.log.log", "a|b.log", "a?.log",
		"a*.log", "a\"b.log", "a<b.log", "a\x00b.log", "a\nb.log", "\\\\server\\share\\x.log", strings.Repeat("a", 256) + ".log", "x.log:stream",
	}
	for _, n := range good {
		if !validName(n) {
			t.Errorf("%q rejected", n)
		}
	}
	for _, n := range bad {
		if validName(n) {
			t.Errorf("%q accepted", n)
		}
	}
}

func TestListingAndStats(t *testing.T) {
	s, _ := newStore(t)
	if f, err := s.Files(); err != nil || f == nil || len(f) != 0 {
		t.Fatalf("empty: %v %v", f, err)
	}
	put(t, s, "pip-install-b.log", "Successfully installed b", 3*time.Hour)
	put(t, s, "pip-install-a.log", "x", 1*time.Hour)
	put(t, s, "pip-uninstall-c.log", "Successfully uninstalled c", 2*time.Hour)
	put(t, s, "notes.txt", "not a log", 0)
	os.Mkdir(filepath.Join(s.dir, "dir.log"), 0o755)
	f, _ := s.Files()
	if strings.Join(f, ",") != "pip-install-a.log,pip-uninstall-c.log,pip-install-b.log" {
		t.Fatalf("files: %v", f)
	}
	in, _ := s.PipLogs("install")
	if len(in) != 2 || in[0] != (PipLog{"a", "pip-install-a.log"}) {
		t.Fatalf("install: %v", in)
	}
	un, _ := s.PipLogs("uninstall")
	if len(un) != 1 || un[0].Name != "c" {
		t.Fatalf("uninstall: %v", un)
	}
	if _, err := s.PipLogs("both"); err == nil {
		t.Fatal("bad kind accepted")
	}
	st, _ := s.Stats()
	if !strings.HasPrefix(st, "**Total Log Files:** 3\n**Total Size:** 0.00 MB") {
		t.Fatalf("stats: %q", st)
	}
	// a missing logs directory is an empty list and is not created
	s2 := New(Options{Root: t.TempDir()})
	if f, err := s2.Files(); err != nil || len(f) != 0 {
		t.Fatalf("missing dir: %v %v", f, err)
	}
	if _, err := os.Stat(s2.dir); !os.IsNotExist(err) {
		t.Fatal("logs dir created by a read")
	}
}

func TestInfoAndRead(t *testing.T) {
	s, _ := newStore(t)
	body := "line one\r\nERROR: Boom\nline three\n" + "bad \xff\xfe bytes\n"
	put(t, s, "pip-install-x.log", body, 0)
	info, err := s.Info("pip-install-x.log")
	if err != nil || !strings.HasPrefix(info, "**File:** pip-install-x.log\n**Size:** 0.0") || !strings.Contains(info, "\n**Modified:** 20") {
		t.Fatalf("info: %q %v", info, err)
	}
	if got, _ := s.Info("nope.log"); got != "File not found" {
		t.Fatalf("missing info: %q", got)
	}
	got, err := s.Read("pip-install-x.log", "", 1000)
	if err != nil || got != "line one\nERROR: Boom\nline three\nbad  bytes\n" {
		t.Fatalf("read: %q %v", got, err)
	}
	if got, _ := s.Read("pip-install-x.log", "error", 1000); got != "ERROR: Boom\n" {
		t.Fatalf("search: %q", got)
	}
	if got, _ := s.Read("pip-install-x.log", "zzz", 1000); got != "No matches found for 'zzz'" {
		t.Fatalf("no match: %q", got)
	}
	if got, _ := s.Read("pip-install-x.log", "", 2); got != "[Showing last 2 of 4 lines]\n\nline three\nbad  bytes\n" {
		t.Fatalf("limit: %q", got)
	}
	if got, _ := s.Read("gone.log", "", 10); got != "File not found" {
		t.Fatalf("gone: %q", got)
	}
	put(t, s, "empty.log", "  \n", 0)
	if got, _ := s.Read("empty.log", "", 10); got != "Log file is empty" {
		t.Fatalf("empty: %q", got)
	}
	for _, n := range []int{0, -1, MaxLinesLimit + 1} {
		if _, err := s.Read("empty.log", "", n); err == nil {
			t.Errorf("max_lines %d accepted", n)
		}
	}
}

func TestAnalyze(t *testing.T) {
	s, _ := newStore(t)
	put(t, s, "pip-install-ok.log", "Collecting x\nSuccessfully installed x-1\nWARNING: old\nWARNING: old\n", 0)
	put(t, s, "pip-install-bad.log", "ERROR: no match\nSuccessfully installed y\n", time.Minute)
	put(t, s, "pip-install-unknown.log", "nothing\n", 2*time.Minute)
	ok, _ := s.Analyze("pip-install-ok.log")
	for _, want := range []string{"## ✅ Status: SUCCESS", "**Total Lines:** 5", "**Errors Found:** 0", "**Warnings Found:** 1", "**Success Messages:** 1", "- Successfully installed x-1", "- WARNING: old"} {
		if !strings.Contains(ok, want) {
			t.Errorf("ok analysis lacks %q:\n%s", want, ok)
		}
	}
	bad, _ := s.Analyze("pip-install-bad.log")
	if !strings.Contains(bad, "## ❌ Status: FAILED") || !strings.Contains(bad, "### ❌ Errors Found:\n- ERROR: no match") {
		t.Errorf("bad analysis:\n%s", bad)
	}
	if u, _ := s.Analyze("pip-install-unknown.log"); !strings.Contains(u, "## ❓ Status: UNKNOWN") {
		t.Errorf("unknown analysis:\n%s", u)
	}
	if g, _ := s.Analyze("none.log"); g != "File not found" {
		t.Errorf("missing: %q", g)
	}
	sum, _ := s.Summary("install")
	if !strings.Contains(sum, "**Installation Logs:** 3\n**Uninstallation Logs:** 0\n") ||
		!strings.Contains(sum, "\n✅ **ok** - 0.1KB - 20") || !strings.Contains(sum, "\n✅ **bad** - ") || !strings.Contains(sum, "\n❓ **unknown** - ") {
		t.Errorf("summary:\n%s", sum)
	}
	if c, _ := s.Summary("uninstall"); c != "**Total Uninstall Logs:** 0" {
		t.Errorf("uninstall summary: %q", c)
	}
}

func TestDelete(t *testing.T) {
	s, root := newStore(t)
	put(t, s, "a.log", "a", time.Hour)
	put(t, s, "b.log", "b", 0)
	outside := filepath.Join(root, "installer_scripts", "keep.log")
	os.WriteFile(outside, []byte("keep"), 0o644)
	os.Mkdir(filepath.Join(s.dir, "dir.log"), 0o755)

	res, err := s.Delete("a.log")
	if err != nil || res.Status != "Successfully deleted a.log" || len(res.Files) != 1 || res.Files[0] != "b.log" {
		t.Fatalf("delete: %+v %v", res, err)
	}
	if res, _ := s.Delete("a.log"); res.Status != "File not found" {
		t.Fatalf("again: %+v", res)
	}
	for _, n := range []string{"..\\keep.log", "../keep.log", "../../keep.log", "x/../../keep.log", "dir.log/x.log", "notes.txt", "", "C:\\Windows\\x.log"} {
		if _, err := s.Delete(n); err == nil {
			t.Errorf("delete %q accepted", n)
		}
	}
	if _, err := s.Delete("dir.log"); err == nil {
		t.Error("deleted a directory")
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("file outside logs deleted: %v", err)
	}
	res, err = s.DeleteAll()
	if err != nil || res.Status != "Deleted 1 log file(s)" || len(res.Files) != 0 {
		t.Fatalf("delete all: %+v %v", res, err)
	}
	if _, err := os.Stat(filepath.Join(s.dir, "dir.log")); err != nil {
		t.Fatalf("directory removed by delete-all: %v", err)
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatal("delete-all touched a file outside the logs dir")
	}
}

func TestSymlinksAreNeverFollowed(t *testing.T) {
	s, root := newStore(t)
	secret := filepath.Join(root, "secret.txt")
	os.WriteFile(secret, []byte("TOP SECRET\n"), 0o644)
	link := filepath.Join(s.dir, "evil.log")
	if err := os.Symlink(secret, link); err != nil {
		t.Skipf("cannot create symlinks here: %v", err)
	}
	if f, _ := s.Files(); len(f) != 0 {
		t.Fatalf("symlink listed: %v", f)
	}
	if _, err := s.Read("evil.log", "", 10); err == nil {
		t.Error("symlink read")
	}
	if _, err := s.Analyze("evil.log"); err == nil {
		t.Error("symlink analysed")
	}
	if _, err := s.Info("evil.log"); err == nil {
		t.Error("symlink info")
	}
	if _, err := s.Delete("evil.log"); err == nil {
		t.Error("symlink deleted")
	}
	if res, _ := s.DeleteAll(); res.Status != "Deleted 0 log file(s)" {
		t.Errorf("delete-all: %+v", res)
	}
	if _, err := os.Lstat(link); err != nil {
		t.Error("symlink removed")
	}
	if b, _ := os.ReadFile(secret); string(b) != "TOP SECRET\n" {
		t.Error("target changed")
	}
}

func TestLargeFileReadsTail(t *testing.T) {
	s, _ := newStore(t)
	var b bytes.Buffer
	b.WriteString("HEAD-MARKER\n")
	b.Write(bytes.Repeat([]byte("x\n"), maxRead/2))
	b.WriteString("TAIL-MARKER\n")
	put(t, s, "big.log", b.String(), 0)
	got, err := s.Read("big.log", "", 3)
	if err != nil || !strings.HasPrefix(got, "[Only the last 32 MiB of the file were read]\n[Showing last 3 of ") || !strings.HasSuffix(got, "x\nTAIL-MARKER\n") || strings.Contains(got, "HEAD-MARKER") {
		t.Fatalf("big: %q", got[:min(len(got), 200)])
	}
}

func TestHTTP(t *testing.T) {
	s, root := newStore(t)
	put(t, s, "pip-install-x.log", "ERROR: bad\nok\n", 0)
	os.WriteFile(filepath.Join(root, "outside.log"), []byte("OUTSIDE"), 0o644)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	do := func(method, op string, q url.Values, body, token, ctype string) (int, string) {
		u := srv.URL + Prefix + op
		if q != nil {
			u += "?" + q.Encode()
		}
		req, _ := http.NewRequest(method, u, strings.NewReader(body))
		if token != "" {
			req.Header.Set(TokenHeader, token)
		}
		if ctype != "" {
			req.Header.Set("Content-Type", ctype)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(b)
	}
	const js = "application/json"
	if c, _ := do("GET", "files", nil, "", "", ""); c != 403 {
		t.Fatalf("no token: %d", c)
	}
	if c, _ := do("GET", "files", nil, "", "bad", ""); c != 403 {
		t.Fatalf("bad token: %d", c)
	}
	if c, _ := do("GET", "nope", nil, "", "tok", ""); c != 404 {
		t.Fatalf("unknown: %d", c)
	}
	if c, _ := do("POST", "files", nil, "", "tok", js); c != 405 {
		t.Fatalf("wrong method: %d", c)
	}
	if c, b := do("GET", "files", nil, "", "tok", ""); c != 200 || strings.TrimSpace(b) != `["pip-install-x.log"]` {
		t.Fatalf("files: %d %s", c, b)
	}
	rd := func(file, search, n string) url.Values {
		return url.Values{"file": {file}, "search": {search}, "max_lines": {n}}
	}
	if c, b := do("GET", "read", rd("pip-install-x.log", "bad", "10"), "", "tok", ""); c != 200 || strings.TrimSpace(b) != `"ERROR: bad\n"` {
		t.Fatalf("read: %d %s", c, b)
	}
	for _, f := range []string{"../outside.log", "..\\outside.log", "..%2foutside.log", "/etc/hosts", "x.txt", ""} {
		if c, b := do("GET", "read", rd(f, "", "10"), "", "tok", ""); c != 400 || strings.Contains(b, "OUTSIDE") {
			t.Fatalf("read %q: %d %s", f, c, b)
		}
	}
	if c, _ := do("GET", "read", rd("pip-install-x.log", "", "abc"), "", "tok", ""); c != 400 {
		t.Fatalf("bad max_lines: %d", c)
	}
	if c, _ := do("GET", "read", url.Values{"file": {"pip-install-x.log"}, "max_lines": {"5"}, "extra": {"1"}}, "", "tok", ""); c != 400 {
		t.Fatalf("unknown param: %d", c)
	}
	if c, _ := do("GET", "pip-logs", url.Values{"kind": {"x"}}, "", "tok", ""); c != 400 {
		t.Fatalf("kind: %d", c)
	}
	if c, _ := do("POST", "delete", nil, `{"file":"../outside.log"}`, "tok", js); c != 400 {
		t.Fatalf("delete traversal: %d", c)
	}
	if c, _ := do("POST", "delete", nil, `{"file":"a.log","x":1}`, "tok", js); c != 400 {
		t.Fatalf("delete extra: %d", c)
	}
	if c, _ := do("POST", "delete", nil, `{"file":"pip-install-x.log"}`, "tok", "text/plain"); c != 400 {
		t.Fatalf("ctype: %d", c)
	}
	if c, _ := do("POST", "delete-all", nil, `{"x":1}`, "tok", js); c != 400 {
		t.Fatalf("delete-all fields: %d", c)
	}
	c, b := do("POST", "delete", nil, `{"file":"pip-install-x.log"}`, "tok", js)
	var res DeleteResult
	if c != 200 || json.Unmarshal([]byte(b), &res) != nil || res.Status != "Successfully deleted pip-install-x.log" || len(res.Files) != 0 {
		t.Fatalf("delete: %d %s", c, b)
	}
	if _, err := os.Stat(filepath.Join(root, "outside.log")); err != nil {
		t.Fatal("outside file gone")
	}
}
