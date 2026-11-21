package extcatalog

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// serveInstall streams an install or uninstall as NDJSON: {"line": "..."} per
// output line, then {"done": true, "ok": bool}. Problems found before anything
// runs (unknown package, another install running) are ordinary JSON errors.
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
	if !s.install.TryLock() {
		s.fail(w, fmt.Errorf("%w: another install is running", errBusy))
		return
	}
	defer s.install.Unlock()

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

	var ok bool
	if op == "uninstall" {
		ok = s.pipUninstall(r.Context(), pkg, emit)
	} else if entry.Str("proxy") == "native" {
		ok = s.venvSetup(r.Context(), entry, emit)
	} else {
		ok = s.pipInstall(r.Context(), entry, emit)
	}
	send(map[string]any{"done": true, "ok": ok})
}

const gradioPin = "gradio==5.49.1"

// pipInstall is pip_install_wrapper: pip install <requirements> gradio==5.49.1
// in the main env.
func (s *Store) pipInstall(ctx context.Context, entry *Obj, emit func(string)) bool {
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
	var log []string
	rec := func(l string) { log = append(log, l); emit(l) }
	args := append([]string{"python", "-m", "pip", "install"}, append(toks, gradioPin)...)
	ok := s.run(ctx, args, rec) == nil
	if ok {
		rec(fmt.Sprintf("Successfully installed %s dependencies", name))
		rec("Please restart the webui to see the changes")
	} else {
		rec(fmt.Sprintf("Failed to install %s dependencies", name))
	}
	s.writeLog("pip-install", name, log)
	return ok
}

// pipUninstall is pip_uninstall_wrapper.
func (s *Store) pipUninstall(ctx context.Context, pkg string, emit func(string)) bool {
	var log []string
	rec := func(l string) { log = append(log, l); emit(l) }
	ok := s.run(ctx, []string{"python", "-m", "pip", "uninstall", "-y", pkg}, rec) == nil
	if ok {
		rec(fmt.Sprintf("Successfully uninstalled %s (%s)", pkg, pkg))
	} else {
		rec(fmt.Sprintf("Failed to uninstall %s (%s)", pkg, pkg))
	}
	s.writeLog("pip-uninstall", pkg, log)
	return ok
}

var pkgDir = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

const compatibility = `gradio<=5.49.1 gradio-goodtabs>=0.0.5 gradio-goodtab>=0.0.5 gradio-iconbutton>=0.0.1 ffmpeg-python==0.2.0 matplotlib`

// venvSetup is venv_setup_wrapper: .venvs/<package> with uv, torch for the
// chosen GPU, torchcodec, xformers, then the requirements.
func (s *Store) venvSetup(ctx context.Context, entry *Obj, emit func(string)) bool {
	name := entry.Str("name")
	pkg := entry.Str("package_name")
	if name == "" {
		name = pkg
	}
	var log []string
	rec := func(l string) { log = append(log, l); emit(l) }
	rec(fmt.Sprintf("Setting up virtual environment and installing dependencies for %s...", name))
	fail := func(format string, a ...any) bool {
		rec(fmt.Sprintf(format, a...))
		s.writeLog("uv-venv-install", name, log)
		return false
	}
	if !pkgDir.MatchString(pkg) {
		return fail("Failed: %q is not a valid package name for a virtual environment", pkg)
	}
	reqToks, err := shellSplit(entry.Str("requirements"))
	if err != nil || len(reqToks) == 0 {
		return fail("Failed: bad requirements for %s", name)
	}
	torch, err := s.torchArgs()
	if err != nil {
		return fail("Failed: %v", err)
	}
	venv := filepath.Join(s.opts.Root, ".venvs", pkg)
	py := filepath.Join(venv, "bin", "python")
	if runtime.GOOS == "windows" {
		py = filepath.Join(venv, "Scripts", "python.exe")
	}
	uvPip := []string{"uv", "pip", "install", "--python", py}
	steps := [][]string{
		{"uv", "venv", venv, "--allow-existing"},
		append(append([]string{}, uvPip...), torch...),
		append(append([]string{}, uvPip...), "torchcodec", "--index-url=https://download.pytorch.org/whl/cpu"),
		append(append([]string{}, uvPip...), "xformers==0.0.35"),
		append(append(append([]string{}, uvPip...), reqToks...), strings.Fields(compatibility)...),
	}
	for _, args := range steps {
		if err := s.run(ctx, args, rec); err != nil {
			return fail("Failed to run %s: %v", strings.Join(args[:min(3, len(args))], " "), err)
		}
	}
	rec(fmt.Sprintf("Successfully set up virtual environment for %s with dependencies", name))
	s.writeLog("uv-venv-install", name, log)
	return true
}

// run runs args (the first element is the program, resolved in the main env
// through the Run prefix) in the project root and passes each output line,
// stdout and stderr merged, to emit.
func (s *Store) run(ctx context.Context, args []string, emit func(string)) error {
	full := append(append([]string{}, s.opts.Run...), args...)
	if s.opts.Python != "" && args[0] == "python" {
		full = append([]string{s.opts.Python}, args[1:]...)
	}
	cmd := exec.CommandContext(ctx, full[0], full[1:]...)
	cmd.Dir = s.opts.Root
	if s.opts.Env != nil {
		cmd.Env = s.opts.Env
	}
	pr, pw := io.Pipe()
	cmd.Stdout, cmd.Stderr = pw, pw
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { pw.CloseWithError(cmd.Wait()) }()
	sc := bufio.NewScanner(pr)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	sc.Split(splitLines)
	for sc.Scan() {
		if l := strings.TrimSpace(sc.Text()); l != "" {
			emit(l)
		}
	}
	return sc.Err() // the command's exit error, passed through CloseWithError
}

// splitLines splits on \n and \r, like universal_newlines (pip's progress
// bars use \r).
func splitLines(data []byte, atEOF bool) (int, []byte, error) {
	for i, c := range data {
		if c == '\n' || c == '\r' {
			return i + 1, data[:i], nil
		}
	}
	if atEOF && len(data) > 0 {
		return len(data), data, nil
	}
	return 0, nil, nil
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
