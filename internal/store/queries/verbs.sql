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
WHERE active
ORDER BY random()
LIMIT 1;

-- name: GetRandomVerbWithFrame :one
SELECT * FROM verbs
WHERE active AND frames ? @frame::text
ORDER BY random()
LIMIT 1;
