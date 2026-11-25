package uv

import (
	"strings"
	"testing"

	"github.com/rsxdalv/TTS-WebUI/core/internal/torchrows"
)

func TestCheckPathLength(t *testing.T) {
	short := `C:\tts\.venvs\tts_webui_extension.kitten_tts`
	if err := checkPathLength("windows", short); err != nil {
		t.Fatal(err)
	}
	deep := `C:\` + strings.Repeat("a", 90) + `\.venvs\tts_webui_extension.kitten_tts`
	err := checkPathLength("windows", deep)
	if err == nil || !strings.Contains(err.Error(), "shorter folder") {
		t.Fatalf("deep path: %v", err)
	}
	if err := checkPathLength("linux", deep); err != nil {
		t.Fatal("the limit is Windows only")
	}
}

func TestCheckDisk(t *testing.T) {
	row := torchrows.Row{Name: "cu128", ApproxMB: 3500}
	if got := needMB(row, false); got != 8000 {
		t.Fatalf("need %d", got)
	}
	if got := needMB(row, true); got != restMB {
		t.Fatalf("shared need %d", got)
	}
	if err := checkDisk(5000<<20, true, row, false); err == nil || !strings.Contains(err.Error(), "not enough free disk space") {
		t.Fatalf("5 GB free for cu128: %v", err)
	}
	if err := checkDisk(5000<<20, true, row, true); err != nil {
		t.Fatalf("5 GB free with the row shared: %v", err)
	}
	if err := checkDisk(0, false, row, false); err != nil {
		t.Fatal("an unreadable free space must not refuse the install")
	}
}
