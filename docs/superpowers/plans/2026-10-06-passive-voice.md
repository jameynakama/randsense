# Passive Voice Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Every object frame gets a passive ("the goose was devoured by the philosophy"), shown in the diagram and builder with "be" as its own node.

**Architecture:** A new fixed terminal `Be` and two phrases, `PassVP` and `Agent`. Agreement inflects `Be` like a verb. Verbs under `PassVP` take a past participle from a new `morph.Verbs.PastParticiple`. `PassVP` and `Agent` pass the subject's agreement down, as `VP` and `PP` already do.

**Tech Stack:** Go (service), TOML grammar and morphology data, SvelteKit + Vitest (web).

**Spec:** `docs/superpowers/specs/2026-10-06-passive-voice-design.md`

## Global Constraints

- Frames constrain syntax only. Never add semantic restrictions on subjects or objects.
- Source data is never edited. `verb_morphology.toml` stays as shipped.
- No em dashes in prose or comments. American spelling. Oxford comma.
- No dates, names, or history in source comments.
- Commits: conventional prefix (`feat:`, `docs:`, `test:`). No `Co-Authored-By` trailer.
- Go tests run from `service/`: `go test ./internal/<pkg>/...`. Web tests run from `web/`: `npx vitest --run`.
- Weights: `VP → Be PassVP` 0.6, `VP → Be PassVP Agent` 0.5. Each `PassVP` rule has its active twin's weight. Each reflexive variant has a fifth of its noun phrase variant's weight. `Agent → Preposition:by NP` 1, `Agent → Preposition:by Pronoun:reflexive` 0.1.

## Review Focus

1. **An irregular verb entry without `past_participle`** would silently print an empty word. `LoadVerbs` should reject it, so a bad curated entry stops the server at startup (Task 1).
2. **"be" as a passive lemma** should give "been", not "bed" (Task 1).
3. **Separable verbs in a passive** should stay whole ("was looked up"), never split around a missing object (Task 3).
4. **A reflexive in a passive's fixed-preposition object** ("she was handed to herself") should agree with the subject, which requires `PassVP` to pass agreement down (Task 3).
5. **The project grammar** should load with every new slot labeled and expand without errors. The server refuses to start otherwise (Task 4, already covered by `TestProjectGrammarLoadsAndExpands`).

---

### Task 1: Past participles in `morph`

**Files:**
- Modify: `service/internal/morph/morph.go`
- Test: `service/internal/morph/morph_test.go`, `service/internal/sentence/sentence_test.go` (fixture only)

**Interfaces:**
- Produces: `func (v *Verbs) PastParticiple(lemma string) string`. `LoadVerbs` returns an error for an `[[irregular]]` entry with no `past_participle`.

- [ ] **Step 1: Write the failing tests**

Add after `TestParticiple` in `morph_test.go`:

```go
func TestPastParticiple(t *testing.T) {
	v := loadVerbs(t)
	tests := []struct {
		lemma string
		want  string
	}{
		{"walk", "walked"},
		{"bake", "baked"},
		{"carry", "carried"},
		{"stop", "stopped"},
		{"eat", "eaten"},
		{"panic", "panicked"},
		{"be", "been"},
		{"give up", "given up"},
		{"take care of", "taken care of"},
		{"talk turkey", "talked turkey"},
		{"stir fry", "stir fried"},
		{"spoon-feed", "spoon-fed"},
		{"double-check", "double-checked"},
	}

	for _, tc := range tests {
		t.Run(tc.lemma, func(t *testing.T) {
			if got := v.PastParticiple(tc.lemma); got != tc.want {
				t.Errorf("expected %q; got %q", tc.want, got)
			}
		})
	}
}

func TestLoadVerbsRejectsIrregularWithoutPastParticiple(t *testing.T) {
	_, err := morph.LoadVerbs(strings.NewReader(`
	[[irregular]]
	base = "eat"
	third = "eats"
	past = "ate"
	present_participle = "eating"
	`))
	if err == nil || !strings.Contains(err.Error(), `"eat"`) {
		t.Errorf("expected an error naming eat; got %v", err)
	}
}
```

`take` is not in `testVerbs`, so "taken care of" needs it. Add this entry to the `testVerbs` constant:

```toml
[[irregular]]
base = "take"
third = "takes"
past = "took"
past_participle = "taken"
present_participle = "taking"
```

In `TestLoadVerbsMergesFiles`, after the `Participle("wine and dine")` check, add:

```go
	if got := v.PastParticiple("wine and dine"); got != "wined and dined" {
		t.Errorf("expected wined and dined; got %q", got)
	}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd service && go test ./internal/morph/...`
Expected: FAIL to compile with `v.PastParticiple undefined`.

- [ ] **Step 3: Implement**

In `morph.go`, add the field to `irregular`:

```go
type irregular struct {
	Base              string `toml:"base"`
	Third             string `toml:"third"`
	Past              string `toml:"past"`
	PastParticiple    string `toml:"past_participle"`
	PresentParticiple string `toml:"present_participle"`
}
```

In `LoadVerbs`, replace the irregular loop:

```go
		for _, irr := range f.Irregular {
			if irr.PastParticiple == "" {
				return nil, fmt.Errorf("morph: irregular %q has no past_participle", irr.Base)
			}
			v.irregular[irr.Base] = irr
		}
```

Add after `Participle`:

```go
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
```

Add after `participleWord`:

```go
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
```

At the end of `conjugateWord`, replace the four regular-past cases with the helper so the switch ends:

```go
	case ok:
		return irr.Past
	default:
		return v.regularPast(w)
	}
```

- [ ] **Step 4: Fix the sentence tests' fixture**

`LoadVerbs` now rejects an irregular without a past participle, so the `give` entry in `service/internal/sentence/sentence_test.go`'s `loadVerbs` needs one:

```go
	v, err := morph.LoadVerbs(strings.NewReader(`
	[[irregular]]
	base = "give"
	third = "gives"
	past = "gave"
	past_participle = "given"
	present_participle = "giving"
	`))
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `cd service && go test ./...`
Expected: PASS, including `TestProjectVerbMorphologyLoads`. That shows every shipped irregular has a `past_participle`.

- [ ] **Step 6: Commit**

```bash
git add service/internal/morph service/internal/sentence/sentence_test.go
git commit -m "feat: Add past participles to verb morphology"
```

---

### Task 2: `Be` terminal and "by" preposition in the grammar package

**Files:**
- Modify: `service/internal/grammar/grammar.go:33-40,124`
- Test: `service/internal/grammar/grammar_test.go`

**Interfaces:**
- Produces: `grammar.Be POS = "Be"`. `"Preposition:by"` is a valid slot.

- [ ] **Step 1: Write the failing test**

Add after `TestLoadAcceptsAdjectiveComplements`:

```go
func TestLoadAcceptsPassive(t *testing.T) {
	mustLoad(t, `
	[[rule]]
	symbol = "S"
	expansion = ["Noun", "Be", "Verb:transitive", "Preposition:by", "Noun"]
	`)
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `cd service && go test ./internal/grammar/ -run TestLoadAcceptsPassive`
Expected: FAIL. `Load` rejects `Be` as a symbol with no rules, or rejects the `by` qualifier.

- [ ] **Step 3: Implement**

In `grammar.go`, after the `To` constant:

```go
	// To marks an infinitive and is always "to".
	To POS = "To"
	// Be is the passive auxiliary and is always "be".
	Be POS = "Be"
)

var allPOS = []POS{Noun, Verb, Adjective, Adverb, Determiner, Preposition, Pronoun, Conjunction, Comma, Complementizer, To, Be}
```

In `qualifiers`:

```go
	Preposition:    {"by", "from", "into", "of", "on", "to", "with"},
```

- [ ] **Step 4: Run the package tests**

Run: `cd service && go test ./internal/grammar/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add service/internal/grammar
git commit -m "feat: Accept Be and Preposition:by in grammars"
```

---

### Task 3: Passive agreement in the generator

**Files:**
- Modify: `service/internal/sentence/sentence.go`
- Test: `service/internal/sentence/sentence_test.go`

**Interfaces:**
- Consumes: `(*morph.Verbs).PastParticiple(lemma string) string` (Task 1), `grammar.Be` (Task 2).
- Produces: phrase symbols `PassVP` and `Agent` with the behavior below. The verb feature `Form: "participle"`.

The sentence tests' fake answers every framed verb with "give", and Task 1 gave its fixture `past_participle = "given"`.

- [ ] **Step 1: Write the failing tests**

Add after `TestRealizeRecordsNonFiniteVerbForms`:

```go
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
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd service && go test ./internal/sentence/ -run 'Passive'`
Expected: FAIL. "be" stays uninflected or empty, and the verbs come out finite ("gives").

- [ ] **Step 3: Implement**

In `sentence.go`, replace the agreement comment and constants:

```go
// Agreement relies on these grammar symbols. An NP followed by a VP among its
// siblings is that VP's subject: its pronouns are nominative and the VP's
// verbs and reflexives agree with it, including reflexives in a PP, a PassVP
// or an Agent. An NP's person and number come from its pronoun, its
// determiner, or, for "NP Conjunction NP", the coordination. A Be leaf
// inflects like a verb. Verbs under an InfVP stay in their base form, verbs
// under a GerVP take -ing and verbs under a PassVP take the past participle,
// up to any clause nested inside them.
const (
	nounPhrase       = "NP"
	verbPhrase       = "VP"
	prepPhrase       = "PP"
	infinitivePhrase = "InfVP"
	gerundPhrase     = "GerVP"
	passivePhrase    = "PassVP"
	agentPhrase      = "Agent"
)

// verbForm is how agreeWithSubjects inflects the verbs in a phrase.
type verbForm int

const (
	finite verbForm = iota
	base
	gerund
	participle
)
```

In `chooseWord`, after the `grammar.To` case:

```go
	case grammar.Be:
		return "be", leafInfo{}, nil
```

In `agreeWithSubjects`, change the verb condition and add the participle case:

```go
		if len(c.Children) == 0 && (c.POS() == grammar.Verb || c.POS() == grammar.Be) {
			switch form {
			case finite:
				c.Word = gen.verbs.Conjugate(c.Lemma, gen.tense, agr.person, agr.number)
				c.Features.Form, c.Features.Tense = "finite", string(gen.tense)
				c.Features.Person, c.Features.Number = int(agr.person), string(agr.number)
			case base:
				c.Features.Form = "base"
			case gerund:
				c.Word = gen.verbs.Participle(c.Lemma)
				c.Features.Form = "gerund"
			case participle:
				c.Word = gen.verbs.PastParticiple(c.Lemma)
				c.Features.Form = "participle"
			}
		}
		var err error
		switch c.Symbol {
		case verbPhrase, prepPhrase, agentPhrase:
			err = gen.agreeWithSubjects(c, agr, form)
		case passivePhrase:
			err = gen.agreeWithSubjects(c, agr, participle)
		case infinitivePhrase:
			err = gen.agreeWithSubjects(c, agr, base)
		case gerundPhrase:
			err = gen.agreeWithSubjects(c, agr, gerund)
		default:
			err = gen.agreeWithSubjects(c, thirdSingular, finite)
		}
```

Update the `agreeWithSubjects` doc comment's last sentence to: `Verbs in an infinitive keep their base form, verbs in a gerund take -ing and verbs in a passive take the past participle; "be" inflects like any verb.`

- [ ] **Step 4: Run the package tests**

Run: `cd service && go test ./internal/sentence/...`
Expected: PASS, the existing tests included.

- [ ] **Step 5: Commit**

```bash
git add service/internal/sentence
git commit -m "feat: Inflect passives in the generator"
```

---

### Task 4: Passive rules and labels in `grammar.toml`

**Files:**
- Modify: `service/data/grammar/grammar.toml`
- Modify: `README.md:152`
- Test: `service/internal/grammar/grammar_test.go` (`TestProjectGrammarLoadsAndExpands`, existing)

**Interfaces:**
- Consumes: `Be` and `Preposition:by` (Task 2). The `PassVP`/`Agent` behavior (Task 3).
- Produces: the served grammar. `/api/v1/grammar` picks it up with no code change.

- [ ] **Step 1: Update the header comment**

Replace lines 3-4's terminal list so that it reads `... (Noun, Verb, Determiner, ..., Comma for punctuation, and Complementizer, To and Be, which are "that", "to" and "be"); any other symbol needs rules.` 
Replace the agreement paragraph (lines 17-22) with:

```toml
# Agreement depends on seven symbol names, NP, VP, PP, InfVP, GerVP, PassVP
# and Agent: an NP followed by a VP among its siblings is the subject
# (nominative pronouns, and the VP's verbs and reflexives, including those in
# a PP, a PassVP or an Agent, agree with it). An NP's person and number come
# from its Pronoun, its Determiner, or "NP Conjunction NP" coordination. Be
# inflects like a verb. Verbs under an InfVP keep their base form, verbs under
# a GerVP take -ing and verbs under a PassVP take the past participle, up to
# any clause nested inside them.
```

- [ ] **Step 2: Add the rules**

Insert after the `VP → Verb:transitive-adjective NP Adjective` rule, before `VP → VP Adverb`:

```toml
[[rule]]
symbol = "VP"
expansion = ["Be", "PassVP"]
weight = 0.6

[[rule]]
symbol = "VP"
expansion = ["Be", "PassVP", "Agent"]
weight = 0.5
```

Insert after the `GerVP` rule:

```toml
[[rule]]
symbol = "PassVP"
expansion = ["Verb:transitive"]
weight = 4

[[rule]]
symbol = "PassVP"
expansion = ["Verb:transitive-pp", "PP"]

[[rule]]
symbol = "PassVP"
expansion = ["Verb:ditransitive", "NP"]
weight = 0.5

[[rule]]
symbol = "PassVP"
expansion = ["Verb:transitive-from", "Preposition:from", "NP"]
weight = 0.25

[[rule]]
symbol = "PassVP"
expansion = ["Verb:transitive-from", "Preposition:from", "Pronoun:reflexive"]
weight = 0.05

[[rule]]
symbol = "PassVP"
expansion = ["Verb:transitive-of", "Preposition:of", "NP"]
weight = 0.25

[[rule]]
symbol = "PassVP"
expansion = ["Verb:transitive-of", "Preposition:of", "Pronoun:reflexive"]
weight = 0.05

[[rule]]
symbol = "PassVP"
expansion = ["Verb:transitive-on", "Preposition:on", "NP"]
weight = 0.25

[[rule]]
symbol = "PassVP"
expansion = ["Verb:transitive-on", "Preposition:on", "Pronoun:reflexive"]
weight = 0.05

[[rule]]
symbol = "PassVP"
expansion = ["Verb:transitive-to", "Preposition:to", "NP"]
weight = 0.25

[[rule]]
symbol = "PassVP"
expansion = ["Verb:transitive-to", "Preposition:to", "Pronoun:reflexive"]
weight = 0.05

[[rule]]
symbol = "PassVP"
expansion = ["Verb:transitive-with", "Preposition:with", "NP"]
weight = 0.25

[[rule]]
symbol = "PassVP"
expansion = ["Verb:transitive-with", "Preposition:with", "Pronoun:reflexive"]
weight = 0.05

[[rule]]
symbol = "PassVP"
expansion = ["Verb:transitive-to-infinitive", "InfVP"]
weight = 0.25

[[rule]]
symbol = "PassVP"
expansion = ["Verb:transitive-adjective", "Adjective"]
weight = 0.2

[[rule]]
symbol = "PassVP"
expansion = ["Verb:transitive-into-gerund", "Preposition:into", "GerVP"]
weight = 0.15

[[rule]]
symbol = "Agent"
expansion = ["Preposition:by", "NP"]

[[rule]]
symbol = "Agent"
expansion = ["Preposition:by", "Pronoun:reflexive"]
weight = 0.1
```

- [ ] **Step 3: Add the labels**

After `[phrase.GerVP]`:

```toml
[phrase.PassVP]
label = "passive verb phrase"
description = "A verb in its past participle form and whatever else it takes. Its object has become the subject."

[phrase.Agent]
label = "agent"
description = "\"By\" and whoever or whatever does the action in a passive."
```

After `[slot."Preposition:into"]`:

```toml
[slot."Preposition:by"]
label = "\"by\""
description = "Introduces the agent of a passive."
```

After `[slot.To]`:

```toml
[slot.Be]
label = "\"be\""
description = "Helps a passive verb and agrees with the subject: is, was, were."
```

- [ ] **Step 4: Run the project grammar test and the whole suite**

Run: `cd service && go test ./...`
Expected: PASS. `TestProjectGrammarLoadsAndExpands` fails if any new phrase or slot lacks a label, or if `Load` rejects a rule.

- [ ] **Step 5: Smoke-test generation**

Run: `just run-be`. In another shell, request sentences until a passive appears:

```bash
for i in $(seq 40); do curl -s localhost:8080/api/v1/sentences/random; done | jq -r 'select(.tree | tostring | test("PassVP")) | .text'
```

Expected: a few passives such as "The goose was devoured by the philosophy." Read them for wrong forms ("was gived", "is gave"). Stop the server.

- [ ] **Step 6: Update the README**

In `README.md:152`, change `` `form` (`finite`, `base` or `gerund`) `` to `` `form` (`finite`, `base`, `gerund` or `participle`) ``. Also update `grammar.go`'s `Features.Form` comment to `// Form is a verb's: "finite", "base", "gerund" or "participle".`

- [ ] **Step 7: Commit**

```bash
git add service/data/grammar/grammar.toml service/internal/grammar/grammar.go README.md
git commit -m "feat: Generate passives of every object frame"
```

---

### Task 5: Frontend roles and labels

**Files:**
- Modify: `web/src/lib/tree.ts:52-70,76`
- Modify: `web/src/lib/types.ts:6`
- Test: `web/src/lib/tree.spec.ts`

**Interfaces:**
- Consumes: tree shape `VP → Be PassVP Agent`. The feature `form: 'participle'`.

- [ ] **Step 1: Write the failing tests**

In `tree.spec.ts`, inside `describe('role', ...)` after the existing `it.each` blocks:

```ts
	// "the goose was given her by them"
	const passive = node(
		'S',
		node('NP', leaf('Determiner', 'the'), leaf('Noun', 'goose')),
		node(
			'VP',
			leaf('Be', 'was'),
			node('PassVP', leaf('Verb:ditransitive', 'given'), node('NP', leaf('Pronoun', 'her'))),
			node('Agent', leaf('Preposition:by', 'by'), node('NP', leaf('Pronoun', 'them')))
		)
	);
	const passiveRoleOf = (word: string) => {
		const l = leaves(passive).find((l) => l.node.word === word)!;
		return role(passive, l.path);
	};

	it.each([
		['goose', 'in the subject'],
		['her', 'in the object'],
		['by', 'in the agent'],
		['them', 'in the agent']
	])('in a passive, %s is %s', (word, want) => {
		expect(passiveRoleOf(word)).toBe(want);
	});

	it.each(['was', 'given'])('in a passive, %s has no role', (word) => {
		expect(passiveRoleOf(word)).toBeNull();
	});
```

In `describe('featureLabels', ...)`:

```ts
	it('labels a past participle', () => {
		expect(featureLabels({ form: 'participle' })).toEqual(['past participle']);
	});
```

- [ ] **Step 2: Run them to verify they fail**

Run: `cd web && npx vitest --run src/lib/tree.spec.ts`
Expected: FAIL. "her" has no role, "by"/"them" have none, the label is `undefined`, and `npm run check` rejects `'participle'`.

- [ ] **Step 3: Implement**

In `types.ts`:

```ts
	form?: 'finite' | 'base' | 'gerund' | 'participle';
```

In `tree.ts`, `role()`. Update the comment and the switch, and widen the object check:

```ts
// role is where the leaf at path sits in its clause, from the nearest
// phrase that says: a prepositional phrase, an infinitive, a gerund phrase,
// a passive's agent, or the subject or object. null when none applies.
```

```ts
			case 'GerVP':
				return 'in a gerund phrase';
			case 'Agent':
				return 'in the agent';
		}
```

```ts
		if (
			['VP', 'PassVP'].includes(parent.symbol) &&
			siblings.slice(0, at).some((s) => pos(s) === 'Verb')
		) {
			return 'in the object';
		}
```

```ts
const forms: Record<string, string> = {
	finite: 'finite',
	base: 'base form',
	gerund: '-ing form',
	participle: 'past participle'
};
```

- [ ] **Step 4: Run the web checks**

Run: `cd web && npx vitest --run && npm run check && npm run lint`
Expected: PASS. Run `npx prettier --write src/lib` first if lint flags formatting.

- [ ] **Step 5: Commit**

```bash
git add web/src/lib
git commit -m "feat: Label passive roles and past participles in the diagram"
```

---

### Task 6: Project records

**Files:**
- Modify: `CLAUDE.md` (Roadmap items 3 and 6, Known quirks)

- [ ] **Step 1: Update `CLAUDE.md`**

In Roadmap item 6 ("More sentence types"), replace the passive bullet with:

```markdown
   - progressive and perfect ("was devouring", "has devoured"), reusing `Be` and the past
     participle
```

At the end of Roadmap item 3 ("Idioms with a broken object frame"), add: `Passives show the same break ("the goose was given birth").`

Under Known quirks, add:

```markdown
- **Adverbs before "be":** `VP → Adverb VP` gives "the goose quickly was devoured". A fix would
  place adverbs between `Be` and `PassVP`.
```

- [ ] **Step 2: Run the full suite**

Run: `just test`
Expected: PASS (Go, web unit, component, and e2e).

- [ ] **Step 3: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: Record passive voice follow-ups and quirks"
```

- [ ] **Step 4: Wrap-up sweep**

Invoke `/wrap` so the records match the shipped branch.
