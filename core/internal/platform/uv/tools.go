package uv

import (
	"archive/zip"
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
	if strings.HasSuffix(t.Archive.URL, ".tar.xz") {
		ext = ".tar.xz"
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
	if strings.HasSuffix(arch, ".zip") {
		err = extractZip(arch, tmp, t.Strip)
	} else {
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

// pythonExe is the pinned Python uv downloaded.
func (p *Platform) pythonExe(ctx context.Context) (string, error) {
	var out string
	err := p.runCapture(ctx, func(l string) {
		if l = strings.TrimSpace(l); out == "" && l != "" {
			out = l
		}
	}, "python", "find", PythonVersion, "--managed-python", "--no-python-downloads", "--no-config")
	if err != nil {
		return "", err
	}
	if out == "" {
		return "", errors.New("uv python find printed nothing")
	}
	return out, nil
}

// toolEnv is what the tools add to a command's environment: PATH first, and
// LD_LIBRARY_PATH for shared libraries on Linux. Only tools that are there count.
// base is the environment it extends (the inherited one).
func (p *Platform) toolEnv() []string {
	var path, lib []string
	for _, t := range p.pinnedTools() {
		if !p.toolThere(t) {
			continue
		}
		for _, d := range t.Path {
			path = append(path, filepath.Join(p.toolDir(t), filepath.FromSlash(d)))
		}
		for _, d := range t.Lib {
			lib = append(lib, filepath.Join(p.toolDir(t), filepath.FromSlash(d)))
		}
	}
	var env []string
	if len(path) > 0 {
		env = append(env, "PATH="+strings.Join(append(path, os.Getenv("PATH")), string(os.PathListSeparator)))
	}
	if len(lib) > 0 && p.goos == "linux" {
		env = append(env, "LD_LIBRARY_PATH="+strings.Join(append(lib, os.Getenv("LD_LIBRARY_PATH")), ":"))
	}
	return env
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
// first, then the machine's).
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
		if st, err := os.Stat(filepath.Join(d, name)); err == nil && !st.IsDir() {
			return true
		}
	}
	return false
}
