package launcher

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
)

func lanStatusFor(t *testing.T) lanStatus {
	t.Helper()
	root, err := paths.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	New(root).handleAppLAN().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/app/lan", nil))
	var st lanStatus
	if err := json.NewDecoder(rec.Body).Decode(&st); err != nil {
		t.Fatal(err)
	}
	return st
}

// The launcher UI gets the LAN warning only in LAN mode, with the same text the
// launcher logs (R2-8).
func TestAppLANStatus(t *testing.T) {
	t.Setenv("TTS_GO_PROXY", "")
	t.Setenv("TTS_GO_PROXY_LISTEN", "")
	if st := lanStatusFor(t); st.LAN || st.Warning != "" {
		t.Fatalf("default: %+v, want no LAN", st)
	}
	t.Setenv("TTS_GO_PROXY_LISTEN", "lan")
	st := lanStatusFor(t)
	if !st.LAN || !strings.Contains(st.Warning, "no password") || !strings.Contains(st.Warning, "reachable from the network") {
		t.Fatalf("lan: %+v", st)
	}
	t.Setenv("TTS_GO_PROXY_LISTEN", "lna")
	if st := lanStatusFor(t); st.LAN {
		t.Fatalf("invalid value: %+v, want no LAN", st)
	}
}
