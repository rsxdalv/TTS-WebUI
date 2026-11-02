package server

import (
	"fmt"
	"net/http"
)

// SSE streams server-sent events to the client until the events channel is
// closed or the request context is cancelled.
//
// Each string received from events is sent as an SSE data line.
// The caller is responsible for formatting (e.g., JSON-encoding) the event
// payload before sending it on the channel.
func SSE(w http.ResponseWriter, r *http.Request, events <-chan string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK) // send headers before any event
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case ev, ok := <-events:
			if !ok {
				return
			}
			_, _ = fmt.Fprintf(w, "data: %s\n\n", ev)
			flusher.Flush()
		}
	}
}

// SSEStart writes SSE response headers and flushes them immediately so the
// browser's EventSource does not hang waiting for the first event.
// Returns false (and writes a 500) if the ResponseWriter does not support
// flushing.
func SSEStart(w http.ResponseWriter) (http.Flusher, bool) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return nil, false
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()
	return flusher, true
}

// WriteSSEDone writes a terminal "event: done" SSE event with the process exit
// code. Call this after all data lines are sent, before returning the handler.
func WriteSSEDone(w http.ResponseWriter, flusher http.Flusher, exitCode int) {
	_, _ = fmt.Fprintf(w, "event: done\ndata: {\"exitCode\":%d}\n\n", exitCode)
	flusher.Flush()
}
