package grammar_test

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/jameynakama/randsense/internal/grammar"
)

func TestLoadRejectsInvalidGrammar(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantErr string
	}{
		{
			"malformed toml",
			`[[rule]`,
			"toml",
		},
		{
			"no start symbol",
			`
			[[rule]]
			symbol = "NP"
			expansion = ["Noun"]
			`,
			`"S"`,
		},
		{
			"empty expansion",
			`
			[[rule]]
			symbol = "S"
			expansion = []
			`,
			`"S"`,
		},
		{
			"negative weight",
			`
			[[rule]]
			symbol = "S"
			expansion = ["Noun"]
			weight = -1
			`,
			`"S"`,
		},
		{
			"zero weight",
			`
			[[rule]]
			symbol = "S"
			expansion = ["Noun"]
			weight = 0
			`,
			`"S"`,
		},
		{
			"undefined symbol",
			`
			[[rule]]
			symbol = "S"
			expansion = ["Noun", "VP"]
			`,
			`"VP"`,
		},
		{
			"rule shadows a POS",
			`
			[[rule]]
			symbol = "S"
			expansion = ["Noun"]

			[[rule]]
			symbol = "Noun"
			expansion = ["Verb"]
			`,
			`"Noun"`,
		},
		{
			"frame on a non-verb",
			`
			[[rule]]
			symbol = "S"
			expansion = ["Noun:transitive"]
			`,
			`"Noun:transitive"`,
		},
		{
			"unknown frame",
			`
			[[rule]]
			symbol = "S"
			expansion = ["Verb:sideways"]
			`,
			`"Verb:sideways"`,
		},
		{
			"verb frame on a conjunction",
			`
			[[rule]]
			symbol = "S"
			expansion = ["Noun", "Conjunction:transitive", "Noun"]
			`,
			`"Conjunction:transitive"`,
		},
		{
			"preposition that no frame fixes",
			`
			[[rule]]
			symbol = "S"
			expansion = ["Noun", "Verb", "Preposition:under", "Noun"]
			`,
			`"Preposition:under"`,
		},
		{
			"complementizer other than whether",
			`
			[[rule]]
			symbol = "S"
			expansion = ["Noun", "Verb", "Complementizer:if", "Verb"]
			`,
			`"Complementizer:if"`,
		},
		{
			"pronoun other than it",
			`
			[[rule]]
			symbol = "S"
			expansion = ["Pronoun:she", "Verb"]
			`,
			`"Pronoun:she"`,
		},
		{
			"conjunction type on a verb",
			`
			[[rule]]
			symbol = "S"
			expansion = ["Verb:np"]
			`,
			`"Verb:np"`,
		},
		{
			"symbol that can never reach terminals",
			`
			[[rule]]
			symbol = "S"
			expansion = ["ADJ", "Noun"]

			[[rule]]
			symbol = "ADJ"
			expansion = ["ADJ", "Adjective"]
			`,
			`"ADJ"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := grammar.Load(strings.NewReader(tc.in))
			if err == nil {
				t.Fatal("expected an error; got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("expected error mentioning %s; got %q", tc.wantErr, err)
			}
		})
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

func TestExpandBuildsTree(t *testing.T) {
	g := mustLoad(t, `
	[[rule]]
	symbol = "S"
	expansion = ["NP", "Verb"]

	[[rule]]
	symbol = "NP"
	expansion = ["Determiner", "Noun"]
	`)

	got, err := g.Expand(rand.New(rand.NewPCG(1, 2)))
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}

	want := &grammar.Node{Symbol: "S", Children: []*grammar.Node{
		{Symbol: "NP", Children: []*grammar.Node{
			{Symbol: "Determiner"},
			{Symbol: "Noun"},
		}},
		{Symbol: "Verb"},
	}}
	if !reflect.DeepEqual(got, want) {
		gotJSON, _ := json.Marshal(got)
		wantJSON, _ := json.Marshal(want)
		t.Errorf("expected %s; got %s", wantJSON, gotJSON)
	}
}

func TestLeavesReturnsPOSInOrder(t *testing.T) {
	tree := &grammar.Node{Symbol: "S", Children: []*grammar.Node{
		{Symbol: "NP", Children: []*grammar.Node{
			{Symbol: "Determiner"},
			{Symbol: "Adjective"},
			{Symbol: "Noun"},
		}},
		{Symbol: "Verb"},
	}}

	got := tree.Leaves()

	want := []grammar.POS{grammar.Determiner, grammar.Adjective, grammar.Noun, grammar.Verb}
	if !slices.Equal(got, want) {
		t.Errorf("expected %v; got %v", want, got)
	}
}

func TestExpandHonorsRelativeWeights(t *testing.T) {
	g := mustLoad(t, `
	[[rule]]
	symbol = "S"
	expansion = ["Noun"]
	weight = 2

	[[rule]]
	symbol = "S"
	expansion = ["Verb"]
	`)
	rng := rand.New(rand.NewPCG(1, 2))

	const n = 3000
	nouns := 0
	for range n {
		tree, err := g.Expand(rng)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}
		if tree.Leaves()[0] == grammar.Noun {
			nouns++
		}
	}

	if got := float64(nouns) / n; got < 0.62 || got > 0.71 {
		t.Errorf("expected Noun about 2/3 of the time; got %.3f", got)
	}
}

func TestExpandFailsOnRunawayGrammar(t *testing.T) {
	g := mustLoad(t, `
	[[rule]]
	symbol = "S"
	expansion = ["S", "S"]
	weight = 10

	[[rule]]
	symbol = "S"
	expansion = ["Noun"]
	`)

	_, err := g.Expand(rand.New(rand.NewPCG(1, 2)))

	if err == nil {
		t.Fatal("expected an error; got nil")
	}
}

func TestProjectGrammarLoadsAndExpands(t *testing.T) {
	f, err := os.Open(filepath.Join("..", "..", "data", "grammar", "grammar.toml"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()

	g, err := grammar.Load(f)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := g.Labeled(); err != nil {
		t.Fatalf("Labeled: %v", err)
	}
	rng := rand.New(rand.NewPCG(1, 2))
	for range 100 {
		if _, err := g.Expand(rng); err != nil {
			t.Fatalf("Expand: %v", err)
		}
	}
}

func TestLoadAcceptsEveryFixedPrepositionFrame(t *testing.T) {
	for frame, prep := range grammar.FixedPrepositions {
		t.Run(string(frame), func(t *testing.T) {
			mustLoad(t, fmt.Sprintf(`
			[[rule]]
			symbol = "S"
			expansion = ["Noun", "Verb:%s", "Noun", "Preposition:%s", "Noun"]
			`, frame, prep))
		})
	}
}

func TestLoadAcceptsThatClause(t *testing.T) {
	mustLoad(t, `
	[[rule]]
	symbol = "S"
	expansion = ["Noun", "Verb:that-clause", "Complementizer", "Noun", "Verb"]
	`)
}

func TestLoadAcceptsToInfinitives(t *testing.T) {
	mustLoad(t, `
	[[rule]]
	symbol = "S"
	expansion = ["Verb:to-infinitive", "To", "Verb", "Verb:transitive-to-infinitive", "Noun", "To", "Verb"]
	`)
}

func TestLoadAcceptsWhetherInfinitive(t *testing.T) {
	mustLoad(t, `
	[[rule]]
	symbol = "S"
	expansion = ["Noun", "Verb:whether-infinitive", "Complementizer:whether", "To", "Verb"]
	`)
}

func TestLoadAcceptsGerund(t *testing.T) {
	mustLoad(t, `
	[[rule]]
	symbol = "S"
	expansion = ["Noun", "Verb:gerund", "Verb"]
	`)
}

func TestLoadAcceptsAdjectiveComplements(t *testing.T) {
	mustLoad(t, `
	[[rule]]
	symbol = "S"
	expansion = ["Noun", "Verb:adjective", "Adjective", "Verb:transitive-adjective", "Noun", "Adjective"]
	`)
}

func TestLoadAcceptsDummySubjects(t *testing.T) {
	mustLoad(t, `
	[[rule]]
	symbol = "S"
	expansion = ["Pronoun:it", "Verb:weather", "Pronoun:it", "Verb:dummy-that-clause", "Complementizer", "Noun", "Verb"]
	`)
}

func TestLoadAcceptsPronounCases(t *testing.T) {
	mustLoad(t, `
	[[rule]]
	symbol = "S"
	expansion = ["Pronoun:genitive", "Verb", "Pronoun:reflexive"]
	`)
}

func TestLoadAcceptsNeitherNor(t *testing.T) {
	mustLoad(t, `
	[[rule]]
	symbol = "S"
	expansion = ["Conjunction:neither", "Noun", "Conjunction:nor", "Noun", "Verb"]
	`)
}

func TestValidateAcceptsTreeOfTerminals(t *testing.T) {
	tree := &grammar.Node{Symbol: "S", Children: []*grammar.Node{
		{Symbol: "NP", Children: []*grammar.Node{{Symbol: "Determiner"}, {Symbol: "Noun"}}},
		{Symbol: "VP", Children: []*grammar.Node{{Symbol: "Verb:transitive"}, {Symbol: "Pronoun:reflexive"}}},
	}}

	if err := tree.Validate(); err != nil {
		t.Errorf("expected no error; got %v", err)
	}
}

func TestValidateRejectsMalformedTrees(t *testing.T) {
	tests := []struct {
		name string
		tree *grammar.Node
		want string
	}{
		{"leaf that isn't a POS", &grammar.Node{Symbol: "S", Children: []*grammar.Node{{Symbol: "NP"}}}, `"NP"`},
		{"unknown qualifier", &grammar.Node{Symbol: "S", Children: []*grammar.Node{{Symbol: "Verb:bogus"}}}, `"Verb:bogus"`},
		{"POS with children", &grammar.Node{Symbol: "S", Children: []*grammar.Node{
			{Symbol: "Noun", Children: []*grammar.Node{{Symbol: "Noun"}}},
		}}, `"Noun"`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.tree.Validate()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("expected error mentioning %s; got %v", tc.want, err)
			}
		})
	}
}

func TestLeafWithFrame(t *testing.T) {
	g := mustLoad(t, `
	[[rule]]
	symbol = "S"
	expansion = ["Noun", "Verb:transitive", "Noun"]
	`)

	tree, err := g.Expand(rand.New(rand.NewPCG(1, 2)))
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}

	verb := tree.Children[1]
	if verb.Symbol != "Verb:transitive" || verb.POS() != grammar.Verb || verb.Qualifier() != string(grammar.Transitive) {
		t.Errorf("expected Verb:transitive leaf; got symbol %q, POS %q, qualifier %q", verb.Symbol, verb.POS(), verb.Qualifier())
	}
	if noun := tree.Children[0]; noun.POS() != grammar.Noun || noun.Qualifier() != "" {
		t.Errorf("expected plain Noun leaf; got POS %q, qualifier %q", noun.POS(), noun.Qualifier())
	}
	want := []grammar.POS{grammar.Noun, grammar.Verb, grammar.Noun}
	if got := tree.Leaves(); !slices.Equal(got, want) {
		t.Errorf("expected leaves %v; got %v", want, got)
	}
}

func TestLoadAcceptsConjunctionTypesAndComma(t *testing.T) {
	for _, q := range []string{"coordinating", "subordinating", "np"} {
		t.Run(q, func(t *testing.T) {
			mustLoad(t, `
			[[rule]]
			symbol = "S"
			expansion = ["Noun", "Comma", "Conjunction:`+q+`", "Noun"]
			`)
		})
	}
}

func TestNodeOmitsEmptyFeatures(t *testing.T) {
	b, err := json.Marshal(&grammar.Node{Symbol: "Comma", Lemma: ",", Word: ","})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	if want := `{"symbol":"Comma","lemma":",","word":","}`; string(b) != want {
		t.Errorf("expected %s; got %s", want, b)
	}
}

func TestNodeKeepsZeroCommonness(t *testing.T) {
	b, err := json.Marshal(&grammar.Node{Symbol: "S", Features: grammar.Features{Commonness: new(0.0)}})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	if want := `{"symbol":"S","features":{"commonness":0}}`; string(b) != want {
		t.Errorf("expected %s; got %s", want, b)
	}
}

func TestLeafNodesAreInSentenceOrder(t *testing.T) {
	det, noun, verb := &grammar.Node{Symbol: "Determiner"}, &grammar.Node{Symbol: "Noun"}, &grammar.Node{Symbol: "Verb"}
	tree := &grammar.Node{Symbol: "S", Children: []*grammar.Node{
		{Symbol: "NP", Children: []*grammar.Node{det, noun}},
		verb,
	}}

	if got := tree.LeafNodes(); !slices.Equal(got, []*grammar.Node{det, noun, verb}) {
		t.Errorf("expected determiner, noun, verb; got %v", got)
	}
}

const labeled = `
[[rule]]
symbol = "S"
expansion = ["NP", "Verb:transitive", "NP"]

[[rule]]
symbol = "NP"
expansion = ["Determiner", "Noun"]

[[rule]]
symbol = "NP"
expansion = ["Pronoun"]

[phrase.S]
label = "sentence"
description = "A complete thought."

[phrase.NP]
label = "noun phrase"
description = "Names a thing."

[slot.Determiner]
label = "determiner"
description = "Points at a noun."

[slot.Noun]
label = "noun"
description = "A thing."

[slot.Pronoun]
label = "pronoun"
description = "Stands in for a noun phrase."

[slot."Verb:transitive"]
label = "transitive verb"
description = "Takes an object."
example = "devoured the goose"
`

func TestDescribeServesRulesInFileOrderWithLabels(t *testing.T) {
	g := mustLoad(t, labeled)

	got := g.Describe()

	want := grammar.Description{
		Start: "S",
		Phrases: map[string]grammar.Phrase{
			"S": {
				Label: grammar.Label{Label: "sentence", Description: "A complete thought."},
				Rules: [][]string{{"NP", "Verb:transitive", "NP"}},
			},
			"NP": {
				Label: grammar.Label{Label: "noun phrase", Description: "Names a thing."},
				Rules: [][]string{{"Determiner", "Noun"}, {"Pronoun"}},
			},
		},
		Slots: map[string]grammar.Label{
			"Determiner":      {Label: "determiner", Description: "Points at a noun."},
			"Noun":            {Label: "noun", Description: "A thing."},
			"Pronoun":         {Label: "pronoun", Description: "Stands in for a noun phrase."},
			"Verb:transitive": {Label: "transitive verb", Description: "Takes an object.", Example: "devoured the goose"},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Describe:\n got %+v\nwant %+v", got, want)
	}
}

func TestDescriptionOmitsAbsentExample(t *testing.T) {
	b, err := json.Marshal(grammar.Label{Label: "noun", Description: "A thing."})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	if want := `{"label":"noun","description":"A thing."}`; string(b) != want {
		t.Errorf("expected %s; got %s", want, b)
	}
}

func TestLabeledAcceptsAFullyLabeledGrammar(t *testing.T) {
	if err := mustLoad(t, labeled).Labeled(); err != nil {
		t.Errorf("expected no error; got %v", err)
	}
}

func TestLabeledReportsWhatIsMissing(t *testing.T) {
	tests := []struct {
		name, drop, want string
	}{
		{"phrase", "[phrase.NP]\nlabel = \"noun phrase\"\ndescription = \"Names a thing.\"\n", `phrase "NP"`},
		{"slot", "[slot.Noun]\nlabel = \"noun\"\ndescription = \"A thing.\"\n", `slot "Noun"`},
		{"description", "description = \"A thing.\"\n", `slot "Noun"`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := strings.Replace(labeled, tc.drop, "", 1)
			if in == labeled {
				t.Fatalf("test grammar doesn't contain %q", tc.drop)
			}
			err := mustLoad(t, in).Labeled()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("expected error mentioning %s; got %v", tc.want, err)
			}
		})
	}
}

func TestLoadRejectsLabelsForUnusedSymbols(t *testing.T) {
	tests := []struct {
		name, extra, want string
	}{
		{"phrase without rules", "[phrase.VP]\nlabel = \"verb phrase\"\ndescription = \"x\"\n", `"VP"`},
		{"slot no rule uses", "[slot.Adverb]\nlabel = \"adverb\"\ndescription = \"x\"\n", `"Adverb"`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := grammar.Load(strings.NewReader(labeled + "\n" + tc.extra))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("expected error mentioning %s; got %v", tc.want, err)
			}
		})
	}
}
