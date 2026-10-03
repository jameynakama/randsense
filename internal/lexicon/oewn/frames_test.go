package oewn_test

import (
	"slices"
	"testing"

	"github.com/jameynakama/randsense/internal/lexicon/oewn"
)

func TestMapFrames(t *testing.T) {
	tests := []struct {
		name  string
		lemma string
		codes []string
		want  []string
	}{
		{"none", "devour", nil, []string{}},
		{"intransitive codes collapse", "devour", []string{"via", "vii", "vibody"}, []string{"intransitive"}},
		{"transitive codes collapse", "devour", []string{"vtaa", "vtai", "vtia", "vtii"}, []string{"transitive"}},
		{"ditransitive", "devour", []string{"ditransitive"}, []string{"ditransitive"}},
		{"intransitive PP", "devour", []string{"via-pp", "vii-pp"}, []string{"intransitive-pp"}},
		{"transitive PP", "devour", []string{"vtaa-pp", "vtai-pp"}, []string{"transitive-pp"}},
		{"unsupported codes dropped", "devour", []string{"via-whether-inf", "via-for", "nonreferential"}, []string{}},
		{"intransitive fixed prepositions", "devour", []string{"via-to", "vii-to", "via-on-anim", "via-on-inanim"}, []string{"intransitive-on", "intransitive-to"}},
		{"transitive fixed prepositions", "devour", []string{"vtai-to", "vtai-from", "vtaa-with", "vtai-with", "vtaa-of", "vtai-on"},
			[]string{"transitive-from", "transitive-of", "transitive-on", "transitive-to", "transitive-with"}},
		{"that-clause", "devour", []string{"via-that"}, []string{"that-clause"}},
		{"preposition already in lemma", "bet on", []string{"via", "via-on-inanim", "vtai-on"}, []string{"intransitive"}},
		{"preposition only inside lemma", "hand onto", []string{"vtai-on"}, []string{"transitive-on"}},
		{"mixed, sorted", "devour", []string{"vtai-pp", "via", "vtaa", "via-to-inf"}, []string{"intransitive", "transitive", "transitive-pp"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := oewn.MapFrames(tc.lemma, tc.codes); !slices.Equal(got, tc.want) || got == nil {
				t.Errorf("expected %q; got %#v", tc.want, got)
			}
		})
	}
}
