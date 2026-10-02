package sentence_test

import (
	"context"
	"errors"
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/jameynakama/randsense/internal/grammar"
	"github.com/jameynakama/randsense/internal/sentence"
	"github.com/jameynakama/randsense/internal/store"
)

// fakeQuerier returns one fixed word per POS. Embedding the interface means
// any query Generate shouldn't call panics.
type fakeQuerier struct {
	store.Querier
	err error
}

func (f fakeQuerier) GetRandomNoun(context.Context) (store.Noun, error) {
	return store.Noun{Lemma: "goose"}, f.err
}

func (f fakeQuerier) GetRandomVerb(context.Context) (store.Verb, error) {
	return store.Verb{Lemma: "devour"}, f.err
}

func (f fakeQuerier) GetRandomAdjective(context.Context) (store.Adjective, error) {
	return store.Adjective{Lemma: "damp"}, f.err
}

func (f fakeQuerier) GetRandomAdverb(context.Context) (store.Adverb, error) {
	return store.Adverb{Lemma: "loudly"}, f.err
}

func (f fakeQuerier) GetRandomDeterminer(context.Context) (store.Determiner, error) {
	return store.Determiner{Lemma: "the"}, f.err
}

func (f fakeQuerier) GetRandomPreposition(context.Context) (store.Preposition, error) {
	return store.Preposition{Lemma: "under"}, f.err
}

func (f fakeQuerier) GetRandomPronoun(context.Context) (store.Pronoun, error) {
	return store.Pronoun{Lemma: "she"}, f.err
}

func (f fakeQuerier) GetRandomConjunction(context.Context) (store.Conjunction, error) {
	return store.Conjunction{Lemma: "and"}, f.err
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

	s, err := sentence.Generate(context.Background(), fakeQuerier{}, g, newRNG())
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	want := "She loudly devour under the damp goose and."
	if s.Text != want {
		t.Errorf("expected %q; got %q", want, s.Text)
	}
}

func TestGenerateStoresWordsOnTreeLeaves(t *testing.T) {
	g := mustLoad(t, `
	[[rule]]
	symbol = "S"
	expansion = ["NP", "Verb"]

	[[rule]]
	symbol = "NP"
	expansion = ["Determiner", "Noun"]
	`)

	s, err := sentence.Generate(context.Background(), fakeQuerier{}, g, newRNG())
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	np := s.Tree.Children[0]
	got := []string{np.Word, np.Children[0].Word, np.Children[1].Word, s.Tree.Children[1].Word}
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

	_, err := sentence.Generate(context.Background(), fakeQuerier{err: boom}, g, newRNG())

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

	_, err := sentence.Generate(context.Background(), fakeQuerier{}, g, newRNG())

	if err == nil {
		t.Error("expected an error; got nil")
	}
}
