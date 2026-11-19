// fakepython is a stub Python binary used in the M1 smoke test.
// It serves HTTP on 127.0.0.1:7770 (the hard-coded Gradio port that
// Supervisor.AppLink() polls). If the port is already in use (e.g. a second
// instance started for the extension manager), it exits 0 quietly.
// It prints one log line to stdout so the SSE stream test can assert a log line.
package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const gradioPort = "7770"

func main() {
	fmt.Fprintln(os.Stdout, "fakepython: started (M1 smoke stub)")

	ln, err := net.Listen("tcp", "127.0.0.1:"+gradioPort)
	if err != nil {
		// Port already taken — another instance is serving. Exit quietly.
		fmt.Fprintf(os.Stderr, "fakepython: port %s in use, exiting\n", gradioPort)
		os.Exit(0)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintln(w, "fakepython: ok")
	})
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}

	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "fakepython: serve error: %v\n", err)
		}
	}()

	fmt.Fprintf(os.Stdout, "fakepython: listening on 127.0.0.1:%s\n", gradioPort)

	// Wait for kill signal.
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	<-ch
}
