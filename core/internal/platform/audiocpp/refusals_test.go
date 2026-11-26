package audiocpp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/rsxdalv/TTS-WebUI/core/internal/platform"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform/dltest"
)

// A broken download of the server archive (404, cut off, reset, truncated, empty,
// wrong bytes) is refused with the URL in the message, leaves nothing in the
// platform's folder, and the same install works once the server is right.
func TestBrokenArchiveDownloadsAreRefusedAndTheRetrySucceeds(t *testing.T) {
	data, sha := zipOf(t, map[string]string{serverName(): "x", "model_specs/a.json": "{}"})
	sw := dltest.NewSwitch(t, data, dltest.OK)
	f := newFixture(t, map[string]string{serverName(): "x"}, nil)
	url := sw.URL + "/audio.zip"
	f.p = New(Options{Root: f.p.root, Archive: &Archive{URL: url, SHA256: sha}})
	for _, m := range dltest.Failures {
		sw.Set(m)
		err := f.p.Install(context.Background(), func(string) {})
		if err == nil || !strings.Contains(err.Error(), url) {
			t.Fatalf("%s: want an error naming %s, got %v", m, url, err)
		}
		if f.p.Status(context.Background()).Ready {
			t.Errorf("%s: ready after a refused archive", m)
		}
		dltest.AssertEmpty(t, f.p.Dir())
	}
	sw.Set(dltest.OK)
	if err := f.p.Install(context.Background(), func(string) {}); err != nil {
		t.Fatal(err)
	}
	if !f.p.Status(context.Background()).Ready {
		t.Fatal("not ready after the retry")
	}
	for _, left := range []string{"audio.zip", "audio.zip.part", "bin.new"} {
		if _, err := os.Stat(filepath.Join(f.p.Dir(), left)); err == nil {
			t.Errorf("%s was left behind by the retry", left)
		}
	}
}

// A model whose second file fails keeps the first (it was checked), writes no
// model.json, leaves no partial file, and the retry fetches only what is missing.
func TestBrokenModelFileKeepsTheCheckedFilesAndTheRetryFetchesTheRest(t *testing.T) {
	gguf, embed := []byte(strings.Repeat("g", 5000)), []byte(strings.Repeat("e", 70000))
	var mode atomic.Value
	var ggufHits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".gguf") {
			ggufHits.Add(1)
			w.Write(gguf)
			return
		}
		dltest.Handler(embed, mode.Load().(dltest.Mode))(w, r)
	}))
	t.Cleanup(srv.Close)
	m := Model{Package: "audiocpp_fake", ID: "fake", Dir: "fake", GGUF: "m.gguf", Files: []File{
		{"m.gguf", srv.URL + "/m.gguf", sum(gguf), int64(len(gguf))},
		{"embeddings/v.safetensors", srv.URL + "/v.safetensors", sum(embed), int64(len(embed))},
	}}
	f := newFixture(t, map[string]string{serverName(): "x"}, nil)
	if err := f.p.Install(context.Background(), func(string) {}); err != nil {
		t.Fatal(err)
	}
	p := New(Options{Root: f.p.root, Archive: f.p.archive, Models: []Model{m}})
	e := platform.Entry{Package: "audiocpp_fake"}
	dir := p.ModelDir(m)
	for _, md := range dltest.Failures {
		mode.Store(md)
		var lines []string
		if p.Create(context.Background(), e, func(l string) { lines = append(lines, l) }) {
			t.Fatalf("%s: Create accepted a broken file", md)
		}
		if log := strings.Join(lines, "\n"); !strings.Contains(log, srv.URL+"/v.safetensors") {
			t.Errorf("%s: the message does not name the file: %v", md, lines)
		}
		if st := p.EntryStatus(context.Background(), e); st.Installed || !st.Partial {
			t.Errorf("%s: want a partial model, got %+v", md, st)
		}
		var names []string
		filepath.WalkDir(dir, func(path string, d os.DirEntry, _ error) error {
			if !d.IsDir() {
				names = append(names, filepath.ToSlash(strings.TrimPrefix(path, dir+string(os.PathSeparator))))
			}
			return nil
		})
		if len(names) != 1 || names[0] != "m.gguf" {
			t.Errorf("%s: want only the checked m.gguf in the model folder, got %v", md, names)
		}
	}
	mode.Store(dltest.OK)
	before := ggufHits.Load()
	if !p.Create(context.Background(), e, func(string) {}) {
		t.Fatal("the retry failed")
	}
	if ggufHits.Load() != before {
		t.Error("the retry downloaded the checked file again")
	}
	if !p.EntryStatus(context.Background(), e).Installed {
		t.Error("not installed after the retry")
	}
}

// A file with the right size and other bytes (a damaged copy) is not trusted: it is
// downloaded again, and nothing but the right bytes ends up under its name.
func TestAKeptModelFileOfTheRightSizeButOtherBytesIsDownloadedAgain(t *testing.T) {
	p, hits := modelFixture(t, false)
	e := platform.Entry{Package: "audiocpp_fake"}
	if !p.Create(context.Background(), e, func(string) {}) {
		t.Fatal("Create failed")
	}
	gguf := filepath.Join(p.ModelDir(p.models[0]), "m.gguf")
	if err := os.WriteFile(gguf, []byte(strings.Repeat("X", 5000)), 0o644); err != nil { // same size as the pinned file
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(p.ModelDir(p.models[0]), ModelFile)); err != nil {
		t.Fatal(err)
	}
	n := hits.Load()
	var lines []string
	if !p.Create(context.Background(), e, func(l string) { lines = append(lines, l) }) {
		t.Fatal(lines)
	}
	if hits.Load() == n || !strings.Contains(strings.Join(lines, "\n"), "does not match its checksum") {
		t.Errorf("the damaged file was kept: hits %d -> %d, %v", n, hits.Load(), lines)
	}
	if !fileMatches(gguf, p.models[0].Files[0].SHA256) {
		t.Error("the file under the model's name is not the pinned one")
	}
}
