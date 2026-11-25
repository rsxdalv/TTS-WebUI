package audiocpp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
)

// The opt-in real run: AUDIOCPP_REAL_ROOT is an empty scratch folder (never an
// install). It downloads the pinned release from GitHub and Hugging Face.
//
//	AUDIOCPP_REAL_ROOT=C:\...\scratch go test ./internal/platform/audiocpp -run TestReal -v
func realRoot(t *testing.T) paths.Root {
	t.Helper()
	dir := os.Getenv("AUDIOCPP_REAL_ROOT")
	if dir == "" {
		t.Skip("AUDIOCPP_REAL_ROOT is not set")
	}
	if entries, _ := os.ReadDir(dir); len(entries) > 0 {
		t.Fatalf("%s is not empty: use an empty scratch folder", dir)
	}
	root, err := paths.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	return root
}

// A real download with a deliberately wrong pin is refused, nothing is left.
func TestRealWrongChecksumIsRefused(t *testing.T) {
	root := realRoot(t)
	a, err := ArchiveFor("windows", "amd64", RowCPU)
	if err != nil {
		t.Fatal(err)
	}
	a.SHA256 = strings.Repeat("0", 64)
	p := New(Options{Root: root, Archive: &a, GOOS: "windows", GOARCH: "amd64"})
	var lines []string
	err = p.Install(context.Background(), func(l string) { lines = append(lines, l) })
	t.Log(strings.Join(lines, "\n"))
	t.Log("error:", err)
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("a wrong checksum was not refused: %v", err)
	}
	if p.Status(context.Background()).Ready {
		t.Error("ready after a refused download")
	}
	for _, f := range []string{"bin", "audio.zip", "audio.zip.part", "platform.json"} {
		if _, err := os.Stat(filepath.Join(p.Dir(), f)); err == nil {
			t.Errorf("%s exists after a refused download", f)
		}
	}
}
