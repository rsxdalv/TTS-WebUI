package uv

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rsxdalv/TTS-WebUI/core/internal/gpu"
	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform/dltest"
)

// Every way a download can go wrong ends the same way: the download is refused
// with a message that names the file's URL, and no file, partial file or folder is
// left behind. Only the pinned bytes are accepted (and then only the file remains).
func sum(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

func TestDownloadRefusesAnythingButThePinnedBytes(t *testing.T) {
	body := bytes.Repeat([]byte("pinned bytes "), 40000) // ~500 kB: several reads
	want := sum(body)
	for _, c := range []struct {
		mode dltest.Mode
		msg  string
	}{
		{dltest.NotFound, "HTTP 404"},
		{dltest.Cut, "the transfer broke after"},
		{dltest.CutChunked, "the transfer broke after"},
		{dltest.Reset, "the transfer broke after"},
		{dltest.Short, "checksum mismatch"},
		{dltest.Empty, "checksum mismatch"},
	} {
		t.Run(string(c.mode), func(t *testing.T) {
			srv := dltest.Server(t, body, c.mode)
			dir := t.TempDir()
			url := srv.URL + "/uv.tar.gz"
			err := download(context.Background(), nil, url, want, filepath.Join(dir, "uv.tar.gz"), nil)
			if err == nil || !strings.Contains(err.Error(), c.msg) || !strings.Contains(err.Error(), url) {
				t.Fatalf("want an error with %q and the URL, got %v", c.msg, err)
			}
			dltest.AssertEmpty(t, dir)
		})
	}
	t.Run("wrong pin", func(t *testing.T) {
		srv := dltest.Server(t, body, dltest.OK)
		dir := t.TempDir()
		url := srv.URL + "/uv.tar.gz"
		err := download(context.Background(), nil, url, strings.Repeat("0", 64), filepath.Join(dir, "uv.tar.gz"), nil)
		if err == nil || !strings.Contains(err.Error(), "checksum mismatch for "+url) || !strings.Contains(err.Error(), want) {
			t.Fatalf("want a checksum error naming the URL and the real hash, got %v", err)
		}
		dltest.AssertEmpty(t, dir)
	})
	t.Run("right pin", func(t *testing.T) {
		srv := dltest.Server(t, body, dltest.OK)
		dir := t.TempDir()
		dest := filepath.Join(dir, "uv.tar.gz")
		if err := download(context.Background(), nil, srv.URL+"/uv.tar.gz", want, dest, nil); err != nil {
			t.Fatal(err)
		}
		if ents, _ := os.ReadDir(dir); len(ents) != 1 || ents[0].Name() != "uv.tar.gz" {
			t.Fatalf("want only the file, got %v", ents)
		}
	})
}

// A broken download of uv itself is refused with the URL in the message, leaves no
// file in bin/ and no platform.json, and the same install works once the server is
// right (the fake uv runs the Python step).
func TestBrokenUvDownloadsAreRefusedAndTheRetrySucceeds(t *testing.T) {
	data, sha := fakeArchive(t)
	sw := dltest.NewSwitch(t, data, dltest.OK)
	root, _ := paths.New(t.TempDir())
	url := sw.URL + "/uv.zip"
	p := New(Options{Root: root, GPU: gpu.CPU, SkipPathCheck: true, Archive: &Archive{URL: url, SHA256: sha}})
	for _, m := range dltest.Failures {
		sw.Set(m)
		err := p.Install(context.Background(), func(string) {})
		if err == nil || !strings.Contains(err.Error(), url) {
			t.Fatalf("%s: want an error naming %s, got %v", m, url, err)
		}
		if p.Status(context.Background()).Ready {
			t.Errorf("%s: ready after a refused uv", m)
		}
		// Install made bin/ before it downloaded; it is empty, and nothing else exists.
		dltest.AssertEmpty(t, filepath.Join(p.Dir(), "bin"))
		if ents, _ := os.ReadDir(p.Dir()); len(ents) != 1 {
			t.Errorf("%s: want only the empty bin/ in %s, got %v", m, p.Dir(), ents)
		}
	}
	sw.Set(dltest.OK)
	if err := p.Install(context.Background(), func(string) {}); err != nil {
		t.Fatal(err)
	}
	if !p.Status(context.Background()).Ready {
		t.Fatal("not ready after the retry")
	}
}

// The same for the tools (git, ffmpeg): a broken download leaves nothing in the
// tools folder, and the retry installs the tool.
func TestBrokenToolDownloadsAreRefusedAndTheRetrySucceeds(t *testing.T) {
	data, sha := toolZip(t, map[string]string{"top/bin/fftool": "x"})
	sw := dltest.NewSwitch(t, data, dltest.OK)
	url := sw.URL + "/a.zip"
	tl := Tool{Name: "fftool", Version: "1", Strip: 1, Archive: Archive{url, sha}, Path: []string{"bin"}, Probe: "bin/fftool"}
	root, _ := paths.New(t.TempDir())
	p := New(Options{Root: root, Tools: []Tool{tl}, GOOS: runtime.GOOS, GOARCH: runtime.GOARCH})
	for _, m := range dltest.Failures {
		sw.Set(m)
		err := p.installTools(context.Background(), func(string) {})
		if err == nil || !strings.Contains(err.Error(), url) || !strings.Contains(err.Error(), "fftool") {
			t.Fatalf("%s: want an error naming the tool and %s, got %v", m, url, err)
		}
		if len(p.ToolsInstalled()) != 0 {
			t.Errorf("%s: counted as installed", m)
		}
		dltest.AssertEmpty(t, p.ToolsDir())
	}
	sw.Set(dltest.OK)
	if err := p.installTools(context.Background(), func(string) {}); err != nil {
		t.Fatal(err)
	}
	if got := p.ToolsInstalled(); len(got) != 1 {
		t.Fatalf("installed after the retry: %v", got)
	}
}
