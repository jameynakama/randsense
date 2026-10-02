package sentence_test

import (
	"context"
	"errors"
	"maps"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"

	"github.com/jameynakama/randsense/internal/grammar"
	"github.com/jameynakama/randsense/internal/morph"
	"github.com/jameynakama/randsense/internal/sentence"
	"github.com/jameynakama/randsense/internal/store"
)

// fakeQuerier returns fixed words per POS, cycling through dets for
// determiners. Embedding the interface means any query Generate shouldn't
// call panics.
type fakeQuerier struct {
	store.Querier
	err  error
	dets []store.Determiner
	n    int
	noun *store.Noun

	// numberDet answers GetRandomDeterminerWithNumber, which records the
	// numbers it was asked for.
	numberDet store.Determiner
	numbers   []string
}

func newFake(dets ...store.Determiner) *fakeQuerier {
	if len(dets) == 0 {
		dets = []store.Determiner{{Lemma: "the", Number: "either"}}
	}
	return &fakeQuerier{dets: dets}
}

func (f *fakeQuerier) GetRandomNoun(context.Context) (store.Noun, error) {
	if f.noun != nil {
		return *f.noun, f.err
	}
	return store.Noun{Lemma: "goose", Inflections: []byte(`{"plural":"geese"}`)}, f.err
}

func (f *fakeQuerier) GetRandomVerb(context.Context) (store.Verb, error) {
	return store.Verb{Lemma: "devour"}, f.err
}

func (f *fakeQuerier) GetRandomAdjective(context.Context) (store.Adjective, error) {
	return store.Adjective{Lemma: "ugly"}, f.err
}

func (f *fakeQuerier) GetRandomAdverb(context.Context) (store.Adverb, error) {
	return store.Adverb{Lemma: "loudly"}, f.err
}

func (f *fakeQuerier) GetRandomDeterminer(context.Context) (store.Determiner, error) {
	d := f.dets[f.n%len(f.dets)]
	f.n++
	return d, f.err
}

func (f *fakeQuerier) GetRandomDeterminerWithNumber(_ context.Context, numbers []string) (store.Determiner, error) {
	f.numbers = numbers
	return f.numberDet, f.err
}

func (f *fakeQuerier) GetRandomPreposition(context.Context) (store.Preposition, error) {
	return store.Preposition{Lemma: "under"}, f.err
}

func (f *fakeQuerier) GetRandomPronoun(context.Context) (store.Pronoun, error) {
	return store.Pronoun{Lemma: "she"}, f.err
}

func (f *fakeQuerier) GetRandomConjunction(context.Context) (store.Conjunction, error) {
	return store.Conjunction{Lemma: "and"}, f.err
}

func loadVerbs(t *testing.T) *morph.Verbs {
	t.Helper()
	v, err := morph.LoadVerbs(strings.NewReader(""))
	if err != nil {
		t.Fatalf("LoadVerbs: %v", err)
	}
	return v
}

const simpleGrammar = `
[[rule]]
symbol = "S"
expansion = ["NP", "VP"]

[[rule]]
symbol = "NP"
expansion = ["Determiner", "Noun"]

[[rule]]
symbol = "VP"
expansion = ["Verb"]
`

// texts generates n sentences over successive seeds and returns how often
// each text came out.
func texts(t *testing.T, grammarTOML string, newQ func() *fakeQuerier, n int) map[string]int {
	t.Helper()
	g := mustLoad(t, grammarTOML)
	v := loadVerbs(t)
	seen := map[string]int{}
	for i := range n {
		s, err := sentence.Generate(context.Background(), newQ(), g, v, rand.New(rand.NewPCG(uint64(i), 0)))
		if err != nil {
			t.Fatalf("Generate: %v", err)
		}
		seen[s.Text]++
	}
	return seen
}

func assertExactly(t *testing.T, seen map[string]int, want ...string) {
	t.Helper()
	got := slices.Sorted(maps.Keys(seen))
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("expected exactly %q; got %q", want, got)
	}
}

func TestGenerateAgreesWithPluralDeterminer(t *testing.T) {
	seen := texts(t, simpleGrammar, func() *fakeQuerier {
		return newFake(store.Determiner{Lemma: "these", Number: "plural"})
	}, 50)

	assertExactly(t, seen, "These geese devour.", "These geese devoured.")
}

func TestGenerateAgreesWithSingularDeterminer(t *testing.T) {
	seen := texts(t, simpleGrammar, func() *fakeQuerier {
		return newFake(store.Determiner{Lemma: "this", Number: "singular"})
	}, 50)

	assertExactly(t, seen, "This goose devours.", "This goose devoured.")
}

func TestGenerateChoosesNumberForEitherDeterminer(t *testing.T) {
	seen := texts(t, simpleGrammar, func() *fakeQuerier {
		return newFake(store.Determiner{Lemma: "the", Number: "either"})
	}, 100)

	assertExactly(t, seen,
		"The goose devours.", "The goose devoured.", "The geese devour.", "The geese devoured.")
}

func TestGenerateAgreesWithSubjectNotObject(t *testing.T) {
	seen := texts(t, `
	[[rule]]
	symbol = "S"
	expansion = ["NP", "VP"]

	[[rule]]
	symbol = "NP"
	expansion = ["Determiner", "Noun"]

	[[rule]]
	symbol = "VP"
	expansion = ["Verb", "NP"]
	`, func() *fakeQuerier {
		return newFake(
			store.Determiner{Lemma: "this", Number: "singular"},
			store.Determiner{Lemma: "these", Number: "plural"},
		)
	}, 50)

	assertExactly(t, seen, "This goose devours these geese.", "This goose devoured these geese.")
}

func TestGenerateChoosesIndefiniteArticle(t *testing.T) {
	seen := texts(t, `
	[[rule]]
	symbol = "S"
	expansion = ["NP", "VP"]

	[[rule]]
	symbol = "NP"
	expansion = ["Determiner", "Adjective", "Noun"]

	[[rule]]
	symbol = "VP"
	expansion = ["Verb"]
	`, func() *fakeQuerier {
		return newFake(store.Determiner{Lemma: "a", Number: "singular"})
	}, 50)

	assertExactly(t, seen, "An ugly goose devours.", "An ugly goose devoured.")
}

func TestGenerateKeepsLemmaAndWordOnLeaves(t *testing.T) {
	g := mustLoad(t, simpleGrammar)
	q := newFake(store.Determiner{Lemma: "these", Number: "plural"})

	s, err := sentence.Generate(context.Background(), q, g, loadVerbs(t), newRNG())
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	noun := s.Tree.Children[0].Children[1]
	if noun.Lemma != "goose" || noun.Word != "geese" {
		t.Errorf("expected lemma goose, word geese; got lemma %q, word %q", noun.Lemma, noun.Word)
	}
}

func mustLoad(t *testing.T, in string) *grammar.Grammar {
	t.Helper()
	g, err := grammar.Load(strings.NewReader(in))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return g
}

func newRNG() *rand.Rand {
	return rand.New(rand.NewPCG(1, 2))
}

func TestGenerateFillsEveryPOS(t *testing.T) {
	g := mustLoad(t, `
	[[rule]]
	symbol = "S"
	expansion = ["Pronoun", "Adverb", "Verb", "Preposition", "Determiner", "Adjective", "Noun", "Conjunction"]
	`)

	s, err := sentence.Generate(context.Background(), newFake(), g, loadVerbs(t), newRNG())
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if s.Text != "She loudly devours under the ugly goose and." && s.Text != "She loudly devoured under the ugly goose and." {
		t.Errorf("expected She loudly devours/devoured under the ugly goose and.; got %q", s.Text)
	}
}

func TestGenerateStoresLemmasOnTreeLeaves(t *testing.T) {
	g := mustLoad(t, `
	[[rule]]
	symbol = "S"
	expansion = ["NP", "Verb"]

	[[rule]]
	symbol = "NP"
	expansion = ["Determiner", "Noun"]
	`)

	s, err := sentence.Generate(context.Background(), newFake(), g, loadVerbs(t), newRNG())
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	np := s.Tree.Children[0]
	got := []string{np.Lemma, np.Children[0].Lemma, np.Children[1].Lemma, s.Tree.Children[1].Lemma}
	want := []string{"", "the", "goose", "devour"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("expected words %q; got %q", want, got)
			break
		}
	}
}

func TestGenerateReturnsLookupErrors(t *testing.T) {
	g := mustLoad(t, `
	[[rule]]
	symbol = "S"
	expansion = ["Noun"]
	`)
	boom := errors.New("boom")

	q := newFake()
	q.err = boom

	_, err := sentence.Generate(context.Background(), q, g, loadVerbs(t), newRNG())

	if !errors.Is(err, boom) {
		t.Errorf("expected boom; got %v", err)
	}
}

func TestGenerateReturnsExpansionErrors(t *testing.T) {
	g := mustLoad(t, `
	[[rule]]
	symbol = "S"
	expansion = ["S", "S"]
	weight = 10

	[[rule]]
	symbol = "S"
	expansion = ["Noun"]
	`)

	_, err := sentence.Generate(context.Background(), newFake(), g, loadVerbs(t), newRNG())

	if err == nil {
		t.Error("expected an error; got nil")
	}
}

func TestGenerateRejectsMalformedNounInflections(t *testing.T) {
	g := mustLoad(t, simpleGrammar)
	q := newFake()
	q.noun = &store.Noun{Lemma: "goose", Inflections: []byte(`["geese"]`)}

	_, err := sentence.Generate(context.Background(), q, g, loadVerbs(t), newRNG())

	if err == nil || !strings.Contains(err.Error(), `"goose"`) {
		t.Errorf("expected an error naming goose; got %v", err)
	}
}

func TestGenerateKeepsPluralLemmaPluralWithFittingDeterminer(t *testing.T) {
	var q *fakeQuerier
	seen := texts(t, simpleGrammar, func() *fakeQuerier {
		q = newFake(store.Determiner{Lemma: "a", Number: "singular"})
		q.noun = &store.Noun{Lemma: "Rastas", Inflections: []byte(`{}`), Plural: true}
		q.numberDet = store.Determiner{Lemma: "the", Number: "either"}
		return q
	}, 50)

	assertExactly(t, seen, "The Rastas devour.", "The Rastas devoured.")
	if !slices.Equal(q.numbers, []string{"plural", "either"}) {
		t.Errorf("expected determiner numbers [plural either]; got %v", q.numbers)
	}
}
