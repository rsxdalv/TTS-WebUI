// Command extcatalogd serves only the extcatalog API, for trying the extension
// list tabs and the Custom Extensions Installer without the launcher:
//
//	extcatalogd -root <project root> -addr 127.0.0.1:18082 -token dev -python <python.exe>
//	TTS_CORE_URL=http://127.0.0.1:18082 TTS_CORE_TOKEN=dev python ...
package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/rsxdalv/TTS-WebUI/core/internal/serverpy/extcatalog"
)

func main() {
	root := flag.String("root", ".", "project root (holds extensions.json, .venvs/, installer_scripts/)")
	addr := flag.String("addr", "127.0.0.1:18082", "listen address (loopback only)")
	token := flag.String("token", "dev", "shared secret for X-TTS-Core-Token")
	python := flag.String("python", "", "python used for pip installs (default: python on PATH)")
	flag.Parse()

	mux := http.NewServeMux()
	mux.Handle(extcatalog.Prefix, extcatalog.New(extcatalog.Options{
		Root: *root, Token: *token, Python: *python, Logf: log.Printf,
	}).Handler())
	srv := &http.Server{Addr: *addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	log.Printf("extcatalogd: %s on %s", *root, *addr)
	log.Fatal(srv.ListenAndServe())
}
