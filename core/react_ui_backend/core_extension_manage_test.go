package react_ui_backend

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExtensionManageForwardsOnlyWhatItShould(t *testing.T) {
	f := newFakeCore(t)
	core := &Core{}
	core.Set(f.srv.URL, fakeToken)
	h := newExtensionManage(core)

	call := func(path, pkg, op, host, ct, body string) int {
		r := httptest.NewRequest("POST", "http://"+host+path, strings.NewReader(body))
		if pkg != "" {
			r.SetPathValue("pkg", pkg)
		}
		if op != "" {
			r.SetPathValue("op", op)
		}
		if ct != "" {
			r.Header.Set("Content-Type", ct)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}
	const js, k = "application/json", "tts_webui_extension.kokoro"
	for name, c := range map[string]struct {
		path, pkg, op, host, ct, body string
		want                          int
	}{
		"stop":              {"/api/go-data/engines/" + k + "/stop", k, "", "127.0.0.1:3000", js, `{}`, 200},
		"stop bad name":     {"/api/go-data/engines/x/stop", "../x", "", "127.0.0.1:3000", js, `{}`, 400},
		"stop lan":          {"/api/go-data/engines/" + k + "/stop", k, "", "192.168.1.5:3000", js, `{}`, 403},
		"stop form":         {"/api/go-data/engines/" + k + "/stop", k, "", "127.0.0.1:3000", "text/plain", `{}`, 415},
		"enable":            {"/api/go-data/extension-config/enable", "", "enable", "127.0.0.1:3000", js, `{"package_name":"` + k + `"}`, 200},
		"disable":           {"/api/go-data/extension-config/disable", "", "disable", "localhost:3000", js, `{"package_name":"` + k + `"}`, 200},
		"autostart on":      {"/api/go-data/extension-config/autostart", "", "autostart", "127.0.0.1:3000", js, `{"package_name":"` + k + `","enabled":true}`, 200},
		"autostart missing": {"/api/go-data/extension-config/autostart", "", "autostart", "127.0.0.1:3000", js, `{"package_name":"` + k + `"}`, 400},
		"enable with flag":  {"/api/go-data/extension-config/enable", "", "enable", "127.0.0.1:3000", js, `{"package_name":"` + k + `","enabled":false}`, 400},
		"other op":          {"/api/go-data/extension-config/set-value", "", "set-value", "127.0.0.1:3000", js, `{"package_name":"x"}`, 404},
		"extra field":       {"/api/go-data/extension-config/enable", "", "enable", "127.0.0.1:3000", js, `{"package_name":"` + k + `","namespace":"x"}`, 400},
		"lan config":        {"/api/go-data/extension-config/enable", "", "enable", "192.168.1.5:3000", js, `{"package_name":"` + k + `"}`, 403},
	} {
		if got := call(c.path, c.pkg, c.op, c.host, c.ct, c.body); got != c.want {
			t.Errorf("%s: %d, want %d", name, got, c.want)
		}
	}
	got := strings.Join(f.requests(), "\n")
	for _, want := range []string{
		"POST /api/extensions/" + k + "/stop {}",
		`POST /api/config/extension-enable {"package_name":"` + k + `"}`,
		`POST /api/config/extension-disable {"package_name":"` + k + `"}`,
		`POST /api/config/extension-autostart {"enabled":true,"package_name":"` + k + `"}`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if n := len(f.requests()); n != 4 {
		t.Errorf("%d requests reached the control API, want 4:\n%s", n, got)
	}
}
