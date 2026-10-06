package oewn

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jameynakama/randsense/internal/store"
)

// SourceName tags every row this package writes. Stored in the `source`
// column so later ingest passes can reconcile rows by lemma+source.
const SourceName = "oewn-2025"

// Stats reports per-POS row counts from a successful Ingest run, plus a
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
// nouns, AllowAdjective for adjectives), dispatches each entry to the per-POS
// table by Entry.POS with its definitions from glosses, and inserts, then
// marks nouns whose lemma is already plural. The whole run is
// one transaction -- any error rolls back. Each per-POS table is
// truncated first so re-runs produce identical state regardless of prior
// content (idempotency by clean slate).
//
// Pass the gzipped XML pre-wrapped in a gzip.Reader if you're reading
// data/oewn-2025/english-wordnet-2025.xml.gz; Ingest itself only cares
// that it gets parseable XML bytes.
func Ingest(ctx context.Context, pool *pgxpool.Pool, r io.Reader, glosses map[string]string) (Stats, error) {
	var stats Stats

	tx, err := pool.Begin(ctx)
	if err != nil {
		return stats, fmt.Errorf("Ingest, pool.Begin: %v", err)
	}
	defer tx.Rollback(ctx)

	q := store.New(tx)

	for _, tr := range []struct {
		kind string
		op   func(context.Context) error
	}{
		{"Nouns", q.TruncateNouns},
		{"Verbs", q.TruncateVerbs},
		{"Adjectives", q.TruncateAdjectives},
		{"Adverbs", q.TruncateAdverbs},
	} {
		err := tr.op(ctx)
		if err != nil {
			return stats, fmt.Errorf("Ingest, Truncate%s: %v", tr.kind, err)
		}
	}

	err = Parse(r, func(e Entry) error {
		return ingestEntry(ctx, q, e, glosses, &stats)
	})
	if err != nil {
		return stats, fmt.Errorf("Ingest, Parse: %v", err)
	}

	// Runs after every noun is in, since a singular can follow its plural.
	if _, err := q.MarkPluralNouns(ctx); err != nil {
		return stats, fmt.Errorf("Ingest, MarkPluralNouns: %v", err)
	}

	return stats, tx.Commit(ctx)
}

// ingestEntry routes a single Entry to its table. Unknown POS codes increment
// Skipped; lemmas that fail AllowLemma are also Skipped.
func ingestEntry(ctx context.Context, q *store.Queries, e Entry, glosses map[string]string, stats *Stats) error {
	if !AllowLemma(e.Lemma) {
		stats.Skipped++
		return nil
	}
	defs, err := definitionsJSON(e.Synsets, glosses)
	if err != nil {
		return err
	}

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
		err = q.InsertNoun(ctx, store.InsertNounParams{
			Lemma:       e.Lemma,
			Inflections: infl,
			Definitions: defs,
			Source:      SourceName,
		})
		if err != nil {
			return err
		}
		stats.Nouns++
	case "v":
		frames, err := verbFramesJSON(e.Lemma, e.Frames)
		if err != nil {
			return err
		}
		err = q.InsertVerb(ctx, store.InsertVerbParams{
			Lemma:       e.Lemma,
			Inflections: []byte("{}"),
			Frames:      frames,
			Definitions: defs,
			Source:      SourceName,
		})
		if err != nil {
			return err
		}
		stats.Verbs++
	case "a", "s":
		if !AllowAdjective(e.Lemma, e.Cardinal) {
			stats.Skipped++
			return nil
		}
		err = q.InsertAdjective(ctx, store.InsertAdjectiveParams{
			Lemma:       e.Lemma,
			Inflections: []byte("{}"),
			Definitions: defs,
			Source:      SourceName,
		})
		if err != nil {
			return err
		}
		stats.Adjectives++
	case "r":
		err = q.InsertAdverb(ctx, store.InsertAdverbParams{
			Lemma:       e.Lemma,
			Inflections: []byte("{}"),
			Definitions: defs,
			Source:      SourceName,
		})
		if err != nil {
			return err
		}
		stats.Adverbs++
	default:
		// Anything else (proper-name codes, unknowns) gets skipped.
		stats.Skipped++
	}
	return nil
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

// definitionsJSON is the glosses of synsets, in sense order, as a JSON
// array. A synset without a gloss is skipped.
func definitionsJSON(synsets []string, glosses map[string]string) ([]byte, error) {
	defs := []string{}
	for _, id := range synsets {
		if g, ok := glosses[id]; ok {
			defs = append(defs, g)
		}
	}
	return json.Marshal(defs)
}
