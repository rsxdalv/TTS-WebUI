package extprocs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform/legacyenv"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
)

var fakeBin string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "extprocs-test-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fakeBin = filepath.Join(dir, "fakepy")
	if runtime.GOOS == "windows" {
		fakeBin += ".exe"
	}
	if out, err := exec.Command("go", "build", "-o", fakeBin, "./testdata/fakepy").CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "building fakepy: %v\n%s\n", err, out)
		os.Exit(1)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

type fakeRoutes struct {
	mu      sync.Mutex
	routes  map[string]string
	calls   []string
	failAdd error
}

func (f *fakeRoutes) AddRoute(prefix, target string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "add "+prefix+" "+target)
	if f.failAdd != nil {
		return f.failAdd
	}
	if f.routes == nil {
		f.routes = map[string]string{}
	}
	f.routes[prefix] = target
	return nil
}

func (f *fakeRoutes) RemoveRoute(prefix string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "remove "+prefix)
	delete(f.routes, prefix)
}

func (f *fakeRoutes) get(prefix string) (string, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	v, ok := f.routes[prefix]
	return v, ok
}

func (f *fakeRoutes) count(call string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, c := range f.calls {
		if strings.HasPrefix(c, call) {
			n++
		}
	}
	return n
}

func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for: %s", what)
}

func newMgr(t *testing.T, mod func(*Options)) (*Manager, *fakeRoutes, paths.Root) {
	t.Helper()
	root, err := paths.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	fr := &fakeRoutes{}
	o := Options{
		Root:         root,
		Platform:     testPlatform(root, []string{fakeBin}),
		Routes:       fr,
		Token:        "tok",
		ReadyTimeout: 20 * time.Second,
	}
	if mod != nil {
		mod(&o)
	}
	m := New(o)
	t.Cleanup(m.StopAll)
	return m, fr, root
}

func freeP(t *testing.T) int {
	t.Helper()
	p, err := freePort()
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func state(m *Manager, pkg string) State {
	for _, s := range m.List() {
		if s.Package == pkg {
			return s.State
		}
	}
	return ""
}

type report struct {
	Pid  int               `json:"pid"`
	Args []string          `json:"args"`
	Env  map[string]string `json:"env"`
	Exe  string            `json:"exe"`
	Cwd  string            `json:"cwd"`
}

func firstReport(t *testing.T, m *Manager, pkg string) report {
	t.Helper()
	var r report
	eventually(t, "report line from "+pkg, func() bool {
		lines, err := m.Lines(pkg)
		if err != nil {
			t.Fatal(err)
		}
		for _, l := range lines {
			if strings.HasPrefix(l, "{") {
				return json.Unmarshal([]byte(l), &r) == nil
			}
		}
		return false
	})
	return r
}

func TestStartReturnsBeforeReadyThenRegistersRoute(t *testing.T) {
	m, fr, root := newMgr(t, func(o *Options) {
		t.Setenv("FAKE_DELAY_MS", "700")
	})
	port := freeP(t)
	res, err := m.Start(context.Background(), "tts_webui_extension.bark", StartRequest{
		Port:            port,
		OpenAIProxyHost: "http://localhost:7778",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Port != port || res.Pid <= 0 || res.AlreadyRunning {
		t.Fatalf("result = %+v", res)
	}
	if got := state(m, "tts_webui_extension.bark"); got != StateStarting {
		t.Fatalf("state right after start = %q", got)
	}
	if _, ok := fr.get("/tts_webui_extension.bark"); ok {
		t.Fatal("route registered before ready")
	}
	eventually(t, "ready", func() bool { return state(m, "tts_webui_extension.bark") == StateReady })
	if got, _ := fr.get("/tts_webui_extension.bark"); got != fmt.Sprintf("http://127.0.0.1:%d", port) {
		t.Fatalf("route target = %q", got)
	}

	r := firstReport(t, m, "tts_webui_extension.bark")
	if r.Pid != res.Pid {
		t.Errorf("reported pid %d != result pid %d", r.Pid, res.Pid)
	}
	if strings.Join(r.Args, " ") != "python -m tts_webui.extensions_loader.proxy_harness" {
		t.Errorf("args = %v", r.Args)
	}
	want := map[string]string{
		"GRADIO_SERVER_PORT":          fmt.Sprint(port),
		"GRADIO_ROOT_PATH":            "/tts_webui_extension.bark",
		"TTS_WEBUI_EXTENSION_PACKAGE": "tts_webui_extension.bark",
		"OPENAI_PROXY_HOST":           "http://localhost:7778",
		"BASE_MARK":                   "base",
	}
	for k, v := range want {
		if r.Env[k] != v {
			t.Errorf("env %s = %q, want %q", k, r.Env[k], v)
		}
	}
	for _, k := range []string{"TTS_CORE_TOKEN", "TTS_CORE_URL"} {
		if _, ok := r.Env[k]; ok {
			t.Errorf("%s leaked into extension env", k)
		}
	}
	if !sameDir(r.Cwd, root.Dir()) {
		t.Errorf("cwd = %q, want %q", r.Cwd, root.Dir())
	}
}

func sameDir(a, b string) bool {
	ai, e1 := os.Stat(a)
	bi, e2 := os.Stat(b)
	return e1 == nil && e2 == nil && os.SameFile(ai, bi)
}

func TestStartIsIdempotent(t *testing.T) {
	m, fr, _ := newMgr(t, nil)
	port := freeP(t)
	a, err := m.Start(context.Background(), "pkg", StartRequest{Port: port})
	if err != nil {
		t.Fatal(err)
	}
	b, err := m.Start(context.Background(), "pkg", StartRequest{Port: freeP(t)})
	if err != nil {
		t.Fatal(err)
	}
	if b.Port != a.Port || b.Pid != a.Pid || !b.AlreadyRunning {
		t.Fatalf("second start = %+v, first = %+v", b, a)
	}
	eventually(t, "ready", func() bool { return state(m, "pkg") == StateReady })
	if n := fr.count("add /pkg"); n != 1 {
		t.Errorf("AddRoute calls = %d", n)
	}
	lines, _ := m.Lines("pkg")
	n := 0
	for _, l := range lines {
		if strings.HasPrefix(l, "{") {
			n++
		}
	}
	if n != 1 {
		t.Errorf("process started %d times", n)
	}
}

func TestStopRemovesRouteAndRestartWorks(t *testing.T) {
	m, fr, _ := newMgr(t, nil)
	a, err := m.Start(context.Background(), "pkg", StartRequest{Port: freeP(t)})
	if err != nil {
		t.Fatal(err)
	}
	eventually(t, "ready", func() bool { return state(m, "pkg") == StateReady })
	if err := m.Stop("pkg"); err != nil {
		t.Fatal(err)
	}
	if _, ok := fr.get("/pkg"); ok {
		t.Error("route still registered after stop")
	}
	if got := state(m, "pkg"); got != StateExited {
		t.Errorf("state = %q", got)
	}
	if c, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", a.Port), 300*time.Millisecond); err == nil {
		c.Close()
		t.Error("port still accepting after stop")
	}
	if err := m.Stop("pkg"); err != nil {
		t.Errorf("second stop: %v", err)
	}
	if err := m.Stop("nope"); err != ErrNotFound {
		t.Errorf("stop unknown = %v", err)
	}

	b, err := m.Start(context.Background(), "pkg", StartRequest{Port: a.Port})
	if err != nil {
		t.Fatal(err)
	}
	if b.AlreadyRunning || b.Pid == a.Pid {
		t.Errorf("restart = %+v (first %+v)", b, a)
	}
	eventually(t, "ready again", func() bool { return state(m, "pkg") == StateReady })
	if _, ok := fr.get("/pkg"); !ok {
		t.Error("route not re-registered after restart")
	}
}

func TestExitDetection(t *testing.T) {
	// Exits on its own after becoming ready.
	m, fr, _ := newMgr(t, func(o *Options) { t.Setenv("FAKE_EXIT_AFTER_MS", "1500"); t.Setenv("FAKE_EXIT_CODE", "0") })
	if _, err := m.Start(context.Background(), "late", StartRequest{Port: freeP(t)}); err != nil {
		t.Fatal(err)
	}
	eventually(t, "ready", func() bool { return state(m, "late") == StateReady })
	eventually(t, "exited", func() bool { return state(m, "late") == StateExited })
	if _, ok := fr.get("/late"); ok {
		t.Error("route kept after exit")
	}
	if s := m.List()[0]; s.ExitCode == nil || *s.ExitCode != 0 {
		t.Errorf("status = %+v", s)
	}

	// Exits before ever listening: failed.
	m2, _, _ := newMgr(t, func(o *Options) {
		t.Setenv("FAKE_NEVER_LISTEN", "1")
		t.Setenv("FAKE_EXIT_AFTER_MS", "100")
		t.Setenv("FAKE_EXIT_CODE", "7")
	})
	if _, err := m2.Start(context.Background(), "early", StartRequest{Port: freeP(t)}); err != nil {
		t.Fatal(err)
	}
	eventually(t, "failed", func() bool { return state(m2, "early") == StateFailed })
	s := m2.List()[0]
	if s.ExitCode == nil || *s.ExitCode != 7 || !strings.Contains(s.Error, "before ready") {
		t.Errorf("status = %+v", s)
	}
}

func TestReadyTimeoutMarksFailedButKeepsProcess(t *testing.T) {
	m, fr, _ := newMgr(t, func(o *Options) {
		o.ReadyTimeout = 300 * time.Millisecond
		t.Setenv("FAKE_NEVER_LISTEN", "1")
	})
	if _, err := m.Start(context.Background(), "slow", StartRequest{Port: freeP(t)}); err != nil {
		t.Fatal(err)
	}
	eventually(t, "failed", func() bool { return state(m, "slow") == StateFailed })
	if !strings.Contains(m.List()[0].Error, "not ready") {
		t.Errorf("error = %q", m.List()[0].Error)
	}
	if _, ok := fr.get("/slow"); ok {
		t.Error("route registered for non-ready extension")
	}
	if err := m.Stop("slow"); err != nil {
		t.Fatal(err)
	}
}

func TestRouteRegistrationFailure(t *testing.T) {
	m, fr, _ := newMgr(t, nil)
	fr.failAdd = fmt.Errorf("boom")
	if _, err := m.Start(context.Background(), "pkg", StartRequest{Port: freeP(t)}); err != nil {
		t.Fatal(err)
	}
	eventually(t, "failed", func() bool { return state(m, "pkg") == StateFailed })
	if !strings.Contains(m.List()[0].Error, "boom") {
		t.Errorf("error = %q", m.List()[0].Error)
	}
}

func TestNilRoutesAllowed(t *testing.T) {
	m, _, _ := newMgr(t, func(o *Options) { o.Routes = nil })
	if _, err := m.Start(context.Background(), "pkg", StartRequest{Port: freeP(t)}); err != nil {
		t.Fatal(err)
	}
	eventually(t, "ready", func() bool { return state(m, "pkg") == StateReady })
	if err := m.Stop("pkg"); err != nil {
		t.Fatal(err)
	}
}

func TestVenvPythonPreferredAndMainPythonPrefix(t *testing.T) {
	m, _, root := newMgr(t, func(o *Options) { o.Platform = testPlatform(o.Root, []string{fakeBin, "--main-prefix"}) })

	if _, err := m.Start(context.Background(), "plain", StartRequest{Port: freeP(t)}); err != nil {
		t.Fatal(err)
	}
	r := firstReport(t, m, "plain")
	if strings.Join(r.Args, " ") != "--main-prefix python -m tts_webui.extensions_loader.proxy_harness" {
		t.Errorf("main args = %v", r.Args)
	}

	rel := filepath.Join(".venvs", "native", "bin", "python")
	if runtime.GOOS == "windows" {
		rel = filepath.Join(".venvs", "native", "Scripts", "python.exe")
	}
	venv := filepath.Join(root.Dir(), rel)
	if err := os.MkdirAll(filepath.Dir(venv), 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(fakeBin)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(venv, data, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Start(context.Background(), "native", StartRequest{Port: freeP(t)}); err != nil {
		t.Fatal(err)
	}
	r = firstReport(t, m, "native")
	if !sameFile(r.Exe, venv) {
		t.Errorf("exe = %q, want venv python %q", r.Exe, venv)
	}
	if strings.Join(r.Args, " ") != "-m tts_webui.extensions_loader.proxy_harness" {
		t.Errorf("venv args = %v", r.Args)
	}
}

func sameFile(a, b string) bool { return sameDir(a, b) }

func TestInvalidPackageNames(t *testing.T) {
	m, _, _ := newMgr(t, nil)
	for _, bad := range []string{"", "..", "../x", "a/b", `a\b`, "a..b", ".hidden", "-x", "a b", "a:b"} {
		if _, err := m.Start(context.Background(), bad, StartRequest{}); err == nil {
			t.Errorf("Start(%q) accepted", bad)
		}
	}
	for _, ok := range []string{"a", "_a", "tts_webui_extension.bark", "a-b.c_d", "A1"} {
		if err := ValidatePackage(ok); err != nil {
			t.Errorf("ValidatePackage(%q): %v", ok, err)
		}
	}
}

func TestPortSelection(t *testing.T) {
	m, _, _ := newMgr(t, nil)
	for i, p := range []int{0, -1, 80, 1023, 65536, 70000} {
		res, err := m.Start(context.Background(), fmt.Sprintf("p%d", i), StartRequest{Port: p})
		if err != nil {
			t.Fatal(err)
		}
		if res.Port < 1024 || res.Port > 65535 || res.Port == p {
			t.Errorf("port %d -> %d", p, res.Port)
		}
	}
	// Busy requested port falls back to a free one.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	busy := l.Addr().(*net.TCPAddr).Port
	res, err := m.Start(context.Background(), "busy", StartRequest{Port: busy})
	if err != nil {
		t.Fatal(err)
	}
	if res.Port == busy {
		t.Error("busy port reused")
	}
	// A port held by another running extension is not handed out twice.
	first := m.List()[0].Port
	res, err = m.Start(context.Background(), "dup", StartRequest{Port: first})
	if err != nil {
		t.Fatal(err)
	}
	if res.Port == first {
		t.Error("port shared between two extensions")
	}
}

func TestStopAll(t *testing.T) {
	m, fr, _ := newMgr(t, nil)
	for _, p := range []string{"a", "b"} {
		if _, err := m.Start(context.Background(), p, StartRequest{Port: freeP(t)}); err != nil {
			t.Fatal(err)
		}
	}
	eventually(t, "both ready", func() bool { return state(m, "a") == StateReady && state(m, "b") == StateReady })
	m.StopAll()
	for _, p := range []string{"a", "b"} {
		if state(m, p) != StateExited {
			t.Errorf("%s state = %q", p, state(m, p))
		}
		if _, ok := fr.get("/" + p); ok {
			t.Errorf("route /%s left", p)
		}
	}
}

func TestLogAccess(t *testing.T) {
	m, _, _ := newMgr(t, nil)
	if _, err := m.Lines("x"); err != ErrNotFound {
		t.Errorf("Lines unknown = %v", err)
	}
	if _, _, err := m.Subscribe("x"); err != ErrNotFound {
		t.Errorf("Subscribe unknown = %v", err)
	}
	if _, err := m.Start(context.Background(), "x", StartRequest{Port: freeP(t)}); err != nil {
		t.Fatal(err)
	}
	_, cancel, err := m.Subscribe("x")
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	firstReport(t, m, "x")
}

// ---- HTTP API ----

func do(t *testing.T, h http.Handler, method, path, ctype, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.RemoteAddr = "127.0.0.1:50000"
	if ctype != "" {
		req.Header.Set("Content-Type", ctype)
	}
	if token != "" {
		req.Header.Set(TokenHeader, token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestAPIAuthRules(t *testing.T) {
	m, _, _ := newMgr(t, nil)
	h := m.Handler()
	j := "application/json"

	// non-loopback
	req := httptest.NewRequest("GET", "/api/extensions", nil)
	req.RemoteAddr = "10.1.2.3:4444"
	req.Header.Set(TokenHeader, "tok")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 403 {
		t.Errorf("non-loopback = %d", rec.Code)
	}
	for name, tok := range map[string]string{"missing": "", "wrong": "nope", "prefix": "to", "longer": "tokk"} {
		if c := do(t, h, "GET", "/api/extensions", "", tok, "").Code; c != 403 {
			t.Errorf("%s token = %d", name, c)
		}
	}
	if c := do(t, h, "POST", "/api/extensions/pkg/start", j, "nope", "{}").Code; c != 403 {
		t.Errorf("start with bad token = %d", c)
	}
	// token checked before content type
	if c := do(t, h, "POST", "/api/extensions/pkg/start", "text/plain", "nope", "{}").Code; c != 403 {
		t.Errorf("bad token + bad ctype = %d", c)
	}
	if c := do(t, h, "POST", "/api/extensions/pkg/start", "text/plain", "tok", "{}").Code; c != 415 {
		t.Errorf("wrong content type = %d", c)
	}
	if c := do(t, h, "POST", "/api/extensions/pkg/stop", "", "tok", "").Code; c != 415 {
		t.Errorf("no content type = %d", c)
	}
	if c := do(t, h, "GET", "/api/extensions/pkg/start", "", "tok", "").Code; c != 405 {
		t.Errorf("GET start = %d", c)
	}
	if c := do(t, h, "POST", "/api/extensions", j, "tok", "{}").Code; c != 405 {
		t.Errorf("POST list = %d", c)
	}
	if c := do(t, h, "POST", "/api/extensions/a..b/start", j, "tok", "{}").Code; c != 400 {
		t.Errorf("bad pkg = %d", c)
	}
	if c := do(t, h, "POST", "/api/extensions/pkg/start", j, "tok", "{not json").Code; c != 400 {
		t.Errorf("bad json = %d", c)
	}
	if c := do(t, h, "POST", "/api/extensions/pkg/stop", j, "tok", "{}").Code; c != 404 {
		t.Errorf("stop unknown = %d", c)
	}
	if c := do(t, h, "GET", "/api/extensions/pkg/other", "", "tok", "").Code; c != 404 {
		t.Errorf("unknown action = %d", c)
	}
	if c := do(t, h, "GET", "/api/extensionsx", "", "tok", "").Code; c != 404 {
		t.Errorf("sibling path = %d", c)
	}
	if c := do(t, h, "POST", "/api/extensions/pkg/start", "application/json; charset=utf-8", "tok", `{"port":0}`).Code; c != 200 {
		t.Errorf("json with charset = %d", c)
	}

	// An unconfigured token rejects everything.
	m2, _, _ := newMgr(t, func(o *Options) { o.Token = "" })
	if c := do(t, m2.Handler(), "GET", "/api/extensions", "", "", "").Code; c != 403 {
		t.Errorf("empty token option = %d", c)
	}
}

func TestAPIEndToEnd(t *testing.T) {
	m, fr, _ := newMgr(t, nil)
	mux := http.NewServeMux()
	mux.Handle("/api/extensions", m.Handler())
	mux.Handle("/api/extensions/", m.Handler())
	srv := httptest.NewServer(mux) // listens on 127.0.0.1:0
	defer srv.Close()

	call := func(method, path, body string) (int, []byte) {
		req, _ := http.NewRequest(method, srv.URL+path, bytes.NewBufferString(body))
		req.Header.Set(TokenHeader, "tok")
		if method == "POST" {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, b
	}

	port := freeP(t)
	code, b := call("POST", "/api/extensions/tts_webui_extension.bark/start", fmt.Sprintf(`{"port":%d,"openai_proxy_host":"http://localhost:7778"}`, port))
	var res StartResult
	if code != 200 || json.Unmarshal(b, &res) != nil || res.Port != port || res.Pid <= 0 {
		t.Fatalf("start = %d %s", code, b)
	}
	eventually(t, "ready", func() bool { return state(m, "tts_webui_extension.bark") == StateReady })

	code, b = call("GET", "/api/extensions", "")
	var list []map[string]any
	if code != 200 || json.Unmarshal(b, &list) != nil || len(list) != 1 {
		t.Fatalf("list = %d %s", code, b)
	}
	if list[0]["package"] != "tts_webui_extension.bark" || list[0]["state"] != "ready" || int(list[0]["port"].(float64)) != port || int(list[0]["pid"].(float64)) != res.Pid {
		t.Errorf("list entry = %v", list[0])
	}
	if code, b = call("GET", "/api/extensions/tts_webui_extension.bark/logs?tail=1", ""); code != 200 || !strings.Contains(string(b), "lines") {
		t.Errorf("logs = %d %s", code, b)
	}
	if code, b = call("POST", "/api/extensions/tts_webui_extension.bark/stop", `{}`); code != 200 {
		t.Fatalf("stop = %d %s", code, b)
	}
	if _, ok := fr.get("/tts_webui_extension.bark"); ok {
		t.Error("route left after API stop")
	}
	code, b = call("GET", "/api/extensions", "")
	if !strings.Contains(string(b), `"exited"`) {
		t.Errorf("after stop list = %d %s", code, b)
	}
}

// testPlatform is the legacy-env platform with a fake child in front of every
// command and the base environment the tests look for in the child's report.
func testPlatform(root paths.Root, prefix []string) platform.Platform {
	return legacyenv.New(legacyenv.Options{
		Root: root, Prefix: prefix,
		Env: []string{"BASE_MARK=base", "TTS_CORE_TOKEN=secret", "TTS_CORE_URL=http://127.0.0.1:1"},
	})
}
