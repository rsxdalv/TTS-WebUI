package uv

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/rsxdalv/TTS-WebUI/core/internal/gpu"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform"
	"github.com/rsxdalv/TTS-WebUI/core/internal/torchrows"
)

// compatibility is installed with every venv so the Gradio pages an engine
// serves keep working (it is what the old native install added after the
// requirements). scipy is what the engine harness itself imports (the
// decorators write wav files with it): P6 found Kokoro died on start without it.
// pip is there because engines shell out to `python -m pip` (Kokoro's G2P
// downloads its spaCy model that way); a uv venv has no pip of its own.
const compatibility = `gradio<=5.49.1 gradio-goodtabs>=0.0.5 gradio-goodtab>=0.0.5 gradio-iconbutton>=0.0.1 ffmpeg-python==0.2.0 matplotlib scipy pip`

var pkgDir = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// EntryState is .venvs/<package>/entry.json, written when the install is done.
type EntryState struct {
	Package     string `json:"package"`
	Platform    string `json:"platform"`
	Python      string `json:"python"`
	TorchRow    string `json:"torch_row"`
	InstalledAt string `json:"installed_at"`
}

func (p *Platform) venv(pkg string) string { return platform.VenvDir(p.root.Dir(), pkg) }

// EntryInfo reads entry.json of an installed entry.
func (p *Platform) EntryInfo(pkg string) (EntryState, bool) {
	b, err := os.ReadFile(filepath.Join(p.venv(pkg), platform.EntryFile))
	if err != nil {
		return EntryState{}, false
	}
	var s EntryState
	if json.Unmarshal(b, &s) != nil || s.Package != pkg {
		return EntryState{}, false
	}
	return s, true
}

// EntryStatus is platform.OwnVenvStatus: entry.json is written last, installing.json
// marks an unfinished install.
func (p *Platform) EntryStatus(_ context.Context, e platform.Entry) platform.EntryStatus {
	return platform.OwnVenvStatus(p.root.Dir(), e.Package)
}

// Check has nothing to check: an own-venv entry has no dependency check.
func (p *Platform) Check(_ context.Context, e platform.Entry, emit platform.Emit) bool {
	emit(e.Label() + " runs in its own venv: there is no dependency check for it")
	return false
}

// Create makes .venvs/<package>: the venv from the platform's Python, torch of
// the row, torchcodec, xformers when the row has it, then the requirements.
// Cancelling stops uv; running it again reuses the cache and finishes.
func (p *Platform) Create(ctx context.Context, e platform.Entry, emit platform.Emit) bool {
	emit = lockedEmit(emit) // the prefetch below talks from its own goroutine
	emit(fmt.Sprintf("Setting up virtual environment and installing dependencies for %s...", e.Label()))
	fail := func(format string, a ...any) bool {
		emit(fmt.Sprintf(format, a...))
		return false
	}
	if !pkgDir.MatchString(e.Package) {
		return fail("Failed: %q is not a valid package name for a virtual environment", e.Package)
	}
	if len(e.Requirements) == 0 {
		return fail("Failed: bad requirements for %s", e.Label())
	}
	st, ok := p.Info()
	if !ok {
		return fail("Failed: the uv platform is not installed")
	}
	row, err := p.rowFor(e, p.GPU())
	if err != nil {
		return fail("Failed: %v", err)
	}
	py := e.Python
	if py == "" {
		py = st.Python
	}
	if err := checkVersion(py); err != nil {
		return fail("Failed: %v", err)
	}
	// Before the tools are fetched: a machine with no git says so now, not after
	// ffmpeg (70 MB) has been downloaded.
	if err := p.CheckGit(e.Label(), e.Requirements); err != nil {
		return fail("Failed: %v", err)
	}
	if err := p.installTools(ctx, emit); err != nil {
		return fail("Failed: %v", err)
	}
	venv := p.venv(e.Package)
	if _, err := os.Stat(filepath.Join(venv, platform.InstallingFile)); err == nil {
		// The last attempt did not finish (cancelled or failed). It may have left
		// a package half linked, which uv will not install over (found in the
		// real cancel run), so the venv starts again; the downloads are in the
		// cache and are not fetched twice.
		emit("Removing the venv the last attempt left unfinished (downloads are kept)")
		if err := removeAll(venv); err != nil {
			return fail("Failed: cannot remove the unfinished %s: %v", venv, err)
		}
	}
	if err := checkPathLength(p.goos, venv); err != nil && !p.skipPathCheck {
		return fail("Failed: %v", err)
	}
	free, known := freeBytes(p.root.Dir())
	if err := checkDisk(free, known, row, p.rowInUse(row)); err != nil {
		return fail("Failed: %v", err)
	}
	if err := os.MkdirAll(venv, 0o755); err != nil {
		return fail("Failed: %v", err)
	}
	_ = os.Remove(filepath.Join(venv, platform.EntryFile))
	if err := os.WriteFile(filepath.Join(venv, platform.InstallingFile), []byte(`{"package":"`+e.Package+`"}`), 0o644); err != nil {
		return fail("Failed: %v", err)
	}
	emit(fmt.Sprintf("Python %s, torch row %s (about %d MB, shared with every other venv on this row)", py, row.Name, row.ApproxMB))
	pyExe := platform.VenvPython(p.root.Dir(), e.Package)
	cache := []string{"--cache-dir", p.CacheDir(), "--no-config"}
	pip := append([]string{"pip", "install", "--python", pyExe}, cache...)
	// Each uv call belongs to one reported step: the environment, torch (torch,
	// torchcodec, xformers) or the requirements.
	type uvStep struct {
		group string
		args  []string
	}
	var steps []uvStep
	if py != st.Python {
		steps = append(steps, uvStep{"venv", []string{"python", "install", py, "--no-bin", "--no-registry"}})
	}
	steps = append(steps,
		uvStep{"venv", append([]string{"venv", venv, "--python", py, "--managed-python", "--no-python-downloads", "--allow-existing"}, cache...)},
		uvStep{"torch", append(append([]string{}, pip...), row.InstallArgs()...)},
		uvStep{"torch", append(append([]string{}, pip...), "torchcodec", "--index-url="+torchrows.TorchcodecIndex)},
	)
	if row.Xformers != "" {
		steps = append(steps, uvStep{"torch", append(append([]string{}, pip...), row.Xformers)})
	}
	steps = append(steps, uvStep{"requirements", append(append(append([]string{}, pip...), e.Requirements...), strings.Fields(compatibility)...)})

	id := func(group string) string { return e.Package + ":" + group }
	// The plan is the caller's; a caller that made none is shown the steps as
	// they come (the page adds an unknown id).
	platform.Step(ctx, platform.StepEvent{ID: id("torch"), Label: fmt.Sprintf("Install torch %s (%s)", torchrows.TorchVersion, row.Name)})
	// While torch downloads, the requirements' wheels are fetched into the cache
	// (prefetch.go); the requirements step waits for that to end.
	wheels := id("wheels")
	var prefetched chan struct{}
	stopPrefetch := func() {}
	endPrefetch := func() {
		if prefetched != nil {
			<-prefetched
			prefetched = nil
		}
	}
	defer func() { stopPrefetch(); endPrefetch() }()
	cur := ""
	closeGroup := func(state, detail string) {
		if cur != "" {
			report(ctx, id(cur), state, detail)
		}
	}
	for _, st := range steps {
		if st.group != cur {
			closeGroup(platform.StepDone, "")
			cur = st.group
			detail := map[string]string{
				"venv":         "",
				"torch":        "downloading and unpacking: uv shows no progress, this is measured from the cache folder",
				"requirements": "resolving",
			}[cur]
			report(ctx, id(cur), platform.StepRunning, detail)
			switch cur {
			case "torch":
				if !p.prefetchEnabled() {
					report(ctx, wheels, platform.StepSkipped, "off")
					break
				}
				pctx, cancel := context.WithCancel(ctx)
				stopPrefetch = cancel
				prefetched = make(chan struct{})
				toks := append(append([]string{}, e.Requirements...), strings.Fields(compatibility)...)
				go func(done chan struct{}) {
					defer close(done)
					p.prefetchWheels(pctx, wheels, e.Package, toks, pyExe, emit)
				}(prefetched)
			case "requirements":
				endPrefetch() // the wheels are in the cache before they are asked for
			}
		}
		// The requirements are many small packages, so uv's own lines give their
		// progress; torch is one big wheel, measured from the cache folder below.
		stepEmit := emit
		if st.group == "requirements" {
			obs := &uvLines{ctx: ctx, id: id(cur)}
			stepEmit = func(l string) {
				emit(l)
				obs.observe(l)
			}
		}
		stop := func() {}
		if st.group == "torch" && len(st.args) > 0 && st.args[0] == "pip" && containsArg(st.args, row.Packages[0]) {
			stop = watchCache(ctx, id("torch"), p.CacheDir(), torchUnpackedBytes(row.ApproxMB), 2*time.Second)
		}
		err := p.run(ctx, stepEmit, st.args...)
		stop()
		if err != nil {
			if ctx.Err() != nil {
				closeGroup(platform.StepFailed, "cancelled: install again to continue; downloads are kept")
				return fail("Cancelled: %s was not finished (install it again to continue; downloads are kept)", e.Label())
			}
			closeGroup(platform.StepFailed, err.Error())
			return fail("Failed to run uv %s: %v", strings.Join(st.args[:min(2, len(st.args))], " "), err)
		}
	}
	closeGroup(platform.StepDone, "")
	state := EntryState{Package: e.Package, Platform: Name, Python: py, TorchRow: row.Name,
		InstalledAt: time.Now().UTC().Format(time.RFC3339)}
	b, _ := json.MarshalIndent(state, "", "  ")
	if err := os.WriteFile(filepath.Join(venv, platform.EntryFile), b, 0o644); err != nil {
		return fail("Failed: %v", err)
	}
	_ = os.Remove(filepath.Join(venv, platform.InstallingFile))
	emit(fmt.Sprintf("Successfully set up virtual environment for %s with dependencies", e.Label()))
	return true
}

func containsArg(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}

func (p *Platform) rowFor(e platform.Entry, g gpu.Label) (torchrows.Row, error) {
	if e.TorchRow != "" {
		return torchrows.ByName(e.TorchRow, p.goos)
	}
	if g == "" {
		return torchrows.Row{}, fmt.Errorf("the uv platform has no GPU choice")
	}
	return torchrows.Default(g, p.goos)
}

var versionRe = regexp.MustCompile(`^3\.\d{1,2}(\.\d{1,2})?$`)

func checkVersion(v string) error {
	if !versionRe.MatchString(v) {
		return fmt.Errorf("%q is not a Python version (use 3.10.11 or 3.11)", v)
	}
	return nil
}

// Remove deletes the entry's venv, which is what removing an own-venv entry
// means, and says what that freed: the venv's size, and the disk the volume
// really got back. Most of a venv's files are hardlinks into the wheel cache, so
// the second number is usually small until the cache is cleared; the cache's
// size is reported next to it.
func (p *Platform) Remove(_ context.Context, e platform.Entry, emit platform.Emit) bool {
	if !pkgDir.MatchString(e.Package) {
		emit(fmt.Sprintf("Failed to remove %s: not a valid package name", e.Package))
		return false
	}
	dir := p.venv(e.Package)
	if _, err := os.Stat(dir); err != nil {
		emit(fmt.Sprintf("%s has no virtual environment: nothing to remove", e.Label()))
		return true
	}
	files, bytes := treeSize(dir)
	before, haveFree := freeBytes(p.root.Dir())
	if err := removeAll(dir); err != nil {
		emit(fmt.Sprintf("Failed to remove %s: %v", dir, err))
		return false
	}
	after, _ := freeBytes(p.root.Dir())
	emit(fmt.Sprintf("Removed %s: %d files, %s as the venv sees them", dir, files, human(bytes)))
	_, cache := treeSize(p.CacheDir())
	if haveFree {
		freed := int64(after) - int64(before)
		if freed < 0 {
			freed = 0
		}
		emit(fmt.Sprintf("Disk freed: %s (free space of the volume before and after)", human(freed)))
	}
	emit(fmt.Sprintf("The wheel cache keeps %s that other engines share; the venv's files were hardlinks into it", human(cache)))
	emit(fmt.Sprintf("Successfully uninstalled %s", e.Label()))
	return true
}

func human(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	}
	return fmt.Sprintf("%d KB", n>>10)
}

// treeSize counts the files under dir and their size.
func treeSize(dir string) (files int, bytes int64) {
	_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if info, err := d.Info(); err == nil {
			files++
			bytes += info.Size()
		}
		return nil
	})
	return
}

// removeAll is os.RemoveAll that also removes read-only files and retries a
// few times: on Windows a file another process just closed can still be locked.
func removeAll(dir string) error {
	var err error
	for i := 0; i < 5; i++ {
		if err = os.RemoveAll(dir); err == nil {
			return nil
		}
		_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, werr error) error {
			if werr == nil {
				_ = os.Chmod(path, 0o777)
			}
			return nil
		})
		time.Sleep(time.Duration(i+1) * 200 * time.Millisecond)
	}
	return err
}
