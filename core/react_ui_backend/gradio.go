package react_ui_backend

// Port of react-ui/src/pages/api/gradio/[name].tsx: POST /api/gradio/<name>
// calls the named Gradio endpoint and answers with the mapped result.
//
// The original's quirks are kept on purpose, because the UI depends on the
// exact responses: a thrown error answers 500 "Internal Server Error", and the
// cases where the original never answers (see errHang) never answer here.
//
// Deliberate differences, recorded in tts-comms/B6.md:
//   - getFile refuses paths outside react-ui/public (CONTRACTS.md section 9).
//   - Names inherited from Object.prototype ("constructor", "toString", ...)
//     are 404 here; in the original they call prototype methods.
//   - The Gradio heartbeat stream closes when the request ends.

import (
	"compress/gzip"
	"compress/zlib"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf16"
)

func init() {
	RegisterClaim(Claim{
		Switch:   "gradio",
		Patterns: []string{"/api/gradio/{name}"},
		Handler: func(d Deps) http.Handler {
			return &gradioRoute{
				backend:   d.GradioBackend,
				core:      d.Core,
				auth:      d.GradioAuth,
				publicDir: filepath.Join(d.Root, "react-ui", "public"),
			}
		},
	})
}

type gradioRoute struct {
	backend   string
	engine    string // set on a per-call copy: the package whose process answers
	core      *Core  // for engines that own an api name (gradio_engines.go)
	auth      string
	publicDir string
}

// bodyLimit is Next's default API body limit, "1mb".
const bodyLimit = 1 << 20

func (g *gradioRoute) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, apiErr := parseNextBody(r)
	if apiErr != nil {
		// Node keeps reading an unread body, so the browser can finish
		// sending and read the answer. Go would reset the connection.
		_, _ = io.Copy(io.Discard, io.LimitReader(r.Body, 256<<20))
		sendNextError(w, apiErr.status, apiErr.message)
		return
	}

	name := r.PathValue("name")
	fn, ok := gradioEndpoints[name]
	if !ok {
		writeNextJSON(w, r, http.StatusNotFound, obj("data", obj("error", "Not found")))
		return
	}

	parsed := body
	if s, ok := body.(string); ok && s != "" {
		v, err := parseJSON([]byte(s))
		if err != nil {
			sendNextError(w, http.StatusInternalServerError, "Internal Server Error")
			return
		}
		parsed = v
	}

	// Like Node, the call to Gradio keeps running if the browser leaves.
	route := g
	if pkg, owned := engineOwners[name]; owned {
		backend, err := engineBackend(context.WithoutCancel(r.Context()), g.core, pkg, true)
		switch {
		case err == nil:
			cp := *g
			cp.backend, cp.engine = backend, pkg
			route = &cp
		case errors.Is(err, errNotEngine), errors.Is(err, errNoCore):
			// Not converted: the Gradio host still has the tab.
		default:
			slog.Warn("gradio route: engine", "name", name, "pkg", pkg, "err", err)
			writeNextJSON(w, r, http.StatusBadGateway, obj("data", obj("error", err.Error())))
			return
		}
	}
	call := &gradioCall{route: route, ctx: context.WithoutCancel(r.Context()), selfHost: r.Host}
	result, err := fn(call, parsed)
	switch {
	case err == nil:
		writeNextJSON(w, r, http.StatusOK, result)
	case errors.Is(err, errHang):
		<-r.Context().Done()
	default:
		slog.Warn("gradio route", "name", name, "err", err)
		if route.engine != "" {
			logEngineTail(call.ctx, route.core, route.engine)
		}
		sendNextError(w, http.StatusInternalServerError, "Internal Server Error")
	}
}

// --- Next.js API behaviour ---

type nextAPIError struct {
	status  int
	message string
}

// parseNextBody is Next's API body parser: JSON for application/json,
// form fields for urlencoded bodies, and the raw text for anything else.
func parseNextBody(r *http.Request) (any, *nextAPIError) {
	mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if r.Header.Get("Content-Type") == "" || err != nil {
		mediaType, params = "text/plain", map[string]string{}
	}
	charset := strings.ToLower(params["charset"])
	switch charset {
	case "", "utf-8", "utf8", "us-ascii", "ascii":
	default:
		return nil, &nextAPIError{400, "Invalid body"}
	}
	tooLarge := &nextAPIError{413, "Body exceeded 1mb limit"}
	if r.ContentLength > bodyLimit {
		return nil, tooLarge
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, bodyLimit+1))
	if err != nil {
		return nil, &nextAPIError{400, "Invalid body"}
	}
	if len(raw) > bodyLimit {
		return nil, tooLarge
	}
	text := strings.ToValidUTF8(string(raw), "�")

	switch mediaType {
	case "application/json", "application/ld+json":
		if text == "" {
			return newObject(), nil
		}
		v, err := parseJSON([]byte(text))
		if err != nil {
			return nil, &nextAPIError{400, "Invalid JSON"}
		}
		return v, nil
	case "application/x-www-form-urlencoded":
		return decodeQueryString(text), nil
	}
	return text, nil
}

// decodeQueryString is Node's querystring.decode.
func decodeQueryString(s string) *gradioObject {
	out := newObject()
	if s == "" {
		return out
	}
	for i, pair := range strings.Split(s, "&") {
		if i >= 1000 {
			break
		}
		if pair == "" {
			continue
		}
		k, v, _ := strings.Cut(pair, "=")
		key, value := unescapeQuery(k), unescapeQuery(v)
		switch existing := out.get(key).(type) {
		case undefinedType:
			out.set(key, value)
		case []any:
			out.set(key, append(existing, value))
		default:
			out.set(key, []any{existing, value})
		}
	}
	return out
}

func unescapeQuery(s string) string {
	if u, err := url.QueryUnescape(s); err == nil {
		return u
	}
	return strings.ReplaceAll(s, "+", " ")
}

// writeNextJSON is res.status(code).json(v) as the original Next route sent it:
// with its ETag (and 304 when the browser already has it), compressed from
// 1 KB up.
func writeNextJSON(w http.ResponseWriter, r *http.Request, code int, v any) {
	h := w.Header()
	h.Set("Content-Type", "application/json; charset=utf-8")
	h.Set("Vary", "Accept-Encoding")
	text, ok := stringifyJSON(v)
	if !ok {
		w.WriteHeader(code) // res.end() with no body
		return
	}
	etag := nextETag(text)
	h["ETag"] = []string{etag} // Next's spelling; Set would send "Etag"
	if etagFresh(r.Header, etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	if len(text) >= 1024 && r.Method != http.MethodHead {
		if enc := negotiateEncoding(r.Header.Get("Accept-Encoding")); enc != "" {
			h.Set("Content-Encoding", enc)
			w.WriteHeader(code)
			var zw io.WriteCloser
			if enc == "gzip" {
				zw = gzip.NewWriter(w)
			} else {
				zw = zlib.NewWriter(w)
			}
			_, _ = io.WriteString(zw, text)
			_ = zw.Close()
			return
		}
	}
	h.Set("Content-Length", fmt.Sprint(len(text)))
	w.WriteHeader(code)
	_, _ = io.WriteString(w, text)
}

// nextETag is generateETag from next/dist/server/lib/etag.js: FNV-1a 52 over
// the UTF-16 code units, then the length, both in base 36.
func nextETag(s string) string {
	units := utf16.Encode([]rune(s))
	var v0, v1, v2, v3 uint64 = 0x2325, 0x8422, 0x9ce4, 0xcbf2
	for _, u := range units {
		v0 ^= uint64(u)
		t0 := v0 * 435
		t1 := v1 * 435
		t2 := v2 * 435
		t3 := v3 * 435
		t2 += v0 << 8
		t3 += v1 << 8
		t1 += t0 >> 16
		v0 = t0 & 65535
		t2 += t1 >> 16
		v1 = t1 & 65535
		v3 = (t3 + (t2 >> 16)) & 65535
		v2 = t2 & 65535
	}
	hash := (v3&15)*281474976710656 + v2*4294967296 + v1*65536 + (v0 ^ (v3 >> 4))
	return `"` + strconv.FormatUint(hash, 36) + strconv.FormatInt(int64(len(units)), 36) + `"`
}

// etagFresh is the `fresh` package's If-None-Match check.
func etagFresh(h http.Header, etag string) bool {
	noneMatch := h.Get("If-None-Match")
	if noneMatch == "" {
		return false
	}
	if cc := h.Get("Cache-Control"); strings.Contains(cc, "no-cache") {
		return false
	}
	if strings.TrimSpace(noneMatch) == "*" {
		return true
	}
	for _, tag := range strings.Split(noneMatch, ",") {
		tag = strings.TrimSpace(tag)
		if tag == etag || tag == "W/"+etag || "W/"+tag == etag {
			return true
		}
	}
	return false
}

// negotiateEncoding picks gzip or deflate as the compression middleware does:
// the highest quality wins, ties go to gzip, then deflate.
func negotiateEncoding(accept string) string {
	best, bestQ := "", 0.0
	for _, enc := range []string{"gzip", "deflate"} {
		q := 0.0
		for _, part := range strings.Split(accept, ",") {
			name, params, _ := strings.Cut(strings.TrimSpace(part), ";")
			if !strings.EqualFold(strings.TrimSpace(name), enc) && strings.TrimSpace(name) != "*" {
				continue
			}
			pq := 1.0
			if k, v, ok := strings.Cut(strings.TrimSpace(params), "="); ok && strings.TrimSpace(k) == "q" {
				if f, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
					pq = f
				}
			}
			if pq > q {
				q = pq
			}
		}
		if q > bestQ {
			best, bestQ = enc, q
		}
	}
	return best
}

// sendNextError is Next's sendError: the message is the whole body, sent
// chunked and without a Content-Type.
func sendNextError(w http.ResponseWriter, code int, message string) {
	w.Header()["Content-Type"] = nil // stop Go from sniffing one
	w.WriteHeader(code)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	_, _ = io.WriteString(w, message)
}

// --- one call ---

type gradioCall struct {
	route *gradioRoute
	ctx   context.Context
	// selfHost is the Host of the request: the origin the UI was served from.
	// getFile may fetch it (see gradio_fetch_guard.go).
	selfHost string
}

func (c *gradioCall) connect() (*gradioClient, error) {
	var auth []string
	if c.route.auth != "" {
		auth = strings.Split(c.route.auth, ":")
	}
	return connectGradio(c.ctx, c.route.backend, auth)
}

// gradioPredict connects, predicts and proxies file URLs in the result.
func (c *gradioCall) gradioPredict(endpoint string, data any) (any, error) {
	client, err := c.connect()
	if err != nil {
		return nil, errReject{err.Error()}
	}
	defer client.close()

	var result *gradioObject
	dataReturned, statusComplete := false, false
	var outcome error = errHang
	_, err = client.submit(c.ctx, endpoint, data, func(e gradioEvent) bool {
		switch e.kind {
		case "data":
			dataReturned = true
			result = e.msg
		case "status":
			if e.stage == "error" {
				outcome = errReject{e.msg}
				return true
			}
			if e.stage == "complete" {
				statusComplete = true
				if dataReturned {
					outcome = nil
					return true
				}
			}
		}
		return false
	})
	if err != nil {
		var thrown errSubmitThrow
		if errors.As(err, &thrown) {
			return nil, errHang
		}
		return nil, err
	}
	_ = statusComplete
	if outcome != nil {
		return nil, outcome
	}
	return proxyGradioFilesInResult(result, c.route.backend, c.route.engine)
}

// gradioSubmitAll is gradioSubmit followed by Array__fromAsync: every data
// event until the job closes, each copied through JSON.
func (c *gradioCall) gradioSubmitAll(endpoint string, data any) ([]any, error) {
	client, err := c.connect()
	if err != nil {
		return nil, errReject{err.Error()}
	}
	defer client.close()

	var events []any
	closed, err := client.submit(c.ctx, endpoint, data, func(e gradioEvent) bool {
		if e.kind == "data" {
			s, _ := stringifyJSON(e.msg)
			v, _ := parseJSON([]byte(s))
			events = append(events, v)
		}
		return false
	})
	if err != nil {
		var thrown errSubmitThrow
		if errors.As(err, &thrown) {
			return nil, errReject{thrown.Error()}
		}
		return nil, err
	}
	if !closed {
		return nil, errHang
	}
	return events, nil
}

// proxyGradioFilesInResult rewrites Gradio file URLs to /api/gradio-file-proxy/.
func proxyGradioFilesInResult(result any, backend, engine string) (any, error) {
	view, err := objectView(result)
	if err != nil {
		return nil, err
	}
	out := view.clone()
	data, ok := out.get("data").([]any)
	if !ok {
		return out, nil
	}
	mapped := make([]any, len(data))
	for i, d := range data {
		if mapped[i], err = proxyGradioFile(d, backend, engine); err != nil {
			return nil, err
		}
	}
	out.set("data", mapped)
	return out, nil
}

// proxyGradioFile is backend-utils/proxyGradioFile.tsx. typeof null is
// "object", so a null entry throws when .url is read.
func proxyGradioFile(d any, backend, engine string) (any, error) {
	switch d.(type) {
	case nil:
		return nil, typeError("Cannot read properties of null (reading 'url')")
	case *gradioObject, []any:
	default:
		return d, nil
	}
	u, err := prop(d, "url")
	if err != nil {
		return nil, err
	}
	s, ok := u.(string)
	if !ok || !strings.HasPrefix(s, backend) {
		return d, nil
	}
	copied, _ := objectView(d)
	copied = copied.clone()
	prefix := "/api/gradio-file-proxy/"
	if engine != "" {
		prefix += engineFilePrefix + engine + "/"
	}
	copied.set("url", strings.Replace(s, backend, prefix, 1))
	return copied, nil
}

// getFile is backend-utils/getFile.tsx: a URL is fetched, anything else is
// read from react-ui/public.
func (c *gradioCall) getFile(file any) (any, error) {
	if !isTruthy(file) {
		return nil, nil
	}
	name, ok := file.(string)
	if !ok {
		return nil, typeError("file.startsWith is not a function")
	}
	if strings.HasPrefix(name, "http") || strings.HasPrefix(name, "data") {
		return c.readFromURL(name)
	}
	return c.readFromDisk(name)
}

func (c *gradioCall) readFromDisk(name string) (any, error) {
	base, err := filepath.Abs(c.route.publicDir)
	if err != nil {
		return nil, err
	}
	target := filepath.Join(base, name)
	if rel, err := filepath.Rel(base, target); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("refusing to read %q outside react-ui/public", name)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		return nil, err
	}
	return jsBuffer{data}, nil
}

func (c *gradioCall) readFromURL(raw string) (any, error) {
	if strings.HasPrefix(raw, "data:") {
		data, err := decodeDataURL(raw)
		if err != nil {
			return nil, err
		}
		return jsBlob{data}, nil
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("fetch failed: invalid URL %q", raw)
	}
	req, err := http.NewRequestWithContext(c.ctx, http.MethodGet, raw, nil)
	if err != nil {
		return nil, err
	}
	resp, err := fileFetchClient(c.selfHost, c.route.backend).Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch failed: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return jsBlob{data}, nil
}

// decodeDataURL decodes data:[<mediatype>][;base64],<data>.
func decodeDataURL(raw string) ([]byte, error) {
	header, payload, ok := strings.Cut(strings.TrimPrefix(raw, "data:"), ",")
	if !ok {
		return nil, errors.New("fetch failed: invalid data URL")
	}
	if strings.HasSuffix(strings.ToLower(strings.TrimSpace(header)), ";base64") {
		cleaned := strings.Map(func(r rune) rune {
			if r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\f' {
				return -1
			}
			return r
		}, payload)
		if dec, err := url.PathUnescape(cleaned); err == nil {
			cleaned = dec
		}
		data, err := base64.StdEncoding.DecodeString(padBase64(cleaned))
		if err != nil {
			return nil, errors.New("fetch failed: invalid base64")
		}
		return data, nil
	}
	dec, err := url.PathUnescape(payload)
	if err != nil {
		return nil, errors.New("fetch failed: invalid data URL")
	}
	return []byte(dec), nil
}

func padBase64(s string) string {
	if m := len(s) % 4; m != 0 {
		s += strings.Repeat("=", 4-m)
	}
	return s
}

// --- helpers mirroring the JavaScript in [name].tsx ---

// field reads a destructured parameter, like ({ name }) => ...
func field(params any, name string) (any, error) {
	view, err := objectView(params)
	if err != nil {
		return nil, err
	}
	return view.get(name), nil
}

// rest is the ...params of a destructuring pattern.
func rest(params any, exclude ...string) (*gradioObject, error) {
	view, err := objectView(params)
	if err != nil {
		return nil, err
	}
	out := newObject()
	for _, k := range view.ownKeys() {
		skip := false
		for _, e := range exclude {
			if k == e {
				skip = true
			}
		}
		if !skip {
			out.set(k, view.get(k))
		}
	}
	return out, nil
}

// destructure is const [a, b, ...] = v for an iterable v.
func destructure(v any, n int) ([]any, error) {
	out := make([]any, n)
	for i := range out {
		out[i] = undefined
	}
	switch t := v.(type) {
	case []any:
		copy(out, t)
	case string:
		units := utf16Units(t)
		for i := 0; i < n && i < len(units); i++ {
			out[i] = units[i]
		}
	case nil, undefinedType:
		return nil, typeError("%s is not iterable", jsTypeName(v))
	default:
		return nil, typeError("object is not iterable")
	}
	return out, nil
}

// resultData is result?.data for a predict result.
func resultData(result any) any { return optProp(result, "data") }

// first is result?.data[0].
func first(result any) (any, error) { return index(resultData(result), 0) }

// optFirst is result?.data?.[0].
func optFirst(result any) any { return optProp(resultData(result), "0") }

func (c *gradioCall) resolveFileParams(params any) (*gradioObject, error) {
	view, err := objectView(params)
	if err != nil {
		return nil, err
	}
	out := newObject()
	for _, k := range view.ownKeys() {
		v := view.get(k)
		if _, isObject := v.(*gradioObject); isObject || v == nil {
			typ, err := prop(v, "__type")
			if err != nil {
				return nil, err
			}
			if typ == "local_cache_file" {
				path, _ := prop(v, "path")
				if v, err = c.getFile(path); err != nil {
					return nil, err
				}
			}
		}
		out.set(k, v)
	}
	return out, nil
}

type mapFn func(any) (any, error)

func identity(v any) (any, error) { return v, nil }

func extractChoicesAt(i int) mapFn {
	return func(result any) (any, error) {
		update, err := first(result)
		if err != nil {
			return nil, err
		}
		choices, err := field(update, "choices")
		if err != nil {
			return nil, err
		}
		list, ok := choices.([]any)
		if !ok {
			return nil, typeError("choices.map is not a function")
		}
		out := make([]any, len(list))
		for j, x := range list {
			if out[j], err = index(x, i); err != nil {
				return nil, err
			}
		}
		return out, nil
	}
}

var (
	getChoices          = extractChoicesAt(0)
	extractChoicesTuple = extractChoicesAt(1)
)

func dataOf(result any) (any, error) { return resultData(result), nil }

func firstOf(result any) (any, error) { return first(result) }

type endpointFn func(c *gradioCall, params any) (any, error)

func simpleOutputMap(result any) (any, error) {
	parts, err := destructure(resultData(result), 3)
	if err != nil {
		return nil, err
	}
	return obj("audio", parts[0], "metadata", parts[1], "folder_root", parts[2]), nil
}

func simpleEndpoint(endpoint string) endpointFn {
	return func(c *gradioCall, params any) (any, error) {
		result, err := c.gradioPredict(endpoint, params)
		if err != nil {
			return nil, err
		}
		return simpleOutputMap(result)
	}
}

func passThrough(endpoint string) endpointFn {
	return func(c *gradioCall, params any) (any, error) {
		return c.gradioPredict(endpoint, params)
	}
}

func resolvedPassThrough(endpoint string, m mapFn) endpointFn {
	return func(c *gradioCall, params any) (any, error) {
		resolved, err := c.resolveFileParams(params)
		if err != nil {
			return nil, err
		}
		result, err := c.gradioPredict(endpoint, resolved)
		if err != nil {
			return nil, err
		}
		return m(result)
	}
}

func resolvedPassThroughWithParamsLoopback(endpoint string, m mapFn) endpointFn {
	return func(c *gradioCall, params any) (any, error) {
		resolved, err := c.resolveFileParams(params)
		if err != nil {
			return nil, err
		}
		result, err := c.gradioPredict(endpoint, resolved)
		if err != nil {
			return nil, err
		}
		mapped, err := m(result)
		if err != nil {
			return nil, err
		}
		out, err := objectView(mapped)
		if err != nil {
			out = newObject()
		}
		out = out.clone()
		out.set("metadata", params)
		return out, nil
	}
}

// fields reads several destructured parameters at once.
func fields(params any, names ...string) (map[string]any, error) {
	view, err := objectView(params)
	if err != nil {
		return nil, err
	}
	out := map[string]any{}
	for _, n := range names {
		out[n] = view.get(n)
	}
	return out, nil
}

func musicgen(c *gradioCall, params any) (any, error) {
	f, err := fields(params, "melody", "model")
	if err != nil {
		return nil, err
	}
	others, _ := rest(params, "melody", "model")
	melodyBlob, err := c.getFile(f["melody"])
	if err != nil {
		return nil, err
	}
	data := obj("melody", melodyBlob, "model_name", f["model"])
	spread(data, others)
	result, err := c.gradioPredict("/musicgen", data)
	if err != nil {
		return nil, err
	}
	return simpleOutputMap(result)
}

func stableAudioGenerate(c *gradioCall, params any) (any, error) {
	f, err := fields(params, "init_audio", "text")
	if err != nil {
		return nil, err
	}
	others, _ := rest(params, "init_audio", "text")
	exampleAudio, err := c.getFile(f["init_audio"])
	if err != nil {
		return nil, err
	}
	data := obj("prompt", f["text"], "init_audio", exampleAudio)
	spread(data, others)
	result, err := c.gradioPredict("/stable_audio_generate", data)
	if err != nil {
		return nil, err
	}
	parts, err := destructure(resultData(result), 2)
	if err != nil {
		return nil, err
	}
	return obj("audio", parts[0], "gallery", parts[1]), nil
}

func barkVoiceTokenizerLoad(c *gradioCall, params any) (any, error) {
	f, err := fields(params, "tokenizer", "use_gpu")
	if err != nil {
		return nil, err
	}
	result, err := c.gradioPredict("/bark_voice_tokenizer_load", []any{f["tokenizer"], f["use_gpu"]})
	if err != nil {
		return nil, err
	}
	return resultData(result), nil
}

func barkVoiceGenerate(c *gradioCall, params any) (any, error) {
	f, err := fields(params, "audio", "use_gpu")
	if err != nil {
		return nil, err
	}
	audioBlob, err := c.getFile(f["audio"])
	if err != nil {
		return nil, err
	}
	result, err := c.gradioPredict("/bark_voice_generate", []any{audioBlob, f["use_gpu"]})
	if err != nil {
		return nil, err
	}
	parts, err := destructure(resultData(result), 2)
	if err != nil {
		return nil, err
	}
	return obj("filename", parts[0], "preview", parts[1]), nil
}

func bark(c *gradioCall, params any) (any, error) {
	names := []string{"seed", "text", "burn_in_prompt", "text_temp", "waveform_temp", "max_length",
		"history_prompt", "history_prompt_semantic", "long_prompt_radio", "long_prompt_history_radio"}
	f, err := fields(params, names...)
	if err != nil {
		return nil, err
	}
	data := newObject()
	for _, n := range names {
		data.set(n, f[n])
	}
	result, err := c.gradioPredict("/bark", data)
	if err != nil {
		return nil, err
	}
	parts, err := destructure(resultData(result), 4)
	if err != nil {
		return nil, err
	}
	audio := newObject()
	if parts[0] != nil && parts[0] != undefined {
		view, _ := objectView(parts[0])
		audio = view.clone()
	}
	audioURL, err := prop(parts[0], "url")
	if err != nil {
		return nil, err
	}
	audio.set("data", audioURL)
	return obj("audio", audio, "npz", parts[1], "metadata", parts[2], "folder_root", parts[3]), nil
}

func barkFavorite(c *gradioCall, params any) (any, error) {
	folderRoot, err := field(params, "folder_root")
	if err != nil {
		return nil, err
	}
	result, err := c.gradioPredict("/bark_favorite", []any{folderRoot})
	if err != nil {
		return nil, err
	}
	return resultData(result), nil
}

func tortoise(c *gradioCall, params any) (any, error) {
	names := []string{"text", "speaker", "preset", "seed", "cvvp_amount", "split_prompt", "samples",
		"diffusion_iterations", "temperature", "length_penalty", "repetition_penalty", "top_p",
		"max_mel_tokens", "cond_free", "cond_free_k", "diffusion_temperature", "model", "generation_name"}
	f, err := fields(params, append(names, "candidates")...)
	if err != nil {
		return nil, err
	}
	data := make([]any, len(names))
	for i, n := range names {
		data[i] = f[n]
	}
	events, err := c.gradioSubmitAll("/generate_tortoise_"+jsToString(f["candidates"]), data)
	if err != nil {
		return nil, err
	}
	results := []any{}
	for _, e := range events {
		if t, _ := prop(e, "type"); t != "data" {
			continue
		}
		proxied, err := proxyGradioFilesInResult(e, c.route.backend, c.route.engine)
		if err != nil {
			return nil, err
		}
		parts, err := destructure(optProp(proxied, "data"), 3)
		if err != nil {
			return nil, err
		}
		results = append(results, obj("audio", parts[0], "seed", f["seed"], "bundle_name", parts[1], "metadata", parts[2]))
	}
	if len(results) == 0 {
		return results, nil
	}
	return results[:len(results)-1], nil
}

func tortoiseApplyModelSettings(c *gradioCall, params any) (any, error) {
	f, err := fields(params, "model", "kv_cache", "use_deepspeed", "half", "tokenizer", "use_basic_cleaners")
	if err != nil {
		return nil, err
	}
	tokenizerFile, err := c.getFile(f["tokenizer"])
	if err != nil {
		return nil, err
	}
	result, err := c.gradioPredict("/tortoise_apply_model_settings",
		[]any{f["model"], f["kv_cache"], f["use_deepspeed"], f["half"], tokenizerFile, f["use_basic_cleaners"]})
	if err != nil {
		return nil, err
	}
	return resultData(result), nil
}

func rvc(c *gradioCall, params any) (any, error) {
	f, err := fields(params, "pitch_up_key", "original_audio", "index", "pitch_collection_method", "model",
		"search_feature_ratio", "filter_radius_pitch", "resample_sample_rate", "voice_envelope_normalizaiton", "protect_breath_sounds")
	if err != nil {
		return nil, err
	}
	blob, err := c.getFile(f["original_audio"])
	if err != nil {
		return nil, err
	}
	result, err := c.gradioPredict("/rvc", obj(
		"pitch_up_key", f["pitch_up_key"],
		"original_audio_path", blob,
		"index_path", f["index"],
		"pitch_collection_method", f["pitch_collection_method"],
		"model_path", f["model"],
		"index_rate", f["search_feature_ratio"],
		"filter_radius", f["filter_radius_pitch"],
		"resample_sr", f["resample_sample_rate"],
		"rms_mix_rate", f["voice_envelope_normalizaiton"],
		"protect", f["protect_breath_sounds"],
	))
	if err != nil {
		return nil, err
	}
	parts, err := destructure(resultData(result), 2)
	if err != nil {
		return nil, err
	}
	return obj("audio", parts[0], "metadata", parts[1]), nil
}

func aceStepInfer(c *gradioCall, params any) (any, error) {
	f, err := fields(params, "audio_duration", "tags", "lyrics", "infer_steps", "guidance_scale", "scheduler_type",
		"cfg_type", "granularity_scale", "manual_seeds", "guidance_interval", "guidance_interval_decay",
		"min_guidance_scale", "use_erg_for_tag", "use_erg_for_lyric", "use_erg_for_diffusion", "oss_steps",
		"guidance_scale_text", "guidance_scale_lyric", "audio2audio_enable", "ref_audio_strength", "ref_audio_input")
	if err != nil {
		return nil, err
	}
	blob, err := c.getFile(f["ref_audio_input"])
	if err != nil {
		return nil, err
	}
	result, err := c.gradioPredict("/ace_step_infer", obj(
		"audio_duration", f["audio_duration"],
		"prompt", f["tags"],
		"lyrics", f["lyrics"],
		"infer_step", f["infer_steps"],
		"guidance_scale", f["guidance_scale"],
		"scheduler_type", f["scheduler_type"],
		"cfg_type", f["cfg_type"],
		"omega_scale", f["granularity_scale"],
		"manual_seeds", f["manual_seeds"],
		"guidance_interval", f["guidance_interval"],
		"guidance_interval_decay", f["guidance_interval_decay"],
		"min_guidance_scale", f["min_guidance_scale"],
		"use_erg_tag", f["use_erg_for_tag"],
		"use_erg_lyric", f["use_erg_for_lyric"],
		"use_erg_diffusion", f["use_erg_for_diffusion"],
		"oss_steps", f["oss_steps"],
		"guidance_scale_text", f["guidance_scale_text"],
		"guidance_scale_lyric", f["guidance_scale_lyric"],
		"audio2audio_enable", f["audio2audio_enable"],
		"ref_audio_strength", f["ref_audio_strength"],
		"ref_audio_input", blob,
	))
	if err != nil {
		return nil, err
	}
	parts, err := destructure(resultData(result), 2)
	if err != nil {
		return nil, err
	}
	return obj("audio", parts[0], "metadata", parts[1]), nil
}

func deleteGeneration(c *gradioCall, params any) (any, error) {
	folderRoot, err := field(params, "folder_root")
	if err != nil {
		return nil, err
	}
	return c.gradioPredict("/delete_generation", []any{folderRoot})
}

func getConfigBark(c *gradioCall, _ any) (any, error) {
	result, err := c.gradioPredict("/get_config_bark", []any{})
	if err != nil {
		return nil, err
	}
	data, ok := resultData(result).([]any)
	if !ok {
		return nil, typeError("Cannot read properties of undefined (reading 'map')")
	}
	values := make([]any, len(data))
	for i, x := range data {
		if values[i], err = prop(x, "value"); err != nil {
			return nil, err
		}
	}
	parts, _ := destructure(values, 8)
	return obj(
		"text_use_gpu", parts[0], "text_use_small", parts[1],
		"coarse_use_gpu", parts[2], "coarse_use_small", parts[3],
		"fine_use_gpu", parts[4], "fine_use_small", parts[5],
		"codec_use_gpu", parts[6], "load_models_on_startup", parts[7],
	), nil
}

func scanHuggingfaceCacheAPI(c *gradioCall, _ any) (any, error) {
	result, err := c.gradioPredict("/scan_huggingface_cache_api", undefined)
	if err != nil {
		return nil, err
	}
	return optFirst(result), nil
}

func deleteHuggingfaceCacheRevisions(c *gradioCall, params any) (any, error) {
	commitHash, err := field(params, "commit_hash")
	if err != nil {
		return nil, err
	}
	return c.gradioPredict("/delete_huggingface_cache_revisions", []any{commitHash})
}

func musicgenAudiogenGetModels(c *gradioCall, _ any) (any, error) {
	result, err := c.gradioPredict("/musicgen_audiogen_get_models", undefined)
	if err != nil {
		return nil, err
	}
	return extractChoicesTuple(result)
}

func objectOf(kv ...any) mapFn {
	return func(result any) (any, error) {
		out := newObject()
		for i := 0; i+1 < len(kv); i += 2 {
			v, err := kv[i+1].(mapFn)(result)
			if err != nil {
				return nil, err
			}
			out.set(kv[i].(string), v)
		}
		return out, nil
	}
}

func optDataAt(i int) mapFn {
	return func(result any) (any, error) { return optProp(resultData(result), fmt.Sprint(i)), nil }
}

// gradioEndpoints is the endpoints table, in the original's order.
var gradioEndpoints = map[string]endpointFn{
	"maha":                    simpleEndpoint("/maha_tts"),
	"maha_tts_refresh_voices": resolvedPassThrough("/maha_tts_refresh_voices", getChoices),
	"demucs":                  resolvedPassThrough("/demucs", dataOf),
	"musicgen":                musicgen,
	"magnet":                  simpleEndpoint("/magnet"),
	"magnet_get_models":       resolvedPassThrough("/magnet_get_models", extractChoicesTuple),
	"magnet_open_model_dir":   passThrough("/magnet_open_model_dir"),
	"magnet_unload_model":     passThrough("/magnet_unload_model"),

	"vocos_wav":      resolvedPassThrough("/vocos_wav", firstOf),
	"vocos_npz":      resolvedPassThrough("/vocos_npz", firstOf),
	"encodec_decode": resolvedPassThrough("/encodec_decode", firstOf),

	"bark_voice_tokenizer_load":      barkVoiceTokenizerLoad,
	"bark_voice_generate":            barkVoiceGenerate,
	"bark":                           bark,
	"reload_old_generation_dropdown": resolvedPassThrough("/reload_old_generation_dropdown", getChoices),
	"bark_favorite":                  barkFavorite,
	"delete_generation":              deleteGeneration,

	"tortoise":                      tortoise,
	"tortoise_refresh_models":       resolvedPassThrough("/tortoise_refresh_models", getChoices),
	"tortoise_refresh_voices":       resolvedPassThrough("/tortoise_refresh_voices", getChoices),
	"tortoise_open_models":          passThrough("/tortoise_open_models"),
	"tortoise_open_voices":          passThrough("/tortoise_open_voices"),
	"tortoise_apply_model_settings": tortoiseApplyModelSettings,

	"rvc":              rvc,
	"rvc_model_reload": resolvedPassThrough("/rvc_model_reload", getChoices),
	"rvc_index_reload": resolvedPassThrough("/rvc_index_reload", getChoices),
	"rvc_model_open":   passThrough("/rvc_model_open"),
	"rvc_index_open":   passThrough("/rvc_index_open"),
	"rvc_model_unload": passThrough("/rvc_model_unload"),
	"save_to_voices":   passThrough("/save_to_voices"),
	"open_folder":      passThrough("/open_folder"),

	"save_env_variables_bark": resolvedPassThrough("/save_env_variables_bark", dataOf),
	"save_config_bark":        resolvedPassThrough("/save_config_bark", firstOf),
	"get_config_bark":         getConfigBark,

	"mms": simpleEndpoint("/mms"),

	"get_gpu_info": resolvedPassThrough("/get_gpu_info", firstOf),

	"vall_e_x_generate":                  simpleEndpoint("/vall_e_x_generate"),
	"vall_e_x_split_text_into_sentences": resolvedPassThrough("/vall_e_x_split_text_into_sentences", objectOf("split_text", optDataAt(0))),
	"vall_e_x_tokenize":                  resolvedPassThrough("/vall_e_x_tokenize", objectOf("tokens", optDataAt(0))),

	"scan_huggingface_cache_api":         scanHuggingfaceCacheAPI,
	"delete_huggingface_cache_revisions": deleteHuggingfaceCacheRevisions,

	"tortoise_unload_model": passThrough("/tortoise_unload_model"),

	"musicgen_audiogen_get_models":     musicgenAudiogenGetModels,
	"musicgen_audiogen_open_model_dir": passThrough("/musicgen_audiogen_open_model_dir"),
	"musicgen_audiogen_unload_model":   passThrough("/musicgen_audiogen_unload_model"),
	"stable_audio_generate":            stableAudioGenerate,
	"stable_audio_inpaint":             passThrough("/stable_audio_inpaint"),
	"stable_audio_get_models":          resolvedPassThrough("/stable_audio_get_models", extractChoicesTuple),

	"ace_step_infer": aceStepInfer,

	"kokoro_generate": resolvedPassThroughWithParamsLoopback("/kokoro", objectOf(
		"audio", optDataAt(0), "tokens", optDataAt(1), "metadata", optDataAt(2), "folder_root", optDataAt(3))),
	"chatterbox_tts": resolvedPassThroughWithParamsLoopback("/chatterbox_tts", objectOf(
		"audio", optDataAt(0), "metadata", optDataAt(1), "folder_root", optDataAt(2))),
	"minimax_cloud_tts": resolvedPassThroughWithParamsLoopback("/minimax_cloud_tts", objectOf(
		"audio", optDataAt(0), "metadata", optDataAt(1), "folder_root", optDataAt(2))),

	"open_ai_api_load_presets":      resolvedPassThrough("/open_ai_api_load_presets", firstOf),
	"open_ai_api_save_presets":      passThrough("/open_ai_api_save_presets"),
	"get_chatterbox_voices":         resolvedPassThrough("/get_chatterbox_voices", firstOf),
	"open_ai_api_test_voice_preset": resolvedPassThrough("/open_ai_api_test_voice_preset", firstURL),
}

// firstURL is result?.data[0].url.
func firstURL(result any) (any, error) {
	v, err := first(result)
	if err != nil {
		return nil, err
	}
	return prop(v, "url")
}
