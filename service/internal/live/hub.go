// Package live fans events out to every open stream, from memory. A second
// server process would need to publish through Postgres LISTEN/NOTIFY.
package live

// ClientBuffer is how many events a subscriber can fall behind by before
// it's dropped. A dropped browser reconnects and refetches.
const ClientBuffer = 16

// Event is one server-sent event: its name and its JSON data.
type Event struct {
	Name string
	Data []byte
}

// Hub keeps the set of subscribers in one goroutine, Run.
type Hub struct {
	register   chan chan Event
	unregister chan chan Event
	publish    chan Event
}

func NewHub() *Hub {
	return &Hub{
		register:   make(chan chan Event),
		unregister: make(chan chan Event),
		publish:    make(chan Event),
	}
}

// Run serves subscriptions and publishes until the process exits.
func (h *Hub) Run() {
	clients := map[chan Event]struct{}{}
	drop := func(c chan Event) {
		if _, ok := clients[c]; ok {
			delete(clients, c)
			close(c)
		}
	}
	for {
		select {
		case c := <-h.register:
			clients[c] = struct{}{}
		case c := <-h.unregister:
			drop(c)
		case e := <-h.publish:
			for c := range clients {
				select {
				case c <- e:
				default:
					drop(c)
				}
			}
		}
	}
}

// Subscribe returns a channel of every event published from now on, and a
// function to stop. The channel closes on stopping or on falling more than
// ClientBuffer events behind.
func (h *Hub) Subscribe() (<-chan Event, func()) {
	c := make(chan Event, ClientBuffer)
	h.register <- c
	return c, func() { h.unregister <- c }
}

// Publish sends e to every subscriber without waiting on any of them.
func (h *Hub) Publish(e Event) {
	h.publish <- e
}
