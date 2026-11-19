//go:build windows

package installer_scripts

import "os/exec"

// comSpec returns the command interpreter, as Node's exec does.
func comSpec(env []string) string {
	if v := envValue(env, "ComSpec"); v != "" {
		return v
	}
	return "cmd.exe"
}

// shellCmd builds the process for a shell line. The arguments are placeholders;
// setShellLine supplies the exact command line.
func shellCmd(_ string, env []string) *exec.Cmd {
	return exec.Command(comSpec(env))
}

// setShellLine passes line to cmd.exe unescaped, the way Node's exec does with
// windowsVerbatimArguments. Go's own argument quoting would turn inner quotes
// into \" which cmd.exe does not understand. Call it after procs.NewTree, which
// creates SysProcAttr.
func setShellLine(cmd *exec.Cmd, line string) {
	cmd.SysProcAttr.CmdLine = `cmd.exe /d /s /c "` + line + `"`
}
