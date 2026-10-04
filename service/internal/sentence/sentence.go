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
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/jameynakama/randsense/internal/grammar"
	"github.com/jameynakama/randsense/internal/morph"
	"github.com/jameynakama/randsense/internal/store"
)

// Agreement relies on these grammar symbols. An NP followed by a VP among its
// siblings is that VP's subject: its pronouns are nominative and the VP's
// verbs and reflexives agree with it, including reflexives in a PP. An NP's
// person and number come from its pronoun, its determiner, or, for "NP
// Conjunction NP", the coordination. Verbs under an InfVP stay in their base
// form and verbs under a GerVP take -ing, up to any clause nested inside them.
const (
	nounPhrase       = "NP"
	verbPhrase       = "VP"
	prepPhrase       = "PP"
	infinitivePhrase = "InfVP"
	gerundPhrase     = "GerVP"
)

// verbForm is how agreeWithSubjects inflects the verbs in a phrase.
type verbForm int

const (
	finite verbForm = iota
	base
	gerund
)

// Sentence is a generated sentence and the parse tree it was built from.
type Sentence struct {
	Text string        `json:"text"`
	Tree *grammar.Node `json:"tree"`
}

// leafInfo is what agreement needs from a leaf's lexicon row.
type leafInfo struct {
	number      string           // determiners: "singular", "plural" or "either"; pronouns: "singular" or "plural"
	person      morph.Person     // pronouns
	gender      string           // pronouns
	plural      string           // nouns: irregular plural, if any
	pluralLemma bool             // nouns: the lemma is already plural ("Rastas")
	separable   bool             // verbs: the object can go after the first word ("look it up")
	features    grammar.Features // what the tree shows about the word before agreement
}

// frequency is a word's Zipf frequency, or nil when SUBTLEX-US lacks it.
func frequency(n pgtype.Numeric) *float64 {
	if !n.Valid {
		return nil
	}
	// A rounded NUMERIC always converts.
	f, _ := n.Float64Value()
	return &f.Float64
}

// agreement is the person and number a verb agrees with, and the gender a
// reflexive agrees with. An empty gender allows any.
type agreement struct {
	person morph.Person
	number morph.Number
	gender string
}

var thirdSingular = agreement{morph.Third, morph.Singular, ""}

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
	if err := gen.agreeWithSubjects(tree, thirdSingular, finite); err != nil {
		return nil, fmt.Errorf("Realize: %w", err)
	}
	tree.Features.Tense = string(gen.tense)
	tree.Features.Commonness = &commonness

	text := map[*grammar.Node]string{}
	gen.separateParticles(tree, text)
	leaves := tree.LeafNodes()
	words := make([]string, len(leaves))
	for i, l := range leaves {
		if l.Lemma == "a" && l.POS() == grammar.Determiner && i+1 < len(leaves) {
			l.Word = morph.Article(leaves[i+1].Word)
		}
		words[i] = l.Word
		if t, ok := text[l]; ok {
			words[i] = t
		}
	}

	return &Sentence{Text: format(words), Tree: tree}, nil
}

// fill gives every leaf under n a word, except reflexives, which wait for
// agreeWithSubjects. subject says n is (part of) a subject NP. An NP fills its
// nouns first so that a plural lemma ("Rastas") can rule out singular
// determiners ("a Rastas").
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
		if c.Lemma != "" || isReflexive(c) {
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

func isReflexive(n *grammar.Node) bool {
	return n.POS() == grammar.Pronoun && n.Qualifier() == grammar.Reflexive
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
	n.Features = info.features
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
		return w.Lemma, leafInfo{
			plural: infl.Plural, pluralLemma: w.Plural,
			features: grammar.Features{Frequency: frequency(w.Frequency)},
		}, nil
	case grammar.Verb:
		var w store.Verb
		var err error
		if frame := n.Qualifier(); frame != "" {
			w, err = q.GetRandomVerbWithFrame(ctx, store.GetRandomVerbWithFrameParams{Frame: frame, Commonness: gen.commonness})
			if errors.Is(err, pgx.ErrNoRows) {
				err = fmt.Errorf("%w: %w", errEmptyFrame, err)
			}
		} else {
			w, err = q.GetRandomVerb(ctx, gen.commonness)
		}
		if err != nil {
			return "", leafInfo{}, err
		}
		var frames []string
		if err := json.Unmarshal(w.Frames, &frames); err != nil {
			return "", leafInfo{}, fmt.Errorf("verb %q frames: %w", w.Lemma, err)
		}
		return w.Lemma, leafInfo{separable: w.Separable, features: grammar.Features{
			Frames: frames, Separable: w.Separable, Frequency: frequency(w.Frequency),
		}}, nil
	case grammar.Adjective:
		w, err := q.GetRandomAdjective(ctx, gen.commonness)
		return w.Lemma, leafInfo{features: grammar.Features{Frequency: frequency(w.Frequency)}}, err
	case grammar.Adverb:
		w, err := q.GetRandomAdverb(ctx, gen.commonness)
		return w.Lemma, leafInfo{features: grammar.Features{Frequency: frequency(w.Frequency)}}, err
	case grammar.Determiner:
		var w store.Determiner
		var err error
		if pluralNoun {
			w, err = q.GetRandomDeterminerWithNumber(ctx, []string{"plural", "either"})
		} else {
			w, err = q.GetRandomDeterminer(ctx)
		}
		return w.Lemma, leafInfo{number: w.Number, features: grammar.Features{Type: w.Type, Number: w.Number}}, err
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
		if n.Qualifier() == grammar.Genitive {
			// "mine" stands for what is owned, not the owner, so it's third
			// person of either number.
			w, err := q.GetRandomPronounWithCase(ctx, grammar.Genitive)
			number := morph.Singular
			if gen.rng.IntN(2) == 1 {
				number = morph.Plural
			}
			return w.Lemma, leafInfo{number: string(number), person: morph.Third, features: grammar.Features{
				Case: grammar.Genitive, Person: int(morph.Third), Number: string(number),
			}}, err
		}
		if word := n.Qualifier(); word != "" {
			return word, leafInfo{number: string(morph.Singular), person: morph.Third, features: grammar.Features{
				Case: pronounCase, Person: int(morph.Third), Number: string(morph.Singular),
			}}, nil
		}
		w, err := q.GetRandomPronounWithCase(ctx, pronounCase)
		return w.Lemma, leafInfo{number: w.Number, person: morph.Person(w.Person), gender: w.Gender, features: grammar.Features{
			Case: pronounCase, Person: int(w.Person), Number: w.Number, Gender: w.Gender,
		}}, err
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
		case grammar.Neither, grammar.Nor:
			return qualifier, leafInfo{}, nil
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
	n.Features = grammar.Features{Person: int(agr.person), Number: string(agr.number)}
	for _, c := range n.Children {
		if c.POS() != grammar.Noun || len(c.Children) > 0 {
			continue
		}
		c.Features.Number = string(agr.number)
		if agr.number == morph.Plural && !gen.leaves[c].pluralLemma {
			c.Word = morph.Pluralize(c.Lemma, gen.leaves[c].plural)
		}
	}
}

// npAgreement is a coordination's (the last part's for "or" and "nor",
// otherwise plural in the lowest person among the parts: "you and she" is
// second person), a pronoun's, or third person with a number from a plural
// lemma or the determiner (a coin flip for "either", singular without one).
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
		agr := agreement{morph.Third, morph.Plural, ""}
		for _, p := range parts {
			agr.person = min(agr.person, gen.agr[p].person)
		}
		return agr
	}

	agr := thirdSingular
	for _, c := range n.Children {
		info := gen.leaves[c]
		switch {
		case c.POS() == grammar.Pronoun && len(c.Children) == 0:
			return agreement{info.person, morph.Number(info.number), info.gender}
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

// agreeWithSubjects conjugates verbs for the sentence tense and fills
// reflexives, both agreeing with the nearest NP before them among their
// siblings, or third singular if there is none. A VP passes its subject's
// agreement down to the words inside it, and so does a PP, so a reflexive in
// an infinitive or a PP agrees with the object before it ("urge her to devour
// herself", "send the goose to itself"). Verbs in an infinitive keep their
// base form; verbs in a gerund take -ing.
func (gen *generator) agreeWithSubjects(n *grammar.Node, agr agreement, form verbForm) error {
	for _, c := range n.Children {
		if c.Symbol == nounPhrase {
			agr = gen.agr[c]
		}
		if isReflexive(c) {
			w, err := gen.q.GetRandomPronounWithAgreement(gen.ctx, store.GetRandomPronounWithAgreementParams{
				Case:   grammar.Reflexive,
				Person: int16(agr.person),
				Number: string(agr.number),
				Gender: agr.gender,
			})
			if err != nil {
				return fmt.Errorf("%s: %w", c.Symbol, err)
			}
			c.Lemma, c.Word = w.Lemma, w.Lemma
			c.Features = grammar.Features{Case: grammar.Reflexive, Person: int(w.Person), Number: w.Number, Gender: w.Gender}
		}
		if len(c.Children) == 0 && c.POS() == grammar.Verb {
			switch form {
			case finite:
				c.Word = gen.verbs.Conjugate(c.Lemma, gen.tense, agr.person, agr.number)
				c.Features.Form, c.Features.Tense = "finite", string(gen.tense)
				c.Features.Person, c.Features.Number = int(agr.person), string(agr.number)
			case base:
				c.Features.Form = "base"
			case gerund:
				c.Word = gen.verbs.Participle(c.Lemma)
				c.Features.Form = "gerund"
			}
		}
		var err error
		switch c.Symbol {
		case verbPhrase, prepPhrase:
			err = gen.agreeWithSubjects(c, agr, form)
		case infinitivePhrase:
			err = gen.agreeWithSubjects(c, agr, base)
		case gerundPhrase:
			err = gen.agreeWithSubjects(c, agr, gerund)
		default:
			err = gen.agreeWithSubjects(c, thirdSingular, finite)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// separateParticles records in text how to write a separable verb and the
// object right after it, which goes between the verb's first word and the
// rest: "looked it up", "set the goose on fire". A lone particle only follows
// a pronoun object, since "looked up the goose" is fine too. The tree keeps
// the verb's word whole.
func (gen *generator) separateParticles(n *grammar.Node, text map[*grammar.Node]string) {
	for i, c := range n.Children {
		gen.separateParticles(c, text)
		if c.POS() != grammar.Verb || !gen.leaves[c].separable || i+1 == len(n.Children) {
			continue
		}
		// Only multi-word lemmas are separable.
		head, rest, _ := strings.Cut(c.Word, " ")
		obj := pronounObject(n.Children[i+1])
		if obj == nil && strings.Contains(rest, " ") && n.Children[i+1].Symbol == nounPhrase {
			leaves := n.Children[i+1].LeafNodes()
			obj = leaves[len(leaves)-1]
		}
		if obj != nil {
			text[c] = head
			text[obj] = obj.Word + " " + rest
		}
	}
}

// pronounObject is n's pronoun if n is a reflexive or an NP of just a
// pronoun, or nil.
func pronounObject(n *grammar.Node) *grammar.Node {
	if isReflexive(n) {
		return n
	}
	if n.Symbol == nounPhrase && len(n.Children) == 1 && n.Children[0].POS() == grammar.Pronoun {
		return n.Children[0]
	}
	return nil
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
