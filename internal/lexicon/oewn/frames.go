package oewn

import (
	"slices"
	"strings"

	"github.com/jameynakama/randsense/internal/grammar"
)

// frameCodes maps OEWN subcat codes to the frames the grammar uses. The codes
// also encode subject animacy (Somebody/Something), which is dropped on
// purpose: only the complement structure matters. Codes not listed (bare
// infinitives and fixed prepositions with a single sense) are unsupported and
// dropped.
var frameCodes = map[string]grammar.Frame{
	"via":                 grammar.Intransitive,
	"vii":                 grammar.Intransitive,
	"vibody":              grammar.Intransitive,
	"vtaa":                grammar.Transitive,
	"vtai":                grammar.Transitive,
	"vtia":                grammar.Transitive,
	"vtii":                grammar.Transitive,
	"ditransitive":        grammar.Ditransitive,
	"via-pp":              grammar.IntransitivePP,
	"vii-pp":              grammar.IntransitivePP,
	"vtaa-pp":             grammar.TransitivePP,
	"vtai-pp":             grammar.TransitivePP,
	"via-on-anim":         grammar.IntransitiveOn,
	"via-on-inanim":       grammar.IntransitiveOn,
	"via-to":              grammar.IntransitiveTo,
	"vii-to":              grammar.IntransitiveTo,
	"vtai-from":           grammar.TransitiveFrom,
	"vtaa-of":             grammar.TransitiveOf,
	"vtai-on":             grammar.TransitiveOn,
	"vtai-to":             grammar.TransitiveTo,
	"vtaa-with":           grammar.TransitiveWith,
	"vtai-with":           grammar.TransitiveWith,
	"via-that":            grammar.ThatClause,
	"via-to-inf":          grammar.ToInfinitive,
	"vtaa-to-inf":         grammar.TransitiveToInfinitive,
	"via-whether-inf":     grammar.WhetherInfinitive,
	"via-ger":             grammar.Gerund,
	"vtaa-into-ger":       grammar.TransitiveIntoGerund,
	"via-adj":             grammar.AdjectiveComplement,
	"vii-adj":             grammar.AdjectiveComplement,
	"vtii-adj":            grammar.TransitiveAdjectiveComplement,
	"nonreferential":      grammar.Weather,
	"nonreferential-sent": grammar.DummyThatClause,
}

// mislabeled lists lemmas OEWN gives a frame they can't take ("shaped
// whether to sing", "hoped singing"). They keep their other frames.
var mislabeled = map[grammar.Frame][]string{
	grammar.WhetherInfinitive: {
		"foreordain", "influence", "mold", "moot", "predestine", "predetermine", "preordain", "regulate", "shape",
	},
	grammar.Gerund: {
		"abstain", "approach", "await", "bask", "call back", "call up", "dabble", "desist", "die", "expect",
		"follow", "hold back", "help oneself", "hope", "lay on the line", "look", "mulct", "plan",
		"play around", "project", "put on the line", "refrain", "retrieve", "scheme", "smatter",
		"supplicate", "think", "wait",
	},
	grammar.TransitiveIntoGerund: {"talk out of"},
	grammar.AdjectiveComplement: {
		"behave", "break even", "call in", "close off", "compact", "do", "drive", "endure", "excavate",
		"fare", "flow", "get along", "get on", "go down", "go off", "go over", "hold out", "make",
		"make out", "move", "pack", "place", "point", "proceed", "rate", "read", "resonate", "ride",
		"roll", "roll up", "savor", "savour", "say", "score", "shut off", "step", "take", "think",
		"unearth", "wash", "work",
	},
	grammar.TransitiveAdjectiveComplement: {
		"do by", "evaluate", "fit", "gloss over", "greet", "handle", "pass judgment", "see", "skate over",
		"skimp over", "slur over", "smooth over", "take for", "tout", "view as",
	},
	grammar.Weather: {"double", "ground", "pull"},
	grammar.DummyThatClause: {
		"add up", "behoove", "behove", "count", "ensue", "facilitate", "go on", "help", "jump", "jump out",
		"leap out", "make out", "pass", "pass off", "provide", "stick out", "turn up", "weigh",
	},
}

// MapFrames converts a verb's subcat codes to sorted, unique frame names. It
// never returns nil, so the JSONB column always holds an array. A lemma that
// already ends in a frame's fixed preposition ("bet on") doesn't get that
// frame, which would say the preposition twice.
//
// OEWN gives "It is ----ing" to weather verbs and also to every "It ----s that
// CLAUSE" verb, so it means weather only on a lemma without the latter.
func MapFrames(lemma string, codes []string) []string {
	frames := []string{}
	for _, c := range codes {
		if c == "nonreferential" && slices.Contains(codes, "nonreferential-sent") {
			continue
		}
		f, ok := frameCodes[c]
		if !ok || slices.Contains(frames, string(f)) || slices.Contains(mislabeled[f], lemma) {
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
