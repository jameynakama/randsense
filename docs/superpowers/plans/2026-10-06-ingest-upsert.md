# Ingest Upsert Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ingest updates the content-word tables in place, keeps curated columns, runs as one transaction, and runs on every deploy.

**Architecture:** `oewn.Ingest` groups entries by lemma in Go, then per table deletes rows that left the source and batch upserts the rest with `unnest` arrays. The upsert's `SET` list names only sourced columns. Every ingest step takes `store.DBTX`, and `cmd/ingest` passes one transaction. Plural corrections live in `nouns.plural_override`, and `plural` is a generated column.

**Tech Stack:** Go, pgx v5, sqlc, golang-migrate, PostgreSQL 18, GitHub Actions.

**Spec:** `docs/superpowers/specs/2026-10-06-ingest-upsert-design.md`

## Global Constraints

- Commands run from `service/` unless noted. Tests need `TEST_DATABASE_URL` (in the repo's `.env`; `just` loads it): run `set -a; . ../.env; set +a` first when calling `go test` directly.
- After any change under `internal/store/queries/` or `migrations/`, run `just generate` from the repo root and commit the regenerated `internal/store/*.go`.
- The upserts never name `active`, `vote_count`, or `plural_override`.
- Source tag is the existing `oewn.SourceName` (`"oewn-2025"`).
- Commit messages: `type: Capitalized summary`, matching `git log`. No `Co-Authored-By` trailer.
- Prose (comments, docs) uses serial commas and no em dashes.
- Every test must be able to fail. Don't assert the implementation back at itself.

## Review Focus

1. **A lemma appearing twice in one run** (OEWN `a` and `s` entries, or entries split by etymology): Postgres rejects an `ON CONFLICT DO UPDATE` that hits the same key twice in one statement, so grouping must yield one row per lemma per table. Pinned by `TestIngestMergesDefinitions` running ingest twice (Task 2).
2. **Rows from another source:** stale deletion must be scoped to `SourceName`. Pinned by `TestIngestDeletesStaleRows` (Task 2).
3. **A table with no entries in the run** (the inline XML has only adjectives): `lemmas` is nil, which pgx sends as NULL. `unnest(NULL)` yields no rows, so the upsert writes nothing and the delete clears that table's OEWN rows, matching what truncation did. Pinned by `TestIngestMergesDefinitions` asserting the noun count is 0 (Task 2).
4. **Derived columns from a previous run** (frequency, separable, plural guess): they must reset, or a lemma dropped from `separable_verbs.toml` or SUBTLEX stays flagged forever. Pinned by `TestIngestRefreshesSourcedColumns` (Task 2).
5. **Case-distinct lemmas** ("Mass" and "mass"): grouping keys on the exact string, the same as the `(lemma, source)` unique key, so they stay separate rows. Already covered by `TestIngest`'s plural checks on "Mass".

---

### Task 1: Plural override column

**Files:**
- Create: `service/migrations/010_ingest_upsert.up.sql`, `service/migrations/010_ingest_upsert.down.sql`
- Modify: `service/internal/store/queries/nouns.sql` (`MarkPluralNouns`)
- Regenerate: `service/internal/store/*.go`
- Test: `service/internal/lexicon/oewn/ingest_test.go`, `service/internal/lexicon/oewn/setup_test.go`

**Interfaces:**
- Produces: `nouns.plural_guess BOOLEAN NOT NULL`, `nouns.plural_override BOOLEAN` (nullable), `nouns.plural BOOLEAN NOT NULL` generated. `store.Noun` gains `PluralGuess bool` and `PluralOverride pgtype.Bool`, and keeps `Plural bool`.
- Produces: `truncateLexicon(t *testing.T)` in `oewn_test`, for every test that ingests.

- [ ] **Step 1: Add the truncate helper**

Ingest will stop truncating (Task 2), and curation edits from one test would leak into the next. Append to `service/internal/lexicon/oewn/setup_test.go`:

```go
// truncateLexicon empties the content-word tables, since Ingest keeps rows
// between runs.
func truncateLexicon(t *testing.T) {
	t.Helper()
	_, err := testPool.Exec(context.Background(), "TRUNCATE nouns, verbs, adjectives, adverbs RESTART IDENTITY")
	if err != nil {
		t.Fatalf("truncate: %v", err)
	}
}
```

Call `truncateLexicon(t)` as the first line of `TestIngest`, `TestIngestMergesDefinitions` (`ingest_test.go`), and `TestMarkSeparable` (`separable_test.go`).

- [ ] **Step 2: Write the failing test**

Add to `service/internal/lexicon/oewn/ingest_test.go`:

```go
// TestPluralOverride checks that a curated correction beats the heuristic in
// both directions.
func TestPluralOverride(t *testing.T) {
	truncateLexicon(t)
	ingestSample(t)
	ctx := context.Background()

	_, err := testPool.Exec(ctx, `
		UPDATE nouns SET plural_override = FALSE WHERE lemma = 'Rastas';
		UPDATE nouns SET plural_override = TRUE WHERE lemma = 'goose';
	`)
	if err != nil {
		t.Fatalf("set overrides: %v", err)
	}

	q := store.New(testPool)
	for lemma, want := range map[string]bool{"Rastas": false, "goose": true, "eyeglasses": true} {
		noun, err := q.GetNounByLemma(ctx, lemma)
		if err != nil {
			t.Fatalf("GetNounByLemma(%s): %v", lemma, err)
		}
		if noun.Plural != want {
			t.Errorf("%s plural: got %t, want %t", lemma, noun.Plural, want)
		}
	}
}

// ingestSample ingests testdata/sample.xml.
func ingestSample(t *testing.T) oewn.Stats {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", "sample.xml"))
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()
	stats, err := oewn.Ingest(context.Background(), testPool, f, sampleGlosses(t))
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	return stats
}
```

- [ ] **Step 3: Run it to verify it fails**

Run: `go test ./internal/lexicon/oewn/ -run TestPluralOverride`
Expected: FAIL, `column "plural_override" does not exist`.

- [ ] **Step 4: Write the migration**

`service/migrations/010_ingest_upsert.up.sql`:

```sql
-- plural_guess is ingest's heuristic, rewritten every run. plural_override is
-- curation, which ingest never writes; NULL means the guess stands.
ALTER TABLE nouns RENAME COLUMN plural TO plural_guess;
ALTER TABLE nouns ADD COLUMN plural_override BOOLEAN;
ALTER TABLE nouns ADD COLUMN plural BOOLEAN NOT NULL
    GENERATED ALWAYS AS (coalesce(plural_override, plural_guess)) STORED;
```

`service/migrations/010_ingest_upsert.down.sql`:

```sql
ALTER TABLE nouns DROP COLUMN plural;
ALTER TABLE nouns DROP COLUMN plural_override;
ALTER TABLE nouns RENAME COLUMN plural_guess TO plural;
```

In `service/internal/store/queries/nouns.sql`, change `MarkPluralNouns`'s first line from `UPDATE nouns p SET plural = TRUE` to `UPDATE nouns p SET plural_guess = TRUE`. Leave its comment and `WHERE` clause alone.

Run from the repo root: `just migrate-up && just generate`. The tests migrate their own databases.

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/lexicon/oewn/ ./internal/sentence/ ./internal/api/`
Expected: PASS. `TestIngest`'s plural checks still pass through the generated column.

- [ ] **Step 6: Commit**

```bash
git add service/migrations/010_ingest_upsert.*.sql service/internal/store service/internal/lexicon/oewn
git commit -m "feat: Keep plural corrections apart from the plural heuristic"
```

---

### Task 2: Ingest upserts in place

**Files:**
- Modify: `service/internal/store/queries/{nouns,verbs,adjectives,adverbs}.sql`
- Regenerate: `service/internal/store/*.go`
- Modify: `service/internal/lexicon/oewn/ingest.go`
- Modify: `service/internal/api/handlers_test.go` (`seedWords`, `seedRareNoun`), `service/internal/lexicon/subtlex/apply_test.go` (`seedLexicon`): they call the `InsertX` queries this task drops
- Test: `service/internal/lexicon/oewn/ingest_test.go`

**Interfaces:**
- Consumes: `truncateLexicon`, `ingestSample` (Task 1); `nouns.plural_guess` (Task 1).
- Produces: `oewn.Ingest(ctx context.Context, db store.DBTX, r io.Reader, glosses map[string]string) (Stats, error)`. It no longer begins or commits a transaction. `*pgxpool.Pool` and `pgx.Tx` both satisfy `store.DBTX`.
- Produces sqlc queries (names and param structs as sqlc generates them):
  - `UpsertNouns(ctx, UpsertNounsParams{Lemmas, Inflections, Definitions []string; Source string})`
  - `UpsertVerbs(ctx, UpsertVerbsParams{Lemmas, Frames, Definitions []string; Source string})`
  - `UpsertAdjectives(ctx, UpsertAdjectivesParams{Lemmas, Definitions []string; Source string})`
  - `UpsertAdverbs(ctx, UpsertAdverbsParams{Lemmas, Definitions []string; Source string})`
  - `DeleteStaleNouns/Verbs/Adjectives/Adverbs(ctx, DeleteStaleXParams{Source string; Lemmas []string})`
- Removes: `InsertNoun`, `InsertVerb`, `InsertAdjective`, `InsertAdverb`, `TruncateNouns`, `TruncateVerbs`, `TruncateAdjectives`, `TruncateAdverbs`.

- [ ] **Step 1: Write the failing tests**

Add to `service/internal/lexicon/oewn/ingest_test.go`:

```go
// lexiconSnapshot is every content-word row's id and sourced columns, as
// JSON. update_time is left out, since a re-run touches every row.
func lexiconSnapshot(t *testing.T) string {
	t.Helper()
	var s string
	err := testPool.QueryRow(context.Background(), `
		SELECT json_build_array(
			(SELECT json_agg(json_build_array(id, lemma, inflections, definitions, plural_guess) ORDER BY id) FROM nouns),
			(SELECT json_agg(json_build_array(id, lemma, frames, definitions) ORDER BY id) FROM verbs),
			(SELECT json_agg(json_build_array(id, lemma, definitions) ORDER BY id) FROM adjectives),
			(SELECT json_agg(json_build_array(id, lemma, definitions) ORDER BY id) FROM adverbs)
		)::text`).Scan(&s)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	return s
}

// TestIngestTwiceIsIdentical checks that a re-run keeps ids and rewrites
// sourced columns to the same values.
func TestIngestTwiceIsIdentical(t *testing.T) {
	truncateLexicon(t)
	first := ingestSample(t)
	before := lexiconSnapshot(t)

	second := ingestSample(t)
	if after := lexiconSnapshot(t); after != before {
		t.Errorf("re-run changed rows:\nbefore %s\nafter  %s", before, after)
	}
	if first != second {
		t.Errorf("stats: first %+v, second %+v", first, second)
	}
}

// TestIngestKeepsCuration checks that a re-run never writes curated columns.
func TestIngestKeepsCuration(t *testing.T) {
	truncateLexicon(t)
	ingestSample(t)
	ctx := context.Background()

	_, err := testPool.Exec(ctx, `
		UPDATE nouns SET active = FALSE, vote_count = 3, plural_override = FALSE WHERE lemma = 'Rastas';
		UPDATE verbs SET active = FALSE, vote_count = 2 WHERE lemma = 'devour';
	`)
	if err != nil {
		t.Fatalf("curate: %v", err)
	}

	ingestSample(t)

	q := store.New(testPool)
	noun, err := q.GetNounByLemma(ctx, "Rastas")
	if err != nil {
		t.Fatalf("GetNounByLemma(Rastas): %v", err)
	}
	if noun.Active || noun.VoteCount != 3 || noun.PluralOverride != (pgtype.Bool{Bool: false, Valid: true}) || noun.Plural {
		t.Errorf("Rastas: active %t, vote_count %d, plural_override %+v, plural %t",
			noun.Active, noun.VoteCount, noun.PluralOverride, noun.Plural)
	}
	verb, err := q.GetVerbByLemma(ctx, "devour")
	if err != nil {
		t.Fatalf("GetVerbByLemma(devour): %v", err)
	}
	if verb.Active || verb.VoteCount != 2 {
		t.Errorf("devour: active %t, vote_count %d", verb.Active, verb.VoteCount)
	}
}

// TestIngestRefreshesSourcedColumns checks that a re-run rewrites what the
// sources own and resets what the later ingest passes fill.
func TestIngestRefreshesSourcedColumns(t *testing.T) {
	truncateLexicon(t)
	ingestSample(t)
	want := lexiconSnapshot(t)
	ctx := context.Background()

	_, err := testPool.Exec(ctx, `
		UPDATE nouns SET definitions = '["hand edit"]', inflections = '{"plural":"x"}', plural_guess = FALSE, frequency = 9
			WHERE lemma = 'Rastas';
		UPDATE verbs SET frames = '["bogus"]', separable = TRUE, frequency = 9 WHERE lemma = 'devour';
		UPDATE adjectives SET definitions = '["hand edit"]', frequency = 9;
		UPDATE adverbs SET definitions = '["hand edit"]', frequency = 9;
	`)
	if err != nil {
		t.Fatalf("hand edit: %v", err)
	}

	ingestSample(t)

	if got := lexiconSnapshot(t); got != want {
		t.Errorf("sourced columns not restored:\nwant %s\ngot  %s", want, got)
	}
	var stale int
	err = testPool.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM nouns WHERE frequency IS NOT NULL)
		     + (SELECT count(*) FROM verbs WHERE frequency IS NOT NULL OR separable)
		     + (SELECT count(*) FROM adjectives WHERE frequency IS NOT NULL)
		     + (SELECT count(*) FROM adverbs WHERE frequency IS NOT NULL)`).Scan(&stale)
	if err != nil {
		t.Fatalf("count stale: %v", err)
	}
	if stale != 0 {
		t.Errorf("rows keeping a previous run's frequency or separable flag: %d", stale)
	}
}

// TestIngestDeletesStaleRows checks that an OEWN row the run lacks is
// deleted, and a row from another source is not.
func TestIngestDeletesStaleRows(t *testing.T) {
	truncateLexicon(t)
	ctx := context.Background()
	// One statement per Exec: pgx rejects several statements with parameters.
	for _, stmt := range []string{
		"INSERT INTO nouns (lemma, source) VALUES ('flumpet', $1), ('flumpet', 'other')",
		"INSERT INTO verbs (lemma, source) VALUES ('flump', $1)",
	} {
		if _, err := testPool.Exec(ctx, stmt, oewn.SourceName); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	ingestSample(t)

	var oewnRows, otherRows int
	err := testPool.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM nouns WHERE lemma = 'flumpet' AND source = $1)
		     + (SELECT count(*) FROM verbs WHERE lemma = 'flump'),
		       (SELECT count(*) FROM nouns WHERE lemma = 'flumpet' AND source = 'other')`,
		oewn.SourceName).Scan(&oewnRows, &otherRows)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if oewnRows != 0 || otherRows != 1 {
		t.Errorf("stale oewn rows %d (want 0), other-source rows %d (want 1)", oewnRows, otherRows)
	}
}
```

Add `"github.com/jackc/pgx/v5/pgtype"` to the test's imports.

Change `TestIngestMergesDefinitions` so it ingests the inline XML twice and checks there are no nouns. Replace its single `oewn.Ingest` call with:

```go
	for range 2 {
		if _, err := oewn.Ingest(ctx, testPool, strings.NewReader(xml), glosses); err != nil {
			t.Fatalf("Ingest: %v", err)
		}
	}
```

and add, after the `CountAdjectives` check:

```go
	if n, err := q.CountNouns(ctx); err != nil || n != 0 {
		t.Fatalf("CountNouns: got %d, %v; want 0", n, err)
	}
```

Update its doc comment to: `// TestIngestMergesDefinitions checks that OEWN entries sharing a lemma end up in one row with every entry's glosses, once, however often ingest runs.`

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/lexicon/oewn/ -run 'TestIngest'`
Expected: `TestIngestKeepsCuration` fails on `active`, and `TestIngestDeletesStaleRows` fails because the truncate also removes the other source's row. `TestIngestTwiceIsIdentical`, `TestIngestRefreshesSourcedColumns`, and the twice-run `TestIngestMergesDefinitions` pass, since truncate-and-reload with `RESTART IDENTITY` already behaves that way. They pin what the rewrite must keep, and each fails under a plausible upsert bug: deleting and reinserting rows, leaving `frequency` or `separable` set, or appending definitions on conflict.

- [ ] **Step 3: Write the queries**

Replace `InsertNoun` and `TruncateNouns` in `service/internal/store/queries/nouns.sql` with:

```sql
-- name: UpsertNouns :exec
-- One row per lemma: ingest pools the entries sharing one first, since an
-- upsert can't hit the same row twice. Sourced columns are replaced and
-- derived ones reset for the later ingest passes; curated columns (active,
-- vote_count, plural_override) are never written.
INSERT INTO nouns (lemma, inflections, definitions, source)
SELECT run.lemma, run.inflections::jsonb, run.definitions::jsonb, @source::text
FROM (
    SELECT unnest(@lemmas::text[]) AS lemma,
           unnest(@inflections::text[]) AS inflections,
           unnest(@definitions::text[]) AS definitions
) run
ON CONFLICT (lemma, source) DO UPDATE SET
    inflections  = EXCLUDED.inflections,
    definitions  = EXCLUDED.definitions,
    frequency    = NULL,
    plural_guess = FALSE;

-- name: DeleteStaleNouns :exec
-- An anti-join, not <> ALL, which would compare every row with every lemma.
DELETE FROM nouns
WHERE source = @source::text
  AND NOT EXISTS (
    SELECT 1 FROM (SELECT unnest(@lemmas::text[]) AS lemma) run
    WHERE run.lemma = nouns.lemma
  );
```

Replace `InsertVerb` and `TruncateVerbs` in `verbs.sql` with:

```sql
-- name: UpsertVerbs :exec
-- One row per lemma: ingest pools the entries sharing one first, since an
-- upsert can't hit the same row twice. Sourced columns are replaced and
-- derived ones reset for the later ingest passes; curated columns (active,
-- vote_count) are never written.
INSERT INTO verbs (lemma, frames, definitions, source)
SELECT run.lemma, run.frames::jsonb, run.definitions::jsonb, @source::text
FROM (
    SELECT unnest(@lemmas::text[]) AS lemma,
           unnest(@frames::text[]) AS frames,
           unnest(@definitions::text[]) AS definitions
) run
ON CONFLICT (lemma, source) DO UPDATE SET
    frames      = EXCLUDED.frames,
    definitions = EXCLUDED.definitions,
    frequency   = NULL,
    separable   = FALSE;

-- name: DeleteStaleVerbs :exec
-- An anti-join, not <> ALL, which would compare every row with every lemma.
DELETE FROM verbs
WHERE source = @source::text
  AND NOT EXISTS (
    SELECT 1 FROM (SELECT unnest(@lemmas::text[]) AS lemma) run
    WHERE run.lemma = verbs.lemma
  );
```

Replace `InsertAdjective` and `TruncateAdjectives` in `adjectives.sql` with:

```sql
-- name: UpsertAdjectives :exec
-- One row per lemma: ingest pools the entries sharing one first, since an
-- upsert can't hit the same row twice. Sourced columns are replaced and
-- derived ones reset for the later ingest passes; curated columns (active,
-- vote_count) are never written.
INSERT INTO adjectives (lemma, definitions, source)
SELECT run.lemma, run.definitions::jsonb, @source::text
FROM (
    SELECT unnest(@lemmas::text[]) AS lemma,
           unnest(@definitions::text[]) AS definitions
) run
ON CONFLICT (lemma, source) DO UPDATE SET
    definitions = EXCLUDED.definitions,
    frequency   = NULL;

-- name: DeleteStaleAdjectives :exec
-- An anti-join, not <> ALL, which would compare every row with every lemma.
DELETE FROM adjectives
WHERE source = @source::text
  AND NOT EXISTS (
    SELECT 1 FROM (SELECT unnest(@lemmas::text[]) AS lemma) run
    WHERE run.lemma = adjectives.lemma
  );
```

Replace `InsertAdverb` and `TruncateAdverbs` in `adverbs.sql` with the same two queries, with `adverbs`/`Adverbs` in place of `adjectives`/`Adjectives`.

Inflections stay at the column default `'{}'` for verbs, adjectives, and adverbs, since no source fills them.

Run from the repo root: `just generate`. Expect `go build ./...` to fail in `ingest.go`, `handlers_test.go`, and `apply_test.go` until Steps 4 and 5.

- [ ] **Step 4: Rewrite `Ingest`**

In `service/internal/lexicon/oewn/ingest.go`, drop the `pgxpool` import and replace `Ingest` and `ingestEntry` with the code below. Keep `nounInflectionsJSON` and `verbFramesJSON` as they are. Replace `definitionsJSON` with `definitions`, since rows now pool glosses before marshaling.

```go
// Ingest streams r as OEW WN-LMF XML, applies AllowLemma (and AllowNoun for
// nouns, AllowAdjective for adjectives), and groups each entry by lemma into
// its per-POS table with its definitions from glosses. Each table is then
// made to match the run: rows from SourceName whose lemma the run lacks are
// deleted, and the rest are upserted, which replaces sourced columns, resets
// derived ones, and never writes curated ones. Last, it marks nouns whose
// lemma is already plural.
//
// Ingest doesn't begin or commit a transaction: pass the run's transaction
// as db, so a failure in any ingest step leaves the previous lexicon.
//
// Pass the gzipped XML pre-wrapped in a gzip.Reader if you're reading
// data/oewn-2025/english-wordnet-2025.xml.gz; Ingest itself only cares
// that it gets parseable XML bytes.
func Ingest(ctx context.Context, db store.DBTX, r io.Reader, glosses map[string]string) (Stats, error) {
	var stats Stats
	var nouns, verbs, adjectives, adverbs table

	err := Parse(r, func(e Entry) error {
		if !AllowLemma(e.Lemma) {
			stats.Skipped++
			return nil
		}
		defs := definitions(e.Synsets, glosses)

		switch e.POS {
		case "n":
			if !AllowNoun(e.Lemma) {
				stats.Skipped++
				return nil
			}
			infl, err := nounInflectionsJSON(e.Forms)
			if err != nil {
				return err
			}
			nouns.add(e.Lemma, row{inflections: string(infl), definitions: defs})
		case "v":
			frames, err := verbFramesJSON(e.Lemma, e.Frames)
			if err != nil {
				return err
			}
			verbs.add(e.Lemma, row{frames: string(frames), definitions: defs})
		case "a", "s":
			if !AllowAdjective(e.Lemma, e.Cardinal) {
				stats.Skipped++
				return nil
			}
			adjectives.add(e.Lemma, row{definitions: defs})
		case "r":
			adverbs.add(e.Lemma, row{definitions: defs})
		default:
			// Anything else (proper-name codes, unknowns) gets skipped.
			stats.Skipped++
		}
		return nil
	})
	if err != nil {
		return stats, fmt.Errorf("Ingest, Parse: %v", err)
	}

	q := store.New(db)
	for _, w := range []struct {
		kind   string
		t      *table
		count  *int
		delete func() error
		upsert func(defs []string) error
	}{
		{"Nouns", &nouns, &stats.Nouns,
			func() error {
				return q.DeleteStaleNouns(ctx, store.DeleteStaleNounsParams{Source: SourceName, Lemmas: nouns.lemmas})
			},
			func(defs []string) error {
				return q.UpsertNouns(ctx, store.UpsertNounsParams{
					Lemmas: nouns.lemmas, Inflections: nouns.column(func(r *row) string { return r.inflections }),
					Definitions: defs, Source: SourceName,
				})
			}},
		{"Verbs", &verbs, &stats.Verbs,
			func() error {
				return q.DeleteStaleVerbs(ctx, store.DeleteStaleVerbsParams{Source: SourceName, Lemmas: verbs.lemmas})
			},
			func(defs []string) error {
				return q.UpsertVerbs(ctx, store.UpsertVerbsParams{
					Lemmas: verbs.lemmas, Frames: verbs.column(func(r *row) string { return r.frames }),
					Definitions: defs, Source: SourceName,
				})
			}},
		{"Adjectives", &adjectives, &stats.Adjectives,
			func() error {
				return q.DeleteStaleAdjectives(ctx, store.DeleteStaleAdjectivesParams{Source: SourceName, Lemmas: adjectives.lemmas})
			},
			func(defs []string) error {
				return q.UpsertAdjectives(ctx, store.UpsertAdjectivesParams{Lemmas: adjectives.lemmas, Definitions: defs, Source: SourceName})
			}},
		{"Adverbs", &adverbs, &stats.Adverbs,
			func() error {
				return q.DeleteStaleAdverbs(ctx, store.DeleteStaleAdverbsParams{Source: SourceName, Lemmas: adverbs.lemmas})
			},
			func(defs []string) error {
				return q.UpsertAdverbs(ctx, store.UpsertAdverbsParams{Lemmas: adverbs.lemmas, Definitions: defs, Source: SourceName})
			}},
	} {
		if err := w.delete(); err != nil {
			return stats, fmt.Errorf("Ingest, DeleteStale%s: %v", w.kind, err)
		}
		defs, err := w.t.definitionsColumn()
		if err != nil {
			return stats, fmt.Errorf("Ingest, %s definitions: %v", w.kind, err)
		}
		if err := w.upsert(defs); err != nil {
			return stats, fmt.Errorf("Ingest, Upsert%s: %v", w.kind, err)
		}
		*w.count = len(w.t.lemmas)
	}

	// Runs after every noun is in, since a singular can follow its plural.
	if _, err := q.MarkPluralNouns(ctx); err != nil {
		return stats, fmt.Errorf("Ingest, MarkPluralNouns: %v", err)
	}

	return stats, nil
}

// row is one lemma's sourced columns. inflections is set for nouns and
// frames for verbs, both as JSON.
type row struct {
	inflections string
	frames      string
	definitions []string
}

// table is one part of speech's rows by lemma, in first-seen order.
type table struct {
	lemmas []string
	rows   map[string]*row
}

// add pools entries that share a lemma: the first supplies inflections and
// frames, and each later one appends its definitions.
func (t *table) add(lemma string, r row) {
	if prev, ok := t.rows[lemma]; ok {
		prev.definitions = append(prev.definitions, r.definitions...)
		return
	}
	if t.rows == nil {
		t.rows = map[string]*row{}
	}
	t.lemmas = append(t.lemmas, lemma)
	t.rows[lemma] = &r
}

// column is one field of every row, in lemma order, for an upsert array.
func (t *table) column(field func(*row) string) []string {
	col := make([]string, len(t.lemmas))
	for i, l := range t.lemmas {
		col[i] = field(t.rows[l])
	}
	return col
}

// definitionsColumn is every row's definitions as a JSON array, in lemma
// order.
func (t *table) definitionsColumn() ([]string, error) {
	col := make([]string, len(t.lemmas))
	for i, l := range t.lemmas {
		b, err := json.Marshal(t.rows[l].definitions)
		if err != nil {
			return nil, err
		}
		col[i] = string(b)
	}
	return col, nil
}
```

Replace `definitionsJSON` with:

```go
// definitions is the glosses of synsets, in sense order. A synset without a
// gloss is skipped. It is never nil, so a row with no glosses stores [].
func definitions(synsets []string, glosses map[string]string) []string {
	defs := []string{}
	for _, id := range synsets {
		if g, ok := glosses[id]; ok {
			defs = append(defs, g)
		}
	}
	return defs
}
```

Update the `Stats` doc comment's first sentence to: `// Stats reports how many distinct lemmas Ingest wrote per POS, plus a`. Keep the rest.

- [ ] **Step 5: Move test seeders to the upserts**

In `service/internal/api/handlers_test.go` `seedWords`, replace the four `InsertX` calls with:

```go
	if err := q.UpsertNouns(ctx, store.UpsertNounsParams{
		Lemmas: []string{"goose"}, Inflections: []string{`{"plural":"geese"}`}, Definitions: []string{`[]`}, Source: "test",
	}); err != nil {
		t.Fatalf("seed noun: %v", err)
	}
	if err := q.UpsertVerbs(ctx, store.UpsertVerbsParams{
		Lemmas: []string{"devour"}, Frames: []string{`["transitive"]`}, Definitions: []string{`[]`}, Source: "test",
	}); err != nil {
		t.Fatalf("seed verb: %v", err)
	}
	if err := q.UpsertAdjectives(ctx, store.UpsertAdjectivesParams{
		Lemmas: []string{"good"}, Definitions: []string{`[]`}, Source: "test",
	}); err != nil {
		t.Fatalf("seed adjective: %v", err)
	}
	if err := q.UpsertAdverbs(ctx, store.UpsertAdverbsParams{
		Lemmas: []string{"quickly"}, Definitions: []string{`[]`}, Source: "test",
	}); err != nil {
		t.Fatalf("seed adverb: %v", err)
	}
```

In `seedRareNoun`, replace the `InsertNoun` call with:

```go
	if err := store.New(testPool).UpsertNouns(ctx, store.UpsertNounsParams{
		Lemmas: []string{"goffer"}, Inflections: []string{`{}`}, Definitions: []string{`[]`}, Source: "test",
	}); err != nil {
		t.Fatalf("seed noun: %v", err)
	}
```

In `service/internal/lexicon/subtlex/apply_test.go` `seedLexicon`, replace the two loops and two single inserts with:

```go
	// "In" (indium) must not pick up the frequency of "in", and "john" (the
	// toilet) must not pick up the frequency of the name.
	nouns := []string{"goose", "America", "john", "In", "hot dog"}
	if err := q.UpsertNouns(ctx, store.UpsertNounsParams{
		Lemmas: nouns, Inflections: repeat("{}", len(nouns)), Definitions: repeat("[]", len(nouns)), Source: "test",
	}); err != nil {
		t.Fatalf("UpsertNouns: %v", err)
	}
	// SUBTLEX only has "baby" as a noun, so the verb gets no frequency.
	verbs := []string{"devour", "baby"}
	if err := q.UpsertVerbs(ctx, store.UpsertVerbsParams{
		Lemmas: verbs, Frames: repeat("[]", len(verbs)), Definitions: repeat("[]", len(verbs)), Source: "test",
	}); err != nil {
		t.Fatalf("UpsertVerbs: %v", err)
	}
	if err := q.UpsertAdjectives(ctx, store.UpsertAdjectivesParams{Lemmas: []string{"good"}, Definitions: []string{"[]"}, Source: "test"}); err != nil {
		t.Fatalf("UpsertAdjectives: %v", err)
	}
	if err := q.UpsertAdverbs(ctx, store.UpsertAdverbsParams{Lemmas: []string{"quickly"}, Definitions: []string{"[]"}, Source: "test"}); err != nil {
		t.Fatalf("UpsertAdverbs: %v", err)
	}
```

and add below `seedLexicon`:

```go
func repeat(s string, n int) []string {
	return slices.Repeat([]string{s}, n)
}
```

with `"slices"` added to the imports.

- [ ] **Step 6: Run the tests to verify they pass**

Run: `go vet ./... && go test ./...`
Expected: PASS. If `TestIngest`'s `stats` subtest fails on a count, the fixture has a lemma with two entries in one table; check the fixture before changing the number, and say so in the task report.

- [ ] **Step 7: Commit**

```bash
git add service/internal
git commit -m "feat: Upsert the lexicon so ingest keeps curation"
```

---

### Task 3: One transaction for the whole ingest

**Files:**
- Modify: `service/internal/lexicon/oewn/separable.go`, `service/internal/lexicon/subtlex/apply.go`, `service/internal/lexicon/closedclass/seed.go`, `service/cmd/ingest/main.go`
- Test: `service/internal/lexicon/closedclass/seed_test.go`

**Interfaces:**
- Consumes: `oewn.Ingest(ctx, store.DBTX, ...)` (Task 2).
- Produces: `oewn.MarkSeparable(ctx context.Context, db store.DBTX, r io.Reader) (int64, error)`, `subtlex.Apply(ctx context.Context, db store.DBTX, r io.Reader) (Stats, error)`, `closedclass.Seed(ctx context.Context, db store.DBTX, r io.Reader) (Stats, error)`. None begins or commits.

The separable and SUBTLEX tests pass `testPool`, which satisfies `store.DBTX`, and don't change: both parse before writing. The closed-class tests do change. In `TestSeedRejectsInvalidEntries`, the duplicate cases fail at an insert after the truncate, so "existing rows untouched" holds only if the caller rolls back. The tests therefore call `Seed` the way `cmd/ingest` does, in a transaction. The single transaction in `cmd/ingest` is glue, checked by hand in Step 5.

- [ ] **Step 1: Change the step signatures**

`separable.go`: change the parameter `pool *pgxpool.Pool` to `db store.DBTX`, use `store.New(db)`, drop the `pgxpool` import, and change the doc comment's last sentence to `It runs after Ingest, whose upsert clears the flags.`

`apply.go`: change `pool *pgxpool.Pool` to `db store.DBTX`, delete the `pool.Begin` block and its `defer tx.Rollback(ctx)`, set `q := store.New(db)`, and change `return stats, tx.Commit(ctx)` to `return stats, nil`. Drop the `pgxpool` import. Change the doc comment's last two sentences to: `It runs after the OEWN ingest, whose upsert clears old frequencies, and doesn't begin or commit a transaction.`

`seed.go`: change `pool *pgxpool.Pool` to `db store.DBTX`, delete the `pool.Begin` block and its `defer tx.Rollback(ctx)`, set `q := store.New(db)`, and change `return stats, tx.Commit(ctx)` to `return stats, nil`. Drop the `pgxpool` import. Change the doc comment to:

```go
// Seed validates r as closed-class TOML, then replaces the contents of all
// four tables. Seed doesn't begin or commit a transaction: pass the ingest
// run's. A validation error comes before any write, but a duplicate entry
// fails at its insert, after the truncate, so the caller must roll back.
```

In `service/internal/lexicon/closedclass/seed_test.go`, add a helper that runs `Seed` the way `cmd/ingest` does, and use it in place of every direct `closedclass.Seed(context.Background(), testPool, ...)` call (in `mustSeed`, `TestSeedRejectsInvalidEntries`, and `TestProjectClosedClassSeeds`):

```go
// seed runs Seed in a transaction, committing on success and rolling back on
// an error, as ingest does.
func seed(t *testing.T, r io.Reader) (closedclass.Stats, error) {
	t.Helper()
	ctx := context.Background()
	tx, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	defer tx.Rollback(ctx)
	stats, err := closedclass.Seed(ctx, tx, r)
	if err != nil {
		return stats, err
	}
	return stats, tx.Commit(ctx)
}
```

Add `"io"` to the imports if it isn't there.

- [ ] **Step 2: One transaction in `cmd/ingest`**

In `service/cmd/ingest/main.go`, after `defer db.Close()`, begin the transaction:

```go
	tx, err := db.Begin(ctx)
	if err != nil {
		log.Fatalf("begin: %v", err)
	}
	// Every step writes through tx, so any failure leaves the previous
	// lexicon. log.Fatalf skips this, and the rollback happens when the
	// connection closes.
	defer tx.Rollback(ctx)
```

Pass `tx` instead of `db` to `oewn.Ingest`, `oewn.MarkSeparable`, `subtlex.Apply`, and `closedclass.Seed`. After the closed-class `log.Printf`, add:

```go
	if err := tx.Commit(ctx); err != nil {
		log.Fatalf("commit: %v", err)
	}
	log.Print("committed")
```

- [ ] **Step 3: Run the tests**

Run: `go vet ./... && go test ./...`
Expected: PASS.

- [ ] **Step 4: Confirm the closed-class rollback test still bites**

Temporarily change the helper's `closedclass.Seed(ctx, tx, r)` to `closedclass.Seed(ctx, testPool, r)` and run `go test ./internal/lexicon/closedclass/ -run TestSeedRejectsInvalidEntries`.
Expected: the duplicate cases FAIL on "existing rows untouched", since each statement autocommits and the truncate sticks. Revert the change.

- [ ] **Step 5: Check by hand that curation survives a real ingest**

From the repo root:

```bash
just ingest
set -a; . ./.env; set +a
psql "$DATABASE_URL" -c "UPDATE nouns SET active = FALSE, plural_override = FALSE WHERE lemma = 'Taos'"
just ingest
psql "$DATABASE_URL" -c "SELECT lemma, active, plural_guess, plural_override, plural, json_array_length(definitions) FROM nouns WHERE lemma = 'Taos'"
psql "$DATABASE_URL" -c "UPDATE nouns SET active = TRUE, plural_override = NULL WHERE lemma = 'Taos'"
```

Expected: both runs end with `committed`; the select shows `active = f`, `plural_override = f`, `plural = f`, and the same definition count as before the second run. Note the second run's wall time for the task report.

- [ ] **Step 6: Commit**

```bash
git add service/internal/lexicon service/cmd/ingest
git commit -m "feat: Run every ingest step in one transaction"
```

---

### Task 4: Ingest on deploy, and the records

**Files:**
- Modify: `.github/workflows/deploy.yml`, `README.md`, `CLAUDE.md`

**Interfaces:**
- Consumes: `cmd/ingest` committing one transaction (Task 3).

No test: the deploy script runs only on the droplet. The first push after merge proves it, via the `ingest` phase marker in `~/randsense-deploy.log`.

- [ ] **Step 1: Add the ingest phase**

In `.github/workflows/deploy.yml`, change the `go build` line to:

```
              (cd service && /usr/local/go/bin/go build -o bin/randsense ./cmd/server && /usr/local/go/bin/go build -o bin/ingest ./cmd/ingest)
```

and between the `migrate` line and `phase "restart"`, add:

```
              phase "ingest"
              (cd service && bin/ingest)
```

`.env` is already exported by the `migrate` phase's `set -a`, so `DATABASE_URL` is set. Ingest runs after migrate, since it writes `plural_guess`.

- [ ] **Step 2: Update the README**

In `README.md`'s Deploy section, change the workflow sentence to: `` a deploy as the `deploy` user that pulls, builds, migrates, ingests, and restarts both services. `` Replace the "Deploys never ingest" paragraph and its code block with:

```markdown
Ingest updates the lexicon in place and keeps curation (`active`, `vote_count`, and
`nouns.plural_override`), so every deploy runs it. Lexicon changes ship with the code.
```

Check the README's other mentions of ingest (`grep -n -i ingest README.md`) for anything that still says it truncates or reloads, and fix those lines only.

- [ ] **Step 3: Update `CLAUDE.md`**

In roadmap item 4, delete the whole `**Prerequisite:**` bullet. In item 7, the sentence "It could also become where curation corrections live, so they survive re-ingest." is now false (they survive already). Change it to: "It could also carry curation corrections, so they travel with the lexicon."

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/deploy.yml README.md CLAUDE.md
git commit -m "feat: Ingest on every deploy"
```
