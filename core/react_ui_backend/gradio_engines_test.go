package react_ui_backend

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// engineSetup is a Gradio "host" stub (GRADIO_BACKEND), a second stub that plays
// the engine behind the router at /<pkg>/, and a control API that says what the
// launcher would say about the engine.
type engineSetup struct {
	host, engine *stubGradio
	front, base  string // base is the engine address the router serves
	mu           sync.Mutex
	calls        []string
}

func (e *engineSetup) called() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]string(nil), e.calls...)
}

func newEngineSetup(t *testing.T, pkg string, states []string, native bool) *engineSetup {
	t.Helper()
	es := &engineSetup{}
	es.host = newStubGradio(t) // sets GRADIO_BACKEND to the host
	es.engine = newStubGradio(t)
	t.Setenv("GRADIO_BACKEND", es.host.srv.URL+"/")

	// The router: /<pkg>/ -> the engine stub.
	router := http.NewServeMux()
	router.Handle("/"+pkg+"/gradio_api/file=C:/tmp/a.wav", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "RIFF-from-engine") }))
	router.Handle("/"+pkg+"/", http.StripPrefix("/"+pkg, es.engine.srv.Config.Handler))
	rsrv := httptest.NewServer(router)
	t.Cleanup(rsrv.Close)
	_, port, _ := net.SplitHostPort(strings.TrimPrefix(rsrv.URL, "http://"))
	pubPort, _ := strconv.Atoi(port)

	step := 0
	core := newFakeCore(t)
	_ = core
	ctl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(coreTokenHeader) != "tok" {
			http.Error(w, "forbidden", 403)
			return
		}
		es.mu.Lock()
		es.calls = append(es.calls, r.Method+" "+r.URL.Path)
		es.mu.Unlock()
		if !native {
			w.WriteHeader(404)
			_, _ = w.Write([]byte(`{"error":"not an engine"}`))
			return
		}
		state := states[min(step, len(states)-1)]
		if r.Method == http.MethodGet || strings.HasSuffix(r.URL.Path, "/open") {
			step++
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"state":"` + state + `","public_port":` + port + `,"path":"/` + pkg + `/"}`))
	}))
	t.Cleanup(ctl.Close)
	_ = pubPort
	es.base = "http://127.0.0.1:" + port + "/" + pkg + "/"

	c := &Core{}
	c.Set(ctl.URL, "tok")
	h, err := NewHandler(Config{Root: t.TempDir(), Port: testPort, Routes: routes(t, "gradio,gradio-file-proxy"), Core: c})
	if err != nil {
		t.Fatal(err)
	}
	front := httptest.NewServer(h)
	t.Cleanup(front.Close)
	es.front = front.URL
	return es
}

func TestOwnedApiNameGoesToTheEngineNotTheHost(t *testing.T) {
	enginePollWas := enginePoll
	enginePoll = 5 * time.Millisecond
	defer func() { enginePoll = enginePollWas }()

	es := newEngineSetup(t, "tts_webui_extension.mms", []string{"stopped", "starting", "ready"}, true)
	es.engine.script[1] = []string{es.engine.completed(`[{"path":"a.wav","url":"u"},{"rate":1},"outputs/x",null]`)}
	es.host.script[1] = []string{es.host.completed(`[{"path":"a.wav","url":"u"},{"rate":99},"HOST",null]`)}

	status, body, _ := post(t, es.front+"/api/gradio/mms", "application/json", `{"text":"hi"}`)
	if status != 200 || !strings.Contains(body, `"folder_root":"outputs/x"`) || strings.Contains(body, "HOST") {
		t.Fatalf("status %d body %s: the answer did not come from the engine", status, body)
	}
	if es.engine.lastJoin() == "" || es.host.lastJoin() != "" {
		t.Fatalf("engine joins %q, host joins %q", es.engine.lastJoin(), es.host.lastJoin())
	}
	got := es.called()
	if len(got) == 0 || got[0] != "POST /api/engines/tts_webui_extension.mms/open" {
		t.Fatalf("the engine was not opened first: %v", got)
	}
}

func TestOwnedApiNameOfANonNativePackageStaysOnTheHost(t *testing.T) {
	es := newEngineSetup(t, "tts_webui_extension.mms", nil, false)
	es.host.script[1] = []string{es.host.completed(`[{"path":"a.wav","url":"u"},{"rate":1},"HOST",null]`)}
	status, body, _ := post(t, es.front+"/api/gradio/mms", "application/json", `{"text":"hi"}`)
	if status != 200 || !strings.Contains(body, "HOST") || es.engine.lastJoin() != "" {
		t.Fatalf("status %d body %s engine joins %q", status, body, es.engine.lastJoin())
	}
}

func TestOwnedApiNameWithAFailedEngineIsAnErrorNotTheHost(t *testing.T) {
	es := newEngineSetup(t, "tts_webui_extension.mms", []string{"failed"}, true)
	status, body, _ := post(t, es.front+"/api/gradio/mms", "application/json", `{"text":"hi"}`)
	if status != http.StatusBadGateway || !strings.Contains(body, "did not start") || es.host.lastJoin() != "" {
		t.Fatalf("status %d body %s host joins %q", status, body, es.host.lastJoin())
	}
}

func TestEngineFileComesThroughTheEngineBackend(t *testing.T) {
	const pkg = "tts_webui_extension.mms"
	es := newEngineSetup(t, pkg, []string{"ready"}, true)
	url := es.base + "gradio_api/file=C:/tmp/a.wav"
	es.engine.script[1] = []string{es.engine.completed(`[{"path":"C:/tmp/a.wav","url":"` + url + `"},{"rate":1},"x",null]`)}
	status, body, _ := post(t, es.front+"/api/gradio/mms", "application/json", `{"text":"hi"}`)
	want := `"url":"/api/gradio-file-proxy/engine/` + pkg + `/gradio_api/file=C:/tmp/a.wav"`
	if status != 200 || !strings.Contains(body, want) {
		t.Fatalf("status %d body %s, want %s", status, body, want)
	}
	get := func(path string) (int, string) {
		req, _ := http.NewRequest("GET", es.front+path, nil)
		req.Host = "127.0.0.1:" + strconv.Itoa(testPort)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(b)
	}
	if code, b := get("/api/gradio-file-proxy/engine/" + pkg + "/gradio_api/file=C:/tmp/a.wav"); code != 200 || b != "RIFF-from-engine" {
		t.Fatalf("file from the engine: %d %q", code, b)
	}
	if code, _ := get("/api/gradio-file-proxy/engine/other.pkg/gradio_api/file=x"); code != 400 {
		t.Fatalf("a package no engine owns: %d, want 400", code)
	}
}
