-- name: InsertVerb :exec
INSERT INTO verbs (lemma, inflections, frames, source)
VALUES ($1, $2, $3, $4)
ON CONFLICT (lemma, source) DO NOTHING;

-- name: TruncateVerbs :exec
TRUNCATE verbs RESTART IDENTITY CASCADE;

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
