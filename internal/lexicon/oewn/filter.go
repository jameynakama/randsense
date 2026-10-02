package oewn

import "regexp"

var lemmaRE = regexp.MustCompile(`^[A-Za-z']+[A-Za-z' -]*$`)

func AllowLemma(lemma string) bool {
	return lemmaRE.MatchString(lemma)
}

// taxonomicRE matches Latin taxonomic names: a capitalized genus followed by
// lowercase epithets, the last with a Latin ending ("Ribes nigrum"). Without
// the ending check it would also catch common nouns like "Norway rat".
var taxonomicRE = regexp.MustCompile(`^[A-Z][a-z]+( [a-z]+)+(us|um|a|ae|is|ii|i|x|ensis|oides)$`)

// abbreviationRE matches lemmas whose first word is all capitals ("OWLT",
// "PSA blood test").
var abbreviationRE = regexp.MustCompile(`^[A-Z]{2,}\b`)

// AllowNoun applies AllowLemma plus noun-only exclusions: taxonomic names and
// abbreviations.
func AllowNoun(lemma string) bool {
	return AllowLemma(lemma) && !taxonomicRE.MatchString(lemma) && !abbreviationRE.MatchString(lemma)
}
