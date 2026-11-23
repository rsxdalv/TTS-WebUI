// Command logd serves only the logview API, for trying the Log Viewer without
// the launcher:
//
//	logd -root <project root> -addr 127.0.0.1:18085 -token dev
//	TTS_CORE_URL=http://127.0.0.1:18085 TTS_CORE_TOKEN=dev python ...
package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/logview"
)

func main() {
	root := flag.String("root", ".", "project root (logs are in <root>/installer_scripts/logs)")
	addr := flag.String("addr", "127.0.0.1:18085", "listen address (loopback only)")
	token := flag.String("token", "dev", "shared secret for X-TTS-Core-Token")
	flag.Parse()

	mux := http.NewServeMux()
	mux.Handle(logview.Prefix, logview.New(logview.Options{Root: *root, Token: *token, Logf: log.Printf}).Handler())
	srv := &http.Server{Addr: *addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	log.Printf("logd: %s on %s", *root, *addr)
	log.Fatal(srv.ListenAndServe())
}
