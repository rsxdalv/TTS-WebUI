package react_ui_backend

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExtensionInstallForwardsOnlyThePackageName(t *testing.T) {
	f := newFakeCore(t)
	core := &Core{}
	core.Set(f.srv.URL, fakeToken)
	h := newExtensionInstall(core)

	call := func(op, host, ct, body string) int {
		r := httptest.NewRequest("POST", "http://"+host+"/api/go-data/extensions/"+op, strings.NewReader(body))
		r.SetPathValue("op", op)
		if ct != "" {
			r.Header.Set("Content-Type", ct)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}
	const js = "application/json"
	for name, c := range map[string]struct {
		op, host, ct, body string
		want               int
	}{
		"install":              {"install", "127.0.0.1:3000", js, `{"package_name":"tts_webui_extension.kokoro"}`, 200},
		"uninstall":            {"uninstall", "localhost:7770", js, `{"package_name":"tts_webui_extension.kokoro"}`, 200},
		"install-entry":        {"install-entry", "127.0.0.1:3000", js, `{"package_name":"x"}`, 404},
		"check-dependencies":   {"check-dependencies", "127.0.0.1:3000", js, `{"package_name":"x"}`, 404},
		"lan host":             {"install", "192.168.1.5:3000", js, `{"package_name":"x"}`, 403},
		"form post":            {"install", "127.0.0.1:3000", "application/x-www-form-urlencoded", `package_name=x`, 415},
		"no body":              {"install", "127.0.0.1:3000", js, ``, 400},
		"requirements smuggle": {"install", "127.0.0.1:3000", js, `{"package_name":"x","requirements":"evil"}`, 400},
		"gpu choice refused":   {"install", "127.0.0.1:3000", js, `{"package_name":"x","gpu":"CPU"}`, 400},
	} {
		if got := call(c.op, c.host, c.ct, c.body); got != c.want {
			t.Errorf("%s: %d, want %d", name, got, c.want)
		}
	}
	got := strings.Join(f.requests(), "\n")
	if !strings.Contains(got, `POST /api/extcatalog/install {"package_name":"tts_webui_extension.kokoro"}`) ||
		!strings.Contains(got, `POST /api/extcatalog/uninstall {"package_name":"tts_webui_extension.kokoro"}`) {
		t.Fatalf("forwarded:\n%s", got)
	}
	if n := len(f.requests()); n != 2 {
		t.Fatalf("%d requests reached the control API, want 2:\n%s", n, got)
	}
}
