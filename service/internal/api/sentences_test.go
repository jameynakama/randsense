package api_test

import (
	"context"
	"net/http"
	"regexp"
	"slices"
	"testing"
	"time"

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
	srv := newRealizeServer(t)
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

func TestListSentencesNewestFirst(t *testing.T) {
	resetSentences(t)
	t0 := time.Now().Add(-time.Hour)
	insertSentence(t, "aaaaaaaa", t0)
	insertSentence(t, "bbbbbbbb", t0.Add(time.Second))
	insertSentence(t, "cccccccc", t0.Add(2*time.Second))
	// Same time as cccccccc: the id breaks the tie.
	insertSentence(t, "dddddddd", t0.Add(2*time.Second))
	srv := newTestServer(t)
	defer srv.Close()

	tests := []struct {
		query string
		want  []string
	}{
		{"", []string{"dddddddd", "cccccccc", "bbbbbbbb", "aaaaaaaa"}},
		{"?limit=2&offset=1", []string{"cccccccc", "bbbbbbbb"}},
		{"?offset=4", []string{}},
	}
	for _, tc := range tests {
		t.Run(tc.query, func(t *testing.T) {
			var body []api.SentenceResponse
			decode(t, call(t, srv, http.MethodGet, "/api/v1/sentences"+tc.query, "", nil), http.StatusOK, &body)

			if body == nil {
				t.Fatal("body: got null, want an array")
			}
			got := make([]string, len(body))
			for i, s := range body {
				got[i] = s.ID
			}
			if !slices.Equal(got, tc.want) {
				t.Errorf("ids: got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestListSentencesRejectsBadPages(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	for _, q := range []string{"limit=0", "limit=101", "limit=ten", "offset=-1", "offset=1.5", "offset=99999999999"} {
		t.Run(q, func(t *testing.T) {
			decode(t, call(t, srv, http.MethodGet, "/api/v1/sentences?"+q, "", nil), http.StatusBadRequest, nil)
		})
	}
}

func TestGetSentence(t *testing.T) {
	resetSentences(t)
	insertSentence(t, "aaaaaaaa", time.Now())
	srv := newTestServer(t)
	defer srv.Close()

	var body api.SentenceResponse
	decode(t, call(t, srv, http.MethodGet, "/api/v1/sentences/aaaaaaaa", "", nil), http.StatusOK, &body)
	if body.ID != "aaaaaaaa" || body.Text != "This goose, devours." {
		t.Errorf("body: got %+v, want aaaaaaaa", body)
	}

	decode(t, call(t, srv, http.MethodGet, "/api/v1/sentences/zzzzzzzz", "", nil), http.StatusNotFound, nil)
}
