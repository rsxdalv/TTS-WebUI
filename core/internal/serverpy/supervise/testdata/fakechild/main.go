// fakechild stands in for `python server.py` and `npm start` in the supervise tests. The supervisor's
// command prefix points at this binary, so it receives e.g.
// "python server.py --no-react" as its arguments.
//
// Role is derived from the arguments (npm -> react, otherwise gradio). Ports:
// react from "-p N", gradio from env FAKE_GRADIO_PORT.
//
// Behaviour per role R (gradio, react) via env:
//
//	FAKE_<R>_NOLISTEN=1    never listen
//	FAKE_<R>_DELAY=300ms   wait before listening
//	FAKE_<R>_EXIT=N        print FAKE_<R>_LINES lines (default 3), exit N
//	FAKE_<R>_READY_FILE=p  answer HTTP 502 until file p exists
//
// FAKE_DUMP_DIR, when set, receives <role>.json with the args and env.
package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func main() {
	args := os.Args[1:]
	role, port := detect(args)
	up := strings.ToUpper(role)

	if dir := os.Getenv("FAKE_DUMP_DIR"); dir != "" {
		env := map[string]string{}
		for _, kv := range os.Environ() {
			if k, v, ok := strings.Cut(kv, "="); ok && k != "" {
				env[k] = v
			}
		}
		b, _ := json.Marshal(map[string]any{"role": role, "args": args, "env": env, "port": port})
		_ = os.WriteFile(filepath.Join(dir, role+".json"), b, 0o644)
	}

	fmt.Printf("fakechild[%s]: start\n", role)

	if code := os.Getenv("FAKE_" + up + "_EXIT"); code != "" {
		n, _ := strconv.Atoi(os.Getenv("FAKE_" + up + "_LINES"))
		if n == 0 {
			n = 3
		}
		for i := 1; i <= n; i++ {
			fmt.Printf("line %d\n", i)
		}
		c, _ := strconv.Atoi(code)
		os.Exit(c)
	}
	if d, err := time.ParseDuration(os.Getenv("FAKE_" + up + "_DELAY")); err == nil {
		time.Sleep(d)
	}
	if os.Getenv("FAKE_"+up+"_NOLISTEN") == "" && port > 0 {
		readyFile := os.Getenv("FAKE_" + up + "_READY_FILE")
		ln, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(port))
		if err != nil {
			fmt.Fprintln(os.Stderr, "listen:", err)
			os.Exit(1)
		}
		go http.Serve(ln, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			if readyFile != "" {
				if _, err := os.Stat(readyFile); err != nil {
					http.Error(w, "upstream unavailable", http.StatusBadGateway)
					return
				}
			}
			fmt.Fprintln(w, "ok")
		}))
		fmt.Printf("fakechild[%s]: listening on %d\n", role, port)
	}
	time.Sleep(24 * time.Hour)
}

func detect(args []string) (role string, port int) {
	flagVal := func(f string) int {
		for i, a := range args {
			if a == f && i+1 < len(args) {
				n, _ := strconv.Atoi(args[i+1])
				return n
			}
		}
		return 0
	}
	switch {
	case len(args) > 0 && args[0] == "npm":
		return "react", flagVal("-p")
	}
	n, _ := strconv.Atoi(os.Getenv("FAKE_GRADIO_PORT"))
	return "gradio", n
}
