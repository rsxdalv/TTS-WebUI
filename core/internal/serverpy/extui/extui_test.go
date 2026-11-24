package extui

import (
	"context"
	"encoding/json"
	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform/legacyenv"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/appconfig"
	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/extcatalog"
	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/piplist"
)

const host = "127.0.0.1:7770"

type fakeCfg struct{ lists appconfig.ExtensionLists }

func (f fakeCfg) ExtensionLists() (appconfig.ExtensionLists, error) { return f.lists, nil }

type fakePip struct{ pkgs []piplist.Package }

func (f fakePip) Cached(context.Context) ([]piplist.Package, error) { return f.pkgs, nil }
func (f fakePip) Invalidate()                                       {}

func ext(pkg, name, extra string) string {
	return `{"package_name":"` + pkg + `","name":"` + name + `","requirements":"git+https://example.com/` + pkg +
		`@main","description":"about ` + name + `","extension_type":"interface","extension_class":"tools",` +
		`"website":"https://example.com/` + pkg + `"` + extra + `}`
}

func newHandler(t *testing.T, exposed bool) *Handler {
	t.Helper()
	root := t.TempDir()
	// Not a command: an install that reaches extcatalog runs it, fails, and
	// still answers with a "done" line, which is all these tests need.
	pr, err := paths.New(root)
	if err != nil {
		t.Fatal(err)
	}
	cat := extcatalog.New(extcatalog.Options{
		Root: root, Token: "cat-token", Platform: legacyenv.New(legacyenv.Options{Root: pr, Prefix: []string{"no-such-command-for-extui-tests"}}),
		Config: fakeCfg{}, Pip: fakePip{pkgs: []piplist.Package{{Name: "ext.one", Version: "1.0"}}},
	})
	body := `{"tabs":[` + ext("ext.one", "One", "") + `,` + ext("ext.two", "Two", "") + `,` +
		ext("extensions.builtin.base", "Base", "") + `]}`
	if err := os.WriteFile(filepath.Join(root, "extensions.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	h, err := New(Options{
		Cat: cat, CatToken: "cat-token", Port: 7770, Exposed: exposed,
		Cfg: fakeCfg{lists: appconfig.ExtensionLists{Disabled: []string{"ext.two"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return h
}

type req struct {
	method, path, host string
	header             map[string]string
	cookie             *http.Cookie
	body               string
}

func do(h http.Handler, q req) *httptest.ResponseRecorder {
	r := httptest.NewRequest(q.method, q.path, strings.NewReader(q.body))
	r.Host = q.host
	if q.host == "" {
		r.Host = host
	}
	for k, v := range q.header {
		r.Header.Set(k, v)
	}
	if q.cookie != nil {
		r.AddCookie(q.cookie)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

// session loads the page like the iframe does and returns what it handed out.
func session(t *testing.T, h http.Handler, kind string) (cookie *http.Cookie, token string) {
	t.Helper()
	w := do(h, req{method: "GET", path: "/ext/list?kind=" + kind})
	if w.Code != 200 {
		t.Fatalf("page: %d %s", w.Code, w.Body)
	}
	cs := w.Result().Cookies()
	if len(cs) != 1 {
		t.Fatalf("cookies: %v", cs)
	}
	m := regexp.MustCompile(`name="tts-ui-token" content="([0-9a-f]+)"`).FindStringSubmatch(w.Body.String())
	if m == nil {
		t.Fatalf("no token in the page:\n%s", w.Body)
	}
	return cs[0], m[1]
}

func post(cookie *http.Cookie, token, path, body string, more map[string]string) req {
	hd := map[string]string{"Content-Type": "application/json", "Origin": "http://" + host, "X-TTS-UI": token}
	for k, v := range more {
		if v == "" {
			delete(hd, k)
		} else {
			hd[k] = v
		}
	}
	return req{method: "POST", path: path, header: hd, cookie: cookie, body: body}
}

func TestHostRule(t *testing.T) {
	h := newHandler(t, false)
	for _, c := range []struct {
		host string
		code int
	}{
		{"127.0.0.1:7770", 200}, {"localhost:7770", 200},
		{"evil.example:7770", 400}, {"127.0.0.1:3000", 400}, {"localhost", 400}, {"192.168.1.5:7770", 400},
	} {
		if w := do(h, req{method: "GET", path: "/ext/list?kind=interface", host: c.host}); w.Code != c.code {
			t.Errorf("Host %q: %d, want %d", c.host, w.Code, c.code)
		}
	}
	// open to other devices: any host, but read-only (see TestExposedIsReadOnly)
	if w := do(newHandler(t, true), req{method: "GET", path: "/ext/list?kind=interface", host: "192.168.1.5:7770"}); w.Code != 200 {
		t.Errorf("exposed: %d", w.Code)
	}
}

func TestPageSetsScopedCookieAndFrameRules(t *testing.T) {
	h := newHandler(t, false)
	w := do(h, req{method: "GET", path: "/ext/list?kind=decorator"})
	c := w.Result().Cookies()[0]
	if !c.HttpOnly || c.SameSite != http.SameSiteStrictMode || c.Path != "/_core/" || c.Name != "tts-ui-7770" || len(c.Value) < 32 {
		t.Errorf("cookie: %+v", c)
	}
	hd := w.Header()
	if csp := hd.Get("Content-Security-Policy"); !strings.Contains(csp, "frame-ancestors 'self'") ||
		!strings.Contains(csp, "script-src 'self'") || strings.Contains(csp, "unsafe") {
		t.Errorf("CSP: %q", csp)
	}
	for k, want := range map[string]string{"X-Frame-Options": "SAMEORIGIN", "X-Content-Type-Options": "nosniff", "Cache-Control": "no-store"} {
		if hd.Get(k) != want {
			t.Errorf("%s = %q", k, hd.Get(k))
		}
	}
	if !strings.Contains(w.Body.String(), `data-kind="decorator"`) {
		t.Errorf("kind missing:\n%s", w.Body)
	}
	for _, bad := range []string{"", "other", `interface"><script>`} {
		if w := do(h, req{method: "GET", path: "/ext/list?kind=" + bad}); w.Code != 400 {
			t.Errorf("kind %q: %d", bad, w.Code)
		}
	}
	// the cookie is not the token, and both are new for every handler
	cookie, token := session(t, h, "interface")
	if cookie.Value == token {
		t.Error("the cookie and the page token must differ")
	}
	cookie2, token2 := session(t, newHandler(t, false), "interface")
	if cookie.Value == cookie2.Value || token == token2 {
		t.Error("secrets must not repeat")
	}
	if w := do(h, req{method: "GET", path: "/ext/api/list?kind=interface", cookie: cookie2}); w.Code != 403 {
		t.Errorf("another run's cookie: %d", w.Code)
	}
}

func TestStaticAssetsServe(t *testing.T) {
	h := newHandler(t, false)
	for path, ct := range map[string]string{"/ext/ui.js": "javascript", "/ext/ui.css": "text/css"} {
		w := do(h, req{method: "GET", path: path})
		if w.Code != 200 || !strings.Contains(w.Header().Get("Content-Type"), ct) {
			t.Errorf("%s: %d %q", path, w.Code, w.Header().Get("Content-Type"))
		}
	}
	if w := do(h, req{method: "GET", path: "/ext/list.html"}); w.Code == 200 {
		t.Error("the page template is not an asset")
	}
}

func TestListNeedsTheCookieAndAnswers(t *testing.T) {
	h := newHandler(t, false)
	if w := do(h, req{method: "GET", path: "/ext/api/list?kind=interface"}); w.Code != 403 {
		t.Fatalf("no cookie: %d", w.Code)
	}
	cookie, _ := session(t, h, "interface")
	w := do(h, req{method: "GET", path: "/ext/api/list?kind=interface", cookie: cookie})
	if w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	var got List
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	by := map[string]Entry{}
	for _, e := range got.Entries {
		by[e.PackageName] = e
	}
	if e := by["ext.one"]; !e.Installed || e.Version != "1.0" || e.Disabled || e.Website == "" {
		t.Errorf("ext.one: %+v", e)
	}
	if e := by["ext.two"]; e.Installed || !e.Disabled {
		t.Errorf("ext.two: %+v", e)
	}
	if e := by["extensions.builtin.base"]; !e.Installed || !e.Builtin {
		t.Errorf("builtin: %+v", e)
	}
	if got.ReadOnly {
		t.Error("a loopback app is not read-only")
	}
	if len(got.Uninstallable) != 1 || got.Uninstallable[0] != "ext.one" {
		t.Errorf("uninstallable: %v (choices %v)", got.Uninstallable, got.Choices)
	}
	if w := do(h, req{method: "GET", path: "/ext/api/list?kind=bogus", cookie: cookie}); w.Code != 400 {
		t.Errorf("bad kind: %d", w.Code)
	}
}

func TestMutatingRules(t *testing.T) {
	h := newHandler(t, false)
	cookie, token := session(t, h, "interface")
	body := `{"package_name":"ext.one"}`
	for _, c := range []struct {
		name string
		q    req
		code int
	}{
		{"no cookie", post(nil, token, "/ext/api/op/install", body, nil), 403},
		{"no token", post(cookie, "", "/ext/api/op/install", body, nil), 403},
		{"wrong token", post(cookie, "0000", "/ext/api/op/install", body, nil), 403},
		{"the cookie value is not the token", post(cookie, cookie.Value, "/ext/api/op/install", body, nil), 403},
		{"no origin", post(cookie, token, "/ext/api/op/install", body, map[string]string{"Origin": ""}), 403},
		{"foreign origin", post(cookie, token, "/ext/api/op/install", body, map[string]string{"Origin": "http://evil.example"}), 403},
		{"null origin", post(cookie, token, "/ext/api/op/install", body, map[string]string{"Origin": "null"}), 403},
		{"cross-site fetch", post(cookie, token, "/ext/api/op/install", body, map[string]string{"Sec-Fetch-Site": "cross-site"}), 403},
		{"not json", post(cookie, token, "/ext/api/op/install", body, map[string]string{"Content-Type": "text/plain"}), 415},
		{"install-entry takes pasted requirements", post(cookie, token, "/ext/api/op/install-entry", `{"entry":{}}`, nil), 404},
		{"parse", post(cookie, token, "/ext/api/op/parse", `{"text":"x"}`, nil), 404},
		{"add", post(cookie, token, "/ext/api/op/add", `{"entries":[]}`, nil), 404},
		{"check-dependencies is not offered", post(cookie, token, "/ext/api/op/check-dependencies", body, nil), 404},
		{"unknown package", post(cookie, token, "/ext/api/op/install", `{"package_name":"nope"}`, nil), 404},
		{"a requirements string is not a field", post(cookie, token, "/ext/api/op/install", `{"package_name":"ext.one","requirements":"-e ."}`, nil), 400},
	} {
		if w := do(h, c.q); w.Code != c.code {
			t.Errorf("%s: %d, want %d (%s)", c.name, w.Code, c.code, strings.TrimSpace(w.Body.String()))
		}
	}
	// the same origin as the page, through the other name for loopback, works
	q := post(cookie, token, "/ext/api/op/install", body, map[string]string{"Origin": "http://localhost:7770"})
	q.host = "localhost:7770"
	if w := do(h, q); w.Code != 200 {
		t.Errorf("localhost: %d %s", w.Code, w.Body)
	}
}

func TestAllowedOperationsReachExtcatalog(t *testing.T) {
	h := newHandler(t, false)
	cookie, token := session(t, h, "interface")
	for _, op := range []string{"install", "uninstall"} {
		w := do(h, post(cookie, token, "/ext/api/op/"+op, `{"package_name":"ext.one"}`, nil))
		if w.Code != 200 || w.Header().Get("Content-Type") != "application/x-ndjson" {
			t.Fatalf("%s: %d %q %s", op, w.Code, w.Header().Get("Content-Type"), w.Body)
		}
		if !strings.Contains(w.Body.String(), `"done":true`) {
			t.Errorf("%s: no done line:\n%s", op, w.Body)
		}
	}
}

func TestExposedIsReadOnly(t *testing.T) {
	h := newHandler(t, true)
	cookie, token := session(t, h, "interface")
	w := do(h, req{method: "GET", path: "/ext/api/list?kind=interface", cookie: cookie})
	var got List
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &got) != nil || !got.ReadOnly {
		t.Fatalf("list: %d %s", w.Code, w.Body)
	}
	if w := do(h, post(cookie, token, "/ext/api/op/install", `{"package_name":"ext.one"}`, nil)); w.Code != 403 || !strings.Contains(w.Body.String(), "read-only") {
		t.Errorf("install: %d %s", w.Code, w.Body)
	}
}
