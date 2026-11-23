package appenv

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// what write_env(generate_env(...)) wrote, from the golden cases of
// TestGenerateEnvGolden (testdata/golden.json), with the four values varied.
const envTail = `# Environment variable for HuggingFace model location
%sHUGGINGFACE_HUB_CACHE=%s
# Environment variable for HuggingFace model location (alternative)
%sHF_HOME=%s
# Default location for Torch Hub models
%sTORCH_HOME=%s
# Default location for Torch Hub models (alternative)
%sXDG_CACHE_HOME=%s

`

func wantEnv(hf, home, th, xdg string) string {
	c := func(v string) string {
		if v == "" {
			return "# "
		}
		return ""
	}
	return envHeader + fmt.Sprintf(envTail, c(hf), hf, c(home), home, c(th), th, c(xdg), xdg)
}

func readEnvFile(t *testing.T, root string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestSaveModelLocationsWritesWhatWriteEnvWrote(t *testing.T) {
	for _, windows := range []bool{false, true} {
		root := t.TempDir()
		m := ModelLocations{HuggingFaceHubCache: "D:/models/hf", TorchHome: "/data/torch"}
		if err := saveModelLocations(root, m, windows); err != nil {
			t.Fatal(err)
		}
		want := wantEnv("D:/models/hf", "", "/data/torch", "")
		if windows {
			want = strings.ReplaceAll(want, "\n", "\r\n") // Python's text mode on Windows
		}
		if got := readEnvFile(t, root); got != want {
			t.Errorf("windows=%v:\n got %q\nwant %q", windows, got, want)
		}
		// the same text as the one EnsureEnv writes from the same variables
		other := t.TempDir()
		if _, err := ensureEnv(other, m.env(), windows); err != nil {
			t.Fatal(err)
		}
		if readEnvFile(t, other) != readEnvFile(t, root) {
			t.Errorf("windows=%v: SaveModelLocations and EnsureEnv disagree", windows)
		}
	}
}

func TestSaveReplacesAndEnsureNeverOverwrites(t *testing.T) {
	root := t.TempDir()
	if created, err := ensureEnv(root, nil, false); err != nil || !created {
		t.Fatalf("ensure: %v %v", created, err)
	}
	if got := readEnvFile(t, root); got != wantEnv("", "", "", "") {
		t.Fatalf("default: %q", got)
	}
	// a hand edit is replaced as a whole, like write_env did
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("MINE=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := saveModelLocations(root, ModelLocations{HFHome: "x"}, false); err != nil {
		t.Fatal(err)
	}
	if got := readEnvFile(t, root); got != wantEnv("", "x", "", "") {
		t.Fatalf("after save: %q", got)
	}
	if created, err := ensureEnv(root, nil, false); err != nil || created {
		t.Fatalf("ensure over an existing file: %v %v", created, err)
	}
	if got := readEnvFile(t, root); got != wantEnv("", "x", "", "") {
		t.Fatalf("ensure overwrote: %q", got)
	}
	left, _ := filepath.Glob(filepath.Join(root, ".env.*.tmp"))
	if len(left) != 0 {
		t.Errorf("temp files left: %v", left)
	}
}

func TestSaveModelLocationsRefusesWhatWouldAddVariables(t *testing.T) {
	root := t.TempDir()
	for name, m := range map[string]ModelLocations{
		"line break":      {HFHome: "a\nPATH=/evil"},
		"carriage return": {TorchHome: "a\rb"},
		"NUL":             {XDGCacheHome: "a\x00b"},
		"too long":        {HuggingFaceHubCache: strings.Repeat("x", maxValue+1)},
	} {
		if err := saveModelLocations(root, m, false); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	if _, err := os.Stat(filepath.Join(root, ".env")); err == nil {
		t.Error("a refused save wrote the file")
	}
}

func TestConcurrentSavesLeaveOneIntactFile(t *testing.T) {
	root := t.TempDir()
	var wg sync.WaitGroup
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if err := saveModelLocations(root, ModelLocations{HFHome: fmt.Sprint("v", i)}, false); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	got := readEnvFile(t, root)
	ok := false
	for i := 0; i < 40; i++ {
		ok = ok || got == wantEnv("", fmt.Sprint("v", i), "", "")
	}
	if !ok {
		t.Fatalf("not one of the saved files: %q", got)
	}
}

func envAPI(t *testing.T) (http.Handler, string, *[]string) {
	t.Helper()
	root := t.TempDir()
	var logs []string
	var mu sync.Mutex
	h := NewAPI(APIOptions{Root: root, Token: "tok", Logf: func(f string, a ...any) {
		mu.Lock()
		logs = append(logs, fmt.Sprintf(f, a...))
		mu.Unlock()
	}}).Handler()
	return h, root, &logs
}

func post(h http.Handler, path, body, token string) (int, string) {
	r := httptest.NewRequest("POST", path, strings.NewReader(body))
	r.RemoteAddr = "127.0.0.1:4000"
	r.Header.Set("Content-Type", "application/json")
	if token != "" {
		r.Header.Set(TokenHeader, token)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w.Code, strings.TrimSpace(w.Body.String())
}

const saveBody = `{"huggingface_hub_cache":"/secret/hf","hf_home":"","torch_home":"/t","xdg_cache_home":""}`

func TestEnvAPISavesAndNeverLogsValues(t *testing.T) {
	h, root, logs := envAPI(t)
	code, body := post(h, APIPrefix+"save-model-locations", saveBody, "tok")
	if code != 200 || body != `"saved"` {
		t.Fatalf("%d %s", code, body)
	}
	if got := readEnvFile(t, root); !strings.Contains(strings.ReplaceAll(got, "\r\n", "\n"), "HUGGINGFACE_HUB_CACHE=/secret/hf\n") {
		t.Fatalf("file: %q", got)
	}
	if len(*logs) == 0 {
		t.Error("nothing logged")
	}
	for _, l := range *logs {
		if strings.Contains(l, "/secret/hf") {
			t.Errorf("a value was logged: %s", l)
		}
	}
}

func TestEnvAPIRefusals(t *testing.T) {
	h, root, _ := envAPI(t)
	url := APIPrefix + "save-model-locations"
	for name, c := range map[string]struct {
		path, body, token string
		want              int
	}{
		"no token":      {url, saveBody, "", 403},
		"wrong token":   {url, saveBody, "nope", 403},
		"unknown op":    {APIPrefix + "set-key", saveBody, "tok", 404},
		"array":         {url, `[]`, "tok", 400},
		"null":          {url, `null`, "tok", 400},
		"two documents": {url, saveBody + saveBody, "tok", 400},
		"unknown field": {url, `{"huggingface_hub_cache":"","hf_home":"","torch_home":"","xdg_cache_home":"","PATH":"x"}`, "tok", 400},
		"missing field": {url, `{"hf_home":""}`, "tok", 400},
		"not a string":  {url, `{"huggingface_hub_cache":1,"hf_home":"","torch_home":"","xdg_cache_home":""}`, "tok", 400},
		"line break":    {url, `{"huggingface_hub_cache":"a\nb","hf_home":"","torch_home":"","xdg_cache_home":""}`, "tok", 400},
	} {
		if code, body := post(h, c.path, c.body, c.token); code != c.want {
			t.Errorf("%s: %d %s, want %d", name, code, body, c.want)
		}
	}
	if _, err := os.Stat(filepath.Join(root, ".env")); err == nil {
		t.Error("a refused request wrote the file")
	}

	for name, c := range map[string]struct {
		method, contentType, remote string
		want                        int
	}{
		"GET":             {"GET", "application/json", "127.0.0.1:4000", 405},
		"no content type": {"POST", "", "127.0.0.1:4000", 415},
		"not loopback":    {"POST", "application/json", "10.0.0.5:4000", 403},
	} {
		r := httptest.NewRequest(c.method, url, strings.NewReader(saveBody))
		r.RemoteAddr = c.remote
		r.Header.Set(TokenHeader, "tok")
		if c.contentType != "" {
			r.Header.Set("Content-Type", c.contentType)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != c.want {
			t.Errorf("%s: %d, want %d", name, w.Code, c.want)
		}
	}
}
