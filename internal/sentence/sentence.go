// Package sentence generates sentences by expanding a grammar and filling
// each POS leaf with a random word from the lexicon.
package sentence

import (
	"context"
	"fmt"
	"math/rand/v2"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/jameynakama/randsense/internal/grammar"
	"github.com/jameynakama/randsense/internal/store"
)

// Sentence is a generated sentence and the parse tree it was built from.
type Sentence struct {
	Text string        `json:"text"`
	Tree *grammar.Node `json:"tree"`
}

// Generate expands g and fills every leaf with a random active word. Words
// are uninflected lemmas.
func Generate(ctx context.Context, q store.Querier, g *grammar.Grammar, rng *rand.Rand) (*Sentence, error) {
	tree, err := g.Expand(rng)
	if err != nil {
		return nil, fmt.Errorf("Generate: %w", err)
	}

	var words []string
	if err := fill(ctx, q, tree, &words); err != nil {
		return nil, fmt.Errorf("Generate: %w", err)
	}

	return &Sentence{Text: format(words), Tree: tree}, nil
}

func fill(ctx context.Context, q store.Querier, n *grammar.Node, words *[]string) error {
	if len(n.Children) == 0 {
		word, err := randomWord(ctx, q, grammar.POS(n.Symbol))
		if err != nil {
			return fmt.Errorf("%s: %w", n.Symbol, err)
		}
		n.Word = word
		*words = append(*words, word)
		return nil
	}
	for _, c := range n.Children {
		if err := fill(ctx, q, c, words); err != nil {
			return err
		}
	}
	return nil
}

func randomWord(ctx context.Context, q store.Querier, pos grammar.POS) (string, error) {
	switch pos {
	case grammar.Noun:
		w, err := q.GetRandomNoun(ctx)
		return w.Lemma, err
	case grammar.Verb:
		w, err := q.GetRandomVerb(ctx)
		return w.Lemma, err
	case grammar.Adjective:
		w, err := q.GetRandomAdjective(ctx)
		return w.Lemma, err
	case grammar.Adverb:
		w, err := q.GetRandomAdverb(ctx)
		return w.Lemma, err
	case grammar.Determiner:
		w, err := q.GetRandomDeterminer(ctx)
		return w.Lemma, err
	case grammar.Preposition:
		w, err := q.GetRandomPreposition(ctx)
		return w.Lemma, err
	case grammar.Pronoun:
		w, err := q.GetRandomPronoun(ctx)
		return w.Lemma, err
	default: // Conjunction: Load guarantees every leaf is a POS.
		w, err := q.GetRandomConjunction(ctx)
		return w.Lemma, err
	}
}

// format joins words into a sentence: first letter capitalized, final period.
func format(words []string) string {
	text := strings.Join(words, " ")
	r, size := utf8.DecodeRuneInString(text)
	return string(unicode.ToUpper(r)) + text[size:] + "."
}
