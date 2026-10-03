package oewn

import (
	"slices"
	"strings"

	"github.com/jameynakama/randsense/internal/grammar"
)

// frameCodes maps OEWN subcat codes to the frames the grammar uses. The codes
// also encode subject animacy (Somebody/Something), which is dropped on
// purpose: only the complement structure matters. Codes not listed
// (infinitives, adjectives, dummy subjects, and fixed prepositions with a
// single sense) are unsupported and dropped.
var frameCodes = map[string]grammar.Frame{
	"via":           grammar.Intransitive,
	"vii":           grammar.Intransitive,
	"vibody":        grammar.Intransitive,
	"vtaa":          grammar.Transitive,
	"vtai":          grammar.Transitive,
	"vtia":          grammar.Transitive,
	"vtii":          grammar.Transitive,
	"ditransitive":  grammar.Ditransitive,
	"via-pp":        grammar.IntransitivePP,
	"vii-pp":        grammar.IntransitivePP,
	"vtaa-pp":       grammar.TransitivePP,
	"vtai-pp":       grammar.TransitivePP,
	"via-on-anim":   grammar.IntransitiveOn,
	"via-on-inanim": grammar.IntransitiveOn,
	"via-to":        grammar.IntransitiveTo,
	"vii-to":        grammar.IntransitiveTo,
	"vtai-from":     grammar.TransitiveFrom,
	"vtaa-of":       grammar.TransitiveOf,
	"vtai-on":       grammar.TransitiveOn,
	"vtai-to":       grammar.TransitiveTo,
	"vtaa-with":     grammar.TransitiveWith,
	"vtai-with":     grammar.TransitiveWith,
	"via-that":      grammar.ThatClause,
}

// MapFrames converts a verb's subcat codes to sorted, unique frame names. It
// never returns nil, so the JSONB column always holds an array. A lemma that
// already ends in a frame's fixed preposition ("bet on") doesn't get that
// frame, which would say the preposition twice.
func MapFrames(lemma string, codes []string) []string {
	frames := []string{}
	for _, c := range codes {
		f, ok := frameCodes[c]
		if !ok || slices.Contains(frames, string(f)) {
			continue
		}
		if prep, fixed := grammar.FixedPrepositions[f]; fixed && strings.HasSuffix(lemma, " "+prep) {
			continue
		}
		frames = append(frames, string(f))
	}
	slices.Sort(frames)
	return frames
}
