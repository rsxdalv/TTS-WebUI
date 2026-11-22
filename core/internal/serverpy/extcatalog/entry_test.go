package extcatalog

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// The Extension Catalog tab (extensions/builtin/extension_extension_catalog)
// hands Go the entries it parsed from pasted JSON. These tests pin what Go
// answers for the same inputs the old Python tab showed, and what it refuses.

// goodEntry is a complete extension entry, as the catalog has them.
const goodEntry = `{
	"package_name": "tts_webui_extension.one",
	"name": "One",
	"requirements": "git+https://github.com/rsxdalv/tts_webui_extension.one@main",
	"description": "The first one",
	"extension_type": "interface",
	"extension_class": "text-to-speech",
	"author": "a",
	"extension_author": "b",
	"license": "MIT",
	"website": "https://w",
	"extension_website": "https://e",
	"extension_platform_version": "0.0.1"
}`

const goodEntryTwo = `{
	"package_name": "tts_webui_extension.two",
	"name": "Two",
	"requirements": "git+https://github.com/rsxdalv/tts_webui_extension.two@main",
	"description": "The second one",
	"extension_type": "interface",
	"extension_class": "text-to-speech",
	"author": "a",
	"extension_author": "b",
	"license": "MIT",
	"website": "https://w",
	"extension_website": "https://e",
	"extension_platform_version": "0.0.1"
}`

// previewValue is the Markdown the parse answer carries.
func previewValue(t *testing.T, r ParseResult) string {
	t.Helper()
	m, ok := r.Preview["props"].(map[string]any)
	if !ok {
		t.Fatalf("preview is not a marker: %v", r.Preview)
	}
	s, _ := m["value"].(string)
	return s
}

// TestParseMatchesTheOldPythonTab pins the answers the old Python tab gave
// for the same inputs (the values were taken from the pre-port main.py).
func TestParseMatchesTheOldPythonTab(t *testing.T) {
	head := "| Name | Package | Class | Type |\n|---|---|---|---|\n"
	for _, tc := range []struct {
		name    string
		text    string
		info    string
		preview string
		entries int
	}{
		{
			name:    "a single object",
			text:    goodEntry,
			info:    "Parsed entries: 1",
			preview: head + "| One | tts_webui_extension.one | text-to-speech | interface |",
			entries: 1,
		},
		{
			name: "an array",
			text: "[" + goodEntry + "," + goodEntryTwo + "]",
			info: "Parsed entries: 2",
			preview: head +
				"| One | tts_webui_extension.one | text-to-speech | interface |\n" +
				"| Two | tts_webui_extension.two | text-to-speech | interface |",
			entries: 2,
		},
		{
			name:    "invalid JSON",
			text:    "{oops",
			info:    "JSON parse error: Expecting property name enclosed in double quotes: line 1 column 2 (char 1)",
			preview: "<i>No valid entries to preview.</i>",
		},
		{
			name:    "nothing pasted",
			text:    "   ",
			info:    "No JSON provided",
			preview: "<i>No valid entries to preview.</i>",
		},
		{
			name:    "a wrong type",
			text:    strings.Replace(goodEntry, `"name": "One"`, `"name": 5`, 1),
			info:    "Parsed entries: 0 | Issues: Entry 0: Wrong types for: name",
			preview: "<i>No valid entries to preview.</i>",
		},
		{
			name:    "a missing key",
			text:    strings.Replace(goodEntry, `"license": "MIT",`, ``, 1),
			info:    "Parsed entries: 0 | Issues: Entry 0: Missing keys: license",
			preview: "<i>No valid entries to preview.</i>",
		},
		{
			name: "a duplicate package_name",
			text: "[" + goodEntry + "," + strings.Replace(goodEntryTwo,
				"tts_webui_extension.two", "tts_webui_extension.one", 1) + "]",
			info: "Parsed entries: 2",
			preview: head +
				"| One | tts_webui_extension.one | text-to-speech | interface |\n" +
				"| Two | tts_webui_extension.one | text-to-speech | interface |",
			entries: 2,
		},
		{
			name: "a non-object array item",
			text: "[" + goodEntry + `,3,"x"]`,
			info: "Parsed entries: 1",
			preview: head +
				"| One | tts_webui_extension.one | text-to-speech | interface |",
			entries: 1,
		},
		{
			name:    "not an object or an array",
			text:    "5",
			info:    "Expected a JSON object or an array of objects",
			preview: "<i>No valid entries to preview.</i>",
		},
		{
			name:    "a string",
			text:    `"x"`,
			info:    "Expected a JSON object or an array of objects",
			preview: "<i>No valid entries to preview.</i>",
		},
		{
			name:    "an array of strings",
			text:    `["a"]`,
			info:    "Parsed entries: 0",
			preview: "<i>No valid entries to preview.</i>",
		},
	} {
		r := ParseInput(tc.text)
		if r.Info != tc.info {
			t.Errorf("%s: info %q, want %q", tc.name, r.Info, tc.info)
		}
		if got := previewValue(t, r); got != tc.preview {
			t.Errorf("%s: preview %q, want %q", tc.name, got, tc.preview)
		}
		if len(r.Entries) != tc.entries {
			t.Errorf("%s: %d entries, want %d", tc.name, len(r.Entries), tc.entries)
		}
	}
}

// TestParseEntriesKeepTheirKeys checks the entries keep their key order, so
// the file Go writes looks like the one Python wrote.
func TestParseEntriesKeepTheirKeys(t *testing.T) {
	r := ParseInput(goodEntry)
	if len(r.Entries) != 1 {
		t.Fatalf("entries: %d", len(r.Entries))
	}
	o, ok := r.Entries[0].(*Obj)
	if !ok {
		t.Fatalf("entry is not an object: %T", r.Entries[0])
	}
	want := []string{"package_name", "name", "requirements", "description",
		"extension_type", "extension_class", "author", "extension_author",
		"license", "website", "extension_website", "extension_platform_version"}
	if strings.Join(o.Keys, ",") != strings.Join(want, ",") {
		t.Errorf("key order: %v", o.Keys)
	}
}

// TestPyParseMatchesCPython pins the JSON error messages and positions to
// json.loads' (taken from CPython 3.10).
func TestPyParseMatchesCPython(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"", "Expecting value: line 1 column 1 (char 0)"},
		{"   ", "Expecting value: line 1 column 4 (char 3)"},
		{"nope", "Expecting value: line 1 column 1 (char 0)"},
		{"tru", "Expecting value: line 1 column 1 (char 0)"},
		{"{oops", "Expecting property name enclosed in double quotes: line 1 column 2 (char 1)"},
		{`{"a":1,}`, "Expecting property name enclosed in double quotes: line 1 column 8 (char 7)"},
		{"[1,", "Expecting value: line 1 column 4 (char 3)"},
		{"[1,]", "Expecting value: line 1 column 4 (char 3)"},
		{`{"a" 1}`, "Expecting ':' delimiter: line 1 column 6 (char 5)"},
		{"{'a': 1}", "Expecting property name enclosed in double quotes: line 1 column 2 (char 1)"},
		{`"unterminated`, "Unterminated string starting at: line 1 column 1 (char 0)"},
		{`"bad\qescape"`, `Invalid \escape: line 1 column 5 (char 4)`},
		{`{"a":1} x`, "Extra data: line 1 column 9 (char 8)"},
		{"1.", "Extra data: line 1 column 2 (char 1)"},
		{"01", "Extra data: line 1 column 2 (char 1)"},
		{"-", "Expecting value: line 1 column 1 (char 0)"},
		{"+1", "Expecting value: line 1 column 1 (char 0)"},
		{"[1 2]", "Expecting ',' delimiter: line 1 column 4 (char 3)"},
		{`{"a":1 "b":2}`, "Expecting ',' delimiter: line 1 column 8 (char 7)"},
		{"[", "Expecting value: line 1 column 2 (char 1)"},
		{"{", "Expecting property name enclosed in double quotes: line 1 column 2 (char 1)"},
		{"\"tab\there\"", "Invalid control character at: line 1 column 5 (char 4)"},
		{`{"a":"x`, "Unterminated string starting at: line 1 column 6 (char 5)"},
		{"nan", "Expecting value: line 1 column 1 (char 0)"},
		{"{,}", "Expecting property name enclosed in double quotes: line 1 column 2 (char 1)"},
		{`["a",]`, "Expecting value: line 1 column 6 (char 5)"},
		{`{"a":1,"b":}`, "Expecting value: line 1 column 12 (char 11)"},
		{"[1,2", "Expecting ',' delimiter: line 1 column 5 (char 4)"},
		{`{"a":1`, "Expecting ',' delimiter: line 1 column 7 (char 6)"},
		{`{"a"`, "Expecting ':' delimiter: line 1 column 5 (char 4)"},
		{`{"a":`, "Expecting value: line 1 column 6 (char 5)"},
		{`"\u12"`, `Invalid \uXXXX escape: line 1 column 3 (char 2)`},
		{`{"a":1,,"b":2}`, "Expecting property name enclosed in double quotes: line 1 column 8 (char 7)"},
		{"1e", "Extra data: line 1 column 2 (char 1)"},
		{"1.2.3", "Extra data: line 1 column 4 (char 3)"},
		{"0x10", "Extra data: line 1 column 2 (char 1)"},
		{"00", "Extra data: line 1 column 2 (char 1)"},
		{"truex", "Extra data: line 1 column 5 (char 4)"},
	} {
		_, err := pyParse(tc.in)
		if err == nil {
			t.Errorf("pyParse(%q) succeeded", tc.in)
			continue
		}
		if err.Error() != tc.want {
			t.Errorf("pyParse(%q) = %q, want %q", tc.in, err.Error(), tc.want)
		}
	}
}

// TestPyParseAcceptsWhatPythonAccepts checks the scanner takes the same
// values (numbers stay as written, the key order is kept).
func TestPyParseAcceptsWhatPythonAccepts(t *testing.T) {
	v, err := pyParse(`{"z":1,"a":[{"y":"é","b":null,"c":true}],"n":[],"f":1.50,"e":2e3}`)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(Marshal(v, "")); got != `{"z":1,"a":[{"y":"é","b":null,"c":true}],"n":[],"f":1.50,"e":2e3}` {
		t.Fatalf("values: %s", got)
	}
}

// TestPyParseUnicodeEscapes checks a \u escape is read and written like
// json.loads does.
func TestPyParseUnicodeEscapes(t *testing.T) {
	v, err := pyParse(`{"a":"é","b":"😀"}`)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(Marshal(v, "")); got != `{"a":"é","b":"😀"}` {
		t.Fatalf("values: %s", got)
	}
}

// TestAddSkipsADuplicatePackage checks the duplicate package_name of one add
// call: the old Python tab added it twice (its set of known package names was
// not updated), Go adds it once.
func TestAddSkipsADuplicatePackage(t *testing.T) {
	s, root := newStore(t)
	raw := []any{}
	for _, x := range []string{goodEntry, strings.Replace(goodEntryTwo,
		"tts_webui_extension.two", "tts_webui_extension.one", 1)} {
		v, err := Parse([]byte(x))
		if err != nil {
			t.Fatal(err)
		}
		raw = append(raw, v)
	}
	res, err := s.Add(raw)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != "Saved | Added: tts_webui_extension.one | Skipped (exists): tts_webui_extension.one" {
		t.Fatalf("status: %q", res.Status)
	}
	b, _ := os.ReadFile(filepath.Join(root, "extensions.external.json"))
	if strings.Count(string(b), `"package_name": "tts_webui_extension.one"`) != 1 {
		t.Fatalf("the entry was added twice:\n%s", b)
	}
	// a second add of the same package skips it
	res, err = s.Add(raw[:1])
	if err != nil || res.Status != "Saved | Added: none | Skipped (exists): tts_webui_extension.one" {
		t.Fatalf("second add: %+v %v", res, err)
	}
}

// TestConcurrentAddsDoNotLoseUpdates adds entries from several goroutines at
// the same time: every one of them must be in the file afterwards.
func TestConcurrentAddsDoNotLoseUpdates(t *testing.T) {
	s, root := newStore(t)
	const n = 8
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			text := strings.NewReplacer(
				"tts_webui_extension.one", fmt.Sprintf("tts_webui_extension.ext%d", i),
				`"name": "One"`, fmt.Sprintf(`"name": "Ext %d"`, i),
			).Replace(goodEntry)
			v, err := Parse([]byte(text))
			if err != nil {
				t.Error(err)
				return
			}
			if _, err := s.Add([]any{v}); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	b, _ := os.ReadFile(filepath.Join(root, "extensions.external.json"))
	got := string(b)
	for i := 0; i < n; i++ {
		want := fmt.Sprintf("tts_webui_extension.ext%d", i)
		if !strings.Contains(got, want) {
			t.Errorf("%s was lost:\n%s", want, got)
		}
	}
	// the file is still valid JSON with all of them
	var doc struct {
		Tabs []map[string]any `json:"tabs"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("the file is not valid JSON: %v\n%s", err, got)
	}
	if len(doc.Tabs) != n {
		t.Errorf("%d entries in the file, want %d", len(doc.Tabs), n)
	}
}

// TestValidateRequirementsRefusesUntrustedInput checks the requirements of a
// pasted entry are checked before pip or uv sees them.
func TestValidateRequirementsRefusesUntrustedInput(t *testing.T) {
	for _, req := range []string{
		"foo; rm -rf /",                  // shell metacharacter
		"foo && echo pwned",              // shell metacharacter
		"foo | tee /tmp/x",               // shell metacharacter
		"foo `id`",                       // shell metacharacter
		"foo $(id)",                      // shell metacharacter
		"foo\nrm -rf /",                  // control character
		"--index-url http://evil",        // an option pip would take
		"--extra-index-url=https://evil", // an option pip would take
		"-e .",                           // an option (editable install)
		"http://evil/pkg.tar.gz",         // a URL that is not verified
		"file:///etc/passwd",             // a local file
		"git+file:///tmp/evil",           // a local git repo (runs setup.py)
		"ssh://git@evil/repo",            // a shell over ssh
		"git+ssh://git@evil/repo",
		"ext::evil --version", // pip runs the command
		"./local/dir",         // a local path
		"/tmp/local",          // an absolute path
		"..\\windows\\evil",   // a local path
	} {
		if toks, err := validateRequirements(req); err == nil {
			t.Errorf("validateRequirements(%q) accepted it: %v", req, toks)
		}
	}
	for _, req := range []string{
		"git+https://github.com/rsxdalv/tts_webui_extension.one@main",
		"https://example.com/pkg-1.0.tar.gz",
		"tts_webui_extension.mimo_audio>=0.0.4",
		"numpy",
		"numpy>=1.0,<2.0",
		"package[extra]==1.0",
	} {
		if _, err := validateRequirements(req); err != nil {
			t.Errorf("validateRequirements(%q) refused it: %v", req, err)
		}
	}
}

// TestInstallEntryInstallsTheEntryItWasGiven checks the install of a pasted
// entry: its own requirements, not a package looked up by name.
func TestInstallEntryInstallsTheEntryItWasGiven(t *testing.T) {
	s, root, log := installStore(t)
	entry := strings.NewReplacer(
		"git+https://github.com/rsxdalv/tts_webui_extension.one@main", "pasted-pkg==1.2.3",
	).Replace(goodEntry)
	_, got, _ := stream(t, s, `install-entry {"entry":`+entry+`}`)
	if !got.ok {
		t.Fatalf("install: %+v", got.lines)
	}
	l := readLog(t, log)
	if len(l) != 1 || l[0] != "python -m pip install pasted-pkg==1.2.3 gradio==5.49.1" {
		t.Fatalf("command: %q", l)
	}
	joined := strings.Join(got.lines, "\n")
	if !strings.Contains(joined, "Successfully installed One dependencies") {
		t.Errorf("messages:\n%s", joined)
	}
	if _, err := os.Stat(filepath.Join(root, "installer_scripts", "logs", "pip-install-One.log")); err != nil {
		t.Errorf("log: %v", err)
	}
}

// TestInstallEntryRefusesUntrustedRequirements checks nothing runs when the
// requirements of the pasted entry are not trusted.
func TestInstallEntryRefusesUntrustedRequirements(t *testing.T) {
	for _, req := range []string{
		"foo; rm -rf /",
		"--index-url http://evil",
		"http://evil/pkg.tar.gz",
		"file:///etc/passwd",
	} {
		s, _, log := installStore(t)
		entry := strings.NewReplacer(
			"git+https://github.com/rsxdalv/tts_webui_extension.one@main", req,
		).Replace(goodEntry)
		_, got, _ := stream(t, s, `install-entry {"entry":`+entry+`}`)
		if got.ok {
			t.Errorf("%q was installed", req)
		}
		if !strings.Contains(strings.Join(got.lines, "\n"), "requirements refused") {
			t.Errorf("%q: %v", req, got.lines)
		}
		if _, err := os.Stat(log); err == nil {
			t.Errorf("%q: something ran", req)
		}
	}
}

// TestInstallEntryRefusesAnIncompleteEntry checks the entry is validated like
// the parse step does.
func TestInstallEntryRefusesAnIncompleteEntry(t *testing.T) {
	s, _, log := installStore(t)
	_, got, raw := stream(t, s, `install-entry {"entry":{"package_name":"x"}}`)
	if !strings.Contains(raw, `"error"`) || !strings.Contains(raw, "Missing keys") {
		t.Fatalf("answer: %s", raw)
	}
	if got.done {
		t.Errorf("the stream started: %+v", got)
	}
	if _, err := os.Stat(log); err == nil {
		t.Error("something ran")
	}
}

// TestInstallEntryNativeUsesItsOwnProxy checks a "native" entry goes to the
// venv path.
func TestInstallEntryNativeUsesItsOwnProxy(t *testing.T) {
	s, root, log := installStore(t)
	writeFile(t, root, "installer_scripts/.gpu", "CPU\n")
	entry := strings.Replace(goodEntry,
		`"extension_platform_version": "0.0.1"`,
		`"extension_platform_version": "0.0.1", "proxy": "native"`, 1)
	_, got, _ := stream(t, s, `install-entry {"entry":`+entry+`}`)
	if !got.ok {
		t.Fatalf("native: %+v", got.lines)
	}
	l := readLog(t, log)
	if len(l) != 5 {
		t.Fatalf("commands: %q", l)
	}
	if !strings.HasPrefix(l[0], "uv venv ") || !strings.Contains(l[0], ".venvs") {
		t.Errorf("venv: %q", l[0])
	}
	if !strings.Contains(l[4], "git+https://github.com/rsxdalv/tts_webui_extension.one@main") {
		t.Errorf("requirements: %q", l[4])
	}
}

// TestInstallEntryIsRefusedWhileAnotherInstallRuns checks the lock.
func TestInstallEntryIsRefusedWhileAnotherInstallRuns(t *testing.T) {
	s, _ := newStore(t)
	s.install.Lock()
	h := s.Handler()
	if c, b := call(t, h, "POST", "/api/extcatalog/install-entry", `{"entry":`+goodEntry+`}`, "tok"); c != 409 || !strings.Contains(b, "another install is running") {
		t.Errorf("busy: %d %s", c, b)
	}
	s.install.Unlock()
}

// TestInstallEntryHTTP checks the endpoint's shape.
func TestInstallEntryHTTP(t *testing.T) {
	s, _ := newStore(t)
	h := s.Handler()
	if c, _ := call(t, h, "POST", "/api/extcatalog/install-entry", `{"entry":"no"}`, "tok"); c != 400 {
		t.Errorf("entry not an object: %d", c)
	}
	if c, _ := call(t, h, "POST", "/api/extcatalog/install-entry", `{"entry":{},"x":1}`, "tok"); c != 400 {
		t.Errorf("unknown field: %d", c)
	}
	if c, _ := call(t, h, "GET", "/api/extcatalog/install-entry", "", "tok"); c != 405 {
		t.Errorf("wrong method: %d", c)
	}
	if c, _ := call(t, h, "POST", "/api/extcatalog/install-entry", `{"entry":`+goodEntry+`}`, ""); c != 403 {
		t.Errorf("no token: %d", c)
	}
}

// TestEndToEndAgainstTheRealHandler drives the whole tab over HTTP, the way
// the Python tab does: parse, then add. It shows the bytes of
// extensions.external.json before and after.
func TestEndToEndAgainstTheRealHandler(t *testing.T) {
	root := t.TempDir()
	s := New(Options{Root: root, Token: "tok"})
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	post := func(path, body string) string {
		t.Helper()
		req, err := http.NewRequest("POST", srv.URL+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(TokenHeader, "tok")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("%s: %d %s", path, resp.StatusCode, b)
		}
		b, _ := io.ReadAll(resp.Body)
		return string(b)
	}

	// the file is not there yet
	path := filepath.Join(root, "extensions.external.json")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("the file exists before the add")
	}
	t.Logf("before: no extensions.external.json")

	// parse: the preview is the one the old Python tab showed
	answer := post("/api/extcatalog/parse", `{"text":`+mustJSON(t, goodEntry)+`}`)
	if !strings.Contains(answer, `| One | tts_webui_extension.one | text-to-speech | interface |`) {
		t.Fatalf("parse: %s", answer)
	}
	t.Logf("preview: %s", answer)

	// add
	answer = post("/api/extcatalog/add", "{\"entries\":["+goodEntry+","+goodEntryTwo+"]}")
	if !strings.Contains(answer, `"status":"Saved | Added: tts_webui_extension.one, tts_webui_extension.two | Skipped (exists): none"`) {
		t.Fatalf("add: %s", answer)
	}

	// after: the file Go wrote
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("after: %s", b)
	want := "{\n    \"tabs\": [\n        {\n            \"package_name\": \"tts_webui_extension.one\",\n" +
		"            \"name\": \"One\",\n" +
		"            \"requirements\": \"git+https://github.com/rsxdalv/tts_webui_extension.one@main\",\n" +
		"            \"description\": \"The first one\",\n" +
		"            \"extension_type\": \"interface\",\n" +
		"            \"extension_class\": \"text-to-speech\",\n" +
		"            \"author\": \"a\",\n" +
		"            \"extension_author\": \"b\",\n" +
		"            \"license\": \"MIT\",\n" +
		"            \"website\": \"https://w\",\n" +
		"            \"extension_website\": \"https://e\",\n" +
		"            \"extension_platform_version\": \"0.0.1\"\n        },\n" +
		"        {\n            \"package_name\": \"tts_webui_extension.two\",\n" +
		"            \"name\": \"Two\",\n" +
		"            \"requirements\": \"git+https://github.com/rsxdalv/tts_webui_extension.two@main\",\n" +
		"            \"description\": \"The second one\",\n" +
		"            \"extension_type\": \"interface\",\n" +
		"            \"extension_class\": \"text-to-speech\",\n" +
		"            \"author\": \"a\",\n" +
		"            \"extension_author\": \"b\",\n" +
		"            \"license\": \"MIT\",\n" +
		"            \"website\": \"https://w\",\n" +
		"            \"extension_website\": \"https://e\",\n" +
		"            \"extension_platform_version\": \"0.0.1\"\n        }\n    ],\n" +
		"    \"decorators\": []\n}"
	if string(b) != want {
		t.Fatalf("the file is not what json.dump(indent=4, ensure_ascii=False) writes:\n%s", b)
	}

	// a second add of the same entries changes nothing
	size := len(b)
	answer = post("/api/extcatalog/add", "{\"entries\":["+goodEntry+","+goodEntryTwo+"]}")
	if !strings.Contains(answer, `"status":"Saved | Added: none | Skipped (exists): tts_webui_extension.one, tts_webui_extension.two"`) {
		t.Fatalf("second add: %s", answer)
	}
	b2, _ := os.ReadFile(path)
	if len(b2) != size {
		t.Fatalf("the file changed on the second add")
	}
}

// mustJSON is the compact JSON of a string.
func mustJSON(t *testing.T, s string) string {
	t.Helper()
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
