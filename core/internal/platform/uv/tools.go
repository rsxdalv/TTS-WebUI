package uv

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/rsxdalv/TTS-WebUI/core/internal/platform"
)

// The tools provider (D26): git and ffmpeg for the engines, fetched into
// platforms/uv/tools/<name>/ and put first on the PATH of every command this
// platform builds (uv itself, so a git+https requirement installs, and an
// own-venv engine, so it finds ffmpeg or loads its shared libraries).
//
//	platforms/uv/tools/<name>/            the extracted archive
//	platforms/uv/tools/<name>/tool.json   written last: no file, not installed

// ToolsDir is platforms/uv/tools.
func (p *Platform) ToolsDir() string { return filepath.Join(p.Dir(), "tools") }

func (p *Platform) toolDir(t Tool) string { return filepath.Join(p.ToolsDir(), t.Name) }

// toolState is tool.json.
type toolState struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	SHA256      string `json:"sha256"`
	InstalledAt string `json:"installed_at"`
}

// pinnedTools are the tools this platform fetches on this OS: the ones set in
// Options.Tools (tests), else the pinned ones, else none when the uv program is a
// stand-in (nothing is downloaded then).
func (p *Platform) pinnedTools() []Tool {
	if p.tools != nil {
		return p.tools
	}
	if !p.fetchesPinned() {
		return nil
	}
	var out []Tool
	for _, n := range ToolNames {
		if t, ok := ToolFor(n, p.goos, p.goarch); ok {
			out = append(out, t)
		}
	}
	return out
}

// fetchesPinned is false when a test stands in for uv or serves its own archive: the
// pinned tools (90 MB) are then not downloaded.
func (p *Platform) fetchesPinned() bool { return len(p.prefix) == 0 && p.archive == nil }

// toolThere reports whether the tool was extracted completely and is the pinned one.
func (p *Platform) toolThere(t Tool) bool {
	b, err := os.ReadFile(filepath.Join(p.toolDir(t), "tool.json"))
	if err != nil {
		return false
	}
	var s toolState
	if json.Unmarshal(b, &s) != nil || s.Name != t.Name || s.Version != t.Version || !strings.EqualFold(s.SHA256, t.Archive.SHA256) {
		return false
	}
	_, err = os.Stat(filepath.Join(p.toolDir(t), filepath.FromSlash(t.Probe)))
	return err == nil
}

// ToolsInstalled is the names of the tools that are there.
func (p *Platform) ToolsInstalled() []string {
	var out []string
	for _, t := range p.pinnedTools() {
		if p.toolThere(t) {
			out = append(out, t.Name)
		}
	}
	return out
}

// installTools fetches what is missing. A tool that is not fetched on this OS is
// said once, and a pinned one that fails stops the install: the engine would fail
// later with a worse message. It can be run again after a cancel.
func (p *Platform) installTools(ctx context.Context, emit platform.Emit) error {
	fetched := map[string]bool{}
	for _, t := range p.pinnedTools() {
		fetched[t.Name] = true
		if p.toolThere(t) {
			emit(fmt.Sprintf("%s %s is already there", t.Name, t.Version))
			continue
		}
		if t.OnlyIfMissing && t.Name == "git" && p.gitAvailable() {
			emit("git is on this machine: the platform's own is not fetched")
			continue
		}
		if err := p.fetchTool(ctx, t, emit); err != nil {
			return fmt.Errorf("%s: %w", t.Name, err)
		}
	}
	if p.tools == nil && p.fetchesPinned() {
		for _, n := range ToolNames {
			if !fetched[n] {
				emit(systemToolHint(n, p.goos))
			}
		}
	}
	return nil
}

func (p *Platform) fetchTool(ctx context.Context, t Tool, emit platform.Emit) error {
	if err := os.MkdirAll(p.ToolsDir(), 0o755); err != nil {
		return err
	}
	ext := ".zip"
	for _, e := range []string{".tar.xz", ".tar.gz"} {
		if strings.HasSuffix(t.Archive.URL, e) {
			ext = e
		}
	}
	arch := filepath.Join(p.ToolsDir(), t.Name+"-archive"+ext)
	emit(fmt.Sprintf("Downloading %s %s from %s", t.Name, t.Version, t.Archive.URL))
	last := -1
	err := download(ctx, p.client, t.Archive.URL, t.Archive.SHA256, arch, func(done, total int64) {
		if pc := int(done * 100 / total); pc != last {
			last = pc
			emit(fmt.Sprintf("%s: %d%% of %d MB", t.Name, pc, total>>20))
		}
	})
	if err != nil {
		return err
	}
	defer os.Remove(arch)
	emit("Checksum matches " + t.Archive.SHA256)
	// Extract beside the final folder and swap, so a cancelled extraction is never
	// mistaken for the tool.
	dir := p.toolDir(t)
	tmp := dir + ".new"
	if err := removeAll(tmp); err != nil {
		return err
	}
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		return err
	}
	emit(fmt.Sprintf("Unpacking %s into %s", t.Name, dir))
	switch ext {
	case ".zip":
		err = extractZip(arch, tmp, t.Strip)
	case ".tar.gz":
		err = extractTarGz(arch, tmp, t.Strip)
	default:
		err = p.extractTarXZ(ctx, arch, tmp, t.Strip)
	}
	if err != nil {
		_ = removeAll(tmp)
		return err
	}
	if _, err := os.Stat(filepath.Join(tmp, filepath.FromSlash(t.Probe))); err != nil {
		_ = removeAll(tmp)
		return fmt.Errorf("%s is not in the %s archive", t.Probe, t.Name)
	}
	if err := smokeTool(ctx, t, tmp); err != nil {
		_ = removeAll(tmp)
		return err
	}
	st := toolState{Name: t.Name, Version: t.Version, SHA256: t.Archive.SHA256, InstalledAt: time.Now().UTC().Format(time.RFC3339)}
	b, _ := json.MarshalIndent(st, "", "  ")
	if err := os.WriteFile(filepath.Join(tmp, "tool.json"), b, 0o644); err != nil {
		_ = removeAll(tmp)
		return err
	}
	if err := removeAll(dir); err != nil {
		return err
	}
	if err := os.Rename(tmp, dir); err != nil {
		return err
	}
	emit(fmt.Sprintf("%s %s is installed", t.Name, t.Version))
	return nil
}

// extractZip unpacks a zip into dest, dropping strip leading components. A member
// that would land outside dest is an error.
func extractZip(archive, dest string, strip int) error {
	zr, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer zr.Close()
	root, err := filepath.Abs(dest)
	if err != nil {
		return err
	}
	for _, f := range zr.File {
		rel, ok, err := stripName(f.Name, strip)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		target := filepath.Join(root, filepath.FromSlash(rel))
		if target != root && !strings.HasPrefix(target, root+string(filepath.Separator)) {
			return fmt.Errorf("%q would be written outside the tool folder", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if f.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%q is a link; archives of tools are expected to have none", f.Name)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		mode := f.Mode().Perm() | 0o644
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
		if err != nil {
			rc.Close()
			return err
		}
		_, err = io.Copy(out, rc)
		rc.Close()
		if cerr := out.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// extractTarGz unpacks a .tar.gz into dest, dropping strip leading components. Symlinks
// are kept when they stay inside dest; anything that would land outside is an error.
func extractTarGz(archive, dest string, strip int) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	root, err := filepath.Abs(dest)
	if err != nil {
		return err
	}
	inside := func(p string) bool { return p == root || strings.HasPrefix(p, root+string(filepath.Separator)) }
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		rel, ok, err := stripName(h.Name, strip)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		target := filepath.Join(root, filepath.FromSlash(rel))
		if !inside(target) {
			return fmt.Errorf("%q would be written outside the tool folder", h.Name)
		}
		switch h.Typeflag {
		case tar.TypeDir:
			err = os.MkdirAll(target, 0o755)
		case tar.TypeReg:
			if err = os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			var out *os.File
			if out, err = os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, h.FileInfo().Mode().Perm()|0o644); err != nil {
				return err
			}
			_, err = io.Copy(out, tr)
			if cerr := out.Close(); err == nil {
				err = cerr
			}
		case tar.TypeSymlink:
			if path.IsAbs(h.Linkname) || filepath.IsAbs(h.Linkname) || !inside(filepath.Join(filepath.Dir(target), filepath.FromSlash(h.Linkname))) {
				return fmt.Errorf("%q links outside the tool folder", h.Name)
			}
			if err = os.MkdirAll(filepath.Dir(target), 0o755); err == nil {
				err = os.Symlink(h.Linkname, target)
			}
		case tar.TypeLink:
			var from string
			if from, ok, err = stripName(h.Linkname, strip); err != nil || !ok {
				return fmt.Errorf("%q is a link to %q, which is not in the archive", h.Name, h.Linkname)
			}
			err = os.Link(filepath.Join(root, filepath.FromSlash(from)), target)
		default: // devices, fifos: not part of a tool
		}
		if err != nil {
			return err
		}
	}
}

// smokeTool runs the tool's smoke programs from dir (the unpacked, not yet installed
// folder). A program that cannot start, is killed, or says its libraries are missing
// fails the install with the program's own words: that is a build this host cannot run.
func smokeTool(ctx context.Context, t Tool, dir string) error {
	for _, sm := range t.Smoke {
		cmd := exec.CommandContext(ctx, filepath.Join(dir, filepath.FromSlash(sm.Prog)), sm.Args...)
		cmd.Env = append(os.Environ(), toolVars(t, dir)...)
		out, err := cmd.CombinedOutput()
		text := strings.TrimSpace(string(out))
		var ee *exec.ExitError
		bad := false
		switch {
		case err == nil:
		case errors.As(err, &ee):
			c := ee.ExitCode()
			bad = c < 0 || c == 126 || c == 127 || strings.Contains(text, "error while loading shared libraries") || strings.Contains(text, "GLIBC_")
		default:
			bad = true
		}
		if bad {
			if text == "" && err != nil {
				text = err.Error()
			}
			return fmt.Errorf("the downloaded %s %s cannot run on this machine (%s %s: %s). It needs glibc 2.34 or newer and libcurl; install %s with your package manager instead", t.Name, t.Version, sm.Prog, strings.Join(sm.Args, " "), text, t.Name)
		}
	}
	return nil
}

// toolVars is the tool's Env with {dir} replaced.
func toolVars(t Tool, dir string) []string {
	var out []string
	for _, kv := range t.Env {
		out = append(out, strings.ReplaceAll(kv, "{dir}", filepath.ToSlash(dir)))
	}
	return out
}

// stripName drops strip leading components of an archive member name. ok is false
// for a member that is only the stripped folder. ".." anywhere is refused.
func stripName(name string, strip int) (string, bool, error) {
	var parts []string
	for _, s := range strings.Split(strings.ReplaceAll(name, "\\", "/"), "/") {
		switch s {
		case "", ".":
		case "..":
			return "", false, fmt.Errorf("%q has a .. in it", name)
		default:
			parts = append(parts, s)
		}
	}
	if len(parts) <= strip {
		return "", false, nil
	}
	return path.Join(parts[strip:]...), true, nil
}

// tarScript unpacks a tar.xz with Python's tarfile (Go has no xz in its standard
// library, and the platform has a Python: that is what it installs first). It keeps
// the symlinks of a shared-library build and refuses anything that would leave dest.
const tarScript = `
import os, sys, tarfile
src, dest, strip = sys.argv[1], os.path.realpath(sys.argv[2]), int(sys.argv[3])
def cut(n):
    parts = [p for p in n.replace("\\", "/").split("/") if p not in ("", ".")]
    if ".." in parts:
        raise SystemExit("%r has a .. in it" % n)
    return "/".join(parts[strip:])
def inside(p):
    return p == dest or p.startswith(dest + os.sep)
with tarfile.open(src, "r:xz") as t:
    for m in t.getmembers():
        name = cut(m.name)
        if not name:
            continue
        m.name = name
        target = os.path.normpath(os.path.join(dest, name))
        if not inside(target):
            raise SystemExit("%r would be written outside the tool folder" % name)
        if m.issym():
            link = os.path.normpath(os.path.join(os.path.dirname(target), m.linkname))
            if os.path.isabs(m.linkname) or not inside(link):
                raise SystemExit("%r links outside the tool folder" % name)
        elif m.islnk():
            m.linkname = cut(m.linkname)
        elif not (m.isfile() or m.isdir()):
            continue
        t.extract(m, dest)
`

func (p *Platform) extractTarXZ(ctx context.Context, archive, dest string, strip int) error {
	py, err := p.pythonExe(ctx)
	if err != nil {
		return fmt.Errorf("unpacking a .tar.xz needs the platform's Python: %w", err)
	}
	cmd := exec.CommandContext(ctx, py, "-I", "-c", tarScript, archive, dest, fmt.Sprint(strip))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("unpack %s: %w: %s", filepath.Base(archive), err, strings.TrimSpace(string(out)))
	}
	return nil
}

// pythonExe is the pinned Python uv downloaded. uv prints the path on stdout, but
// the lines of stdout and stderr are merged and uv may warn first (for example that
// UV_NATIVE_TLS is deprecated), so the first line is not the path: it is the last
// line that is an absolute path of a file.
func (p *Platform) pythonExe(ctx context.Context) (string, error) {
	var out string
	var seen []string
	err := p.runCapture(ctx, func(l string) {
		if l = strings.TrimSpace(l); l != "" {
			seen = append(seen, l)
			if st, err := os.Stat(l); err == nil && filepath.IsAbs(l) && !st.IsDir() {
				out = l
			}
		}
	}, "python", "find", PythonVersion, "--managed-python", "--no-python-downloads", "--no-config")
	if err != nil {
		return "", err
	}
	if out == "" {
		return "", fmt.Errorf("uv python find printed no path to a Python (it printed: %s)", strings.Join(seen, " | "))
	}
	return out, nil
}

// toolEnv is what the tools add to a command's environment: the tools' folders for
// PATH (put first by environ, in front of whatever PATH the command otherwise has),
// and LD_LIBRARY_PATH for shared libraries on Linux. Only tools that are there count.
func (p *Platform) toolEnv() (pathDirs []string, env []string) {
	var lib []string
	for _, t := range p.pinnedTools() {
		if !p.toolThere(t) {
			continue
		}
		for _, d := range t.Path {
			pathDirs = append(pathDirs, filepath.Join(p.toolDir(t), filepath.FromSlash(d)))
		}
		for _, d := range t.Lib {
			lib = append(lib, filepath.Join(p.toolDir(t), filepath.FromSlash(d)))
		}
		env = append(env, toolVars(t, p.toolDir(t))...)
	}
	if len(lib) > 0 && p.goos == "linux" {
		env = append(env, "LD_LIBRARY_PATH="+strings.Join(append(lib, os.Getenv("LD_LIBRARY_PATH")), ":"))
	}
	return pathDirs, env
}

// needsGit reports whether a requirement is installed from a git URL.
func needsGit(reqs []string) bool {
	for _, r := range reqs {
		if strings.Contains(r, "git+") {
			return true
		}
	}
	return false
}

// gitAvailable looks for git on the PATH the install will use (the tools folder
// first, then the machine's). A file that cannot be run does not count.
func (p *Platform) gitAvailable() bool {
	pathVar := os.Getenv("PATH")
	for _, kv := range p.environ() {
		if len(kv) > 5 && strings.EqualFold(kv[:5], "PATH=") {
			pathVar = kv[5:]
		}
	}
	name := "git"
	if p.goos == "windows" {
		name = "git.exe"
	}
	for _, d := range filepath.SplitList(pathVar) {
		if d == "" {
			continue // an empty PATH entry means the current folder to a shell; never look there
		}
		if st, err := os.Stat(filepath.Join(d, name)); err == nil && !st.IsDir() && (p.goos == "windows" || st.Mode().Perm()&0o111 != 0) {
			return true
		}
	}
	return false
}

// providesGit is whether the platform fetches git on this OS (Windows). Where it
// does, a git that is not there yet is not a problem: the install brings it.
func (p *Platform) providesGit() bool {
	for _, t := range p.pinnedTools() {
		if t.Name == "git" {
			return true
		}
	}
	return false
}

// CheckGit is the pre-flight for an entry whose requirements are installed from a
// git URL: nil when it needs no git, when git is on the PATH an install would use,
// or when the platform fetches git itself. Otherwise the error says what is missing
// and how to fix it. It reads files and downloads nothing, so it can run before
// the plan's first step (the uv platform, Python and ffmpeg are 100 MB or more).
func (p *Platform) CheckGit(label string, reqs []string) error {
	if !needsGit(reqs) || p.providesGit() || p.gitAvailable() {
		return nil
	}
	return &GitMissingError{Label: label, Hint: gitInstallHint(p.goos)}
}

// GitMissingError is a refused install: the entry needs git and the machine has none.
type GitMissingError struct {
	Label string
	Hint  string
}

func (e *GitMissingError) Error() string {
	return fmt.Sprintf("%s installs from a git URL and git is not on this machine. %s. Then install again", e.Label, e.Hint)
}
