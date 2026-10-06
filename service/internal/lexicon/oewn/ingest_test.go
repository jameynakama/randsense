package oewn_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/jameynakama/randsense/internal/lexicon/oewn"
	"github.com/jameynakama/randsense/internal/store"
)

// TestIngest is an integration test: it runs the real Ingest pipeline
// against the testdata/sample.xml fixture and checks the resulting DB
// state. 22 LexicalEntries total: 10 nouns, 4 verbs, 2 adjectives, 2 adverbs
// pass the filters; 3 nouns (.22-caliber, Sorex araneus, OWLT) and the
// Roman-numeral adjective lxxiii are Skipped.
func TestIngest(t *testing.T) {
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
// up in one row with every entry's glosses.
func TestIngestMergesDefinitions(t *testing.T) {
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

	if _, err := oewn.Ingest(ctx, testPool, strings.NewReader(xml), glosses); err != nil {
		t.Fatalf("Ingest: %v", err)
	}

	q := store.New(testPool)
	if n, err := q.CountAdjectives(ctx); err != nil || n != 1 {
		t.Fatalf("CountAdjectives: got %d, %v; want 1", n, err)
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
