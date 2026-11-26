package onnx

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rsxdalv/TTS-WebUI/core/internal/platform"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform/openai"
)

func setup(t *testing.T) (*fixture, *Handler) {
	f := newFixture(t)
	emit, _ := collect()
	ctx := context.Background()
	if err := f.p.Install(ctx, emit); err != nil || !f.p.Create(ctx, platform.Entry{Package: "tts.fake"}, emit) {
		t.Fatal("setup")
	}
	return f, NewHandler(f.p, "tok", nil)
}

func call(h http.Handler, method, path, body string, token string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.RemoteAddr = "127.0.0.1:5555"
	r.Header.Set("Content-Type", "application/json")
	if token != "" {
		r.Header.Set(openai.TokenHeader, token)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestHandlerNeedsToken(t *testing.T) {
	_, h := setup(t)
	if w := call(h, "GET", "/api/onnx/engines", "", ""); w.Code != 403 {
		t.Fatalf("no token: %d", w.Code)
	}
	if w := call(h, "GET", "/api/onnx/engines", "", "wrong"); w.Code != 403 {
		t.Fatalf("wrong token: %d", w.Code)
	}
	r := httptest.NewRequest("GET", "/api/onnx/engines", nil)
	r.RemoteAddr = "192.168.1.5:1"
	r.Header.Set(openai.TokenHeader, "tok")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatalf("remote peer: %d", w.Code)
	}
}

func TestHandlerEnginesAndSpeechSavesClip(t *testing.T) {
	f, h := setup(t)
	w := call(h, "GET", "/api/onnx/engines", "", "tok")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"installed":true`) {
		t.Fatalf("engines: %d %s", w.Code, w.Body)
	}
	w = call(h, "POST", "/api/onnx/speech", `{"package":"tts.fake","input":"hello there","voice":2,"speed":1}`, "tok")
	if w.Code != 200 {
		t.Fatalf("speech: %d %s", w.Code, w.Body)
	}
	if !strings.Contains(w.Body.String(), `"wav":"outputs/`) {
		t.Fatal(w.Body)
	}
	dirs, _ := filepath.Glob(filepath.Join(f.p.root.Dir(), "outputs", "*", "*.wav"))
	if len(dirs) != 1 {
		t.Fatalf("want one clip in outputs/, got %v", dirs)
	}
	for _, ext := range []string{".json", ".png"} {
		if _, err := os.Stat(strings.TrimSuffix(dirs[0], ".wav") + ext); err != nil {
			t.Fatal(err)
		}
	}
	if w := call(h, "POST", "/api/onnx/speech", `{"package":"tts.fake","input":"--help"}`, "tok"); w.Code != 400 {
		t.Fatalf("dash text: %d", w.Code)
	}
	if w := call(h, "POST", "/api/onnx/speech", `{"package":"tts.fake","input":"x","extra":1}`, "tok"); w.Code != 400 {
		t.Fatalf("unknown field: %d", w.Code)
	}
}

func TestHandlerCancelSavesNothing(t *testing.T) {
	f, h := setup(t)
	ctx, cancel := context.WithCancel(context.Background())
	r := httptest.NewRequest("POST", "/api/onnx/speech", strings.NewReader(`{"package":"tts.fake","input":"slow"}`)).WithContext(ctx)
	r.RemoteAddr = "127.0.0.1:1"
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set(openai.TokenHeader, "tok")
	w := httptest.NewRecorder()
	done := make(chan struct{})
	go func() { h.ServeHTTP(w, r); close(done) }()
	time.Sleep(700 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("request did not end after cancel")
	}
	if w.Code == 200 {
		t.Fatal("cancelled request answered 200")
	}
	if m, _ := filepath.Glob(filepath.Join(f.p.root.Dir(), "outputs", "*")); len(m) != 0 {
		t.Fatalf("something was saved: %v", m)
	}
}
