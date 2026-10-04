package subtlex_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jameynakama/randsense/internal/lexicon/subtlex"
	"github.com/jameynakama/randsense/internal/store"
)

func seedLexicon(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	q := store.New(testPool)

	_, err := testPool.Exec(ctx, "TRUNCATE nouns, verbs, adjectives, adverbs RESTART IDENTITY CASCADE")
	if err != nil {
		t.Fatalf("truncate: %v", err)
	}
	// "In" (indium) must not pick up the frequency of "in", and "john" (the
	// toilet) must not pick up the frequency of the name.
	for _, lemma := range []string{"goose", "America", "john", "In", "hot dog"} {
		if err := q.InsertNoun(ctx, store.InsertNounParams{Lemma: lemma, Inflections: []byte("{}"), Source: "test"}); err != nil {
			t.Fatalf("InsertNoun(%s): %v", lemma, err)
		}
	}
	// SUBTLEX only has "baby" as a noun, so the verb gets no frequency.
	for _, lemma := range []string{"devour", "baby"} {
		if err := q.InsertVerb(ctx, store.InsertVerbParams{Lemma: lemma, Inflections: []byte("{}"), Frames: []byte("[]"), Source: "test"}); err != nil {
			t.Fatalf("InsertVerb(%s): %v", lemma, err)
		}
	}
	if err := q.InsertAdjective(ctx, store.InsertAdjectiveParams{Lemma: "good", Inflections: []byte("{}"), Source: "test"}); err != nil {
		t.Fatalf("InsertAdjective: %v", err)
	}
	if err := q.InsertAdverb(ctx, store.InsertAdverbParams{Lemma: "quickly", Inflections: []byte("{}"), Source: "test"}); err != nil {
		t.Fatalf("InsertAdverb: %v", err)
	}
}

func frequency(t *testing.T, table, lemma string) *float64 {
	t.Helper()
	var f *float64
	err := testPool.QueryRow(context.Background(), "SELECT frequency::float8 FROM "+table+" WHERE lemma = $1", lemma).Scan(&f)
	if err != nil {
		t.Fatalf("frequency(%s, %s): %v", table, lemma, err)
	}
	return f
}

func TestApply(t *testing.T) {
	seedLexicon(t)

	f, err := os.Open(filepath.Join("testdata", "sample.txt"))
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()

	stats, err := subtlex.Apply(context.Background(), testPool, f)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	want := subtlex.Stats{Nouns: 3, Verbs: 1, Adjectives: 1, Adverbs: 1}
	if stats != want {
		t.Errorf("stats: got %+v, want %+v", stats, want)
	}

	for _, tc := range []struct {
		table, lemma string
		want         float64
	}{
		{"nouns", "goose", 3.89},
		{"nouns", "America", 4.89},
		{"nouns", "john", 3.55},
		{"verbs", "devour", 3.02},
		{"adjectives", "good", 4.29},
		{"adverbs", "quickly", 4.74},
	} {
		got := frequency(t, tc.table, tc.lemma)
		if got == nil || *got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.lemma, got, tc.want)
		}
	}

	for _, tc := range []struct{ table, lemma string }{
		{"nouns", "In"},
		{"nouns", "hot dog"},
		{"verbs", "baby"},
	} {
		if got := frequency(t, tc.table, tc.lemma); got != nil {
			t.Errorf("%s: got %v, want NULL", tc.lemma, *got)
		}
	}
}

func TestApplyRejectsMalformedRows(t *testing.T) {
	for name, input := range map[string]string{
		"short row": "word\tpos\tcount\nthe\tArticle\n",
		"bad count": "word\tpos\tcount\nthe\tArticle\tlots\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := subtlex.Apply(context.Background(), testPool, strings.NewReader(input)); err == nil {
				t.Error("Apply: got nil error")
			}
		})
	}
}
