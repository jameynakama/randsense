package oewn

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/jameynakama/randsense/internal/store"
)

// SourceName tags every row this package writes. Stored in the `source`
// column so later ingest passes can reconcile rows by lemma+source.
const SourceName = "oewn-2025"

// Stats reports how many distinct lemmas Ingest wrote per POS, plus a
// Skipped count covering both filtered-out lemmas (failed AllowLemma) and
// entries with a POS code we don't handle.
type Stats struct {
	Nouns      int
	Verbs      int
	Adjectives int
	Adverbs    int
	Skipped    int
}

// Ingest streams r as OEW WN-LMF XML, applies AllowLemma (and AllowNoun for
// nouns, AllowAdjective for adjectives), and groups each entry by lemma into
// its per-POS table with its definitions from glosses. Each table is then
// made to match the run: rows from SourceName whose lemma the run lacks are
// deleted, and the rest are upserted, which replaces sourced columns, resets
// derived ones, and never writes curated ones. Last, it marks nouns whose
// lemma is already plural.
//
// Ingest doesn't begin or commit a transaction: pass the run's transaction
// as db, so a failure in any ingest step leaves the previous lexicon.
//
// Pass the gzipped XML pre-wrapped in a gzip.Reader if you're reading
// data/oewn-2025/english-wordnet-2025.xml.gz; Ingest itself only cares
// that it gets parseable XML bytes.
func Ingest(ctx context.Context, db store.DBTX, r io.Reader, glosses map[string]string) (Stats, error) {
	var stats Stats
	var nouns, verbs, adjectives, adverbs table

	err := Parse(r, func(e Entry) error {
		if !AllowLemma(e.Lemma) {
			stats.Skipped++
			return nil
		}
		defs := definitions(e.Synsets, glosses)

		switch e.POS {
		case "n":
			if !AllowNoun(e.Lemma) {
				stats.Skipped++
				return nil
			}
			infl, err := nounInflectionsJSON(e.Forms)
			if err != nil {
				return err
			}
			nouns.add(e.Lemma, row{inflections: string(infl), definitions: defs})
		case "v":
			frames, err := verbFramesJSON(e.Lemma, e.Frames)
			if err != nil {
				return err
			}
			verbs.add(e.Lemma, row{frames: string(frames), definitions: defs})
		case "a", "s":
			if !AllowAdjective(e.Lemma, e.Cardinal) {
				stats.Skipped++
				return nil
			}
			adjectives.add(e.Lemma, row{definitions: defs})
		case "r":
			adverbs.add(e.Lemma, row{definitions: defs})
		default:
			// Anything else (proper-name codes, unknowns) gets skipped.
			stats.Skipped++
		}
		return nil
	})
	if err != nil {
		return stats, fmt.Errorf("Ingest, Parse: %v", err)
	}

	q := store.New(db)
	for _, w := range []struct {
		kind   string
		t      *table
		count  *int
		delete func() error
		upsert func(defs []string) error
	}{
		{"Nouns", &nouns, &stats.Nouns,
			func() error {
				return q.DeleteStaleNouns(ctx, store.DeleteStaleNounsParams{Source: SourceName, Lemmas: nouns.lemmas})
			},
			func(defs []string) error {
				return q.UpsertNouns(ctx, store.UpsertNounsParams{
					Lemmas: nouns.lemmas, Inflections: nouns.column(func(r *row) string { return r.inflections }),
					Definitions: defs, Source: SourceName,
				})
			}},
		{"Verbs", &verbs, &stats.Verbs,
			func() error {
				return q.DeleteStaleVerbs(ctx, store.DeleteStaleVerbsParams{Source: SourceName, Lemmas: verbs.lemmas})
			},
			func(defs []string) error {
				return q.UpsertVerbs(ctx, store.UpsertVerbsParams{
					Lemmas: verbs.lemmas, Frames: verbs.column(func(r *row) string { return r.frames }),
					Definitions: defs, Source: SourceName,
				})
			}},
		{"Adjectives", &adjectives, &stats.Adjectives,
			func() error {
				return q.DeleteStaleAdjectives(ctx, store.DeleteStaleAdjectivesParams{Source: SourceName, Lemmas: adjectives.lemmas})
			},
			func(defs []string) error {
				return q.UpsertAdjectives(ctx, store.UpsertAdjectivesParams{Lemmas: adjectives.lemmas, Definitions: defs, Source: SourceName})
			}},
		{"Adverbs", &adverbs, &stats.Adverbs,
			func() error {
				return q.DeleteStaleAdverbs(ctx, store.DeleteStaleAdverbsParams{Source: SourceName, Lemmas: adverbs.lemmas})
			},
			func(defs []string) error {
				return q.UpsertAdverbs(ctx, store.UpsertAdverbsParams{Lemmas: adverbs.lemmas, Definitions: defs, Source: SourceName})
			}},
	} {
		if err := w.delete(); err != nil {
			return stats, fmt.Errorf("Ingest, DeleteStale%s: %v", w.kind, err)
		}
		defs, err := w.t.definitionsColumn()
		if err != nil {
			return stats, fmt.Errorf("Ingest, %s definitions: %v", w.kind, err)
		}
		if err := w.upsert(defs); err != nil {
			return stats, fmt.Errorf("Ingest, Upsert%s: %v", w.kind, err)
		}
		*w.count = len(w.t.lemmas)
	}

	// Runs after every noun is in, since a singular can follow its plural.
	if _, err := q.MarkPluralNouns(ctx); err != nil {
		return stats, fmt.Errorf("Ingest, MarkPluralNouns: %v", err)
	}

	return stats, nil
}

// row is one lemma's sourced columns. inflections is set for nouns and
// frames for verbs, both as JSON.
type row struct {
	inflections string
	frames      string
	definitions []string
}

// table is one part of speech's rows by lemma, in first-seen order.
type table struct {
	lemmas []string
	rows   map[string]*row
}

// add pools entries that share a lemma: the first supplies inflections and
// frames, and each later one appends its definitions.
func (t *table) add(lemma string, r row) {
	if prev, ok := t.rows[lemma]; ok {
		prev.definitions = append(prev.definitions, r.definitions...)
		return
	}
	if t.rows == nil {
		t.rows = map[string]*row{}
	}
	t.lemmas = append(t.lemmas, lemma)
	t.rows[lemma] = &r
}

// column is one field of every row, in lemma order, for an upsert array.
func (t *table) column(field func(*row) string) []string {
	col := make([]string, len(t.lemmas))
	for i, l := range t.lemmas {
		col[i] = field(t.rows[l])
	}
	return col
}

// definitionsColumn is every row's definitions as a JSON array, in lemma
// order.
func (t *table) definitionsColumn() ([]string, error) {
	col := make([]string, len(t.lemmas))
	for i, l := range t.lemmas {
		b, err := json.Marshal(t.rows[l].definitions)
		if err != nil {
			return nil, err
		}
		col[i] = string(b)
	}
	return col, nil
}

// nounInflectionsJSON converts the parser's Forms slice into the JSONB
// shape stored on the nouns row: {"plural": "geese"}. Empty Forms returns
// the empty object so the column never holds NULL or invalid JSON.
//
// OEW only attaches Form children to irregular plurals; regular nouns
// arrive with empty Forms and fall back to morph.Pluralize's spelling rules.
func nounInflectionsJSON(forms []string) ([]byte, error) {
	if len(forms) == 0 {
		return []byte("{}"), nil
	}
	return json.Marshal(map[string]string{"plural": forms[0]})
}

func verbFramesJSON(lemma string, codes []string) ([]byte, error) {
	return json.Marshal(MapFrames(lemma, codes))
}

// definitions is the glosses of synsets, in sense order. A synset without a
// gloss is skipped. It is never nil, so a row with no glosses stores [].
func definitions(synsets []string, glosses map[string]string) []string {
	defs := []string{}
	for _, id := range synsets {
		if g, ok := glosses[id]; ok {
			defs = append(defs, g)
		}
	}
	return defs
}
