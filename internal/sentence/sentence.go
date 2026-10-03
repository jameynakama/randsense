// Package sentence generates sentences by expanding a grammar, filling each
// POS leaf with a random word from the lexicon, and inflecting the words so
// they agree.
package sentence

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"github.com/jameynakama/randsense/internal/grammar"
	"github.com/jameynakama/randsense/internal/morph"
	"github.com/jameynakama/randsense/internal/store"
)

// Agreement relies on these grammar symbols. An NP followed by a VP among its
// siblings is that VP's subject: its pronouns are nominative and the VP's
// verbs agree with it. An NP's person and number come from its pronoun, its
// determiner, or, for "NP Conjunction NP", the coordination. Verbs under an
// InfVP stay in their base form, up to any clause nested inside it.
const (
	nounPhrase       = "NP"
	verbPhrase       = "VP"
	infinitivePhrase = "InfVP"
)

// Sentence is a generated sentence and the parse tree it was built from.
type Sentence struct {
	Text string        `json:"text"`
	Tree *grammar.Node `json:"tree"`
}

// leafInfo is what agreement needs from a leaf's lexicon row.
type leafInfo struct {
	number      string       // determiners: "singular", "plural" or "either"; pronouns: "singular" or "plural"
	person      morph.Person // pronouns
	plural      string       // nouns: irregular plural, if any
	pluralLemma bool         // nouns: the lemma is already plural ("Rastas")
}

// agreement is the person and number a verb agrees with.
type agreement struct {
	person morph.Person
	number morph.Number
}

var thirdSingular = agreement{morph.Third, morph.Singular}

type generator struct {
	ctx        context.Context
	q          store.Querier
	verbs      *morph.Verbs
	rng        *rand.Rand
	commonness float64
	tense      morph.Tense
	leaves     map[*grammar.Node]leafInfo
	agr        map[*grammar.Node]agreement
}

// maxAttempts bounds how many trees Generate tries when a frame has no verbs.
const maxAttempts = 10

// errEmptyFrame marks a verb frame with no verbs at the commonness floor.
var errEmptyFrame = errors.New("no verb with this frame")

// Generate expands g and realizes the resulting tree. Content words are at
// least as common as commonness, a Zipf frequency; 0 allows any word. A high
// floor can leave a frame without verbs, so a tree that needs one is
// replaced by a fresh expansion.
func Generate(ctx context.Context, q store.Querier, g *grammar.Grammar, verbs *morph.Verbs, rng *rand.Rand, commonness float64) (*Sentence, error) {
	var err error
	for range maxAttempts {
		var tree *grammar.Node
		tree, err = g.Expand(rng)
		if err != nil {
			return nil, fmt.Errorf("Generate: %w", err)
		}
		var s *Sentence
		s, err = Realize(ctx, q, tree, verbs, rng, commonness)
		if !errors.Is(err, errEmptyFrame) {
			return s, err
		}
	}
	return nil, err
}

// Realize fills every leaf of tree with a random active word and inflects the
// words: nouns agree with their determiner, verbs with their subject, all in
// one tense chosen at random. Content words are at least as common as
// commonness.
func Realize(ctx context.Context, q store.Querier, tree *grammar.Node, verbs *morph.Verbs, rng *rand.Rand, commonness float64) (*Sentence, error) {
	gen := &generator{
		ctx:        ctx,
		q:          q,
		verbs:      verbs,
		rng:        rng,
		commonness: commonness,
		tense:      morph.Present,
		leaves:     map[*grammar.Node]leafInfo{},
		agr:        map[*grammar.Node]agreement{},
	}
	if rng.IntN(2) == 1 {
		gen.tense = morph.Past
	}

	if err := gen.fill(tree, false); err != nil {
		return nil, fmt.Errorf("Realize: %w", err)
	}
	gen.agreeNouns(tree)
	gen.agreeVerbs(tree, thirdSingular, false)

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

// fill gives every leaf under n a word. subject says n is (part of) a subject
// NP. An NP fills its nouns first so that a plural lemma ("Rastas") can rule
// out singular determiners ("a Rastas").
func (gen *generator) fill(n *grammar.Node, subject bool) error {
	pluralNoun := false
	if n.Symbol == nounPhrase {
		for _, c := range n.Children {
			if c.POS() == grammar.Noun {
				if err := gen.fillLeaf(c, false, false); err != nil {
					return err
				}
				pluralNoun = pluralNoun || gen.leaves[c].pluralLemma
			}
		}
	}
	for i, c := range n.Children {
		if c.Lemma != "" {
			continue
		}
		if len(c.Children) == 0 {
			if err := gen.fillLeaf(c, pluralNoun, subject); err != nil {
				return err
			}
			continue
		}
		childSubject := c.Symbol == nounPhrase &&
			((n.Symbol == nounPhrase && subject) || verbPhraseFollows(n.Children[i+1:]))
		if err := gen.fill(c, childSubject); err != nil {
			return err
		}
	}
	return nil
}

func verbPhraseFollows(siblings []*grammar.Node) bool {
	for _, s := range siblings {
		if s.Symbol == verbPhrase {
			return true
		}
	}
	return false
}

func (gen *generator) fillLeaf(n *grammar.Node, pluralNoun, subject bool) error {
	lemma, info, err := gen.randomWord(n, pluralNoun, subject)
	if err != nil {
		return fmt.Errorf("%s: %w", n.Symbol, err)
	}
	n.Lemma, n.Word = lemma, lemma
	gen.leaves[n] = info
	return nil
}

// randomWord picks a word for leaf n, honoring its qualifier. pluralNoun
// restricts a determiner to ones that can go with a plural noun; subject
// makes a pronoun nominative rather than accusative.
func (gen *generator) randomWord(n *grammar.Node, pluralNoun, subject bool) (string, leafInfo, error) {
	ctx, q := gen.ctx, gen.q
	switch n.POS() {
	case grammar.Noun:
		w, err := q.GetRandomNoun(ctx, gen.commonness)
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
		if frame := n.Qualifier(); frame != "" {
			w, err := q.GetRandomVerbWithFrame(ctx, store.GetRandomVerbWithFrameParams{Frame: frame, Commonness: gen.commonness})
			if errors.Is(err, pgx.ErrNoRows) {
				err = fmt.Errorf("%w: %w", errEmptyFrame, err)
			}
			return w.Lemma, leafInfo{}, err
		}
		w, err := q.GetRandomVerb(ctx, gen.commonness)
		return w.Lemma, leafInfo{}, err
	case grammar.Adjective:
		w, err := q.GetRandomAdjective(ctx, gen.commonness)
		return w.Lemma, leafInfo{}, err
	case grammar.Adverb:
		w, err := q.GetRandomAdverb(ctx, gen.commonness)
		return w.Lemma, leafInfo{}, err
	case grammar.Determiner:
		if pluralNoun {
			w, err := q.GetRandomDeterminerWithNumber(ctx, []string{"plural", "either"})
			return w.Lemma, leafInfo{number: w.Number}, err
		}
		w, err := q.GetRandomDeterminer(ctx)
		return w.Lemma, leafInfo{number: w.Number}, err
	case grammar.Preposition:
		if prep := n.Qualifier(); prep != "" {
			return prep, leafInfo{}, nil
		}
		w, err := q.GetRandomPreposition(ctx)
		return w.Lemma, leafInfo{}, err
	case grammar.Pronoun:
		pronounCase := "accusative"
		if subject {
			pronounCase = "nominative"
		}
		w, err := q.GetRandomPronounWithCase(ctx, pronounCase)
		return w.Lemma, leafInfo{number: w.Number, person: morph.Person(w.Person)}, err
	case grammar.Comma:
		return ",", leafInfo{}, nil
	case grammar.Complementizer:
		if word := n.Qualifier(); word != "" {
			return word, leafInfo{}, nil
		}
		return "that", leafInfo{}, nil
	case grammar.To:
		return "to", leafInfo{}, nil
	default: // Conjunction: Load guarantees every leaf is a POS.
		var w store.Conjunction
		var err error
		switch qualifier := n.Qualifier(); qualifier {
		case grammar.JoinsNPs:
			w, err = q.GetRandomNPConjunction(ctx)
		case "":
			w, err = q.GetRandomConjunction(ctx)
		default:
			w, err = q.GetRandomConjunctionOfType(ctx, qualifier)
		}
		return w.Lemma, leafInfo{}, err
	}
}

// agreeNouns works out every NP's agreement, inner NPs first, and pluralizes
// nouns to match.
func (gen *generator) agreeNouns(n *grammar.Node) {
	for _, c := range n.Children {
		gen.agreeNouns(c)
	}
	if n.Symbol != nounPhrase {
		return
	}

	agr := gen.npAgreement(n)
	gen.agr[n] = agr
	if agr.number == morph.Plural {
		for _, c := range n.Children {
			if c.POS() == grammar.Noun && len(c.Children) == 0 && !gen.leaves[c].pluralLemma {
				c.Word = morph.Pluralize(c.Lemma, gen.leaves[c].plural)
			}
		}
	}
}

// npAgreement is a coordination's (plural for "and", the last part's for
// "or"), a pronoun's, or third person with a number from a plural lemma or
// the determiner (a coin flip for "either", singular without one).
func (gen *generator) npAgreement(n *grammar.Node) agreement {
	var parts []*grammar.Node
	conjunction := ""
	for _, c := range n.Children {
		switch {
		case c.Symbol == nounPhrase:
			parts = append(parts, c)
		case len(c.Children) == 0 && c.POS() == grammar.Conjunction:
			conjunction = c.Lemma
		}
	}
	if len(parts) > 1 {
		if conjunction == "or" || conjunction == "nor" {
			return gen.agr[parts[len(parts)-1]]
		}
		return agreement{morph.Third, morph.Plural}
	}

	agr := thirdSingular
	for _, c := range n.Children {
		info := gen.leaves[c]
		switch {
		case c.POS() == grammar.Pronoun && len(c.Children) == 0:
			return agreement{info.person, morph.Number(info.number)}
		case info.pluralLemma:
			agr.number = morph.Plural
		}
	}
	for _, c := range n.Children {
		if c.POS() != grammar.Determiner || agr.number == morph.Plural {
			continue
		}
		switch gen.leaves[c].number {
		case "plural":
			agr.number = morph.Plural
		case "either":
			if gen.rng.IntN(2) == 1 {
				agr.number = morph.Plural
			}
		}
	}
	return agr
}

// agreeVerbs conjugates verbs for the sentence tense and the agreement of the
// nearest NP before them among their siblings, or third singular if there is
// none. A VP passes its subject's agreement down to the verbs inside it.
// Verbs in an infinitive keep their base form.
func (gen *generator) agreeVerbs(n *grammar.Node, agr agreement, infinitive bool) {
	for _, c := range n.Children {
		if c.Symbol == nounPhrase {
			agr = gen.agr[c]
		}
		if len(c.Children) == 0 && c.POS() == grammar.Verb && !infinitive {
			c.Word = gen.verbs.Conjugate(c.Lemma, gen.tense, agr.person, agr.number)
		}
		switch c.Symbol {
		case verbPhrase:
			gen.agreeVerbs(c, agr, infinitive)
		case infinitivePhrase:
			gen.agreeVerbs(c, agr, true)
		default:
			gen.agreeVerbs(c, thirdSingular, false)
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

// format joins words into a sentence: commas attached to the word before,
// first letter capitalized, final period.
func format(words []string) string {
	var b strings.Builder
	for i, w := range words {
		if i > 0 && w != "," {
			b.WriteByte(' ')
		}
		b.WriteString(w)
	}
	text := b.String()
	r, size := utf8.DecodeRuneInString(text)
	return string(unicode.ToUpper(r)) + text[size:] + "."
}
