package react_ui_backend

// Serving the React UI static export (S2), following CONTRACTS.md section 10:
// react-ui/out holds the export, and react-ui/public is served live for files
// the export lacks, so uploads in file-input-cache are reachable at once.

import (
	"compress/gzip"
	"compress/zlib"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

// ExportMarker is the file that exists only in a complete export.
const ExportMarker = "_tts-export.json"

// ExportDir returns where the static export lives for an install root.
func ExportDir(root string) string { return filepath.Join(root, "react-ui", "out") }

// HasExport reports whether root has a complete static export.
func HasExport(root string) bool {
	st, err := os.Stat(filepath.Join(ExportDir(root), ExportMarker))
	return err == nil && st.Mode().IsRegular()
}

// staticExport serves the export and react-ui/public.
type staticExport struct {
	out    string
	public string
}

func newStaticExport(root string) *staticExport {
	return &staticExport{out: ExportDir(root), public: filepath.Join(root, "react-ui", "public")}
}

const (
	cachePage      = "s-maxage=31536000"
	cacheImmutable = "public, max-age=31536000, immutable"
	cacheNotFound  = "private, no-cache, no-store, max-age=0, must-revalidate"
)

var (
	extensionPage = regexp.MustCompile(`^/extensions/[^/]+$`)
	extensionData = regexp.MustCompile(`^/_next/data/([^/]+)/extensions/[^/]+\.json$`)
)

func (s *staticExport) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	p := r.URL.Path

	// API paths no claim handled are 404, never a page.
	if p == "/api" || strings.HasPrefix(p, "/api/") {
		http.NotFound(w, r)
		return
	}

	// Rule 1: dot segments, backslashes, NUL, encoded separators and empty
	// segments are refused before any file is looked up.
	if !safeExportPath(r) {
		s.notFound(w, r)
		return
	}

	// Rule 4's trailing slash: redirect as Next does. The path is clean, so
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

	// Rules 3 and 4: a file of the export.
	if p == "/" {
		s.page(w, r, filepath.Join(s.out, "index.html"), http.StatusOK)
		return
	}
	if file, st, ok := s.file(s.out, p); ok {
		s.exportFile(w, r, p, file, st)
		return
	}
	for _, candidate := range []string{p + ".html", p + "/index.html"} {
		if file, _, ok := s.file(s.out, candidate); ok {
			s.page(w, r, file, http.StatusOK)
			return
		}
	}

	// Rule 2, second half: any other file of public the export lacks.
	if file, st, ok := s.file(s.public, p); ok {
		serveSendStatic(w, r, file, st)
		return
	}

	// Rules 5 and 6: the extension shell serves any extension name.
	if extensionPage.MatchString(p) {
		if file, _, ok := s.file(s.out, "/extensions/_shell.html"); ok {
			s.page(w, r, file, http.StatusOK)
			return
		}
	}
	if m := extensionData.FindStringSubmatch(p); m != nil {
		if file, _, ok := s.file(s.out, "/_next/data/"+m[1]+"/extensions/_shell.json"); ok {
			s.page(w, r, file, http.StatusOK)
			return
		}
	}

	// Rule 7.
	s.notFound(w, r)
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
	return full, st, true
}

// exportFile serves a file of the export found under its own name.
func (s *staticExport) exportFile(w http.ResponseWriter, r *http.Request, p, file string, st os.FileInfo) {
	switch {
	case strings.HasPrefix(p, "/_next/static/"):
		serveSendStatic(cacheControlWriter{w, cacheImmutable}, r, file, st)
	case strings.HasPrefix(p, "/_next/data/") || strings.HasSuffix(p, ".html"):
		s.page(w, r, file, http.StatusOK)
	default:
		serveSendStatic(w, r, file, st)
	}
}

func (s *staticExport) notFound(w http.ResponseWriter, r *http.Request) {
	file := filepath.Join(s.out, "404.html")
	if _, err := os.Stat(file); err != nil {
		http.NotFound(w, r)
		return
	}
	s.page(w, r, file, http.StatusNotFound)
}

// page sends an HTML page or a data JSON file as `next start` does: a strong
// ETag from Next's hash, 304 for a matching If-None-Match, and compression from
// 1 KB up.
func (s *staticExport) page(w http.ResponseWriter, r *http.Request, file string, status int) {
	data, err := os.ReadFile(file)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	h := w.Header()
	if strings.HasSuffix(file, ".json") {
		h.Set("Content-Type", "application/json; charset=utf-8")
	} else {
		h.Set("Content-Type", "text/html; charset=utf-8")
	}
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
