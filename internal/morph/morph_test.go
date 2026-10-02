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
		number morph.Number
		want   string
	}{
		{"walk", morph.Present, morph.Singular, "walks"},
		{"walk", morph.Present, morph.Plural, "walk"},
		{"walk", morph.Past, morph.Singular, "walked"},
		{"walk", morph.Past, morph.Plural, "walked"},
		{"kiss", morph.Present, morph.Singular, "kisses"},
		{"fix", morph.Present, morph.Singular, "fixes"},
		{"buzz", morph.Present, morph.Singular, "buzzes"},
		{"catch", morph.Present, morph.Singular, "catches"},
		{"wash", morph.Present, morph.Singular, "washes"},
		{"go", morph.Present, morph.Singular, "goes"},
		{"carry", morph.Present, morph.Singular, "carries"},
		{"carry", morph.Past, morph.Singular, "carried"},
		{"play", morph.Present, morph.Singular, "plays"},
		{"play", morph.Past, morph.Singular, "played"},
		{"bake", morph.Past, morph.Singular, "baked"},
		{"stop", morph.Past, morph.Singular, "stopped"},
		{"refer", morph.Past, morph.Plural, "referred"},
		{"visit", morph.Past, morph.Singular, "visited"},
		{"eat", morph.Past, morph.Singular, "ate"},
		{"have", morph.Present, morph.Singular, "has"},
		{"have", morph.Present, morph.Plural, "have"},
		{"be", morph.Present, morph.Singular, "is"},
		{"be", morph.Present, morph.Plural, "are"},
		{"be", morph.Past, morph.Singular, "was"},
		{"be", morph.Past, morph.Plural, "were"},
		{"cull out", morph.Present, morph.Singular, "culls out"},
		{"give up", morph.Past, morph.Singular, "gave up"},
		{"take care of", morph.Present, morph.Singular, "takes care of"},
		{"test drive", morph.Present, morph.Singular, "test drives"},
		{"spoon-feed", morph.Past, morph.Singular, "spoon-fed"},
		{"double-check", morph.Present, morph.Singular, "double-checks"},
	}

	for _, tc := range tests {
		t.Run(tc.lemma+"/"+tc.want, func(t *testing.T) {
			if got := v.Conjugate(tc.lemma, tc.tense, tc.number); got != tc.want {
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
		{"Ugandan", "an"},
		{"damp", "a"},
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
	if got := v.Conjugate("swim", morph.Past, morph.Singular); got != "swam" {
		t.Errorf("expected swam; got %q", got)
	}
	if got := v.Conjugate("prefer", morph.Past, morph.Singular); got != "preferred" {
		t.Errorf("expected preferred; got %q", got)
	}
}
