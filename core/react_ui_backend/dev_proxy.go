package react_ui_backend

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
)

// newDevProxy forwards to a Vite dev server (pages, modules and the HMR
// WebSocket). The upstream sees its own host, as for a browser that opened it
// directly; Vite checks the Host header of what it serves.
func newDevProxy(target string) (http.Handler, error) {
	u, err := url.Parse(target)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("the frontend dev server %q must look like http://127.0.0.1:3002", target)
	}
	return &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(u)
			pr.Out.Host = u.Host
			// Vite refuses a cross-origin Origin on its WebSocket: give it its own.
			if pr.Out.Header.Get("Origin") != "" {
				pr.Out.Header.Set("Origin", u.Scheme+"://"+u.Host)
			}
		},
		FlushInterval: -1,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			http.Error(w, "The frontend dev server is not answering ("+u.Host+"): start it with `npm run dev` in frontend/. "+err.Error(), http.StatusBadGateway)
		},
	}, nil
}
