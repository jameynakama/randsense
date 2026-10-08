package sentence_test

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"maps"
	"math/rand/v2"
	"reflect"
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
	// framed, when set, answers GetRandomVerbWithFrame.
	framed *store.Verb

	// pronouns answers GetRandomPronounWithCase by case; cases records the
	// cases asked for.
	pronouns map[string]store.Pronoun
	cases    []string

	// nominatives, when set, answers nominative lookups in turn.
	nominatives []store.Pronoun
	nNom        int

	// agreements records what GetRandomPronounWithAgreement was asked for;
	// it answers from reflexives, or with agreementErr.
	agreements   []store.GetRandomPronounWithAgreementParams
	agreementErr error

	// npConj answers GetRandomNPConjunction; conjTypes records the types
	// GetRandomConjunctionOfType was asked for.
	npConj    store.Conjunction
	conjTypes []string

	// commonness records the floor each content-word lookup was given.
	commonness []float64

	// lookedUp records the lemmas every Lookup query was asked for. Each
	// finds any lemma but "nope". LookupDeterminer gives "a" singular
	// number and LookupNoun makes "Rastas" a plural lemma.
	lookedUp []string
	// singularNouns counts GetRandomSingularNoun calls.
	singularNouns int
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
	if c == "nominative" && len(f.nominatives) > 0 {
		p := f.nominatives[f.nNom%len(f.nominatives)]
		f.nNom++
		return p, f.err
	}
	return f.pronouns[c], f.err
}

var reflexives = []store.Pronoun{
	{Lemma: "myself", Person: 1, Number: "singular", Gender: "epicene"},
	{Lemma: "ourselves", Person: 1, Number: "plural", Gender: "epicene"},
	{Lemma: "yourself", Person: 2, Number: "singular", Gender: "epicene"},
	{Lemma: "yourselves", Person: 2, Number: "plural", Gender: "epicene"},
	{Lemma: "himself", Person: 3, Number: "singular", Gender: "masc"},
	{Lemma: "herself", Person: 3, Number: "singular", Gender: "fem"},
	{Lemma: "itself", Person: 3, Number: "singular", Gender: "neuter"},
	{Lemma: "themselves", Person: 3, Number: "plural", Gender: "epicene"},
}

func (f *fakeQuerier) GetRandomPronounWithAgreement(_ context.Context, arg store.GetRandomPronounWithAgreementParams) (store.Pronoun, error) {
	f.agreements = append(f.agreements, arg)
	if f.agreementErr != nil {
		return store.Pronoun{}, f.agreementErr
	}
	for _, p := range reflexives {
		if arg.Case == "reflexive" && p.Person == arg.Person && p.Number == arg.Number && (arg.Gender == "" || p.Gender == arg.Gender) {
			return p, nil
		}
	}
	return store.Pronoun{}, pgx.ErrNoRows
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
	return store.Verb{Lemma: "devour", Frames: []byte(`["transitive"]`)}, f.err
}

func (f *fakeQuerier) GetRandomVerbWithFrame(_ context.Context, arg store.GetRandomVerbWithFrameParams) (store.Verb, error) {
	f.frame = arg.Frame
	f.commonness = append(f.commonness, arg.Commonness)
	if slices.Contains(f.emptyFrames, arg.Frame) {
		return store.Verb{}, pgx.ErrNoRows
	}
	if f.framed != nil {
		v := *f.framed
		// Tests set framed for its lemma; a row always has frames.
		if v.Frames == nil {
			v.Frames = []byte(`["transitive"]`)
		}
		return v, f.err
	}
	return store.Verb{Lemma: "give", Frames: []byte(`["transitive"]`)}, f.err
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
	past_participle = "given"
	present_participle = "giving"
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

func TestGenerateUsesRegularPluralOverOEWNsWhenFlagged(t *testing.T) {
	seen := texts(t, simpleGrammar, func() *fakeQuerier {
		q := newFake(store.Determiner{Lemma: "these", Number: "plural"})
		q.noun = &store.Noun{Lemma: "camera", Inflections: []byte(`{"plural":"camerae"}`), RegularPlural: true}
		return q
	}, 20)

	assertExactly(t, seen, "These cameras devour.", "These cameras devoured.")
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
		s, err := sentence.Realize(context.Background(), newQ(), mustLoad(t, simpleGrammar), build(), v, rand.New(rand.NewPCG(uint64(i), 0)), 0)
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

func TestGenerateGenitivePronounIsThirdPersonOfEitherNumber(t *testing.T) {
	var q *fakeQuerier
	seen := texts(t, `
	[[rule]]
	symbol = "S"
	expansion = ["NP", "VP"]

	[[rule]]
	symbol = "NP"
	expansion = ["Pronoun:genitive"]

	[[rule]]
	symbol = "VP"
	expansion = ["Verb:transitive", "NP"]
	`, func() *fakeQuerier {
		q = newFake()
		q.pronouns["genitive"] = store.Pronoun{Lemma: "mine", Person: 1, Number: "singular"}
		return q
	}, 50)

	// "Mine" stands for whatever is owned, so the possessor's person and
	// number don't apply.
	assertExactly(t, seen, "Mine gives mine.", "Mine give mine.", "Mine gave mine.")
	if !slices.Equal(q.cases, []string{"genitive", "genitive"}) {
		t.Errorf("expected cases [genitive genitive]; got %v", q.cases)
	}
}

// reflexiveObject builds "subject Verb:transitive Pronoun:reflexive".
func reflexiveObject(subject *grammar.Node) *grammar.Node {
	return node("S", subject, node("VP", leaf("Verb:transitive"), leaf("Pronoun:reflexive")))
}

func TestRealizeReflexiveAgreesWithPronounSubject(t *testing.T) {
	tests := []struct {
		subject store.Pronoun
		want    []string
	}{
		{store.Pronoun{Lemma: "I", Person: 1, Number: "singular", Gender: "epicene"}, []string{"I give myself.", "I gave myself."}},
		{store.Pronoun{Lemma: "we", Person: 1, Number: "plural", Gender: "epicene"}, []string{"We give ourselves.", "We gave ourselves."}},
		{store.Pronoun{Lemma: "you", Person: 2, Number: "singular", Gender: "epicene"}, []string{"You give yourself.", "You gave yourself."}},
		{store.Pronoun{Lemma: "you", Person: 2, Number: "plural", Gender: "epicene"}, []string{"You give yourselves.", "You gave yourselves."}},
		{store.Pronoun{Lemma: "she", Person: 3, Number: "singular", Gender: "fem"}, []string{"She gives herself.", "She gave herself."}},
		{store.Pronoun{Lemma: "it", Person: 3, Number: "singular", Gender: "neuter"}, []string{"It gives itself.", "It gave itself."}},
		{store.Pronoun{Lemma: "they", Person: 3, Number: "plural", Gender: "epicene"}, []string{"They give themselves.", "They gave themselves."}},
	}

	for _, tc := range tests {
		t.Run(tc.want[0], func(t *testing.T) {
			seen := realized(t, func() *grammar.Node {
				return reflexiveObject(node("NP", leaf("Pronoun")))
			}, func() *fakeQuerier {
				q := newFake()
				q.pronouns["nominative"] = tc.subject
				return q
			}, 20)

			assertExactly(t, seen, tc.want...)
		})
	}
}

func TestRealizeReflexiveAfterNounSubjectTakesAnyGender(t *testing.T) {
	tests := []struct {
		det  store.Determiner
		want store.GetRandomPronounWithAgreementParams
	}{
		{store.Determiner{Lemma: "this", Number: "singular"}, store.GetRandomPronounWithAgreementParams{Case: "reflexive", Person: 3, Number: "singular"}},
		{store.Determiner{Lemma: "these", Number: "plural"}, store.GetRandomPronounWithAgreementParams{Case: "reflexive", Person: 3, Number: "plural"}},
	}

	for _, tc := range tests {
		t.Run(tc.det.Lemma, func(t *testing.T) {
			q := newFake(tc.det)
			_, err := sentence.Realize(context.Background(), q, mustLoad(t, simpleGrammar), reflexiveObject(detNoun()), loadVerbs(t), rand.New(rand.NewPCG(1, 0)), 0)
			if err != nil {
				t.Fatalf("Realize: %v", err)
			}
			if !slices.Equal(q.agreements, []store.GetRandomPronounWithAgreementParams{tc.want}) {
				t.Errorf("expected agreement lookups %+v; got %+v", tc.want, q.agreements)
			}
		})
	}
}

func TestRealizeReflexiveInInfinitiveAgreesWithObject(t *testing.T) {
	seen := realized(t, func() *grammar.Node {
		inf := node("InfVP", leaf("To"), node("VP", leaf("Verb:transitive"), leaf("Pronoun:reflexive")))
		return node("S", detNoun(), node("VP", leaf("Verb:transitive-to-infinitive"), node("NP", leaf("Pronoun")), inf))
	}, func() *fakeQuerier {
		q := newFake(store.Determiner{Lemma: "these", Number: "plural"})
		q.pronouns["accusative"] = store.Pronoun{Lemma: "her", Person: 3, Number: "singular", Gender: "fem"}
		return q
	}, 20)

	// "Her" is who gives, so she gives herself, not the geese themselves.
	assertExactly(t, seen, "These geese give her to give herself.", "These geese gave her to give herself.")
}

func TestRealizeReflexiveInPrepositionalPhrase(t *testing.T) {
	reflexivePP := func() *grammar.Node { return node("PP", leaf("Preposition"), leaf("Pronoun:reflexive")) }
	tests := []struct {
		name  string
		build func() *grammar.Node
		want  []string
	}{
		{"agrees with the subject", func() *grammar.Node {
			return node("S", node("NP", leaf("Pronoun")), node("VP", leaf("Verb:intransitive-pp"), reflexivePP()))
		}, []string{"She gives under herself.", "She gave under herself."}},
		{"agrees with an object before it", func() *grammar.Node {
			return node("S", node("NP", leaf("Pronoun")), node("VP", leaf("Verb:transitive-pp"), detNoun(), reflexivePP()))
		}, []string{"She gives these geese under themselves.", "She gave these geese under themselves."}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			seen := realized(t, tc.build, func() *fakeQuerier {
				q := newFake(store.Determiner{Lemma: "these", Number: "plural"})
				q.pronouns["nominative"] = store.Pronoun{Lemma: "she", Person: 3, Number: "singular", Gender: "fem"}
				return q
			}, 20)

			assertExactly(t, seen, tc.want...)
		})
	}
}

func TestRealizeReflexiveAfterCoordinatedSubjectTakesLowestPerson(t *testing.T) {
	you := store.Pronoun{Lemma: "you", Person: 2, Number: "singular", Gender: "epicene"}
	she := store.Pronoun{Lemma: "she", Person: 3, Number: "singular", Gender: "fem"}
	he := store.Pronoun{Lemma: "he", Person: 3, Number: "singular", Gender: "masc"}
	i := store.Pronoun{Lemma: "I", Person: 1, Number: "singular", Gender: "epicene"}
	tests := []struct {
		first, second store.Pronoun
		want          []string
	}{
		{you, she, []string{"You and she give yourselves.", "You and she gave yourselves."}},
		{she, i, []string{"She and I give ourselves.", "She and I gave ourselves."}},
		{she, he, []string{"She and he give themselves.", "She and he gave themselves."}},
	}

	for _, tc := range tests {
		t.Run(tc.want[0], func(t *testing.T) {
			seen := realized(t, func() *grammar.Node {
				return reflexiveObject(node("NP", node("NP", leaf("Pronoun")), leaf("Conjunction:np"), node("NP", leaf("Pronoun"))))
			}, func() *fakeQuerier {
				q := newFake()
				q.nominatives = []store.Pronoun{tc.first, tc.second}
				return q
			}, 20)

			assertExactly(t, seen, tc.want...)
		})
	}
}

func TestRealizeReturnsReflexiveLookupErrors(t *testing.T) {
	boom := errors.New("boom")
	q := newFake()
	q.agreementErr = boom

	_, err := sentence.Realize(context.Background(), q, mustLoad(t, simpleGrammar), reflexiveObject(detNoun()), loadVerbs(t), rand.New(rand.NewPCG(1, 0)), 0)
	if !errors.Is(err, boom) {
		t.Errorf("expected boom; got %v", err)
	}
}

func TestRealizeSplitsSeparableVerbAroundObject(t *testing.T) {
	lookUp := store.Verb{Lemma: "look up", Separable: true}
	countOn := store.Verb{Lemma: "count on"}
	pronoun := func() *grammar.Node { return node("NP", leaf("Pronoun")) }
	tests := []struct {
		name   string
		verb   store.Verb
		object func() *grammar.Node
		want   []string
	}{
		{"pronoun", lookUp, pronoun, []string{"She looks her up.", "She looked her up."}},
		{"reflexive", lookUp, func() *grammar.Node { return leaf("Pronoun:reflexive") }, []string{"She looks herself up.", "She looked herself up."}},
		{"noun phrase", lookUp, detNoun, []string{"She looks up these geese.", "She looked up these geese."}},
		{"coordination", lookUp, func() *grammar.Node { return node("NP", pronoun(), leaf("Conjunction:np"), detNoun()) },
			[]string{"She looks up her and these geese.", "She looked up her and these geese."}},
		{"inseparable", countOn, pronoun, []string{"She counts on her.", "She counted on her."}},
		{"idiom with a noun phrase", store.Verb{Lemma: "call into question", Separable: true}, detNoun,
			[]string{"She calls these geese into question.", "She called these geese into question."}},
		{"idiom with a pronoun", store.Verb{Lemma: "call to order", Separable: true}, pronoun,
			[]string{"She calls her to order.", "She called her to order."}},
		{"verb-first idiom", store.Verb{Lemma: "tickle pink", Separable: true}, pronoun,
			[]string{"She tickles her pink.", "She tickled her pink."}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			seen := realized(t, func() *grammar.Node {
				return node("S", node("NP", leaf("Pronoun")), node("VP", leaf("Verb:transitive"), tc.object()))
			}, func() *fakeQuerier {
				q := newFake(store.Determiner{Lemma: "these", Number: "plural"})
				q.pronouns["nominative"] = store.Pronoun{Lemma: "she", Person: 3, Number: "singular", Gender: "fem"}
				q.framed = &tc.verb
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

func TestRealizeNeitherNorAgreesWithLastPart(t *testing.T) {
	singular := store.Determiner{Lemma: "this", Number: "singular"}
	plural := store.Determiner{Lemma: "these", Number: "plural"}
	build := func() *grammar.Node {
		return node("S",
			node("NP", leaf("Conjunction:neither"), detNoun(), leaf("Conjunction:nor"), detNoun()),
			node("VP", leaf("Verb")))
	}

	assertExactly(t, realized(t, build, func() *fakeQuerier { return newFake(singular, plural) }, 20),
		"Neither this goose nor these geese devour.", "Neither this goose nor these geese devoured.")
	assertExactly(t, realized(t, build, func() *fakeQuerier { return newFake(plural, singular) }, 20),
		"Neither these geese nor this goose devours.", "Neither these geese nor this goose devoured.")
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

func TestRealizeWhetherInfinitive(t *testing.T) {
	seen := realized(t, func() *grammar.Node {
		inf := node("InfVP", leaf("To"), node("VP", leaf("Verb:intransitive")))
		return node("S", detNoun(), node("VP", leaf("Verb:whether-infinitive"), leaf("Complementizer:whether"), inf))
	}, func() *fakeQuerier {
		return newFake(store.Determiner{Lemma: "this", Number: "singular"})
	}, 20)

	assertExactly(t, seen, "This goose gives whether to give.", "This goose gave whether to give.")
}

func TestRealizeGerundAfterAccusativeObject(t *testing.T) {
	seen := realized(t, func() *grammar.Node {
		ger := node("GerVP", node("VP", leaf("Verb:intransitive")))
		vp := node("VP", leaf("Verb:transitive-into-gerund"), node("NP", leaf("Pronoun")), leaf("Preposition:into"), ger)
		return node("S", detNoun(), vp)
	}, func() *fakeQuerier {
		return newFake(store.Determiner{Lemma: "this", Number: "singular"})
	}, 20)

	assertExactly(t, seen, "This goose gives her into giving.", "This goose gave her into giving.")
}

func TestRealizeClauseInsideGerundIsFinite(t *testing.T) {
	seen := realized(t, func() *grammar.Node {
		embedded := node("Clause", node("NP", leaf("Pronoun")), node("VP", leaf("Verb:intransitive")))
		says := node("VP", leaf("Verb:that-clause"), leaf("Complementizer"), embedded)
		return node("S", detNoun(), node("VP", leaf("Verb:gerund"), node("GerVP", says)))
	}, func() *fakeQuerier {
		return newFake(store.Determiner{Lemma: "this", Number: "singular"})
	}, 20)

	assertExactly(t, seen, "This goose gives giving that she gives.", "This goose gave giving that she gave.")
}

func TestGeneratePredicatesAdjectiveOfObject(t *testing.T) {
	var q *fakeQuerier
	seen := texts(t, `
	[[rule]]
	symbol = "S"
	expansion = ["NP", "VP"]

	[[rule]]
	symbol = "NP"
	expansion = ["Pronoun"]

	[[rule]]
	symbol = "VP"
	expansion = ["Verb:transitive-adjective", "NP", "Adjective"]
	`, func() *fakeQuerier { q = newFake(); return q }, 20)

	assertExactly(t, seen, "She gives her ugly.", "She gave her ugly.")
	if q.frame != "transitive-adjective" {
		t.Errorf("expected a verb lookup for frame transitive-adjective; got %q", q.frame)
	}
}

func TestGenerateWeatherVerbTakesDummyIt(t *testing.T) {
	var q *fakeQuerier
	seen := texts(t, `
	[[rule]]
	symbol = "S"
	expansion = ["Pronoun:it", "Verb:weather"]
	`, func() *fakeQuerier { q = newFake(); return q }, 20)

	assertExactly(t, seen, "It gives.", "It gave.")
	if q.frame != "weather" {
		t.Errorf("expected a verb lookup for frame weather; got %q", q.frame)
	}
}

func TestRealizeDummyThatClause(t *testing.T) {
	seen := realized(t, func() *grammar.Node {
		embedded := node("Clause", detNoun(), node("VP", leaf("Verb:intransitive")))
		return node("S", leaf("Pronoun:it"), leaf("Verb:dummy-that-clause"), leaf("Complementizer"), embedded)
	}, func() *fakeQuerier {
		return newFake(store.Determiner{Lemma: "these", Number: "plural"})
	}, 20)

	assertExactly(t, seen, "It gives that these geese give.", "It gave that these geese gave.")
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

func zipf(t *testing.T, v string) pgtype.Numeric {
	t.Helper()
	var n pgtype.Numeric
	if err := n.Scan(v); err != nil {
		t.Fatalf("Scan %q: %v", v, err)
	}
	return n
}

func TestRealizeRecordsFeatures(t *testing.T) {
	q := newFake(store.Determiner{Lemma: "these", Type: "demonstrative", Number: "plural"})
	q.noun = &store.Noun{Lemma: "goose", Inflections: []byte(`{"plural":"geese"}`), Frequency: zipf(t, "4.5")}
	q.framed = &store.Verb{Lemma: "look up", Frames: []byte(`["transitive","intransitive"]`), Separable: true, Frequency: zipf(t, "3.25")}
	tree := node("S", detNoun(), node("VP", leaf("Verb:transitive"), leaf("Pronoun:reflexive")))

	s, err := sentence.Realize(context.Background(), q, mustLoad(t, simpleGrammar), tree, loadVerbs(t), newRNG(), 2.5)
	if err != nil {
		t.Fatalf("Realize: %v", err)
	}
	tree = s.Tree

	tense := tree.Features.Tense
	if tense != "present" && tense != "past" {
		t.Errorf("expected root tense present or past; got %q", tense)
	}
	if c := tree.Features.Commonness; c == nil || *c != 2.5 {
		t.Errorf("expected root commonness 2.5; got %v", c)
	}
	np, vp := tree.Children[0], tree.Children[1]
	tests := []struct {
		name string
		got  grammar.Features
		want grammar.Features
	}{
		{"NP", np.Features, grammar.Features{Person: 3, Number: "plural"}},
		{"determiner", np.Children[0].Features, grammar.Features{Type: "demonstrative", Number: "plural"}},
		{"noun", np.Children[1].Features, grammar.Features{Number: "plural", Frequency: new(4.5)}},
		{"verb", vp.Children[0].Features, grammar.Features{
			Tense: tense, Form: "finite", Person: 3, Number: "plural",
			Frames: []string{"transitive", "intransitive"}, Separable: true, Frequency: new(3.25),
		}},
		{"reflexive", vp.Children[1].Features, grammar.Features{Case: "reflexive", Person: 3, Number: "plural", Gender: "epicene"}},
	}
	for _, tc := range tests {
		if !reflect.DeepEqual(tc.got, tc.want) {
			t.Errorf("%s: expected %+v; got %+v", tc.name, tc.want, tc.got)
		}
	}
}

func TestRealizeRecordsNonFiniteVerbForms(t *testing.T) {
	tests := []struct{ phrase, form string }{{"InfVP", "base"}, {"GerVP", "gerund"}}

	for _, tc := range tests {
		t.Run(tc.form, func(t *testing.T) {
			tree := node("S", detNoun(), node("VP", leaf("Verb:to-infinitive"), node(tc.phrase, node("VP", leaf("Verb")))))

			s, err := sentence.Realize(context.Background(), newFake(), mustLoad(t, simpleGrammar), tree, loadVerbs(t), newRNG(), 0)
			if err != nil {
				t.Fatalf("Realize: %v", err)
			}

			// No tense, person or number: a non-finite verb agrees with nothing.
			want := grammar.Features{Form: tc.form, Frames: []string{"transitive"}}
			if got := s.Tree.Children[1].Children[1].Children[0].Children[0].Features; !reflect.DeepEqual(got, want) {
				t.Errorf("expected %+v; got %+v", want, got)
			}
		})
	}
}

// passive is a VP of "be", a passive verb with the given frame and its
// complements, and an optional agent.
func passive(frame string, rest []*grammar.Node, agent ...*grammar.Node) *grammar.Node {
	pass := node("PassVP", append([]*grammar.Node{leaf("Verb:" + frame)}, rest...)...)
	return node("VP", append([]*grammar.Node{leaf("Be"), pass}, agent...)...)
}

func TestRealizePassiveBeAgreesWithSubject(t *testing.T) {
	this := store.Determiner{Lemma: "this", Number: "singular"}
	these := store.Determiner{Lemma: "these", Number: "plural"}
	i := store.Pronoun{Lemma: "I", Person: 1, Number: "singular", Gender: "epicene"}
	you := store.Pronoun{Lemma: "you", Person: 2, Number: "singular", Gender: "epicene"}
	she := store.Pronoun{Lemma: "she", Person: 3, Number: "singular", Gender: "fem"}
	pronoun := func() *grammar.Node { return node("NP", leaf("Pronoun")) }
	tests := []struct {
		name    string
		subject func() *grammar.Node
		det     store.Determiner
		noms    []store.Pronoun
		want    []string
	}{
		{"singular noun", detNoun, this, nil, []string{"This goose is given.", "This goose was given."}},
		{"plural noun", detNoun, these, nil, []string{"These geese are given.", "These geese were given."}},
		{"first person", pronoun, this, []store.Pronoun{i}, []string{"I am given.", "I was given."}},
		{"second person", pronoun, this, []store.Pronoun{you}, []string{"You are given.", "You were given."}},
		{"coordinated", func() *grammar.Node {
			return node("NP", pronoun(), leaf("Conjunction:np"), pronoun())
		}, this, []store.Pronoun{she, i}, []string{"She and I are given.", "She and I were given."}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			seen := realized(t, func() *grammar.Node {
				return node("S", tc.subject(), passive("transitive", nil))
			}, func() *fakeQuerier {
				q := newFake(tc.det)
				q.nominatives = tc.noms
				return q
			}, 20)

			assertExactly(t, seen, tc.want...)
		})
	}
}

func TestRealizePassiveInNonFinitePhrases(t *testing.T) {
	tests := []struct {
		name  string
		build func() *grammar.Node
		want  []string
	}{
		{"infinitive", func() *grammar.Node {
			inf := node("InfVP", leaf("To"), passive("transitive", nil))
			return node("S", detNoun(), node("VP", leaf("Verb:to-infinitive"), inf))
		}, []string{"These geese give to be given.", "These geese gave to be given."}},
		{"gerund", func() *grammar.Node {
			return node("S", detNoun(), node("VP", leaf("Verb:gerund"), node("GerVP", passive("transitive", nil))))
		}, []string{"These geese give being given.", "These geese gave being given."}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			seen := realized(t, tc.build, func() *fakeQuerier {
				return newFake(store.Determiner{Lemma: "these", Number: "plural"})
			}, 20)

			assertExactly(t, seen, tc.want...)
		})
	}
}

func TestRealizePassiveComplementsAndAgent(t *testing.T) {
	pronoun := func() *grammar.Node { return node("NP", leaf("Pronoun")) }
	tests := []struct {
		name  string
		build func() *grammar.Node
		want  []string
	}{
		{"agent pronoun is accusative", func() *grammar.Node {
			return node("S", pronoun(), passive("transitive", nil, node("Agent", leaf("Preposition:by"), pronoun())))
		}, []string{"She is given by her.", "She was given by her."}},
		{"agent reflexive agrees with the subject", func() *grammar.Node {
			return node("S", pronoun(), passive("transitive", nil, node("Agent", leaf("Preposition:by"), leaf("Pronoun:reflexive"))))
		}, []string{"She is given by herself.", "She was given by herself."}},
		{"reflexive object agrees with the subject", func() *grammar.Node {
			return node("S", pronoun(), passive("transitive-to", []*grammar.Node{leaf("Preposition:to"), leaf("Pronoun:reflexive")}))
		}, []string{"She is given to herself.", "She was given to herself."}},
		{"ditransitive keeps its object", func() *grammar.Node {
			return node("S", pronoun(), passive("ditransitive", []*grammar.Node{pronoun()}))
		}, []string{"She is given her.", "She was given her."}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			seen := realized(t, tc.build, func() *fakeQuerier {
				q := newFake()
				q.pronouns["nominative"] = store.Pronoun{Lemma: "she", Person: 3, Number: "singular", Gender: "fem"}
				return q
			}, 20)

			assertExactly(t, seen, tc.want...)
		})
	}
}

func TestRealizePassiveKeepsSeparableVerbWhole(t *testing.T) {
	seen := realized(t, func() *grammar.Node {
		return node("S", detNoun(), passive("transitive", nil))
	}, func() *fakeQuerier {
		q := newFake(store.Determiner{Lemma: "this", Number: "singular"})
		q.framed = &store.Verb{Lemma: "look up", Separable: true}
		return q
	}, 20)

	assertExactly(t, seen, "This goose is looked up.", "This goose was looked up.")
}

func TestRealizeRecordsPassiveVerbForms(t *testing.T) {
	tree := node("S", detNoun(), passive("transitive", nil))

	s, err := sentence.Realize(context.Background(), newFake(), mustLoad(t, simpleGrammar), tree, loadVerbs(t), newRNG(), 0)
	if err != nil {
		t.Fatalf("Realize: %v", err)
	}

	vp := s.Tree.Children[1]
	if got := vp.Children[0].Features.Form; got != "finite" {
		t.Errorf("expected be to be finite; got %q", got)
	}
	want := grammar.Features{Form: "participle", Frames: []string{"transitive"}}
	if got := vp.Children[1].Children[0].Features; !reflect.DeepEqual(got, want) {
		t.Errorf("expected %+v; got %+v", want, got)
	}
}

func TestRealizeRecordsPronounFeatures(t *testing.T) {
	q := newFake()
	q.pronouns["nominative"] = store.Pronoun{Lemma: "she", Person: 3, Number: "singular", Gender: "fem"}
	q.pronouns["accusative"] = store.Pronoun{Lemma: "us", Person: 1, Number: "plural", Gender: "epicene"}
	q.pronouns["genitive"] = store.Pronoun{Lemma: "mine", Person: 1, Number: "singular", Gender: "epicene"}
	tree := node("S", node("NP", leaf("Pronoun")), node("VP", leaf("Verb:ditransitive"), node("NP", leaf("Pronoun")), leaf("Pronoun:genitive")))

	s, err := sentence.Realize(context.Background(), q, mustLoad(t, simpleGrammar), tree, loadVerbs(t), newRNG(), 0)
	if err != nil {
		t.Fatalf("Realize: %v", err)
	}
	tree = s.Tree

	vp := tree.Children[1]
	genitive := vp.Children[2].Features
	tests := []struct {
		name string
		got  grammar.Features
		want grammar.Features
	}{
		{"subject", tree.Children[0].Children[0].Features, grammar.Features{Case: "nominative", Person: 3, Number: "singular", Gender: "fem"}},
		{"object", vp.Children[1].Children[0].Features, grammar.Features{Case: "accusative", Person: 1, Number: "plural", Gender: "epicene"}},
		// "Mine" stands for what is owned: third person, either number.
		{"genitive", genitive, grammar.Features{Case: "genitive", Person: 3, Number: genitive.Number}},
	}
	for _, tc := range tests {
		if !reflect.DeepEqual(tc.got, tc.want) {
			t.Errorf("%s: expected %+v; got %+v", tc.name, tc.want, tc.got)
		}
	}
	if genitive.Number != "singular" && genitive.Number != "plural" {
		t.Errorf("genitive: expected number singular or plural; got %q", genitive.Number)
	}
}

func TestRealizeGivesNoFeaturesToFixedWords(t *testing.T) {
	tree := node("S", leaf("Comma"), leaf("To"), leaf("Complementizer"), leaf("Preposition:with"), leaf("Conjunction:nor"))

	s, err := sentence.Realize(context.Background(), newFake(), mustLoad(t, simpleGrammar), tree, loadVerbs(t), newRNG(), 0)
	if err != nil {
		t.Fatalf("Realize: %v", err)
	}
	tree = s.Tree

	for _, c := range tree.Children {
		if !reflect.ValueOf(c.Features).IsZero() {
			t.Errorf("%s: expected no features; got %+v", c.Symbol, c.Features)
		}
	}
}

func TestGenerateRejectsMalformedVerbFrames(t *testing.T) {
	q := newFake()
	q.framed = &store.Verb{Lemma: "give", Frames: []byte(`{"transitive":true}`)}
	g := mustLoad(t, `
	[[rule]]
	symbol = "S"
	expansion = ["Verb:transitive"]
	`)

	_, err := sentence.Generate(context.Background(), q, g, loadVerbs(t), newRNG(), 0)

	if err == nil || !strings.Contains(err.Error(), `"give"`) {
		t.Errorf("expected an error naming give; got %v", err)
	}
}

func TestRealizeShowsSplitSeparableVerbOnItsLeaves(t *testing.T) {
	q := newFake()
	q.pronouns["nominative"] = store.Pronoun{Lemma: "she", Person: 3, Number: "singular", Gender: "fem"}
	q.framed = &store.Verb{Lemma: "look up", Separable: true}
	tree := node("S", node("NP", leaf("Pronoun")), node("VP", leaf("Verb:transitive"), node("NP", leaf("Pronoun"))))

	s, err := sentence.Realize(context.Background(), q, mustLoad(t, simpleGrammar), tree, loadVerbs(t), newRNG(), 0)
	if err != nil {
		t.Fatalf("Realize: %v", err)
	}
	tree = s.Tree

	subject := tree.Children[0].Children[0]
	verb, object := tree.Children[1].Children[0], tree.Children[1].Children[1].Children[0]
	head, _, _ := strings.Cut(verb.Word, " ")
	if verb.Display != head || object.Display != "her up" {
		t.Errorf("expected displays %q and \"her up\"; got %q and %q", head, verb.Display, object.Display)
	}
	if subject.Display != "" {
		t.Errorf("expected no display on an unsplit word; got %q", subject.Display)
	}
}

func TestRealizeExpandsHolesAndLeavesThePostedTreeAlone(t *testing.T) {
	g := mustLoad(t, simpleGrammar)
	tree := &grammar.Node{Symbol: "S"}

	s, err := sentence.Realize(context.Background(), newFake(), g, tree, loadVerbs(t), newRNG(), 0)
	if err != nil {
		t.Fatalf("Realize: %v", err)
	}

	if len(tree.Children) != 0 {
		t.Errorf("posted tree changed: %+v", tree)
	}
	if err := g.Check(s.Tree); err != nil || slices.ContainsFunc(s.Tree.LeafNodes(), func(n *grammar.Node) bool { return n.Word == "" }) {
		t.Errorf("realized tree has an unfilled leaf or fails Check (%v): %+v", err, s.Tree)
	}
}

func TestRealizeNamesTheLeafNoWordFits(t *testing.T) {
	g := mustLoad(t, emptyFrameGrammar)
	q := newFake()
	q.emptyFrames = []string{"transitive-on"}
	// "Pronoun Verb:transitive-on Pronoun Preposition:on Pronoun", with the
	// subject NP left a hole.
	tree := &grammar.Node{Symbol: "S", Children: []*grammar.Node{
		{Symbol: "NP"},
		{Symbol: "VP", Children: []*grammar.Node{
			{Symbol: "Verb:transitive-on"}, {Symbol: "Pronoun"}, {Symbol: "Preposition:on"}, {Symbol: "Pronoun"},
		}},
	}}

	_, err := sentence.Realize(context.Background(), q, g, tree, loadVerbs(t), newRNG(), 0)

	var leafErr *sentence.LeafError
	if !errors.As(err, &leafErr) || leafErr.Index != 1 || !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("expected a LeafError at 1 wrapping pgx.ErrNoRows; got %v", err)
	}
}

func TestRealizeReexpandsAHoleWithAnEmptyFrame(t *testing.T) {
	g := mustLoad(t, emptyFrameGrammar)
	for i := range 20 {
		q := newFake()
		q.emptyFrames = []string{"transitive-on"}
		tree := &grammar.Node{Symbol: "S", Children: []*grammar.Node{
			{Symbol: "NP", Children: []*grammar.Node{{Symbol: "Pronoun"}}},
			{Symbol: "VP"},
		}}

		s, err := sentence.Realize(context.Background(), q, g, tree, loadVerbs(t), rand.New(rand.NewPCG(uint64(i), 0)), 0)
		if err != nil {
			t.Fatalf("Realize: %v", err)
		}
		if s.Text != "She gives." && s.Text != "She gave." {
			t.Fatalf("text: got %q, want the intransitive VP", s.Text)
		}
	}
}

func TestTextUsesDisplayOverWord(t *testing.T) {
	tree := &grammar.Node{Symbol: "S", Children: []*grammar.Node{
		{Symbol: "Pronoun", Word: "she"},
		{Symbol: "Verb", Word: "looked up", Display: "looked"},
		{Symbol: "Pronoun", Word: "her", Display: "her up"},
		{Symbol: "Comma", Word: ","},
	}}

	if got := sentence.Text(tree); got != "She looked her up,." {
		t.Errorf("Text: got %q", got)
	}
}

func (f *fakeQuerier) lookup(lemma string) error {
	f.lookedUp = append(f.lookedUp, lemma)
	if lemma == "nope" {
		return pgx.ErrNoRows
	}
	return nil
}

func (f *fakeQuerier) LookupNoun(_ context.Context, lemma string) (store.Noun, error) {
	return store.Noun{Lemma: lemma, Inflections: []byte(`{}`), Plural: lemma == "Rastas"}, f.lookup(lemma)
}

func (f *fakeQuerier) GetRandomSingularNoun(_ context.Context, commonness float64) (store.Noun, error) {
	f.singularNouns++
	return store.Noun{Lemma: "goose", Inflections: []byte(`{"plural":"geese"}`)}, f.err
}

func (f *fakeQuerier) LookupVerb(_ context.Context, arg store.LookupVerbParams) (store.Verb, error) {
	return store.Verb{Lemma: arg.Lemma, Frames: []byte(`["transitive"]`)}, f.lookup(arg.Lemma)
}

func (f *fakeQuerier) LookupAdjective(_ context.Context, lemma string) (store.Adjective, error) {
	return store.Adjective{Lemma: lemma}, f.lookup(lemma)
}

func (f *fakeQuerier) LookupAdverb(_ context.Context, lemma string) (store.Adverb, error) {
	return store.Adverb{Lemma: lemma}, f.lookup(lemma)
}

func (f *fakeQuerier) LookupDeterminer(_ context.Context, lemma string) (store.Determiner, error) {
	number := "either"
	if lemma == "a" {
		number = "singular"
	}
	return store.Determiner{Lemma: lemma, Number: number}, f.lookup(lemma)
}

func (f *fakeQuerier) LookupPreposition(_ context.Context, lemma string) (store.Preposition, error) {
	return store.Preposition{Lemma: lemma}, f.lookup(lemma)
}

func (f *fakeQuerier) LookupPronoun(_ context.Context, arg store.LookupPronounParams) (store.Pronoun, error) {
	return store.Pronoun{Lemma: arg.Lemma, Person: 3, Number: "singular"}, f.lookup(arg.Lemma)
}

func (f *fakeQuerier) LookupConjunction(_ context.Context, arg store.LookupConjunctionParams) (store.Conjunction, error) {
	return store.Conjunction{Lemma: arg.Lemma}, f.lookup(arg.Lemma)
}

// lone is a tree of one leaf under S, and a grammar that derives it.
func lone(t *testing.T, leaf *grammar.Node) (*grammar.Grammar, *grammar.Node) {
	t.Helper()
	g := mustLoad(t, fmt.Sprintf("[[rule]]\nsymbol = \"S\"\nexpansion = [%q]\n", leaf.Symbol))
	return g, &grammar.Node{Symbol: "S", Children: []*grammar.Node{leaf}}
}

func TestRealizeLooksUpLockedLemmas(t *testing.T) {
	for _, slot := range []string{
		"Noun", "Verb:transitive", "Adjective", "Adverb", "Determiner", "Preposition",
		"Pronoun", "Pronoun:genitive", "Conjunction", "Conjunction:np", "Conjunction:coordinating",
	} {
		t.Run(slot, func(t *testing.T) {
			g, tree := lone(t, &grammar.Node{Symbol: slot, Lemma: "zany", Locked: true})
			q := newFake()

			s, err := sentence.Realize(context.Background(), q, g, tree, loadVerbs(t), newRNG(), 0)
			if err != nil {
				t.Fatalf("Realize: %v", err)
			}
			if leaf := s.Tree.Children[0]; leaf.Lemma != "zany" || !leaf.Locked {
				t.Errorf("leaf: got %+v, want the locked lemma zany", leaf)
			}
			if !slices.Equal(q.lookedUp, []string{"zany"}) || len(q.commonness) != 0 {
				t.Errorf("lookups %v and random draws %v: want one lookup and no draws", q.lookedUp, q.commonness)
			}
		})
	}
}

func TestRealizeIgnoresLocksOnFixedWords(t *testing.T) {
	for slot, want := range map[string]string{"Comma": ",", "Preposition:with": "with", "Conjunction:nor": "nor", "To": "to"} {
		t.Run(slot, func(t *testing.T) {
			g, tree := lone(t, &grammar.Node{Symbol: slot, Lemma: "zany", Locked: true})
			q := newFake()

			s, err := sentence.Realize(context.Background(), q, g, tree, loadVerbs(t), newRNG(), 0)
			if err != nil {
				t.Fatalf("Realize: %v", err)
			}
			if got := s.Tree.Children[0].Lemma; got != want || len(q.lookedUp) != 0 {
				t.Errorf("lemma: got %q after lookups %v, want %q and none", got, q.lookedUp, want)
			}
		})
	}
}

func TestRealizeRejectsALockedLemmaThatDoesNotFit(t *testing.T) {
	g, tree := lone(t, &grammar.Node{Symbol: "Noun", Lemma: "nope", Locked: true})

	_, err := sentence.Realize(context.Background(), newFake(), g, tree, loadVerbs(t), newRNG(), 0)

	var leafErr *sentence.LeafError
	if !errors.As(err, &leafErr) || leafErr.Index != 0 || !errors.Is(err, sentence.ErrLockMismatch) {
		t.Errorf("expected a LeafError at 0 wrapping ErrLockMismatch; got %v", err)
	}
}

const npGrammar = `
[[rule]]
symbol = "S"
expansion = ["NP"]

[[rule]]
symbol = "NP"
expansion = ["Determiner", "Noun"]
`

func TestRealizeKeepsPluralLemmasOffALockedSingularDeterminer(t *testing.T) {
	tree := &grammar.Node{Symbol: "S", Children: []*grammar.Node{{Symbol: "NP", Children: []*grammar.Node{
		{Symbol: "Determiner", Lemma: "a", Locked: true}, {Symbol: "Noun"},
	}}}}
	q := newFake()

	if _, err := sentence.Realize(context.Background(), q, mustLoad(t, npGrammar), tree, loadVerbs(t), newRNG(), 0); err != nil {
		t.Fatalf("Realize: %v", err)
	}
	if q.singularNouns != 1 {
		t.Errorf("singular noun draws: got %d, want 1", q.singularNouns)
	}
}

func TestRealizeRejectsALockedPluralLemmaAfterALockedSingularDeterminer(t *testing.T) {
	tree := &grammar.Node{Symbol: "S", Children: []*grammar.Node{{Symbol: "NP", Children: []*grammar.Node{
		{Symbol: "Determiner", Lemma: "a", Locked: true}, {Symbol: "Noun", Lemma: "Rastas", Locked: true},
	}}}}

	_, err := sentence.Realize(context.Background(), newFake(), mustLoad(t, npGrammar), tree, loadVerbs(t), newRNG(), 0)

	var leafErr *sentence.LeafError
	if !errors.As(err, &leafErr) || leafErr.Index != 0 || !errors.Is(err, sentence.ErrLockMismatch) {
		t.Errorf("expected a LeafError at the determiner wrapping ErrLockMismatch; got %v", err)
	}
}
