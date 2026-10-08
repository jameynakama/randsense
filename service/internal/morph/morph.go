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

type Person int

const (
	First  Person = 1
	Second Person = 2
	Third  Person = 3
)

type Number string

const (
	Singular Number = "singular"
	Plural   Number = "plural"
)

// particles mark where a multi-word noun's head ends ("talks of the town").
var particles = []string{
	"about", "across", "after", "along", "apart", "around", "aside", "away", "back",
	"by", "down", "for", "forth", "in", "into", "of", "off", "on", "out", "over",
	"through", "to", "together", "up", "upon", "with",
}

// irregularHeads are hand-curated plurals for a compound noun's head word
// when OEWN gives the compound none ("sweet teeth", "man-children"). Taking
// the head word's own OEWN plural instead would spread its odd ones ("tv
// camerae"). Only "child" also ends closed compounds ("schoolchildren"):
// other heads hide inside unrelated words ("blouse", "mongoose").
var irregularHeads = map[string]string{
	"child": "children",
	"foot":  "feet",
	"goose": "geese",
	"louse": "lice",
	"mouse": "mice",
	"tooth": "teeth",
}

type irregular struct {
	Base              string `toml:"base"`
	Third             string `toml:"third"`
	Past              string `toml:"past"`
	PastParticiple    string `toml:"past_participle"`
	PresentParticiple string `toml:"present_participle"`
}

// Verbs holds the verb data the spelling rules can't derive.
type Verbs struct {
	irregular map[string]irregular
	doubled   map[string]bool
	compounds map[string]bool
}

// LoadVerbs parses verb morphology TOML: a `doubled` list of bases whose final
// consonant doubles before -ed and -ing, a `compounds` list of multi-word verbs
// inflected on their last word, and `[[irregular]]` paradigms, which may be
// whole multi-word lemmas ("wine and dine"). Later files add to earlier ones.
func LoadVerbs(rs ...io.Reader) (*Verbs, error) {
	v := &Verbs{irregular: map[string]irregular{}, doubled: map[string]bool{}, compounds: map[string]bool{}}
	for _, r := range rs {
		var f struct {
			Doubled   []string    `toml:"doubled"`
			Compounds []string    `toml:"compounds"`
			Irregular []irregular `toml:"irregular"`
		}
		if _, err := toml.NewDecoder(r).Decode(&f); err != nil {
			return nil, fmt.Errorf("morph: decode toml: %w", err)
		}
		for _, b := range f.Doubled {
			v.doubled[b] = true
		}
		for _, c := range f.Compounds {
			v.compounds[c] = true
		}
		for _, irr := range f.Irregular {
			if irr.PastParticiple == "" {
				return nil, fmt.Errorf("morph: irregular %q has no past_participle", irr.Base)
			}
			v.irregular[irr.Base] = irr
		}
	}
	return v, nil
}

// Conjugate inflects a verb lemma for tense and subject person and number. In a
// multi-word lemma only the head word changes: the first ("culls out", "talks
// turkey"), or the last in a compound ("test drives"). A lemma with its own
// irregular paradigm changes whole ("wined and dined").
func (v *Verbs) Conjugate(lemma string, t Tense, p Person, n Number) string {
	if _, ok := v.irregular[lemma]; ok {
		return v.conjugateWord(lemma, t, p, n)
	}
	words := strings.Fields(lemma)
	head := v.head(words)
	words[head] = v.conjugateWord(words[head], t, p, n)
	return strings.Join(words, " ")
}

// Participle gives a verb lemma's -ing form, changing the same head word as
// Conjugate ("giving up").
func (v *Verbs) Participle(lemma string) string {
	if _, ok := v.irregular[lemma]; ok {
		return v.participleWord(lemma)
	}
	words := strings.Fields(lemma)
	head := v.head(words)
	words[head] = v.participleWord(words[head])
	return strings.Join(words, " ")
}

// PastParticiple gives a verb lemma's past participle ("taken", "looked up"),
// changing the same head word as Conjugate.
func (v *Verbs) PastParticiple(lemma string) string {
	if _, ok := v.irregular[lemma]; ok {
		return v.pastParticipleWord(lemma)
	}
	words := strings.Fields(lemma)
	head := v.head(words)
	words[head] = v.pastParticipleWord(words[head])
	return strings.Join(words, " ")
}

func (v *Verbs) head(words []string) int {
	if v.compounds[strings.Join(words, " ")] {
		return len(words) - 1
	}
	return 0
}

func (v *Verbs) participleWord(w string) string {
	irr, ok := v.irregular[w]
	if ok {
		return irr.PresentParticiple
	}
	if i := strings.LastIndex(w, "-"); i >= 0 && !v.doubled[w] {
		return w[:i+1] + v.participleWord(w[i+1:])
	}
	switch {
	case w == "be":
		return "being"
	case v.doubled[w]:
		return w + w[len(w)-1:] + "ing"
	case strings.HasSuffix(w, "ie"):
		return w[:len(w)-2] + "ying"
	case hasAnySuffix(w, "ee", "oe", "ye"):
		return w + "ing"
	case strings.HasSuffix(w, "e"):
		return w[:len(w)-1] + "ing"
	default:
		return w + "ing"
	}
}

func (v *Verbs) pastParticipleWord(w string) string {
	if irr, ok := v.irregular[w]; ok {
		return irr.PastParticiple
	}
	if i := strings.LastIndex(w, "-"); i >= 0 && !v.doubled[w] {
		return w[:i+1] + v.pastParticipleWord(w[i+1:])
	}
	if w == "be" {
		return "been"
	}
	return v.regularPast(w)
}

func (v *Verbs) conjugateWord(w string, t Tense, p Person, n Number) string {
	singular := n == Singular && p != Second
	if w == "be" {
		switch {
		case t == Present && singular && p == First:
			return "am"
		case t == Present && singular:
			return "is"
		case t == Present:
			return "are"
		case singular:
			return "was"
		default:
			return "were"
		}
	}
	if t == Present && (n == Plural || p != Third) {
		return w
	}

	// A hyphenated verb not in the data inflects its last part (spoon-fed).
	_, known := v.irregular[w]
	if i := strings.LastIndex(w, "-"); i >= 0 && !known && !v.doubled[w] {
		return w[:i+1] + v.conjugateWord(w[i+1:], t, p, n)
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
	default:
		return v.regularPast(w)
	}
}

// regularPast is the past form the spelling rules give a word with no
// irregular paradigm.
func (v *Verbs) regularPast(w string) string {
	switch {
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
// otherwise from irregularHeads or by spelling rule on the head word, which is
// the last word or the one before the first particle ("talks of the town").
func Pluralize(lemma, irregular string) string {
	if irregular != "" {
		return irregular
	}
	words := strings.Fields(lemma)
	head := len(words) - 1
	for i := 1; i < len(words); i++ {
		if slices.Contains(particles, words[i]) {
			head = i - 1
			break
		}
	}
	words[head] = pluralizeHead(words[head])
	return strings.Join(words, " ")
}

func pluralizeHead(w string) string {
	prefix, last := "", w
	if i := strings.LastIndex(w, "-"); i >= 0 {
		prefix, last = w[:i+1], w[i+1:]
	}
	if p, ok := irregularHeads[last]; ok {
		return prefix + p
	}
	if strings.HasSuffix(w, "child") {
		return w + "ren"
	}
	return addS(w)
}

// Prefixes whose sound overrides the first letter: a vowel letter that
// sounds like "you" or "w" ("a unicorn", "a one-off"), or a silent "h" ("an
// hour"). The exceptions keep the first-letter rule ("an uninvited guest",
// "a herbivore"). Abbreviations like "mph" and "nth" still get it wrong.
var (
	consonantSoundPrefixes = []string{
		"eu", "ewe", "one", "once", "uni", "unanim", "use", "usi", "usu", "usa",
		"uti", "ute", "uta", "uto", "ura", "ure", "uri", "uro", "uru", "ubi",
		"uka", "uku", "ukr", "ufo", "uga", "ugr", "uvu",
	}
	consonantSoundExceptions = []string{"unin", "unim", "unid", "onerous", "oneir"}
	silentHPrefixes          = []string{"hour", "honest", "honor", "honour", "heir", "herb"}
	silentHExceptions        = []string{"herbi", "herbar"}
)

// Article picks "a" or "an" for the next word by its first letter, unless a
// known prefix says it sounds otherwise.
func Article(next string) string {
	w := strings.ToLower(next)
	switch {
	case hasAnyPrefix(w, silentHPrefixes...) && !hasAnyPrefix(w, silentHExceptions...):
		return "an"
	case hasAnyPrefix(w, consonantSoundPrefixes...) && !hasAnyPrefix(w, consonantSoundExceptions...):
		return "a"
	case strings.ContainsAny(w[:1], "aeiou"):
		return "an"
	default:
		return "a"
	}
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

func hasAnyPrefix(w string, prefixes ...string) bool {
	return slices.ContainsFunc(prefixes, func(p string) bool { return strings.HasPrefix(w, p) })
}

func hasAnySuffix(w string, suffixes ...string) bool {
	return slices.ContainsFunc(suffixes, func(s string) bool { return strings.HasSuffix(w, s) })
}
