package launcher_test

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

const extList = `{"tabs":[
 {"package_name":"extensions.builtin.alpha","name":"Alpha","description":"d","website":"w","extension_type":"interface","extension_class":"tools"},
 {"package_name":"extension_beta","name":"Beta","description":"d","website":"w","extension_type":"interface","extension_class":"tools","requirements":"extension_beta"},
 {"package_name":"extension_native","name":"Native","description":"d","website":"w","extension_type":"interface","extension_class":"text-to-speech","proxy":"native","requirements":"x"}
],"decorators":[{"package_name":"extension_deco","name":"Deco","description":"d","website":"w","extension_type":"decorator","extension_class":"outer"}]}`

func TestExtensionsListEnableAndOps(t *testing.T) {
	root, srv, token := newTestInstall(t)
	dir := root.Dir()
	if err := os.WriteFile(filepath.Join(dir, "extensions.json"), []byte(extList), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".venvs", "extension_native"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Disable Beta in config.json, then list: the page shows it disabled.
	resp := doRequest(t, srv, token, http.MethodPost, "/api/ext/enabled", `{"package_name":"extension_beta","enabled":false}`)
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("enabled: %d %s", resp.StatusCode, b)
	}
	cfg, _ := os.ReadFile(filepath.Join(dir, "config.json"))
	var doc struct {
		Extensions struct {
			Disabled []string `json:"disabled"`
		} `json:"extensions"`
	}
	if err := json.Unmarshal(cfg, &doc); err != nil || len(doc.Extensions.Disabled) != 1 || doc.Extensions.Disabled[0] != "extension_beta" {
		t.Fatalf("config.json = %s (%v)", cfg, err)
	}

	resp = doRequest(t, srv, token, http.MethodGet, "/api/ext/list", "")
	if resp.StatusCode != 200 {
		t.Fatalf("list: %d", resp.StatusCode)
	}
	var ans struct {
		Extensions []struct {
			PackageName string `json:"package_name"`
			Kind        string `json:"kind"`
			Builtin     bool   `json:"builtin"`
			Installed   bool   `json:"installed"`
			Disabled    bool   `json:"disabled"`
		} `json:"extensions"`
		AppRunning bool   `json:"app_running"`
		Warning    string `json:"warning"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&ans); err != nil {
		t.Fatal(err)
	}
	got := map[string]int{}
	for i, e := range ans.Extensions {
		got[e.PackageName] = i
	}
	byName := func(n string) int {
		i, ok := got[n]
		if !ok {
			t.Fatalf("%s missing from %+v", n, ans.Extensions)
		}
		return i
	}
	if e := ans.Extensions[byName("extensions.builtin.alpha")]; !e.Builtin || !e.Installed {
		t.Errorf("builtin: %+v", e)
	}
	if e := ans.Extensions[byName("extension_native")]; !e.Installed {
		t.Errorf("native with a venv should be installed: %+v", e)
	}
	if e := ans.Extensions[byName("extension_beta")]; e.Installed || !e.Disabled {
		t.Errorf("beta: %+v", e)
	}
	if e := ans.Extensions[byName("extension_deco")]; e.Kind != "decorator" || e.Disabled {
		t.Errorf("decorator: %+v", e)
	}
	// There is no micromamba in the temp root, so pip cannot answer; the page is told.
	if ans.Warning == "" || ans.AppRunning {
		t.Errorf("warning=%q appRunning=%v", ans.Warning, ans.AppRunning)
	}

	// Enable again removes it.
	if resp := doRequest(t, srv, token, http.MethodPost, "/api/ext/enabled", `{"package_name":"extension_beta","enabled":true}`); resp.StatusCode != 200 {
		t.Fatalf("enable: %d", resp.StatusCode)
	}
	// A bad name is a 400, an unknown field too.
	for _, body := range []string{`{"package_name":"a b","enabled":true}`, `{"package_name":"x","enabled":true,"x":1}`} {
		if resp := doRequest(t, srv, token, http.MethodPost, "/api/ext/enabled", body); resp.StatusCode != 400 {
			t.Errorf("%s: want 400, got %d", body, resp.StatusCode)
		}
	}

	// The op adapter reaches extcatalog (it answers 400 for a missing package,
	// not 403 for a missing token), and refuses ops it does not mount.
	if resp := doRequest(t, srv, token, http.MethodPost, "/api/ext/op/uninstall", `{"package_name":"extensions.builtin.alpha"}`); resp.StatusCode == http.StatusForbidden {
		t.Errorf("uninstall through the adapter was refused as forbidden")
	}
	if resp := doRequest(t, srv, token, http.MethodPost, "/api/ext/op/install-entry", `{}`); resp.StatusCode != 404 {
		t.Errorf("install-entry is not mounted: got %d", resp.StatusCode)
	}
	// And without the launcher's token nothing is served.
	if resp := doRequest(t, srv, "wrong", http.MethodGet, "/api/ext/list", ""); resp.StatusCode != 403 {
		t.Errorf("wrong token: got %d", resp.StatusCode)
	}
}
