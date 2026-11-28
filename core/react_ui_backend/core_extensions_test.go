package react_ui_backend

import (
	"net/http"
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

func TestExtensionStateMergesResolveOfEveryClass(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(coreTokenHeader) != fakeToken {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.RequestURI() {
		case "/api/extcatalog/list?kind=interface":
			_, _ = w.Write([]byte(`[{"extension_class":"text-to-speech"},{"extension_class":"tools"},{"extension_class":"text-to-speech"}]`))
		case "/api/extcatalog/resolve?kind=interface&extension_class=text-to-speech":
			_, _ = w.Write([]byte(`{"extensions":[{"package_name":"a.kokoro","enabled":true,"autostart":true,"installed":true,"proxy":"native"}]}`))
		case "/api/extcatalog/resolve?kind=interface&extension_class=tools":
			_, _ = w.Write([]byte(`{"extensions":[{"package_name":"b.tool","enabled":false,"installed":false}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	core := &Core{}
	core.Set(srv.URL, fakeToken)

	r := httptest.NewRequest("GET", "/x", nil)
	r.SetPathValue("kind", "interface")
	w := httptest.NewRecorder()
	newExtensionState(core).ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	got := strings.TrimSpace(w.Body.String())
	want := `{"a.kokoro":{"installed":true,"enabled":true,"autostart":true,"proxy":"native"},"b.tool":{"installed":false,"enabled":false,"autostart":false}}`
	if got != want {
		t.Errorf("got  %s, want %s", got, want)
	}

	r = httptest.NewRequest("GET", "/x", nil)
	r.SetPathValue("kind", "bogus")
	w = httptest.NewRecorder()
	newExtensionState(core).ServeHTTP(w, r)
	if w.Code != 404 {
		t.Errorf("bogus kind: %d", w.Code)
	}
}
