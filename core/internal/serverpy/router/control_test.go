package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func ctl(t *testing.T, h http.Handler, method, target, body string, mod func(*http.Request)) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.RemoteAddr = "127.0.0.1:50000"
	req.Header.Set(TokenHeader, "tok")
	if method == "POST" {
		req.Header.Set("Content-Type", "application/json")
	}
	if mod != nil {
		mod(req)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestControlAuth(t *testing.T) {
	rt := New(Options{Token: "tok", Logger: quiet()})
	h := rt.ControlHandler()
	body := `{"prefix":"/pkg","port":27771}`

	cases := []struct {
		name string
		mod  func(*http.Request)
		want int
	}{
		{"ok", nil, 200},
		{"ok with charset", func(r *http.Request) { r.Header.Set("Content-Type", "application/json; charset=utf-8") }, 200},
		{"no token", func(r *http.Request) { r.Header.Del(TokenHeader) }, 403},
		{"wrong token", func(r *http.Request) { r.Header.Set(TokenHeader, "nope") }, 403},
		{"token prefix", func(r *http.Request) { r.Header.Set(TokenHeader, "to") }, 403},
		{"remote host", func(r *http.Request) { r.RemoteAddr = "192.0.2.7:4000" }, 403},
		{"remote, right token, bad type", func(r *http.Request) { r.RemoteAddr = "192.0.2.7:4000"; r.Header.Set("Content-Type", "text/plain") }, 403},
		{"ipv6 loopback", func(r *http.Request) { r.RemoteAddr = "[::1]:4000" }, 200},
		{"mapped loopback", func(r *http.Request) { r.RemoteAddr = "[::ffff:127.0.0.1]:4000" }, 200},
		{"garbage remote", func(r *http.Request) { r.RemoteAddr = "pipe" }, 403},
		{"text/plain", func(r *http.Request) { r.Header.Set("Content-Type", "text/plain") }, 415},
		{"no content type", func(r *http.Request) { r.Header.Del("Content-Type") }, 415},
	}
	for _, c := range cases {
		if got := ctl(t, h, "POST", "/api/routes", body, c.mod).Code; got != c.want {
			t.Errorf("%s: %d, want %d", c.name, got, c.want)
		}
	}
	// GET and DELETE are authenticated too and need no content type.
	if got := ctl(t, h, "GET", "/api/routes", "", func(r *http.Request) { r.Header.Del(TokenHeader) }).Code; got != 403 {
		t.Errorf("GET without token: %d", got)
	}
	if got := ctl(t, h, "DELETE", "/api/routes?prefix=/pkg", "", func(r *http.Request) { r.RemoteAddr = "192.0.2.7:1" }).Code; got != 403 {
		t.Errorf("DELETE from remote: %d", got)
	}
}

func TestControlWithoutTokenRefusesEverything(t *testing.T) {
	h := New(Options{Logger: quiet()}).ControlHandler()
	for _, tok := range []string{"", "anything"} {
		got := ctl(t, h, "GET", "/api/routes", "", func(r *http.Request) { r.Header.Set(TokenHeader, tok) }).Code
		if got != 403 {
			t.Errorf("token %q accepted with empty configured token: %d", tok, got)
		}
	}
}

func TestControlRoutesCRUD(t *testing.T) {
	rt := New(Options{Token: "tok", Logger: quiet()})
	h := rt.ControlHandler()

	rec := ctl(t, h, "POST", "/api/routes", `{"prefix":"pkg/","port":27771}`, nil)
	if rec.Code != 200 || rt.Routes()["/pkg"] != "http://127.0.0.1:27771" {
		t.Fatalf("add: %d %s %v", rec.Code, rec.Body, rt.Routes())
	}
	ctl(t, h, "POST", "/api/routes", `{"prefix":"/pkg","port":27772}`, nil) // idempotent replace
	rec = ctl(t, h, "GET", "/api/routes", "", nil)
	if rec.Code != 200 || strings.TrimSpace(rec.Body.String()) != `{"/pkg":"http://127.0.0.1:27772"}` {
		t.Fatalf("get: %d %s", rec.Code, rec.Body)
	}
	rec = ctl(t, h, "DELETE", "/api/routes?prefix=/pkg", "", nil)
	if rec.Code != 200 || len(rt.Routes()) != 0 || !strings.Contains(rec.Body.String(), `"removed":true`) {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body)
	}
	if rec = ctl(t, h, "DELETE", "/api/routes?prefix=/pkg", "", nil); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"removed":false`) {
		t.Fatalf("second delete: %d %s", rec.Code, rec.Body)
	}

	_ = rt.AddRoute("/", "http://127.0.0.1:9")
	bad := []struct{ method, target, body string }{
		{"POST", "/api/routes", `{"prefix":"/","port":5}`},
		{"POST", "/api/routes", `{"prefix":"","port":5}`},
		{"POST", "/api/routes", `{"prefix":"/a","port":0}`},
		{"POST", "/api/routes", `{"prefix":"/a","port":70000}`},
		{"POST", "/api/routes", `{"prefix":"/a"}`},
		{"POST", "/api/routes", `{"prefix":"/a","port":"5"}`},
		{"POST", "/api/routes", `{"prefix":"/a b","port":5}`},
		{"POST", "/api/routes", `not json`},
		{"DELETE", "/api/routes?prefix=/", ""},
		{"DELETE", "/api/routes", ""},
	}
	for _, b := range bad {
		if got := ctl(t, h, b.method, b.target, b.body, nil).Code; got != 400 {
			t.Errorf("%s %s: %d, want 400", b.method, b.body, got)
		}
	}
	if rt.Routes()["/"] != "http://127.0.0.1:9" {
		t.Error("catch-all was changed through the control API")
	}
	if got := ctl(t, h, "PUT", "/api/routes", "{}", nil).Code; got != 405 {
		t.Errorf("PUT: %d", got)
	}
	if got := ctl(t, h, "GET", "/api/other", "", nil).Code; got != 404 {
		t.Errorf("other path: %d", got)
	}
	if got := ctl(t, h, "POST", "/api/routes", `{"prefix":"/a","port":5,"x":"`+strings.Repeat("a", 20000)+`"}`, nil).Code; got != 413 {
		t.Errorf("oversized: %d", got)
	}
}
