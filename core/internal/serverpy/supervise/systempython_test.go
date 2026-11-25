package supervise

import (
	"testing"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
	"github.com/rsxdalv/TTS-WebUI/core/internal/platform"
)

// A child's command line is the platform's: the system platform runs the
// program itself, the legacy env puts its prefix in front.
func TestChildCommandOnEachPlatform(t *testing.T) {
	root, _ := paths.New(t.TempDir())
	argv := []string{"python", "server_core.py", "--no-react"}

	sys, err := Config{Root: root, SystemPython: true}.normalized()
	if err != nil {
		t.Fatal(err)
	}
	c, _ := sys.Platform.Command(nil, argv)
	if sys.Platform.Name() != "system" || c.Program != "python" || len(c.Args) != 2 || c.Args[0] != "server_core.py" {
		t.Errorf("system: %s %q %v", sys.Platform.Name(), c.Program, c.Args)
	}

	pre := prefixPlatform(root, []string{"mm", "-p", "env", "run"})
	c, _ = pre.Command(nil, []string{"python", "server_core.py"})
	if c.Program != "mm" || len(c.Args) != 5 || c.Args[0] != "-p" || c.Args[4] != "server_core.py" {
		t.Errorf("prefixed: %q %v", c.Program, c.Args)
	}
}

func TestSystemPythonSelectsSystemAndSkipsTheGPUGate(t *testing.T) {
	root, _ := paths.New(t.TempDir())
	other := prefixPlatform(root, []string{"ignored"})
	c, err := Config{Root: root, SystemPython: true, Platform: other}.normalized()
	if err != nil {
		t.Fatal(err)
	}
	var p platform.Platform = c.Platform
	if p.Name() != "system" {
		t.Errorf("platform=%s", p.Name())
	}
}
