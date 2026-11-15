// Command tts-webui is the Go core of TTS WebUI.
//
// Subcommands live in their own cmd_<name>.go files and register themselves
// from init(), so checkpoints that add commands never edit this file.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sort"

	"github.com/rsxdalv/TTS-WebUI/core/internal/version"
)

// command is one subcommand. run receives the arguments after the command name
// and a context that is cancelled on Ctrl+C.
type command struct {
	name    string
	summary string
	run     func(ctx context.Context, args []string) error
}

var commands = map[string]command{}

// register adds a subcommand. Call it from init() in cmd_<name>.go.
func register(c command) {
	if _, dup := commands[c.name]; dup {
		panic("duplicate command: " + c.name)
	}
	commands[c.name] = c
}

func main() {
	fs := flag.NewFlagSet("tts-webui", flag.ExitOnError)
	showVersion := fs.Bool("version", false, "print version information and exit")
	fs.Usage = func() { usage(fs) }
	_ = fs.Parse(os.Args[1:])

	if *showVersion {
		fmt.Printf("tts-webui %s\n", version.String())
		return
	}
	if fs.NArg() == 0 {
		fs.Usage()
		return
	}

	name := fs.Arg(0)
	c, ok := commands[name]
	if !ok {
		fmt.Fprintf(os.Stderr, "tts-webui: unknown command %q\n\n", name)
		fs.Usage()
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := c.run(ctx, fs.Args()[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "tts-webui %s: %v\n", name, err)
		os.Exit(1)
	}
}

func usage(fs *flag.FlagSet) {
	out := fs.Output()
	fmt.Fprintf(out, "Usage: tts-webui [flags] <command> [args]\n\nCommands:\n")
	names := make([]string, 0, len(commands))
	for n := range commands {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		fmt.Fprintf(out, "  %-12s %s\n", n, commands[n].summary)
	}
	fmt.Fprintf(out, "\nFlags:\n")
	fs.PrintDefaults()
}
