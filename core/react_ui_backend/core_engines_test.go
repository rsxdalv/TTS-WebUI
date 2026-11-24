package react_ui_backend

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEngineRoutesForwardOnlyOpenAndState(t *testing.T) {
	f := newFakeCore(t)
	core := &Core{}
	core.Set(f.srv.URL, fakeToken)
	h, err := NewHandler(Config{Root: exportFixture(t), Port: testPort, Routes: routes(t, "core-engines"), Core: core})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	front := srv.URL
	const pkg = "tts_webui_extension.piper_tts"

	if r := request(t, "GET", front+"/api/go-data/engines/"+pkg, nil, nil); r.StatusCode != 200 {
		t.Fatalf("state: %d", r.StatusCode)
	}
	// Opening needs a JSON content type (the CSRF rule) and another site's origin is refused.
	if r := request(t, "POST", front+"/api/go-data/engines/"+pkg+"/open", strings.NewReader(""), map[string]string{"Content-Type": "text/plain"}); r.StatusCode != 415 {
		t.Fatalf("text/plain: %d", r.StatusCode)
	}
	if r := request(t, "POST", front+"/api/go-data/engines/"+pkg+"/open", strings.NewReader("{}"), map[string]string{"Content-Type": "application/json", "Origin": "http://evil.example"}); r.StatusCode != 403 {
		t.Fatalf("other origin: %d", r.StatusCode)
	}
	if r := request(t, "POST", front+"/api/go-data/engines/"+pkg+"/open", strings.NewReader("{}"), jsonHdr); r.StatusCode != 200 {
		t.Fatalf("open: %d", r.StatusCode)
	}
	// Nothing but those two reaches the core, and a bad name does not either.
	for _, c := range [][2]string{{"POST", "/api/go-data/engines/" + pkg + "/stop"}, {"POST", "/api/go-data/engines/" + pkg + "/start"}, {"GET", "/api/go-data/engines/a..b"}, {"POST", "/api/go-data/engines/a..b/open"}} {
		if r := request(t, c[0], front+c[1], strings.NewReader("{}"), jsonHdr); r.StatusCode == 200 {
			t.Errorf("%v answered 200", c)
		}
	}
	got := f.requests()
	if len(got) != 2 || !strings.HasPrefix(got[0], "GET /api/engines/"+pkg+" ") || !strings.HasPrefix(got[1], "POST /api/engines/"+pkg+"/open ") {
		t.Fatalf("forwarded: %v", got)
	}
}
