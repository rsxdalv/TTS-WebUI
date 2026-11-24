package extprocs

import (
	"context"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform/system"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
)

// TestRealExtension starts one real installed extension through the Manager.
// Opt-in: set EXTPROCS_REAL_PYTHON to a python.exe that has gradio and the
// extension installed, EXTPROCS_REAL_ROOT to the repo root (cwd and
// PYTHONPATH of the child) and optionally EXTPROCS_REAL_PKG.
func TestRealExtension(t *testing.T) {
	py, root := os.Getenv("EXTPROCS_REAL_PYTHON"), os.Getenv("EXTPROCS_REAL_ROOT")
	if py == "" || root == "" {
		t.Skip("EXTPROCS_REAL_PYTHON / EXTPROCS_REAL_ROOT not set")
	}
	pkg := os.Getenv("EXTPROCS_REAL_PKG")
	if pkg == "" {
		pkg = "tts_webui_extension.log_viewer"
	}
	r, err := paths.New(root)
	if err != nil {
		t.Fatal(err)
	}
	fr := &fakeRoutes{}
	m := New(Options{
		Root: r,
		Platform: system.New(system.Options{
			Root: r, Python: py,
			Env: []string{"PYTHONPATH=" + root, "GRADIO_ANALYTICS_ENABLED=False", "PYTHONIOENCODING=utf-8"},
		}),
		Routes:       fr,
		Token:        "tok",
		ReadyTimeout: 170 * time.Second,
	})
	defer m.StopAll()
	res, err := m.Start(context.Background(), pkg, StartRequest{})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("started %s pid=%d port=%d", pkg, res.Pid, res.Port)
	deadline := time.Now().Add(175 * time.Second)
	for state(m, pkg) != StateReady {
		if time.Now().After(deadline) || state(m, pkg) == StateFailed {
			lines, _ := m.Lines(pkg)
			t.Fatalf("state %q; log:\n%s", state(m, pkg), strings.Join(lines, "\n"))
		}
		time.Sleep(200 * time.Millisecond)
	}
	if _, ok := fr.get("/" + pkg); !ok {
		t.Fatal("route not registered")
	}
	resp, err := http.Get("http://127.0.0.1:" + strconv.Itoa(res.Port) + "/" + pkg + "/")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	t.Logf("GET /%s/ -> %d", pkg, resp.StatusCode)
	if err := m.Stop(pkg); err != nil {
		t.Fatal(err)
	}
	if _, ok := fr.get("/" + pkg); ok {
		t.Fatal("route left after stop")
	}
}
