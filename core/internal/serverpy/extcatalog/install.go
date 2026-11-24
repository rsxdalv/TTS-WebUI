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
)

// serveInstall streams an install, an uninstall or a dependency check as
// NDJSON: {"line": "..."} per output line, then {"done": true, "ok": bool}.
// Problems found before anything runs (unknown package, a check of a native
// extension, another install running) are ordinary JSON errors.
func (s *Store) serveInstall(w http.ResponseWriter, r *http.Request, op string) {
	o, err := body(w, r)
	if err == nil {
		err = only(o, "package_name")
	}
	if err != nil {
		s.fail(w, err)
		return
	}
	pkg := o.Str("package_name")
	entry, err := s.find(pkg)
	if err != nil {
		s.fail(w, err)
		return
	}
	native := entry.Str("proxy") == "native"
	if op == "check-dependencies" && native {
		s.fail(w, invalidf("%s runs in its own venv: there is no dependency check for it", pkg))
		return
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
			return s.pipUninstall(ctx, pkg, emit)
		case op == "check-dependencies":
			return s.pipInstall(ctx, entry, true, emit)
		case native:
			return s.venvSetup(ctx, entry, emit)
		}
		return s.pipInstall(ctx, entry, false, emit)
	})
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
		TorchArgs: s.torchArgs,
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
	ok := s.opts.Platform.Create(ctx, e, rec)
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
