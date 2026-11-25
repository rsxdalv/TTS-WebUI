package onnx

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"mime"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rsxdalv/TTS-WebUI/core/internal/platform"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform/openai"
)

// Prefix is where the handler is mounted on the control API.
const Prefix = "/api/onnx/"

// Handler is the control API of the platform (loopback, token), like the openai
// platform's. The browser reaches it only through port 3000's routes.
//
//	GET  /api/onnx/engines   the entries, whether each is installed, the voices
//	POST /api/onnx/speech    generate and save: {package, input, voice, speed}
//
// A request that is cancelled (the browser went away or pressed Cancel) kills
// the program, and nothing is saved.
type Handler struct {
	p     *Platform
	token string
	logf  func(string, ...any)
	now   func() time.Time
}

// NewHandler returns the handler.
func NewHandler(p *Platform, token string, logf func(string, ...any)) *Handler {
	if logf == nil {
		logf = func(string, ...any) {}
	}
	return &Handler{p: p, token: token, logf: logf, now: time.Now}
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
	got := sha256.Sum256([]byte(r.Header.Get(openai.TokenHeader)))
	want := sha256.Sum256([]byte(h.token))
	return subtle.ConstantTimeCompare(got[:], want[:]) == 1
}

// Engine is one row of GET engines.
type Engine struct {
	Package    string `json:"package"`
	Name       string `json:"name"`
	Installed  bool   `json:"installed"`
	Voices     int    `json:"voices"`
	Language   string `json:"language"`
	License    string `json:"license"`
	PlatformOK bool   `json:"platform_ready"`
	Detail     string `json:"detail,omitempty"`
}

// Engines lists the pinned models and whether each can generate now.
func (p *Platform) Engines(ctx context.Context) []Engine {
	st := p.Status(ctx)
	var out []Engine
	for _, m := range p.allModels() {
		es := p.EntryStatus(ctx, platform.Entry{Package: m.Package})
		out = append(out, Engine{Package: m.Package, Name: m.Title, Installed: es.Installed && st.Ready, Voices: m.Voices,
			Language: m.Language, License: m.License, PlatformOK: st.Ready, Detail: st.Detail})
	}
	return out
}

func (p *Platform) allModels() []Model {
	if p.models != nil {
		return p.models
	}
	return Models()
}

// SpeechRequest is the body of POST speech.
type SpeechRequest struct {
	Package string  `json:"package"`
	Input   string  `json:"input"`
	Voice   int     `json:"voice"`
	Speed   float64 `json:"speed"`
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !h.authorized(r) {
		writeErr(w, http.StatusForbidden, "forbidden")
		return
	}
	switch strings.TrimPrefix(r.URL.Path, Prefix) {
	case "engines":
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		writeJSON(w, http.StatusOK, h.p.Engines(r.Context()))
	case "speech":
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		h.speech(w, r)
	default:
		writeErr(w, http.StatusNotFound, "not found")
	}
}

func (h *Handler) speech(w http.ResponseWriter, r *http.Request) {
	if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mt != "application/json" {
		writeErr(w, http.StatusBadRequest, "Content-Type must be application/json")
		return
	}
	var in SpeechRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, "bad JSON body")
		return
	}
	wav, err := h.p.Generate(r.Context(), Request{Package: in.Package, Text: in.Input, Voice: in.Voice, Speed: in.Speed})
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalid):
			writeErr(w, http.StatusBadRequest, strings.TrimPrefix(err.Error(), ErrInvalid.Error()+": "))
		case r.Context().Err() != nil:
			h.logf("onnx %s: generation cancelled", in.Package)
			writeErr(w, 499, "cancelled")
		default:
			h.logf("onnx %s: %v", in.Package, err)
			writeErr(w, http.StatusBadGateway, err.Error())
		}
		return
	}
	m, _ := h.p.model(in.Package)
	speed := in.Speed
	if speed == 0 {
		speed = 1
	}
	res, err := openai.SaveClip(h.p.root.Dir(), h.now(), openai.ClipMeta{Engine: "onnx_" + m.Dir, Model: m.Title,
		Voice: voiceName(in.Voice), Text: in.Input, Speed: speed}, wav)
	if err != nil {
		h.logf("onnx %s: %v", in.Package, err)
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.logf("onnx %s: saved %s (%.1f s)", in.Package, res.Folder, res.Seconds)
	writeJSON(w, http.StatusOK, res)
}

func voiceName(v int) string { return "voice " + strconv.Itoa(v) }
