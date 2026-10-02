package oewn_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/jameynakama/randsense/internal/lexicon/oewn"
	"github.com/jameynakama/randsense/internal/store"
)

// TestIngest is an integration test: it runs the real Ingest pipeline
// against the testdata/sample.xml fixture and checks the resulting DB
// state. 19 LexicalEntries total: 8 nouns, 4 verbs, 2 adjectives, 2 adverbs
// pass the filters; 3 nouns (.22-caliber, Sorex araneus, OWLT) are Skipped.
func TestIngest(t *testing.T) {
	ctx := context.Background()

	f, err := os.Open(filepath.Join("testdata", "sample.xml"))
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()

	stats, err := oewn.Ingest(ctx, testPool, f)
	if err != nil {
		t.Fatalf("Ingest: %#v %v", stats, err)
	}

	q := store.New(testPool)

	t.Run("stats", func(t *testing.T) {
		if stats.Nouns != 8 {
			t.Errorf("Nouns: got %d, want 8", stats.Nouns)
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
		if stats.Skipped != 3 {
			t.Errorf("Skipped: got %d, want 3", stats.Skipped)
		}
	})

	t.Run("nouns", func(t *testing.T) {
		count, err := q.CountNouns(ctx)
		if err != nil {
			t.Fatalf("CountNouns: %v", err)
		}
		if count != 8 {
			t.Errorf("CountNouns: got %d, want 8", count)
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
		for lemma, want := range map[string]bool{"Rastas": true, "Rasta": false, "Mass": false, "goose": false} {
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
}
