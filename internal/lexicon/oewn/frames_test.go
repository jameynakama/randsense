package oewn_test

import (
	"slices"
	"testing"

	"github.com/jameynakama/randsense/internal/lexicon/oewn"
)

func TestMapFrames(t *testing.T) {
	tests := []struct {
		name  string
		codes []string
		want  []string
	}{
		{"none", nil, []string{}},
		{"intransitive codes collapse", []string{"via", "vii", "vibody"}, []string{"intransitive"}},
		{"transitive codes collapse", []string{"vtaa", "vtai", "vtia", "vtii"}, []string{"transitive"}},
		{"ditransitive", []string{"ditransitive"}, []string{"ditransitive"}},
		{"intransitive PP", []string{"via-pp", "vii-pp"}, []string{"intransitive-pp"}},
		{"transitive PP", []string{"vtaa-pp", "vtai-pp"}, []string{"transitive-pp"}},
		{"unsupported codes dropped", []string{"via-that", "vtai-to", "nonreferential"}, []string{}},
		{"mixed, sorted", []string{"vtai-pp", "via", "vtaa", "via-to-inf"}, []string{"intransitive", "transitive", "transitive-pp"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := oewn.MapFrames(tc.codes); !slices.Equal(got, tc.want) || got == nil {
				t.Errorf("expected %q; got %#v", tc.want, got)
			}
		})
	}
}
