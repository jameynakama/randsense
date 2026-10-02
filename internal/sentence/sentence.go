// Package sentence generates sentences by expanding a grammar, filling each
// POS leaf with a random word from the lexicon, and inflecting the words so
// they agree.
package sentence

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/jameynakama/randsense/internal/grammar"
	"github.com/jameynakama/randsense/internal/morph"
	"github.com/jameynakama/randsense/internal/store"
)

// Agreement relies on these grammar symbols: an NP's determiner sets the
// number of its nouns, and a VP's verbs agree with the nearest NP before the
// VP among its siblings (the subject).
const (
	nounPhrase = "NP"
	verbPhrase = "VP"
)

// Sentence is a generated sentence and the parse tree it was built from.
type Sentence struct {
	Text string        `json:"text"`
	Tree *grammar.Node `json:"tree"`
}

// leafInfo is what agreement needs from a leaf's lexicon row.
type leafInfo struct {
	number      string // determiners: "singular", "plural" or "either"
	plural      string // nouns: irregular plural, if any
	pluralLemma bool   // nouns: the lemma is already plural ("Rastas")
}

type generator struct {
	ctx    context.Context
	q      store.Querier
	verbs  *morph.Verbs
	rng    *rand.Rand
	tense  morph.Tense
	leaves map[*grammar.Node]leafInfo
	number map[*grammar.Node]morph.Number
}

// Generate expands g, fills every leaf with a random active word, and
// inflects the words: nouns agree with their determiner, verbs with their
// subject, all in one tense chosen at random.
func Generate(ctx context.Context, q store.Querier, g *grammar.Grammar, verbs *morph.Verbs, rng *rand.Rand) (*Sentence, error) {
	tree, err := g.Expand(rng)
	if err != nil {
		return nil, fmt.Errorf("Generate: %w", err)
	}

	gen := &generator{
		ctx:    ctx,
		q:      q,
		verbs:  verbs,
		rng:    rng,
		tense:  morph.Present,
		leaves: map[*grammar.Node]leafInfo{},
		number: map[*grammar.Node]morph.Number{},
	}
	if rng.IntN(2) == 1 {
		gen.tense = morph.Past
	}

	if err := gen.fill(tree); err != nil {
		return nil, fmt.Errorf("Generate: %w", err)
	}
	gen.agreeNouns(tree)
	gen.agreeVerbs(tree, morph.Singular)

	leaves := leafNodes(tree)
	words := make([]string, len(leaves))
	for i, l := range leaves {
		if l.Lemma == "a" && l.POS() == grammar.Determiner && i+1 < len(leaves) {
			l.Word = morph.Article(leaves[i+1].Word)
		}
		words[i] = l.Word
	}

	return &Sentence{Text: format(words), Tree: tree}, nil
}

// fill gives every leaf under n a word. An NP fills its nouns first so that
// a plural lemma ("Rastas") can rule out singular determiners ("a Rastas").
func (gen *generator) fill(n *grammar.Node) error {
	pluralNoun := false
	if n.Symbol == nounPhrase {
		for _, c := range n.Children {
			if c.POS() == grammar.Noun {
				if err := gen.fillLeaf(c, false); err != nil {
					return err
				}
				pluralNoun = pluralNoun || gen.leaves[c].pluralLemma
			}
		}
	}
	for _, c := range n.Children {
		if c.Lemma != "" {
			continue
		}
		if len(c.Children) == 0 {
			if err := gen.fillLeaf(c, pluralNoun); err != nil {
				return err
			}
			continue
		}
		if err := gen.fill(c); err != nil {
			return err
		}
	}
	return nil
}

func (gen *generator) fillLeaf(n *grammar.Node, pluralNoun bool) error {
	lemma, info, err := gen.randomWord(n.POS(), n.Frame(), pluralNoun)
	if err != nil {
		return fmt.Errorf("%s: %w", n.Symbol, err)
	}
	n.Lemma, n.Word = lemma, lemma
	gen.leaves[n] = info
	return nil
}

// randomWord picks a word for pos. A verb must have frame, if one is given;
// pluralNoun restricts a determiner to ones that can go with a plural noun.
func (gen *generator) randomWord(pos grammar.POS, frame grammar.Frame, pluralNoun bool) (string, leafInfo, error) {
	ctx, q := gen.ctx, gen.q
	switch pos {
	case grammar.Noun:
		w, err := q.GetRandomNoun(ctx)
		if err != nil {
			return "", leafInfo{}, err
		}
		var infl struct {
			Plural string `json:"plural"`
		}
		if err := json.Unmarshal(w.Inflections, &infl); err != nil {
			return "", leafInfo{}, fmt.Errorf("noun %q inflections: %w", w.Lemma, err)
		}
		return w.Lemma, leafInfo{plural: infl.Plural, pluralLemma: w.Plural}, nil
	case grammar.Verb:
		if frame != "" {
			w, err := q.GetRandomVerbWithFrame(ctx, string(frame))
			return w.Lemma, leafInfo{}, err
		}
		w, err := q.GetRandomVerb(ctx)
		return w.Lemma, leafInfo{}, err
	case grammar.Adjective:
		w, err := q.GetRandomAdjective(ctx)
		return w.Lemma, leafInfo{}, err
	case grammar.Adverb:
		w, err := q.GetRandomAdverb(ctx)
		return w.Lemma, leafInfo{}, err
	case grammar.Determiner:
		if pluralNoun {
			w, err := q.GetRandomDeterminerWithNumber(ctx, []string{"plural", "either"})
			return w.Lemma, leafInfo{number: w.Number}, err
		}
		w, err := q.GetRandomDeterminer(ctx)
		return w.Lemma, leafInfo{number: w.Number}, err
	case grammar.Preposition:
		w, err := q.GetRandomPreposition(ctx)
		return w.Lemma, leafInfo{}, err
	case grammar.Pronoun:
		w, err := q.GetRandomPronoun(ctx)
		return w.Lemma, leafInfo{}, err
	default: // Conjunction: Load guarantees every leaf is a POS.
		w, err := q.GetRandomConjunction(ctx)
		return w.Lemma, leafInfo{}, err
	}
}

// agreeNouns gives every NP a number: plural if a noun's lemma is already
// plural, otherwise from its determiner (a coin flip for "either", singular
// without one). Its nouns are pluralized to match.
func (gen *generator) agreeNouns(n *grammar.Node) {
	if n.Symbol == nounPhrase {
		number := morph.Singular
		for _, c := range n.Children {
			if gen.leaves[c].pluralLemma {
				number = morph.Plural
			}
		}
		for _, c := range n.Children {
			if c.POS() != grammar.Determiner || number == morph.Plural {
				continue
			}
			switch gen.leaves[c].number {
			case "plural":
				number = morph.Plural
			case "either":
				if gen.rng.IntN(2) == 1 {
					number = morph.Plural
				}
			}
		}
		gen.number[n] = number
		if number == morph.Plural {
			for _, c := range n.Children {
				if c.POS() == grammar.Noun && !gen.leaves[c].pluralLemma {
					c.Word = morph.Pluralize(c.Lemma, gen.leaves[c].plural)
				}
			}
		}
	}
	for _, c := range n.Children {
		gen.agreeNouns(c)
	}
}

// agreeVerbs conjugates verbs for the sentence tense and the number of the
// nearest NP before them among their siblings, or singular if there is none.
// A VP passes its subject's number down to the verbs inside it.
func (gen *generator) agreeVerbs(n *grammar.Node, number morph.Number) {
	for _, c := range n.Children {
		if c.Symbol == nounPhrase {
			number = gen.number[c]
		}
		if len(c.Children) == 0 && c.POS() == grammar.Verb {
			c.Word = gen.verbs.Conjugate(c.Lemma, gen.tense, number)
		}
		if c.Symbol == verbPhrase {
			gen.agreeVerbs(c, number)
		} else {
			gen.agreeVerbs(c, morph.Singular)
		}
	}
}

func leafNodes(n *grammar.Node) []*grammar.Node {
	if len(n.Children) == 0 {
		return []*grammar.Node{n}
	}
	var leaves []*grammar.Node
	for _, c := range n.Children {
		leaves = append(leaves, leafNodes(c)...)
	}
	return leaves
}

// format joins words into a sentence: first letter capitalized, final period.
func format(words []string) string {
	text := strings.Join(words, " ")
	r, size := utf8.DecodeRuneInString(text)
	return string(unicode.ToUpper(r)) + text[size:] + "."
}
