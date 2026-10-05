-- name: InsertDeterminer :exec
INSERT INTO determiners (lemma, type, number)
VALUES ($1, $2, $3);

-- name: TruncateDeterminers :exec
TRUNCATE determiners RESTART IDENTITY CASCADE;

-- name: ListDeterminers :many
SELECT * FROM determiners
ORDER BY lemma;

-- name: GetRandomDeterminer :one
SELECT * FROM determiners
WHERE active
ORDER BY random()
LIMIT 1;

-- name: GetRandomDeterminerWithNumber :one
SELECT * FROM determiners
WHERE active AND number = ANY(@numbers::text[])
ORDER BY random()
LIMIT 1;

-- name: LookupDeterminer :one
SELECT * FROM determiners
WHERE active AND lemma = $1;
