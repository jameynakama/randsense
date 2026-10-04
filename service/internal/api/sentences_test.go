package api_test

import (
	"context"
	"net/http"
	"regexp"
	"testing"

	"github.com/jameynakama/randsense/internal/api"
)

func TestRandomSentenceIsSaved(t *testing.T) {
	seedWords(t)
	seedRareNoun(t)
	mustExec(t, "UPDATE verbs SET frequency = 3.03")
	srv := newTestServer(t)
	defer srv.Close()

	resp := call(t, srv, http.MethodGet, "/api/v1/sentences/random?commonness=3", "", nil)
	var body api.SentenceResponse
	decode(t, resp, http.StatusOK, &body)

	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("Access-Control-Allow-Origin: got %q, want *", got)
	}
	if !regexp.MustCompile(`^[0-9A-Za-z]{8}$`).MatchString(body.ID) {
		t.Errorf("id: got %q, want 8 base-62 characters", body.ID)
	}
	if body.CreatedAt.IsZero() || body.StarCount != 0 || len(body.Tree) == 0 {
		t.Errorf("body: got %+v, want created_at, star_count 0 and a tree", body)
	}
	var text string
	var commonness float64
	err := testPool.QueryRow(context.Background(), "SELECT text, commonness::float8 FROM sentences WHERE id = $1", body.ID).
		Scan(&text, &commonness)
	if err != nil {
		t.Fatalf("saved row: %v", err)
	}
	if text != body.Text || commonness != 3 {
		t.Errorf("saved: got %q at %v, want %q at 3", text, commonness, body.Text)
	}
}

func TestRealizedSentenceIsNotSaved(t *testing.T) {
	seedWords(t)
	resetSentences(t)
	srv := newTestServer(t)
	defer srv.Close()

	resp := postTree(t, srv, "", realizeTree)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want 200", resp.StatusCode)
	}

	var n int
	if err := testPool.QueryRow(context.Background(), "SELECT count(*) FROM sentences").Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Errorf("saved sentences: got %d, want 0", n)
	}
}
