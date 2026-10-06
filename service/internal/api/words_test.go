package api_test

import (
	"net/http"
	"slices"
	"testing"
)

func TestDefinitions(t *testing.T) {
	seedWords(t)
	// Inactive words still have definitions.
	mustExec(t, `UPDATE nouns SET definitions = '["a bird", "a fool"]', active = FALSE WHERE lemma = 'goose'`)
	mustExec(t, `INSERT INTO nouns (lemma, definitions, source) VALUES ('sea anemone', '["a polyp"]', 'test')`)
	mustExec(t, `INSERT INTO adverbs (lemma, definitions, source) VALUES ('o''clock', '["of the clock"]', 'test')`)
	srv := newTestServer(t)
	defer srv.Close()

	for _, tc := range []struct {
		path string
		want []string
	}{
		{"/api/v1/words/noun/goose", []string{"a bird", "a fool"}},
		{"/api/v1/words/noun/sea%20anemone", []string{"a polyp"}},
		{"/api/v1/words/adverb/o'clock", []string{"of the clock"}},
		{"/api/v1/words/adverb/o%27clock", []string{"of the clock"}},
		{"/api/v1/words/verb/devour", []string{}},
	} {
		t.Run(tc.path, func(t *testing.T) {
			resp := call(t, srv, http.MethodGet, tc.path, "", nil)
			var body struct {
				Definitions []string `json:"definitions"`
			}
			decode(t, resp, http.StatusOK, &body)
			if !slices.Equal(body.Definitions, tc.want) {
				t.Errorf("definitions: got %v, want %v", body.Definitions, tc.want)
			}
			if cc := resp.Header.Get("Cache-Control"); cc != "public, max-age=86400" {
				t.Errorf("Cache-Control: got %q", cc)
			}
		})
	}

	for _, path := range []string{
		"/api/v1/words/noun/unicorn",
		"/api/v1/words/determiner/this",
		"/api/v1/words/planet/goose",
	} {
		t.Run(path, func(t *testing.T) {
			decode(t, call(t, srv, http.MethodGet, path, "", nil), http.StatusNotFound, nil)
		})
	}
}
