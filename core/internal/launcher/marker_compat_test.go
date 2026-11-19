package launcher_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/rsxdalv/TTS-WebUI/core/installer_scripts"
	"github.com/rsxdalv/TTS-WebUI/core/internal/gpu"
	"github.com/rsxdalv/TTS-WebUI/core/internal/launcher"
)

// testdata/legacy-markers holds the bytes the legacy flow really wrote: they were
// copied from the baseline install made by the Go launcher running the unmodified
// scripts (init_app.js and initializeApp.js write these files). The files have no
// leading dot only so that tools do not hide them; testdata/.gitattributes keeps
// their bytes (python_version ends in CRLF).
var legacyMarkers = []string{"gpu", "major_version", "pip_packages", "python_version", "git_version"}

func goldenMarker(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "legacy-markers", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func installWithLegacyMarkers(t *testing.T) string {
	t.Helper()
	root, _, _ := newTestInstall(t)
	for _, m := range legacyMarkers {
		if err := os.WriteFile(filepath.Join(root.InstallerScripts(), "."+m), goldenMarker(t, m), 0o666); err != nil {
			t.Fatal(err)
		}
	}
	return root.Dir()
}

func TestMarkers_GoldenBytesAreWhatTheLegacyFlowWrites(t *testing.T) {
	if got := string(goldenMarker(t, "gpu")); got != "NVIDIA GPU" {
		t.Fatalf(".gpu = %q", got)
	}
	if got := string(goldenMarker(t, "major_version")); got != "1.0.0" {
		t.Fatalf(".major_version = %q", got)
	}
	if got := string(goldenMarker(t, "python_version")); got != "Python 3.10.11\r\n" {
		t.Fatalf(".python_version = %q", got)
	}
	if got := goldenMarker(t, "git_version"); len(got) != 40 || bytes.ContainsAny(got, "\r\n ") {
		t.Fatalf(".git_version = %q", got)
	}
	// The pinned versions in the repo are the ones the legacy flow wrote.
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "installer_scripts", "versions.json"))
	if err != nil {
		t.Skipf("versions.json not available: %v", err)
	}
	var v struct {
		Version     string `json:"version"`
		PipPackages string `json:"pip_packages"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	if v.Version != string(goldenMarker(t, "major_version")) || v.PipPackages != string(goldenMarker(t, "pip_packages")) {
		t.Fatalf("versions.json %+v differs from the markers a real install wrote: re-run an install and update testdata", v)
	}
}

func TestMarkers_LauncherReadsLegacyMarkers(t *testing.T) {
	root, srv, token := newTestInstall(t)
	for _, m := range legacyMarkers {
		os.WriteFile(filepath.Join(root.InstallerScripts(), "."+m), goldenMarker(t, m), 0o666)
	}
	// micromamba exists too, so every marker-based step is done.
	os.MkdirAll(filepath.Join(root.InstallerFiles(), "mamba"), 0o755)
	os.WriteFile(filepath.Join(root.InstallerFiles(), "mamba", exeName()), []byte("x"), 0o755)
	writeEnvHistory(t, root)
	os.WriteFile(filepath.Join(root.InstallerScripts(), ".preflight_ok"), []byte("ok\n"), 0o666)

	resp := doRequest(t, srv, token, http.MethodGet, "/api/steps/status", "")
	var statuses []launcher.StepStatus
	if err := json.NewDecoder(resp.Body).Decode(&statuses); err != nil {
		t.Fatal(err)
	}
	done := map[string]bool{}
	for _, s := range statuses {
		done[s.Step] = s.Done
	}
	for _, step := range []string{"download", "preflight", "mamba", "gpu", "install"} {
		if !done[step] {
			t.Errorf("step %s is not done with the legacy markers in place", step)
		}
	}
	// The app supervisor accepts the legacy .gpu text as one of its labels.
	if _, err := gpu.Parse(string(goldenMarker(t, "gpu"))); err != nil {
		t.Fatalf("gpu.Parse of the legacy .gpu: %v", err)
	}
}

func TestMarkers_GPUStepWritesTheLegacyBytes(t *testing.T) {
	for _, label := range []string{"NVIDIA GPU", "AMD GPU (ROCM, Linux only)", "Apple M Series Chip", "Intel GPU (XPU)", "CPU", "Custom (Manual torch install)"} {
		root, srv, token := newTestInstall(t)
		body, _ := json.Marshal(map[string]string{"label": label})
		resp := doRequest(t, srv, token, http.MethodPost, "/api/steps/gpu", string(body))
		io.Copy(io.Discard, resp.Body)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s: status %d", label, resp.StatusCode)
		}
		got, err := os.ReadFile(filepath.Join(root.InstallerScripts(), ".gpu"))
		if err != nil || string(got) != label {
			t.Fatalf("%s: .gpu = %q (err %v), want the label with no newline", label, got, err)
		}
	}
	// Same bytes as the golden legacy file for the label the legacy flow wrote.
	root, srv, token := newTestInstall(t)
	doRequest(t, srv, token, http.MethodPost, "/api/steps/gpu", `{"label":"NVIDIA GPU"}`)
	got, _ := os.ReadFile(filepath.Join(root.InstallerScripts(), ".gpu"))
	if !bytes.Equal(got, goldenMarker(t, "gpu")) {
		t.Fatalf(".gpu = %q, golden %q", got, goldenMarker(t, "gpu"))
	}
}

// markerRunner answers the commands of init_app.js. git rev-parse HEAD returns head.
type markerRunner struct {
	mu   sync.Mutex
	head string
	cmds []string
}

func (r *markerRunner) Run(ctx context.Context, c installer_scripts.Command) (installer_scripts.Result, error) {
	key := c.Shell
	if key == "" {
		key = strings.TrimSpace(c.Name + " " + strings.Join(c.Args, " "))
	}
	r.mu.Lock()
	r.cmds = append(r.cmds, key)
	r.mu.Unlock()
	if key == "git rev-parse HEAD" {
		return installer_scripts.Result{Stdout: r.head + "\n"}, nil
	}
	return installer_scripts.Result{}, nil
}

func (r *markerRunner) installed() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, c := range r.cmds {
		if strings.HasPrefix(c, "pip install") || strings.Contains(c, "npm install") {
			return true
		}
	}
	return false
}

// The Go init_app port reads an install made by the legacy flow: with the legacy
// .git_version equal to HEAD it installs nothing and leaves every marker alone, and
// with a different HEAD it reinstalls and writes the new hash as the legacy flow
// does (trimmed, no newline).
func TestMarkers_GoInitAppReadsLegacyInstall(t *testing.T) {
	for _, tc := range []struct {
		name, head string
		reinstalls bool
	}{
		{"same-head", strings.TrimSpace(string(goldenMarker(t, "git_version"))), false},
		{"new-head", "0123456789abcdef0123456789abcdef01234567", true},
	} {
		dir := installWithLegacyMarkers(t)
		os.MkdirAll(filepath.Join(dir, ".git"), 0o755)
		os.MkdirAll(filepath.Join(dir, "react-ui", "src", "extensions"), 0o755)
		os.WriteFile(filepath.Join(dir, "installer_scripts", "versions.json"), []byte(`{"version":"1.0.0","pip_packages":"1.2.0","npm_packages":4,"react_ui":4}`), 0o666)
		r := &markerRunner{head: tc.head}
		var log []string
		s := &installer_scripts.Step{Root: dir, Run: r, Log: func(l string) { log = append(log, l) }}
		code, err := installer_scripts.InitAppJS(context.Background(), s)
		if err != nil || code != 0 {
			t.Fatalf("%s: code=%d err=%v log=%q", tc.name, code, err, log)
		}
		if r.installed() != tc.reinstalls {
			t.Fatalf("%s: installs=%v, want %v (commands %q)", tc.name, r.installed(), tc.reinstalls, r.cmds)
		}
		// python_version is rewritten by the port's own "python --version" (stubbed
		// here), so only the markers the port must leave alone are compared.
		for _, m := range []string{"gpu", "major_version", "pip_packages"} {
			got, _ := os.ReadFile(filepath.Join(dir, "installer_scripts", "."+m))
			if !bytes.Equal(got, goldenMarker(t, m)) {
				t.Errorf("%s: .%s = %q, legacy had %q", tc.name, m, got, goldenMarker(t, m))
			}
		}
		got, _ := os.ReadFile(filepath.Join(dir, "installer_scripts", ".git_version"))
		if string(got) != tc.head {
			t.Errorf("%s: .git_version = %q, want %q", tc.name, got, tc.head)
		}
	}
}
