-- name: InsertPreposition :exec
INSERT INTO prepositions (lemma)
VALUES ($1);

-- name: TruncatePrepositions :exec
TRUNCATE prepositions RESTART IDENTITY CASCADE;

-- name: ListPrepositions :many
SELECT * FROM prepositions
ORDER BY lemma;

-- name: GetRandomPreposition :one
SELECT * FROM prepositions
WHERE active
ORDER BY random()
LIMIT 1;

-- name: LookupPreposition :one
SELECT * FROM prepositions
WHERE active AND lemma = $1;
