// Command compare replays cases from a JSON file against Next and Go.
//
//	compare -next http://127.0.0.1:34001 -go http://127.0.0.1:34002 cases.json
//
// The file is a JSON array of compare.Case. Exit status 1 means a difference.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/rsxdalv/TTS-WebUI/core/react_ui_backend/compare"
)

func main() {
	next := flag.String("next", "", "base URL of Next")
	goBase := flag.String("go", "", "base URL of the Go server")
	flag.Parse()
	if *next == "" || *goBase == "" || flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: compare -next URL -go URL cases.json")
		os.Exit(2)
	}
	data, err := os.ReadFile(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	var cases []compare.Case
	if err := json.Unmarshal(data, &cases); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if compare.Report(os.Stdout, compare.Run(context.Background(), *next, *goBase, cases)) > 0 {
		os.Exit(1)
	}
}
