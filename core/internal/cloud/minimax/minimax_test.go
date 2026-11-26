package minimax

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
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

	"github.com/rsxdalv/TTS-WebUI/core/internal/platform/openai"
	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/history"
)

const secretKey = "mm-SENTINEL-1234567890abcdef"

// testWav is a 0.5 s 440 Hz tone, 16-bit mono at 16 kHz.
func testWav() []byte {
	const rate, n = 16000, 8000
	data := make([]byte, n*2)
	for i := 0; i < n; i++ {
		v := int16(12000 * math.Sin(2*math.Pi*440*float64(i)/rate))
		binary.LittleEndian.PutUint16(data[i*2:], uint16(v))
	}
	return wavFile(data)
}

func wavFile(data []byte) []byte {
	h := make([]byte, 44)
	copy(h, "RIFF")
	binary.LittleEndian.PutUint32(h[4:], uint32(36+len(data)))
	copy(h[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(h[16:], 16)
	binary.LittleEndian.PutUint16(h[20:], 1)
	binary.LittleEndian.PutUint16(h[22:], 1)
	binary.LittleEndian.PutUint32(h[24:], 16000)
	binary.LittleEndian.PutUint32(h[28:], 32000)
	binary.LittleEndian.PutUint16(h[32:], 2)
	binary.LittleEndian.PutUint16(h[34:], 16)
	copy(h[36:], "data")
	binary.LittleEndian.PutUint32(h[40:], uint32(len(data)))
	return append(h, data...)
}

// okBody is a MiniMax success answer carrying audio as hex.
func okBody(audio []byte) string {
	b, _ := json.Marshal(map[string]any{
		"data":       map[string]any{"audio": hex.EncodeToString(audio), "status": 2},
		"extra_info": map[string]any{"audio_length": 500, "audio_format": "wav"},
		"trace_id":   "trace-123",
		"base_resp":  map[string]any{"status_code": 0, "status_msg": "success"},
	})
	return string(b)
}

// mock is a stand-in for api.minimax.io: it checks the method, path, headers and
// body of the call and answers with whatever the test chooses.
type mock struct {
	*httptest.Server
	calls   atomic.Int32
	lastReq atomic.Value // map[string]any
	answer  func(w http.ResponseWriter, r *http.Request)
}

func newMock(t *testing.T) *mock {
	t.Helper()
	m := &mock{}
	m.answer = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, okBody(testWav()))
	}
	m.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.calls.Add(1)
		if r.Method != http.MethodPost || r.URL.Path != "/v1/t2a_v2" {
			http.Error(w, "wrong route "+r.Method+" "+r.URL.Path, 404)
			return
		}
		if r.Header.Get("Content-Type") != "application/json" {
			http.Error(w, "content type", 400)
			return
		}
		var body map[string]any
		if json.NewDecoder(r.Body).Decode(&body) == nil {
			m.lastReq.Store(body)
		}
		if r.Header.Get("Authorization") != "Bearer "+secretKey {
			// MiniMax answers 200 with base_resp 1004; this one also echoes the key it was sent.
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"base_resp":{"status_code":1004,"status_msg":"login fail: bad key `+
				strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")+`"},"trace_id":"trace-bad"}`)
			return
		}
		m.answer(w, r)
	}))
	t.Cleanup(m.Close)
	return m
}

var fixedNow = func() time.Time { return time.Date(2026, 11, 26, 10, 30, 5, 123456000, time.Local) }

func newClient(t *testing.T, base string, key string, env map[string]string) (*Client, *Store, string) {
	t.Helper()
	root := t.TempDir()
	st := NewStore(root, func(k string) string { return env[k] })
	in := SettingsIn{BaseURL: base}
	if key != "" {
		in.Key = &key
	}
	if _, err := st.Put(in); err != nil {
		t.Fatal(err)
	}
	return NewClient(Options{Store: st, Root: root, Now: fixedNow}), st, root
}

func speak(c *Client, text string) (Result, error) {
	return c.Speak(context.Background(), SpeechRequest{Text: text})
}

func TestSpeakSendsTheDocumentedRequestAndSavesTheClip(t *testing.T) {
	m := newMock(t)
	c, _, root := newClient(t, m.URL, secretKey, nil)
	res, err := c.Speak(context.Background(), SpeechRequest{Text: "  Hello there  ", Model: "speech-2.8-turbo", VoiceID: "Deep_Voice_Man"})
	if err != nil {
		t.Fatal(err)
	}
	req := m.lastReq.Load().(map[string]any)
	if req["model"] != "speech-2.8-turbo" || req["text"] != "Hello there" {
		t.Fatalf("request %v", req)
	}
	if vs, _ := req["voice_setting"].(map[string]any); vs["voice_id"] != "Deep_Voice_Man" {
		t.Fatalf("voice_setting %v", req["voice_setting"])
	}
	if as, _ := req["audio_setting"].(map[string]any); as["format"] != "wav" {
		t.Fatalf("audio_setting %v", req["audio_setting"])
	}
	if res.Seconds < 0.49 || res.Seconds > 0.51 || res.Model != "speech-2.8-turbo" || res.VoiceID != "Deep_Voice_Man" {
		t.Fatalf("result %+v", res)
	}
	if want := "outputs/2026-11-26_10-30-05__minimax__Hello_there"; res.Folder != want {
		t.Fatalf("folder %q, want %q", res.Folder, want)
	}
	got, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(res.Wav)))
	if string(got) != string(testWav()) {
		t.Fatal("the saved wav is not the audio MiniMax sent")
	}
	for _, p := range []string{res.PNG, res.JSON} {
		if fi, err := os.Stat(filepath.Join(root, filepath.FromSlash(p))); err != nil || fi.Size() == 0 {
			t.Fatalf("%s: %v", p, err)
		}
	}
	var meta map[string]any
	b, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(res.JSON)))
	if err := json.Unmarshal(b, &meta); err != nil {
		t.Fatal(err)
	}
	for k, v := range map[string]string{"text": "Hello there", "model": "speech-2.8-turbo", "voice_id": "Deep_Voice_Man",
		"provider": "minimax", "_type": "minimax_cloud_tts", "_version": "0.1.0", "_hash_version": "0.0.2"} {
		if meta[k] != v {
			t.Errorf("metadata %s = %v, want %v", k, meta[k], v)
		}
	}
	if !strings.HasPrefix(meta["date"].(string), "2026-11-26 10:30:05.123456") || len(meta["hash"].(string)) != 64 {
		t.Errorf("date/hash: %v %v", meta["date"], meta["hash"])
	}
	if strings.Contains(string(b), secretKey) {
		t.Error("the key is in the metadata")
	}
}

func TestDefaultsComeFromTheSettings(t *testing.T) {
	m := newMock(t)
	c, _, _ := newClient(t, m.URL, secretKey, nil)
	if _, err := speak(c, "Hi"); err != nil {
		t.Fatal(err)
	}
	req := m.lastReq.Load().(map[string]any)
	if req["model"] != DefaultModel || req["voice_setting"].(map[string]any)["voice_id"] != DefaultVoice {
		t.Fatalf("request %v", req)
	}
}

// copyTree copies the legacy fixture of the openai platform.
func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dst, rel), b, 0o644)
	})
}

// A real legacy generation (a Chatterbox folder the openai platform keeps as a
// fixture), a clip written by this package and the folder the Python extension
// writes, in one root: History lists the first two with the same shapes and does
// not list the Python extension's (it has no .wav and no .png).
func TestLayoutMatchesALegacyGenerationAndPythonsFolderIsNotListed(t *testing.T) {
	m := newMock(t)
	c, _, root := newClient(t, m.URL, secretKey, nil)
	legacy := filepath.Join("..", "..", "platform", "openai", "testdata", "legacy", "outputs")
	if err := copyTree(legacy, filepath.Join(root, "outputs")); err != nil {
		t.Fatal(err)
	}
	// What extension_minimax_cloud_tts/main.py writes: outputs/<ts>_<text>/<text>.mp3 + .json.
	py := filepath.Join(root, "outputs", "20261125_101010_Hello_World")
	if err := os.MkdirAll(py, 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(py, "Hello_World.mp3"), []byte("ID3"), 0o644)
	_ = os.WriteFile(filepath.Join(py, "Hello_World.json"), []byte(`{"text":"Hello World","model":"speech-2.8-hd","voice_id":"x","provider":"minimax","_type":"minimax_cloud_tts","_version":"0.1.0"}`), 0o644)

	res, err := speak(c, "Hi there")
	if err != nil {
		t.Fatal(err)
	}
	h := history.New(history.Options{Root: root, Token: "t"})
	items, err := h.Items("outputs")
	if err != nil || len(items) != 2 {
		t.Fatalf("items %v %v (the Python folder must not be listed, ours and the legacy one must)", items, err)
	}
	rows, err := h.Rows("outputs")
	if err != nil {
		t.Fatal(err)
	}
	// Rows lists every folder (the Python one with an unknown date), newest first; ours is first.
	if rows[0][3] != res.Wav || !strings.HasPrefix(rows[0][0], "2026-11-26T10:30:05") || rows[0][1] != "Minimax hi_there" {
		t.Fatalf("our row: %v", rows[0])
	}
	var legacyRow []string
	for _, r := range rows {
		if strings.Contains(r[3], "chatterbox") {
			legacyRow = r
		}
	}
	if len(legacyRow) != len(rows[0]) {
		t.Fatalf("row shapes differ: %v %v", rows[0], legacyRow)
	}
	ours, err := h.Select(res.Wav)
	if err != nil {
		t.Fatal(err)
	}
	old, err := h.Select(legacyRow[3])
	if err != nil {
		t.Fatal(err)
	}
	if len(ours) != len(old) {
		t.Fatalf("select keys differ: %v / %v", ours, old)
	}
	for k := range old {
		if _, ok := ours[k]; !ok {
			t.Fatalf("select has no %q", k)
		}
	}
	// Metadata: the legacy fields History and the Outputs page read, plus the Python extension's own.
	var lm, om map[string]any
	lb, _ := os.ReadFile(filepath.Join(legacy, "2026-04-07_20-07-40__chatterbox__hi_ma_na_is", "2026-04-07_20-07-40__chatterbox__hi_ma_na_is.json"))
	ob, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(res.JSON)))
	if json.Unmarshal(lb, &lm) != nil || json.Unmarshal(ob, &om) != nil {
		t.Fatal("metadata is not JSON")
	}
	for k := range lm {
		if k == "text" || strings.HasPrefix(k, "_") || k == "outputs" || k == "date" || k == "hash" {
			if _, ok := om[k]; !ok {
				t.Errorf("ours has no %q", k)
			}
		}
	}
	for _, k := range []string{"model", "voice_id", "provider", "_type"} {
		if _, ok := om[k]; !ok {
			t.Errorf("ours has no %q (the Python extension writes it)", k)
		}
	}
	if len(om["date"].(string)) != len(lm["date"].(string)) || len(om["hash"].(string)) != len(lm["hash"].(string)) {
		t.Errorf("date/hash shapes differ")
	}
	// The same file names and extensions: <folder>/<folder>.{wav,json,png}.
	of, _ := os.ReadDir(filepath.Join(root, filepath.FromSlash(res.Folder)))
	name := filepath.Base(res.Folder)
	var got []string
	for _, e := range of {
		got = append(got, e.Name())
	}
	if strings.Join(got, ",") != name+".json,"+name+".png,"+name+".wav" {
		t.Errorf("files %v", got)
	}
	// No half folder is left behind.
	ents, _ := os.ReadDir(filepath.Join(root, "outputs"))
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), ".") {
			t.Errorf("left over %s", e.Name())
		}
	}
	// A second clip in the same second gets _2.
	res2, err := speak(c, "Hi there")
	if err != nil || res2.Folder != res.Folder+"_2" {
		t.Fatalf("second clip: %+v %v", res2, err)
	}
}

func TestWrongKey(t *testing.T) {
	m := newMock(t)
	c, st, root := newClient(t, m.URL, "mm-WRONG-key-123456", nil)
	for name, fn := range map[string]func() error{
		"speak": func() error { _, err := speak(c, "Hi"); return err },
		"test": func() error {
			r, err := c.Test(context.Background())
			if err == nil && r.OK {
				t.Error("test passed with a wrong key")
			}
			if err == nil && !strings.Contains(r.Message, "refused the key") {
				t.Errorf("test message %q", r.Message)
			}
			return err
		},
	} {
		err := fn()
		if name == "speak" {
			var ue *openai.UpstreamError
			if !errors.As(err, &ue) || !strings.Contains(ue.Msg, "refused the key (1004") || !strings.Contains(ue.Msg, "trace id trace-bad") {
				t.Fatalf("speak: %v", err)
			}
			if strings.Contains(ue.Msg, "mm-WRONG-key-123456") {
				t.Fatalf("the error carries the key: %s", ue.Msg)
			}
		}
	}
	if set, _ := st.Get(); !set.HasKey {
		t.Fatal("key lost")
	}
	// Nothing was saved.
	if _, err := os.Stat(filepath.Join(root, "outputs")); err == nil {
		t.Fatal("a clip was saved after a failed call")
	}
}

func TestHTTPStatusErrors(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{"401", 401, `{"msg":"nope"}`, "refused the key (HTTP 401)"},
		{"403", 403, `forbidden`, "refused the key (HTTP 403)"},
		{"404", 404, `not here`, "HTTP 404 for "},
		{"429", 429, ``, "too many requests"},
		{"500", 500, `<html>oops</html>`, "MiniMax failed (HTTP 500): <html>oops</html>"},
		{"502 with base_resp", 502, `{"base_resp":{"status_code":1000,"status_msg":"unknown error"}}`, "MiniMax error 1000: unknown error"},
		{"400", 400, `{"detail":"bad"}`, "refused the request (HTTP 400)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newMock(t)
			m.answer = func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				if strings.HasPrefix(tc.body, "{") {
					_, _ = io.WriteString(w, tc.body)
				} else {
					_, _ = io.WriteString(w, tc.body+" echo:"+secretKey)
				}
			}
			c, _, _ := newClient(t, m.URL, secretKey, nil)
			_, err := speak(c, "Hi")
			var ue *openai.UpstreamError
			if !errors.As(err, &ue) || !strings.Contains(ue.Msg, tc.want) {
				t.Fatalf("got %v, want %q", err, tc.want)
			}
			if strings.Contains(ue.Msg, secretKey) {
				t.Fatalf("the error carries the key: %s", ue.Msg)
			}
		})
	}
}

func TestBaseRespCodes(t *testing.T) {
	for code, want := range map[int]string{
		1002: "rate limit", 1039: "rate limit", 1001: "timed out", 1042: "invalid characters", 2013: "request parameters",
		1000: "MiniMax error 1000", 1004: "refused the key", 7777: "MiniMax error 7777",
	} {
		m := newMock(t)
		m.answer = func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, `{"base_resp":{"status_code":`+itoa(code)+`,"status_msg":"msg `+secretKey+`"},"trace_id":"tr"}`)
		}
		c, _, _ := newClient(t, m.URL, secretKey, nil)
		_, err := speak(c, "Hi")
		var ue *openai.UpstreamError
		if !errors.As(err, &ue) || !strings.Contains(ue.Msg, want) || strings.Contains(ue.Msg, secretKey) {
			t.Errorf("code %d: %v, want %q", code, err, want)
		}
	}
}

func itoa(n int) string { b, _ := json.Marshal(n); return string(b) }

func TestMalformedAnswers(t *testing.T) {
	mp3 := append([]byte("ID3\x04\x00"), make([]byte, 64)...)
	noAudio := wavFile(nil)
	cases := []struct {
		name   string
		answer string
		want   string
	}{
		{"not json", `<html>gateway</html>`, "not JSON"},
		{"truncated json", `{"data":{"audio":"4949`, "not JSON"},
		{"empty object", `{}`, "without audio data"},
		{"empty audio", `{"data":{"audio":""},"base_resp":{"status_code":0}}`, "without audio data"},
		{"null data", `{"data":null}`, "without audio data"},
		{"audio is a number", `{"data":{"audio":12}}`, "not JSON"},
		{"not hex", `{"data":{"audio":"zz11"}}`, "not valid hex"},
		{"odd hex", `{"data":{"audio":"abc"}}`, "not valid hex"},
		{"mp3 instead of wav", okBody(mp3), "looks like MP3"},
		{"unknown bytes", okBody([]byte("hello world, not audio")), "unknown format"},
		{"wav with no audio", okBody(noAudio), "no audio in it"},
		{"truncated wav header", okBody(testWav()[:30]), "cannot be read"},
		{"url instead of hex", `{"data":{"audio":"https://cdn.example/a.wav"}}`, "not valid hex"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newMock(t)
			m.answer = func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, tc.answer) }
			c, _, root := newClient(t, m.URL, secretKey, nil)
			_, err := speak(c, "Hi")
			var ue *openai.UpstreamError
			if !errors.As(err, &ue) || !strings.Contains(ue.Msg, tc.want) {
				t.Fatalf("got %v, want %q", err, tc.want)
			}
			if _, err := os.Stat(filepath.Join(root, "outputs")); err == nil {
				t.Fatal("a clip was saved from a malformed answer")
			}
		})
	}
}

func TestOversizedAnswer(t *testing.T) {
	m := newMock(t)
	m.answer = func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, okBody(testWav())) }
	c, st, root := newClient(t, m.URL, secretKey, nil)
	c = NewClient(Options{Store: st, Root: root, Now: fixedNow, MaxResponse: 1 << 20})
	// Under the cap: fine. The test wav is 16 KB, 32 KB as hex.
	if _, err := speak(c, "Hi"); err != nil {
		t.Fatal(err)
	}
	m.answer = func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"data":{"audio":"`)
		chunk := strings.Repeat("00", 1<<16)
		for i := 0; i < 20; i++ { // 2.6 MB of hex
			_, _ = io.WriteString(w, chunk)
		}
		_, _ = io.WriteString(w, `"}}`)
	}
	_, err := speak(c, "Hi again")
	var ue *openai.UpstreamError
	if !errors.As(err, &ue) || !strings.Contains(ue.Msg, "larger than 1 MB") {
		t.Fatalf("got %v", err)
	}
}

func TestRedirectIsReportedNotFollowed(t *testing.T) {
	var hits atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.Header.Get("Authorization") != "" {
			t.Error("the key followed a redirect")
		}
	}))
	defer target.Close()
	m := newMock(t)
	m.answer = func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/steal", http.StatusTemporaryRedirect)
	}
	c, _, _ := newClient(t, m.URL, secretKey, nil)
	_, err := speak(c, "Hi")
	var ue *openai.UpstreamError
	if !errors.As(err, &ue) || !strings.Contains(ue.Msg, "redirects") || hits.Load() != 0 {
		t.Fatalf("got %v, hits %d", err, hits.Load())
	}
}

func TestUnreachableAndCancelled(t *testing.T) {
	m := newMock(t)
	url := m.URL
	m.Close()
	c, _, _ := newClient(t, url, secretKey, nil)
	_, err := speak(c, "Hi")
	var ue *openai.UpstreamError
	if !errors.As(err, &ue) || !strings.Contains(ue.Msg, "cannot reach") || strings.Contains(ue.Msg, secretKey) {
		t.Fatalf("got %v", err)
	}
	// A call that hangs ends when the request is cancelled, with nothing saved.
	slow := newMock(t)
	slow.answer = func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }
	c, _, root := newClient(t, slow.URL, secretKey, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if _, err := c.Speak(ctx, SpeechRequest{Text: "Hi"}); err == nil {
		t.Fatal("no error")
	}
	if _, err := os.Stat(filepath.Join(root, "outputs")); err == nil {
		t.Fatal("saved after cancel")
	}
}

func TestInputRules(t *testing.T) {
	m := newMock(t)
	c, _, _ := newClient(t, m.URL, secretKey, nil)
	for name, req := range map[string]SpeechRequest{
		"empty":       {Text: ""},
		"blank":       {Text: " \n\t"},
		"too long":    {Text: strings.Repeat("a", 10000)},
		"model lines": {Text: "x", Model: "a\nb"},
		"voice long":  {Text: "x", VoiceID: strings.Repeat("v", 201)},
	} {
		if _, err := c.Speak(context.Background(), req); !errors.Is(err, openai.ErrInvalid) {
			t.Errorf("%s: want ErrInvalid, got %v", name, err)
		}
	}
	if _, err := speak(c, strings.Repeat("é", 9999)); err != nil {
		t.Errorf("9999 characters (not bytes) must pass: %v", err)
	}
	if m.calls.Load() != 1 {
		t.Errorf("a refused input reached MiniMax: %d calls", m.calls.Load())
	}
}

func TestNoKey(t *testing.T) {
	m := newMock(t)
	c, _, _ := newClient(t, m.URL, "", nil)
	if _, err := speak(c, "Hi"); !errors.Is(err, openai.ErrInvalid) || !strings.Contains(err.Error(), "no MiniMax API key") {
		t.Fatalf("got %v", err)
	}
	r, err := c.Test(context.Background())
	if err != nil || r.OK || !strings.Contains(r.Message, "no MiniMax API key") {
		t.Fatalf("test: %+v %v", r, err)
	}
	if m.calls.Load() != 0 {
		t.Fatal("a call was made without a key")
	}
}

func TestKeyFromTheEnvironmentAndStoredKeyWins(t *testing.T) {
	m := newMock(t)
	c, st, root := newClient(t, m.URL, "", map[string]string{EnvKey: " " + secretKey + " "})
	set, _ := st.Get()
	if !set.HasKey || set.KeySource != "env" {
		t.Fatalf("settings %+v", set)
	}
	if _, err := speak(c, "Hi"); err != nil {
		t.Fatalf("env key: %v", err)
	}
	// The env key is never copied to disk.
	filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".json") {
			if b, _ := os.ReadFile(p); strings.Contains(string(b), secretKey) {
				t.Errorf("the env key was written to %s", p)
			}
		}
		return nil
	})
	wrong := "mm-stored-wrong"
	if _, err := st.Put(SettingsIn{BaseURL: m.URL, Key: &wrong}); err != nil {
		t.Fatal(err)
	}
	if set, _ = st.Get(); set.KeySource != "stored" {
		t.Fatalf("settings %+v", set)
	}
	if _, err := speak(c, "Hi"); err == nil || !strings.Contains(err.Error(), "refused the key") {
		t.Fatalf("a stored key must win over the environment: %v", err)
	}
	empty := ""
	if _, err := st.Put(SettingsIn{BaseURL: m.URL, Key: &empty}); err != nil {
		t.Fatal(err)
	}
	if set, _ = st.Get(); set.KeySource != "env" {
		t.Fatalf("removing the stored key must fall back to the environment: %+v", set)
	}
}

func TestStoreRules(t *testing.T) {
	root := t.TempDir()
	st := NewStore(root, func(string) string { return "" })
	set, err := st.Get()
	if err != nil || set.BaseURL != DefaultBaseURL || set.Model != DefaultModel || set.Voice != DefaultVoice || set.HasKey || set.KeySource != "none" {
		t.Fatalf("defaults %+v %v", set, err)
	}
	key := secretKey
	set, err = st.Put(SettingsIn{BaseURL: " https://api.example.com/ ", Model: " m ", Voice: "v", Key: &key})
	if err != nil || set.BaseURL != "https://api.example.com" || set.Model != "m" || !set.HasKey || set.KeySource != "stored" {
		t.Fatalf("put %+v %v", set, err)
	}
	// Absent key keeps it; empty fields go back to the defaults.
	set, err = st.Put(SettingsIn{})
	if err != nil || !set.HasKey || set.BaseURL != DefaultBaseURL || set.Model != DefaultModel {
		t.Fatalf("put without key %+v %v", set, err)
	}
	if runtime.GOOS != "windows" {
		fi, err := os.Stat(filepath.Join(root, "platforms", "minimax", "key.json"))
		if err != nil || fi.Mode().Perm() != 0o600 {
			t.Fatalf("key file: %v %v", fi, err)
		}
	}
	// Only key.json holds the key.
	filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && filepath.Base(p) != "key.json" {
			if b, _ := os.ReadFile(p); strings.Contains(string(b), secretKey) {
				t.Errorf("the key is in %s", p)
			}
		}
		return nil
	})
	bad := map[string]SettingsIn{
		"public http with a key": {BaseURL: "http://api.example.com"},
		"ftp":                    {BaseURL: "ftp://x"},
		"no scheme":              {BaseURL: "api.minimax.io"},
		"user in URL":            {BaseURL: "https://u:p@api.minimax.io"},
		"query":                  {BaseURL: "https://api.minimax.io?x=1"},
		"ends in /v1":            {BaseURL: "https://api.minimax.io/v1"},
		"model newline":          {Model: "a\nb"},
		"voice too long":         {Voice: strings.Repeat("v", 201)},
		"key newline":            {Key: ptr("a\nb")},
		"key too long":           {Key: ptr(strings.Repeat("k", 5000))},
	}
	for name, in := range bad {
		if _, err := st.Put(in); !errors.Is(err, openai.ErrInvalid) {
			t.Errorf("%s: want ErrInvalid, got %v", name, err)
		}
	}
	// Plain http is fine for this machine and the private network, with a key.
	for _, u := range []string{"http://127.0.0.1:9", "http://localhost:9", "http://192.168.1.5:9"} {
		if _, err := st.Put(SettingsIn{BaseURL: u}); err != nil {
			t.Errorf("%s: %v", u, err)
		}
	}
	// The environment key counts too: no public http for it either.
	envSt := NewStore(t.TempDir(), func(string) string { return "k" })
	if _, err := envSt.Put(SettingsIn{BaseURL: "http://api.example.com"}); !errors.Is(err, openai.ErrInvalid) {
		t.Errorf("env key over public http: %v", err)
	}
	// A refused change leaves the files as they were.
	if set, _ = st.Get(); set.BaseURL != "http://192.168.1.5:9" {
		t.Errorf("settings after refusals: %+v", set)
	}
}

func TestBrokenFilesAreReported(t *testing.T) {
	root := t.TempDir()
	st := NewStore(root, nil)
	_ = os.MkdirAll(filepath.Join(root, "platforms", "minimax"), 0o755)
	_ = os.WriteFile(filepath.Join(root, "platforms", "minimax", "settings.json"), []byte("{not json"), 0o600)
	if _, err := st.Get(); err == nil || !strings.Contains(err.Error(), "settings.json is not valid JSON") {
		t.Fatalf("got %v", err)
	}
}

func ptr(s string) *string { return &s }

func TestTestCallSavesNothing(t *testing.T) {
	m := newMock(t)
	c, _, root := newClient(t, m.URL, secretKey, nil)
	r, err := c.Test(context.Background())
	if err != nil || !r.OK || !strings.Contains(r.Message, "0.5 s") {
		t.Fatalf("test %+v %v", r, err)
	}
	if _, err := os.Stat(filepath.Join(root, "outputs")); err == nil {
		t.Fatal("the test call saved a clip")
	}
}

func TestLogsNeverCarryTheKey(t *testing.T) {
	var logs strings.Builder
	m := newMock(t)
	m.answer = func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"base_resp":{"status_code":1004,"status_msg":"`+secretKey+`"}}`)
	}
	root := t.TempDir()
	st := NewStore(root, nil)
	key := secretKey
	if _, err := st.Put(SettingsIn{BaseURL: m.URL, Key: &key}); err != nil {
		t.Fatal(err)
	}
	c := NewClient(Options{Store: st, Root: root, Logf: func(f string, a ...any) {
		logs.WriteString(strings.TrimSpace(strings.ReplaceAll(f, "%s", "")) + " ")
		for _, x := range a {
			if s, ok := x.(string); ok {
				logs.WriteString(s + " ")
			}
			if e, ok := x.(error); ok {
				logs.WriteString(e.Error() + " ")
			}
		}
	}})
	_, _ = speak(c, "Hi")
	_, _ = c.Test(context.Background())
	if logs.Len() == 0 {
		t.Fatal("nothing was logged")
	}
	if strings.Contains(logs.String(), secretKey) {
		t.Fatalf("the log carries the key: %s", logs.String())
	}
}
