//go:build !windows

package launcher

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
)

func writeRecord(t *testing.T, root paths.Root, rec appRecord) {
	t.Helper()
	data, _ := json.Marshal(rec)
	_ = os.MkdirAll(filepath.Dir(recordPath(root)), 0o755)
	if err := os.WriteFile(recordPath(root), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSweepIgnoresARecordOfAnotherRoot(t *testing.T) {
	root, _ := paths.New(t.TempDir())
	_ = os.MkdirAll(root.InstallerFiles(), 0o755)
	writeRecord(t, root, appRecord{Root: "/somewhere/else", Groups: []groupRecord{{Name: "x", Pgid: os.Getpid()}}})
	if n := sweepOrphans(root); n != 0 {
		t.Fatalf("stopped %d processes for a record of another root", n)
	}
}

// A new folder has no installer_files: the record is still written, and writing it
// does not make the folder look like an install with the old env.
func TestRecordOnANewFolderLeavesNoInstallerFiles(t *testing.T) {
	root, _ := paths.New(t.TempDir())
	recordGroups(root, []appProc{{Name: "x", PID: os.Getpid()}})
	if _, err := os.Stat(recordPath(root)); err != nil {
		t.Fatalf("the record was not written on a new folder: %v", err)
	}
	if _, err := os.Stat(root.InstallerFiles()); err == nil {
		t.Fatal("recording created installer_files")
	}
	if legacyInstall(root, false) {
		t.Fatal("a new folder looks like an old-env install after recording")
	}
}
