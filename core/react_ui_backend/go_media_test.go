package react_ui_backend

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func postJSON(h http.Handler, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("POST", "/x", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestMiniMaxTTS(t *testing.T) {
	const key = "sk-test-key-777"
	var gotAuth, gotBody string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		b := make([]byte, 4096)
		n, _ := r.Body.Read(b)
		gotBody = string(b[:n])
		if strings.Contains(gotBody, `"text":"fail"`) {
			_, _ = w.Write([]byte(`{"base_resp":{"status_code":1004,"status_msg":"bad key sk-test-key-777"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"base_resp":{"status_code":0},"data":{"audio":"` + hex.EncodeToString([]byte("ID3mp3")) + `"}}`))
	}))
	defer api.Close()
	old := minimaxURL
	minimaxURL = api.URL
	defer func() { minimaxURL = old }()

	root := t.TempDir()
	h := newMiniMaxTTS(Deps{Root: root})

	if w := postJSON(h, `{"text":"hi","model":"m","voice_id":"v"}`); w.Code != 400 {
		t.Fatalf("no key: %d", w.Code)
	}
	w := postJSON(h, `{"text":"Hello, wörld! con","model":"speech-2.8-hd","voice_id":"v","api_key":"`+key+`","seed":3,"use_random_seed":true}`)
	if w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if gotAuth != "Bearer "+key || !strings.Contains(gotBody, `"voice_id":"v"`) {
		t.Fatalf("request: %q %q", gotAuth, gotBody)
	}
	if strings.Contains(w.Body.String(), key) {
		t.Fatal("the key is in the answer")
	}
	var res struct {
		Audio    struct{ URL, Name string } `json:"audio"`
		Metadata map[string]any             `json:"metadata"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(res.Audio.URL, "/api/webui-generations/outputs/") || !strings.HasSuffix(res.Audio.URL, "/Hello_wörld_con.mp3") && !strings.Contains(res.Audio.URL, "Hello_w") {
		t.Fatalf("url %s", res.Audio.URL)
	}
	files, _ := filepath.Glob(filepath.Join(root, "outputs", "*", "*"))
	if len(files) != 2 {
		t.Fatalf("files: %v", files)
	}
	for _, f := range files {
		b, _ := os.ReadFile(f)
		if strings.Contains(string(b), key) {
			t.Fatal("the key was written to disk")
		}
		if strings.HasSuffix(f, ".json") && !strings.Contains(string(b), `"_type": "minimax_cloud_tts"`) {
			t.Fatalf("metadata: %s", b)
		}
	}
	if res.Metadata["provider"] != "minimax" {
		t.Fatalf("metadata %v", res.Metadata)
	}
	// An error from MiniMax is passed on without the key.
	w = postJSON(h, `{"text":"fail","model":"m","voice_id":"v","api_key":"`+key+`"}`)
	if w.Code != 502 || strings.Contains(w.Body.String(), key) {
		t.Fatalf("api error: %d %s", w.Code, w.Body)
	}
	// Same second, same text: a second folder, nothing overwritten.
	postJSON(h, `{"text":"Hello, wörld! con","model":"speech-2.8-hd","voice_id":"v","api_key":"`+key+`"}`)
	postJSON(h, `{"text":"Hello, wörld! con","model":"speech-2.8-hd","voice_id":"v","api_key":"`+key+`"}`)
	if dirs, _ := filepath.Glob(filepath.Join(root, "outputs", "*")); len(dirs) < 2 {
		t.Fatalf("dirs: %v", dirs)
	}
}

func TestSafeTextName(t *testing.T) {
	for in, want := range map[string]string{"Hello, world!": "Hello_world", "!!!": "minimax_tts", "con": "minimax_tts", "a/b\\c": "abc"} {
		if got := safeTextName(in); got != want {
			t.Errorf("%q -> %q, want %q", in, got, want)
		}
	}
}

func TestFFmpegMetadata(t *testing.T) {
	root := t.TempDir()
	cache := filepath.Join(root, "react-ui", "public", "file-input-cache")
	_ = os.MkdirAll(cache, 0o755)
	_ = os.WriteFile(filepath.Join(cache, "a.flac"), []byte("x"), 0o644)
	_ = os.WriteFile(filepath.Join(cache, "b.wav"), []byte("x"), 0o644)
	_ = os.WriteFile(filepath.Join(root, "secret.flac"), []byte("x"), 0o644)
	old := ffprobeRun
	defer func() { ffprobeRun = old }()
	ffprobeRun = func(_ context.Context, _, file string) ([]byte, error) {
		if strings.HasSuffix(file, "a.flac") {
			return []byte(`{"format":{"format_name":"flac","tags":{"COMMENT":"{\"text\":\"hi\"}"}},"streams":[]}`), nil
		}
		return []byte(`{"format":{"format_name":"wav"},"streams":[]}`), nil
	}
	h := newFFmpegMetadata(Deps{Root: root})
	if w := postJSON(h, `{"name":"a.flac"}`); w.Code != 200 || strings.TrimSpace(w.Body.String()) != `{"text":"hi"}` {
		t.Fatalf("flac: %d %s", w.Code, w.Body)
	}
	if w := postJSON(h, `{"name":"b.wav"}`); w.Code != 200 || !strings.Contains(w.Body.String(), `"format_name":"wav"`) {
		t.Fatalf("wav: %d %s", w.Code, w.Body)
	}
	for _, bad := range []string{`{"name":"../secret.flac"}`, `{"name":"..\\secret.flac"}`, `{"name":"C:\\x.wav"}`, `{"name":"nope.wav"}`, `{"name":"x.txt"}`, `{"name":".a.wav"}`, `{}`, `{"name":"a.flac","x":1}`} {
		if w := postJSON(h, bad); w.Code < 400 || strings.Contains(w.Body.String(), "format_name") {
			t.Errorf("%s: %d", bad, w.Code)
		}
	}
	ffprobeRun = func(context.Context, string, string) ([]byte, error) { return nil, errNoFFprobe }
	if w := postJSON(h, `{"name":"b.wav"}`); w.Code != 503 {
		t.Fatalf("no ffprobe: %d", w.Code)
	}
	ogg := `{"format":{"format_name":"ogg"},"streams":[{"tags":{"comment":"{\"a\":1}"}}]}`
	if out, err := commentMetadata([]byte(ogg), "z.ogg"); err != nil || string(out) != `{"a":1}` {
		t.Fatalf("ogg: %s %v", out, err)
	}
}
