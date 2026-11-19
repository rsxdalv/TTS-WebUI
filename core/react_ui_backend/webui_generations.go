package react_ui_backend

// Port of react-ui/src/pages/api/webui-generations/[...name].ts, which is
// express.static() (serve-static 1.15 and send 0.18) on a list of directories.
//
// What is kept:
//   - only the six SERVED_DIRECTORIES are served; the directory name matches
//     case-insensitively, as an express route does;
//   - dotfiles are denied: any path segment that starts with "." is a 404, and
//     so is a directory, a missing file and anything that is not GET or HEAD
//     (an empty 404, from the handler at the end of the original's chain);
//   - send's headers (Accept-Ranges, Cache-Control: public, max-age=0,
//     Last-Modified, a weak ETag, Content-Type from mime@1.6.0), 304 handling,
//     single byte ranges, If-Range, and the HTML error pages for send's 412
//     and 416 errors, with the headers send had already set.
//
// Differences, recorded in B2.md: on Windows a segment containing ":" or
// ending in "." or " " is refused (NTFS alternate data streams, and names
// Windows silently rewrites).
//
// Traversal: ".." and empty segments, encoded separators and backslashes never
// reach this handler (CONTRACTS.md section 5), and every segment is checked
// again here.

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// servedDirectories is SERVED_DIRECTORIES in the original.
var servedDirectories = []string{"outputs", "favorites", "voices", "collections", "outputs-rvc", "voices-tortoise"}

func init() {
	RegisterClaim(Claim{
		Switch:   "webui-generations",
		Patterns: []string{"GET /api/webui-generations/{path...}"},
		Handler:  newWebuiGenerations,
	})
}

func newWebuiGenerations(deps Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		file, ok := generationFile(deps.Root, r.PathValue("path"))
		if !ok {
			notFoundEmpty(w)
			return
		}
		st, err := os.Stat(file)
		if err != nil || !st.Mode().IsRegular() {
			notFoundEmpty(w)
			return
		}
		serveSendStatic(w, r, file, st)
	})
}

// generationFile maps the request path to a file below one served directory.
func generationFile(root, reqPath string) (string, bool) {
	segs := strings.Split(reqPath, "/")
	if len(segs) < 2 {
		return "", false // the directory itself: index is off, so 404
	}
	dir := ""
	for _, d := range servedDirectories {
		if strings.EqualFold(d, segs[0]) {
			dir = d
		}
	}
	if dir == "" {
		return "", false
	}
	rest := segs[1:]
	for _, s := range rest {
		if s == "" || s == "." || s == ".." || strings.HasPrefix(s, ".") {
			return "", false // dotfiles: deny
		}
		if strings.ContainsAny(s, "\x00\\/") || (runtime.GOOS == "windows" && (strings.Contains(s, ":") || strings.HasSuffix(s, ".") || strings.HasSuffix(s, " "))) {
			return "", false
		}
	}
	return filepath.Join(append([]string{root, dir}, rest...)...), true
}

// notFoundEmpty is res.status(404).end(): no body and no Content-Length (Node
// sends it chunked), so the response is flushed to avoid Go adding one.
func notFoundEmpty(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNotFound)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

// sendError is what send's 412 and 416 errors become: finalhandler's HTML page,
// with the headers send had already set.
func sendError(w http.ResponseWriter, status int) {
	body := "<!DOCTYPE html>\n<html lang=\"en\">\n<head>\n<meta charset=\"utf-8\">\n<title>Error</title>\n</head>\n<body>\n<pre>" +
		statusText(status) + "</pre>\n</body>\n</html>\n"
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Content-Security-Policy", "default-src 'none'")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(status)
	_, _ = io.WriteString(w, body)
}

// serveSendStatic is what send 0.18 does for one regular file.
func serveSendStatic(w http.ResponseWriter, r *http.Request, file string, st os.FileInfo) {
	h := w.Header()
	h.Set("Accept-Ranges", "bytes")
	h.Set("Cache-Control", "public, max-age=0")
	lastModified := st.ModTime().UTC().Format(http.TimeFormat)
	h.Set("Last-Modified", lastModified)
	etag := `W/"` + strconv.FormatInt(st.Size(), 16) + "-" + strconv.FormatInt(st.ModTime().UnixMilli(), 16) + `"`
	h.Set("ETag", etag)
	h.Set("Content-Type", nodeContentType(file))

	if r.Header.Get("If-Match") != "" || r.Header.Get("If-Unmodified-Since") != "" ||
		r.Header.Get("If-None-Match") != "" || r.Header.Get("If-Modified-Since") != "" {
		if preconditionFailed(r, etag, lastModified) {
			sendError(w, http.StatusPreconditionFailed)
			return
		}
		if fresh(r, etag, lastModified) {
			for k := range h {
				if strings.HasPrefix(strings.ToLower(k), "content-") {
					delete(h, k)
				}
			}
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}

	size := st.Size()
	offset, length, status := int64(0), size, http.StatusOK
	if rangeHeader := r.Header.Get("Range"); strings.HasPrefix(strings.TrimLeft(rangeHeader, " "), "bytes=") {
		ranges, state := parseByteRanges(size, rangeHeader)
		if !rangeFresh(r, etag, lastModified) {
			state = rangeSyntax
		}
		switch {
		case state == rangeUnsatisfiable:
			h.Set("Content-Range", "bytes */"+strconv.FormatInt(size, 10))
			sendError(w, http.StatusRequestedRangeNotSatisfiable)
			return
		case state == rangeOK && len(ranges) == 1:
			status = http.StatusPartialContent
			offset, length = ranges[0][0], ranges[0][1]-ranges[0][0]+1
			h.Set("Content-Range", "bytes "+strconv.FormatInt(ranges[0][0], 10)+"-"+strconv.FormatInt(ranges[0][1], 10)+"/"+strconv.FormatInt(size, 10))
		}
	}
	h.Set("Content-Length", strconv.FormatInt(length, 10))
	if r.Method == http.MethodHead {
		w.WriteHeader(status)
		return
	}
	f, err := os.Open(file)
	if err != nil {
		h.Del("Content-Length")
		h.Del("Content-Range")
		notFoundEmpty(w)
		return
	}
	defer f.Close()
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		h.Del("Content-Length")
		h.Del("Content-Range")
		notFoundEmpty(w)
		return
	}
	w.WriteHeader(status)
	_, _ = io.CopyN(w, f, length)
}

// nodeContentType is mime.lookup plus mime.charsets.lookup from mime@1.6.0.
func nodeContentType(path string) string {
	ext := path
	if i := strings.LastIndexAny(path, "./\\"); i >= 0 {
		ext = path[i+1:]
	}
	typ, ok := nodeMimeTypes[strings.ToLower(ext)]
	if !ok {
		typ = "application/octet-stream"
	}
	if strings.HasPrefix(typ, "text/") || typ == "application/json" || typ == "application/javascript" ||
		strings.HasPrefix(typ, "application/json") || strings.HasPrefix(typ, "application/javascript") {
		typ += "; charset=UTF-8"
	}
	return typ
}

func parseHTTPDate(s string) (time.Time, bool) {
	t, err := http.ParseTime(s)
	return t, err == nil
}

// tokenList splits a header on commas and trims spaces, like send's parseTokenList.
func tokenList(s string) []string {
	var out []string
	for _, t := range strings.Split(s, ",") {
		out = append(out, strings.Trim(t, " "))
	}
	return out
}

func etagMatches(token, etag string) bool {
	return token == etag || token == "W/"+etag || "W/"+token == etag
}

func preconditionFailed(r *http.Request, etag, lastModified string) bool {
	if match := r.Header.Get("If-Match"); match != "" {
		if match == "*" {
			return false
		}
		for _, t := range tokenList(match) {
			if etagMatches(t, etag) {
				return false
			}
		}
		return true
	}
	if v := r.Header.Get("If-Unmodified-Since"); v != "" {
		if since, ok := parseHTTPDate(v); ok {
			lm, ok := parseHTTPDate(lastModified)
			return !ok || lm.After(since)
		}
	}
	return false
}

// fresh is the fresh@0.5.2 check send uses for 304.
func fresh(r *http.Request, etag, lastModified string) bool {
	modifiedSince, noneMatch := r.Header.Get("If-Modified-Since"), r.Header.Get("If-None-Match")
	if modifiedSince == "" && noneMatch == "" {
		return false
	}
	if cc := r.Header.Get("Cache-Control"); cc != "" && hasNoCache(cc) {
		return false
	}
	if noneMatch != "" && noneMatch != "*" {
		stale := true
		for _, t := range tokenList(noneMatch) {
			if etagMatches(t, etag) {
				stale = false
				break
			}
		}
		if stale {
			return false
		}
	}
	if modifiedSince != "" {
		lm, ok1 := parseHTTPDate(lastModified)
		since, ok2 := parseHTTPDate(modifiedSince)
		if !ok1 || !ok2 || lm.After(since) {
			return false
		}
	}
	return true
}

// hasNoCache is /(?:^|,)\s*?no-cache\s*?(?:,|$)/.
func hasNoCache(cc string) bool {
	for _, part := range strings.Split(cc, ",") {
		if strings.TrimSpace(part) == "no-cache" {
			return true
		}
	}
	return false
}

func rangeFresh(r *http.Request, etag, lastModified string) bool {
	ifRange := r.Header.Get("If-Range")
	if ifRange == "" {
		return true
	}
	if strings.Contains(ifRange, `"`) {
		return strings.Contains(ifRange, etag)
	}
	lm, ok1 := parseHTTPDate(lastModified)
	since, ok2 := parseHTTPDate(ifRange)
	return ok1 && ok2 && !lm.After(since)
}

type rangeState int

const (
	rangeOK rangeState = iota
	rangeSyntax
	rangeUnsatisfiable
)

// parseByteRanges is range-parser 1.2.1 with combine: true. Each range is
// {start, end}, inclusive.
func parseByteRanges(size int64, header string) ([][2]int64, rangeState) {
	eq := strings.Index(header, "=")
	if eq < 0 {
		return nil, rangeSyntax
	}
	var ranges [][2]int64
	for _, part := range strings.Split(header[eq+1:], ",") {
		bits := strings.Split(part, "-")
		start, startOK := jsParseInt(bits[0])
		end, endOK := int64(0), false
		if len(bits) > 1 {
			end, endOK = jsParseInt(bits[1])
		}
		if !startOK {
			start, end = size-end, size-1
			startOK, endOK = endOK, true
		} else if !endOK {
			end, endOK = size-1, true
		}
		if end > size-1 {
			end = size - 1
		}
		if !startOK || !endOK || start > end || start < 0 {
			continue
		}
		ranges = append(ranges, [2]int64{start, end})
	}
	if len(ranges) < 1 {
		return nil, rangeUnsatisfiable
	}
	// combine: sort by start, merge overlapping or adjacent (gap <= 1) ranges.
	if len(ranges) > 1 {
		ordered := make([][2]int64, len(ranges))
		copy(ordered, ranges)
		for i := 1; i < len(ordered); i++ {
			for j := i; j > 0 && ordered[j][0] < ordered[j-1][0]; j-- {
				ordered[j], ordered[j-1] = ordered[j-1], ordered[j]
			}
		}
		merged := [][2]int64{ordered[0]}
		for _, cur := range ordered[1:] {
			last := &merged[len(merged)-1]
			if cur[0]-last[1] <= 1 {
				if cur[1] > last[1] {
					last[1] = cur[1]
				}
			} else {
				merged = append(merged, cur)
			}
		}
		ranges = merged
	}
	return ranges, rangeOK
}

// jsParseInt is parseInt(s, 10): leading spaces, an optional sign, then digits
// up to the first other character. ok is false for NaN.
func jsParseInt(s string) (int64, bool) {
	s = strings.TrimLeft(s, " \t\r\n")
	i, neg := 0, false
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		neg = s[i] == '-'
		i++
	}
	j := i
	for j < len(s) && s[j] >= '0' && s[j] <= '9' {
		j++
	}
	if j == i {
		return 0, false
	}
	n, err := strconv.ParseInt(s[i:j], 10, 64)
	if err != nil {
		return 0, false
	}
	if neg {
		n = -n
	}
	return n, true
}

// statusText is Node's http.STATUS_CODES for the two codes send raises; Go's
// text for 416 differs ("Requested Range Not Satisfiable").
func statusText(status int) string {
	if status == http.StatusRequestedRangeNotSatisfiable {
		return "Range Not Satisfiable"
	}
	return http.StatusText(status)
}
