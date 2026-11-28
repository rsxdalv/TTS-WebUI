package launcher

import (
	"testing"

	"github.com/rsxdalv/TTS-WebUI/core/internal/preflight"
)

func TestWithoutLegacyChecks(t *testing.T) {
	in := []preflight.Check{{ID: "SpacesInPath"}, {ID: "ConflictingConda"}, {ID: "MSVC"}}
	got := withoutLegacyChecks(in)
	if len(got) != 2 || got[0].ID != "SpacesInPath" || got[1].ID != "MSVC" {
		t.Fatalf("got %+v", got)
	}
	if len(in) != 3 {
		t.Fatal("the input slice was changed")
	}
}
