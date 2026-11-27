package extcatalog

import (
	"encoding/json"
	"github.com/rsxdalv/TTS-WebUI/core/internal/gpu"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform/legacyenv"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform/uv"
)

// uvStandIn is the uv platform with this test binary (TestHelperProcess) in the
// place of uv: it records what uv would be asked and downloads nothing.
func uvStandIn(t *testing.T, root string) *uv.Platform {
	t.Helper()
	pr, err := paths.New(root)
	if err != nil {
		t.Fatal(err)
	}
	return uv.New(uv.Options{Root: pr, SkipPathCheck: true, Prefix: []string{os.Args[0], "-test.run=^TestHelperProcess$", "--"}})
}

// markUVInstalled writes what a finished uv platform install leaves.
func markUVInstalled(t *testing.T, root, gpuLabel string) {
	t.Helper()
	dir := filepath.Join(root, "platforms", "uv")
	if err := os.MkdirAll(filepath.Join(dir, "python"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, "platforms/uv/platform.json", `{"name":"uv","uv_version":"`+uv.Version+`","python":"`+uv.PythonVersion+`","gpu":"`+gpuLabel+`","installed_at":"2026-01-01T00:00:00Z"}`)
}

func nativeStore(t *testing.T) (*Store, string, string) {
	t.Helper()
	s, root, log := installStore(t)
	s.opts.UV = uvStandIn(t, root)
	writeFile(t, root, "extensions.json", `{"tabs":[`+
		entry("ext.native", "Native Ext", `"proxy":"native"`)+`,`+
		entry("ext.other", "Other Ext", `"proxy":"native"`)+`]}`)
	return s, root, log
}

func TestNativeInstallAsksForTheGPUThenInstallsUVFirst(t *testing.T) {
	s, root, log := nativeStore(t)
	// No GPU choice: nothing runs, and the answer carries the proposal.
	code, body := call(t, s.Handler(), "POST", Prefix+"install", `{"package_name":"ext.native"}`, "tok")
	var ask struct {
		Needs    string `json:"needs"`
		Proposed string `json:"proposed"`
		Choices  []struct{ Label, Row string }
	}
	if code != 409 || json.Unmarshal([]byte(body), &ask) != nil || ask.Needs != "gpu_choice" || ask.Proposed == "" || len(ask.Choices) < 2 {
		t.Fatalf("no choice: %d %s", code, body)
	}
	// The older-NVIDIA choice is offered on torch cu126 beside the cu128 one.
	rows := map[string]string{}
	for _, c := range ask.Choices {
		rows[c.Label] = c.Row
	}
	if rows["NVIDIA GPU"] != "cu128" || rows[string(gpu.NVIDIALegacy)] != "cu126" {
		t.Errorf("NVIDIA choices: %v", rows)
	}
	if _, err := os.Stat(log); err == nil {
		t.Fatal("something ran before the GPU was chosen")
	}
	if code, body = call(t, s.Handler(), "POST", Prefix+"install", `{"package_name":"ext.native","gpu":"Quantum"}`, "tok"); code != 400 {
		t.Fatalf("a bad label: %d %s", code, body)
	}

	_, got, _ := stream(t, s, `install {"package_name":"ext.native","gpu":"Custom (Manual torch install)"}`)
	if !got.ok {
		t.Fatalf("install: %+v", got.lines)
	}
	joined := strings.Join(got.lines, "\n")
	for _, want := range []string{"Installing the uv platform (step 1 of 2)", "Installing ext.native (step 2 of 2)",
		"Setting up virtual environment and installing dependencies for Native Ext...",
		"Successfully set up virtual environment for Native Ext with dependencies"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in:\n%s", want, joined)
		}
	}
	l := readLog(t, log)
	venv := filepath.Join(root, ".venvs", "ext.native")
	py := filepath.Join(venv, "bin", "python")
	if os.PathSeparator == '\\' {
		py = filepath.Join(venv, "Scripts", "python.exe")
	}
	cache := " --cache-dir " + filepath.Join(root, "cache") + " --no-config"
	want := []string{
		"python install 3.10.11 --no-bin --no-registry",
		"venv " + venv + " --python 3.10.11 --managed-python --no-python-downloads --allow-existing" + cache,
		"pip install --python " + py + cache + " torch==2.11.0 torchvision torchaudio==2.11.0",
		"pip install --python " + py + cache + " torchcodec --index-url=https://download.pytorch.org/whl/cpu",
		"pip install --python " + py + cache + " git+https://example.com/ext.native@main gradio<=5.49.1 gradio-goodtabs>=0.0.5 gradio-goodtab>=0.0.5 gradio-iconbutton>=0.0.1 ffmpeg-python==0.2.0 matplotlib scipy pip",
	}
	if len(l) != len(want) {
		t.Fatalf("commands:\n%s", strings.Join(l, "\n"))
	}
	for i := range want {
		if l[i] != want[i] {
			t.Errorf("command %d:\n got %s\nwant %s", i+1, l[i], want[i])
		}
	}
	for _, f := range []string{"platforms/uv/platform.json", ".venvs/ext.native/entry.json", "installer_scripts/logs/uv-venv-install-Native Ext.log", "installer_scripts/logs/uv-platform-install-uv.log"} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(f))); err != nil {
			t.Errorf("%s: %v", f, err)
		}
	}
	if _, err := os.Stat(filepath.Join(venv, "installing.json")); err == nil {
		t.Error("the install marker was left")
	}

	// A second engine needs no platform step and no choice.
	os.Remove(log)
	_, got, _ = stream(t, s, `install {"package_name":"ext.other"}`)
	if !got.ok || strings.Contains(strings.Join(got.lines, "\n"), "platform") {
		t.Fatalf("second engine: ok=%v\n%s", got.ok, strings.Join(got.lines, "\n"))
	}
	if l := readLog(t, log); len(l) != 4 {
		t.Fatalf("second engine commands: %q", l)
	}
	g, _ := s.RequiresGraph(t.Context())
	if !g.Entries["ext.native"].Installed || !g.Entries["ext.other"].Installed {
		t.Errorf("installed: %+v", g.Entries)
	}
	if v := g.State("ext.native"); len(v) != 1 || v[0].State != "ready" || v[0].Via != "uv" {
		t.Errorf("requirement state: %+v", v)
	}
}

func TestUninstallingANativeEntryDeletesItsVenv(t *testing.T) {
	s, root, log := nativeStore(t)
	markUVInstalled(t, root, "CPU")
	if _, got, _ := stream(t, s, `install {"package_name":"ext.native"}`); !got.ok {
		t.Fatal("install failed")
	}
	venv := filepath.Join(root, ".venvs", "ext.native")
	if _, err := os.Stat(venv); err != nil {
		t.Fatal(err)
	}
	os.Remove(log)
	var stopped []string
	s.opts.StopEngine = func(pkg string) bool { stopped = append(stopped, pkg); return true }
	_, got, _ := stream(t, s, `uninstall {"package_name":"ext.native"}`)
	text := strings.Join(got.lines, "\n")
	if !got.ok || !strings.Contains(text, "Disk freed:") || !strings.Contains(text, "Successfully uninstalled Native Ext") {
		t.Fatalf("uninstall: ok=%v\n%s", got.ok, text)
	}
	if _, err := os.Stat(venv); err == nil {
		t.Fatal("the venv is still there")
	}
	if len(stopped) != 1 || stopped[0] != "ext.native" || !strings.Contains(text, "Stopped the running engine") {
		t.Fatalf("the running engine was not stopped first: %v\n%s", stopped, text)
	}
	if _, err := os.Stat(log); err == nil {
		t.Fatal("removing a venv ran a command (pip uninstall in the main env)")
	}
}

func TestAFailedNativeInstallIsPartialAndRunsAgain(t *testing.T) {
	s, root, _ := nativeStore(t)
	markUVInstalled(t, root, "CPU")
	good := entry("ext.native", "Native Ext", `"proxy":"native"`)
	writeFile(t, root, "extensions.json", `{"tabs":[`+strings.Replace(good, "git+https://example.com/ext.native@main", "FAILME", 1)+`]}`)
	if _, got, _ := stream(t, s, `install {"package_name":"ext.native"}`); got.ok {
		t.Fatal("the failing step reported success")
	}
	g, _ := s.RequiresGraph(t.Context())
	if g.Entries["ext.native"].Installed {
		t.Fatal("an unfinished install counts as installed")
	}
	if _, err := os.Stat(filepath.Join(root, ".venvs", "ext.native", "installing.json")); err != nil {
		t.Errorf("the install marker: %v", err)
	}
	writeFile(t, root, "extensions.json", `{"tabs":[`+good+`]}`)
	if _, got, _ := stream(t, s, `install {"package_name":"ext.native"}`); !got.ok {
		t.Fatal("running it again did not complete")
	}
	if g, _ = s.RequiresGraph(t.Context()); !g.Entries["ext.native"].Installed {
		t.Fatal("not installed after the second run")
	}
}

func TestEntryFieldsPythonAndTorchRowReachTheInstall(t *testing.T) {
	s, root, log := nativeStore(t)
	markUVInstalled(t, root, "CPU")
	writeFile(t, root, "extensions.json", `{"tabs":[`+entry("ext.native", "Native Ext", `"proxy":"native","python":"3.11","torch":"pypi"`)+`]}`)
	if _, got, _ := stream(t, s, `install {"package_name":"ext.native"}`); !got.ok {
		t.Fatal("install failed")
	}
	l := readLog(t, log)
	if l[0] != "python install 3.11 --no-bin --no-registry" || !strings.Contains(l[1], "--python 3.11 ") || strings.Contains(l[2], "--index-url") {
		t.Fatalf("commands: %q", l)
	}
	writeFile(t, root, "extensions.json", `{"tabs":[`+entry("ext.native", "Native Ext", `"proxy":"native","torch":"cu999"`)+`]}`)
	os.Remove(log)
	_, got, _ := stream(t, s, `install {"package_name":"ext.native"}`)
	if got.ok || !strings.Contains(strings.Join(got.lines, "\n"), "unknown torch row") {
		t.Fatalf("an unknown row must fail loudly: %+v", got.lines)
	}
}

func TestUVPlatformEndpoint(t *testing.T) {
	s, root, _ := nativeStore(t)
	_, body := call(t, s.Handler(), "GET", Prefix+"uv-platform", "", "tok")
	var a map[string]any
	if err := json.Unmarshal([]byte(body), &a); err != nil || a["available"] != true || a["installed"] != false || a["proposed"] == "" {
		t.Fatalf("not installed: %v %s", err, body)
	}
	markUVInstalled(t, root, "CPU")
	_, body = call(t, s.Handler(), "GET", Prefix+"uv-platform", "", "tok")
	if err := json.Unmarshal([]byte(body), &a); err != nil || a["installed"] != true || a["gpu"] != "CPU" {
		t.Fatalf("installed: %v %s", err, body)
	}
	bare := New(Options{Root: root, Token: "tok"})
	_, body = call(t, bare.Handler(), "GET", Prefix+"uv-platform", "", "tok")
	if !strings.Contains(body, `"available":false`) && !strings.Contains(body, `"available": false`) {
		t.Fatalf("no uv: %s", body)
	}
}

// With no uv platform object at all (a dev server), a native entry says what it
// needs and nothing runs.
func TestNativeInstallWithoutAUVPlatformIsRefused(t *testing.T) {
	root := t.TempDir()
	pr, _ := paths.New(root)
	s := New(Options{Root: root, Token: "tok", Platform: legacyenv.New(legacyenv.Options{Root: pr})})
	writeFile(t, root, "extensions.json", `{"tabs":[`+entry("eng", "Eng", `"proxy":"native"`)+`,`+entry("tab", "Tab")+`]}`)
	code, body := call(t, s.Handler(), "POST", Prefix+"install", `{"package_name":"eng"}`, "tok")
	if code != 400 || !strings.Contains(body, "eng requires uv (the uv platform is not installed)") {
		t.Fatalf("native: %d %s", code, body)
	}
	code, body = call(t, s.Handler(), "POST", Prefix+"install", `{"package_name":"tab"}`, "tok")
	if code != 400 || !strings.Contains(body, "tab requires gradio-host") {
		t.Fatalf("tab: %d %s", code, body)
	}
	if _, err := os.Stat(filepath.Join(root, ".venvs")); err == nil {
		t.Error("a refused install made .venvs")
	}
}

// finishedVenv makes the folder a finished own-venv install leaves.
func finishedVenv(t *testing.T, root, pkg string) {
	t.Helper()
	writeFile(t, root, ".venvs/"+pkg+"/entry.json", `{"package":"`+pkg+`","platform":"uv"}`)
}

func TestNativeInstallWithoutGitIsRefusedBeforeAnyStep(t *testing.T) {
	s, root, log := nativeStore(t)
	t.Setenv("PATH", t.TempDir()) // a machine with no git
	// With and without the GPU choice: the git answer comes first, so the page does
	// not ask for a GPU and then fail.
	for _, body := range []string{`{"package_name":"ext.native"}`, `{"package_name":"ext.native","gpu":"CPU"}`} {
		code, got := call(t, s.Handler(), "POST", Prefix+"install", body, "tok")
		var ans struct{ Error, Needs string }
		if code != 409 || json.Unmarshal([]byte(got), &ans) != nil || ans.Needs != "git" {
			t.Fatalf("%s: %d %s", body, code, got)
		}
		// The install hint is per operating system (apt on Linux, xcode-select on macOS, a plain
		// "Install Git" elsewhere); the rest of the message is the same everywhere.
		hint := map[string]string{"linux": "apt install git", "darwin": "xcode-select --install"}[runtime.GOOS]
		if hint == "" {
			hint = "Install Git for your system"
		}
		for _, want := range []string{"Native Ext installs from a git URL", "git is not on this machine", hint, "nothing was installed"} {
			if !strings.Contains(ans.Error, want) {
				t.Errorf("error lacks %q: %s", want, ans.Error)
			}
		}
	}
	if _, err := os.Stat(log); err == nil {
		t.Error("uv was run")
	}
	for _, f := range []string{"platforms", ".venvs"} {
		if _, err := os.Stat(filepath.Join(root, f)); err == nil {
			t.Errorf("%s was made", f)
		}
	}
	// The same install with a git on PATH is not refused.
	bin := t.TempDir()
	name := "git"
	if os.PathSeparator == '\\' {
		name = "git.exe"
	}
	if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	if code, got := call(t, s.Handler(), "POST", Prefix+"install", `{"package_name":"ext.native"}`, "tok"); code != 409 || !strings.Contains(got, "gpu_choice") {
		t.Fatalf("with git: %d %s", code, got)
	}
}

// The older-NVIDIA label installs the cu126 row, with no xformers.
func TestNativeInstallOnTheOlderNVIDIAChoiceUsesCu126(t *testing.T) {
	s, _, log := nativeStore(t)
	_, got, _ := stream(t, s, `install {"package_name":"ext.native","gpu":"`+string(gpu.NVIDIALegacy)+`"}`)
	if !got.ok {
		t.Fatalf("install: %+v", got.lines)
	}
	l := strings.Join(readLog(t, log), "\n")
	if !strings.Contains(l, "torch==2.11.0+cu126") || !strings.Contains(l, "https://download.pytorch.org/whl/cu126") || strings.Contains(l, "xformers") {
		t.Errorf("uv log:\n%s", l)
	}
}
