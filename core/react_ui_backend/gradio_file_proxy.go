package react_ui_backend

// Port of react-ui/src/pages/api/gradio-file-proxy/[...path].tsx.
//
// Behaviour kept from the original, on purpose:
//   - the decoded path segments are joined with "/" and appended to the Gradio
//     address as text, so the original's URL parsing rules apply (a decoded
//     "?" starts a query);
//   - the upstream is fetched with GET whatever the method of the request;
//   - GRADIO_AUTH is not sent (the original's auth code is commented out);
//   - only Content-Type, Content-Length and Content-Disposition are passed on;
//   - the whole body is read before anything is sent, as the original does with
//     arrayBuffer(), so an upstream failure half way gives a 500 and not a
//     truncated file;
//   - errors are JSON {"error": ...}.
//
// Not reproduced: Next's ETag header on the response and its 304 handling.

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

func init() {
	RegisterClaim(Claim{
		Switch:   "gradio-file-proxy",
		Patterns: []string{"/api/gradio-file-proxy/{path...}"},
		Handler:  newGradioFileProxy,
	})
}

func newGradioFileProxy(deps Deps) http.Handler {
	// Redirects are followed, as fetch() does. No timeout: files can be large.
	client := &http.Client{Transport: &http.Transport{DisableCompression: false}}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.PathValue("path")
		if path == "" {
			writeJSONError(w, http.StatusBadRequest, "Invalid path")
			return
		}
		// Both branches of the original build the same string.
		target, err := url.Parse(deps.GradioBackend + path)
		if err != nil {
			gradioFileFailed(w)
			return
		}
		req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, target.String(), nil)
		if err != nil {
			gradioFileFailed(w)
			return
		}
		resp, err := client.Do(req)
		if err != nil {
			gradioFileFailed(w)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			statusText := strings.TrimSpace(strings.TrimPrefix(resp.Status, strconv.Itoa(resp.StatusCode)))
			writeJSONError(w, resp.StatusCode, "Failed to fetch file from Gradio backend: "+statusText)
			return
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			gradioFileFailed(w)
			return
		}
		h := w.Header()
		if v := resp.Header.Get("Content-Type"); v != "" {
			h.Set("Content-Type", v)
		} else {
			h.Set("Content-Type", "application/octet-stream") // what res.send(Buffer) defaults to
		}
		if v := resp.Header.Get("Content-Length"); v != "" && !resp.Uncompressed {
			h.Set("Content-Length", v)
		} else {
			h.Set("Content-Length", strconv.Itoa(len(body)))
		}
		if v := resp.Header.Get("Content-Disposition"); v != "" {
			h.Set("Content-Disposition", v)
		}
		w.WriteHeader(http.StatusOK)
		if r.Method != http.MethodHead {
			_, _ = w.Write(body)
		}
	})
}

func gradioFileFailed(w http.ResponseWriter) {
	writeJSONError(w, http.StatusInternalServerError, "Failed to proxy file")
}

// writeJSONError writes what Next's res.status(n).json({error}) writes.
func writeJSONError(w http.ResponseWriter, status int, msg string) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false) // JSON.stringify does not escape < > &
	_ = enc.Encode(map[string]string{"error": msg})
	b := bytes.TrimRight(buf.Bytes(), "\n")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(b)))
	w.WriteHeader(status)
	_, _ = w.Write(b)
}
