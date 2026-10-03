package morph_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jameynakama/randsense/internal/morph"
)

const testVerbs = `
doubled = ["stop", "refer"]

[[irregular]]
base = "eat"
third = "eats"
past = "ate"
past_participle = "eaten"
present_participle = "eating"

[[irregular]]
base = "have"
third = "has"
past = "had"
past_participle = "had"
present_participle = "having"

[[irregular]]
base = "give"
third = "gives"
past = "gave"
past_participle = "given"
present_participle = "giving"

[[irregular]]
base = "go"
third = "goes"
past = "went"
past_participle = "gone"
present_participle = "going"

[[irregular]]
base = "panic"
third = "panics"
past = "panicked"
past_participle = "panicked"
present_participle = "panicking"

[[irregular]]
base = "feed"
third = "feeds"
past = "fed"
past_participle = "fed"
present_participle = "feeding"
`

func loadVerbs(t *testing.T) *morph.Verbs {
	t.Helper()
	v, err := morph.LoadVerbs(strings.NewReader(testVerbs))
	if err != nil {
		t.Fatalf("LoadVerbs: %v", err)
	}
	return v
}

func TestConjugate(t *testing.T) {
	v := loadVerbs(t)
	tests := []struct {
		lemma  string
		tense  morph.Tense
		person morph.Person
		number morph.Number
		want   string
	}{
		{"walk", morph.Present, morph.Third, morph.Singular, "walks"},
		{"walk", morph.Present, morph.Third, morph.Plural, "walk"},
		{"walk", morph.Past, morph.Third, morph.Singular, "walked"},
		{"walk", morph.Past, morph.Third, morph.Plural, "walked"},
		{"kiss", morph.Present, morph.Third, morph.Singular, "kisses"},
		{"fix", morph.Present, morph.Third, morph.Singular, "fixes"},
		{"buzz", morph.Present, morph.Third, morph.Singular, "buzzes"},
		{"catch", morph.Present, morph.Third, morph.Singular, "catches"},
		{"wash", morph.Present, morph.Third, morph.Singular, "washes"},
		{"go", morph.Present, morph.Third, morph.Singular, "goes"},
		{"veto", morph.Present, morph.Third, morph.Singular, "vetoes"},
		{"radio", morph.Present, morph.Third, morph.Singular, "radios"},
		{"carry", morph.Present, morph.Third, morph.Singular, "carries"},
		{"carry", morph.Past, morph.Third, morph.Singular, "carried"},
		{"play", morph.Present, morph.Third, morph.Singular, "plays"},
		{"play", morph.Past, morph.Third, morph.Singular, "played"},
		{"bake", morph.Past, morph.Third, morph.Singular, "baked"},
		{"stop", morph.Past, morph.Third, morph.Singular, "stopped"},
		{"refer", morph.Past, morph.Third, morph.Plural, "referred"},
		{"visit", morph.Past, morph.Third, morph.Singular, "visited"},
		{"eat", morph.Past, morph.Third, morph.Singular, "ate"},
		{"have", morph.Present, morph.Third, morph.Singular, "has"},
		{"have", morph.Present, morph.Third, morph.Plural, "have"},
		{"be", morph.Present, morph.Third, morph.Singular, "is"},
		{"be", morph.Present, morph.Third, morph.Plural, "are"},
		{"be", morph.Past, morph.Third, morph.Singular, "was"},
		{"be", morph.Past, morph.Third, morph.Plural, "were"},
		{"be", morph.Present, morph.First, morph.Singular, "am"},
		{"be", morph.Present, morph.Second, morph.Singular, "are"},
		{"be", morph.Present, morph.First, morph.Plural, "are"},
		{"be", morph.Past, morph.First, morph.Singular, "was"},
		{"be", morph.Past, morph.Second, morph.Singular, "were"},
		{"walk", morph.Present, morph.First, morph.Singular, "walk"},
		{"walk", morph.Present, morph.Second, morph.Singular, "walk"},
		{"have", morph.Present, morph.First, morph.Singular, "have"},
		{"eat", morph.Past, morph.First, morph.Singular, "ate"},
		{"cull out", morph.Present, morph.Third, morph.Singular, "culls out"},
		{"give up", morph.Past, morph.Third, morph.Singular, "gave up"},
		{"take care of", morph.Present, morph.Third, morph.Singular, "takes care of"},
		{"test drive", morph.Present, morph.Third, morph.Singular, "test drives"},
		{"go ballistic", morph.Past, morph.Third, morph.Singular, "went ballistic"},
		{"go ballistic", morph.Present, morph.Third, morph.Singular, "goes ballistic"},
		{"stop dead", morph.Present, morph.Third, morph.Singular, "stops dead"},
		{"spoon-feed", morph.Past, morph.Third, morph.Singular, "spoon-fed"},
		{"double-check", morph.Present, morph.Third, morph.Singular, "double-checks"},
	}

	for _, tc := range tests {
		t.Run(tc.lemma+"/"+tc.want, func(t *testing.T) {
			if got := v.Conjugate(tc.lemma, tc.tense, tc.person, tc.number); got != tc.want {
				t.Errorf("expected %q; got %q", tc.want, got)
			}
		})
	}
}

func TestParticiple(t *testing.T) {
	v := loadVerbs(t)
	tests := []struct {
		lemma string
		want  string
	}{
		{"walk", "walking"},
		{"stop", "stopping"},
		{"bake", "baking"},
		{"see", "seeing"},
		{"dye", "dyeing"},
		{"hoe", "hoeing"},
		{"die", "dying"},
		{"be", "being"},
		{"panic", "panicking"},
		{"give up", "giving up"},
		{"take care of", "taking care of"},
		{"test drive", "test driving"},
		{"go ballistic", "going ballistic"},
		{"spoon-feed", "spoon-feeding"},
	}

	for _, tc := range tests {
		t.Run(tc.lemma, func(t *testing.T) {
			if got := v.Participle(tc.lemma); got != tc.want {
				t.Errorf("expected %q; got %q", tc.want, got)
			}
		})
	}
}

func TestPluralize(t *testing.T) {
	tests := []struct {
		lemma     string
		irregular string
		want      string
	}{
		{"dog", "", "dogs"},
		{"bus", "", "buses"},
		{"box", "", "boxes"},
		{"church", "", "churches"},
		{"dish", "", "dishes"},
		{"city", "", "cities"},
		{"day", "", "days"},
		{"photo", "", "photos"},
		{"goose", "geese", "geese"},
		{"male sibling", "", "male siblings"},
		{"crown jewel", "", "crown jewels"},
		{"talk of the town", "", "talks of the town"},
		{"jack in the box", "", "jacks in the box"},
	}

	for _, tc := range tests {
		t.Run(tc.lemma, func(t *testing.T) {
			if got := morph.Pluralize(tc.lemma, tc.irregular); got != tc.want {
				t.Errorf("expected %q; got %q", tc.want, got)
			}
		})
	}
}

func TestArticle(t *testing.T) {
	tests := []struct {
		next string
		want string
	}{
		{"goose", "a"},
		{"egg", "an"},
		{"damp", "a"},
		{"umbrella", "an"},
		// A "you" sound takes "a".
		{"Ugandan", "a"},
		{"European", "a"},
		{"ewe", "a"},
		{"unique", "a"},
		{"unanimous", "a"},
		{"using", "a"},
		{"uterus", "a"},
		{"Uruguayan", "a"},
		{"unimportant", "an"},
		{"uninvited", "an"},
		{"unidentified", "an"},
		{"unannounced", "an"},
		// So does a "w" sound.
		{"one-year", "a"},
		{"once-over", "a"},
		{"onerous", "an"},
		// A silent "h" takes "an".
		{"hour", "an"},
		{"honest", "an"},
		{"honorable", "an"},
		{"heir", "an"},
		{"herb", "an"},
		{"herbal", "an"},
		{"herbivore", "a"},
		{"herbicide", "a"},
		{"herbarium", "a"},
	}

	for _, tc := range tests {
		t.Run(tc.next, func(t *testing.T) {
			if got := morph.Article(tc.next); got != tc.want {
				t.Errorf("expected %q; got %q", tc.want, got)
			}
		})
	}
}

func TestLoadVerbsRejectsMalformedTOML(t *testing.T) {
	if _, err := morph.LoadVerbs(strings.NewReader(`doubled = [`)); err == nil {
		t.Error("expected an error; got nil")
	}
}

func TestProjectVerbMorphologyLoads(t *testing.T) {
	f, err := os.Open(filepath.Join("..", "..", "data", "lexicon", "verb_morphology.toml"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()

	v, err := morph.LoadVerbs(f)
	if err != nil {
		t.Fatalf("LoadVerbs: %v", err)
	}
	if got := v.Conjugate("swim", morph.Past, morph.Third, morph.Singular); got != "swam" {
		t.Errorf("expected swam; got %q", got)
	}
	if got := v.Conjugate("prefer", morph.Past, morph.Third, morph.Singular); got != "preferred" {
		t.Errorf("expected preferred; got %q", got)
	}
	if got := v.Participle("singe"); got != "singeing" {
		t.Errorf("expected singeing; got %q", got)
	}
}
