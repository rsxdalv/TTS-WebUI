package launcher

import (
	"encoding/json"
	"fmt"
	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform/legacyenv"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform/system"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/appenv"
	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/launchopts"
)

// envMap turns KEY=VALUE pairs into a map.
func envMap(env []string) map[string]string {
	m := map[string]string{}
	for _, kv := range env {
		k, v, _ := strings.Cut(kv, "=")
		m[k] = v
	}
	return m
}

func get(t *testing.T, url, token string) (int, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("X-TTS-Core-Token", token)
	}
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func TestServerPyRoutesToGradioAndServesControlAPI(t *testing.T) {
	sp, err := startServerPyOn(makeRoot(t), "127.0.0.1", 0, testPlat(t, []string{"no-such-micromamba", "run"}), appenv.Flags{NoReact: true})
	if err != nil {
		t.Fatal(err)
	}
	defer sp.Close()

	env := envMap(sp.gradioEnv)
	for _, k := range []string{"TTS_CORE_URL", "TTS_CORE_TOKEN"} {
		if env[k] == "" {
			t.Fatalf("gradio env has no %s: %q", k, sp.gradioEnv)
		}
	}
	// The address Gradio binds is not in the environment: Python asks for its
	// launch options, and they carry the private host and port.
	if _, ok := env["TTS_GRADIO_PORT"]; ok {
		t.Errorf("TTS_GRADIO_PORT is still in the gradio env: %q", sp.gradioEnv)
	}
	if !strings.HasPrefix(env["TTS_CORE_URL"], "http://127.0.0.1:") {
		t.Errorf("TTS_CORE_URL = %q, want a loopback URL", env["TTS_CORE_URL"])
	}
	if len(env["TTS_CORE_TOKEN"]) != 64 {
		t.Errorf("token has %d hex chars, want 64", len(env["TTS_CORE_TOKEN"]))
	}

	token := env["TTS_CORE_TOKEN"]
	code, body := get(t, env["TTS_CORE_URL"]+launchopts.Path, token)
	var launch launchopts.Result
	if code != 200 || json.Unmarshal([]byte(body), &launch) != nil {
		t.Fatalf("GET %s = %d %s", launchopts.Path, code, body)
	}
	if launch.Options["server_name"] != "127.0.0.1" || launch.Options["inbrowser"] != false {
		t.Errorf("launch options: %v", launch.Options)
	}
	gradioPort := fmt.Sprint(launch.Options["server_port"])

	// A stand-in Gradio on the port the gradio child is told to bind.
	gln, err := net.Listen("tcp", "127.0.0.1:"+gradioPort)
	if err != nil {
		t.Fatal(err)
	}
	gsrv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "gradio "+r.URL.Path)
	})}
	go func() { _ = gsrv.Serve(gln) }()
	defer gsrv.Close()

	if code, body := get(t, "http://"+sp.publicAddr+"/config", ""); code != 200 || body != "gradio /config" {
		t.Fatalf("router GET /config = %d %q, want 200 \"gradio /config\"", code, body)
	}

	if code, _ := get(t, env["TTS_CORE_URL"]+"/api/extensions", ""); code != http.StatusForbidden {
		t.Errorf("control API without token = %d, want 403", code)
	}
	if code, body := get(t, env["TTS_CORE_URL"]+"/api/extensions", token); code != 200 {
		t.Errorf("GET /api/extensions = %d %q, want 200", code, body)
	}
	if code, body := get(t, env["TTS_CORE_URL"]+"/api/routes", token); code != 200 || !strings.Contains(body, gradioPort) {
		t.Errorf("GET /api/routes = %d %q, want 200 listing the gradio route", code, body)
	}

	sp.Close()
	sp.Close() // idempotent
	if c, err := net.DialTimeout("tcp", sp.publicAddr, time.Second); err == nil {
		c.Close()
		t.Errorf("router still listening on %s after Close", sp.publicAddr)
	}
	if c, err := net.DialTimeout("tcp", strings.TrimPrefix(env["TTS_CORE_URL"], "http://"), time.Second); err == nil {
		c.Close()
		t.Errorf("control API still listening after Close")
	}
}

func TestServerPyPublicPortInUse(t *testing.T) {
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer busy.Close()
	port := busy.Addr().(*net.TCPAddr).Port
	_, err = startServerPyOn(makeRoot(t), "127.0.0.1", port, testPlat(t, nil), appenv.Flags{NoReact: true})
	if err == nil || !strings.Contains(err.Error(), "in use") {
		t.Fatalf("got %v, want a port-in-use error", err)
	}
}

func TestServerPyUsesConfigPort(t *testing.T) {
	root := makeRoot(t)
	port, err := freePort()
	if err != nil {
		t.Fatal(err)
	}
	cfg := `{"gradio_interface_options": {"auth": null, "server_name": "127.0.0.1", "server_port": ` + strconv.Itoa(port) + `}}`
	if err := os.WriteFile(filepath.Join(root.Dir(), "config.json"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	sp, err := startServerPy(root, systemPlat(root), appenv.Flags{NoReact: true})
	if err != nil {
		t.Fatal(err)
	}
	defer sp.Close()
	if want := "127.0.0.1:" + strconv.Itoa(port); sp.publicAddr != want {
		t.Fatalf("router on %s, want %s (config.json server_port)", sp.publicAddr, want)
	}
}

func TestServerPyPrepsFoldersEnvAndPassesItToChildren(t *testing.T) {
	root := makeRoot(t)
	port, err := freePort()
	if err != nil {
		t.Fatal(err)
	}
	cfg := `{"gradio_interface_options": {"auth": null, "server_name": "127.0.0.1", "server_port": ` + strconv.Itoa(port) + `}}`
	if err := os.WriteFile(filepath.Join(root.Dir(), "config.json"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root.Dir(), ".env.user"), []byte("TTS_PREP_USER=user\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sp, err := startServerPy(root, systemPlat(root, "TTS_PREP_BASE=base"), appenv.Flags{NoReact: true})
	if err != nil {
		t.Fatal(err)
	}
	defer sp.Close()

	for _, d := range []string{"outputs", "favorites"} {
		if st, err := os.Stat(filepath.Join(root.Dir(), d)); err != nil || !st.IsDir() {
			t.Errorf("%s was not created: %v", d, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root.Dir(), ".env")); err != nil {
		t.Errorf(".env was not generated: %v", err)
	}
	env := envMap(sp.gradioEnv)
	if env["TTS_PREP_BASE"] != "base" || env["TTS_PREP_USER"] != "user" {
		t.Errorf("gradio env lacks base or .env.user values: base=%q user=%q", env["TTS_PREP_BASE"], env["TTS_PREP_USER"])
	}
	if env["TTS_CORE_URL"] == "" {
		t.Error("gradio env lacks TTS_CORE_URL")
	}
}

// --share reaches Gradio's launch options through the launcher's flags.
func TestServerPyShareFlagReachesLaunchOptions(t *testing.T) {
	root := makeRoot(t)
	port, err := freePort()
	if err != nil {
		t.Fatal(err)
	}
	cfg := `{"gradio_interface_options": {"auth": null, "server_name": "127.0.0.1", "server_port": ` + strconv.Itoa(port) + `}}`
	if err := os.WriteFile(filepath.Join(root.Dir(), "config.json"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, share := range []bool{false, true} {
		sp, err := startServerPy(root, systemPlat(root), options{share: share}.flags())
		if err != nil {
			t.Fatal(err)
		}
		env := envMap(sp.gradioEnv)
		code, body := get(t, env["TTS_CORE_URL"]+launchopts.Path, env["TTS_CORE_TOKEN"])
		sp.Close()
		var launch launchopts.Result
		if code != 200 || json.Unmarshal([]byte(body), &launch) != nil {
			t.Fatalf("GET %s = %d %s", launchopts.Path, code, body)
		}
		if got := launch.Options["share"] == true; got != share {
			t.Errorf("share=%v: options.share = %v", share, launch.Options["share"])
		}
	}
}

// The launcher's --share and --docker go to the gradio child's arguments, so
// server.py (the legacy path) sees them too; --no-react is always there.
func TestSuperviseConfigPassesShareAndDocker(t *testing.T) {
	for _, tc := range []struct {
		opts options
		want []string
	}{
		{options{}, []string{"--no-react"}},
		{options{share: true}, []string{"--no-react", "--share"}},
		{options{share: true, docker: true}, []string{"--no-react", "--share", "--docker"}},
	} {
		got := superviseConfig(makeRoot(t), tc.opts).GradioArgs
		if strings.Join(got, " ") != strings.Join(tc.want, " ") {
			t.Errorf("%+v: gradio args %v, want %v", tc.opts, got, tc.want)
		}
	}
}

func TestOptionsFlags(t *testing.T) {
	o := buildOptions([]Option{WithNoBrowser(), WithShare(), WithDocker()})
	if want := (appenv.Flags{NoReact: true, Share: true, Docker: true}); o.flags() != want {
		t.Errorf("flags = %+v, want %+v", o.flags(), want)
	}
	if o := buildOptions(nil); o.flags() != (appenv.Flags{NoReact: true}) {
		t.Errorf("default flags = %+v, want only NoReact", o.flags())
	}
}

// testPlat is the legacy-env platform with prefix in front of every command, on
// a temp root (no prefix: the system platform).
func testPlat(t *testing.T, prefix []string) platform.Platform {
	t.Helper()
	root := makeRoot(t)
	if len(prefix) == 0 {
		return systemPlat(root)
	}
	return legacyenv.New(legacyenv.Options{Root: root, Prefix: prefix})
}

// systemPlat is the system platform with env added to every command.
func systemPlat(root paths.Root, env ...string) platform.Platform {
	return system.New(system.Options{Root: root, Env: env})
}
