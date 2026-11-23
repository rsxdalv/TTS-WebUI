package react_ui_backend

import (
	"context"
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

// Reserved device names are a Windows rule; elsewhere "con" is a fine name.
