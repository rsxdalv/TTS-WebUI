package uv

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/rsxdalv/TTS-WebUI/core/internal/gpu"
	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform"
)

// TestMain makes the test binary a fake uv when it is run under that name: the
// platform runs the program it installed, so the fake has to be a real
// executable. It stops at the boundary: it creates the folders and files a real
// uv would, and records its arguments and environment.
func TestMain(m *testing.M) {
	base := strings.TrimSuffix(filepath.Base(os.Args[0]), ".exe")
	if base == "uv" {
		os.Exit(fakeUV(os.Args[1:]))
	}
	os.Exit(m.Run())
}

func fakeUV(args []string) int {
	if log := os.Getenv("UV_FAKE_LOG"); log != "" {
		f, _ := os.OpenFile(log, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		f.WriteString(strings.Join(args, " ") + "\n")
		f.WriteString("ENV UV_NO_CONFIG=" + os.Getenv("UV_NO_CONFIG") + " UV_CACHE_DIR=" + os.Getenv("UV_CACHE_DIR") +
			" UV_PYTHON_INSTALL_DIR=" + os.Getenv("UV_PYTHON_INSTALL_DIR") + "\n")
		f.Close()
	}
	if len(args) == 0 {
		return 0
	}
	switch args[0] {
	case "--version":
		os.Stdout.WriteString("uv " + Version + "\n")
	case "python":
		if len(args) > 1 && args[1] == "find" {
			// A warning comes first, as uv prints UV_NATIVE_TLS's deprecation before its answer.
			os.Stderr.WriteString("warning: The `UV_NATIVE_TLS` environment variable is deprecated\n")
			exe, _ := os.Executable()
			os.Stdout.WriteString(exe + "\n")
			return 0
		}
		os.MkdirAll(filepath.Join(os.Getenv("UV_PYTHON_INSTALL_DIR"), "cpython-"+args[2]), 0o755)
		os.Stdout.WriteString("Installed Python " + args[2] + "\n")
	case "venv":
		dir := args[1]
		bin := "bin"
		exe := "python"
		if runtime.GOOS == "windows" {
			bin, exe = "Scripts", "python.exe"
		}
		os.MkdirAll(filepath.Join(dir, bin), 0o755)
		os.WriteFile(filepath.Join(dir, bin, exe), []byte("fake"), 0o755)
		os.Stdout.WriteString("Creating virtual environment\n")
	case "pip":
		if strings.Contains(os.Getenv("UV_FAKE_FAIL"), "pip") {
			os.Stderr.WriteString("error: no matching distribution\n")
			return 1
		}
		os.Stdout.WriteString("Installed packages\n")
	}
	return 0
}

// fakeArchive is a zip holding this test binary as uv(.exe).
func fakeArchive(t *testing.T) (data []byte, sum string) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	name := "uv"
	if runtime.GOOS == "windows" {
		name = "uv.exe"
	}
	w, _ := zw.Create("uv-pkg/" + name)
	w.Write(b)
	zw.Close()
	h := sha256.Sum256(buf.Bytes())
	return buf.Bytes(), hex.EncodeToString(h[:])
}

type fixture struct {
	t    *testing.T
	root paths.Root
	srv  *httptest.Server
	p    *Platform
	log  string
	out  *[]string
	emit platform.Emit
}

func newFixture(t *testing.T, l gpu.Label) *fixture {
	t.Helper()
	data, sum := fakeArchive(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(data)
	}))
	t.Cleanup(srv.Close)
	root, _ := paths.New(t.TempDir())
	f := &fixture{t: t, root: root, srv: srv, log: filepath.Join(t.TempDir(), "uv.log")}
	f.p = New(Options{Root: root, GPU: l, SkipPathCheck: true, Archive: &Archive{URL: srv.URL + "/uv.zip", SHA256: sum},
		Env: []string{"UV_FAKE_LOG=" + f.log}})
	var out []string
	f.out = &out
	f.emit = func(s string) { out = append(out, s) }
	return f
}

func (f *fixture) calls() []string {
	b, _ := os.ReadFile(f.log)
	var out []string
	for _, l := range strings.Split(string(b), "\n") {
		if l != "" && !strings.HasPrefix(l, "ENV ") {
			out = append(out, l)
		}
	}
	return out
}

func TestInstallMakesThePlatformAndRunsUvDirectly(t *testing.T) {
	f := newFixture(t, gpu.CPU)
	if st := f.p.Status(context.Background()); st.Ready || !strings.Contains(st.Detail, "not installed") {
		t.Fatalf("before install: %+v", st)
	}
	if err := f.p.Install(context.Background(), f.emit); err != nil {
		t.Fatalf("%v\n%s", err, strings.Join(*f.out, "\n"))
	}
	if st := f.p.Status(context.Background()); !st.Ready {
		t.Fatalf("after install: %+v", st)
	}
	s, ok := f.p.Info()
	if !ok || s.UVVersion != Version || s.Python != PythonVersion || s.GPU != "CPU" {
		t.Fatalf("platform.json = %+v", s)
	}
	calls := f.calls()
	want := "python install " + PythonVersion + " --no-bin --no-registry"
	if len(calls) < 1 || calls[len(calls)-1] != want {
		t.Fatalf("uv calls = %q, want the last to be %q", calls, want)
	}
	env, _ := os.ReadFile(f.log)
	for _, w := range []string{"UV_NO_CONFIG=1", "UV_CACHE_DIR=" + filepath.Join(f.root.Dir(), "cache"),
		"UV_PYTHON_INSTALL_DIR=" + filepath.Join(f.root.Dir(), "platforms", "uv", "python")} {
		if !strings.Contains(string(env), w) {
			t.Errorf("uv did not see %s:\n%s", w, env)
		}
	}
	if !strings.Contains(strings.Join(*f.out, "\n"), "one volume") {
		t.Errorf("the volume check said nothing:\n%s", strings.Join(*f.out, "\n"))
	}
	if f.p.GPU() != gpu.CPU {
		t.Errorf("GPU() = %v", f.p.GPU())
	}
}

func TestInstallNeedsAGPUChoice(t *testing.T) {
	f := newFixture(t, "")
	if err := f.p.Install(context.Background(), f.emit); err == nil || !strings.Contains(err.Error(), "GPU choice") {
		t.Fatalf("err = %v", err)
	}
	if _, err := os.Stat(f.p.Dir()); err == nil {
		t.Fatal("nothing may be made before the choice")
	}
}

func TestChecksumMismatchInstallsNothing(t *testing.T) {
	f := newFixture(t, gpu.CPU)
	f.p.archive.SHA256 = strings.Repeat("0", 64)
	err := f.p.Install(context.Background(), f.emit)
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("err = %v", err)
	}
	if _, ok := f.p.Info(); ok {
		t.Fatal("platform.json written after a failed install")
	}
	if _, err := os.Stat(f.p.UVPath()); err == nil {
		t.Fatal("an unchecked uv was left in bin/")
	}
	if m, _ := filepath.Glob(filepath.Join(f.p.Dir(), "bin", "*")); len(m) != 0 {
		t.Fatalf("leftovers: %v", m)
	}
}

func TestCancelledDownloadLeavesNothingAndTheNextInstallCompletes(t *testing.T) {
	data, sum := fakeArchive(t)
	started := make(chan struct{})
	block := true
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "99999999")
		w.Write(data[:100])
		w.(http.Flusher).Flush()
		if block {
			close(started)
			<-r.Context().Done()
			return
		}
		w.Header().Set("Content-Length", "")
	}))
	defer srv.Close()
	root, _ := paths.New(t.TempDir())
	p := New(Options{Root: root, GPU: gpu.CPU, Archive: &Archive{URL: srv.URL + "/uv.zip", SHA256: sum}})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- p.Install(ctx, func(string) {}) }()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("the download never started")
	}
	cancel()
	if err := <-done; err == nil {
		t.Fatal("a cancelled install reported success")
	}
	if _, ok := p.Info(); ok {
		t.Fatal("platform.json after a cancel")
	}
	if m, _ := filepath.Glob(filepath.Join(p.Dir(), "bin", "*")); len(m) != 0 {
		t.Fatalf("leftovers after cancel: %v", m)
	}
	// Run again against a server that answers properly.
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(data) }))
	defer good.Close()
	p = New(Options{Root: root, GPU: gpu.CPU, Archive: &Archive{URL: good.URL + "/uv.zip", SHA256: sum}})
	if err := p.Install(context.Background(), func(string) {}); err != nil {
		t.Fatalf("second install: %v", err)
	}
	if !p.Status(context.Background()).Ready {
		t.Fatal("not ready after the second install")
	}
}

func installed(t *testing.T, l gpu.Label) *fixture {
	f := newFixture(t, l)
	if err := f.p.Install(context.Background(), func(string) {}); err != nil {
		t.Fatal(err)
	}
	os.Remove(f.log)
	return f
}

func TestCreateRunsTheStepsOfTheRowAndWritesEntryJSONLast(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("the NVIDIA row does not exist on macOS")
	}
	f := installed(t, gpu.NVIDIA)
	e := platform.Entry{Package: "tts_webui_extension.bark", Name: "Bark", Own: true, Requirements: []string{"bark>=1", "scipy"}}
	if !f.p.Create(context.Background(), e, f.emit) {
		t.Fatalf("create failed:\n%s", strings.Join(*f.out, "\n"))
	}
	calls := f.calls()
	venv := filepath.Join(f.root.Dir(), ".venvs", e.Package)
	py := platform.VenvPython(f.root.Dir(), e.Package)
	cache := " --cache-dir " + filepath.Join(f.root.Dir(), "cache") + " --no-config"
	want := []string{
		"venv " + venv + " --python 3.10.11 --managed-python --no-python-downloads --allow-existing" + cache,
		"pip install --python " + py + cache + " torch==2.11.0+cu128 torchvision torchaudio==2.11.0+cu128 --index-url https://download.pytorch.org/whl/cu128",
		"pip install --python " + py + cache + " torchcodec --index-url=https://download.pytorch.org/whl/cpu",
		"pip install --python " + py + cache + " xformers==0.0.35",
	}
	if len(calls) != 5 {
		t.Fatalf("calls:\n%s", strings.Join(calls, "\n"))
	}
	for i, w := range want {
		if calls[i] != w {
			t.Errorf("call %d:\n got %s\nwant %s", i, calls[i], w)
		}
	}
	if !strings.HasPrefix(calls[4], "pip install --python "+py+cache+" bark>=1 scipy gradio<=5.49.1 ") {
		t.Errorf("requirements call: %s", calls[4])
	}
	if s, ok := f.p.EntryInfo(e.Package); !ok || s.TorchRow != "cu128" || s.Python != "3.10.11" {
		t.Errorf("entry.json = %+v", s)
	}
	if _, err := os.Stat(filepath.Join(venv, "installing.json")); err == nil {
		t.Error("the install marker is still there")
	}
	if st := f.p.EntryStatus(context.Background(), e); !st.Installed || st.Partial {
		t.Errorf("status = %+v", st)
	}
}

func TestAFailedCreateIsPartialAndRunningItAgainCompletes(t *testing.T) {
	f := installed(t, gpu.Custom)
	e := platform.Entry{Package: "tts_webui_extension.x", Own: true, Requirements: []string{"x"}}
	f.p.env = append(f.p.env, "UV_FAKE_FAIL=pip")
	if f.p.Create(context.Background(), e, f.emit) {
		t.Fatal("create succeeded although pip failed")
	}
	if st := f.p.EntryStatus(context.Background(), e); st.Installed || !st.Partial {
		t.Fatalf("after a failure: %+v", st)
	}
	// A package the failed attempt left half linked must not survive into the retry.
	stale := filepath.Join(f.root.Dir(), ".venvs", e.Package, "stale-from-last-attempt")
	os.WriteFile(stale, []byte("x"), 0o644)
	f.p.env = f.p.env[:1]
	if !f.p.Create(context.Background(), e, f.emit) {
		t.Fatalf("second create failed:\n%s", strings.Join(*f.out, "\n"))
	}
	if st := f.p.EntryStatus(context.Background(), e); !st.Installed || st.Partial {
		t.Fatalf("after the second run: %+v", st)
	}
	if _, err := os.Stat(stale); err == nil {
		t.Fatal("the unfinished venv was reused instead of started again")
	}
}

func TestAnOlderVenvWithoutEntryJSONIsInstalled(t *testing.T) {
	f := installed(t, gpu.Custom)
	e := platform.Entry{Package: "tts_webui_extension.old", Own: true}
	dir := filepath.Join(f.root.Dir(), ".venvs", e.Package)
	bin, exe := "bin", "python"
	if runtime.GOOS == "windows" {
		bin, exe = "Scripts", "python.exe"
	}
	os.MkdirAll(filepath.Join(dir, bin), 0o755)
	os.WriteFile(filepath.Join(dir, bin, exe), nil, 0o755)
	if st := f.p.EntryStatus(context.Background(), e); !st.Installed {
		t.Fatalf("%+v", st)
	}
	if st := f.p.EntryStatus(context.Background(), platform.Entry{Package: "nothing", Own: true}); st.Installed || st.Partial {
		t.Fatalf("absent entry: %+v", st)
	}
}

func TestRemoveDeletesTheVenvAndSaysWhatItFreed(t *testing.T) {
	f := installed(t, gpu.Custom)
	e := platform.Entry{Package: "tts_webui_extension.rm", Own: true, Requirements: []string{"x"}}
	if !f.p.Create(context.Background(), e, f.emit) {
		t.Fatal("create")
	}
	*f.out = nil
	if !f.p.Remove(context.Background(), e, f.emit) {
		t.Fatalf("remove failed:\n%s", strings.Join(*f.out, "\n"))
	}
	if _, err := os.Stat(filepath.Join(f.root.Dir(), ".venvs", e.Package)); err == nil {
		t.Fatal("the venv is still there")
	}
	text := strings.Join(*f.out, "\n")
	for _, w := range []string{"Removed ", "Disk freed:", "wheel cache keeps"} {
		if !strings.Contains(text, w) {
			t.Errorf("missing %q in:\n%s", w, text)
		}
	}
}

func TestCommand(t *testing.T) {
	f := installed(t, gpu.Custom)
	c, err := f.p.Command(nil, []string{"uv", "--version"})
	if err != nil || c.Program != f.p.UVPath() || c.Args[0] != "--version" {
		t.Fatalf("%+v %v", c, err)
	}
	if _, err := f.p.Command(nil, []string{"python", "-V"}); err == nil {
		t.Fatal("there is no main python on this platform")
	}
	if _, err := f.p.Command(nil, []string{"npm"}); err == nil {
		t.Fatal("npm is not provided")
	}
	e := platform.Entry{Package: "tts_webui_extension.c", Own: true, Requirements: []string{"x"}}
	f.p.Create(context.Background(), e, func(string) {})
	c, err = f.p.Command(&e, []string{"python", "-m", "pip"})
	if err != nil || c.Program != platform.VenvPython(f.root.Dir(), e.Package) {
		t.Fatalf("own venv python: %+v %v", c, err)
	}
}

func TestUnpinnedPlatformIsAnError(t *testing.T) {
	if _, err := ArchiveFor("plan9", "arm"); err == nil {
		t.Fatal("an unknown OS must not fall back to another archive")
	}
	for k, a := range archives {
		if len(a.SHA256) != 64 || !strings.Contains(a.URL, "/"+Version+"/") {
			t.Errorf("%s: %+v", k, a)
		}
	}
}

// uv prints warnings on the same stream as its answer; the first line is not the path.
func TestPythonExeSkipsUvWarnings(t *testing.T) {
	f := newFixture(t, gpu.CPU)
	if err := f.p.Install(context.Background(), f.emit); err != nil {
		t.Fatal(err)
	}
	got, err := f.p.pythonExe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if st, err := os.Stat(got); err != nil || !filepath.IsAbs(got) || st.IsDir() {
		t.Fatalf("pythonExe = %q, not the path of a file", got)
	}
}
