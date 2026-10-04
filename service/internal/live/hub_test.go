package live_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/jameynakama/randsense/internal/live"
)

func newHub() *live.Hub {
	h := live.NewHub()
	go h.Run()
	return h
}

// receive waits briefly for the next event; ok is false once the channel
// is closed.
func receive(t *testing.T, events <-chan live.Event) (live.Event, bool) {
	t.Helper()
	select {
	case e, ok := <-events:
		return e, ok
	case <-time.After(time.Second):
		t.Fatal("expected an event or a close; got nothing")
		return live.Event{}, false
	}
}

func TestPublishReachesEverySubscriber(t *testing.T) {
	h := newHub()
	a, _ := h.Subscribe()
	b, _ := h.Subscribe()

	h.Publish(live.Event{Name: "sentence", Data: []byte(`{}`)})

	for name, events := range map[string]<-chan live.Event{"a": a, "b": b} {
		if e, ok := receive(t, events); !ok || e.Name != "sentence" {
			t.Errorf("%s: expected the sentence event; got %+v, open %v", name, e, ok)
		}
	}
}

// Review Focus 2: a stalled client mustn't stall publishing.
func TestPublishDropsASubscriberThatFallsBehind(t *testing.T) {
	h := newHub()
	slow, _ := h.Subscribe()

	// Publish returns each time even though nobody reads. The hub handles
	// one publish at a time, so once the last returns, the one before it,
	// which overflowed the buffer, has been handled.
	for i := range live.ClientBuffer + 2 {
		h.Publish(live.Event{Name: "sentence", Data: fmt.Appendf(nil, "%d", i)})
	}

	for i := range live.ClientBuffer {
		if e, ok := receive(t, slow); !ok || string(e.Data) != fmt.Sprint(i) {
			t.Fatalf("event %d: got %+v, open %v", i, e, ok)
		}
	}
	if _, ok := receive(t, slow); ok {
		t.Error("expected the slow subscriber's channel closed")
	}
}

func TestUnsubscribeClosesAndStopsDelivery(t *testing.T) {
	h := newHub()
	events, unsubscribe := h.Subscribe()

	unsubscribe()
	h.Publish(live.Event{Name: "stars"})

	if e, ok := receive(t, events); ok {
		t.Errorf("expected a closed channel; got %+v", e)
	}
}

func TestUnsubscribeAfterBeingDroppedIsSafe(t *testing.T) {
	h := newHub()
	events, unsubscribe := h.Subscribe()
	for range live.ClientBuffer + 2 {
		h.Publish(live.Event{Name: "sentence"})
	}
	for range live.ClientBuffer {
		receive(t, events)
	}
	receive(t, events) // closed by the drop

	unsubscribe() // would panic on a second close
	h.Publish(live.Event{Name: "sentence"})
}
