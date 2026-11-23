package launcher

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/appenv"
	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/extprocs"
)

// Opt-in tests that run real Python through the launcher's own wiring. They
// need an install: a copy of the repo (git archive) with installer_files
// junctioned to a real one, a config.json whose server_port is free, and (for
// the app run below) a react-ui/out/_tts-export.json. Remove the junction with
// rmdir before deleting the copy.
//
// The full launcher run is done by hand on the same copy: with
// TTS_GO_SERVERPY=1 run `tts-webui launch --root <copy> --port 179xx
// --no-browser`, POST /api/app/start with the X-TTS-Token from GET /, and read
// the tabs from the app's /config. Extensions are enabled by leaving them out
// of config.json's extensions.disabled.

// EnvRealRoot names an install (a copy of the repo with installer_files, for
// example a junction to a real one) for the opt-in tests that run real Python.
const EnvRealRoot = "LAUNCHER_REAL_ROOT"

// realRoot is the install of EnvRealRoot, or a skip.
func realRoot(t *testing.T) paths.Root {
	t.Helper()
	dir := os.Getenv(EnvRealRoot)
	if dir == "" {
		t.Skip(EnvRealRoot + " not set")
	}
	root, err := paths.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	return root
}

// realPrefix is the micromamba command the supervisor runs Python with.
func realPrefix(root paths.Root) []string {
	return []string{micromambaPath(root), "-p", installEnvPath(root), "run"}
}

// postJSON sends body to the control API as Python's bridge does.
func postJSON(t *testing.T, url, token string, body any) (int, string) {
	t.Helper()
	b, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(extprocs.TokenHeader, token)
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(out)
}

// waitReady polls the extension list until pkg is ready, or fails with its log.
func waitReady(t *testing.T, ctlURL, token, pkg string, within time.Duration) {
	t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		_, body := get(t, ctlURL+"/api/extensions", token)
		var list []extprocs.Status
		_ = json.Unmarshal([]byte(body), &list)
		for _, st := range list {
			if st.Package != pkg {
				continue
			}
			if st.State == extprocs.StateReady {
				return
			}
			if st.State == extprocs.StateFailed || st.State == extprocs.StateExited {
				t.Fatalf("%s is %s: %s\n%s", pkg, st.State, st.Error, extensionLog(t, ctlURL, token, pkg))
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("%s not ready within %s\n%s", pkg, within, extensionLog(t, ctlURL, token, pkg))
}

func extensionLog(t *testing.T, ctlURL, token, pkg string) string {
	t.Helper()
	_, body := get(t, ctlURL+"/api/extensions/"+pkg+"/logs", token)
	var out struct {
		Lines []string `json:"lines"`
	}
	_ = json.Unmarshal([]byte(body), &out)
	return strings.Join(out.Lines, "\n")
}

// TestRealExtensionProcessAppliesDecorators starts a real extension (it
// imports decorator_extensions) through the launcher's own wiring: the
// control API, extprocs, and the decorators that extcatalog resolves. The
// process has no TTS_CORE_URL, so the only way decorator_extensions can load
// is the TTS_WEBUI_DECORATORS variable Go sets.
//
// Opt-in: EnvRealRoot (and optionally LAUNCHER_REAL_PKG, default
// tts_webui_extension.mms). The root's config.json sets the public port.
func TestRealExtensionProcessAppliesDecorators(t *testing.T) {
	root := realRoot(t)
	pkg := os.Getenv("LAUNCHER_REAL_PKG")
	if pkg == "" {
		pkg = "tts_webui_extension.mms"
	}
	// Unbuffered, so the log has the decorator lines by the time it is ready.
	baseEnv := append(os.Environ(), "MAMBA_ROOT_PREFIX="+filepath.Join(root.InstallerFiles(), "mamba"), "PYTHONUNBUFFERED=1", "PYTHONIOENCODING=utf-8")
	sp, err := startServerPy(root, realPrefix(root), baseEnv, appenv.Flags{NoReact: true})
	if err != nil {
		t.Fatal(err)
	}
	defer sp.Close()
	env := envMap(sp.gradioEnv)
	url, token := env["TTS_CORE_URL"], env["TTS_CORE_TOKEN"]

	port, err := freePort()
	if err != nil {
		t.Fatal(err)
	}
	code, body := postJSON(t, url+"/api/extensions/"+pkg+"/start", token, extprocs.StartRequest{Port: port})
	if code != http.StatusOK {
		t.Fatalf("start %s = %d %s", pkg, code, body)
	}
	waitReady(t, url, token, pkg, 170*time.Second)

	log := extensionLog(t, url, token, pkg)
	t.Logf("extension log:\n%s", log)
	if !strings.Contains(log, "Loading decorator") {
		t.Errorf("the extension process did not load decorators from TTS_WEBUI_DECORATORS:\n%s", log)
	}
	for _, bad := range []string{"CoreBridgeError", "TTS_CORE_URL"} {
		if strings.Contains(log, bad) {
			t.Errorf("the extension process asked for the core (%q):\n%s", bad, log)
		}
	}
	if code, body := get(t, "http://"+sp.publicAddr+"/"+pkg+"/", ""); code != http.StatusOK {
		t.Errorf("GET /%s/ through the router = %d %.200s", pkg, code, body)
	}
	if code, body := postJSON(t, url+"/api/extensions/"+pkg+"/stop", token, struct{}{}); code != http.StatusOK {
		t.Errorf("stop %s = %d %s", pkg, code, body)
	}
}

// EnvRealPipEnv names a throwaway Python environment (a venv made with
// --system-site-packages from the main env, so gradio is already there) that
// the install test runs pip in.
const EnvRealPipEnv = "LAUNCHER_REAL_PIP_ENV"

// TestRealPipInstallThroughBridge runs one real pip install through
// bridge.extcatalog.install: the Python bridge calls the control API, Go looks
// the package up in extensions.external.json and runs `micromamba -p <env> run
// python -m pip install <requirements> gradio==...`. The environment is the
// throwaway one, so the main env is untouched.
//
// Opt-in: EnvRealRoot, EnvRealPipEnv, LAUNCHER_REAL_PYTHON (the main env's
// python.exe, which runs the bridge) and LAUNCHER_REAL_INSTALL_PKG: the
// package_name of an entry of the root's extensions.external.json whose
// requirements are one tiny package.
func TestRealPipInstallThroughBridge(t *testing.T) {
	root := realRoot(t)
	pipEnv, py, pkg := os.Getenv(EnvRealPipEnv), os.Getenv("LAUNCHER_REAL_PYTHON"), os.Getenv("LAUNCHER_REAL_INSTALL_PKG")
	if pipEnv == "" || py == "" || pkg == "" {
		t.Skip(EnvRealPipEnv + ", LAUNCHER_REAL_PYTHON or LAUNCHER_REAL_INSTALL_PKG not set")
	}
	baseEnv := append(os.Environ(), "MAMBA_ROOT_PREFIX="+filepath.Join(root.InstallerFiles(), "mamba"), "PYTHONIOENCODING=utf-8")
	prefix := []string{micromambaPath(root), "-p", pipEnv, "run"}
	sp, err := startServerPy(root, prefix, baseEnv, appenv.Flags{NoReact: true})
	if err != nil {
		t.Fatal(err)
	}
	defer sp.Close()

	script := "import sys\n" +
		"from tts_webui.core_bridge import bridge\n" +
		"last = ''\n" +
		"for html in bridge.extcatalog.install(sys.argv[1]):\n" +
		"    last = html\n" +
		"print(last.replace('<br />', chr(10)))\n"
	cmd := exec.Command(py, "-c", script, pkg)
	cmd.Dir = root.Dir()
	cmd.Env = append(append([]string(nil), sp.gradioEnv...), "PYTHONPATH="+root.Dir())
	out, err := cmd.CombinedOutput()
	t.Logf("bridge output:\n%s", out)
	if err != nil {
		t.Fatalf("bridge install: %v", err)
	}
	if !strings.Contains(string(out), "Successfully installed") {
		t.Errorf("no \"Successfully installed\" in the install console")
	}
}

// TestRealBridgeStartsAndStopsExtension drives a real extension from Python:
// bridge.extensions.start, wait_ready and stop against the launcher's control
// API, as setup_proxy_extension does. Opt-in like the others, plus
// LAUNCHER_REAL_PYTHON (the main env's python.exe, which runs the bridge).
func TestRealBridgeStartsAndStopsExtension(t *testing.T) {
	root := realRoot(t)
	py := os.Getenv("LAUNCHER_REAL_PYTHON")
	if py == "" {
		t.Skip("LAUNCHER_REAL_PYTHON not set")
	}
	pkg := os.Getenv("LAUNCHER_REAL_PKG")
	if pkg == "" {
		pkg = "tts_webui_extension.mms"
	}
	baseEnv := append(os.Environ(), "MAMBA_ROOT_PREFIX="+filepath.Join(root.InstallerFiles(), "mamba"), "PYTHONIOENCODING=utf-8")
	sp, err := startServerPy(root, realPrefix(root), baseEnv, appenv.Flags{NoReact: true})
	if err != nil {
		t.Fatal(err)
	}
	defer sp.Close()

	script := "import sys\n" +
		"from tts_webui.core_bridge import bridge\n" +
		"pkg = sys.argv[1]\n" +
		"print('started', bridge.extensions.start(pkg, 'http://localhost:7778'))\n" +
		"print('ready', bridge.extensions.wait_ready(pkg, 170))\n" +
		"bridge.extensions.stop(pkg)\n" +
		"bridge.extensions.stop(pkg)\n" +
		"print('stopped', [s['state'] for s in bridge.extensions.status()])\n"
	cmd := exec.Command(py, "-c", script, pkg)
	cmd.Dir = root.Dir()
	cmd.Env = append(append([]string(nil), sp.gradioEnv...), "PYTHONPATH="+root.Dir())
	out, err := cmd.CombinedOutput()
	t.Logf("bridge output:\n%s", out)
	if err != nil {
		t.Fatalf("bridge: %v", err)
	}
	for _, want := range []string{"started {", "ready {", "'state': 'ready'", "stopped ['exited']"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("output lacks %q", want)
		}
	}
}

// TestRealNativeExtensionInstallAndStart installs a native extension
// (proxy: native) through bridge.extcatalog.install, which sets up
// .venvs/<package> with uv, torch and the requirements, then starts it through
// bridge.extensions: Go picks the venv's python, the process listens, the
// router serves /<package>/, and stop removes the route. It downloads torch.
//
// Opt-in: EnvRealRoot (a throwaway copy: .venvs is created in it),
// LAUNCHER_REAL_PYTHON and LAUNCHER_REAL_NATIVE_PKG, for example
// tts_webui_extension.piper_tts. installer_scripts/.gpu must exist.
func TestRealNativeExtensionInstallAndStart(t *testing.T) {
	root := realRoot(t)
	py, pkg := os.Getenv("LAUNCHER_REAL_PYTHON"), os.Getenv("LAUNCHER_REAL_NATIVE_PKG")
	if py == "" || pkg == "" {
		t.Skip("LAUNCHER_REAL_PYTHON or LAUNCHER_REAL_NATIVE_PKG not set")
	}
	baseEnv := append(os.Environ(), "MAMBA_ROOT_PREFIX="+filepath.Join(root.InstallerFiles(), "mamba"), "PYTHONIOENCODING=utf-8", "PYTHONUNBUFFERED=1")
	sp, err := startServerPy(root, realPrefix(root), baseEnv, appenv.Flags{NoReact: true})
	if err != nil {
		t.Fatal(err)
	}
	defer sp.Close()
	env := envMap(sp.gradioEnv)
	run := func(script string) string {
		cmd := exec.Command(py, "-c", script, pkg)
		cmd.Dir = root.Dir()
		cmd.Env = append(append([]string(nil), sp.gradioEnv...), "PYTHONPATH="+root.Dir())
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("python: %v\n%s", err, out)
		}
		return string(out)
	}

	install := run("import sys\n" +
		"from tts_webui.core_bridge import bridge\n" +
		"last = ''\n" +
		"for html in bridge.extcatalog.install(sys.argv[1]):\n" +
		"    last = html\n" +
		"print(last.replace('<br />', chr(10)))\n")
	t.Logf("install tail:\n%s", tail(install, 40))
	venvPython := filepath.Join(root.Dir(), ".venvs", pkg, "Scripts", "python.exe")
	if _, err := os.Stat(venvPython); err != nil {
		t.Fatalf("no venv python after the install: %v", err)
	}
	if !strings.Contains(install, "Successfully installed") {
		t.Errorf("the install console has no \"Successfully installed\"")
	}

	started := run("import sys\n" +
		"from tts_webui.core_bridge import bridge\n" +
		"pkg = sys.argv[1]\n" +
		"print('started', bridge.extensions.start(pkg))\n" +
		"print('ready', bridge.extensions.wait_ready(pkg, 170))\n")
	t.Logf("start:\n%s", started)
	log := extensionLog(t, env["TTS_CORE_URL"], env["TTS_CORE_TOKEN"], pkg)
	t.Logf("extension log (tail):\n%s", tail(log, 30))
	if code, body := get(t, "http://"+sp.publicAddr+"/"+pkg+"/", ""); code != http.StatusOK {
		t.Errorf("GET /%s/ through the router = %d %.200s", pkg, code, body)
	}
	if out := run("import sys\nfrom tts_webui.core_bridge import bridge\nbridge.extensions.stop(sys.argv[1])\nprint([s['state'] for s in bridge.extensions.status()])\n"); !strings.Contains(out, "exited") {
		t.Errorf("after stop: %s", out)
	}
}

// tail returns the last n lines of s.
func tail(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
