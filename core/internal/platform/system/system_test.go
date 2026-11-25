package system

import (
	"context"
	"reflect"
	"testing"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
)

func TestCommandRunsFromPath(t *testing.T) {
	root, _ := paths.New(t.TempDir())
	p := New(Options{Root: root, Env: []string{"A=1"}})
	c, err := p.Command(nil, []string{"uv", "pip", "install", "gradio>=1"})
	if err != nil {
		t.Fatal(err)
	}
	if c.Program != "uv" || !reflect.DeepEqual(c.Args, []string{"pip", "install", "gradio>=1"}) {
		t.Fatalf("got %q %q", c.Program, c.Args)
	}
	if c.Env[len(c.Env)-1] != "A=1" || c.Dir != root.Dir() {
		t.Fatalf("env/dir: %q %q", c.Env[len(c.Env)-1], c.Dir)
	}
}

func TestPythonOverride(t *testing.T) {
	root, _ := paths.New(t.TempDir())
	c, _ := New(Options{Root: root, Python: "/opt/py"}).Command(nil, []string{"python", "-m", "pip"})
	if c.Program != "/opt/py" || !reflect.DeepEqual(c.Args, []string{"-m", "pip"}) {
		t.Fatalf("got %q %q", c.Program, c.Args)
	}
	st := New(Options{Root: root, Python: "/no/such/python"}).Status(context.Background())
	if st.Ready || st.Detail == "" {
		t.Fatalf("status = %+v", st)
	}
}
