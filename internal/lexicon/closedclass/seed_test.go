package closedclass_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jameynakama/randsense/internal/lexicon/closedclass"
	"github.com/jameynakama/randsense/internal/store"
)

const fixture = `
[[determiner]]
lemma = "the"
type = "definite"
number = "either"

[[determiner]]
lemma = "these"
type = "demonstrative"
number = "plural"

[[preposition]]
lemma = "under"

[[pronoun]]
lemma = "she"
case = "nominative"
person = 3
number = "singular"
gender = "fem"

[[conjunction]]
lemma = "and"
type = "coordinating"
`

const otherFixture = `
[[determiner]]
lemma = "a"
type = "indefinite"
number = "singular"

[[preposition]]
lemma = "over"

[[pronoun]]
lemma = "us"
case = "accusative"
person = 1
number = "plural"
gender = "epicene"

[[conjunction]]
lemma = "because"
type = "subordinating"
`

// snapshot is every closed-class row, minus IDs and the active flag, which
// Seed doesn't set.
type snapshot struct {
	Determiners  []store.InsertDeterminerParams
	Prepositions []string
	Pronouns     []store.InsertPronounParams
	Conjunctions []store.InsertConjunctionParams
}

func takeSnapshot(t *testing.T) snapshot {
	t.Helper()
	ctx := context.Background()
	q := store.New(testPool)
	var s snapshot

	dets, err := q.ListDeterminers(ctx)
	if err != nil {
		t.Fatalf("ListDeterminers: %v", err)
	}
	for _, d := range dets {
		s.Determiners = append(s.Determiners, store.InsertDeterminerParams{Lemma: d.Lemma, Type: d.Type, Number: d.Number})
	}

	preps, err := q.ListPrepositions(ctx)
	if err != nil {
		t.Fatalf("ListPrepositions: %v", err)
	}
	for _, p := range preps {
		s.Prepositions = append(s.Prepositions, p.Lemma)
	}

	prons, err := q.ListPronouns(ctx)
	if err != nil {
		t.Fatalf("ListPronouns: %v", err)
	}
	for _, p := range prons {
		s.Pronouns = append(s.Pronouns, store.InsertPronounParams{Lemma: p.Lemma, Case: p.Case, Person: p.Person, Number: p.Number, Gender: p.Gender})
	}

	conjs, err := q.ListConjunctions(ctx)
	if err != nil {
		t.Fatalf("ListConjunctions: %v", err)
	}
	for _, c := range conjs {
		s.Conjunctions = append(s.Conjunctions, store.InsertConjunctionParams{Lemma: c.Lemma, Type: c.Type})
	}
	return s
}

func mustSeed(t *testing.T, in string) closedclass.Stats {
	t.Helper()
	stats, err := closedclass.Seed(context.Background(), testPool, strings.NewReader(in))
	if err != nil {
		t.Fatalf("Seed: %v", err)
	}
	return stats
}

func TestSeedWritesEveryTable(t *testing.T) {
	stats := mustSeed(t, fixture)

	wantStats := closedclass.Stats{Determiners: 2, Prepositions: 1, Pronouns: 1, Conjunctions: 1}
	if stats != wantStats {
		t.Errorf("stats: expected %+v; got %+v", wantStats, stats)
	}

	want := snapshot{
		Determiners: []store.InsertDeterminerParams{
			{Lemma: "the", Type: "definite", Number: "either"},
			{Lemma: "these", Type: "demonstrative", Number: "plural"},
		},
		Prepositions: []string{"under"},
		Pronouns: []store.InsertPronounParams{
			{Lemma: "she", Case: "nominative", Person: 3, Number: "singular", Gender: "fem"},
		},
		Conjunctions: []store.InsertConjunctionParams{
			{Lemma: "and", Type: "coordinating"},
		},
	}
	if got := takeSnapshot(t); !reflect.DeepEqual(got, want) {
		t.Errorf("expected %+v; got %+v", want, got)
	}
}

func TestSeedReplacesExistingRows(t *testing.T) {
	mustSeed(t, fixture)
	mustSeed(t, otherFixture)

	want := snapshot{
		Determiners:  []store.InsertDeterminerParams{{Lemma: "a", Type: "indefinite", Number: "singular"}},
		Prepositions: []string{"over"},
		Pronouns: []store.InsertPronounParams{
			{Lemma: "us", Case: "accusative", Person: 1, Number: "plural", Gender: "epicene"},
		},
		Conjunctions: []store.InsertConjunctionParams{{Lemma: "because", Type: "subordinating"}},
	}
	if got := takeSnapshot(t); !reflect.DeepEqual(got, want) {
		t.Errorf("expected %+v; got %+v", want, got)
	}
}

func TestSeedRejectsInvalidEntries(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantErr string
	}{
		{"malformed toml", `[[determiner]`, "toml"},
		{"empty lemma", `
			[[preposition]]
			lemma = ""
			`, "empty lemma"},
		{"determiner type", `
			[[determiner]]
			lemma = "wibble"
			type = "definit"
			number = "either"
			`, `"wibble"`},
		{"determiner number", `
			[[determiner]]
			lemma = "wibble"
			type = "definite"
			number = "both"
			`, `"wibble"`},
		{"pronoun case", `
			[[pronoun]]
			lemma = "wibble"
			case = "dative"
			person = 3
			number = "singular"
			gender = "fem"
			`, `"wibble"`},
		{"pronoun person", `
			[[pronoun]]
			lemma = "wibble"
			case = "nominative"
			person = 4
			number = "singular"
			gender = "fem"
			`, `"wibble"`},
		{"pronoun number", `
			[[pronoun]]
			lemma = "wibble"
			case = "nominative"
			person = 3
			number = "either"
			gender = "fem"
			`, `"wibble"`},
		{"pronoun gender", `
			[[pronoun]]
			lemma = "wibble"
			case = "nominative"
			person = 3
			number = "singular"
			gender = "female"
			`, `"wibble"`},
		{"conjunction type", `
			[[conjunction]]
			lemma = "wibble"
			type = "correlative"
			`, `"wibble"`},
		{"duplicate preposition", `
			[[preposition]]
			lemma = "wibble"

			[[preposition]]
			lemma = "wibble"
			`, `"wibble"`},
		{"duplicate determiner", `
			[[determiner]]
			lemma = "wibble"
			type = "definite"
			number = "either"

			[[determiner]]
			lemma = "wibble"
			type = "definite"
			number = "plural"
			`, `"wibble"`},
		{"duplicate pronoun", `
			[[pronoun]]
			lemma = "wibble"
			case = "nominative"
			person = 3
			number = "singular"
			gender = "fem"

			[[pronoun]]
			lemma = "wibble"
			case = "nominative"
			person = 3
			number = "singular"
			gender = "fem"
			`, `"wibble"`},
		{"duplicate conjunction", `
			[[conjunction]]
			lemma = "wibble"
			type = "coordinating"

			[[conjunction]]
			lemma = "wibble"
			type = "coordinating"
			`, `"wibble"`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mustSeed(t, fixture)
			before := takeSnapshot(t)

			_, err := closedclass.Seed(context.Background(), testPool, strings.NewReader(tc.in))

			if err == nil {
				t.Fatal("expected an error; got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("expected error mentioning %s; got %q", tc.wantErr, err)
			}
			if after := takeSnapshot(t); !reflect.DeepEqual(after, before) {
				t.Errorf("expected existing rows untouched; got %+v", after)
			}
		})
	}
}

func TestProjectClosedClassSeeds(t *testing.T) {
	f, err := os.Open(filepath.Join("..", "..", "..", "data", "lexicon", "closed_class.toml"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()

	stats, err := closedclass.Seed(context.Background(), testPool, f)
	if err != nil {
		t.Fatalf("Seed: %v", err)
	}
	if stats.Determiners == 0 || stats.Prepositions == 0 || stats.Pronouns == 0 || stats.Conjunctions == 0 {
		t.Errorf("expected rows in every table; got %+v", stats)
	}
}
