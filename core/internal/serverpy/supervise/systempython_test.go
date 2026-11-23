package supervise

import (
	"strings"
	"testing"

	"github.com/rsxdalv/TTS-WebUI/core/internal/paths"
)

func TestChildCommandWithAndWithoutPrefix(t *testing.T) {
	cmd, args := childCommand([]string{"mm", "-p", "env", "run"}, []string{"python", "server_core.py"})
	if cmd != "mm" || strings.Join(args, " ") != "-p env run python server_core.py" {
		t.Errorf("prefixed: %q %v", cmd, args)
	}
	cmd, args = childCommand(nil, []string{"python", "server_core.py", "--no-react"})
	if cmd != "python" || strings.Join(args, " ") != "server_core.py --no-react" {
		t.Errorf("system python: %q %v", cmd, args)
	}
}

func TestSystemPythonHasNoPrefixAndSkipsTheGPUGate(t *testing.T) {
	root, _ := paths.New(t.TempDir())
	c, err := Config{Root: root, SystemPython: true, CommandPrefix: []string{"ignored"}}.normalized()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.CommandPrefix) != 0 || !c.SkipGPUCheck {
		t.Errorf("prefix=%v skipGPU=%v", c.CommandPrefix, c.SkipGPUCheck)
	}
}
