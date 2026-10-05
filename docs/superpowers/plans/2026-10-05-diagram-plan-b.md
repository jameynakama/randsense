# Diagram Plan B: holes, locks, builder, Keep and Remix

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let anyone build a sentence top-down from the grammar's own rules, fill it, lock words
and reroll the rest, then keep it with a Homemade badge, or remix any saved sentence.

**Architecture:** `realize` gains holes, phrase leaves the server expands by weight, and locks,
leaves that keep their lemma by looking it up instead of drawing a word. A failure names its
leaf's index in the posted tree. The response is signed with HMAC under `BUILD_SECRET`, and a new `POST
/api/v1/sentences` saves a signed tree with `origin = 'built'`. In the browser, pure state
functions in `web/src/lib/builder.ts` drive a `/build` page that reuses `Diagram.svelte` in an
editable mode and a new `ExpansionSheet.svelte`.

**Tech Stack:** Go 1.26 (chi, pgx, sqlc, golang-migrate), SvelteKit with Svelte 5 runes,
TypeScript, Vitest (browser and node projects), Playwright with axe.

**Spec:** `docs/superpowers/specs/2026-10-05-diagram-builder-design.md` (build-order stages 3
to 6). Stages 1 and 2 are built.

## Global Constraints

- Go tests are table-driven where there are cases. API tests run against real Postgres through
  `just test-be`, which loads the root `.env`. Packages without a database run with
  `cd service && go test ./internal/<pkg>/`.
- After changing a `.sql` query file, run `just generate` (needs `sqlc`). After adding a
  migration, run `just migrate-up`, because the e2e tests use the dev database.
- Web: TypeScript, Svelte 5 runes, prettier formatting (`cd web && npx prettier --write src`).
  `just test-fe` runs check, lint, vitest and Playwright.
- Builder state is immutable: functions in `builder.ts` return new objects. The page holds the
  state in `$state.raw` or a writable `$derived`, never deep `$state`, because `structuredClone`
  can't copy a deep `$state` proxy.
- Contrast: `dodgerblue` (`--word`) is for text 24px and up only. Diagram text uses `--text`,
  `--secondary`, `--action`, `--error` and `--word-selected`.
- Interactive targets are at least 44 by 44 CSS px. The editable diagram never scales down
  below full size, so its targets keep their size.
- Axe runs on every page and key open state with no violations allowed.
- Playwright matches names by case-insensitive substring, and sentence words are buttons, so
  locators whose names could be words (`Fill`, `Reroll`, `Undo`, `Keep this one`, `Remix`,
  `Build`) use `exact: true`.
- Commit messages: conventional prefix (`feat:`, `test:`, `docs:`, `refactor:`), sentence case
  after it, and no `Co-Authored-By` trailer.
- Comments and docs: no dates, names or history. American English.

## Decisions this plan makes

The spec leaves these open. Each is the simplest choice that keeps the spec's promises.

- **Leaf indexes count the posted tree.** A 422's `leaf` indexes the leaves of the tree the
  client posted, holes included, in sentence order. That matches `leaves()` in `tree.ts`. A
  failure inside a hole the server expanded names the hole.
- **An empty frame inside a hole gets a fresh expansion**, up to 10 tries, as `Generate`
  already does for whole trees. A failure in a slot the client chose is reported at once.
- **Fixed words and reflexives can't be locked.** Their word comes from the slot (`to`, `with`,
  a comma) or from agreement (`herself`). The builder shows no lock on them, and `realize`
  ignores `locked` there.
- **Locks don't invalidate a fill.** Keep posts the last realize response untouched, so
  toggling a lock leaves Keep enabled. A structure change clears it.
- **The kept tree keeps its echoed `locked` flags.** Remix strips them.

## Review Focus

1. A remixed sentence saved under an older grammar fails `Grammar.Check` (400). The builder
   should explain that and offer Start over, not show a generic failure. Owned by Task 11.
2. Toggling a lock after a fill must leave Keep working, and Keep must save the filled words,
   not the locally changed tree. Owned by Task 5 (unit) and Task 10 (e2e keeps after a lock).
3. A 422 from a slot inside a server-expanded hole must highlight the hole in the builder's
   own tree. Owned by Task 3 (Go index) and Task 6 (the diagram marks a hole as the problem).
4. Pressing Keep twice while the first request is in flight must save one sentence. Owned by
   Task 10.
5. Escape on the expansion sheet must return focus to the phrase that opened it. Owned by
   Task 8.

---

### Task 1: Default `commonness` to 1

`CLAUDE.md`'s roadmap asks for this until a slider exposes the floor, and the builder uses the
API's default.

**Files:**
- Modify: `service/internal/api/handlers.go` (`commonness`)
- Modify: `service/internal/api/handlers_test.go` (`seedWords`, `TestRealizeSentenceRecomputesPostedFeatures`)
- Modify: `README.md` (the `commonness` paragraph)

**Interfaces:**
- Produces: every endpoint taking `commonness` defaults to 1 when it's absent.

- [ ] **Step 1: Write the failing test**

In `handlers_test.go`, change the end of `TestRealizeSentenceRecomputesPostedFeatures`:

```go
	if c := body.Tree.Features.Commonness; c == nil || *c != 1 {
		t.Errorf("commonness: got %v, want the default, 1", c)
	}
```

At the end of `seedWords`, give every seeded content word a frequency that clears the default
floor (`seedRareNoun` still adds a noun without one):

```go
	if _, err := testPool.Exec(ctx, `
		UPDATE nouns SET frequency = 1.5;
		UPDATE verbs SET frequency = 1.5;
		UPDATE adjectives SET frequency = 1.5;
		UPDATE adverbs SET frequency = 1.5`); err != nil {
		t.Fatalf("seed frequencies: %v", err)
	}
```

Add a test that a request without the param skips words below 1:

```go
func TestCommonnessDefaultsToOne(t *testing.T) {
	seedWords(t)
	seedRareNoun(t)
	srv := newTestServer(t)
	defer srv.Close()

	for range 20 {
		var body map[string]any
		decode(t, call(t, srv, http.MethodGet, "/api/v1/words/random?pos=noun", "", nil), http.StatusOK, &body)
		if body["lemma"] != "goose" {
			t.Fatalf("lemma: got %v, want goose, since goffer has no frequency", body["lemma"])
		}
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `just test-be`
Expected: FAIL in `TestCommonnessDefaultsToOne` (`goffer` comes back) and
`TestRealizeSentenceRecomputesPostedFeatures` (`got 0`).

- [ ] **Step 3: Implement**

In `handlers.go`:

```go
// defaultCommonness is the floor when a request names none: it keeps out
// the rarest words, which make the weakest sentences.
const defaultCommonness = 1

// commonness parses the optional commonness query param: content words must
// be at least this common, as a Zipf frequency.
func commonness(r *http.Request) (float64, error) {
	v := r.URL.Query().Get("commonness")
	if v == "" {
		return defaultCommonness, nil
	}
```

(The rest of the function is unchanged.)

In `README.md`, change "`commonness` (0 to 7, default 0)" to "`commonness` (0 to 7, default
1)".

- [ ] **Step 4: Run the tests to verify they pass**

Run: `just test-be`
Expected: PASS. If another test seeds its own words and generates without `commonness`, give
those words a frequency too; don't add `?commonness=0` to its requests.

- [ ] **Step 5: Commit**

```bash
git add service/internal/api/handlers.go service/internal/api/handlers_test.go README.md
git commit -m "feat: Default the commonness floor to 1"
```

---

### Task 2: Holes and locks in the grammar

**Files:**
- Modify: `service/internal/grammar/grammar.go` (`Node`, `check`, new `ExpandHoles`, `Clone`, `Contains`)
- Test: `service/internal/grammar/grammar_test.go`

**Interfaces:**
- Produces:
  - `Node.Locked bool` (json `locked,omitempty`)
  - `func (g *Grammar) ExpandHoles(tree *Node, rng *rand.Rand) error`
  - `func (n *Node) Clone() *Node`
  - `func (n *Node) Contains(m *Node) bool`
  - `Check` accepts a phrase leaf (a hole) and rejects `locked` on a phrase.

- [ ] **Step 1: Write the failing tests**

In `TestCheckRejectsTreesTheGrammarCannotDerive`, delete the `"phrase left empty"` and `"lone
start symbol"` cases (both are holes now) and add:

```go
		{"locked phrase", tn("S", &grammar.Node{Symbol: "NP", Locked: true}, tn("VP", tn("Verb:transitive"), np())), `"NP"`},
```

Add:

```go
func TestCheckAcceptsHoles(t *testing.T) {
	g := mustLoad(t, checkGrammar)
	for _, tree := range []*grammar.Node{
		tn("S"),
		tn("S", tn("NP"), tn("VP", tn("Verb:transitive"), tn("NP"))),
	} {
		if err := g.Check(tree); err != nil {
			t.Errorf("expected no error; got %v", err)
		}
	}
}

func TestExpandHolesExpandsOnlyHoles(t *testing.T) {
	g := mustLoad(t, checkGrammar)
	tree := tn("S", tn("NP"), tn("VP", &grammar.Node{Symbol: "Verb:transitive", Lemma: "devour", Locked: true}, tn("NP", tn("Pronoun"))))

	if err := g.ExpandHoles(tree, rand.New(rand.NewPCG(1, 2))); err != nil {
		t.Fatalf("ExpandHoles: %v", err)
	}

	if err := g.Check(tree); err != nil {
		t.Errorf("expanded tree: %v", err)
	}
	if len(tree.Children[0].Children) == 0 {
		t.Error("the NP hole has no children")
	}
	if v := tree.Children[1].Children[0]; v.Lemma != "devour" || !v.Locked {
		t.Errorf("the locked verb changed: %+v", v)
	}
	if obj := tree.Children[1].Children[1]; len(obj.Children) != 1 || obj.Children[0].Symbol != "Pronoun" {
		t.Errorf("the filled NP changed: %+v", obj)
	}
}

func TestExpandHolesFailsOnRunawayGrammar(t *testing.T) {
	g := mustLoad(t, `
	[[rule]]
	symbol = "S"
	expansion = ["S", "S"]
	weight = 10

	[[rule]]
	symbol = "S"
	expansion = ["Noun"]
	`)

	if err := g.ExpandHoles(tn("S"), rand.New(rand.NewPCG(1, 2))); err == nil {
		t.Fatal("expected an error; got nil")
	}
}

func TestCloneCopiesEveryNode(t *testing.T) {
	tree := tn("S", tn("NP", &grammar.Node{Symbol: "Noun", Lemma: "goose", Locked: true}))

	c := tree.Clone()
	c.Children[0].Children[0].Lemma = "moon"
	c.Children[0].Children = append(c.Children[0].Children, tn("Noun"))

	if !reflect.DeepEqual(tree, tn("S", tn("NP", &grammar.Node{Symbol: "Noun", Lemma: "goose", Locked: true}))) {
		t.Errorf("changing the clone changed the original: %+v", tree.Children[0])
	}
}

func TestContains(t *testing.T) {
	noun := tn("Noun")
	np := tn("NP", noun)
	tree := tn("S", np, tn("Verb"))

	if !tree.Contains(noun) || !np.Contains(np) || np.Contains(tree.Children[1]) {
		t.Error("Contains: want true for a descendant and the node itself, false for a sibling's")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd service && go test ./internal/grammar/`
Expected: compile failure: `Locked`, `ExpandHoles`, `Clone` and `Contains` undefined.

- [ ] **Step 3: Implement**

In `Node`, after `Display`:

```go
	// Locked keeps a leaf's lemma when the tree is realized again.
	Locked   bool     `json:"locked,omitempty"`
```

Update the `Node` doc comment's last sentence block to mention holes:

```go
// Node is one constituent of a parse tree. A leaf's Symbol is a POS,
// optionally qualified ("Verb:transitive"), or a phrase with no children
// yet, a hole. Once the leaf is filled from the lexicon, Lemma is the
// dictionary form and Word the inflected one. Display is how the leaf is
// written in the sentence when that differs from Word: a separable verb
// split around its object ("looked" and "her up").
```

Replace `Check`'s doc comment and `check`:

```go
// Check reports whether tree derives from the grammar: the root is the
// start symbol, every inner node's children spell one of its symbol's
// rules, and every leaf is a part of speech or a hole. Only words can be
// locked.
func (g *Grammar) Check(tree *Node) error {
	if tree.Symbol != start {
		return fmt.Errorf("grammar: root is %q, not %q", tree.Symbol, start)
	}
	return g.check(tree)
}

func (g *Grammar) check(n *Node) error {
	_, phrase := g.rules[n.Symbol]
	if phrase && n.Locked {
		return fmt.Errorf("grammar: %q is a phrase, and only words can be locked", n.Symbol)
	}
	if len(n.Children) == 0 {
		// Rules only spell valid terminals, so a leaf that matched its
		// parent's rule is a part of speech or a hole.
		return nil
	}
	if !phrase {
		return fmt.Errorf("grammar: %q is a part of speech and cannot have children", n.Symbol)
	}
	rules := g.rules[n.Symbol]
```

(The rest of `check`, from `symbols := make(...)`, is unchanged.)

After `Expand`'s helpers (below `pick`), add:

```go
// ExpandHoles expands every hole in tree, choosing among rules by weight as
// Expand does. Nodes that aren't holes stay as they are.
func (g *Grammar) ExpandHoles(tree *Node, rng *rand.Rand) error {
	return g.expandHoles(tree, 0, rng)
}

func (g *Grammar) expandHoles(n *Node, depth int, rng *rand.Rand) error {
	if len(n.Children) == 0 {
		if _, phrase := g.rules[n.Symbol]; !phrase {
			return nil
		}
		expanded, err := g.expand(n.Symbol, depth, rng)
		if err != nil {
			return err
		}
		n.Children = expanded.Children
		return nil
	}
	for _, c := range n.Children {
		if err := g.expandHoles(c, depth+1, rng); err != nil {
			return err
		}
	}
	return nil
}
```

After `LeafNodes`, add:

```go
// Clone copies the tree, so changing the copy's nodes leaves n's alone.
func (n *Node) Clone() *Node {
	c := *n
	if n.Children != nil {
		c.Children = make([]*Node, len(n.Children))
		for i, child := range n.Children {
			c.Children[i] = child.Clone()
		}
	}
	return &c
}

// Contains reports whether m is n or a node under it.
func (n *Node) Contains(m *Node) bool {
	return n == m || slices.ContainsFunc(n.Children, func(c *Node) bool { return c.Contains(m) })
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `cd service && go test ./internal/grammar/`
Expected: PASS.

`just test-be` now fails `TestRealizeSentenceRejectsBadRequests/phrase_left_empty` (a 500, since
`realize` can't fill a hole yet). Task 3 makes it a 200, so leave it.

- [ ] **Step 5: Commit**

```bash
git add service/internal/grammar
git commit -m "feat: Accept holes and word locks in grammar trees"
```

---

### Task 3: `realize` fills holes and names the failing leaf

**Files:**
- Modify: `service/internal/sentence/sentence.go` (`Realize`, `Generate`, `fillLeaf`, `agreeWithSubjects`, new `LeafError`, `slotError`, `Text`)
- Modify: `service/internal/api/handlers.go` (`realizeSentence`)
- Modify: `README.md` (the `realize` paragraphs)
- Test: `service/internal/sentence/sentence_test.go`, `service/internal/api/handlers_test.go`

**Interfaces:**
- Consumes: `ExpandHoles`, `Clone`, `Contains` (Task 2).
- Produces:
  - `func Realize(ctx context.Context, q store.Querier, g *grammar.Grammar, tree *grammar.Node, verbs *morph.Verbs, rng *rand.Rand, commonness float64) (*Sentence, error)`. It realizes a copy of `tree`, leaving `tree` unchanged.
  - `type LeafError struct { Index int; Err error }` with `Error()` and `Unwrap()`
  - `func Text(tree *grammar.Node) string`
  - A 422 from `realize` is `{"error": string, "leaf": int}`.

- [ ] **Step 1: Write the failing tests**

In `sentence_test.go`, add:

```go
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
```

In `handlers_test.go`, add two rules to `realizeGrammar`:

```toml
[[rule]]
symbol = "S"
expansion = ["NP", "Gift"]

[[rule]]
symbol = "Gift"
expansion = ["Verb:ditransitive", "NP", "NP"]
```

In `TestRealizeSentenceRejectsBadRequests`, delete the `"phrase left empty"` case. Add:

```go
// leafProblem is a 422 body from realize.
type leafProblem struct {
	Error string `json:"error"`
	Leaf  int    `json:"leaf"`
}

func TestRealizeSentenceFillsHoles(t *testing.T) {
	seedWords(t)
	srv := newRealizeServer(t)
	defer srv.Close()

	resp := postTree(t, srv, "", `{"symbol": "S", "children": [{"symbol": "NP"}, {"symbol": "VP"}]}`)
	defer resp.Body.Close()
	var body struct {
		Text string       `json:"text"`
		Tree grammar.Node `json:"tree"`
	}
	decode(t, resp, http.StatusOK, &body)

	if !regexp.MustCompile(`^This goose (devours|devoured)( this goose)?\.$`).MatchString(body.Text) {
		t.Errorf("text: got %q", body.Text)
	}
	if len(body.Tree.Children[0].Children) != 2 || len(body.Tree.Children[1].Children) == 0 {
		t.Errorf("holes left in the tree: %+v", body.Tree)
	}
}

func TestRealizeSentenceNamesTheLeafNoWordFits(t *testing.T) {
	seedWords(t)
	srv := newRealizeServer(t)
	defer srv.Close()

	tests := []struct {
		name, tree string
		leaf       int
	}{
		{"a slot", `{"symbol": "S", "children": [{"symbol": "Verb:ditransitive"}]}`, 0},
		{"a hole, after it is expanded afresh", `{"symbol": "S", "children": [{"symbol": "NP"}, {"symbol": "Gift"}]}`, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp := postTree(t, srv, "", tc.tree)
			defer resp.Body.Close()
			var body leafProblem
			decode(t, resp, http.StatusUnprocessableEntity, &body)
			if body.Leaf != tc.leaf || body.Error == "" {
				t.Errorf("body: got %+v, want leaf %d and an error", body, tc.leaf)
			}
		})
	}
}
```

Add `"regexp"` to the imports.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd service && go test ./internal/sentence/`
Expected: compile failure: `Realize` takes too few arguments; `LeafError` and `Text` undefined.

- [ ] **Step 3: Implement `sentence.go`**

Below `errEmptyFrame`, add:

```go
// LeafError is a leaf no word could fill. Index counts the posted tree's
// leaves from 0 in sentence order, holes included, so a slot inside a hole
// the server expanded is reported as the hole.
type LeafError struct {
	Index int
	Err   error
}

func (e *LeafError) Error() string { return fmt.Sprintf("leaf %d: %v", e.Index, e.Err) }

func (e *LeafError) Unwrap() error { return e.Err }

// slotError is a leaf of the tree being realized that no word fits.
type slotError struct {
	leaf *grammar.Node
	err  error
}

func (e *slotError) Error() string { return fmt.Sprintf("%s: %v", e.leaf.Symbol, e.err) }

func (e *slotError) Unwrap() error { return e.err }
```

In `Generate`, change the `Realize` call to pass the grammar:

```go
		s, err = Realize(ctx, q, g, tree, verbs, rng, commonness)
```

Replace `Realize` with an exported wrapper that expands holes and an unexported `realize` holding
the old body:

```go
// Realize fills a copy of tree with words, after expanding each hole by
// weight, and inflects them: nouns agree with their determiner, verbs with
// their subject, all in one tense chosen at random. Content words are at
// least as common as commonness. A hole that needed a verb frame with no
// verbs at that floor is expanded afresh. A leaf no word fits is a
// *LeafError.
func Realize(ctx context.Context, q store.Querier, g *grammar.Grammar, tree *grammar.Node, verbs *morph.Verbs, rng *rand.Rand, commonness float64) (*Sentence, error) {
	var leafErr *LeafError
	for range maxAttempts {
		t := tree.Clone()
		posted := t.LeafNodes()
		if err := g.ExpandHoles(t, rng); err != nil {
			return nil, fmt.Errorf("Realize: %w", err)
		}
		s, err := realize(ctx, q, t, verbs, rng, commonness)
		var slot *slotError
		if !errors.As(err, &slot) {
			if err != nil {
				return nil, fmt.Errorf("Realize: %w", err)
			}
			return s, nil
		}
		i := slices.IndexFunc(posted, func(p *grammar.Node) bool { return p.Contains(slot.leaf) })
		leafErr = &LeafError{Index: i, Err: slot.err}
		// A hole has children once expanded; a posted slot never does.
		if len(posted[i].Children) == 0 || !errors.Is(slot.err, errEmptyFrame) {
			break
		}
	}
	return nil, fmt.Errorf("Realize: %w", leafErr)
}

func realize(ctx context.Context, q store.Querier, tree *grammar.Node, verbs *morph.Verbs, rng *rand.Rand, commonness float64) (*Sentence, error) {
	gen := &generator{
```

In the body of `realize` (the old `Realize`), the two `return nil, fmt.Errorf("Realize: %w", err)`
become `return nil, err`, and the text-building tail becomes:

```go
	text := map[*grammar.Node]string{}
	gen.separateParticles(tree, text)
	leaves := tree.LeafNodes()
	for i, l := range leaves {
		if l.Lemma == "a" && l.POS() == grammar.Determiner && i+1 < len(leaves) {
			l.Word = morph.Article(leaves[i+1].Word)
		}
		if t, ok := text[l]; ok {
			l.Display = t
		}
	}

	return &Sentence{Text: Text(tree), Tree: tree}, nil
}

// Text writes out a realized tree: each leaf's display if it has one, or
// else its word.
func Text(tree *grammar.Node) string {
	leaves := tree.LeafNodes()
	words := make([]string, len(leaves))
	for i, l := range leaves {
		words[i] = cmp.Or(l.Display, l.Word)
	}
	return format(words)
}
```

Add `"cmp"` and `"slices"` to the imports.

In `fillLeaf`, wrap a missing word as a `slotError`:

```go
func (gen *generator) fillLeaf(n *grammar.Node, pluralNoun, subject bool) error {
	lemma, info, err := gen.randomWord(n, pluralNoun, subject)
	if errors.Is(err, pgx.ErrNoRows) {
		return &slotError{leaf: n, err: err}
	}
	if err != nil {
		return fmt.Errorf("%s: %w", n.Symbol, err)
	}
```

In `agreeWithSubjects`, the reflexive lookup's error return becomes:

```go
			if errors.Is(err, pgx.ErrNoRows) {
				return &slotError{leaf: c, err: err}
			}
			if err != nil {
				return fmt.Errorf("%s: %w", c.Symbol, err)
			}
```

- [ ] **Step 4: Implement the handler**

In `handlers.go`, `realizeSentence`'s doc comment and its realize call:

```go
// realizeSentence fills a posted tree, in the shape randomSentence returns,
// with words. The tree must derive from the grammar, and may have holes,
// which are expanded first. Any words and features already in it are
// replaced. When no word fits a slot, the 422 names the slot's leaf.
```

```go
	s, err := sentence.Realize(r.Context(), h.queries, h.grammar, &tree, h.verbs, rng, c)
	var leafErr *sentence.LeafError
	if errors.As(err, &leafErr) {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"error": "no word in the lexicon fits this slot at this commonness",
			"leaf":  leafErr.Index,
		})
		return
	}
```

Remove the `errors.Is(err, pgx.ErrNoRows)` branch it replaces, and the `pgx` import if nothing
else in the file uses it.

- [ ] **Step 5: Update the README**

In the `realize` paragraph, replace the last two sentences ("Bodies are capped ... returns 422.")
with:

```markdown
A phrase with no children is a hole, and `realize` expands it by weight before filling. Bodies
are capped at 64 KiB. A slot no word fits, such as a frame with no verbs above the floor,
returns 422 with `leaf`, the slot's index among the posted tree's leaves (holes included), in
sentence order. A frame left empty inside a hole gets a fresh expansion first.
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `cd service && go test ./internal/sentence/ && cd .. && just test-be`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add service/internal/sentence service/internal/api README.md
git commit -m "feat: Fill holes in realize and name the leaf no word fits"
```

---

### Task 4: Locks in `realize`

**Files:**
- Modify: `service/internal/store/queries/{nouns,verbs,adjectives,adverbs,determiners,prepositions,pronouns,conjunctions}.sql`
- Regenerate: `service/internal/store/*.sql.go`, `querier.go`
- Modify: `service/internal/sentence/sentence.go` (`fill`, `fillLeaf`, `randomWord` renamed `chooseWord`, new `ErrLockMismatch`)
- Modify: `service/internal/api/handlers.go` (`clearWords`, the 422 message)
- Modify: `README.md`
- Test: `service/internal/sentence/sentence_test.go`, `service/internal/api/handlers_test.go`

**Interfaces:**
- Consumes: `Node.Locked`, `Check` rejecting locked phrases (Task 2); `LeafError`, `slotError` (Task 3).
- Produces:
  - `var ErrLockMismatch = errors.New("the locked word doesn't fit this slot")`
  - Queries: `LookupNoun(ctx, lemma string) (Noun, error)`, `GetRandomSingularNoun(ctx, commonness float64) (Noun, error)`, `LookupVerb(ctx, LookupVerbParams{Lemma, Frame string}) (Verb, error)`, `LookupAdjective(ctx, lemma)`, `LookupAdverb(ctx, lemma)`, `LookupDeterminer(ctx, lemma)`, `LookupPreposition(ctx, lemma)`, `LookupPronoun(ctx, LookupPronounParams{Lemma, Case string})`, `LookupConjunction(ctx, LookupConjunctionParams{Lemma, Type string; JoinsNps bool})`. Check the generated field names in the `.sql.go` files after `just generate` and use those.

- [ ] **Step 1: Add the queries**

`nouns.sql`, after `GetRandomNoun`:

```sql
-- name: GetRandomSingularNoun :one
-- For a locked singular determiner, which can't go with "Rastas".
SELECT * FROM nouns
WHERE active AND NOT plural AND coalesce(frequency, 0) >= @commonness::float8
ORDER BY random()
LIMIT 1;

-- name: LookupNoun :one
-- A locked word: the floor doesn't apply.
SELECT * FROM nouns
WHERE active AND lemma = $1
ORDER BY id
LIMIT 1;
```

`verbs.sql`:

```sql
-- name: LookupVerb :one
-- A locked word: the floor doesn't apply. An empty frame matches any verb.
SELECT * FROM verbs
WHERE active AND lemma = @lemma AND (@frame::text = '' OR frames ? @frame::text)
ORDER BY id
LIMIT 1;
```

`adjectives.sql` and `adverbs.sql` (with `adverbs` and `LookupAdverb` in the second):

```sql
-- name: LookupAdjective :one
-- A locked word: the floor doesn't apply.
SELECT * FROM adjectives
WHERE active AND lemma = $1
ORDER BY id
LIMIT 1;
```

`determiners.sql` and `prepositions.sql` (lemmas are unique there):

```sql
-- name: LookupDeterminer :one
SELECT * FROM determiners
WHERE active AND lemma = $1;
```

```sql
-- name: LookupPreposition :one
SELECT * FROM prepositions
WHERE active AND lemma = $1;
```

`pronouns.sql`:

```sql
-- name: LookupPronoun :one
-- "you" is singular and plural, so either can come back.
SELECT * FROM pronouns
WHERE active AND lemma = @lemma AND case_ = @case_
ORDER BY random()
LIMIT 1;
```

`conjunctions.sql`:

```sql
-- name: LookupConjunction :one
-- An empty type matches either; joins_nps requires a conjunction that can
-- join noun phrases.
SELECT * FROM conjunctions
WHERE active AND lemma = @lemma AND (@type::text = '' OR type = @type::text)
  AND (joins_nps OR NOT @joins_nps::bool)
ORDER BY id
LIMIT 1;
```

Run: `just generate && cd service && go build ./...`
Expected: builds.

- [ ] **Step 2: Write the failing tests**

In `sentence_test.go`, add fields to `fakeQuerier`:

```go
	// lookedUp records the lemmas every Lookup query was asked for. Each
	// finds any lemma but "nope". LookupDeterminer gives "a" singular
	// number and LookupNoun makes "Rastas" a plural lemma.
	lookedUp []string
	// singularNouns counts GetRandomSingularNoun calls.
	singularNouns int
```

and these methods:

```go
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
```

and the tests:

```go
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
```

Add `"fmt"` to the imports if it isn't there.

In `handlers_test.go`, add to `TestRealizeSentenceRejectsBadRequests`'s table:

```go
		{"locked phrase", "", `{"symbol": "S", "children": [{"symbol": "NP", "locked": true}, {"symbol": "VP", "children": [{"symbol": "Verb"}]}]}`, http.StatusBadRequest},
```

and add:

```go
func TestRealizeSentenceKeepsLockedLemmas(t *testing.T) {
	seedWords(t)
	seedRareNoun(t)
	mustExec(t, "INSERT INTO determiners (lemma, type, number) VALUES ('these', 'demonstrative', 'plural')")
	srv := newRealizeServer(t)
	defer srv.Close()

	// goffer has no frequency, so it clears the default floor only because
	// it's locked. "these" makes it plural.
	resp := postTree(t, srv, "", `{"symbol": "S", "children": [
		{"symbol": "NP", "children": [
			{"symbol": "Determiner", "lemma": "these", "locked": true},
			{"symbol": "Noun", "lemma": "goffer", "word": "stale", "locked": true}
		]},
		{"symbol": "VP", "children": [{"symbol": "Verb"}]}
	]}`)
	defer resp.Body.Close()
	var body struct {
		Text string       `json:"text"`
		Tree grammar.Node `json:"tree"`
	}
	decode(t, resp, http.StatusOK, &body)

	if body.Text != "These goffers devour." && body.Text != "These goffers devoured." {
		t.Errorf("text: got %q, want These goffers devour/devoured", body.Text)
	}
	if noun := body.Tree.Children[0].Children[1]; noun.Lemma != "goffer" || !noun.Locked {
		t.Errorf("noun: got %+v, want goffer, still locked", noun)
	}
}

func TestRealizeSentenceRejectsALockThatDoesNotFit(t *testing.T) {
	seedWords(t)
	srv := newRealizeServer(t)
	defer srv.Close()

	resp := postTree(t, srv, "", `{"symbol": "S", "children": [
		{"symbol": "NP", "children": [{"symbol": "Determiner"}, {"symbol": "Noun", "lemma": "devour", "locked": true}]},
		{"symbol": "VP", "children": [{"symbol": "Verb"}]}
	]}`)
	defer resp.Body.Close()
	var body leafProblem
	decode(t, resp, http.StatusUnprocessableEntity, &body)

	if body.Leaf != 1 || !strings.Contains(body.Error, "locked") {
		t.Errorf("body: got %+v, want leaf 1 and an error about the locked word", body)
	}
}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `cd service && go test ./internal/sentence/`
Expected: FAIL: locked leaves get random words, and `sentence.ErrLockMismatch` is undefined
(compile failure first).

- [ ] **Step 4: Implement `sentence.go`**

Below `errEmptyFrame`:

```go
// ErrLockMismatch is a locked lemma that doesn't fit its slot: a word the
// slot's table lacks, a verb without the slot's frame, or a plural lemma
// after a locked singular determiner.
var ErrLockMismatch = errors.New("the locked word doesn't fit this slot")
```

Replace `fill`'s doc comment and its NP block, and change its skip test:

```go
// fill gives every leaf under n a word, except reflexives, which wait for
// agreeWithSubjects. A locked leaf keeps its lemma. subject says n is (part
// of) a subject NP. An NP fills a locked determiner first, so that a
// singular one ("a") keeps plural lemmas ("Rastas") off its nouns, then its
// nouns, so that a plural lemma can rule out singular determiners.
func (gen *generator) fill(n *grammar.Node, subject bool) error {
	pluralNoun := false
	if n.Symbol == nounPhrase {
		var singular *grammar.Node
		for _, c := range n.Children {
			if c.POS() == grammar.Determiner && c.Locked {
				if err := gen.fillLeaf(c, false, false, false); err != nil {
					return err
				}
				if gen.leaves[c].number == "singular" {
					singular = c
				}
			}
		}
		for _, c := range n.Children {
			if c.POS() == grammar.Noun {
				if err := gen.fillLeaf(c, false, singular != nil, false); err != nil {
					return err
				}
				pluralNoun = pluralNoun || gen.leaves[c].pluralLemma
			}
		}
		// Only a locked plural lemma gets past a singular determiner.
		if pluralNoun && singular != nil {
			return &slotError{leaf: singular, err: ErrLockMismatch}
		}
	}
	for i, c := range n.Children {
		if _, filled := gen.leaves[c]; filled || isReflexive(c) {
			continue
		}
		if len(c.Children) == 0 {
			if err := gen.fillLeaf(c, pluralNoun, false, subject); err != nil {
				return err
			}
			continue
		}
```

(The rest of the loop is unchanged.)

Replace `fillLeaf`:

```go
// fillLeaf gives n a word, its locked lemma's if it's locked.
// singularNoun restricts a noun to lemmas that aren't plural.
func (gen *generator) fillLeaf(n *grammar.Node, pluralNoun, singularNoun, subject bool) error {
	lemma, info, err := gen.chooseWord(n, pluralNoun, singularNoun, subject)
	if errors.Is(err, pgx.ErrNoRows) {
		if n.Locked {
			err = ErrLockMismatch
		}
		return &slotError{leaf: n, err: err}
	}
	if err != nil {
		return fmt.Errorf("%s: %w", n.Symbol, err)
	}
	n.Lemma, n.Word = lemma, lemma
	n.Features = info.features
	gen.leaves[n] = info
	return nil
}
```

Rename `randomWord` to `chooseWord`, give it the new parameter, and look locked lemmas up.
Its doc comment and the changed cases:

```go
// chooseWord picks a word for leaf n, honoring its qualifier: its locked
// lemma, looked up without the commonness floor, or a random one. Fixed
// words ("to", "with", a comma) ignore a lock. pluralNoun restricts a
// determiner to ones that can go with a plural noun; singularNoun restricts
// a noun to lemmas that aren't plural; subject makes a pronoun nominative
// rather than accusative.
func (gen *generator) chooseWord(n *grammar.Node, pluralNoun, singularNoun, subject bool) (string, leafInfo, error) {
	ctx, q := gen.ctx, gen.q
	switch n.POS() {
	case grammar.Noun:
		var w store.Noun
		var err error
		switch {
		case n.Locked:
			w, err = q.LookupNoun(ctx, n.Lemma)
		case singularNoun:
			w, err = q.GetRandomSingularNoun(ctx, gen.commonness)
		default:
			w, err = q.GetRandomNoun(ctx, gen.commonness)
		}
		if err != nil {
			return "", leafInfo{}, err
		}
```

(the rest of the noun case is unchanged)

```go
	case grammar.Verb:
		var w store.Verb
		var err error
		switch frame := n.Qualifier(); {
		case n.Locked:
			w, err = q.LookupVerb(ctx, store.LookupVerbParams{Lemma: n.Lemma, Frame: frame})
		case frame != "":
			w, err = q.GetRandomVerbWithFrame(ctx, store.GetRandomVerbWithFrameParams{Frame: frame, Commonness: gen.commonness})
			if errors.Is(err, pgx.ErrNoRows) {
				err = fmt.Errorf("%w: %w", errEmptyFrame, err)
			}
		default:
			w, err = q.GetRandomVerb(ctx, gen.commonness)
		}
```

```go
	case grammar.Adjective:
		var w store.Adjective
		var err error
		if n.Locked {
			w, err = q.LookupAdjective(ctx, n.Lemma)
		} else {
			w, err = q.GetRandomAdjective(ctx, gen.commonness)
		}
		return w.Lemma, leafInfo{features: grammar.Features{Frequency: frequency(w.Frequency)}}, err
	case grammar.Adverb:
		var w store.Adverb
		var err error
		if n.Locked {
			w, err = q.LookupAdverb(ctx, n.Lemma)
		} else {
			w, err = q.GetRandomAdverb(ctx, gen.commonness)
		}
		return w.Lemma, leafInfo{features: grammar.Features{Frequency: frequency(w.Frequency)}}, err
	case grammar.Determiner:
		var w store.Determiner
		var err error
		switch {
		case n.Locked:
			w, err = q.LookupDeterminer(ctx, n.Lemma)
		case pluralNoun:
			w, err = q.GetRandomDeterminerWithNumber(ctx, []string{"plural", "either"})
		default:
			w, err = q.GetRandomDeterminer(ctx)
		}
		return w.Lemma, leafInfo{number: w.Number, features: grammar.Features{Type: w.Type, Number: w.Number}}, err
	case grammar.Preposition:
		if prep := n.Qualifier(); prep != "" {
			return prep, leafInfo{}, nil
		}
		var w store.Preposition
		var err error
		if n.Locked {
			w, err = q.LookupPreposition(ctx, n.Lemma)
		} else {
			w, err = q.GetRandomPreposition(ctx)
		}
		return w.Lemma, leafInfo{}, err
```

In the pronoun case, a small helper picks lookup or draw for both the genitive and the plain
pronoun:

```go
	case grammar.Pronoun:
		pronounCase := "accusative"
		if subject {
			pronounCase = "nominative"
		}
		pronoun := func(c string) (store.Pronoun, error) {
			if n.Locked {
				return q.LookupPronoun(ctx, store.LookupPronounParams{Lemma: n.Lemma, Case: c})
			}
			return q.GetRandomPronounWithCase(ctx, c)
		}
		if n.Qualifier() == grammar.Genitive {
			// "mine" stands for what is owned, not the owner, so it's third
			// person of either number.
			w, err := pronoun(grammar.Genitive)
```

(the genitive branch continues unchanged)

```go
		w, err := pronoun(pronounCase)
		return w.Lemma, leafInfo{number: w.Number, person: morph.Person(w.Person), gender: w.Gender, features: grammar.Features{
```

In the conjunction default case:

```go
		switch qualifier := n.Qualifier(); {
		case qualifier == grammar.Neither || qualifier == grammar.Nor:
			return qualifier, leafInfo{}, nil
		case n.Locked:
			lookup := store.LookupConjunctionParams{Lemma: n.Lemma, JoinsNps: qualifier == grammar.JoinsNPs}
			if !lookup.JoinsNps {
				lookup.Type = qualifier
			}
			w, err = q.LookupConjunction(ctx, lookup)
		case qualifier == grammar.JoinsNPs:
			w, err = q.GetRandomNPConjunction(ctx)
		case qualifier == "":
			w, err = q.GetRandomConjunction(ctx)
		default:
			w, err = q.GetRandomConjunctionOfType(ctx, qualifier)
		}
		return w.Lemma, leafInfo{}, err
```

`Pronoun:it`, `Comma`, `Complementizer` and `To` return before any lookup, as they do now.
Reflexives are skipped by `fill` and filled by agreement, so their lock is ignored too.

- [ ] **Step 5: Implement the handler**

In `handlers.go`, give the lock its own message and keep locked lemmas through `clearWords`:

```go
	if errors.As(err, &leafErr) {
		msg := "no word in the lexicon fits this slot at this commonness"
		if errors.Is(leafErr, sentence.ErrLockMismatch) {
			msg = "the locked word doesn't fit this slot"
		}
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": msg, "leaf": leafErr.Index})
		return
	}
```

```go
// clearWords empties every word and feature, except a locked leaf's lemma.
func clearWords(n *grammar.Node) {
	if !n.Locked {
		n.Lemma = ""
	}
	n.Word, n.Display, n.Features = "", "", grammar.Features{}
	for _, c := range n.Children {
		clearWords(c)
	}
}
```

Update `realizeSentence`'s doc comment: "Any words and features already in it are replaced,
except the lemmas of locked leaves."

- [ ] **Step 6: Update the README**

After the hole paragraph from Task 3, add:

```markdown
A leaf with `"locked": true` keeps its `lemma` and is inflected again to agree, so a locked
noun still follows its determiner. It must fit its slot, as a verb with the slot's frame or a
pronoun of the slot's case, or the 422 says so. The commonness floor doesn't apply to it.
Fixed words (`to`, a qualified preposition, a comma) and reflexives ignore a lock. A locked
singular determiner keeps plural-only nouns ("Rastas") out of its noun phrase.
```

- [ ] **Step 7: Run the tests to verify they pass**

Run: `cd service && go test ./internal/sentence/ && cd .. && just test-be`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add service/internal README.md
git commit -m "feat: Keep locked lemmas when realize fills a tree"
```

---

### Task 5: Builder state functions

**Files:**
- Create: `web/src/lib/builder.ts`
- Modify: `web/src/lib/types.ts` (`TreeNode.locked`)
- Test: `web/src/lib/builder.spec.ts`

**Interfaces:**
- Produces:
  - `interface Realized { text: string; tree: TreeNode }`
  - `interface Draft { tree: TreeNode; filled: Realized | null }`
  - `interface Builder { draft: Draft; undo: Draft[] }`
  - `start(grammar: Grammar): Builder`
  - `nodeAt(tree: TreeNode, path: number[]): TreeNode`
  - `choose(b: Builder, path: number[], rule: string[]): Builder`
  - `clear(b: Builder, path: number[]): Builder`
  - `toggleLock(b: Builder, path: number[]): Builder`
  - `fill(b: Builder, realized: Realized): Builder`
  - `undo(b: Builder): Builder`
  - `lockable(node: TreeNode): boolean`
  - `draftText(tree: TreeNode): string`

- [ ] **Step 1: Add `locked` to `TreeNode`**

In `types.ts`, after `display`:

```ts
	// Keeps the leaf's lemma when the tree is filled again.
	locked?: boolean;
```

- [ ] **Step 2: Write the failing tests**

`web/src/lib/builder.spec.ts`:

```ts
import { describe, expect, it } from 'vitest';
import {
	choose,
	clear,
	draftText,
	fill,
	lockable,
	nodeAt,
	start,
	toggleLock,
	undo,
	type Realized
} from './builder';
import { grammar, sentence } from './testing/fixtures';

const realized: Realized = { text: sentence.text, tree: sentence.tree };

describe('builder', () => {
	it('starts from a lone start hole with nothing to undo', () => {
		const b = start(grammar);
		expect(b.draft).toEqual({ tree: { symbol: 'S' }, filled: null });
		expect(b.undo).toEqual([]);
	});

	it('expands a hole into the rule’s symbols, and undoes it', () => {
		const b = choose(start(grammar), [], ['Clause']);
		expect(b.draft.tree).toEqual({ symbol: 'S', children: [{ symbol: 'Clause' }] });
		expect(undo(b).draft.tree).toEqual({ symbol: 'S' });
	});

	it('changes a filled phrase, dropping its words, its locks and the fill', () => {
		const locked = toggleLock(fill(start(grammar), realized), [0, 0, 1]);
		const b = choose(locked, [0, 0], ['Pronoun']);
		expect(nodeAt(b.draft.tree, [0, 0])).toEqual({ symbol: 'NP', children: [{ symbol: 'Pronoun' }] });
		expect(nodeAt(b.draft.tree, [0, 1, 0]).word).toBe('devoured');
		expect(b.draft.filled).toBeNull();
	});

	it('clears a phrase back to a hole', () => {
		const b = clear(fill(start(grammar), realized), [0, 1]);
		expect(nodeAt(b.draft.tree, [0, 1])).toEqual({ symbol: 'VP' });
		expect(b.draft.filled).toBeNull();
	});

	it('toggles a lock without losing the fill', () => {
		const filled = fill(start(grammar), realized);
		const once = toggleLock(filled, [0, 0, 1]);
		expect(nodeAt(once.draft.tree, [0, 0, 1]).locked).toBe(true);
		expect(once.draft.filled).toBe(realized);
		expect(nodeAt(toggleLock(once, [0, 0, 1]).draft.tree, [0, 0, 1]).locked).toBeUndefined();
	});

	it('fills with a copy, so locking never changes what Keep would post', () => {
		const b = toggleLock(fill(start(grammar), realized), [0, 0, 1]);
		expect(nodeAt(realized.tree, [0, 0, 1]).locked).toBeUndefined();
		expect(b.draft.filled!.tree).toBe(realized.tree);
	});

	it('undoes a fill back to the unfilled draft', () => {
		const b = choose(start(grammar), [], ['Clause']);
		expect(undo(fill(b, realized)).draft).toBe(b.draft);
	});

	it('leaves the state alone when there is nothing to undo', () => {
		const b = start(grammar);
		expect(undo(b)).toBe(b);
	});

	it.each([
		['Noun', true],
		['Verb:transitive', true],
		['Preposition', true],
		['Pronoun', true],
		['Conjunction:coordinating', true],
		['Comma', false],
		['To', false],
		['Complementizer:whether', false],
		['Preposition:with', false],
		['Pronoun:it', false],
		['Pronoun:reflexive', false],
		['Conjunction:nor', false]
	])('says whether %s can be locked: %s', (symbol, want) => {
		expect(lockable({ symbol })).toBe(want);
	});

	it('writes out a draft with every word, and nothing while a slot is empty', () => {
		expect(draftText(sentence.tree)).toBe('The goose devoured her, but she sang.');
		expect(draftText(choose(start(grammar), [], ['Clause']).draft.tree)).toBe('');
	});
});
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `cd web && npx vitest --run --project server src/lib/builder.spec.ts`
Expected: FAIL, `./builder` can't be resolved.

- [ ] **Step 4: Implement**

`web/src/lib/builder.ts`:

```ts
import { leaves, pos, tokens } from './tree';
import type { Grammar, TreeNode } from './types';

// Realized is what realize returns for a draft.
export interface Realized {
	text: string;
	tree: TreeNode;
}

// Draft is the tree being built, and the last fill if only locks have
// changed since.
export interface Draft {
	tree: TreeNode;
	filled: Realized | null;
}

// Builder is the draft and every earlier draft, latest last. Functions here
// return new state and never change what they're given.
export interface Builder {
	draft: Draft;
	undo: Draft[];
}

export function start(grammar: Grammar): Builder {
	return { draft: { tree: { symbol: grammar.start }, filled: null }, undo: [] };
}

export function nodeAt(tree: TreeNode, path: number[]): TreeNode {
	return path.reduce((node, i) => node.children![i], tree);
}

// edit changes a copy of the draft's tree, keeping the draft for undo. Only
// a change that leaves the words alone keeps the fill.
function edit(b: Builder, change: (tree: TreeNode) => void, keepsFill = false): Builder {
	const tree = structuredClone(b.draft.tree);
	change(tree);
	return {
		draft: { tree, filled: keepsFill ? b.draft.filled : null },
		undo: [...b.undo, b.draft]
	};
}

// choose gives the phrase at path the rule's symbols as children: phrases
// become holes and parts of speech empty slots. Whatever was under it goes,
// locks and all.
export function choose(b: Builder, path: number[], rule: string[]): Builder {
	return edit(b, (tree) => {
		nodeAt(tree, path).children = rule.map((symbol) => ({ symbol }));
	});
}

// clear empties the phrase at path back to a hole.
export function clear(b: Builder, path: number[]): Builder {
	return edit(b, (tree) => {
		delete nodeAt(tree, path).children;
	});
}

export function toggleLock(b: Builder, path: number[]): Builder {
	return edit(
		b,
		(tree) => {
			const node = nodeAt(tree, path);
			if (node.locked) delete node.locked;
			else node.locked = true;
		},
		true
	);
}

// fill shows a realize response, keeping the locks it echoes. The draft
// gets a copy, so locking a word never changes the tree Keep posts.
export function fill(b: Builder, realized: Realized): Builder {
	return {
		draft: { tree: structuredClone(realized.tree), filled: realized },
		undo: [...b.undo, b.draft]
	};
}

export function undo(b: Builder): Builder {
	if (!b.undo.length) return b;
	return { draft: b.undo[b.undo.length - 1], undo: b.undo.slice(0, -1) };
}

// Slots whose word comes from the grammar rather than the lexicon: fixed
// words, and reflexives, which follow their subject.
const fixed = new Set([
	'Comma',
	'To',
	'Complementizer',
	'Pronoun:it',
	'Pronoun:reflexive',
	'Conjunction:neither',
	'Conjunction:nor'
]);

// lockable says whether a leaf's word could be locked. realize ignores
// locks on the others.
export function lockable(node: TreeNode): boolean {
	if (fixed.has(node.symbol) || fixed.has(pos(node))) return false;
	return !(pos(node) === 'Preposition' && node.symbol.includes(':'));
}

// draftText writes out the tree, or '' while any slot has no word.
export function draftText(tree: TreeNode): string {
	if (leaves(tree).some(({ node }) => !node.word)) return '';
	const words = tokens(tree).map((t) => (t.index > 0 && !t.punctuation ? ' ' : '') + t.text);
	return words.join('') + '.';
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `cd web && npx prettier --write src && npx vitest --run --project server src/lib/builder.spec.ts && npm run check`
Expected: PASS, 0 errors.

- [ ] **Step 6: Commit**

```bash
git add web/src/lib/builder.ts web/src/lib/builder.spec.ts web/src/lib/types.ts
git commit -m "feat: Add the builder's state functions"
```

---

### Task 6: Editable diagram

**Files:**
- Modify: `web/src/lib/diagram.ts` (sizes, `Editing`)
- Modify: `web/src/lib/diagram.spec.ts`
- Modify: `web/src/lib/components/Diagram.svelte`
- Test: `web/src/lib/components/Diagram.svelte.spec.ts`

**Interfaces:**
- Consumes: `lockable` (Task 5).
- Produces:
  - `interface Sizes { row: number; label: number; word: number; gap: number }`, `READ: Sizes`, `EDIT: Sizes`
  - `layout(tree: TreeNode, measure: Measure, sizes: Sizes = READ): Layout`
  - `interface Editing { onphrase: (path: number[]) => void; onword: (path: number[]) => void; problemPath: number[] | null }`
  - `Diagram` prop `editing?: Editing | null`. With it, each phrase is a button with `id="slot-{key(path)}"`, named "Choose {label}" for a hole or "Change {label}". Each lockable word is a toggle button named by its word, with `aria-pressed` set to its lock.

- [ ] **Step 1: Write the failing tests**

In `diagram.spec.ts`, change the import to `import { EDIT, key, layout, READ, short, type Box } from './diagram';`,
replace `GAP` with `READ.gap` and `ROW` with `READ.row`, and add:

```ts
	it('spaces rows and columns by the sizes it is given', () => {
		const edit = layout(sentence.tree, measure, EDIT);
		const placed = (path: number[]) => edit.placed.get(key(path))!;
		expect(placed([0, 1, 1, 0]).label.y).toBe(4 * EDIT.row);
		expect(placed([0, 0, 1]).label.x - placed([0, 0, 0]).label.x).toBe(
			(measure('Det') + measure('goose')) / 2 + EDIT.gap
		);
	});
```

(With the test's `measure`, the first column is 30 wide for both `Det` and `the`, and the second
is as wide as `goose`.)

In `Diagram.svelte.spec.ts`, add:

```ts
	describe('editing', () => {
		// The second clause's verb phrase is a hole.
		const tree = structuredClone(sentence.tree);
		tree.children![3].children![1] = { symbol: 'VP' };
		tree.children![0].children![0].children![0].locked = true;
		const editing = () => ({ onphrase: vi.fn(), onword: vi.fn(), problemPath: null });

		it('makes holes and phrases buttons that open their rules', async () => {
			const e = editing();
			render(Diagram, { tree, grammar, editing: e });

			await page.getByRole('button', { name: 'Choose verb phrase' }).click();
			expect(e.onphrase).toHaveBeenCalledWith([3, 1]);
			await page.getByRole('button', { name: 'Change verb phrase' }).click();
			expect(e.onphrase).toHaveBeenCalledWith([0, 1]);
		});

		it('makes lockable words lock toggles, and leaves fixed words alone', async () => {
			const e = editing();
			render(Diagram, { tree, grammar, editing: e });

			await expect
				.element(page.getByRole('button', { name: 'the', exact: true }))
				.toHaveAttribute('aria-pressed', 'true');
			const goose = page.getByRole('button', { name: 'goose', exact: true });
			await expect.element(goose).toHaveAttribute('aria-pressed', 'false');
			await goose.click();
			expect(e.onword).toHaveBeenCalledWith([0, 0, 1]);
			await expect
				.element(page.getByRole('button', { name: ',', exact: true }))
				.not.toBeInTheDocument();
		});

		it('marks the problem slot, a hole included', async () => {
			render(Diagram, { tree, grammar, editing: { ...editing(), problemPath: [3, 1] } });

			await expect.element(page.getByRole('button', { name: 'Choose verb phrase' })).toHaveClass(/problem/);
		});

		it('never shrinks, so its targets stay 44px', async () => {
			await page.viewport(320, 640);
			render(Diagram, { tree, grammar, editing: editing() });

			await expect.element(page.getByRole('button', { name: 'Zoom' })).not.toBeInTheDocument();
			const box = page
				.getByRole('button', { name: 'goose', exact: true })
				.element()
				.getBoundingClientRect();
			expect(box.height).toBeGreaterThanOrEqual(44);
			expect(box.width).toBeGreaterThanOrEqual(44);
		});
	});
```

Add `vi` to the `vitest` import.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd web && npx vitest --run src/lib/diagram.spec.ts src/lib/components/Diagram.svelte.spec.ts`
Expected: FAIL: `EDIT` and `READ` aren't exported, and no buttons are found.

- [ ] **Step 3: Implement `diagram.ts`**

Replace the four size constants with:

```ts
// Pixel heights of a row, a label and a word, and the gap between columns.
export interface Sizes {
	row: number;
	label: number;
	word: number;
	gap: number;
}

// Kept in step with Diagram.svelte's styles.
export const READ: Sizes = { row: 40, label: 20, word: 24, gap: 16 };
// An editable tree's labels and words are 44px buttons.
export const EDIT: Sizes = { row: 64, label: 44, word: 44, gap: 8 };

// Editing makes phrases open their rules and words toggle their locks.
export interface Editing {
	onphrase: (path: number[]) => void;
	onword: (path: number[]) => void;
	// The slot no word fits, to highlight.
	problemPath: number[] | null;
}
```

In `layout`, take the sizes and use them:

```ts
export function layout(tree: TreeNode, measure: Measure, sizes: Sizes = READ): Layout {
	const placed = new Map<string, Placed>();
	const edges: Edge[] = [];
	const baseline = (deepestLeaf(tree) + 1) * sizes.row;
```

and replace each `ROW` with `sizes.row`, `LABEL_HEIGHT` with `sizes.label`, `GAP` with
`sizes.gap` and `WORD_HEIGHT` with `sizes.word`.

- [ ] **Step 4: Implement `Diagram.svelte`**

Script changes:

```ts
	import { onMount } from 'svelte';
	import { lockable } from '#lib/builder.js';
	import { EDIT, key, layout, READ, short, type Editing, type Measure } from '#lib/diagram.js';
	import type { Grammar, TreeNode } from '#lib/types.js';

	let {
		tree,
		grammar,
		selectedPath = null,
		editing = null
	}: {
		tree: TreeNode;
		grammar: Grammar;
		selectedPath?: number[] | null;
		editing?: Editing | null;
	} = $props();
```

```ts
	// Editable labels and words are buttons: padded, at least 44px wide, and a
	// label has room for a hole's " ?".
	const measure: Measure = (text, kind) => {
		context ??= document.createElement('canvas').getContext('2d')!;
		context.font = fonts[kind];
		const width = context.measureText(text).width;
		return editing ? Math.max(44, width + (kind === 'label' ? 40 : 36)) : width;
	};

	const drawn = $derived.by(() => {
		void fontsReady;
		return layout(tree, measure, editing ? EDIT : READ);
	});
```

```ts
	// An editable tree never shrinks, so its targets keep their size; it
	// scrolls sideways instead.
	const scale = $derived(editing || zoomed || fits ? 1 : available / drawn.width);

	function same(a: number[] | null | undefined, b: number[]): boolean {
		return !!a && a.length === b.length && a.every((v, i) => b[i] === v);
	}
```

Replace the `branch` snippet:

```svelte
{#snippet branch(node: TreeNode, path: number[])}
	{@const placed = drawn.placed.get(key(path))!}
	{@const problem = same(editing?.problemPath, path)}
	<li>
		{#if editing && node.symbol in grammar.phrases}
			{@const hole = !node.children?.length}
			<button
				type="button"
				id="slot-{key(path)}"
				class="label slot"
				class:hole
				class:problem
				style:left="{placed.label.x}px"
				style:top="{placed.label.y}px"
				onclick={() => editing?.onphrase(path)}
				><span aria-hidden="true">{short(node)}{hole ? ' ?' : ''}</span><span
					class="visually-hidden">{hole ? 'Choose' : 'Change'} {name(node)}</span
				></button
			>
		{:else}
			<span
				class="label"
				class:on={onBranch(path)}
				class:problem
				style:left="{placed.label.x}px"
				style:top="{placed.label.y}px"
				><span aria-hidden="true">{short(node)}</span><span class="visually-hidden"
					>{name(node)}</span
				></span
			>
		{/if}
		{#if placed.word}
			{#if editing && lockable(node)}
				<button
					type="button"
					class="word lock"
					aria-pressed={!!node.locked}
					style:left="{placed.word.x}px"
					style:top="{placed.word.y}px"
					onclick={() => editing?.onword(path)}
					>{placed.word.text}{#if node.locked}<svg
							class="lock-icon"
							aria-hidden="true"
							viewBox="0 0 12 14"
							width="12"
							height="14"
							><rect x="1" y="6" width="10" height="8" rx="1.5" fill="currentColor" /><path
								d="M3.5 6V4a2.5 2.5 0 0 1 5 0v2"
								fill="none"
								stroke="currentColor"
								stroke-width="1.5"
							/></svg
						>{/if}</button
				>
			{:else}
				<span
					class="word"
					aria-current={onBranch(path) && path.length === selectedPath?.length ? 'true' : undefined}
					style:left="{placed.word.x}px"
					style:top="{placed.word.y}px">{placed.word.text}</span
				>
			{/if}
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
```

Markup: add `class:editing={!!editing}` to `.diagram`, show Zoom only when `!editing && !fits`,
and let an editable viewport scroll (its buttons take focus, so it needs no tabindex of its
own):

```svelte
<div class="diagram" class:editing={!!editing}>
	<div bind:clientWidth={available}></div>
	{#if !editing && !fits}
```

```svelte
	<div
		class="viewport"
		class:zoomed={zoomed || !!editing}
		style:height="{drawn.height * scale}px"
		tabindex={zoomed && !editing ? 0 : undefined}
		role={zoomed && !editing ? 'region' : undefined}
		aria-label={zoomed && !editing ? 'Sentence diagram, full size' : undefined}
	>
```

Styles to add:

```css
	.editing .label,
	.editing .word {
		line-height: 44px;
	}

	.slot,
	.lock {
		min-block-size: 44px;
		min-inline-size: 44px;
		border-radius: 8px;
		cursor: pointer;
	}

	.slot {
		padding: 0 12px;
		color: var(--text);
		background: var(--bg);
		border: 1.5px solid var(--separator);
	}

	.slot.hole {
		color: var(--action);
		border-style: dashed;
		border-color: var(--action);
	}

	.lock {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		padding: 0 8px;
		background: none;
		border: none;
	}

	.lock[aria-pressed='true'] {
		color: var(--action);
		background: var(--action-tint);
	}

	.label.problem,
	.slot.problem {
		color: var(--error);
		border-color: var(--error);
	}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `cd web && npx prettier --write src && npx vitest --run && npm run check && npm run lint`
Expected: PASS, 0 errors.

- [ ] **Step 6: Commit**

```bash
git add web/src/lib/diagram.ts web/src/lib/diagram.spec.ts web/src/lib/components/Diagram.svelte web/src/lib/components/Diagram.svelte.spec.ts
git commit -m "feat: Edit diagrams with phrase buttons and word locks"
```

---

### Task 7: Expansion sheet

**Files:**
- Create: `web/src/lib/components/ExpansionSheet.svelte`
- Test: `web/src/lib/components/ExpansionSheet.svelte.spec.ts`
- Modify: `web/src/app.css` (new `.sheet`, moved from the word card)
- Modify: `web/src/lib/components/WordCard.svelte` (uses `.sheet`)

**Interfaces:**
- Produces: `ExpansionSheet` with props `{ symbol: string; grammar: Grammar; filled: boolean; onchoose: (rule: string[]) => void; onclear: () => void; onclose: () => void }`. It's a region named "Choose {label}", or "Change {label}" when `filled`. Each rule is a button named by its symbols' labels joined with " + ", with any verb frame's example after the button.

- [ ] **Step 1: Write the failing tests**

`ExpansionSheet.svelte.spec.ts`:

```ts
import { page, userEvent } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { grammar } from '#lib/testing/fixtures.js';
import ExpansionSheet from './ExpansionSheet.svelte';

const handlers = () => ({ onchoose: vi.fn(), onclear: vi.fn(), onclose: vi.fn() });

describe('ExpansionSheet', () => {
	it('lists a hole’s rules by their labels and picks one', async () => {
		const h = handlers();
		render(ExpansionSheet, { symbol: 'NP', grammar, filled: false, ...h });

		const sheet = page.getByRole('region', { name: 'Choose noun phrase' });
		await expect.element(sheet.getByText('Names a thing.')).toBeInTheDocument();
		await sheet.getByRole('button', { name: 'determiner + noun', exact: true }).click();
		expect(h.onchoose).toHaveBeenCalledWith(['Determiner', 'Noun']);
		await expect.element(sheet.getByRole('button', { name: 'Clear' })).not.toBeInTheDocument();
	});

	it('shows a verb frame’s example', async () => {
		render(ExpansionSheet, { symbol: 'VP', grammar, filled: false, ...handlers() });

		await expect
			.element(page.getByRole('button', { name: 'transitive verb + noun phrase', exact: true }))
			.toBeInTheDocument();
		await expect.element(page.getByText('“devoured the goose”')).toBeInTheDocument();
	});

	it('offers Clear on a filled phrase', async () => {
		const h = handlers();
		render(ExpansionSheet, { symbol: 'VP', grammar, filled: true, ...h });

		await page.getByRole('region', { name: 'Change verb phrase' }).getByRole('button', { name: 'Clear' }).click();
		expect(h.onclear).toHaveBeenCalledOnce();
	});

	it('takes focus and closes on Escape', async () => {
		const h = handlers();
		render(ExpansionSheet, { symbol: 'NP', grammar, filled: false, ...h });

		await expect.element(page.getByRole('heading', { name: 'Choose noun phrase' })).toHaveFocus();
		await userEvent.keyboard('{Escape}');
		expect(h.onclose).toHaveBeenCalledOnce();
	});
});
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd web && npx vitest --run src/lib/components/ExpansionSheet.svelte.spec.ts`
Expected: FAIL, `./ExpansionSheet.svelte` can't be resolved.

- [ ] **Step 3: Move the sheet styles**

Move the word card's container styles into `app.css` as a shared `.sheet`. Append to
`app.css`:

```css
/* A card on wide screens and a bottom sheet on phones: the word card and the
   builder's expansion sheet. */
.sheet {
	text-align: start;
	border-radius: 16px;
	padding: 1rem 1.25rem;
	background: var(--fill);
	max-inline-size: 40rem;
	margin: 0 auto 1rem;
	overflow-wrap: anywhere;
}

@media (max-width: 640px) {
	/* Room to scroll everything above the sheet, and focus scrolled clear
	   of it, so the sheet never hides a focused control. */
	html:has(.sheet) {
		scroll-padding-block-end: 60vh;
	}

	body:has(.sheet) {
		padding-block-end: 60vh;
	}

	.sheet {
		position: fixed;
		inset-inline: 0;
		inset-block-end: 0;
		max-block-size: 60vh;
		overflow-y: auto;
		border-radius: 16px 16px 0 0;
		margin: 0;
		padding-block-start: 1.5rem;
		background: var(--bg);
		box-shadow: 0 -0.25rem 1rem rgb(0 0 0 / 0.2);
	}

	/* A sheet's grabber, for the look: Close and Escape dismiss it. */
	.sheet::before {
		content: '';
		position: absolute;
		inset-block-start: 0.5rem;
		inset-inline-start: 50%;
		translate: -50%;
		inline-size: 36px;
		block-size: 5px;
		border-radius: 999px;
		background: var(--separator);
	}
}
```

In `WordCard.svelte`, change `<section class="card"` to `<section class="sheet"`, delete the
`.card` rule, and reduce the media query to what's left:

```css
	@media (max-width: 640px) {
		.features li {
			background: var(--fill);
		}
	}
```

- [ ] **Step 4: Implement**

`ExpansionSheet.svelte`:

```svelte
<script lang="ts">
	import { short } from '#lib/diagram.js';
	import type { Grammar } from '#lib/types.js';

	let {
		symbol,
		grammar,
		filled,
		onchoose,
		onclear,
		onclose
	}: {
		symbol: string;
		grammar: Grammar;
		filled: boolean;
		onchoose: (rule: string[]) => void;
		onclear: () => void;
		onclose: () => void;
	} = $props();

	const phrase = $derived(grammar.phrases[symbol]);
	const headingId = $props.id();
	let heading: HTMLHeadingElement | undefined = $state();

	// Each newly opened phrase moves focus to its heading.
	$effect(() => {
		void symbol;
		heading?.focus();
	});

	function label(s: string): string {
		return (grammar.phrases[s] ?? grammar.slots[s])?.label ?? s;
	}

	function example(rule: string[]): string | undefined {
		return rule.map((s) => grammar.slots[s]?.example).find(Boolean);
	}
</script>

<svelte:window onkeydown={(e) => e.key === 'Escape' && onclose()} />

<section class="sheet" aria-labelledby={headingId}>
	<h2 id={headingId} tabindex="-1" bind:this={heading}>
		{filled ? 'Change' : 'Choose'}
		{phrase.label}
	</h2>
	<p class="description">{phrase.description}</p>
	<ul class="rules">
		{#each phrase.rules as rule, i (i)}
			{@const e = example(rule)}
			<li>
				<button type="button" class="rule" onclick={() => onchoose(rule)}
					><span>{rule.map(label).join(' + ')}</span><span class="symbols" aria-hidden="true"
						>{rule.map((s) => short({ symbol: s })).join(' ')}</span
					></button
				>
				{#if e}<p class="example">“{e}”</p>{/if}
			</li>
		{/each}
	</ul>
	<div class="actions">
		{#if filled}
			<button type="button" class="button plain" onclick={onclear}>Clear</button>
		{/if}
		<button type="button" class="button" onclick={onclose}>Close</button>
	</div>
</section>

<style>
	h2 {
		margin-block-start: 0;
	}

	.description {
		color: var(--secondary);
	}

	.rules {
		display: grid;
		gap: 0.5rem;
		padding: 0;
		list-style: none;
	}

	.rule {
		display: flex;
		flex-wrap: wrap;
		justify-content: space-between;
		align-items: center;
		gap: 0.25rem 1rem;
		inline-size: 100%;
		min-block-size: 44px;
		padding: 0.5rem 1rem;
		font: inherit;
		font-weight: 600;
		text-align: start;
		color: var(--action);
		background: var(--bg);
		border: none;
		border-radius: 12px;
		cursor: pointer;
	}

	.symbols {
		font-size: var(--text-sm);
		color: var(--secondary);
	}

	.example {
		margin: 0.25rem 1rem 0;
		font-size: var(--text-sm);
		color: var(--secondary);
	}

	.actions {
		display: flex;
		gap: 0.5rem;
		justify-content: flex-end;
	}

	@media (max-width: 640px) {
		.rule {
			background: var(--fill);
		}
	}
</style>
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `cd web && npx prettier --write src && npx vitest --run && npm run check && npm run lint`
Expected: PASS, including the existing `WordCard` tests, 0 errors.

- [ ] **Step 6: Commit**

```bash
git add web/src/app.css web/src/lib/components
git commit -m "feat: Add the expansion sheet and share the word card's sheet styles"
```

---

### Task 8: The `/build` page

**Files:**
- Create: `web/src/routes/build/+page.svelte`
- Create: `web/src/routes/build/page.e2e.ts`
- Modify: `web/src/routes/+layout.svelte` (Build link, page headings)
- Modify: `README.md` (one line on the builder, under the web app setup)

**Interfaces:**
- Consumes: everything in `builder.ts` (Task 5), `Diagram`'s `editing` (Task 6), `ExpansionSheet` (Task 7), the layout's `data.grammar`.
- Produces: `/build`. A header link named "Build" and an `h1` "Build a sentence" in the header crumbs on that page.

- [ ] **Step 1: Write the failing e2e tests**

`web/src/routes/build/page.e2e.ts`:

```ts
import { expect, test, type Page } from '@playwright/test';
import { expectNoAxeViolations, expectNoSidewaysScroll } from '../../lib/testing/e2e';
import { leaves } from '../../lib/tree';
import type { TreeNode } from '../../lib/types';

const diagram = (page: Page) => page.getByRole('list', { name: 'Sentence diagram' });

// pick opens a slot's sheet and picks a rule, by keyboard.
async function pick(page: Page, slot: string, rule: string) {
	await diagram(page).getByRole('button', { name: `Choose ${slot}`, exact: true }).focus();
	await page.keyboard.press('Enter');
	const sheet = page.getByRole('region', { name: `Choose ${slot}` });
	await expect(sheet.getByRole('heading')).toBeFocused();
	await sheet.getByRole('button', { name: rule, exact: true }).focus();
	await page.keyboard.press('Enter');
	await expect(sheet).toHaveCount(0);
}

// build makes "Det Noun Verb" by keyboard, fills it and returns the
// realize response.
async function build(page: Page): Promise<{ text: string; tree: TreeNode }> {
	await page.goto('/build');
	await pick(page, 'sentence', 'clause');
	await pick(page, 'clause', 'noun phrase + verb phrase');
	await pick(page, 'noun phrase', 'determiner + noun');
	await pick(page, 'verb phrase', 'intransitive verb');
	const filled = page.waitForResponse('**/api/v1/sentences/realize');
	await page.getByRole('button', { name: 'Fill', exact: true }).focus();
	await page.keyboard.press('Enter');
	return (await filled).json();
}

test('builds, fills, locks and rerolls a sentence by keyboard', async ({ page }) => {
	const first = await build(page);
	await expect(page.getByText(first.text, { exact: true })).toBeVisible();

	// Det, Noun and Verb are the word toggles, in order.
	const noun = diagram(page).locator('button[aria-pressed]').nth(1);
	await noun.focus();
	await page.keyboard.press('Enter');
	await expect(noun).toHaveAttribute('aria-pressed', 'true');

	const rerolled = page.waitForResponse('**/api/v1/sentences/realize');
	await page.getByRole('button', { name: 'Reroll', exact: true }).focus();
	await page.keyboard.press('Enter');
	const second: { tree: TreeNode } = await (await rerolled).json();
	const lemma = (r: { tree: TreeNode }) => leaves(r.tree).find((l) => l.node.symbol === 'Noun')!.node.lemma;
	expect(lemma(second)).toBe(lemma(first));
});

test('returns focus to the phrase when the sheet closes on Escape', async ({ page }) => {
	await page.goto('/build');
	const slot = diagram(page).getByRole('button', { name: 'Choose sentence', exact: true });
	await slot.focus();
	await page.keyboard.press('Enter');
	await expect(page.getByRole('region', { name: 'Choose sentence' })).toBeVisible();

	await page.keyboard.press('Escape');
	await expect(page.getByRole('region', { name: 'Choose sentence' })).toHaveCount(0);
	await expect(slot).toBeFocused();
});

test('undoes a fill', async ({ page }) => {
	await build(page);
	await page.getByRole('button', { name: 'Undo', exact: true }).click();

	await expect(page.getByRole('button', { name: 'Fill', exact: true })).toBeVisible();
});

test('passes axe with the sheet open and with a filled diagram', async ({ page }) => {
	await page.goto('/build');
	await diagram(page).getByRole('button', { name: 'Choose sentence', exact: true }).click();
	await expectNoAxeViolations(page);

	await page.keyboard.press('Escape');
	await page.getByRole('button', { name: 'Fill', exact: true }).click();
	await expect(page.getByRole('button', { name: 'Reroll', exact: true })).toBeVisible();
	await expectNoAxeViolations(page);
});

test('reflows at 320px without sideways scrolling', async ({ page }) => {
	await page.setViewportSize({ width: 320, height: 640 });
	await page.goto('/build');
	await page.getByRole('button', { name: 'Fill', exact: true }).click();
	await expect(page.getByRole('button', { name: 'Reroll', exact: true })).toBeVisible();

	await expectNoSidewaysScroll(page);
});

test('the header links to the builder', async ({ page }) => {
	await page.goto('/stars');
	await page.getByRole('navigation').getByRole('link', { name: 'Build', exact: true }).click();

	await expect(page.getByRole('heading', { name: 'Build a sentence', level: 1 })).toBeVisible();
});
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `just build-fe && cd web && npx playwright test src/routes/build`
Expected: FAIL: `/build` is a 404.

- [ ] **Step 3: Implement the header**

In `+layout.svelte`, replace the stars-only logic with a list of linked pages:

```ts
	// Each page names itself in the header, so its own link would repeat it.
	const pages = [
		{ href: '/build', link: 'Build', heading: 'Build a sentence' },
		{ href: '/stars', link: 'Your stars', heading: 'Your stars' }
	];
	const current = $derived(pages.find((p) => p.href === page.url.pathname));
```

```svelte
<header>
	<div class="crumbs">
		<a class="title" href="/">RandSense</a>
		{#if current}
			<span class="divider" aria-hidden="true">/</span>
			<h1>{current.heading}</h1>
		{/if}
	</div>
	<nav aria-label="Main">
		{#each pages.filter((p) => p !== current) as p (p.href)}
			<a class="button plain" href={p.href}>{p.link}</a>
		{/each}
	</nav>
</header>
```

Add `flex-wrap: wrap;` to the `header` rule, so the title and two links wrap at 320px.

- [ ] **Step 4: Implement the page**

`web/src/routes/build/+page.svelte`:

```svelte
<script lang="ts">
	import { tick } from 'svelte';
	import Diagram from '#lib/components/Diagram.svelte';
	import ExpansionSheet from '#lib/components/ExpansionSheet.svelte';
	import {
		choose,
		clear,
		draftText,
		fill,
		nodeAt,
		start,
		toggleLock,
		undo,
		type Builder
	} from '#lib/builder.js';
	import { key } from '#lib/diagram.js';
	import { leaves } from '#lib/tree.js';

	let { data } = $props();

	// Builder functions return new state, and structuredClone can't copy a
	// deep $state proxy, so the state is raw.
	// svelte-ignore state_referenced_locally
	let b = $state.raw<Builder>(start(data.grammar));
	// The phrase whose sheet is open.
	let open = $state<number[] | null>(null);
	let busy = $state(false);
	let problem = $state('');
	let problemPath = $state<number[] | null>(null);

	const text = $derived(draftText(b.draft.tree));
	const openNode = $derived(open && nodeAt(b.draft.tree, open));

	// apply shows the next state, closes the sheet and any problem, and
	// returns focus to the phrase whose sheet was open.
	async function apply(next: Builder) {
		const path = open;
		b = next;
		open = null;
		problem = '';
		problemPath = null;
		if (!path) return;
		await tick();
		document.getElementById(`slot-${key(path)}`)?.focus();
	}

	async function realize() {
		busy = true;
		problem = '';
		problemPath = null;
		try {
			const res = await fetch('/api/v1/sentences/realize', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify(b.draft.tree)
			});
			if (res.status === 422) {
				const { leaf } = (await res.json()) as { leaf: number };
				const target = leaves(b.draft.tree)[leaf];
				problemPath = target.path;
				problem = target.node.locked
					? 'No word fits here with that word locked. Unlock it, or change the phrase above it.'
					: 'No word fits this slot right now. Change the phrase above it.';
				return;
			}
			if (!res.ok) throw new Error(`status ${res.status}`);
			b = fill(b, await res.json());
		} catch {
			problem = 'Couldn’t fill the sentence. Try again.';
		} finally {
			busy = false;
		}
	}
</script>

<svelte:head>
	<title>Build a sentence · RandSense</title>
	<meta name="description" content="Build a grammatically sound sentence from the grammar down." />
</svelte:head>

<p class="intro">
	Tap a dashed slot to choose what goes in it, or leave it for Fill to choose. After a fill, tap a
	word to lock it, then reroll the rest.
</p>

<div class="actions">
	<button type="button" class="button primary" disabled={busy} onclick={realize}
		>{text ? 'Reroll' : 'Fill'}</button
	>
	<button type="button" class="button plain" disabled={!b.undo.length} onclick={() => apply(undo(b))}
		>Undo</button
	>
</div>
<p class="problem" role="status">{problem}</p>
<p class="draft" aria-live="polite">{text}</p>

<Diagram
	tree={b.draft.tree}
	grammar={data.grammar}
	editing={{
		onphrase: (path) => (open = path),
		onword: (path) => apply(toggleLock(b, path)),
		problemPath
	}}
/>

{#if open && openNode}
	<ExpansionSheet
		symbol={openNode.symbol}
		grammar={data.grammar}
		filled={!!openNode.children?.length}
		onchoose={(rule) => apply(choose(b, open!, rule))}
		onclear={() => apply(clear(b, open!))}
		onclose={() => apply(b)}
	/>
{/if}

<style>
	.intro {
		color: var(--secondary);
		max-inline-size: 40rem;
		margin-inline: auto;
	}

	.actions {
		display: flex;
		flex-wrap: wrap;
		justify-content: center;
		gap: 0.5rem;
	}

	.draft {
		font-size: var(--text-xl);
		overflow-wrap: anywhere;
	}
</style>
```

`apply(b)` on close keeps the state and only closes the sheet and returns focus.

- [ ] **Step 5: Document the page**

In `README.md`, after the paragraph about `just run` starting both servers, add:

```markdown
`/build` builds a sentence from `S` down: each slot offers only its rules from `grammar.toml`,
Fill and Reroll call `realize`, and a tapped word is locked through the next reroll.
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `cd web && npx prettier --write src && cd .. && just test-fe`
Expected: PASS, in both the desktop and phone projects.

- [ ] **Step 7: Commit**

```bash
git add web/src/routes README.md
git commit -m "feat: Build sentences from the grammar down at /build"
```

---

### Task 9: Signature, Keep and `origin`

**Files:**
- Create: `service/migrations/008_sentence_origin.up.sql`, `008_sentence_origin.down.sql`
- Modify: `service/internal/store/queries/sentences.sql` (`InsertSentence`), regenerate
- Create: `service/internal/api/keep.go`
- Create: `service/internal/api/keep_test.go`, `service/internal/api/keep_internal_test.go`
- Modify: `service/internal/api/router.go`, `handlers.go`, `sentences.go`, `resps.go`
- Modify: `service/internal/api/helpers_test.go` (`newServer`), `sentences_test.go`
- Modify: `service/cmd/server/main.go`, `.env.example`, `README.md`
- Modify: `docs/superpowers/specs/2026-10-03-frontend-design.md` ("Rate limits")

**Interfaces:**
- Consumes: `sentence.Text` (Task 3).
- Produces:
  - `RouterConfig.BuildSecret []byte`
  - `realize` responds `{text, tree, signature}`
  - `POST /api/v1/sentences` with `{tree, signature}` → 201 and a sentence, or 400
  - `SentenceResponse.Origin string` (json `origin`: `generated` or `built`)

- [ ] **Step 1: Add the migration and query**

`008_sentence_origin.up.sql`:

```sql
-- built sentences were kept from the builder; generated ones came from random.
ALTER TABLE sentences ADD COLUMN origin TEXT NOT NULL DEFAULT 'generated'
    CHECK (origin IN ('generated', 'built'));
```

`008_sentence_origin.down.sql`:

```sql
ALTER TABLE sentences DROP COLUMN origin;
```

`sentences.sql`:

```sql
-- name: InsertSentence :one
INSERT INTO sentences (id, text, tree, commonness, origin)
VALUES (@id, @text, @tree, @commonness::float8, @origin)
RETURNING *;
```

Run: `just generate && just migrate-up`

Add a secret to your local `.env`, since the server won't start without one:

```bash
grep -q '^BUILD_SECRET=' .env || echo 'BUILD_SECRET=dev-only-build-secret-change-me-in-prod' >> .env
```

- [ ] **Step 2: Write the failing tests**

`keep_internal_test.go`:

```go
package api

import (
	"strings"
	"testing"
	"time"
)

func TestValidTree(t *testing.T) {
	secret := []byte("build-secret-for-tests-32-bytes-x")
	tree := []byte(`{"symbol":"S"}`)
	issued := time.Unix(1_800_000_000, 0)
	sig := signTree(secret, tree, issued)
	at, mac, _ := strings.Cut(sig, ".")

	tests := []struct {
		name         string
		secret, tree []byte
		sig          string
		now          time.Time
		want         bool
	}{
		{"fresh", secret, tree, sig, issued.Add(time.Hour), true},
		{"a day old", secret, tree, sig, issued.Add(buildTTL), false},
		{"another tree", secret, []byte(`{"symbol":"NP"}`), sig, issued, false},
		{"another secret", []byte("another-build-secret-32-bytes-xx"), tree, sig, issued, false},
		{"another time", secret, tree, "1800000001." + mac, issued, false},
		{"no dot", secret, tree, at + mac, issued, false},
		{"time that isn't a number", secret, tree, "soon." + mac, issued, false},
		{"mac that isn't base64", secret, tree, at + ".!!", issued, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := validTree(tc.secret, tc.tree, tc.sig, tc.now); got != tc.want {
				t.Errorf("validTree: got %v, want %v", got, tc.want)
			}
		})
	}
}
```

`keep_test.go`:

```go
package api_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jameynakama/randsense/internal/api"
)

// realized is a realize response for realizeTree.
type realized struct {
	Text      string          `json:"text"`
	Tree      json.RawMessage `json:"tree"`
	Signature string          `json:"signature"`
}

func realizeForKeep(t *testing.T, srv *httptest.Server) realized {
	t.Helper()
	resp := postTree(t, srv, "", realizeTree)
	defer resp.Body.Close()
	var r realized
	decode(t, resp, http.StatusOK, &r)
	if r.Signature == "" {
		t.Fatal("realize returned no signature")
	}
	return r
}

func keepBody(tree json.RawMessage, signature string) string {
	return fmt.Sprintf(`{"tree": %s, "signature": %q}`, tree, signature)
}

func TestKeepSavesABuiltSentence(t *testing.T) {
	seedWords(t)
	srv := newRealizeServer(t)
	t.Cleanup(srv.Close)
	_, events := openStream(t, srv)
	r := realizeForKeep(t, srv)

	var kept api.SentenceResponse
	decode(t, call(t, srv, http.MethodPost, "/api/v1/sentences", keepBody(r.Tree, r.Signature), nil), http.StatusCreated, &kept)

	if kept.Origin != "built" || kept.Text != r.Text {
		t.Errorf("kept: got %q %q, want built %q", kept.Origin, kept.Text, r.Text)
	}
	var got api.SentenceResponse
	decode(t, call(t, srv, http.MethodGet, "/api/v1/sentences/"+kept.ID, "", nil), http.StatusOK, &got)
	if got.Origin != "built" {
		t.Errorf("saved origin: got %q, want built", got.Origin)
	}
	if name, data := nextEvent(t, events); name != "sentence" || !strings.Contains(data, kept.ID) {
		t.Errorf("event: got %s %s, want the kept sentence", name, data)
	}
}

func TestKeepRejectsTreesItDidNotSign(t *testing.T) {
	seedWords(t)
	srv := newRealizeServer(t)
	defer srv.Close()
	r := realizeForKeep(t, srv)
	tampered := json.RawMessage(strings.ReplaceAll(string(r.Tree), "goose", "gander"))

	tests := []struct{ name, body string }{
		{"tampered tree", keepBody(tampered, r.Signature)},
		{"no signature", keepBody(r.Tree, "")},
		{"garbage signature", keepBody(r.Tree, "x.y")},
		{"not JSON", "keep it"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			decode(t, call(t, srv, http.MethodPost, "/api/v1/sentences", tc.body, nil), http.StatusBadRequest, nil)
		})
	}
}
```

In `sentences_test.go`, `TestRandomSentenceIsSaved`, add after the body check:

```go
	if body.Origin != "generated" {
		t.Errorf("origin: got %q, want generated", body.Origin)
	}
```

In `helpers_test.go`, add `var testBuildSecret = []byte("test-build-secret-32-bytes-xxxxx")` and,
in `newServer`, before `cfg.Verbs = v`:

```go
	if cfg.BuildSecret == nil {
		cfg.BuildSecret = testBuildSecret
	}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `just test-be`
Expected: compile failure: `signTree`, `validTree`, `buildTTL`, `BuildSecret` and `Origin`
undefined.

- [ ] **Step 4: Implement**

`resps.go`, `SentenceResponse` and `sentenceResponse`:

```go
type SentenceResponse struct {
	ID        string          `json:"id"`
	Text      string          `json:"text"`
	Tree      json.RawMessage `json:"tree"`
	StarCount int32           `json:"star_count"`
	Origin    string          `json:"origin"`
	CreatedAt time.Time       `json:"created_at"`
}

func sentenceResponse(s store.Sentence) SentenceResponse {
	return SentenceResponse{ID: s.ID, Text: s.Text, Tree: s.Tree, StarCount: s.StarCount, Origin: s.Origin, CreatedAt: s.CreatedAt.Time}
}
```

`sentences.go`, after `insertWithNewID`:

```go
// A sentence's origin: random generated it, or someone built and kept it.
const (
	originGenerated = "generated"
	originBuilt     = "built"
)

// save stores a sentence under a new ID.
func (h *Handler) save(ctx context.Context, text string, tree []byte, commonness float64, origin string) (store.Sentence, error) {
	return insertWithNewID(newSentenceID, func(id string) (store.Sentence, error) {
		return h.queries.InsertSentence(ctx, store.InsertSentenceParams{
			ID: id, Text: text, Tree: tree, Commonness: commonness, Origin: origin,
		})
	})
}
```

Add `"context"` to its imports. In `handlers.go`, `randomSentence` saves with it:

```go
	saved, err := h.save(r.Context(), s.Text, tree, c, originGenerated)
```

`router.go`: add to `RouterConfig` and `Handler`, set it in `NewRouter`, and route Keep:

```go
	// BuildSecret signs realized trees, so Keep saves only what realize made.
	BuildSecret []byte
```

```go
	buildSecret []byte
```

```go
		buildSecret: cfg.BuildSecret,
```

```go
		r.Get("/sentences", h.listSentences)
		r.Post("/sentences", h.keepSentence)
```

`keep.go`:

```go
package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jameynakama/randsense/internal/grammar"
	"github.com/jameynakama/randsense/internal/sentence"
)

// buildTTL is how long a realized tree can wait to be kept.
const buildTTL = 24 * time.Hour

// maxKeepBytes caps a keep request. A realized tree is bigger than the one
// posted to realize: it carries words and features.
const maxKeepBytes = 256 << 10

// signTree signs a realized tree's canonical JSON and when it was realized,
// as "unix.mac". The server keeps nothing between realize and keep.
func signTree(secret, tree []byte, issued time.Time) string {
	at := strconv.FormatInt(issued.Unix(), 10)
	return at + "." + base64.RawURLEncoding.EncodeToString(treeMAC(secret, at, tree))
}

// validTree reports whether sig signs tree and was issued less than
// buildTTL before now.
func validTree(secret, tree []byte, sig string, now time.Time) bool {
	at, mac, ok := strings.Cut(sig, ".")
	if !ok {
		return false
	}
	unix, err := strconv.ParseInt(at, 10, 64)
	if err != nil {
		return false
	}
	got, err := base64.RawURLEncoding.DecodeString(mac)
	return err == nil && hmac.Equal(got, treeMAC(secret, at, tree)) && now.Before(time.Unix(unix, 0).Add(buildTTL))
}

func treeMAC(secret []byte, at string, tree []byte) []byte {
	m := hmac.New(sha256.New, secret)
	m.Write([]byte(at + "."))
	m.Write(tree)
	return m.Sum(nil)
}

// realizeResponse is a realized sentence and the signature Keep checks.
type realizeResponse struct {
	*sentence.Sentence
	Signature string `json:"signature"`
}

type keepRequest struct {
	Tree      grammar.Node `json:"tree"`
	Signature string       `json:"signature"`
}

// keepSentence saves a tree realize returned, under its signature, so
// nobody can save words of their own under RandSense's name. The text is
// written from the tree, never taken from the client.
func (h *Handler) keepSentence(w http.ResponseWriter, r *http.Request) {
	var req keepRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxKeepBytes)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("body must be JSON {tree, signature} of at most %d bytes: %v", maxKeepBytes, err))
		return
	}
	// Marshaling the decoded tree gives the same bytes realize signed.
	tree, err := json.Marshal(&req.Tree)
	if err != nil {
		log.Printf("keepSentence: %v", err)
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	if !validTree(h.buildSecret, tree, req.Signature, time.Now()) {
		writeError(w, http.StatusBadRequest, "the signature doesn't match this tree, or is a day old: reroll, then keep")
		return
	}
	// Realize always sets the root's floor, and the signature vouches for
	// the tree.
	saved, err := h.save(r.Context(), sentence.Text(&req.Tree), tree, *req.Tree.Features.Commonness, originBuilt)
	if err != nil {
		log.Printf("keepSentence: save: %v", err)
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	resp := sentenceResponse(saved)
	h.publish("sentence", resp)
	writeJSON(w, http.StatusCreated, resp)
}
```

In `handlers.go`, `realizeSentence` ends by signing:

```go
	tree, err := json.Marshal(s.Tree)
	if err != nil {
		log.Printf("realizeSentence: %v", err)
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	writeJSON(w, http.StatusOK, realizeResponse{Sentence: s, Signature: signTree(h.buildSecret, tree, time.Now())})
```

Add `"time"` to its imports.

`cmd/server/main.go`: add `buildSecret string` to `config`, `buildSecret: required("BUILD_SECRET"),`
to `loadConfig`, this check after the `SESSION_SECRET` one:

```go
	if len(cfg.buildSecret) < 32 {
		log.Fatal("BUILD_SECRET must be at least 32 bytes")
	}
```

and `BuildSecret: []byte(cfg.buildSecret),` in `routerCfg`.

`.env.example`, after `SESSION_SECRET`:

```bash
# Signs realized trees, so Keep saves only words realize chose. 32+ bytes.
BUILD_SECRET=dev-only-build-secret-change-me-in-prod
```

- [ ] **Step 5: Document**

In `README.md`'s API block, add `origin` to every sentence shape (`{id, text, tree, star_count,
origin, created_at}`), change the realize line to `-> {text, tree, signature}`, and add:

```
POST /api/v1/sentences                        {tree, signature} -> 201 {id, text, tree, star_count, origin, created_at}
```

After the `realize` paragraphs, add:

```markdown
`realize` signs each tree it returns. `POST /sentences` keeps one: it saves the tree with
`origin: "built"` and broadcasts it like `random`, if the signature matches the tree and is less
than a day old. Otherwise it's a 400. The text is written from the tree. `BUILD_SECRET` (32+
bytes) signs the trees, so changing it only means unkept trees need a reroll. Generated sentences
have `origin: "generated"`.
```

In the stream paragraph, change "whenever `random` saves one" to "whenever `random` or `POST
/sentences` saves one".

In the frontend spec's "Rate limits" list, add:

```markdown
- realize (`POST /api/v1/sentences/realize`): like generation, since every builder reroll calls it
- keep (`POST /api/v1/sentences`): tight, like stars
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `just test-be`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add service .env.example README.md docs/superpowers/specs/2026-10-03-frontend-design.md
git commit -m "feat: Sign realized trees and keep them as built sentences"
```

---

### Task 10: Keep button and Homemade badge

**Files:**
- Modify: `web/src/lib/types.ts` (`Sentence.origin`)
- Modify: `web/src/lib/builder.ts` (`Realized.signature`)
- Modify: `web/src/lib/builder.spec.ts`, `web/src/lib/testing/fixtures.ts`
- Modify: `web/src/routes/build/+page.svelte`, `web/src/routes/build/page.e2e.ts`
- Modify: `web/src/lib/components/Feed.svelte`, `SentenceView.svelte`, `web/src/app.css`
- Test: `Feed.svelte.spec.ts`, `SentenceView.svelte.spec.ts`

**Interfaces:**
- Consumes: `POST /api/v1/sentences`, `signature` on realize (Task 9).
- Produces: `Sentence.origin: 'generated' | 'built'`; `Realized.signature: string`; a global `.badge` class.

- [ ] **Step 1: Write the failing tests**

`fixtures.ts`: add `origin: 'generated',` to `sentence`. `builder.spec.ts`: give `realized` a
signature, `const realized: Realized = { text: sentence.text, tree: sentence.tree, signature: 's' };`.

`Feed.svelte.spec.ts`:

```ts
	it('marks built sentences Homemade', async () => {
		render(Feed, { initial: [{ ...second, origin: 'built' }, third] });

		await expect.element(page.getByText('Homemade')).toBeInTheDocument();
		expect(page.getByText('Homemade').all()).toHaveLength(1);
	});
```

`SentenceView.svelte.spec.ts`:

```ts
	it('marks a built sentence Homemade, and only a built one', async () => {
		const { rerender } = render(SentenceView, { sentence, grammar, count: 2 });
		await expect.element(page.getByText('Homemade')).not.toBeInTheDocument();

		await rerender({ sentence: { ...sentence, origin: 'built' } });
		await expect.element(page.getByText('Homemade')).toBeInTheDocument();
	});
```

In `build/page.e2e.ts`, add:

```ts
test('keeps a built sentence after a lock, with a Homemade badge on its permalink and in the feed', async ({ page }) => {
	const first = await build(page);
	await diagram(page).locator('button[aria-pressed]').nth(1).click();

	await page.getByRole('button', { name: 'Keep this one', exact: true }).click();
	await expect(page).toHaveURL(/\/s\/[0-9A-Za-z]{8}$/);
	await expect(page.getByRole('group', { name: first.text })).toBeVisible();
	await expect(page.getByText('Homemade', { exact: true })).toBeVisible();

	await page.goto('/');
	const row = page.getByRole('listitem').filter({ has: page.getByRole('link', { name: first.text, exact: true }) });
	await expect(row.first().getByText('Homemade', { exact: true })).toBeVisible();
});

test('saves once when Keep is pressed twice', async ({ page }) => {
	await build(page);
	let posts = 0;
	await page.route('**/api/v1/sentences', async (route) => {
		posts++;
		await new Promise((resolve) => setTimeout(resolve, 500));
		await route.continue();
	});

	await page.getByRole('button', { name: 'Keep this one', exact: true }).dblclick();
	await expect(page).toHaveURL(/\/s\//);
	expect(posts).toBe(1);
});

test('keeps nothing until a fill', async ({ page }) => {
	await page.goto('/build');

	await expect(page.getByRole('button', { name: 'Keep this one', exact: true })).toBeDisabled();
});
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd web && npx vitest --run && npm run check`
Expected: FAIL: no Homemade badge, and type errors for `origin` and `signature`.

- [ ] **Step 3: Implement**

`types.ts`, in `Sentence`:

```ts
	// built: kept from the builder; generated: from random.
	origin: 'generated' | 'built';
```

`builder.ts`, in `Realized`:

```ts
	// Keep posts this with the tree, untouched.
	signature: string;
```

`app.css`, after `.sentence-list a:hover`:

```css
/* Marks a sentence someone built and kept. */
.badge {
	color: var(--secondary);
	font-size: var(--text-sm);
	font-weight: 600;
}
```

`Feed.svelte`, in each row:

```svelte
				<a href="/s/{s.id}">{s.text}</a>
				{#if s.origin === 'built'}<span class="badge">Homemade</span>{/if}
				<StarButton id={s.id} count={s.star_count} />
```

`SentenceView.svelte`, after `<Sentence {sentence} bind:selected />`:

```svelte
{#if sentence.origin === 'built'}<p class="badge">Homemade</p>{/if}
```

`build/+page.svelte`: import `goto` from `$app/navigation` and `type Sentence` from
`#lib/types.js`, then add:

```ts
	async function keep() {
		busy = true;
		problem = '';
		try {
			const { tree, signature } = b.draft.filled!;
			const res = await fetch('/api/v1/sentences', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ tree, signature })
			});
			if (res.status === 400) {
				problem = 'This one waited too long to be kept. Reroll, then keep.';
				return;
			}
			if (!res.ok) throw new Error(`status ${res.status}`);
			const kept: Sentence = await res.json();
			await goto(`/s/${kept.id}`);
		} catch {
			problem = 'Couldn’t keep the sentence. Try again.';
		} finally {
			busy = false;
		}
	}
```

and the button, after Undo:

```svelte
	<button type="button" class="button" disabled={busy || !b.draft.filled} onclick={keep}
		>Keep this one</button
	>
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `cd web && npx prettier --write src && cd .. && just test-fe`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/src
git commit -m "feat: Keep built sentences and mark them Homemade"
```

---

### Task 11: Remix

**Files:**
- Create: `web/src/routes/build/+page.server.ts`
- Modify: `web/src/lib/builder.ts` (`remix`), `web/src/lib/builder.spec.ts`
- Modify: `web/src/routes/build/+page.svelte`, `web/src/routes/build/page.e2e.ts`
- Modify: `web/src/lib/components/SentenceView.svelte`, `SentenceView.svelte.spec.ts`
- Modify: `README.md` (the `/build` line)

**Interfaces:**
- Consumes: `Builder`, `start` (Task 5); the page from Tasks 8 and 10.
- Produces: `remix(tree: TreeNode): Builder`; `/build?from={id}`; a "Remix" link under every
  sentence on the home and permalink pages.

- [ ] **Step 1: Write the failing tests**

`builder.spec.ts`:

```ts
	it('remixes a saved tree with its words, no locks and nothing to keep', () => {
		const saved = structuredClone(sentence.tree);
		saved.children![0].children![0].children![1].locked = true;

		const b = remix(saved);
		expect(nodeAt(b.draft.tree, [0, 0, 1])).toEqual(nodeAt(sentence.tree, [0, 0, 1]));
		expect(nodeAt(b.draft.tree, [0, 0, 1]).locked).toBeUndefined();
		expect(b.draft.filled).toBeNull();
		expect(b.undo).toEqual([]);
		expect(nodeAt(saved, [0, 0, 1]).locked).toBe(true);
	});
```

(add `remix` to the import).

`SentenceView.svelte.spec.ts`:

```ts
	it('links to a remix of the sentence', async () => {
		render(SentenceView, { sentence, grammar, count: 2 });

		await expect
			.element(page.getByRole('link', { name: 'Remix', exact: true }))
			.toHaveAttribute('href', '/build?from=aaaaaaaa');
	});
```

`build/page.e2e.ts` (add `newSentence` to the `e2e` import):

```ts
test('remixes a saved sentence, keeping only after a reroll', async ({ page }) => {
	const s = await newSentence(page.request);
	await page.goto(`/s/${s.id}`);
	await page.getByRole('link', { name: 'Remix', exact: true }).click();

	await expect(page).toHaveURL(`/build?from=${s.id}`);
	await expect(page.getByText(s.text, { exact: true })).toBeVisible();
	const keep = page.getByRole('button', { name: 'Keep this one', exact: true });
	await expect(keep).toBeDisabled();

	await page.getByRole('button', { name: 'Reroll', exact: true }).click();
	await expect(keep).toBeEnabled();
});

test('offers a fresh start when a remixed tree is no longer in the grammar', async ({ page }) => {
	const s = await newSentence(page.request);
	await page.route('**/api/v1/sentences/realize', (route) =>
		route.fulfill({ status: 400, contentType: 'application/json', body: '{"error": "not a rule"}' })
	);
	await page.goto(`/build?from=${s.id}`);

	await page.getByRole('button', { name: 'Reroll', exact: true }).click();
	await expect(page.getByRole('status')).toContainText('isn’t one the grammar makes anymore');
	await page.getByRole('link', { name: 'Start over', exact: true }).click();
	await expect(page.getByRole('button', { name: 'Choose sentence', exact: true })).toBeVisible();
});
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd web && npx vitest --run`
Expected: FAIL: `remix` isn't exported, and there's no Remix link.

- [ ] **Step 3: Implement**

`builder.ts`:

```ts
function unlock(node: TreeNode) {
	delete node.locked;
	node.children?.forEach(unlock);
}

// remix starts from a saved tree and its words, with nothing locked. Only
// realize's own output is signed, so Keep waits for a reroll.
export function remix(tree: TreeNode): Builder {
	const copy = structuredClone(tree);
	unlock(copy);
	return { draft: { tree: copy, filled: null }, undo: [] };
}
```

`build/+page.server.ts`:

```ts
import { getJSON } from '#lib/server/api.js';
import type { Sentence } from '#lib/types.js';
import type { PageServerLoad } from './$types';

// ?from= remixes a saved sentence; without it the builder starts empty.
export const load: PageServerLoad = async ({ url, fetch }) => {
	const from = url.searchParams.get('from');
	return {
		from: from
			? await getJSON<Sentence>(fetch, `/api/v1/sentences/${encodeURIComponent(from)}`)
			: null
	};
};
```

`build/+page.svelte`: the state starts from the remix when there is one, and resets when the
page moves to another (Start over goes from `?from=` to plain `/build`). A writable `$derived`
does both and isn't proxied:

```ts
	// Resets when the page moves to another sentence or to a fresh start.
	// Builder functions return new state, and structuredClone can't copy a
	// deep $state proxy; a derived isn't one.
	let b = $derived<Builder>(data.from ? remix(data.from.tree) : start(data.grammar));
	let stale = $state(false);
```

(This replaces the `$state.raw` line and its comment. Import `remix`.)

In `realize`, before the 422 branch:

```ts
			if (res.status === 400) {
				stale = true;
				return;
			}
```

set `stale = false;` at the top of `realize` and `apply`, and render the message with its link
in place of the plain status line:

```svelte
<p class="problem" role="status">
	{#if stale}
		This sentence’s structure isn’t one the grammar makes anymore. <a href="/build">Start over</a>
	{:else}
		{problem}
	{/if}
</p>
```

`SentenceView.svelte`, after the Show diagram button:

```svelte
<a class="button plain" href="/build?from={sentence.id}">Remix</a>
```

`README.md`, extend the `/build` line: "`/build?from={id}` remixes a saved sentence: its tree and
words, nothing locked, and Keep waits for a reroll."

- [ ] **Step 4: Run the tests to verify they pass**

Run: `cd web && npx prettier --write src && cd .. && just test-fe`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/src README.md
git commit -m "feat: Remix any sentence in the builder"
```
