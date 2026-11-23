package appenv

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// golden.json is produced by testdata/gen_golden.py running the real Python
// (python-dotenv, writer.py, server.py) on Windows, so the env cases use
// fold=true (os.environ upper-cases names there).
type golden struct {
	OS         string     `json:"os"`
	GenCases   []genCase  `json:"genenv_cases"`
	PlanCases  []planCase `json:"plan_cases"`
	EnvCasesJS []envCase  `json:"env_cases"`
}

type envCase struct {
	Name      string
	Base      map[string]string
	EnvFile   string  `json:"env_file"`
	EnvUser   *string `json:"env_user"`
	Stray     map[string]string
	Expected  map[string]string
	WarnLines []int `json:"warn_lines"`
}

type genCase struct {
	Name         string
	Base         map[string]string
	ExpectedFile string `json:"expected_file"`
}

type planCase struct {
	Name    string
	Flags   []string
	Config  map[string]any
	EnvUser string `json:"env_user"`
	Result  struct {
		Launch struct {
			ServerPort portVal `json:"server_port"`
			ServerName string  `json:"server_name"`
			Share      bool
			InBrowser  bool `json:"inbrowser"`
		}
		Tree struct {
			Host string
			Port int
		}
		Routes      [][]any
		Popen       *struct{ Cmd, Backend string }
		Browser     []string
		ReactPort   string  `json:"react_ui_port"`
		EnvTreePort *string `json:"env_tree_port"`
		EnvTreeURL  *string `json:"env_tree_url"`
	}
}

func loadGolden(t *testing.T) golden {
	t.Helper()
	b, err := os.ReadFile("testdata/golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var g golden
	if err := json.Unmarshal(b, &g); err != nil {
		t.Fatal(err)
	}
	return g
}

func mapToEnv(m map[string]string) []string {
	var e []string
	for k, v := range m {
		e = append(e, k+"="+v)
	}
	sort.Strings(e)
	return e
}

func envToMap(e []string, fold bool) map[string]string {
	m := map[string]string{}
	for _, s := range e {
		k, v, _ := splitEnv(s)
		m[canon(k, fold)] = v
	}
	return m
}

func write(t *testing.T, root, rel, text string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadEnvGolden(t *testing.T) {
	g := loadGolden(t)
	if len(g.EnvCasesJS) == 0 {
		t.Fatal("no env cases")
	}
	for _, c := range g.EnvCasesJS {
		t.Run(c.Name, func(t *testing.T) {
			root := t.TempDir()
			write(t, root, ".env", c.EnvFile)
			if c.EnvUser != nil {
				write(t, root, ".env.user", *c.EnvUser)
			}
			for p, s := range c.Stray {
				write(t, root, p, s)
			}
			rep, err := loadEnv(root, mapToEnv(c.Base), true)
			if err != nil {
				t.Fatal(err)
			}
			got := envToMap(rep.Env, true)
			if !reflect.DeepEqual(got, c.Expected) {
				t.Errorf("env mismatch\n got: %q\nwant: %q", got, c.Expected)
			}
			if !reflect.DeepEqual(append([]int(nil), rep.WarnLines...), append([]int(nil), c.WarnLines...)) {
				t.Errorf("warning lines got %v want %v", rep.WarnLines, c.WarnLines)
			}
		})
	}
}

func TestGenerateEnvGolden(t *testing.T) {
	g := loadGolden(t)
	for _, c := range g.GenCases {
		t.Run(c.Name, func(t *testing.T) {
			// Python ran on Windows: names fold, files get CRLF.
			want := c.ExpectedFile
			got := strings.ReplaceAll(generateEnv(mapToEnv(c.Base), true), "\n", "\r\n")
			if got != want {
				t.Errorf("got\n%q\nwant\n%q", got, want)
			}
			root := t.TempDir()
			created, err := ensureEnv(root, mapToEnv(c.Base), true)
			if err != nil || !created {
				t.Fatalf("created=%v err=%v", created, err)
			}
			b, _ := os.ReadFile(filepath.Join(root, ".env"))
			if string(b) != want {
				t.Errorf("file differs from golden")
			}
		})
	}
}

func TestEnsureEnvKeepsExisting(t *testing.T) {
	root := t.TempDir()
	write(t, root, ".env", "KEEP=1\n")
	created, err := EnsureEnv(root, nil)
	if err != nil || created {
		t.Fatalf("created=%v err=%v", created, err)
	}
	b, _ := os.ReadFile(filepath.Join(root, ".env"))
	if string(b) != "KEEP=1\n" {
		t.Fatalf("overwritten: %q", b)
	}
}

func TestEnsureFolders(t *testing.T) {
	root := t.TempDir()
	write(t, root, "favorites/x.txt", "keep")
	for i := 0; i < 2; i++ {
		if err := EnsureFolders(root); err != nil {
			t.Fatal(err)
		}
	}
	for _, d := range []string{"outputs", "favorites"} {
		if st, err := os.Stat(filepath.Join(root, d)); err != nil || !st.IsDir() {
			t.Errorf("%s missing", d)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "favorites", "x.txt")); err != nil {
		t.Error("existing content lost")
	}
}

func TestLoadEnvPosixCase(t *testing.T) {
	root := t.TempDir()
	write(t, root, ".env", "a=file\nA=file2\nB=${a}\n")
	rep, err := loadEnv(root, []string{"a=base"}, false)
	if err != nil {
		t.Fatal(err)
	}
	m := envToMap(rep.Env, false)
	if m["a"] != "base" || m["A"] != "file2" || m["B"] != "base" {
		t.Errorf("got %v", m)
	}
}

func TestLoadEnvDoesNotMutateBaseAndKeepsOrder(t *testing.T) {
	root := t.TempDir()
	write(t, root, ".env", "N=1\n")
	base := []string{"=C:=C:\\x", "Z=z", "A=a"}
	cp := append([]string(nil), base...)
	out, err := LoadEnv(root, base)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(base, cp) {
		t.Error("base modified")
	}
	if len(out) != 4 || out[0] != "=C:=C:\\x" || out[3] != "N=1" {
		t.Errorf("out %q", out)
	}
}

func TestLoadEnvRejectsBadUTF8AndNames(t *testing.T) {
	root := t.TempDir()
	write(t, root, ".env", "A=\xff\n")
	if _, err := LoadEnv(root, nil); err == nil {
		t.Error("want utf-8 error")
	}
	write(t, root, ".env", "'A=B'=1\n")
	if _, err := LoadEnv(root, nil); err == nil {
		t.Error("want invalid name error")
	}
}

func TestParseFlags(t *testing.T) {
	f := ParseFlags([]string{"--share", "x", "--docker", "--no-tree-proxy", "--no-react", "--no-database", "--bogus"})
	if f != (Flags{true, true, true, true, true}) {
		t.Errorf("%+v", f)
	}
	if ParseFlags(nil) != (Flags{}) {
		t.Error("nil args")
	}
}

func TestPlanGolden(t *testing.T) {
	g := loadGolden(t)
	if len(g.PlanCases) < 100 {
		t.Fatalf("only %d plan cases", len(g.PlanCases))
	}
	for _, c := range g.PlanCases {
		name := c.Name + strings.Join(c.Flags, "")
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			cfg, _ := json.Marshal(map[string]any{"gradio_interface_options": c.Config})
			write(t, root, "config.json", string(cfg))
			write(t, root, ".env", "")
			write(t, root, ".env.user", c.EnvUser)
			env, err := LoadEnv(root, nil)
			if err != nil {
				t.Fatal(err)
			}
			p, err := Plan(root, env, ParseFlags(c.Flags))
			if err != nil {
				t.Fatal(err)
			}
			r := c.Result
			eq := func(what string, got, want any) {
				t.Helper()
				if !reflect.DeepEqual(got, want) {
					t.Errorf("%s: got %v want %v", what, got, want)
				}
			}
			eq("GradioPort", p.GradioPort, int(r.Launch.ServerPort))
			eq("Host", p.Host, r.Launch.ServerName)
			eq("Share", p.Share, r.Launch.Share)
			eq("GradioInBrowser", p.GradioInBrowser, r.Launch.InBrowser)
			eq("TreePort", p.TreePort, r.Tree.Port)
			eq("TreeHost", p.Host, r.Tree.Host)
			eq("ReactPort", strconv.Itoa(p.ReactPort), r.ReactPort)
			eq("Browser", append([]string(nil), p.BrowserURLs...), append([]string(nil), r.Browser...))
			eq("react started", !p.NoReact, r.Popen != nil)
			if r.Popen != nil {
				eq("ReactBackend", p.ReactBackend, r.Popen.Backend)
				eq("react cmd", fmt.Sprintf("npm start --prefix react-ui -- -p %d", p.ReactPort), r.Popen.Cmd)
			}
			// Env python sees for the tree.
			envm := envToMap(p.Env(), false)
			tp, hasTP := envm["GRADIO_TREE_PORT"]
			eq("GRADIO_TREE_PORT set", hasTP, p.TreeProxy)
			if p.TreeProxy {
				eq("GRADIO_TREE_PORT", tp, *r.EnvTreePort)
				eq("GRADIO_TREE_URL", envm["GRADIO_TREE_URL"], *r.EnvTreeURL)
				eq("PublicPort", strconv.Itoa(p.PublicPort), *r.EnvTreePort)
			} else {
				eq("PublicPort", p.PublicPort, int(r.Launch.ServerPort))
				eq("no tree env", len(envm), 2)
			}
			eq("TTS_GRADIO_PORT", envm["TTS_GRADIO_PORT"], strconv.Itoa(int(r.Launch.ServerPort)))
			eq("TTS_GRADIO_HOST", envm["TTS_GRADIO_HOST"], r.Launch.ServerName)
		})
	}
}

func TestPlanTable(t *testing.T) {
	cfg := Config{Port: 7770, Host: "127.0.0.1", InBrowser: true}
	cases := []struct {
		flags          []string
		public, gradio int
		host           string
		tree           bool
	}{
		{nil, 7770, 7767, "127.0.0.1", true},
		{[]string{"--no-tree-proxy"}, 7770, 7770, "127.0.0.1", false},
		{[]string{"--docker"}, 7770, 7767, "0.0.0.0", true},
		{[]string{"--docker", "--no-tree-proxy"}, 7770, 7767, "0.0.0.0", true},
	}
	for _, c := range cases {
		p, err := PlanFor(cfg, nil, ParseFlags(c.flags))
		if err != nil {
			t.Fatal(err)
		}
		if p.PublicPort != c.public || p.GradioPort != c.gradio || p.Host != c.host || p.TreeProxy != c.tree || p.ReactPort != 3000 {
			t.Errorf("%v: %v", c.flags, p)
		}
	}
	cfg.Port = 8123
	p, _ := PlanFor(cfg, []string{"REACT_UI_PORT=3100"}, Flags{})
	if p.PublicPort != 8123 || p.GradioPort != 7767 || p.ReactPort != 3100 || p.ReactBackend != "http://127.0.0.1:8123/" {
		t.Errorf("%v", p)
	}
	if _, err := PlanFor(cfg, []string{"REACT_UI_PORT=x"}, Flags{}); err == nil {
		t.Error("want error")
	}
	if _, err := PlanFor(cfg, []string{"REACT_UI_PORT=x"}, Flags{NoReact: true}); err != nil {
		t.Error(err)
	}
}

func TestLoadConfig(t *testing.T) {
	root := t.TempDir()
	c, err := LoadConfig(root)
	if err != nil || !c.FromDefault || c.Port != 7770 || !c.InBrowser {
		t.Fatalf("%+v %v", c, err)
	}
	bad := map[string]string{
		"not json":  `{`,
		"no gio":    `{}`,
		"no auth":   `{"gradio_interface_options":{"server_name":"x","server_port":1}}`,
		"no port":   `{"gradio_interface_options":{"auth":null,"server_name":"x"}}`,
		"bad port":  `{"gradio_interface_options":{"auth":null,"server_name":"x","server_port":"abc"}}`,
		"big port":  `{"gradio_interface_options":{"auth":null,"server_name":"x","server_port":70000}}`,
		"bad auth":  `{"gradio_interface_options":{"auth":5,"server_name":"x","server_port":1}}`,
		"auth nopw": `{"gradio_interface_options":{"auth":"justuser","server_name":"x","server_port":1}}`,
	}
	for n, s := range bad {
		write(t, root, "config.json", s)
		if _, err := LoadConfig(root); err == nil {
			t.Errorf("%s: want error", n)
		} else if _, ok := err.(*ConfigError); !ok && n != "not json" {
			t.Errorf("%s: %T", n, err)
		}
	}
	write(t, root, "config.json", `{"gradio_interface_options":{"auth":"bob:s3cret","server_name":null,"server_port":"7801","inbrowser":0,"share":1}}`)
	c, err = LoadConfig(root)
	if err != nil || c.Port != 7801 || c.Host != "127.0.0.1" || c.InBrowser || !c.Share {
		t.Fatalf("%+v %v", c, err)
	}
}

func TestAuth(t *testing.T) {
	a, _ := parseAuth("bob:s3cret:more")
	if len(a.Credentials) != 1 || a.Credentials[0].User != "bob" || a.Credentials[0].Password != "s3cret" {
		t.Errorf("%+v", a.Credentials)
	}
	var l any
	json.Unmarshal([]byte(`["u","p"]`), &l)
	a, _ = parseAuth(l)
	if !a.Enabled() || a.Credentials[0].Password != "p" {
		t.Error("list")
	}
	json.Unmarshal([]byte(`[["u","p"],["v","q"]]`), &l)
	a, _ = parseAuth(l)
	if len(a.Credentials) != 2 || a.Credentials[1].User != "v" {
		t.Error("nested")
	}
	if a, _ := parseAuth(nil); a.Enabled() {
		t.Error("nil auth enabled")
	}
	a, _ = parseAuth("bob:s3cret")
	cfg := Config{Port: 7770, Host: "h", Auth: a}
	p, _ := PlanFor(cfg, nil, Flags{})
	for _, s := range []string{fmt.Sprint(a), fmt.Sprintf("%v %+v %#v %s", p, p, a, a.Credentials[0]), p.String(), fmt.Sprintf("%+v", a.Credentials)} {
		if strings.Contains(s, "s3cret") {
			t.Errorf("password leaked: %s", s)
		}
	}
	for _, e := range append(p.Env(), p.ReactEnv()...) {
		if strings.Contains(e, "s3cret") {
			t.Error("password in env")
		}
	}
}

func TestFindDotenvShadow(t *testing.T) {
	root := t.TempDir()
	write(t, root, ".env", "")
	if got := FindDotenv(root); got != filepath.Join(root, ".env") {
		t.Errorf("got %s", got)
	}
	write(t, root, "tts_webui/.env", "")
	if got := FindDotenv(root); got != filepath.Join(root, "tts_webui", ".env") {
		t.Errorf("got %s", got)
	}
}

// portVal accepts the number or numeric string gradio was handed.
type portVal int

func (p *portVal) UnmarshalJSON(b []byte) error {
	n, err := strconv.Atoi(strings.Trim(string(b), `"`))
	*p = portVal(n)
	return err
}

func TestFlagsArgsRoundTrip(t *testing.T) {
	for _, args := range [][]string{
		nil,
		{"--no-react"},
		{"--no-react", "--share", "--docker", "--no-tree-proxy"},
	} {
		if got := ParseFlags(args).Args(); strings.Join(got, " ") != strings.Join(args, " ") {
			t.Errorf("Args(ParseFlags(%v)) = %v", args, got)
		}
	}
}
