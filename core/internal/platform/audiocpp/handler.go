package audiocpp

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"mime"
	"net"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform/openai"
)

const (
	// TokenHeader carries the shared secret of the control API.
	TokenHeader = "X-TTS-Core-Token"
	// Prefix is where the handler is mounted on the control API.
	Prefix = "/api/audiocpp/"

	maxBody = 1 << 20
	// maxText is the longest text a generation takes (characters).
	maxText = 20000
)

// Handler is the control API of the platform (loopback, token). The browser
// reaches it only through port 3000's routes.
//
//	GET  /api/audiocpp/status   the platform, its models (installed or not) and the server
//	POST /api/audiocpp/speech   {model, input, voice?}: generate and save to outputs/
//	POST /api/audiocpp/cancel   stop the running generation
//	POST /api/audiocpp/stop     stop the server (it starts again with the next generation)
type Handler struct {
	root  paths.Root
	token string
	logf  func(string, ...any)
	// Now is the clock of the saved clip's name (tests).
	Now func() time.Time
}

// NewHandler returns the handler.
func NewHandler(root paths.Root, token string, logf func(string, ...any)) *Handler {
	if logf == nil {
		logf = func(string, ...any) {}
	}
	return &Handler{root: root, token: token, logf: logf, Now: time.Now}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func (h *Handler) authorized(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() || h.token == "" {
		return false
	}
	got := sha256.Sum256([]byte(r.Header.Get(TokenHeader)))
	want := sha256.Sum256([]byte(h.token))
	return subtle.ConstantTimeCompare(got[:], want[:]) == 1
}

// ModelInfo is one model on the status page.
type ModelInfo struct {
	ID           string   `json:"id"`
	Package      string   `json:"package"`
	Name         string   `json:"name"`
	Installed    bool     `json:"installed"`
	Voices       []string `json:"voices"`
	DefaultVoice string   `json:"default_voice"`
	SizeMB       int      `json:"size_mb"`
	License      string   `json:"license"`
}

// StatusAnswer is GET status.
type StatusAnswer struct {
	// Supported is false on an OS and architecture with no pinned build.
	Supported bool   `json:"supported"`
	Ready     bool   `json:"ready"`
	Detail    string `json:"detail"`
	Version   string `json:"version"`
	Row       string `json:"row,omitempty"`
	Rows      []Row  `json:"rows"`
	// Models are all the pinned models, installed or not.
	Models []ModelInfo `json:"models"`
	Server Info        `json:"server"`
}

// Status is the answer of GET status.
func Status(ctx context.Context, root paths.Root) StatusAnswer {
	p := New(Options{Root: root})
	st := p.Status(ctx)
	_, err := ArchiveFor(runtime.GOOS, runtime.GOARCH, RowCPU)
	a := StatusAnswer{Supported: err == nil, Ready: st.Ready, Detail: st.Detail, Version: Version, Rows: Rows,
		Models: []ModelInfo{}, Server: ServerFor(root).Info()}
	if err != nil {
		a.Detail = err.Error()
	}
	if s, ok := p.Info(); ok {
		a.Row = s.Row
	}
	for _, m := range p.models {
		a.Models = append(a.Models, ModelInfo{ID: m.ID, Package: m.Package, Name: m.Name, Voices: m.Voices,
			DefaultVoice: m.DefaultVoice, SizeMB: m.SizeMB(), License: m.License,
			Installed: p.EntryStatus(ctx, platform.Entry{Package: m.Package}).Installed})
	}
	return a
}

// SpeechRequest is POST speech.
type SpeechRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
	Voice string `json:"voice"`
}

// Result is the answer of POST speech: where the clip went.
type Result struct {
	Folder  string  `json:"folder"`
	Wav     string  `json:"wav"`
	PNG     string  `json:"png"`
	JSON    string  `json:"json"`
	Seconds float64 `json:"seconds"`
	Model   string  `json:"model"`
	Voice   string  `json:"voice"`
}

func decode(w http.ResponseWriter, r *http.Request, v any) error {
	if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mt != "application/json" {
		return errors.New("Content-Type must be application/json")
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return errors.New("bad JSON body")
	}
	return nil
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !h.authorized(r) {
		writeErr(w, http.StatusForbidden, "forbidden")
		return
	}
	rest, ok := strings.CutPrefix(r.URL.Path, Prefix)
	if !ok {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	method := func(want string) bool {
		if r.Method == want {
			return true
		}
		w.Header().Set("Allow", want)
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return false
	}
	srv := ServerFor(h.root)
	switch rest {
	case "status":
		if method(http.MethodGet) {
			writeJSON(w, http.StatusOK, Status(r.Context(), h.root))
		}
	case "cancel":
		if method(http.MethodPost) {
			cr := srv.Cancel()
			h.logf("audiocpp: cancel: %+v", cr)
			writeJSON(w, http.StatusOK, cr)
		}
	case "stop":
		if method(http.MethodPost) {
			srv.Stop()
			writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
		}
	case "speech":
		if !method(http.MethodPost) {
			return
		}
		var in SpeechRequest
		if err := decode(w, r, &in); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		if len([]rune(in.Input)) > maxText {
			writeErr(w, http.StatusBadRequest, "the text is longer than 20000 characters")
			return
		}
		clip, err := srv.Speak(r.Context(), SpeakRequest{Model: in.Model, Input: in.Input, Voice: in.Voice})
		switch {
		case errors.Is(err, ErrCancelled):
			writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error(), "cancelled": true})
			return
		case errors.Is(err, ErrBusy):
			writeErr(w, http.StatusConflict, err.Error())
			return
		case err != nil:
			h.logf("audiocpp: generate failed: %v", err)
			writeErr(w, http.StatusBadGateway, err.Error())
			return
		}
		sv, secs, err := openai.SaveClip(h.root.Dir(), h.Now(), "audiocpp_"+clip.Model.ID, in.Input, clip.Model.ID, clip.Voice, clip.WAV)
		if err != nil {
			h.logf("audiocpp: saving the clip: %v", err)
			writeErr(w, http.StatusInternalServerError, "saving the clip in outputs/ failed: "+err.Error())
			return
		}
		h.logf("audiocpp: saved %s (%.1f s)", sv.Folder, secs)
		writeJSON(w, http.StatusOK, Result{Folder: sv.Folder, Wav: sv.Wav, PNG: sv.PNG, JSON: sv.JSON, Seconds: secs,
			Model: clip.Model.ID, Voice: clip.Voice})
	default:
		writeErr(w, http.StatusNotFound, "not found")
	}
}
