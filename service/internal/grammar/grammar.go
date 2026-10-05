// Package grammar loads a weighted context-free grammar and expands it into
// parse trees whose leaves are part-of-speech slots.
package grammar

import (
	"fmt"
	"io"
	"maps"
	"math/rand/v2"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
)

// start is the symbol every expansion begins from.
const start = "S"

// POS is a terminal symbol: a slot filled with a word from the lexicon.
type POS string

const (
	Noun        POS = "Noun"
	Verb        POS = "Verb"
	Adjective   POS = "Adjective"
	Adverb      POS = "Adverb"
	Determiner  POS = "Determiner"
	Preposition POS = "Preposition"
	Pronoun     POS = "Pronoun"
	Conjunction POS = "Conjunction"
	// Comma is punctuation, not a word: it isn't filled from the lexicon.
	Comma POS = "Comma"
	// Complementizer introduces a clause or infinitive. It is "that" unless
	// qualified: "Complementizer:whether" is always "whether".
	Complementizer POS = "Complementizer"
	// To marks an infinitive and is always "to".
	To POS = "To"
)

var allPOS = []POS{Noun, Verb, Adjective, Adverb, Determiner, Preposition, Pronoun, Conjunction, Comma, Complementizer, To}

// Frame is a verb's complement structure. A grammar can require one on a
// verb slot: "Verb:transitive". A fixed-preposition frame ("transitive-with")
// needs its preposition spelled out in the grammar: "Preposition:with".
type Frame string

const (
	Intransitive   Frame = "intransitive"
	Transitive     Frame = "transitive"
	Ditransitive   Frame = "ditransitive"
	IntransitivePP Frame = "intransitive-pp"
	TransitivePP   Frame = "transitive-pp"
	IntransitiveOn Frame = "intransitive-on"
	IntransitiveTo Frame = "intransitive-to"
	TransitiveFrom Frame = "transitive-from"
	TransitiveOf   Frame = "transitive-of"
	TransitiveOn   Frame = "transitive-on"
	TransitiveTo   Frame = "transitive-to"
	TransitiveWith Frame = "transitive-with"
	ThatClause     Frame = "that-clause"
	ToInfinitive   Frame = "to-infinitive"
	// TransitiveToInfinitive has an object before its infinitive: "urge her
	// to go".
	TransitiveToInfinitive Frame = "transitive-to-infinitive"
	WhetherInfinitive      Frame = "whether-infinitive"
	Gerund                 Frame = "gerund"
	// TransitiveIntoGerund has an object and "into" before its gerund:
	// "coax her into going".
	TransitiveIntoGerund Frame = "transitive-into-gerund"
	// Adjective complements: "seem ugly", "consider her ugly".
	AdjectiveComplement           Frame = "adjective"
	TransitiveAdjectiveComplement Frame = "transitive-adjective"
	// Dummy-subject frames take "Pronoun:it": "it rains", "it seems that...".
	Weather         Frame = "weather"
	DummyThatClause Frame = "dummy-that-clause"
)

// FixedPrepositions maps each fixed-preposition frame to its preposition.
var FixedPrepositions = map[Frame]string{
	IntransitiveOn: "on",
	IntransitiveTo: "to",
	TransitiveFrom: "from",
	TransitiveOf:   "of",
	TransitiveOn:   "on",
	TransitiveTo:   "to",
	TransitiveWith: "with",

	TransitiveIntoGerund: "into",
}

// Conjunction qualifiers: what a conjunction slot joins. "np" conjunctions
// (and, or) can join noun phrases. Neither and Nor are those words, the two
// halves of "neither...nor".
const (
	Coordinating  = "coordinating"
	Subordinating = "subordinating"
	JoinsNPs      = "np"
	Neither       = "neither"
	Nor           = "nor"
)

// Pronoun cases a pronoun slot can be qualified with: "Pronoun:genitive"
// takes a pronoun like "mine", and "Pronoun:reflexive" one like "herself".
const (
	Genitive  = "genitive"
	Reflexive = "reflexive"
)

// qualifiers lists what each POS can be qualified with ("Verb:transitive").
// A qualified preposition, complementizer or pronoun is that word:
// "Preposition:with" is always "with". The pronoun cases are the exception, and
// conjunctions are qualified by type except for Neither and Nor.
var qualifiers = map[POS][]string{
	Verb: {
		string(Intransitive), string(Transitive), string(Ditransitive), string(IntransitivePP), string(TransitivePP),
		string(IntransitiveOn), string(IntransitiveTo), string(TransitiveFrom), string(TransitiveOf),
		string(TransitiveOn), string(TransitiveTo), string(TransitiveWith), string(ThatClause),
		string(ToInfinitive), string(TransitiveToInfinitive), string(WhetherInfinitive),
		string(Gerund), string(TransitiveIntoGerund),
		string(AdjectiveComplement), string(TransitiveAdjectiveComplement),
		string(Weather), string(DummyThatClause),
	},
	Pronoun:        {"it", Genitive, Reflexive},
	Preposition:    {"from", "into", "of", "on", "to", "with"},
	Complementizer: {"whether"},
	Conjunction:    {Coordinating, Subordinating, JoinsNPs, Neither, Nor},
}

func isPOS(symbol string) bool {
	return slices.Contains(allPOS, POS(symbol))
}

// splitSymbol separates a terminal like "Verb:transitive" into its POS and
// qualifier, which is empty when there isn't one.
func splitSymbol(symbol string) (POS, string) {
	pos, qualifier, _ := strings.Cut(symbol, ":")
	return POS(pos), qualifier
}

// checkTerminal reports whether symbol is a POS, optionally qualified, and
// errors on a qualifier that POS doesn't take.
func checkTerminal(symbol string) (bool, error) {
	pos, qualifier := splitSymbol(symbol)
	if !isPOS(string(pos)) {
		return false, nil
	}
	if qualifier != "" && !slices.Contains(qualifiers[pos], qualifier) {
		return false, fmt.Errorf("grammar: %q: qualifiers allowed are %v", symbol, qualifiers)
	}
	return true, nil
}

// rule is one weighted expansion of a symbol. Weights are relative among a
// symbol's rules: weights 2 and 1 mean 2/3 and 1/3.
type rule struct {
	Expansion []string
	Weight    float64
}

// Grammar maps each non-terminal symbol to its rules, and symbols to the
// labels people read.
type Grammar struct {
	rules   map[string][]rule
	phrases map[string]Label
	slots   map[string]Label
}

// Label is how a symbol reads to people: "noun phrase", what it does, and
// for a verb frame an example.
type Label struct {
	Label       string `toml:"label" json:"label"`
	Description string `toml:"description" json:"description"`
	Example     string `toml:"example" json:"example,omitempty"`
}

type fileRule struct {
	Symbol    string   `toml:"symbol"`
	Expansion []string `toml:"expansion"`
	Weight    *float64 `toml:"weight"`
}

type file struct {
	Rules   []fileRule       `toml:"rule"`
	Phrases map[string]Label `toml:"phrase"`
	Slots   map[string]Label `toml:"slot"`
}

// Load parses TOML grammar and validates it. A rule without a weight gets
// weight 1.
func Load(r io.Reader) (*Grammar, error) {
	var f file
	if _, err := toml.NewDecoder(r).Decode(&f); err != nil {
		return nil, fmt.Errorf("grammar: decode toml: %w", err)
	}

	rules := map[string][]rule{}
	for _, fr := range f.Rules {
		w := 1.0
		if fr.Weight != nil {
			w = *fr.Weight
		}
		if len(fr.Expansion) == 0 {
			return nil, fmt.Errorf("grammar: rule for %q has an empty expansion", fr.Symbol)
		}
		if w <= 0 {
			return nil, fmt.Errorf("grammar: rule for %q has non-positive weight %v", fr.Symbol, w)
		}
		if pos, _ := splitSymbol(fr.Symbol); isPOS(string(pos)) {
			return nil, fmt.Errorf("grammar: %q is a part of speech and cannot have rules", fr.Symbol)
		}
		rules[fr.Symbol] = append(rules[fr.Symbol], rule{Expansion: fr.Expansion, Weight: w})
	}

	g := &Grammar{rules: rules, phrases: f.Phrases, slots: f.Slots}
	if err := g.validate(); err != nil {
		return nil, err
	}
	return g, nil
}

func (g *Grammar) validate() error {
	if _, ok := g.rules[start]; !ok {
		return fmt.Errorf("grammar: no rule for start symbol %q", start)
	}

	symbols := slices.Sorted(maps.Keys(g.rules))
	for _, sym := range symbols {
		for _, rule := range g.rules[sym] {
			for _, s := range rule.Expansion {
				if _, ok := g.rules[s]; ok {
					continue
				}
				terminal, err := checkTerminal(s)
				if err != nil {
					return err
				}
				if !terminal {
					return fmt.Errorf("grammar: rule for %q uses undefined symbol %q", sym, s)
				}
			}
		}
	}

	productive := g.productiveSymbols()
	for _, sym := range symbols {
		if !productive[sym] {
			return fmt.Errorf("grammar: %q can never expand to parts of speech", sym)
		}
	}

	used := g.slotSymbols()
	for _, sym := range slices.Sorted(maps.Keys(g.phrases)) {
		if _, ok := g.rules[sym]; !ok {
			return fmt.Errorf("grammar: phrase label for %q, which has no rules", sym)
		}
	}
	for _, sym := range slices.Sorted(maps.Keys(g.slots)) {
		if !used[sym] {
			return fmt.Errorf("grammar: slot label for %q, which no rule uses", sym)
		}
	}
	return nil
}

// productiveSymbols finds every symbol with at least one finite derivation,
// by iterating until no new symbol can be shown productive.
func (g *Grammar) productiveSymbols() map[string]bool {
	productive := map[string]bool{}
	for changed := true; changed; {
		changed = false
		for sym, rules := range g.rules {
			if productive[sym] {
				continue
			}
			for _, rule := range rules {
				if !slices.ContainsFunc(rule.Expansion, func(s string) bool {
					terminal, _ := checkTerminal(s)
					return !terminal && !productive[s]
				}) {
					productive[sym] = true
					changed = true
					break
				}
			}
		}
	}
	return productive
}

// Features is what generation worked out about a node, for readers digging
// into the tree. Which fields a node gets depends on what it is; empty
// ones are omitted.
type Features struct {
	Tense string `json:"tense,omitempty"`
	// Commonness is the root's commonness floor. It's a pointer because 0
	// is a floor too.
	Commonness *float64 `json:"commonness,omitempty"`
	// Form is a verb's: "finite", "base" or "gerund".
	Form   string `json:"form,omitempty"`
	Person int    `json:"person,omitempty"`
	Number string `json:"number,omitempty"`
	Case   string `json:"case,omitempty"`
	Gender string `json:"gender,omitempty"`
	// Type is a determiner's: "definite", "demonstrative" and so on.
	Type string `json:"type,omitempty"`
	// Frames is every frame a verb's lemma has, not just its slot's.
	Frames    []string `json:"frames,omitempty"`
	Separable bool     `json:"separable,omitempty"`
	// Frequency is the word's SUBTLEX-US Zipf value, absent when it has none.
	Frequency *float64 `json:"frequency,omitempty"`
}

// Node is one constituent of a parse tree. A leaf's Symbol is a POS,
// optionally qualified ("Verb:transitive"). Once the leaf is filled from
// the lexicon, Lemma is the dictionary form and Word the inflected one.
// Display is how the leaf is written in the sentence when that differs
// from Word: a separable verb split around its object ("looked" and
// "her up").
type Node struct {
	Symbol   string   `json:"symbol"`
	Lemma    string   `json:"lemma,omitempty"`
	Word     string   `json:"word,omitempty"`
	Display  string   `json:"display,omitempty"`
	Features Features `json:"features,omitzero"`
	Children []*Node  `json:"children,omitempty"`
}

// POS is a leaf's part of speech, without any frame qualifier.
func (n *Node) POS() POS {
	pos, _ := splitSymbol(n.Symbol)
	return pos
}

// Qualifier is a leaf's qualifier ("transitive" in "Verb:transitive"), or
// empty.
func (n *Node) Qualifier() string {
	_, qualifier := splitSymbol(n.Symbol)
	return qualifier
}

// Leaves returns the tree's POS slots in sentence order.
func (n *Node) Leaves() []POS {
	if len(n.Children) == 0 {
		return []POS{n.POS()}
	}
	var leaves []POS
	for _, c := range n.Children {
		leaves = append(leaves, c.Leaves()...)
	}
	return leaves
}

// LeafNodes returns the tree's leaves in sentence order.
func (n *Node) LeafNodes() []*Node {
	if len(n.Children) == 0 {
		return []*Node{n}
	}
	var leaves []*Node
	for _, c := range n.Children {
		leaves = append(leaves, c.LeafNodes()...)
	}
	return leaves
}

// Check reports whether tree derives from the grammar: the root is the
// start symbol, every inner node's children spell one of its symbol's
// rules, and every leaf is a part of speech.
func (g *Grammar) Check(tree *Node) error {
	if tree.Symbol != start {
		return fmt.Errorf("grammar: root is %q, not %q", tree.Symbol, start)
	}
	return g.check(tree)
}

func (g *Grammar) check(n *Node) error {
	if len(n.Children) == 0 {
		// Rules only spell valid terminals, so a leaf that matched its
		// parent's rule is either a part of speech or an empty phrase.
		if _, phrase := g.rules[n.Symbol]; phrase {
			return fmt.Errorf("grammar: %q has no children", n.Symbol)
		}
		return nil
	}
	rules, ok := g.rules[n.Symbol]
	if !ok {
		return fmt.Errorf("grammar: %q is a part of speech and cannot have children", n.Symbol)
	}
	symbols := make([]string, len(n.Children))
	for i, c := range n.Children {
		symbols[i] = c.Symbol
	}
	if !slices.ContainsFunc(rules, func(r rule) bool { return slices.Equal(r.Expansion, symbols) }) {
		return fmt.Errorf("grammar: %q -> %v is not a rule", n.Symbol, symbols)
	}
	for _, c := range n.Children {
		if err := g.check(c); err != nil {
			return err
		}
	}
	return nil
}

// slotSymbols is every part-of-speech symbol a rule uses, qualified ones
// included.
func (g *Grammar) slotSymbols() map[string]bool {
	used := map[string]bool{}
	for _, rules := range g.rules {
		for _, r := range rules {
			for _, s := range r.Expansion {
				if _, phrase := g.rules[s]; !phrase {
					used[s] = true
				}
			}
		}
	}
	return used
}

// Labeled reports the first phrase, then the first slot a rule uses, that
// has no label or no description. Load accepts unlabeled grammars so tests
// can stay small; the server requires labels.
func (g *Grammar) Labeled() error {
	missing := func(l Label) bool { return l.Label == "" || l.Description == "" }
	for _, sym := range slices.Sorted(maps.Keys(g.rules)) {
		if missing(g.phrases[sym]) {
			return fmt.Errorf("grammar: phrase %q has no label and description", sym)
		}
	}
	for _, sym := range slices.Sorted(maps.Keys(g.slotSymbols())) {
		if missing(g.slots[sym]) {
			return fmt.Errorf("grammar: slot %q has no label and description", sym)
		}
	}
	return nil
}

// Phrase is a non-terminal's label and its rules' expansions, in file order.
type Phrase struct {
	Label
	Rules [][]string `json:"rules"`
}

// Description is the grammar as the frontend reads it: what to start from,
// what each phrase can expand to, and how every symbol reads.
type Description struct {
	Start   string            `json:"start"`
	Phrases map[string]Phrase `json:"phrases"`
	Slots   map[string]Label  `json:"slots"`
}

func (g *Grammar) Describe() Description {
	d := Description{Start: start, Phrases: map[string]Phrase{}, Slots: map[string]Label{}}
	for sym, rules := range g.rules {
		p := Phrase{Label: g.phrases[sym]}
		for _, r := range rules {
			p.Rules = append(p.Rules, r.Expansion)
		}
		d.Phrases[sym] = p
	}
	for sym := range g.slotSymbols() {
		d.Slots[sym] = g.slots[sym]
	}
	return d
}

// maxDepth bounds tree depth so a grammar whose recursion outweighs its base
// cases fails instead of growing without limit.
const maxDepth = 32

// Expand derives a parse tree from start, choosing among each symbol's rules
// by weight.
func (g *Grammar) Expand(rng *rand.Rand) (*Node, error) {
	return g.expand(start, 0, rng)
}

func (g *Grammar) expand(symbol string, depth int, rng *rand.Rand) (*Node, error) {
	if depth > maxDepth {
		return nil, fmt.Errorf("grammar: expansion of %q exceeded max depth %d", symbol, maxDepth)
	}
	n := &Node{Symbol: symbol}
	rules, ok := g.rules[symbol]
	if !ok {
		return n, nil
	}
	for _, s := range pick(rules, rng).Expansion {
		child, err := g.expand(s, depth+1, rng)
		if err != nil {
			return nil, err
		}
		n.Children = append(n.Children, child)
	}
	return n, nil
}

func pick(rules []rule, rng *rand.Rand) rule {
	var total float64
	for _, r := range rules {
		total += r.Weight
	}
	x := rng.Float64() * total
	for _, r := range rules {
		x -= r.Weight
		if x < 0 {
			return r
		}
	}
	// Reached only if float rounding leaves x at exactly zero.
	return rules[len(rules)-1]
}
