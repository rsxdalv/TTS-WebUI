// Package compare replays one request against the real Next route and the Go
// route and reports every difference in status, key headers and body bytes.
// See documentation/core-rewrite/CONTRACTS.md, section 8.
package compare

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// KeyHeaders are the response headers compared for every case.
var KeyHeaders = []string{
	"Content-Type", "Content-Length", "Content-Disposition", "Cache-Control",
	"Accept-Ranges", "Content-Range", "Location",
}

// Case is one request to replay against both servers.
type Case struct {
	Name   string      `json:"name"`
	Method string      `json:"method"` // default GET
	Path   string      `json:"path"`   // raw, sent as written (keeps "..", "%2e%2e", "%2f")
	Header http.Header `json:"header,omitempty"`
	Body   []byte      `json:"body,omitempty"`
	// Ignore allows single headers to differ. Each rule needs a reason, so a
	// loose test case is visible in review.
	Ignore []IgnoreRule `json:"ignore,omitempty"`
	// IgnoreBody, when not empty, is the reason the bodies may differ. Status
	// and headers are still compared.
	IgnoreBody string `json:"ignoreBody,omitempty"`
	// BodyFrom, when set, supplies the request body instead of Body. Use it
	// for large uploads, so the body is never held in memory.
	BodyFrom func() io.Reader `json:"-"`
}

// IgnoreRule allows one header to differ.
type IgnoreRule struct {
	Header string `json:"header"`
	Reason string `json:"reason"`
}

// Response is what one server answered.
type Response struct {
	Status    int
	Header    http.Header
	BodyLen   int64
	BodySum   string
	Body      []byte // the first MaxKeep bytes, for readable diffs
	FirstByte time.Duration
	Total     time.Duration
	Err       error
}

// MaxKeep is how many body bytes are kept in a Response.
const MaxKeep = 64 << 10

// Result is the outcome of one case.
type Result struct {
	Case  Case
	Next  Response
	Go    Response
	Diffs []string
}

// OK reports whether Next and Go answered the same.
func (r Result) OK() bool { return len(r.Diffs) == 0 }

// Do sends the case to base (for example "http://127.0.0.1:34000").
func Do(ctx context.Context, client *http.Client, base string, c Case) Response {
	var resp Response
	u, err := url.Parse(base + c.Path)
	if err != nil {
		resp.Err = err
		return resp
	}
	method := c.Method
	if method == "" {
		method = http.MethodGet
	}
	var body io.Reader
	if c.BodyFrom != nil {
		body = c.BodyFrom()
	} else if c.Body != nil {
		body = bytes.NewReader(c.Body)
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		resp.Err = err
		return resp
	}
	for k, vs := range c.Header {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	start := time.Now()
	res, err := client.Do(req)
	if err != nil {
		resp.Err = err
		return resp
	}
	defer res.Body.Close()
	resp.Status = res.StatusCode
	resp.Header = res.Header
	var h hash.Hash = sha256.New()
	buf := make([]byte, 32<<10)
	for {
		n, rerr := res.Body.Read(buf)
		if n > 0 {
			if resp.BodyLen == 0 {
				resp.FirstByte = time.Since(start)
			}
			resp.BodyLen += int64(n)
			h.Write(buf[:n])
			if room := MaxKeep - len(resp.Body); room > 0 {
				resp.Body = append(resp.Body, buf[:min(n, room)]...)
			}
		}
		if rerr != nil {
			if rerr != io.EOF {
				resp.Err = rerr
			}
			break
		}
	}
	resp.Total = time.Since(start)
	resp.BodySum = hex.EncodeToString(h.Sum(nil))
	return resp
}

// NewClient returns a client that never follows redirects and never
// compresses, so what is sent and what comes back is exactly what is seen.
func NewClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{DisableCompression: true, DisableKeepAlives: true},
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// Run replays each case against nextBase and goBase.
func Run(ctx context.Context, nextBase, goBase string, cases []Case) []Result {
	client := NewClient()
	out := make([]Result, 0, len(cases))
	for _, c := range cases {
		r := Result{Case: c}
		r.Next = Do(ctx, client, nextBase, c)
		r.Go = Do(ctx, client, goBase, c)
		r.Diffs = Diff(c, r.Next, r.Go)
		out = append(out, r)
	}
	return out
}

// Diff lists every difference between the Next and Go answers.
func Diff(c Case, n, g Response) []string {
	var d []string
	if n.Err != nil || g.Err != nil {
		if (n.Err == nil) != (g.Err == nil) {
			d = append(d, fmt.Sprintf("transport error: next=%v go=%v", n.Err, g.Err))
		}
		return d
	}
	if n.Status != g.Status {
		d = append(d, fmt.Sprintf("status: next=%d go=%d", n.Status, g.Status))
	}
	ignored := map[string]bool{}
	for _, r := range c.Ignore {
		if r.Reason == "" {
			d = append(d, "ignore rule for "+r.Header+" has no reason")
			continue
		}
		ignored[http.CanonicalHeaderKey(r.Header)] = true
	}
	for _, name := range KeyHeaders {
		if ignored[name] {
			continue
		}
		nv, gv := strings.Join(n.Header.Values(name), ", "), strings.Join(g.Header.Values(name), ", ")
		if nv != gv {
			d = append(d, fmt.Sprintf("header %s: next=%q go=%q", name, nv, gv))
		}
	}
	if c.IgnoreBody == "" && (n.BodyLen != g.BodyLen || n.BodySum != g.BodySum) {
		d = append(d, fmt.Sprintf("body: next=%d bytes sha256=%.12s go=%d bytes sha256=%.12s%s",
			n.BodyLen, n.BodySum, g.BodyLen, g.BodySum, firstDiff(n.Body, g.Body)))
	}
	return d
}

func firstDiff(a, b []byte) string {
	i := 0
	for i < len(a) && i < len(b) && a[i] == b[i] {
		i++
	}
	if i == len(a) && i == len(b) {
		return fmt.Sprintf(" (differs after the first %d bytes)", MaxKeep)
	}
	snip := func(x []byte) string {
		if i >= len(x) {
			return "<end>"
		}
		return fmt.Sprintf("%q", x[i:min(i+40, len(x))])
	}
	return fmt.Sprintf(", first difference at byte %d: next=%s go=%s", i, snip(a), snip(b))
}

// Report writes results for a terminal and returns how many cases failed.
func Report(w io.Writer, results []Result) int {
	failed := 0
	for _, r := range results {
		if r.OK() {
			fmt.Fprintf(w, "ok    %s (%d)\n", r.Case.Name, r.Next.Status)
			continue
		}
		failed++
		fmt.Fprintf(w, "FAIL  %s\n", r.Case.Name)
		for _, d := range r.Diffs {
			fmt.Fprintf(w, "        %s\n", d)
		}
	}
	fmt.Fprintf(w, "%d cases, %d failed\n", len(results), failed)
	return failed
}
