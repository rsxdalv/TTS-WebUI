// fakepython is a stub Python binary used in the M1 smoke test.
// It stays alive (sleeping) until killed, simulating a running Gradio server.
// It prints one log line to stdout so the SSE stream test can assert a log line.
package main

import (
	"fmt"
	"time"
)

func main() {
	fmt.Println("fakepython: started (M1 smoke stub)")
	// Stay alive until killed by the supervisor or the test.
	time.Sleep(120 * time.Second)
}
