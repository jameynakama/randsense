-- name: UpsertAdverbs :exec
-- One row per lemma: ingest pools the entries sharing one first, since an
-- upsert can't hit the same row twice. Sourced columns are replaced and
-- derived ones reset for the later ingest passes. Curated columns
-- (active, vote_count) are never written.
INSERT INTO adverbs (lemma, definitions, source)
SELECT run.lemma, run.definitions::jsonb, @source::text
FROM (
    SELECT unnest(@lemmas::text[]) AS lemma,
           unnest(@definitions::text[]) AS definitions
) run
ON CONFLICT (lemma, source) DO UPDATE SET
    definitions = EXCLUDED.definitions,
    frequency   = NULL;

-- name: DeleteStaleAdverbs :exec
-- An anti-join, not <> ALL, which would compare every row with every lemma.
DELETE FROM adverbs
WHERE source = @source::text
  AND NOT EXISTS (
    SELECT 1 FROM (SELECT unnest(@lemmas::text[]) AS lemma) run
    WHERE run.lemma = adverbs.lemma
  );

-- name: CountAdverbs :one
SELECT COUNT(*) FROM adverbs;

-- name: GetAdverbByLemma :one
SELECT * FROM adverbs
WHERE lemma = $1;

-- name: GetRandomAdverb :one
SELECT * FROM adverbs
WHERE active AND coalesce(frequency, 0) >= @commonness::float8
ORDER BY random()
LIMIT 1;

-- name: SetAdverbFrequencies :execrows
-- Words are lowercase, so only lowercase lemmas match.
UPDATE adverbs SET frequency = round(f.zipf::numeric, 2)
FROM (SELECT unnest(@words::text[]) AS word, unnest(@zipfs::float8[]) AS zipf) f
WHERE adverbs.lemma = f.word;

-- name: LookupAdverb :one
-- A locked word: the floor doesn't apply.
SELECT * FROM adverbs
WHERE active AND lemma = $1
ORDER BY id
LIMIT 1;

-- name: GetAdverbDefinitions :one
-- Active or not: an old sentence's word still shows its meaning.
SELECT definitions FROM adverbs
WHERE lemma = $1
ORDER BY id
LIMIT 1;
