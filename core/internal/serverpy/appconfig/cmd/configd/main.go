// Command configd serves only the appconfig API (the whole document and the
// single-setting operations), for trying bridge.config without the launcher:
//
//	configd -root <dir holding config.json> -addr 127.0.0.1:18083 -token dev
//	TTS_CORE_URL=http://127.0.0.1:18083 TTS_CORE_TOKEN=dev python ...
package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/appconfig"
)

func main() {
	root := flag.String("root", ".", "directory holding config.json")
	addr := flag.String("addr", "127.0.0.1:18083", "listen address (loopback only)")
	token := flag.String("token", "dev", "shared secret for X-TTS-Core-Token")
	flag.Parse()

	store := appconfig.New(appconfig.Options{Root: *root, Token: *token, Logf: log.Printf})
	mux := http.NewServeMux()
	mux.Handle(appconfig.Path, store.Handler())
	mux.Handle(appconfig.OpsPrefix, store.OpsHandler())
	srv := &http.Server{Addr: *addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	log.Printf("configd: %s on %s", *root, *addr)
	log.Fatal(srv.ListenAndServe())
}
