// Package extui serves the Extensions List page that Gradio shows in an iframe,
// from the same origin as Gradio (the router sends /_core to this handler and
// strips the prefix). It is the browser-facing side of extcatalog: the control
// API stays loopback plus a token only Python holds, and this handler has its
// own, narrower rules (see the policy below).
//
// Policy, in the order a request meets it:
//
//   - Host: a loopback name and the public port (a DNS-rebinding guard). When
//     the router listens beyond loopback (Exposed), any host passes and the
//     page is read-only, because nothing here can tell a trusted network from
//     an open one.
//   - The page sets a random cookie (HttpOnly, SameSite=Strict, Path=/_core/, so
//     Gradio and the extensions never receive it) and embeds a second random
//     token. Every /ext/api/* request needs the cookie; a POST also needs the
//     token in X-TTS-UI, an Origin that matches the Host, a JSON body, and
//     Sec-Fetch-Site not cross-site. Both secrets are new on every start.
//   - Only install and uninstall by package name are passed on. install-entry,
//     parse and add (pasted requirements or text) are not reachable from here.
//   - Responses carry a CSP that allows only this origin's scripts and styles
//     and only the same origin as a frame ancestor, so another site cannot
//     embed the page, and extension text is only ever set as text.
package extui

import (
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"mime"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/rsxdalv/TTS-WebUI/core/internal/platform"
	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/appconfig"
	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/extcatalog"
)

// Prefix is the router route that sends requests here. The router strips it.
const Prefix = "/_core"

//go:embed assets
var assetsFS embed.FS

// ConfigSource gives the disabled lists (appconfig.Store).
type ConfigSource interface {
	ExtensionLists() (appconfig.ExtensionLists, error)
}

// Options configures a Handler.
type Options struct {
	Cat      *extcatalog.Store
	CatToken string // the store's own token, added when an operation is passed on
	Cfg      ConfigSource
	// Port is the public port the browser uses (the router's).
	Port int
	// Exposed says the router listens beyond loopback: the page is read-only
	// and any Host is accepted.
	Exposed bool
	Logf    func(format string, args ...any)
}

// Handler serves /ext/*.
type Handler struct {
	opts   Options
	cookie string // cookie name
	secret string // cookie value
	token  string // X-TTS-UI value
	html   string
	mux    *http.ServeMux
}

// allowedOps are the extcatalog operations the page may pass on, each by package name.
var allowedOps = map[string]bool{"install": true, "uninstall": true}

var kinds = map[string]bool{"interface": true, "decorator": true}

func randomHex() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("extui: generating a secret: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// New builds the handler with fresh secrets.
func New(o Options) (*Handler, error) {
	if o.Logf == nil {
		o.Logf = func(string, ...any) {}
	}
	secret, err := randomHex()
	if err != nil {
		return nil, err
	}
	token, err := randomHex()
	if err != nil {
		return nil, err
	}
	page, err := assetsFS.ReadFile("assets/list.html")
	if err != nil {
		return nil, err
	}
	h := &Handler{
		opts: o, secret: secret, token: token, html: string(page),
		cookie: fmt.Sprintf("tts-ui-%d", o.Port), mux: http.NewServeMux(),
	}
	assets, err := fs.Sub(assetsFS, "assets")
	if err != nil {
		return nil, err
	}
	static := http.FileServerFS(assets)
	h.mux.HandleFunc("GET /ext/list", h.servePage)
	h.mux.Handle("GET /ext/ui.js", http.StripPrefix("/ext/", static))
	h.mux.Handle("GET /ext/ui.css", http.StripPrefix("/ext/", static))
	h.mux.HandleFunc("GET /ext/api/list", h.guarded(false, h.serveList))
	h.mux.HandleFunc("POST /ext/api/op/{op}", h.guarded(true, h.serveOp))
	return h, nil
}

// ServeHTTP applies the Host rule and the response headers to every request.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	hd := w.Header()
	hd.Set("Cache-Control", "no-store")
	hd.Set("X-Content-Type-Options", "nosniff")
	hd.Set("Referrer-Policy", "no-referrer")
	hd.Set("X-Frame-Options", "SAMEORIGIN")
	hd.Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; style-src 'self'; connect-src 'self'; "+
		"img-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'self'")
	if !h.validHost(r.Host) {
		http.Error(w, "bad request: invalid Host", http.StatusBadRequest)
		return
	}
	h.mux.ServeHTTP(w, r)
}

func (h *Handler) validHost(host string) bool {
	if h.opts.Exposed {
		return true
	}
	name, port, err := net.SplitHostPort(host)
	if err != nil || port != fmt.Sprint(h.opts.Port) {
		return false
	}
	return name == "127.0.0.1" || name == "localhost" || name == "::1"
}

func (h *Handler) servePage(w http.ResponseWriter, r *http.Request) {
	kind := r.URL.Query().Get("kind")
	if !kinds[kind] {
		http.Error(w, "kind must be interface or decorator", http.StatusBadRequest)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: h.cookie, Value: h.secret, Path: Prefix + "/",
		HttpOnly: true, SameSite: http.SameSiteStrictMode,
	})
	page := strings.NewReplacer("{{TOKEN}}", h.token, "{{KIND}}", kind).Replace(h.html)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprint(w, page)
}

func eq(a, b string) bool { return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1 }

// guarded runs the API rules before next. A mutating request also needs the
// token, a matching Origin, a JSON body and an app that is closed to other devices.
func (h *Handler) guarded(mutating bool, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(h.cookie)
		if err != nil || !eq(c.Value, h.secret) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "open this page from the app's Extensions tab"})
			return
		}
		if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "cross-site request"})
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" || mutating {
			u, err := url.Parse(origin)
			if err != nil || origin == "" || u.Host != r.Host || (u.Scheme != "http" && u.Scheme != "https") {
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "bad Origin"})
				return
			}
		}
		if mutating {
			if h.opts.Exposed {
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "read-only while the app is open to other devices"})
				return
			}
			if !eq(r.Header.Get("X-TTS-UI"), h.token) {
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "missing token"})
				return
			}
			if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mt != "application/json" {
				writeJSON(w, http.StatusUnsupportedMediaType, map[string]string{"error": "Content-Type must be application/json"})
				return
			}
		}
		next(w, r)
	}
}

// Entry is one row of the list.
type Entry struct {
	PackageName string `json:"package_name"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Website     string `json:"website"`
	Class       string `json:"class"`
	Proxy       string `json:"proxy"`
	Builtin     bool   `json:"builtin"`
	Installed   bool   `json:"installed"`
	Version     string `json:"version"`
	Disabled    bool   `json:"disabled"`
}

// List is the answer of GET /ext/api/list.
type List struct {
	Kind          string   `json:"kind"`
	ReadOnly      bool     `json:"read_only"`
	Warning       string   `json:"warning,omitempty"`
	Entries       []Entry  `json:"entries"`
	Choices       []string `json:"choices"`       // what the install picker offers (extcatalog Choices)
	Uninstallable []string `json:"uninstallable"` // the installed ones among them
}

func (h *Handler) serveList(w http.ResponseWriter, r *http.Request) {
	kind := r.URL.Query().Get("kind")
	if !kinds[kind] {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "kind must be interface or decorator"})
		return
	}
	ans, err := h.list(r, kind)
	if err != nil {
		h.opts.Logf("extui: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, ans)
}

func (h *Handler) list(r *http.Request, kind string) (List, error) {
	ans := List{Kind: kind, ReadOnly: h.opts.Exposed, Entries: []Entry{}, Choices: []string{}, Uninstallable: []string{}}
	entries, err := h.opts.Cat.List(kind)
	if err != nil {
		return ans, err
	}
	choices, err := h.opts.Cat.Choices(kind)
	if err != nil {
		return ans, err
	}
	disabled := map[string]bool{}
	if lists, err := h.opts.Cfg.ExtensionLists(); err == nil {
		names := lists.Disabled
		if kind == "decorator" {
			names = lists.DisabledDecorators
		}
		for _, n := range names {
			disabled[n] = true
		}
	}
	installs := h.opts.Cat.Installs(r.Context())
	if installs.Err != nil {
		ans.Warning = "Could not read the installed packages: " + installs.Err.Error()
	}
	installed := map[string]bool{}
	for _, e := range entries {
		pkg := e.Str("package_name")
		if pkg == "" {
			continue
		}
		proxy := e.Str("proxy")
		ok, version := installs.Of(pkg, proxy)
		installed[pkg] = ok
		name := e.Str("name")
		if name == "" {
			name = pkg
		}
		ans.Entries = append(ans.Entries, Entry{
			PackageName: pkg, Name: name, Description: e.Str("description"), Website: e.Str("website"),
			Class: e.Str("extension_class"), Proxy: proxy, Builtin: platform.BuiltinTab(pkg, proxy),
			Installed: ok, Version: version, Disabled: disabled[pkg],
		})
	}
	for _, c := range choices {
		ans.Choices = append(ans.Choices, c)
		if installed[c] {
			ans.Uninstallable = append(ans.Uninstallable, c)
		}
	}
	sort.Strings(ans.Uninstallable)
	return ans, nil
}

// serveOp passes install and uninstall to extcatalog's own handler, which
// checks the package against the lists. The guard has run, so the adapter adds
// the store's internal token and marks the request loopback.
func (h *Handler) serveOp(w http.ResponseWriter, r *http.Request) {
	op := r.PathValue("op")
	if !allowedOps[op] {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	r2 := r.Clone(r.Context())
	r2.URL.Path = extcatalog.Prefix + op
	r2.URL.RawQuery = ""
	r2.Header = http.Header{"Content-Type": r.Header["Content-Type"]}
	r2.Header.Set(extcatalog.TokenHeader, h.opts.CatToken)
	r2.RemoteAddr = "127.0.0.1:0"
	h.opts.Cat.Handler().ServeHTTP(w, r2)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
