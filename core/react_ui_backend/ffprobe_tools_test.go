package react_ui_backend

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// A new install has no env: ffprobe comes from the uv platform's tools provider.
func TestFindFFprobeInTheToolsProvider(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	root := t.TempDir()
	if got := findFFprobe(root); got != "" {
		t.Fatalf("found %q in an empty folder", got)
	}
	name := "ffprobe"
	if runtime.GOOS == "windows" {
		name = "ffprobe.exe"
	}
	dir := filepath.Join(root, "platforms", "uv", "tools", "ffmpeg", "bin")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, name)
	if err := os.WriteFile(want, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := findFFprobe(root); got != want {
		t.Fatalf("findFFprobe = %q, want %q", got, want)
	}
}
