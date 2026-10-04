# Backend for the Frontend (Stages 1–5) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move the repo into a `service/` + `web/` monorepo. Then give the Go service everything the frontend needs: tree features, saved sentences, stars, flags, a live SSE stream and single-admin auth.

**Architecture:** The Go module moves unchanged into `service/`, and a root Justfile runs it from there. Stage 6 adds `web/`. Generated sentences get a short random ID and are saved as they're served. One migration adds `sentences`, `stars` and `flags`. An in-memory hub goroutine fans events out to SSE clients. Admin auth checks the password against a bcrypt hash in `ADMIN_PASSWORD_HASH` and issues an HMAC-signed expiry cookie. There's no user table.

**Tech Stack:** Go 1.26, chi v5, pgx v5, sqlc v1.31.1, golang-migrate, Postgres 17, just, `golang.org/x/crypto/bcrypt`, `golang.org/x/term`.

**Spec:** `docs/superpowers/specs/2026-10-03-frontend-design.md`. Read it alongside this plan. Project rules are in `CLAUDE.md`, and setup is in `README.md`.

## Global Constraints

- The Go module path stays `github.com/jameynakama/randsense`. The `go` directive stays 1.26, so `new(expr)`, the `omitzero` JSON tag and `sync.WaitGroup.Go` are all available.
- `sqlc` isn't installed. Install the version the generated code was made with: `go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1`. Regenerate with `just generate`, and never hand-edit `*.sql.go`, `models.go` or `querier.go`.
- New Go dependencies are limited to `golang.org/x/crypto` (bcrypt) and `golang.org/x/term`. `github.com/jackc/pgerrcode` goes from indirect to direct.
- New endpoints live under `/api/v1` and answer JSON, except the stream and the bodyless 204s from login and logout.
- List endpoints use limit-offset: default `limit` 30, maximum 100, `offset` ≥ 0, and a bad value gets a 400. Lists are newest first by their own row's `created_at` (a star's for `/stars`, a flag's for `/admin/flags`), with `id` descending as the tie-breaker. Flagged words are the exception: most-flagged first, then by lemma and part of speech. An empty list is `[]`, never `null`.
- A sentence ID is "8 random base-62 characters, generated in Go, retried on collision."
- A flag comment "must be 10 to 1,000 characters after trimming". Count characters, not bytes.
- `word_index` is "zero-based over the tree's leaves in order, commas included. A comma leaf can't be flagged: the server rejects that index with 400."
- The stream sets `Content-Type: text/event-stream`, `Cache-Control: no-cache` and `X-Accel-Buffering: no`, and sends a `:` comment every 25 seconds. Its events are `sentence` (the full sentence JSON) and `stars` (`{id, count}`).
- The session cookie is `HttpOnly`, `Secure` (except in development), `SameSite=Strict`, and expires after 14 days.
- A failed login "waits about a second before answering".
- Each database-backed test package gets its own database suffix (`CLAUDE.md`). The new packages `internal/live` and `internal/auth` don't touch the database.
- Commits use a conventional prefix and a capitalized summary (`feat: Save generated sentences`), with **no `Co-Authored-By` trailer**.
- Comments are sparse and describe what the code does now. No dates, names or history.
- Match each test file's assertion style: `sentence` tests say `expected X; got Y`, and `api` tests say `status: got %d, want %d`.

## Review Focus

These five inputs follow from the spec but no endpoint description covers them. Each has a test in its owning task.

1. **A double-click or racing stars from one voter** should leave the count equal to the number of `stars` rows: one per voter, never two. (Task 7: `TestConcurrentStarsKeepTheCountRight`)
2. **A stream client that stops reading** must not stall `random` or the star endpoints for everyone else. `Publish` must return while a subscriber's buffer is full. (Task 9: `TestPublishDropsASubscriberThatFallsBehind`)
3. **A `word_index` that isn't a flaggable word** (negative, past the end, a comma, `1.5`, `"1"`) should get a 400, never a 500 or a flag with no lemma. (Task 8: `TestFlagRejectsBadWordIndexes`)
4. **A comment padded with spaces or written in non-ASCII text** should be measured in characters after trimming. Ten spaces plus nine letters is too short, and 1,000 `é`s is fine. (Task 8: `TestFlagCommentLength`)
5. **A tree posted to `realize` with `features` already on it** (copied from a saved sentence) should come back with freshly computed features, never the stale ones it was sent. (Task 4: `TestRealizeSentenceRecomputesPostedFeatures`)

---

## Stage 1: Monorepo

### Task 1: Move the Go module into `service/`

The spec says to move the files with `git mv` "in one commit before any other change, so history follows the files". This task makes exactly that commit. Task 2 fixes the recipes and paths.

**Files:**
- Move: `cmd/`, `internal/`, `migrations/`, `data/`, `sqlc.yml`, `.air.toml`, `go.mod`, `go.sum` → `service/`
- Stay at the root: `Justfile`, `README.md`, `CLAUDE.md`, `docs/`, `docker-compose.yml`, `.env`, `.env.example`, `.gitignore`, `.claude/`

- [ ] **Step 1: Confirm a clean, green start**

Run: `git status --short && just test`
Expected: no output from `git status`, and every test passes.

- [ ] **Step 2: Move the files**

```bash
mkdir service
git mv cmd internal migrations data sqlc.yml .air.toml go.mod go.sum service/
rm -rf bin   # ignored build output; it rebuilds under service/bin
```

- [ ] **Step 3: Verify the module builds and tests pass from `service/`**

The Justfile still runs from the root, so run the tests by hand with `.env` loaded:

Run: `cd service && set -a && . ../.env && set +a && go build ./... && go test ./... ; cd ..`
Expected: the build succeeds and every package reports `ok`. The relative paths (`file://../../migrations`, `testdata/`, `data/...` in `cmd/`) resolve because they all moved together.

- [ ] **Step 4: Commit the move alone**

```bash
git add -A
git status --short   # expect only R (rename) entries
git commit -m "refactor: Move the Go module into service/"
```

### Task 2: Root recipes and doc paths

**Files:**
- Modify: `Justfile`, `.gitignore`, `README.md`, `CLAUDE.md`

The web half doesn't exist until stage 6, so `run` and `test` call only the `-be` recipes for now. Stage 6 adds `run-fe` and `test-fe` and makes `run` and `test` start both.

- [ ] **Step 1: Rewrite the Justfile**

```just
set dotenv-load

alias mu := migrate-up
alias md := migrate-down

default: test

# Run all tests
test args="": (test-be args)

# Start everything with hot reload
run: run-be

# Build everything
build: build-be

# Run the Go tests
[working-directory: 'service']
test-be args="":
    gotestsum ./... -- {{ args }}

[working-directory: 'service']
gotest args="":
    go test ./... {{ args }}

[working-directory: 'service']
cover:
    go test -coverprofile=coverage.out -coverpkg=./internal/api/...,./internal/grammar/...,./internal/lexicon/...,./internal/morph/...,./internal/sentence/... ./... && go tool cover -func=coverage.out

# Start the Go server with hot reload
[working-directory: 'service']
run-be:
    air

# Build the Go server binary
[working-directory: 'service']
build-be:
    go build -o bin/randsense ./cmd/server

# Run pending migrations
[working-directory: 'service']
migrate-up:
    migrate -path migrations -database "$DATABASE_URL" up

# Roll back one migration
[working-directory: 'service']
migrate-down num="1":
    migrate -path migrations -database "$DATABASE_URL" down {{ num }}

# Regenerate sqlc types after query changes
[working-directory: 'service']
generate:
    rm -f internal/store/*.sql.go
    sqlc generate

# Ingest the OEWN lexicon and the curated closed-class words into the database
[working-directory: 'service']
ingest:
    go run ./cmd/ingest

@get num="1" commonness="0":
    for i in $(seq {{ num }}); do printf '%s: ' "$i"; http http://localhost:8080/api/v1/sentences/random?commonness={{ commonness }} | jq .text; done
```

`set dotenv-load` reads `.env` from the Justfile's directory (the root) whatever a recipe's working directory is.

- [ ] **Step 2: Anchor the moved ignore patterns**

In `.gitignore`, a pattern with a slash in the middle is anchored to the root, so the data patterns need the new prefix:

```
.env
bin/
coverage.out

.claude/settings.local.json

service/data/**/*.xml
service/data/subtlex-us/*.xlsx
```

- [ ] **Step 3: Update `README.md`**

- After the intro paragraph that ends "...for word frequency.", add: `The Go service lives in \`service/\`; run the \`just\` recipes from the repository root.`
- In the Commands table, replace the `just run` and `just build` rows with these, and keep the other rows:

```
| `just run`                      | Start the app with hot reload            |
| `just run-be`                   | Start the Go server with hot reload      |
| `just test-be`                  | Run the Go tests                         |
| `just build-be`                 | Build binary to `service/bin/randsense`  |
```

- Prefix the data paths with `service/`: `service/data/subtlex-us/subtlex-us-pos.tsv.gz`, `service/data/subtlex-us/convert.py`, `service/data/grammar/grammar.toml`, `service/data/lexicon/verb_morphology*.toml`, and `service/data/lexicon/` in the `just ingest` row.

- [ ] **Step 4: Update `CLAUDE.md` paths**

Change `data/lexicon/` to `service/data/lexicon/` in three places: the morphology-source bullet, the separable-verbs bullet and roadmap item 6. Leave `oewn/frames.go` alone. It names the package, not a path from the root.

- [ ] **Step 5: Verify the recipes**

Run: `just test && just build-be && ls service/bin/randsense`
Expected: every test passes and the binary exists.

Then smoke-test the dev server:
Run: `just run-be` in the background, then `curl -s localhost:8080/health && just get 2`, then stop it.
Expected: `{"status":"ok"}` and two sentences.

- [ ] **Step 6: Commit**

```bash
git add Justfile .gitignore README.md CLAUDE.md
git commit -m "chore: Run recipes from service/ and update paths"
```

### Task 3: Rename the GitHub repo

**This is outward-facing. Ask Jamey before running Step 1.** `jameynakama/randsense` doesn't exist yet. The existing repos are `randsense-service`, `randsense-api`, `randsense-client` and `android_randsense`.

- [ ] **Step 1: Rename and repoint the remote** (after Jamey confirms)

```bash
gh repo rename randsense --repo jameynakama/randsense-service --yes
git remote set-url origin git@github.com:jameynakama/randsense.git
git fetch origin
```

Expected: `git remote -v` shows `randsense.git`, and the fetch succeeds.

- [ ] **Step 2: Mention the local directory to Jamey**

Renaming `~/code/github.com/jameynakama/randsense-service` to `randsense` is Jamey's call. Claude Code keys its project memory on the path, so the memory directory would need moving too. Don't rename it yourself.

---

## Stage 2: Tree features and saved sentences

### Task 4: Tree features

Nodes gain a typed `Features` field, serialized as `features` with empty fields omitted (the spec's "Tree data" section). The root gets `tense` and `commonness`. Generation already works these values out and currently throws them away.

**Files:**
- Modify: `service/internal/grammar/grammar.go` (the `Node` type, plus the new `Features`)
- Modify: `service/internal/grammar/grammar_test.go`
- Modify: `service/internal/sentence/sentence.go`
- Modify: `service/internal/sentence/sentence_test.go`
- Modify: `service/internal/api/handlers.go` (`clearWords`)
- Modify: `service/internal/api/handlers_test.go`
- Modify: `README.md`

**Interfaces:**
- Produces: `grammar.Features` and `grammar.Node.Features` (a value, not a pointer). Tasks 5–8 store and return trees that carry these.

**Interpretation:** only a finite verb gets `tense`, `person` and `number`. Infinitives and gerunds don't agree with anything and carry no tense, so they get `form` plus their lexical features (`frames`, `separable`, `frequency`).

- [ ] **Step 1: Write the failing grammar tests**

Append to `service/internal/grammar/grammar_test.go`:

```go
func TestNodeOmitsEmptyFeatures(t *testing.T) {
	b, err := json.Marshal(&grammar.Node{Symbol: "Comma", Lemma: ",", Word: ","})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	if want := `{"symbol":"Comma","lemma":",","word":","}`; string(b) != want {
		t.Errorf("expected %s; got %s", want, b)
	}
}

func TestNodeKeepsZeroCommonness(t *testing.T) {
	b, err := json.Marshal(&grammar.Node{Symbol: "S", Features: grammar.Features{Commonness: new(0.0)}})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	if want := `{"symbol":"S","features":{"commonness":0}}`; string(b) != want {
		t.Errorf("expected %s; got %s", want, b)
	}
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `just gotest '-run TestNode ./internal/grammar/'`
Expected: a compile failure: `unknown field Features`.

- [ ] **Step 3: Add `Features` to `grammar.go`**

Replace the `Node` type with:

```go
// Features is what generation worked out about a node, for readers digging
// into the tree. Which fields a node gets depends on what it is; empty
// ones are omitted.
type Features struct {
	Tense string `json:"tense,omitempty"`
	// Commonness is the root's commonness floor. It's a pointer because 0
	// is a floor too.
	Commonness *float64 `json:"commonness,omitempty"`
	// Form is a verb's: "finite", "base" or "gerund".
	Form   string `json:"form,omitempty"`
	Person int    `json:"person,omitempty"`
	Number string `json:"number,omitempty"`
	Case   string `json:"case,omitempty"`
	Gender string `json:"gender,omitempty"`
	// Type is a determiner's: "definite", "demonstrative" and so on.
	Type string `json:"type,omitempty"`
	// Frames is every frame a verb's lemma has, not just its slot's.
	Frames    []string `json:"frames,omitempty"`
	Separable bool     `json:"separable,omitempty"`
	// Frequency is the word's SUBTLEX-US Zipf value, absent when it has none.
	Frequency *float64 `json:"frequency,omitempty"`
}

// Node is one constituent of a parse tree. A leaf's Symbol is a POS,
// optionally qualified ("Verb:transitive"). Once the leaf is filled from
// the lexicon, Lemma is the dictionary form and Word the inflected one.
type Node struct {
	Symbol   string   `json:"symbol"`
	Lemma    string   `json:"lemma,omitempty"`
	Word     string   `json:"word,omitempty"`
	Features Features `json:"features,omitzero"`
	Children []*Node  `json:"children,omitempty"`
}
```

- [ ] **Step 4: Run the grammar tests**

Run: `just gotest './internal/grammar/'`
Expected: PASS.

- [ ] **Step 5: Make the sentence fakes return verb frames**

Generation will decode `frames`, which the database column always has (`NOT NULL DEFAULT '[]'`). In `service/internal/sentence/sentence_test.go`, change the fake's verb methods:

```go
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
```

- [ ] **Step 6: Write the failing sentence tests**

Add `"reflect"` and `"github.com/jackc/pgx/v5/pgtype"` to the imports, then append:

```go
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

	if _, err := sentence.Realize(context.Background(), q, tree, loadVerbs(t), newRNG(), 2.5); err != nil {
		t.Fatalf("Realize: %v", err)
	}

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
			inner := node("VP", leaf("Verb"))
			tree := node("S", detNoun(), node("VP", leaf("Verb:to-infinitive"), node(tc.phrase, inner)))

			if _, err := sentence.Realize(context.Background(), newFake(), tree, loadVerbs(t), newRNG(), 0); err != nil {
				t.Fatalf("Realize: %v", err)
			}

			// No tense, person or number: a non-finite verb agrees with nothing.
			want := grammar.Features{Form: tc.form, Frames: []string{"transitive"}}
			if got := inner.Children[0].Features; !reflect.DeepEqual(got, want) {
				t.Errorf("expected %+v; got %+v", want, got)
			}
		})
	}
}

func TestRealizeRecordsPronounFeatures(t *testing.T) {
	q := newFake()
	q.pronouns["nominative"] = store.Pronoun{Lemma: "she", Person: 3, Number: "singular", Gender: "fem"}
	q.pronouns["accusative"] = store.Pronoun{Lemma: "us", Person: 1, Number: "plural", Gender: "epicene"}
	q.pronouns["genitive"] = store.Pronoun{Lemma: "mine", Person: 1, Number: "singular", Gender: "epicene"}
	tree := node("S", node("NP", leaf("Pronoun")), node("VP", leaf("Verb:ditransitive"), node("NP", leaf("Pronoun")), leaf("Pronoun:genitive")))

	if _, err := sentence.Realize(context.Background(), q, tree, loadVerbs(t), newRNG(), 0); err != nil {
		t.Fatalf("Realize: %v", err)
	}

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

	if _, err := sentence.Realize(context.Background(), newFake(), tree, loadVerbs(t), newRNG(), 0); err != nil {
		t.Fatalf("Realize: %v", err)
	}

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
```

- [ ] **Step 7: Run them to verify they fail**

Run: `just gotest '-run "TestRealizeRecords|TestRealizeGivesNoFeatures|TestGenerateRejectsMalformedVerbFrames" ./internal/sentence/'`
Expected: FAIL. The features are empty and the malformed frames get no error.

- [ ] **Step 8: Fill features in `sentence.go`**

Add `"github.com/jackc/pgx/v5/pgtype"` to the imports. Add a field to `leafInfo`:

```go
	features    grammar.Features // what the tree shows about the word before agreement
```

Add the frequency helper below `leafInfo`:

```go
// frequency is a word's Zipf frequency, or nil when SUBTLEX-US lacks it.
func frequency(n pgtype.Numeric) *float64 {
	if !n.Valid {
		return nil
	}
	// A rounded NUMERIC always converts.
	f, _ := n.Float64Value()
	return &f.Float64
}
```

In `fillLeaf`, store the features:

```go
	n.Lemma, n.Word = lemma, lemma
	n.Features = info.features
	gen.leaves[n] = info
```

Replace `randomWord`'s cases for Noun, Verb, Adjective, Adverb, Determiner and Pronoun with these. Preposition, Comma, Complementizer, To and Conjunction stay as they are, so they get no features:

```go
	case grammar.Noun:
		w, err := q.GetRandomNoun(ctx, gen.commonness)
		if err != nil {
			return "", leafInfo{}, err
		}
		var infl struct {
			Plural string `json:"plural"`
		}
		if err := json.Unmarshal(w.Inflections, &infl); err != nil {
			return "", leafInfo{}, fmt.Errorf("noun %q inflections: %w", w.Lemma, err)
		}
		return w.Lemma, leafInfo{
			plural: infl.Plural, pluralLemma: w.Plural,
			features: grammar.Features{Frequency: frequency(w.Frequency)},
		}, nil
	case grammar.Verb:
		var w store.Verb
		var err error
		if frame := n.Qualifier(); frame != "" {
			w, err = q.GetRandomVerbWithFrame(ctx, store.GetRandomVerbWithFrameParams{Frame: frame, Commonness: gen.commonness})
			if errors.Is(err, pgx.ErrNoRows) {
				err = fmt.Errorf("%w: %w", errEmptyFrame, err)
			}
		} else {
			w, err = q.GetRandomVerb(ctx, gen.commonness)
		}
		if err != nil {
			return "", leafInfo{}, err
		}
		var frames []string
		if err := json.Unmarshal(w.Frames, &frames); err != nil {
			return "", leafInfo{}, fmt.Errorf("verb %q frames: %w", w.Lemma, err)
		}
		return w.Lemma, leafInfo{separable: w.Separable, features: grammar.Features{
			Frames: frames, Separable: w.Separable, Frequency: frequency(w.Frequency),
		}}, nil
	case grammar.Adjective:
		w, err := q.GetRandomAdjective(ctx, gen.commonness)
		return w.Lemma, leafInfo{features: grammar.Features{Frequency: frequency(w.Frequency)}}, err
	case grammar.Adverb:
		w, err := q.GetRandomAdverb(ctx, gen.commonness)
		return w.Lemma, leafInfo{features: grammar.Features{Frequency: frequency(w.Frequency)}}, err
	case grammar.Determiner:
		var w store.Determiner
		var err error
		if pluralNoun {
			w, err = q.GetRandomDeterminerWithNumber(ctx, []string{"plural", "either"})
		} else {
			w, err = q.GetRandomDeterminer(ctx)
		}
		return w.Lemma, leafInfo{number: w.Number, features: grammar.Features{Type: w.Type, Number: w.Number}}, err
```

```go
	case grammar.Pronoun:
		pronounCase := "accusative"
		if subject {
			pronounCase = "nominative"
		}
		if n.Qualifier() == grammar.Genitive {
			// "mine" stands for what is owned, not the owner, so it's third
			// person of either number.
			w, err := q.GetRandomPronounWithCase(ctx, grammar.Genitive)
			number := morph.Singular
			if gen.rng.IntN(2) == 1 {
				number = morph.Plural
			}
			return w.Lemma, leafInfo{number: string(number), person: morph.Third, features: grammar.Features{
				Case: grammar.Genitive, Person: int(morph.Third), Number: string(number),
			}}, err
		}
		if word := n.Qualifier(); word != "" {
			return word, leafInfo{number: string(morph.Singular), person: morph.Third, features: grammar.Features{
				Case: pronounCase, Person: int(morph.Third), Number: string(morph.Singular),
			}}, nil
		}
		w, err := q.GetRandomPronounWithCase(ctx, pronounCase)
		return w.Lemma, leafInfo{number: w.Number, person: morph.Person(w.Person), gender: w.Gender, features: grammar.Features{
			Case: pronounCase, Person: int(w.Person), Number: w.Number, Gender: w.Gender,
		}}, err
```

In `agreeNouns`, replace everything after the `if n.Symbol != nounPhrase` return with:

```go
	agr := gen.npAgreement(n)
	gen.agr[n] = agr
	n.Features = grammar.Features{Person: int(agr.person), Number: string(agr.number)}
	for _, c := range n.Children {
		if c.POS() != grammar.Noun || len(c.Children) > 0 {
			continue
		}
		c.Features.Number = string(agr.number)
		if agr.number == morph.Plural && !gen.leaves[c].pluralLemma {
			c.Word = morph.Pluralize(c.Lemma, gen.leaves[c].plural)
		}
	}
```

In `agreeWithSubjects`, record the reflexive's features after its lookup:

```go
			c.Lemma, c.Word = w.Lemma, w.Lemma
			c.Features = grammar.Features{Case: grammar.Reflexive, Person: int(w.Person), Number: w.Number, Gender: w.Gender}
```

and replace the verb `switch form` with:

```go
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
			}
```

In `Realize`, set the root features right after the `agreeWithSubjects` call. Assign the fields rather than replacing the struct, so a one-leaf tree keeps its leaf features:

```go
	tree.Features.Tense = string(gen.tense)
	tree.Features.Commonness = &commonness
```

- [ ] **Step 9: Run the sentence tests**

Run: `just gotest './internal/sentence/'`
Expected: PASS, including every pre-existing test.

- [ ] **Step 10: Write the failing realize-endpoint test (Review Focus 5)**

Append to `service/internal/api/handlers_test.go`:

```go
func TestRealizeSentenceRecomputesPostedFeatures(t *testing.T) {
	seedWords(t)
	srv := newTestServer(t)
	defer srv.Close()

	// As if copied from a saved sentence, with features that no longer
	// apply. Filling replaces a leaf's and an NP's features, but nothing
	// else touches a VP's.
	tree := `{"symbol": "S", "features": {"tense": "future", "commonness": 6}, "children": [
		{"symbol": "NP", "children": [{"symbol": "Determiner"}, {"symbol": "Noun"}]},
		{"symbol": "VP", "features": {"gender": "stale"}, "children": [{"symbol": "Verb"}]}
	]}`
	resp := postTree(t, srv, "", tree)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want 200", resp.StatusCode)
	}
	var body struct {
		Tree grammar.Node `json:"tree"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if tense := body.Tree.Features.Tense; tense != "present" && tense != "past" {
		t.Errorf("tense: got %q, want present or past", tense)
	}
	if c := body.Tree.Features.Commonness; c == nil || *c != 0 {
		t.Errorf("commonness: got %v, want 0", c)
	}
	if g := body.Tree.Children[1].Features.Gender; g != "" {
		t.Errorf("VP gender: got %q, want none", g)
	}
}
```

Run: `just gotest '-run TestRealizeSentenceRecomputesPostedFeatures ./internal/api/'`
Expected: FAIL on the stale VP gender. `Realize` overwrites the root's fields, and filling replaces leaf and NP features, but nothing clears a VP's.

- [ ] **Step 11: Clear posted features in `clearWords`**

In `service/internal/api/handlers.go`:

```go
func clearWords(n *grammar.Node) {
	n.Lemma, n.Word, n.Features = "", "", grammar.Features{}
	for _, c := range n.Children {
		clearWords(c)
	}
}
```

Update the comment on `realizeSentence` to: `// realizeSentence fills a posted tree, in the shape randomSentence returns, with words. Any words and features already in the tree are replaced.`

- [ ] **Step 12: Run everything**

Run: `just test`
Expected: PASS.

- [ ] **Step 13: Document the features in `README.md`**

After the `realize` paragraph ("...returns 422."), add:

```markdown
Every node in a returned tree may carry a `features` object with what generation worked out:
the root's `tense` and `commonness`; an NP's `person` and `number`; a noun's `number`; a verb's
`form` (`finite`, `base` or `gerund`), `frames` (every frame its lemma has) and `separable`,
plus `tense`, `person` and `number` when it's finite; a pronoun's `case`, `person`, `number`
and `gender`; a determiner's `type` and `number`. Content words carry their Zipf `frequency`
when SUBTLEX-US has it. Empty fields are omitted.
```

- [ ] **Step 14: Commit**

```bash
git add service/internal README.md
git commit -m "feat: Record generation features on tree nodes"
```

### Task 5: Save generated sentences

**Files:**
- Create: `service/migrations/007_sentences_stars_flags.up.sql`, `.down.sql`
- Create: `service/internal/store/queries/sentences.sql` (plus the generated `sentences.sql.go`; `models.go` and `querier.go` regenerate)
- Create: `service/internal/api/sentences.go`
- Create: `service/internal/api/sentences_internal_test.go` (package `api`)
- Create: `service/internal/api/helpers_test.go` (package `api_test`)
- Create: `service/internal/api/sentences_test.go` (package `api_test`)
- Modify: `service/internal/api/handlers.go` (`randomSentence`), `service/internal/api/resps.go`
- Modify: `README.md`

**Interfaces:**
- Produces: `store.Sentence{ID string, Text string, Tree []byte, Commonness pgtype.Numeric, StarCount int32, CreatedAt pgtype.Timestamptz}`, `store.InsertSentenceParams{ID, Text string; Tree []byte; Commonness float64}` and `Queries.GetSentence(ctx, id string) (store.Sentence, error)`.
- Produces: `api.SentenceResponse{ID string; Text string; Tree json.RawMessage; StarCount int32; CreatedAt time.Time}` (JSON keys `id, text, tree, star_count, created_at`), plus `sentenceResponse(store.Sentence) SentenceResponse`.
- Produces these test helpers in `helpers_test.go`, used by every later API task: `call`, `decode`, `mustExec`, `resetSentences`, `insertSentence`, `nextID`, `savedTree`.

The spec says "One migration adds" all three tables, so `stars` and `flags` are created here even though Tasks 7 and 8 are the first to use them.

- [ ] **Step 1: Write the migration**

`service/migrations/007_sentences_stars_flags.up.sql`:

```sql
-- id is 8 random base-62 characters made in Go: unlike a sequence, it
-- doesn't reveal how many sentences exist.
CREATE TABLE sentences (
    id          TEXT        PRIMARY KEY,
    text        TEXT        NOT NULL,
    tree        JSONB       NOT NULL,
    commonness  NUMERIC     NOT NULL,
    -- Kept in step with stars in the same statement, so lists never count rows.
    star_count  INTEGER     NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX sentences_created_at_idx ON sentences (created_at DESC, id DESC);

-- voter is a random token the browser keeps, not an account.
CREATE TABLE stars (
    sentence_id TEXT        NOT NULL REFERENCES sentences (id) ON DELETE CASCADE,
    voter       TEXT        NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (sentence_id, voter)
);
CREATE INDEX stars_voter_idx ON stars (voter, created_at DESC);

-- A null word_index flags the whole sentence. lemma and pos are copied from
-- the tree so the most-flagged words are a plain GROUP BY.
CREATE TABLE flags (
    id          BIGSERIAL   PRIMARY KEY,
    sentence_id TEXT        NOT NULL REFERENCES sentences (id) ON DELETE CASCADE,
    word_index  INTEGER,
    lemma       TEXT,
    pos         TEXT,
    comment     TEXT        NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX flags_created_at_idx ON flags (created_at DESC, id DESC);
```

`service/migrations/007_sentences_stars_flags.down.sql`:

```sql
DROP TABLE flags;
DROP TABLE stars;
DROP TABLE sentences;
```

- [ ] **Step 2: Write the queries and generate**

`service/internal/store/queries/sentences.sql`:

```sql
-- name: InsertSentence :one
INSERT INTO sentences (id, text, tree, commonness)
VALUES (@id, @text, @tree, @commonness::float8)
RETURNING *;

-- name: GetSentence :one
SELECT * FROM sentences
WHERE id = $1;
```

Run: `go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 && just generate && just migrate-up`
Expected: `service/internal/store/sentences.sql.go` exists, `models.go` has `Sentence`, `Star` and `Flag`, and the dev database is at version 7. Check that `InsertSentenceParams.Commonness` is a `float64`.

- [ ] **Step 3: Write the failing ID tests**

`service/internal/api/sentences_internal_test.go`:

```go
package api

import (
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/jameynakama/randsense/internal/store"
)

func TestNewSentenceIDIsEightBase62Characters(t *testing.T) {
	pattern := regexp.MustCompile(`^[0-9A-Za-z]{8}$`)
	for range 100 {
		if id := newSentenceID(); !pattern.MatchString(id) {
			t.Fatalf("id: got %q, want 8 base-62 characters", id)
		}
	}
}

// insertTaken fails with a unique violation for IDs starting "taken" and
// records every ID it was given.
func insertTaken(tried *[]string) func(string) (store.Sentence, error) {
	return func(id string) (store.Sentence, error) {
		*tried = append(*tried, id)
		if strings.HasPrefix(id, "taken") {
			return store.Sentence{}, &pgconn.PgError{Code: pgerrcode.UniqueViolation}
		}
		return store.Sentence{ID: id}, nil
	}
}

func TestInsertWithNewIDRetriesTakenIDs(t *testing.T) {
	ids := []string{"taken111", "taken222", "free3333"}
	var tried []string

	s, err := insertWithNewID(func() string { return ids[len(tried)] }, insertTaken(&tried))

	if err != nil || s.ID != "free3333" {
		t.Errorf("got %q, %v; want free3333, nil", s.ID, err)
	}
	if len(tried) != 3 {
		t.Errorf("attempts: got %d, want 3", len(tried))
	}
}

func TestInsertWithNewIDGivesUp(t *testing.T) {
	var tried []string

	_, err := insertWithNewID(func() string { return "takenxxx" }, insertTaken(&tried))

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != pgerrcode.UniqueViolation {
		t.Errorf("err: got %v, want a unique violation", err)
	}
	if len(tried) != maxIDAttempts {
		t.Errorf("attempts: got %d, want %d", len(tried), maxIDAttempts)
	}
}

func TestInsertWithNewIDDoesNotRetryOtherErrors(t *testing.T) {
	boom := errors.New("boom")
	attempts := 0

	_, err := insertWithNewID(newSentenceID, func(string) (store.Sentence, error) {
		attempts++
		return store.Sentence{}, boom
	})

	if !errors.Is(err, boom) || attempts != 1 {
		t.Errorf("got %v after %d attempts; want boom after 1", err, attempts)
	}
}
```

Run: `just gotest '-run "TestNewSentenceID|TestInsertWithNewID" ./internal/api/'`
Expected: a compile failure: `undefined: newSentenceID`.

- [ ] **Step 4: Implement the IDs**

`service/internal/api/sentences.go`:

```go
package api

import (
	"errors"
	"math/rand/v2"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/jameynakama/randsense/internal/store"
)

const (
	idAlphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	idLength   = 8
	// maxIDAttempts bounds retries on an ID collision, which takes a vast
	// table to happen even once.
	maxIDAttempts = 5
)

// newSentenceID is a random base-62 ID.
func newSentenceID() string {
	b := make([]byte, idLength)
	for i := range b {
		b[i] = idAlphabet[rand.IntN(len(idAlphabet))]
	}
	return string(b)
}

// insertWithNewID calls insert with IDs from newID until one isn't taken.
func insertWithNewID(newID func() string, insert func(id string) (store.Sentence, error)) (store.Sentence, error) {
	var s store.Sentence
	var err error
	for range maxIDAttempts {
		s, err = insert(newID())
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != pgerrcode.UniqueViolation {
			return s, err
		}
	}
	return s, err
}
```

Run: `cd service && go mod tidy; cd ..` (this makes `pgerrcode` a direct dependency), then `just gotest '-run "TestNewSentenceID|TestInsertWithNewID" ./internal/api/'`
Expected: PASS.

- [ ] **Step 5: Add the shared API test helpers**

`service/internal/api/helpers_test.go`:

```go
package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// savedTree is a stored tree, "this goose , devours", with its comma leaf
// at index 2.
const savedTree = `{"symbol": "S", "children": [
	{"symbol": "NP", "children": [
		{"symbol": "Determiner", "lemma": "this", "word": "this"},
		{"symbol": "Noun", "lemma": "goose", "word": "goose"}
	]},
	{"symbol": "Comma", "lemma": ",", "word": ","},
	{"symbol": "Verb:intransitive", "lemma": "devour", "word": "devours"}
]}`

var idSeq atomic.Int64

// nextID is a fresh sentence ID for test rows.
func nextID() string {
	return fmt.Sprintf("t%07d", idSeq.Add(1))
}

func mustExec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := testPool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("exec %q: %v", sql, err)
	}
}

// resetSentences empties sentences, and with them stars and flags.
func resetSentences(t *testing.T) {
	t.Helper()
	mustExec(t, "TRUNCATE sentences CASCADE")
}

// insertSentence saves savedTree under id, created at createdAt.
func insertSentence(t *testing.T, id string, createdAt time.Time) {
	t.Helper()
	mustExec(t, "INSERT INTO sentences (id, text, tree, commonness, created_at) VALUES ($1, 'This goose, devours.', $2, 0, $3)",
		id, savedTree, createdAt)
}

// call sends a request with an optional body and headers. The response
// body closes when the test ends.
func call(t *testing.T, srv *httptest.Server, method, path, body string, header http.Header) *http.Response {
	t.Helper()
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, srv.URL+path, r)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	maps.Copy(req.Header, header)
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

// decode checks resp's status and decodes its JSON body into v, unless v
// is nil.
func decode(t *testing.T, resp *http.Response, status int, v any) {
	t.Helper()
	if resp.StatusCode != status {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status: got %d, want %d: %s", resp.StatusCode, status, b)
	}
	if v == nil {
		return
	}
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatalf("decode: %v", err)
	}
}
```

- [ ] **Step 6: Write the failing save tests**

`service/internal/api/sentences_test.go`:

```go
package api_test

import (
	"context"
	"net/http"
	"regexp"
	"testing"

	"github.com/jameynakama/randsense/internal/api"
)

func TestRandomSentenceIsSaved(t *testing.T) {
	seedWords(t)
	seedRareNoun(t)
	mustExec(t, "UPDATE verbs SET frequency = 3.03")
	srv := newTestServer(t)
	defer srv.Close()

	resp := call(t, srv, http.MethodGet, "/api/v1/sentences/random?commonness=3", "", nil)
	var body api.SentenceResponse
	decode(t, resp, http.StatusOK, &body)

	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("Access-Control-Allow-Origin: got %q, want *", got)
	}
	if !regexp.MustCompile(`^[0-9A-Za-z]{8}$`).MatchString(body.ID) {
		t.Errorf("id: got %q, want 8 base-62 characters", body.ID)
	}
	if body.CreatedAt.IsZero() || body.StarCount != 0 || len(body.Tree) == 0 {
		t.Errorf("body: got %+v, want created_at, star_count 0 and a tree", body)
	}
	var text string
	var commonness float64
	err := testPool.QueryRow(context.Background(), "SELECT text, commonness::float8 FROM sentences WHERE id = $1", body.ID).
		Scan(&text, &commonness)
	if err != nil {
		t.Fatalf("saved row: %v", err)
	}
	if text != body.Text || commonness != 3 {
		t.Errorf("saved: got %q at %v, want %q at 3", text, commonness, body.Text)
	}
}

func TestRealizedSentenceIsNotSaved(t *testing.T) {
	seedWords(t)
	resetSentences(t)
	srv := newTestServer(t)
	defer srv.Close()

	resp := postTree(t, srv, "", realizeTree)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want 200", resp.StatusCode)
	}

	var n int
	if err := testPool.QueryRow(context.Background(), "SELECT count(*) FROM sentences").Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Errorf("saved sentences: got %d, want 0", n)
	}
}
```

Run: `just gotest '-run "TestRandomSentenceIsSaved|TestRealizedSentenceIsNotSaved" ./internal/api/'`
Expected: a compile failure: `undefined: api.SentenceResponse`.

- [ ] **Step 7: Save in `randomSentence`**

Append to `service/internal/api/resps.go` (and add `"time"` to its imports):

```go
type SentenceResponse struct {
	ID        string          `json:"id"`
	Text      string          `json:"text"`
	Tree      json.RawMessage `json:"tree"`
	StarCount int32           `json:"star_count"`
	CreatedAt time.Time       `json:"created_at"`
}

func sentenceResponse(s store.Sentence) SentenceResponse {
	return SentenceResponse{ID: s.ID, Text: s.Text, Tree: s.Tree, StarCount: s.StarCount, CreatedAt: s.CreatedAt.Time}
}
```

Replace `randomSentence` in `service/internal/api/handlers.go`. Add `"github.com/jameynakama/randsense/internal/store"` to its imports.

```go
// randomSentence generates a sentence, saves it and returns it. Any site
// may call it: signatures on other sites embed it.
func (h *Handler) randomSentence(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	c, err := commonness(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	// A *rand.Rand isn't safe for concurrent use, so each request gets its own.
	rng := rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
	s, err := sentence.Generate(r.Context(), h.queries, h.grammar, h.verbs, rng, c)
	if err != nil {
		log.Printf("randomSentence: %v", err)
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	tree, err := json.Marshal(s.Tree)
	if err != nil {
		log.Printf("randomSentence: %v", err)
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	saved, err := insertWithNewID(newSentenceID, func(id string) (store.Sentence, error) {
		return h.queries.InsertSentence(r.Context(), store.InsertSentenceParams{ID: id, Text: s.Text, Tree: tree, Commonness: c})
	})
	if err != nil {
		log.Printf("randomSentence: save: %v", err)
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	writeJSON(w, http.StatusOK, sentenceResponse(saved))
}
```

- [ ] **Step 8: Run all tests**

Run: `just test`
Expected: PASS. The older `TestRandomSentence*` tests still decode `text` and `tree` from the bigger response.

- [ ] **Step 9: Update `README.md`**

In the API block, change the `random` line to:

```
GET /api/v1/sentences/random[?commonness=N]   -> {id, text, tree, star_count, created_at}
```

Below the block, add: `\`random\` saves each sentence it returns and allows any origin, so other sites can embed it. \`realize\` saves nothing.`

- [ ] **Step 10: Commit**

```bash
git add service README.md
git commit -m "feat: Save generated sentences under short random IDs"
```

### Task 6: List and single-sentence endpoints

**Files:**
- Modify: `service/internal/store/queries/sentences.sql` (then regenerate)
- Modify: `service/internal/api/sentences.go`, `service/internal/api/router.go`
- Modify: `service/internal/api/setup_test.go` (keep the test database URL)
- Modify: `service/internal/api/handlers_test.go` (move `newTestServer` out)
- Modify: `service/internal/api/helpers_test.go` (add `newServer`)
- Modify: `service/internal/api/sentences_test.go`
- Create: `service/internal/api/performance_test.go`
- Modify: `README.md`

**Interfaces:**
- Consumes: `store.Sentence`, `sentenceResponse` and the helpers from Task 5.
- Produces: `page(r *http.Request) (limit, offset int32, err error)`, `sentenceResponses([]store.Sentence) []SentenceResponse` and `writeNotFound`. Produces `newServer(t, api.RouterConfig) *httptest.Server` and the `listEndpoints` table in `performance_test.go`, which Tasks 7 and 12 extend.

- [ ] **Step 1: Add the queries and generate**

Append to `service/internal/store/queries/sentences.sql`:

```sql
-- name: ListSentences :many
SELECT * FROM sentences
ORDER BY created_at DESC, id DESC
LIMIT @page_limit OFFSET @page_offset;
```

Run: `just generate`
Expected: `ListSentencesParams{PageLimit int32, PageOffset int32}` in `sentences.sql.go`.

- [ ] **Step 2: Let tests build servers with their own config**

In `setup_test.go`, add a package variable `var testDBURL string` next to `testPool`. In `TestMain`, change `testDBURL := getRequiredEnvVar(...)` to `testDBURL = getRequiredEnvVar(...)`, so the counting pool in Step 4 can connect.

Delete `newTestServer` from `handlers_test.go` and add both of these to `helpers_test.go`, with the imports they need (`strings` is already there; add the `grammar`, `morph`, `store` and `api` packages):

```go
// newServer serves the API with cfg, filling in the test grammar and verbs,
// and testPool unless cfg has its own queries.
func newServer(t *testing.T, cfg api.RouterConfig) *httptest.Server {
	t.Helper()
	g, err := grammar.Load(strings.NewReader(testGrammar))
	if err != nil {
		t.Fatalf("grammar.Load: %v", err)
	}
	v, err := morph.LoadVerbs(strings.NewReader(""))
	if err != nil {
		t.Fatalf("morph.LoadVerbs: %v", err)
	}
	if cfg.Queries == nil {
		cfg.Queries = store.New(testPool)
	}
	cfg.Grammar, cfg.Verbs = g, v
	return httptest.NewServer(api.NewRouter(cfg))
}

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return newServer(t, api.RouterConfig{})
}
```

Remove whatever imports `handlers_test.go` no longer uses.

- [ ] **Step 3: Write the failing endpoint tests**

Append to `sentences_test.go` (add `"time"` to the imports):

```go
func TestListSentencesNewestFirst(t *testing.T) {
	resetSentences(t)
	t0 := time.Now().Add(-time.Hour)
	insertSentence(t, "aaaaaaaa", t0)
	insertSentence(t, "bbbbbbbb", t0.Add(time.Second))
	insertSentence(t, "cccccccc", t0.Add(2*time.Second))
	// Same time as cccccccc: the id breaks the tie.
	insertSentence(t, "dddddddd", t0.Add(2*time.Second))
	srv := newTestServer(t)
	defer srv.Close()

	tests := []struct {
		query string
		want  []string
	}{
		{"", []string{"dddddddd", "cccccccc", "bbbbbbbb", "aaaaaaaa"}},
		{"?limit=2&offset=1", []string{"cccccccc", "bbbbbbbb"}},
		{"?offset=4", []string{}},
	}
	for _, tc := range tests {
		t.Run(tc.query, func(t *testing.T) {
			var body []api.SentenceResponse
			decode(t, call(t, srv, http.MethodGet, "/api/v1/sentences"+tc.query, "", nil), http.StatusOK, &body)

			if body == nil {
				t.Fatal("body: got null, want an array")
			}
			got := make([]string, len(body))
			for i, s := range body {
				got[i] = s.ID
			}
			if !slices.Equal(got, tc.want) {
				t.Errorf("ids: got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestListSentencesRejectsBadPages(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	for _, q := range []string{"limit=0", "limit=101", "limit=ten", "offset=-1", "offset=1.5", "offset=99999999999"} {
		t.Run(q, func(t *testing.T) {
			decode(t, call(t, srv, http.MethodGet, "/api/v1/sentences?"+q, "", nil), http.StatusBadRequest, nil)
		})
	}
}

func TestGetSentence(t *testing.T) {
	resetSentences(t)
	insertSentence(t, "aaaaaaaa", time.Now())
	srv := newTestServer(t)
	defer srv.Close()

	var body api.SentenceResponse
	decode(t, call(t, srv, http.MethodGet, "/api/v1/sentences/aaaaaaaa", "", nil), http.StatusOK, &body)
	if body.ID != "aaaaaaaa" || body.Text != "This goose, devours." {
		t.Errorf("body: got %+v, want aaaaaaaa", body)
	}

	decode(t, call(t, srv, http.MethodGet, "/api/v1/sentences/zzzzzzzz", "", nil), http.StatusNotFound, nil)
}
```

Add `"slices"` to the imports.

Run: `just gotest '-run "TestListSentences|TestGetSentence" ./internal/api/'`
Expected: FAIL with 404 or 405 statuses, because the routes don't exist yet.

- [ ] **Step 4: Write the failing N+1 test**

`service/internal/api/performance_test.go`:

```go
package api_test

import (
	"context"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jameynakama/randsense/internal/api"
	"github.com/jameynakama/randsense/internal/store"
)

// queryCounter counts the queries a pool runs.
type queryCounter struct{ n atomic.Int64 }

func (c *queryCounter) TraceQueryStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryStartData) context.Context {
	c.n.Add(1)
	return ctx
}

func (c *queryCounter) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func countingServer(t *testing.T) (*httptest.Server, *queryCounter) {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(testDBURL)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	c := &queryCounter{}
	cfg.ConnConfig.Tracer = c
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewWithConfig: %v", err)
	}
	t.Cleanup(pool.Close)
	srv := newServer(t, api.RouterConfig{Queries: store.New(pool)})
	t.Cleanup(srv.Close)
	return srv, c
}

// listEndpoints are checked for N+1 queries. seed adds n more rows the
// endpoint lists.
var listEndpoints = []struct {
	name   string
	path   string
	header http.Header
	seed   func(t *testing.T, n int)
}{
	{"sentences", "/api/v1/sentences", nil, func(t *testing.T, n int) {
		for range n {
			insertSentence(t, nextID(), time.Now())
		}
	}},
}

func TestListEndpointsRunOneQueryWhateverTheRowCount(t *testing.T) {
	for _, e := range listEndpoints {
		t.Run(e.name, func(t *testing.T) {
			resetSentences(t)
			srv, c := countingServer(t)
			queries := func() int64 {
				c.n.Store(0)
				decode(t, call(t, srv, http.MethodGet, e.path, "", e.header), http.StatusOK, nil)
				return c.n.Load()
			}

			e.seed(t, 2+rand.IntN(5))
			first := queries()
			e.seed(t, 2+rand.IntN(5))
			second := queries()

			if first != 1 || second != 1 {
				t.Errorf("queries: got %d then %d, want 1 both times", first, second)
			}
		})
	}
}
```

- [ ] **Step 5: Implement the endpoints**

Append to `service/internal/api/sentences.go`, adding the imports it needs (`fmt`, `log`, `net/http`, `strconv`, `chi`, `pgx`):

```go
const (
	defaultPageLimit = 30
	maxPageLimit     = 100
)

// page parses a list endpoint's limit and offset query params.
func page(r *http.Request) (limit, offset int32, err error) {
	limit = defaultPageLimit
	q := r.URL.Query()
	if v := q.Get("limit"); v != "" {
		n, err := strconv.ParseInt(v, 10, 32)
		if err != nil || n < 1 || n > maxPageLimit {
			return 0, 0, fmt.Errorf("limit must be a whole number from 1 to %d", maxPageLimit)
		}
		limit = int32(n)
	}
	if v := q.Get("offset"); v != "" {
		n, err := strconv.ParseInt(v, 10, 32)
		if err != nil || n < 0 {
			return 0, 0, errors.New("offset must be a whole number from 0")
		}
		offset = int32(n)
	}
	return limit, offset, nil
}

// sentenceResponses never returns nil, so an empty list encodes as [].
func sentenceResponses(rows []store.Sentence) []SentenceResponse {
	resp := make([]SentenceResponse, len(rows))
	for i, s := range rows {
		resp[i] = sentenceResponse(s)
	}
	return resp
}

func writeNotFound(w http.ResponseWriter) {
	writeError(w, http.StatusNotFound, "no sentence with that id")
}

func (h *Handler) listSentences(w http.ResponseWriter, r *http.Request) {
	limit, offset, err := page(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	rows, err := h.queries.ListSentences(r.Context(), store.ListSentencesParams{PageLimit: limit, PageOffset: offset})
	if err != nil {
		log.Printf("listSentences: %v", err)
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	writeJSON(w, http.StatusOK, sentenceResponses(rows))
}

func (h *Handler) getSentence(w http.ResponseWriter, r *http.Request) {
	s, err := h.queries.GetSentence(r.Context(), chi.URLParam(r, "id"))
	if errors.Is(err, pgx.ErrNoRows) {
		writeNotFound(w)
		return
	}
	if err != nil {
		log.Printf("getSentence: %v", err)
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	writeJSON(w, http.StatusOK, sentenceResponse(s))
}
```

In `router.go`, route them. chi matches static segments before `{id}`:

```go
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/words/random", h.randomWord)
		r.Get("/sentences", h.listSentences)
		r.Get("/sentences/random", h.randomSentence)
		r.Post("/sentences/realize", h.realizeSentence)
		r.Get("/sentences/{id}", h.getSentence)
	})
```

- [ ] **Step 6: Run all tests**

Run: `just test`
Expected: PASS.

- [ ] **Step 7: Update `README.md`**

Add these to the API block:

```
GET /api/v1/sentences[?limit=30&offset=0]     -> [{id, text, tree, star_count, created_at}]
GET /api/v1/sentences/{id}                    -> {id, text, tree, star_count, created_at}
```

Below the block, add: `List endpoints return newest first, \`limit\` 1 to 100 (default 30).`

- [ ] **Step 8: Commit**

```bash
git add service README.md
git commit -m "feat: List saved sentences and fetch one by ID"
```

---

## Stage 3: Stars and flags

### Task 7: Stars

**Files:**
- Create: `service/internal/store/queries/stars.sql` (then regenerate)
- Create: `service/internal/api/stars.go`, `service/internal/api/stars_test.go`
- Modify: `service/internal/api/router.go`, `service/internal/api/performance_test.go`, `README.md`

**Interfaces:**
- Consumes: `page`, `sentenceResponses` and `writeNotFound` (Task 6), and the test helpers.
- Produces: `Queries.AddStar(ctx, AddStarParams{SentenceID, Voter string}) (int32, error)`, `Queries.RemoveStar(ctx, RemoveStarParams{SentenceID, Voter string}) (int32, error)` and `Queries.ListStarredSentences(ctx, ListStarredSentencesParams{Voter string; PageLimit, PageOffset int32}) ([]store.Sentence, error)`. Produces `changeStar`, which Task 10 extends to broadcast, and the test constants `voterA` and `voterB`.

- [ ] **Step 1: Write the queries and generate**

`service/internal/store/queries/stars.sql`:

```sql
-- name: AddStar :one
-- One statement, so star_count and stars change together. Starring twice
-- inserts nothing and adds 0. An unknown sentence fails the foreign key.
WITH added AS (
    INSERT INTO stars (sentence_id, voter)
    VALUES (@sentence_id, @voter)
    ON CONFLICT DO NOTHING
    RETURNING 1
)
UPDATE sentences SET star_count = star_count + (SELECT count(*) FROM added)
WHERE id = @sentence_id
RETURNING star_count;

-- name: RemoveStar :one
-- An unknown sentence returns no rows.
WITH removed AS (
    DELETE FROM stars
    WHERE sentence_id = @sentence_id AND voter = @voter
    RETURNING 1
)
UPDATE sentences SET star_count = star_count - (SELECT count(*) FROM removed)
WHERE id = @sentence_id
RETURNING star_count;

-- name: ListStarredSentences :many
SELECT sentences.* FROM sentences
JOIN stars ON stars.sentence_id = sentences.id
WHERE stars.voter = @voter
ORDER BY stars.created_at DESC, sentences.id DESC
LIMIT @page_limit OFFSET @page_offset;
```

Run: `just generate`
Expected: `AddStar` and `RemoveStar` return `int32`, and `ListStarredSentences` returns `[]Sentence`. sqlc reuses the table model when the columns match it exactly.

- [ ] **Step 2: Write the failing tests**

`service/internal/api/stars_test.go`:

```go
package api_test

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jameynakama/randsense/internal/api"
)

const (
	voterA = "0b9e7d3c-4f1a-4c2b-8e5d-6a7f8b9c0d1e"
	voterB = "5c6d7e8f-9a0b-4c1d-8e2f-3a4b5c6d7e8f"
)

func star(t *testing.T, srv *httptest.Server, method, id, voter string) *http.Response {
	t.Helper()
	return call(t, srv, method, "/api/v1/sentences/"+id+"/stars", "", http.Header{"X-Voter": {voter}})
}

func TestStarsCountOncePerVoter(t *testing.T) {
	resetSentences(t)
	insertSentence(t, "aaaaaaaa", time.Now())
	srv := newTestServer(t)
	defer srv.Close()

	steps := []struct {
		method, voter string
		want          int32
	}{
		{http.MethodPost, voterA, 1},
		{http.MethodPost, voterA, 1},
		{http.MethodPost, strings.ToUpper(voterA), 1},
		{http.MethodPost, voterB, 2},
		{http.MethodDelete, voterA, 1},
		{http.MethodDelete, voterA, 1},
	}
	for i, s := range steps {
		var body struct {
			Count int32 `json:"count"`
		}
		decode(t, star(t, srv, s.method, "aaaaaaaa", s.voter), http.StatusOK, &body)
		if body.Count != s.want {
			t.Errorf("step %d (%s by %s): count got %d, want %d", i, s.method, s.voter, body.Count, s.want)
		}
	}

	var saved api.SentenceResponse
	decode(t, call(t, srv, http.MethodGet, "/api/v1/sentences/aaaaaaaa", "", nil), http.StatusOK, &saved)
	if saved.StarCount != 1 {
		t.Errorf("star_count: got %d, want 1", saved.StarCount)
	}
}

func TestStarsRejectBadRequests(t *testing.T) {
	resetSentences(t)
	insertSentence(t, "aaaaaaaa", time.Now())
	srv := newTestServer(t)
	defer srv.Close()

	for _, method := range []string{http.MethodPost, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			decode(t, star(t, srv, method, "aaaaaaaa", ""), http.StatusBadRequest, nil)
			decode(t, star(t, srv, method, "aaaaaaaa", "not-a-uuid"), http.StatusBadRequest, nil)
			decode(t, star(t, srv, method, "zzzzzzzz", voterA), http.StatusNotFound, nil)
		})
	}
	decode(t, call(t, srv, http.MethodGet, "/api/v1/stars", "", nil), http.StatusBadRequest, nil)
}

// Review Focus 1: a double-click or racing voters.
func TestConcurrentStarsKeepTheCountRight(t *testing.T) {
	tests := []struct {
		name  string
		voter func(i int) string
		want  int32
	}{
		{"one voter", func(int) string { return voterA }, 1},
		{"many voters", func(i int) string { return fmt.Sprintf("00000000-0000-4000-8000-%012d", i) }, 10},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resetSentences(t)
			insertSentence(t, "aaaaaaaa", time.Now())
			srv := newTestServer(t)
			defer srv.Close()

			var wg sync.WaitGroup
			for i := range 10 {
				wg.Go(func() {
					req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/sentences/aaaaaaaa/stars", nil)
					req.Header.Set("X-Voter", tc.voter(i))
					resp, err := srv.Client().Do(req)
					if err != nil {
						t.Errorf("POST: %v", err)
						return
					}
					resp.Body.Close()
					if resp.StatusCode != http.StatusOK {
						t.Errorf("status: got %d, want 200", resp.StatusCode)
					}
				})
			}
			wg.Wait()

			var count, rows int32
			err := testPool.QueryRow(context.Background(),
				"SELECT star_count, (SELECT count(*) FROM stars WHERE sentence_id = 'aaaaaaaa') FROM sentences WHERE id = 'aaaaaaaa'").
				Scan(&count, &rows)
			if err != nil {
				t.Fatalf("query: %v", err)
			}
			if count != tc.want || rows != tc.want {
				t.Errorf("star_count %d, stars rows %d; want %d", count, rows, tc.want)
			}
		})
	}
}

func TestListStarsNewestStarFirst(t *testing.T) {
	resetSentences(t)
	for _, id := range []string{"aaaaaaaa", "bbbbbbbb", "cccccccc"} {
		insertSentence(t, id, time.Now())
	}
	srv := newTestServer(t)
	defer srv.Close()
	decode(t, star(t, srv, http.MethodPost, "aaaaaaaa", voterA), http.StatusOK, nil)
	decode(t, star(t, srv, http.MethodPost, "bbbbbbbb", voterA), http.StatusOK, nil)
	decode(t, star(t, srv, http.MethodPost, "cccccccc", voterB), http.StatusOK, nil)
	// Star time, not sentence time, orders the list.
	mustExec(t, "UPDATE stars SET created_at = now() + interval '1 hour' WHERE sentence_id = 'aaaaaaaa'")

	tests := []struct {
		query string
		want  []string
	}{
		{"", []string{"aaaaaaaa", "bbbbbbbb"}},
		{"?limit=1&offset=1", []string{"bbbbbbbb"}},
	}
	for _, tc := range tests {
		t.Run(tc.query, func(t *testing.T) {
			var body []api.SentenceResponse
			decode(t, call(t, srv, http.MethodGet, "/api/v1/stars"+tc.query, "", http.Header{"X-Voter": {voterA}}), http.StatusOK, &body)
			got := make([]string, len(body))
			for i, s := range body {
				got[i] = s.ID
			}
			if !slices.Equal(got, tc.want) {
				t.Errorf("ids: got %v, want %v", got, tc.want)
			}
		})
	}
}
```

Add `"net/http/httptest"` to the imports.

Run: `just gotest '-run "TestStars|TestConcurrentStars|TestListStars" ./internal/api/'`
Expected: FAIL with 404 or 405 statuses, because the routes don't exist yet.

- [ ] **Step 3: Implement**

`service/internal/api/stars.go`:

```go
package api

import (
	"context"
	"errors"
	"log"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/jameynakama/randsense/internal/store"
)

// voterPattern matches the random UUID a browser keeps to star with.
var voterPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// voter is the X-Voter header, lowercased so its case doesn't matter.
func voter(r *http.Request) (string, bool) {
	v := r.Header.Get("X-Voter")
	return strings.ToLower(v), voterPattern.MatchString(v)
}

const badVoter = "X-Voter header must be a UUID"

func (h *Handler) addStar(w http.ResponseWriter, r *http.Request) {
	h.changeStar(w, r, func(ctx context.Context, id, voter string) (int32, error) {
		return h.queries.AddStar(ctx, store.AddStarParams{SentenceID: id, Voter: voter})
	})
}

func (h *Handler) removeStar(w http.ResponseWriter, r *http.Request) {
	h.changeStar(w, r, func(ctx context.Context, id, voter string) (int32, error) {
		return h.queries.RemoveStar(ctx, store.RemoveStarParams{SentenceID: id, Voter: voter})
	})
}

// changeStar stars or unstars a sentence with change and answers with its
// new count. Both are idempotent.
func (h *Handler) changeStar(w http.ResponseWriter, r *http.Request, change func(ctx context.Context, id, voter string) (int32, error)) {
	v, ok := voter(r)
	if !ok {
		writeError(w, http.StatusBadRequest, badVoter)
		return
	}
	count, err := change(r.Context(), chi.URLParam(r, "id"), v)
	var pgErr *pgconn.PgError
	if errors.Is(err, pgx.ErrNoRows) || (errors.As(err, &pgErr) && pgErr.Code == pgerrcode.ForeignKeyViolation) {
		writeNotFound(w)
		return
	}
	if err != nil {
		log.Printf("changeStar: %v", err)
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]int32{"count": count})
}

func (h *Handler) listStars(w http.ResponseWriter, r *http.Request) {
	v, ok := voter(r)
	if !ok {
		writeError(w, http.StatusBadRequest, badVoter)
		return
	}
	limit, offset, err := page(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	rows, err := h.queries.ListStarredSentences(r.Context(), store.ListStarredSentencesParams{Voter: v, PageLimit: limit, PageOffset: offset})
	if err != nil {
		log.Printf("listStars: %v", err)
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	writeJSON(w, http.StatusOK, sentenceResponses(rows))
}
```

Add the routes to `router.go`:

```go
		r.Post("/sentences/{id}/stars", h.addStar)
		r.Delete("/sentences/{id}/stars", h.removeStar)
		r.Get("/stars", h.listStars)
```

- [ ] **Step 4: Add the N+1 entry**

Append to `listEndpoints` in `performance_test.go`:

```go
	{"starred sentences", "/api/v1/stars", http.Header{"X-Voter": {voterA}}, func(t *testing.T, n int) {
		for range n {
			id := nextID()
			insertSentence(t, id, time.Now())
			mustExec(t, "INSERT INTO stars (sentence_id, voter) VALUES ($1, $2)", id, voterA)
		}
	}},
```

- [ ] **Step 5: Run all tests**

Run: `just test`
Expected: PASS.

- [ ] **Step 6: Update `README.md`**

Add these to the API block:

```
POST   /api/v1/sentences/{id}/stars           -> {count}   (X-Voter: <uuid>)
DELETE /api/v1/sentences/{id}/stars           -> {count}   (X-Voter: <uuid>)
GET    /api/v1/stars[?limit&offset]           -> [sentence] starred by X-Voter, newest star first
```

Below the block, add: `The voter token is a random UUID the browser keeps. Starring and unstarring are idempotent.`

- [ ] **Step 7: Commit**

```bash
git add service README.md
git commit -m "feat: Star and unstar sentences anonymously"
```

### Task 8: Flags

**Files:**
- Modify: `service/internal/grammar/grammar.go` (add `LeafNodes`), `service/internal/grammar/grammar_test.go`
- Modify: `service/internal/sentence/sentence.go` (use `LeafNodes` and delete its private copy)
- Create: `service/internal/store/queries/flags.sql` (then regenerate)
- Create: `service/internal/api/flags.go`, `service/internal/api/flags_test.go`
- Modify: `service/internal/api/router.go`, `README.md`

**Interfaces:**
- Produces: `(*grammar.Node).LeafNodes() []*grammar.Node` and `Queries.InsertFlag(ctx, InsertFlagParams{SentenceID string; WordIndex pgtype.Int4; Lemma, Pos pgtype.Text; Comment string}) (int64, error)`.

- [ ] **Step 1: Write the failing `LeafNodes` test**

Append to `grammar_test.go`:

```go
func TestLeafNodesAreInSentenceOrder(t *testing.T) {
	det, noun, verb := &grammar.Node{Symbol: "Determiner"}, &grammar.Node{Symbol: "Noun"}, &grammar.Node{Symbol: "Verb"}
	tree := &grammar.Node{Symbol: "S", Children: []*grammar.Node{
		{Symbol: "NP", Children: []*grammar.Node{det, noun}},
		verb,
	}}

	if got := tree.LeafNodes(); !slices.Equal(got, []*grammar.Node{det, noun, verb}) {
		t.Errorf("expected determiner, noun, verb; got %v", got)
	}
}
```

Run: `just gotest '-run TestLeafNodes ./internal/grammar/'`
Expected: a compile failure.

- [ ] **Step 2: Move `leafNodes` into `grammar`**

Add to `grammar.go`, after `Leaves`:

```go
// LeafNodes returns the tree's leaves in sentence order.
func (n *Node) LeafNodes() []*Node {
	if len(n.Children) == 0 {
		return []*Node{n}
	}
	var leaves []*Node
	for _, c := range n.Children {
		leaves = append(leaves, c.LeafNodes()...)
	}
	return leaves
}
```

In `sentence.go`, delete the `leafNodes` function. Then replace `leafNodes(tree)` with `tree.LeafNodes()` in `Realize`, and `leafNodes(n.Children[i+1])` with `n.Children[i+1].LeafNodes()` in `separateParticles`.

Run: `just gotest './internal/grammar/ ./internal/sentence/'`
Expected: PASS.

- [ ] **Step 3: Write the query and generate**

`service/internal/store/queries/flags.sql`:

```sql
-- name: InsertFlag :one
INSERT INTO flags (sentence_id, word_index, lemma, pos, comment)
VALUES ($1, $2, $3, $4, $5)
RETURNING id;
```

Run: `just generate`

- [ ] **Step 4: Write the failing tests**

`service/internal/api/flags_test.go`:

```go
package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func flagBody(comment, wordIndex string) string {
	c, _ := json.Marshal(comment)
	if wordIndex == "" {
		return fmt.Sprintf(`{"comment": %s}`, c)
	}
	return fmt.Sprintf(`{"comment": %s, "word_index": %s}`, c, wordIndex)
}

func postFlag(t *testing.T, srv *httptest.Server, id, body string) *http.Response {
	t.Helper()
	return call(t, srv, http.MethodPost, "/api/v1/sentences/"+id+"/flags", body, http.Header{"Content-Type": {"application/json"}})
}

type savedFlag struct {
	wordIndex  *int32
	lemma, pos *string
	comment    string
}

func getFlag(t *testing.T, id int64) savedFlag {
	t.Helper()
	var f savedFlag
	err := testPool.QueryRow(context.Background(), "SELECT word_index, lemma, pos, comment FROM flags WHERE id = $1", id).
		Scan(&f.wordIndex, &f.lemma, &f.pos, &f.comment)
	if err != nil {
		t.Fatalf("flag %d: %v", id, err)
	}
	return f
}

func TestFlagSentenceOrWord(t *testing.T) {
	resetSentences(t)
	insertSentence(t, "aaaaaaaa", time.Now())
	srv := newTestServer(t)
	defer srv.Close()
	const comment = "this makes no sense at all"

	tests := []struct {
		name, wordIndex  string
		wantLemma, wantPOS string // empty means null
	}{
		{"whole sentence", "", "", ""},
		{"determiner", "0", "this", "Determiner"},
		{"noun", "1", "goose", "Noun"},
		{"framed verb", "3", "devour", "Verb"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var body struct {
				ID int64 `json:"id"`
			}
			decode(t, postFlag(t, srv, "aaaaaaaa", flagBody(comment, tc.wordIndex)), http.StatusCreated, &body)

			f := getFlag(t, body.ID)
			deref := func(s *string) string {
				if s == nil {
					return ""
				}
				return *s
			}
			if deref(f.lemma) != tc.wantLemma || deref(f.pos) != tc.wantPOS || f.comment != comment {
				t.Errorf("flag: got lemma %q pos %q comment %q, want %q %q %q",
					deref(f.lemma), deref(f.pos), f.comment, tc.wantLemma, tc.wantPOS, comment)
			}
			if (tc.wordIndex == "") != (f.wordIndex == nil) {
				t.Errorf("word_index: got %v for %q", f.wordIndex, tc.wordIndex)
			}
		})
	}
}

// Review Focus 3.
func TestFlagRejectsBadWordIndexes(t *testing.T) {
	resetSentences(t)
	insertSentence(t, "aaaaaaaa", time.Now())
	srv := newTestServer(t)
	defer srv.Close()

	for _, idx := range []string{"2", "-1", "4", "1.5", `"1"`, "null"} {
		t.Run(idx, func(t *testing.T) {
			resp := postFlag(t, srv, "aaaaaaaa", flagBody("this makes no sense at all", idx))
			// null is no index: the whole sentence.
			want := http.StatusBadRequest
			if idx == "null" {
				want = http.StatusCreated
			}
			decode(t, resp, want, nil)
		})
	}
}

// Review Focus 4: characters after trimming, not bytes.
func TestFlagCommentLength(t *testing.T) {
	resetSentences(t)
	insertSentence(t, "aaaaaaaa", time.Now())
	srv := newTestServer(t)
	defer srv.Close()

	tests := []struct {
		name, comment string
		want          int
	}{
		{"exactly 10 after trimming", "   0123456789   ", http.StatusCreated},
		{"9 after trimming", "          012345678          ", http.StatusBadRequest},
		{"1000 two-byte characters", strings.Repeat("é", 1000), http.StatusCreated},
		{"1001 characters", strings.Repeat("a", 1001), http.StatusBadRequest},
		{"empty", "", http.StatusBadRequest},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			decode(t, postFlag(t, srv, "aaaaaaaa", flagBody(tc.comment, "")), tc.want, nil)
		})
	}
}

func TestFlagRejectsBadRequests(t *testing.T) {
	resetSentences(t)
	insertSentence(t, "aaaaaaaa", time.Now())
	srv := newTestServer(t)
	defer srv.Close()

	decode(t, postFlag(t, srv, "zzzzzzzz", flagBody("this makes no sense at all", "")), http.StatusNotFound, nil)
	decode(t, postFlag(t, srv, "aaaaaaaa", "not json"), http.StatusBadRequest, nil)
}
```

Run: `just gotest '-run TestFlag ./internal/api/'`
Expected: FAIL with 404 or 405 statuses.

- [ ] **Step 5: Implement**

`service/internal/api/flags.go`:

```go
package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/jameynakama/randsense/internal/grammar"
	"github.com/jameynakama/randsense/internal/store"
)

const (
	minCommentLen = 10
	maxCommentLen = 1000
	maxFlagBytes  = 16 << 10
)

// flagSentence records a complaint about a sentence, or one word in it.
// word_index counts the tree's leaves in order, commas included, but a
// comma can't be flagged.
func (h *Handler) flagSentence(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Comment   string `json:"comment"`
		WordIndex *int   `json:"word_index"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxFlagBytes)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "body must be JSON with a comment and an optional whole-number word_index")
		return
	}
	comment := strings.TrimSpace(body.Comment)
	if n := utf8.RuneCountInString(comment); n < minCommentLen || n > maxCommentLen {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("comment must be %d to %d characters", minCommentLen, maxCommentLen))
		return
	}

	s, err := h.queries.GetSentence(r.Context(), chi.URLParam(r, "id"))
	if errors.Is(err, pgx.ErrNoRows) {
		writeNotFound(w)
		return
	}
	if err != nil {
		log.Printf("flagSentence: %v", err)
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}

	params := store.InsertFlagParams{SentenceID: s.ID, Comment: comment}
	if body.WordIndex != nil {
		var tree grammar.Node
		if err := json.Unmarshal(s.Tree, &tree); err != nil {
			log.Printf("flagSentence: tree of %s: %v", s.ID, err)
			writeError(w, http.StatusInternalServerError, "server error")
			return
		}
		leaves := tree.LeafNodes()
		i := *body.WordIndex
		if i < 0 || i >= len(leaves) || leaves[i].POS() == grammar.Comma {
			writeError(w, http.StatusBadRequest, "word_index must point at a word in the sentence")
			return
		}
		params.WordIndex = pgtype.Int4{Int32: int32(i), Valid: true}
		params.Lemma = pgtype.Text{String: leaves[i].Lemma, Valid: true}
		params.Pos = pgtype.Text{String: string(leaves[i].POS()), Valid: true}
	}

	id, err := h.queries.InsertFlag(r.Context(), params)
	if err != nil {
		log.Printf("flagSentence: %v", err)
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]int64{"id": id})
}
```

Add the route to `router.go`: `r.Post("/sentences/{id}/flags", h.flagSentence)`.

- [ ] **Step 6: Run all tests**

Run: `just test`
Expected: PASS.

- [ ] **Step 7: Update `README.md`**

Add this to the API block:

```
POST   /api/v1/sentences/{id}/flags           {comment, word_index?} -> 201 {id}
```

Below the block, add: `A flag's comment is 10 to 1,000 characters after trimming. \`word_index\` counts the tree's leaves from 0, commas included; a comma can't be flagged, and without an index the flag is for the whole sentence.`

- [ ] **Step 8: Commit**

```bash
git add service README.md
git commit -m "feat: Flag a sentence or one of its words"
```

---

## Stage 4: Live stream

### Task 9: Broadcast hub

**Files:**
- Create: `service/internal/live/hub.go`, `service/internal/live/hub_test.go`
- Modify: `Justfile` (add `./internal/live/...` to `cover`'s `-coverpkg`)

**Interfaces:**
- Produces: `live.Event{Name string; Data []byte}`, `live.NewHub() *Hub`, `(*Hub).Run()` (blocks; start it with `go`), `(*Hub).Subscribe() (<-chan Event, func())` and `(*Hub).Publish(Event)`. A subscriber's channel is closed when it unsubscribes or falls behind.

- [ ] **Step 1: Write the failing tests**

`service/internal/live/hub_test.go`:

```go
package live_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/jameynakama/randsense/internal/live"
)

func newHub() *live.Hub {
	h := live.NewHub()
	go h.Run()
	return h
}

// receive waits briefly for the next event; ok is false once the channel
// is closed.
func receive(t *testing.T, events <-chan live.Event) (live.Event, bool) {
	t.Helper()
	select {
	case e, ok := <-events:
		return e, ok
	case <-time.After(time.Second):
		t.Fatal("expected an event or a close; got nothing")
		return live.Event{}, false
	}
}

func TestPublishReachesEverySubscriber(t *testing.T) {
	h := newHub()
	a, _ := h.Subscribe()
	b, _ := h.Subscribe()

	h.Publish(live.Event{Name: "sentence", Data: []byte(`{}`)})

	for name, events := range map[string]<-chan live.Event{"a": a, "b": b} {
		if e, ok := receive(t, events); !ok || e.Name != "sentence" {
			t.Errorf("%s: expected the sentence event; got %+v, open %v", name, e, ok)
		}
	}
}

// Review Focus 2: a stalled client mustn't stall publishing.
func TestPublishDropsASubscriberThatFallsBehind(t *testing.T) {
	h := newHub()
	slow, _ := h.Subscribe()

	// Publish returns each time even though nobody reads.
	for i := range live.ClientBuffer + 1 {
		h.Publish(live.Event{Name: "sentence", Data: fmt.Appendf(nil, "%d", i)})
	}

	for i := range live.ClientBuffer {
		if e, ok := receive(t, slow); !ok || string(e.Data) != fmt.Sprint(i) {
			t.Fatalf("event %d: got %+v, open %v", i, e, ok)
		}
	}
	if _, ok := receive(t, slow); ok {
		t.Error("expected the slow subscriber's channel closed")
	}
}

func TestUnsubscribeClosesAndStopsDelivery(t *testing.T) {
	h := newHub()
	events, unsubscribe := h.Subscribe()

	unsubscribe()
	h.Publish(live.Event{Name: "stars"})

	if e, ok := receive(t, events); ok {
		t.Errorf("expected a closed channel; got %+v", e)
	}
}

func TestUnsubscribeAfterBeingDroppedIsSafe(t *testing.T) {
	h := newHub()
	events, unsubscribe := h.Subscribe()
	for range live.ClientBuffer + 1 {
		h.Publish(live.Event{Name: "sentence"})
	}
	for range live.ClientBuffer {
		receive(t, events)
	}
	receive(t, events) // closed by the drop

	unsubscribe() // would panic on a second close
	h.Publish(live.Event{Name: "sentence"})
}
```

Run: `just gotest './internal/live/'`
Expected: a compile failure. The package doesn't exist yet.

- [ ] **Step 2: Implement**

`service/internal/live/hub.go`:

```go
// Package live fans events out to every open stream, from memory. A second
// server process would need to publish through Postgres LISTEN/NOTIFY.
package live

// ClientBuffer is how many events a subscriber can fall behind by before
// it's dropped. A dropped browser reconnects and refetches.
const ClientBuffer = 16

// Event is one server-sent event: its name and its JSON data.
type Event struct {
	Name string
	Data []byte
}

// Hub keeps the set of subscribers in one goroutine, Run.
type Hub struct {
	register   chan chan Event
	unregister chan chan Event
	publish    chan Event
}

func NewHub() *Hub {
	return &Hub{
		register:   make(chan chan Event),
		unregister: make(chan chan Event),
		publish:    make(chan Event),
	}
}

// Run serves subscriptions and publishes until the process exits.
func (h *Hub) Run() {
	clients := map[chan Event]struct{}{}
	drop := func(c chan Event) {
		if _, ok := clients[c]; ok {
			delete(clients, c)
			close(c)
		}
	}
	for {
		select {
		case c := <-h.register:
			clients[c] = struct{}{}
		case c := <-h.unregister:
			drop(c)
		case e := <-h.publish:
			for c := range clients {
				select {
				case c <- e:
				default:
					drop(c)
				}
			}
		}
	}
}

// Subscribe returns a channel of every event published from now on, and a
// function to stop. The channel closes on stopping or on falling more than
// ClientBuffer events behind.
func (h *Hub) Subscribe() (<-chan Event, func()) {
	c := make(chan Event, ClientBuffer)
	h.register <- c
	return c, func() { h.unregister <- c }
}

// Publish sends e to every subscriber without waiting on any of them.
func (h *Hub) Publish(e Event) {
	h.publish <- e
}
```

- [ ] **Step 3: Run the tests, with the race detector**

Run: `just gotest '-race ./internal/live/'`
Expected: PASS.

- [ ] **Step 4: Add the package to coverage**

In the Justfile's `cover` recipe, append `,./internal/live/...` to the `-coverpkg` list.

- [ ] **Step 5: Commit**

```bash
git add service/internal/live Justfile
git commit -m "feat: Add an in-memory hub that broadcasts live events"
```

### Task 10: Stream endpoint and broadcasting

**Files:**
- Create: `service/internal/api/stream.go`, `service/internal/api/stream_test.go`
- Modify: `service/internal/api/router.go` (hub, keepalive, route), `service/internal/api/handlers.go` (`randomSentence`), `service/internal/api/stars.go` (`changeStar`)
- Modify: `README.md`

**Interfaces:**
- Consumes: `live.Hub` (Task 9) and `changeStar` (Task 7).
- Produces: `RouterConfig.Keepalive time.Duration` (zero means 25 seconds; only tests set it), `(*Handler).publish(name string, v any)` and the `StarsEvent{ID string; Count int32}` type.

- [ ] **Step 1: Write the failing tests**

`service/internal/api/stream_test.go`:

```go
package api_test

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jameynakama/randsense/internal/api"
)

// openStream connects to the event stream. It closes when the test ends,
// before the server does: register t.Cleanup(srv.Close) before calling it,
// since Close waits for open requests.
func openStream(t *testing.T, srv *httptest.Server) (*http.Response, *bufio.Reader) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/v1/sentences/stream", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("GET stream: %v", err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp, bufio.NewReader(resp.Body)
}

func readLine(t *testing.T, r *bufio.Reader) string {
	t.Helper()
	line, err := r.ReadString('\n')
	if err != nil {
		t.Fatalf("read stream: %v", err)
	}
	return strings.TrimSuffix(line, "\n")
}

// nextEvent reads up to the end of the next event, skipping keepalives.
func nextEvent(t *testing.T, r *bufio.Reader) (name, data string) {
	t.Helper()
	for {
		line := readLine(t, r)
		switch {
		case strings.HasPrefix(line, "event: "):
			name = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			data = strings.TrimPrefix(line, "data: ")
		case line == "" && name != "":
			return name, data
		}
	}
}

func TestStreamHeaders(t *testing.T) {
	srv := newTestServer(t)
	t.Cleanup(srv.Close)

	resp, _ := openStream(t, srv)

	want := map[string]string{
		"Content-Type":      "text/event-stream",
		"Cache-Control":     "no-cache",
		"X-Accel-Buffering": "no",
	}
	for k, v := range want {
		if got := resp.Header.Get(k); got != v {
			t.Errorf("%s: got %q, want %q", k, got, v)
		}
	}
}

func TestStreamSendsGeneratedSentences(t *testing.T) {
	seedWords(t)
	srv := newTestServer(t)
	t.Cleanup(srv.Close)
	_, events := openStream(t, srv)

	var generated api.SentenceResponse
	decode(t, call(t, srv, http.MethodGet, "/api/v1/sentences/random", "", nil), http.StatusOK, &generated)

	name, data := nextEvent(t, events)
	var streamed api.SentenceResponse
	if err := json.Unmarshal([]byte(data), &streamed); err != nil {
		t.Fatalf("data %q: %v", data, err)
	}
	if name != "sentence" || streamed.ID != generated.ID || streamed.Text != generated.Text {
		t.Errorf("event: got %s %+v, want sentence %s", name, streamed, generated.ID)
	}
}

func TestStreamSendsStarCounts(t *testing.T) {
	resetSentences(t)
	insertSentence(t, "aaaaaaaa", time.Now())
	srv := newTestServer(t)
	t.Cleanup(srv.Close)
	_, events := openStream(t, srv)

	for _, method := range []string{http.MethodPost, http.MethodDelete} {
		decode(t, star(t, srv, method, "aaaaaaaa", voterA), http.StatusOK, nil)
	}

	for _, want := range []string{`{"id":"aaaaaaaa","count":1}`, `{"id":"aaaaaaaa","count":0}`} {
		if name, data := nextEvent(t, events); name != "stars" || data != want {
			t.Errorf("event: got %s %s, want stars %s", name, data, want)
		}
	}
}

func TestStreamSendsKeepalives(t *testing.T) {
	srv := newServer(t, api.RouterConfig{Keepalive: 10 * time.Millisecond})
	t.Cleanup(srv.Close)
	_, events := openStream(t, srv)

	if line := readLine(t, events); line != ":" {
		t.Errorf("first line: got %q, want the keepalive comment", line)
	}
}
```

Run: `just gotest '-run TestStream ./internal/api/'`
Expected: FAIL. `Keepalive` is undefined, and the stream route doesn't exist.

- [ ] **Step 2: Wire the hub into the router**

In `router.go`, add `"time"` and the `live` package to the imports, then:

```go
// defaultKeepalive is how often an idle stream sends a comment, so proxies
// don't close it.
const defaultKeepalive = 25 * time.Second

type RouterConfig struct {
	Queries *store.Queries
	Grammar *grammar.Grammar
	Verbs   *morph.Verbs
	// Keepalive overrides defaultKeepalive.
	Keepalive time.Duration
}

type Handler struct {
	queries   *store.Queries
	grammar   *grammar.Grammar
	verbs     *morph.Verbs
	hub       *live.Hub
	keepalive time.Duration
}

func NewRouter(cfg RouterConfig) http.Handler {
	hub := live.NewHub()
	go hub.Run()
	h := &Handler{
		queries:   cfg.Queries,
		grammar:   cfg.Grammar,
		verbs:     cfg.Verbs,
		hub:       hub,
		keepalive: cmp.Or(cfg.Keepalive, defaultKeepalive),
	}
	// ...middleware unchanged...
```

Add `"cmp"` to the imports. Add the route next to `/sentences/random`: `r.Get("/sentences/stream", h.stream)`.

- [ ] **Step 3: Implement the stream and publishing**

`service/internal/api/stream.go`:

```go
package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/jameynakama/randsense/internal/live"
)

// StarsEvent is a sentence's new star count.
type StarsEvent struct {
	ID    string `json:"id"`
	Count int32  `json:"count"`
}

// publish sends v to every open stream as event name.
func (h *Handler) publish(name string, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		log.Printf("publish %s: %v", name, err)
		return
	}
	h.hub.Publish(live.Event{Name: name, Data: data})
}

// stream sends sentence and stars events as Server-Sent Events. There's no
// replay: a reconnecting browser refetches the latest sentences.
func (h *Handler) stream(w http.ResponseWriter, r *http.Request) {
	// Subscribe before answering, so a client holding the headers misses
	// nothing published after.
	events, unsubscribe := h.hub.Subscribe()
	defer unsubscribe()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	// nginx buffers responses unless told not to.
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	rc := http.NewResponseController(w)
	if err := rc.Flush(); err != nil {
		return
	}

	keepalive := time.NewTicker(h.keepalive)
	defer keepalive.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case e, ok := <-events:
			if !ok {
				return // dropped for falling behind; the browser reconnects
			}
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", e.Name, e.Data)
		case <-keepalive.C:
			fmt.Fprint(w, ":\n\n")
		}
		if err := rc.Flush(); err != nil {
			return
		}
	}
}
```

In `randomSentence`, publish before answering:

```go
	resp := sentenceResponse(saved)
	h.publish("sentence", resp)
	writeJSON(w, http.StatusOK, resp)
```

In `changeStar`, publish before answering:

```go
	id := chi.URLParam(r, "id")
	count, err := change(r.Context(), id, v)
	// ...error handling unchanged...
	h.publish("stars", StarsEvent{ID: id, Count: count})
	writeJSON(w, http.StatusOK, map[string]int32{"count": count})
```

- [ ] **Step 4: Run all tests, with the race detector**

Run: `just test -race`
Expected: PASS.

- [ ] **Step 5: Try it by hand**

Run `just run-be` in the background. Then run `curl -N localhost:8080/api/v1/sentences/stream` in a second background shell, and `just get 2`.
Expected: curl prints two `event: sentence` blocks. Stop both.

- [ ] **Step 6: Update `README.md`**

Add this to the API block:

```
GET    /api/v1/sentences/stream               Server-Sent Events: sentence, stars
```

Below the block, add: `The stream sends \`sentence\` (the full sentence) whenever \`random\` saves one and \`stars\` (\`{id, count}\`) after every star or unstar, even one that leaves the count unchanged, with a \`:\` keepalive every 25 seconds. It doesn't replay missed events.`

- [ ] **Step 7: Commit**

```bash
git add service README.md
git commit -m "feat: Stream new sentences and star counts over SSE"
```

---

## Stage 5: Admin

### Task 11: Signed session values

**Files:**
- Create: `service/internal/auth/session.go`, `service/internal/auth/session_test.go`
- Modify: `Justfile` (add `./internal/auth/...` to `cover`'s `-coverpkg`)

**Interfaces:**
- Produces: `auth.SessionTTL` (14 days), `auth.CookieName`, `auth.NewSession(secret []byte, expires time.Time) string` and `auth.ValidSession(secret []byte, value string, now time.Time) bool`.

- [ ] **Step 1: Write the failing tests**

`service/internal/auth/session_test.go`:

```go
package auth_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jameynakama/randsense/internal/auth"
)

var secret = []byte("0123456789abcdef0123456789abcdef")

func TestValidSession(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	valid := auth.NewSession(secret, now.Add(time.Hour))
	exp, sig, _ := strings.Cut(valid, ".")

	tests := []struct {
		name, value string
		secret      []byte
		want        bool
	}{
		{"unexpired", valid, secret, true},
		{"expired", auth.NewSession(secret, now.Add(-time.Second)), secret, false},
		{"expiring now", auth.NewSession(secret, now), secret, false},
		{"signed with another secret", valid, []byte("another-secret-another-secret-xx"), false},
		{"expiry pushed later", fmt.Sprint(now.Add(48*time.Hour).Unix()) + "." + sig, secret, false},
		{"signature tampered", exp + "." + strings.Repeat("A", len(sig)), secret, false},
		{"signature not base64", exp + ".!!!", secret, false},
		{"expiry not a number", "soon." + sig, secret, false},
		{"no signature", exp, secret, false},
		{"empty", "", secret, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := auth.ValidSession(tc.secret, tc.value, now); got != tc.want {
				t.Errorf("expected %v; got %v", tc.want, got)
			}
		})
	}
}
```

Run: `just gotest './internal/auth/'`
Expected: a compile failure.

- [ ] **Step 2: Implement**

`service/internal/auth/session.go`:

```go
// Package auth signs the single admin's session: a cookie holding an
// expiry time, signed with HMAC-SHA256. There's no session table, so
// changing the secret logs every session out.
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strconv"
	"strings"
	"time"
)

const (
	CookieName = "randsense_admin"
	SessionTTL = 14 * 24 * time.Hour
)

// NewSession is a cookie value that's valid until expires.
func NewSession(secret []byte, expires time.Time) string {
	exp := strconv.FormatInt(expires.Unix(), 10)
	return exp + "." + base64.RawURLEncoding.EncodeToString(mac(secret, exp))
}

// ValidSession reports whether value was signed with secret and hasn't
// expired by now.
func ValidSession(secret []byte, value string, now time.Time) bool {
	exp, sig, ok := strings.Cut(value, ".")
	if !ok {
		return false
	}
	unix, err := strconv.ParseInt(exp, 10, 64)
	if err != nil {
		return false
	}
	got, err := base64.RawURLEncoding.DecodeString(sig)
	return err == nil && hmac.Equal(got, mac(secret, exp)) && now.Before(time.Unix(unix, 0))
}

func mac(secret []byte, msg string) []byte {
	m := hmac.New(sha256.New, secret)
	m.Write([]byte(msg))
	return m.Sum(nil)
}
```

- [ ] **Step 3: Run the tests**

Run: `just gotest './internal/auth/'`
Expected: PASS.

- [ ] **Step 4: Add the package to coverage, then commit**

Append `,./internal/auth/...` to the `-coverpkg` list in the Justfile.

```bash
git add service/internal/auth Justfile
git commit -m "feat: Sign admin session expiries with HMAC"
```

### Task 12: Admin login, admin endpoints and `just hash-password`

**Files:**
- Create: `service/internal/api/admin.go`, `service/internal/api/admin_test.go`
- Create: `service/cmd/hashpassword/main.go`
- Modify: `service/internal/store/queries/flags.sql` (then regenerate)
- Modify: `service/internal/api/router.go`, `service/internal/api/helpers_test.go` (`newServer` fills in `Admin`), `service/internal/api/performance_test.go`
- Modify: `service/cmd/server/main.go`, `service/go.mod`, `service/go.sum`
- Modify: `Justfile`, `.env.example`, `README.md`

**Interfaces:**
- Consumes: `auth` (Task 11), `page`, and the `listEndpoints` table.
- Produces: `api.AdminConfig{PasswordHash []byte; SessionSecret []byte; InsecureCookies bool}` and `RouterConfig.Admin`. Produces the queries `ListFlags` (rows: `ID int64, SentenceID string, WordIndex pgtype.Int4, Lemma pgtype.Text, Pos pgtype.Text, Comment string, CreatedAt pgtype.Timestamptz, SentenceText string, SentenceTree []byte`) and `ListFlaggedWords` (rows: `Lemma pgtype.Text, Pos pgtype.Text, Count int64`).

- [ ] **Step 1: Add the dependencies**

Run: `cd service && go get golang.org/x/crypto/bcrypt golang.org/x/term && cd ..`

- [ ] **Step 2: Add the admin queries and generate**

Append to `service/internal/store/queries/flags.sql`:

```sql
-- name: ListFlags :many
SELECT flags.id, flags.sentence_id, flags.word_index, flags.lemma, flags.pos, flags.comment, flags.created_at,
       sentences.text AS sentence_text, sentences.tree AS sentence_tree
FROM flags
JOIN sentences ON sentences.id = flags.sentence_id
ORDER BY flags.created_at DESC, flags.id DESC
LIMIT @page_limit OFFSET @page_offset;

-- name: ListFlaggedWords :many
SELECT lemma, pos, count(*) AS count
FROM flags
WHERE lemma IS NOT NULL
GROUP BY lemma, pos
ORDER BY count DESC, lemma, pos
LIMIT @page_limit OFFSET @page_offset;
```

Run: `just generate`
Expected: `ListFlagsRow` and `ListFlaggedWordsRow` with the fields listed under Interfaces.

- [ ] **Step 3: Give test servers an admin**

In `helpers_test.go`, add `"time"`, `"golang.org/x/crypto/bcrypt"` and the `auth` package to the imports, then add:

```go
const testPassword = "correct horse battery"

var testSecret = []byte("test-session-secret-32-bytes-xxx")

// adminCookie is a session cookie, as a Cookie header, that's valid until
// expires.
func adminCookie(expires time.Time) http.Header {
	c := &http.Cookie{Name: auth.CookieName, Value: auth.NewSession(testSecret, expires)}
	return http.Header{"Cookie": {c.String()}}
}
```

In `newServer`, before building the router:

```go
	if cfg.Admin.PasswordHash == nil {
		hash, err := bcrypt.GenerateFromPassword([]byte(testPassword), bcrypt.MinCost)
		if err != nil {
			t.Fatalf("bcrypt: %v", err)
		}
		cfg.Admin = api.AdminConfig{PasswordHash: hash, SessionSecret: testSecret, InsecureCookies: cfg.Admin.InsecureCookies}
	}
```

- [ ] **Step 4: Write the failing auth tests**

`service/internal/api/admin_test.go`:

```go
package api_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/jameynakama/randsense/internal/api"
	"github.com/jameynakama/randsense/internal/auth"
)

func login(t *testing.T, srv *httptest.Server, body string) *http.Response {
	t.Helper()
	return call(t, srv, http.MethodPost, "/api/v1/admin/login", body, http.Header{"Content-Type": {"application/json"}})
}

func sessionCookie(t *testing.T, resp *http.Response) *http.Cookie {
	t.Helper()
	for _, c := range resp.Cookies() {
		if c.Name == auth.CookieName {
			return c
		}
	}
	t.Fatal("expected a session cookie; got none")
	return nil
}

func TestLoginSetsASessionCookie(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	resp := login(t, srv, `{"password": "`+testPassword+`"}`)
	decode(t, resp, http.StatusNoContent, nil)

	c := sessionCookie(t, resp)
	if !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteStrictMode || c.Path != "/" || c.MaxAge != int(auth.SessionTTL.Seconds()) {
		t.Errorf("cookie: got %+v, want HttpOnly, Secure, SameSite=Strict, Path=/, 14 days", c)
	}
	if !auth.ValidSession(testSecret, c.Value, time.Now()) {
		t.Errorf("cookie value %q isn't a valid session", c.Value)
	}
}

func TestLoginCookieIsInsecureInDevelopment(t *testing.T) {
	srv := newServer(t, api.RouterConfig{Admin: api.AdminConfig{InsecureCookies: true}})
	defer srv.Close()

	resp := login(t, srv, `{"password": "`+testPassword+`"}`)
	decode(t, resp, http.StatusNoContent, nil)

	if sessionCookie(t, resp).Secure {
		t.Error("cookie: got Secure, want plain http to work")
	}
}

func TestLoginFailuresAreSlowAndSetNoCookie(t *testing.T) {
	srv := newTestServer(t)
	t.Cleanup(srv.Close)

	for name, body := range map[string]string{
		"wrong password": `{"password": "wrong"}`,
		"no password":    `{}`,
		"not JSON":       `password`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			start := time.Now()
			resp := login(t, srv, body)
			decode(t, resp, http.StatusUnauthorized, nil)

			if elapsed := time.Since(start); elapsed < 900*time.Millisecond {
				t.Errorf("answered after %v, want about a second", elapsed)
			}
			if len(resp.Cookies()) != 0 {
				t.Errorf("cookies: got %v, want none", resp.Cookies())
			}
		})
	}
}

func TestAdminRoutesNeedAValidSession(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()
	tampered := adminCookie(time.Now().Add(time.Hour))
	tampered.Set("Cookie", tampered.Get("Cookie")+"x")
	other := &http.Cookie{Name: auth.CookieName, Value: auth.NewSession([]byte("another-secret-another-secret-xx"), time.Now().Add(time.Hour))}

	sessions := map[string]http.Header{
		"no cookie":    nil,
		"tampered":     tampered,
		"expired":      adminCookie(time.Now().Add(-time.Second)),
		"other secret": {"Cookie": {other.String()}},
	}
	routes := []struct{ method, path string }{
		{http.MethodPost, "/api/v1/admin/logout"},
		{http.MethodGet, "/api/v1/admin/flags"},
		{http.MethodGet, "/api/v1/admin/flagged-words"},
	}
	for name, header := range sessions {
		for _, r := range routes {
			t.Run(name+" "+r.path, func(t *testing.T) {
				decode(t, call(t, srv, r.method, r.path, "", header), http.StatusUnauthorized, nil)
			})
		}
	}
}

func TestLogoutClearsTheCookie(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	resp := call(t, srv, http.MethodPost, "/api/v1/admin/logout", "", adminCookie(time.Now().Add(time.Hour)))
	decode(t, resp, http.StatusNoContent, nil)

	if c := sessionCookie(t, resp); c.MaxAge >= 0 || c.Value != "" {
		t.Errorf("cookie: got %+v, want it cleared", c)
	}
}
```

Add `"net/http/httptest"` to the imports.

Run: `just gotest '-run "TestLogin|TestAdminRoutes|TestLogout" ./internal/api/'`
Expected: a compile failure: `undefined: api.AdminConfig`.

- [ ] **Step 5: Implement auth in the API**

In `router.go`, add the config type and wiring:

```go
// AdminConfig is how the single admin logs in.
type AdminConfig struct {
	PasswordHash  []byte // bcrypt
	SessionSecret []byte // HMAC key for session cookies
	// InsecureCookies lets the session cookie work over plain http, for
	// development.
	InsecureCookies bool
}
```

Add `Admin AdminConfig` to `RouterConfig`, set `admin: cfg.Admin` in `NewRouter`, and add `admin AdminConfig` to `Handler`. Inside `/api/v1`, add:

```go
		r.Route("/admin", func(r chi.Router) {
			r.Post("/login", h.login)
			r.Group(func(r chi.Router) {
				r.Use(h.requireAdmin)
				r.Post("/logout", h.logout)
				r.Get("/flags", h.listFlags)
				r.Get("/flagged-words", h.listFlaggedWords)
			})
		})
```

`service/internal/api/admin.go`:

```go
package api

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/bcrypt"

	"github.com/jameynakama/randsense/internal/auth"
	"github.com/jameynakama/randsense/internal/store"
)

// loginFailureDelay slows password guessing.
const loginFailureDelay = time.Second

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Password string `json:"password"`
	}
	err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&body)
	if err != nil || bcrypt.CompareHashAndPassword(h.admin.PasswordHash, []byte(body.Password)) != nil {
		time.Sleep(loginFailureDelay)
		writeError(w, http.StatusUnauthorized, "wrong password")
		return
	}
	value := auth.NewSession(h.admin.SessionSecret, time.Now().Add(auth.SessionTTL))
	http.SetCookie(w, h.sessionCookie(value, int(auth.SessionTTL.Seconds())))
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, h.sessionCookie("", -1))
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) sessionCookie(value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     auth.CookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   !h.admin.InsecureCookies,
		SameSite: http.SameSiteStrictMode,
	}
}

func (h *Handler) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(auth.CookieName)
		if err != nil || !auth.ValidSession(h.admin.SessionSecret, c.Value, time.Now()) {
			writeError(w, http.StatusUnauthorized, "admin login required")
			return
		}
		next.ServeHTTP(w, r)
	})
}
```

The router now names `listFlags` and `listFlaggedWords`, which Step 7 adds, so the package compiles again only after Step 7.

- [ ] **Step 6: Write the failing admin endpoint tests**

Append to `admin_test.go` (add `"slices"` to the imports):

```go
func TestAdminListsFlagsNewestFirst(t *testing.T) {
	resetSentences(t)
	insertSentence(t, "aaaaaaaa", time.Now())
	t0 := time.Now().Add(-time.Hour)
	mustExec(t, "INSERT INTO flags (sentence_id, comment, created_at) VALUES ('aaaaaaaa', 'older whole sentence', $1)", t0)
	mustExec(t, "INSERT INTO flags (sentence_id, word_index, lemma, pos, comment, created_at) VALUES ('aaaaaaaa', 1, 'goose', 'Noun', 'newer goose', $1)", t0.Add(time.Minute))
	srv := newTestServer(t)
	defer srv.Close()

	var body []struct {
		WordIndex *int32  `json:"word_index"`
		Lemma     *string `json:"lemma"`
		Comment   string  `json:"comment"`
		Sentence  struct {
			ID   string          `json:"id"`
			Text string          `json:"text"`
			Tree json.RawMessage `json:"tree"`
		} `json:"sentence"`
	}
	decode(t, call(t, srv, http.MethodGet, "/api/v1/admin/flags", "", adminCookie(time.Now().Add(time.Hour))), http.StatusOK, &body)

	if len(body) != 2 || body[0].Comment != "newer goose" || body[1].Comment != "older whole sentence" {
		t.Fatalf("flags: got %+v, want newer goose then older whole sentence", body)
	}
	if body[0].WordIndex == nil || *body[0].WordIndex != 1 || body[0].Lemma == nil || *body[0].Lemma != "goose" {
		t.Errorf("word flag: got %+v, want index 1, lemma goose", body[0])
	}
	if body[1].WordIndex != nil || body[1].Lemma != nil {
		t.Errorf("sentence flag: got %+v, want null index and lemma", body[1])
	}
	if s := body[0].Sentence; s.ID != "aaaaaaaa" || s.Text != "This goose, devours." || len(s.Tree) == 0 {
		t.Errorf("sentence: got %+v, want aaaaaaaa with its text and tree", s)
	}
}

func TestAdminListsMostFlaggedWords(t *testing.T) {
	resetSentences(t)
	insertSentence(t, "aaaaaaaa", time.Now())
	for _, f := range []struct {
		idx        int
		lemma, pos string
	}{{1, "goose", "Noun"}, {3, "devour", "Verb"}, {1, "goose", "Noun"}} {
		mustExec(t, "INSERT INTO flags (sentence_id, word_index, lemma, pos, comment) VALUES ('aaaaaaaa', $1, $2, $3, 'a long enough comment')", f.idx, f.lemma, f.pos)
	}
	mustExec(t, "INSERT INTO flags (sentence_id, comment) VALUES ('aaaaaaaa', 'the whole thing is off')")
	srv := newTestServer(t)
	defer srv.Close()

	var body []struct {
		Lemma string `json:"lemma"`
		POS   string `json:"pos"`
		Count int64  `json:"count"`
	}
	decode(t, call(t, srv, http.MethodGet, "/api/v1/admin/flagged-words", "", adminCookie(time.Now().Add(time.Hour))), http.StatusOK, &body)

	got := make([]string, len(body))
	for i, w := range body {
		got[i] = fmt.Sprintf("%s/%s/%d", w.Lemma, w.POS, w.Count)
	}
	if want := []string{"goose/Noun/2", "devour/Verb/1"}; !slices.Equal(got, want) {
		t.Errorf("flagged words: got %v, want %v", got, want)
	}
}
```

Add `"encoding/json"` and `"fmt"` to the imports.

Append to `listEndpoints` in `performance_test.go`:

```go
	{"admin flags", "/api/v1/admin/flags", adminCookie(time.Now().Add(time.Hour)), func(t *testing.T, n int) {
		for range n {
			id := nextID()
			insertSentence(t, id, time.Now())
			mustExec(t, "INSERT INTO flags (sentence_id, comment) VALUES ($1, 'a long enough comment')", id)
		}
	}},
	{"flagged words", "/api/v1/admin/flagged-words", adminCookie(time.Now().Add(time.Hour)), func(t *testing.T, n int) {
		id := nextID()
		insertSentence(t, id, time.Now())
		for range n {
			mustExec(t, "INSERT INTO flags (sentence_id, word_index, lemma, pos, comment) VALUES ($1, 1, $2, 'Noun', 'a long enough comment')", id, nextID())
		}
	}},
```

- [ ] **Step 7: Implement the admin lists**

Append to `admin.go`:

```go
type FlaggedSentence struct {
	ID   string          `json:"id"`
	Text string          `json:"text"`
	Tree json.RawMessage `json:"tree"`
}

type FlagResponse struct {
	ID        int64           `json:"id"`
	WordIndex *int32          `json:"word_index"`
	Lemma     *string         `json:"lemma"`
	POS       *string         `json:"pos"`
	Comment   string          `json:"comment"`
	CreatedAt time.Time       `json:"created_at"`
	Sentence  FlaggedSentence `json:"sentence"`
}

type FlaggedWordResponse struct {
	Lemma string `json:"lemma"`
	POS   string `json:"pos"`
	Count int64  `json:"count"`
}

func nullInt(v pgtype.Int4) *int32 {
	if !v.Valid {
		return nil
	}
	return &v.Int32
}

func nullText(v pgtype.Text) *string {
	if !v.Valid {
		return nil
	}
	return &v.String
}

func (h *Handler) listFlags(w http.ResponseWriter, r *http.Request) {
	limit, offset, err := page(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	rows, err := h.queries.ListFlags(r.Context(), store.ListFlagsParams{PageLimit: limit, PageOffset: offset})
	if err != nil {
		log.Printf("listFlags: %v", err)
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	resp := make([]FlagResponse, len(rows))
	for i, f := range rows {
		resp[i] = FlagResponse{
			ID: f.ID, WordIndex: nullInt(f.WordIndex), Lemma: nullText(f.Lemma), POS: nullText(f.Pos),
			Comment: f.Comment, CreatedAt: f.CreatedAt.Time,
			Sentence: FlaggedSentence{ID: f.SentenceID, Text: f.SentenceText, Tree: f.SentenceTree},
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) listFlaggedWords(w http.ResponseWriter, r *http.Request) {
	limit, offset, err := page(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	rows, err := h.queries.ListFlaggedWords(r.Context(), store.ListFlaggedWordsParams{PageLimit: limit, PageOffset: offset})
	if err != nil {
		log.Printf("listFlaggedWords: %v", err)
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	// The query skips flags without a lemma, so Lemma and Pos are set.
	resp := make([]FlaggedWordResponse, len(rows))
	for i, fw := range rows {
		resp[i] = FlaggedWordResponse{Lemma: fw.Lemma.String, POS: fw.Pos.String, Count: fw.Count}
	}
	writeJSON(w, http.StatusOK, resp)
}
```

- [ ] **Step 8: Run all tests**

Run: `just test`
Expected: PASS. The login-failure subtests run in parallel, so they add about a second, not three.

- [ ] **Step 9: Add the hash command and recipe**

`service/cmd/hashpassword/main.go`:

```go
// Command hashpassword prints a bcrypt hash of a password typed at the
// terminal, for ADMIN_PASSWORD_HASH.
package main

import (
	"fmt"
	"log"
	"os"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"
)

func main() {
	fmt.Fprint(os.Stderr, "Password: ")
	pw, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		log.Fatalf("read password: %v", err)
	}
	if len(pw) == 0 {
		log.Fatal("password is empty")
	}
	hash, err := bcrypt.GenerateFromPassword(pw, bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("hash: %v", err)
	}
	fmt.Println(string(hash))
}
```

Add to the Justfile, after `ingest`:

```just
# Print a bcrypt hash of a typed password, for ADMIN_PASSWORD_HASH
[working-directory: 'service']
hash-password:
    go run ./cmd/hashpassword
```

- [ ] **Step 10: Read the admin settings in `main.go`**

Add `"golang.org/x/crypto/bcrypt"` to the imports. Extend `config`:

```go
type config struct {
	databaseURL       string
	port              string
	adminPasswordHash string
	sessionSecret     string
	insecureCookies   bool
}
```

In `loadConfig`'s return:

```go
	return config{
		databaseURL:       required("DATABASE_URL"),
		port:              withDefault("PORT", "8080"),
		adminPasswordHash: required("ADMIN_PASSWORD_HASH"),
		sessionSecret:     required("SESSION_SECRET"),
		insecureCookies:   os.Getenv("INSECURE_COOKIES") == "true",
	}
```

In `main`, right after `cfg := loadConfig()`:

```go
	if _, err := bcrypt.Cost([]byte(cfg.adminPasswordHash)); err != nil {
		log.Fatalf("ADMIN_PASSWORD_HASH must be a bcrypt hash (single-quote it in .env so its $s survive): %v", err)
	}
	if len(cfg.sessionSecret) < 32 {
		log.Fatal("SESSION_SECRET must be at least 32 bytes")
	}
```

Pass it to the router:

```go
	routerCfg := api.RouterConfig{
		Queries: store.New(db), Grammar: g, Verbs: v,
		Admin: api.AdminConfig{
			PasswordHash:    []byte(cfg.adminPasswordHash),
			SessionSecret:   []byte(cfg.sessionSecret),
			InsecureCookies: cfg.insecureCookies,
		},
	}
```

- [ ] **Step 11: Add the dev settings to `.env.example`**

Run `just hash-password` and type `randsense`. Then append the following, pasting the printed hash between the single quotes. just's dotenv loader expands `$` in unquoted values, which would mangle the hash:

```
# Development admin password: randsense. `just hash-password` makes another.
# Single quotes stop the $s in the hash being expanded.
ADMIN_PASSWORD_HASH='<paste the hash here>'
SESSION_SECRET=dev-only-session-secret-change-me-in-prod
# Development only: lets the admin cookie work over plain http.
INSECURE_COOKIES=true
```

**Tell Jamey to copy these lines into their own `.env`.** The server won't start without `ADMIN_PASSWORD_HASH` and `SESSION_SECRET`, and plain-http development needs `INSECURE_COOKIES=true` for the cookie to stick.

- [ ] **Step 12: Try it by hand**

Copy the lines into `.env` if Jamey has said to, or export them for this shell. Then start `just run-be` in the background and run:

```bash
curl -si -X POST localhost:8080/api/v1/admin/login -d '{"password":"randsense"}' | grep -i set-cookie
curl -s -o /dev/null -w '%{http_code}\n' localhost:8080/api/v1/admin/flags
```

Expected: a `Set-Cookie: randsense_admin=...; HttpOnly; SameSite=Strict` line without `Secure`, then `401`. Stop the server.

- [ ] **Step 13: Update `README.md`**

Add these to the API block:

```
POST   /api/v1/admin/login                    {password} -> 204 + session cookie
POST   /api/v1/admin/logout                   -> 204
GET    /api/v1/admin/flags[?limit&offset]     -> [{id, word_index, lemma, pos, comment, created_at, sentence: {id, text, tree}}]
GET    /api/v1/admin/flagged-words[?limit&offset] -> [{lemma, pos, count}]
```

Below the block, add:

```markdown
There's one admin and no user table. `ADMIN_PASSWORD_HASH` holds a bcrypt hash (`just
hash-password` makes one) and `SESSION_SECRET` (32+ bytes) signs the 14-day session cookie, so
changing either and restarting logs everyone out. Every `/admin` route but `login` needs the
cookie. `INSECURE_COOKIES=true` drops the cookie's `Secure` flag for plain-http development.
```

Add `hash-password` to the Commands table:

```
| `just hash-password`            | Print a bcrypt hash for `ADMIN_PASSWORD_HASH` |
```

- [ ] **Step 14: Commit**

```bash
git add service Justfile .env.example README.md
git commit -m "feat: Add single-admin login and flag review endpoints"
```

---

## Finish

- [ ] **Run the full suite with the race detector, then check coverage**

Run: `just test -race && just cover | tail -30`
Expected: PASS. Report each coverage gap with its reason. Expect the marshal and stored-tree decode failures, which no input reaches, and the `Flush` error returns, which need a client to vanish mid-write. `cmd/` isn't in the coverage list.

- [ ] **Run `/wrap`**

Run `/wrap` to reconcile the project records, including pointing `CLAUDE.md`'s roadmap item 1 at stage 6.
