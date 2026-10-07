package oewn_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/jameynakama/randsense/internal/lexicon/oewn"
	"github.com/jameynakama/randsense/internal/store"
)

// TestIngest is an integration test: it runs the real Ingest pipeline
// against the testdata/sample.xml fixture and checks the resulting DB
// state. 22 LexicalEntries total: 10 nouns, 4 verbs, 2 adjectives, 2 adverbs
// pass the filters; 3 nouns (.22-caliber, Sorex araneus, OWLT) and the
// Roman-numeral adjective lxxiii are Skipped.
func TestIngest(t *testing.T) {
	truncateLexicon(t)
	ctx := context.Background()

	f, err := os.Open(filepath.Join("testdata", "sample.xml"))
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()

	stats, err := oewn.Ingest(ctx, testPool, f, sampleGlosses(t))
	if err != nil {
		t.Fatalf("Ingest: %#v %v", stats, err)
	}

	q := store.New(testPool)

	t.Run("stats", func(t *testing.T) {
		if stats.Nouns != 10 {
			t.Errorf("Nouns: got %d, want 10", stats.Nouns)
		}
		if stats.Verbs != 4 {
			t.Errorf("Verbs: got %d, want 4", stats.Verbs)
		}
		if stats.Adjectives != 2 {
			t.Errorf("Adjectives: got %d, want 2", stats.Adjectives)
		}
		if stats.Adverbs != 2 {
			t.Errorf("Adverbs: got %d, want 2", stats.Adverbs)
		}
		if stats.Skipped != 4 {
			t.Errorf("Skipped: got %d, want 4", stats.Skipped)
		}
	})

	t.Run("nouns", func(t *testing.T) {
		count, err := q.CountNouns(ctx)
		if err != nil {
			t.Fatalf("CountNouns: %v", err)
		}
		if count != 10 {
			t.Errorf("CountNouns: got %d, want 10", count)
		}

		// irregular plural
		noun, err := q.GetNounByLemma(ctx, "goose")
		if err != nil {
			t.Fatalf("GetNounByLemma(goose): %v", err)
		}
		var infl map[string]string
		if err := json.Unmarshal(noun.Inflections, &infl); err != nil {
			t.Fatalf("json.Unmarshal(noun.Inflections): %v", err)
		}
		if infl["plural"] != "geese" {
			t.Errorf("goose plural: got %q, want %q", infl["plural"], "geese")
		}

		// proper noun: case preserved
		noun, err = q.GetNounByLemma(ctx, "Microsoft")
		if err != nil {
			t.Fatalf("GetNounByLemma(Microsoft): %v", err)
		}
		if noun.Lemma != "Microsoft" {
			t.Errorf("proper noun lemma: got %q, want %q", noun.Lemma, "Microsoft")
		}

		// plural lemmas: marked only when the singular is also a lemma
		for lemma, want := range map[string]bool{"Rastas": true, "Rasta": false, "eyeglasses": true, "eyeglass": false, "Mass": false, "goose": false} {
			noun, err := q.GetNounByLemma(ctx, lemma)
			if err != nil {
				t.Fatalf("GetNounByLemma(%s): %v", lemma, err)
			}
			if noun.Plural != want {
				t.Errorf("%s plural: got %t, want %t", lemma, noun.Plural, want)
			}
		}
	})

	t.Run("verbs", func(t *testing.T) {
		count, err := q.CountVerbs(ctx)
		if err != nil {
			t.Fatalf("CountVerbs: %v", err)
		}
		if count != 4 {
			t.Errorf("CountVerbs: got %d, want 4", count)
		}

		// subcat codes mapped to frame names
		verb, err := q.GetVerbByLemma(ctx, "devour")
		if err != nil {
			t.Fatalf("GetVerbByLemma(devour): %v", err)
		}
		var frames []string
		if err := json.Unmarshal(verb.Frames, &frames); err != nil {
			t.Fatalf("json.Unmarshal(devour.Frames): %v", err)
		}
		wantFrames := []string{"transitive"}
		if len(frames) != len(wantFrames) {
			t.Errorf("devour frames: got %v, want %v", frames, wantFrames)
		} else {
			for i, f := range wantFrames {
				if frames[i] != f {
					t.Errorf("devour frames[%d]: got %q, want %q", i, frames[i], f)
				}
			}
		}

		verb, err = q.GetVerbByLemma(ctx, "sleep")
		if err != nil {
			t.Fatalf("GetVerbByLemma(sleep): %v", err)
		}
		var sleepFrames []string
		if err := json.Unmarshal(verb.Frames, &sleepFrames); err != nil {
			t.Fatalf("json.Unmarshal(sleep.Frames): %v", err)
		}
		if !slices.Equal(sleepFrames, []string{"intransitive", "intransitive-pp"}) {
			t.Errorf("sleep frames: got %v, want [intransitive intransitive-pp]", sleepFrames)
		}

		// no subcat attribute: frames must be empty array, not null
		verb, err = q.GetVerbByLemma(ctx, "loiter")
		if err != nil {
			t.Fatalf("GetVerbByLemma(loiter): %v", err)
		}
		var loiterFrames []string
		if err := json.Unmarshal(verb.Frames, &loiterFrames); err != nil {
			t.Fatalf("json.Unmarshal(loiter.Frames): %v", err)
		}
		if len(loiterFrames) != 0 {
			t.Errorf("loiter frames: got %v, want empty", loiterFrames)
		}
	})

	t.Run("adjectives", func(t *testing.T) {
		count, err := q.CountAdjectives(ctx)
		if err != nil {
			t.Fatalf("CountAdjectives: %v", err)
		}
		if count != 2 {
			t.Errorf("CountAdjectives: got %d, want 2", count)
		}

		// POS 's' (satellite adjective) must land in adjectives table
		adj, err := q.GetAdjectiveByLemma(ctx, "effervescent")
		if err != nil {
			t.Fatalf("GetAdjectiveByLemma(effervescent): %v", err)
		}
		if adj.Lemma != "effervescent" {
			t.Errorf("adjective lemma: got %q, want %q", adj.Lemma, "effervescent")
		}
	})

	t.Run("adverbs", func(t *testing.T) {
		count, err := q.CountAdverbs(ctx)
		if err != nil {
			t.Fatalf("CountAdverbs: %v", err)
		}
		if count != 2 {
			t.Errorf("CountAdverbs: got %d, want 2", count)
		}

		// apostrophe entity decoded correctly
		adv, err := q.GetAdverbByLemma(ctx, "o'clock")
		if err != nil {
			t.Fatalf("GetAdverbByLemma(o'clock): %v", err)
		}
		if adv.Lemma != "o'clock" {
			t.Errorf("adverb lemma: got %q, want %q", adv.Lemma, "o'clock")
		}
	})

	t.Run("definitions", func(t *testing.T) {
		// in sense order
		verb, err := q.GetVerbByLemma(ctx, "goose")
		if err != nil {
			t.Fatalf("GetVerbByLemma(goose): %v", err)
		}
		var defs []string
		if err := json.Unmarshal(verb.Definitions, &defs); err != nil {
			t.Fatalf("json.Unmarshal(goose.Definitions): %v", err)
		}
		if !slices.Equal(defs, []string{"poke in the buttocks", "prod into action"}) {
			t.Errorf("goose definitions: got %v", defs)
		}

		// a synset without a gloss: an empty array, not null
		noun, err := q.GetNounByLemma(ctx, "shrimp")
		if err != nil {
			t.Fatalf("GetNounByLemma(shrimp): %v", err)
		}
		if string(noun.Definitions) != "[]" {
			t.Errorf("shrimp definitions: got %s, want []", noun.Definitions)
		}
	})
}

// TestIngestMergesDefinitions checks that OEWN entries sharing a lemma end
// up in one row with every entry's glosses, once, however often ingest runs.
func TestIngestMergesDefinitions(t *testing.T) {
	truncateLexicon(t)
	ctx := context.Background()
	xml := `<?xml version="1.0" encoding="UTF-8"?>
<LexicalResource>
  <Lexicon id="oewn" label="test" language="en" email="x@y" license="cc-by-4.0" version="test">
    <LexicalEntry id="oewn-good-a">
      <Lemma writtenForm="good" partOfSpeech="a"/>
      <Sense id="oewn-good__3.00.00.." synset="oewn-1-a"/>
    </LexicalEntry>
    <LexicalEntry id="oewn-good-s">
      <Lemma writtenForm="good" partOfSpeech="s"/>
      <Sense id="oewn-good__5.00.00.." synset="oewn-2-s"/>
    </LexicalEntry>
  </Lexicon>
</LexicalResource>`
	glosses := map[string]string{"oewn-1-a": "having desirable qualities", "oewn-2-s": "morally admirable"}

	for range 2 {
		if _, err := oewn.Ingest(ctx, testPool, strings.NewReader(xml), glosses); err != nil {
			t.Fatalf("Ingest: %v", err)
		}
	}

	q := store.New(testPool)
	if n, err := q.CountAdjectives(ctx); err != nil || n != 1 {
		t.Fatalf("CountAdjectives: got %d, %v; want 1", n, err)
	}
	if n, err := q.CountNouns(ctx); err != nil || n != 0 {
		t.Fatalf("CountNouns: got %d, %v; want 0", n, err)
	}
	adj, err := q.GetAdjectiveByLemma(ctx, "good")
	if err != nil {
		t.Fatalf("GetAdjectiveByLemma(good): %v", err)
	}
	var defs []string
	if err := json.Unmarshal(adj.Definitions, &defs); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if !slices.Equal(defs, []string{"having desirable qualities", "morally admirable"}) {
		t.Errorf("good definitions: got %v", defs)
	}
}

// sampleGlosses is the fixture's synset glosses.
func sampleGlosses(t *testing.T) map[string]string {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", "sample.xml"))
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()
	g, err := oewn.Glosses(f)
	if err != nil {
		t.Fatalf("Glosses: %v", err)
	}
	return g
}

// TestPluralOverride checks that a curated correction beats the heuristic in
// both directions.
func TestPluralOverride(t *testing.T) {
	truncateLexicon(t)
	ingestSample(t)
	ctx := context.Background()

	_, err := testPool.Exec(ctx, `
		UPDATE nouns SET plural_override = FALSE WHERE lemma = 'Rastas';
		UPDATE nouns SET plural_override = TRUE WHERE lemma = 'goose';
	`)
	if err != nil {
		t.Fatalf("set overrides: %v", err)
	}

	q := store.New(testPool)
	for lemma, want := range map[string]bool{"Rastas": false, "goose": true, "eyeglasses": true} {
		noun, err := q.GetNounByLemma(ctx, lemma)
		if err != nil {
			t.Fatalf("GetNounByLemma(%s): %v", lemma, err)
		}
		if noun.Plural != want {
			t.Errorf("%s plural: got %t, want %t", lemma, noun.Plural, want)
		}
	}
}

// ingestSample ingests testdata/sample.xml.
func ingestSample(t *testing.T) oewn.Stats {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", "sample.xml"))
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()
	stats, err := oewn.Ingest(context.Background(), testPool, f, sampleGlosses(t))
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	return stats
}

// lexiconSnapshot is every content-word row's id and sourced columns, as
// JSON. update_time is left out, since a re-run touches every row.
func lexiconSnapshot(t *testing.T) string {
	t.Helper()
	var s string
	err := testPool.QueryRow(context.Background(), `
		SELECT json_build_array(
			(SELECT json_agg(json_build_array(id, lemma, inflections, definitions, plural_guess) ORDER BY id) FROM nouns),
			(SELECT json_agg(json_build_array(id, lemma, frames, definitions) ORDER BY id) FROM verbs),
			(SELECT json_agg(json_build_array(id, lemma, definitions) ORDER BY id) FROM adjectives),
			(SELECT json_agg(json_build_array(id, lemma, definitions) ORDER BY id) FROM adverbs)
		)::text`).Scan(&s)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	return s
}

// TestIngestTwiceIsIdentical checks that a re-run keeps ids and rewrites
// sourced columns to the same values.
func TestIngestTwiceIsIdentical(t *testing.T) {
	truncateLexicon(t)
	first := ingestSample(t)
	before := lexiconSnapshot(t)

	second := ingestSample(t)
	if after := lexiconSnapshot(t); after != before {
		t.Errorf("re-run changed rows:\nbefore %s\nafter  %s", before, after)
	}
	if first != second {
		t.Errorf("stats: first %+v, second %+v", first, second)
	}
}

// TestIngestKeepsCuration checks that a re-run never writes curated columns.
func TestIngestKeepsCuration(t *testing.T) {
	truncateLexicon(t)
	ingestSample(t)
	ctx := context.Background()

	_, err := testPool.Exec(ctx, `
		UPDATE nouns SET active = FALSE, vote_count = 3, plural_override = FALSE WHERE lemma = 'Rastas';
		UPDATE verbs SET active = FALSE, vote_count = 2 WHERE lemma = 'devour';
	`)
	if err != nil {
		t.Fatalf("curate: %v", err)
	}

	ingestSample(t)

	q := store.New(testPool)
	noun, err := q.GetNounByLemma(ctx, "Rastas")
	if err != nil {
		t.Fatalf("GetNounByLemma(Rastas): %v", err)
	}
	if noun.Active || noun.VoteCount != 3 || noun.PluralOverride != (pgtype.Bool{Bool: false, Valid: true}) || noun.Plural {
		t.Errorf("Rastas: active %t, vote_count %d, plural_override %+v, plural %t",
			noun.Active, noun.VoteCount, noun.PluralOverride, noun.Plural)
	}
	verb, err := q.GetVerbByLemma(ctx, "devour")
	if err != nil {
		t.Fatalf("GetVerbByLemma(devour): %v", err)
	}
	if verb.Active || verb.VoteCount != 2 {
		t.Errorf("devour: active %t, vote_count %d", verb.Active, verb.VoteCount)
	}
}

// TestIngestRefreshesSourcedColumns checks that a re-run rewrites what the
// sources own and resets what the later ingest passes fill.
func TestIngestRefreshesSourcedColumns(t *testing.T) {
	truncateLexicon(t)
	ingestSample(t)
	want := lexiconSnapshot(t)
	ctx := context.Background()

	_, err := testPool.Exec(ctx, `
		UPDATE nouns SET definitions = '["hand edit"]', inflections = '{"plural":"x"}', plural_guess = FALSE, frequency = 9
			WHERE lemma = 'Rastas';
		UPDATE verbs SET frames = '["bogus"]', separable = TRUE, frequency = 9 WHERE lemma = 'devour';
		UPDATE adjectives SET definitions = '["hand edit"]', frequency = 9;
		UPDATE adverbs SET definitions = '["hand edit"]', frequency = 9;
	`)
	if err != nil {
		t.Fatalf("hand edit: %v", err)
	}

	ingestSample(t)

	if got := lexiconSnapshot(t); got != want {
		t.Errorf("sourced columns not restored:\nwant %s\ngot  %s", want, got)
	}
	var stale int
	err = testPool.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM nouns WHERE frequency IS NOT NULL)
		     + (SELECT count(*) FROM verbs WHERE frequency IS NOT NULL OR separable)
		     + (SELECT count(*) FROM adjectives WHERE frequency IS NOT NULL)
		     + (SELECT count(*) FROM adverbs WHERE frequency IS NOT NULL)`).Scan(&stale)
	if err != nil {
		t.Fatalf("count stale: %v", err)
	}
	if stale != 0 {
		t.Errorf("rows keeping a previous run's frequency or separable flag: %d", stale)
	}
}

// TestIngestDeletesStaleRows checks that an OEWN row the run lacks is
// deleted, and a row from another source is not.
func TestIngestDeletesStaleRows(t *testing.T) {
	truncateLexicon(t)
	ctx := context.Background()
	// One statement per Exec: pgx rejects several statements with parameters.
	for _, stmt := range []string{
		"INSERT INTO nouns (lemma, source) VALUES ('flumpet', $1), ('flumpet', 'other')",
		"INSERT INTO verbs (lemma, source) VALUES ('flump', $1)",
	} {
		if _, err := testPool.Exec(ctx, stmt, oewn.SourceName); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	ingestSample(t)

	var oewnRows, otherRows int
	err := testPool.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM nouns WHERE lemma = 'flumpet' AND source = $1)
		     + (SELECT count(*) FROM verbs WHERE lemma = 'flump'),
		       (SELECT count(*) FROM nouns WHERE lemma = 'flumpet' AND source = 'other')`,
		oewn.SourceName).Scan(&oewnRows, &otherRows)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if oewnRows != 0 || otherRows != 1 {
		t.Errorf("stale oewn rows %d (want 0), other-source rows %d (want 1)", oewnRows, otherRows)
	}
}
