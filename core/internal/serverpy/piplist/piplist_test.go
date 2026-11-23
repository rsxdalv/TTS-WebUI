package piplist

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// TestHelperProcess is the fake "python -m pip": FAKE_PIP selects the behaviour.
func TestHelperProcess(t *testing.T) {
	mode := os.Getenv("FAKE_PIP")
	if mode == "" {
		return
	}
	var args []string
	for i, a := range os.Args {
		if a == "--" {
			args = os.Args[i+1:]
		}
	}
	if got := strings.Join(args, " "); got != "python -m pip list --format=json --disable-pip-version-check" {
		fmt.Fprintln(os.Stderr, "unexpected args: "+got)
		os.Exit(9)
	}
	switch mode {
	case "ok":
		fmt.Print(`[{"name": "zlib-ng", "version": "1.0"}, {"name": "Torch", "version": "2.1"}, {"name": "numpy", "version": "1.26"}, {"name": "Alpha", "version": "0.1"}]`)
	case "fail":
		fmt.Fprintln(os.Stderr, "ERROR: no pip here\nsecond line")
		os.Exit(2)
	case "garbage":
		fmt.Print("not json")
	case "hang":
		time.Sleep(10 * time.Second)
	}
	os.Exit(0)
}

func fake(t *testing.T, mode string) *Store {
	t.Helper()
	t.Setenv("FAKE_PIP", mode)
	return New(Options{
		Root: t.TempDir(), Token: "tok", Timeout: 2 * time.Second,
		Run: []string{os.Args[0], "-test.run=^TestHelperProcess$", "--"},
	})
}

func TestListSortedCaseInsensitive(t *testing.T) {
	got, err := fake(t, "ok").List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, p := range got {
		names = append(names, p.Name)
	}
	if strings.Join(names, ",") != "Alpha,numpy,Torch,zlib-ng" {
		t.Fatalf("order: %v", names)
	}
	if got[2].Version != "2.1" {
		t.Fatalf("%v", got)
	}
}

func TestFailures(t *testing.T) {
	if _, err := fake(t, "fail").List(context.Background()); err == nil || !strings.Contains(err.Error(), "ERROR: no pip here | second line") {
		t.Fatalf("fail: %v", err)
	}
	if _, err := fake(t, "garbage").List(context.Background()); err == nil || !strings.Contains(err.Error(), "unreadable output") {
		t.Fatalf("garbage: %v", err)
	}
	s := fake(t, "hang")
	s.opts.Timeout = 300 * time.Millisecond
	if _, err := s.List(context.Background()); err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("hang: %v", err)
	}
	s = New(Options{Root: t.TempDir(), Token: "tok", Run: []string{"definitely-not-a-program"}})
	if _, err := s.List(context.Background()); err == nil {
		t.Fatal("missing program accepted")
	}
}

func TestParse(t *testing.T) {
	got, err := parse([]byte("WARNING x\n[{\"name\":\"b\",\"version\":\"1\"},{\"name\":\"\",\"version\":\"9\"},{\"name\":\"B\",\"version\":\"0\"}]"))
	if err != nil || len(got) != 2 || got[0].Name != "B" {
		t.Fatalf("%v %v", got, err)
	}
	if got, err := parse([]byte("[]")); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("%v %v", got, err)
	}
}

func TestHTTP(t *testing.T) {
	s := fake(t, "ok")
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	do := func(method, path, token string) (int, string) {
		req, _ := http.NewRequest(method, srv.URL+path, nil)
		if token != "" {
			req.Header.Set(TokenHeader, token)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(b)
	}
	if c, _ := do("GET", Path, ""); c != 403 {
		t.Fatalf("no token %d", c)
	}
	if c, _ := do("GET", Path, "bad"); c != 403 {
		t.Fatalf("bad token %d", c)
	}
	if c, _ := do("POST", Path, "tok"); c != 405 {
		t.Fatalf("post %d", c)
	}
	if c, _ := do("GET", "/api/pip/other", "tok"); c != 404 {
		t.Fatalf("other %d", c)
	}
	c, body := do("GET", Path, "tok")
	var list []Package
	if c != 200 || json.Unmarshal([]byte(body), &list) != nil || len(list) != 4 {
		t.Fatalf("get: %d %s", c, body)
	}
	t.Setenv("FAKE_PIP", "fail")
	c, body = do("GET", Path, "tok")
	if c != 502 || !strings.Contains(body, "no pip here") {
		t.Fatalf("fail: %d %s", c, body)
	}
}

func TestCachedRemembersUntilInvalidated(t *testing.T) {
	s := fake(t, "ok")
	ctx := context.Background()
	first, err := s.Cached(ctx)
	if err != nil || len(first) != 4 {
		t.Fatalf("first: %v %v", first, err)
	}
	// pip now fails, but the answer is remembered
	t.Setenv("FAKE_PIP", "fail")
	if again, err := s.Cached(ctx); err != nil || len(again) != 4 {
		t.Fatalf("not cached: %v %v", again, err)
	}
	// List itself never caches
	if _, err := s.List(ctx); err == nil {
		t.Fatal("List must run pip every time")
	}
	s.Invalidate()
	if _, err := s.Cached(ctx); err == nil {
		t.Fatal("after Invalidate pip runs again")
	}
	// a failure is not remembered
	t.Setenv("FAKE_PIP", "ok")
	if got, err := s.Cached(ctx); err != nil || len(got) != 4 {
		t.Fatalf("after the failure: %v %v", got, err)
	}
}
