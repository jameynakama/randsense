-- name: UpsertVerbs :exec
-- One row per lemma: ingest pools the entries sharing one first, since an
-- upsert can't hit the same row twice. Sourced columns are replaced and
-- derived ones reset for the later ingest passes. Curated columns
-- (active, vote_count) are never written.
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

-- name: CountVerbs :one
SELECT COUNT(*) FROM verbs;

-- name: GetVerbByLemma :one
SELECT * FROM verbs
WHERE lemma = $1;

-- name: GetRandomVerb :one
SELECT * FROM verbs
WHERE active AND coalesce(frequency, 0) >= @commonness::float8
ORDER BY random()
LIMIT 1;

-- name: GetRandomVerbWithFrame :one
SELECT * FROM verbs
WHERE active AND frames ? @frame::text
  AND coalesce(frequency, 0) >= @commonness::float8
ORDER BY random()
LIMIT 1;

-- name: SetVerbFrequencies :execrows
-- Words are lowercase, so only lowercase lemmas match.
UPDATE verbs SET frequency = round(f.zipf::numeric, 2)
FROM (SELECT unnest(@words::text[]) AS word, unnest(@zipfs::float8[]) AS zipf) f
WHERE verbs.lemma = f.word;

-- name: SetSeparableVerbs :execrows
UPDATE verbs SET separable = TRUE
WHERE lemma = ANY(@lemmas::text[]);

-- name: LookupVerb :one
-- A locked word: the floor doesn't apply. An empty frame matches any verb.
SELECT * FROM verbs
WHERE active AND lemma = @lemma AND (@frame::text = '' OR frames ? @frame::text)
ORDER BY id
LIMIT 1;

-- name: GetVerbDefinitions :one
-- Active or not: an old sentence's word still shows its meaning.
SELECT definitions FROM verbs
WHERE lemma = $1
ORDER BY id
LIMIT 1;
