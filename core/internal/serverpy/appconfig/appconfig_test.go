package appconfig

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

type golden struct{ Name, Input, Output string }

func goldens(t *testing.T) []golden {
	t.Helper()
	b, err := os.ReadFile("testdata/golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var g []golden
	if err := json.Unmarshal(b, &g); err != nil {
		t.Fatal(err)
	}
	return g
}

func newStore(t *testing.T) (*Store, *[]string) {
	var logs []string
	var mu sync.Mutex
	s := New(Options{Root: t.TempDir(), Token: "tok", Logf: func(f string, a ...any) {
		mu.Lock()
		logs = append(logs, fmt.Sprintf(f, a...))
		mu.Unlock()
	}})
	return s, &logs
}

func TestGoldenBytes(t *testing.T) {
	for _, g := range goldens(t) {
		t.Run(g.Name, func(t *testing.T) {
			s, _ := newStore(t)
			if err := s.Save([]byte(g.Input)); err != nil {
				t.Fatal(err)
			}
			got, _ := os.ReadFile(s.path)
			if string(got) != g.Output {
				t.Fatalf("got\n%s\nwant\n%s", got, g.Output)
			}
		})
	}
}

func TestDefaultCreatedLikePython(t *testing.T) {
	s, _ := newStore(t)
	got, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range goldens(t) {
		if g.Name == "default" {
			if string(got) != g.Output {
				t.Fatalf("default differs from load_config.py:\n%s", got)
			}
			disk, _ := os.ReadFile(s.path)
			if string(disk) != g.Output {
				t.Fatal("default file not written")
			}
			return
		}
	}
	t.Fatal("no default golden")
}

func TestValidation(t *testing.T) {
	s, _ := newStore(t)
	for _, in := range []string{`[1]`, `"x"`, `null`, `{`, `{} {}`,
		`{"gradio_interface_options":{"server_port":"7770"}}`,
		`{"gradio_interface_options":{"server_port":1.5}}`,
		`{"gradio_interface_options":{"server_port":true}}`} {
		if err := s.Save([]byte(in)); err == nil {
			t.Errorf("%s accepted", in)
		}
	}
	if _, err := os.Stat(s.path); err == nil {
		t.Error("rejected input wrote a file")
	}
	if err := s.Save([]byte(`{"gradio_interface_options":{"server_port":1}}`)); err != nil {
		t.Error(err)
	}
}

func TestConcurrentSaves(t *testing.T) {
	s, _ := newStore(t)
	var wg sync.WaitGroup
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			doc := fmt.Sprintf(`{"n": %d, "pad": "%s"}`, i, strings.Repeat("x", 5000))
			if err := s.Save([]byte(doc)); err != nil {
				t.Error(err)
			}
			if _, err := s.Load(); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	b, _ := os.ReadFile(s.path)
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("file corrupt: %v", err)
	}
	left, _ := filepath.Glob(filepath.Join(s.opts.Root, ".config.json.*"))
	if len(left) != 0 {
		t.Fatalf("temp files left: %v", left)
	}
}

func TestChangeLog(t *testing.T) {
	s, logs := newStore(t)
	_ = s.Save([]byte(`{"gradio_interface_options":{"server_port":7770,"auth":null,"debug":false}}`))
	*logs = nil
	_ = s.Save([]byte(`{"gradio_interface_options":{"server_port":7771,"auth":"u:secret","debug":true}}`))
	joined := strings.Join(*logs, "\n")
	if !strings.Contains(joined, "auth, server_port") || !strings.Contains(joined, "restart") {
		t.Fatalf("logs: %q", joined)
	}
	if strings.Contains(joined, "secret") || strings.Contains(joined, "debug") {
		t.Fatalf("logs leak or over-report: %q", joined)
	}
	*logs = nil
	_ = s.Save([]byte(`{"gradio_interface_options":{"server_port":7771,"auth":"u:secret","debug":false}}`))
	if strings.Contains(strings.Join(*logs, ""), "launch-time") {
		t.Fatalf("no launch key changed: %q", *logs)
	}
}

func TestHTTP(t *testing.T) {
	s, _ := newStore(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	do := func(method, body, token, ctype string) (int, string) {
		req, _ := http.NewRequest(method, srv.URL+Path, strings.NewReader(body))
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
		var b strings.Builder
		buf := make([]byte, 4096)
		for {
			n, err := resp.Body.Read(buf)
			b.Write(buf[:n])
			if err != nil {
				break
			}
		}
		return resp.StatusCode, b.String()
	}
	if c, _ := do("GET", "", "", ""); c != 403 {
		t.Fatalf("no token: %d", c)
	}
	if c, _ := do("GET", "", "bad", ""); c != 403 {
		t.Fatalf("bad token: %d", c)
	}
	c, body := do("GET", "", "tok", "")
	if c != 200 || !strings.Contains(body, `"server_port": 7770`) {
		t.Fatalf("get: %d %s", c, body)
	}
	if c, _ := do("PUT", `{}`, "tok", "text/plain"); c != 415 {
		t.Fatalf("ctype: %d", c)
	}
	if c, _ := do("PUT", `[1]`, "tok", "application/json"); c != 400 {
		t.Fatalf("array: %d", c)
	}
	if c, _ := do("PUT", `{"a": "é"}`, "tok", "application/json"); c != 200 {
		t.Fatalf("put: %d", c)
	}
	if _, body := do("GET", "", "tok", ""); body != "{\n  \"a\": \"\\u00e9\"\n}" {
		t.Fatalf("roundtrip: %q", body)
	}
	if c, _ := do("DELETE", "", "tok", ""); c != 405 {
		t.Fatalf("delete: %d", c)
	}
}
