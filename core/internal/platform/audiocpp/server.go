package audiocpp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform"
	"github.com/rsxdalv/TTS-WebUI/core/internal/procs"
)

// The server is one process per install folder, shared by every launcher object
// that asks for it. It serves every installed model; when the set of installed
// models changes it is restarted with a new config on the next generation.

var (
	regMu    sync.Mutex
	registry = map[string]*Server{}
)

// ServerFor is the server of an install folder.
func ServerFor(root paths.Root) *Server {
	regMu.Lock()
	defer regMu.Unlock()
	s := registry[root.Dir()]
	if s == nil {
		s = &Server{root: root}
		registry[root.Dir()] = s
	}
	return s
}

// ErrBusy is the answer to a generation asked while another one runs: the
// server has one slot.
var ErrBusy = errors.New("the audio.cpp server is busy with another generation")

// ErrCancelled is what a cancelled generation returns.
var ErrCancelled = errors.New("the generation was cancelled")

// Server supervises audiocpp_server.
type Server struct {
	root paths.Root

	mu     sync.Mutex
	proc   *procs.Proc
	port   int
	models string // the sorted ids the running config holds
	// gen is the running generation, nil when there is none.
	gen *generation

	// ReadyTimeout and SettleTimeout default to 90 s and 8 s (tests shorten them).
	ReadyTimeout  time.Duration
	SettleTimeout time.Duration
	// Client replaces the HTTP client and Models the pinned model list (tests).
	Client *http.Client
	Models []Model
}

type generation struct {
	cancel  context.CancelFunc
	settled chan struct{}
	// killed is set by settle: true when the server did not go idle and was stopped.
	killed bool
}

// Info is the server's state.
type Info struct {
	Running bool     `json:"running"`
	Port    int      `json:"port,omitempty"`
	Pid     int      `json:"pid,omitempty"`
	Models  []string `json:"models"`
	Busy    bool     `json:"busy"`
}

// Info says whether the server is up and which models it serves.
func (s *Server) Info() Info {
	s.mu.Lock()
	defer s.mu.Unlock()
	in := Info{Models: []string{}, Busy: s.gen != nil}
	if s.proc != nil && s.proc.Running() {
		in.Running, in.Port, in.Pid = true, s.port, s.proc.Pid()
		if s.models != "" {
			in.Models = strings.Split(s.models, ",")
		}
	}
	return in
}

// Lines are the server's recent output lines.
func (s *Server) Lines() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.proc == nil {
		return nil
	}
	return s.proc.Lines()
}

func (s *Server) platform() *Platform { return New(Options{Root: s.root, Models: s.Models}) }

func (s *Server) modelByID(id string) (Model, bool) {
	for _, m := range s.platform().models {
		if m.ID == id {
			return m, true
		}
	}
	return Model{}, false
}

func (s *Server) client() *http.Client {
	if s.Client != nil {
		return s.Client
	}
	return http.DefaultClient
}

// installedModels are the models whose files are all there.
func (p *Platform) installedModels(ctx context.Context) []Model {
	var out []Model
	for _, m := range p.models {
		if p.EntryStatus(ctx, platform.Entry{Package: m.Package}).Installed {
			out = append(out, m)
		}
	}
	return out
}

type modelConf struct {
	ID          string         `json:"id"`
	Family      string         `json:"family"`
	Path        string         `json:"path"`
	Task        string         `json:"task"`
	Mode        string         `json:"mode"`
	LoadOptions map[string]any `json:"load_options,omitempty"`
	Preset      map[string]any `json:"default_voice_preset,omitempty"`
}

// config is the server.json for these models: loopback only, one model loaded
// at a time, streaming mode (the only mode a client disconnect can stop).
func (p *Platform) config(models []Model, port int) ([]byte, error) {
	backend := "cpu"
	if p.RowName() == RowVulkan {
		backend = "vulkan"
	}
	threads := runtime.NumCPU() / 2
	if threads < 2 {
		threads = 2
	}
	if threads > 8 {
		threads = 8
	}
	conf := map[string]any{
		"host": "127.0.0.1", "port": port, "backend": backend, "threads": threads,
		"lazy_load": true, "max_loaded_models": 1, "log_request_body": false,
		"max_request_body_bytes": 1 << 20,
	}
	var ms []modelConf
	for _, m := range models {
		ms = append(ms, modelConf{ID: m.ID, Family: m.Family, Path: filepath.ToSlash(filepath.Join(p.ModelDir(m), m.GGUF)),
			Task: "tts", Mode: "streaming", LoadOptions: m.LoadOptions,
			Preset: map[string]any{"voice_id": m.DefaultVoice}})
	}
	conf["models"] = ms
	return json.MarshalIndent(conf, "", "  ")
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// ensure starts the server (or restarts it when the installed models changed)
// and returns its base URL.
func (s *Server) ensure(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.platform()
	if st := p.Status(ctx); !st.Ready {
		return "", errors.New("the audiocpp platform is not installed: " + st.Detail)
	}
	models := p.installedModels(ctx)
	if len(models) == 0 {
		return "", errors.New("no audio.cpp model is installed: install one from the Extensions page")
	}
	var ids []string
	for _, m := range models {
		ids = append(ids, m.ID)
	}
	key := strings.Join(ids, ",")
	if s.proc != nil && s.proc.Running() && s.models == key {
		return "http://127.0.0.1:" + strconv.Itoa(s.port), nil
	}
	if s.proc != nil && s.proc.Running() {
		_ = s.proc.Stop()
	}
	port, err := freePort()
	if err != nil {
		return "", err
	}
	conf, err := p.config(models, port)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(p.ServerDir(), 0o755); err != nil {
		return "", err
	}
	confPath := filepath.Join(p.ServerDir(), "server.json")
	if err := os.WriteFile(confPath, conf, 0o644); err != nil {
		return "", err
	}
	c, err := p.Command(nil, []string{"server", "--config", confPath})
	if err != nil {
		return "", err
	}
	proc := &procs.Proc{}
	if err := proc.Start(context.Background(), c.Program, c.Args, c.Dir, c.Env); err != nil {
		return "", fmt.Errorf("starting the audio.cpp server: %w", err)
	}
	base := "http://127.0.0.1:" + strconv.Itoa(port)
	wait := s.ReadyTimeout
	if wait == 0 {
		wait = 90 * time.Second
	}
	deadline := time.Now().Add(wait)
	for {
		select {
		case <-proc.Done():
			return "", fmt.Errorf("the audio.cpp server exited at start (code %d): %s", proc.ExitCode(), tail(proc.Lines(), 4))
		case <-ctx.Done():
			_ = proc.Stop()
			return "", ctx.Err()
		case <-time.After(150 * time.Millisecond):
		}
		if s.healthy(ctx, base) {
			break
		}
		if time.Now().After(deadline) {
			_ = proc.Stop()
			return "", fmt.Errorf("the audio.cpp server did not answer within %s: %s", wait, tail(proc.Lines(), 4))
		}
	}
	s.proc, s.port, s.models = proc, port, key
	return base, nil
}

func tail(lines []string, n int) string {
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, " | ")
}

func (s *Server) healthy(ctx context.Context, base string) bool {
	rctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(rctx, http.MethodGet, base+"/health", nil)
	resp, err := s.client().Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// Stop stops the server (a no-op when it is not running). Removing a model or
// the platform calls it first: a running server holds the files open.
func (s *Server) Stop() {
	s.mu.Lock()
	proc := s.proc
	s.proc, s.models = nil, ""
	s.mu.Unlock()
	if proc != nil {
		_ = proc.Stop()
	}
}

// SpeakRequest is one generation.
type SpeakRequest struct {
	Model string // a Model.ID
	Input string
	Voice string // "" = the model's default
}

// Clip is the audio of a finished generation.
type Clip struct {
	WAV     []byte
	Seconds float64
	Model   Model
	Voice   string
}

const maxPCM = 512 << 20

// Speak generates one clip: it asks the server for a streamed answer (SSE) and
// collects the PCM. Closing the stream is how the server is told to stop, so
// Cancel works by cancelling the request's context.
func (s *Server) Speak(ctx context.Context, r SpeakRequest) (Clip, error) {
	m, ok := s.modelByID(r.Model)
	if !ok {
		return Clip{}, fmt.Errorf("%q is not an audio.cpp model this version knows", r.Model)
	}
	if strings.TrimSpace(r.Input) == "" {
		return Clip{}, errors.New("the text is empty")
	}
	voice := r.Voice
	if voice == "" {
		voice = m.DefaultVoice
	}
	if !m.HasVoice(voice) {
		return Clip{}, fmt.Errorf("%q has no voice %q (it has %s)", m.ID, voice, strings.Join(m.Voices, ", "))
	}
	s.mu.Lock()
	if s.gen != nil {
		s.mu.Unlock()
		return Clip{}, ErrBusy
	}
	gctx, cancel := context.WithCancel(ctx)
	g := &generation{cancel: cancel, settled: make(chan struct{})}
	s.gen = g
	s.mu.Unlock()
	defer func() {
		cancel()
		s.mu.Lock()
		s.gen = nil
		s.mu.Unlock()
		close(g.settled)
	}()

	base, err := s.ensure(gctx)
	if err != nil {
		if gctx.Err() != nil {
			return Clip{}, ErrCancelled
		}
		return Clip{}, err
	}
	pcm, err := s.stream(gctx, base, m, voice, r.Input)
	if gctx.Err() != nil {
		// The client side of the stream is closed: the server stops within an event
		// or two. Check that it did, and stop it when it did not.
		g.killed = s.settle(base)
		return Clip{}, ErrCancelled
	}
	if err != nil {
		return Clip{}, err
	}
	if len(pcm) == 0 {
		return Clip{}, errors.New("the audio.cpp server returned no audio")
	}
	return Clip{WAV: pcmWAV(pcm, m.rate()), Seconds: float64(len(pcm)) / 2 / float64(m.rate()), Model: m, Voice: voice}, nil
}

func (m Model) rate() int {
	if m.SampleRate > 0 {
		return m.SampleRate
	}
	return 24000
}

func (s *Server) stream(ctx context.Context, base string, m Model, voice, input string) ([]byte, error) {
	body, _ := json.Marshal(map[string]any{"model": m.ID, "input": input, "voice": voice,
		"response_format": "pcm", "stream_format": "sse"})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/v1/audio/speech", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	resp, err := s.client().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("the audio.cpp server answered HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	var pcm []byte
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 64<<10), 8<<20)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}
		var ev struct {
			Type  string `json:"type"`
			Audio string `json:"audio"`
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal([]byte(data), &ev) != nil {
			continue
		}
		switch ev.Type {
		case "error":
			return nil, fmt.Errorf("the audio.cpp server: %s", ev.Error.Message)
		case "speech.audio.delta":
			b, err := base64.StdEncoding.DecodeString(ev.Audio)
			if err != nil {
				return nil, fmt.Errorf("a bad audio event: %w", err)
			}
			if len(pcm)+len(b) > maxPCM {
				return nil, errors.New("the clip is longer than 512 MB of audio")
			}
			pcm = append(pcm, b...)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return pcm, nil
}

// settle checks, after a cancelled stream, that the server went back to idle: a
// request with a short busy timeout is refused with 503 while a generation still
// runs. If it is still busy after SettleTimeout the server is stopped (the next
// generation starts it again). It reports whether it had to stop it.
func (s *Server) settle(base string) (killed bool) {
	limit := s.SettleTimeout
	if limit == 0 {
		limit = 8 * time.Second
	}
	deadline := time.Now().Add(limit)
	for time.Now().Before(deadline) {
		if s.idle(base) {
			return false
		}
		time.Sleep(250 * time.Millisecond)
	}
	s.Stop()
	return true
}

// idle asks for a one-word clip with busy_timeout_ms 1000: 200 means the model
// was free, 503 that it is still busy with the abandoned run.
func (s *Server) idle(base string) bool {
	s.mu.Lock()
	model := ""
	if s.models != "" {
		model = strings.Split(s.models, ",")[0]
	}
	s.mu.Unlock()
	m, ok := s.modelByID(model)
	if !ok {
		return true
	}
	body, _ := json.Marshal(map[string]any{"model": m.ID, "input": "ok", "voice": m.DefaultVoice,
		"response_format": "pcm", "stream_format": "sse", "busy_timeout_ms": 1000})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, base+"/v1/audio/speech", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client().Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode == http.StatusOK
}

// CancelResult is what a cancel did.
type CancelResult struct {
	Cancelled bool `json:"cancelled"`
	// ServerStopped is true when the stream's close did not stop the run and the
	// server process was stopped instead.
	ServerStopped bool `json:"server_stopped"`
}

// Cancel stops the running generation and waits until the server is idle again
// (or stopped). With nothing running it says so.
func (s *Server) Cancel() CancelResult {
	s.mu.Lock()
	g := s.gen
	s.mu.Unlock()
	if g == nil {
		return CancelResult{}
	}
	g.cancel()
	<-g.settled
	return CancelResult{Cancelled: true, ServerStopped: g.killed}
}

// pcmWAV wraps 16-bit mono little-endian PCM in a WAV header.
func pcmWAV(pcm []byte, rate int) []byte {
	var b bytes.Buffer
	b.WriteString("RIFF")
	binary.Write(&b, binary.LittleEndian, uint32(36+len(pcm)))
	b.WriteString("WAVEfmt ")
	binary.Write(&b, binary.LittleEndian, uint32(16))
	binary.Write(&b, binary.LittleEndian, uint16(1)) // PCM
	binary.Write(&b, binary.LittleEndian, uint16(1)) // mono
	binary.Write(&b, binary.LittleEndian, uint32(rate))
	binary.Write(&b, binary.LittleEndian, uint32(rate*2))
	binary.Write(&b, binary.LittleEndian, uint16(2))
	binary.Write(&b, binary.LittleEndian, uint16(16))
	b.WriteString("data")
	binary.Write(&b, binary.LittleEndian, uint32(len(pcm)))
	b.Write(pcm)
	return b.Bytes()
}
