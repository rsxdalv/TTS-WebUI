package launcher

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform/detect"
)

func hostStatusOf(t *testing.T, dir, token string) (int, HostStatus) {
	t.Helper()
	root, err := paths.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	p := detect.Detect(detect.Options{Root: root})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, hostStatusPath, nil)
	if token != "" {
		req.Header.Set("X-TTS-Core-Token", token)
	}
	hostStatusHandler(p, "tok", dir).ServeHTTP(rec, req)
	var st HostStatus
	_ = json.Unmarshal(rec.Body.Bytes(), &st)
	return rec.Code, st
}

func TestHostStatusNewFolderHasNoHost(t *testing.T) {
	dir := t.TempDir()
	launcherPorts.Store(dir, 7700)
	code, st := hostStatusOf(t, dir, "tok")
	if code != 200 || st.Installed || !st.Installable || st.Detail == "" || st.LauncherPort != 7700 {
		t.Fatalf("code %d status %+v", code, st)
	}
}

func TestHostStatusNeedsTheToken(t *testing.T) {
	if code, _ := hostStatusOf(t, t.TempDir(), "wrong"); code != http.StatusForbidden {
		t.Fatalf("wrong token: %d", code)
	}
	if code, _ := hostStatusOf(t, t.TempDir(), ""); code != http.StatusForbidden {
		t.Fatalf("no token: %d", code)
	}
}
