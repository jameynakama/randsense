package subtlex_test

import (
	"context"
	"os"
	"path/filepath"
	"slices"
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
	// OEWN's plural for "camera" is the rare "camerae"; "geese" and "alae"
	// stay, the second because SUBTLEX barely has "alas" as a noun.
	nouns := []string{"goose", "America", "john", "In", "hot dog", "camera", "ala"}
	inflections := append(repeat("{}", 5), `{"plural":"camerae"}`, `{"plural":"alae"}`)
	inflections[0] = `{"plural":"geese"}`
	if err := q.UpsertNouns(ctx, store.UpsertNounsParams{
		Lemmas: nouns, Inflections: inflections, Definitions: repeat("[]", len(nouns)), Source: "test",
	}); err != nil {
		t.Fatalf("UpsertNouns: %v", err)
	}
	// SUBTLEX only has "baby" as a noun, so the verb gets no frequency.
	verbs := []string{"devour", "baby"}
	if err := q.UpsertVerbs(ctx, store.UpsertVerbsParams{
		Lemmas: verbs, Frames: repeat("[]", len(verbs)), Definitions: repeat("[]", len(verbs)), Source: "test",
	}); err != nil {
		t.Fatalf("UpsertVerbs: %v", err)
	}
	if err := q.UpsertAdjectives(ctx, store.UpsertAdjectivesParams{Lemmas: []string{"good"}, Definitions: []string{"[]"}, Source: "test"}); err != nil {
		t.Fatalf("UpsertAdjectives: %v", err)
	}
	if err := q.UpsertAdverbs(ctx, store.UpsertAdverbsParams{Lemmas: []string{"quickly"}, Definitions: []string{"[]"}, Source: "test"}); err != nil {
		t.Fatalf("UpsertAdverbs: %v", err)
	}
}

func repeat(s string, n int) []string {
	return slices.Repeat([]string{s}, n)
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

	want := subtlex.Stats{Nouns: 4, Verbs: 1, Adjectives: 1, Adverbs: 1, RegularPlurals: 1}
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

func TestApplyPrefersAFarCommonerRegularPlural(t *testing.T) {
	seedLexicon(t)
	f, err := os.Open(filepath.Join("testdata", "sample.txt"))
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()

	if _, err := subtlex.Apply(context.Background(), testPool, f); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	for lemma, want := range map[string]bool{"camera": true, "goose": false, "ala": false} {
		var got bool
		if err := testPool.QueryRow(context.Background(), "SELECT regular_plural FROM nouns WHERE lemma = $1", lemma).Scan(&got); err != nil {
			t.Fatalf("%s: %v", lemma, err)
		}
		if got != want {
			t.Errorf("%s: regular_plural %t, want %t", lemma, got, want)
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
