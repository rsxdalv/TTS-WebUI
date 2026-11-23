package launcher

// The Go core boots without Gradio. These tests run the launcher's own
// supervisor wiring (superviseConfig: react prep, the Go side with its real
// control API and services, the React front) on a temp install that has no
// Python environment, and with a Gradio child that dies at once.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/supervise"
)

// TestGradioExitsAtOnce is the "Gradio" the tests below run: this test binary,
// which ends with exit code 3 when LAUNCHER_FAKE_GRADIO is set.
func TestGradioExitsAtOnce(t *testing.T) {
	if os.Getenv("LAUNCHER_FAKE_GRADIO") == "1" {
		os.Exit(3)
	}
}

func freeLoopbackPort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

// coreInstall is a temp install with a static export, a config.json on free
// ports and nothing else: no installer_files, no Python.
func coreInstall(t *testing.T) (root paths.Root, publicPort, reactPort int) {
	t.Helper()
	dir := t.TempDir()
	root, err := paths.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "react-ui", "out")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"_tts-export.json": `{}`, "index.html": "<html><body>TTS WebUI</body></html>"} {
		if err := os.WriteFile(filepath.Join(out, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	publicPort, reactPort = freeLoopbackPort(t), freeLoopbackPort(t)
	cfg := fmt.Sprintf(`{"gradio_interface_options":{"server_port":%d,"server_name":"127.0.0.1","auth":null,"inbrowser":false}}`, publicPort)
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, publicPort, reactPort
}

func httpGet(t *testing.T, url string) (int, string) {
	t.Helper()
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func gradioState(t *testing.T, sup *supervise.Supervisor) supervise.ProcessStatus {
	t.Helper()
	for _, st := range sup.Statuses() {
		if st.Name == supervise.NameGradio {
			return st
		}
	}
	t.Fatal("no gradio status")
	return supervise.ProcessStatus{}
}

func startCore(t *testing.T, root paths.Root, reactPort int, mutate func(*supervise.Config)) *supervise.Supervisor {
	t.Helper()
	cfg := superviseConfig(root, options{noBrowser: true})
	cfg.ReactPort = reactPort
	cfg.SkipReactTab = true
	if mutate != nil {
		mutate(&cfg)
	}
	sup, err := supervise.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(sup.Stop)
	return sup
}

func TestCoreBootsWithNoPythonEnvironment(t *testing.T) {
	root, _, reactPort := coreInstall(t)
	sup := startCore(t, root, reactPort, nil) // no .gpu either
	if err := sup.Start(context.Background()); err != nil {
		t.Fatalf("Start with no Python env must succeed: %v", err)
	}
	if g := gradioState(t, sup); g.State != supervise.StateFailed || !strings.Contains(g.Error, "no Python environment") {
		t.Fatalf("gradio: %+v", g)
	}
	base := fmt.Sprintf("http://127.0.0.1:%d", reactPort)
	if code, body := httpGet(t, base+"/"); code != 200 || !strings.Contains(body, "TTS WebUI") {
		t.Fatalf("front: %d %q", code, body)
	}
	// Routes backed by the real control API and services, with no Python.
	if code, body := httpGet(t, base+"/api/go-data/logs/files"); code != 200 || strings.TrimSpace(body) != "[]" {
		t.Errorf("logs: %d %q", code, body)
	}
	if code, body := httpGet(t, base+"/api/go-data/model-locations"); code != 200 || !strings.Contains(body, "hf_home") {
		t.Errorf("model locations: %d %q", code, body)
	}
	if code, body := httpGet(t, base+"/api/go-data/pip-list"); code != 409 || !strings.Contains(body, "no Python environment") {
		t.Errorf("pip list: %d %q", code, body)
	}
	if code, body := httpGet(t, base+"/api/go-data/server-settings"); code != 200 || !strings.Contains(body, "server_port") {
		t.Errorf("settings: %d %q", code, body)
	}
	sup.Stop()
	if c, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", reactPort), 200*time.Millisecond); err == nil {
		_ = c.Close()
		t.Error("the front must be closed after Stop")
	}
}

func TestControlAPIStaysUpWhenGradioDies(t *testing.T) {
	root, _, reactPort := coreInstall(t)
	if err := os.MkdirAll(root.InstallerScripts(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(root.MarkerFile(".gpu"), []byte("CPU\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sup := startCore(t, root, reactPort, func(c *supervise.Config) {
		// A "Python" that exists but whose Gradio exits with code 3 at once.
		c.CommandPrefix = []string{os.Args[0], "-test.run=^TestGradioExitsAtOnce$", "--"}
		c.ExtraEnv = append(c.ExtraEnv, "LAUNCHER_FAKE_GRADIO=1")
	})
	if err := sup.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	deadline := time.Now().Add(15 * time.Second)
	for gradioState(t, sup).State != supervise.StateExited && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	g := gradioState(t, sup)
	if g.State != supervise.StateExited || g.ExitCode == nil || *g.ExitCode != 3 {
		t.Fatalf("gradio: %+v", g)
	}
	// Gradio is gone; the control API and the services behind it are not.
	base := fmt.Sprintf("http://127.0.0.1:%d", reactPort)
	for i := 0; i < 3; i++ {
		if code, body := httpGet(t, base+"/api/go-data/logs/stats"); code != 200 || !strings.Contains(body, "Total Log Files") {
			t.Fatalf("logs/stats after gradio exit: %d %q", code, body)
		}
		time.Sleep(100 * time.Millisecond)
	}
	var settings map[string]any
	code, body := httpGet(t, base+"/api/go-data/server-settings")
	if code != 200 || json.Unmarshal([]byte(body), &settings) != nil {
		t.Fatalf("settings: %d %q", code, body)
	}
}

// No .gpu marker with a Python that can start: Start goes on, the status says so.
func TestMissingGPULabelIsOnlyAWarning(t *testing.T) {
	root, _, reactPort := coreInstall(t) // no installer_scripts/.gpu
	sup := startCore(t, root, reactPort, func(c *supervise.Config) {
		c.CommandPrefix = []string{os.Args[0], "-test.run=^TestGradioExitsAtOnce$", "--"}
		c.ExtraEnv = append(c.ExtraEnv, "LAUNCHER_FAKE_GRADIO=1")
	})
	if err := sup.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if g := gradioState(t, sup); !strings.Contains(g.Warning, "GPU label not set") {
		t.Fatalf("gradio status: %+v", g)
	}
	base := fmt.Sprintf("http://127.0.0.1:%d", reactPort)
	if code, _ := httpGet(t, base+"/api/go-data/logs/files"); code != 200 {
		t.Fatalf("core route: %d", code)
	}
}
