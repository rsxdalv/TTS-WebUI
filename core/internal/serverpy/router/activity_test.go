package router

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestActivityHookWrapsEachProxiedRequest(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("ok")) }))
	defer up.Close()
	rt := New(Options{Token: "t"})
	if err := rt.AddRoute("/eng", up.URL); err != nil {
		t.Fatal(err)
	}
	var started, finished atomic.Int32
	var prefix atomic.Value
	rt.SetActivity(func(p string) func() {
		started.Add(1)
		prefix.Store(p)
		return func() { finished.Add(1) }
	})
	rec := httptest.NewRecorder()
	rt.ServeHTTP(rec, httptest.NewRequest("GET", "/eng/", nil))
	if rec.Code != 200 || started.Load() != 1 || finished.Load() != 1 || prefix.Load() != "/eng" {
		t.Fatalf("code %d started %d finished %d prefix %v", rec.Code, started.Load(), finished.Load(), prefix.Load())
	}
	// A request no route owns is not an engine's activity.
	rt.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/nothing", nil))
	if started.Load() != 1 {
		t.Fatal("an unrouted request counted as activity")
	}
}
