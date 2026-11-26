package extcatalog

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/rsxdalv/TTS-WebUI/core/internal/platform"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform/audiocpp"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform/gradiohost"
)

// serveInstall streams an install, an uninstall or a dependency check as
// NDJSON: {"line": "..."} per output line, then {"done": true, "ok": bool}.
// Problems found before anything runs (unknown package, a check of a native
// extension, another install running) are ordinary JSON errors.
func (s *Store) serveInstall(w http.ResponseWriter, r *http.Request, op string) {
	o, err := body(w, r)
	if err == nil {
		err = only(o, "package_name", "gpu", "audiocpp_row")
		if err == nil && op != "install" && o.Str("gpu") != "" {
			err = invalidf("gpu is only for an install")
		}
		if err == nil && op != "install" && o.Str("audiocpp_row") != "" {
			err = invalidf("audiocpp_row is only for an install")
		}
	}
	if err != nil {
		s.fail(w, err)
		return
	}
	pkg := o.Str("package_name")
	if pkg == gradiohost.Name {
		s.serveHost(w, r, op, o)
		return
	}
	if pkg == audiocpp.Name {
		s.serveAudioCpp(w, r, op, o)
		return
	}
	entry, err := s.find(pkg)
	if err != nil {
		s.fail(w, err)
		return
	}
	if op == "check-dependencies" && s.hostActive() && !s.opts.Platform.Status(r.Context()).Ready && entry.Str("proxy") != "native" && !s.isONNX(pkg) {
		s.fail(w, invalidf("%s runs in the Gradio host, which is not installed: install it first", pkg))
		return
	}
	native := entry.Str("proxy") == "native" || s.isONNX(pkg)
	if op == "check-dependencies" && native {
		s.fail(w, invalidf("%s runs in its own venv: there is no dependency check for it", pkg))
		return
	}
	var pl plan
	if op == "install" || op == "uninstall" {
		g, err := s.RequiresGraph(r.Context())
		if err == nil && op == "install" {
			pl, err = s.planInstall(r.Context(), g, pkg)
			if err == nil {
				// Before the GPU question: git missing cannot be answered on the page.
				err = s.checkGitForPlan(pl, entry)
			}
			if err == nil {
				err = s.chooseGPU(&pl, o.Str("gpu"))
			}
			if err == nil {
				err = s.chooseAudioRow(&pl, o.Str("audiocpp_row"))
			}
		} else if err == nil {
			err = refuseIfRequired(g, pkg)
		}
		if err != nil {
			s.failInstall(w, err)
			return
		}
	}
	if !s.install.TryLock() {
		s.fail(w, fmt.Errorf("%w: another install is running", errBusy))
		return
	}
	defer s.install.Unlock()

	if s.opts.Pip != nil && op != "check-dependencies" {
		// The packages change whatever the outcome (a failed install may have
		// done part of the work).
		defer s.opts.Pip.Invalidate()
	}
	s.stream(w, r, func(ctx context.Context, emit func(string)) bool {
		switch {
		case op == "uninstall":
			return s.uninstall(ctx, entry, emit)
		case op == "check-dependencies":
			return s.pipInstall(ctx, entry, true, emit)
		}
		return s.runSteps(ctx, pl, entry, emit)
	})
}

// runSteps installs the platforms of a plan, then its entries in order, the last
// one being entry. A failed step stops the plan.
func (s *Store) runSteps(ctx context.Context, pl plan, entry *Obj, emit func(string)) bool {
	n := 0
	say := func(what string) {
		n++
		if pl.Total > 1 {
			emit(fmt.Sprintf("Installing %s (step %d of %d)", what, n, pl.Total))
		}
	}
	stopped := func(what string) bool {
		if pl.Total > 1 {
			emit(fmt.Sprintf("Stopped: step %d of %d (%s) failed", n, pl.Total, what))
		}
		return false
	}
	for _, name := range pl.Platforms {
		say("the " + name + " platform")
		if name == audiocpp.Name {
			if !s.installAudioCpp(ctx, pl.AudioRow, emit) {
				return stopped("the " + name + " platform")
			}
			continue
		}
		if !s.installPlatform(ctx, name, pl.GPU, emit) {
			return stopped("the " + name + " platform")
		}
	}
	for _, step := range pl.Steps {
		if step == gradiohost.Name {
			say("the " + gradiohost.Title)
			if !s.installHost(ctx, emit) {
				return stopped("the " + gradiohost.Title)
			}
			continue
		}
		e := entry
		if entry == nil || step != entry.Str("package_name") {
			var err error
			if e, err = s.find(step); err != nil {
				emit(fmt.Sprintf("Failed: %v", err))
				return false
			}
		}
		say(step)
		var ok bool
		if s.isONNX(e.Str("package_name")) {
			ok = s.onnxCreate(ctx, e, emit)
		} else if requiresAudio(e) {
			ok = s.audioModelSetup(ctx, e, emit)
		} else if e.Str("proxy") == "native" {
			ok = s.venvSetup(ctx, e, emit)
		} else {
			ok = s.pipInstall(ctx, e, false, emit)
		}
		if !ok {
			return stopped(step)
		}
	}
	return true
}

// noNewerVersion is what the update check says when pip names no new version.
const noNewerVersion = "Already up to date (sometimes incorrect)"

// serveLatestVersion answers {"package_name"} with the version a pip dry run
// would install, as a JSON string. Only a package the lists offer is checked,
// and its name must be a plain package name, because it becomes a pip argument.
func (s *Store) serveLatestVersion(w http.ResponseWriter, r *http.Request) {
	o, err := body(w, r)
	if err == nil {
		err = only(o, "package_name")
	}
	if err != nil {
		s.fail(w, err)
		return
	}
	pkg := o.Str("package_name")
	if _, err := s.find(pkg); err != nil {
		s.fail(w, err)
		return
	}
	if err := checkRequirement(pkg); err != nil {
		s.fail(w, invalidf("%v", err))
		return
	}
	writeJSON(w, http.StatusOK, s.latestVersion(r.Context(), pkg))
}

// latestVersion is get_latest_version: the last word of pip's "Would install"
// line for `pip install --dry-run --no-deps <package> gradio==5.49.1`.
func (s *Store) latestVersion(ctx context.Context, pkg string) string {
	version := ""
	args := []string{"python", "-m", "pip", "install", "--dry-run", "--no-deps", pkg, platform.GradioPin}
	if c, err := s.opts.Platform.Command(nil, args); err == nil {
		c.Dir = s.opts.Root
		_ = platform.Run(ctx, c, func(line string) {
			if f := strings.Fields(line); version == "" && strings.Contains(line, "Would install") && len(f) > 0 {
				version = f[len(f)-1]
			}
		})
	}
	if version == "" {
		return noNewerVersion
	}
	return version
}

// stream writes the NDJSON answer of an install: one {"line": "..."} per
// output line, then {"done": true, "ok": bool}.
func (s *Store) stream(w http.ResponseWriter, r *http.Request, run func(context.Context, func(string)) bool) {
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	fl, _ := w.(http.Flusher)
	send := func(v any) {
		_, _ = w.Write(Marshal(v, ""))
		_, _ = io.WriteString(w, "\n")
		if fl != nil {
			fl.Flush()
		}
	}
	emit := func(line string) { send(map[string]string{"line": line}) }
	send(map[string]any{"done": true, "ok": run(r.Context(), emit)})
}

const gradioPin = "gradio==5.49.1"

// platformEntry is the entry as the platform sees it.
func (s *Store) platformEntry(entry *Obj, toks []string) platform.Entry {
	pkg := entry.Str("package_name")
	name := entry.Str("name")
	if name == "" {
		name = pkg
	}
	return platform.Entry{
		Package: pkg, Name: name, Own: entry.Str("proxy") == "native", Requirements: toks,
		Python: entry.Str("python"), TorchRow: entry.Str("torch"),
	}
}

// pipInstall is pip_install_wrapper: pip install <requirements> gradio==5.49.1
// in the main env. With dryRun it is the dependency check (--dry-run).
func (s *Store) pipInstall(ctx context.Context, entry *Obj, dryRun bool, emit func(string)) bool {
	name := entry.Str("name")
	if name == "" {
		name = entry.Str("package_name")
	}
	req := entry.Str("requirements")
	if req == "" || req == "builtin" {
		emit(fmt.Sprintf("%s has no requirements to install", name))
		return false
	}
	toks, err := shellSplit(req)
	if err != nil {
		emit(fmt.Sprintf("Failed to install %s dependencies: bad requirements: %v", name, err))
		return false
	}
	return s.pipInstallTokens(ctx, entry, toks, dryRun, emit)
}

// pipInstallTokens is pipInstall with the requirements already split. A dry run
// says the same words as an install (the old check did) but leaves the log of
// the real install alone.
func (s *Store) pipInstallTokens(ctx context.Context, entry *Obj, toks []string, dryRun bool, emit func(string)) bool {
	var log []string
	rec := func(l string) { log = append(log, l); emit(l) }
	e := s.platformEntry(entry, toks)
	e.Own = false // the in-process path, whatever the entry says
	var ok bool
	if dryRun {
		ok = s.opts.Platform.Check(ctx, e, rec)
	} else {
		ok = s.opts.Platform.Create(ctx, e, rec)
		s.writeLog("pip-install", e.Name, log)
	}
	return ok
}

// uninstall removes an entry: its venv when it runs in one, otherwise pip
// uninstall in the main env.
func (s *Store) uninstall(ctx context.Context, entry *Obj, emit func(string)) bool {
	if s.isONNX(entry.Str("package_name")) {
		return s.onnxRemove(ctx, entry, emit)
	}
	if requiresAudio(entry) {
		return s.audioModelRemove(ctx, entry, emit)
	}
	if entry.Str("proxy") != "native" {
		return s.pipUninstall(ctx, entry.Str("package_name"), emit)
	}
	var log []string
	rec := func(l string) { log = append(log, l); emit(l) }
	if s.opts.UV == nil {
		rec("Failed: the uv platform is not available")
		return false
	}
	if s.opts.StopEngine != nil && s.opts.StopEngine(entry.Str("package_name")) {
		rec("Stopped the running engine")
	}
	e := s.platformEntry(entry, nil)
	ok := s.opts.UV.Remove(ctx, e, rec)
	s.writeLog("uv-venv-uninstall", e.Name, log)
	return ok
}

// pipUninstall is pip_uninstall_wrapper.
func (s *Store) pipUninstall(ctx context.Context, pkg string, emit func(string)) bool {
	var log []string
	rec := func(l string) { log = append(log, l); emit(l) }
	ok := s.opts.Platform.Remove(ctx, platform.Entry{Package: pkg}, rec)
	s.writeLog("pip-uninstall", pkg, log)
	return ok
}

// venvSetup is venv_setup_wrapper: .venvs/<package> with uv, torch for the
// chosen GPU, torchcodec, xformers, then the requirements.
func (s *Store) venvSetup(ctx context.Context, entry *Obj, emit func(string)) bool {
	reqToks, err := shellSplit(entry.Str("requirements"))
	if err != nil {
		reqToks = nil
	}
	return s.venvSetupTokens(ctx, entry, reqToks, emit)
}

// venvSetupTokens is venvSetup with the requirements already split.
func (s *Store) venvSetupTokens(ctx context.Context, entry *Obj, reqToks []string, emit func(string)) bool {
	var log []string
	rec := func(l string) { log = append(log, l); emit(l) }
	e := s.platformEntry(entry, reqToks)
	e.Own = true
	if s.opts.UV == nil {
		rec("Failed: the uv platform is not available")
		return false
	}
	ok := s.opts.UV.Create(ctx, e, rec)
	s.writeLog("uv-venv-install", e.Name, log)
	return ok
}

var unsafeName = regexp.MustCompile(`[<>:"/\\|?*]`)

// writeLog writes installer_scripts/logs/<type>-<name>.log like write_log.
func (s *Store) writeLog(kind, name string, lines []string) {
	dir := filepath.Join(s.opts.Root, "installer_scripts", "logs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		s.opts.Logf("extcatalog: log dir: %v", err)
		return
	}
	file := filepath.Join(dir, kind+"-"+unsafeName.ReplaceAllString(name, "_")+".log")
	if err := os.WriteFile(file, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		s.opts.Logf("extcatalog: log: %v", err)
	}
}

// shellSplit splits like Python's shlex.split (POSIX): whitespace separates,
// single quotes are literal, double quotes allow \" and \\, a backslash
// escapes the next character.
func shellSplit(s string) ([]string, error) {
	var out []string
	var cur strings.Builder
	in := false
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		c := rs[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			if in {
				out = append(out, cur.String())
				cur.Reset()
				in = false
			}
		case c == '\'':
			in = true
			i++
			for i < len(rs) && rs[i] != '\'' {
				cur.WriteRune(rs[i])
				i++
			}
			if i >= len(rs) {
				return nil, fmt.Errorf("no closing quotation")
			}
		case c == '"':
			in = true
			i++
			for i < len(rs) && rs[i] != '"' {
				if rs[i] == '\\' && i+1 < len(rs) && (rs[i+1] == '"' || rs[i+1] == '\\') {
					i++
				}
				cur.WriteRune(rs[i])
				i++
			}
			if i >= len(rs) {
				return nil, fmt.Errorf("no closing quotation")
			}
		case c == '\\':
			in = true
			if i+1 >= len(rs) {
				return nil, fmt.Errorf("no escaped character")
			}
			i++
			cur.WriteRune(rs[i])
		default:
			in = true
			cur.WriteRune(c)
		}
	}
	if in {
		out = append(out, cur.String())
	}
	return out, nil
}

// checkGitForPlan refuses a plan, before its first step, when one of its own-venv
// entries installs from a git URL and the machine has no git (Linux and macOS do
// not get one from the platform). Without this the uv platform, Python and ffmpeg
// would be downloaded first and the engine would fail afterwards.
func (s *Store) checkGitForPlan(pl plan, entry *Obj) error {
	if s.opts.UV == nil {
		return nil
	}
	for _, step := range pl.Steps {
		e := entry
		if entry == nil || step != entry.Str("package_name") {
			var err error
			if e, err = s.find(step); err != nil {
				return nil // runSteps says so when it gets there
			}
		}
		if e.Str("proxy") != "native" || s.isONNX(step) || requiresAudio(e) {
			continue
		}
		toks, err := shellSplit(e.Str("requirements"))
		if err != nil {
			continue
		}
		name := e.Str("name")
		if name == "" {
			name = step
		}
		if err := s.opts.UV.CheckGit(name, toks); err != nil {
			return err
		}
	}
	return nil
}
