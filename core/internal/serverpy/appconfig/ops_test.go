package appconfig

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
)

func readDoc(t *testing.T, s *Store) map[string]any {
	t.Helper()
	b, err := os.ReadFile(s.path)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("file corrupt: %v\n%s", err, b)
	}
	return m
}

func list(t *testing.T, s *Store, key string) []any {
	t.Helper()
	ext, _ := readDoc(t, s)["extensions"].(map[string]any)
	l, _ := ext[key].([]any)
	return l
}

func TestExtensionDisableEnable(t *testing.T) {
	s, _ := newStore(t)
	if c, err := s.SetExtensionDisabled("extension_a", true); err != nil || !c {
		t.Fatalf("disable: %v %v", c, err)
	}
	if c, _ := s.SetExtensionDisabled("extension_a", true); c {
		t.Fatal("second disable must be a no-op")
	}
	s.SetExtensionDisabled("extension_b", true)
	if l := list(t, s, "disabled"); len(l) != 2 || l[0] != "extension_a" || l[1] != "extension_b" {
		t.Fatalf("disabled = %v", l)
	}
	if c, _ := s.SetExtensionDisabled("extension_a", false); !c {
		t.Fatal("enable should change")
	}
	if c, _ := s.SetExtensionDisabled("extension_a", false); c {
		t.Fatal("second enable must be a no-op")
	}
	if l := list(t, s, "disabled"); len(l) != 1 || l[0] != "extension_b" {
		t.Fatalf("disabled = %v", l)
	}
	// the rest of the document is untouched
	if readDoc(t, s)["gradio_interface_options"] == nil {
		t.Fatal("gradio_interface_options lost")
	}
}

func TestToggleAndAutostart(t *testing.T) {
	s, _ := newStore(t)
	for i, want := range []bool{true, false, true} {
		got, err := s.ToggleExtension("x.y-z")
		if err != nil || got != want {
			t.Fatalf("toggle %d: %v %v", i, got, err)
		}
	}
	if c, _ := s.SetExtensionAutostart("ext", true); !c {
		t.Fatal("autostart add")
	}
	if c, _ := s.SetExtensionAutostart("ext", true); c {
		t.Fatal("autostart add twice")
	}
	if l := list(t, s, "autostart"); len(l) != 1 {
		t.Fatalf("autostart = %v", l)
	}
	if c, _ := s.SetExtensionAutostart("ext", false); !c {
		t.Fatal("autostart remove")
	}
	if c, _ := s.SetExtensionAutostart("ext", false); c {
		t.Fatal("autostart remove twice")
	}
	if l := list(t, s, "autostart"); len(l) != 0 {
		t.Fatalf("autostart = %v", l)
	}
}

// Extension names change the lists only; bytes stay json.dump(indent=2).
func TestOpsBytes(t *testing.T) {
	s, _ := newStore(t)
	in := "{\n  \"a\": 1.0,\n  \"extensions\": {\n    \"disabled\": [\n      \"é\"\n    ],\n    \"other\": 1e-7\n  }\n}"
	if err := s.Save([]byte(in)); err != nil {
		t.Fatal(err)
	}
	s.SetExtensionDisabled("n", true)
	got, _ := os.ReadFile(s.path)
	want := "{\n  \"a\": 1.0,\n  \"extensions\": {\n    \"disabled\": [\n      \"\\u00e9\",\n      \"n\"\n    ],\n    \"other\": 1e-7\n  }\n}"
	if string(got) != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
	// a missing list is created and the key order of the section is kept
	s2, _ := newStore(t)
	s2.Save([]byte(`{"extensions": {"disabled": []}}`))
	s2.SetExtensionAutostart("n", true)
	got, _ = os.ReadFile(s2.path)
	if string(got) != "{\n  \"extensions\": {\n    \"disabled\": [],\n    \"autostart\": [\n      \"n\"\n    ]\n  }\n}" {
		t.Fatalf("got\n%s", got)
	}
}

func TestSetValueAndReplaceSection(t *testing.T) {
	s, logs := newStore(t)
	if err := s.SetValue("extension_openai_tts_api", "port", json.Number("7778")); err != nil {
		t.Fatal(err)
	}
	ns := readDoc(t, s)["extension_openai_tts_api"].(map[string]any)
	if ns["port"] != float64(7778) {
		t.Fatalf("ns = %v", ns)
	}
	if err := s.SetValue("gradio_interface_options", "auth", "user:SECRET"); err != nil {
		t.Fatal(err)
	}
	vals, _ := parse([]byte(`{"server_port": 7771, "auth": null, "server_name": "0.0.0.0"}`))
	if err := s.ReplaceSection("gradio_interface_options", vals.(*object)); err != nil {
		t.Fatal(err)
	}
	g := readDoc(t, s)["gradio_interface_options"].(map[string]any)
	if len(g) != 3 || g["server_port"] != float64(7771) {
		t.Fatalf("section = %v", g)
	}
	// launch-time changes are logged by name only; never a value
	all := strings.Join(*logs, "\n")
	if !strings.Contains(all, "server_port") || strings.Contains(all, "SECRET") || strings.Contains(all, "0.0.0.0") {
		t.Fatalf("logs:\n%s", all)
	}
	// a bad server_port is refused and the file is untouched
	before, _ := os.ReadFile(s.path)
	if err := s.SetValue("gradio_interface_options", "server_port", "7771"); err == nil {
		t.Fatal("string server_port accepted")
	}
	after, _ := os.ReadFile(s.path)
	if string(before) != string(after) {
		t.Fatal("file changed by a refused operation")
	}
}

func TestOpsInvalid(t *testing.T) {
	s, _ := newStore(t)
	for _, n := range []string{"", "a b", "../x", "a/b", "-x", strings.Repeat("a", 201), "é"} {
		if _, err := s.SetExtensionDisabled(n, true); err == nil {
			t.Errorf("package %q accepted", n)
		}
		if _, err := s.ToggleExtension(n); err == nil {
			t.Errorf("toggle %q accepted", n)
		}
	}
	if err := s.SetValue("", "k", 1); err == nil {
		t.Error("empty namespace accepted")
	}
	if err := s.SetValue("ns", "k\n", 1); err == nil {
		t.Error("control char key accepted")
	}
	// wrong shapes in the file are refused, not overwritten
	s.Save([]byte(`{"extensions": []}`))
	if _, err := s.SetExtensionDisabled("a", true); err == nil {
		t.Error("non-object extensions accepted")
	}
	s.Save([]byte(`{"extensions": {"disabled": "oops"}}`))
	if _, err := s.SetExtensionDisabled("a", true); err == nil {
		t.Error("non-list disabled accepted")
	}
	if got := readDoc(t, s); got["extensions"].(map[string]any)["disabled"] != "oops" {
		t.Errorf("file overwritten: %v", got)
	}
}

func TestConcurrentToggles(t *testing.T) {
	s, _ := newStore(t)
	var wg sync.WaitGroup
	const n = 40
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := fmt.Sprintf("ext_%d", i)
			if _, err := s.SetExtensionDisabled(name, true); err != nil {
				t.Error(err)
			}
			if _, err := s.SetExtensionAutostart(name, true); err != nil {
				t.Error(err)
			}
			if err := s.SetValue("ns", name, json.Number("1")); err != nil {
				t.Error(err)
			}
			if i%2 == 0 {
				s.SetExtensionDisabled(name, false)
			}
		}(i)
	}
	wg.Wait()
	if l := list(t, s, "disabled"); len(l) != n/2 {
		t.Fatalf("disabled has %d entries, want %d: %v", len(l), n/2, l)
	}
	if l := list(t, s, "autostart"); len(l) != n {
		t.Fatalf("autostart has %d entries, want %d", len(l), n)
	}
	if ns := readDoc(t, s)["ns"].(map[string]any); len(ns) != n {
		t.Fatalf("ns has %d keys, want %d", len(ns), n)
	}
}

func TestOpsHTTP(t *testing.T) {
	s, _ := newStore(t)
	srv := httptest.NewServer(s.OpsHandler())
	defer srv.Close()
	do := func(method, op, body, token, ctype string) (int, map[string]any) {
		req, _ := http.NewRequest(method, srv.URL+OpsPrefix+op, strings.NewReader(body))
		if token != "" {
			req.Header.Set(TokenHeader, token)
		}
		if ctype != "" {
			req.Header.Set("Content-Type", ctype)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		var m map[string]any
		_ = json.Unmarshal(b, &m)
		return resp.StatusCode, m
	}
	const js = "application/json"
	if c, _ := do("POST", "extension-disable", `{"package_name":"a"}`, "", js); c != 403 {
		t.Fatalf("no token: %d", c)
	}
	if c, _ := do("POST", "nope", `{}`, "tok", js); c != 404 {
		t.Fatalf("unknown op: %d", c)
	}
	if c, _ := do("GET", "extension-disable", ``, "tok", js); c != 405 {
		t.Fatalf("GET: %d", c)
	}
	if c, _ := do("POST", "extension-disable", `{}`, "tok", "text/plain"); c != 415 {
		t.Fatalf("ctype: %d", c)
	}
	for _, bad := range []string{`[1]`, `{}`, `{"package_name": 3}`, `{"package_name":"a b"}`, `{"package_name":"a","x":1}`} {
		if c, m := do("POST", "extension-disable", bad, "tok", js); c != 400 || m["error"] == nil {
			t.Fatalf("%s: %d %v", bad, c, m)
		}
	}
	c, m := do("POST", "extension-disable", `{"package_name":"a"}`, "tok", js)
	if c != 200 || m["changed"] != true || m["disabled"] != true || !strings.Contains(m["info"].(string), "Disabled extension") {
		t.Fatalf("disable: %d %v", c, m)
	}
	c, m = do("POST", "extension-toggle", `{"package_name":"a"}`, "tok", js)
	if c != 200 || m["disabled"] != false || !strings.Contains(m["info"].(string), "Enabled extension") {
		t.Fatalf("toggle: %d %v", c, m)
	}
	if c, m := do("POST", "extension-autostart", `{"package_name":"a","enabled":"yes"}`, "tok", js); c != 400 {
		t.Fatalf("autostart enabled string: %d %v", c, m)
	}
	if c, m := do("POST", "extension-autostart", `{"package_name":"a","enabled":true}`, "tok", js); c != 200 || m["changed"] != true {
		t.Fatalf("autostart: %d %v", c, m)
	}
	if c, _ := do("POST", "set-value", `{"namespace":"n","key":"k","value":{"z":1,"a":[1,2]}}`, "tok", js); c != 200 {
		t.Fatalf("set-value: %d", c)
	}
	if c, _ := do("POST", "set-value", `{"namespace":"n","key":"k"}`, "tok", js); c != 400 {
		t.Fatalf("set-value without value: %d", c)
	}
	if c, _ := do("POST", "replace-section", `{"namespace":"n","values":[1]}`, "tok", js); c != 400 {
		t.Fatalf("replace-section array: %d", c)
	}
	if c, _ := do("POST", "replace-section", `{"namespace":"n","values":{"k":"é"}}`, "tok", js); c != 200 {
		t.Fatalf("replace-section: %d", c)
	}
	b, _ := os.ReadFile(s.path)
	if !strings.Contains(string(b), "\"n\": {\n    \"k\": \"\\u00e9\"\n  }") {
		t.Fatalf("file:\n%s", b)
	}
}
