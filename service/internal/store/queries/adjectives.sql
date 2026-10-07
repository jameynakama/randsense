-- name: UpsertAdjectives :exec
-- One row per lemma: ingest pools the entries sharing one first, since an
-- upsert can't hit the same row twice. Sourced columns are replaced and
-- derived ones reset for the later ingest passes. Curated columns
-- (active, vote_count) are never written.
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

-- name: CountAdjectives :one
SELECT COUNT(*) FROM adjectives;

-- name: GetAdjectiveByLemma :one
SELECT * FROM adjectives
WHERE lemma = $1;

-- name: GetRandomAdjective :one
SELECT * FROM adjectives
WHERE active AND coalesce(frequency, 0) >= @commonness::float8
ORDER BY random()
LIMIT 1;

-- name: SetAdjectiveFrequencies :execrows
-- Words are lowercase, so only lowercase lemmas match.
UPDATE adjectives SET frequency = round(f.zipf::numeric, 2)
FROM (SELECT unnest(@words::text[]) AS word, unnest(@zipfs::float8[]) AS zipf) f
WHERE adjectives.lemma = f.word;

-- name: LookupAdjective :one
-- A locked word: the floor doesn't apply.
SELECT * FROM adjectives
WHERE active AND lemma = $1
ORDER BY id
LIMIT 1;

-- name: GetAdjectiveDefinitions :one
-- Active or not: an old sentence's word still shows its meaning.
SELECT definitions FROM adjectives
WHERE lemma = $1
ORDER BY id
LIMIT 1;
