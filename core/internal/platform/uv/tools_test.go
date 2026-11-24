package uv

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
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
	// What the plan names: Windows gets both, Linux gets ffmpeg and uses the system's git.
	for _, c := range []struct{ tool, os, arch string }{{"git", "windows", "amd64"}, {"ffmpeg", "windows", "amd64"}, {"ffmpeg", "linux", "amd64"}, {"ffmpeg", "linux", "arm64"}} {
		if _, ok := ToolFor(c.tool, c.os, c.arch); !ok {
			t.Errorf("no pin for %s on %s/%s", c.tool, c.os, c.arch)
		}
	}
	if _, ok := ToolFor("git", "linux", "amd64"); ok {
		t.Error("git on Linux is the system's: nothing is fetched")
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
