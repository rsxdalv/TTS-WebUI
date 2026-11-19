package launcher

import (
	"archive/zip"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// The Go Download keeps the permission bits of zip entries made on Unix, never
// more than 0777, and gives everything else 0644 (R2-2).
func TestExtractKeepsZipPermissionBits(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits are a Linux and macOS matter")
	}
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "r.zip")
	zf, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(zf)
	add := func(name string, mode os.FileMode, unix bool) {
		h := &zip.FileHeader{Name: name, Method: zip.Deflate}
		if unix {
			h.SetMode(mode)
		}
		w, err := zw.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte("x"))
	}
	add("start_tts_webui.sh", 0o755, true)
	add("installer_scripts/init_app.sh", 0o755, true)
	add("README.md", 0o644, true)
	add("suid.sh", os.ModeSetuid|0o755, true)
	add("windows-made.txt", 0, false)
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	zf.Close()

	dest := filepath.Join(dir, "out")
	if err := extractZip(context.Background(), zipPath, dest, newLog()); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]os.FileMode{
		"start_tts_webui.sh": 0o755, "installer_scripts/init_app.sh": 0o755, "README.md": 0o644,
		"suid.sh": 0o755, "windows-made.txt": 0o644,
	} {
		info, err := os.Stat(filepath.Join(dest, filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode() & (os.ModePerm | os.ModeSetuid | os.ModeSetgid | os.ModeSticky); got != want {
			t.Errorf("%s: mode %v, want %v", name, got, want)
		}
	}
}

// Before the Go Update pulls, files git tracks as 100755 get their exec bit
// back, and nothing else changes (R2-2, trees extracted before the fix).
func TestRestoreExecBits(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("exec bits are a Linux and macOS matter")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	dir := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@t", "-c", "core.filemode=true"}, args...)...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-q")
	git("config", "core.filemode", "true")
	for name, mode := range map[string]os.FileMode{"run.sh": 0o755, "sub/tool.sh": 0o755, "notes.txt": 0o644} {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), mode); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(p, mode); err != nil {
			t.Fatal(err)
		}
	}
	git("add", ".")
	git("commit", "-q", "-m", "c1")
	for _, name := range []string{"run.sh", "sub/tool.sh"} {
		if err := os.Chmod(filepath.Join(dir, filepath.FromSlash(name)), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	fixed, err := restoreExecBits(context.Background(), dir, defaultGitRunner)
	if err != nil {
		t.Fatal(err)
	}
	if len(fixed) != 2 {
		t.Fatalf("fixed %q, want run.sh and sub/tool.sh", fixed)
	}
	for name, want := range map[string]os.FileMode{"run.sh": 0o755, "sub/tool.sh": 0o755, "notes.txt": 0o644} {
		info, _ := os.Stat(filepath.Join(dir, filepath.FromSlash(name)))
		if info.Mode().Perm() != want {
			t.Errorf("%s: %v, want %v", name, info.Mode().Perm(), want)
		}
	}
	if again, _ := restoreExecBits(context.Background(), dir, defaultGitRunner); len(again) != 0 {
		t.Errorf("second run changed %q", again)
	}
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = dir
	if out, _ := cmd.Output(); len(out) != 0 {
		t.Errorf("git status not clean after the repair:\n%s", out)
	}
}
