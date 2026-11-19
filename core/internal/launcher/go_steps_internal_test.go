package launcher

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
)

// With the Go steps on, install and update refuse to start while the app runs,
// because the export build sets the running Next's .next aside.
func TestGoInstallAndUpdateRefuseWhileTheAppRuns(t *testing.T) {
	t.Setenv("TTS_GO_STEPS", "install,update")
	dir := t.TempDir()
	root, err := paths.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{root.InstallerScripts(), filepath.Join(root.InstallerFiles(), "mamba")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	mamba := "micromamba"
	if runtime.GOOS == "windows" {
		mamba += ".exe"
	}
	if err := os.WriteFile(filepath.Join(root.InstallerFiles(), "mamba", mamba), nil, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(root.MarkerFile(".gpu"), []byte("CPU"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeEnvHistory(t, root)
	l := New(root)
	fake := &runningSup{appSupervisor: l.sup, running: true}
	l.sup = fake

	rec := httptest.NewRecorder()
	l.handleInstall().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/steps/install", nil))
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "stop the app") {
		t.Fatalf("install: %d %s", rec.Code, rec.Body.String())
	}
	if p := l.getProc("install"); p != nil {
		t.Fatal("install started while the app runs")
	}
	if !l.appRunning() {
		t.Fatal("appRunning")
	}
	fake.running = false
	if l.appRunning() {
		t.Fatal("appRunning with no process")
	}
}

// runningSup is a supervisor whose Running answer the test controls.
type runningSup struct {
	appSupervisor
	running bool
}

func (r *runningSup) Running() bool { return r.running }
