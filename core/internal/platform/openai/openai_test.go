package openai

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rsxdalv/TTS-WebUI/core/internal/platform"
	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/history"
)

const secretKey = "sk-SENTINEL-1234567890abcdef"

// testWav is a 0.5 s 440 Hz tone, 16-bit mono at 16 kHz.
func testWav() []byte {
	const rate, n = 16000, 8000
	data := make([]byte, n*2)
	for i := 0; i < n; i++ {
		v := int16(12000 * math.Sin(2*math.Pi*440*float64(i)/rate))
		binary.LittleEndian.PutUint16(data[i*2:], uint16(v))
	}
	h := make([]byte, 44)
	copy(h, "RIFF")
	binary.LittleEndian.PutUint32(h[4:], uint32(36+len(data)))
	copy(h[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(h[16:], 16)
	binary.LittleEndian.PutUint16(h[20:], 1)
	binary.LittleEndian.PutUint16(h[22:], 1)
	binary.LittleEndian.PutUint32(h[24:], rate)
	binary.LittleEndian.PutUint32(h[28:], rate*2)
	binary.LittleEndian.PutUint16(h[32:], 2)
	binary.LittleEndian.PutUint16(h[34:], 16)
	copy(h[36:], "data")
	binary.LittleEndian.PutUint32(h[40:], uint32(len(data)))
	return append(h, data...)
}

func newClient(t *testing.T, srv *httptest.Server, key *string) (*Client, *Store, string) {
	t.Helper()
	root := t.TempDir()
	st := NewStore(root)
	if _, err := st.Put(Endpoint{Name: "main", URL: srv.URL + "/v1", Model: "tts-1", Voice: "alloy"}, key); err != nil {
		t.Fatal(err)
	}
	c := NewClient(Options{Store: st, Root: root, Now: func() time.Time { return time.Date(2026, 11, 25, 10, 30, 5, 123456000, time.Local) }})
	return c, st, root
}

func speech(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "audio/wav")
	_, _ = w.Write(testWav())
}

func TestStorePutListDelete(t *testing.T) {
	st := NewStore(t.TempDir())
	key := secretKey
	e, err := st.Put(Endpoint{Name: "a", URL: " https://api.example.com/v1/ ", Model: "tts-1"}, &key)
	if err != nil {
		t.Fatal(err)
	}
	if e.URL != "https://api.example.com/v1" || !e.HasKey {
		t.Fatalf("got %+v", e)
	}
	// A save without a key keeps the stored one.
	if e, err = st.Put(Endpoint{Name: "a", URL: "https://api.example.com/v1", Voice: "alloy"}, nil); err != nil || !e.HasKey {
		t.Fatalf("key lost: %+v %v", e, err)
	}
	// An empty key removes it.
	empty := ""
	if e, err = st.Put(Endpoint{Name: "a", URL: "https://api.example.com/v1"}, &empty); err != nil || e.HasKey {
		t.Fatalf("key kept: %+v %v", e, err)
	}
	if _, err := st.Put(Endpoint{Name: "b", URL: "http://127.0.0.1:7778/v1"}, nil); err != nil {
		t.Fatal(err)
	}
	list, _ := st.List()
	if len(list) != 2 || list[0].Name != "a" || list[1].Name != "b" {
		t.Fatalf("list %+v", list)
	}
	if err := st.Delete("a"); err != nil {
		t.Fatal(err)
	}
	if err := st.Delete("a"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second delete: %v", err)
	}
	b, _ := os.ReadFile(st.keysPath())
	if strings.Contains(string(b), secretKey) {
		t.Fatalf("deleted endpoint's key is still on disk: %s", b)
	}
}

func TestStoreRefusesBadInput(t *testing.T) {
	st := NewStore(t.TempDir())
	key := "k"
	for name, c := range map[string]struct {
		e Endpoint
		k *string
	}{
		"name with a slash":    {Endpoint{Name: "a/b", URL: "https://x.example/v1"}, nil},
		"empty name":           {Endpoint{Name: "", URL: "https://x.example/v1"}, nil},
		"no scheme":            {Endpoint{Name: "a", URL: "api.example.com"}, nil},
		"ftp":                  {Endpoint{Name: "a", URL: "ftp://x.example"}, nil},
		"key in the URL":       {Endpoint{Name: "a", URL: "https://user:pw@x.example/v1"}, nil},
		"query":                {Endpoint{Name: "a", URL: "https://x.example/v1?key=1"}, nil},
		"key over public http": {Endpoint{Name: "a", URL: "http://api.example.com/v1"}, &key},
		"key with a newline":   {Endpoint{Name: "a", URL: "https://x.example/v1"}, ptr("a\nb")},
	} {
		if _, err := st.Put(c.e, c.k); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: want ErrInvalid, got %v", name, err)
		}
	}
	for _, ok := range []string{"http://127.0.0.1:7778/v1", "http://localhost:7778/v1", "http://192.168.1.5:8000/v1", "http://[::1]:1/v1"} {
		if _, err := st.Put(Endpoint{Name: "p", URL: ok}, &key); err != nil {
			t.Errorf("%s with a key: %v", ok, err)
		}
	}
}

func ptr(s string) *string { return &s }

func TestKeyFileIsPrivateAndSeparate(t *testing.T) {
	st := NewStore(t.TempDir())
	key := secretKey
	if _, err := st.Put(Endpoint{Name: "a", URL: "https://x.example/v1"}, &key); err != nil {
		t.Fatal(err)
	}
	ep, _ := os.ReadFile(st.endpointsPath())
	if strings.Contains(string(ep), secretKey) {
		t.Fatalf("the key is in endpoints.json: %s", ep)
	}
	if runtime.GOOS != "windows" {
		fi, _ := os.Stat(st.keysPath())
		if fi.Mode().Perm() != 0o600 {
			t.Fatalf("keys.json mode %v", fi.Mode().Perm())
		}
	}
	list, _ := st.List()
	b, _ := json.Marshal(list)
	if strings.Contains(string(b), secretKey) || !strings.Contains(string(b), `"has_key":true`) {
		t.Fatalf("list: %s", b)
	}
}

func TestSpeakSavesTheLegacyLayout(t *testing.T) {
	var gotAuth atomic.Value
	var gotBody atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/audio/speech" || r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		gotAuth.Store(r.Header.Get("Authorization"))
		var b map[string]any
		_ = json.NewDecoder(r.Body).Decode(&b)
		gotBody.Store(b)
		speech(w, r)
	}))
	defer srv.Close()
	key := secretKey
	c, _, root := newClient(t, srv, &key)
	res, err := c.Speak(context.Background(), SpeechRequest{Endpoint: "main", Input: "Hello, world! This is a test."})
	if err != nil {
		t.Fatal(err)
	}
	if gotAuth.Load() != "Bearer "+secretKey {
		t.Fatalf("Authorization %v", gotAuth.Load())
	}
	body := gotBody.Load().(map[string]any)
	if body["model"] != "tts-1" || body["voice"] != "alloy" || body["response_format"] != "wav" || body["input"] != "Hello, world! This is a test." {
		t.Fatalf("body %v", body)
	}
	want := "outputs/2026-11-25_10-30-05__openai_main__Hello_world_Thi"
	if res.Folder != want {
		t.Fatalf("folder %q, want %q", res.Folder, want)
	}
	for _, p := range []string{res.Wav, res.PNG, res.JSON} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(p))); err != nil {
			t.Fatal(err)
		}
	}
	if res.Seconds < 0.49 || res.Seconds > 0.51 {
		t.Fatalf("seconds %v", res.Seconds)
	}
	// Nothing of the key or of a half-written folder is left.
	ents, _ := os.ReadDir(filepath.Join(root, "outputs"))
	if len(ents) != 1 {
		t.Fatalf("outputs holds %d entries", len(ents))
	}
	js, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(res.JSON)))
	if strings.Contains(string(js), secretKey) {
		t.Fatal("the key is in the metadata")
	}
	// The same second and text: a second folder, not an overwrite.
	res2, err := c.Speak(context.Background(), SpeechRequest{Endpoint: "main", Input: "Hello, world! This is a test."})
	if err != nil || res2.Folder != want+"_2" {
		t.Fatalf("second: %+v %v", res2, err)
	}
}

// TestLayoutMatchesALegacyGeneration reads a folder a real legacy generation
// wrote (testdata/legacy, a Chatterbox clip) and one Go wrote through the
// History package and checks both are listed and selected the same way.
func TestLayoutMatchesALegacyGeneration(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(speech))
	defer srv.Close()
	c, _, root := newClient(t, srv, nil)
	if err := copyTree(filepath.Join("testdata", "legacy", "outputs"), filepath.Join(root, "outputs")); err != nil {
		t.Fatal(err)
	}
	res, err := c.Speak(context.Background(), SpeechRequest{Endpoint: "main", Input: "Hi there"})
	if err != nil {
		t.Fatal(err)
	}
	h := history.New(history.Options{Root: root, Token: "t"})
	items, err := h.Items("outputs")
	if err != nil || len(items) != 2 {
		t.Fatalf("items %v %v", items, err)
	}
	rows, err := h.Rows("outputs")
	if err != nil || len(rows) != 2 {
		t.Fatalf("rows %v %v", rows, err)
	}
	// Newest first: today's folder, then the legacy one of 2026-04-07.
	if rows[0][3] != res.Wav || !strings.HasPrefix(rows[0][0], "2026-11-25T10:30:05") {
		t.Fatalf("our row: %v", rows[0])
	}
	if !strings.HasPrefix(rows[1][0], "2026-04-07T20:07:40") {
		t.Fatalf("legacy row: %v", rows[1])
	}
	if len(rows[0]) != len(rows[1]) {
		t.Fatalf("row shapes differ: %v %v", rows[0], rows[1])
	}
	legacy, err := h.Select(rows[1][3])
	if err != nil {
		t.Fatal(err)
	}
	ours, err := h.Select(rows[0][3])
	if err != nil {
		t.Fatal(err)
	}
	if len(legacy) != len(ours) {
		t.Fatalf("select keys differ: %v / %v", keys(legacy), keys(ours))
	}
	for k := range legacy {
		if _, ok := ours[k]; !ok {
			t.Fatalf("select has no %q: %v", k, keys(ours))
		}
	}
	// Both JSON files carry what the Outputs page and History need.
	lj, _ := os.ReadFile(filepath.Join(root, "outputs", "2026-04-07_20-07-40__chatterbox__hi_ma_na_is", "2026-04-07_20-07-40__chatterbox__hi_ma_na_is.json"))
	oj, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(res.JSON)))
	var lm, om map[string]any
	if json.Unmarshal(lj, &lm) != nil || json.Unmarshal(oj, &om) != nil {
		t.Fatal("metadata is not JSON")
	}
	for _, k := range []string{"_version", "_hash_version", "text", "outputs", "date", "hash"} {
		if _, ok := lm[k]; !ok {
			t.Fatalf("legacy has no %q", k)
		}
		if _, ok := om[k]; !ok {
			t.Fatalf("ours has no %q", k)
		}
	}
	if len(om["date"].(string)) != len(lm["date"].(string)) || len(om["hash"].(string)) != len(lm["hash"].(string)) {
		t.Fatalf("date/hash shapes differ: %v %v / %v %v", om["date"], om["hash"], lm["date"], lm["hash"])
	}
	// The same file names: <folder>/<folder>.{wav,json,png}.
	lf, _ := os.ReadDir(filepath.Join(root, "outputs", "2026-04-07_20-07-40__chatterbox__hi_ma_na_is"))
	of, _ := os.ReadDir(filepath.Join(root, filepath.FromSlash(res.Folder)))
	var lext, oext []string
	for _, e := range lf {
		lext = append(lext, filepath.Ext(e.Name()))
	}
	for _, e := range of {
		oext = append(oext, filepath.Ext(e.Name()))
	}
	if strings.Join(lext, ",") != strings.Join(oext, ",") {
		t.Fatalf("files differ: %v vs %v", lext, oext)
	}
}

func keys(m map[string]any) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}

func copyTree(from, to string) error {
	return filepath.Walk(from, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(from, p)
		dst := filepath.Join(to, rel)
		if fi.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(dst, b, 0o644)
	})
}

func TestPromptToTitleMatchesPython(t *testing.T) {
	for in, want := range map[string]string{
		"Hello, world! This is a test.": "Hello_world_Thi",
		"hi ma na is":                   "hi_ma_na_is",
		"a: b":                          "a_b",
		"openai_main":                   "openai_main",
		"x  y":                          "x_y", // "  " -> "__" -> "_"
	} {
		if got := promptToTitle(in); got != want {
			t.Errorf("%q: %q, want %q", in, got, want)
		}
	}
}

func TestClearErrors(t *testing.T) {
	for name, c := range map[string]struct {
		handler http.HandlerFunc
		want    string
	}{
		"wrong key": {func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(401)
			_, _ = w.Write([]byte(`{"error":{"message":"Incorrect API key provided: ` + secretKey + `"}}`))
		}, "refused the key (HTTP 401)"},
		"wrong URL": {func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) }, "HTTP 404 for"},
		"not wav": {func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "audio/mpeg")
			_, _ = w.Write([]byte("ID3\x03\x00\x00\x00\x00\x00\x00"))
		}, "not a WAV file"},
		"html page": {func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte("<html>Login</html>"))
		}, "text/html"},
		"redirect": {func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "https://elsewhere.example/v1/audio/speech", http.StatusTemporaryRedirect)
		}, "redirects"},
		"quota": {func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(429) }, "too many requests"},
		"server": {func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(500)
			_, _ = w.Write([]byte(`{"detail":"model not loaded"}`))
		}, "model not loaded"},
	} {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(c.handler)
			defer srv.Close()
			key := secretKey
			cl, _, root := newClient(t, srv, &key)
			_, err := cl.Speak(context.Background(), SpeechRequest{Endpoint: "main", Input: "hi"})
			var ue *UpstreamError
			if !errors.As(err, &ue) || !strings.Contains(ue.Msg, c.want) {
				t.Fatalf("got %v, want text %q", err, c.want)
			}
			if strings.Contains(ue.Msg, secretKey) {
				t.Fatalf("the key is in the message: %s", ue.Msg)
			}
			if _, err := os.Stat(filepath.Join(root, "outputs")); err == nil {
				t.Fatal("a failed call left an outputs folder")
			}
		})
	}
}

func TestUnreachableAndDNS(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(speech))
	cl, st, _ := newClient(t, srv, nil)
	srv.Close() // now nothing listens
	_, err := cl.Speak(context.Background(), SpeechRequest{Endpoint: "main", Input: "hi"})
	var ue *UpstreamError
	if !errors.As(err, &ue) || !strings.Contains(ue.Msg, "cannot reach") {
		t.Fatalf("got %v", err)
	}
	if _, err := st.Put(Endpoint{Name: "nohost", URL: "https://no-such-host.invalid/v1", Model: "m", Voice: "v"}, nil); err != nil {
		t.Fatal(err)
	}
	_, err = cl.Speak(context.Background(), SpeechRequest{Endpoint: "nohost", Input: "hi"})
	if !errors.As(err, &ue) || !strings.Contains(ue.Msg, "no-such-host.invalid") {
		t.Fatalf("got %v", err)
	}
}

func TestSpeakChecksInput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(speech))
	defer srv.Close()
	cl, st, _ := newClient(t, srv, nil)
	if _, err := cl.Speak(context.Background(), SpeechRequest{Endpoint: "main", Input: "  "}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("empty text: %v", err)
	}
	if _, err := cl.Speak(context.Background(), SpeechRequest{Endpoint: "nope", Input: "x"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown endpoint: %v", err)
	}
	if _, err := st.Put(Endpoint{Name: "bare", URL: srv.URL + "/v1"}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := cl.Speak(context.Background(), SpeechRequest{Endpoint: "bare", Input: "x"}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("no model and voice: %v", err)
	}
	if _, err := cl.Speak(context.Background(), SpeechRequest{Endpoint: "main", Input: "x", Speed: 9}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("speed: %v", err)
	}
}

func TestTestCall(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/audio/speech":
			if r.Header.Get("Authorization") != "Bearer "+secretKey {
				w.WriteHeader(401)
				return
			}
			speech(w, r)
		case "/v1/models":
			if r.Header.Get("Authorization") != "Bearer "+secretKey {
				w.WriteHeader(401)
				return
			}
			_, _ = w.Write([]byte(`{"data":[]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	key := secretKey
	cl, st, _ := newClient(t, srv, &key)
	if r, err := cl.Test(context.Background(), "main"); err != nil || !r.OK || !strings.Contains(r.Message, "speech request") {
		t.Fatalf("speech test: %+v %v", r, err)
	}
	if _, err := st.Put(Endpoint{Name: "main", URL: srv.URL + "/v1", Model: "m"}, nil); err != nil {
		t.Fatal(err)
	}
	if r, err := cl.Test(context.Background(), "main"); err != nil || !r.OK || !strings.Contains(r.Message, "model list") {
		t.Fatalf("models test: %+v %v", r, err)
	}
	bad := "wrong"
	if _, err := st.Put(Endpoint{Name: "main", URL: srv.URL + "/v1", Model: "m", Voice: "v"}, &bad); err != nil {
		t.Fatal(err)
	}
	if r, err := cl.Test(context.Background(), "main"); err != nil || r.OK || !strings.Contains(r.Message, "refused the key") {
		t.Fatalf("wrong key: %+v %v", r, err)
	}
}

func TestWavFormats(t *testing.T) {
	if _, err := parseWav([]byte("ID3")); !errors.Is(err, errNotWav) {
		t.Fatalf("%v", err)
	}
	// The real legacy file is IEEE float 32-bit with a fact chunk.
	b, err := os.ReadFile(filepath.Join("testdata", "legacy", "outputs", "2026-04-07_20-07-40__chatterbox__hi_ma_na_is", "2026-04-07_20-07-40__chatterbox__hi_ma_na_is.wav"))
	if err != nil {
		t.Fatal(err)
	}
	w, err := parseWav(b)
	if err != nil || w.format != 3 || w.rate != 24000 || w.seconds() < 1 {
		t.Fatalf("legacy wav: %+v %v", w, err)
	}
	if png, err := w.waveformPNG(); err != nil || len(png) < 100 || string(png[1:4]) != "PNG" {
		t.Fatalf("png: %v", err)
	}
	// A streamed WAV has size 0xFFFFFFFF in both headers.
	s := testWav()
	binary.LittleEndian.PutUint32(s[4:], 0xFFFFFFFF)
	binary.LittleEndian.PutUint32(s[40:], 0xFFFFFFFF)
	if w, err := parseWav(s); err != nil || w.frames() != 8000 {
		t.Fatalf("streamed: %+v %v", w, err)
	}
}

func TestPlatformIsAlwaysReadyAndRunsNothing(t *testing.T) {
	p := New()
	if !p.Status(context.Background()).Ready || p.Name() != "openai" {
		t.Fatal("not ready")
	}
	if err := p.Install(context.Background(), func(string) {}); !errors.Is(err, platform.ErrNotInstallable) {
		t.Fatal(err)
	}
	if _, err := p.Command(nil, []string{"python"}); err == nil {
		t.Fatal("a remote endpoint has no command")
	}
}

func TestControlAPIWantsTheToken(t *testing.T) {
	root := t.TempDir()
	h := NewHandler(NewClient(Options{Store: NewStore(root), Root: root}), "tok")
	for _, c := range []struct {
		tok, peer string
		want      int
	}{
		{"", "127.0.0.1:1", 403}, {"wrong", "127.0.0.1:1", 403}, {"tok", "192.168.1.2:1", 403}, {"tok", "127.0.0.1:1", 200},
	} {
		req := httptest.NewRequest("GET", "/api/openai/endpoints", nil)
		req.RemoteAddr = c.peer
		if c.tok != "" {
			req.Header.Set(TokenHeader, c.tok)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != c.want {
			t.Errorf("%+v: %d", c, rec.Code)
		}
	}
}
