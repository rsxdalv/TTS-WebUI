package react_ui_backend

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExtensionListIsReadOnly(t *testing.T) {
	f := newFakeCore(t)
	core := &Core{}
	core.Set(f.srv.URL, fakeToken)
	h := newExtensionList(core)
	for kind, want := range map[string]int{"interface": 200, "decorator": 200, "install": 404, "..": 404, "": 404} {
		r := httptest.NewRequest("GET", "/x", nil)
		r.SetPathValue("kind", kind)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != want {
			t.Errorf("%q: %d", kind, w.Code)
		}
	}
	got := f.requests()
	// The map above is walked in random order, so only the set is checked.
	all := strings.Join(got, "\n")
	if len(got) != 2 || !strings.Contains(all, "GET /api/extcatalog/list?kind=interface") || !strings.Contains(all, "GET /api/extcatalog/list?kind=decorator") {
		t.Fatalf("forwarded: %v", got)
	}
}
