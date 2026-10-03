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
)

var allPOS = []POS{Noun, Verb, Adjective, Adverb, Determiner, Preposition, Pronoun, Conjunction, Comma}

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
}

// Conjunction qualifiers: what a conjunction slot joins. "np" conjunctions
// (and, or) can join noun phrases.
const (
	Coordinating  = "coordinating"
	Subordinating = "subordinating"
	JoinsNPs      = "np"
)

// qualifiers lists what each POS can be qualified with ("Verb:transitive").
// A qualified preposition is that word: "Preposition:with" is always "with".
var qualifiers = map[POS][]string{
	Verb: {
		string(Intransitive), string(Transitive), string(Ditransitive), string(IntransitivePP), string(TransitivePP),
		string(IntransitiveOn), string(IntransitiveTo), string(TransitiveFrom), string(TransitiveOf),
		string(TransitiveOn), string(TransitiveTo), string(TransitiveWith),
	},
	Preposition: {"from", "of", "on", "to", "with"},
	Conjunction: {Coordinating, Subordinating, JoinsNPs},
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

// Grammar maps each non-terminal symbol to its rules.
type Grammar struct {
	rules map[string][]rule
}

type fileRule struct {
	Symbol    string   `toml:"symbol"`
	Expansion []string `toml:"expansion"`
	Weight    *float64 `toml:"weight"`
}

type file struct {
	Rules []fileRule `toml:"rule"`
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

	g := &Grammar{rules: rules}
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

// Node is one constituent of a parse tree. A leaf's Symbol is a POS,
// optionally qualified ("Verb:transitive"). Once the leaf is filled from
// the lexicon, Lemma is the dictionary form and Word the inflected one.
type Node struct {
	Symbol   string  `json:"symbol"`
	Lemma    string  `json:"lemma,omitempty"`
	Word     string  `json:"word,omitempty"`
	Children []*Node `json:"children,omitempty"`
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
