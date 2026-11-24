package launchopts

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/appenv"
)

type loader struct {
	doc string
	err error
}

func (l loader) Load() ([]byte, error) { return []byte(l.doc), l.err }

// The default gradio_interface_options of load_config.py, with overrides.
func config(extra string) string {
	base := `"inline":false,"inbrowser":true,"share":false,"debug":false,"max_threads":40,"auth":null,"auth_message":null,"prevent_thread_lock":false,"show_error":false,"server_name":"0.0.0.0","server_port":7770,"height":500,"width":"100%","favicon_path":null,"ssl_keyfile":null,"ssl_certfile":null,"ssl_keyfile_password":null,"ssl_verify":true,"quiet":true,"show_api":true,"_frontend":true`
	if extra != "" {
		base += "," + extra
	}
	return `{"gradio_interface_options":{` + base + `},"extensions":{}}`
}

func server(doc string, f appenv.Flags) *Server {
	return New(Options{Config: loader{doc: doc}, Token: "tok", GradioHost: "127.0.0.1", GradioPort: 27700, Flags: f})
}

func compute(t *testing.T, doc string, f appenv.Flags) Result {
	t.Helper()
	r, err := server(doc, f).Compute()
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func roundTrip(t *testing.T, r Result) map[string]any {
	t.Helper()
	b, err := json.Marshal(r.Options)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestPlainOptions(t *testing.T) {
	r := compute(t, config(""), appenv.Flags{})
	want := map[string]any{
		"inline": false, "inbrowser": false, "share": false, "debug": false, "max_threads": 40.0,
		"auth": nil, "auth_message": nil, "show_error": false,
		"server_name": "127.0.0.1", "server_port": 27700.0, "height": 500.0, "width": "100%",
		"ssl_verify": true,
		"quiet":      true, "show_api": true, "_frontend": true,
		"allowed_paths": []any{"."}, "favicon_path": "./react-ui/public/favicon.ico", "prevent_thread_lock": true,
	}
	if got := roundTrip(t, r); !reflect.DeepEqual(got, want) {
		t.Errorf("options:\n got %v\nwant %v", got, want)
	}
	if len(r.Lines) == 0 || !strings.HasPrefix(r.Lines[0], "Starting Gradio server: http://127.0.0.1:27700") {
		t.Errorf("lines: %q", r.Lines)
	}
	for _, l := range r.Lines {
		if strings.Contains(l, "authentication") || strings.Contains(l, "share mode") {
			t.Errorf("unexpected line %q", l)
		}
	}
}

func TestShareFlagAndPrivateAddress(t *testing.T) {
	r := compute(t, config(""), appenv.Flags{Share: true})
	if m := roundTrip(t, r); m["share"] != true {
		t.Errorf("share = %v", m["share"])
	}
	if r.Lines[0] != "Gradio share mode enabled" {
		t.Errorf("lines: %q", r.Lines)
	}
	// --docker does not change what Gradio binds: the router faces the network
	// (appenv.Plan), Gradio stays on the private address.
	d := roundTrip(t, compute(t, config(""), appenv.Flags{Docker: true}))
	p := roundTrip(t, compute(t, config(""), appenv.Flags{}))
	if !reflect.DeepEqual(d, p) {
		t.Errorf("--docker changed the options:\n%v\n%v", d, p)
	}
	// share from config.json stays
	if m := roundTrip(t, compute(t, strings.Replace(config(""), `"share":false`, `"share":true`, 1), appenv.Flags{})); m["share"] != true {
		t.Errorf("config share lost: %v", m["share"])
	}
}

func TestAuthForms(t *testing.T) {
	for name, c := range map[string]struct {
		auth string
		want any
	}{
		"string":         {`"user:pass"`, []any{[]any{"user", "pass"}}},
		"colon in pass":  {`"user:pa:ss"`, []any{[]any{"user", "pa"}}}, // gradio uses the first two parts
		"pair":           {`["u","p"]`, []any{[]any{"u", "p"}}},
		"several":        {`[["u","p"],["v","q"]]`, []any{[]any{"u", "p"}, []any{"v", "q"}}},
		"null":           {`null`, nil},
		"empty list":     {`[]`, nil},
		"empty-ish none": {`null`, nil},
	} {
		doc := strings.Replace(config(""), `"auth":null`, `"auth":`+c.auth, 1)
		r := compute(t, doc, appenv.Flags{})
		if got := roundTrip(t, r)["auth"]; !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: auth = %v, want %v", name, got, c.want)
		}
		enabled := c.want != nil
		hasLine := false
		for _, l := range r.Lines {
			hasLine = hasLine || l == "Gradio server authentication enabled"
		}
		if hasLine != enabled {
			t.Errorf("%s: authentication line = %v, want %v", name, hasLine, enabled)
		}
	}
	for _, bad := range []string{`"nocolon"`, `5`, `[["u"]]`, `["u",1]`} {
		doc := strings.Replace(config(""), `"auth":null`, `"auth":`+bad, 1)
		if _, err := server(doc, appenv.Flags{}).Compute(); err == nil {
			t.Errorf("auth %s accepted", bad)
		}
	}
}

func TestLegacyKeysAreDroppedAndFixedOnesWin(t *testing.T) {
	doc := config(`"file_directories":["x"],"show_tips":true,"enable_queue":true,"allowed_paths":["/etc"],"root_path":"/gradio"`)
	doc = strings.Replace(doc, `"favicon_path":null`, `"favicon_path":"mine.ico"`, 1)
	doc = strings.Replace(doc, `"prevent_thread_lock":false`, `"prevent_thread_lock":false`, 1)
	m := roundTrip(t, compute(t, doc, appenv.Flags{}))
	for _, k := range []string{"file_directories", "show_tips", "enable_queue"} {
		if _, ok := m[k]; ok {
			t.Errorf("%s kept", k)
		}
	}
	if !reflect.DeepEqual(m["allowed_paths"], []any{"."}) || m["favicon_path"] != "./react-ui/public/favicon.ico" || m["prevent_thread_lock"] != true {
		t.Errorf("fixed options: %v %v %v", m["allowed_paths"], m["favicon_path"], m["prevent_thread_lock"])
	}
	if m["root_path"] != "/gradio" {
		t.Errorf("an option of the file was lost: %v", m["root_path"])
	}
}

// Gradio gets the login but not the certificate: the router serves TLS and
// Gradio is plain HTTP behind it (a Gradio that also spoke TLS made the router's
// plain proxy fail with EOF).
func TestSecretsReachGradioButNotTheLines(t *testing.T) {
	doc := strings.Replace(config(`"ssl_certfile":"c.pem"`), `"auth":null`, `"auth":"admin:hunter2hunter2hunter2"`, 1)
	doc = strings.Replace(doc, `"ssl_keyfile":null`, `"ssl_keyfile":"k.pem"`, 1)
	doc = strings.Replace(doc, `"ssl_keyfile_password":null`, `"ssl_keyfile_password":"sslsecretpassword-0123456789"`, 1)
	r := compute(t, doc, appenv.Flags{})
	m := roundTrip(t, r)
	if !reflect.DeepEqual(m["auth"], []any{[]any{"admin", "hunter2hunter2hunter2"}}) {
		t.Fatalf("Gradio needs the real login: %v", m["auth"])
	}
	for _, k := range []string{"ssl_certfile", "ssl_keyfile", "ssl_keyfile_password"} {
		if _, ok := m[k]; ok {
			t.Errorf("Gradio is handed %s: the router owns TLS", k)
		}
	}
	all := strings.Join(r.Lines, "\n")
	for _, secret := range []string{"hunter2", "sslsecret", "sslsecretpas"} {
		if strings.Contains(all, secret) {
			t.Errorf("a secret is printed (%s):\n%s", secret, all)
		}
	}
	if !strings.Contains(all, "HTTPS enabled on the public port (certificate c.pem)") {
		t.Errorf("no HTTPS line:\n%s", all)
	}
}

func TestDescribeFormat(t *testing.T) {
	r := compute(t, config(`"auth_message":"a message that is longer than twenty characters"`), appenv.Flags{})
	want := []string{
		"Starting Gradio server: http://127.0.0.1:27700",
		`  • Server   > server_name: "127.0.0.1", server_port: 27700, max_threads: 40, prevent_thread_lock: True, quiet: True`,
		`  • Display  > inline: False, inbrowser: False, width: "100%", height: 500, favicon_path: "./react-ui/public...", show_api: True`,
		`  • Access   > share: False, auth_message: "a message that is..."`,
		`  • Security > ssl_verify: True, allowed_paths: ['.']`,
		`  • Debug    > debug: False, show_error: False`,
	}
	if !reflect.DeepEqual(r.Lines, want) {
		t.Errorf("lines:\n%s\nwant:\n%s", strings.Join(r.Lines, "\n"), strings.Join(want, "\n"))
	}
	open := New(Options{Config: loader{doc: config("")}, GradioHost: "0.0.0.0", GradioPort: 7767})
	res, _ := open.Compute()
	last := res.Lines[len(res.Lines)-2:]
	if last[0] != "Notice: Server is open to the internet" || last[1] != "Gradio server will be available on http://localhost:7767" {
		t.Errorf("open notice: %q", last)
	}
}

func TestInvalidConfigIsAnError(t *testing.T) {
	for name, l := range map[string]loader{
		"read error":   {err: errors.New("disk")},
		"not json":     {doc: `{`},
		"no section":   {doc: `{"extensions":{}}`},
		"no auth key":  {doc: `{"gradio_interface_options":{"server_name":"x","server_port":1}}`},
		"port a word":  {doc: strings.Replace(config(""), `"server_port":7770`, `"server_port":"abc"`, 1)},
		"port too big": {doc: strings.Replace(config(""), `"server_port":7770`, `"server_port":70000`, 1)},
	} {
		if _, err := New(Options{Config: l}).Compute(); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
}

func TestHTTP(t *testing.T) {
	h := server(config(""), appenv.Flags{}).Handler()
	do := func(method, path, token, remote string) (int, string) {
		r := httptest.NewRequest(method, path, nil)
		r.RemoteAddr = remote
		if token != "" {
			r.Header.Set(TokenHeader, token)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code, w.Body.String()
	}
	code, body := do("GET", Path, "tok", "127.0.0.1:1000")
	var res Result
	if code != 200 || json.Unmarshal([]byte(body), &res) != nil || res.Options["server_name"] != "127.0.0.1" || len(res.Lines) == 0 {
		t.Fatalf("%d %s", code, body)
	}
	for name, c := range map[string]struct {
		method, path, token, remote string
		want                        int
	}{
		"no token":     {"GET", Path, "", "127.0.0.1:1000", http.StatusForbidden},
		"wrong token":  {"GET", Path, "x", "127.0.0.1:1000", http.StatusForbidden},
		"not loopback": {"GET", Path, "tok", "10.0.0.2:1000", http.StatusForbidden},
		"other path":   {"GET", Path + "/x", "tok", "127.0.0.1:1000", http.StatusNotFound},
		"POST":         {"POST", Path, "tok", "127.0.0.1:1000", http.StatusMethodNotAllowed},
	} {
		if code, _ := do(c.method, c.path, c.token, c.remote); code != c.want {
			t.Errorf("%s: %d, want %d", name, code, c.want)
		}
	}
	bad := New(Options{Config: loader{doc: `{`}, Token: "tok"}).Handler()
	r := httptest.NewRequest("GET", Path, nil)
	r.RemoteAddr = "127.0.0.1:1000"
	r.Header.Set(TokenHeader, "tok")
	w := httptest.NewRecorder()
	bad.ServeHTTP(w, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("bad config: %d", w.Code)
	}
}
