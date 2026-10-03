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
		{"unsupported codes dropped", "devour", []string{"vtaa-inf", "via-for", "nonreferential"}, []string{}},
		{"intransitive fixed prepositions", "devour", []string{"via-to", "vii-to", "via-on-anim", "via-on-inanim"}, []string{"intransitive-on", "intransitive-to"}},
		{"transitive fixed prepositions", "devour", []string{"vtai-to", "vtai-from", "vtaa-with", "vtai-with", "vtaa-of", "vtai-on"},
			[]string{"transitive-from", "transitive-of", "transitive-on", "transitive-to", "transitive-with"}},
		{"that-clause", "devour", []string{"via-that"}, []string{"that-clause"}},
		{"to-infinitives", "devour", []string{"via-to-inf", "vtaa-to-inf"}, []string{"to-infinitive", "transitive-to-infinitive"}},
		{"whether-infinitive", "wonder", []string{"via-whether-inf"}, []string{"whether-infinitive"}},
		{"mislabeled whether-infinitive", "shape", []string{"vtai", "via-whether-inf"}, []string{"transitive"}},
		{"gerunds", "devour", []string{"via-ger", "vtaa-into-ger"}, []string{"gerund", "transitive-into-gerund"}},
		{"mislabeled gerund", "hope", []string{"via", "via-ger"}, []string{"intransitive"}},
		{"mislabeled into-gerund", "talk out of", []string{"vtaa-into-ger"}, []string{}},
		{"into already in lemma", "talk into", []string{"vtaa-into-ger"}, []string{}},
		{"adjective complements", "devour", []string{"via-adj", "vii-adj", "vtii-adj"}, []string{"adjective", "transitive-adjective"}},
		{"mislabeled adjective complements", "break even", []string{"via", "via-adj"}, []string{"intransitive"}},
		{"mislabeled transitive adjective complement", "view as", []string{"vtii-adj"}, []string{}},
		{"preposition already in lemma", "bet on", []string{"via", "via-on-inanim", "vtai-on"}, []string{"intransitive"}},
		{"preposition only inside lemma", "hand onto", []string{"vtai-on"}, []string{"transitive-on"}},
		{"mixed, sorted", "devour", []string{"vtai-pp", "via", "vtaa", "via-inf"}, []string{"intransitive", "transitive", "transitive-pp"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := oewn.MapFrames(tc.lemma, tc.codes); !slices.Equal(got, tc.want) || got == nil {
				t.Errorf("expected %q; got %#v", tc.want, got)
			}
		})
	}
}
