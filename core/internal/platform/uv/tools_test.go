package uv

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/rsxdalv/TTS-WebUI/core/internal/gpu"
	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform"
)

var sha64 = regexp.MustCompile(`^[0-9a-f]{64}$`)

func TestToolPinsAreComplete(t *testing.T) {
	for _, name := range ToolNames {
		n := 0
		for _, os := range []string{"windows", "linux", "darwin"} {
			for _, arch := range []string{"amd64", "arm64"} {
				tl, ok := ToolFor(name, os, arch)
				if !ok {
					continue
				}
				n++
				if !sha64.MatchString(tl.Archive.SHA256) || !strings.HasPrefix(tl.Archive.URL, "https://") || tl.Version == "" || tl.Probe == "" || len(tl.Path) == 0 {
					t.Errorf("%s %s/%s: incomplete pin %+v", name, os, arch, tl)
				}
				if strings.HasSuffix(tl.Archive.URL, ".tar.xz") && len(tl.Lib) == 0 {
					t.Errorf("%s %s/%s: a Linux shared build needs its lib folder on LD_LIBRARY_PATH", name, os, arch)
				}
			}
		}
		if n == 0 {
			t.Errorf("%s is pinned nowhere", name)
		}
	}
	// What the plan names: Windows and Linux (amd64, arm64) get both; macOS uses the system's.
	for _, c := range []struct{ tool, os, arch string }{{"git", "windows", "amd64"}, {"git", "linux", "amd64"}, {"git", "linux", "arm64"}, {"ffmpeg", "windows", "amd64"}, {"ffmpeg", "linux", "amd64"}, {"ffmpeg", "linux", "arm64"}} {
		if _, ok := ToolFor(c.tool, c.os, c.arch); !ok {
			t.Errorf("no pin for %s on %s/%s", c.tool, c.os, c.arch)
		}
	}
	if _, ok := ToolFor("git", "darwin", "arm64"); ok {
		t.Error("git on macOS is the system's: nothing is fetched")
	}
	// The Linux git links the host's libc and libcurl: never replace a git that works,
	// and run it once before it counts.
	for _, arch := range []string{"amd64", "arm64"} {
		g, _ := ToolFor("git", "linux", arch)
		if !g.OnlyIfMissing || len(g.Smoke) < 2 || len(g.Env) != 2 || !strings.Contains(g.Archive.URL, "/desktop/dugite-native/releases/download/v"+g.Version+"/") {
			t.Errorf("linux/%s git pin: %+v", arch, g)
		}
	}
}

func toolZip(t *testing.T, files map[string]string) ([]byte, string) {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range files {
		w, _ := zw.Create(name)
		w.Write([]byte(body))
	}
	zw.Close()
	sum := sha256.Sum256(buf.Bytes())
	return buf.Bytes(), hex.EncodeToString(sum[:])
}

func toolPlatform(t *testing.T, srv *httptest.Server, tools []Tool) *Platform {
	t.Helper()
	root, _ := paths.New(t.TempDir())
	return New(Options{Root: root, Client: srv.Client(), Tools: tools, GOOS: runtime.GOOS, GOARCH: runtime.GOARCH})
}

func TestInstallToolsExtractsChecksVerifiesAndIsRepeatable(t *testing.T) {
	data, sum := toolZip(t, map[string]string{"top/bin/fftool": "x", "top/lib/libav.so": "y", "top/": ""})
	hits := 0
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { hits++; w.Write(data) }))
	defer srv.Close()
	tl := Tool{Name: "fftool", Version: "1", Strip: 1, Archive: Archive{srv.URL + "/a.zip", sum}, Path: []string{"bin"}, Lib: []string{"lib"}, Probe: "bin/fftool"}
	p := toolPlatform(t, srv, []Tool{tl})
	var lines []string
	emit := func(l string) { lines = append(lines, l) }
	if err := p.installTools(context.Background(), emit); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(p.ToolsDir(), "fftool", "bin", "fftool")); err != nil {
		t.Fatalf("the top folder was not stripped: %v", err)
	}
	if got := p.ToolsInstalled(); len(got) != 1 || got[0] != "fftool" {
		t.Fatalf("installed: %v", got)
	}
	if entries, _ := os.ReadDir(p.ToolsDir()); len(entries) != 1 {
		t.Fatalf("leftovers in the tools folder (archive or .new): %v", entries)
	}
	// Run again: nothing is fetched.
	if err := p.installTools(context.Background(), emit); err != nil || hits != 1 {
		t.Fatalf("second install: err=%v downloads=%d", err, hits)
	}
	if !strings.Contains(strings.Join(lines, "\n"), "already there") {
		t.Fatalf("no 'already there' line: %v", lines)
	}
}

func TestInstallToolsRefusesABadChecksumAndLeavesNothing(t *testing.T) {
	data, _ := toolZip(t, map[string]string{"bin/fftool": "x"})
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.Write(data) }))
	defer srv.Close()
	tl := Tool{Name: "fftool", Version: "1", Archive: Archive{srv.URL + "/a.zip", strings.Repeat("0", 64)}, Path: []string{"bin"}, Probe: "bin/fftool"}
	p := toolPlatform(t, srv, []Tool{tl})
	err := p.installTools(context.Background(), func(string) {})
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("want a checksum error, got %v", err)
	}
	if len(p.ToolsInstalled()) != 0 {
		t.Fatal("a tool with a bad checksum counts as installed")
	}
	if _, err := os.Stat(filepath.Join(p.ToolsDir(), "fftool")); err == nil {
		t.Fatal("the folder exists")
	}
}

func TestInstallToolsRefusesAnArchiveThatLacksTheProbe(t *testing.T) {
	data, sum := toolZip(t, map[string]string{"bin/other": "x"})
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.Write(data) }))
	defer srv.Close()
	p := toolPlatform(t, srv, []Tool{{Name: "fftool", Version: "1", Archive: Archive{srv.URL + "/a.zip", sum}, Path: []string{"bin"}, Probe: "bin/fftool"}})
	if err := p.installTools(context.Background(), func(string) {}); err == nil || !strings.Contains(err.Error(), "is not in the fftool archive") {
		t.Fatalf("got %v", err)
	}
	if len(p.ToolsInstalled()) != 0 {
		t.Fatal("counted as installed")
	}
}

func TestExtractZipRefusesAPathOutsideTheFolder(t *testing.T) {
	for _, name := range []string{"../evil", "a/../../evil"} {
		data, _ := toolZip(t, map[string]string{name: "x"})
		arch := filepath.Join(t.TempDir(), "a.zip")
		os.WriteFile(arch, data, 0o644)
		dest := filepath.Join(t.TempDir(), "d")
		os.MkdirAll(dest, 0o755)
		if err := extractZip(arch, dest, 0); err == nil {
			t.Errorf("%q was extracted", name)
		}
	}
}

func TestToolsAreFirstOnThePathOfEveryCommand(t *testing.T) {
	data, sum := toolZip(t, map[string]string{"bin/fftool": "x", "lib/libav.so": "y"})
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.Write(data) }))
	defer srv.Close()
	tl := Tool{Name: "fftool", Version: "1", Archive: Archive{srv.URL + "/a.zip", sum}, Path: []string{"bin"}, Lib: []string{"lib"}, Probe: "bin/fftool"}
	p := toolPlatform(t, srv, []Tool{tl})
	bin := filepath.Join(p.ToolsDir(), "fftool", "bin")
	envOf := func() map[string]string {
		c, err := p.Command(&platform.Entry{Package: "x"}, []string{"uv", "--version"})
		if err != nil {
			t.Fatal(err)
		}
		m := map[string]string{}
		for _, kv := range c.Env {
			if k, v, ok := strings.Cut(kv, "="); ok {
				m[strings.ToUpper(k)] = v
			}
		}
		return m
	}
	if strings.Contains(envOf()["PATH"], bin) {
		t.Fatal("a tool that is not installed is on PATH")
	}
	if err := p.installTools(context.Background(), func(string) {}); err != nil {
		t.Fatal(err)
	}
	e := envOf()
	if !strings.HasPrefix(e["PATH"], bin+string(os.PathListSeparator)) {
		t.Fatalf("tools are not first on PATH: %s", e["PATH"])
	}
	if runtime.GOOS == "linux" && !strings.HasPrefix(e["LD_LIBRARY_PATH"], filepath.Join(p.ToolsDir(), "fftool", "lib")) {
		t.Fatalf("lib is not on LD_LIBRARY_PATH: %q", e["LD_LIBRARY_PATH"])
	}
}

func TestNeedsGit(t *testing.T) {
	if !needsGit([]string{"foo==1", "git+https://github.com/a/b.git"}) || needsGit([]string{"foo==1", "bar>=2"}) {
		t.Fatal("needsGit")
	}
}

func TestACreateThatNeedsGitSaysSoWhenThereIsNone(t *testing.T) {
	f := installed(t, gpu.CPU)
	p := f.p
	t.Setenv("PATH", t.TempDir()) // a machine with no git
	var lines []string
	e := platform.Entry{Package: "tts_webui_extension.g", Own: true, Name: "G", Requirements: []string{"git+https://github.com/a/b.git"}}
	if p.Create(context.Background(), e, func(l string) { lines = append(lines, l) }) {
		t.Fatal("Create passed without git")
	}
	if got := strings.Join(lines, "\n"); !strings.Contains(got, "git is not on this machine") {
		t.Fatalf("message: %s", got)
	}
	if _, err := os.Stat(filepath.Join(p.root.Dir(), ".venvs", "tts_webui_extension.g")); err == nil {
		t.Fatal("a venv was started before the check")
	}
}

// fakeGit puts a git program in a new folder and returns the folder.
func fakeGit(t *testing.T, mode os.FileMode) string {
	t.Helper()
	dir := t.TempDir()
	name := "git"
	if runtime.GOOS == "windows" {
		name = "git.exe"
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"), mode); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestCheckGit(t *testing.T) {
	gitReq := []string{"foo==1", "git+https://github.com/a/b.git@main"}
	p := toolPlatform(t, httptest.NewTLSServer(http.NotFoundHandler()), nil)
	p.goos, p.goarch = "linux", "riscv64" // no pinned git for it

	t.Setenv("PATH", t.TempDir()) // no git
	err := p.CheckGit("Kitten", gitReq)
	var miss *GitMissingError
	if !errors.As(err, &miss) {
		t.Fatalf("no git: %v", err)
	}
	for _, want := range []string{"Kitten installs from a git URL", "git is not on this machine", "apt install git", "dnf install git", "install again"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("message lacks %q: %s", want, err)
		}
	}
	if p.CheckGit("Kitten", []string{"foo==1"}) != nil {
		t.Error("an entry with no git URL was refused")
	}
	if runtime.GOOS != "windows" {
		t.Setenv("PATH", fakeGit(t, 0o644)) // there, but cannot be run
		if p.CheckGit("Kitten", gitReq) == nil {
			t.Error("a git that is not executable counted")
		}
	}
	t.Setenv("PATH", fakeGit(t, 0o755))
	if err := p.CheckGit("Kitten", gitReq); err != nil {
		t.Errorf("git on PATH: %v", err)
	}
	// Where the platform fetches git itself (Windows), a missing one is not a refusal.
	t.Setenv("PATH", t.TempDir())
	p.tools = []Tool{{Name: "git", Version: "1"}}
	if err := p.CheckGit("Kitten", gitReq); err != nil {
		t.Errorf("a pinned git is fetched by the install: %v", err)
	}
	p.tools = nil
	p.goarch = "amd64" // the pinned Linux git
	if err := p.CheckGit("Kitten", gitReq); err != nil {
		t.Errorf("linux/amd64 has a pinned git: %v", err)
	}
	p.goos, p.goarch = "darwin", "arm64"
	if err := p.CheckGit("Kitten", gitReq); err == nil || !strings.Contains(err.Error(), "xcode-select --install") {
		t.Errorf("macOS: %v", err)
	}
}

// A machine with no git is told before anything is fetched: the tools server is
// never asked, no tools folder is made, and no venv is started.
func TestCreateWithoutGitFetchesNothing(t *testing.T) {
	f := installed(t, gpu.CPU)
	data, sum := toolZip(t, map[string]string{"bin/fftool": "x"})
	hits := 0
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { hits++; w.Write(data) }))
	defer srv.Close()
	f.p.client = srv.Client()
	f.p.tools = []Tool{{Name: "fftool", Version: "1", Archive: Archive{srv.URL + "/a.zip", sum}, Path: []string{"bin"}, Probe: "bin/fftool"}}
	f.p.goos = "linux"
	t.Setenv("PATH", t.TempDir())
	e := platform.Entry{Package: "tts_webui_extension.g", Own: true, Name: "G", Requirements: []string{"git+https://github.com/a/b.git"}}
	if f.p.Create(context.Background(), e, f.emit) {
		t.Fatal("Create passed without git")
	}
	if got := strings.Join(*f.out, "\n"); !strings.Contains(got, "git is not on this machine") || !strings.Contains(got, "apt install git") {
		t.Fatalf("message: %s", got)
	}
	if hits != 0 {
		t.Errorf("the tools server was asked %d times before the git check", hits)
	}
	if _, err := os.Stat(f.p.ToolsDir()); err == nil {
		t.Error("a tools folder was made")
	}
	if _, err := os.Stat(filepath.Join(f.root.Dir(), ".venvs", e.Package)); err == nil {
		t.Error("a venv was started")
	}
	if c := f.calls(); len(c) != 0 {
		t.Errorf("uv was run: %v", c)
	}
}

type tarMember struct {
	name, body, link string
	mode             int64
	typ              byte
}

func toolTarGz(t *testing.T, members []tarMember) ([]byte, string) {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, m := range members {
		typ := m.typ
		if typ == 0 {
			typ = tar.TypeReg
		}
		h := &tar.Header{Name: m.name, Typeflag: typ, Mode: m.mode, Linkname: m.link}
		if typ == tar.TypeReg {
			h.Size = int64(len(m.body))
		}
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if typ == tar.TypeReg {
			tw.Write([]byte(m.body))
		}
	}
	tw.Close()
	gz.Close()
	sum := sha256.Sum256(buf.Bytes())
	return buf.Bytes(), hex.EncodeToString(sum[:])
}

// fakeGitTar is laid out like dugite-native's Linux archive: members start with "./",
// no top folder, a symlink among the helpers, programs that are shell scripts.
func fakeGitTar(t *testing.T, remoteHTTPS string) ([]byte, string) {
	return toolTarGz(t, []tarMember{
		{name: "./", typ: tar.TypeDir, mode: 0o755},
		{name: "./bin/", typ: tar.TypeDir, mode: 0o755},
		{name: "./bin/git", body: "#!/bin/sh\necho git version fake\n", mode: 0o755},
		{name: "./libexec/git-core/", typ: tar.TypeDir, mode: 0o755},
		{name: "./libexec/git-core/git-remote-https", body: remoteHTTPS, mode: 0o755},
		{name: "./libexec/git-core/git-add", typ: tar.TypeSymlink, link: "../../bin/git", mode: 0o777},
		{name: "./share/git-core/templates/description", body: "x", mode: 0o644},
	})
}

const okRemoteHTTPS = "#!/bin/sh\necho 'error: remote-curl: usage: git remote-curl <remote> [<url>]' >&2\nexit 1\n"

func fakeGitTool(url, sum string) Tool {
	return Tool{Name: "git", Version: "9", OnlyIfMissing: true, Archive: Archive{url + "/git.tar.gz", sum},
		Path: []string{"bin"}, Probe: "bin/git",
		Env:   []string{"GIT_EXEC_PATH={dir}/libexec/git-core", "GIT_TEMPLATE_DIR={dir}/share/git-core/templates"},
		Smoke: []Smoke{{Prog: "bin/git", Args: []string{"--version"}}, {Prog: "libexec/git-core/git-remote-https"}}}
}

func TestExtractTarGz(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks and modes")
	}
	data, _ := fakeGitTar(t, okRemoteHTTPS)
	arch := filepath.Join(t.TempDir(), "a.tar.gz")
	os.WriteFile(arch, data, 0o644)
	dest := filepath.Join(t.TempDir(), "d")
	os.MkdirAll(dest, 0o755)
	if err := extractTarGz(arch, dest, 0); err != nil {
		t.Fatal(err)
	}
	if st, err := os.Stat(filepath.Join(dest, "bin", "git")); err != nil || st.Mode().Perm()&0o111 == 0 {
		t.Fatalf("bin/git: %v %v", st, err)
	}
	if l, err := os.Readlink(filepath.Join(dest, "libexec", "git-core", "git-add")); err != nil || l != "../../bin/git" {
		t.Fatalf("symlink: %q %v", l, err)
	}
	if _, err := os.Stat(filepath.Join(dest, "share", "git-core", "templates", "description")); err != nil {
		t.Fatal(err)
	}
	for name, ms := range map[string][]tarMember{
		"dotdot":            {{name: "../evil", body: "x", mode: 0o644}},
		"absolute symlink":  {{name: "l", typ: tar.TypeSymlink, link: "/etc/passwd"}},
		"escaping symlink":  {{name: "a/l", typ: tar.TypeSymlink, link: "../../out"}},
		"link not in there": {{name: "h", typ: tar.TypeLink, link: "missing"}},
	} {
		data, _ := toolTarGz(t, ms)
		arch := filepath.Join(t.TempDir(), "b.tar.gz")
		os.WriteFile(arch, data, 0o644)
		d := filepath.Join(t.TempDir(), "d")
		os.MkdirAll(d, 0o755)
		if err := extractTarGz(arch, d, 0); err == nil {
			t.Errorf("%s was extracted", name)
		}
	}
}

func TestPinnedGitIsFetchedOnlyWhenTheMachineHasNone(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell scripts")
	}
	data, sum := fakeGitTar(t, okRemoteHTTPS)
	hits := 0
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { hits++; w.Write(data) }))
	defer srv.Close()
	tl := fakeGitTool(srv.URL, sum)

	// A machine with a git: nothing is fetched and nothing is made.
	t.Setenv("PATH", fakeGit(t, 0o755))
	p := toolPlatform(t, srv, []Tool{tl})
	var lines []string
	emit := func(l string) { lines = append(lines, l) }
	if err := p.installTools(context.Background(), emit); err != nil || hits != 0 {
		t.Fatalf("with a git: err=%v downloads=%d", err, hits)
	}
	if _, err := os.Stat(p.ToolsDir()); err == nil || !strings.Contains(strings.Join(lines, "\n"), "git is on this machine") {
		t.Fatalf("with a git: tools folder made or not said: %v", lines)
	}

	// A machine without: fetched, smoke-tested, on PATH, with its variables.
	t.Setenv("PATH", t.TempDir())
	p = toolPlatform(t, srv, []Tool{tl})
	if err := p.installTools(context.Background(), emit); err != nil || hits != 1 {
		t.Fatalf("without a git: err=%v downloads=%d", err, hits)
	}
	dir := filepath.Join(p.ToolsDir(), "git")
	c, _ := p.Command(&platform.Entry{Package: "x"}, []string{"uv", "--version"})
	env := map[string]string{}
	for _, kv := range c.Env {
		if k, v, ok := strings.Cut(kv, "="); ok {
			env[k] = v
		}
	}
	if !strings.HasPrefix(env["PATH"], filepath.Join(dir, "bin")+string(os.PathListSeparator)) {
		t.Errorf("PATH: %s", env["PATH"])
	}
	if env["GIT_EXEC_PATH"] != filepath.ToSlash(filepath.Join(dir, "libexec", "git-core")) || env["GIT_TEMPLATE_DIR"] != filepath.ToSlash(filepath.Join(dir, "share", "git-core", "templates")) {
		t.Errorf("git variables: %q %q", env["GIT_EXEC_PATH"], env["GIT_TEMPLATE_DIR"])
	}
	if !p.gitAvailable() {
		t.Error("the fetched git is not found on the platform's PATH")
	}
	if err := p.CheckGit("G", []string{"git+https://x/y"}); err != nil {
		t.Error(err)
	}
	// Again: already there, no second download.
	if err := p.installTools(context.Background(), emit); err != nil || hits != 1 {
		t.Fatalf("again: err=%v downloads=%d", err, hits)
	}
}

func TestPinnedGitRefusedForABadChecksumOrABuildThatCannotRun(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell scripts")
	}
	t.Setenv("PATH", t.TempDir())
	good, _ := fakeGitTar(t, okRemoteHTTPS)
	broken, brokenSum := fakeGitTar(t, "#!/bin/sh\necho 'git-remote-https: error while loading shared libraries: libcurl.so.4: cannot open shared object file' >&2\nexit 127\n")
	old, oldSum := toolTarGz(t, []tarMember{
		{name: "./bin/git", body: "#!/bin/sh\necho \"git: /lib/libc.so.6: version \\`GLIBC_2.34' not found\" >&2\nexit 1\n", mode: 0o755},
		{name: "./libexec/git-core/git-remote-https", body: okRemoteHTTPS, mode: 0o755},
	})
	for _, c := range []struct {
		name, want string
		data       []byte
		sum        string
	}{
		{"wrong checksum", "checksum mismatch", good, strings.Repeat("0", 64)},
		{"no libcurl", "libcurl.so.4", broken, brokenSum},
		{"old glibc", "GLIBC_2.34", old, oldSum},
	} {
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.Write(c.data) }))
		p := toolPlatform(t, srv, []Tool{fakeGitTool(srv.URL, c.sum)})
		err := p.installTools(context.Background(), func(string) {})
		srv.Close()
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Fatalf("%s: got %v", c.name, err)
		}
		if c.name != "wrong checksum" && !strings.Contains(err.Error(), "cannot run on this machine") {
			t.Errorf("%s: message: %v", c.name, err)
		}
		if len(p.ToolsInstalled()) != 0 {
			t.Errorf("%s: counts as installed", c.name)
		}
		if left, _ := os.ReadDir(p.ToolsDir()); len(left) != 0 {
			t.Errorf("%s: left in the tools folder: %v", c.name, left)
		}
	}
}
