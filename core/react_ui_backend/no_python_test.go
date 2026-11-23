package react_ui_backend

// The core must run with no Python environment and no Gradio child. These
// tests start the React front with no Gradio backend and the real Go services
// behind it (config, env, logs, pip list, extension catalog) pointed at an
// environment that does not exist, and call every new route. Nothing may hang
// or crash; each route answers a plain state.

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/appconfig"
	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/appenv"
	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/extcatalog"
	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/logview"
	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/piplist"
)

const noPyToken = "no-python-token"

const newRouteNames = "core-pip,core-logs,core-model-locations,core-settings,core-extensions,go-gpu-info,go-ffmpeg-metadata"

// realCore starts the real loopback services of the launcher over root, with
// a command prefix whose program and env do not exist.
func realCore(t *testing.T, root string) *Core {
	t.Helper()
	missing := []string{filepath.Join(root, "installer_files", "micromamba-missing"), "-p", filepath.Join(root, "installer_files", "env"), "run"}
	cfg := appconfig.New(appconfig.Options{Root: root, Token: noPyToken})
	ext := extcatalog.New(extcatalog.Options{Root: root, Token: noPyToken, Config: cfg, Run: missing})
	mux := http.NewServeMux()
	mux.Handle(appconfig.Path, cfg.Handler())
	mux.Handle(appconfig.OpsPrefix, cfg.OpsHandler())
	mux.Handle(extcatalog.Prefix, ext.Handler())
	mux.Handle(appenv.APIPrefix, appenv.NewAPI(appenv.APIOptions{Root: root, Token: noPyToken}).Handler())
	mux.Handle(piplist.Path, piplist.New(piplist.Options{Root: root, Token: noPyToken, Run: missing}).Handler())
	mux.Handle(logview.Prefix, logview.New(logview.Options{Root: root, Token: noPyToken}).Handler())
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	core := &Core{}
	core.Set(srv.URL, noPyToken)
	return core
}

func allNewRoutes(t *testing.T, core *Core, root string) string {
	t.Helper()
	h, err := NewHandler(Config{Root: root, Port: testPort, Routes: routes(t, newRouteNames), Core: core}) // the new claims, no Gradio backend
	if err != nil {
		t.Fatal(err)
	}
	front := httptest.NewServer(h)
	t.Cleanup(front.Close)
	return front.URL
}

func call(t *testing.T, method, url, body string) (int, string) {
	t.Helper()
	var r io.Reader
	hdr := map[string]string{}
	if body != "" {
		r = strings.NewReader(body)
		hdr["Content-Type"] = "application/json"
	}
	req, _ := http.NewRequest(method, url, r)
	req.Host = "127.0.0.1:" + "31050"
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	client := &http.Client{Timeout: 15 * time.Second} // a hang fails the test
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func TestRoutesWithoutPythonAndGradio(t *testing.T) {
	root := exportFixture(t)
	t.Setenv("PATH", t.TempDir()) // no ffprobe, no python
	core := realCore(t, root)
	front := allNewRoutes(t, core, root)

	expect := func(method, path, body string, status int, contains string) string {
		t.Helper()
		code, got := call(t, method, front+path, body)
		if code != status || !strings.Contains(got, contains) {
			t.Errorf("%s %s: %d %q, want %d containing %q", method, path, code, got, status, contains)
		}
		return got
	}

	// Python is the only thing missing: the pip list says so.
	expect("GET", "/api/go-data/pip-list", "", 409, "no Python environment")
	// Logs: an empty directory is an empty list, not an error.
	expect("GET", "/api/go-data/logs/files", "", 200, "[]")
	expect("GET", "/api/go-data/logs/stats", "", 200, "Total Log Files:** 0")
	expect("GET", "/api/go-data/logs/pip-logs?kind=install", "", 200, "[]")
	// Model locations: read and write .env without Python.
	expect("GET", "/api/go-data/model-locations", "", 200, `"hf_home"`)
	expect("PUT", "/api/go-data/model-locations", `{"huggingface_hub_cache":"","hf_home":"D:/hf","torch_home":"","xdg_cache_home":""}`, 200, "saved")
	if b, err := os.ReadFile(filepath.Join(root, ".env")); err != nil || !strings.Contains(string(b), "D:/hf") {
		t.Errorf(".env: %v %q", err, b)
	}
	expect("GET", "/api/go-data/model-locations", "", 200, `"hf_home":"D:/hf"`)
	// Server settings: config.json is created from the defaults and edited.
	expect("GET", "/api/go-data/server-settings", "", 200, `"auth_users"`)
	expect("PUT", "/api/go-data/server-settings", `{"server_port":7771}`, 200, `"server_port":7771`)
	// GPU: a list, empty when there is no NVIDIA GPU.
	if code, got := call(t, "GET", front+"/api/go-data/gpu-info", ""); code != 200 || !strings.HasPrefix(got, "[") {
		t.Errorf("gpu-info: %d %q", code, got)
	}
	// FFMPEG Metadata: a clear error when ffprobe is not installed.
	cache := filepath.Join(root, "react-ui", "public", "file-input-cache")
	_ = os.MkdirAll(cache, 0o755)
	_ = os.WriteFile(filepath.Join(cache, "x.flac"), []byte("x"), 0o644)
	expect("POST", "/api/go-data/ffmpeg-metadata", `{"name":"x.flac"}`, 503, "ffprobe was not found")
	if code, got := call(t, "GET", front+"/api/go-data/extensions/interface", ""); code != 200 || !strings.HasPrefix(strings.TrimSpace(got), "[") {
		t.Errorf("extensions: %d %q", code, got)
	}
	// The pages themselves are static files.
	for _, p := range []string{"/tools/pip_list", "/tools/log_viewer", "/tools/gpu_info", "/tools/simple_remixer"} {
		expect("GET", p, "", 200, "")
	}
}

func TestRoutesWithoutAnyCore(t *testing.T) {
	root := exportFixture(t)
	t.Setenv("PATH", t.TempDir())
	// The control API is gone (the Gradio side closed it): routes that need it
	// say so; routes that do not need it still work.
	front := allNewRoutes(t, &Core{}, root)
	for _, p := range []string{"/api/go-data/pip-list", "/api/go-data/logs/files", "/api/go-data/model-locations", "/api/go-data/server-settings", "/api/go-data/extensions/interface"} {
		if code, got := call(t, "GET", front+p, ""); code != 503 || !strings.Contains(got, "core API is not available") {
			t.Errorf("%s: %d %q", p, code, got)
		}
	}
	if code, got := call(t, "GET", front+"/api/go-data/gpu-info", ""); code != 200 || !strings.HasPrefix(got, "[") {
		t.Errorf("gpu-info: %d %q", code, got)
	}
}
