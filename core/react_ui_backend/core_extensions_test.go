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
	if len(got) != 2 || !strings.HasPrefix(got[0], "GET /api/extcatalog/list?kind=interface") {
		t.Fatalf("forwarded: %v", got)
	}
}
