package uv

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rsxdalv/TTS-WebUI/core/internal/gpu"
	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform"
)

// TestRealInstall installs the pinned uv and Python from the internet into the
// folder named by UV_REAL_ROOT (a scratch folder, never an install) and makes a
// venv with the flags Create uses. It is skipped unless the variable is set.
func TestRealInstall(t *testing.T) {
	dir := os.Getenv("UV_REAL_ROOT")
	if dir == "" {
		t.Skip("UV_REAL_ROOT is not set")
	}
	root, _ := paths.New(dir)
	p := New(Options{Root: root, GPU: gpu.CPU})
	var lines []string
	emit := func(l string) { t.Log(l); lines = append(lines, l) }
	if err := p.Install(context.Background(), emit); err != nil {
		t.Fatal(err)
	}
	if !p.Status(context.Background()).Ready {
		t.Fatal("not ready")
	}
	venv := filepath.Join(dir, ".venvs", "probe")
	cache := []string{"--cache-dir", p.CacheDir(), "--no-config"}
	if err := p.run(context.Background(), emit, append([]string{"venv", venv, "--python", PythonVersion, "--managed-python", "--no-python-downloads", "--allow-existing"}, cache...)...); err != nil {
		t.Fatal(err)
	}
	py := platform.VenvPython(dir, "probe")
	if err := p.run(context.Background(), emit, append([]string{"pip", "install", "--python", py}, append(cache, "six")...)...); err != nil {
		t.Fatal(err)
	}
	c, _ := p.Command(&platform.Entry{Package: "probe", Own: true}, []string{"python", "-c", "import six,sys;print(sys.version)"})
	var out []string
	if err := platform.Run(context.Background(), c, func(l string) { out = append(out, l) }); err != nil || !strings.Contains(strings.Join(out, ""), "3.10.11") {
		t.Fatalf("python: %v %v", out, err)
	}
}

// TestRealTools installs the platform with its pinned tools (git and ffmpeg
// downloads) into UV_REAL_ROOT and checks what an engine would see: git is found
// by uv on a PATH that has none of the machine's (Windows), a git+https
// requirement installs, and the venv's python finds ffmpeg through the command
// the platform builds. With UV_REAL_ENGINE_PYTHON set to the python of an
// installed engine that has torchcodec, that python decodes a wav through the
// ffmpeg shared libraries. Skipped unless UV_REAL_ROOT is set.
func TestRealTools(t *testing.T) {
	dir := os.Getenv("UV_REAL_ROOT")
	if dir == "" {
		t.Skip("UV_REAL_ROOT is not set")
	}
	if runtime.GOOS == "windows" { // a machine with no git of its own
		sys := os.Getenv("SystemRoot")
		t.Setenv("PATH", filepath.Join(sys, "System32")+string(os.PathListSeparator)+sys)
	} else if runtime.GOOS == "linux" { // the same: the basic userland (git's scripts need sed and more), no git
		t.Setenv("PATH", userlandWithoutGit(t))
	}
	root, _ := paths.New(dir)
	p := New(Options{Root: root, GPU: gpu.CPU})
	emit := func(l string) { t.Log(l) }
	if err := p.Install(context.Background(), emit); err != nil {
		t.Fatal(err)
	}
	t.Logf("tools installed: %v", p.ToolsInstalled())
	if _, pinned := ToolFor("git", runtime.GOOS, runtime.GOARCH); pinned && len(p.ToolsInstalled()) != 2 {
		t.Fatalf("want git and ffmpeg, have %v", p.ToolsInstalled())
	}
	venv := filepath.Join(dir, ".venvs", "probe")
	cache := []string{"--cache-dir", p.CacheDir(), "--no-config"}
	if err := p.run(context.Background(), emit, append([]string{"venv", venv, "--python", PythonVersion, "--managed-python", "--no-python-downloads", "--allow-existing"}, cache...)...); err != nil {
		t.Fatal(err)
	}
	py := platform.VenvPython(dir, "probe")
	// git+https: uv runs git itself, so this passes only if git is on its PATH.
	if err := p.run(context.Background(), emit, append([]string{"pip", "install", "--python", py}, append(cache, "git+https://github.com/pypa/sampleproject.git")...)...); err != nil {
		t.Fatalf("git+https install: %v", err)
	}
	e := &platform.Entry{Package: "probe", Own: true}
	run := func(argv ...string) string {
		c, err := p.Command(e, []string{"python"})
		if err != nil {
			t.Fatal(err)
		}
		c.Args = append(c.Args, argv...)
		var out []string
		if err := platform.Run(context.Background(), c, func(l string) { out = append(out, l) }); err != nil {
			t.Fatalf("python %v: %v %v", argv, err, out)
		}
		return strings.Join(out, "\n")
	}
	got := run("-c", "import shutil,subprocess;print(shutil.which('ffmpeg'));print(subprocess.run(['ffmpeg','-version'],capture_output=True,text=True).stdout.splitlines()[0]);print(shutil.which('git'))")
	t.Log(got)
	if !strings.Contains(got, "ffmpeg version") || !strings.Contains(got, filepath.Join("tools", "ffmpeg")) {
		t.Fatalf("the engine's python does not get the platform's ffmpeg:\n%s", got)
	}
	if enginePy := os.Getenv("UV_REAL_ENGINE_PYTHON"); enginePy != "" {
		c, _ := p.Command(e, []string{"python"})
		c.Program = enginePy
		c.Args = []string{"-c", torchcodecProbe}
		var out []string
		if err := platform.Run(context.Background(), c, func(l string) { out = append(out, l) }); err != nil {
			t.Fatalf("torchcodec with the platform's ffmpeg: %v\n%s", err, strings.Join(out, "\n"))
		}
		t.Log(strings.Join(out, "\n"))
		if !strings.Contains(strings.Join(out, "\n"), "decoded") {
			t.Fatalf("no decode:\n%s", strings.Join(out, "\n"))
		}
	}
}

const torchcodecProbe = `
import math, os, struct, tempfile, wave
p = os.path.join(tempfile.gettempdir(), "tc_probe.wav")
w = wave.open(p, "wb"); w.setnchannels(1); w.setsampwidth(2); w.setframerate(16000)
w.writeframes(b"".join(struct.pack("<h", int(3000*math.sin(i/10))) for i in range(16000))); w.close()
from torchcodec.decoders import AudioDecoder
print("decoded", tuple(AudioDecoder(p).get_all_samples().data.shape))
`

// userlandWithoutGit is a folder of links to the machine's basic programs: what a
// minimal Linux has when git is not installed.
func userlandWithoutGit(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, n := range strings.Fields("sh bash sed grep awk cat cp mv rm mkdir rmdir ls ln chmod basename dirname expr tr uname sort uniq wc env readlink id head tail tee date sleep true false test cut find xargs") {
		if p, err := exec.LookPath(n); err == nil {
			_ = os.Symlink(p, filepath.Join(dir, n))
		}
	}
	return dir
}
