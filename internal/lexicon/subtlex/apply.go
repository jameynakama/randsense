// Package subtlex sets each content word's frequency from SUBTLEX-US, a word
// list counted from film and TV subtitles and tagged by part of speech, so
// "baby" the noun is common while "baby" the verb is not. Frequencies are on
// the Zipf scale: log10 of occurrences per billion words, from about 1.3
// (seen once in the corpus) to about 7.5 ("the"). Lemmas SUBTLEX lacks in
// their part of speech, such as multiword lemmas, keep a NULL frequency.
package subtlex

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jameynakama/randsense/internal/store"
)

// corpusMillions is the size of the SUBTLEX-US corpus in millions of words.
const corpusMillions = 51

// Stats reports how many rows per table got a frequency.
type Stats struct {
	Nouns      int64
	Verbs      int64
	Adjectives int64
	Adverbs    int64
}

// tagged holds the words SUBTLEX tagged with one part of speech, alongside
// their Zipf frequencies as that part of speech.
type tagged struct {
	words []string
	zipfs []float64
}

// Apply reads r as the word, part of speech and count rows of
// data/subtlex-us/subtlex-us-pos.tsv.gz and sets the frequency of every lemma
// SUBTLEX has in that part of speech, in one transaction. It runs after the
// OEWN ingest, whose truncation clears old frequencies.
func Apply(ctx context.Context, pool *pgxpool.Pool, r io.Reader) (Stats, error) {
	var stats Stats

	byTag, err := parse(r)
	if err != nil {
		return stats, fmt.Errorf("Apply: %w", err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return stats, fmt.Errorf("Apply, pool.Begin: %w", err)
	}
	defer tx.Rollback(ctx)

	q := store.New(tx)
	var properNouns int64
	for _, set := range []struct {
		kind  string
		op    func() (int64, error)
		count *int64
	}{
		{"Noun", func() (int64, error) {
			t := byTag["Noun"]
			return q.SetNounFrequencies(ctx, store.SetNounFrequenciesParams{Words: t.words, Zipfs: t.zipfs})
		}, &stats.Nouns},
		{"ProperNoun", func() (int64, error) {
			t := byTag["Name"]
			return q.SetProperNounFrequencies(ctx, store.SetProperNounFrequenciesParams{Words: t.words, Zipfs: t.zipfs})
		}, &properNouns},
		{"Verb", func() (int64, error) {
			t := byTag["Verb"]
			return q.SetVerbFrequencies(ctx, store.SetVerbFrequenciesParams{Words: t.words, Zipfs: t.zipfs})
		}, &stats.Verbs},
		{"Adjective", func() (int64, error) {
			t := byTag["Adjective"]
			return q.SetAdjectiveFrequencies(ctx, store.SetAdjectiveFrequenciesParams{Words: t.words, Zipfs: t.zipfs})
		}, &stats.Adjectives},
		{"Adverb", func() (int64, error) {
			t := byTag["Adverb"]
			return q.SetAdverbFrequencies(ctx, store.SetAdverbFrequenciesParams{Words: t.words, Zipfs: t.zipfs})
		}, &stats.Adverbs},
	} {
		n, err := set.op()
		if err != nil {
			return stats, fmt.Errorf("Apply, Set%sFrequencies: %w", set.kind, err)
		}
		*set.count = n
	}
	stats.Nouns += properNouns

	return stats, tx.Commit(ctx)
}

// parse groups the words by part-of-speech tag, skipping the header row.
func parse(r io.Reader) (map[string]tagged, error) {
	byTag := map[string]tagged{}

	sc := bufio.NewScanner(r)
	sc.Scan() // header
	for line := 2; sc.Scan(); line++ {
		fields := strings.Split(sc.Text(), "\t")
		if len(fields) != 3 {
			return nil, fmt.Errorf("line %d: got %d fields, want 3", line, len(fields))
		}
		count, err := strconv.ParseFloat(fields[2], 64)
		if err != nil {
			return nil, fmt.Errorf("line %d: count: %w", line, err)
		}
		t := byTag[fields[1]]
		t.words = append(t.words, fields[0])
		t.zipfs = append(t.zipfs, math.Log10(count/corpusMillions)+3)
		byTag[fields[1]] = t
	}
	return byTag, sc.Err()
}
