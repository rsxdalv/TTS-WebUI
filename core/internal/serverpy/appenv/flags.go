package appenv

// Flags are the server.py command-line switches (matched by exact string, as
// `"--x" in sys.argv` does). Unknown arguments are ignored.
type Flags struct {
	Share       bool
	Docker      bool
	NoTreeProxy bool
	NoReact     bool
	NoDatabase  bool // accepted, ignored (the database start is disabled in server.py)
}

// Args is the command line that ParseFlags turns back into f (--no-database
// is left out: it has no effect).
func (f Flags) Args() []string {
	var args []string
	for _, s := range []struct {
		on   bool
		name string
	}{{f.NoReact, "--no-react"}, {f.Share, "--share"}, {f.Docker, "--docker"}, {f.NoTreeProxy, "--no-tree-proxy"}} {
		if s.on {
			args = append(args, s.name)
		}
	}
	return args
}

// ParseFlags scans args (without the program name).
func ParseFlags(args []string) Flags {
	var f Flags
	for _, a := range args {
		switch a {
		case "--share":
			f.Share = true
		case "--docker":
			f.Docker = true
		case "--no-tree-proxy":
			f.NoTreeProxy = true
		case "--no-react":
			f.NoReact = true
		case "--no-database":
			f.NoDatabase = true
		}
	}
	return f
}
