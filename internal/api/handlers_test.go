package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jameynakama/randsense/internal/api"
	"github.com/jameynakama/randsense/internal/store"
)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(api.NewRouter(api.RouterConfig{Queries: store.New(testPool)}))
}

func seedWords(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	q := store.New(testPool)

	_, err := testPool.Exec(ctx, "TRUNCATE nouns, verbs, adjectives, adverbs RESTART IDENTITY CASCADE")
	if err != nil {
		t.Fatalf("seedWords#truncate: %v", err)
	}

	if err := q.InsertNoun(ctx, store.InsertNounParams{
		Lemma:       "goose",
		Inflections: []byte(`{"plural":"geese"}`),
		Source:      "test",
	}); err != nil {
		t.Fatalf("seed noun: %v", err)
	}
	if err := q.InsertVerb(ctx, store.InsertVerbParams{
		Lemma:       "devour",
		Inflections: []byte(`{}`),
		Frames:      []byte(`["vtaa","vtai"]`),
		Source:      "test",
	}); err != nil {
		t.Fatalf("seed verb: %v", err)
	}
	if err := q.InsertAdjective(ctx, store.InsertAdjectiveParams{
		Lemma:       "good",
		Inflections: []byte(`{}`),
		Source:      "test",
	}); err != nil {
		t.Fatalf("seed adjective: %v", err)
	}
	if err := q.InsertAdverb(ctx, store.InsertAdverbParams{
		Lemma:       "quickly",
		Inflections: []byte(`{}`),
		Source:      "test",
	}); err != nil {
		t.Fatalf("seed adverb: %v", err)
	}
}

func TestRandomWord(t *testing.T) {
	seedWords(t)
	srv := newTestServer(t)
	defer srv.Close()

	t.Run("missing pos returns 400", func(t *testing.T) {
		resp, err := http.Get(srv.URL + "/api/v1/words/random")
		if err != nil {
			t.Fatalf("GET: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("status: got %d, want 400", resp.StatusCode)
		}
	})

	t.Run("invalid pos returns 400", func(t *testing.T) {
		resp, err := http.Get(srv.URL + "/api/v1/words/random?pos=planet")
		if err != nil {
			t.Fatalf("GET: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("status: got %d, want 400", resp.StatusCode)
		}
	})

	t.Run("noun", func(t *testing.T) {
		body := getWordBody(t, srv, "noun")

		if body["lemma"] == "" {
			t.Error("lemma: empty")
		}
		// inflections must be a JSON object, not base64
		if _, ok := body["inflections"].(map[string]any); !ok {
			t.Errorf("inflections: got %T, want object", body["inflections"])
		}
		if _, ok := body["frames"]; ok {
			t.Error("frames: should be absent for nouns")
		}
	})

	t.Run("verb", func(t *testing.T) {
		body := getWordBody(t, srv, "verb")

		if body["lemma"] == "" {
			t.Error("lemma: empty")
		}
		// frames must be a JSON array
		if _, ok := body["frames"].([]any); !ok {
			t.Errorf("frames: got %T, want array", body["frames"])
		}
	})

	t.Run("adjective", func(t *testing.T) {
		body := getWordBody(t, srv, "adjective")

		if body["lemma"] == "" {
			t.Error("lemma: empty")
		}
		if _, ok := body["frames"]; ok {
			t.Error("frames: should be absent for adjectives")
		}
	})

	t.Run("adverb", func(t *testing.T) {
		body := getWordBody(t, srv, "adverb")

		if body["lemma"] == "" {
			t.Error("lemma: empty")
		}
		if _, ok := body["frames"]; ok {
			t.Error("frames: should be absent for adverbs")
		}
	})
}

func getWordBody(t *testing.T, srv *httptest.Server, pos string) map[string]any {
	t.Helper()
	resp, err := http.Get(srv.URL + "/api/v1/words/random?pos=" + pos)
	if err != nil {
		t.Fatalf("GET pos=%s: %v", pos, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want 200", resp.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return body
}
