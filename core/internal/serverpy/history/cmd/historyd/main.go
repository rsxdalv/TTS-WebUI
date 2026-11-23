// Command historyd serves only the history API, for trying the Gallery History
// tab without the whole launcher:
//
//	historyd -root <project root> -addr 127.0.0.1:18081 -token dev
//	TTS_CORE_URL=http://127.0.0.1:18081 TTS_CORE_TOKEN=dev python ...
package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/history"
)

func main() {
	root := flag.String("root", ".", "project root (holds outputs/, favorites/, collections/)")
	addr := flag.String("addr", "127.0.0.1:18081", "listen address (loopback only)")
	token := flag.String("token", "dev", "shared secret for X-TTS-Core-Token")
	flag.Parse()

	mux := http.NewServeMux()
	mux.Handle(history.Prefix, history.New(history.Options{Root: *root, Token: *token, Logf: log.Printf}).Handler())
	srv := &http.Server{Addr: *addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	log.Printf("historyd: %s on %s", *root, *addr)
	log.Fatal(srv.ListenAndServe())
}
