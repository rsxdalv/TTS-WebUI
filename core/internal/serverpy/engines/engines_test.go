package engines

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/extcatalog"
	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/extprocs"
)

type fakeCat map[string]error // pkg -> error; absent = unknown engine

func (c fakeCat) Engines() ([]extcatalog.InstalledEngine, error) {
	var out []extcatalog.InstalledEngine
	for p, err := range c {
		if err == nil {
			out = append(out, extcatalog.InstalledEngine{Package: p})
		}
	}
	return out, nil
}

func (c fakeCat) Engine(pkg string) (string, error) {
	err, ok := c[pkg]
	if !ok {
		return "", fmt.Errorf("%w: %q", extcatalog.ErrNotEngine, pkg)
	}
	return "Name of " + pkg, err
}

type fakeProcs struct {
	started []string
	state   extprocs.State
}

func (p *fakeProcs) Start(_ context.Context, pkg string, _ extprocs.StartRequest) (extprocs.StartResult, error) {
	p.started = append(p.started, pkg)
	p.state = extprocs.StateStarting
	return extprocs.StartResult{Port: 9999, Pid: 1}, nil
}

func (p *fakeProcs) List() []extprocs.Status {
	if p.state == "" {
		return nil
	}
	return []extprocs.Status{{Package: "tts_webui_extension.piper_tts", State: p.state}}
}

func do(h http.Handler, method, path, token string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(""))
	r.RemoteAddr = "127.0.0.1:5555"
	if token != "" {
		r.Header.Set(tokenHeader, token)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

const piper = "tts_webui_extension.piper_tts"

func newH(cat fakeCat, p *fakeProcs) *Handler {
	return New(Options{Cat: cat, Procs: p, Token: "tok", PublicPort: 7770})
}

func TestOpenStartsInstalledEngine(t *testing.T) {
	p := &fakeProcs{}
	h := newH(fakeCat{piper: nil}, p)
	w := do(h, "POST", Prefix+piper+"/open", "tok")
	if w.Code != 200 {
		t.Fatalf("status %d: %s", w.Code, w.Body)
	}
	var a Answer
	if err := json.Unmarshal(w.Body.Bytes(), &a); err != nil {
		t.Fatal(err)
	}
	if a.State != extprocs.StateStarting || a.PublicPort != 7770 || a.Path != "/"+piper+"/" || a.Name == "" {
		t.Fatalf("answer %+v", a)
	}
	if len(p.started) != 1 || p.started[0] != piper {
		t.Fatalf("started %v", p.started)
	}
	// GET never starts anything and reports the state.
	p.state = extprocs.StateReady
	w = do(h, "GET", Prefix+piper, "tok")
	_ = json.Unmarshal(w.Body.Bytes(), &a)
	if w.Code != 200 || a.State != extprocs.StateReady || len(p.started) != 1 {
		t.Fatalf("get: %d %+v started %v", w.Code, a, p.started)
	}
}

func TestStoppedWhenNeverStarted(t *testing.T) {
	h := newH(fakeCat{piper: nil}, &fakeProcs{})
	w := do(h, "GET", Prefix+piper, "tok")
	var a Answer
	_ = json.Unmarshal(w.Body.Bytes(), &a)
	if a.State != "stopped" {
		t.Fatalf("state %q", a.State)
	}
}

func TestRefusals(t *testing.T) {
	p := &fakeProcs{}
	h := newH(fakeCat{piper: nil, "half": extcatalog.ErrNotInstalled}, p)
	cases := []struct {
		name, method, path, token string
		want                      int
	}{
		{"no token", "POST", Prefix + piper + "/open", "", 403},
		{"wrong token", "POST", Prefix + piper + "/open", "x", 403},
		{"unknown name", "POST", Prefix + "nope/open", "tok", 404},
		{"not installed", "POST", Prefix + "half/open", "tok", 409},
		{"bad name", "POST", Prefix + "a..b/open", "tok", 400},
		{"get with open", "GET", Prefix + piper + "/open", "tok", 405},
		{"post without action", "POST", Prefix + piper, "tok", 405},
		{"other action", "POST", Prefix + piper + "/stop", "tok", 404},
	}
	for _, c := range cases {
		if w := do(h, c.method, c.path, c.token); w.Code != c.want {
			t.Errorf("%s: status %d, want %d (%s)", c.name, w.Code, c.want, w.Body)
		}
	}
	if len(p.started) != 0 {
		t.Fatalf("a refused request started %v", p.started)
	}
}

func TestNonLoopbackRefused(t *testing.T) {
	h := newH(fakeCat{piper: nil}, &fakeProcs{})
	r := httptest.NewRequest("POST", Prefix+piper+"/open", nil)
	r.RemoteAddr = "192.168.1.5:1"
	r.Header.Set(tokenHeader, "tok")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatalf("status %d", w.Code)
	}
}

func TestListNeedsTheToken(t *testing.T) {
	h := newH(fakeCat{piper: nil, "half": extcatalog.ErrNotInstalled}, &fakeProcs{})
	if w := do(h, "GET", Prefix, ""); w.Code != 403 {
		t.Fatalf("no token: %d", w.Code)
	}
	w := do(h, "GET", Prefix, "tok")
	var list []extcatalog.InstalledEngine
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil || w.Code != 200 || len(list) != 1 || list[0].Package != piper {
		t.Fatalf("%d %v %s", w.Code, err, w.Body)
	}
	if w := do(h, "POST", Prefix, "tok"); w.Code != 405 {
		t.Fatalf("post: %d", w.Code)
	}
}
