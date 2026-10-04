package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/jameynakama/randsense/internal/live"
)

// StarsEvent is a sentence's new star count.
type StarsEvent struct {
	ID    string `json:"id"`
	Count int32  `json:"count"`
}

// publish sends v to every open stream as event name.
func (h *Handler) publish(name string, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		log.Printf("publish %s: %v", name, err)
		return
	}
	h.hub.Publish(live.Event{Name: name, Data: data})
}

// stream sends sentence and stars events as Server-Sent Events. There's no
// replay: a reconnecting browser refetches the latest sentences.
func (h *Handler) stream(w http.ResponseWriter, r *http.Request) {
	// Subscribe before answering, so a client holding the headers misses
	// nothing published after.
	events, unsubscribe := h.hub.Subscribe()
	defer unsubscribe()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	// nginx buffers responses unless told not to.
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	rc := http.NewResponseController(w)
	if err := rc.Flush(); err != nil {
		return
	}

	keepalive := time.NewTicker(h.keepalive)
	defer keepalive.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case e, ok := <-events:
			if !ok {
				return // dropped for falling behind; the browser reconnects
			}
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", e.Name, e.Data)
		case <-keepalive.C:
			fmt.Fprint(w, ":\n\n")
		}
		if err := rc.Flush(); err != nil {
			return
		}
	}
}
