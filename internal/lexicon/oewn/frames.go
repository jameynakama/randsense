package oewn

import (
	"slices"

	"github.com/jameynakama/randsense/internal/grammar"
)

// frameCodes maps OEWN subcat codes to the frames the grammar uses. The codes
// also encode subject animacy (Somebody/Something), which is dropped on
// purpose: only the complement structure matters. Codes not listed (fixed
// prepositions, clauses, infinitives, adjectives, "It is raining") are
// unsupported and dropped.
var frameCodes = map[string]grammar.Frame{
	"via":          grammar.Intransitive,
	"vii":          grammar.Intransitive,
	"vibody":       grammar.Intransitive,
	"vtaa":         grammar.Transitive,
	"vtai":         grammar.Transitive,
	"vtia":         grammar.Transitive,
	"vtii":         grammar.Transitive,
	"ditransitive": grammar.Ditransitive,
	"via-pp":       grammar.IntransitivePP,
	"vii-pp":       grammar.IntransitivePP,
	"vtaa-pp":      grammar.TransitivePP,
	"vtai-pp":      grammar.TransitivePP,
}

// MapFrames converts subcat codes to sorted, unique frame names. It never
// returns nil, so the JSONB column always holds an array.
func MapFrames(codes []string) []string {
	frames := []string{}
	for _, c := range codes {
		if f, ok := frameCodes[c]; ok && !slices.Contains(frames, string(f)) {
			frames = append(frames, string(f))
		}
	}
	slices.Sort(frames)
	return frames
}
