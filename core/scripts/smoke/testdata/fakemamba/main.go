// fakemamba is a stub micromamba binary used in the M1 smoke test.
// When called as "fakemamba -p <envPath> run node <script> --silent", it
// writes installer_scripts/.git_version and exits 0, simulating a successful
// init_app.js run.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	fmt.Println("fakemamba: " + strings.Join(os.Args[1:], " "))
	cwd, _ := os.Getwd()
	for i, a := range os.Args {
		if a == "run" && i+2 < len(os.Args) && os.Args[i+1] == "node" {
			marker := filepath.Join(cwd, "installer_scripts", ".git_version")
			if err := os.WriteFile(marker, []byte("smoke-test-version\n"), 0o644); err != nil {
				fmt.Fprintf(os.Stderr, "fakemamba: write .git_version: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("fakemamba: init_app.js complete")
			return
		}
	}
}
