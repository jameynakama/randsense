# Diagram Plan A: grammar endpoint and drawn diagram

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Serve the labeled grammar, check `realize` trees against it, and draw every generated
sentence as a constituency tree in place of the structure outline.

**Architecture:** Labels live beside the rules in `grammar.toml`. The Go `grammar` package
gains `Labeled`, `Describe` and `Check`. A new `GET /api/v1/grammar` serves `Describe()`, and
`realize` calls `Check` instead of `Node.Validate`. SvelteKit loads the grammar once in a root
layout load. A pure layout function in `web/src/lib/diagram.ts` places nodes, and
`Diagram.svelte` renders them as absolutely positioned HTML over one SVG layer of lines.

**Tech Stack:** Go 1.26 (chi, BurntSushi/toml, pgx), SvelteKit with Svelte 5 runes, TypeScript,
Vitest (browser and node projects), Playwright with axe.

**Spec:** `docs/superpowers/specs/2026-10-05-diagram-builder-design.md` (build-order stages 1
and 2). Plan B (holes, locks, builder, Keep, Remix) is written after this plan ships, against
the code this plan leaves.

## Global Constraints

- Go tests: table-driven where there are cases. API tests run against real Postgres via `just test-be`, which needs the root `.env` loaded (the Justfile does it).
- Web: TypeScript, Svelte 5 runes, prettier formatting (`npx prettier --write` on touched files), `just test-fe` runs check, lint, vitest and Playwright.
- Contrast: `dodgerblue` (`--word`) is for text at 24px and up only. Diagram text uses `--text`, `--secondary` and `--word-selected`.
- Interactive targets are at least 44 by 44 CSS px. The read-only diagram has no interactive nodes. Its only control is the Zoom button, which is a `.button`.
- Axe runs on every page and key open state with no violations allowed.
- Commit messages: conventional prefix (`feat:`, `test:`, `docs:`, `refactor:`), sentence case after it, and no `Co-Authored-By` trailer.
- Comments and docs: no dates, names or history. American English.

## Review Focus

1. A long coordinated sentence on a 320px phone, with the diagram open, must not scroll the page sideways. The diagram scales to fit and offers Zoom. Owned by Task 7 (e2e).
2. A separable verb ("looked ... her up") must show its `display` text, not its `word`, in the diagram. Owned by Task 5.
3. A comma leaf is a real leaf. It gets a column and a short label, and never collapses to zero width. Owned by Task 5.
4. Generating a new sentence while the diagram is open redraws the diagram for the new tree. Owned by Task 7.
5. Posting a part of speech with children, or an unknown qualifier, to `realize` is a 400, never a 500. Owned by Task 2.

---

### Task 1: Grammar labels, `Labeled` and `Describe`

**Files:**
- Modify: `service/internal/grammar/grammar.go` (types near `fileRule`/`file`, `Load`, `validate`; new `Label`, `Phrase`, `Description`, `slotSymbols`, `Labeled`, `Describe`)
- Modify: `service/data/grammar/grammar.toml` (append label tables; extend the header comment)
- Modify: `service/cmd/server/main.go:83-87`
- Test: `service/internal/grammar/grammar_test.go`

**Interfaces:**
- Produces:
  - `type Label struct { Label, Description, Example string }` (toml tags `label`, `description`, `example`; json the same, `example` omitempty)
  - `type Phrase struct { Label; Rules [][]string }` (json `rules`)
  - `type Description struct { Start string; Phrases map[string]Phrase; Slots map[string]Label }` (json `start`, `phrases`, `slots`)
  - `func (g *Grammar) Labeled() error`
  - `func (g *Grammar) Describe() Description`

- [ ] **Step 1: Write the failing tests**

Add to `grammar_test.go`:

```go
const labeled = `
[[rule]]
symbol = "S"
expansion = ["NP", "Verb:transitive", "NP"]

[[rule]]
symbol = "NP"
expansion = ["Determiner", "Noun"]

[[rule]]
symbol = "NP"
expansion = ["Pronoun"]

[phrase.S]
label = "sentence"
description = "A complete thought."

[phrase.NP]
label = "noun phrase"
description = "Names a thing."

[slot.Determiner]
label = "determiner"
description = "Points at a noun."

[slot.Noun]
label = "noun"
description = "A thing."

[slot.Pronoun]
label = "pronoun"
description = "Stands in for a noun phrase."

[slot."Verb:transitive"]
label = "transitive verb"
description = "Takes an object."
example = "devoured the goose"
`

func TestDescribeServesRulesInFileOrderWithLabels(t *testing.T) {
	g := mustLoad(t, labeled)

	got := g.Describe()

	want := grammar.Description{
		Start: "S",
		Phrases: map[string]grammar.Phrase{
			"S": {
				Label: grammar.Label{Label: "sentence", Description: "A complete thought."},
				Rules: [][]string{{"NP", "Verb:transitive", "NP"}},
			},
			"NP": {
				Label: grammar.Label{Label: "noun phrase", Description: "Names a thing."},
				Rules: [][]string{{"Determiner", "Noun"}, {"Pronoun"}},
			},
		},
		Slots: map[string]grammar.Label{
			"Determiner":      {Label: "determiner", Description: "Points at a noun."},
			"Noun":            {Label: "noun", Description: "A thing."},
			"Pronoun":         {Label: "pronoun", Description: "Stands in for a noun phrase."},
			"Verb:transitive": {Label: "transitive verb", Description: "Takes an object.", Example: "devoured the goose"},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Describe:\n got %+v\nwant %+v", got, want)
	}
}

func TestDescriptionOmitsAbsentExample(t *testing.T) {
	b, err := json.Marshal(grammar.Label{Label: "noun", Description: "A thing."})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	if want := `{"label":"noun","description":"A thing."}`; string(b) != want {
		t.Errorf("expected %s; got %s", want, b)
	}
}

func TestLabeledAcceptsAFullyLabeledGrammar(t *testing.T) {
	if err := mustLoad(t, labeled).Labeled(); err != nil {
		t.Errorf("expected no error; got %v", err)
	}
}

func TestLabeledReportsWhatIsMissing(t *testing.T) {
	tests := []struct {
		name, drop, want string
	}{
		{"phrase", "[phrase.NP]\nlabel = \"noun phrase\"\ndescription = \"Names a thing.\"\n", `phrase "NP"`},
		{"slot", "[slot.Noun]\nlabel = \"noun\"\ndescription = \"A thing.\"\n", `slot "Noun"`},
		{"description", "description = \"A thing.\"\n", `slot "Noun"`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := strings.Replace(labeled, tc.drop, "", 1)
			if in == labeled {
				t.Fatalf("test grammar doesn't contain %q", tc.drop)
			}
			err := mustLoad(t, in).Labeled()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("expected error mentioning %s; got %v", tc.want, err)
			}
		})
	}
}

func TestLoadRejectsLabelsForUnusedSymbols(t *testing.T) {
	tests := []struct {
		name, extra, want string
	}{
		{"phrase without rules", "[phrase.VP]\nlabel = \"verb phrase\"\ndescription = \"x\"\n", `"VP"`},
		{"slot no rule uses", "[slot.Adverb]\nlabel = \"adverb\"\ndescription = \"x\"\n", `"Adverb"`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := grammar.Load(strings.NewReader(labeled + "\n" + tc.extra))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("expected error mentioning %s; got %v", tc.want, err)
			}
		})
	}
}
```

The "description" case relies on `Noun`'s description line being the first `description = "A thing."` in `labeled`, which it is. That leaves `Noun` with a label but no description.

Then extend `TestProjectGrammarLoadsAndExpands`: right after the `Load` error check, add

```go
	if err := g.Labeled(); err != nil {
		t.Fatalf("Labeled: %v", err)
	}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd service && go test ./internal/grammar/`
Expected: compile failure: `grammar.Description`, `grammar.Phrase`, `grammar.Label`, `Labeled` and `Describe` undefined.

- [ ] **Step 3: Implement in `grammar.go`**

Replace the `Grammar` and `file` types:

```go
// Grammar maps each non-terminal symbol to its rules, and symbols to the
// labels people read.
type Grammar struct {
	rules   map[string][]rule
	phrases map[string]Label
	slots   map[string]Label
}

// Label is how a symbol reads to people: "noun phrase", what it does, and
// for a verb frame an example.
type Label struct {
	Label       string `toml:"label" json:"label"`
	Description string `toml:"description" json:"description"`
	Example     string `toml:"example" json:"example,omitempty"`
}

type file struct {
	Rules   []fileRule       `toml:"rule"`
	Phrases map[string]Label `toml:"phrase"`
	Slots   map[string]Label `toml:"slot"`
}
```

In `Load`, change `g := &Grammar{rules: rules}` to:

```go
	g := &Grammar{rules: rules, phrases: f.Phrases, slots: f.Slots}
```

At the end of `validate`, before `return nil`, add:

```go
	used := g.slotSymbols()
	for _, sym := range slices.Sorted(maps.Keys(g.phrases)) {
		if _, ok := g.rules[sym]; !ok {
			return fmt.Errorf("grammar: phrase label for %q, which has no rules", sym)
		}
	}
	for _, sym := range slices.Sorted(maps.Keys(g.slots)) {
		if !used[sym] {
			return fmt.Errorf("grammar: slot label for %q, which no rule uses", sym)
		}
	}
```

Add after `productiveSymbols`:

```go
// slotSymbols is every part-of-speech symbol a rule uses, qualified ones
// included.
func (g *Grammar) slotSymbols() map[string]bool {
	used := map[string]bool{}
	for _, rules := range g.rules {
		for _, r := range rules {
			for _, s := range r.Expansion {
				if _, phrase := g.rules[s]; !phrase {
					used[s] = true
				}
			}
		}
	}
	return used
}

// Labeled reports the first phrase, then the first slot a rule uses, that
// has no label or no description. Load accepts unlabeled grammars so tests
// can stay small; the server requires labels.
func (g *Grammar) Labeled() error {
	missing := func(l Label) bool { return l.Label == "" || l.Description == "" }
	for _, sym := range slices.Sorted(maps.Keys(g.rules)) {
		if missing(g.phrases[sym]) {
			return fmt.Errorf("grammar: phrase %q has no label and description", sym)
		}
	}
	for _, sym := range slices.Sorted(maps.Keys(g.slotSymbols())) {
		if missing(g.slots[sym]) {
			return fmt.Errorf("grammar: slot %q has no label and description", sym)
		}
	}
	return nil
}

// Phrase is a non-terminal's label and its rules' expansions, in file order.
type Phrase struct {
	Label
	Rules [][]string `json:"rules"`
}

// Description is the grammar as the frontend reads it: what to start from,
// what each phrase can expand to, and how every symbol reads.
type Description struct {
	Start   string            `json:"start"`
	Phrases map[string]Phrase `json:"phrases"`
	Slots   map[string]Label  `json:"slots"`
}

func (g *Grammar) Describe() Description {
	d := Description{Start: start, Phrases: map[string]Phrase{}, Slots: map[string]Label{}}
	for sym, rules := range g.rules {
		p := Phrase{Label: g.phrases[sym]}
		for _, r := range rules {
			p.Rules = append(p.Rules, r.Expansion)
		}
		d.Phrases[sym] = p
	}
	for sym := range g.slotSymbols() {
		d.Slots[sym] = g.slots[sym]
	}
	return d
}
```

- [ ] **Step 4: Run the package tests (the project grammar test still fails)**

Run: `cd service && go test ./internal/grammar/`
Expected: every new test passes. `TestProjectGrammarLoadsAndExpands` fails with `phrase "ADJ" has no label and description`.

- [ ] **Step 5: Label the project grammar**

Add this paragraph to the end of the header comment in `service/data/grammar/grammar.toml`:

```toml
#
# Every phrase needs a [phrase.X] table and every slot a rule uses a
# [slot."X"] table, each with a label and a description; verb frames also
# get an example. The frontend shows them, and the server won't start
# without them.
```

Append to the end of the file:

```toml
[phrase.S]
label = "sentence"
description = "A complete thought: one clause, or clauses joined by a conjunction."

[phrase.Clause]
label = "clause"
description = "A subject and what it does: a noun phrase and a verb phrase."

[phrase.NP]
label = "noun phrase"
description = "Names a person, place or thing, with any words that point at it or describe it."

[phrase.VP]
label = "verb phrase"
description = "A verb and whatever it takes: objects, phrases or whole clauses."

[phrase.InfVP]
label = "infinitive"
description = "\"To\" and a verb phrase in its base form."

[phrase.GerVP]
label = "gerund phrase"
description = "A verb phrase in its -ing form, used like a noun."

[phrase.PP]
label = "prepositional phrase"
description = "A preposition and its object."

[phrase.ADJ]
label = "adjective phrase"
description = "One or more adjectives describing a noun."

[slot.Noun]
label = "noun"
description = "A person, place, thing or idea."

[slot.Determiner]
label = "determiner"
description = "Points at a noun: the, a, this, every."

[slot.Adjective]
label = "adjective"
description = "Describes a noun."

[slot.Adverb]
label = "adverb"
description = "Describes a verb: how, when or where."

[slot.Preposition]
label = "preposition"
description = "Relates a noun phrase to the rest of the sentence: in, under, despite."

[slot."Preposition:on"]
label = "\"on\""
description = "The preposition its verb asks for."

[slot."Preposition:to"]
label = "\"to\""
description = "The preposition its verb asks for."

[slot."Preposition:from"]
label = "\"from\""
description = "The preposition its verb asks for."

[slot."Preposition:of"]
label = "\"of\""
description = "The preposition its verb asks for."

[slot."Preposition:with"]
label = "\"with\""
description = "The preposition its verb asks for."

[slot."Preposition:into"]
label = "\"into\""
description = "The preposition its verb asks for."

[slot.Pronoun]
label = "pronoun"
description = "Stands in for a noun phrase: she, them, you."

[slot."Pronoun:it"]
label = "dummy \"it\""
description = "A subject that refers to nothing, as in \"it rains\"."

[slot."Pronoun:genitive"]
label = "possessive pronoun"
description = "Stands for something owned: mine, theirs."

[slot."Pronoun:reflexive"]
label = "reflexive pronoun"
description = "Points back to the subject: herself, themselves."

[slot."Conjunction:coordinating"]
label = "coordinating conjunction"
description = "Joins two equal clauses: and, but, or."

[slot."Conjunction:subordinating"]
label = "subordinating conjunction"
description = "Attaches a clause that depends on the main one: because, although."

[slot."Conjunction:np"]
label = "\"and\" or \"or\""
description = "Joins two noun phrases."

[slot."Conjunction:neither"]
label = "\"neither\""
description = "Opens a neither...nor pair."

[slot."Conjunction:nor"]
label = "\"nor\""
description = "Closes a neither...nor pair."

[slot.Comma]
label = "comma"
description = "Punctuation before a conjunction that joins two clauses."

[slot.Complementizer]
label = "\"that\""
description = "Introduces a clause that a verb takes."

[slot."Complementizer:whether"]
label = "\"whether\""
description = "Introduces a choice the verb weighs."

[slot.To]
label = "\"to\""
description = "Marks an infinitive."

[slot."Verb:intransitive"]
label = "intransitive verb"
description = "Takes no object."
example = "slept"

[slot."Verb:transitive"]
label = "transitive verb"
description = "Takes an object."
example = "devoured the goose"

[slot."Verb:ditransitive"]
label = "ditransitive verb"
description = "Takes two objects."
example = "gave her the goose"

[slot."Verb:intransitive-pp"]
label = "verb with a phrase"
description = "Takes a prepositional phrase."
example = "lived under the bridge"

[slot."Verb:transitive-pp"]
label = "verb with an object and a phrase"
description = "Takes an object and a prepositional phrase."
example = "put the goose under the bridge"

[slot."Verb:intransitive-on"]
label = "verb with \"on\""
description = "Takes \"on\" and a noun phrase."
example = "relied on the goose"

[slot."Verb:intransitive-to"]
label = "verb with \"to\""
description = "Takes \"to\" and a noun phrase."
example = "listened to the goose"

[slot."Verb:transitive-from"]
label = "verb with an object and \"from\""
description = "Takes an object, then \"from\" and a noun phrase."
example = "stole the bread from the goose"

[slot."Verb:transitive-of"]
label = "verb with an object and \"of\""
description = "Takes an object, then \"of\" and a noun phrase."
example = "robbed the goose of its bread"

[slot."Verb:transitive-on"]
label = "verb with an object and \"on\""
description = "Takes an object, then \"on\" and a noun phrase."
example = "blamed the mess on the goose"

[slot."Verb:transitive-to"]
label = "verb with an object and \"to\""
description = "Takes an object, then \"to\" and a noun phrase."
example = "handed the bread to the goose"

[slot."Verb:transitive-with"]
label = "verb with an object and \"with\""
description = "Takes an object, then \"with\" and a noun phrase."
example = "fed the goose with bread"

[slot."Verb:that-clause"]
label = "verb with a clause"
description = "Takes a whole clause, often after \"that\"."
example = "said that the goose sang"

[slot."Verb:to-infinitive"]
label = "verb with an infinitive"
description = "Takes an infinitive."
example = "wanted to sing"

[slot."Verb:transitive-to-infinitive"]
label = "verb with an object and an infinitive"
description = "Takes an object, then an infinitive."
example = "urged her to sing"

[slot."Verb:whether-infinitive"]
label = "verb with \"whether\""
description = "Takes \"whether\" and an infinitive."
example = "wondered whether to sing"

[slot."Verb:gerund"]
label = "verb with a gerund"
description = "Takes a gerund phrase."
example = "enjoyed singing"

[slot."Verb:transitive-into-gerund"]
label = "verb with an object and \"into\""
description = "Takes an object, then \"into\" and a gerund phrase."
example = "coaxed her into singing"

[slot."Verb:adjective"]
label = "linking verb"
description = "Takes an adjective that describes the subject."
example = "seemed ugly"

[slot."Verb:transitive-adjective"]
label = "verb with an object and an adjective"
description = "Takes an object, then an adjective that describes it."
example = "considered her ugly"

[slot."Verb:weather"]
label = "weather verb"
description = "Takes the dummy subject \"it\" and nothing else."
example = "it rains"

[slot."Verb:dummy-that-clause"]
label = "verb with dummy \"it\" and a clause"
description = "Takes the dummy subject \"it\" and a clause."
example = "it seems that the goose sang"
```

- [ ] **Step 6: Make the server require labels**

In `service/cmd/server/main.go`, replace

```go
	g, err := grammar.Load(f)
	f.Close()
	if err != nil {
		log.Fatalf("load %s: %v", grammarPath, err)
	}
```

with

```go
	g, err := grammar.Load(f)
	f.Close()
	if err == nil {
		err = g.Labeled()
	}
	if err != nil {
		log.Fatalf("load %s: %v", grammarPath, err)
	}
```

- [ ] **Step 7: Run the package tests**

Run: `cd service && go test ./internal/grammar/ && go build ./...`
Expected: PASS, and the build succeeds.

- [ ] **Step 8: Commit**

```bash
git add service/internal/grammar/grammar.go service/internal/grammar/grammar_test.go service/data/grammar/grammar.toml service/cmd/server/main.go
git commit -m "feat: Label every phrase and slot in the grammar"
```

---

### Task 2: `Grammar.Check` in `realize`

**Files:**
- Modify: `service/internal/grammar/grammar.go` (add `Check`/`check`; delete `Node.Validate`, whose only caller is `realize`)
- Modify: `service/internal/api/handlers.go:137` (`tree.Validate()` becomes `h.grammar.Check(&tree)`)
- Modify: `service/internal/api/helpers_test.go` (`newServer` keeps a caller's grammar; add `loadGrammar`)
- Modify: `service/internal/api/handlers_test.go` (add `realizeGrammar` and `newRealizeServer`; update the realize tests)
- Modify: `service/internal/api/sentences_test.go` (`TestRealizedSentenceIsNotSaved` uses `newRealizeServer`)
- Test: `service/internal/grammar/grammar_test.go` (replace the two `Validate` tests)
- Modify: `README.md` (realize paragraph and example)

**Interfaces:**
- Consumes: `Grammar.rules`, `checkTerminal` (existing)
- Produces: `func (g *Grammar) Check(tree *Node) error`. In this plan a childless phrase is an error. Plan B relaxes that for holes.

- [ ] **Step 1: Write the failing grammar tests**

Delete `TestValidateAcceptsTreeOfTerminals` and `TestValidateRejectsMalformedTrees` from `grammar_test.go` and add:

```go
const checkGrammar = `
[[rule]]
symbol = "S"
expansion = ["NP", "VP"]

[[rule]]
symbol = "NP"
expansion = ["Determiner", "Noun"]

[[rule]]
symbol = "NP"
expansion = ["Pronoun"]

[[rule]]
symbol = "VP"
expansion = ["Verb:transitive", "NP"]
`

func tn(symbol string, children ...*grammar.Node) *grammar.Node {
	return &grammar.Node{Symbol: symbol, Children: children}
}

func TestCheckAcceptsTreeTheGrammarDerives(t *testing.T) {
	g := mustLoad(t, checkGrammar)
	tree := tn("S", tn("NP", tn("Determiner"), tn("Noun")), tn("VP", tn("Verb:transitive"), tn("NP", tn("Pronoun"))))

	if err := g.Check(tree); err != nil {
		t.Errorf("expected no error; got %v", err)
	}
}

func TestCheckRejectsTreesTheGrammarCannotDerive(t *testing.T) {
	g := mustLoad(t, checkGrammar)
	np := func() *grammar.Node { return tn("NP", tn("Pronoun")) }
	tests := []struct {
		name string
		tree *grammar.Node
		want string
	}{
		{"root that isn't the start symbol", tn("NP", tn("Pronoun")), `root is "NP"`},
		{"expansion that isn't a rule", tn("S", np(), tn("VP", tn("Verb:transitive"))), `"VP"`},
		{"rule with symbols out of order", tn("S", tn("VP", tn("Verb:transitive"), np()), np()), `"S"`},
		{"phrase left empty", tn("S", tn("NP"), tn("VP", tn("Verb:transitive"), np())), `"NP"`},
		{"part of speech with children", tn("S", np(), tn("VP", tn("Verb:transitive", tn("Noun")), np())), `"Verb:transitive"`},
		{"unknown qualifier", tn("S", np(), tn("VP", tn("Verb:bogus"), np())), `"VP"`},
		{"lone start symbol", tn("S"), `"S"`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := g.Check(tc.tree)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("expected error mentioning %s; got %v", tc.want, err)
			}
		})
	}
}
```

A part of speech with children still matches its parent's rule (`VP → [Verb:transitive, NP]`), so the error comes from recursing into `Verb:transitive`, which has no rules. An unknown qualifier fails at the parent, because no rule spells `Verb:bogus`.

- [ ] **Step 2: Run to verify failure**

Run: `cd service && go test ./internal/grammar/`
Expected: compile failure, `g.Check undefined`.

- [ ] **Step 3: Implement `Check` and remove `Validate`**

Delete `func (n *Node) Validate() error` from `grammar.go` and add in its place:

```go
// Check reports whether tree derives from the grammar: the root is the
// start symbol, every inner node's children spell one of its symbol's
// rules, and every leaf is a part of speech.
func (g *Grammar) Check(tree *Node) error {
	if tree.Symbol != start {
		return fmt.Errorf("grammar: root is %q, not %q", tree.Symbol, start)
	}
	return g.check(tree)
}

func (g *Grammar) check(n *Node) error {
	if len(n.Children) == 0 {
		// Rules only spell valid terminals, so a leaf that matched its
		// parent's rule is either a part of speech or an empty phrase.
		if _, phrase := g.rules[n.Symbol]; phrase {
			return fmt.Errorf("grammar: %q has no children", n.Symbol)
		}
		return nil
	}
	rules, ok := g.rules[n.Symbol]
	if !ok {
		return fmt.Errorf("grammar: %q is a part of speech and cannot have children", n.Symbol)
	}
	symbols := make([]string, len(n.Children))
	for i, c := range n.Children {
		symbols[i] = c.Symbol
	}
	if !slices.ContainsFunc(rules, func(r rule) bool { return slices.Equal(r.Expansion, symbols) }) {
		return fmt.Errorf("grammar: %q -> %v is not a rule", n.Symbol, symbols)
	}
	for _, c := range n.Children {
		if err := g.check(c); err != nil {
			return err
		}
	}
	return nil
}
```

- [ ] **Step 4: Run grammar tests**

Run: `cd service && go test ./internal/grammar/`
Expected: PASS.

- [ ] **Step 5: Write the failing API tests**

In `helpers_test.go`, replace the grammar lines in `newServer`:

```go
	g, err := grammar.Load(strings.NewReader(testGrammar))
	if err != nil {
		t.Fatalf("grammar.Load: %v", err)
	}
```

with nothing, and change `cfg.Grammar, cfg.Verbs = g, v` to:

```go
	if cfg.Grammar == nil {
		cfg.Grammar = loadGrammar(t, testGrammar)
	}
	cfg.Verbs = v
```

Update its doc comment to "newServer serves the API with cfg, filling in the test grammar unless cfg has its own, the verbs, and testPool unless cfg has its own queries." Then add:

```go
func loadGrammar(t *testing.T, in string) *grammar.Grammar {
	t.Helper()
	g, err := grammar.Load(strings.NewReader(in))
	if err != nil {
		t.Fatalf("grammar.Load: %v", err)
	}
	return g
}
```

In `handlers_test.go`, add `"github.com/jameynakama/randsense/internal/api"` to the imports and, next to `realizeTree`, add:

```go
// realizeGrammar derives the trees the realize tests post.
const realizeGrammar = `
[[rule]]
symbol = "S"
expansion = ["NP", "VP"]

[[rule]]
symbol = "S"
expansion = ["Verb:ditransitive"]

[[rule]]
symbol = "NP"
expansion = ["Determiner", "Noun"]

[[rule]]
symbol = "VP"
expansion = ["Verb:transitive", "NP"]

[[rule]]
symbol = "VP"
expansion = ["Verb"]
`

func newRealizeServer(t *testing.T) *httptest.Server {
	t.Helper()
	return newServer(t, api.RouterConfig{Grammar: loadGrammar(t, realizeGrammar)})
}
```

Replace `newTestServer(t)` with `newRealizeServer(t)` in `TestRealizeSentence`, `TestRealizeSentenceRejectsBadRequests` and `TestRealizeSentenceRecomputesPostedFeatures` (`handlers_test.go`), and in `TestRealizedSentenceIsNotSaved` (`sentences_test.go`). In `TestRealizeSentenceRejectsBadRequests`, replace the `"leaf that isn't a POS"` case with:

```go
		{"expansion that isn't a rule", "", `{"symbol": "S", "children": [{"symbol": "Noun"}]}`, http.StatusBadRequest},
		{"root that isn't S", "", `{"symbol": "NP", "children": [{"symbol": "Determiner"}, {"symbol": "Noun"}]}`, http.StatusBadRequest},
		{"phrase left empty", "", `{"symbol": "S", "children": [{"symbol": "NP"}, {"symbol": "VP", "children": [{"symbol": "Verb"}]}]}`, http.StatusBadRequest},
		{"part of speech with children", "", `{"symbol": "S", "children": [{"symbol": "Verb:ditransitive", "children": [{"symbol": "Noun"}]}]}`, http.StatusBadRequest},
		{"unknown qualifier", "", `{"symbol": "S", "children": [{"symbol": "Verb:bogus"}]}`, http.StatusBadRequest},
```

Keep the `"frame without verbs"` case as it is: `S → [Verb:ditransitive]` is now a rule, and no seeded verb has that frame, so it's still a 422.

- [ ] **Step 6: Run to verify failure**

Run: `just test-be`
Expected: build failure in `internal/api`: `tree.Validate undefined`.

- [ ] **Step 7: Switch `realize` to `Check`**

In `handlers.go`, change

```go
	if err := tree.Validate(); err != nil {
```

to

```go
	if err := h.grammar.Check(&tree); err != nil {
```

and update the handler's doc comment to: "realizeSentence fills a posted tree, in the shape randomSentence returns, with words. The tree must derive from the grammar. Any words and features already in it are replaced."

- [ ] **Step 8: Run the backend suite**

Run: `just test-be`
Expected: PASS.

- [ ] **Step 9: Update the README**

In `README.md`, replace the sentence "Leaves must be parts of speech, optionally qualified as in `grammar.toml`." with "The tree must derive from `grammar.toml`: the root is `S`, and every node's children spell one of its rules." Replace the realize example's JSON with:

```bash
echo '{"symbol": "S", "children": [{"symbol": "Clause", "children": [
  {"symbol": "NP", "children": [{"symbol": "Pronoun"}]},
  {"symbol": "VP", "children": [{"symbol": "Verb:transitive"}, {"symbol": "Pronoun:reflexive"}]}
]}]}' | http POST :8080/api/v1/sentences/realize | jq .text
```

- [ ] **Step 10: Commit**

```bash
git add service/internal/grammar service/internal/api README.md
git commit -m "feat: Check realize trees against the grammar's rules"
```

---

### Task 3: `GET /api/v1/grammar`

**Files:**
- Create: `service/internal/api/grammar.go`
- Modify: `service/internal/api/router.go` (route)
- Test: `service/internal/api/grammar_test.go`
- Modify: `README.md` (API list and a paragraph)

**Interfaces:**
- Consumes: `(*grammar.Grammar).Describe() grammar.Description` (Task 1), `writeJSON` (router.go)
- Produces: `GET /api/v1/grammar` returning `grammar.Description` as JSON with `Cache-Control: public, max-age=300`

- [ ] **Step 1: Write the failing test**

Create `service/internal/api/grammar_test.go`:

```go
package api_test

import (
	"net/http"
	"reflect"
	"testing"

	"github.com/jameynakama/randsense/internal/api"
	"github.com/jameynakama/randsense/internal/grammar"
)

const labeledGrammar = `
[[rule]]
symbol = "S"
expansion = ["NP", "Verb"]

[[rule]]
symbol = "NP"
expansion = ["Determiner", "Noun"]

[phrase.S]
label = "sentence"
description = "A complete thought."

[phrase.NP]
label = "noun phrase"
description = "Names a thing."

[slot.Determiner]
label = "determiner"
description = "Points at a noun."

[slot.Noun]
label = "noun"
description = "A thing."

[slot.Verb]
label = "verb"
description = "Says what happens."
`

func TestGrammarServesRulesAndLabels(t *testing.T) {
	srv := newServer(t, api.RouterConfig{Grammar: loadGrammar(t, labeledGrammar)})
	defer srv.Close()

	resp := call(t, srv, http.MethodGet, "/api/v1/grammar", "", nil)
	var got grammar.Description
	decode(t, resp, http.StatusOK, &got)

	want := grammar.Description{
		Start: "S",
		Phrases: map[string]grammar.Phrase{
			"S":  {Label: grammar.Label{Label: "sentence", Description: "A complete thought."}, Rules: [][]string{{"NP", "Verb"}}},
			"NP": {Label: grammar.Label{Label: "noun phrase", Description: "Names a thing."}, Rules: [][]string{{"Determiner", "Noun"}}},
		},
		Slots: map[string]grammar.Label{
			"Determiner": {Label: "determiner", Description: "Points at a noun."},
			"Noun":       {Label: "noun", Description: "A thing."},
			"Verb":       {Label: "verb", Description: "Says what happens."},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("grammar:\n got %+v\nwant %+v", got, want)
	}
	if cc := resp.Header.Get("Cache-Control"); cc != "public, max-age=300" {
		t.Errorf("Cache-Control: got %q", cc)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `just test-be`
Expected: FAIL, `status: got 404, want 200`.

- [ ] **Step 3: Implement**

Create `service/internal/api/grammar.go`:

```go
package api

import "net/http"

// grammarCacheSeconds is how long clients may reuse the grammar. It only
// changes on deploy.
const grammarCacheSeconds = "300"

// getGrammar serves the grammar's rules and labels, so the frontend never
// hardcodes either.
func (h *Handler) getGrammar(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "public, max-age="+grammarCacheSeconds)
	writeJSON(w, http.StatusOK, h.grammar.Describe())
}
```

In `router.go`, inside `r.Route("/api/v1", ...)`, add after the `/words/random` line:

```go
		r.Get("/grammar", h.getGrammar)
```

- [ ] **Step 4: Run the backend suite**

Run: `just test-be`
Expected: PASS.

- [ ] **Step 5: Document**

In `README.md`'s API block, add after the `/words/random` line:

```
GET /api/v1/grammar                           -> {start, phrases, slots}
```

and add after the paragraph that ends "`realize` saves nothing.":

```markdown
`grammar` serves `grammar.toml` for the frontend: `start` is `S`; `phrases` maps each phrase to
its `label`, `description` and `rules` (each a list of symbols, in file order); `slots` maps each
part-of-speech symbol a rule uses to its `label`, `description` and, for verb frames, `example`.
```

- [ ] **Step 6: Commit**

```bash
git add service/internal/api/grammar.go service/internal/api/grammar_test.go service/internal/api/router.go README.md
git commit -m "feat: Serve the grammar and its labels"
```

---

### Task 4: Grammar types and the root layout load

**Files:**
- Modify: `web/src/lib/types.ts`
- Create: `web/src/routes/+layout.server.ts`
- Modify: `web/src/lib/testing/fixtures.ts` (add `grammar`)

**Interfaces:**
- Consumes: `getJSON` (`#lib/server/api.js`)
- Produces:
  - `interface Label { label: string; description: string; example?: string }`
  - `interface Phrase extends Label { rules: string[][] }`
  - `interface Grammar { start: string; phrases: Record<string, Phrase>; slots: Record<string, Label> }`
  - layout data `grammar: Grammar`, which every page's `data` carries
  - fixture `grammar: Grammar` labeling every symbol in the fixture `sentence`

This task has no test of its own. The load is one line over the tested `getJSON`, and the e2e tests in Task 7 exercise it through every page.

- [ ] **Step 1: Add the types**

Append to `web/src/lib/types.ts`:

```ts
export interface Label {
	label: string;
	description: string;
	example?: string;
}

export interface Phrase extends Label {
	// Each rule is the symbols a phrase can expand to, in grammar.toml order.
	rules: string[][];
}

export interface Grammar {
	start: string;
	phrases: Record<string, Phrase>;
	slots: Record<string, Label>;
}
```

- [ ] **Step 2: Load the grammar for every page**

Create `web/src/routes/+layout.server.ts`:

```ts
import { getJSON } from '#lib/server/api.js';
import type { Grammar } from '#lib/types.js';
import type { LayoutServerLoad } from './$types';

export const load: LayoutServerLoad = async ({ fetch }) => ({
	grammar: await getJSON<Grammar>(fetch, '/api/v1/grammar')
});
```

- [ ] **Step 3: Add the fixture grammar**

In `web/src/lib/testing/fixtures.ts`, change the import to `import type { Grammar, Sentence, TreeNode } from '#lib/types.js';` and append:

```ts
// grammar labels every symbol in sentence's tree.
export const grammar: Grammar = {
	start: 'S',
	phrases: {
		S: { label: 'sentence', description: 'A complete thought.', rules: [['Clause']] },
		Clause: { label: 'clause', description: 'A subject and what it does.', rules: [['NP', 'VP']] },
		NP: { label: 'noun phrase', description: 'Names a thing.', rules: [['Determiner', 'Noun']] },
		VP: { label: 'verb phrase', description: 'A verb and what it takes.', rules: [['Verb:transitive', 'NP']] }
	},
	slots: {
		Determiner: { label: 'determiner', description: 'Points at a noun.' },
		Noun: { label: 'noun', description: 'A thing.' },
		Pronoun: { label: 'pronoun', description: 'Stands in for a noun phrase.' },
		Comma: { label: 'comma', description: 'Punctuation.' },
		'Conjunction:coordinating': { label: 'coordinating conjunction', description: 'Joins clauses.' },
		'Verb:transitive': { label: 'transitive verb', description: 'Takes an object.', example: 'devoured the goose' },
		'Verb:intransitive': { label: 'intransitive verb', description: 'Takes no object.', example: 'slept' }
	}
};
```

- [ ] **Step 4: Type-check**

Run: `cd web && npx prettier --write src && npm run check`
Expected: 0 errors.

- [ ] **Step 5: Commit**

```bash
git add web/src/lib/types.ts web/src/routes/+layout.server.ts web/src/lib/testing/fixtures.ts
git commit -m "feat: Load the grammar for every page"
```

---

### Task 5: Diagram layout

**Files:**
- Create: `web/src/lib/diagram.ts`
- Test: `web/src/lib/diagram.spec.ts` (node project)

**Interfaces:**
- Consumes: `TreeNode` (`#lib/types.js`), `pos` (`#lib/tree.js`)
- Produces:
  - `ROW = 40`, `LABEL_HEIGHT = 20`, `WORD_HEIGHT = 24`, `GAP = 16` (px)
  - `interface Box { x: number; y: number; width: number }`, where `x` is the center and `y` the top
  - `interface Placed { node: TreeNode; path: number[]; label: Box; word?: Box & { text: string } }`
  - `interface Edge { from: [number, number]; to: [number, number]; dotted: boolean; path: number[] }`, where `path` is the child's path (a dotted edge's is the leaf's own)
  - `interface Layout { placed: Map<string, Placed>; edges: Edge[]; width: number; height: number }`
  - `type Measure = (text: string, kind: 'label' | 'word') => number`
  - `function key(path: number[]): string`
  - `function short(node: TreeNode): string`: the label drawn for a node
  - `function layout(tree: TreeNode, measure: Measure): Layout`

- [ ] **Step 1: Write the failing tests**

Create `web/src/lib/diagram.spec.ts`:

```ts
import { describe, expect, it } from 'vitest';
import { sentence } from './testing/fixtures';
import { GAP, key, layout, ROW, short, type Box } from './diagram';
import { leaves } from './tree';
import type { TreeNode } from './types';

const measure = (text: string) => text.length * 10;
const drawn = layout(sentence.tree, measure);
const at = (path: number[]) => drawn.placed.get(key(path))!;

describe('layout', () => {
	it('places leaves left to right in sentence order, a gap apart', () => {
		const xs = leaves(sentence.tree).map((l) => at(l.path));
		const half = (p: (typeof xs)[number]) => Math.max(p.label.width, p.word?.width ?? 0) / 2;
		for (let i = 1; i < xs.length; i++) {
			expect(xs[i].label.x - half(xs[i]) - (xs[i - 1].label.x + half(xs[i - 1]))).toBe(GAP);
		}
		expect(xs.map((p) => p.word?.text)).toEqual(['the', 'goose', 'devoured', 'her', ',', 'but', 'she', 'sang']);
	});

	it('centers each phrase over its first and last child', () => {
		const clause = at([0]);
		expect(clause.label.x).toBe((at([0, 0]).label.x + at([0, 1]).label.x) / 2);
		const np = at([0, 0]);
		expect(np.label.x).toBe((at([0, 0, 0]).label.x + at([0, 0, 1]).label.x) / 2);
	});

	it('puts each node on its depth’s row and every word on one baseline below the deepest', () => {
		expect(at([]).label.y).toBe(0);
		expect(at([0, 1, 1, 0]).label.y).toBe(4 * ROW);
		const baselines = new Set(leaves(sentence.tree).map((l) => at(l.path).word!.y));
		expect([...baselines]).toEqual([5 * ROW]);
	});

	it('keeps boxes on the same row apart', () => {
		const rows = new Map<number, Box[]>();
		for (const p of drawn.placed.values()) {
			for (const b of [p.label, p.word].filter((b) => b !== undefined)) {
				rows.set(b.y, [...(rows.get(b.y) ?? []), b]);
			}
		}
		for (const boxes of rows.values()) {
			boxes.sort((a, b) => a.x - b.x);
			for (let i = 1; i < boxes.length; i++) {
				expect(boxes[i].x - boxes[i].width / 2).toBeGreaterThan(boxes[i - 1].x + boxes[i - 1].width / 2);
			}
		}
	});

	it('draws one solid edge per child and one dotted edge per word', () => {
		const solid = drawn.edges.filter((e) => !e.dotted);
		const dotted = drawn.edges.filter((e) => e.dotted);
		expect(solid).toHaveLength(drawn.placed.size - 1);
		expect(dotted).toHaveLength(leaves(sentence.tree).length);
	});

	it('sizes the drawing to its contents', () => {
		const last = at(leaves(sentence.tree).at(-1)!.path);
		expect(drawn.width).toBe(last.label.x + Math.max(last.label.width, last.word!.width) / 2);
		expect(drawn.height).toBe(5 * ROW + 24);
	});

	it('writes a separable verb as it reads in the sentence', () => {
		const tree: TreeNode = {
			symbol: 'S',
			children: [{ symbol: 'Verb:transitive', lemma: 'look up', word: 'looked up', display: 'looked' }]
		};
		expect(layout(tree, measure).placed.get(key([0]))!.word!.text).toBe('looked');
	});

	it('gives a comma its own column', () => {
		const comma = at([1]);
		expect(comma.word!.text).toBe(',');
		expect(comma.label.width).toBeGreaterThan(0);
		expect(comma.label.x).toBeGreaterThan(at([0]).label.x);
	});

	it('lays out a tree that is a single leaf under the root', () => {
		const one = layout({ symbol: 'S', children: [{ symbol: 'Verb', word: 'rains' }] }, measure);
		expect(one.width).toBe(Math.max(measure('Verb'), measure('rains')));
		expect(one.placed.get(key([]))!.label.x).toBe(one.placed.get(key([0]))!.label.x);
	});
});

describe('short', () => {
	it.each([
		['Determiner', 'Det'],
		['Verb:transitive', 'Verb'],
		['Comma', 'Punct'],
		['Conjunction:np', 'Conj'],
		['NP', 'NP'],
		['Mystery', 'Mystery']
	])('labels %s as %s', (symbol, want) => {
		expect(short({ symbol, children: symbol === 'NP' ? [{ symbol: 'Noun' }] : undefined })).toBe(want);
	});
});
```

- [ ] **Step 2: Run to verify failure**

Run: `cd web && npx vitest --run --project server src/lib/diagram.spec.ts`
Expected: FAIL, cannot resolve `./diagram`.

- [ ] **Step 3: Implement**

Create `web/src/lib/diagram.ts`:

```ts
import { pos } from './tree';
import type { TreeNode } from './types';

// Pixel sizes, kept in step with Diagram.svelte's styles.
export const ROW = 40;
export const LABEL_HEIGHT = 20;
export const WORD_HEIGHT = 24;
export const GAP = 16;

// A box's x is its center and y its top.
export interface Box {
	x: number;
	y: number;
	width: number;
}

export interface Placed {
	node: TreeNode;
	path: number[];
	label: Box;
	word?: Box & { text: string };
}

// An edge's path is its child's, or for a dotted edge to a word, its leaf's.
export interface Edge {
	from: [number, number];
	to: [number, number];
	dotted: boolean;
	path: number[];
}

export interface Layout {
	placed: Map<string, Placed>;
	edges: Edge[];
	width: number;
	height: number;
}

export type Measure = (text: string, kind: 'label' | 'word') => number;

export function key(path: number[]): string {
	return path.join('.');
}

const abbreviations: Record<string, string> = {
	Determiner: 'Det',
	Adjective: 'Adj',
	Adverb: 'Adv',
	Preposition: 'Prep',
	Pronoun: 'Pron',
	Conjunction: 'Conj',
	Complementizer: 'Comp',
	Comma: 'Punct'
};

// short is the label drawn for a node: a phrase's symbol, or a leaf's part
// of speech, abbreviated the way trees usually are.
export function short(node: TreeNode): string {
	if (node.children?.length) return node.symbol;
	const p = pos(node);
	return abbreviations[p] ?? p;
}

function deepestLeaf(node: TreeNode, depth = 0): number {
	if (!node.children?.length) return depth;
	return Math.max(...node.children.map((c) => deepestLeaf(c, depth + 1)));
}

// layout places a tree for drawing. Leaves get columns left to right, as
// wide as their label or word; each phrase centers over its first and last
// child; each depth is a row; and every word sits on one baseline under
// the deepest leaf.
export function layout(tree: TreeNode, measure: Measure): Layout {
	const placed = new Map<string, Placed>();
	const edges: Edge[] = [];
	const baseline = (deepestLeaf(tree) + 1) * ROW;
	let cursor = 0;

	function place(node: TreeNode, path: number[]): number {
		const y = path.length * ROW;
		const text = short(node);
		const labelWidth = measure(text, 'label');

		if (!node.children?.length) {
			const written = node.display ?? node.word ?? '';
			const wordWidth = written ? measure(written, 'word') : 0;
			const column = Math.max(labelWidth, wordWidth);
			const x = cursor + column / 2;
			cursor += column + GAP;
			const p: Placed = { node, path, label: { x, y, width: labelWidth } };
			if (written) {
				p.word = { x, y: baseline, width: wordWidth, text: written };
				edges.push({ from: [x, y + LABEL_HEIGHT], to: [x, baseline], dotted: true, path });
			}
			placed.set(key(path), p);
			return x;
		}

		const xs = node.children.map((c, i) => place(c, [...path, i]));
		const x = (xs[0] + xs[xs.length - 1]) / 2;
		placed.set(key(path), { node, path, label: { x, y, width: labelWidth } });
		xs.forEach((cx, i) =>
			edges.push({ from: [x, y + LABEL_HEIGHT], to: [cx, y + ROW], dotted: false, path: [...path, i] })
		);
		return x;
	}

	place(tree, []);
	return { placed, edges, width: cursor - GAP, height: baseline + WORD_HEIGHT };
}
```

- [ ] **Step 4: Run the tests**

Run: `cd web && npx vitest --run --project server src/lib/diagram.spec.ts`
Expected: PASS. If "keeps boxes on the same row apart" fails, a phrase label is wider than the span of its children. Fix that in `layout`, not in the test.

- [ ] **Step 5: Commit**

```bash
cd web && npx prettier --write src/lib/diagram.ts src/lib/diagram.spec.ts && cd ..
git add web/src/lib/diagram.ts web/src/lib/diagram.spec.ts
git commit -m "feat: Lay out sentence diagrams"
```

---

### Task 6: `Diagram.svelte`

**Files:**
- Create: `web/src/lib/components/Diagram.svelte`
- Test: `web/src/lib/components/Diagram.svelte.spec.ts` (browser project)

**Interfaces:**
- Consumes: `layout`, `key`, `short`, `ROW`-family constants (Task 5); `Grammar`, `TreeNode` (Task 4)
- Produces: `<Diagram tree={TreeNode} grammar={Grammar} selectedPath={number[] | null} />`
  - renders `ul[aria-label="Sentence diagram"]`, a nested list in tree order, where each item's accessible text is the symbol's grammar label ("noun phrase")
  - marks the selected word `aria-current="true"` and adds class `on` to every label on its branch
  - shows a `Zoom` button (`aria-pressed`) only when the tree is wider than its container

- [ ] **Step 1: Write the failing tests**

Create `web/src/lib/components/Diagram.svelte.spec.ts`:

```ts
import { page } from 'vitest/browser';
import { afterEach, describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { grammar, sentence } from '#lib/testing/fixtures.js';
import Diagram from './Diagram.svelte';

describe('Diagram', () => {
	afterEach(() => page.viewport(1280, 800));

	it('lists the tree under its grammar labels', async () => {
		render(Diagram, { tree: sentence.tree, grammar });

		const list = page.getByRole('list', { name: 'Sentence diagram' });
		await expect.element(list).toBeVisible();
		await expect.element(list.getByText('noun phrase').first()).toBeInTheDocument();
		await expect.element(list.getByText('transitive verb')).toBeInTheDocument();
		await expect.element(list.getByText('devoured')).toBeVisible();
	});

	it('marks the selected word and its branch', async () => {
		render(Diagram, { tree: sentence.tree, grammar, selectedPath: [0, 0, 1] });

		const current = document.querySelector('[aria-current="true"]');
		expect(current?.textContent).toBe('goose');
		expect(document.querySelectorAll('.label.on')).toHaveLength(4);
	});

	it('offers Zoom only when the tree is wider than the screen', async () => {
		render(Diagram, { tree: sentence.tree, grammar });
		await expect.element(page.getByRole('button', { name: 'Zoom' })).not.toBeInTheDocument();

		await page.viewport(320, 640);
		const zoom = page.getByRole('button', { name: 'Zoom' });
		await expect.element(zoom).toHaveAttribute('aria-pressed', 'false');
		await zoom.click();
		await expect.element(zoom).toHaveAttribute('aria-pressed', 'true');
	});
});
```

- [ ] **Step 2: Run to verify failure**

Run: `cd web && npx vitest --run --project client src/lib/components/Diagram.svelte.spec.ts`
Expected: FAIL, cannot resolve `./Diagram.svelte`.

- [ ] **Step 3: Implement**

Create `web/src/lib/components/Diagram.svelte`:

```svelte
<script lang="ts">
	import { onMount } from 'svelte';
	import { key, layout, short, type Measure } from '#lib/diagram.js';
	import type { Grammar, TreeNode } from '#lib/types.js';

	let {
		tree,
		grammar,
		selectedPath = null
	}: { tree: TreeNode; grammar: Grammar; selectedPath?: number[] | null } = $props();

	// Kept in step with .label and .word below, since the layout measures text
	// in these fonts.
	const fonts = {
		label: "600 13px 'Atkinson Hyperlegible Next Variable', sans-serif",
		word: "500 17px 'Atkinson Hyperlegible Next Variable', sans-serif"
	};
	let context: CanvasRenderingContext2D | null = null;
	// Measuring before the web font arrives would use the fallback's widths,
	// so the layout runs again once it has.
	let fontsReady = $state(false);
	onMount(() => {
		document.fonts.ready.then(() => (fontsReady = true));
	});
	const measure: Measure = (text, kind) => {
		context ??= document.createElement('canvas').getContext('2d')!;
		context.font = fonts[kind];
		return context.measureText(text).width;
	};

	const drawn = $derived.by(() => {
		void fontsReady;
		return layout(tree, measure);
	});
	let available = $state(0);
	let zoomed = $state(false);
	const fits = $derived(drawn.width <= available);
	const scale = $derived(zoomed || fits || !available ? 1 : available / drawn.width);

	function onBranch(path: number[]): boolean {
		return (
			selectedPath !== null &&
			path.length <= selectedPath.length &&
			path.every((v, i) => selectedPath[i] === v)
		);
	}

	function name(node: TreeNode): string {
		return (grammar.phrases[node.symbol] ?? grammar.slots[node.symbol])?.label ?? node.symbol;
	}
</script>

{#snippet branch(node: TreeNode, path: number[])}
	{@const placed = drawn.placed.get(key(path))!}
	<li>
		<span
			class="label"
			class:on={onBranch(path)}
			style:left="{placed.label.x}px"
			style:top="{placed.label.y}px"
			><span aria-hidden="true">{short(node)}</span><span class="visually-hidden"
				>{name(node)}</span
			></span
		>
		{#if placed.word}
			<span
				class="word"
				aria-current={onBranch(path) && path.length === selectedPath?.length ? 'true' : undefined}
				style:left="{placed.word.x}px"
				style:top="{placed.word.y}px">{placed.word.text}</span
			>
		{/if}
		{#if node.children?.length}
			<ul>
				{#each node.children as child, i (i)}
					{@render branch(child, [...path, i])}
				{/each}
			</ul>
		{/if}
	</li>
{/snippet}

<div class="diagram" bind:clientWidth={available}>
	{#if !fits}
		<button type="button" class="button plain" aria-pressed={zoomed} onclick={() => (zoomed = !zoomed)}
			>Zoom</button
		>
	{/if}
	<div class="viewport" class:zoomed style:height="{drawn.height * scale}px">
		<div
			class="canvas"
			style:width="{drawn.width}px"
			style:height="{drawn.height}px"
			style:scale
		>
			<svg aria-hidden="true" width={drawn.width} height={drawn.height}>
				{#each drawn.edges as e, i (i)}
					<line
						x1={e.from[0]}
						y1={e.from[1]}
						x2={e.to[0]}
						y2={e.to[1]}
						class:dotted={e.dotted}
						class:on={onBranch(e.path)}
					/>
				{/each}
			</svg>
			<ul aria-label="Sentence diagram">
				{@render branch(tree, [])}
			</ul>
		</div>
	</div>
</div>

<style>
	.diagram {
		margin-block: 1rem;
	}

	.viewport {
		overflow: hidden;
	}

	.viewport.zoomed {
		overflow-x: auto;
	}

	.canvas {
		position: relative;
		margin-inline: auto;
		transform-origin: 0 0;
	}

	svg {
		position: absolute;
		inset: 0;
	}

	line {
		stroke: var(--separator);
		stroke-width: 1.5;
	}

	line.dotted {
		stroke-dasharray: 2 3;
	}

	line.on {
		stroke: var(--word-selected);
		stroke-width: 2.5;
	}

	ul {
		list-style: none;
		margin: 0;
		padding: 0;
	}

	.label,
	.word {
		position: absolute;
		translate: -50% 0;
		white-space: nowrap;
	}

	.label {
		font: 600 13px/20px 'Atkinson Hyperlegible Next Variable', sans-serif;
		color: var(--secondary);
	}

	.label.on {
		color: var(--word-selected);
	}

	.word {
		font: 500 17px/24px 'Atkinson Hyperlegible Next Variable', sans-serif;
		color: var(--text);
	}

	.word[aria-current='true'] {
		color: var(--word-selected);
		text-decoration: underline;
		text-underline-offset: 0.15em;
	}
</style>
```

When the canvas is scaled down, its layout box is still `drawn.width` wide, and `.viewport`'s `overflow: hidden` clips the leftover space. That's intended: `transform-origin: 0 0` keeps the visible tree at the left edge.

- [ ] **Step 4: Run the tests**

Run: `cd web && npx vitest --run --project client src/lib/components/Diagram.svelte.spec.ts`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
cd web && npx prettier --write src/lib/components/Diagram.svelte src/lib/components/Diagram.svelte.spec.ts && npm run lint && cd ..
git add web/src/lib/components/Diagram.svelte web/src/lib/components/Diagram.svelte.spec.ts
git commit -m "feat: Draw sentence diagrams"
```

---

### Task 7: Show the diagram in place of the outline

**Files:**
- Modify: `web/src/lib/components/SentenceView.svelte` (Diagram in place of Structure, new `grammar` prop)
- Modify: `web/src/routes/+page.svelte`, `web/src/routes/s/[id]/+page.svelte` (pass `data.grammar`)
- Delete: `web/src/lib/components/Structure.svelte`, `Structure.svelte.spec.ts`, `Branch.svelte`
- Modify: `web/src/lib/tree.ts` (remove `phraseNames` and `symbolName`, now unused)
- Modify: `web/src/lib/tree.spec.ts` (drop the `symbolName` test and import)
- Test: `web/src/lib/components/SentenceView.svelte.spec.ts`, `web/src/routes/s/[id]/page.e2e.ts`, `web/src/routes/page.e2e.ts`
- Modify: `docs/superpowers/specs/2026-10-03-frontend-design.md` (Sentence component and Testing sections)

**Interfaces:**
- Consumes: `Diagram` (Task 6), layout data `grammar` (Task 4), fixtures `grammar`, `another` (Task 4)
- Produces: `<SentenceView sentence grammar bind:count />`, plus a "Show diagram" / "Hide diagram" toggle (`aria-expanded`)

- [ ] **Step 1: Write the failing component tests**

In `SentenceView.svelte.spec.ts`:
- change the import to `import { another, grammar, sentence } from '#lib/testing/fixtures.js';`
- add `grammar` to every `render(SentenceView, { ... })` props object
- in the first test, rename it "opens a word’s card and keeps the diagram closed until asked" and change `'Show structure'` to `'Show diagram'`

Then add:

```ts
	it('draws the diagram with the selected word marked', async () => {
		render(SentenceView, { sentence, grammar, count: 2 });

		await page.getByRole('button', { name: 'goose' }).click();
		await page.getByRole('button', { name: 'Show diagram' }).click();

		const diagram = page.getByRole('list', { name: 'Sentence diagram' });
		await expect.element(diagram).toBeVisible();
		expect(diagram.getByText('goose').element().getAttribute('aria-current')).toBe('true');
		await expect
			.element(page.getByRole('button', { name: 'Hide diagram' }))
			.toHaveAttribute('aria-expanded', 'true');
	});

	it('redraws an open diagram for the next sentence', async () => {
		const screen = render(SentenceView, { sentence, grammar, count: 2 });
		await page.getByRole('button', { name: 'Show diagram' }).click();

		const next = another('bbbbbbbb', 'The goose devoured her, but she wept.');
		next.tree.children![3].children![1].children![0].word = 'wept';
		await screen.rerender({ sentence: next, grammar, count: 0 });

		const diagram = page.getByRole('list', { name: 'Sentence diagram' });
		await expect.element(diagram.getByText('wept')).toBeVisible();
		await expect.element(diagram.getByText('sang')).not.toBeInTheDocument();
	});
```

- [ ] **Step 2: Run to verify failure**

Run: `cd web && npx vitest --run --project client src/lib/components/SentenceView.svelte.spec.ts`
Expected: FAIL, no button named "Show diagram".

- [ ] **Step 3: Swap in the diagram**

In `SentenceView.svelte`:
- replace `import Structure from './Structure.svelte';` with `import Diagram from './Diagram.svelte';`
- change the type import to `import type { Grammar, Sentence as SentenceData } from '#lib/types.js';`
- change the props line to:

```ts
	let {
		sentence,
		grammar,
		count = $bindable()
	}: { sentence: SentenceData; grammar: Grammar; count: number } = $props();
```

- add, below `let flagButton ...`:

```ts
	// Stays open from one sentence to the next.
	let showDiagram = $state(false);
```

- replace `<Structure tree={sentence.tree} {selectedPath} />` with:

```svelte
<button
	type="button"
	class="button plain"
	aria-expanded={showDiagram}
	onclick={() => (showDiagram = !showDiagram)}>{showDiagram ? 'Hide diagram' : 'Show diagram'}</button
>
{#if showDiagram}
	<Diagram tree={sentence.tree} {grammar} {selectedPath} />
{/if}
```

In `web/src/routes/+page.svelte`, change `<SentenceView sentence={current} bind:count={current.star_count} />` to `<SentenceView sentence={current} grammar={data.grammar} bind:count={current.star_count} />`. In `web/src/routes/s/[id]/+page.svelte`, change `<SentenceView sentence={data.sentence} bind:count={stars} />` to `<SentenceView sentence={data.sentence} grammar={data.grammar} bind:count={stars} />`.

Delete `Structure.svelte`, `Structure.svelte.spec.ts` and `Branch.svelte`. In `tree.ts`, delete `phraseNames` and `symbolName` with its comment. In `tree.spec.ts`, remove `symbolName` from the import, rename `describe('frame and symbolName'` to `describe('frame'`, and delete its `'names phrases and parts of speech'` test.

- [ ] **Step 4: Run the component tests**

Run: `cd web && npx vitest --run`
Expected: PASS.

- [ ] **Step 5: Write the e2e tests**

In `web/src/routes/s/[id]/page.e2e.ts`, add:

```ts
test('draws the diagram with no accessibility violations', async ({ page }) => {
	const s = await newSentence(page.request);
	await page.goto(`/s/${s.id}`);

	await page.getByRole('button', { name: 'Show diagram' }).click();

	await expect(page.getByRole('list', { name: 'Sentence diagram' })).toBeVisible();
	await expectNoAxeViolations(page);
});
```

In `web/src/routes/page.e2e.ts`, change the 320px test to open the diagram before checking for sideways scroll:

```ts
test('reflows the home page at 320px without sideways scrolling', async ({ page }) => {
	await page.setViewportSize({ width: 320, height: 640 });
	await page.goto('/');
	await ownSentence(page);
	await page.getByRole('button', { name: 'Show diagram' }).click();
	await expect(page.getByRole('list', { name: 'Sentence diagram' })).toBeVisible();

	await expectNoSidewaysScroll(page);
});
```

- [ ] **Step 6: Run the whole frontend suite**

Run: `just test-fe`
Expected: check, lint, vitest and Playwright all pass, on both the desktop and phone projects.

- [ ] **Step 7: Update the frontend spec**

In `docs/superpowers/specs/2026-10-03-frontend-design.md`:
- In "Sentence component", replace the bullet beginning `"Show structure" expands the tree` with: `- "Show diagram" draws the tree; the selected word highlights its branch. See \`2026-10-05-diagram-builder-design.md\`.`
- In "Testing", change `(word card, flag form, structure outline)` to `(word card, flag form, diagram)`.

- [ ] **Step 8: Commit**

```bash
cd web && npx prettier --write src && cd ..
git add -A web/src docs/superpowers/specs/2026-10-03-frontend-design.md
git commit -m "feat: Show the drawn diagram in place of the structure outline"
```
