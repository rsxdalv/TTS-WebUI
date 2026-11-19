package switches

import (
	"strings"
	"testing"
)

func env(m map[string]string) func(string) (string, bool) {
	return func(k string) (string, bool) { v, ok := m[k]; return v, ok }
}

func TestDefaultsAreOff(t *testing.T) {
	s, err := Parse(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	if s.Proxy || s.Route("upload") || s.Step("mamba") {
		t.Fatalf("expected everything off, got %+v", s)
	}
}

func TestEnabledNames(t *testing.T) {
	s, err := Parse(env(map[string]string{
		EnvProxy:  "1",
		EnvRoutes: "upload, gradio",
		EnvSteps:  "mamba",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !s.Proxy || !s.Route("upload") || !s.Route("gradio") || s.Route("data-outputs") {
		t.Fatalf("routes wrong: %+v", s)
	}
	if !s.Step("mamba") || s.Step("install") {
		t.Fatalf("steps wrong: %+v", s)
	}
}

func TestAll(t *testing.T) {
	s, err := Parse(env(map[string]string{EnvProxy: "1", EnvRoutes: "all", EnvSteps: "all"}))
	if err != nil {
		t.Fatal(err)
	}
	if !s.Route("anything") || !s.Step("anything") {
		t.Fatalf("all did not enable: %+v", s)
	}
}

func TestRejected(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want string
	}{
		{"proxy typo", map[string]string{EnvProxy: "true"}, EnvProxy},
		{"routes without proxy", map[string]string{EnvRoutes: "upload"}, "needs"},
		{"all routes without proxy", map[string]string{EnvRoutes: "all"}, "needs"},
		{"bad route name", map[string]string{EnvProxy: "1", EnvRoutes: "Up_load"}, "invalid name"},
		{"empty list item", map[string]string{EnvSteps: "mamba,,install"}, "invalid name"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Parse(env(c.env))
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want it to contain %q", err, c.want)
			}
		})
	}
}

func TestCheckKnownNames(t *testing.T) {
	s, err := Parse(env(map[string]string{EnvProxy: "1", EnvRoutes: "upload", EnvSteps: "mamba"}))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CheckRoutes([]string{"upload"}); err != nil {
		t.Fatal(err)
	}
	if err := s.CheckRoutes([]string{"gradio"}); err == nil {
		t.Fatal("unknown route accepted")
	}
	if err := s.CheckSteps([]string{"install"}); err == nil {
		t.Fatal("unknown step accepted")
	}
}
