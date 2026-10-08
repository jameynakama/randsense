// Package subtlex sets each content word's frequency from SUBTLEX-US, a word
// list counted from film and TV subtitles and tagged by part of speech, so
// "baby" the noun is common while "baby" the verb is not. Frequencies are on
// the Zipf scale: log10 of occurrences per billion words, from about 1.3
// (seen once in the corpus) to about 7.5 ("the"). Lemmas SUBTLEX lacks in
// their part of speech, such as multiword lemmas, keep a NULL frequency.
//
// SUBTLEX also decides between a noun's OEWN plural and its regular one.
// OEWN lists irregular forms from WordNet's exception lists, some of them rare
// variants ("camerae", "brethren").
package subtlex

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/jameynakama/randsense/internal/morph"
	"github.com/jameynakama/randsense/internal/store"
)

// corpusMillions is the size of the SUBTLEX-US corpus in millions of words.
const corpusMillions = 51

// A noun's regular plural replaces its OEWN plural when SUBTLEX has it at
// least regularPluralRatio times as often, and at least regularPluralMinCount
// times, so a few stray occurrences of a rare word don't decide.
const (
	regularPluralRatio    = 5
	regularPluralMinCount = 5
)

// Stats reports how many rows per table got a frequency, and how many nouns
// take their regular plural over OEWN's.
type Stats struct {
	Nouns          int64
	Verbs          int64
	Adjectives     int64
	Adverbs        int64
	RegularPlurals int64
}

// tagged holds the words SUBTLEX tagged with one part of speech, alongside
// their Zipf frequencies and counts as that part of speech.
type tagged struct {
	words  []string
	zipfs  []float64
	counts map[string]float64
}

// Apply reads r as the word, part of speech and count rows of
// data/subtlex-us/subtlex-us-pos.tsv.gz and sets the frequency of every lemma
// SUBTLEX has in that part of speech. It runs after the OEWN ingest, whose
// upsert clears old frequencies, and doesn't begin or commit a transaction.
func Apply(ctx context.Context, db store.DBTX, r io.Reader) (Stats, error) {
	var stats Stats

	byTag, err := parse(r)
	if err != nil {
		return stats, fmt.Errorf("Apply: %w", err)
	}

	q := store.New(db)
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

	plurals, err := q.ListNounPlurals(ctx)
	if err != nil {
		return stats, fmt.Errorf("Apply, ListNounPlurals: %w", err)
	}
	stats.RegularPlurals, err = q.SetRegularPlurals(ctx, regularPlurals(plurals, byTag["Noun"].counts))
	if err != nil {
		return stats, fmt.Errorf("Apply, SetRegularPlurals: %w", err)
	}

	return stats, nil
}

// regularPlurals lists the nouns whose regular plural SUBTLEX has far more
// often than their OEWN plural.
func regularPlurals(plurals []store.ListNounPluralsRow, counts map[string]float64) []string {
	var lemmas []string
	for _, p := range plurals {
		regular := counts[morph.Pluralize(p.Lemma, "")]
		if regular >= regularPluralMinCount && regular >= regularPluralRatio*counts[p.Plural] {
			lemmas = append(lemmas, p.Lemma)
		}
	}
	return lemmas
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
		if t.counts == nil {
			t.counts = map[string]float64{}
		}
		t.words = append(t.words, fields[0])
		t.zipfs = append(t.zipfs, math.Log10(count/corpusMillions)+3)
		t.counts[fields[0]] = count
		byTag[fields[1]] = t
	}
	return byTag, sc.Err()
}
