package audiocpp

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform"
)

// TestMain makes the test binary a stand-in for audiocpp_server when it is run
// with AUDIOCPP_FAKE=1: it reads --config, serves /health and a streamed
// /v1/audio/speech, and is busy while a stream is open (503 for a request that
// brings busy_timeout_ms). AUDIOCPP_FAKE_MODE=stubborn makes it ignore a closed
// stream and stay busy for 30 s, like a non-streaming model does.
func TestMain(m *testing.M) {
	if os.Getenv("AUDIOCPP_FAKE") == "1" {
		fakeServer()
		return
	}
	os.Exit(m.Run())
}

func fakeServer() {
	var conf struct {
		Port   int `json:"port"`
		Models []struct {
			ID, Path, Mode string
		} `json:"models"`
	}
	for i, a := range os.Args {
		if a == "--config" {
			b, _ := os.ReadFile(os.Args[i+1])
			_ = json.Unmarshal(b, &conf)
		}
	}
	var busy atomic.Bool
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{"status":"ok"}`) })
	mux.HandleFunc("/v1/audio/speech", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Model, Input, Voice string
			BusyTimeoutMS       int `json:"busy_timeout_ms"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if busy.Load() {
			wait := time.Duration(req.BusyTimeoutMS) * time.Millisecond
			time.Sleep(wait)
			if busy.Load() {
				w.WriteHeader(http.StatusServiceUnavailable)
				fmt.Fprint(w, `{"error":{"code":"server_busy"}}`)
				return
			}
		}
		if req.Voice == "broken" {
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, "data: {\"type\":\"error\",\"error\":{\"message\":\"failed to open safetensors file\"}}\n\n")
			return
		}
		busy.Store(true)
		defer busy.Store(false)
		w.Header().Set("Content-Type", "text/event-stream")
		fl := w.(http.Flusher)
		n := 5
		if strings.Contains(req.Input, "long") {
			n = 100000
		}
		pcm := make([]byte, 3840)
		for i := range pcm {
			pcm[i] = byte(i)
		}
		enc := base64.StdEncoding.EncodeToString(pcm)
		stubborn := os.Getenv("AUDIOCPP_FAKE_MODE") == "stubborn"
		deadline := time.Now().Add(30 * time.Second)
		for i := 0; i < n; i++ {
			fmt.Fprintf(w, "data: {\"type\":\"speech.audio.delta\",\"audio\":%q}\n\n", enc)
			fl.Flush()
			time.Sleep(10 * time.Millisecond)
			if r.Context().Err() != nil {
				if !stubborn {
					return
				}
				for time.Now().Before(deadline) {
					time.Sleep(50 * time.Millisecond)
				}
				return
			}
		}
		fmt.Fprint(w, "data: {\"type\":\"speech.audio.done\"}\n\ndata: [DONE]\n\n")
	})
	srv := &http.Server{Addr: "127.0.0.1:" + strconv.Itoa(conf.Port), Handler: mux}
	if err := srv.ListenAndServe(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// serverFixture is an install folder with the fake server as audiocpp_server and
// one installed model.
func serverFixture(t *testing.T, mode string) (*Server, Model) {
	t.Helper()
	root, _ := paths.New(t.TempDir())
	exe, _ := os.Executable()
	m := Model{Package: "audiocpp_fake", ID: "fake", Dir: "fake", GGUF: "m.gguf", Family: "pocket_tts", SampleRate: 24000,
		Voices: []string{"alba", "broken"}, DefaultVoice: "alba", Files: []File{{"m.gguf", "https://x/m.gguf", strings.Repeat("a", 64), 3}}}
	p := New(Options{Root: root, Models: []Model{m}})
	if err := os.MkdirAll(p.BinDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(exe)
	if err := os.WriteFile(p.ServerPath(), b, 0o755); err != nil {
		t.Fatal(err)
	}
	st, _ := json.Marshal(State{Name: Name, Version: Version, Row: RowCPU})
	os.WriteFile(p.stateFile(), st, 0o644)
	dir := p.ModelDir(m)
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "m.gguf"), []byte("abc"), 0o644)
	os.WriteFile(filepath.Join(dir, ModelFile), []byte("{}"), 0o644)
	t.Setenv("AUDIOCPP_FAKE", "1")
	t.Setenv("AUDIOCPP_FAKE_MODE", mode)
	s := &Server{root: root, Models: []Model{m}, ReadyTimeout: 20 * time.Second, SettleTimeout: 2 * time.Second}
	regMu.Lock()
	registry[root.Dir()] = s
	regMu.Unlock()
	t.Cleanup(func() {
		s.Stop()
		regMu.Lock()
		delete(registry, root.Dir())
		regMu.Unlock()
	})
	return s, m
}

func TestSpeakReturnsAWavOfWhatTheServerStreamed(t *testing.T) {
	s, _ := serverFixture(t, "")
	clip, err := s.Speak(context.Background(), SpeakRequest{Model: "fake", Input: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if got := binary.LittleEndian.Uint32(clip.WAV[24:28]); got != 24000 {
		t.Errorf("sample rate %d", got)
	}
	if want := 5 * 1920.0 / 24000; clip.Seconds < want-1e-6 || clip.Seconds > want+1e-6 {
		t.Errorf("seconds = %v, want %v", clip.Seconds, want)
	}
	if string(clip.WAV[:4]) != "RIFF" || len(clip.WAV) != 44+5*3840 {
		t.Errorf("wav: %q %d bytes", clip.WAV[:4], len(clip.WAV))
	}
	if in := s.Info(); !in.Running || len(in.Models) != 1 || in.Models[0] != "fake" {
		t.Errorf("info = %+v", in)
	}
}

func TestSpeakRefusesWhatItCanTell(t *testing.T) {
	s, _ := serverFixture(t, "")
	for _, c := range []struct {
		req  SpeakRequest
		want string
	}{
		{SpeakRequest{Model: "nope", Input: "x"}, "not an audio.cpp model"},
		{SpeakRequest{Model: "fake", Input: "  "}, "text is empty"},
		{SpeakRequest{Model: "fake", Input: "x", Voice: "zed"}, `no voice "zed"`},
		{SpeakRequest{Model: "fake", Input: "x", Voice: "broken"}, "failed to open safetensors"},
	} {
		if _, err := s.Speak(context.Background(), c.req); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%+v: err = %v, want %q", c.req, err, c.want)
		}
	}
}

func TestNoModelOrNoPlatformIsSaidBeforeAnythingStarts(t *testing.T) {
	root, _ := paths.New(t.TempDir())
	s := &Server{root: root, Models: []Model{{Package: "p", ID: "fake", Voices: []string{"a"}, DefaultVoice: "a"}}}
	if _, err := s.Speak(context.Background(), SpeakRequest{Model: "fake", Input: "x"}); err == nil || !strings.Contains(err.Error(), "not installed") {
		t.Errorf("err = %v", err)
	}
	if s.Info().Running {
		t.Error("something started")
	}
}

func TestCancelStopsTheRunAndTheNextOneWorksOnTheSameServer(t *testing.T) {
	s, _ := serverFixture(t, "")
	res := make(chan error, 1)
	go func() {
		_, err := s.Speak(context.Background(), SpeakRequest{Model: "fake", Input: "long text"})
		res <- err
	}()
	waitFor(t, func() bool { return s.Info().Busy && s.Info().Running })
	time.Sleep(300 * time.Millisecond) // the stream is flowing
	pid := s.Info().Pid
	cr := s.Cancel()
	if !cr.Cancelled || cr.ServerStopped {
		t.Fatalf("cancel = %+v", cr)
	}
	if err := <-res; !errors.Is(err, ErrCancelled) {
		t.Fatalf("Speak = %v", err)
	}
	if s.Info().Busy {
		t.Error("still busy after the cancel")
	}
	if _, err := s.Speak(context.Background(), SpeakRequest{Model: "fake", Input: "short"}); err != nil {
		t.Fatal(err)
	}
	if s.Info().Pid != pid {
		t.Error("the server was restarted: the stream's close should have been enough")
	}
}

func TestCancelStopsTheServerWhenTheRunDoesNotStop(t *testing.T) {
	s, _ := serverFixture(t, "stubborn")
	res := make(chan error, 1)
	go func() {
		_, err := s.Speak(context.Background(), SpeakRequest{Model: "fake", Input: "long text"})
		res <- err
	}()
	waitFor(t, func() bool { return s.Info().Busy && s.Info().Running })
	time.Sleep(300 * time.Millisecond)
	pid := s.Info().Pid
	cr := s.Cancel()
	if !cr.Cancelled || !cr.ServerStopped {
		t.Fatalf("cancel = %+v", cr)
	}
	if err := <-res; !errors.Is(err, ErrCancelled) {
		t.Fatalf("Speak = %v", err)
	}
	if s.Info().Running {
		t.Error("the server is still running")
	}
	if _, err := s.Speak(context.Background(), SpeakRequest{Model: "fake", Input: "short"}); err != nil {
		t.Fatal(err)
	}
	if s.Info().Pid == pid {
		t.Error("the next generation did not start a new server")
	}
}

func TestASecondGenerationAtOnceIsRefusedAndCancelWithNothingRunningSaysSo(t *testing.T) {
	s, _ := serverFixture(t, "")
	if cr := s.Cancel(); cr.Cancelled {
		t.Errorf("cancel with nothing running: %+v", cr)
	}
	go s.Speak(context.Background(), SpeakRequest{Model: "fake", Input: "long text"})
	waitFor(t, func() bool { return s.Info().Busy })
	if _, err := s.Speak(context.Background(), SpeakRequest{Model: "fake", Input: "x"}); !errors.Is(err, ErrBusy) {
		t.Errorf("err = %v", err)
	}
	s.Cancel()
}

func TestRemovingAModelStopsTheServerFirst(t *testing.T) {
	s, m := serverFixture(t, "")
	if _, err := s.Speak(context.Background(), SpeakRequest{Model: "fake", Input: "x"}); err != nil {
		t.Fatal(err)
	}
	if !s.Info().Running {
		t.Fatal("the server is not running")
	}
	p := New(Options{Root: s.root, Models: []Model{m}})
	if !p.Remove(context.Background(), platform.Entry{Package: m.Package}, func(string) {}) {
		t.Fatal("Remove failed")
	}
	if s.Info().Running {
		t.Error("the server still runs after the model was removed")
	}
	if _, err := os.Stat(p.ModelDir(m)); err == nil {
		t.Error("the model folder is still there")
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	for i := 0; i < 200; i++ {
		if cond() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("condition not met in 10 s")
}
