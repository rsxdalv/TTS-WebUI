// Fakepy stands in for python: it reports its args/env as one JSON line,
// listens on GRADIO_SERVER_PORT and sleeps. Behaviour knobs (env):
// FAKE_DELAY_MS before listening, FAKE_NEVER_LISTEN, FAKE_EXIT_AFTER_MS,
// FAKE_EXIT_CODE.
package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"
	"time"
)

func ms(k string) time.Duration {
	n, _ := strconv.Atoi(os.Getenv(k))
	return time.Duration(n) * time.Millisecond
}

func main() {
	keys := []string{"GRADIO_SERVER_PORT", "GRADIO_ROOT_PATH", "TTS_WEBUI_EXTENSION_PACKAGE", "OPENAI_PROXY_HOST", "TTS_CORE_TOKEN", "TTS_CORE_URL", "BASE_MARK", "DELTA_MARK"}
	env := map[string]string{}
	for _, k := range keys {
		if v, ok := os.LookupEnv(k); ok {
			env[k] = v
		}
	}
	exe, _ := os.Executable()
	cwd, _ := os.Getwd()
	b, _ := json.Marshal(map[string]any{"pid": os.Getpid(), "args": os.Args[1:], "env": env, "exe": exe, "cwd": cwd})
	fmt.Println(string(b))

	if d := ms("FAKE_EXIT_AFTER_MS"); d > 0 {
		go func() {
			time.Sleep(d)
			c, _ := strconv.Atoi(os.Getenv("FAKE_EXIT_CODE"))
			os.Exit(c)
		}()
	}
	if os.Getenv("FAKE_NEVER_LISTEN") == "" {
		time.Sleep(ms("FAKE_DELAY_MS"))
		l, err := net.Listen("tcp", "127.0.0.1:"+os.Getenv("GRADIO_SERVER_PORT"))
		if err != nil {
			fmt.Fprintln(os.Stderr, "listen:", err)
			os.Exit(3)
		}
		go func() {
			for {
				c, err := l.Accept()
				if err != nil {
					return
				}
				c.Close()
			}
		}()
	}
	time.Sleep(time.Hour)
}
