package react_ui_backend

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestStringifyMatchesJavaScript(t *testing.T) {
	cases := []struct{ in, want string }{
		{`1.0`, `1`},
		{`-0`, `0`},
		{`0.1`, `0.1`},
		{`1e21`, `1e+21`},
		{`123456789012345680000`, `123456789012345680000`},
		{`1e-7`, `1e-7`},
		{`0.000001`, `0.000001`},
		{`1.5e-10`, `1.5e-10`},
		{`1e400`, `null`},
		{`"\u00e9\u2028\u0001\"\\/"`, "\"é\u2028\\u0001\\\"\\\\/\""},
		{`"<a>&"`, `"<a>&"`},
		{`{"b":1,"2":2,"a":3,"1":4,"01":5}`, `{"1":4,"2":2,"b":1,"a":3,"01":5}`},
		{`{"a":1,"a":2}`, `{"a":2}`},
		{`[1,[true,null],{"x":"y"}]`, `[1,[true,null],{"x":"y"}]`},
	}
	for _, c := range cases {
		v, err := parseJSON([]byte(c.in))
		if err != nil {
			t.Fatalf("parse %s: %v", c.in, err)
		}
		if got, _ := stringifyJSON(v); got != c.want {
			t.Errorf("%s: got %s, want %s", c.in, got, c.want)
		}
	}
	a, b := 0.1, 0.2
	if got, _ := stringifyJSON(obj("a", undefined, "b", []any{undefined}, "c", a+b)); got != `{"b":[null],"c":0.30000000000000004}` {
		t.Errorf("undefined handling: %s", got)
	}
	if _, ok := stringifyJSON(undefined); ok {
		t.Error("undefined stringified to a value")
	}
	if got := jsNumberString(math.Inf(1)); got != "Infinity" {
		t.Errorf("Infinity: %s", got)
	}
	for _, bad := range []string{``, `{`, `[1,]`, `{"a":1} x`, `NaN`} {
		if _, err := parseJSON([]byte(bad)); err == nil {
			t.Errorf("%q parsed", bad)
		}
	}
}

// stubGradio is a small Gradio 5 server speaking the sse_v3 queue protocol.
type stubGradio struct {
	t   *testing.T
	srv *httptest.Server

	mu      sync.Mutex
	joins   []string         // queue/join bodies
	uploads [][]byte         // uploaded file contents
	script  map[int][]string // fn_index -> SSE messages after process_starts
	pending map[string][]string
}

const stubConfig = `{
 "version": "5.49.1", "protocol": "sse_v3", "api_prefix": "/gradio_api", "enable_queue": true,
 "components": [
  {"id": 1, "type": "textbox"}, {"id": 2, "type": "state"}, {"id": 3, "type": "audio"},
  {"id": 4, "type": "json"}, {"id": 5, "type": "textbox"}, {"id": 6, "type": "number"},
  {"id": 7, "type": "dropdown"}, {"id": 8, "type": "file"}
 ],
 "dependencies": [
  {"id": 0, "api_name": false, "inputs": [], "outputs": []},
  {"id": 1, "api_name": "mms", "inputs": [1, 6, 2], "outputs": [3, 4, 5, 2]},
  {"id": 2, "api_name": "get_gpu_info", "inputs": [], "outputs": [4]},
  {"id": 3, "api_name": "generate_tortoise_1", "inputs": [1], "outputs": [3, 5, 4], "types": {"generator": true}},
  {"id": 4, "api_name": "rvc_model_reload", "inputs": [], "outputs": [7], "queue": false},
  {"id": 5, "api_name": "vocos_wav", "inputs": [8], "outputs": [4]}
 ]
}`

const stubInfo = `{"named_endpoints": {
 "/mms": {"parameters": [
   {"parameter_name": "text", "parameter_has_default": false, "component": "Textbox"},
   {"parameter_name": "speed", "parameter_has_default": true, "parameter_default": 1.0, "component": "Number"}
 ], "returns": []},
 "/get_gpu_info": {"parameters": [], "returns": []},
 "/generate_tortoise_1": {"parameters": [{"parameter_name": "text", "parameter_has_default": false, "component": "Textbox"}], "returns": []},
 "/rvc_model_reload": {"parameters": [], "returns": []},
 "/vocos_wav": {"parameters": [{"parameter_name": "audio", "parameter_has_default": false, "component": "File"}], "returns": []}
}, "unnamed_endpoints": {}}`

func newStubGradio(t *testing.T) *stubGradio {
	s := &stubGradio{t: t, script: map[int][]string{}, pending: map[string][]string{}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /config", func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, stubConfig) })
	mux.HandleFunc("GET /gradio_api/info", func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, stubInfo) })
	mux.HandleFunc("POST /gradio_api/upload", func(w http.ResponseWriter, r *http.Request) {
		f, h, err := r.FormFile("files")
		if err != nil || h.Filename != "blob" {
			http.Error(w, "bad upload", 400)
			return
		}
		data, _ := io.ReadAll(f)
		s.mu.Lock()
		s.uploads = append(s.uploads, data)
		n := len(s.uploads)
		s.mu.Unlock()
		fmt.Fprintf(w, `["C:\\tmp\\gradio\\%d\\blob"]`, n)
	})
	mux.HandleFunc("POST /gradio_api/run/rvc_model_reload", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"data":[{"choices":[["a","1"],["b","2"]],"__type__":"update"}],"duration":0.1}`)
	})
	mux.HandleFunc("POST /gradio_api/queue/join", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var join struct {
			FnIndex     int    `json:"fn_index"`
			SessionHash string `json:"session_hash"`
		}
		_ = json.Unmarshal(body, &join)
		s.mu.Lock()
		s.joins = append(s.joins, string(body))
		s.pending[join.SessionHash] = s.script[join.FnIndex]
		s.mu.Unlock()
		_, _ = io.WriteString(w, `{"event_id":"ev1"}`)
	})
	mux.HandleFunc("GET /gradio_api/queue/data", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		msgs := s.pending[r.URL.Query().Get("session_hash")]
		s.mu.Unlock()
		w.Header().Set("Content-Type", "text/event-stream")
		send := func(m string) {
			fmt.Fprintf(w, "data: %s\n\n", m)
			w.(http.Flusher).Flush()
		}
		send(`{"msg":"estimation","event_id":"ev1","rank":0,"queue_size":1,"rank_eta":null}`)
		send(`{"msg":"process_starts","event_id":"ev1","eta":null}`)
		for _, m := range msgs {
			if m == "hold" {
				<-r.Context().Done()
				return
			}
			send(m)
		}
		send(`{"msg":"close_stream","event_id":null}`)
	})
	s.srv = httptest.NewServer(mux)
	t.Cleanup(s.srv.Close)
	t.Setenv("GRADIO_BACKEND", s.srv.URL+"/")
	t.Setenv("GRADIO_AUTH", "")
	return s
}

func (s *stubGradio) completed(data string) string {
	return `{"msg":"process_completed","event_id":"ev1","output":{"data":` + data + `,"is_generating":false,"duration":0.5},"success":true}`
}

func (s *stubGradio) lastJoin() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.joins) == 0 {
		return ""
	}
	return s.joins[len(s.joins)-1]
}

// gradioFront serves the route with the gradio claim on, rooted at root.
func gradioFront(t *testing.T, root string) string {
	t.Helper()
	h, err := NewHandler(Config{Root: root, Port: testPort, Routes: routes(t, "gradio")})
	if err != nil {
		t.Fatal(err)
	}
	front := httptest.NewServer(h)
	t.Cleanup(front.Close)
	return front.URL
}

func post(t *testing.T, url, contentType, body string) (int, string, http.Header) {
	t.Helper()
	req, _ := http.NewRequest("POST", url, strings.NewReader(body))
	req.Host = "127.0.0.1:" + strconv.Itoa(testPort)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b), resp.Header
}

func TestGradioPredictMapsTheResult(t *testing.T) {
	stub := newStubGradio(t)
	stub.script[1] = []string{stub.completed(
		`[{"path":"C:\\tmp\\a.wav","url":"` + stub.srv.URL + `/gradio_api/file=C:/tmp/a.wav","meta":{"_type":"gradio.FileData"}},` +
			`{"rate":1.0,"name":"caf\u00e9"},"outputs/x",null]`)}
	front := gradioFront(t, t.TempDir())

	// The browser sends text/plain, which Next leaves as a string.
	status, body, header := post(t, front+"/api/gradio/mms", "text/plain;charset=UTF-8", `{"text":"hi"}`)
	want := `{"audio":{"path":"C:\\tmp\\a.wav","url":"/api/gradio-file-proxy/gradio_api/file=C:/tmp/a.wav","meta":{"_type":"gradio.FileData"}},"metadata":{"rate":1,"name":"café"},"folder_root":"outputs/x"}`
	if status != 200 || body != want {
		t.Fatalf("got %d %s\nwant %s", status, body, want)
	}
	if ct := header.Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type %q", ct)
	}
	join := stub.lastJoin()
	if !strings.HasPrefix(join, `{"data":["hi",1,null],"event_data":null,"fn_index":1,"trigger_id":null,"session_hash":"`) {
		t.Fatalf("join body %s", join)
	}
}

func TestNextETagMatchesNext(t *testing.T) {
	// Values taken from real `next start` responses.
	cases := map[string]string{
		`{"data":{"error":"Not found"}}`: `"189mdbjr3gau"`,
		`[]`:                             `"38jmpejbxv2"`,
	}
	for body, want := range cases {
		if got := nextETag(body); got != want {
			t.Errorf("%s: %s, want %s", body, got, want)
		}
	}
	if !etagFresh(http.Header{"If-None-Match": {`W/"38jmpejbxv2", "x"`}}, `"38jmpejbxv2"`) {
		t.Error("weak match refused")
	}
	if got := negotiateEncoding("gzip, deflate, br"); got != "gzip" {
		t.Errorf("negotiated %q", got)
	}
	if got := negotiateEncoding("br, deflate;q=0.5"); got != "deflate" {
		t.Errorf("negotiated %q", got)
	}
	if got := negotiateEncoding("gzip;q=0, br"); got != "" {
		t.Errorf("negotiated %q", got)
	}
}

func TestGradioErrorsAre500(t *testing.T) {
	stub := newStubGradio(t)
	stub.script[1] = []string{`{"msg":"process_completed","event_id":"ev1","output":{"error":"CUDA error"},"success":false}`}
	front := gradioFront(t, t.TempDir())
	status, body, header := post(t, front+"/api/gradio/mms", "", `{"text":"hi"}`)
	if status != 500 || body != "Internal Server Error" || header.Get("Content-Type") != "" {
		t.Fatalf("got %d %q %v", status, body, header)
	}
}

func TestGradioNullOutputIs500(t *testing.T) {
	stub := newStubGradio(t)
	stub.script[1] = []string{stub.completed(`[null,{},"x",null]`)}
	front := gradioFront(t, t.TempDir())
	if status, body, _ := post(t, front+"/api/gradio/mms", "", `{"text":"hi"}`); status != 500 {
		t.Fatalf("got %d %s; a null output makes proxyGradioFile throw", status, body)
	}
}

func TestGradioRequestErrors(t *testing.T) {
	newStubGradio(t)
	front := gradioFront(t, t.TempDir())
	cases := []struct {
		name, path, ctype, body string
		status                  int
		want                    string
	}{
		{"unknown name", "/api/gradio/nope", "", "", 404, `{"data":{"error":"Not found"}}`},
		{"prototype name", "/api/gradio/constructor", "", "", 404, `{"data":{"error":"Not found"}}`},
		{"bad JSON body", "/api/gradio/mms", "application/json", "{", 400, "Invalid JSON"},
		{"bad text body", "/api/gradio/mms", "text/plain", "{", 500, "Internal Server Error"},
		{"too large", "/api/gradio/mms", "application/json", `"` + strings.Repeat("x", 1<<20) + `"`, 413, "Body exceeded 1mb limit"},
		// Destructuring null throws in an async function, so the request fails.
		// (For /mms the same body never gets an answer: see the hang test.)
		{"null body", "/api/gradio/musicgen", "application/json", "null", 500, "Internal Server Error"},
	}
	for _, c := range cases {
		if status, body, _ := post(t, front+c.path, c.ctype, c.body); status != c.status || body != c.want {
			t.Errorf("%s: got %d %q, want %d %q", c.name, status, body, c.status, c.want)
		}
	}
}

func TestGradioHangsWhereTheOriginalNeverAnswers(t *testing.T) {
	newStubGradio(t)
	front := gradioFront(t, t.TempDir())
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	// "seed" is not a parameter of /mms: map_data_to_params throws inside
	// predict's async executor, so Next never answers.
	req, _ := http.NewRequestWithContext(ctx, "POST", front+"/api/gradio/mms", strings.NewReader(`{"text":"hi","seed":1}`))
	req.Host = "127.0.0.1:" + strconv.Itoa(testPort)
	resp, err := http.DefaultClient.Do(req)
	if err == nil {
		resp.Body.Close()
		t.Fatalf("got status %d, want no answer", resp.StatusCode)
	}
}

func TestGradioGeneratorAppliesDiffsAndDropsTheLast(t *testing.T) {
	stub := newStubGradio(t)
	gen := func(data string) string {
		return `{"msg":"process_generating","event_id":"ev1","output":{"data":` + data + `,"is_generating":true},"success":true}`
	}
	stub.script[3] = []string{
		gen(`[{"url":"` + stub.srv.URL + `/gradio_api/file=1.wav"},"b1",{"n":1}]`),
		gen(`[[["replace",["url"],"` + stub.srv.URL + `/gradio_api/file=2.wav"]],[["append",[],"x"]],[["replace",["n"],2]]]`),
		// The last event is dropped by the original ("return {}" in generate_tortoise_long).
		stub.completed(`[{},"",{}]`),
	}
	front := gradioFront(t, t.TempDir())
	status, body, _ := post(t, front+"/api/gradio/tortoise", "", `{"text":"hi","candidates":1,"seed":7}`)
	want := `[{"audio":{"url":"/api/gradio-file-proxy/gradio_api/file=1.wav"},"seed":7,"bundle_name":"b1","metadata":{"n":1}},` +
		`{"audio":{"url":"/api/gradio-file-proxy/gradio_api/file=2.wav"},"seed":7,"bundle_name":"b1x","metadata":{"n":2}}]`
	if status != 200 || body != want {
		t.Fatalf("got %d %s\nwant %s", status, body, want)
	}
}

func TestGradioUnqueuedEndpointUsesRun(t *testing.T) {
	newStubGradio(t)
	front := gradioFront(t, t.TempDir())
	if status, body, _ := post(t, front+"/api/gradio/rvc_model_reload", "", ""); status != 200 || body != `["a","b"]` {
		t.Fatalf("got %d %s", status, body)
	}
}

func TestGradioUploadsLocalCacheFiles(t *testing.T) {
	stub := newStubGradio(t)
	stub.script[5] = []string{stub.completed(`[{"ok":true}]`)}
	root := t.TempDir()
	cache := filepath.Join(root, "frontend", "public", "file-input-cache")
	if err := os.MkdirAll(cache, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cache, "a.wav"), []byte("RIFF-test"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "secret.txt"), []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	front := gradioFront(t, root)

	status, body, _ := post(t, front+"/api/gradio/vocos_wav", "", `{"audio":{"__type":"local_cache_file","path":"file-input-cache/a.wav"}}`)
	if status != 200 || body != `{"ok":true}` {
		t.Fatalf("got %d %s", status, body)
	}
	if len(stub.uploads) != 1 || string(stub.uploads[0]) != "RIFF-test" {
		t.Fatalf("uploads %q", stub.uploads)
	}
	if join := stub.lastJoin(); !strings.HasPrefix(join, `{"data":[{"path":"C:\\tmp\\gradio\\1\\blob","meta":{"_type":"gradio.FileData"}}],`) {
		t.Fatalf("join body %s", join)
	}

	status, _, _ = post(t, front+"/api/gradio/vocos_wav", "", `{"audio":{"__type":"local_cache_file","path":"../../secret.txt"}}`)
	if status != 500 || len(stub.uploads) != 1 {
		t.Fatalf("a path outside public was read: status %d, uploads %d", status, len(stub.uploads))
	}

	// A null parameter makes resolveFileParams throw in the original.
	if status, _, _ := post(t, front+"/api/gradio/vocos_wav", "", `{"audio":null}`); status != 500 {
		t.Fatalf("null parameter: status %d", status)
	}
}

func TestGradioBackendDownIs500(t *testing.T) {
	t.Setenv("GRADIO_BACKEND", "http://127.0.0.1:1/")
	front := gradioFront(t, t.TempDir())
	if status, _, _ := post(t, front+"/api/gradio/get_gpu_info", "", ""); status != 500 {
		t.Fatalf("status %d", status)
	}
}
