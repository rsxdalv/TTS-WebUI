package compare

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// serve answers every path with the given status, content type and body.
func serve(status int, ctype, body string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", ctype)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
}

func run(a, b *httptest.Server, c Case) Result {
	return Run(context.Background(), a.URL, b.URL, []Case{c})[0]
}

func TestIdenticalServersMatch(t *testing.T) {
	a, b := serve(200, "text/plain", "hello"), serve(200, "text/plain", "hello")
	defer a.Close()
	defer b.Close()
	if r := run(a, b, Case{Name: "same", Path: "/x"}); !r.OK() {
		t.Fatalf("identical servers reported %v", r.Diffs)
	}
}

// The harness must be able to fail: each kind of difference is reported.
func TestDeliberateDifferencesAreCaught(t *testing.T) {
	base := serve(200, "text/plain", "hello")
	defer base.Close()
	cases := []struct {
		name, want string
		other      *httptest.Server
	}{
		{"status", "status:", serve(404, "text/plain", "hello")},
		{"header", "header Content-Type", serve(200, "application/json", "hello")},
		{"body", "body:", serve(200, "text/plain", "hellp")},
		{"body length", "body:", serve(200, "text/plain", "hello!")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer tc.other.Close()
			r := run(base, tc.other, Case{Name: tc.name, Path: "/x"})
			if r.OK() || !strings.Contains(strings.Join(r.Diffs, "\n"), tc.want) {
				t.Fatalf("want a %q difference, got %v", tc.want, r.Diffs)
			}
		})
	}
}

func TestRawPathIsSentUnchanged(t *testing.T) {
	var got []string
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = append(got, r.URL.EscapedPath())
	}))
	defer s.Close()
	want := []string{"/a/../b", "/a/%2e%2e/b", "/a%2fb", "/a%5cb"}
	for _, p := range want {
		Do(context.Background(), NewClient(), s.URL, Case{Path: p})
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("sent %q, server saw %q", want[i], got[i])
		}
	}
}

func TestIgnoreRuleNeedsReason(t *testing.T) {
	a, b := serve(200, "text/plain", "hello"), serve(200, "application/json", "hello")
	defer a.Close()
	defer b.Close()
	c := Case{Name: "i", Path: "/x", Ignore: []IgnoreRule{{Header: "Content-Type"}}}
	if r := run(a, b, c); r.OK() {
		t.Fatal("an ignore rule without a reason must not hide a difference")
	}
	c.Ignore = []IgnoreRule{{Header: "Content-Type", Reason: "test"}}
	if r := run(a, b, c); !r.OK() {
		t.Fatalf("a reasoned ignore rule should hide the header difference, got %v", r.Diffs)
	}
}

func TestRedirectsAreNotFollowed(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/elsewhere", http.StatusFound)
	}))
	defer s.Close()
	resp := Do(context.Background(), NewClient(), s.URL, Case{Path: "/x"})
	if resp.Status != 302 || resp.Header.Get("Location") != "/elsewhere" {
		t.Fatalf("got %d %q", resp.Status, resp.Header.Get("Location"))
	}
}
