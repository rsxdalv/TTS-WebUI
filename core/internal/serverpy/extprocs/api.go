package extprocs

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"strconv"
	"strings"
)

const (
	// TokenHeader carries the shared secret.
	TokenHeader = "X-TTS-Core-Token"
	apiPrefix   = "/api/extensions"
)

// Handler returns the control API. Mount it at both "/api/extensions" and
// "/api/extensions/" on the launcher's mux.
func (m *Manager) Handler() http.Handler { return http.HandlerFunc(m.serve) }

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func (m *Manager) authorized(r *http.Request) (int, string) {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() {
		return http.StatusForbidden, "loopback only"
	}
	if m.opts.Token == "" {
		return http.StatusForbidden, "token not configured"
	}
	got := sha256.Sum256([]byte(r.Header.Get(TokenHeader)))
	want := sha256.Sum256([]byte(m.opts.Token))
	if subtle.ConstantTimeCompare(got[:], want[:]) != 1 {
		return http.StatusForbidden, "bad token"
	}
	return 0, ""
}

func (m *Manager) serve(w http.ResponseWriter, r *http.Request) {
	if code, msg := m.authorized(r); code != 0 {
		writeErr(w, code, msg)
		return
	}
	p := r.URL.Path
	if p != apiPrefix && !strings.HasPrefix(p, apiPrefix+"/") {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	rest := strings.Trim(strings.TrimPrefix(p, apiPrefix), "/")
	var pkg, action string
	if rest != "" {
		parts := strings.Split(rest, "/")
		if len(parts) != 2 {
			writeErr(w, http.StatusNotFound, "not found")
			return
		}
		pkg, action = parts[0], parts[1]
	}
	want := http.MethodPost
	switch action {
	case "", "logs":
		want = http.MethodGet
	case "start", "stop":
	default:
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	if r.Method != want {
		w.Header().Set("Allow", want)
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if want == http.MethodPost {
		mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || mt != "application/json" {
			writeErr(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
			return
		}
	}
	if action != "" {
		if err := ValidatePackage(pkg); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	switch action {
	case "":
		writeJSON(w, http.StatusOK, m.List())
	case "logs":
		lines, err := m.Lines(pkg)
		if err != nil {
			writeErr(w, http.StatusNotFound, err.Error())
			return
		}
		if n, err := strconv.Atoi(r.URL.Query().Get("tail")); err == nil && n >= 0 && n < len(lines) {
			lines = lines[len(lines)-n:]
		}
		writeJSON(w, http.StatusOK, map[string]any{"lines": lines})
	case "start":
		var req StartRequest
		body := http.MaxBytesReader(w, r.Body, 1<<20)
		if err := json.NewDecoder(body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			writeErr(w, http.StatusBadRequest, "bad JSON body: "+err.Error())
			return
		}
		res, err := m.Start(r.Context(), pkg, req)
		if err != nil {
			code := http.StatusInternalServerError
			if errors.Is(err, ErrInvalidPackage) {
				code = http.StatusBadRequest
			}
			writeErr(w, code, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, res)
	case "stop":
		if err := m.Stop(pkg); err != nil {
			code := http.StatusInternalServerError
			if errors.Is(err, ErrNotFound) {
				code = http.StatusNotFound
			}
			writeErr(w, code, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"stopped": true})
	}
}
