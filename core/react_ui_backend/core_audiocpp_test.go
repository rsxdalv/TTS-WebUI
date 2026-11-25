package react_ui_backend

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform/audiocpp"
)

// The chain with no platform installed: the browser routes reach the control API,
// the answers say what is missing, and nothing the browser sends can stop the
// server or reach a route that is not claimed.
func TestAudioCppRoutesAnswerWithNothingInstalled(t *testing.T) {
	root := exportFixture(t)
	pr, _ := paths.New(root)
	ctl := httptest.NewServer(audiocpp.NewHandler(pr, fakeToken, nil))
	t.Cleanup(ctl.Close)
	core := &Core{}
	core.Set(ctl.URL, fakeToken)
	h, err := NewHandler(Config{Root: root, Port: testPort, Routes: routes(t, "core-audiocpp"), Core: core})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	call := func(method, path, body string, hdr map[string]string, want int) string {
		t.Helper()
		var rd io.Reader
		if body != "" {
			rd = strings.NewReader(body)
		}
		r := request(t, method, srv.URL+path, rd, hdr)
		b, _ := io.ReadAll(r.Body)
		r.Body.Close()
		if r.StatusCode != want {
			t.Fatalf("%s %s: %d, want %d: %s", method, path, r.StatusCode, want, b)
		}
		return string(b)
	}
	st := call("GET", "/api/go-data/audiocpp/status", "", nil, 200)
	for _, want := range []string{`"ready":false`, `"id":"pocket-tts-en"`, `"installed":false`, `"rows"`, `"running":false`} {
		if !strings.Contains(st, want) {
			t.Errorf("status lacks %s: %s", want, st)
		}
	}
	if sp := call("POST", "/api/go-data/audiocpp/speech", `{"model":"pocket-tts-en","input":"hello"}`, jsonHdr, 502); !strings.Contains(sp, "not installed") {
		t.Errorf("speech: %s", sp)
	}
	if c := call("POST", "/api/go-data/audiocpp/cancel", `{}`, jsonHdr, 200); !strings.Contains(c, `"cancelled":false`) {
		t.Errorf("cancel: %s", c)
	}
	// A form post cannot reach the generate route (CSRF rule of the front door), and
	// the server cannot be stopped or the platform changed from the browser.
	call("POST", "/api/go-data/audiocpp/speech", `model=x`, map[string]string{"Content-Type": "application/x-www-form-urlencoded"}, 415)
	for _, p := range []string{"/api/go-data/audiocpp/stop", "/api/go-data/audiocpp/install"} {
		r := request(t, http.MethodPost, srv.URL+p, strings.NewReader(`{}`), jsonHdr)
		r.Body.Close()
		if r.StatusCode == 200 {
			t.Errorf("%s answered 200", p)
		}
	}
}
