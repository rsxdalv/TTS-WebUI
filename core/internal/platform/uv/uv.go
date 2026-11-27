// Package uv is the uv platform (P2, D23, D25, D28, D29): a pinned uv, the
// pinned Python it downloads, one wheel cache, and one venv per own-venv entry.
// Everything lives in the install folder and goes away by deleting it:
//
//	platforms/uv/bin/uv       the pinned uv (checksum checked)
//	platforms/uv/python/      Pythons uv downloaded, one directory per version
//	platforms/uv/platform.json written last: no file, not installed
//	cache/                    the wheel cache, on the volume of .venvs
//	.venvs/<package>/         one venv per entry (entry.json written last)
//
// Nothing from another Python env is involved. It runs uv directly,
// with no shell, so there is nothing to escape.
package uv

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/rsxdalv/TTS-WebUI/core/internal/gpu"
	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform"
)

// Name is the platform's name.
const Name = "uv"

// Options configure the platform.
type Options struct {
	Root paths.Root
	// GPU is the choice made for this install (D29). Install refuses without it;
	// once installed it is read from platform.json.
	GPU gpu.Label
	// Env (KEY=VALUE) is added to every command's environment, after the
	// inherited one and the platform's own variables.
	Env []string
	// Archive replaces the pinned uv archive (tests serve a fake one).
	Archive *Archive
	Client  *http.Client
	// Prefix replaces the pinned uv program (and its arguments) with a stand-in:
	// tests point it at a fake. The platform then downloads nothing.
	Prefix []string
	// GOOS and GOARCH default to the running machine.
	GOOS, GOARCH string
	// SkipPathCheck turns off the Windows path-length check of a new venv (tests
	// install under long temporary folder names).
	SkipPathCheck bool
	// Tools replaces the pinned tools (git, ffmpeg); tests serve fake archives. Nil
	// means the pinned ones, or none while Prefix is set.
	Tools []Tool
}

// Platform is the uv platform.
type Platform struct {
	root          paths.Root
	gpu           gpu.Label
	env           []string
	archive       *Archive
	prefix        []string
	client        *http.Client
	goos, goarch  string
	skipPathCheck bool
	tools         []Tool
}

var _ platform.Platform = (*Platform)(nil)

// New returns the platform for one install folder.
func New(o Options) *Platform {
	p := &Platform{root: o.Root, gpu: o.GPU, env: append([]string(nil), o.Env...), archive: o.Archive, prefix: o.Prefix,
		client: o.Client, goos: o.GOOS, goarch: o.GOARCH, skipPathCheck: o.SkipPathCheck, tools: o.Tools}
	if p.goos == "" {
		p.goos = runtime.GOOS
	}
	if p.goarch == "" {
		p.goarch = runtime.GOARCH
	}
	return p
}

// WithGPU returns a copy that installs with this GPU choice.
func (p *Platform) WithGPU(l gpu.Label) *Platform { q := *p; q.gpu = l; return &q }

// WithEnv returns a copy whose commands also carry env.
func (p *Platform) WithEnv(env []string) platform.Platform {
	q := *p
	q.env = append(append([]string(nil), p.env...), env...)
	return &q
}

func (p *Platform) Name() string       { return Name }
func (p *Platform) Provides() []string { return []string{"uv"} }

// Dir is platforms/uv.
func (p *Platform) Dir() string { return filepath.Join(p.root.Dir(), "platforms", "uv") }

// CacheDir is the wheel cache. It is in the install folder, next to .venvs, so
// both are on one volume (D28).
func (p *Platform) CacheDir() string { return filepath.Join(p.root.Dir(), "cache") }

// PythonDir is where uv puts the Pythons it downloads.
func (p *Platform) PythonDir() string { return filepath.Join(p.Dir(), "python") }

// UVPath is the pinned uv program.
func (p *Platform) UVPath() string {
	name := "uv"
	if p.goos == "windows" {
		name = "uv.exe"
	}
	return filepath.Join(p.Dir(), "bin", name)
}

func (p *Platform) stateFile() string { return filepath.Join(p.Dir(), "platform.json") }

// State is platforms/uv/platform.json.
type State struct {
	Name        string `json:"name"`
	UVVersion   string `json:"uv_version"`
	Python      string `json:"python"`
	GPU         string `json:"gpu"`
	InstalledAt string `json:"installed_at"`
}

// Info reads platform.json; ok is false when the platform is not installed.
func (p *Platform) Info() (State, bool) {
	b, err := os.ReadFile(p.stateFile())
	if err != nil {
		return State{}, false
	}
	var s State
	if json.Unmarshal(b, &s) != nil || s.Name != Name {
		return State{}, false
	}
	return s, true
}

// Status is Ready when platform.json is there (it is written last) and the uv
// and the Python it names are still where it left them.
func (p *Platform) Status(context.Context) platform.Status {
	s, ok := p.Info()
	if !ok {
		if _, err := os.Stat(p.Dir()); err == nil {
			return platform.Status{Detail: "the uv platform install did not finish: install it again"}
		}
		return platform.Status{Detail: "the uv platform is not installed"}
	}
	if _, err := os.Stat(p.UVPath()); err != nil && len(p.prefix) == 0 {
		return platform.Status{Detail: "the uv platform is damaged: " + p.UVPath() + " is missing (repair it)"}
	}
	if _, err := os.Stat(p.PythonDir()); err != nil {
		return platform.Status{Detail: "the uv platform is damaged: Python " + s.Python + " is missing (repair it)"}
	}
	return platform.Status{Ready: true}
}

// GPU is the GPU choice of the installed platform, or the one this value was
// built with while it is not installed yet.
func (p *Platform) GPU() gpu.Label {
	if s, ok := p.Info(); ok {
		if l, err := gpu.Parse(s.GPU); err == nil {
			return l
		}
	}
	return p.gpu
}

// Install makes the platform: the pinned uv, the Python, the cache folder,
// platform.json. It can be run again after a cancel or a failure; steps that are
// done are skipped. The GPU choice must be set first.
func (p *Platform) Install(ctx context.Context, emit platform.Emit) error {
	if p.gpu == "" {
		return errors.New("the GPU choice is required to install the uv platform")
	}
	if _, err := gpu.Parse(string(p.gpu)); err != nil {
		return err
	}
	if _, err := ArchiveFor(p.goos, p.goarch); err != nil && p.archive == nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(p.Dir(), "bin"), 0o755); err != nil {
		return err
	}
	_ = os.Remove(p.stateFile()) // a repair is "not installed" until it is done
	if err := p.fetchUV(ctx, emit); err != nil {
		return err
	}
	emit(fmt.Sprintf("Installing Python %s into %s (uv downloads it)", PythonVersion, p.PythonDir()))
	if err := os.MkdirAll(p.PythonDir(), 0o755); err != nil {
		return err
	}
	report(ctx, StepPython, platform.StepRunning, "uv downloads it (about 35 MB)")
	if err := p.run(ctx, emit, "python", "install", PythonVersion, "--no-bin", "--no-registry"); err != nil {
		report(ctx, StepPython, platform.StepFailed, err.Error())
		return fmt.Errorf("uv python install %s: %w", PythonVersion, err)
	}
	report(ctx, StepPython, platform.StepDone, "")
	if err := os.MkdirAll(p.CacheDir(), 0o755); err != nil {
		return err
	}
	p.checkSameVolume(emit)
	if err := p.installTools(ctx, emit); err != nil {
		return err
	}
	st := State{Name: Name, UVVersion: Version, Python: PythonVersion, GPU: string(p.gpu),
		InstalledAt: time.Now().UTC().Format(time.RFC3339)}
	b, _ := json.MarshalIndent(st, "", "  ")
	if err := os.WriteFile(p.stateFile(), b, 0o644); err != nil {
		return err
	}
	emit("The uv platform is installed")
	return nil
}

// Repair is Install again.
func (p *Platform) Repair(ctx context.Context, emit platform.Emit) error {
	if g := p.GPU(); g != "" {
		p = p.WithGPU(g)
	}
	return p.Install(ctx, emit)
}

// Uninstall removes platforms/uv. The caller refuses while an entry's venv
// needs it: a venv breaks when its base Python goes.
func (p *Platform) Uninstall(_ context.Context, emit platform.Emit) error {
	if err := os.RemoveAll(p.Dir()); err != nil {
		return err
	}
	emit("The uv platform is removed (the wheel cache in " + p.CacheDir() + " is kept)")
	return nil
}

// fetchUV downloads and checks the pinned uv unless the one in bin/ already
// answers with the pinned version.
func (p *Platform) fetchUV(ctx context.Context, emit platform.Emit) error {
	if len(p.prefix) > 0 {
		report(ctx, StepUV, platform.StepSkipped, "")
		return nil
	}
	if _, err := os.Stat(p.UVPath()); err == nil && p.uvVersionOK(ctx) {
		emit("uv " + Version + " is already there")
		report(ctx, StepUV, platform.StepSkipped, "")
		return nil
	}
	a := p.archive
	if a == nil {
		pinned, err := ArchiveFor(p.goos, p.goarch)
		if err != nil {
			return err
		}
		a = &pinned
	}
	ext := ".tar.gz"
	if strings.HasSuffix(a.URL, ".zip") {
		ext = ".zip"
	}
	dest := filepath.Join(p.Dir(), "bin", "uv-archive"+ext)
	emit("Downloading uv " + Version + " from " + a.URL)
	report(ctx, StepUV, platform.StepRunning, "downloading")
	last := -1
	progress := downloadProgress(ctx, StepUV)
	err := download(ctx, p.client, a.URL, a.SHA256, dest, func(done, total int64) {
		progress(done, total)
		if pc := int(done * 100 / total); pc != last {
			last = pc
			emit(fmt.Sprintf("uv: %d%% of %d MB", pc, total>>20))
		}
	})
	if err != nil {
		report(ctx, StepUV, platform.StepFailed, err.Error())
		return err
	}
	defer os.Remove(dest)
	emit("Checksum matches " + a.SHA256)
	if err := extractMember(dest, filepath.Base(p.UVPath()), p.UVPath()); err != nil {
		report(ctx, StepUV, platform.StepFailed, err.Error())
		return err
	}
	report(ctx, StepUV, platform.StepDone, "checksum matches")
	return nil
}

// uvVersionOK reports whether bin/uv runs and says the pinned version.
func (p *Platform) uvVersionOK(ctx context.Context) bool {
	var out string
	_ = p.runCapture(ctx, func(l string) {
		if out == "" {
			out = l
		}
	}, "--version")
	return strings.Contains(out, Version)
}

// run runs uv with args, streaming its lines.
func (p *Platform) run(ctx context.Context, emit platform.Emit, args ...string) error {
	return p.runCapture(ctx, emit, args...)
}

func (p *Platform) runCapture(ctx context.Context, emit platform.Emit, args ...string) error {
	c, err := p.Command(nil, append([]string{"uv"}, args...))
	if err != nil {
		return err
	}
	return platform.Run(ctx, c, emit)
}

// checkSameVolume says so when the cache and the venvs are not on one volume:
// uv then copies every file instead of linking it, so each venv holds its own
// copy of torch (PLAN, P2). It tries a hardlink, which is what uv would do.
func (p *Platform) checkSameVolume(emit platform.Emit) {
	venvs := filepath.Join(p.root.Dir(), ".venvs")
	if err := os.MkdirAll(venvs, 0o755); err != nil {
		emit("Warning: cannot create " + venvs + ": " + err.Error())
		return
	}
	src := filepath.Join(p.CacheDir(), ".link-test")
	dst := filepath.Join(venvs, ".link-test")
	_ = os.Remove(src)
	_ = os.Remove(dst)
	if err := os.WriteFile(src, []byte("x"), 0o644); err != nil {
		emit("Warning: cannot write in " + p.CacheDir() + ": " + err.Error())
		return
	}
	defer os.Remove(src)
	if err := os.Link(src, dst); err != nil {
		emit("Warning: the wheel cache (" + p.CacheDir() + ") and the venvs (" + venvs +
			") cannot hold the same file (" + err.Error() + "): every venv will keep its own copy of torch")
		return
	}
	_ = os.Remove(dst)
	emit("The wheel cache and the venvs are on one volume: files are shared by hardlink")
}

// environ is the environment of every uv command: the inherited one, then the
// platform's folders (so uv never reads ./pyproject.toml or ~/.cache), then the
// options' variables.
func (p *Platform) environ() []string { return p.EnvWith(nil) }

// EnvWith is environ with more variables layered on top (a caller's own environment, as the
// Gradio host entry has). The last value of a variable wins, and the tools' folders stay in
// front of the one PATH that results.
func (p *Platform) EnvWith(extra []string) []string {
	env := append(os.Environ(),
		"UV_NO_CONFIG=1",
		"UV_CACHE_DIR="+p.CacheDir(),
		"UV_PYTHON_INSTALL_DIR="+p.PythonDir(),
		"UV_PYTHON_DOWNLOADS=manual",
	)
	if p.goos != "darwin" { // APFS clones by default; elsewhere say hardlink out loud
		env = append(env, "UV_LINK_MODE=hardlink")
	}
	pathDirs, toolVars := p.toolEnv()
	env = append(env, toolVars...)
	env = dedupeEnv(p.goos, append(append(env, p.env...), extra...))
	return withToolsFirst(p.goos, env, pathDirs)
}

// withToolsFirst puts the tools' folders in front of the one PATH the command has.
// Whatever name the inherited variable has ("Path" on Windows), it is replaced, not
// joined by a second PATH: a child that sees two takes one of them, and a real Kokoro
// run could not start ffmpeg because it took the one without the tools.
func withToolsFirst(goos string, env, dirs []string) []string {
	if len(dirs) == 0 {
		return env
	}
	out := make([]string, 0, len(env)+1)
	old := ""
	for _, kv := range env {
		k, v, _ := strings.Cut(kv, "=")
		if k == "PATH" || (goos == "windows" && strings.EqualFold(k, "PATH")) {
			old = v
			continue
		}
		out = append(out, kv)
	}
	return append(out, "PATH="+strings.Join(append(append([]string(nil), dirs...), old), string(os.PathListSeparator)))
}

// dedupeEnv keeps one entry per variable, the last one. On Windows names compare
// without case: the inherited environment has "Path" and the tools add "PATH", and a
// child that sees both may take the first, so the tools' PATH (ffmpeg, git) would
// not be found (found by a real Kokoro run: its Save Ogg decorator could not start ffmpeg).
func dedupeEnv(goos string, env []string) []string {
	key := func(kv string) string {
		k, _, _ := strings.Cut(kv, "=")
		if goos == "windows" {
			k = strings.ToUpper(k)
		}
		return k
	}
	last := map[string]int{}
	for i, kv := range env {
		last[key(kv)] = i
	}
	out := make([]string, 0, len(env))
	for i, kv := range env {
		if last[key(kv)] == i {
			out = append(out, kv)
		}
	}
	return out
}

// Command resolves a logical program: "uv" is the pinned uv; "python" of an
// own-venv entry is the venv's python, run directly. There is no main-env
// python on this platform.
func (p *Platform) Command(e *platform.Entry, argv []string) (platform.Command, error) {
	if len(argv) == 0 {
		return platform.Command{}, errors.New("empty command")
	}
	env := p.environ()
	if c, ok := platform.OwnVenvCommand(p.root.Dir(), e, argv, env); ok {
		return c, nil
	}
	switch argv[0] {
	case "uv":
		if len(p.prefix) > 0 {
			return platform.Command{Program: p.prefix[0], Args: append(append([]string(nil), p.prefix[1:]...), argv[1:]...), Env: env, Dir: p.root.Dir()}, nil
		}
		return platform.Command{Program: p.UVPath(), Args: append([]string(nil), argv[1:]...), Env: env, Dir: p.root.Dir()}, nil
	case "python":
		return platform.Command{}, errors.New("the uv platform has no main Python: only an installed entry has one")
	}
	return platform.Command{}, fmt.Errorf("the uv platform does not provide %q", argv[0])
}

// OS is the operating system the platform installs for.
func (p *Platform) OS() string { return p.goos }
