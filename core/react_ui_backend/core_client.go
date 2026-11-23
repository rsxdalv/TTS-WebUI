package react_ui_backend

// Browser-facing routes for the loopback core services (pip list, logs, env,
// config, extcatalog). Those services answer only with the X-TTS-Core-Token
// header, which exists only inside the launcher process. This file holds the
// client side: the Core handle the launcher fills in once the control API is
// up, and the helpers the routes share. The browser never sees the token.
//
// CSRF / same-origin rule (decided once, here): port 3000 has no token
// (CONTRACTS.md section 6). The front door already checks Host on every request
// and Origin on every /api request and on every request that is not GET or
// HEAD. On top of that, every route here that changes something requires
// Content-Type: application/json, so a cross-site form post cannot reach it and
// a cross-site fetch needs a preflight, which has no Origin the front door
// accepts.

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"
	"sync"
	"time"
)

const coreTokenHeader = "X-TTS-Core-Token"

// maxCoreBody caps a request body the browser sends to a core route.
const maxCoreBody = 1 << 20

// Core is the address and token of the launcher's control API. The zero value
// has none; routes answer 503 until Set is called.
type Core struct {
	mu         sync.RWMutex
	url, token string
}

// Set stores the control API address (http://127.0.0.1:port) and its token.
func (c *Core) Set(url, token string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.url, c.token = strings.TrimRight(url, "/"), token
	c.mu.Unlock()
}

var errNoCore = errors.New("the core API is not available")

// do sends one request to the control API with the token.
func (c *Core) do(ctx context.Context, method, path string, body io.Reader, contentType string) (*http.Response, error) {
	if c == nil {
		return nil, errNoCore
	}
	c.mu.RLock()
	base, token := c.url, c.token
	c.mu.RUnlock()
	if base == "" || token == "" {
		return nil, errNoCore
	}
	req, err := http.NewRequestWithContext(ctx, method, base+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set(coreTokenHeader, token)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	client := &http.Client{Timeout: 10 * time.Minute}
	return client.Do(req)
}

func writeJSONStatus(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeAPIError(w http.ResponseWriter, code int, msg string) {
	writeJSONStatus(w, code, map[string]string{"error": msg})
}

// coreError answers for a failed call to the control API.
func coreError(w http.ResponseWriter, err error) {
	if errors.Is(err, errNoCore) {
		writeAPIError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	writeAPIError(w, http.StatusBadGateway, "the core API did not answer")
}

// requireJSON refuses a request whose body is not declared as JSON.
func requireJSON(w http.ResponseWriter, r *http.Request) bool {
	mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mt != "application/json" {
		writeAPIError(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
		return false
	}
	return true
}

// readBody reads a capped request body.
func readBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	b, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxCoreBody))
	if err != nil {
		writeAPIError(w, http.StatusRequestEntityTooLarge, "request body too large")
		return nil, false
	}
	return b, true
}

func bytesReader(b []byte) io.Reader { return strings.NewReader(string(b)) }

// relay copies a control API answer to the browser: status, content type, body.
// Nothing is buffered, so a streamed (NDJSON) answer arrives as it is written.
func relay(w http.ResponseWriter, resp *http.Response) {
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(resp.StatusCode)
	flusher, _ := w.(http.Flusher)
	buf := make([]byte, 32<<10)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			_, _ = w.Write(buf[:n])
			if flusher != nil {
				flusher.Flush()
			}
		}
		if err != nil {
			return
		}
	}
}

// getJSON calls the control API and decodes a JSON answer.
func (c *Core) getJSON(ctx context.Context, path string, out any) error {
	resp, err := c.do(ctx, http.MethodGet, path, nil, "")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return errors.New("core API status " + resp.Status)
	}
	return json.Unmarshal(data, out)
}
