// Command gosrv runs the Go proxy handler on one port, in front of a Next that
// is already running, for side-by-side tests with the compare command. It is a
// test tool: it is not part of the app.
//
//	gosrv -root C:\scratch\routes-sbs -port 34012 -next 127.0.0.1:34011 -routes gradio-file-proxy
package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/rsxdalv/TTS-WebUI/core/internal/switches"
	"github.com/rsxdalv/TTS-WebUI/core/react_ui_backend"
)

func main() {
	root := flag.String("root", "", "install root (Deps.Root)")
	port := flag.Int("port", 0, "public port")
	next := flag.String("next", "", "host:port of Next")
	routes := flag.String("routes", "", "TTS_GO_ROUTES value")
	flag.Parse()
	set, err := switches.Parse(func(k string) (string, bool) {
		switch k {
		case switches.EnvProxy:
			return "1", true
		case switches.EnvRoutes:
			return *routes, *routes != ""
		}
		return "", false
	})
	if err != nil || *root == "" || *port == 0 || *next == "" {
		fmt.Fprintln(os.Stderr, "usage: gosrv -root DIR -port N -next HOST:PORT [-routes LIST]", err)
		os.Exit(2)
	}
	h, err := react_ui_backend.NewHandler(react_ui_backend.Config{Root: *root, Port: *port, NextAddr: *next, Routes: set})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	fmt.Println("gosrv listening on 127.0.0.1:" + strconv.Itoa(*port))
	fmt.Fprintln(os.Stderr, http.ListenAndServe("127.0.0.1:"+strconv.Itoa(*port), h))
	os.Exit(1)
}
