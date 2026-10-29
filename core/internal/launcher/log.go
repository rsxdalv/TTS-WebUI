// Package launcher implements the step-by-step installation HTTP API
// used by the Go launcher (tts-webui launch).
package launcher

import "sync"

const logCap = 4096
const subBufSize = 256

// Log is a thread-safe, subscribable ring-buffer of lines.
// It drives SSE progress streams for steps that are not backed by a subprocess
// (e.g. the download step).
type Log struct {
	mu   sync.Mutex
	buf  [logCap]string
	head int
	n    int
	subs []*logSub
}

type logSub struct{ ch chan string }

// AddLine appends a line and fans it out to all active subscribers.
func (l *Log) AddLine(line string) {
	l.mu.Lock()
	l.buf[l.head] = line
	l.head = (l.head + 1) % logCap
	if l.n < logCap {
		l.n++
	}
	subs := make([]*logSub, len(l.subs))
	copy(subs, l.subs)
	l.mu.Unlock()
	for _, s := range subs {
		select {
		case s.ch <- line:
		default: // slow subscriber — drop
		}
	}
}

// Lines returns all buffered lines in chronological order.
func (l *Log) Lines() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.n == 0 {
		return nil
	}
	out := make([]string, l.n)
	tail := (l.head - l.n + logCap) % logCap
	for i := range out {
		out[i] = l.buf[(tail+i)%logCap]
	}
	return out
}

// Subscribe returns a channel for future lines and an unsubscribe function.
func (l *Log) Subscribe() (<-chan string, func()) {
	sub := &logSub{ch: make(chan string, subBufSize)}
	l.mu.Lock()
	l.subs = append(l.subs, sub)
	l.mu.Unlock()
	cancel := func() {
		l.mu.Lock()
		defer l.mu.Unlock()
		for i, s := range l.subs {
			if s == sub {
				l.subs = append(l.subs[:i], l.subs[i+1:]...)
				close(sub.ch)
				return
			}
		}
	}
	return sub.ch, cancel
}
