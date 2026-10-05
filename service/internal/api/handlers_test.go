package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/jameynakama/randsense/internal/api"
	"github.com/jameynakama/randsense/internal/grammar"
	"github.com/jameynakama/randsense/internal/store"
)

// testGrammar has one expansion, so with one seeded word per POS the
// generated sentence is deterministic.
const testGrammar = `
[[rule]]
symbol = "S"
expansion = ["NP", "Verb"]

[[rule]]
symbol = "NP"
expansion = ["Determiner", "Noun"]
`

func seedWords(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	q := store.New(testPool)

	_, err := testPool.Exec(ctx, "TRUNCATE nouns, verbs, adjectives, adverbs, determiners RESTART IDENTITY CASCADE")
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
		Frames:      []byte(`["transitive"]`),
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
	if err := q.InsertDeterminer(ctx, store.InsertDeterminerParams{
		Lemma:  "this",
		Type:   "demonstrative",
		Number: "singular",
	}); err != nil {
		t.Fatalf("seed determiner: %v", err)
	}
	if _, err := testPool.Exec(ctx, `
		UPDATE nouns SET frequency = 1.5;
		UPDATE verbs SET frequency = 1.5;
		UPDATE adjectives SET frequency = 1.5;
		UPDATE adverbs SET frequency = 1.5`); err != nil {
		t.Fatalf("seed frequencies: %v", err)
	}
}

func TestRandomSentence(t *testing.T) {
	seedWords(t)
	srv := newTestServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/v1/sentences/random")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want 200", resp.StatusCode)
	}

	var body struct {
		Text string        `json:"text"`
		Tree *grammar.Node `json:"tree"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Text != "This goose devours." && body.Text != "This goose devoured." {
		t.Errorf("text: got %q, want This goose devours/devoured", body.Text)
	}
	if body.Tree == nil || body.Tree.Symbol != "S" || len(body.Tree.Children) != 2 {
		t.Errorf("tree: got %+v, want S with NP and Verb children", body.Tree)
	}
}

func TestRandomSentenceWithEmptyLexiconReturns500(t *testing.T) {
	ctx := context.Background()
	if _, err := testPool.Exec(ctx, "TRUNCATE nouns, verbs, determiners RESTART IDENTITY CASCADE"); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	srv := newTestServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/v1/sentences/random")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", resp.StatusCode)
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

func TestInvalidCommonnessReturns400(t *testing.T) {
	seedWords(t)
	srv := newTestServer(t)
	defer srv.Close()

	for _, path := range []string{"/api/v1/words/random?pos=noun&commonness=", "/api/v1/sentences/random?commonness="} {
		for _, c := range []string{"lots", "-1", "7.5", "NaN"} {
			t.Run(path+c, func(t *testing.T) {
				resp, err := http.Get(srv.URL + path + c)
				if err != nil {
					t.Fatalf("GET: %v", err)
				}
				defer resp.Body.Close()
				if resp.StatusCode != http.StatusBadRequest {
					t.Errorf("status: got %d, want 400", resp.StatusCode)
				}
			})
		}
	}
}

// seedRareNoun adds "goffer", which has no frequency, and gives "goose" one,
// so a commonness floor leaves only "goose".
func seedRareNoun(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	if err := store.New(testPool).InsertNoun(ctx, store.InsertNounParams{
		Lemma:       "goffer",
		Inflections: []byte(`{}`),
		Source:      "test",
	}); err != nil {
		t.Fatalf("seed noun: %v", err)
	}
	if _, err := testPool.Exec(ctx, "UPDATE nouns SET frequency = 4.12 WHERE lemma = 'goose'"); err != nil {
		t.Fatalf("set frequency: %v", err)
	}
}

func TestCommonnessDefaultsToOne(t *testing.T) {
	seedWords(t)
	seedRareNoun(t)
	srv := newTestServer(t)
	defer srv.Close()

	for range 20 {
		var body map[string]any
		decode(t, call(t, srv, http.MethodGet, "/api/v1/words/random?pos=noun", "", nil), http.StatusOK, &body)
		if body["lemma"] != "goose" {
			t.Fatalf("lemma: got %v, want goose, since goffer has no frequency", body["lemma"])
		}
	}
}

func TestRandomWordHonorsCommonness(t *testing.T) {
	seedWords(t)
	seedRareNoun(t)
	srv := newTestServer(t)
	defer srv.Close()

	for range 20 {
		resp, err := http.Get(srv.URL + "/api/v1/words/random?pos=noun&commonness=4")
		if err != nil {
			t.Fatalf("GET: %v", err)
		}
		var body map[string]any
		err = json.NewDecoder(resp.Body).Decode(&body)
		resp.Body.Close()
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		if body["lemma"] != "goose" {
			t.Fatalf("lemma: got %v, want goose", body["lemma"])
		}
	}
}

func TestRandomSentenceHonorsCommonness(t *testing.T) {
	seedWords(t)
	seedRareNoun(t)
	if _, err := testPool.Exec(context.Background(), "UPDATE verbs SET frequency = 3.03"); err != nil {
		t.Fatalf("set frequency: %v", err)
	}
	srv := newTestServer(t)
	defer srv.Close()

	for range 20 {
		resp, err := http.Get(srv.URL + "/api/v1/sentences/random?commonness=3")
		if err != nil {
			t.Fatalf("GET: %v", err)
		}
		var body struct {
			Text string `json:"text"`
		}
		err = json.NewDecoder(resp.Body).Decode(&body)
		resp.Body.Close()
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		if body.Text != "This goose devours." && body.Text != "This goose devoured." {
			t.Fatalf("text: got %q, want This goose devours/devoured", body.Text)
		}
	}
}

// realizeTree is "Determiner Noun Verb:transitive Determiner Noun".
const realizeTree = `{"symbol": "S", "children": [
	{"symbol": "NP", "children": [{"symbol": "Determiner"}, {"symbol": "Noun"}]},
	{"symbol": "VP", "children": [
		{"symbol": "Verb:transitive"},
		{"symbol": "NP", "children": [{"symbol": "Determiner"}, {"symbol": "Noun"}]}
	]}
]}`

// realizeGrammar derives the trees the realize tests post.
const realizeGrammar = `
[[rule]]
symbol = "S"
expansion = ["NP", "VP"]

[[rule]]
symbol = "S"
expansion = ["Verb:ditransitive"]

[[rule]]
symbol = "NP"
expansion = ["Determiner", "Noun"]

[[rule]]
symbol = "VP"
expansion = ["Verb:transitive", "NP"]

[[rule]]
symbol = "VP"
expansion = ["Verb"]

[[rule]]
symbol = "S"
expansion = ["NP", "Gift"]

[[rule]]
symbol = "Gift"
expansion = ["Verb:ditransitive", "NP", "NP"]
`

func newRealizeServer(t *testing.T) *httptest.Server {
	t.Helper()
	return newServer(t, api.RouterConfig{Grammar: loadGrammar(t, realizeGrammar)})
}

func postTree(t *testing.T, srv *httptest.Server, query, tree string) *http.Response {
	t.Helper()
	resp, err := http.Post(srv.URL+"/api/v1/sentences/realize"+query, "application/json", strings.NewReader(tree))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	return resp
}

func TestRealizeSentence(t *testing.T) {
	seedWords(t)
	srv := newRealizeServer(t)
	defer srv.Close()

	// A tree from a previous response gets fresh words.
	tree := strings.Replace(realizeTree, `{"symbol": "Noun"}`, `{"symbol": "Noun", "lemma": "moon", "word": "moons"}`, 1)
	resp := postTree(t, srv, "", tree)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want 200", resp.StatusCode)
	}

	var body struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Text != "This goose devours this goose." && body.Text != "This goose devoured this goose." {
		t.Errorf("text: got %q, want This goose devours/devoured this goose", body.Text)
	}
}

func TestRealizeSentenceRejectsBadRequests(t *testing.T) {
	seedWords(t)
	srv := newRealizeServer(t)
	defer srv.Close()

	tests := []struct {
		name, query, tree string
		want              int
	}{
		{"invalid commonness", "?commonness=lots", realizeTree, http.StatusBadRequest},
		{"not JSON", "", "S -> NP VP", http.StatusBadRequest},
		{"too large", "", `{"symbol": "S", "children": [` + strings.Repeat(`{"symbol": "Noun"},`, 10000) + `{"symbol": "Noun"}]}`, http.StatusBadRequest},
		{"expansion that isn't a rule", "", `{"symbol": "S", "children": [{"symbol": "Noun"}]}`, http.StatusBadRequest},
		{"root that isn't S", "", `{"symbol": "NP", "children": [{"symbol": "Determiner"}, {"symbol": "Noun"}]}`, http.StatusBadRequest},
		{"part of speech with children", "", `{"symbol": "S", "children": [{"symbol": "Verb:ditransitive", "children": [{"symbol": "Noun"}]}]}`, http.StatusBadRequest},
		{"unknown qualifier", "", `{"symbol": "S", "children": [{"symbol": "Verb:bogus"}]}`, http.StatusBadRequest},
		{"frame without verbs", "", `{"symbol": "S", "children": [{"symbol": "Verb:ditransitive"}]}`, http.StatusUnprocessableEntity},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp := postTree(t, srv, tc.query, tc.tree)
			defer resp.Body.Close()
			if resp.StatusCode != tc.want {
				t.Errorf("status: got %d, want %d", resp.StatusCode, tc.want)
			}
		})
	}
}

func TestRealizeSentenceRecomputesPostedFeatures(t *testing.T) {
	seedWords(t)
	srv := newRealizeServer(t)
	defer srv.Close()

	// As if copied from a saved sentence, with features that no longer
	// apply. Filling replaces a leaf's and an NP's features, but nothing
	// else touches a VP's.
	tree := `{"symbol": "S", "features": {"tense": "future", "commonness": 6}, "children": [
		{"symbol": "NP", "children": [{"symbol": "Determiner"}, {"symbol": "Noun", "display": "stale"}]},
		{"symbol": "VP", "features": {"gender": "stale"}, "children": [{"symbol": "Verb"}]}
	]}`
	resp := postTree(t, srv, "", tree)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want 200", resp.StatusCode)
	}
	var body struct {
		Tree grammar.Node `json:"tree"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if tense := body.Tree.Features.Tense; tense != "present" && tense != "past" {
		t.Errorf("tense: got %q, want present or past", tense)
	}
	if c := body.Tree.Features.Commonness; c == nil || *c != 1 {
		t.Errorf("commonness: got %v, want the default, 1", c)
	}
	if g := body.Tree.Children[1].Features.Gender; g != "" {
		t.Errorf("VP gender: got %q, want none", g)
	}
	if d := body.Tree.Children[0].Children[1].Display; d != "" {
		t.Errorf("noun display: got %q, want none", d)
	}
}

// leafProblem is a 422 body from realize.
type leafProblem struct {
	Error string `json:"error"`
	Leaf  int    `json:"leaf"`
}

func TestRealizeSentenceFillsHoles(t *testing.T) {
	seedWords(t)
	srv := newRealizeServer(t)
	defer srv.Close()

	resp := postTree(t, srv, "", `{"symbol": "S", "children": [{"symbol": "NP"}, {"symbol": "VP"}]}`)
	defer resp.Body.Close()
	var body struct {
		Text string       `json:"text"`
		Tree grammar.Node `json:"tree"`
	}
	decode(t, resp, http.StatusOK, &body)

	if !regexp.MustCompile(`^This goose (devours|devoured)( this goose)?\.$`).MatchString(body.Text) {
		t.Errorf("text: got %q", body.Text)
	}
	if len(body.Tree.Children[0].Children) != 2 || len(body.Tree.Children[1].Children) == 0 {
		t.Errorf("holes left in the tree: %+v", body.Tree)
	}
}

func TestRealizeSentenceNamesTheLeafNoWordFits(t *testing.T) {
	seedWords(t)
	srv := newRealizeServer(t)
	defer srv.Close()

	tests := []struct {
		name, tree string
		leaf       int
	}{
		{"a slot", `{"symbol": "S", "children": [{"symbol": "Verb:ditransitive"}]}`, 0},
		{"a hole, after it is expanded afresh", `{"symbol": "S", "children": [{"symbol": "NP"}, {"symbol": "Gift"}]}`, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp := postTree(t, srv, "", tc.tree)
			defer resp.Body.Close()
			var body leafProblem
			decode(t, resp, http.StatusUnprocessableEntity, &body)
			if body.Leaf != tc.leaf || body.Error == "" {
				t.Errorf("body: got %+v, want leaf %d and an error", body, tc.leaf)
			}
		})
	}
}
