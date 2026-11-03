package procs

import "sync"

const ringCap = 4096

// ring is a fixed-capacity circular line buffer, safe for concurrent use.
type ring struct {
	mu   sync.Mutex
	buf  [ringCap]string
	head int // index of the next write slot (mod ringCap)
	n    int // number of valid entries, capped at ringCap
}

// push appends a line, overwriting the oldest entry when full.
func (r *ring) push(line string) {
	r.mu.Lock()
	r.buf[r.head] = line
	r.head = (r.head + 1) % ringCap
	if r.n < ringCap {
		r.n++
	}
	r.mu.Unlock()
}

// lines returns a snapshot of all buffered lines in chronological order.
func (r *ring) lines() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.n == 0 {
		return nil
	}
	out := make([]string, r.n)
	tail := (r.head - r.n + ringCap) % ringCap
	for i := range out {
		out[i] = r.buf[(tail+i)%ringCap]
	}
	return out
}
