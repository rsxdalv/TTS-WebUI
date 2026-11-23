// Command gosrv serves one install root's React UI on a port, for the browser
// check in ../../browser. It is a test tool: it is not part of the app.
//
//	gosrv -root C:\scratch\install -port 34012 -routes all
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
	root := flag.String("root", "", "install root (it holds react-ui/out and react-ui/public)")
	port := flag.Int("port", 0, "public port")
	routes := flag.String("routes", "", "TTS_GO_ROUTES value (empty: every claim)")
	coreURL := flag.String("core-url", "", "control API address for the core routes (dev: a pipd or logd)")
	coreToken := flag.String("core-token", "dev", "its X-TTS-Core-Token")
	flag.Parse()
	set, err := switches.Parse(func(k string) (string, bool) {
		if k == switches.EnvRoutes {
			return *routes, *routes != ""
		}
		return "", false
	})
	if err != nil || *root == "" || *port == 0 {
		fmt.Fprintln(os.Stderr, "usage: gosrv -root DIR -port N [-routes LIST]", err)
		os.Exit(2)
	}
	core := &react_ui_backend.Core{}
	if *coreURL != "" {
		core.Set(*coreURL, *coreToken)
	}
	h, err := react_ui_backend.NewHandler(react_ui_backend.Config{Root: *root, Port: *port, Routes: set, Core: core})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	fmt.Println("gosrv listening on 127.0.0.1:" + strconv.Itoa(*port))
	fmt.Fprintln(os.Stderr, http.ListenAndServe("127.0.0.1:"+strconv.Itoa(*port), h))
	os.Exit(1)
}
