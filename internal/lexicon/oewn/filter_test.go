package oewn_test

import (
	"testing"

	"github.com/jameynakama/randsense/internal/lexicon/oewn"
)

func TestAllowLemmaNouns(t *testing.T) {
	tests := []struct {
		name     string
		noun     string
		expected bool
	}{
		{"basic single-token noun", "good", true},
		{"proper noun, capital preserved", "Microsoft", true},
		{"multi-word with space", "sea anemone", true},
		{"internal apostrophe", "o'clock", true},
		{"internal hyphen", "well-known", true},
		{"leading apostrophe", "'tween", true},
		{"leading apostrophe (slang)", "'hood", true},
		{"one character", "a", true},
		{"leading period and digits", ".22", false},
		{"leading period", ".22-caliber", false},
		{"leading digit and hyphen", "9-11", false},
		{"leading digit", "123abc", false},
		{"embedded periods", "U.S.A", false},
		{"embedded comma + digits (chemical)", "1,4-dihydroxybenzene", false},
		{"nothing to match", "", false},
		{"digits", "goose9", false},
		{"more digits", "well-known1234", false},
		{"underscores", "goose_a", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res := oewn.AllowLemma(tc.noun)
			if res != tc.expected {
				t.Errorf("expected %t; got %t", tc.expected, res)
			}
		})
	}
}

func TestAllowNoun(t *testing.T) {
	tests := []struct {
		name     string
		noun     string
		expected bool
	}{
		{"common noun", "goose", true},
		{"multi-word common noun", "sea anemone", true},
		{"proper noun", "Microsoft", true},
		{"multi-word proper noun", "Old French", true},
		{"proper noun with lowercase connective", "Bill of Rights", true},
		{"plural proper noun", "Boy Scouts", true},
		{"single-word genus", "Sinornis", true},
		{"proper modifier on common noun", "Norway rat", true},
		{"proper modifier on common noun, -s plural", "Brussels sprouts", true},
		{"taxonomic binomial", "Sorex araneus", false},
		{"taxonomic binomial, -um", "Ribes nigrum", false},
		{"taxonomic binomial, -i", "Indri indri", false},
		{"taxonomic trinomial", "Odocoileus hemionus columbianus", false},
		{"abbreviation", "OWLT", false},
		{"abbreviation then words", "PSA blood test", false},
		{"one capital letter", "A", true},
		{"fails AllowLemma", ".22-caliber", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if res := oewn.AllowNoun(tc.noun); res != tc.expected {
				t.Errorf("expected %t; got %t", tc.expected, res)
			}
		})
	}
}
