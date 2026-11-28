package react_ui_backend

// Serving the React UI static export (S2), following CONTRACTS.md section 10:
// frontend/out holds the single-page app (Vite), and frontend/public is served
// live for files the export lacks, so uploads in file-input-cache are reachable
// at once.

import (
	"compress/gzip"
	"compress/zlib"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// ExportClaims are the route claims the static export cannot work without:
// with Next gone, nothing else answers these API paths.
var ExportClaims = []string{"gradio", "gradio-file-proxy", "upload", "webui-generations", "data-outputs", "data-bark-voices"}

// ExportMarker is the file that exists only in a complete export.
const ExportMarker = "_tts-export.json"

// ExportDir returns where the static export lives for an install root.
func ExportDir(root string) string { return filepath.Join(root, "frontend", "out") }

// HasExport reports whether root has a complete static export.
func HasExport(root string) bool {
	st, err := os.Stat(filepath.Join(ExportDir(root), ExportMarker))
	return err == nil && st.Mode().IsRegular()
}

// staticExport serves the export and frontend/public.
type staticExport struct {
	out    string
	public string
	// dev, when set, answers what the export would (the app's pages and assets)
	// from a Vite dev server. Uploads stay live files and /api stays 404 here.
	dev http.Handler
}

func newStaticExport(root string) *staticExport {
	return &staticExport{out: ExportDir(root), public: filepath.Join(root, "frontend", "public")}
}

const (
	cachePage      = "s-maxage=31536000"
	cacheImmutable = "public, max-age=31536000, immutable"
	cacheNotFound  = "private, no-cache, no-store, max-age=0, must-revalidate"
)

func (s *staticExport) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Path

	// API paths no claim handled are 404, never a page, whatever the method.
	if p == "/api" || strings.HasPrefix(p, "/api/") {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// Development: the pages and modules come from the Vite dev server (hot
	// reload), before rule 1: Vite serves /node_modules/.vite/deps/... and other
	// dot paths. Uploads stay live files (rule 2).
	if s.dev != nil && !strings.HasPrefix(p, "/file-input-cache/") {
		s.dev.ServeHTTP(w, r)
		return
	}

	// Rule 1: dot segments, backslashes, NUL, encoded separators and empty
	// segments are refused before any file is looked up.
	if !safeExportPath(r) {
		s.notFound(w, r)
		return
	}

	// Rule 4's trailing slash: redirect. The path is clean, so
	// the target is always a local path.
	if len(p) > 1 && strings.HasSuffix(p, "/") {
		target := strings.TrimRight(p, "/")
		if r.URL.RawQuery != "" {
			target += "?" + r.URL.RawQuery
		}
		w.Header().Set("Location", target)
		w.WriteHeader(http.StatusPermanentRedirect)
		_, _ = io.WriteString(w, target)
		return
	}

	// Rule 2: uploads are always the live files.
	if strings.HasPrefix(p, "/file-input-cache/") {
		if file, st, ok := s.file(s.public, p); ok {
			serveSendStatic(w, r, file, st)
			return
		}
		s.notFound(w, r)
		return
	}

	// Rule 3: the app's entry page.
	if p == "/" {
		s.page(w, r, filepath.Join(s.out, "index.html"), http.StatusOK)
		return
	}

	// Rule 4: a file of the export under its own name.
	if file, st, ok := s.file(s.out, p); ok {
		s.exportFile(w, r, p, file, st)
		return
	}

	// Rule 5: any other file of public the export lacks.
	if file, st, ok := s.file(s.public, p); ok {
		serveSendStatic(w, r, file, st)
		return
	}

	// Rule 6: a path with a file extension is a missing asset, so 404. Any
	// other path is a route of the app: the entry page, and the app renders
	// its own not-found view for a route it does not know.
	if hasExtension(p) {
		s.notFound(w, r)
		return
	}
	s.page(w, r, filepath.Join(s.out, "index.html"), http.StatusOK)
}

// hasExtension reports whether the last segment of p has a dot.
func hasExtension(p string) bool {
	return strings.Contains(p[strings.LastIndex(p, "/")+1:], ".")
}

// safeExportPath applies rule 1 to the raw and the decoded path.
func safeExportPath(r *http.Request) bool {
	lower := strings.ToLower(r.URL.EscapedPath())
	if strings.Contains(lower, "%2f") || strings.Contains(lower, "%5c") || strings.Contains(lower, "%00") {
		return false
	}
	p := r.URL.Path
	if !strings.HasPrefix(p, "/") {
		return false
	}
	segs := strings.Split(strings.TrimSuffix(p[1:], "/"), "/")
	if p == "/" {
		return true
	}
	for _, seg := range segs {
		if seg == "" || strings.HasPrefix(seg, ".") || strings.ContainsAny(seg, "\\\x00") {
			return false
		}
		if runtime.GOOS == "windows" && (strings.Contains(seg, ":") || strings.HasSuffix(seg, " ") || strings.HasSuffix(seg, ".")) {
			return false
		}
	}
	return true
}

// file maps a clean URL path to a regular file below dir.
func (s *staticExport) file(dir, urlPath string) (string, os.FileInfo, bool) {
	rel := filepath.FromSlash(strings.TrimPrefix(urlPath, "/"))
	full := filepath.Join(dir, rel)
	if r, err := filepath.Rel(dir, full); err != nil || r == ".." || strings.HasPrefix(r, ".."+string(filepath.Separator)) {
		return "", nil, false
	}
	st, err := os.Stat(full)
	if err != nil || !st.Mode().IsRegular() {
		return "", nil, false
	}
	if realNameHidden(dir, full) {
		return "", nil, false // a dotfile reached through its 8.3 alias
	}
	return full, st, true
}

// exportFile serves a file of the export found under its own name. The build
// names its hashed bundles under /assets/, which never change, so they are
// immutable.
func (s *staticExport) exportFile(w http.ResponseWriter, r *http.Request, p, file string, st os.FileInfo) {
	switch {
	case strings.HasPrefix(p, "/assets/"):
		serveSendStatic(cacheControlWriter{w, cacheImmutable}, r, file, st)
	case strings.HasSuffix(p, ".html"):
		s.page(w, r, file, http.StatusOK)
	default:
		serveSendStatic(w, r, file, st)
	}
}

func (s *staticExport) notFound(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", cacheNotFound)
	http.NotFound(w, r)
}

// page sends the app's HTML page with a strong ETag, 304 for a matching
// If-None-Match, and compression from 1 KB up.
func (s *staticExport) page(w http.ResponseWriter, r *http.Request, file string, status int) {
	data, err := os.ReadFile(file)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Vary", "Accept-Encoding")
	if status == http.StatusNotFound {
		h.Set("Cache-Control", cacheNotFound)
	} else {
		h.Set("Cache-Control", cachePage)
		etag := nextETag(string(data))
		h["ETag"] = []string{etag}
		if etagFresh(r.Header, etag) {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}
	if len(data) >= 1024 {
		if enc := negotiateEncoding(r.Header.Get("Accept-Encoding")); enc != "" {
			h.Set("Content-Encoding", enc)
			w.WriteHeader(status)
			if r.Method == http.MethodHead {
				return
			}
			var zw io.WriteCloser
			if enc == "gzip" {
				zw = gzip.NewWriter(w)
			} else {
				zw = zlib.NewWriter(w)
			}
			_, _ = zw.Write(data)
			_ = zw.Close()
			return
		}
	}
	h.Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(status)
	if r.Method != http.MethodHead {
		_, _ = w.Write(data)
	}
}

// cacheControlWriter replaces the Cache-Control header when the response
// starts, for files served by serveSendStatic.
type cacheControlWriter struct {
	http.ResponseWriter
	value string
}

func (c cacheControlWriter) WriteHeader(code int) {
	if code < 400 {
		c.Header().Set("Cache-Control", c.value)
	}
	c.ResponseWriter.WriteHeader(code)
}

func (c cacheControlWriter) Write(b []byte) (int, error) { return c.ResponseWriter.Write(b) }

func (c cacheControlWriter) Flush() {
	if f, ok := c.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
