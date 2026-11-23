package react_ui_backend

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

const fakeToken = "secret-token-123"

// fakeCore is a control API that insists on the token and records requests.
type fakeCore struct {
	mu       sync.Mutex
	seen     []string // "METHOD path?query body"
	config   string
	srv      *httptest.Server
	badToken int
}

func newFakeCore(t *testing.T) *fakeCore {
	t.Helper()
	f := &fakeCore{config: `{"gradio_interface_options":{"server_name":"0.0.0.0","server_port":7770,"auth":["bob","pw"],"share":true,"ssl_keyfile_password":"kp"}}`}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		if r.Header.Get(coreTokenHeader) != fakeToken {
			f.badToken++
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		body, _ := io.ReadAll(r.Body)
		f.seen = append(f.seen, r.Method+" "+r.URL.RequestURI()+" "+string(body))
		switch r.URL.Path {
		case "/api/pip/list":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `[{"name":"gradio","version":"5.0"}]`)
		case "/api/config":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, f.config)
		case "/api/env/model-locations":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"hf_home":"D:\\hf"}`)
		default:
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `"ok"`)
		}
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeCore) requests() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.seen...)
}

func coreFront(t *testing.T, core *Core) string {
	t.Helper()
	h, err := NewHandler(Config{Root: exportFixture(t), Port: testPort, Routes: routes(t, "core-pip,core-logs,core-model-locations,core-settings"), Core: core})
	if err != nil {
		t.Fatal(err)
	}
	front := httptest.NewServer(h)
	t.Cleanup(front.Close)
	return front.URL
}

func readAll(t *testing.T, r *http.Response) string {
	t.Helper()
	b, _ := io.ReadAll(r.Body)
	return string(b)
}

var jsonHdr = map[string]string{"Content-Type": "application/json"}

func TestCoreRoutesNeedACore(t *testing.T) {
	for _, core := range []*Core{nil, {}} {
		front := coreFront(t, core)
		resp := request(t, "GET", front+"/api/go-data/pip-list", nil, nil)
		if resp.StatusCode != 503 {
			t.Errorf("no core: %d", resp.StatusCode)
		}
	}
}

func TestPipListAddsTheToken(t *testing.T) {
	f := newFakeCore(t)
	core := &Core{}
	core.Set(f.srv.URL, fakeToken)
	front := coreFront(t, core)
	resp := request(t, "GET", front+"/api/go-data/pip-list", nil, nil)
	body := readAll(t, resp)
	if resp.StatusCode != 200 || !strings.Contains(body, `"gradio"`) || f.badToken != 0 {
		t.Fatalf("%d %q bad=%d", resp.StatusCode, body, f.badToken)
	}
	if strings.Contains(body, fakeToken) || resp.Header.Get(coreTokenHeader) != "" {
		t.Fatal("the token reached the browser")
	}
	// A token the browser sends is not forwarded: the Go side adds its own.
	request(t, "GET", front+"/api/go-data/pip-list", nil, map[string]string{coreTokenHeader: "x"})
	if f.badToken != 0 {
		t.Fatal("a browser header replaced the token")
	}
}

func TestLogsOperations(t *testing.T) {
	f := newFakeCore(t)
	core := &Core{}
	core.Set(f.srv.URL, fakeToken)
	front := coreFront(t, core)

	if r := request(t, "GET", front+"/api/go-data/logs/read?file=pip-install-a.log&max_lines=100", nil, nil); r.StatusCode != 200 {
		t.Fatalf("read: %d", r.StatusCode)
	}
	got := f.requests()
	if len(got) != 1 || !strings.HasPrefix(got[0], "GET /api/logs/read?file=pip-install-a.log&max_lines=100") {
		t.Fatalf("forwarded: %v", got)
	}
	// Unknown operations never reach the core, whatever the method.
	for _, c := range [][2]string{{"GET", "config"}, {"GET", "delete"}, {"POST", "read"}, {"POST", "..%2fconfig"}, {"GET", "files%2f..%2f..%2fconfig"}} {
		r := request(t, c[0], front+"/api/go-data/logs/"+c[1], strings.NewReader("{}"), jsonHdr)
		if r.StatusCode == 200 {
			t.Errorf("%v answered 200", c)
		}
	}
	if len(f.requests()) != 1 {
		t.Fatalf("an unknown operation was forwarded: %v", f.requests())
	}
	// Delete needs a JSON body type (the CSRF rule) and is forwarded as POST.
	if r := request(t, "POST", front+"/api/go-data/logs/delete", strings.NewReader(`{"file":"a.log"}`), map[string]string{"Content-Type": "text/plain"}); r.StatusCode != 415 {
		t.Fatalf("text/plain: %d", r.StatusCode)
	}
	if r := request(t, "POST", front+"/api/go-data/logs/delete", strings.NewReader(`{"file":"a.log"}`), jsonHdr); r.StatusCode != 200 {
		t.Fatalf("delete: %d", r.StatusCode)
	}
	if got := f.requests(); len(got) != 2 || got[1] != `POST /api/logs/delete {"file":"a.log"}` {
		t.Fatalf("forwarded: %v", got)
	}
}

func TestMutationsRefuseOtherOrigins(t *testing.T) {
	f := newFakeCore(t)
	core := &Core{}
	core.Set(f.srv.URL, fakeToken)
	front := coreFront(t, core)
	hdr := map[string]string{"Content-Type": "application/json", "Origin": "http://evil.example"}
	for _, c := range [][2]string{{"POST", "/api/go-data/logs/delete-all"}, {"PUT", "/api/go-data/model-locations"}, {"PUT", "/api/go-data/server-settings"}} {
		if r := request(t, c[0], front+c[1], strings.NewReader("{}"), hdr); r.StatusCode != 403 {
			t.Errorf("%v: %d", c, r.StatusCode)
		}
	}
	if r := request(t, "GET", front+"/api/go-data/pip-list", nil, map[string]string{"Origin": "http://evil.example"}); r.StatusCode != 403 {
		t.Errorf("GET from another origin: %d", r.StatusCode)
	}
	if len(f.requests()) != 0 {
		t.Fatalf("a refused request reached the core: %v", f.requests())
	}
}

func TestModelLocations(t *testing.T) {
	f := newFakeCore(t)
	core := &Core{}
	core.Set(f.srv.URL, fakeToken)
	front := coreFront(t, core)
	r := request(t, "GET", front+"/api/go-data/model-locations", nil, nil)
	if b := readAll(t, r); r.StatusCode != 200 || !strings.Contains(b, "hf_home") {
		t.Fatalf("%d %q", r.StatusCode, b)
	}
	r = request(t, "PUT", front+"/api/go-data/model-locations", strings.NewReader(`{"hf_home":"x"}`), jsonHdr)
	if r.StatusCode != 200 {
		t.Fatalf("put: %d", r.StatusCode)
	}
	got := f.requests()
	if got[1] != `POST /api/env/save-model-locations {"hf_home":"x"}` {
		t.Fatalf("forwarded: %v", got)
	}
}

func TestServerSettingsHideSecretsAndMerge(t *testing.T) {
	f := newFakeCore(t)
	core := &Core{}
	core.Set(f.srv.URL, fakeToken)
	front := coreFront(t, core)

	r := request(t, "GET", front+"/api/go-data/server-settings", nil, nil)
	b := readAll(t, r)
	if r.StatusCode != 200 || strings.Contains(b, `"pw"`) || strings.Contains(b, `kp`) || strings.Contains(b, "share") {
		t.Fatalf("%d %s", r.StatusCode, b)
	}
	if !strings.Contains(b, `"auth_users":["bob"]`) || !strings.Contains(b, `"ssl_keyfile_password_set":true`) || !strings.Contains(b, `"server_port":7770`) {
		t.Fatalf("view: %s", b)
	}

	// Change the port only: everything else, including Gradio-only options and
	// the secrets, stays.
	r = request(t, "PUT", front+"/api/go-data/server-settings", strings.NewReader(`{"server_port":7800}`), jsonHdr)
	if r.StatusCode != 200 {
		t.Fatalf("put: %d %s", r.StatusCode, readAll(t, r))
	}
	got := f.requests()
	last := got[len(got)-1]
	for _, want := range []string{"POST /api/config/replace-section", `"server_port":7800`, `"share":true`, `"auth":["bob","pw"]`, `"ssl_keyfile_password":"kp"`} {
		if !strings.Contains(last, want) {
			t.Errorf("missing %s in %s", want, last)
		}
	}

	// null removes the password; a pair sets it; bad input is refused before the core.
	n := len(f.requests())
	for _, bad := range []string{`{"server_port":0}`, `{"server_port":70000}`, `{"nope":1}`, `{"auth":{"user":"a"}}`, `{"server_name":"a\nb"}`, `[1]`, `{"server_port":"1"}`} {
		if r := request(t, "PUT", front+"/api/go-data/server-settings", strings.NewReader(bad), jsonHdr); r.StatusCode != 400 {
			t.Errorf("%s: %d", bad, r.StatusCode)
		}
	}
	for _, q := range f.requests()[n:] {
		if strings.HasPrefix(q, "POST") {
			t.Fatalf("a bad request wrote: %s", q)
		}
	}
	request(t, "PUT", front+"/api/go-data/server-settings", strings.NewReader(`{"auth":null,"ssl_keyfile_password":null}`), jsonHdr)
	got = f.requests()
	last = got[len(got)-1]
	if strings.Contains(last, `"auth"`) || strings.Contains(last, "ssl_keyfile_password") {
		t.Errorf("not removed: %s", last)
	}
}
