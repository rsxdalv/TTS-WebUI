package audiocpp

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
	"sync/atomic"
	"testing"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform"
)

func zipOf(t *testing.T, files map[string]string) ([]byte, string) {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for n, c := range files {
		w, err := zw.Create(n)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(c))
	}
	zw.Close()
	sum := sha256.Sum256(buf.Bytes())
	return buf.Bytes(), hex.EncodeToString(sum[:])
}

func sum(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }

func serverName() string {
	if runtime.GOOS == "windows" {
		return "audiocpp_server.exe"
	}
	return "audiocpp_server"
}

type fixture struct {
	p    *Platform
	srv  *httptest.Server
	hits *atomic.Int32
	root string
}

func newFixture(t *testing.T, files map[string]string, mutate func(*Archive)) fixture {
	t.Helper()
	data, sha := zipOf(t, files)
	var hits atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Write(data)
	}))
	t.Cleanup(srv.Close)
	a := &Archive{URL: srv.URL + "/audio.zip", SHA256: sha, Size: int64(len(data))}
	if mutate != nil {
		mutate(a)
	}
	dir := t.TempDir()
	root, _ := paths.New(dir)
	p := New(Options{Root: root, Archive: a, Client: srv.Client()})
	return fixture{p: p, srv: srv, hits: &hits, root: dir}
}

func TestInstallChecksTheArchiveAndWritesTheStateLast(t *testing.T) {
	f := newFixture(t, map[string]string{serverName(): "x", "model_specs/a.json": "{}"}, nil)
	var lines []string
	if err := f.p.Install(context.Background(), func(l string) { lines = append(lines, l) }); err != nil {
		t.Fatal(err)
	}
	if st := f.p.Status(context.Background()); !st.Ready {
		t.Fatalf("not ready: %s", st.Detail)
	}
	if _, err := os.Stat(filepath.Join(f.p.BinDir(), "model_specs", "a.json")); err != nil {
		t.Error("the archive's folders were not unpacked:", err)
	}
	if !strings.Contains(strings.Join(lines, "\n"), "Checksum matches") {
		t.Errorf("no checksum line: %v", lines)
	}
	if s, ok := f.p.Info(); !ok || s.Version != Version || s.Row != RowCPU {
		t.Errorf("state = %+v %v", s, ok)
	}
	for _, left := range []string{"audio.zip", "audio.zip.part", "bin.new"} {
		if _, err := os.Stat(filepath.Join(f.p.Dir(), left)); err == nil {
			t.Errorf("%s was left behind", left)
		}
	}
	// A second install is a no-op: nothing is downloaded again.
	n := f.hits.Load()
	if err := f.p.Install(context.Background(), func(string) {}); err != nil {
		t.Fatal(err)
	}
	if f.hits.Load() != n {
		t.Error("the second install downloaded again")
	}
}

func TestAWrongChecksumInstallsNothing(t *testing.T) {
	f := newFixture(t, map[string]string{serverName(): "x"}, func(a *Archive) { a.SHA256 = strings.Repeat("0", 64) })
	err := f.p.Install(context.Background(), func(string) {})
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("err = %v", err)
	}
	if st := f.p.Status(context.Background()); st.Ready {
		t.Error("ready after a refused archive")
	}
	if _, err := os.Stat(f.p.BinDir()); err == nil {
		t.Error("bin/ exists after a refused archive")
	}
	for _, left := range []string{"audio.zip", "audio.zip.part"} {
		if _, err := os.Stat(filepath.Join(f.p.Dir(), left)); err == nil {
			t.Errorf("%s was left behind", left)
		}
	}
}

func TestAnArchiveWithoutTheServerOrWithAnEscapingNameIsRefused(t *testing.T) {
	f := newFixture(t, map[string]string{"readme.txt": "x"}, nil)
	if err := f.p.Install(context.Background(), func(string) {}); err == nil || !strings.Contains(err.Error(), "has no") {
		t.Errorf("no server: err = %v", err)
	}
	if _, err := os.Stat(f.p.BinDir()); err == nil {
		t.Error("bin/ exists")
	}
	g := newFixture(t, map[string]string{serverName(): "x", "../evil.txt": "x"}, nil)
	if err := g.p.Install(context.Background(), func(string) {}); err == nil {
		t.Error("an archive with ../ was accepted")
	}
	if _, err := os.Stat(filepath.Join(g.p.Dir(), "..", "evil.txt")); err == nil {
		t.Error("a file was written outside the folder")
	}
}

func TestACancelledDownloadLeavesNothingAndTheNextInstallCompletes(t *testing.T) {
	data, sha := zipOf(t, map[string]string{serverName(): "x"})
	started := make(chan struct{})
	var block atomic.Bool
	block.Store(true)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if block.Load() {
			w.Header().Set("Content-Length", "100000")
			w.Write(data[:10])
			w.(http.Flusher).Flush()
			select {
			case started <- struct{}{}:
			default:
			}
			<-r.Context().Done()
			return
		}
		w.Write(data)
	}))
	defer srv.Close()
	root, _ := paths.New(t.TempDir())
	p := New(Options{Root: root, Archive: &Archive{URL: srv.URL, SHA256: sha}, Client: srv.Client()})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- p.Install(ctx, func(string) {}) }()
	<-started
	cancel()
	err := <-done
	if err == nil || !strings.Contains(err.Error(), "cancelled") {
		t.Fatalf("err = %v", err)
	}
	if p.Status(context.Background()).Ready {
		t.Fatal("ready after a cancel")
	}
	if _, err := os.Stat(p.BinDir()); err == nil {
		t.Error("bin/ exists after a cancel")
	}
	block.Store(false)
	if err := p.Install(context.Background(), func(string) {}); err != nil {
		t.Fatal(err)
	}
	if !p.Status(context.Background()).Ready {
		t.Error("not ready after the second install")
	}
}

func TestOnlyThePinnedRowsAndPlatformsInstall(t *testing.T) {
	if _, err := ArchiveFor("linux", "amd64", RowCPU); err == nil || !strings.Contains(err.Error(), "only Windows x64") {
		t.Errorf("linux: %v", err)
	}
	if _, err := ArchiveFor("windows", "amd64", "cuda"); err == nil {
		t.Error("cuda is not a row")
	}
	sha64 := regexp.MustCompile(`^[0-9a-f]{64}$`)
	for _, r := range Rows {
		a, err := ArchiveFor("windows", "amd64", r.Name)
		if err != nil || !sha64.MatchString(a.SHA256) || !strings.HasPrefix(a.URL, "https://github.com/0xShug0/audio.cpp/releases/download/"+Version+"/") || a.Size == 0 {
			t.Errorf("row %s: %+v %v", r.Name, a, err)
		}
	}
	for _, m := range Models {
		if m.Package == "" || m.ID == "" || m.Dir == "" || !m.HasVoice(m.DefaultVoice) {
			t.Errorf("model %+v is incomplete", m)
		}
		var hasGGUF bool
		for _, f := range m.Files {
			if !sha64.MatchString(f.SHA256) || !strings.HasPrefix(f.URL, "https://") || f.Size == 0 || strings.Contains(f.Rel, "..") {
				t.Errorf("%s: file %+v", m.Package, f)
			}
			hasGGUF = hasGGUF || f.Rel == m.GGUF
		}
		if !hasGGUF {
			t.Errorf("%s: the GGUF %s is not in Files", m.Package, m.GGUF)
		}
	}
}

func modelFixture(t *testing.T, bad bool) (*Platform, *atomic.Int32) {
	t.Helper()
	gguf, embed := []byte(strings.Repeat("g", 5000)), []byte("embedding")
	var hits atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if strings.HasSuffix(r.URL.Path, ".gguf") {
			if bad {
				w.Write([]byte(strings.Repeat("h", 5000)))
				return
			}
			w.Write(gguf)
			return
		}
		w.Write(embed)
	}))
	t.Cleanup(srv.Close)
	m := Model{Package: "audiocpp_fake", ID: "fake", Dir: "fake", GGUF: "m.gguf", Files: []File{
		{"m.gguf", srv.URL + "/m.gguf", sum(gguf), int64(len(gguf))},
		{"embeddings/v.safetensors", srv.URL + "/v.safetensors", sum(embed), int64(len(embed))},
	}}
	f := newFixture(t, map[string]string{serverName(): "x"}, nil)
	p := New(Options{Root: f.p.root, Archive: f.p.archive, Client: srv.Client(), Models: []Model{m}})
	if err := p.Install(context.Background(), func(string) {}); err != nil {
		t.Fatal(err)
	}
	return p, &hits
}

func TestModelInstallStatusAndRemove(t *testing.T) {
	p, hits := modelFixture(t, false)
	e := platform.Entry{Package: "audiocpp_fake"}
	ctx := context.Background()
	if p.EntryStatus(ctx, e).Installed {
		t.Fatal("installed before Create")
	}
	if !p.Create(ctx, e, func(string) {}) {
		t.Fatal("Create failed")
	}
	if !p.EntryStatus(ctx, e).Installed {
		t.Fatal("not installed after Create")
	}
	n := hits.Load()
	if !p.Create(ctx, e, func(string) {}) || hits.Load() != n {
		t.Error("a second Create downloaded again")
	}
	if !p.Remove(ctx, e, func(string) {}) {
		t.Fatal("Remove failed")
	}
	if st := p.EntryStatus(ctx, e); st.Installed || st.Partial {
		t.Errorf("after Remove: %+v", st)
	}
}

func TestModelWithABadChecksumIsNotInstalled(t *testing.T) {
	p, _ := modelFixture(t, true)
	e := platform.Entry{Package: "audiocpp_fake"}
	var lines []string
	if p.Create(context.Background(), e, func(l string) { lines = append(lines, l) }) {
		t.Fatal("Create accepted a file with the wrong checksum")
	}
	if !strings.Contains(strings.Join(lines, "\n"), "checksum mismatch") {
		t.Errorf("lines = %v", lines)
	}
	if st := p.EntryStatus(context.Background(), e); st.Installed {
		t.Errorf("installed: %+v", st)
	}
	if _, err := os.Stat(filepath.Join(p.ModelDir(p.models[0]), "m.gguf")); err == nil {
		t.Error("the bad file has its final name")
	}
}

func TestAModelNeedsThePlatform(t *testing.T) {
	root, _ := paths.New(t.TempDir())
	p := New(Options{Root: root})
	var lines []string
	if p.Create(context.Background(), platform.Entry{Package: "audiocpp_pocket_tts_en"}, func(l string) { lines = append(lines, l) }) {
		t.Fatal("Create worked without the platform")
	}
	if !strings.Contains(strings.Join(lines, "\n"), "requires audiocpp") {
		t.Errorf("lines = %v", lines)
	}
}
