// Package morph inflects English words: noun plurals, verb conjugation and
// the indefinite article. Regular forms come from spelling rules; irregular
// verbs and consonant doubling come from a data file.
package morph

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
)

type Tense string

const (
	Present Tense = "present"
	Past    Tense = "past"
)

type Number string

const (
	Singular Number = "singular"
	Plural   Number = "plural"
)

// particles end phrasal verbs, whose first word carries the inflection
// ("culls out", "takes care of"). Other multi-word verbs inflect their last
// word ("test drives").
var particles = []string{
	"about", "across", "after", "along", "apart", "around", "aside", "away", "back",
	"by", "down", "for", "forth", "in", "into", "of", "off", "on", "out", "over",
	"through", "to", "together", "up", "upon", "with",
}

type irregular struct {
	Base  string `toml:"base"`
	Third string `toml:"third"`
	Past  string `toml:"past"`
}

// Verbs holds the verb data the spelling rules can't derive.
type Verbs struct {
	irregular map[string]irregular
	doubled   map[string]bool
}

// LoadVerbs parses verb morphology TOML: a `doubled` list of bases whose final
// consonant doubles before -ed, and `[[irregular]]` paradigms.
func LoadVerbs(r io.Reader) (*Verbs, error) {
	var f struct {
		Doubled   []string    `toml:"doubled"`
		Irregular []irregular `toml:"irregular"`
	}
	if _, err := toml.NewDecoder(r).Decode(&f); err != nil {
		return nil, fmt.Errorf("morph: decode toml: %w", err)
	}

	v := &Verbs{irregular: map[string]irregular{}, doubled: map[string]bool{}}
	for _, b := range f.Doubled {
		v.doubled[b] = true
	}
	for _, irr := range f.Irregular {
		v.irregular[irr.Base] = irr
	}
	return v, nil
}

// Conjugate inflects a verb lemma for tense and subject number. In a
// multi-word lemma only the head word changes.
func (v *Verbs) Conjugate(lemma string, t Tense, n Number) string {
	words := strings.Fields(lemma)
	head := len(words) - 1
	if slices.Contains(particles, words[head]) {
		head = 0
	}
	words[head] = v.conjugateWord(words[head], t, n)
	return strings.Join(words, " ")
}

func (v *Verbs) conjugateWord(w string, t Tense, n Number) string {
	if w == "be" {
		switch {
		case t == Present && n == Singular:
			return "is"
		case t == Present:
			return "are"
		case n == Singular:
			return "was"
		default:
			return "were"
		}
	}
	if t == Present && n == Plural {
		return w
	}

	// A hyphenated verb not in the data inflects its last part (spoon-fed).
	_, known := v.irregular[w]
	if i := strings.LastIndex(w, "-"); i >= 0 && !known && !v.doubled[w] {
		return w[:i+1] + v.conjugateWord(w[i+1:], t, n)
	}

	irr, ok := v.irregular[w]
	switch {
	case t == Present && ok:
		return irr.Third
	case t == Present && hasAnySuffix(w, "o") && !hasAnySuffix(w, "ao", "eo", "io", "oo", "uo"):
		return w + "es"
	case t == Present:
		return addS(w)
	case ok:
		return irr.Past
	case v.doubled[w]:
		return w + w[len(w)-1:] + "ed"
	case strings.HasSuffix(w, "e"):
		return w + "d"
	case endsConsonantY(w):
		return w[:len(w)-1] + "ied"
	default:
		return w + "ed"
	}
}

// Pluralize returns a noun's plural: irregular when given (OEWN supplies these),
// otherwise by spelling rule on the last word.
func Pluralize(lemma, irregular string) string {
	if irregular != "" {
		return irregular
	}
	i := strings.LastIndex(lemma, " ") + 1
	return lemma[:i] + addS(lemma[i:])
}

// Article picks "a" or "an" by the next word's first letter. Words like
// "hour" and "university" get it wrong.
func Article(next string) string {
	if strings.ContainsAny(strings.ToLower(next[:1]), "aeiou") {
		return "an"
	}
	return "a"
}

// addS applies the -s/-es/-ies rules shared by plurals and third-person verbs.
// Final -o differs between them (goes, photos), so callers handle it.
func addS(w string) string {
	switch {
	case endsConsonantY(w):
		return w[:len(w)-1] + "ies"
	case hasAnySuffix(w, "s", "x", "z", "ch", "sh"):
		return w + "es"
	default:
		return w + "s"
	}
}

func endsConsonantY(w string) bool {
	return strings.HasSuffix(w, "y") && len(w) > 1 && !strings.ContainsRune("aeiou", rune(w[len(w)-2]))
}

func hasAnySuffix(w string, suffixes ...string) bool {
	return slices.ContainsFunc(suffixes, func(s string) bool { return strings.HasSuffix(w, s) })
}
