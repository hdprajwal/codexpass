package proxy

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// sseWriter writes Server-Sent Events and flushes after each.
type sseWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
}

// newSSEWriter sets SSE headers. ok is false if the writer cannot flush.
func newSSEWriter(w http.ResponseWriter) (*sseWriter, bool) {
	fl, ok := w.(http.Flusher)
	if !ok {
		return nil, false
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	return &sseWriter{w: w, flusher: fl}, true
}

// send writes one `data: <json>` event.
func (s *sseWriter) send(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(s.w, "data: %s\n\n", b); err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}

// done writes the terminating `data: [DONE]` event.
func (s *sseWriter) done() error {
	if _, err := fmt.Fprint(s.w, "data: [DONE]\n\n"); err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}
