# Word Definitions Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Show a content word's OEWN definitions on its word card, and link every card to Wiktionary.

**Architecture:** Ingest reads the OEWN file twice: once to map synset IDs to glosses, once to
ingest entries with their glosses in sense order, into a new `definitions` JSONB column on the four
content tables. A new `GET /api/v1/words/{pos}/{lemma}` serves them, and `WordCard` fetches them
when it opens.

**Tech Stack:** Go (chi, pgx, sqlc, golang-migrate), PostgreSQL, SvelteKit (Svelte 5 runes),
Vitest browser mode, Playwright, nginx.

**Spec:** `docs/superpowers/specs/2026-10-06-word-definitions-design.md`

## Global Constraints

- Content `pos` values are exactly `noun`, `verb`, `adjective`, `adverb`.
- Glosses are stored as shipped. No cleanup of usage notes.
- `definitions` is never JSON `null`: an entry with no glosses stores `[]`.
- Cache header: `Cache-Control: public, max-age=86400`.
- Attribution text: "Definitions from Open English WordNet (CC BY 4.0)", linking to `https://en-word.net/`.
- Wiktionary URL: `https://en.wiktionary.org/wiki/<lemma>#English`, spaces as underscores, the rest URL-encoded, built from the lemma.
- Show 3 senses, then a "Show all N" button with `aria-expanded`.
- Commits: `type: Capitalized summary`, no `Co-Authored-By` trailer.
- No dates, names or history in source comments.

## Review Focus

- A lemma that needs URL encoding ("sea anemone", "o'clock", or "o%27clock" from a client that
  escapes apostrophes) should still find its row. Pinned in Task 3.
- Tapping word B before word A's definitions arrive should show only B's. Pinned in Task 5.
- OEWN entries sharing a lemma and table (`a`/`s` adjectives) should end up as one row with both
  entries' glosses. Pinned in Task 2.
- An entry whose synsets have no gloss, or ingest run with no glosses, should store `[]`, not
  `null`. Pinned in Task 2.
- A failed definitions request should leave the card usable: no definitions section, Wiktionary
  link still there. Pinned in Task 5.

## Files

- `service/internal/lexicon/oewn/parser.go`: `Entry.Synsets`, new `Glosses`.
- `service/internal/lexicon/oewn/testdata/sample.xml`: `<Synset>` fixtures.
- `service/migrations/009_definitions.{up,down}.sql`: the column.
- `service/internal/store/queries/{nouns,verbs,adjectives,adverbs}.sql`: insert merges definitions; `Get*Definitions` lookups. Regenerated `service/internal/store/*.sql.go`.
- `service/internal/lexicon/oewn/ingest.go`: definitions per entry.
- `service/cmd/ingest/main.go`: the gloss pass.
- `service/internal/api/words.go` (new): the handler. `router.go`: the route.
- `deploy/randsense.nginx`, `README.md`.
- `web/src/lib/definitions.ts` (new): fetch and Wiktionary URL.
- `web/src/lib/components/WordCard.svelte`: the section and the link.
- `web/src/routes/s/[id]/page.e2e.ts`: the e2e check.

---

### Task 1: Parse synset IDs and glosses

**Files:**
- Modify: `service/internal/lexicon/oewn/parser.go`
- Modify: `service/internal/lexicon/oewn/testdata/sample.xml`
- Test: `service/internal/lexicon/oewn/parser_test.go`

**Interfaces:**
- Produces: `Entry.Synsets []string` (each `<Sense>`'s `synset` attribute, in sense order, nil
  when there are no senses); `func Glosses(r io.Reader) (map[string]string, error)` (synset ID to
  gloss; several `<Definition>`s joined with `"; "`).

- [ ] **Step 1: Add synsets to the fixture**

In `testdata/sample.xml`, insert before `  </Lexicon>`:

```xml
    <!-- synsets follow every entry, as in the release -->
    <Synset id="oewn-01858313-n" partOfSpeech="n">
      <Definition>web-footed long-necked migratory aquatic bird</Definition>
    </Synset>
    <Synset id="oewn-01459708-v" partOfSpeech="v">
      <Definition>poke in the buttocks</Definition>
    </Synset>
    <Synset id="oewn-01233625-v" partOfSpeech="v">
      <Definition>prod into action</Definition>
    </Synset>
    <!-- a few synsets have more than one Definition -->
    <Synset id="oewn-02212345-s" partOfSpeech="s">
      <Definition>marked by high spirits or excitement</Definition>
      <Definition>giving off bubbles</Definition>
    </Synset>
```

- [ ] **Step 2: Write the failing tests**

In `TestParse`, every `want` entry gains `Synsets` holding its case's `synset` attributes in order.
For example:

```go
[]oewn.Entry{{Lemma: "goose", POS: "n", Forms: []string{"geese"}, Frames: nil, Synsets: []string{"oewn-01858313-n"}}},
```

The full list, by case name:
- "noun with irregular plural": `[]string{"oewn-01858313-n"}`
- the shrimp case: `[]string{"oewn-02314320-n"}`
- "verb single sense single subcat code": `[]string{"oewn-00018651-v"}`
- "verb single sense multiple subcat codes": `[]string{"oewn-01172275-v"}`
- "verb multiple senses overlapping codes (dedup)": `[]string{"oewn-01459708-v", "oewn-01233625-v"}`
- "verb with no subcat": `[]string{"oewn-02061425-v"}`
- "adjective head (a)": `[]string{"oewn-00231927-a"}`
- "adjective satellite (s)": `[]string{"oewn-02212345-s"}`
- "cardinal number": `[]string{"oewn-02201083-s"}`
- "adverb (r)": `[]string{"oewn-00120000-r"}`
- "multi-word lemma with space": `[]string{"oewn-02316707-n"}`
- "lemma with apostrophe entity": `[]string{"oewn-00010000-r"}`
- "lemma with Pronunciation child": `[]string{"oewn-01459708-v", "oewn-01233625-v"}`

Check any remaining case (`sed -n 40,70p parser_test.go`) and give it its own synset the same way.

Append to `parser_test.go`:

```go
func TestGlosses(t *testing.T) {
	f, err := os.Open(filepath.Join("testdata", "sample.xml"))
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()

	got, err := oewn.Glosses(f)
	if err != nil {
		t.Fatalf("Glosses: %v", err)
	}

	want := map[string]string{
		"oewn-01858313-n": "web-footed long-necked migratory aquatic bird",
		"oewn-01459708-v": "poke in the buttocks",
		"oewn-01233625-v": "prod into action",
		"oewn-02212345-s": "marked by high spirits or excitement; giving off bubbles",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Glosses:\n got %v\nwant %v", got, want)
	}
}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `cd service && go test ./internal/lexicon/oewn/ -run 'TestParse|TestGlosses'`
Expected: build failure, `unknown field Synsets` and `undefined: oewn.Glosses`.

- [ ] **Step 4: Implement**

In `parser.go`, add to `Entry` after `Frames`:

```go
	// Synsets is each <Sense>'s synset ID, in sense order. Glosses maps
	// them to definitions.
	Synsets []string
```

Add `Synset string `xml:"synset,attr"`` to the `Senses` struct in `lexicalEntry`. In `toEntry`,
declare `var synsets []string`, append `s.Synset` inside the senses loop, and set
`Synsets: synsets` on the returned `Entry`.

Append:

```go
// synset mirrors the part of <Synset> randsense uses.
type synset struct {
	ID          string   `xml:"id,attr"`
	Definitions []string `xml:"Definition"`
}

// Glosses streams r as WN-LMF XML and maps each synset ID to its definition.
// The few synsets with several <Definition>s get them joined with "; ".
// Synsets follow every <LexicalEntry> in the release, so ingest reads the
// file once for these before Parse.
func Glosses(r io.Reader) (map[string]string, error) {
	glosses := map[string]string{}
	dec := xml.NewDecoder(r)
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			return glosses, nil
		}
		if err != nil {
			return nil, err
		}

		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Local != "Synset" {
			continue
		}

		var s synset
		if err := dec.DecodeElement(&s, &se); err != nil {
			return nil, err
		}
		if len(s.Definitions) > 0 {
			glosses[s.ID] = strings.Join(s.Definitions, "; ")
		}
	}
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `cd service && go test ./internal/lexicon/oewn/ -run 'TestParse|TestGlosses'`
Expected: PASS. `TestParseSampleFile` still counts 22 entries.

- [ ] **Step 6: Commit**

```bash
git add service/internal/lexicon/oewn/parser.go service/internal/lexicon/oewn/parser_test.go service/internal/lexicon/oewn/testdata/sample.xml
git commit -m "feat: Parse OEWN synset IDs and glosses"
```

---

### Task 2: Store definitions at ingest

**Files:**
- Create: `service/migrations/009_definitions.up.sql`, `service/migrations/009_definitions.down.sql`
- Modify: `service/internal/store/queries/nouns.sql`, `verbs.sql`, `adjectives.sql`, `adverbs.sql`
- Regenerate: `service/internal/store/*.sql.go`, `models.go`
- Modify: `service/internal/lexicon/oewn/ingest.go`, `service/cmd/ingest/main.go`
- Modify (new required field): `service/internal/api/handlers_test.go`, `service/internal/lexicon/subtlex/apply_test.go`, `service/internal/lexicon/oewn/separable_test.go`
- Test: `service/internal/lexicon/oewn/ingest_test.go`

**Interfaces:**
- Consumes: `Entry.Synsets`, `oewn.Glosses` (Task 1).
- Produces: `func Ingest(ctx context.Context, pool *pgxpool.Pool, r io.Reader, glosses map[string]string) (Stats, error)`;
  `Insert{Noun,Verb,Adjective,Adverb}Params.Definitions []byte`;
  `store.Noun.Definitions []byte` (and Verb, Adjective, Adverb);
  `Get{Noun,Verb,Adjective,Adverb}Definitions(ctx, lemma string) ([]byte, error)`.

- [ ] **Step 1: Write the migration**

`009_definitions.up.sql`:

```sql
-- A content word's OEWN glosses in sense order, filled by ingest.
ALTER TABLE nouns      ADD COLUMN definitions JSONB NOT NULL DEFAULT '[]';
ALTER TABLE verbs      ADD COLUMN definitions JSONB NOT NULL DEFAULT '[]';
ALTER TABLE adjectives ADD COLUMN definitions JSONB NOT NULL DEFAULT '[]';
ALTER TABLE adverbs    ADD COLUMN definitions JSONB NOT NULL DEFAULT '[]';
```

`009_definitions.down.sql`:

```sql
ALTER TABLE nouns      DROP COLUMN definitions;
ALTER TABLE verbs      DROP COLUMN definitions;
ALTER TABLE adjectives DROP COLUMN definitions;
ALTER TABLE adverbs    DROP COLUMN definitions;
```

- [ ] **Step 2: Update the queries**

Replace each content insert. `nouns.sql`:

```sql
-- name: InsertNoun :exec
-- OEWN entries that share a lemma pool their definitions.
INSERT INTO nouns (lemma, inflections, definitions, source)
VALUES ($1, $2, $3, $4)
ON CONFLICT (lemma, source) DO UPDATE SET definitions = nouns.definitions || EXCLUDED.definitions;
```

`verbs.sql` (frames still come from the first entry):

```sql
-- name: InsertVerb :exec
-- OEWN entries that share a lemma pool their definitions; frames come from
-- the first.
INSERT INTO verbs (lemma, inflections, frames, definitions, source)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (lemma, source) DO UPDATE SET definitions = verbs.definitions || EXCLUDED.definitions;
```

`adjectives.sql` and `adverbs.sql`: the `nouns.sql` form with the table name swapped
(`adjectives.definitions`, `adverbs.definitions`).

Append to each of the four files, with the table and name swapped (`GetVerbDefinitions`,
`GetAdjectiveDefinitions`, `GetAdverbDefinitions`):

```sql
-- name: GetNounDefinitions :one
-- Active or not: an old sentence's word still shows its meaning.
SELECT definitions FROM nouns
WHERE lemma = $1
ORDER BY id
LIMIT 1;
```

Run: `just generate`
Expected: `internal/store/*.sql.go` regenerated. Callers without `Definitions` still compile, so
find them with `grep -rn -E 'Insert(Noun|Verb|Adjective|Adverb)Params\{' service`.

- [ ] **Step 3: Pass `Definitions` at every existing insert call site**

`jsonb NOT NULL` rejects a nil `[]byte` (pgx sends NULL). Add `Definitions: []byte("[]")` (or
`` []byte(`[]`) `` to match the file) to every `Insert*Params` literal in
`service/internal/api/handlers_test.go` (five) and `service/internal/lexicon/subtlex/apply_test.go`
(four).

- [ ] **Step 4: Write the failing ingest tests**

In `ingest_test.go`, add the helper:

```go
// sampleGlosses is the fixture's synset glosses.
func sampleGlosses(t *testing.T) map[string]string {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", "sample.xml"))
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()
	g, err := oewn.Glosses(f)
	if err != nil {
		t.Fatalf("Glosses: %v", err)
	}
	return g
}
```

Change the call in `TestIngest` to `oewn.Ingest(ctx, testPool, f, sampleGlosses(t))`, and in
`separable_test.go` to `oewn.Ingest(ctx, testPool, f, nil)`.

Add a subtest at the end of `TestIngest`:

```go
	t.Run("definitions", func(t *testing.T) {
		// in sense order
		verb, err := q.GetVerbByLemma(ctx, "goose")
		if err != nil {
			t.Fatalf("GetVerbByLemma(goose): %v", err)
		}
		var defs []string
		if err := json.Unmarshal(verb.Definitions, &defs); err != nil {
			t.Fatalf("json.Unmarshal(goose.Definitions): %v", err)
		}
		if !slices.Equal(defs, []string{"poke in the buttocks", "prod into action"}) {
			t.Errorf("goose definitions: got %v", defs)
		}

		// a synset without a gloss: an empty array, not null
		noun, err := q.GetNounByLemma(ctx, "shrimp")
		if err != nil {
			t.Fatalf("GetNounByLemma(shrimp): %v", err)
		}
		if string(noun.Definitions) != "[]" {
			t.Errorf("shrimp definitions: got %s, want []", noun.Definitions)
		}
	})
```

Add a new test:

```go
// TestIngestMergesDefinitions checks that OEWN entries sharing a lemma end
// up in one row with every entry's glosses.
func TestIngestMergesDefinitions(t *testing.T) {
	ctx := context.Background()
	xml := `<?xml version="1.0" encoding="UTF-8"?>
<LexicalResource>
  <Lexicon id="oewn" label="test" language="en" email="x@y" license="cc-by-4.0" version="test">
    <LexicalEntry id="oewn-good-a">
      <Lemma writtenForm="good" partOfSpeech="a"/>
      <Sense id="oewn-good__3.00.00.." synset="oewn-1-a"/>
    </LexicalEntry>
    <LexicalEntry id="oewn-good-s">
      <Lemma writtenForm="good" partOfSpeech="s"/>
      <Sense id="oewn-good__5.00.00.." synset="oewn-2-s"/>
    </LexicalEntry>
  </Lexicon>
</LexicalResource>`
	glosses := map[string]string{"oewn-1-a": "having desirable qualities", "oewn-2-s": "morally admirable"}

	if _, err := oewn.Ingest(ctx, testPool, strings.NewReader(xml), glosses); err != nil {
		t.Fatalf("Ingest: %v", err)
	}

	q := store.New(testPool)
	if n, err := q.CountAdjectives(ctx); err != nil || n != 1 {
		t.Fatalf("CountAdjectives: got %d, %v; want 1", n, err)
	}
	adj, err := q.GetAdjectiveByLemma(ctx, "good")
	if err != nil {
		t.Fatalf("GetAdjectiveByLemma(good): %v", err)
	}
	var defs []string
	if err := json.Unmarshal(adj.Definitions, &defs); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if !slices.Equal(defs, []string{"having desirable qualities", "morally admirable"}) {
		t.Errorf("good definitions: got %v", defs)
	}
}
```

Add `"strings"` to the imports.

- [ ] **Step 5: Run the tests to verify they fail**

Run: `cd service && go test ./internal/lexicon/oewn/`
Expected: build failure, too many arguments in call to `oewn.Ingest`.

- [ ] **Step 6: Implement ingest**

In `ingest.go`, change the signature and doc comment's first line:

```go
// Ingest streams r as OEW WN-LMF XML, applies AllowLemma (and AllowNoun for
// nouns, AllowAdjective for adjectives), dispatches each entry to the per-POS
// table by Entry.POS with its definitions from glosses, and inserts, then
// marks nouns whose lemma is already plural. The whole run is
```

```go
func Ingest(ctx context.Context, pool *pgxpool.Pool, r io.Reader, glosses map[string]string) (Stats, error) {
```

Pass `glosses` through: `return ingestEntry(ctx, q, e, glosses, &stats)`, and
`func ingestEntry(ctx context.Context, q *store.Queries, e Entry, glosses map[string]string, stats *Stats) error`.

In `ingestEntry`, after the `AllowLemma` check:

```go
	defs, err := definitionsJSON(e.Synsets, glosses)
	if err != nil {
		return err
	}
```

Add `Definitions: defs,` to all four `Insert*Params` literals. The noun and verb branches declare
`err` with `:=` on their first assignment (`infl, err :=`, `frames, err :=`), which still compiles
because they introduce a new variable; change the adjective and adverb branches' `err :=` to
`err =`.

Append:

```go
// definitionsJSON is the glosses of synsets, in sense order, as a JSON
// array. A synset without a gloss is skipped.
func definitionsJSON(synsets []string, glosses map[string]string) ([]byte, error) {
	defs := []string{}
	for _, id := range synsets {
		if g, ok := glosses[id]; ok {
			defs = append(defs, g)
		}
	}
	return json.Marshal(defs)
}
```

- [ ] **Step 7: Implement the gloss pass in `cmd/ingest/main.go`**

Before `log.Printf("ingesting %s ...", path)`:

```go
	log.Printf("reading glosses from %s ...", path)
	glosses, err := readGlosses(path)
	if err != nil {
		log.Fatalf("glosses: %v", err)
	}
```

Change the ingest call to `oewn.Ingest(ctx, db, gz, glosses)`. Append:

```go
// readGlosses is ingest's first pass over the OEWN file. Synsets follow
// every entry, so their glosses are read before the entries.
func readGlosses(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer gz.Close()

	return oewn.Glosses(gz)
}
```

- [ ] **Step 8: Run the backend suite**

Run: `just test-be`
Expected: PASS.

Then run the real ingest against the dev database and spot-check:

Run: `just ingest && psql "$DATABASE_URL" -c "SELECT lemma, definitions FROM nouns WHERE lemma = 'hyaline'"`
Expected: a non-empty array including a gloss about a glassy substance.

- [ ] **Step 9: Commit**

```bash
git add service/migrations/009_definitions.up.sql service/migrations/009_definitions.down.sql service/internal/store service/internal/lexicon service/cmd/ingest service/internal/api/handlers_test.go
git commit -m "feat: Store OEWN definitions at ingest"
```

---

### Task 3: Serve definitions

**Files:**
- Create: `service/internal/api/words.go`
- Modify: `service/internal/api/router.go`
- Modify: `deploy/randsense.nginx`, `README.md`
- Test: `service/internal/api/words_test.go`

**Interfaces:**
- Consumes: `Get{Noun,Verb,Adjective,Adverb}Definitions` (Task 2).
- Produces: `GET /api/v1/words/{pos}/{lemma}` returning `200 {"definitions": string[]}` or 404.

- [ ] **Step 1: Write the failing tests**

`service/internal/api/words_test.go`:

```go
package api_test

import (
	"net/http"
	"slices"
	"testing"
)

func TestDefinitions(t *testing.T) {
	seedWords(t)
	// Inactive words still have definitions.
	mustExec(t, `UPDATE nouns SET definitions = '["a bird", "a fool"]', active = FALSE WHERE lemma = 'goose'`)
	mustExec(t, `INSERT INTO nouns (lemma, definitions, source) VALUES ('sea anemone', '["a polyp"]', 'test')`)
	mustExec(t, `INSERT INTO adverbs (lemma, definitions, source) VALUES ('o''clock', '["of the clock"]', 'test')`)
	srv := newTestServer(t)
	defer srv.Close()

	for _, tc := range []struct {
		path string
		want []string
	}{
		{"/api/v1/words/noun/goose", []string{"a bird", "a fool"}},
		{"/api/v1/words/noun/sea%20anemone", []string{"a polyp"}},
		{"/api/v1/words/adverb/o'clock", []string{"of the clock"}},
		{"/api/v1/words/adverb/o%27clock", []string{"of the clock"}},
		{"/api/v1/words/verb/devour", []string{}},
	} {
		t.Run(tc.path, func(t *testing.T) {
			resp := call(t, srv, http.MethodGet, tc.path, "", nil)
			var body struct {
				Definitions []string `json:"definitions"`
			}
			decode(t, resp, http.StatusOK, &body)
			if !slices.Equal(body.Definitions, tc.want) {
				t.Errorf("definitions: got %v, want %v", body.Definitions, tc.want)
			}
			if cc := resp.Header.Get("Cache-Control"); cc != "public, max-age=86400" {
				t.Errorf("Cache-Control: got %q", cc)
			}
		})
	}

	for _, path := range []string{
		"/api/v1/words/noun/unicorn",
		"/api/v1/words/determiner/this",
		"/api/v1/words/planet/goose",
	} {
		t.Run(path, func(t *testing.T) {
			decode(t, call(t, srv, http.MethodGet, path, "", nil), http.StatusNotFound, nil)
		})
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd service && go test ./internal/api/ -run TestDefinitions`
Expected: FAIL, status 404 (or 405) for every 200 case.

- [ ] **Step 3: Implement**

`service/internal/api/words.go`:

```go
package api

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/url"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

// definitionsCacheSeconds is how long clients may reuse a word's
// definitions. They only change on reingest.
const definitionsCacheSeconds = "86400"

type definitionsResponse struct {
	Definitions json.RawMessage `json:"definitions"`
}

// getDefinitions serves a content word's OEWN glosses in sense order.
// Closed-class parts of speech have none, so they're a 404 like an unknown
// lemma.
func (h *Handler) getDefinitions(w http.ResponseWriter, r *http.Request) {
	lookup := map[string]func(context.Context, string) ([]byte, error){
		"noun":      h.queries.GetNounDefinitions,
		"verb":      h.queries.GetVerbDefinitions,
		"adjective": h.queries.GetAdjectiveDefinitions,
		"adverb":    h.queries.GetAdverbDefinitions,
	}[chi.URLParam(r, "pos")]
	// chi matches on the escaped path when a client escapes more than Go
	// would ("o%27clock"), so the lemma may still be escaped.
	lemma, err := url.PathUnescape(chi.URLParam(r, "lemma"))
	if lookup == nil || err != nil {
		writeError(w, http.StatusNotFound, "no such word")
		return
	}

	defs, err := lookup(r.Context(), lemma)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "no such word")
		return
	}
	if err != nil {
		log.Printf("getDefinitions: %v", err)
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}

	w.Header().Set("Cache-Control", "public, max-age="+definitionsCacheSeconds)
	writeJSON(w, http.StatusOK, definitionsResponse{Definitions: defs})
}
```

In `router.go`, after `r.Get("/words/random", h.randomWord)`:

```go
		r.Get("/words/{pos}/{lemma}", h.getDefinitions)
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `cd service && go test ./internal/api/ -run TestDefinitions`
Expected: PASS. If the `o'clock` (unescaped) case fails, check that `PathUnescape` of an already
decoded lemma is a no-op for it (it is: no `%`).

- [ ] **Step 5: Rate-limit the endpoint in nginx**

In `deploy/randsense.nginx`, after the `location = /api/v1/sentences/realize { ... }` block:

```nginx
    # Word lookups: cheap indexed reads, one per word card opened.
    location ~ ^/api/v1/words/ {
        limit_req zone=randsense_generate burst=30 nodelay;
        proxy_pass http://randsense_api;
    }
```

This also covers `/api/v1/words/random`, which was under the unlimited `/api/` block. Validate
the syntax if nginx is installed locally (`nginx -t -c` needs the full config; otherwise review by
eye against the neighboring blocks).

- [ ] **Step 6: Document the endpoint**

In `README.md`'s API block, after the `words/random` line:

```
GET /api/v1/words/{pos}/{lemma}               -> {definitions}
```

After the `grammar` paragraph, add:

```markdown
`words/{pos}/{lemma}` returns a content word's OEWN glosses in sense order, whether or not the
word is active. `pos` is noun, verb, adjective or adverb; any other `pos`, or a lemma the lexicon
lacks, is a 404. Glosses change only on reingest, so clients may cache them for a day.
```

- [ ] **Step 7: Run the backend suite and commit**

Run: `just test-be`
Expected: PASS.

```bash
git add service/internal/api/words.go service/internal/api/words_test.go service/internal/api/router.go deploy/randsense.nginx README.md
git commit -m "feat: Serve a word's definitions"
```

---

### Task 4: Fetch definitions and build Wiktionary links

**Files:**
- Create: `web/src/lib/definitions.ts`
- Test: `web/src/lib/definitions.spec.ts`

**Interfaces:**
- Consumes: the endpoint (Task 3).
- Produces: `CONTENT_POS: string[]`;
  `definitions(fetch: typeof globalThis.fetch, pos: string, lemma: string): Promise<string[]>`;
  `wiktionaryUrl(lemma: string): string`.

- [ ] **Step 1: Write the failing tests**

`web/src/lib/definitions.spec.ts`:

```ts
import { describe, expect, it, vi } from 'vitest';
import { definitions, wiktionaryUrl } from './definitions';

describe('definitions', () => {
	it('asks for the lemma escaped and returns its glosses', async () => {
		const fetch = vi.fn(async () => Response.json({ definitions: ['a polyp'] }));

		expect(await definitions(fetch, 'noun', 'sea anemone')).toEqual(['a polyp']);
		expect(fetch).toHaveBeenCalledWith('/api/v1/words/noun/sea%20anemone');
	});

	it('has none for a word the lexicon lacks', async () => {
		const fetch = vi.fn(async () => new Response(null, { status: 404 }));

		expect(await definitions(fetch, 'noun', 'unicorn')).toEqual([]);
	});

	it('throws when the API fails', async () => {
		const fetch = vi.fn(async () => new Response(null, { status: 500 }));

		await expect(definitions(fetch, 'noun', 'goose')).rejects.toThrow();
	});
});

describe('wiktionaryUrl', () => {
	it.each([
		['goose', 'https://en.wiktionary.org/wiki/goose#English'],
		['sea anemone', 'https://en.wiktionary.org/wiki/sea_anemone#English'],
		['Hopi', 'https://en.wiktionary.org/wiki/Hopi#English'],
		["o'clock", "https://en.wiktionary.org/wiki/o'clock#English"],
		['café', 'https://en.wiktionary.org/wiki/caf%C3%A9#English']
	])('links %s to its English entry', (lemma, url) => {
		expect(wiktionaryUrl(lemma)).toBe(url);
	});
});
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd web && npx vitest --run --project server src/lib/definitions.spec.ts`
Expected: FAIL, cannot resolve `./definitions`.

- [ ] **Step 3: Implement**

`web/src/lib/definitions.ts`:

```ts
// CONTENT_POS are the parts of speech OEWN defines. The rest are closed-class.
export const CONTENT_POS = ['noun', 'verb', 'adjective', 'adverb'];

// definitions fetches a content word's OEWN glosses in sense order, none when
// the lexicon lacks it. It throws when the API can't answer.
export async function definitions(
	fetch: typeof globalThis.fetch,
	pos: string,
	lemma: string
): Promise<string[]> {
	const res = await fetch(`/api/v1/words/${pos}/${encodeURIComponent(lemma)}`);
	if (res.status === 404) return [];
	if (!res.ok) throw new Error(`status ${res.status}`);
	return (await res.json()).definitions;
}

// wiktionaryUrl is lemma's English Wiktionary entry. Titles keep their case,
// as lemmas do ("Hopi").
export function wiktionaryUrl(lemma: string): string {
	return `https://en.wiktionary.org/wiki/${encodeURIComponent(lemma.replaceAll(' ', '_'))}#English`;
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `cd web && npx vitest --run --project server src/lib/definitions.spec.ts`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/src/lib/definitions.ts web/src/lib/definitions.spec.ts
git commit -m "feat: Fetch word definitions and build Wiktionary links"
```

---

### Task 5: Show definitions and the Wiktionary link on the word card

**Files:**
- Modify: `web/src/lib/components/WordCard.svelte`
- Test: `web/src/lib/components/WordCard.svelte.spec.ts`
- Modify: `web/src/routes/s/[id]/page.e2e.ts`

**Interfaces:**
- Consumes: `CONTENT_POS`, `definitions`, `wiktionaryUrl` (Task 4); `leaves`, `pos` from `#lib/tree.js`.

The fixture sentence's leaves, by index: 0 "the" (Determiner), 1 "goose" (Noun), 2 "devoured"
(Verb, lemma "devour"), 3 "her" (Pronoun), 4 ",", 5 "but", 6 "she", 7 "sang".

- [ ] **Step 1: Write the failing component tests**

In `WordCard.svelte.spec.ts`, change the vitest import to
`import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';`, and at the top of the
`describe` block add a default stub, so the existing tests don't reach the network:

```ts
	const answer = (defs: string[] | null) => async () =>
		defs ? Response.json({ definitions: defs }) : new Response(null, { status: 404 });

	beforeEach(() => vi.stubGlobal('fetch', vi.fn(answer(null))));
	afterEach(() => vi.unstubAllGlobals());
```

Append inside the `describe` block:

```ts
	it('shows three definitions, then all of them on request', async () => {
		const fetch = vi.fn(answer(['bird', 'fool', 'poke', 'cook', 'tailor']));
		vi.stubGlobal('fetch', fetch);
		render(WordCard, { tree: sentence.tree, index: 1, onclose: () => {} });

		const list = page.getByRole('list', { name: 'Definitions' });
		await expect.element(list.getByRole('listitem').nth(2)).toHaveTextContent('poke');
		expect(list.getByRole('listitem').elements()).toHaveLength(3);
		expect(fetch).toHaveBeenCalledWith('/api/v1/words/noun/goose');
		await expect.element(page.getByText('Open English WordNet')).toBeInTheDocument();

		const more = page.getByRole('button', { name: 'Show all 5' });
		await expect.element(more).toHaveAttribute('aria-expanded', 'false');
		await more.click();
		await expect.element(list.getByRole('listitem').nth(4)).toHaveTextContent('tailor');
	});

	it('looks up the lemma, not the inflected word', async () => {
		const fetch = vi.fn(answer(['eat greedily']));
		vi.stubGlobal('fetch', fetch);
		render(WordCard, { tree: sentence.tree, index: 2, onclose: () => {} });

		await expect.element(page.getByText('eat greedily')).toBeInTheDocument();
		expect(fetch).toHaveBeenCalledWith('/api/v1/words/verb/devour');
		await expect
			.element(page.getByRole('link', { name: 'Open in Wiktionary' }))
			.toHaveAttribute('href', 'https://en.wiktionary.org/wiki/devour#English');
	});

	it('links a closed-class word to Wiktionary without looking it up', async () => {
		const fetch = vi.fn(answer(['should not appear']));
		vi.stubGlobal('fetch', fetch);
		render(WordCard, { tree: sentence.tree, index: 0, onclose: () => {} });

		const link = page.getByRole('link', { name: 'Open in Wiktionary' });
		await expect.element(link).toHaveAttribute('href', 'https://en.wiktionary.org/wiki/the#English');
		await expect.element(link).toHaveAttribute('target', '_blank');
		expect(fetch).not.toHaveBeenCalled();
	});

	it('has no definitions section when there are none or the lookup fails', async () => {
		const { rerender } = render(WordCard, { tree: sentence.tree, index: 1, onclose: () => {} });
		await expect.element(page.getByRole('link', { name: 'Open in Wiktionary' })).toBeInTheDocument();
		await expect.element(page.getByText('Definitions')).not.toBeInTheDocument();

		vi.stubGlobal('fetch', vi.fn(async () => new Response(null, { status: 500 })));
		await rerender({ index: 2 });
		await expect.element(page.getByRole('heading', { name: 'devoured' })).toBeInTheDocument();
		await expect.element(page.getByText('Definitions')).not.toBeInTheDocument();
	});

	it("drops a word's late definitions once another word is open", async () => {
		let releaseGoose = () => {};
		vi.stubGlobal(
			'fetch',
			vi.fn((url: string) =>
				url.endsWith('/goose')
					? new Promise<Response>(
							(r) => (releaseGoose = () => r(Response.json({ definitions: ['bird'] })))
						)
					: Promise.resolve(Response.json({ definitions: ['eat greedily'] }))
			)
		);
		const { rerender } = render(WordCard, { tree: sentence.tree, index: 1, onclose: () => {} });

		await rerender({ index: 2 });
		await expect.element(page.getByText('eat greedily')).toBeInTheDocument();
		releaseGoose();
		await new Promise((r) => setTimeout(r));
		await expect.element(page.getByText('bird')).not.toBeInTheDocument();
	});
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd web && npx vitest --run --project client src/lib/components/WordCard.svelte.spec.ts`
Expected: the new tests FAIL (no list, no link); the existing four PASS.

- [ ] **Step 3: Implement**

In `WordCard.svelte`'s script, add the import and state:

```ts
	import { CONTENT_POS, definitions, wiktionaryUrl } from '#lib/definitions.js';
```

```ts
	// SHOWN is how many senses show before "Show all".
	const SHOWN = 3;
	const lemma = $derived(leaf.node.lemma ?? leaf.node.word ?? '');
	const defsId = `${headingId}-defs`;
	let defs: string[] = $state([]);
	let showAll = $state(false);

	// Each newly opened word fetches its own definitions. An answer for a word
	// no longer open is dropped, and a failed lookup shows none.
	$effect(() => {
		const p = pos(leaf.node).toLowerCase();
		const l = lemma;
		defs = [];
		showAll = false;
		if (!CONTENT_POS.includes(p)) return;
		let current = true;
		definitions(fetch, p, l).then(
			(d) => {
				if (current) defs = d;
			},
			() => {}
		);
		return () => {
			current = false;
		};
	});
```

`headingId` is declared below the existing `wordRole` line; place `defsId` after it.

In the markup, between the features `{/if}` and the Close button:

```svelte
	{#if defs.length}
		<h3 id={defsId}>Definitions</h3>
		<ol aria-labelledby={defsId}>
			{#each showAll ? defs : defs.slice(0, SHOWN) as d, i (i)}
				<li>{d}</li>
			{/each}
		</ol>
		{#if defs.length > SHOWN}
			<button
				type="button"
				class="button plain"
				aria-expanded={showAll}
				onclick={() => (showAll = !showAll)}>{showAll ? 'Show fewer' : `Show all ${defs.length}`}</button
			>
		{/if}
		<p class="credit">
			Definitions from <a href="https://en-word.net/">Open English WordNet</a> (CC BY 4.0)
		</p>
	{/if}
	<p>
		<a href={wiktionaryUrl(lemma)} target="_blank" rel="noopener">Open in Wiktionary</a>
	</p>
```

Add to `<style>`:

```css
	h3 {
		margin-block-end: 0.25rem;
		font-size: var(--text-base);
	}

	ol {
		margin-block-start: 0;
		padding-inline-start: 1.5rem;
	}

	.credit {
		color: var(--secondary);
		font-size: var(--text-sm);
	}
```

Check `web/src/app.css` for the size tokens (`grep -n "\-\-text-" web/src/app.css`); if
`--text-base` doesn't exist, drop that line rather than invent a token.

- [ ] **Step 4: Run the component tests to verify they pass**

Run: `cd web && npx vitest --run --project client src/lib/components/WordCard.svelte.spec.ts`
Expected: PASS.

- [ ] **Step 5: Fix the e2e heading query and add the e2e test**

The card now can hold two headings. In `web/src/routes/s/[id]/page.e2e.ts`, the test "opens a word
card by keyboard and returns focus on Escape" queries `page.getByRole('region').getByRole('heading')`,
which breaks strict mode when the first word is a content word. Change it to
`page.getByRole('region').getByRole('heading', { level: 2 })`. This is a test change, not a code
change: the card gained a heading, and the test means the word's heading.

Add the import `import { leaves, pos } from '../../../lib/tree';` and append:

```ts
test("shows a noun's definitions and a Wiktionary link on its card", async ({ page }) => {
	// Sentences don't always have a noun ("You told either..."), so fish for one.
	let s = await newSentence(page.request);
	let index = leaves(s.tree).findIndex((l) => pos(l.node) === 'Noun');
	for (let tries = 0; index < 0 && tries < 20; tries++) {
		s = await newSentence(page.request);
		index = leaves(s.tree).findIndex((l) => pos(l.node) === 'Noun');
	}
	expect(index).toBeGreaterThanOrEqual(0);

	await page.goto(`/s/${s.id}`);
	await page.locator(`#word-${s.id}-${index}`).click();

	const card = page.getByRole('region');
	await expect(card.getByRole('list', { name: 'Definitions' }).getByRole('listitem').first()).toBeVisible();
	await expect(card.getByRole('link', { name: 'Open in Wiktionary' })).toHaveAttribute(
		'href',
		/^https:\/\/en\.wiktionary\.org\/wiki\/.+#English$/
	);
	await expectNoAxeViolations(page);
});
```

- [ ] **Step 6: Run the full frontend suite**

The e2e tests use the dev database, which needs Task 2's migration and a reingest first.

Run: `just mu && just ingest && just test-fe`
Expected: PASS (check, lint, vitest, playwright).

- [ ] **Step 7: Commit**

```bash
git add web/src/lib/components/WordCard.svelte web/src/lib/components/WordCard.svelte.spec.ts "web/src/routes/s/[id]/page.e2e.ts"
git commit -m "feat: Show definitions and a Wiktionary link on the word card"
```

---

## After the last task

- Run `just test` once more from the repo root.
- Rollout, for Jamey: the deploy runs migration 009. Then reingest on the droplet with the
  command in `README.md`. Until then, cards show no definitions.
