package router

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net"
	"net/http"
	"net/netip"
)

// TokenHeader carries the shared secret on control API requests.
const TokenHeader = "X-TTS-Core-Token"

// controlPath is the single absolute path of the control API.
const controlPath = "/api/routes"

// maxControlBody bounds a control request body; real bodies are ~40 bytes.
const maxControlBody = 16 << 10

// ControlHandler returns the routes control API as a plain http.Handler on the
// absolute path /api/routes, so it can be mounted on any mux:
//
//	POST   /api/routes {"prefix":"/pkg","port":N}  route /pkg to http://127.0.0.1:N
//	DELETE /api/routes?prefix=/pkg                 remove a route
//	GET    /api/routes                             {"/pkg":"http://127.0.0.1:N"}
//
// Every request must come from a loopback address and carry the Options.Token
// in the X-TTS-Core-Token header (otherwise 403); a POST must also have
// Content-Type application/json (otherwise 415). The catch-all "/" route is
// owned by Go itself ([Router.AddRoute]) and cannot be changed through this API.
func (rt *Router) ControlHandler() http.Handler {
	return http.HandlerFunc(rt.serveControl)
}

func (rt *Router) serveControl(w http.ResponseWriter, r *http.Request) {
	if !rt.authorized(r) {
		controlError(w, http.StatusForbidden, "forbidden")
		return
	}
	if r.URL.Path != controlPath {
		controlError(w, http.StatusNotFound, "not found")
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, rt.Routes())
	case http.MethodPost:
		rt.controlAdd(w, r)
	case http.MethodDelete:
		rt.controlRemove(w, r)
	default:
		w.Header().Set("Allow", "GET, POST, DELETE")
		controlError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// authorized checks the caller is on loopback and knows the token.
func (rt *Router) authorized(r *http.Request) bool {
	if rt.token == "" || !isLoopback(r.RemoteAddr) {
		return false
	}
	got := sha256.Sum256([]byte(r.Header.Get(TokenHeader)))
	want := sha256.Sum256([]byte(rt.token))
	return subtle.ConstantTimeCompare(got[:], want[:]) == 1
}

func isLoopback(remoteAddr string) bool {
	if ap, err := netip.ParseAddrPort(remoteAddr); err == nil {
		return ap.Addr().Unmap().IsLoopback()
	}
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return false
	}
	addr, err := netip.ParseAddr(host)
	return err == nil && addr.Unmap().IsLoopback()
}

func (rt *Router) controlAdd(w http.ResponseWriter, r *http.Request) {
	if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mt != "application/json" {
		controlError(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
		return
	}
	var req struct {
		Prefix string `json:"prefix"`
		Port   *int   `json:"port"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxControlBody))
	if err := dec.Decode(&req); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			controlError(w, http.StatusRequestEntityTooLarge, "body too large")
			return
		}
		controlError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	prefix, ok := controlPrefix(w, req.Prefix)
	if !ok {
		return
	}
	if req.Port == nil || *req.Port < 1 || *req.Port > 65535 {
		controlError(w, http.StatusBadRequest, "port must be 1-65535")
		return
	}
	target := fmt.Sprintf("http://127.0.0.1:%d", *req.Port)
	if err := rt.AddRoute(prefix, target); err != nil {
		controlError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"prefix": prefix, "target": target})
}

func (rt *Router) controlRemove(w http.ResponseWriter, r *http.Request) {
	prefix, ok := controlPrefix(w, r.URL.Query().Get("prefix"))
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"prefix": prefix, "removed": rt.removeRoute(prefix)})
}

// controlPrefix validates a prefix received over the control API and writes the
// 400 itself when it is unusable.
func controlPrefix(w http.ResponseWriter, raw string) (string, bool) {
	if raw == "" {
		controlError(w, http.StatusBadRequest, "prefix is required")
		return "", false
	}
	p, err := NormalizePrefix(raw)
	if err != nil {
		controlError(w, http.StatusBadRequest, err.Error())
		return "", false
	}
	if p == "" {
		controlError(w, http.StatusBadRequest, `the "/" route cannot be changed through the control API`)
		return "", false
	}
	return p, true
}

func controlError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
