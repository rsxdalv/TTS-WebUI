package react_ui_backend

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHostStatusRouteForwardsOnlyTheStatus(t *testing.T) {
	f := newFakeCore(t)
	core := &Core{}
	core.Set(f.srv.URL, fakeToken)
	h, err := NewHandler(Config{Root: exportFixture(t), Port: testPort, Routes: routes(t, "core-host"), Core: core})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	if r := request(t, "GET", srv.URL+"/api/go-data/host-status", nil, nil); r.StatusCode != 200 {
		t.Fatalf("status: %d", r.StatusCode)
	}
	if got := f.requests(); len(got) != 1 || !strings.HasPrefix(got[0], "GET /api/host/status ") {
		t.Fatalf("core saw %v", got)
	}
	// Read only: no other method reaches the core.
	if r := request(t, "POST", srv.URL+"/api/go-data/host-status", strings.NewReader("{}"), jsonHdr); r.StatusCode == 200 {
		t.Fatalf("POST answered 200")
	}
	if len(f.requests()) != 1 {
		t.Fatalf("a POST reached the core: %v", f.requests())
	}
}
