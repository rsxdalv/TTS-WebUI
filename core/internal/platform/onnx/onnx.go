// Package onnx is the onnx platform (P8, D23, D28): a pinned sherpa-onnx runtime
// (a native program with onnxruntime inside; no Python, no uv) and, as its
// entries, pinned ONNX model packages. Everything lives in the install folder and
// goes away by deleting it:
//
//	platforms/onnx/runtime/              the unpacked runtime (bin/sherpa-onnx-offline-tts)
//	platforms/onnx/platform.json         written last: no file, not installed
//	platforms/onnx/models/<dir>/         one folder per entry; entry.json written last
//
// One generation is one child process. Cancelling it kills the process and the
// temporary clip goes with it.
package onnx

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

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform"
)

// Name is the platform's name.
const Name = "onnx"

// Options configure the platform.
type Options struct {
	Root paths.Root
	// Env (KEY=VALUE) is added to every command's environment.
	Env []string
	// Runtime and Models replace the pinned archives (tests serve fake ones).
	Runtime *Archive
	Models  []Model
	Client  *http.Client
	// Program replaces the runtime's program (tests point it at a fake).
	Program []string
	// Threads is passed as --num-threads (0 = 2).
	Threads int
	// GOOS and GOARCH default to the running machine.
	GOOS, GOARCH string
}

// Platform is the onnx platform.
type Platform struct {
	root         paths.Root
	env          []string
	runtime      *Archive
	models       []Model
	client       *http.Client
	program      []string
	threads      int
	goos, goarch string
}

var _ platform.Platform = (*Platform)(nil)

// New returns the platform for one install folder.
func New(o Options) *Platform {
	p := &Platform{root: o.Root, env: append([]string(nil), o.Env...), runtime: o.Runtime, models: o.Models,
		client: o.Client, program: o.Program, threads: o.Threads, goos: o.GOOS, goarch: o.GOARCH}
	if p.goos == "" {
		p.goos = runtime.GOOS
	}
	if p.goarch == "" {
		p.goarch = runtime.GOARCH
	}
	if p.threads <= 0 {
		p.threads = 2
	}
	return p
}

// WithEnv returns a copy whose commands also carry env.
func (p *Platform) WithEnv(env []string) platform.Platform {
	q := *p
	q.env = append(append([]string(nil), p.env...), env...)
	return &q
}

func (p *Platform) Name() string       { return Name }
func (p *Platform) Provides() []string { return []string{Name} }

// Dir is platforms/onnx.
func (p *Platform) Dir() string { return filepath.Join(p.root.Dir(), "platforms", Name) }

// RuntimeDir is the unpacked runtime.
func (p *Platform) RuntimeDir() string { return filepath.Join(p.Dir(), "runtime") }

// ModelsDir is where the entries' models are.
func (p *Platform) ModelsDir() string { return filepath.Join(p.Dir(), "models") }

// ModelDir is the folder of one entry's model.
func (p *Platform) ModelDir(m Model) string { return filepath.Join(p.ModelsDir(), m.Dir) }

// ProgramPath is the runtime's TTS program.
func (p *Platform) ProgramPath() string {
	name := "sherpa-onnx-offline-tts"
	if p.goos == "windows" {
		name += ".exe"
	}
	return filepath.Join(p.RuntimeDir(), "bin", name)
}

func (p *Platform) stateFile() string { return filepath.Join(p.Dir(), "platform.json") }

// State is platforms/onnx/platform.json.
type State struct {
	Name        string `json:"name"`
	Runtime     string `json:"runtime"`
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

// Status is Ready when platform.json is there (it is written last) and the
// program is still where it was left.
func (p *Platform) Status(context.Context) platform.Status {
	if _, ok := p.Info(); !ok {
		if _, err := os.Stat(p.Dir()); err == nil {
			return platform.Status{Detail: "the onnx platform install did not finish: install it again"}
		}
		return platform.Status{Detail: "the onnx platform is not installed"}
	}
	if len(p.program) == 0 {
		if _, err := os.Stat(p.ProgramPath()); err != nil {
			return platform.Status{Detail: "the onnx platform is damaged: " + p.ProgramPath() + " is missing (repair it)"}
		}
	}
	return platform.Status{Ready: true}
}

// Supported reports whether this OS and architecture have a pinned runtime to install.
func (p *Platform) Supported() bool {
	_, err := p.archive()
	return err == nil
}

func (p *Platform) archive() (Archive, error) {
	if p.runtime != nil {
		return *p.runtime, nil
	}
	return ArchiveFor(p.goos, p.goarch)
}

func (p *Platform) model(pkg string) (Model, bool) {
	if p.models != nil {
		for _, m := range p.models {
			if m.Package == pkg {
				return m, true
			}
		}
		return Model{}, false
	}
	return ModelFor(pkg)
}

// fetch downloads a checked archive and unpacks it into final, through
// final+".new", so a cancel or a bad archive leaves final as it was.
func (p *Platform) fetch(ctx context.Context, emit platform.Emit, what string, a Archive, final string, probe func(dir string) error) error {
	if err := os.MkdirAll(filepath.Dir(final), 0o755); err != nil {
		return err
	}
	dest := final + ".download"
	emit(fmt.Sprintf("Downloading %s from %s", what, a.URL))
	last := -1
	if err := download(ctx, p.client, a.URL, a.SHA256, dest, func(done, total int64) {
		if pc := int(done * 100 / total); pc != last {
			last = pc
			emit(fmt.Sprintf("%s: %d%% of %d MB", what, pc, total>>20))
		}
	}); err != nil {
		return err
	}
	defer os.Remove(dest)
	emit("Checksum matches " + a.SHA256)
	tmp := final + ".new"
	_ = os.RemoveAll(tmp)
	if err := extractTarBz2(ctx, dest, tmp); err != nil {
		_ = os.RemoveAll(tmp)
		return fmt.Errorf("unpack %s: %w", what, err)
	}
	if err := probe(tmp); err != nil {
		_ = os.RemoveAll(tmp)
		return err
	}
	if err := os.RemoveAll(final); err != nil {
		_ = os.RemoveAll(tmp)
		return err
	}
	return os.Rename(tmp, final)
}

// Install makes the platform. It can be run again after a cancel or a failure.
func (p *Platform) Install(ctx context.Context, emit platform.Emit) error {
	a, err := p.archive()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(p.Dir(), 0o755); err != nil {
		return err
	}
	_ = os.Remove(p.stateFile()) // a repair is "not installed" until it is done
	probe := func(dir string) error {
		if len(p.program) > 0 {
			return nil
		}
		prog := filepath.Join(dir, "bin", filepath.Base(p.ProgramPath()))
		if st, err := os.Stat(prog); err != nil || st.IsDir() {
			return fmt.Errorf("the runtime archive has no bin/%s", filepath.Base(p.ProgramPath()))
		}
		return nil
	}
	if err := p.fetch(ctx, emit, "sherpa-onnx "+RuntimeVersion, a, p.RuntimeDir(), probe); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return err
	}
	st := State{Name: Name, Runtime: "sherpa-onnx " + RuntimeVersion, InstalledAt: time.Now().UTC().Format(time.RFC3339)}
	b, _ := json.MarshalIndent(st, "", "  ")
	if err := os.WriteFile(p.stateFile(), b, 0o644); err != nil {
		return err
	}
	emit("The onnx platform is installed (CPU)")
	return nil
}

// Repair is Install again.
func (p *Platform) Repair(ctx context.Context, emit platform.Emit) error { return p.Install(ctx, emit) }

// Uninstall removes platforms/onnx, models included. The caller refuses while
// an entry needs it.
func (p *Platform) Uninstall(_ context.Context, emit platform.Emit) error {
	if err := os.RemoveAll(p.Dir()); err != nil {
		return err
	}
	emit("The onnx platform is removed")
	return nil
}

func (p *Platform) environ() []string {
	env := os.Environ()
	return append(env, p.env...)
}

// Command resolves the logical program "offline-tts" of an entry: the runtime's
// program with the arguments that name the entry's model files, then argv[1:].
func (p *Platform) Command(e *platform.Entry, argv []string) (platform.Command, error) {
	if len(argv) == 0 {
		return platform.Command{}, errors.New("empty command")
	}
	if argv[0] != "offline-tts" || e == nil {
		return platform.Command{}, fmt.Errorf("the onnx platform does not provide %q", argv[0])
	}
	m, ok := p.model(e.Package)
	if !ok {
		return platform.Command{}, fmt.Errorf("%s is not an onnx entry this version knows", e.Package)
	}
	args, err := p.modelArgs(m)
	if err != nil {
		return platform.Command{}, err
	}
	prog := p.ProgramPath()
	if len(p.program) > 0 {
		prog = p.program[0]
		args = append(append([]string(nil), p.program[1:]...), args...)
	}
	args = append(args, "--num-threads="+fmt.Sprint(p.threads))
	return platform.Command{Program: prog, Args: append(args, argv[1:]...), Env: p.environ(), Dir: p.root.Dir()}, nil
}

func (p *Platform) modelArgs(m Model) ([]string, error) {
	d := p.ModelDir(m)
	switch m.Kind {
	case "kitten":
		return []string{
			"--kitten-model=" + filepath.Join(d, "model.int8.onnx"),
			"--kitten-voices=" + filepath.Join(d, "voices.bin"),
			"--kitten-tokens=" + filepath.Join(d, "tokens.txt"),
			"--kitten-data-dir=" + filepath.Join(d, "espeak-ng-data"),
		}, nil
	}
	return nil, fmt.Errorf("unknown model kind %q", m.Kind)
}

// Check says what Create would do; an onnx entry has nothing to dry-run.
func (p *Platform) Check(_ context.Context, e platform.Entry, emit platform.Emit) bool {
	m, ok := p.model(e.Package)
	if !ok {
		emit("Failed: " + e.Package + " is not an onnx entry this version knows")
		return false
	}
	emit(fmt.Sprintf("Would download %s (%d MB)", m.Title, m.Archive.Size>>20))
	return true
}

func (p *Platform) entryFile(m Model) string { return filepath.Join(p.ModelDir(m), platform.EntryFile) }

// EntryStatus: installed when entry.json is there; a folder without it is partial.
func (p *Platform) EntryStatus(_ context.Context, e platform.Entry) platform.EntryStatus {
	m, ok := p.model(e.Package)
	if !ok {
		return platform.EntryStatus{}
	}
	if _, err := os.Stat(p.entryFile(m)); err == nil {
		return platform.EntryStatus{Installed: true}
	}
	if _, err := os.Stat(p.ModelDir(m)); err == nil {
		return platform.EntryStatus{Partial: true}
	}
	return platform.EntryStatus{}
}

// Create downloads and checks the entry's model, then writes entry.json.
func (p *Platform) Create(ctx context.Context, e platform.Entry, emit platform.Emit) bool {
	m, ok := p.model(e.Package)
	if !ok {
		emit("Failed: " + e.Package + " is not an onnx entry this version knows")
		return false
	}
	if !p.Status(ctx).Ready {
		emit("Failed: the onnx platform is not installed")
		return false
	}
	probe := func(dir string) error {
		for _, f := range m.Files {
			if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
				return fmt.Errorf("the model archive has no %s", f)
			}
		}
		return nil
	}
	if err := p.fetch(ctx, emit, m.Title, m.Archive, p.ModelDir(m), probe); err != nil {
		if ctx.Err() != nil {
			emit("Cancelled: " + m.Title + " was not finished (install again to continue)")
		} else {
			emit("Failed to install " + m.Title + ": " + err.Error())
		}
		return false
	}
	b, _ := json.MarshalIndent(map[string]string{"package": e.Package, "platform": Name,
		"installed_at": time.Now().UTC().Format(time.RFC3339)}, "", "  ")
	if err := os.WriteFile(p.entryFile(m), b, 0o644); err != nil {
		emit("Failed: " + err.Error())
		return false
	}
	emit(m.Title + " is installed")
	return true
}

// Remove deletes the entry's model folder; the platform stays.
func (p *Platform) Remove(_ context.Context, e platform.Entry, emit platform.Emit) bool {
	m, ok := p.model(e.Package)
	if !ok {
		emit("Failed: " + e.Package + " is not an onnx entry this version knows")
		return false
	}
	if err := os.RemoveAll(p.ModelDir(m)); err != nil {
		emit("Failed to remove " + m.Title + ": " + err.Error())
		return false
	}
	_ = os.RemoveAll(p.ModelDir(m) + ".new")
	emit(m.Title + " is removed")
	return true
}

// IsOnnxEntry reports whether pkg is an entry this platform knows.
func (p *Platform) IsOnnxEntry(pkg string) bool { _, ok := p.model(pkg); return ok }

// trimTail keeps the last lines of a program's output for an error message.
func trimTail(lines []string, n int) string {
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
