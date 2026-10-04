package api_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jameynakama/randsense/internal/api"
)

const (
	voterA = "0b9e7d3c-4f1a-4c2b-8e5d-6a7f8b9c0d1e"
	voterB = "5c6d7e8f-9a0b-4c1d-8e2f-3a4b5c6d7e8f"
)

func star(t *testing.T, srv *httptest.Server, method, id, voter string) *http.Response {
	t.Helper()
	return call(t, srv, method, "/api/v1/sentences/"+id+"/stars", "", http.Header{"X-Voter": {voter}})
}

func TestStarsCountOncePerVoter(t *testing.T) {
	resetSentences(t)
	insertSentence(t, "aaaaaaaa", time.Now())
	srv := newTestServer(t)
	defer srv.Close()

	steps := []struct {
		method, voter string
		want          int32
	}{
		{http.MethodPost, voterA, 1},
		{http.MethodPost, voterA, 1},
		{http.MethodPost, strings.ToUpper(voterA), 1},
		{http.MethodPost, voterB, 2},
		{http.MethodDelete, voterA, 1},
		{http.MethodDelete, voterA, 1},
	}
	for i, s := range steps {
		var body struct {
			Count int32 `json:"count"`
		}
		decode(t, star(t, srv, s.method, "aaaaaaaa", s.voter), http.StatusOK, &body)
		if body.Count != s.want {
			t.Errorf("step %d (%s by %s): count got %d, want %d", i, s.method, s.voter, body.Count, s.want)
		}
	}

	var saved api.SentenceResponse
	decode(t, call(t, srv, http.MethodGet, "/api/v1/sentences/aaaaaaaa", "", nil), http.StatusOK, &saved)
	if saved.StarCount != 1 {
		t.Errorf("star_count: got %d, want 1", saved.StarCount)
	}
}

func TestStarsRejectBadRequests(t *testing.T) {
	resetSentences(t)
	insertSentence(t, "aaaaaaaa", time.Now())
	srv := newTestServer(t)
	defer srv.Close()

	for _, method := range []string{http.MethodPost, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			decode(t, star(t, srv, method, "aaaaaaaa", ""), http.StatusBadRequest, nil)
			decode(t, star(t, srv, method, "aaaaaaaa", "not-a-uuid"), http.StatusBadRequest, nil)
			decode(t, star(t, srv, method, "zzzzzzzz", voterA), http.StatusNotFound, nil)
		})
	}
	decode(t, call(t, srv, http.MethodGet, "/api/v1/stars", "", nil), http.StatusBadRequest, nil)
}

// A double-click or racing voters.
func TestConcurrentStarsKeepTheCountRight(t *testing.T) {
	tests := []struct {
		name  string
		voter func(i int) string
		want  int32
	}{
		{"one voter", func(int) string { return voterA }, 1},
		{"many voters", func(i int) string { return fmt.Sprintf("00000000-0000-4000-8000-%012d", i) }, 10},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resetSentences(t)
			insertSentence(t, "aaaaaaaa", time.Now())
			srv := newTestServer(t)
			defer srv.Close()

			var wg sync.WaitGroup
			for i := range 10 {
				wg.Go(func() {
					req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/sentences/aaaaaaaa/stars", nil)
					req.Header.Set("X-Voter", tc.voter(i))
					resp, err := srv.Client().Do(req)
					if err != nil {
						t.Errorf("POST: %v", err)
						return
					}
					resp.Body.Close()
					if resp.StatusCode != http.StatusOK {
						t.Errorf("status: got %d, want 200", resp.StatusCode)
					}
				})
			}
			wg.Wait()

			var count, rows int32
			err := testPool.QueryRow(context.Background(),
				"SELECT star_count, (SELECT count(*) FROM stars WHERE sentence_id = 'aaaaaaaa') FROM sentences WHERE id = 'aaaaaaaa'").
				Scan(&count, &rows)
			if err != nil {
				t.Fatalf("query: %v", err)
			}
			if count != tc.want || rows != tc.want {
				t.Errorf("star_count %d, stars rows %d; want %d", count, rows, tc.want)
			}
		})
	}
}

func TestListStarsNewestStarFirst(t *testing.T) {
	resetSentences(t)
	for _, id := range []string{"aaaaaaaa", "bbbbbbbb", "cccccccc"} {
		insertSentence(t, id, time.Now())
	}
	srv := newTestServer(t)
	defer srv.Close()
	decode(t, star(t, srv, http.MethodPost, "aaaaaaaa", voterA), http.StatusOK, nil)
	decode(t, star(t, srv, http.MethodPost, "bbbbbbbb", voterA), http.StatusOK, nil)
	decode(t, star(t, srv, http.MethodPost, "cccccccc", voterB), http.StatusOK, nil)
	// Star time, not sentence time, orders the list.
	mustExec(t, "UPDATE stars SET created_at = now() + interval '1 hour' WHERE sentence_id = 'aaaaaaaa'")

	tests := []struct {
		query string
		want  []string
	}{
		{"", []string{"aaaaaaaa", "bbbbbbbb"}},
		{"?limit=1&offset=1", []string{"bbbbbbbb"}},
	}
	for _, tc := range tests {
		t.Run(tc.query, func(t *testing.T) {
			var body []api.SentenceResponse
			decode(t, call(t, srv, http.MethodGet, "/api/v1/stars"+tc.query, "", http.Header{"X-Voter": {voterA}}), http.StatusOK, &body)
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
