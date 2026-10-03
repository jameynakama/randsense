package sentence_test

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"maps"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"

	"github.com/jameynakama/randsense/internal/grammar"
	"github.com/jameynakama/randsense/internal/morph"
	"github.com/jameynakama/randsense/internal/sentence"
	"github.com/jameynakama/randsense/internal/store"
)

// fakeQuerier returns fixed words per POS, cycling through dets for
// determiners. Embedding the interface means any query Generate shouldn't
// call panics.
type fakeQuerier struct {
	store.Querier
	err  error
	dets []store.Determiner
	n    int
	noun *store.Noun

	// numberDet answers GetRandomDeterminerWithNumber, which records the
	// numbers it was asked for.
	numberDet store.Determiner
	numbers   []string

	// frame records what GetRandomVerbWithFrame was asked for; frames with
	// no verbs in emptyFrames return pgx.ErrNoRows.
	frame       string
	emptyFrames []string

	// pronouns answers GetRandomPronounWithCase by case; cases records the
	// cases asked for.
	pronouns map[string]store.Pronoun
	cases    []string

	// npConj answers GetRandomNPConjunction; conjTypes records the types
	// GetRandomConjunctionOfType was asked for.
	npConj    store.Conjunction
	conjTypes []string

	// commonness records the floor each content-word lookup was given.
	commonness []float64
}

func newFake(dets ...store.Determiner) *fakeQuerier {
	if len(dets) == 0 {
		dets = []store.Determiner{{Lemma: "the", Number: "either"}}
	}
	return &fakeQuerier{
		dets: dets,
		pronouns: map[string]store.Pronoun{
			"nominative": {Lemma: "she", Person: 3, Number: "singular"},
			"accusative": {Lemma: "her", Person: 3, Number: "singular"},
		},
		npConj: store.Conjunction{Lemma: "and"},
	}
}

func (f *fakeQuerier) GetRandomPronounWithCase(_ context.Context, c string) (store.Pronoun, error) {
	f.cases = append(f.cases, c)
	return f.pronouns[c], f.err
}

func (f *fakeQuerier) GetRandomConjunctionOfType(_ context.Context, t string) (store.Conjunction, error) {
	f.conjTypes = append(f.conjTypes, t)
	return map[string]store.Conjunction{
		"coordinating":  {Lemma: "but"},
		"subordinating": {Lemma: "because"},
	}[t], f.err
}

func (f *fakeQuerier) GetRandomNPConjunction(context.Context) (store.Conjunction, error) {
	return f.npConj, f.err
}

func (f *fakeQuerier) GetRandomNoun(_ context.Context, commonness float64) (store.Noun, error) {
	f.commonness = append(f.commonness, commonness)
	if f.noun != nil {
		return *f.noun, f.err
	}
	return store.Noun{Lemma: "goose", Inflections: []byte(`{"plural":"geese"}`)}, f.err
}

func (f *fakeQuerier) GetRandomVerb(_ context.Context, commonness float64) (store.Verb, error) {
	f.commonness = append(f.commonness, commonness)
	return store.Verb{Lemma: "devour"}, f.err
}

func (f *fakeQuerier) GetRandomVerbWithFrame(_ context.Context, arg store.GetRandomVerbWithFrameParams) (store.Verb, error) {
	f.frame = arg.Frame
	f.commonness = append(f.commonness, arg.Commonness)
	if slices.Contains(f.emptyFrames, arg.Frame) {
		return store.Verb{}, pgx.ErrNoRows
	}
	return store.Verb{Lemma: "give"}, f.err
}

func (f *fakeQuerier) GetRandomAdjective(_ context.Context, commonness float64) (store.Adjective, error) {
	f.commonness = append(f.commonness, commonness)
	return store.Adjective{Lemma: "ugly"}, f.err
}

func (f *fakeQuerier) GetRandomAdverb(_ context.Context, commonness float64) (store.Adverb, error) {
	f.commonness = append(f.commonness, commonness)
	return store.Adverb{Lemma: "loudly"}, f.err
}

func (f *fakeQuerier) GetRandomDeterminer(context.Context) (store.Determiner, error) {
	d := f.dets[f.n%len(f.dets)]
	f.n++
	return d, f.err
}

func (f *fakeQuerier) GetRandomDeterminerWithNumber(_ context.Context, numbers []string) (store.Determiner, error) {
	f.numbers = numbers
	return f.numberDet, f.err
}

func (f *fakeQuerier) GetRandomPreposition(context.Context) (store.Preposition, error) {
	return store.Preposition{Lemma: "under"}, f.err
}

func (f *fakeQuerier) GetRandomConjunction(context.Context) (store.Conjunction, error) {
	return store.Conjunction{Lemma: "and"}, f.err
}

func loadVerbs(t *testing.T) *morph.Verbs {
	t.Helper()
	v, err := morph.LoadVerbs(strings.NewReader(`
	[[irregular]]
	base = "give"
	third = "gives"
	past = "gave"
	`))
	if err != nil {
		t.Fatalf("LoadVerbs: %v", err)
	}
	return v
}

const simpleGrammar = `
[[rule]]
symbol = "S"
expansion = ["NP", "VP"]

[[rule]]
symbol = "NP"
expansion = ["Determiner", "Noun"]

[[rule]]
symbol = "VP"
expansion = ["Verb"]
`

// texts generates n sentences over successive seeds and returns how often
// each text came out.
func texts(t *testing.T, grammarTOML string, newQ func() *fakeQuerier, n int) map[string]int {
	t.Helper()
	g := mustLoad(t, grammarTOML)
	v := loadVerbs(t)
	seen := map[string]int{}
	for i := range n {
		s, err := sentence.Generate(context.Background(), newQ(), g, v, rand.New(rand.NewPCG(uint64(i), 0)), 0)
		if err != nil {
			t.Fatalf("Generate: %v", err)
		}
		seen[s.Text]++
	}
	return seen
}

func assertExactly(t *testing.T, seen map[string]int, want ...string) {
	t.Helper()
	got := slices.Sorted(maps.Keys(seen))
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("expected exactly %q; got %q", want, got)
	}
}

func TestGenerateAgreesWithPluralDeterminer(t *testing.T) {
	seen := texts(t, simpleGrammar, func() *fakeQuerier {
		return newFake(store.Determiner{Lemma: "these", Number: "plural"})
	}, 50)

	assertExactly(t, seen, "These geese devour.", "These geese devoured.")
}

func TestGenerateAgreesWithSingularDeterminer(t *testing.T) {
	seen := texts(t, simpleGrammar, func() *fakeQuerier {
		return newFake(store.Determiner{Lemma: "this", Number: "singular"})
	}, 50)

	assertExactly(t, seen, "This goose devours.", "This goose devoured.")
}

func TestGenerateChoosesNumberForEitherDeterminer(t *testing.T) {
	seen := texts(t, simpleGrammar, func() *fakeQuerier {
		return newFake(store.Determiner{Lemma: "the", Number: "either"})
	}, 100)

	assertExactly(t, seen,
		"The goose devours.", "The goose devoured.", "The geese devour.", "The geese devoured.")
}

func TestGenerateAgreesWithSubjectNotObject(t *testing.T) {
	seen := texts(t, `
	[[rule]]
	symbol = "S"
	expansion = ["NP", "VP"]

	[[rule]]
	symbol = "NP"
	expansion = ["Determiner", "Noun"]

	[[rule]]
	symbol = "VP"
	expansion = ["Verb", "NP"]
	`, func() *fakeQuerier {
		return newFake(
			store.Determiner{Lemma: "this", Number: "singular"},
			store.Determiner{Lemma: "these", Number: "plural"},
		)
	}, 50)

	assertExactly(t, seen, "This goose devours these geese.", "This goose devoured these geese.")
}

func TestGenerateChoosesIndefiniteArticle(t *testing.T) {
	seen := texts(t, `
	[[rule]]
	symbol = "S"
	expansion = ["NP", "VP"]

	[[rule]]
	symbol = "NP"
	expansion = ["Determiner", "Adjective", "Noun"]

	[[rule]]
	symbol = "VP"
	expansion = ["Verb"]
	`, func() *fakeQuerier {
		return newFake(store.Determiner{Lemma: "a", Number: "singular"})
	}, 50)

	assertExactly(t, seen, "An ugly goose devours.", "An ugly goose devoured.")
}

func TestGenerateKeepsLemmaAndWordOnLeaves(t *testing.T) {
	g := mustLoad(t, simpleGrammar)
	q := newFake(store.Determiner{Lemma: "these", Number: "plural"})

	s, err := sentence.Generate(context.Background(), q, g, loadVerbs(t), newRNG(), 0)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	noun := s.Tree.Children[0].Children[1]
	if noun.Lemma != "goose" || noun.Word != "geese" {
		t.Errorf("expected lemma goose, word geese; got lemma %q, word %q", noun.Lemma, noun.Word)
	}
}

func mustLoad(t *testing.T, in string) *grammar.Grammar {
	t.Helper()
	g, err := grammar.Load(strings.NewReader(in))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return g
}

func newRNG() *rand.Rand {
	return rand.New(rand.NewPCG(1, 2))
}

func TestGenerateFillsEveryPOS(t *testing.T) {
	g := mustLoad(t, `
	[[rule]]
	symbol = "S"
	expansion = ["Pronoun", "Adverb", "Verb", "Preposition", "Determiner", "Adjective", "Noun", "Conjunction"]
	`)

	s, err := sentence.Generate(context.Background(), newFake(), g, loadVerbs(t), newRNG(), 0)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	// The pronoun isn't in a subject NP, so it's accusative.
	if s.Text != "Her loudly devours under the ugly goose and." && s.Text != "Her loudly devoured under the ugly goose and." {
		t.Errorf("expected Her loudly devours/devoured under the ugly goose and.; got %q", s.Text)
	}
}

func TestGenerateAppliesCommonnessToEveryContentWord(t *testing.T) {
	g := mustLoad(t, `
	[[rule]]
	symbol = "S"
	expansion = ["Adverb", "Verb", "Verb:transitive", "Adjective", "Noun"]
	`)
	q := newFake()

	if _, err := sentence.Generate(context.Background(), q, g, loadVerbs(t), newRNG(), 3.5); err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if want := []float64{3.5, 3.5, 3.5, 3.5, 3.5}; !slices.Equal(q.commonness, want) {
		t.Errorf("expected commonness %v; got %v", want, q.commonness)
	}
}

func TestGenerateStoresLemmasOnTreeLeaves(t *testing.T) {
	g := mustLoad(t, `
	[[rule]]
	symbol = "S"
	expansion = ["NP", "Verb"]

	[[rule]]
	symbol = "NP"
	expansion = ["Determiner", "Noun"]
	`)

	s, err := sentence.Generate(context.Background(), newFake(), g, loadVerbs(t), newRNG(), 0)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	np := s.Tree.Children[0]
	got := []string{np.Lemma, np.Children[0].Lemma, np.Children[1].Lemma, s.Tree.Children[1].Lemma}
	want := []string{"", "the", "goose", "devour"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("expected words %q; got %q", want, got)
			break
		}
	}
}

func TestGenerateReturnsLookupErrors(t *testing.T) {
	g := mustLoad(t, `
	[[rule]]
	symbol = "S"
	expansion = ["Noun"]
	`)
	boom := errors.New("boom")

	q := newFake()
	q.err = boom

	_, err := sentence.Generate(context.Background(), q, g, loadVerbs(t), newRNG(), 0)

	if !errors.Is(err, boom) {
		t.Errorf("expected boom; got %v", err)
	}
}

// emptyFrameGrammar picks between a frame the fake has verbs for and one it
// doesn't.
const emptyFrameGrammar = `
[[rule]]
symbol = "S"
expansion = ["NP", "VP"]

[[rule]]
symbol = "NP"
expansion = ["Pronoun"]

[[rule]]
symbol = "VP"
expansion = ["Verb:intransitive"]

[[rule]]
symbol = "VP"
expansion = ["Verb:transitive-on", "Pronoun", "Preposition:on", "Pronoun"]
`

func TestGenerateRepicksAFrameWithNoVerbs(t *testing.T) {
	seen := texts(t, emptyFrameGrammar, func() *fakeQuerier {
		q := newFake()
		q.emptyFrames = []string{"transitive-on"}
		return q
	}, 20)

	assertExactly(t, seen, "She gives.", "She gave.")
}

func TestGenerateGivesUpWhenEveryFrameIsEmpty(t *testing.T) {
	q := newFake()
	q.emptyFrames = []string{"intransitive", "transitive-on"}

	_, err := sentence.Generate(context.Background(), q, mustLoad(t, emptyFrameGrammar), loadVerbs(t), newRNG(), 0)

	if !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("expected pgx.ErrNoRows; got %v", err)
	}
}

func TestGenerateDoesNotRepickOnOtherMissingWords(t *testing.T) {
	q := newFake()
	q.err = pgx.ErrNoRows
	g := mustLoad(t, `
	[[rule]]
	symbol = "S"
	expansion = ["Noun", "Verb:intransitive"]
	`)

	_, err := sentence.Generate(context.Background(), q, g, loadVerbs(t), newRNG(), 0)

	if !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("expected pgx.ErrNoRows; got %v", err)
	}
	if len(q.commonness) != 1 {
		t.Errorf("expected one lookup and no retry; got %d", len(q.commonness))
	}
}

func TestGenerateReturnsExpansionErrors(t *testing.T) {
	g := mustLoad(t, `
	[[rule]]
	symbol = "S"
	expansion = ["S", "S"]
	weight = 10

	[[rule]]
	symbol = "S"
	expansion = ["Noun"]
	`)

	_, err := sentence.Generate(context.Background(), newFake(), g, loadVerbs(t), newRNG(), 0)

	if err == nil {
		t.Error("expected an error; got nil")
	}
}

func TestGenerateRejectsMalformedNounInflections(t *testing.T) {
	g := mustLoad(t, simpleGrammar)
	q := newFake()
	q.noun = &store.Noun{Lemma: "goose", Inflections: []byte(`["geese"]`)}

	_, err := sentence.Generate(context.Background(), q, g, loadVerbs(t), newRNG(), 0)

	if err == nil || !strings.Contains(err.Error(), `"goose"`) {
		t.Errorf("expected an error naming goose; got %v", err)
	}
}

func TestGenerateKeepsPluralLemmaPluralWithFittingDeterminer(t *testing.T) {
	var q *fakeQuerier
	seen := texts(t, simpleGrammar, func() *fakeQuerier {
		q = newFake(store.Determiner{Lemma: "a", Number: "singular"})
		q.noun = &store.Noun{Lemma: "Rastas", Inflections: []byte(`{}`), Plural: true}
		q.numberDet = store.Determiner{Lemma: "the", Number: "either"}
		return q
	}, 50)

	assertExactly(t, seen, "The Rastas devour.", "The Rastas devoured.")
	if !slices.Equal(q.numbers, []string{"plural", "either"}) {
		t.Errorf("expected determiner numbers [plural either]; got %v", q.numbers)
	}
}

func TestGeneratePicksVerbWithRequiredFrame(t *testing.T) {
	g := mustLoad(t, `
	[[rule]]
	symbol = "S"
	expansion = ["NP", "VP"]

	[[rule]]
	symbol = "NP"
	expansion = ["Determiner", "Noun"]

	[[rule]]
	symbol = "VP"
	expansion = ["Verb:ditransitive", "NP", "NP"]
	`)
	q := newFake(store.Determiner{Lemma: "this", Number: "singular"})

	s, err := sentence.Generate(context.Background(), q, g, loadVerbs(t), newRNG(), 0)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if q.frame != "ditransitive" {
		t.Errorf("expected a verb lookup for frame ditransitive; got %q", q.frame)
	}
	if s.Text != "This goose gives this goose this goose." && s.Text != "This goose gave this goose this goose." {
		t.Errorf("expected This goose gives/gave this goose this goose.; got %q", s.Text)
	}
}

func TestGenerateFillsFixedPrepositionFromItsQualifier(t *testing.T) {
	g := mustLoad(t, `
	[[rule]]
	symbol = "S"
	expansion = ["NP", "VP"]

	[[rule]]
	symbol = "NP"
	expansion = ["Determiner", "Noun"]

	[[rule]]
	symbol = "VP"
	expansion = ["Verb:transitive-with", "NP", "Preposition:with", "NP"]
	`)
	q := newFake(store.Determiner{Lemma: "this", Number: "singular"})

	s, err := sentence.Generate(context.Background(), q, g, loadVerbs(t), newRNG(), 0)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if q.frame != "transitive-with" {
		t.Errorf("expected a verb lookup for frame transitive-with; got %q", q.frame)
	}
	if s.Text != "This goose gives this goose with this goose." && s.Text != "This goose gave this goose with this goose." {
		t.Errorf("expected This goose gives/gave this goose with this goose.; got %q", s.Text)
	}
}

// leaf and node build trees for Realize.
func leaf(symbol string) *grammar.Node { return &grammar.Node{Symbol: symbol} }

func node(symbol string, children ...*grammar.Node) *grammar.Node {
	return &grammar.Node{Symbol: symbol, Children: children}
}

func detNoun() *grammar.Node { return node("NP", leaf("Determiner"), leaf("Noun")) }

// realized realizes a fresh copy of the tree from build over n seeds and
// returns how often each text came out.
func realized(t *testing.T, build func() *grammar.Node, newQ func() *fakeQuerier, n int) map[string]int {
	t.Helper()
	v := loadVerbs(t)
	seen := map[string]int{}
	for i := range n {
		s, err := sentence.Realize(context.Background(), newQ(), build(), v, rand.New(rand.NewPCG(uint64(i), 0)), 0)
		if err != nil {
			t.Fatalf("Realize: %v", err)
		}
		seen[s.Text]++
	}
	return seen
}

const pronounGrammar = `
[[rule]]
symbol = "S"
expansion = ["NP", "VP"]

[[rule]]
symbol = "NP"
expansion = ["Pronoun"]

[[rule]]
symbol = "VP"
expansion = ["Verb:transitive", "NP"]
`

func TestGenerateUsesNominativeSubjectAndAccusativeObject(t *testing.T) {
	var q *fakeQuerier
	seen := texts(t, pronounGrammar, func() *fakeQuerier { q = newFake(); return q }, 20)

	assertExactly(t, seen, "She gives her.", "She gave her.")
	if !slices.Equal(q.cases, []string{"nominative", "accusative"}) {
		t.Errorf("expected cases [nominative accusative]; got %v", q.cases)
	}
}

func TestGenerateAgreesWithPronounPerson(t *testing.T) {
	tests := []struct {
		pronoun store.Pronoun
		want    []string
	}{
		{store.Pronoun{Lemma: "I", Person: 1, Number: "singular"}, []string{"I give her.", "I gave her."}},
		{store.Pronoun{Lemma: "you", Person: 2, Number: "singular"}, []string{"You give her.", "You gave her."}},
		{store.Pronoun{Lemma: "they", Person: 3, Number: "plural"}, []string{"They give her.", "They gave her."}},
	}

	for _, tc := range tests {
		t.Run(tc.pronoun.Lemma, func(t *testing.T) {
			seen := texts(t, pronounGrammar, func() *fakeQuerier {
				q := newFake()
				q.pronouns["nominative"] = tc.pronoun
				return q
			}, 20)

			assertExactly(t, seen, tc.want...)
		})
	}
}

func TestRealizeCoordinatedSubjectWithAndIsPlural(t *testing.T) {
	seen := realized(t, func() *grammar.Node {
		return node("S", node("NP", detNoun(), leaf("Conjunction:np"), detNoun()), node("VP", leaf("Verb")))
	}, func() *fakeQuerier {
		return newFake(store.Determiner{Lemma: "this", Number: "singular"})
	}, 20)

	assertExactly(t, seen, "This goose and this goose devour.", "This goose and this goose devoured.")
}

func TestRealizeCoordinatedSubjectWithOrAgreesWithLastPart(t *testing.T) {
	singular := store.Determiner{Lemma: "this", Number: "singular"}
	plural := store.Determiner{Lemma: "these", Number: "plural"}
	build := func() *grammar.Node {
		return node("S", node("NP", detNoun(), leaf("Conjunction:np"), detNoun()), node("VP", leaf("Verb")))
	}
	withOr := func(dets ...store.Determiner) func() *fakeQuerier {
		return func() *fakeQuerier {
			q := newFake(dets...)
			q.npConj = store.Conjunction{Lemma: "or"}
			return q
		}
	}

	assertExactly(t, realized(t, build, withOr(singular, plural), 20),
		"This goose or these geese devour.", "This goose or these geese devoured.")
	assertExactly(t, realized(t, build, withOr(plural, singular), 20),
		"These geese or this goose devours.", "These geese or this goose devoured.")
}

func TestRealizePronounInCoordinatedSubjectIsNominative(t *testing.T) {
	seen := realized(t, func() *grammar.Node {
		return node("S",
			node("NP", node("NP", leaf("Pronoun")), leaf("Conjunction:np"), detNoun()),
			node("VP", leaf("Verb:transitive"), node("NP", leaf("Pronoun"))))
	}, func() *fakeQuerier {
		return newFake(store.Determiner{Lemma: "this", Number: "singular"})
	}, 20)

	assertExactly(t, seen, "She and this goose give her.", "She and this goose gave her.")
}

func TestRealizeVerbInNestedVPAgreesWithSubject(t *testing.T) {
	seen := realized(t, func() *grammar.Node {
		return node("S", detNoun(), node("VP", node("VP", leaf("Verb")), leaf("Adverb")))
	}, func() *fakeQuerier {
		return newFake(store.Determiner{Lemma: "these", Number: "plural"})
	}, 20)

	assertExactly(t, seen, "These geese devour loudly.", "These geese devoured loudly.")
}

func TestRealizeThatClauseHasItsOwnSubject(t *testing.T) {
	seen := realized(t, func() *grammar.Node {
		embedded := node("Clause", node("NP", leaf("Pronoun")), node("VP", leaf("Verb:intransitive")))
		return node("S", detNoun(), node("VP", leaf("Verb:that-clause"), leaf("Complementizer"), embedded))
	}, func() *fakeQuerier {
		return newFake(store.Determiner{Lemma: "these", Number: "plural"})
	}, 20)

	// The embedded pronoun is a nominative subject, and its verb agrees with
	// it rather than with the plural outer subject.
	assertExactly(t, seen, "These geese give that she gives.", "These geese gave that she gave.")
}

func TestRealizeInfinitiveKeepsBaseFormAfterAccusativeObject(t *testing.T) {
	seen := realized(t, func() *grammar.Node {
		inf := node("InfVP", leaf("To"), node("VP", leaf("Verb:intransitive")))
		return node("S", detNoun(), node("VP", leaf("Verb:transitive-to-infinitive"), node("NP", leaf("Pronoun")), inf))
	}, func() *fakeQuerier {
		return newFake(store.Determiner{Lemma: "this", Number: "singular"})
	}, 20)

	assertExactly(t, seen, "This goose gives her to give.", "This goose gave her to give.")
}

func TestRealizeClauseInsideInfinitiveIsFinite(t *testing.T) {
	seen := realized(t, func() *grammar.Node {
		embedded := node("Clause", node("NP", leaf("Pronoun")), node("VP", leaf("Verb:intransitive")))
		says := node("VP", leaf("Verb:that-clause"), leaf("Complementizer"), embedded)
		return node("S", detNoun(), node("VP", leaf("Verb:to-infinitive"), node("InfVP", leaf("To"), says)))
	}, func() *fakeQuerier {
		return newFake(store.Determiner{Lemma: "this", Number: "singular"})
	}, 20)

	assertExactly(t, seen, "This goose gives to give that she gives.", "This goose gave to give that she gave.")
}

func TestGenerateJoinsClausesWithTypedConjunctionsInOneTense(t *testing.T) {
	clauses := `
	[[rule]]
	symbol = "Clause"
	expansion = ["NP", "VP"]

	[[rule]]
	symbol = "NP"
	expansion = ["Determiner", "Noun"]

	[[rule]]
	symbol = "VP"
	expansion = ["Verb"]
	`
	tests := []struct {
		name      string
		expansion string
		conjType  string
		want      []string
	}{
		{"coordinating, with comma", `["Clause", "Comma", "Conjunction:coordinating", "Clause"]`, "coordinating",
			[]string{"This goose devours, but this goose devours.", "This goose devoured, but this goose devoured."}},
		{"subordinating", `["Clause", "Conjunction:subordinating", "Clause"]`, "subordinating",
			[]string{"This goose devours because this goose devours.", "This goose devoured because this goose devoured."}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var q *fakeQuerier
			seen := texts(t, `
			[[rule]]
			symbol = "S"
			expansion = `+tc.expansion+clauses, func() *fakeQuerier {
				q = newFake(store.Determiner{Lemma: "this", Number: "singular"})
				return q
			}, 20)

			assertExactly(t, seen, tc.want...)
			if !slices.Equal(q.conjTypes, []string{tc.conjType}) {
				t.Errorf("expected conjunction types [%s]; got %v", tc.conjType, q.conjTypes)
			}
		})
	}
}
