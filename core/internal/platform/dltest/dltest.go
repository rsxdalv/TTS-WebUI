// Package dltest is a fake download server for the tests of the platforms that
// fetch pinned archives (uv, audiocpp, onnx): a server that answers the way a real
// one fails, so each platform can show that a download that is not exactly the
// pinned file is refused and leaves nothing behind.
package dltest

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
)

// Mode is how the server answers.
type Mode string

const (
	OK         Mode = "ok"          // the file
	NotFound   Mode = "404"         // HTTP 404
	Cut        Mode = "cut"         // the whole length is announced, 40% is sent, the connection is closed
	CutChunked Mode = "cut-chunked" // no length (chunked), 40% is sent, the connection is closed
	Reset      Mode = "reset"       // like Cut, but with a TCP reset
	Short      Mode = "short"       // a clean answer with 40% of the file (a truncated file)
	Empty      Mode = "empty"       // a clean answer with no bytes
)

// Failures are the modes a download must refuse.
var Failures = []Mode{NotFound, Cut, CutChunked, Reset, Short, Empty}

// Handler serves body in the mode.
func Handler(body []byte, m Mode) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cut := len(body) * 4 / 10
		switch m {
		case NotFound:
			http.NotFound(w, r)
		case Cut, Reset, CutChunked:
			if m != CutChunked {
				w.Header().Set("Content-Length", fmt.Sprint(len(body)))
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(body[:cut])
			w.(http.Flusher).Flush()
			conn, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				return
			}
			if tcp, ok := conn.(*net.TCPConn); ok && m == Reset {
				_ = tcp.SetLinger(0)
			}
			_ = conn.Close()
		case Short:
			w.Header().Set("Content-Length", fmt.Sprint(cut))
			_, _ = w.Write(body[:cut])
		case Empty:
			w.Header().Set("Content-Length", "0")
		default:
			w.Header().Set("Content-Length", fmt.Sprint(len(body)))
			_, _ = w.Write(body)
		}
	}
}

// Server is a plain HTTP server that serves body in the mode; it is closed when
// the test ends.
func Server(t *testing.T, body []byte, m Mode) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(Handler(body, m))
	t.Cleanup(srv.Close)
	return srv
}

// Switch is a server whose mode a test can change between two installs: the first
// install meets the failure, the retry the real file.
type Switch struct {
	*httptest.Server
	mode atomic.Value // Mode
}

// NewSwitch starts a server that begins in mode m. Set changes it.
func NewSwitch(t *testing.T, body []byte, m Mode) *Switch {
	t.Helper()
	s := &Switch{}
	s.mode.Store(m)
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Handler(body, s.mode.Load().(Mode))(w, r)
	}))
	t.Cleanup(s.Server.Close)
	return s
}

// Set changes the mode of the next answers.
func (s *Switch) Set(m Mode) { s.mode.Store(m) }

// AssertEmpty fails the test when dir holds anything: a refused download must leave no
// file, no partial file and no half-unpacked folder.
func AssertEmpty(t *testing.T, dir string) {
	t.Helper()
	var left []string
	var walk func(d string)
	walk = func(d string) {
		ents, _ := os.ReadDir(d)
		for _, e := range ents {
			p := d + string(os.PathSeparator) + e.Name()
			left = append(left, p)
			if e.IsDir() {
				walk(p)
			}
		}
	}
	walk(dir)
	if len(left) > 0 {
		t.Errorf("left behind in %s: %v", dir, left)
	}
}

// RealSHA256 downloads url (streamed, nothing is stored) and returns the sha256 and
// the size of what the server really serves. It is for the opt-in real tests that
// check a pin against the download it names.
func RealSHA256(t *testing.T, url string) (sum string, size int64) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: HTTP %d", url, resp.StatusCode)
	}
	h := sha256.New()
	size, err = io.Copy(h, resp.Body)
	if err != nil {
		t.Fatalf("GET %s: after %d bytes: %v", url, size, err)
	}
	return hex.EncodeToString(h.Sum(nil)), size
}
