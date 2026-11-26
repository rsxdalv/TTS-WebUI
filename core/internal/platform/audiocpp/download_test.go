package audiocpp

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rsxdalv/TTS-WebUI/core/internal/platform/dltest"
)

// Every way a download can go wrong ends the same way: the download is refused
// with a message that names the file's URL, and no file, partial file or folder is
// left behind. Only the pinned bytes are accepted (and then only the file remains).
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
			url := srv.URL + "/audio.zip"
			err := download(context.Background(), nil, url, want, filepath.Join(dir, "audio.zip"), nil)
			if err == nil || !strings.Contains(err.Error(), c.msg) || !strings.Contains(err.Error(), url) {
				t.Fatalf("want an error with %q and the URL, got %v", c.msg, err)
			}
			dltest.AssertEmpty(t, dir)
		})
	}
	t.Run("wrong pin", func(t *testing.T) {
		srv := dltest.Server(t, body, dltest.OK)
		dir := t.TempDir()
		url := srv.URL + "/audio.zip"
		err := download(context.Background(), nil, url, strings.Repeat("0", 64), filepath.Join(dir, "audio.zip"), nil)
		if err == nil || !strings.Contains(err.Error(), "checksum mismatch for "+url) || !strings.Contains(err.Error(), want) {
			t.Fatalf("want a checksum error naming the URL and the real hash, got %v", err)
		}
		dltest.AssertEmpty(t, dir)
	})
	t.Run("right pin", func(t *testing.T) {
		srv := dltest.Server(t, body, dltest.OK)
		dir := t.TempDir()
		dest := filepath.Join(dir, "audio.zip")
		if err := download(context.Background(), nil, srv.URL+"/audio.zip", want, dest, nil); err != nil {
			t.Fatal(err)
		}
		if ents, _ := os.ReadDir(dir); len(ents) != 1 || ents[0].Name() != "audio.zip" {
			t.Fatalf("want only the file, got %v", ents)
		}
	})
}
