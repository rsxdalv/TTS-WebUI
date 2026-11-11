// Child is a test helper binary for the procs package orphan-kill test.
//
// When run with no arguments, it spawns a "grandchild" subprocess (itself
// with arg "grandchild") then prints the TCP port the grandchild is listening
// on and sleeps.
//
// When run with "grandchild" as the first argument, it listens on a random
// TCP port, prints the port, and sleeps until killed.
package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/exec"
	"time"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "grandchild" {
		runGrandchild()
		return
	}
	runParent()
}

func runGrandchild() {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Fprintln(os.Stderr, "grandchild listen:", err)
		os.Exit(1)
	}
	port := l.Addr().(*net.TCPAddr).Port
	fmt.Println(port) // report port to parent (and then to test via parent)
	os.Stdout.Sync()
	// Keep listening so the port stays open while alive.
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()
	time.Sleep(24 * time.Hour)
}

func runParent() {
	cmd := exec.Command(os.Args[0], "grandchild")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Fprintln(os.Stderr, "stdout pipe:", err)
		os.Exit(1)
	}
	if err := cmd.Start(); err != nil {
		fmt.Fprintln(os.Stderr, "start grandchild:", err)
		os.Exit(1)
	}

	// Read grandchild's port and relay it to our stdout.
	scanner := bufio.NewScanner(stdout)
	if scanner.Scan() {
		fmt.Println(scanner.Text())
		os.Stdout.Sync()
	}

	time.Sleep(24 * time.Hour)
}
