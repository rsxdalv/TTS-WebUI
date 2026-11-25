package extcatalog

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform/legacyenv"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func entry(pkg, name string, extra ...string) string {
	s := fmt.Sprintf(`{"package_name":%q,"name":%q,"requirements":"git+https://example.com/%s@main","description":"d %s","extension_type":"interface","extension_class":"tools","author":"a","extension_author":"a","license":"MIT","website":"https://w/%s","extension_website":"https://e/%s","extension_platform_version":"0.0.1"`, pkg, name, pkg, name, pkg, pkg)
	for _, e := range extra {
		s += "," + e
	}
	return s + "}"
}

func writeFile(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func newStore(t *testing.T) (*Store, string) {
	t.Helper()
	root := t.TempDir()
	return New(Options{Root: root, Token: "tok"}), root
}

func pkgs(l []*Obj) string {
	var out []string
	for _, e := range l {
		out = append(out, e.Str("package_name"))
	}
	return strings.Join(out, ",")
}

func TestOrderedJSONRoundTrip(t *testing.T) {
	in := `{"z":1,"a":[{"y":"é","b":null,"c":true},{}],"m":{},"n":[],"s":"q\"\\\n\u0001"}`
	v, err := Parse([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	if got := string(Marshal(v, "")); got != `{"z":1,"a":[{"y":"é","b":null,"c":true},{}],"m":{},"n":[],"s":"q\"\\\n\u0001"}` {
		t.Fatalf("compact: %s", got)
	}
	want := "{\n    \"z\": 1,\n    \"a\": [\n        {\n            \"y\": \"é\",\n            \"b\": null,\n            \"c\": true\n        },\n        {}\n    ],\n    \"m\": {},\n    \"n\": [],\n    \"s\": \"q\\\"\\\\\\n\\u0001\"\n}"
	if got := string(Marshal(v, "    ")); got != want {
		t.Fatalf("indent 4:\n%s\nwant:\n%s", got, want)
	}
	for _, bad := range []string{``, `{`, `{"a":1} x`, `[1,`, `nope`} {
		if _, err := Parse([]byte(bad)); err == nil {
			t.Errorf("Parse(%q) succeeded", bad)
		}
	}
	// big numbers stay as written
	v, _ = Parse([]byte(`{"n":12345678901234567890,"f":1.50}`))
	if got := string(Marshal(v, "")); got != `{"n":12345678901234567890,"f":1.50}` {
		t.Fatalf("numbers: %s", got)
	}
}

func TestMergePrecedenceAndGroups(t *testing.T) {
	s, root := newStore(t)
	writeFile(t, root, "extensions.json", `{"tabs":[`+entry("base.one", "Base One")+`],"tabsInGroups":{"g1":[`+entry("base.two", "Base Two")+`,`+entry("shared", "Shared Base")+`]},"decorators":[`+entry("deco.base", "Deco Base")+`,`+entry("deco.shared", "Deco Shared Base")+`]}`)
	writeFile(t, root, "data/extensions-catalog/lib/extensions.json", `{"tabs":[`+entry("cat.one", "Cat One")+`,`+entry("shared", "Shared Catalog")+`],"decorators":[`+entry("deco.cat", "Deco Cat")+`]}`)
	writeFile(t, root, "extensions.external.json", `{"tabs":[`+entry("ext.one", "Ext One")+`,`+entry("shared", "Shared External")+`],"decorators":[`+entry("deco.shared", "Deco Shared External")+`]}`)
	l := s.Interface()
	if got := pkgs(l); got != "ext.one,shared,cat.one,base.one,base.two" {
		t.Fatalf("interface: %s", got)
	}
	for _, e := range l {
		if e.Str("package_name") == "shared" && e.Str("name") != "Shared External" {
			t.Errorf("external should win: %s", e.Str("name"))
		}
	}
	d := s.Decorators()
	if got := pkgs(d); got != "deco.shared,deco.cat,deco.base" {
		t.Fatalf("decorators: %s", got)
	}
	if d[0].Str("name") != "Deco Shared External" {
		t.Errorf("decorator precedence: %s", d[0].Str("name"))
	}
	if _, err := s.List("nope"); err == nil {
		t.Error("bad kind accepted")
	}
}

func TestMissingAndBrokenFilesAreEmpty(t *testing.T) {
	s, root := newStore(t)
	if len(s.Interface()) != 0 || len(s.Decorators()) != 0 {
		t.Fatal("empty root should give empty lists")
	}
	writeFile(t, root, "extensions.json", `{not json`)
	writeFile(t, root, "extensions.external.json", `[1,2]`)
	if len(s.Interface()) != 0 {
		t.Fatal("broken files should be ignored")
	}
}

func TestTableAndChoices(t *testing.T) {
	s, root := newStore(t)
	writeFile(t, root, "extensions.json", `{"tabs":[`+entry("extensions.builtin.x", "Builtin")+`,`+entry("zeta", "zeta Tool")+`,`+entry("alpha", "Beta Tool")+`,`+entry("gamma", "alpha tool")+`],"decorators":[`+entry("deco.z", "Z")+`,`+entry("deco.a", "A")+`]}`)
	tbl, err := s.Table("interface")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(tbl, "| Title | Description |\n| --- | --- |\n| Builtin | d Builtin (website: https://w/extensions.builtin.x) (extension_website: https://e/extensions.builtin.x) |\n") {
		t.Fatalf("table: %q", tbl)
	}
	c, _ := s.Choices("interface")
	if strings.Join(c, ",") != "gamma,alpha,zeta" { // sorted by lower-case name: "alpha tool", "Beta Tool", "zeta Tool"
		t.Fatalf("interface choices: %v", c)
	}
	c, _ = s.Choices("decorator")
	if strings.Join(c, ",") != "deco.z,deco.a" { // decorators keep file order
		t.Fatalf("decorator choices: %v", c)
	}
}

func TestParseInput(t *testing.T) {
	one := entry("p.one", "One")
	r := ParseInput(one)
	if len(r.Entries) != 1 || r.Info != "Parsed entries: 1" || !strings.Contains(r.Preview["props"].(map[string]any)["value"].(string), "| One | p.one | tools | interface |") {
		t.Fatalf("one: %+v", r)
	}
	r = ParseInput("[" + one + ",{\"package_name\":\"x\"},3," + strings.Replace(entry("p.two", "Two"), `"name":"Two"`, `"name":5`, 1) + "]")
	if len(r.Entries) != 1 || !strings.Contains(r.Info, "Parsed entries: 1 | Issues: Entry 1: Missing keys: name, requirements") || !strings.Contains(r.Info, "Entry 2: Wrong types for: name") {
		t.Fatalf("mixed: %q", r.Info)
	}
	for in, want := range map[string]string{"": "No JSON provided", "   ": "No JSON provided", "5": "Expected a JSON object or an array of objects", `"x"`: "Expected a JSON object or an array of objects"} {
		if r := ParseInput(in); r.Info != want || len(r.Entries) != 0 {
			t.Errorf("%q: %q", in, r.Info)
		}
	}
	if r := ParseInput("{oops"); !strings.HasPrefix(r.Info, "JSON parse error: ") {
		t.Errorf("parse error: %q", r.Info)
	}
	b, _ := json.Marshal(ParseInput(one))
	if !strings.Contains(string(b), `"entries":[{"package_name":"p.one","name":"One","requirements"`) {
		t.Errorf("entry order lost: %s", b)
	}
}

func TestAdd(t *testing.T) {
	s, root := newStore(t)
	raw := func(ss ...string) []any {
		var out []any
		for _, x := range ss {
			v, err := Parse([]byte(x))
			if err != nil {
				t.Fatal(err)
			}
			out = append(out, v)
		}
		return out
	}
	// no file yet: default document, written with indent 4
	res, err := s.Add(raw(entry("p.one", "Ünï One")))
	if err != nil || res.Status != "Saved | Added: p.one | Skipped (exists): none" {
		t.Fatalf("%+v %v", res, err)
	}
	b, _ := os.ReadFile(filepath.Join(root, "extensions.external.json"))
	if !strings.HasPrefix(string(b), "{\n    \"tabs\": [\n        {\n            \"package_name\": \"p.one\",") || !strings.Contains(string(b), "Ünï One") || !strings.HasSuffix(string(b), "],\n    \"decorators\": []\n}") {
		t.Fatalf("file:\n%s", b)
	}
	if !strings.HasPrefix(res.JSON, "{\n  \"tabs\": [\n    {\n      \"package_name\"") {
		t.Fatalf("shown json is indent 2: %q", res.JSON[:40])
	}
	// second add: one new, one existing; unknown keys and key order survive
	hand := `{"tabsInGroups": {"g": []}, "custom": {"b": 1, "a": 2}, "tabs": [` + entry("p.one", "Ünï One") + `], "decorators": []}`
	writeFile(t, root, "extensions.external.json", hand)
	res, err = s.Add(raw(entry("p.one", "dup"), entry("p.two", "Two", `"proxy":"native"`)))
	if err != nil || res.Status != "Saved | Added: p.two | Skipped (exists): p.one" {
		t.Fatalf("%+v %v", res, err)
	}
	b, _ = os.ReadFile(filepath.Join(root, "extensions.external.json"))
	got := string(b)
	if !(strings.Index(got, `"tabsInGroups"`) < strings.Index(got, `"custom"`) && strings.Index(got, `"custom"`) < strings.Index(got, `"tabs"`)) ||
		strings.Index(got, `"b": 1`) > strings.Index(got, `"a": 2`) || !strings.Contains(got, `"proxy": "native"`) {
		t.Fatalf("order or fields lost:\n%s", got)
	}
	if res, err = s.Add(nil); err != nil || res.Status != "Nothing to add." || res.JSON != "" {
		t.Fatalf("empty: %+v %v", res, err)
	}
	// invalid input and a broken file are refused without touching anything
	if _, err = s.Add(raw(`{"package_name":"x"}`)); err == nil {
		t.Error("invalid entry accepted")
	}
	if _, err = s.Add([]any{"str"}); err == nil {
		t.Error("non-object accepted")
	}
	writeFile(t, root, "extensions.external.json", `{broken`)
	if _, err = s.Add(raw(entry("p.three", "Three"))); err == nil {
		t.Error("broken file was overwritten")
	}
	if b, _ = os.ReadFile(filepath.Join(root, "extensions.external.json")); string(b) != `{broken` {
		t.Errorf("broken file changed: %s", b)
	}
	if ents, _ := os.ReadDir(root); len(ents) != 1 {
		t.Errorf("temp file left: %v", ents)
	}
}

func call(t *testing.T, h http.Handler, method, path, body, tok string) (int, string) {
	t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.RemoteAddr = "127.0.0.1:5555"
	if body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	if tok != "" {
		r.Header.Set(TokenHeader, tok)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w.Code, w.Body.String()
}

func TestHTTP(t *testing.T) {
	s, root := newStore(t)
	h := s.Handler()
	writeFile(t, root, "extensions.json", `{"tabs":[`+entry("a.one", "A One")+`]}`)

	if c, _ := call(t, h, "GET", "/api/extcatalog/list?kind=interface", "", ""); c != 403 {
		t.Errorf("no token: %d", c)
	}
	r := httptest.NewRequest("GET", "/api/extcatalog/list?kind=interface", nil)
	r.RemoteAddr = "10.0.0.5:1"
	r.Header.Set(TokenHeader, "tok")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Errorf("non-loopback: %d", w.Code)
	}
	if c, b := call(t, h, "GET", "/api/extcatalog/list?kind=interface", "", "tok"); c != 200 || !strings.HasPrefix(b, `[{"package_name":"a.one","name":"A One"`) {
		t.Errorf("list: %d %s", c, b)
	}
	if c, _ := call(t, h, "GET", "/api/extcatalog/list?kind=zzz", "", "tok"); c != 400 {
		t.Errorf("bad kind: %d", c)
	}
	if c, b := call(t, h, "GET", "/api/extcatalog/table?kind=interface", "", "tok"); c != 200 || !strings.Contains(b, `"__gradio__":"Markdown"`) || !strings.Contains(b, `| A One |`) {
		t.Errorf("table: %d %s", c, b)
	}
	if c, b := call(t, h, "GET", "/api/extcatalog/picker?kind=interface&label=Pick+me", "", "tok"); c != 200 || b != `{"__gradio__":"Dropdown","props":{"choices":["a.one"],"label":"Pick me"}}`+"\n" {
		t.Errorf("picker: %d %q", c, b)
	}
	if c, _ := call(t, h, "POST", "/api/extcatalog/list", "{}", "tok"); c != 405 {
		t.Errorf("wrong method: %d", c)
	}
	if c, _ := call(t, h, "GET", "/api/extcatalog/nope", "", "tok"); c != 404 {
		t.Errorf("unknown op: %d", c)
	}
	body, _ := json.Marshal(map[string]string{"text": entry("n.one", "New One")})
	if c, b := call(t, h, "POST", "/api/extcatalog/parse", string(body), "tok"); c != 200 || !strings.Contains(b, `"info":"Parsed entries: 1"`) {
		t.Errorf("parse: %d %s", c, b)
	}
	if c, _ := call(t, h, "POST", "/api/extcatalog/parse", `{"text":"x","extra":1}`, "tok"); c != 400 {
		t.Errorf("unknown field: %d", c)
	}
	add := `{"entries":[` + entry("n.one", "New One") + `]}`
	if c, b := call(t, h, "POST", "/api/extcatalog/add", add, "tok"); c != 200 || !strings.Contains(b, `"status":"Saved | Added: n.one | Skipped (exists): none"`) {
		t.Errorf("add: %d %s", c, b)
	}
	if c, _ := call(t, h, "POST", "/api/extcatalog/add", `{"entries":[{"package_name":"x"}]}`, "tok"); c != 400 {
		t.Errorf("add invalid: %d", c)
	}
	if c, _ := call(t, h, "POST", "/api/extcatalog/add", `{"entries":"no"}`, "tok"); c != 400 {
		t.Errorf("add not a list: %d", c)
	}
	if c, _ := call(t, h, "POST", "/api/extcatalog/install", `{"package_name":"missing"}`, "tok"); c != 404 {
		t.Errorf("install unknown: %d", c)
	}
	if c, _ := call(t, h, "POST", "/api/extcatalog/install", `{"package_name":"extensions.builtin.x"}`, "tok"); c != 404 {
		t.Errorf("install builtin: %d", c)
	}
	if c, _ := call(t, h, "POST", "/api/extcatalog/install", `{"package_name":"a.one","x":1}`, "tok"); c != 400 {
		t.Errorf("install unknown field: %d", c)
	}
}

func TestShellSplit(t *testing.T) {
	for in, want := range map[string][]string{
		`git+https://x/y@main`:                 {"git+https://x/y@main"},
		`  a  b\tc `:                           {"a", "btc"},
		`"a b" 'c d' e"f"g`:                    {"a b", "c d", "efg"},
		`pkg>=1.0 --extra-index-url https://x`: {"pkg>=1.0", "--extra-index-url", "https://x"},
		`"say \"hi\""`:                         {`say "hi"`},
		`''`:                                   {""},
		``:                                     nil,
	} {
		got, err := shellSplit(in)
		if err != nil || strings.Join(got, "|") != strings.Join(want, "|") || len(got) != len(want) {
			t.Errorf("shellSplit(%q) = %q %v, want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{`"open`, `'open`, `end\`} {
		if _, err := shellSplit(bad); err == nil {
			t.Errorf("shellSplit(%q) succeeded", bad)
		}
	}
}

// --- install, with this test binary standing in for the main env ---

// TestHelperProcess is the fake "python" / "uv": it logs its arguments to
// $HELPER_LOG and prints lines, failing when an argument is "FAILME".
func TestHelperProcess(t *testing.T) {
	if os.Getenv("EXTCATALOG_HELPER") != "1" {
		return
	}
	args := os.Args
	for i, a := range args {
		if a == "--" {
			args = args[i+1:]
			break
		}
	}
	if f, err := os.OpenFile(os.Getenv("HELPER_LOG"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
		fmt.Fprintln(f, strings.Join(args, " "))
		f.Close()
	}
	if len(args) > 1 && args[0] == "venv" { // what uv leaves: a python in the venv
		bin, exe := "bin", "python"
		if os.PathSeparator == '\\' {
			bin, exe = "Scripts", "python.exe"
		}
		_ = os.MkdirAll(filepath.Join(args[1], bin), 0o755)
		_ = os.WriteFile(filepath.Join(args[1], bin, exe), []byte("fake"), 0o755)
	}
	fmt.Println("out: " + strings.Join(args[:min(2, len(args))], " "))
	fmt.Fprintln(os.Stderr, "err: progress\rmore")
	for _, a := range args {
		if a == "--dry-run" && os.Getenv("HELPER_WOULD_INSTALL") != "" {
			fmt.Println("Would install " + os.Getenv("HELPER_WOULD_INSTALL"))
		}
		if a == "FAILME" {
			os.Exit(3)
		}
	}
	os.Exit(0)
}

func installStore(t *testing.T) (*Store, string, string) {
	t.Helper()
	root := t.TempDir()
	log := filepath.Join(root, "helper.log")
	t.Setenv("EXTCATALOG_HELPER", "1")
	t.Setenv("HELPER_LOG", log)
	pr, err := paths.New(root)
	if err != nil {
		t.Fatal(err)
	}
	s := New(Options{
		Root: root, Token: "tok",
		Platform: legacyenv.New(legacyenv.Options{Root: pr, Prefix: []string{os.Args[0], "-test.run=^TestHelperProcess$", "--"}}),
	})
	return s, root, log
}

type streamed struct {
	lines []string
	ok    bool
	done  bool
}

func stream(t *testing.T, s *Store, body string) (int, streamed, string) {
	t.Helper()
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	req, _ := http.NewRequest("POST", srv.URL+"/api/extcatalog/"+body[:strings.IndexByte(body, ' ')], strings.NewReader(body[strings.IndexByte(body, ' ')+1:]))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(TokenHeader, "tok")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out streamed
	var raw strings.Builder
	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		raw.WriteString(sc.Text() + "\n")
		var m struct {
			Line string `json:"line"`
			Done bool   `json:"done"`
			OK   bool   `json:"ok"`
		}
		if json.Unmarshal(sc.Bytes(), &m) == nil {
			if m.Done {
				out.done, out.ok = true, m.OK
			} else if m.Line != "" {
				out.lines = append(out.lines, m.Line)
			}
		}
	}
	return resp.StatusCode, out, raw.String()
}

func readLog(t *testing.T, p string) []string {
	t.Helper()
	b, _ := os.ReadFile(p)
	return strings.Split(strings.TrimSpace(string(b)), "\n")
}

func TestPipInstallUninstall(t *testing.T) {
	s, root, log := installStore(t)
	writeFile(t, root, "extensions.json", `{"tabs":[`+entry("ext.pip", "Pip Ext")+`],"decorators":[`+entry("ext.deco", "Deco Ext")+`]}`)
	code, got, _ := stream(t, s, `install {"package_name":"ext.pip"}`)
	if code != 200 || !got.done || !got.ok {
		t.Fatalf("install: %d %+v", code, got)
	}
	joined := strings.Join(got.lines, "\n")
	for _, want := range []string{"out: python -m", "err: progress", "more", "Successfully installed Pip Ext dependencies", "Please restart the webui to see the changes"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in:\n%s", want, joined)
		}
	}
	if l := readLog(t, log); len(l) != 1 || l[0] != "python -m pip install git+https://example.com/ext.pip@main gradio==5.49.1" {
		t.Fatalf("command: %q", l)
	}
	if b, err := os.ReadFile(filepath.Join(root, "installer_scripts", "logs", "pip-install-Pip Ext.log")); err != nil || !strings.Contains(string(b), "Successfully installed") {
		t.Errorf("log file: %v %s", err, b)
	}
	// a decorator, and uninstall
	if _, got, _ = stream(t, s, `install {"package_name":"ext.deco"}`); !got.ok {
		t.Errorf("decorator install: %+v", got)
	}
	_, got, _ = stream(t, s, `uninstall {"package_name":"ext.pip"}`)
	if !got.ok || !strings.Contains(strings.Join(got.lines, "\n"), "Successfully uninstalled ext.pip (ext.pip)") {
		t.Fatalf("uninstall: %+v", got)
	}
	if l := readLog(t, log); l[len(l)-1] != "python -m pip uninstall -y ext.pip" {
		t.Fatalf("uninstall command: %q", l)
	}
	// failure
	writeFile(t, root, "extensions.json", `{"tabs":[`+strings.Replace(entry("ext.bad", "Bad"), "git+https://example.com/ext.bad@main", "FAILME", 1)+`]}`)
	_, got, _ = stream(t, s, `install {"package_name":"ext.bad"}`)
	if got.ok || !got.done || !strings.Contains(strings.Join(got.lines, "\n"), "Failed to install Bad dependencies") {
		t.Fatalf("failure: %+v", got)
	}
}

func TestOneInstallAtATime(t *testing.T) {
	s, root, _ := installStore(t)
	writeFile(t, root, "extensions.json", `{"tabs":[`+entry("ext.pip", "Pip Ext")+`]}`)
	s.install.Lock() // an install is "running"
	h := s.Handler()
	if c, b := call(t, h, "POST", "/api/extcatalog/install", `{"package_name":"ext.pip"}`, "tok"); c != 409 || !strings.Contains(b, "another install is running") {
		t.Errorf("busy: %d %s", c, b)
	}
	s.install.Unlock()
}
