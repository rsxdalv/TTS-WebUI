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
