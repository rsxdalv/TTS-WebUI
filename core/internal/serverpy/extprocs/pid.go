package extprocs

import (
	"os/exec"
	"reflect"

	"github.com/rsxdalv/TTS-WebUI/core/internal/procs"
)

type pidProvider interface{ Pid() int }

// pidOf returns the child's pid. procs.Proc has no Pid() at the pinned
// commit, so fall back to reading its unexported cmd field. If a
// `func (p *Proc) Pid() int` is added to procs, it is used automatically and
// the reflect fallback becomes dead code that can be deleted.
func pidOf(p *procs.Proc) (pid int) {
	if pp, ok := any(p).(pidProvider); ok {
		return pp.Pid()
	}
	defer func() {
		if recover() != nil {
			pid = 0
		}
	}()
	f := reflect.ValueOf(p).Elem().FieldByName("cmd")
	if !f.IsValid() || f.IsNil() {
		return 0
	}
	cmd := (*exec.Cmd)(f.UnsafePointer())
	if cmd.Process == nil {
		return 0
	}
	return cmd.Process.Pid
}
