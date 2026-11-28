package react_ui_backend

import (
	"io"
	"net/http"
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

func TestCustomEntriesAreParsedByGoBeforeTheyAreAdded(t *testing.T) {
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		seen = append(seen, r.Method+" "+r.URL.Path+" "+string(b))
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/extcatalog/parse":
			if strings.Contains(string(b), "good") {
				_, _ = w.Write([]byte(`{"entries":[{"package_name":"ext.one","name":"One"}],"preview":{},"info":"Parsed entries: 1"}`))
				return
			}
			_, _ = w.Write([]byte(`{"entries":[],"preview":{},"info":"JSON parse error: nope"}`))
		case "/api/extcatalog/add":
			_, _ = w.Write([]byte(`{"status":"Added: ext.one","json":"{}"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	core := &Core{}
	core.Set(srv.URL, fakeToken)
	h := newExtensionInstall(core)

	call := func(op, ct, body string) (int, string) {
		r := httptest.NewRequest("POST", "http://127.0.0.1:3000/api/go-data/extensions/"+op, strings.NewReader(body))
		r.SetPathValue("op", op)
		r.Header.Set("Content-Type", ct)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code, w.Body.String()
	}
	const js = "application/json"

	if code, body := call("parse", js, `{"text":"good"}`); code != 200 || !strings.Contains(body, `"ext.one"`) {
		t.Errorf("parse: %d %s", code, body)
	}
	if len(seen) != 1 || strings.Contains(strings.Join(seen, "\n"), "/add") {
		t.Fatalf("parse must not write: %v", seen)
	}

	seen = nil
	if code, body := call("add", js, `{"text":"good"}`); code != 200 || !strings.Contains(body, "Added: ext.one") {
		t.Errorf("add: %d %s", code, body)
	}
	if len(seen) != 2 || !strings.Contains(seen[0], "/parse") || !strings.Contains(seen[1], `/add {"entries":[{"package_name":"ext.one","name":"One"}]}`) {
		t.Errorf("add forwarded: %v", seen)
	}

	seen = nil
	if code, _ := call("add", js, `{"text":"bad"}`); code != 400 {
		t.Errorf("add of invalid text: %d", code)
	}
	if strings.Contains(strings.Join(seen, "\n"), "/add") {
		t.Errorf("an invalid text reached add: %v", seen)
	}

	seen = nil
	for name, c := range map[string]struct{ op, body string }{
		"entries from the browser": {"add", `{"entries":[{"package_name":"x","requirements":"evil"}]}`},
		"extra field":              {"add", `{"text":"good","entries":[]}`},
		"parse with entries":       {"parse", `{"entries":[]}`},
	} {
		if code, _ := call(c.op, js, c.body); code != 400 {
			t.Errorf("%s: %d, want 400", name, code)
		}
	}
	if len(seen) != 0 {
		t.Errorf("a refused request reached the control API: %v", seen)
	}
}
