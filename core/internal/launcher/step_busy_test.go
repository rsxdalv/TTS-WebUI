package launcher

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
	"github.com/rsxdalv/TTS-WebUI/core/internal/procs"
)

// While one install step runs, every step request is refused with 409, a
// second click on the running step included, and the status reports the
// running step as running and not done, even when its files already exist
// (R2-3).
func TestStepsRefuseWhileAnotherStepRuns(t *testing.T) {
	root, err := paths.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root.InstallerFiles(), "mamba"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(root.InstallerScripts(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(micromambaPath(root), nil, 0o755); err != nil {
		t.Fatal(err)
	}
	writeEnvHistory(t, root)
	if err := os.WriteFile(root.MarkerFile(".gpu"), []byte("CPU"), 0o644); err != nil {
		t.Fatal(err)
	}
	l := New(root)

	release := make(chan struct{})
	p := &procs.Proc{}
	if err := p.StartFunc(context.Background(), func(ctx context.Context, log func(string)) int {
		<-release
		return 0
	}); err != nil {
		t.Fatal(err)
	}
	l.setProc("mamba", p)

	status := func() map[string]StepStatus {
		rec := httptest.NewRecorder()
		l.handleStatus().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/steps/status", nil))
		var list []StepStatus
		if err := json.NewDecoder(rec.Body).Decode(&list); err != nil {
			t.Fatal(err)
		}
		out := map[string]StepStatus{}
		for _, s := range list {
			out[s.Step] = s
		}
		return out
	}
	if m := status()["mamba"]; m.Done || !m.Running {
		t.Fatalf("mamba while running: %+v, want running and not done", m)
	}

	for name, h := range map[string]http.Handler{
		"download": l.handleDownload(), "preflight": l.handlePreflight(), "mamba": l.handleMamba(),
		"gpu": l.handleGPU(), "install": l.handleInstall(), "update": l.handleUpdate(),
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/steps/"+name, strings.NewReader(`{"label":"CPU"}`)))
		if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "mamba") {
			t.Errorf("%s while mamba runs: %d %q, want 409 naming mamba", name, rec.Code, rec.Body.String())
		}
	}
	if l.getProc("install") != nil || l.getLog("download") != nil {
		t.Fatal("a refused step started anyway")
	}

	close(release)
	deadline := time.Now().Add(5 * time.Second)
	for p.Running() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if m := status()["mamba"]; !m.Done || m.Running {
		t.Fatalf("mamba after it finished: %+v, want done", m)
	}
	rec := httptest.NewRecorder()
	l.handleGPU().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/steps/gpu", strings.NewReader(`{"label":"CPU"}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("gpu after mamba finished: %d %s", rec.Code, rec.Body.String())
	}
}
