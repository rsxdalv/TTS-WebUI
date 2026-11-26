// Mock of GitHub and Hugging Face for the checksum checks. Every path is
// /<host>/<path>; the real bytes are fetched once from https://<host>/<path> and
// cached, then served in the mode a rule selects:
//
//	ok         the real bytes
//	404        HTTP 404
//	cut        Content-Length of the whole file, 40% of the bytes, then a clean close (FIN)
//	cut-chunked  no Content-Length (chunked), 40% of the bytes, then a clean close without the last chunk
//	reset      Content-Length of the whole file, 40% of the bytes, then a TCP reset (RST)
//	short      200 with an honest Content-Length of a file 40% as long (a truncated file)
//	empty      200 with an empty body
//
// Rules: POST /__rule?match=<substring of the path>&mode=<mode> (mode ok clears it).
// GET /__log is the request log.
package main

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	mu    sync.Mutex
	rules = map[string]string{}
	logs  []string
	cache = os.Args[1]
)

func note(f string, a ...any) {
	mu.Lock()
	defer mu.Unlock()
	logs = append(logs, time.Now().UTC().Format("15:04:05.000")+" "+fmt.Sprintf(f, a...))
}

func fetch(path string) (string, error) {
	h := sha1.Sum([]byte(path))
	f := filepath.Join(cache, hex.EncodeToString(h[:8])+"-"+filepath.Base(path))
	if _, err := os.Stat(f); err == nil {
		return f, nil
	}
	resp, err := http.Get("https://" + strings.TrimPrefix(path, "/"))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("upstream %d", resp.StatusCode)
	}
	out, err := os.Create(f + ".tmp")
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(out, resp.Body); err != nil {
		out.Close()
		return "", err
	}
	out.Close()
	return f, os.Rename(f+".tmp", f)
}

func mode(path string) string {
	mu.Lock()
	defer mu.Unlock()
	for m, v := range rules {
		if strings.Contains(path, m) {
			return v
		}
	}
	return "ok"
}

func main() {
	os.MkdirAll(cache, 0o755)
	http.HandleFunc("/__rule", func(w http.ResponseWriter, r *http.Request) {
		m, v := r.URL.Query().Get("match"), r.URL.Query().Get("mode")
		mu.Lock()
		if v == "ok" || v == "" {
			delete(rules, m)
		} else {
			rules[m] = v
		}
		mu.Unlock()
		note("rule %q -> %s", m, v)
		fmt.Fprintln(w, "ok")
	})
	http.HandleFunc("/__log", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		fmt.Fprintln(w, strings.Join(logs, "\n"))
	})
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		m := mode(r.URL.Path)
		if m == "404" {
			note("GET %s -> 404", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		f, err := fetch(r.URL.Path)
		if err != nil {
			note("GET %s -> upstream error %v", r.URL.Path, err)
			http.Error(w, err.Error(), 502)
			return
		}
		b, _ := os.ReadFile(f)
		cut := len(b) * 4 / 10
		switch m {
		case "cut", "reset":
			w.Header().Set("Content-Length", fmt.Sprint(len(b)))
			w.WriteHeader(200)
			w.Write(b[:cut])
			w.(http.Flusher).Flush()
			conn, _, _ := w.(http.Hijacker).Hijack()
			if m == "reset" {
				conn.(*net.TCPConn).SetLinger(0)
			}
			conn.Close()
			note("GET %s -> %s: %d of %d bytes then closed", r.URL.Path, m, cut, len(b))
		case "cut-chunked":
			w.WriteHeader(200)
			w.Write(b[:cut])
			w.(http.Flusher).Flush()
			conn, _, _ := w.(http.Hijacker).Hijack()
			conn.Close()
			note("GET %s -> cut-chunked: %d of %d bytes then dropped", r.URL.Path, cut, len(b))
		case "short":
			w.Header().Set("Content-Length", fmt.Sprint(cut))
			w.Write(b[:cut])
			note("GET %s -> short: honest %d of %d bytes", r.URL.Path, cut, len(b))
		case "empty":
			w.Header().Set("Content-Length", "0")
			note("GET %s -> empty", r.URL.Path)
		default:
			w.Header().Set("Content-Length", fmt.Sprint(len(b)))
			w.Write(b)
			note("GET %s -> ok %d bytes", r.URL.Path, len(b))
		}
	})
	log.Fatal(http.ListenAndServe("127.0.0.1:18999", nil))
}
