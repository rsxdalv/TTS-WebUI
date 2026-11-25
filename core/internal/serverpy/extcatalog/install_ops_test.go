package extcatalog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckDependenciesIsADryRunThatLeavesTheInstallLogAlone(t *testing.T) {
	s, root, log := installStore(t)
	writeFile(t, root, "extensions.json", `{"tabs":[`+entry("ext.pip", "Pip Ext")+`,`+entry("ext.native", "Native Ext", `"proxy":"native"`)+`]}`)
	_, got, _ := stream(t, s, `install {"package_name":"ext.pip"}`)
	if !got.ok {
		t.Fatalf("install: %+v", got)
	}
	logFile := filepath.Join(root, "installer_scripts", "logs", "pip-install-Pip Ext.log")
	before, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatal(err)
	}

	code, got, _ := stream(t, s, `check-dependencies {"package_name":"ext.pip"}`)
	if code != 200 || !got.ok {
		t.Fatalf("check: %d %+v", code, got)
	}
	l := readLog(t, log)
	if want := "python -m pip install --dry-run git+https://example.com/ext.pip@main gradio==5.49.1"; l[len(l)-1] != want {
		t.Fatalf("command:\n got %s\nwant %s", l[len(l)-1], want)
	}
	if after, _ := os.ReadFile(logFile); string(after) != string(before) {
		t.Errorf("the dry run rewrote the install log:\n%s", after)
	}

	// There is no dependency check for a native extension, and nothing runs.
	n := len(readLog(t, log))
	h := s.Handler()
	if c, b := call(t, h, "POST", Prefix+"check-dependencies", `{"package_name":"ext.native"}`, "tok"); c != 400 || !strings.Contains(b, "own venv") {
		t.Errorf("native: %d %s", c, b)
	}
	if c, _ := call(t, h, "POST", Prefix+"check-dependencies", `{"package_name":"unknown"}`, "tok"); c != 404 {
		t.Errorf("unknown: %d", c)
	}
	if c, _ := call(t, h, "POST", Prefix+"check-dependencies", `{"package_name":"ext.pip","requirements":"-e ."}`, "tok"); c != 400 {
		t.Errorf("a caller cannot add fields: %d", c)
	}
	if len(readLog(t, log)) != n {
		t.Error("a refused check ran something")
	}
}

func latest(t *testing.T, s *Store, pkg string) (int, string) {
	t.Helper()
	code, body := call(t, s.Handler(), "POST", Prefix+"latest-version", `{"package_name":"`+pkg+`"}`, "tok")
	var v string
	if code == 200 {
		if err := json.Unmarshal([]byte(body), &v); err != nil {
			t.Fatalf("not a JSON string: %s", body)
		}
	}
	return code, v
}

func TestLatestVersion(t *testing.T) {
	s, root, log := installStore(t)
	writeFile(t, root, "extensions.json", `{"tabs":[`+entry("ext.pip", "Pip Ext")+`,`+entry("-e", "Option")+`]}`)

	// pip names nothing new
	if code, v := latest(t, s, "ext.pip"); code != 200 || v != noNewerVersion {
		t.Fatalf("no output: %d %q", code, v)
	}
	l := readLog(t, log)
	if want := "python -m pip install --dry-run --no-deps ext.pip gradio==5.49.1"; l[0] != want {
		t.Fatalf("command:\n got %s\nwant %s", l[0], want)
	}

	// the last word of the "Would install" line
	t.Setenv("HELPER_WOULD_INSTALL", "gradio-5.49.1 ext.pip-1.2.3")
	if code, v := latest(t, s, "ext.pip"); code != 200 || v != "ext.pip-1.2.3" {
		t.Fatalf("version: %d %q", code, v)
	}

	// only listed packages, and only plain names: the name becomes a pip argument
	n := len(readLog(t, log))
	if code, _ := latest(t, s, "not.listed"); code != 404 {
		t.Errorf("unlisted: %d", code)
	}
	if code, _ := latest(t, s, "-e"); code != 400 {
		t.Errorf("option-looking name: %d", code)
	}
	if len(readLog(t, log)) != n {
		t.Error("a refused check ran something")
	}
	if code, _ := call(t, s.Handler(), "GET", Prefix+"latest-version", "", "tok"); code != 405 {
		t.Errorf("GET: %d", code)
	}
}
