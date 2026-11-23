// Command pipd serves only the piplist API, for trying Installed Packages
// without the launcher:
//
//	pipd -root <project root> -addr 127.0.0.1:18084 -token dev -python <python.exe>
//	TTS_CORE_URL=http://127.0.0.1:18084 TTS_CORE_TOKEN=dev python ...
package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/piplist"
)

func main() {
	root := flag.String("root", ".", "project root (working directory of pip)")
	addr := flag.String("addr", "127.0.0.1:18084", "listen address (loopback only)")
	token := flag.String("token", "dev", "shared secret for X-TTS-Core-Token")
	python := flag.String("python", "python", "python to run pip with")
	flag.Parse()

	mux := http.NewServeMux()
	mux.Handle(piplist.Path, piplist.New(piplist.Options{Root: *root, Token: *token, Python: *python, Logf: log.Printf}).Handler())
	srv := &http.Server{Addr: *addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	log.Printf("pipd: %s on %s", *root, *addr)
	log.Fatal(srv.ListenAndServe())
}
