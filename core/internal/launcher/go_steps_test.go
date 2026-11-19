package launcher_test

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rsxdalv/TTS-WebUI/core/internal/launcher"
	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
	"github.com/rsxdalv/TTS-WebUI/core/internal/server"
)

// TestMain lets the test binary stand in for micromamba: a copy of it placed at
// installer_files/mamba/micromamba answers "--version" and exits 0.
func TestMain(m *testing.M) {
	if os.Getenv("LAUNCHER_TEST_FAKE_MAMBA") == "1" {
		fmt.Println("9.9.9")
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func exeName() string {
	if runtime.GOOS == "windows" {
		return "micromamba.exe"
	}
	return "micromamba"
}

// stepInstall makes an install whose mamba step can run without a network: the
// fake micromamba exists and the env folder exists, so init_mamba only prints that
// it is skipping. The legacy script is a stub that prints LEGACY-MAMBA.
func stepInstall(t *testing.T) (paths.Root, *server.Server, string) {
	t.Helper()
	root, srv, token := newTestInstall(t)
	mambaDir := filepath.Join(root.InstallerFiles(), "mamba")
	if err := os.MkdirAll(mambaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root.InstallerFiles(), "env"), 0o755); err != nil {
		t.Fatal(err)
	}
	self, err := os.Open(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	defer self.Close()
	dst, err := os.OpenFile(filepath.Join(mambaDir, exeName()), os.O_CREATE|os.O_WRONLY, 0o755)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(dst, self); err != nil {
		t.Fatal(err)
	}
	dst.Close()
	t.Setenv("LAUNCHER_TEST_FAKE_MAMBA", "1")

	scripts := root.InstallerScripts()
	if runtime.GOOS == "windows" {
		os.WriteFile(filepath.Join(scripts, "init_mamba.bat"), []byte("@echo LEGACY-MAMBA\r\n"), 0o644)
		os.MkdirAll(filepath.Join(scripts, "tools"), 0o755)
		os.WriteFile(filepath.Join(scripts, "tools", "diagnostic.bat"), []byte("@echo LEGACY-DIAGNOSTIC\r\n"), 0o644)
	} else {
		os.WriteFile(filepath.Join(scripts, "init_mamba.sh"), []byte("echo LEGACY-MAMBA\n"), 0o644)
		os.MkdirAll(filepath.Join(scripts, "tools"), 0o755)
		os.WriteFile(filepath.Join(scripts, "tools", "diagnostic.sh"), []byte("echo LEGACY-DIAGNOSTIC\n"), 0o644)
	}
	return root, srv, token
}

// runStep posts the step and reads its stream to the end.
func runStep(t *testing.T, srv *server.Server, token, post, stream string) (status int, started string, out string) {
	t.Helper()
	resp := doRequest(t, srv, token, http.MethodPost, post, "")
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return resp.StatusCode, string(b), ""
	}
	resp = doRequest(t, srv, token, http.MethodGet, stream, "")
	b2, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b), string(b2)
}

func TestGoSteps_OffByDefaultRunsTheScript(t *testing.T) {
	t.Setenv("TTS_GO_STEPS", "")
	_, srv, token := stepInstall(t)
	status, started, out := runStep(t, srv, token, "/api/steps/mamba", "/api/steps/mamba/stream")
	if status != 200 || !strings.Contains(started, `"started":true`) {
		t.Fatalf("status=%d started=%s", status, started)
	}
	if !strings.Contains(out, "LEGACY-MAMBA") || strings.Contains(out, "skipping installation") {
		t.Fatalf("expected the legacy script, got:\n%s", out)
	}
	status, _, out = runStep(t, srv, token, "/api/app/diagnostic", "/api/app/diagnostic/stream")
	if status != 200 || !strings.Contains(out, "LEGACY-DIAGNOSTIC") {
		t.Fatalf("diagnostic default path: %d\n%s", status, out)
	}
}

func TestGoSteps_OnRunsTheGoPortAndNotTheScript(t *testing.T) {
	t.Setenv("TTS_GO_STEPS", "mamba")
	root, srv, token := stepInstall(t)
	status, started, out := runStep(t, srv, token, "/api/steps/mamba", "/api/steps/mamba/stream")
	if status != 200 || !strings.Contains(started, `"started":true`) {
		t.Fatalf("status=%d started=%s", status, started)
	}
	if strings.Contains(out, "LEGACY-MAMBA") {
		t.Fatalf("the script ran although the Go port is on:\n%s", out)
	}
	if !strings.Contains(out, "already installed in "+filepath.Join(root.InstallerFiles(), "env")+", skipping installation") {
		t.Fatalf("Go port output missing:\n%s", out)
	}
	if !strings.Contains(out, "event: done") {
		t.Fatalf("stream did not end with done:\n%s", out)
	}
	// Other steps still run their scripts.
	_, _, out = runStep(t, srv, token, "/api/app/diagnostic", "/api/app/diagnostic/stream")
	if !strings.Contains(out, "LEGACY-DIAGNOSTIC") {
		t.Fatalf("diagnostic must stay on the script:\n%s", out)
	}
}

func TestGoSteps_AllRunsDiagnosticPort(t *testing.T) {
	t.Setenv("TTS_GO_STEPS", "all")
	root, srv, token := stepInstall(t)
	// The port writes installer_scripts/diagnostic.txt through the shell.
	_, _, out := runStep(t, srv, token, "/api/app/diagnostic", "/api/app/diagnostic/stream")
	if strings.Contains(out, "LEGACY-DIAGNOSTIC") {
		t.Fatalf("the script ran although all steps are on:\n%s", out)
	}
	if runtime.GOOS != "windows" {
		// On Unix every diagnostic line runs inside the env, which the fake micromamba cannot provide.
		return
	}
	b, err := os.ReadFile(filepath.Join(root.InstallerScripts(), "diagnostic.txt"))
	if err != nil || !strings.HasPrefix(string(b), "Starting diagnostic...") {
		t.Fatalf("diagnostic.txt: %q err=%v", b, err)
	}
}

func TestGoSteps_BadSwitchValueIsAnError(t *testing.T) {
	for _, v := range []string{"mambo", "Bad Name", "mamba,,x y"} {
		t.Setenv("TTS_GO_STEPS", v)
		_, srv, token := stepInstall(t)
		resp := doRequest(t, srv, token, http.MethodPost, "/api/steps/mamba", "")
		b, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusInternalServerError || !strings.Contains(string(b), "TTS_GO_STEPS") {
			t.Fatalf("value %q: status=%d body=%s", v, resp.StatusCode, b)
		}
	}
}

var _ = launcher.New
