# Ingest upsert design

Ingest truncates and reloads the content-word tables, which wipes everything curation will write:
`active`, `vote_count`, and corrections to the heuristic plural flag. Ingest instead updates rows
in place, keeps curated columns, and runs on every deploy.

## Goals

- Re-running ingest keeps `active`, `vote_count`, and plural corrections.
- Running ingest twice gives identical tables: same ids, no doubled `definitions`.
- Sourced columns refresh on every run, so changes to filters, the `mislabeled` list, or lexicon
  TOML take effect.
- Deploys run ingest, so lexicon changes ship with the code.

## Non-goals

- Curation UI or API. This only makes curation survivable.
- Closed-class curation in the database. `closed_class.toml` is where those words are curated, and
  its tables still truncate and reload.
- Skipping ingest on deploys with no lexicon change. `mislabeled` lives in Go, so a path filter
  would miss changes, and an unchanged run costs only time.

## Decisions

- **Group in Go, upsert in place.** Ingest pools each table's entries by lemma in memory, then
  writes each lemma once. The upsert's `SET` list includes only sourced columns, so curation
  survives by construction, not by remembering to copy it. Staging tables were rejected because sqlc would
  need them in a migration and every derived-column query would be duplicated. Snapshotting
  curation around the truncate was rejected because ids would change on every deploy and each new
  curated column would need adding to the snapshot.
- **One transaction for the whole run.** The OEWN pass, separable verbs, SUBTLEX-US, and
  closed-class words commit together. The live site never sees words without frequencies, and a failure leaves
  the previous lexicon in place.
- **Plural corrections live in their own column.** `plural_guess` is ingest's heuristic,
  `plural_override` is curation, and `plural` is generated from the two. This keeps sourced and
  curated values apart, as the "Curation never edits source data" decision requires.
- **Words that leave the source are deleted.** The tables reflect the current sources and
  filters, and the row's curation goes with it. Nothing references word ids, and stored
  sentences keep their words in the tree.

## Schema

Migration `010_ingest_upsert`:

- Renames `nouns.plural` to `plural_guess`.
- Adds `nouns.plural_override BOOLEAN`. NULL means the guess stands.
- Adds `nouns.plural BOOLEAN GENERATED ALWAYS AS (coalesce(plural_override, plural_guess)) STORED`.

Readers of `plural` (`sentence.go`, `GetRandomSingularNoun`) don't change. The old server keeps
working between migrate and restart.

## Queries

For each of `nouns`, `verbs`, `adjectives`, and `adverbs`:

- `UpsertXs` replaces `InsertX`: one statement over `unnest` arrays of lemma, inflections, and
  definitions, plus frames for verbs. `ON CONFLICT (lemma, source) DO UPDATE` sets those columns
  from `EXCLUDED` and resets `frequency = NULL`, plus `separable = FALSE` on verbs and
  `plural_guess = FALSE` on nouns, for the later passes to refill. It never names `active`,
  `vote_count`, or `plural_override`.
- `DeleteStaleXs(source, lemmas)` deletes the rows from `source` whose lemma is not in `lemmas`.
- `TruncateX` is dropped.

`MarkPluralNouns` sets `plural_guess`.

## Ingest flow

`cmd/ingest` begins one transaction, passes it to each step, and commits at the end. Each step takes
a `store.DBTX` instead of a pool and no longer begins or commits a transaction. A transaction
and a pool both satisfy `store.DBTX`, so tests keep passing the pool:

1. `oewn.Ingest(ctx, tx, r, glosses)` parses with the same filters and frame mapping. It collects
   each table into an ordered map from lemma to pending row. A repeated lemma appends its
   definitions. The first entry supplies frames and inflections. Then, per table, it runs
   `DeleteStaleXs` with the run's lemmas, then `UpsertXs`, and finally `MarkPluralNouns`.
2. `oewn.MarkSeparable(ctx, tx, r)`.
3. `subtlex.Apply(ctx, tx, r)`.
4. `closedclass.Seed(ctx, tx, r)`, which still truncates its four tables. Readers of those tables
   wait on the truncate's lock until commit, about a second.
5. Commit. Any error rolls back the whole run.

`oewn.Stats` counts the distinct lemmas written per table, not entries, so a lemma with two OEWN
entries counts once.

## Deploy

- `deploy.yml`'s `go build` phase also builds `bin/ingest`.
- A new `ingest` phase runs `(cd service && bin/ingest)` after `migrate` and before `restart`.
  Data paths are relative to `service/`.

## Testing

In `internal/lexicon/oewn`:

- **Re-run:** ingesting the fixture twice gives the same row counts, ids, definitions, frames, and
  inflections, and the shared `a`/`s` adjective's definitions are not doubled.
- **Curation survives:** set `active = false`, `vote_count = 3`, and `plural_override = false` on a
  noun the heuristic marks plural, then re-ingest. All three remain, and `plural` reads false.
- **Override wins:** `plural_override = true` on a singular noun makes `plural` true.
- **Sourced columns refresh:** after hand edits to `definitions`, `frames`, `plural_guess`, and
  `frequency`, a re-run restores the sourced values and resets `frequency` to NULL.
- **Stale rows:** a seeded `oewn-2025` row whose lemma isn't in the fixture is deleted. A row from
  another source with a lemma the fixture lacks is kept.

The `subtlex` and separable tests don't change. The `closedclass` tests call `Seed` in a
transaction, the way `cmd/ingest` does: a duplicate entry fails after the truncate, so "existing rows
untouched" depends on the caller's rollback. There is no rollback test for `oewn.Ingest`: once the
steps take the caller's transaction, rollback is `cmd/ingest`'s job.
