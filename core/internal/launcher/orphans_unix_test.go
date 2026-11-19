//go:build !windows

package launcher

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
)

func writeRecord(t *testing.T, root paths.Root, rec appRecord) {
	t.Helper()
	data, _ := json.Marshal(rec)
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
