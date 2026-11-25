package piplist

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNoPythonIsAState(t *testing.T) {
	missingEnv := filepath.Join(t.TempDir(), "env")
	for name, run := range map[string][]string{
		"no program":     {"definitely-not-a-program", "-p", missingEnv, "run"},
		"no env":         {os.Args[0], "-p", missingEnv, "run"},
		"no python path": nil,
	} {
		dir := t.TempDir()
		s := New(Options{Root: dir, Token: "tok", Platform: prefixed(t, dir, run)})
		if name == "no python path" {
			t.Setenv("PATH", t.TempDir())
		}
		if _, err := s.List(context.Background()); !errors.Is(err, ErrNoPython) {
			t.Errorf("%s: %v", name, err)
		}
	}
	dir := t.TempDir()
	s := New(Options{Root: dir, Token: "tok", Platform: prefixed(t, dir, []string{os.Args[0], "-p", missingEnv, "run"})})
	r := httptest.NewRequest("GET", Path, nil)
	r.RemoteAddr = "127.0.0.1:1"
	r.Header.Set(TokenHeader, "tok")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), `"code":"no_python"`) {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
}
