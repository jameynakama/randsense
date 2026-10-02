package grammar_test

import (
	"encoding/json"
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
	rng := rand.New(rand.NewPCG(1, 2))
	for range 100 {
		if _, err := g.Expand(rng); err != nil {
			t.Fatalf("Expand: %v", err)
		}
	}
}
