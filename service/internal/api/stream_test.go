package api_test

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jameynakama/randsense/internal/api"
)

// openStream connects to the event stream. It closes when the test ends,
// before the server does: register t.Cleanup(srv.Close) before calling it,
// since Close waits for open requests.
func openStream(t *testing.T, srv *httptest.Server) (*http.Response, *bufio.Reader) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/v1/sentences/stream", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("GET stream: %v", err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp, bufio.NewReader(resp.Body)
}

func readLine(t *testing.T, r *bufio.Reader) string {
	t.Helper()
	line, err := r.ReadString('\n')
	if err != nil {
		t.Fatalf("read stream: %v", err)
	}
	return strings.TrimSuffix(line, "\n")
}

// nextEvent reads up to the end of the next event, skipping keepalives.
func nextEvent(t *testing.T, r *bufio.Reader) (name, data string) {
	t.Helper()
	for {
		line := readLine(t, r)
		switch {
		case strings.HasPrefix(line, "event: "):
			name = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			data = strings.TrimPrefix(line, "data: ")
		case line == "" && name != "":
			return name, data
		}
	}
}

func TestStreamHeaders(t *testing.T) {
	srv := newTestServer(t)
	t.Cleanup(srv.Close)

	resp, _ := openStream(t, srv)

	want := map[string]string{
		"Content-Type":      "text/event-stream",
		"Cache-Control":     "no-cache",
		"X-Accel-Buffering": "no",
	}
	for k, v := range want {
		if got := resp.Header.Get(k); got != v {
			t.Errorf("%s: got %q, want %q", k, got, v)
		}
	}
}

func TestStreamSendsGeneratedSentences(t *testing.T) {
	seedWords(t)
	srv := newTestServer(t)
	t.Cleanup(srv.Close)
	_, events := openStream(t, srv)

	var generated api.SentenceResponse
	decode(t, call(t, srv, http.MethodGet, "/api/v1/sentences/random", "", nil), http.StatusOK, &generated)

	name, data := nextEvent(t, events)
	var streamed api.SentenceResponse
	if err := json.Unmarshal([]byte(data), &streamed); err != nil {
		t.Fatalf("data %q: %v", data, err)
	}
	if name != "sentence" || streamed.ID != generated.ID || streamed.Text != generated.Text {
		t.Errorf("event: got %s %+v, want sentence %s", name, streamed, generated.ID)
	}
}

func TestStreamSendsStarCounts(t *testing.T) {
	resetSentences(t)
	insertSentence(t, "aaaaaaaa", time.Now())
	srv := newTestServer(t)
	t.Cleanup(srv.Close)
	_, events := openStream(t, srv)

	for _, method := range []string{http.MethodPost, http.MethodDelete} {
		decode(t, star(t, srv, method, "aaaaaaaa", voterA), http.StatusOK, nil)
	}

	for _, want := range []string{`{"id":"aaaaaaaa","count":1}`, `{"id":"aaaaaaaa","count":0}`} {
		if name, data := nextEvent(t, events); name != "stars" || data != want {
			t.Errorf("event: got %s %s, want stars %s", name, data, want)
		}
	}
}

func TestStreamSendsKeepalives(t *testing.T) {
	srv := newServer(t, api.RouterConfig{Keepalive: 10 * time.Millisecond})
	t.Cleanup(srv.Close)
	_, events := openStream(t, srv)

	if line := readLine(t, events); line != ":" {
		t.Errorf("first line: got %q, want the keepalive comment", line)
	}
}
